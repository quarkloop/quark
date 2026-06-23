package com.quarkloop.quark.app.metrics;

import com.quarkloop.quark.core.engine.metrics.NamespaceMetrics;
import io.quarkus.runtime.StartupEvent;
import jakarta.annotation.Priority;
import jakarta.enterprise.context.ApplicationScoped;
import jakarta.enterprise.event.Observes;
import jakarta.inject.Inject;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import java.util.Collections;
import java.util.LinkedHashMap;
import java.util.Map;
import java.util.concurrent.ConcurrentHashMap;
import java.util.concurrent.ConcurrentMap;
import java.util.concurrent.atomic.AtomicReference;

/**
 * Periodically snapshots {@link NamespaceMetrics} and computes per-namespace
 * rates (messages/sec, errors/sec, CPU %) over a sliding window.
 *
 * <p>The collector stores the last two snapshots and computes the delta
 * between them. The interval is 2 seconds (matching the CLI's
 * {@code stats --watch} refresh rate). On each tick, the current
 * {@link NamespaceMetrics#snapshot()} is captured and the rates are
 * recomputed.
 *
 * <p>The computed rates are exposed to the REST API via {@link #getRates()},
 * which returns a map of namespace → {@link NamespaceRate}. The REST layer
 * (NamespaceEndpoint) uses this to populate the {@code metrics} field in the
 * namespace detail response.
 *
 * <p>For isolated namespaces (running in a dedicated data-plane JVM — see
 * Task 1), the CPU % reflects the entire process's CPU usage attributed to
 * that single namespace. For shared namespaces, the CPU % reflects only the
 * CPU time consumed by message handlers for that namespace (measured via
 * {@link java.lang.management.ThreadMXBean#getCurrentThreadCpuTime()} inside
 * the handler path).
 */
@ApplicationScoped
public class NamespaceMetricsCollector {

    private static final Logger log = LoggerFactory.getLogger(NamespaceMetricsCollector.class);

    private final NamespaceMetrics metrics;

    /** The most recent raw snapshot. */
    private final AtomicReference<Map<String, NamespaceMetrics.Snapshot>> latestSnapshot =
            new AtomicReference<>(Collections.emptyMap());

    /** The previous raw snapshot (for delta computation). */
    private final AtomicReference<Map<String, NamespaceMetrics.Snapshot>> previousSnapshot =
            new AtomicReference<>(Collections.emptyMap());

    /** The computed rates, updated on each tick. */
    private final ConcurrentMap<String, NamespaceRate> rates = new ConcurrentHashMap<>();

    /** The interval between snapshots, in milliseconds. */
    private static final long INTERVAL_MS = 2000;

    /** The timestamp (millis since epoch) of the last snapshot. */
    private volatile long lastSnapshotTime = 0;

    /** The available processors — used to convert CPU nanos to a percentage. */
    private final int availableProcessors;

    @Inject
    public NamespaceMetricsCollector(NamespaceMetrics metrics) {
        this.metrics = metrics;
        this.availableProcessors = Runtime.getRuntime().availableProcessors();
    }

    /**
     * Initialize the metrics collector on startup. Enables CPU time
     * measurement and takes the first snapshot.
     *
     * <p>Runs at priority 5 (after the registry initializer at priority 1,
     * but before DeployService recovery at priority 10) so that the first
     * snapshot captures a clean baseline before any systems are recovered.
     */
    void onStart(@Observes @Priority(5) StartupEvent event) {
        metrics.init();
        takeSnapshot();
        Thread collector = Thread.ofPlatform()
                .name("quark-metrics-collector")
                .daemon(true)
                .start(this::collectionLoop);
        log.info("NamespaceMetricsCollector started (interval={}ms, cpus={})",
                INTERVAL_MS, availableProcessors);
    }

    /**
     * The collection loop. Runs forever on a daemon thread, sleeping for
     * {@link #INTERVAL_MS} between snapshots.
     */
    private void collectionLoop() {
        while (!Thread.currentThread().isInterrupted()) {
            try {
                Thread.sleep(INTERVAL_MS);
                takeSnapshot();
            } catch (InterruptedException e) {
                Thread.currentThread().interrupt();
                break;
            } catch (Exception e) {
                log.warn("Error in metrics collection loop", e);
            }
        }
    }

    /**
     * Take a snapshot of the current metrics and compute rates.
     *
     * <p>Synchronized to ensure the previous/latest snapshots are updated
     * atomically.
     */
    private synchronized void takeSnapshot() {
        Map<String, NamespaceMetrics.Snapshot> current = metrics.snapshot();
        long now = System.currentTimeMillis();

        Map<String, NamespaceMetrics.Snapshot> prev = latestSnapshot.get();
        long elapsedMs = lastSnapshotTime > 0 ? now - lastSnapshotTime : INTERVAL_MS;

        Map<String, NamespaceRate> newRates = new LinkedHashMap<>();
        for (var entry : current.entrySet()) {
            String ns = entry.getKey();
            NamespaceMetrics.Snapshot cur = entry.getValue();
            NamespaceMetrics.Snapshot prevSnap = prev != null ? prev.get(ns) : null;

            long prevPublished = prevSnap != null ? prevSnap.messagesPublished() : 0;
            long prevReceived = prevSnap != null ? prevSnap.messagesReceived() : 0;
            long prevErrors = prevSnap != null ? prevSnap.errors() : 0;
            long prevCpu = prevSnap != null ? prevSnap.cpuTimeNanos() : 0;

            double elapsedSec = elapsedMs / 1000.0;
            double publishRate = elapsedSec > 0 ? (cur.messagesPublished() - prevPublished) / elapsedSec : 0;
            double receiveRate = elapsedSec > 0 ? (cur.messagesReceived() - prevReceived) / elapsedSec : 0;
            double errorRate = elapsedSec > 0 ? (cur.errors() - prevErrors) / elapsedSec : 0;

            long cpuDeltaNanos = cur.cpuTimeNanos() - prevCpu;
            // CPU % = (cpuTime in the window) / (wall time in the window)
            // This gives the fraction of a single CPU core used by this namespace.
            // Multiply by 100 for a percentage. Values >100% mean the namespace
            // is using more than one core (multi-threaded handlers).
            double windowNanos = elapsedMs * 1_000_000.0;
            double cpuPercent = windowNanos > 0
                    ? (cpuDeltaNanos / windowNanos) * 100.0
                    : 0.0;

            newRates.put(ns, new NamespaceRate(
                    cur.messagesPublished(),
                    cur.messagesReceived(),
                    cur.errors(),
                    cur.cpuTimeNanos(),
                    publishRate,
                    receiveRate,
                    errorRate,
                    cpuPercent
            ));
        }

        // Remove rates for namespaces that no longer exist
        rates.keySet().removeIf(ns -> !current.containsKey(ns));
        rates.putAll(newRates);

        previousSnapshot.set(prev);
        latestSnapshot.set(current);
        lastSnapshotTime = now;
    }

    /**
     * Get the current per-namespace rates. Returns an unmodifiable copy of
     * the internal map.
     */
    public Map<String, NamespaceRate> getRates() {
        return Collections.unmodifiableMap(new LinkedHashMap<>(rates));
    }

    /**
     * Get the rate for a single namespace, or null if the namespace has no
     * metrics (no systems deployed).
     */
    public NamespaceRate getRate(String namespace) {
        return rates.get(namespace);
    }

    /**
     * Per-namespace rate snapshot.
     *
     * @param messagesPublished cumulative count of messages published by nodes in this namespace
     * @param messagesReceived  cumulative count of messages dispatched to node handlers in this namespace
     * @param errors            cumulative count of handler errors in this namespace
     * @param cpuTimeNanos      cumulative CPU time (nanoseconds) consumed by message handlers in this namespace
     * @param publishRate       messages published per second (over the last interval)
     * @param receiveRate       messages received per second (over the last interval)
     * @param errorRate         errors per second (over the last interval)
     * @param cpuPercent        CPU usage as a percentage of a single core (0.0–100.0+)
     */
    public record NamespaceRate(
            long messagesPublished,
            long messagesReceived,
            long errors,
            long cpuTimeNanos,
            double publishRate,
            double receiveRate,
            double errorRate,
            double cpuPercent
    ) {}
}
