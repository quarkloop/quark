package com.quarkloop.quark.core.registry.inmemory;

import com.quarkloop.quark.core.registry.NodeImplementationFactory;
import com.quarkloop.quark.core.registry.NodeRegistry;
import io.quarkus.runtime.StartupEvent;
import jakarta.annotation.Priority;
import jakarta.enterprise.context.ApplicationScoped;
import jakarta.enterprise.event.Observes;
import jakarta.enterprise.inject.Instance;
import jakarta.inject.Inject;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

/**
 * Initializes the {@link NodeRegistry} by discovering all
 * {@link NodeImplementationFactory} CDI beans and registering them.
 *
 * <p>Runs at {@link Priority#MIN_PRIORITY} (lower number = earlier) on
 * {@link StartupEvent} so that the registry is fully populated BEFORE
 * other startup observers (e.g. {@code DeployService.recoverFromDisk()})
 * attempt to look up node URIs. Without this explicit priority, CDI does
 * not guarantee observer ordering, and recovery could fail with
 * "Unknown node URIs" if it ran before the registry was populated.
 */
@ApplicationScoped
public class RegistryInitializer {

    private static final Logger log = LoggerFactory.getLogger(RegistryInitializer.class);

    @Inject
    Instance<NodeImplementationFactory> factories;

    @Inject
    NodeRegistry registry;

    void onStart(@Observes @Priority(1) StartupEvent event) {
        int count = 0;
        for (NodeImplementationFactory factory : factories) {
            registry.register(factory);
            count++;
        }
        log.info("Initialized Node Registry with {} implementations", count);
    }
}
