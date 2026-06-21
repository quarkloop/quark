package com.quarkloop.quark.core.engine.lifecycle;

import com.quarkloop.quark.core.domain.category.NodeCategory;
import com.quarkloop.quark.core.domain.identity.Namespace;
import com.quarkloop.quark.core.domain.metadata.NodeMetadata;
import com.quarkloop.quark.core.domain.node.Endpoint;
import com.quarkloop.quark.core.domain.node.Function;
import com.quarkloop.quark.core.domain.node.Node;
import com.quarkloop.quark.core.domain.node.Policy;
import com.quarkloop.quark.core.domain.node.Source;
import com.quarkloop.quark.core.domain.node.Store;
import com.quarkloop.quark.core.domain.system.NodeDefinition;
import com.quarkloop.quark.core.domain.system.SystemDefinition;
import com.quarkloop.quark.core.registry.NodeDescriptor;
import com.quarkloop.quark.core.registry.NodeImplementationFactory;
import com.quarkloop.quark.core.registry.NodeRegistry;
import jakarta.enterprise.context.ApplicationScoped;
import jakarta.inject.Inject;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import java.util.ArrayList;
import java.util.List;
import java.util.Optional;

/**
 * Deploys a system: resolves every node URI against the registry,
 * instantiates the SPI provider via the factory, creates the domain
 * {@link Node} object, registers it with the {@link SystemRuntimeRegistry},
 * and asks the {@link LifecycleManager} to start everything.
 */
@ApplicationScoped
public class SystemDeployer {

    private static final Logger log = LoggerFactory.getLogger(SystemDeployer.class);

    private final NodeRegistry registry;
    private final LifecycleManager lifecycleManager;
    private final SystemRuntimeRegistry runtimeRegistry;

    @Inject
    public SystemDeployer(NodeRegistry registry,
                            LifecycleManager lifecycleManager,
                            SystemRuntimeRegistry runtimeRegistry) {
        this.registry = registry;
        this.lifecycleManager = lifecycleManager;
        this.runtimeRegistry = runtimeRegistry;
    }

    public RuntimeSystem deploy(SystemDefinition system) {
        log.info("Deploying system {}/{}", system.namespace().value(), system.name());

        runtimeRegistry.get(system.namespace(), system.name())
                .ifPresent(existing -> {
                    log.info("Undeploying existing system before redeploy");
                    lifecycleManager.stopAll(existing);
                    runtimeRegistry.remove(system.namespace(), system.name());
                });

        RuntimeSystem runtime = new RuntimeSystem(system, system.namespace());

        List<String> missingUris = new ArrayList<>();
        for (NodeDefinition def : system.nodes().values()) {
            Optional<NodeImplementationFactory<?>> factoryOpt = registry.lookupFactory(def.uri());
            if (factoryOpt.isEmpty()) {
                missingUris.add(def.uri().toString());
                continue;
            }

            NodeImplementationFactory<?> factory = factoryOpt.get();
            Object spiProvider;
            try {
                spiProvider = factory.create(def.config());
            } catch (Exception e) {
                throw new DeploymentException(
                        "Factory " + factory.getClass().getName() + " failed to create provider for " +
                                def.uri() + " (" + def.name() + "): " + e.getMessage(), e);
            }

            Node domainObj = createDomainNode(def, factory.descriptor());
            RuntimeNode rr = new RuntimeNode(domainObj, spiProvider);
            runtime.register(rr);
        }

        if (!missingUris.isEmpty()) {
            throw new DeploymentException("Unknown node URIs (not registered): " + missingUris);
        }

        runtimeRegistry.register(runtime);
        lifecycleManager.startAll(runtime);

        log.info("System {}/{} deployed successfully ({} nodes)",
                system.namespace().value(), system.name(), runtime.nodes().size());
        return runtime;
    }

    public void undeploy(Namespace namespace, String systemName) {
        runtimeRegistry.get(namespace, systemName).ifPresentOrElse(
                runtime -> {
                    lifecycleManager.stopAll(runtime);
                    runtimeRegistry.remove(namespace, systemName);
                    log.info("Undeployed system {}/{}", namespace.value(), systemName);
                },
                () -> log.warn("Cannot undeploy: system {}/{} not found", namespace.value(), systemName)
        );
    }

    private Node createDomainNode(NodeDefinition def, NodeDescriptor descriptor) {
        NodeMetadata metadata = NodeMetadata.initial()
                .withLabels(def.labels())
                .withAnnotations(def.annotations());

        NodeCategory category = descriptor.category();

        return switch (category) {
            case SOURCE -> new Source(def.name(), def.uri(), def.config(), metadata);
            case FUNCTION -> new Function(def.name(), def.uri(), def.config(), metadata);
            case STORE -> new Store(def.name(), def.uri(), def.config(), metadata);
            case ENDPOINT -> new Endpoint(def.name(), def.uri(), def.config(), metadata);
            case POLICY -> new Policy(def.name(), def.uri(), def.config(), metadata);
            case system -> throw new DeploymentException(
                    "System-as-node composition is not yet supported. Node " +
                            def.name() + " references system URI " + def.uri());
        };
    }
}
