package com.quarkloop.quark.core.domain.spi;

import java.util.Map;

/**
 * Publisher for sending messages to NATS.
 *
 * <p>Providers use this to publish events. The engine resolves the event name
 * to a full NATS subject: {@code <system>.<namespace>.<nodeName>.<event>}.
 * ACLs are enforced — a provider can only publish events declared in its
 * {@code events: [...]} configuration.
 */
public interface QuarkPublisher {

    /**
     * Publish an event.
     *
     * @param event   the event type (e.g., "data", "updated", "tick")
     * @param payload the message payload
     * @throws SecurityException if the event is not in the node's declared events
     */
    void publish(String event, Map<String, Object> payload);
}
