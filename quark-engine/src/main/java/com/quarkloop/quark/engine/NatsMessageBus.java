package com.quarkloop.quark.engine;

import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.datatype.jsr310.JavaTimeModule;
import com.quarkloop.quark.core.domain.spi.QuarkMessage;
import com.quarkloop.quark.core.domain.spi.QuarkPublisher;
import com.quarkloop.quark.core.engine.bus.MessageBus;
import com.quarkloop.quark.core.engine.bus.MessageHandler;
import com.quarkloop.quark.core.engine.bus.Subscription;
import io.nats.client.Connection;
import io.nats.client.Dispatcher;
import io.nats.client.Message;
import jakarta.annotation.PreDestroy;
import jakarta.enterprise.context.ApplicationScoped;
import jakarta.inject.Inject;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import java.nio.charset.StandardCharsets;
import java.time.Instant;
import java.util.*;
import java.util.concurrent.ConcurrentHashMap;
import java.util.concurrent.CopyOnWriteArrayList;

/**
 * NATS message bus with in-memory fallback.
 *
 * <p>When NATS is connected, messages flow through the NATS server.
 * When NATS is not available, messages are delivered in-process via
 * an in-memory pub/sub map. This allows the platform to work
 * standalone for local development and testing without requiring
 * an external NATS server.
 *
 * <p>{@link #isConnected()} always returns {@code true} because the
 * in-memory fallback is always available.
 */
@ApplicationScoped
public class NatsMessageBus implements MessageBus {

    private static final Logger log = LoggerFactory.getLogger(NatsMessageBus.class);

    private static final ObjectMapper mapper = new ObjectMapper();
    static { mapper.registerModule(new JavaTimeModule()); }

    private final NatsConnectionManager connectionManager;

    /** In-memory subscriptions for fallback when NATS is not connected. */
    private final Map<String, List<MessageHandler>> inMemorySubs = new ConcurrentHashMap<>();

    @Inject
    public NatsMessageBus(NatsConnectionManager connectionManager) {
        this.connectionManager = connectionManager;
    }

    @Override
    public void publish(String subject, byte[] payload) {
        Connection conn = getNatsConnection();
        if (conn != null) {
            conn.publish(subject, payload);
        } else {
            deliverInMemory(subject, payload);
        }
    }

    @Override
    public Subscription subscribe(String subject, MessageHandler handler) {
        Connection conn = getNatsConnection();
        if (conn != null) {
            Dispatcher dispatcher = conn.createDispatcher(msg -> {
                try {
                    handler.onMessage(toQuarkMessage(msg));
                    msg.ack();
                } catch (Exception e) {
                    log.error("Error processing message on {}", msg.getSubject(), e);
                    msg.nak();
                }
            });
            dispatcher.subscribe(subject);
            log.debug("Subscribed to {} (NATS)", subject);
            return new NatsSubscription(subject, conn, dispatcher);
        } else {
            inMemorySubs.computeIfAbsent(subject, k -> new CopyOnWriteArrayList<>()).add(handler);
            log.debug("Subscribed to {} (in-memory fallback)", subject);
            return new InMemorySubscription(subject, handler);
        }
    }

    @Override
    public QuarkPublisher createPublisher(String systemName, String namespace,
                                           String nodeName, Set<String> allowedEvents) {
        return new FallbackPublisher(systemName, namespace, nodeName, allowedEvents);
    }

    @Override
    public boolean isConnected() {
        // Always returns true — the in-memory fallback is always available.
        return true;
    }

    @Override
    public void connect() {
        // Try to connect to NATS. If it fails, the in-memory fallback kicks in.
        connectionManager.getConnection();
    }

    @Override
    @PreDestroy
    public void close() {
        inMemorySubs.clear();
    }

    // ------------------------------------------------------------------
    // Internal
    // ------------------------------------------------------------------

    private Connection getNatsConnection() {
        try {
            Connection conn = connectionManager.getConnection();
            if (conn != null && conn.getStatus() == Connection.Status.CONNECTED) {
                return conn;
            }
        } catch (Exception e) {
            log.debug("NATS connection check failed, using in-memory fallback", e);
        }
        return null;
    }

    @SuppressWarnings("unchecked")
    private void deliverInMemory(String subject, byte[] payload) {
        List<MessageHandler> handlers = inMemorySubs.get(subject);
        if (handlers == null || handlers.isEmpty()) return;

        Map<String, Object> payloadMap;
        try {
            payloadMap = mapper.readValue(payload, Map.class);
        } catch (Exception e) {
            payloadMap = Map.of("__raw__", new String(payload, StandardCharsets.UTF_8));
        }

        String[] parts = subject.split("\\.");
        String systemName = parts.length > 0 ? parts[0] : "";
        String namespace = parts.length > 1 ? parts[1] : "";
        String nodeName = parts.length > 2 ? parts[2] : "";

        QuarkMessage msg = new InMemoryQuarkMessage(subject, payloadMap, systemName, namespace, nodeName);

        for (MessageHandler handler : handlers) {
            try {
                handler.onMessage(msg);
            } catch (Exception e) {
                log.error("Error processing in-memory message on {}", subject, e);
            }
        }
    }

    private static QuarkMessage toQuarkMessage(Message natsMsg) {
        String subject = natsMsg.getSubject();
        String[] parts = subject.split("\\.");
        String systemName = parts.length > 0 ? parts[0] : "";
        String namespace = parts.length > 1 ? parts[1] : "";
        String nodeName = parts.length > 2 ? parts[2] : "";
        return new NatsQuarkMessage(natsMsg, systemName, namespace, nodeName);
    }

    // ------------------------------------------------------------------
    // Inner classes
    // ------------------------------------------------------------------

    /** Publisher that uses NATS when connected, in-memory when not. */
    private final class FallbackPublisher implements QuarkPublisher {
        private final String systemName;
        private final String namespace;
        private final String nodeName;
        private final Set<String> allowedEvents;

        FallbackPublisher(String systemName, String namespace, String nodeName, Set<String> allowedEvents) {
            this.systemName = systemName;
            this.namespace = namespace;
            this.nodeName = nodeName;
            this.allowedEvents = allowedEvents;
        }

        @Override
        public void publish(String event, Map<String, Object> payload) {
            if (!allowedEvents.contains(event)) {
                throw new SecurityException(
                    "Node '" + nodeName + "' is not allowed to publish event '" + event +
                    "'. Declared events: " + allowedEvents);
            }

            String subject = systemName + "." + namespace + "." + nodeName + "." + event;
            byte[] data;
            try {
                data = mapper.writeValueAsBytes(payload);
            } catch (Exception e) {
                log.error("Failed to serialize payload for {}", subject, e);
                return;
            }

            NatsMessageBus.this.publish(subject, data);
        }
    }

    /** In-memory QuarkMessage implementation. */
    private static final class InMemoryQuarkMessage implements QuarkMessage {
        private final String subject;
        private final Map<String, Object> payload;
        private final String systemName;
        private final String namespace;
        private final String nodeName;

        InMemoryQuarkMessage(String subject, Map<String, Object> payload,
                              String systemName, String namespace, String nodeName) {
            this.subject = subject;
            this.payload = payload != null ? payload : Map.of();
            this.systemName = systemName;
            this.namespace = namespace;
            this.nodeName = nodeName;
        }

        @Override public String subject() { return subject; }
        @Override public Map<String, Object> payload() { return payload; }
        @Override public Map<String, String> headers() { return Map.of(); }
        @Override public Instant timestamp() { return Instant.now(); }
        @Override public String systemName() { return systemName; }
        @Override public String namespace() { return namespace; }
        @Override public String nodeName() { return nodeName; }
    }

    /** In-memory subscription that removes the handler on close. */
    private final class InMemorySubscription implements Subscription {
        private final String subject;
        private final MessageHandler handler;
        private volatile boolean closed;

        InMemorySubscription(String subject, MessageHandler handler) {
            this.subject = subject;
            this.handler = handler;
        }

        @Override public String subject() { return subject; }

        @Override
        public void close() {
            if (closed) return;
            closed = true;
            List<MessageHandler> handlers = inMemorySubs.get(subject);
            if (handlers != null) handlers.remove(handler);
        }
    }

    /** NATS-backed subscription. */
    private static final class NatsSubscription implements Subscription {
        private final String subject;
        private final Connection connection;
        private final Dispatcher dispatcher;
        private volatile boolean closed;

        NatsSubscription(String subject, Connection connection, Dispatcher dispatcher) {
            this.subject = subject;
            this.connection = connection;
            this.dispatcher = dispatcher;
        }

        @Override public String subject() { return subject; }

        @Override
        public synchronized void close() {
            if (closed) return;
            closed = true;
            try { connection.closeDispatcher(dispatcher); }
            catch (Exception e) { log.warn("Failed to close dispatcher for {}", subject, e); }
        }
    }
}
