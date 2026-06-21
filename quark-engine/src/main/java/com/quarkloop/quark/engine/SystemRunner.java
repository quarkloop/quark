package com.quarkloop.quark.engine;

import com.quarkloop.quark.core.domain.config.NodeConfig;
import com.quarkloop.quark.core.domain.identity.Namespace;
import com.quarkloop.quark.core.domain.system.NodeDefinition;
import com.quarkloop.quark.core.domain.system.SystemDefinition;
import com.quarkloop.quark.core.domain.spi.EndpointProvider;
import com.quarkloop.quark.core.domain.spi.FunctionProvider;
import com.quarkloop.quark.core.domain.spi.SourceProvider;
import com.quarkloop.quark.core.domain.spi.StoreProvider;
import com.quarkloop.quark.core.registry.NodeImplementationFactory;
import com.quarkloop.quark.core.registry.NodeRegistry;
import io.nats.client.Connection;
import io.nats.client.Message;
import io.nats.client.Nats;
import io.nats.client.Options;
import io.nats.client.impl.NatsMessage;
import jakarta.enterprise.context.ApplicationScoped;
import jakarta.inject.Inject;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import java.time.Duration;
import java.util.HashMap;
import java.util.HashSet;
import java.util.Map;
import java.util.Set;
import java.util.concurrent.ConcurrentHashMap;
import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;

/**
 * The engine: takes a {@link SystemDefinition} and executes it on NATS JetStream.
 *
 * <p>For each node:
 * <ol>
 *   <li>Resolves the URI via the registry to get a provider instance</li>
 *   <li>Creates a {@link NatsQuarkPublisher} scoped to the node (with ACL)</li>
 *   <li>If the node has {@code listens}, creates a NATS subscription</li>
 *   <li>If the node is a Source/Endpoint, calls {@code start()}</li>
 * </ol>
 *
 * <p>Messages flow through NATS — no direct method calls between nodes.
 */
@ApplicationScoped
public class SystemRunner {

    private static final Logger log = LoggerFactory.getLogger(SystemRunner.class);

    /** Config keys injected by the engine to give providers their runtime identity. */
    public static final String CONFIG_KEY_SYSTEM = "_quark_system";
    public static final String CONFIG_KEY_NAMESPACE = "_quark_namespace";
    public static final String CONFIG_KEY_NODE = "_quark_node";

    private final NodeRegistry registry;
    private final NatsConnectionManager natsConnectionManager;
    private final ExecutorService executor;

    // Active subscriptions for cleanup, keyed by system name + namespace
    private final java.util.List<io.nats.client.Dispatcher> dispatchers = new java.util.concurrent.CopyOnWriteArrayList<>();
    private final java.util.List<SourceProvider> activeSources = new java.util.concurrent.CopyOnWriteArrayList<>();
    private final java.util.List<EndpointProvider> activeEndpoints = new java.util.concurrent.CopyOnWriteArrayList<>();
    private final java.util.Map<String, RuntimeSystemDeployment> deployments = new java.util.concurrent.ConcurrentHashMap<>();

    @Inject
    public SystemRunner(NodeRegistry registry, NatsConnectionManager natsConnectionManager) {
        this.registry = registry;
        this.natsConnectionManager = natsConnectionManager;
        this.executor = Executors.newThreadPerTaskExecutor(
            Thread.ofVirtual().name("quark-engine-", 0).factory()
        );
    }

    /**
     * Deploy and start a system. Idempotent: re-deploying an existing
     * system first undeploys it cleanly.
     */
    public void deploy(SystemDefinition system) {
        String key = deploymentKey(system.namespace().value(), system.name());
        log.info("Deploying system {}/{} with {} nodes",
                system.namespace().value(), system.name(), system.nodes().size());

        // If already deployed, undeploy first (idempotent redeploy).
        RuntimeSystemDeployment existing = deployments.get(key);
        if (existing != null) {
            log.info("System {}/{} already deployed — undeploying first",
                    system.namespace().value(), system.name());
            undeploy(system.namespace().value(), system.name());
        }

        RuntimeSystemDeployment deployment = new RuntimeSystemDeployment(
                system.namespace().value(), system.name());

        for (NodeDefinition nodeDef : system.nodes().values()) {
            deployNode(system, nodeDef, deployment);
        }

        deployments.put(key, deployment);
        log.info("System {}/{} deployed successfully ({} nodes)",
                system.namespace().value(), system.name(), deployment.nodeCount());
    }

    /**
     * Undeploy a single system by namespace + name.
     */
    public void undeploy(String namespace, String systemName) {
        String key = deploymentKey(namespace, systemName);
        RuntimeSystemDeployment deployment = deployments.remove(key);
        if (deployment == null) {
            log.warn("Cannot undeploy: system {}/{} not found", namespace, systemName);
            return;
        }
        log.info("Undeploying system {}/{}", namespace, systemName);

        // Stop endpoints first (so HTTP clients get closed cleanly)
        for (EndpointProvider ep : deployment.endpoints()) {
            try { ep.stop(); } catch (Exception e) { log.warn("Failed to stop endpoint", e); }
        }
        // Stop sources
        for (SourceProvider sp : deployment.sources()) {
            try { sp.stop(); } catch (Exception e) { log.warn("Failed to stop source", e); }
        }
        // Close NATS dispatchers (if connected)
        Connection conn = natsConnectionManager.getConnection();
        if (conn != null) {
            for (io.nats.client.Dispatcher d : deployment.dispatchers()) {
                try { conn.closeDispatcher(d); } catch (Exception e) { log.warn("Failed to close dispatcher", e); }
            }
        }
        // Also clear from global lists (legacy)
        activeEndpoints.removeAll(deployment.endpoints());
        activeSources.removeAll(deployment.sources());
        dispatchers.removeAll(deployment.dispatchers());

        log.info("System {}/{} undeployed", namespace, systemName);
    }

    /**
     * Undeploy ALL systems (graceful shutdown).
     */
    public void undeployAll() {
        log.info("Undeploying all systems");
        for (RuntimeSystemDeployment d : new java.util.ArrayList<>(deployments.values())) {
            undeploy(d.namespace(), d.systemName());
        }
        // Shutdown the executor only when fully done
        if (!executor.isShutdown()) {
            executor.shutdown();
        }
    }

    /**
     * Legacy no-arg undeploy — undeploys everything. Kept for backward
     * compatibility with {@code QuarkServer.onStop}.
     */
    public void undeploy() {
        undeployAll();
    }

    public int activeSystemCount() {
        return deployments.size();
    }

    @SuppressWarnings("unchecked")
    private void deployNode(SystemDefinition system, NodeDefinition nodeDef,
                             RuntimeSystemDeployment deployment) {
        log.info("Deploying node {} ({})", nodeDef.name(), nodeDef.uri());

        // 1. Resolve the provider via registry
        var factoryOpt = registry.lookupFactory(nodeDef.uri());
        if (factoryOpt.isEmpty()) {
            throw new IllegalStateException("No factory registered for URI: " + nodeDef.uri());
        }

        NodeImplementationFactory<?> factory = factoryOpt.get();

        // 2. Inject runtime identity into the config so providers (esp.
        //    EndpointProvider) can know their own (system, namespace, node)
        //    without the SPI needing to expose it.
        Map<String, Object> configWithMeta = new HashMap<>(nodeDef.config().asMap());
        configWithMeta.put(CONFIG_KEY_SYSTEM, system.name());
        configWithMeta.put(CONFIG_KEY_NAMESPACE, system.namespace().value());
        configWithMeta.put(CONFIG_KEY_NODE, nodeDef.name());
        com.quarkloop.quark.core.domain.config.NodeConfig config =
                com.quarkloop.quark.core.domain.config.NodeConfig.of(configWithMeta);

        Object provider = factory.create(config);

        // 3. If NATS is available, create publisher and wire subscriptions.
        //    If NATS is NOT available (degraded mode), skip wiring — the
        //    system is still tracked in the runtime registry and lifecycle
        //    events are emitted, but no messages flow.
        Connection natsConnection = natsConnectionManager.getConnection();
        if (natsConnection != null) {
            // Create publisher with ACL
            Set<String> allowedEvents = new HashSet<>(nodeDef.events());
            NatsQuarkPublisher publisher = new NatsQuarkPublisher(
                    natsConnection,
                    system.name(),
                    system.namespace().value(),
                    nodeDef.name(),
                    allowedEvents
            );

            // If the node has listens, create NATS subscriptions
            if (!nodeDef.listens().isEmpty()) {
                createSubscriptions(system, nodeDef, provider, publisher, deployment, natsConnection);
            }

            // If the node is a Source or Endpoint, call start()
            if (provider instanceof SourceProvider sp) {
                sp.start(publisher, config);
                activeSources.add(sp);
                deployment.addSource(sp);
            } else if (provider instanceof EndpointProvider ep) {
                ep.start(publisher, config);
                activeEndpoints.add(ep);
                deployment.addEndpoint(ep);
            }
        } else {
            log.warn("NATS not connected — skipping wiring for node {} (degraded mode)", nodeDef.name());
        }

        deployment.incrementNodeCount();
        log.info("Node {} deployed (listens={}, events={}, nats={})",
                nodeDef.name(), nodeDef.listens(), nodeDef.events(),
                natsConnection != null ? "connected" : "disconnected");
    }

    private void createSubscriptions(SystemDefinition system, NodeDefinition nodeDef,
                                      Object provider, NatsQuarkPublisher publisher,
                                      RuntimeSystemDeployment deployment,
                                      Connection natsConnection) {
        io.nats.client.Dispatcher dispatcher = natsConnection.createDispatcher(msg -> {
            // This callback runs on NATS dispatcher thread
            try {
                NatsQuarkMessage quarkMsg = new NatsQuarkMessage(
                        msg, system.name(), system.namespace().value(), nodeDef.name());

                // Dispatch to the provider's onMessage()
                if (provider instanceof FunctionProvider fp) {
                    fp.onMessage(quarkMsg, publisher);
                } else if (provider instanceof StoreProvider sp) {
                    sp.onMessage(quarkMsg, publisher);
                } else if (provider instanceof EndpointProvider ep) {
                    ep.onMessage(quarkMsg, publisher);
                } else if (provider instanceof com.quarkloop.quark.core.domain.spi.PolicyProvider pp) {
                    pp.onMessage(quarkMsg, publisher);
                }

                // Acknowledge the message
                msg.ack();
            } catch (Exception e) {
                log.error("Error processing message on {} for node {}",
                        msg.getSubject(), nodeDef.name(), e);
                // NAK the message — NATS will retry if consumer is configured for retries
                msg.nak();
            }
        });

        // Subscribe to each listens subject
        for (String relativeSubject : nodeDef.listens()) {
            String fullSubject = SubjectResolver.resolve(
                    system.name(), system.namespace(), relativeSubject);
            dispatcher.subscribe(fullSubject);
            log.debug("Node {} subscribed to {}", nodeDef.name(), fullSubject);
        }

        dispatchers.add(dispatcher);
        deployment.addDispatcher(dispatcher);
    }

    private static String deploymentKey(String namespace, String systemName) {
        return namespace + "/" + systemName;
    }

    /**
     * Tracks per-deployment runtime resources so undeploy can clean up
     * only the providers/dispatchers belonging to one system.
     */
    private static final class RuntimeSystemDeployment {
        private final String namespace;
        private final String systemName;
        private final java.util.List<SourceProvider> sources = new java.util.concurrent.CopyOnWriteArrayList<>();
        private final java.util.List<EndpointProvider> endpoints = new java.util.concurrent.CopyOnWriteArrayList<>();
        private final java.util.List<io.nats.client.Dispatcher> dispatchers = new java.util.concurrent.CopyOnWriteArrayList<>();
        private volatile int nodeCount = 0;

        RuntimeSystemDeployment(String namespace, String systemName) {
            this.namespace = namespace;
            this.systemName = systemName;
        }

        String namespace() { return namespace; }
        String systemName() { return systemName; }
        int nodeCount() { return nodeCount; }
        void incrementNodeCount() { nodeCount++; }
        java.util.List<SourceProvider> sources() { return sources; }
        java.util.List<EndpointProvider> endpoints() { return endpoints; }
        java.util.List<io.nats.client.Dispatcher> dispatchers() { return dispatchers; }
        void addSource(SourceProvider sp) { sources.add(sp); }
        void addEndpoint(EndpointProvider ep) { endpoints.add(ep); }
        void addDispatcher(io.nats.client.Dispatcher d) { dispatchers.add(d); }
    }
}
