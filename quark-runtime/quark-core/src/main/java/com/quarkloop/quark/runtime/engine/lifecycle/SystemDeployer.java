package com.quarkloop.quark.runtime.engine.lifecycle;

import com.quarkloop.quark.runtime.domain.config.NodeConfig;
import com.quarkloop.quark.runtime.domain.identity.Namespace;
import com.quarkloop.quark.runtime.domain.metadata.NodeMetadata;
import com.quarkloop.quark.runtime.domain.node.SimpleNode;
import com.quarkloop.quark.runtime.domain.spi.NodeProvider;
import com.quarkloop.quark.runtime.domain.spi.QuarkMessage;
import com.quarkloop.quark.runtime.domain.spi.QuarkPublisher;
import com.quarkloop.quark.runtime.domain.system.NodeDefinition;
import com.quarkloop.quark.runtime.domain.system.SystemDefinition;
import com.quarkloop.quark.runtime.engine.bus.MessageBus;
import com.quarkloop.quark.runtime.engine.bus.Subscription;
import com.quarkloop.quark.runtime.engine.metrics.NamespaceMetrics;
import com.quarkloop.quark.runtime.engine.polyglot.PolyglotNodeLookup;
import com.quarkloop.quark.runtime.engine.runtime.RuntimeContext;
import com.quarkloop.quark.runtime.registry.NodeImplementationFactory;
import com.quarkloop.quark.runtime.registry.NodeRegistry;
import jakarta.annotation.PreDestroy;
import jakarta.enterprise.context.ApplicationScoped;
import jakarta.inject.Inject;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import java.util.*;

@ApplicationScoped
public class SystemDeployer {

    private static final Logger log = LoggerFactory.getLogger(SystemDeployer.class);

    public static final String CONFIG_KEY_SYSTEM = "_quark_system";
    public static final String CONFIG_KEY_NAMESPACE = "_quark_namespace";
    public static final String CONFIG_KEY_NODE = "_quark_node";

    private final NodeRegistry registry;
    private final LifecycleManager lifecycleManager;
    private final MessageBus messageBus;
    private final RuntimeContext runtimeContext;
    private final NamespaceMetrics namespaceMetrics;
    private final PolyglotNodeLookup polyglotLookup;

    @Inject
    public SystemDeployer(NodeRegistry registry, LifecycleManager lifecycleManager,
                           MessageBus messageBus, RuntimeContext runtimeContext,
                           NamespaceMetrics namespaceMetrics,
                           PolyglotNodeLookup polyglotLookup) {
        this.registry = registry;
        this.lifecycleManager = lifecycleManager;
        this.messageBus = messageBus;
        this.runtimeContext = runtimeContext;
        this.namespaceMetrics = namespaceMetrics;
        this.polyglotLookup = polyglotLookup;
    }

    public RuntimeSystem deploy(SystemDefinition system) {
        String ns = system.namespace().value();
        String name = system.name();
        log.info("Deploying system {}/{} with {} nodes", ns, name, system.nodes().size());

        runtimeContext.lock(ns, name);
        try {
            if (runtimeContext.getSystem(ns, name).isPresent()) {
                log.info("System {}/{} already deployed — undeploying first", ns, name);
                undeployInternal(ns, name);
            }

            Map<String, NodeImplementationFactory> factories = new HashMap<>();
            List<String> missingUris = new ArrayList<>();
            for (NodeDefinition def : system.nodes().values()) {
                Optional<NodeImplementationFactory> f = registry.lookupFactory(def.uri());
                if (f.isEmpty()) {
                    f = polyglotLookup.lookupFactory(def.uri());
                }
                if (f.isEmpty()) missingUris.add(def.uri().toString());
                else factories.put(def.name(), f.get());
            }
            if (!missingUris.isEmpty())
                throw new DeploymentException("Unknown node URIs (not registered): " + missingUris);

            RuntimeSystem runtime = new RuntimeSystem(system, system.namespace());

            for (NodeDefinition def : system.nodes().values())
                deployNode(system, def, factories.get(def.name()), runtime);

            runtimeContext.registerSystem(runtime);
            lifecycleManager.startAll(runtime);

            log.info("System {}/{} deployed ({} nodes, bus={})",
                    ns, name, runtime.nodes().size(), messageBus.isConnected() ? "connected" : "degraded");
            return runtime;
        } finally {
            runtimeContext.unlock(ns, name);
        }
    }

    public void undeploy(String namespace, String systemName) {
        runtimeContext.lock(namespace, systemName);
        try { undeployInternal(namespace, systemName); }
        finally { runtimeContext.unlock(namespace, systemName); }
    }

    @PreDestroy
    public void undeployAll() {
        log.info("Undeploying all systems");
        for (RuntimeSystem rs : new ArrayList<>(runtimeContext.getAllSystems()))
            undeploy(rs.namespace().value(), rs.name());
    }

    private void undeployInternal(String ns, String name) {
        Optional<RuntimeSystem> opt = runtimeContext.getSystem(ns, name);
        if (opt.isEmpty()) { log.warn("Cannot undeploy: system {}/{} not found", ns, name); return; }
        RuntimeSystem runtime = opt.get();
        log.info("Undeploying system {}/{}", ns, name);

        // Close all startable providers (call close() on every NodeProvider)
        for (NodeProvider provider : runtimeContext.getStartableProviders(ns, name))
            try { provider.close(); } catch (Exception e) { log.warn("Failed to close provider", e); }
        // Close all subscriptions
        for (Subscription sub : runtimeContext.getSubscriptions(ns, name))
            try { sub.close(); } catch (Exception e) { log.warn("Failed to close subscription", e); }

        lifecycleManager.stopAll(runtime);
        runtimeContext.removeSystem(ns, name);
        runtimeContext.clear(ns, name);
        log.info("System {}/{} undeployed", ns, name);
    }

    private void deployNode(SystemDefinition system, NodeDefinition def,
                             NodeImplementationFactory factory, RuntimeSystem runtime) {
        log.info("Deploying node {} ({})", def.name(), def.uri());

        Map<String, Object> configWithMeta = new HashMap<>(def.config().asMap());
        configWithMeta.put(CONFIG_KEY_SYSTEM, system.name());
        configWithMeta.put(CONFIG_KEY_NAMESPACE, system.namespace().value());
        configWithMeta.put(CONFIG_KEY_NODE, def.name());
        NodeConfig config = NodeConfig.of(configWithMeta);

        NodeProvider provider;
        try { provider = factory.create(config); }
        catch (Exception e) {
            throw new DeploymentException("Factory " + factory.getClass().getName() +
                    " failed for " + def.uri() + " (" + def.name() + "): " + e.getMessage(), e);
        }

        // Initialize the provider
        provider.init(config);

        Set<String> allowedEvents = new HashSet<>(def.events());
        QuarkPublisher publisher = messageBus.createPublisher(
                system.name(), system.namespace().value(), def.name(), allowedEvents);

        // If the node has listens, create subscriptions that call onMessage
        if (!def.listens().isEmpty())
            createSubscriptions(system, def, provider, publisher);

        // If the node has no listens (autonomous), call start()
        // The engine calls start() for all providers — the default no-op
        // makes this safe for providers that don't override it.
        if (def.listens().isEmpty()) {
            provider.start(publisher, config);
            runtimeContext.recordStartableProvider(system.namespace().value(), system.name(), def.name(), provider);
        } else {
            // For nodes with listens, also record them as startable so close() is called on undeploy
            runtimeContext.recordStartableProvider(system.namespace().value(), system.name(), def.name(), provider);
        }

        var metadata = NodeMetadata.initial().withLabels(def.labels()).withAnnotations(def.annotations());
        var domainNode = new SimpleNode(def.name(), def.uri(), def.config(), metadata);
        runtime.register(new RuntimeNode(domainNode, provider));
        log.info("Node {} deployed (listens={}, events={}, bus={})",
                def.name(), def.listens(), def.events(), messageBus.isConnected() ? "connected" : "degraded");
    }

    private void createSubscriptions(SystemDefinition system, NodeDefinition def,
                                      NodeProvider provider, QuarkPublisher publisher) {
        List<Subscription> subs = new ArrayList<>();
        String namespace = system.namespace().value();
        for (String rel : def.listens()) {
            String full = namespace + "." + system.name() + "." + rel;
            subs.add(messageBus.subscribe(full, msg -> dispatchToProvider(msg, provider, publisher, namespace)));
            log.debug("Node {} subscribed to {}", def.name(), full);
        }
        runtimeContext.recordSubscriptions(namespace, system.name(), def.name(), subs);
    }

    private void dispatchToProvider(QuarkMessage msg, NodeProvider provider, QuarkPublisher publisher, String namespace) {
        long cpuBefore = namespaceMetrics.getCurrentThreadCpuTimeNanos();
        try {
            provider.onMessage(msg, publisher);
            long cpuAfter = namespaceMetrics.getCurrentThreadCpuTimeNanos();
            long cpuDelta = cpuBefore >= 0 && cpuAfter >= 0 ? cpuAfter - cpuBefore : -1;
            namespaceMetrics.recordMessageHandled(namespace, cpuDelta);
        } catch (Exception e) {
            namespaceMetrics.recordError(namespace);
            throw e;
        }
    }
}
