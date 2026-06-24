package com.quarkloop.quark.runtime;

import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.datatype.jsr310.JavaTimeModule;
import com.quarkloop.quark.core.engine.dataplane.DataPlaneIpc;
import com.quarkloop.quark.core.engine.metrics.NamespaceMetrics;
import com.quarkloop.quark.core.engine.nats.NatsConnectionManager;
import io.quarkus.runtime.StartupEvent;
import jakarta.annotation.Priority;
import jakarta.enterprise.context.ApplicationScoped;
import jakarta.enterprise.event.Observes;
import jakarta.inject.Inject;
import org.eclipse.microprofile.config.inject.ConfigProperty;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import java.nio.charset.StandardCharsets;
import java.util.Map;

/**
 * Forwards per-namespace metrics snapshots from the data plane to the
 * control plane via NATS heartbeat.
 *
 * <p>Runs in data-plane mode only. Every 2 seconds, takes a snapshot of
 * {@link NamespaceMetrics} and publishes it to
 * {@code quark.data.<runtimeId>.heartbeat}. The control plane's
 * {@code NamespaceMetricsCollector} receives these and uses them as the
 * authoritative metrics for namespaces running in this data plane.
 *
 * <p>The payload is a JSON map of namespace → {@link NamespaceMetrics.Snapshot}:
 * <pre>
 *   {"alice":{"messagesPublished":30,"messagesReceived":48,"errors":0,"cpuTimeNanos":22074409}}
 * </pre>
 */
@ApplicationScoped
public class DataPlaneMetricsForwarder {

    private static final Logger log = LoggerFactory.getLogger(DataPlaneMetricsForwarder.class);

    private static final long INTERVAL_MS = 2000;

    private final ObjectMapper mapper = new ObjectMapper();
    { mapper.registerModule(new JavaTimeModule()); }

    private final NamespaceMetrics namespaceMetrics;
    private final NatsConnectionManager natsConnectionManager;

    @ConfigProperty(name = "quark.mode", defaultValue = "standalone")
    String mode;

    @ConfigProperty(name = "quark.dataplane.runtimeId", defaultValue = "none")
    String runtimeId;

    @Inject
    public DataPlaneMetricsForwarder(NamespaceMetrics namespaceMetrics,
                                      NatsConnectionManager natsConnectionManager) {
        this.namespaceMetrics = namespaceMetrics;
        this.natsConnectionManager = natsConnectionManager;
    }

    /**
     * On startup, if running in data-plane mode, start the heartbeat loop.
     *
     * <p>Runs at priority 15 (after the command handler subscriptions).
     */
    void onStart(@Observes @Priority(15) StartupEvent event) {
        if (!"data".equals(mode)) {
            log.debug("DataPlaneMetricsForwarder inactive (mode={}, not data)", mode);
            return;
        }
        Thread forwarder = Thread.ofPlatform()
                .name("dataplane-metrics-forwarder-" + runtimeId)
                .daemon(true)
                .start(this::forwardLoop);
        log.info("DataPlaneMetricsForwarder active — sending metrics to {} every {}ms",
                DataPlaneIpc.heartbeatSubject(runtimeId), INTERVAL_MS);
    }

    /**
     * The forwarding loop. Runs forever on a daemon thread, sleeping for
     * {@link #INTERVAL_MS} between snapshots.
     */
    private void forwardLoop() {
        while (!Thread.currentThread().isInterrupted()) {
            try {
                Thread.sleep(INTERVAL_MS);
                forwardSnapshot();
            } catch (InterruptedException e) {
                Thread.currentThread().interrupt();
                break;
            } catch (Exception e) {
                log.warn("Error in metrics forwarding loop", e);
            }
        }
    }

    /**
     * Take a snapshot and publish it to the control plane.
     */
    private void forwardSnapshot() {
        try {
            Map<String, NamespaceMetrics.Snapshot> snapshot = namespaceMetrics.snapshot();
            if (snapshot.isEmpty()) return; // no systems deployed yet
            byte[] data = mapper.writeValueAsBytes(snapshot);
            String subject = DataPlaneIpc.heartbeatSubject(runtimeId);
            natsConnectionManager.getConnection().publish(subject, data);
        } catch (Exception e) {
            log.warn("Failed to forward metrics snapshot", e);
        }
    }
}
