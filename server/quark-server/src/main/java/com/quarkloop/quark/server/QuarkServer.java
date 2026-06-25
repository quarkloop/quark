package com.quarkloop.quark.server;

import io.quarkus.runtime.Quarkus;
import io.quarkus.runtime.QuarkusApplication;
import io.quarkus.runtime.ShutdownEvent;
import io.quarkus.runtime.StartupEvent;
import jakarta.enterprise.context.ApplicationScoped;
import jakarta.enterprise.event.Observes;
import org.eclipse.microprofile.config.inject.ConfigProperty;
import org.jboss.logging.Logger;

import com.quarkloop.quark.runtime.engine.lifecycle.SystemDeployer;

import jakarta.inject.Inject;

/**
 * Quarkus application entry point.
 *
 * <p>Supports two modes controlled by the {@code quark.mode} config property:
 * <ul>
 *   <li><b>standalone</b> (default) — the control plane. Runs the REST API,
 *       Catalog persistence, and the {@link ProcessManager} that spawns
 *       data-plane processes for system execution.</li>
 *   <li><b>data</b> — a data-plane process. Connects to NATS, subscribes to
 *       deploy/undeploy command subjects (scoped by {@code runtimeId}), and
 *       executes systems locally via {@link SystemDeployer}. The REST API
 *       is disabled (no Swagger, no OpenAPI) — the HTTP server is used only
 *       for health checks on a high port.</li>
 * </ul>
 *
 * <p>Run as control plane:
 * <pre>
 *   java -jar quark-server/target/quark-server-0.1.0-SNAPSHOT-runner.jar
 * </pre>
 *
 * <p>Run as data-plane process (spawned automatically by ProcessManager):
 * <pre>
 *   java -Dquark.mode=data -Dquark.dataplane.runtimeId=shared \
 *        -jar quark-server/target/quark-server-0.1.0-SNAPSHOT-runner.jar
 * </pre>
 */
@ApplicationScoped
public class QuarkServer implements QuarkusApplication {

    private static final Logger log = Logger.getLogger(QuarkServer.class);

    @ConfigProperty(name = "quark.state.root", defaultValue = "./quark-state")
    String stateRoot;

    @ConfigProperty(name = "quarkus.http.port", defaultValue = "8080")
    int httpPort;

    @ConfigProperty(name = "quark.version", defaultValue = "0.1.0-SNAPSHOT")
    String version;

    @ConfigProperty(name = "quark.mode", defaultValue = "standalone")
    String mode;

    @ConfigProperty(name = "quark.dataplane.runtimeId", defaultValue = "")
    String runtimeId;

    @Inject
    SystemDeployer systemDeployer;

    void onStart(@Observes StartupEvent event) {
        if ("data".equals(mode)) {
            log.infof("=== Quark Data-Plane v%s starting (runtimeId=%s) ===", version, runtimeId);
            log.infof("HTTP port: %d (health-only)", httpPort);
            log.infof("State root: %s", stateRoot);
        } else {
            log.infof("=== Quark Platform v%s starting (control plane) ===", version);
            log.infof("HTTP port: %d", httpPort);
            log.infof("State root: %s", stateRoot);
            log.infof("Swagger UI: http://localhost:%d/swagger-ui", httpPort);
            log.infof("OpenAPI:     http://localhost:%d/openapi", httpPort);
            log.infof("REST API:    http://localhost:%d/systems", httpPort);
        }
    }

    void onStop(@Observes ShutdownEvent event) {
        if ("data".equals(mode)) {
            log.infof("=== Quark Data-Plane %s shutting down ===", runtimeId);
        } else {
            log.info("=== Quark Platform shutting down ===");
        }
        try {
            systemDeployer.undeployAll();
        } catch (Exception e) {
            log.warn("Error during SystemDeployer shutdown", e);
        }
        if ("data".equals(mode)) {
            log.infof("=== Quark Data-Plane %s stopped ===", runtimeId);
        } else {
            log.info("=== Quark Platform stopped ===");
        }
    }

    @Override
    public int run(String... args) throws Exception {
        Quarkus.waitForExit();
        return 0;
    }
}
