package com.quarkloop.quark.runtime.event.internal;

import com.quarkloop.quark.runtime.domain.event.NodeEvent;
import com.quarkloop.quark.runtime.event.EventStore;
import jakarta.enterprise.context.ApplicationScoped;
import jakarta.enterprise.event.Observes;
import jakarta.inject.Inject;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

/**
 * Bridges the in-memory {@link com.quarkloop.quark.runtime.event.EventBus} to the
 * persistent {@link EventStore}.
 *
 * <p>Every event published on the bus is observed (synchronously, on the
 * publishing thread) and appended to the store. Failures are logged but do not
 * propagate to the publisher — losing the in-memory notification would be
 * worse than losing the durable copy in most cases.
 *
 * <p>The {@code @Observes} annotation hooks into the CDI event bus. The
 * application publishes a CDI event of type {@link NodeEvent} (see
 * {@link EventBusPersistenceBridge#onEvent(NodeEvent)}); this observer
 * picks it up and persists it.
 *
 * <p>The bridge is wired by having {@code InMemoryEventBus#publish(NodeEvent)}
 * fire a CDI event in addition to its in-memory subscriber notifications. This
 * keeps the in-memory path fast (subscribers notified directly) while ensuring
 * every event is durably recorded.
 */
@ApplicationScoped
public class EventBusPersistenceBridge {

    private static final Logger log = LoggerFactory.getLogger(EventBusPersistenceBridge.class);

    private final EventStore eventStore;

    @Inject
    public EventBusPersistenceBridge(EventStore eventStore) {
        this.eventStore = eventStore;
    }

    /**
     * Observe NodeEvent payloads fired via the CDI event bus and persist
     * them. The JsonlEventStore is not transactional (line-level atomic
     * appends), so no {@code @Transactional} annotation is needed. If a
     * future DB-backed store is plugged in, add JTA support and re-add the
     * annotation here.
     */
    void onEvent(@Observes NodeEvent event) {
        if (event == null) return;
        try {
            eventStore.append(event);
        } catch (Exception e) {
            log.error("Failed to persist event {} (kind={}, node={})",
                    event.id(), event.kind(), event.nodeName(), e);
        }
    }
}
