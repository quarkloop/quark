package com.quarkloop.quark.core.registry.inmemory;

import com.quarkloop.quark.core.registry.NodeImplementationFactory;
import com.quarkloop.quark.core.registry.NodeRegistry;
import io.quarkus.runtime.StartupEvent;
import jakarta.enterprise.context.ApplicationScoped;
import jakarta.enterprise.event.Observes;
import jakarta.enterprise.inject.Instance;
import jakarta.inject.Inject;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

@ApplicationScoped
public class RegistryInitializer {

    private static final Logger log = LoggerFactory.getLogger(RegistryInitializer.class);

    @Inject
    Instance<NodeImplementationFactory<?>> factories;

    @Inject
    NodeRegistry registry;

    void onStart(@Observes StartupEvent event) {
        int count = 0;
        for (NodeImplementationFactory<?> factory : factories) {
            registry.register(factory);
            count++;
        }
        log.info("Initialized Node Registry with {} implementations", count);
    }
}
