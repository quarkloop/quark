package com.quarkloop.quark.server;

import io.quarkus.runtime.Quarkus;
import io.quarkus.runtime.QuarkusApplication;
import io.quarkus.runtime.ShutdownEvent;
import io.quarkus.runtime.StartupEvent;
import jakarta.enterprise.context.ApplicationScoped;
import jakarta.enterprise.event.Observes;
import org.eclipse.microprofile.config.inject.ConfigProperty;
import org.jboss.logging.Logger;

import com.quarkloop.quark.engine.SystemRunner;

import jakarta.inject.Inject;

/**
 * Quarkus application entry point.
 *
 * <p>Quarkus auto-discovers this via the {@code quarkus-maven-plugin} (it scans
 * for classes implementing {@link QuarkusApplication}). We use it for clean
 * startup/shutdown hooks:
 *
 * <ul>
 *   <li><b>Startup</b>: log the active state root, port, and version so
 *       operators can verify the configuration at a glance.</li>
 *   <li><b>Shutdown</b>: gracefully shut down the {@link SystemRunner}'s
 *       virtual-thread executor so in-flight system executions don't get
 *       killed mid-chain.</li>
 * </ul>
 *
 * <p>Run with: {@code java -jar quark-server/target/quarkus-app/quarkus-run.jar}
 * (in production) or {@code ./mvnw quarkus:dev} (in dev with hot reload).
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

    @Inject
    SystemRunner systemRunner;

    void onStart(@Observes StartupEvent event) {
        log.infof("=== Quark Platform v%s starting ===", version);
        log.infof("HTTP port: %d", httpPort);
        log.infof("State root: %s", stateRoot);
        log.infof("Swagger UI: http://localhost:%d/swagger-ui", httpPort);
        log.infof("OpenAPI:     http://localhost:%d/openapi", httpPort);
        log.infof("REST API:    http://localhost:%d/systems", httpPort);
    }

    void onStop(@Observes ShutdownEvent event) {
        log.info("=== Quark Platform shutting down ===");
        try {
            systemRunner.undeploy();
        } catch (Exception e) {
            log.warn("Error during SystemRunner shutdown", e);
        }
        log.info("=== Quark Platform stopped ===");
    }

    @Override
    public int run(String... args) throws Exception {
        Quarkus.waitForExit();
        return 0;
    }
}
