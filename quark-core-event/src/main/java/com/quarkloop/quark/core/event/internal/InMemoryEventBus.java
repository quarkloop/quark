package com.quarkloop.quark.core.event.internal;

import com.quarkloop.quark.core.domain.event.NodeEvent;
import com.quarkloop.quark.core.domain.event.NodeEventKind;
import com.quarkloop.quark.core.event.EventBus;
import com.quarkloop.quark.core.event.EventHandler;
import jakarta.enterprise.context.ApplicationScoped;
import jakarta.enterprise.event.Event;
import jakarta.inject.Inject;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import java.util.List;
import java.util.Set;
import java.util.concurrent.ConcurrentHashMap;
import java.util.concurrent.CopyOnWriteArrayList;

/**
 * In-process pub/sub for {@link NodeEvent}s.
 *
 * <p>Two notification paths run for every published event:
 * <ol>
 *   <li><b>In-memory subscribers</b> — registered via {@link #subscribe} /
 *       {@link #subscribeAll}. These are notified synchronously on the
 *       publishing thread. Exceptions in individual subscribers are caught
 *       and logged so one bad subscriber cannot break the chain.</li>
 *   <li><b>CDI event</b> — fired via {@link Event#select} / {@link Event#fire}.
 *       Observed by {@link EventBusPersistenceBridge} which durably appends
 *       the event to the {@link com.quarkloop.quark.core.event.EventStore}.</li>
 * </ol>
 *
 * <p>This dual-path design means the in-memory notification latency is not
 * affected by disk I/O on the persistence path (the CDI observer runs on the
 * same thread, but only after in-memory subscribers have been notified — to
 * change this, make the observer {@code @ObservesAsync}).
 */
@ApplicationScoped
public class InMemoryEventBus implements EventBus {

    private static final Logger log = LoggerFactory.getLogger(InMemoryEventBus.class);

    private final ConcurrentHashMap<NodeEventKind, List<EventHandler>> subscriptions = new ConcurrentHashMap<>();
    private final List<EventHandler> allSubscriptions = new CopyOnWriteArrayList<>();

    /** CDI event firer used to bridge into the persistence layer. */
    private final Event<NodeEvent> cdiEventFirer;

    @Inject
    public InMemoryEventBus(Event<NodeEvent> cdiEventFirer) {
        this.cdiEventFirer = cdiEventFirer;
    }

    @Override
    public void publish(NodeEvent event) {
        if (event == null) return;

        // 1. Notify kind-specific subscribers.
        List<EventHandler> handlers = subscriptions.get(event.kind());
        if (handlers != null) {
            for (EventHandler handler : handlers) {
                notifyHandler(handler, event);
            }
        }

        // 2. Notify all-subscribers.
        for (EventHandler handler : allSubscriptions) {
            notifyHandler(handler, event);
        }

        // 3. Fire CDI event so the persistence bridge can append to the store.
        try {
            cdiEventFirer.fire(event);
        } catch (Exception e) {
            // The persistence bridge catches its own errors, but if CDI itself
            // fails (no active context, etc.) we must not propagate.
            log.error("Failed to fire CDI event for {} (node={})",
                    event.kind(), event.nodeName(), e);
        }
    }

    private void notifyHandler(EventHandler handler, NodeEvent event) {
        try {
            handler.onEvent(event);
        } catch (Exception e) {
            log.error("Error in event handler for event {}: {}", event.id(), e.getMessage(), e);
        }
    }

    @Override
    public void subscribe(NodeEventKind kind, EventHandler handler) {
        if (kind == null || handler == null) return;
        subscriptions.computeIfAbsent(kind, k -> new CopyOnWriteArrayList<>()).add(handler);
    }

    @Override
    public void subscribe(Set<NodeEventKind> kinds, EventHandler handler) {
        if (kinds == null) return;
        for (NodeEventKind kind : kinds) {
            subscribe(kind, handler);
        }
    }

    @Override
    public void subscribeAll(EventHandler handler) {
        if (handler != null) {
            allSubscriptions.add(handler);
        }
    }

    @Override
    public void unsubscribe(EventHandler handler) {
        allSubscriptions.remove(handler);
        for (List<EventHandler> handlers : subscriptions.values()) {
            handlers.remove(handler);
        }
    }
}
