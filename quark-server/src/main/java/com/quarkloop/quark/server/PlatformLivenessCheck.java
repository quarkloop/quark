package com.quarkloop.quark.server;

import com.quarkloop.quark.core.engine.lifecycle.SystemRuntimeRegistry;
import jakarta.enterprise.context.ApplicationScoped;
import jakarta.inject.Inject;
import org.eclipse.microprofile.health.HealthCheck;
import org.eclipse.microprofile.health.HealthCheckResponse;
import org.eclipse.microprofile.health.Liveness;

/**
 * Liveness check — confirms the runtime registry is responsive.
 *
 * <p>If this check fails, Kubernetes (or any orchestrator) would restart
 * the pod. The check is trivial because the registry is in-memory; the
 * real signal of liveness is that the JVM is running and CDI is functional.
 */
@Liveness
@ApplicationScoped
public class PlatformLivenessCheck implements HealthCheck {

    private final SystemRuntimeRegistry runtimeRegistry;

    @Inject
    public PlatformLivenessCheck(SystemRuntimeRegistry runtimeRegistry) {
        this.runtimeRegistry = runtimeRegistry;
    }

    @Override
    public HealthCheckResponse call() {
        int systemCount = runtimeRegistry.all().size();
        return HealthCheckResponse.named("platform-liveness")
                .up()
                .withData("systems", (long) systemCount)
                .build();
    }
}
