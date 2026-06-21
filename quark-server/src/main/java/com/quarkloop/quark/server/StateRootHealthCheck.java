package com.quarkloop.quark.server;

import com.quarkloop.quark.adapter.state.StateRoot;
import jakarta.enterprise.context.ApplicationScoped;
import jakarta.inject.Inject;
import org.eclipse.microprofile.health.HealthCheck;
import org.eclipse.microprofile.health.HealthCheckResponse;
import org.eclipse.microprofile.health.HealthCheckResponseBuilder;
import org.eclipse.microprofile.health.Readiness;

import java.nio.file.Files;
import java.nio.file.Path;

/**
 * Readiness check that verifies the platform state root is writable.
 *
 * <p>If the state root is not accessible (permission denied, disk full,
 * path is a file not a directory, etc.) the platform cannot persist
 * system declarations or events, so it's not ready to serve requests.
 */
@Readiness
@ApplicationScoped
public class StateRootHealthCheck implements HealthCheck {

    private final StateRoot stateRoot;

    @Inject
    public StateRootHealthCheck(StateRoot stateRoot) {
        this.stateRoot = stateRoot;
    }

    @Override
    public HealthCheckResponse call() {
        Path root = stateRoot.path();
        boolean ok = false;
        String reason = null;
        try {
            if (!Files.exists(root)) {
                Files.createDirectories(root);
            }
            if (!Files.isDirectory(root)) {
                reason = "State root path is not a directory: " + root;
            } else if (!Files.isWritable(root)) {
                reason = "State root is not writable: " + root;
            } else {
                ok = true;
            }
        } catch (Exception e) {
            reason = "State root inaccessible: " + e.getMessage();
        }
        HealthCheckResponseBuilder builder = HealthCheckResponse.named("state-root")
                .status(ok);
        if (reason != null) builder.withData("reason", reason);
        builder.withData("path", root.toString());
        return builder.build();
    }
}
