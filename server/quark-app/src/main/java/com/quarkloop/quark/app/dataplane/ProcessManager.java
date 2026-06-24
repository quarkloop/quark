package com.quarkloop.quark.app.dataplane;

import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.datatype.jsr310.JavaTimeModule;
import com.quarkloop.quark.core.engine.dataplane.DataPlaneIpc;
import com.quarkloop.quark.core.engine.dataplane.DataPlaneProcess;
import io.quarkus.runtime.ShutdownEvent;
import io.quarkus.runtime.StartupEvent;
import jakarta.annotation.Priority;
import jakarta.enterprise.context.ApplicationScoped;
import jakarta.enterprise.event.Observes;
import jakarta.inject.Inject;
import org.eclipse.microprofile.config.inject.ConfigProperty;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import java.io.IOException;
import java.nio.file.Files;
import java.nio.file.Path;
import java.nio.file.Paths;
import java.util.Map;
import java.util.concurrent.ConcurrentHashMap;
import java.util.concurrent.ConcurrentMap;
import java.util.concurrent.TimeUnit;

/**
 * Manages the lifecycle of data-plane JVM processes.
 *
 * <p>The control plane delegates system deployment to data-plane processes:
 * <ul>
 *   <li><b>Shared runtime</b> — one data-plane process ({@code runtimeId="shared"})
 *       hosts all non-isolated namespaces.</li>
 *   <li><b>Isolated runtime</b> — a dedicated data-plane process
 *       ({@code runtimeId="ns-<namespace>"}) per namespace with
 *       {@code runtime: "isolated"} in the .quark.ts file.</li>
 * </ul>
 *
 * <p>Process lifecycle:
 * <ol>
 *   <li>{@link #ensureProcess(String)} — lazily spawns a data-plane process
 *       for the given runtimeId if one doesn't already exist.</li>
 *   <li>The process connects to the same NATS server and listens for
 *       deploy/undeploy commands on its runtimeId-scoped subjects.</li>
 *   <li>On shutdown ({@code @Observes ShutdownEvent}), all data-plane
 *       processes are gracefully stopped.</li>
 * </ul>
 *
 * <p>Restart policy: if a data-plane process crashes unexpectedly, the next
 * {@link #ensureProcess(String)} call will detect it's dead and spawn a
 * replacement. The ProcessManager does NOT auto-restart on crash — it
 * restarts on the next deploy/recover request. This is intentional: an
 * empty data-plane process (no systems) has no work to do, so there's no
 * need to keep it alive if it crashes while idle.
 */
@ApplicationScoped
public class ProcessManager {

    private static final Logger log = LoggerFactory.getLogger(ProcessManager.class);

    private final ObjectMapper mapper = new ObjectMapper();
    { mapper.registerModule(new JavaTimeModule()); }

    @ConfigProperty(name = "quark.state.root", defaultValue = "./quark-state")
    String stateRootPath;

    @ConfigProperty(name = "quark.nats.url", defaultValue = "nats://localhost:4222")
    String natsUrl;

    @ConfigProperty(name = "quarkus.http.port", defaultValue = "8080")
    int controlHttpPort;

    /** runtimeId → DataPlaneProcess */
    private final ConcurrentMap<String, DataPlaneProcess> processes = new ConcurrentHashMap<>();

    /** Base HTTP port for data-plane processes (control port + 100). */
    private static final int DATA_PLANE_PORT_BASE = 9100;

    /** Port counter for assigning unique ports to each data-plane process. */
    private int nextPort = DATA_PLANE_PORT_BASE;

    /**
     * On startup, log the ProcessManager configuration.
     * Runs at priority 2 (after RegistryInitializer at priority 1).
     */
    void onStart(@Observes @Priority(2) StartupEvent event) {
        log.info("ProcessManager started (stateRoot={}, nats={}, controlHttp={})",
                stateRootPath, natsUrl, controlHttpPort);
    }

    /**
     * On shutdown, stop all data-plane processes gracefully.
     */
    void onStop(@Observes ShutdownEvent event) {
        log.info("Stopping {} data-plane process(es)", processes.size());
        for (DataPlaneProcess proc : processes.values()) {
            try {
                proc.stop();
            } catch (Exception e) {
                log.warn("Error stopping data-plane process {}", proc.runtimeId(), e);
            }
        }
        processes.clear();
    }

    /**
     * Ensure a data-plane process exists for the given runtimeId.
     *
     * <p>If a process already exists and is alive, returns it immediately.
     * If a process exists but is dead, removes it and spawns a new one.
     * If no process exists, spawns a new one.
     *
     * @param runtimeId the data-plane runtime identifier
     * @return the running DataPlaneProcess
     * @throws IOException if the process cannot be started
     */
    public synchronized DataPlaneProcess ensureProcess(String runtimeId) throws IOException {
        DataPlaneProcess existing = processes.get(runtimeId);
        if (existing != null && existing.isAlive()) {
            return existing;
        }
        if (existing != null) {
            log.info("Data-plane process {} is dead — spawning replacement", runtimeId);
            processes.remove(runtimeId);
        }

        String binary = findBinary();
        int port = nextPort++;
        DataPlaneProcess proc = new DataPlaneProcess(
                runtimeId, binary, stateRootPath, natsUrl, port);
        proc.start();

        // Wait for the process to be ready (up to 30s)
        if (!proc.waitForReady(30)) {
            log.error("Data-plane process {} did not become ready in 30s", runtimeId);
            proc.stop();
            throw new IOException("Data-plane process " + runtimeId + " failed to start");
        }
        processes.put(runtimeId, proc);
        log.info("Data-plane process {} ready (PID {}, port {})",
                runtimeId, proc.pid(), port);
        return proc;
    }

    /**
     * Stop a specific data-plane process by runtimeId.
     * Used when an isolated namespace's last system is undeployed.
     */
    public synchronized void stopProcess(String runtimeId) {
        DataPlaneProcess proc = processes.remove(runtimeId);
        if (proc != null) {
            proc.stop();
        }
    }

    /**
     * Check whether a data-plane process exists and is alive for the given
     * runtimeId.
     */
    public boolean isProcessAlive(String runtimeId) {
        DataPlaneProcess proc = processes.get(runtimeId);
        return proc != null && proc.isAlive();
    }

    /**
     * Get all running data-plane processes (for status/health endpoints).
     */
    public Map<String, DataPlaneProcess> getAllProcesses() {
        return Map.copyOf(processes);
    }

    /**
     * Locate the data-plane binary (native executable or JAR).
     *
     * <p>After the core/server/runtime split, the data plane runs the
     * <b>runtime</b> module's binary ({@code runtime/quark-runtime}), which
     * includes GraalJS/Truffle. The control plane runs the <b>server</b>
     * module's binary, which is GraalJS-free.
     *
     * <p>Search order (first match wins):
     * <ol>
     *   <li><b>Native binary</b> at {@code runtime/quark-runtime/target/quark-runtime-runner}
     *       (produced by {@code mvn -pl runtime/quark-runtime -am -Pnative install}).</li>
     *   <li><b>Native binary</b> relative to the state root's parent.</li>
     *   <li><b>JVM JAR</b> at {@code runtime/quark-runtime/target/quark-runtime-runner-0.1.0-SNAPSHOT-runner.jar}.</li>
     *   <li><b>JVM JAR</b> relative to the state root's parent.</li>
     *   <li><b>Fallback</b>: the old single-binary layout — {@code quark-server/target/quark-server-0.1.0-SNAPSHOT-runner}
     *       (native) or {@code .jar} (JVM). Kept for backwards compatibility.</li>
     * </ol>
     *
     * @return the absolute path to the binary (native or JAR)
     * @throws IOException if no binary is found
     */
    private String findBinary() throws IOException {
        // 1. Native runtime binary at standard Maven target path.
        //    Quarkus's native-image output is named <finalName>-runner
        //    where finalName is set in the pom (we set it to "quark-runtime-runner",
        //    so the binary is "quark-runtime-runner-runner").
        Path nativeMaven = Paths.get("runtime", "quark-runtime", "target",
                "quark-runtime-runner-runner");
        if (Files.isExecutable(nativeMaven)) {
            return nativeMaven.toAbsolutePath().toString();
        }
        // 2. Native runtime binary relative to state root's parent
        Path nativeState = Paths.get(stateRootPath).toAbsolutePath().resolve("..")
                .resolve("runtime").resolve("quark-runtime").resolve("target")
                .resolve("quark-runtime-runner-runner").normalize();
        if (Files.isExecutable(nativeState)) {
            return nativeState.toString();
        }

        // 3. JVM JAR at standard Maven target path.
        //    Quarkus produces TWO jars: <finalName>.jar (thin, no main manifest)
        //    and <finalName>-runner.jar (fat, runnable). We must use the -runner
        //    variant. With finalName=quark-runtime-runner, the runnable jar is
        //    quark-runtime-runner-runner.jar (yes, double -runner suffix).
        Path jarMaven = Paths.get("runtime", "quark-runtime", "target",
                "quark-runtime-runner-runner.jar");
        if (Files.isRegularFile(jarMaven)) {
            return jarMaven.toAbsolutePath().toString();
        }
        // 4. JVM JAR relative to state root's parent
        Path jarState = Paths.get(stateRootPath).toAbsolutePath().resolve("..")
                .resolve("runtime").resolve("quark-runtime").resolve("target")
                .resolve("quark-runtime-runner-runner.jar").normalize();
        if (Files.isRegularFile(jarState)) {
            return jarState.toString();
        }

        // 5. Fallback: legacy single-binary layout (quark-server in data mode)
        Path legacyNative = Paths.get("quark-server", "target",
                "quark-server-0.1.0-SNAPSHOT-runner");
        if (Files.isExecutable(legacyNative)) {
            log.warn("Using legacy quark-server binary as data plane — GraalJS will be missing in native mode. " +
                    "Build runtime/quark-runtime instead.");
            return legacyNative.toAbsolutePath().toString();
        }
        Path legacyJar = Paths.get("quark-server", "target",
                "quark-server-0.1.0-SNAPSHOT-runner.jar");
        if (Files.isRegularFile(legacyJar)) {
            return legacyJar.toAbsolutePath().toString();
        }

        // 6. JVM JAR from java.class.path (IDE / fat classpath)
        String classPath = System.getProperty("java.class.path");
        if (classPath != null) {
            for (String entry : classPath.split(java.io.File.pathSeparator)) {
                if (entry.contains("quark-runtime-runner") && entry.endsWith(".jar")) {
                    return entry;
                }
            }
        }

        throw new IOException("Cannot find data-plane binary (native or JAR). " +
                "Searched: " + nativeMaven + ", " + nativeState + ", " +
                jarMaven + ", " + jarState + ", " + legacyNative + ", " + legacyJar);
    }
}
