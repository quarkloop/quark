package com.quarkloop.quark.runtime.engine.dataplane;

import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import java.io.IOException;
import java.net.URI;
import java.net.http.HttpClient;
import java.net.http.HttpRequest;
import java.net.http.HttpResponse;
import java.nio.file.Files;
import java.nio.file.Path;
import java.nio.file.Paths;
import java.time.Duration;
import java.util.ArrayList;
import java.util.List;
import java.util.concurrent.TimeUnit;
import java.util.concurrent.atomic.AtomicBoolean;

/**
 * Represents a spawned data-plane process (JVM or native).
 *
 * <p>Each data-plane process runs the Quark server binary with
 * {@code -Dquark.mode=data} (JVM mode) or as a native binary with
 * {@code quark.mode=data} passed as a system property. The process connects
 * to the same NATS server as the control plane and listens for deploy/undeploy
 * commands on its runtimeId-scoped subjects.
 *
 * <h2>Binary detection</h2>
 * The {@code binaryPath} may point to either:
 * <ul>
 *   <li>A JAR file ({@code .jar} extension) — JVM mode: the process is spawned
 *       as {@code java -jar <path>} with JVM flags.</li>
 *   <li>A native executable (no {@code .jar} extension) — native mode: the
 *       process is spawned directly as the binary, with system properties
 *       passed via {@code -D} flags that the Quarkus native runtime reads from
 *       command-line arguments.</li>
 * </ul>
 *
 * <p>Lifecycle:
 * <ol>
 *   <li>{@link #start()} — spawns the process via {@link ProcessBuilder}.</li>
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
    private final String binaryPath;
    private final String stateRootPath;
    private final String natsUrl;
    private final int httpPort;

    private Process process;
    private final AtomicBoolean started = new AtomicBoolean(false);

    /**
     * @param runtimeId    the data-plane runtime identifier ("shared" or "ns-&lt;namespace&gt;")
     * @param binaryPath   path to the quark-server binary (JAR for JVM mode, native executable for native mode)
     * @param stateRootPath the platform state root (for catalog database + log files)
     * @param natsUrl      the NATS URL to connect to
     * @param httpPort     the HTTP port for the data-plane process
     */
    public DataPlaneProcess(String runtimeId, String binaryPath,
                             String stateRootPath, String natsUrl, int httpPort) {
        this.runtimeId = runtimeId;
        this.binaryPath = binaryPath;
        this.stateRootPath = stateRootPath;
        this.natsUrl = natsUrl;
        this.httpPort = httpPort;
    }

    /**
     * @return true if the binary path ends with {@code .jar} (JVM mode);
     *         false if it's a native executable
     */
    private boolean isJarBinary() {
        return binaryPath.endsWith(".jar");
    }

    /**
     * Spawn the data-plane process.
     *
     * <p>In JVM mode: {@code java -D... -jar <path>}
     * In native mode: {@code <path> -D...}
     *
     * @throws IOException if the process cannot be started
     */
    public synchronized void start() throws IOException {
        if (started.get()) {
            log.warn("Data-plane process {} already started", runtimeId);
            return;
        }

        List<String> command = new ArrayList<>();

        if (isJarBinary()) {
            // JVM mode: java -D... -jar <path>
            command.add(resolveJavaExecutable());
            addSystemProperties(command, "-D");
            command.add("-jar");
            command.add(binaryPath);
        } else {
            // Native mode: <binary> -D...
            command.add(binaryPath);
            addSystemProperties(command, "-D");
        }

        Path logDir = Paths.get(stateRootPath, "dataplane-logs");
        Files.createDirectories(logDir);
        Path stdoutLog = logDir.resolve("dataplane-" + runtimeId + ".log");

        log.info("Starting data-plane process {} (mode={}, HTTP port={}, logs at {})",
                runtimeId, isJarBinary() ? "jvm" : "native", httpPort, stdoutLog);
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
                            log.warn("Data-plane process {} exited with code {} (unexpected)",
                                    runtimeId, exitCode);
                        } else {
                            log.info("Data-plane process {} exited with code {} (shutdown)",
                                    runtimeId, exitCode);
                        }
                    } catch (InterruptedException e) {
                        Thread.currentThread().interrupt();
                    }
                });
    }

    /**
     * Add system properties to the command list with the given prefix.
     * In JVM mode, these are {@code -D} flags before {@code -jar}.
     * In native mode, Quarkus native reads {@code -D} flags as command-line args.
     */
    private void addSystemProperties(List<String> command, String prefix) {
        command.add(prefix + "quark.mode=data");
        command.add(prefix + "quark.dataplane.runtimeId=" + runtimeId);
        command.add(prefix + "quark.state.root=" + stateRootPath);
        command.add(prefix + "quark.nats.url=" + natsUrl);
        command.add(prefix + "quarkus.http.port=" + httpPort);
        command.add(prefix + "quarkus.http.host=127.0.0.1");
        // Disable Swagger/OpenAPI in data-plane (not needed, saves memory)
        command.add(prefix + "quarkus.swagger-ui.always-include=false");
        // In native mode, pass quark.native=true so providers know to use
        // platform threads (Truffle JIT doesn't support virtual threads).
        if (!isJarBinary()) {
            command.add(prefix + "quark.native=true");
        }
    }

    /**
     * Resolve the Java executable path.
     *
     * Uses {@code JAVA_HOME/bin/java} if set, otherwise falls back to the
     * {@code java} on {@code PATH}. In native mode this method is not called
     * (the native binary runs directly).
     */
    private String resolveJavaExecutable() {
        String javaHome = System.getenv("JAVA_HOME");
        if (javaHome != null && !javaHome.isBlank()) {
            Path javaPath = Paths.get(javaHome, "bin", "java");
            if (Files.isExecutable(javaPath)) {
                return javaPath.toString();
            }
        }
        // Fall back to the java that's running the control plane
        return ProcessHandle.current().info().command().orElse("java");
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
     * <p>Uses Java's built-in {@link HttpClient} instead of spawning a {@code curl}
     * subprocess — this works in both JVM and native modes without requiring
     * {@code curl} on the system.
     *
     * @param timeoutSeconds max time to wait
     * @return true if the process became ready, false if it timed out
     */
    public boolean waitForReady(int timeoutSeconds) {
        // Quarkus's SmallRye Health default path is /q/health/live (not /health/live).
        // The data plane exposes only the health endpoint (no Swagger, no REST API).
        String healthUrl = "http://127.0.0.1:" + httpPort + "/q/health/live";
        long deadline = System.currentTimeMillis() + (timeoutSeconds * 1000L);
        HttpClient client = HttpClient.newBuilder()
                .connectTimeout(Duration.ofSeconds(2))
                .build();
        HttpRequest request = HttpRequest.newBuilder()
                .uri(URI.create(healthUrl))
                .timeout(Duration.ofSeconds(2))
                .GET()
                .build();

        while (System.currentTimeMillis() < deadline) {
            if (!isAlive()) return false;
            try {
                HttpResponse<Void> response = client.send(request,
                        HttpResponse.BodyHandlers.discarding());
                if (response.statusCode() == 200) {
                    return true;
                }
            } catch (Exception ignored) {
                // Not ready yet — connection refused
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
