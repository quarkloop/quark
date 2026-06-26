package com.quarkloop.quark.runtime.runner;

import com.quarkloop.quark.runtime.engine.lifecycle.SystemDeployer;
import io.quarkus.runtime.Quarkus;
import io.quarkus.runtime.QuarkusApplication;
import io.quarkus.runtime.ShutdownEvent;
import io.quarkus.runtime.StartupEvent;
import jakarta.enterprise.context.ApplicationScoped;
import jakarta.enterprise.event.Observes;
import jakarta.inject.Inject;
import org.eclipse.microprofile.config.inject.ConfigProperty;
import org.jboss.logging.Logger;

/**
 * Quarkus application entry point for the data plane.
 *
 * <p>The runtime runner is the data-plane counterpart to the control-plane
 * {@code QuarkServer}. It is intentionally minimal:
 * <ul>
 *   <li>No REST API (no Swagger, no OpenAPI, no resource classes).
 *   <li>Only the SmallRye Health endpoint on {@code /q/health/*} is exposed.
 *   <li>On startup, {@link DataPlaneCommandHandler} subscribes to NATS
 *       subjects scoped by {@code quark.dataplane.runtimeId} and waits
 *       for deploy/undeploy commands from the control plane.
 *   <li>On shutdown, {@link SystemDeployer#undeployAll()} is called to
 *       cleanly stop all running systems.
 * </ul>
 *
 * <p>Run as a data-plane process (spawned by ProcessManager):
 * <pre>
 *   java -Dquark.mode=data -Dquark.dataplane.runtimeId=shared \
 *        -jar runtime/quark-runtime/target/quark-runtime-runner-0.1.0-SNAPSHOT-runner.jar
 * </pre>
 *
 * <p>In native mode (preferred for production):
 * <pre>
 *   runtime/quark-runtime/target/quark-runtime-runner-0.1.0-SNAPSHOT-runner \
 *       -Dquark.mode=data -Dquark.dataplane.runtimeId=shared
 * </pre>
 */
@ApplicationScoped
public class QuarkRuntime implements QuarkusApplication {

    private static final Logger log = Logger.getLogger(QuarkRuntime.class);

    @ConfigProperty(name = "quark.state.root", defaultValue = "./quark-state")
    String stateRoot;

    @ConfigProperty(name = "quarkus.http.port", defaultValue = "8080")
    int httpPort;

    @ConfigProperty(name = "quark.version", defaultValue = "0.1.0-SNAPSHOT")
    String version;

    @ConfigProperty(name = "quark.dataplane.runtimeId", defaultValue = "")
    String runtimeId;

    @Inject
    SystemDeployer systemDeployer;

    void onStart(@Observes StartupEvent event) {
        log.infof("=== Quark Runtime v%s starting (runtimeId=%s) ===", version, runtimeId);
        log.infof("HTTP port: %d (health-only)", httpPort);
        log.infof("State root: %s", stateRoot);
        log.infof("GraalJS/Truffle: enabled (data plane can execute TypeScript nodes)");
    }

    void onStop(@Observes ShutdownEvent event) {
        log.infof("=== Quark Runtime %s shutting down ===", runtimeId);
        try {
            systemDeployer.undeployAll();
        } catch (Exception e) {
            log.warn("Error during SystemDeployer shutdown", e);
        }
        log.infof("=== Quark Runtime %s stopped ===", runtimeId);
    }

    @Override
    public int run(String... args) throws Exception {
        Quarkus.waitForExit();
        return 0;
    }
}
