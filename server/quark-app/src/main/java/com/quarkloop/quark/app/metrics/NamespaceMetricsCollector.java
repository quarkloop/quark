package com.quarkloop.quark.app.metrics;

import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.datatype.jsr310.JavaTimeModule;
import com.quarkloop.quark.core.engine.dataplane.DataPlaneIpc;
import com.quarkloop.quark.core.engine.metrics.NamespaceMetrics;
import com.quarkloop.quark.core.engine.nats.NatsConnectionManager;
import io.nats.client.Connection;
import io.nats.client.Dispatcher;
import io.nats.client.Message;
import io.quarkus.runtime.StartupEvent;
import jakarta.annotation.Priority;
import jakarta.enterprise.context.ApplicationScoped;
import jakarta.enterprise.event.Observes;
import jakarta.inject.Inject;
import org.eclipse.microprofile.config.inject.ConfigProperty;
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
 * <p>In control-plane mode, the actual message handling happens in data-plane
 * processes — the control plane's local {@link NamespaceMetrics} is empty.
 * To get real metrics, this collector subscribes to NATS heartbeat subjects
 * ({@code quark.data.>.heartbeat}) and receives snapshots from data-plane
 * processes. These remote snapshots are merged into the local NamespaceMetrics
 * so that {@link #getRates()} returns accurate per-namespace values.
 *
 * <p>In data-plane mode, this collector runs locally (the data plane's own
 * metrics are collected and forwarded by {@code DataPlaneMetricsForwarder}).
 *
 * <p>The computed rates are exposed to the REST API via {@link #getRates()}.
 */
@ApplicationScoped
public class NamespaceMetricsCollector {

    private static final Logger log = LoggerFactory.getLogger(NamespaceMetricsCollector.class);

    private final ObjectMapper mapper = new ObjectMapper();
    { mapper.registerModule(new JavaTimeModule()); }

    private final NamespaceMetrics metrics;
    private final NatsConnectionManager natsConnectionManager;

    /** The most recent raw snapshot (local + remote merged). */
    private final AtomicReference<Map<String, NamespaceMetrics.Snapshot>> latestSnapshot =
            new AtomicReference<>(Collections.emptyMap());

    /** The previous raw snapshot (for delta computation). */
    private final AtomicReference<Map<String, NamespaceMetrics.Snapshot>> previousSnapshot =
            new AtomicReference<>(Collections.emptyMap());

    /** The computed rates, updated on each tick. */
    private final ConcurrentMap<String, NamespaceRate> rates = new ConcurrentHashMap<>();

    /** Remote snapshots received from data planes (namespace → snapshot). */
    private final ConcurrentMap<String, NamespaceMetrics.Snapshot> remoteSnapshots =
            new ConcurrentHashMap<>();

    private static final long INTERVAL_MS = 2000;
    private volatile long lastSnapshotTime = 0;
    private final int availableProcessors;

    @ConfigProperty(name = "quark.mode", defaultValue = "standalone")
    String mode;

    @Inject
    public NamespaceMetricsCollector(NamespaceMetrics metrics,
                                      NatsConnectionManager natsConnectionManager) {
        this.metrics = metrics;
        this.natsConnectionManager = natsConnectionManager;
        this.availableProcessors = Runtime.getRuntime().availableProcessors();
    }

    void onStart(@Observes @Priority(5) StartupEvent event) {
        metrics.init();
        takeSnapshot();

        // In control-plane mode, subscribe to data-plane heartbeats
        if ("standalone".equals(mode)) {
            try {
                Connection conn = natsConnectionManager.getConnection();
                Dispatcher dispatcher = conn.createDispatcher(this::handleHeartbeat);
                dispatcher.subscribe(DataPlaneIpc.HEARTBEAT_WILDCARD);
                log.info("Subscribed to data-plane heartbeats on {}", DataPlaneIpc.HEARTBEAT_WILDCARD);
            } catch (Exception e) {
                log.warn("Failed to subscribe to data-plane heartbeats", e);
            }
        }

        Thread collector = Thread.ofPlatform()
                .name("quark-metrics-collector")
                .daemon(true)
                .start(this::collectionLoop);
        log.info("NamespaceMetricsCollector started (mode={}, interval={}ms, cpus={})",
                mode, INTERVAL_MS, availableProcessors);
    }

    /**
     * Handle a heartbeat message from a data-plane process.
     * The payload is a JSON map of namespace → Snapshot.
     */
    @SuppressWarnings("unchecked")
    private void handleHeartbeat(Message natsMsg) {
        try {
            String json = new String(natsMsg.getData(), java.nio.charset.StandardCharsets.UTF_8);
            Map<String, NamespaceMetrics.Snapshot> remote =
                    mapper.readValue(json, mapper.getTypeFactory().constructMapType(
                            Map.class, String.class, NamespaceMetrics.Snapshot.class));
            // Merge remote snapshots into our local NamespaceMetrics
            for (var entry : remote.entrySet()) {
                String ns = entry.getKey();
                NamespaceMetrics.Snapshot snap = entry.getValue();
                // Replace local counters with the data plane's values
                // (the data plane is the authoritative source for these namespaces)
                metrics.replaceSnapshot(ns, snap);
                remoteSnapshots.put(ns, snap);
            }
        } catch (Exception e) {
            log.warn("Failed to process heartbeat from {}", natsMsg.getSubject(), e);
        }
    }

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

        rates.keySet().removeIf(ns -> !current.containsKey(ns));
        rates.putAll(newRates);

        previousSnapshot.set(prev);
        latestSnapshot.set(current);
        lastSnapshotTime = now;
    }

    public Map<String, NamespaceRate> getRates() {
        return Collections.unmodifiableMap(new LinkedHashMap<>(rates));
    }

    public NamespaceRate getRate(String namespace) {
        return rates.get(namespace);
    }

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
