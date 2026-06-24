package com.quarkloop.quark.app.dataplane;

import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.datatype.jsr310.JavaTimeModule;
import com.quarkloop.quark.core.domain.event.NodeEvent;
import com.quarkloop.quark.core.engine.dataplane.DataPlaneIpc;
import com.quarkloop.quark.core.event.EventStore;
import com.quarkloop.quark.engine.NatsConnectionManager;
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

import java.nio.charset.StandardCharsets;

/**
 * Receives lifecycle events from data-plane processes via NATS and persists
 * them to the Catalog.
 *
 * <p>Subscribes to the NATS wildcard subject {@code quark.data.>.event} to
 * receive events from ALL data-plane processes (shared + isolated). Each
 * event is a JSON-serialized {@link NodeEvent} that is deserialized and
 * appended to the {@link EventStore} (Catalog).
 *
 * <p>Only active in control-plane mode ({@code quark.mode=standalone}).
 * In data-plane mode, the {@link DataPlaneEventForwarder} handles event
 * forwarding instead.
 */
@ApplicationScoped
public class ControlPlaneEventReceiver {

    private static final Logger log = LoggerFactory.getLogger(ControlPlaneEventReceiver.class);

    private final ObjectMapper mapper = new ObjectMapper();
    { mapper.registerModule(new JavaTimeModule()); }

    private final EventStore eventStore;
    private final NatsConnectionManager natsConnectionManager;

    @ConfigProperty(name = "quark.mode", defaultValue = "standalone")
    String mode;

    @Inject
    public ControlPlaneEventReceiver(EventStore eventStore, NatsConnectionManager natsConnectionManager) {
        this.eventStore = eventStore;
        this.natsConnectionManager = natsConnectionManager;
    }

    /**
     * On startup, if running in control-plane mode, subscribe to the NATS
     * event wildcard to receive events from all data-plane processes.
     *
     * <p>Runs at priority 3 (after ProcessManager at 2, before
     * NamespaceMetricsCollector at 5 and DeployService recovery at 10).
     */
    void onStart(@Observes @Priority(3) StartupEvent event) {
        if (!"standalone".equals(mode)) {
            log.debug("ControlPlaneEventReceiver inactive (mode={}, not standalone)", mode);
            return;
        }

        Connection conn = natsConnectionManager.getConnection();
        Dispatcher dispatcher = conn.createDispatcher(this::handleEvent);
        dispatcher.subscribe(DataPlaneIpc.EVENT_WILDCARD);
        log.info("ControlPlaneEventReceiver subscribed to {} — receiving events from all data planes",
                DataPlaneIpc.EVENT_WILDCARD);
    }

    /**
     * NATS message handler — deserialize the NodeEvent and persist it.
     */
    private void handleEvent(Message natsMsg) {
        try {
            String json = new String(natsMsg.getData(), StandardCharsets.UTF_8);
            NodeEvent nodeEvent = mapper.readValue(json, NodeEvent.class);
            eventStore.append(nodeEvent);
            log.trace("Received and persisted event {} (kind={}, node={}/{})",
                    nodeEvent.id(), nodeEvent.kind(), nodeEvent.namespace(), nodeEvent.nodeName());
        } catch (Exception e) {
            log.error("Failed to receive/persist event from {}", natsMsg.getSubject(), e);
        }
    }
}
