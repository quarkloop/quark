package com.quarkloop.quark.core.engine.dataplane;

import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import java.io.IOException;
import java.nio.file.Files;
import java.nio.file.Path;
import java.nio.file.Paths;
import java.util.ArrayList;
import java.util.List;
import java.util.concurrent.TimeUnit;
import java.util.concurrent.atomic.AtomicBoolean;

/**
 * Represents a spawned data-plane JVM process.
 *
 * <p>Each data-plane process runs the Quark server JAR with
 * {@code -Dquark.mode=data} and a unique {@code -Dquark.dataplane.runtimeId}.
 * The process connects to the same NATS server as the control plane and
 * listens for deploy/undeploy commands on its runtimeId-scoped subjects.
 *
 * <p>Lifecycle:
 * <ol>
 *   <li>{@link #start()} — spawns the JVM via {@link ProcessBuilder}.</li>
 *   <li>The process runs until {@link #stop()} is called or it crashes.</li>
 *   <li>{@link #isAlive()} — checks whether the process is still running.</li>
 *   <li>{@link #stop()} — sends SIGTERM, waits up to 5s, then SIGKILLs if needed.</li>
 * </ol>
 *
 * <p>Stdout/stderr from the data-plane process is piped to log files under
 * {@code $STATE_ROOT/dataplane-logs/} so operators can inspect data-plane
 * output without it flooding the control-plane console.
 */
public class DataPlaneProcess {

    private static final Logger log = LoggerFactory.getLogger(DataPlaneProcess.class);

    private final String runtimeId;
    private final String serverJarPath;
    private final String stateRootPath;
    private final String natsUrl;
    private final int httpPort;

    private Process process;
    private final AtomicBoolean started = new AtomicBoolean(false);

    /**
     * @param runtimeId    the data-plane runtime identifier ("shared" or "ns-&lt;namespace&gt;")
     * @param serverJarPath path to the quark-server runner JAR
     * @param stateRootPath the platform state root (for DuckDB + log files)
     * @param natsUrl      the NATS URL to connect to
     * @param httpPort     the HTTP port (data-plane uses port + 100 to avoid conflict)
     */
    public DataPlaneProcess(String runtimeId, String serverJarPath,
                             String stateRootPath, String natsUrl, int httpPort) {
        this.runtimeId = runtimeId;
        this.serverJarPath = serverJarPath;
        this.stateRootPath = stateRootPath;
        this.natsUrl = natsUrl;
        this.httpPort = httpPort;
    }

    /**
     * Spawn the data-plane JVM process.
     *
     * @throws IOException if the process cannot be started
     */
    public synchronized void start() throws IOException {
        if (started.get()) {
            log.warn("Data-plane process {} already started", runtimeId);
            return;
        }

        List<String> command = new ArrayList<>();
        command.add(ProcessHandle.current().info().command().orElse("java"));
        // JVM flags
        command.add("-Dquark.mode=data");
        command.add("-Dquark.dataplane.runtimeId=" + runtimeId);
        command.add("-Dquark.state.root=" + stateRootPath);
        command.add("-Dquark.nats.url=" + natsUrl);
        // Data-plane HTTP port: control port + 100 + runtimeId hash to avoid conflicts
        command.add("-Dquarkus.http.port=" + httpPort);
        command.add("-Dquarkus.http.host=127.0.0.1");
        // Disable Swagger/OpenAPI in data-plane (not needed, saves memory)
        command.add("-Dquarkus.swagger-ui.always-include=false");
        // The JAR
        command.add("-jar");
        command.add(serverJarPath);

        Path logDir = Paths.get(stateRootPath, "dataplane-logs");
        Files.createDirectories(logDir);
        Path stdoutLog = logDir.resolve("dataplane-" + runtimeId + ".log");

        log.info("Starting data-plane process {} (HTTP port {}, logs at {})", runtimeId, httpPort, stdoutLog);
        log.debug("Command: {}", String.join(" ", command));

        ProcessBuilder pb = new ProcessBuilder(command)
                .redirectOutput(stdoutLog.toFile())
                .redirectErrorStream(true);
        process = pb.start();
        started.set(true);

        // Start a monitor thread that logs process exit
        Thread monitor = Thread.ofPlatform()
                .name("dataplane-" + runtimeId + "-monitor")
                .daemon(true)
                .start(() -> {
                    try {
                        int exitCode = process.waitFor();
                        if (started.get()) {
                            log.warn("Data-plane process {} exited with code {} (unexpected)", runtimeId, exitCode);
                        } else {
                            log.info("Data-plane process {} exited with code {} (shutdown)", runtimeId, exitCode);
                        }
                    } catch (InterruptedException e) {
                        Thread.currentThread().interrupt();
                    }
                });
    }

    /**
     * Check whether the data-plane process is still running.
     */
    public synchronized boolean isAlive() {
        return process != null && process.isAlive();
    }

    /**
     * Get the OS-level PID of the data-plane process, or -1 if not running.
     */
    public synchronized long pid() {
        if (process == null) return -1;
        return process.pid();
    }

    /**
     * Stop the data-plane process gracefully.
     *
     * <p>Sends SIGTERM (via {@link Process#destroy()}), waits up to 5 seconds
     * for the process to exit, then sends SIGKILL ({@link Process#destroyForcibly()})
     * if it's still alive.
     */
    public synchronized void stop() {
        if (!started.get() || process == null) {
            log.debug("Data-plane process {} not running — stop() is a no-op", runtimeId);
            return;
        }
        started.set(false);
        log.info("Stopping data-plane process {} (PID {})", runtimeId, process.pid());
        process.destroy();
        try {
            if (!process.waitFor(5, TimeUnit.SECONDS)) {
                log.warn("Data-plane process {} did not exit in 5s — force killing", runtimeId);
                process.destroyForcibly();
                process.waitFor(2, TimeUnit.SECONDS);
            }
        } catch (InterruptedException e) {
            Thread.currentThread().interrupt();
            process.destroyForcibly();
        }
        log.info("Data-plane process {} stopped", runtimeId);
    }

    /**
     * Wait for the data-plane process to be ready (polling its health endpoint).
     *
     * @param timeoutSeconds max time to wait
     * @return true if the process became ready, false if it timed out
     */
    public boolean waitForReady(int timeoutSeconds) {
        String healthUrl = "http://127.0.0.1:" + httpPort + "/health/live";
        long deadline = System.currentTimeMillis() + (timeoutSeconds * 1000L);
        while (System.currentTimeMillis() < deadline) {
            if (!isAlive()) return false;
            try {
                Process curl = new ProcessBuilder("curl", "-sf", healthUrl).start();
                if (curl.waitFor(2, TimeUnit.SECONDS) && curl.exitValue() == 0) {
                    return true;
                }
            } catch (Exception ignored) {
                // curl not available or not ready yet
            }
            try {
                Thread.sleep(500);
            } catch (InterruptedException e) {
                Thread.currentThread().interrupt();
                return false;
            }
        }
        return false;
    }

    public String runtimeId() {
        return runtimeId;
    }
}
