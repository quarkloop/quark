package com.quarkloop.quark.core.engine.lifecycle;

import com.quarkloop.quark.core.domain.category.NodeCategory;
import com.quarkloop.quark.core.domain.config.NodeConfig;
import com.quarkloop.quark.core.domain.identity.Namespace;
import com.quarkloop.quark.core.domain.metadata.NodeMetadata;
import com.quarkloop.quark.core.domain.node.*;
import com.quarkloop.quark.core.domain.spi.*;
import com.quarkloop.quark.core.domain.system.NodeDefinition;
import com.quarkloop.quark.core.domain.system.SystemDefinition;
import com.quarkloop.quark.core.engine.bus.MessageBus;
import com.quarkloop.quark.core.engine.bus.Subscription;
import com.quarkloop.quark.core.engine.runtime.RuntimeContext;
import com.quarkloop.quark.core.registry.NodeDescriptor;
import com.quarkloop.quark.core.registry.NodeImplementationFactory;
import com.quarkloop.quark.core.registry.NodeRegistry;
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

    @Inject
    public SystemDeployer(NodeRegistry registry, LifecycleManager lifecycleManager,
                           MessageBus messageBus, RuntimeContext runtimeContext) {
        this.registry = registry;
        this.lifecycleManager = lifecycleManager;
        this.messageBus = messageBus;
        this.runtimeContext = runtimeContext;
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

            Map<String, NodeImplementationFactory<?>> factories = new HashMap<>();
            List<String> missingUris = new ArrayList<>();
            for (NodeDefinition def : system.nodes().values()) {
                Optional<NodeImplementationFactory<?>> f = registry.lookupFactory(def.uri());
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

        for (EndpointProvider ep : runtimeContext.getEndpoints(ns, name))
            try { ep.stop(); } catch (Exception e) { log.warn("Failed to stop endpoint", e); }
        for (SourceProvider sp : runtimeContext.getSources(ns, name))
            try { sp.stop(); } catch (Exception e) { log.warn("Failed to stop source", e); }
        for (Subscription sub : runtimeContext.getSubscriptions(ns, name))
            try { sub.close(); } catch (Exception e) { log.warn("Failed to close subscription", e); }

        lifecycleManager.stopAll(runtime);
        runtimeContext.removeSystem(ns, name);
        runtimeContext.clear(ns, name);
        log.info("System {}/{} undeployed", ns, name);
    }

    private void deployNode(SystemDefinition system, NodeDefinition def,
                             NodeImplementationFactory<?> factory, RuntimeSystem runtime) {
        log.info("Deploying node {} ({})", def.name(), def.uri());

        Map<String, Object> configWithMeta = new HashMap<>(def.config().asMap());
        configWithMeta.put(CONFIG_KEY_SYSTEM, system.name());
        configWithMeta.put(CONFIG_KEY_NAMESPACE, system.namespace().value());
        configWithMeta.put(CONFIG_KEY_NODE, def.name());
        NodeConfig config = NodeConfig.of(configWithMeta);

        Object provider;
        try { provider = factory.create(config); }
        catch (Exception e) {
            throw new DeploymentException("Factory " + factory.getClass().getName() +
                    " failed for " + def.uri() + " (" + def.name() + "): " + e.getMessage(), e);
        }

        Set<String> allowedEvents = new HashSet<>(def.events());
        QuarkPublisher publisher = messageBus.createPublisher(
                system.name(), system.namespace().value(), def.name(), allowedEvents);

        if (!def.listens().isEmpty())
            createSubscriptions(system, def, provider, publisher);

        if (provider instanceof SourceProvider sp) {
            sp.start(publisher, config);
            runtimeContext.recordSource(system.namespace().value(), system.name(), def.name(), sp);
        } else if (provider instanceof EndpointProvider ep) {
            ep.start(publisher, config);
            runtimeContext.recordEndpoint(system.namespace().value(), system.name(), def.name(), ep);
        }

        Node domainNode = createDomainNode(def, factory.descriptor());
        runtime.register(new RuntimeNode(domainNode, provider));
        log.info("Node {} deployed (listens={}, events={}, bus={})",
                def.name(), def.listens(), def.events(), messageBus.isConnected() ? "connected" : "degraded");
    }

    private void createSubscriptions(SystemDefinition system, NodeDefinition def,
                                      Object provider, QuarkPublisher publisher) {
        List<Subscription> subs = new ArrayList<>();
        for (String rel : def.listens()) {
            String full = system.name() + "." + system.namespace().value() + "." + rel;
            subs.add(messageBus.subscribe(full, msg -> dispatchToProvider(msg, provider, publisher)));
            log.debug("Node {} subscribed to {}", def.name(), full);
        }
        runtimeContext.recordSubscriptions(system.namespace().value(), system.name(), def.name(), subs);
    }

    private void dispatchToProvider(QuarkMessage msg, Object provider, QuarkPublisher publisher) {
        if (provider instanceof FunctionProvider fp) fp.onMessage(msg, publisher);
        else if (provider instanceof StoreProvider sp) sp.onMessage(msg, publisher);
        else if (provider instanceof EndpointProvider ep) ep.onMessage(msg, publisher);
        else if (provider instanceof PolicyProvider pp) pp.onMessage(msg, publisher);
    }

    private Node createDomainNode(NodeDefinition def, NodeDescriptor descriptor) {
        NodeMetadata metadata = NodeMetadata.initial().withLabels(def.labels()).withAnnotations(def.annotations());
        return switch (descriptor.category()) {
            case SOURCE -> new Source(def.name(), def.uri(), def.config(), metadata);
            case FUNCTION -> new Function(def.name(), def.uri(), def.config(), metadata);
            case STORE -> new Store(def.name(), def.uri(), def.config(), metadata);
            case ENDPOINT -> new Endpoint(def.name(), def.uri(), def.config(), metadata);
            case POLICY -> new Policy(def.name(), def.uri(), def.config(), metadata);
        };
    }
}
