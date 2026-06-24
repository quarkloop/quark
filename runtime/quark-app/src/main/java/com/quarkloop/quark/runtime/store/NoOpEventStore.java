package com.quarkloop.quark.runtime.store;

import com.quarkloop.quark.core.domain.event.NodeEvent;
import com.quarkloop.quark.core.event.EventFilter;
import com.quarkloop.quark.core.event.EventStore;
import jakarta.enterprise.context.ApplicationScoped;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import java.util.List;

/**
 * No-op EventStore for the data plane.
 *
 * <p>The data plane forwards events to the control plane via NATS
 * (DataPlaneEventForwarder). This EventStore satisfies the CDI dependency
 * for EventBusPersistenceBridge without doing any actual persistence.
 * The actual persistence happens on the control plane side
 * (ControlPlaneEventReceiver → NatsCatalogClient → Catalog service).
 */
@ApplicationScoped
public class NoOpEventStore implements EventStore {

    private static final Logger log = LoggerFactory.getLogger(NoOpEventStore.class);

    @Override
    public void append(NodeEvent event) {
        // No-op — events are forwarded via DataPlaneEventForwarder
    }

    @Override
    public void appendAll(List<NodeEvent> events) {
        // No-op — events are forwarded via DataPlaneEventForwarder
    }

    @Override
    public List<NodeEvent> query(EventFilter filter) {
        return List.of();
    }

    @Override
    public long count(EventFilter filter) {
        return 0;
    }
}
