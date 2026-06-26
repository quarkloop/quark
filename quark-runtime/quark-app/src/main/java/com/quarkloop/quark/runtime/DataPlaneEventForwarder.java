package com.quarkloop.quark.runtime;

import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.datatype.jsr310.JavaTimeModule;
import com.quarkloop.quark.runtime.domain.event.NodeEvent;
import com.quarkloop.quark.runtime.engine.dataplane.DataPlaneIpc;
import com.quarkloop.quark.runtime.event.EventBus;
import com.quarkloop.quark.runtime.event.EventHandler;
import com.quarkloop.quark.runtime.engine.nats.NatsConnectionManager;
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
 * Forwards lifecycle events from the data plane to the control plane via NATS.
 *
 * <p>In data-plane mode, the {@code EventBusPersistenceBridge} is a no-op
 * (the Catalog client is not active). This forwarder replaces it: it subscribes
 * to the internal {@link EventBus} and publishes each {@link NodeEvent} to
 * the NATS subject {@code quark.data.<runtimeId>.event}. The control plane's
 * {@code ControlPlaneEventReceiver} subscribes to these subjects and
 * persists the events to the Catalog.
 *
 * <p>Only active in data-plane mode ({@code quark.mode=data}).
 */
@ApplicationScoped
public class DataPlaneEventForwarder implements EventHandler {

    private static final Logger log = LoggerFactory.getLogger(DataPlaneEventForwarder.class);

    private final ObjectMapper mapper = new ObjectMapper();
    { mapper.registerModule(new JavaTimeModule()); }

    private final EventBus eventBus;
    private final NatsConnectionManager natsConnectionManager;

    @ConfigProperty(name = "quark.mode", defaultValue = "standalone")
    String mode;

    @ConfigProperty(name = "quark.dataplane.runtimeId", defaultValue = "none")
    String runtimeId;

    @Inject
    public DataPlaneEventForwarder(EventBus eventBus, NatsConnectionManager natsConnectionManager) {
        this.eventBus = eventBus;
        this.natsConnectionManager = natsConnectionManager;
    }

    /**
     * On startup, if running in data-plane mode, subscribe to the EventBus
     * to forward all events.
     *
     * <p>Runs at priority 15 (after the command handler at 20) so the NATS
     * subscriptions are ready before events start flowing.
     */
    void onStart(@Observes @Priority(15) StartupEvent event) {
        if (!"data".equals(mode)) {
            log.debug("DataPlaneEventForwarder inactive (mode={}, not data)", mode);
            return;
        }
        eventBus.subscribeAll(this);
        log.info("DataPlaneEventForwarder active — forwarding events to NATS subject {}",
                DataPlaneIpc.eventSubject(runtimeId));
    }

    @Override
    public void onEvent(NodeEvent event) {
        if (event == null) return;
        try {
            String subject = DataPlaneIpc.eventSubject(runtimeId);
            byte[] data = mapper.writeValueAsBytes(event);
            natsConnectionManager.getConnection().publish(subject, data);
        } catch (Exception e) {
            log.error("Failed to forward event {} (kind={}, node={})",
                    event.id(), event.kind(), event.nodeName(), e);
        }
    }
}
