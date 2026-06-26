package com.quarkloop.quark.runtime.engine.metrics;

import jakarta.enterprise.context.ApplicationScoped;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import java.lang.management.ManagementFactory;
import java.lang.management.ThreadMXBean;
import java.util.Collections;
import java.util.LinkedHashMap;
import java.util.Map;
import java.util.concurrent.ConcurrentHashMap;
import java.util.concurrent.ConcurrentMap;
import java.util.concurrent.atomic.AtomicLong;

/**
 * Per-namespace metrics accumulator for CPU attribution, message throughput,
 * and error tracking.
 *
 * <p>For shared namespaces running in the same JVM, CPU time is attributed by
 * measuring {@link ThreadMXBean#getCurrentThreadCpuTime()} inside the message
 * handler path (see {@code SystemDeployer.dispatchToProvider}). This captures
 * the actual CPU spent processing messages for each namespace — more accurate
 * than thread-group-based attribution, which doesn't work with virtual threads
 * (virtual threads don't have per-thread CPU time visible via ThreadMXBean).
 *
 * <p>For isolated namespaces (running in a dedicated data-plane JVM — see
 * Task 1), all metrics are exact at the process level because the entire JVM
 * serves a single namespace.
 *
 * <p>Thread-safety: all counters are {@link AtomicLong}. The internal maps
 * are {@link ConcurrentMap}. Methods may be called concurrently from NATS
 * dispatcher threads, source provider threads, and the scheduled collector.
 *
 * <p>Snapshot semantics: {@link #snapshot()} returns a point-in-time view.
 * The caller computes rates (messages/sec, CPU %) by diffing two snapshots
 * taken at known intervals.
 */
@ApplicationScoped
public class NamespaceMetrics {

    private static final Logger log = LoggerFactory.getLogger(NamespaceMetrics.class);

    private final ThreadMXBean threadMXBean = ManagementFactory.getThreadMXBean();

    /** Per-namespace counters. Key = namespace string. */
    private final ConcurrentMap<String, Counters> counters = new ConcurrentHashMap<>();

    /**
     * Record a message published by a node in the given namespace.
     *
     * @param namespace the namespace the publishing node belongs to
     */
    public void recordPublish(String namespace) {
        getOrCreate(namespace).messagesPublished.incrementAndGet();
    }

    /**
     * Record a message received (dispatched to a provider) in the given
     * namespace, along with the CPU time consumed by the handler.
     *
     * <p>This MUST be called on the thread that executed the handler so that
     * {@link ThreadMXBean#getCurrentThreadCpuTime()} reflects the handler's
     * CPU usage. The delta is computed as {@code cpuAfter - cpuBefore} by
     * the caller and passed here.
     *
     * @param namespace the namespace the receiving node belongs to
     * @param cpuDeltaNanos the CPU time (in nanoseconds) consumed by the
     *                      message handler, measured via
     *                      {@link ThreadMXBean#getCurrentThreadCpuTime()}
     */
    public void recordMessageHandled(String namespace, long cpuDeltaNanos) {
        Counters c = getOrCreate(namespace);
        c.messagesReceived.incrementAndGet();
        if (cpuDeltaNanos > 0) {
            c.cpuTimeNanos.addAndGet(cpuDeltaNanos);
        }
    }

    /**
     * Record a handler error for the given namespace.
     *
     * @param namespace the namespace the failing node belongs to
     */
    public void recordError(String namespace) {
        getOrCreate(namespace).errors.incrementAndGet();
    }

    /**
     * Remove all counters for a namespace. Called when a namespace's last
     * system is undeployed so stale entries don't linger in the snapshot.
     *
     * @param namespace the namespace to clear
     */
    public void remove(String namespace) {
        counters.remove(namespace);
    }

    /**
     * Replace the counters for a namespace with the values from a remote
     * (data-plane) snapshot. Used by the control plane's
     * {@code NamespaceMetricsCollector} when it receives a heartbeat from
     * a data-plane process — the data plane is the authoritative source
     * for metrics of namespaces running in it.
     *
     * <p>This method directly sets the counter values to match the remote
     * snapshot, overwriting any local values. The next {@link #snapshot()}
     * call will reflect the updated values.
     *
     * @param namespace the namespace to update
     * @param snapshot  the remote snapshot to apply
     */
    public void replaceSnapshot(String namespace, Snapshot snapshot) {
        Counters c = getOrCreate(namespace);
        c.messagesPublished.set(snapshot.messagesPublished());
        c.messagesReceived.set(snapshot.messagesReceived());
        c.errors.set(snapshot.errors());
        c.cpuTimeNanos.set(snapshot.cpuTimeNanos());
    }

    /**
     * Take a point-in-time snapshot of all namespace counters.
     *
     * <p>The returned map is a copy; mutations to it do not affect the live
     * counters. Each {@link Snapshot} value contains the raw cumulative
     * counters at the moment of the call.
     *
     * @return an unmodifiable map of namespace → snapshot
     */
    public Map<String, Snapshot> snapshot() {
        Map<String, Snapshot> out = new LinkedHashMap<>();
        for (var entry : counters.entrySet()) {
            Counters c = entry.getValue();
            out.put(entry.getKey(), new Snapshot(
                    c.messagesPublished.get(),
                    c.messagesReceived.get(),
                    c.errors.get(),
                    c.cpuTimeNanos.get()
            ));
        }
        return Collections.unmodifiableMap(out);
    }

    /**
     * Check whether CPU time measurement is available on this JVM.
     * {@link ThreadMXBean#getCurrentThreadCpuTime()} returns -1 if the
     * JVM implementation does not support CPU time measurement.
     */
    public boolean isCpuTimeSupported() {
        return threadMXBean.isCurrentThreadCpuTimeSupported()
                && threadMXBean.isThreadCpuTimeEnabled();
    }

    /**
     * Get the current thread's CPU time in nanoseconds. Returns -1 if not
     * supported.
     */
    public long getCurrentThreadCpuTimeNanos() {
        if (!isCpuTimeSupported()) return -1;
        return threadMXBean.getCurrentThreadCpuTime();
    }

    private Counters getOrCreate(String namespace) {
        return counters.computeIfAbsent(namespace, k -> new Counters());
    }

    /**
     * Enable CPU time measurement if the JVM supports it. Called once at
     * startup by the {@link NamespaceMetricsCollector}.
     */
    public void init() {
        if (threadMXBean.isCurrentThreadCpuTimeSupported()
                && !threadMXBean.isThreadCpuTimeEnabled()) {
            threadMXBean.setThreadCpuTimeEnabled(true);
            log.info("Enabled ThreadMXBean CPU time measurement for per-namespace attribution");
        }
    }

    // ----- Inner classes -----

    private static final class Counters {
        final AtomicLong messagesPublished = new AtomicLong(0);
        final AtomicLong messagesReceived = new AtomicLong(0);
        final AtomicLong errors = new AtomicLong(0);
        final AtomicLong cpuTimeNanos = new AtomicLong(0);
    }

    /**
     * Immutable point-in-time snapshot of a single namespace's counters.
     *
     * @param messagesPublished cumulative count of messages published by nodes in this namespace
     * @param messagesReceived  cumulative count of messages dispatched to node handlers in this namespace
     * @param errors            cumulative count of handler errors in this namespace
     * @param cpuTimeNanos      cumulative CPU time (nanoseconds) consumed by message handlers in this namespace
     */
    public record Snapshot(
            long messagesPublished,
            long messagesReceived,
            long errors,
            long cpuTimeNanos
    ) {}
}
