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

import java.util.Set;

/**
 * NATS implementation of the {@link MessageBus} SPI.
 *
 * <p>Requires a running NATS server. No fallbacks, no in-memory
 * alternatives, no degraded mode. If NATS is unavailable, the
 * platform refuses to start (see {@link NatsConnectionManager}).
 */
@ApplicationScoped
public class NatsMessageBus implements MessageBus {

    private static final Logger log = LoggerFactory.getLogger(NatsMessageBus.class);

    private final NatsConnectionManager connectionManager;

    @Inject
    public NatsMessageBus(NatsConnectionManager connectionManager) {
        this.connectionManager = connectionManager;
    }

    @Override
    public void publish(String subject, byte[] payload) {
        connectionManager.getConnection().publish(subject, payload);
    }

    @Override
    public Subscription subscribe(String subject, MessageHandler handler) {
        Connection conn = connectionManager.getConnection();
        Dispatcher dispatcher = conn.createDispatcher(msg -> {
            try {
                QuarkMessage quarkMsg = toQuarkMessage(msg);
                handler.onMessage(quarkMsg);
                msg.ack();
            } catch (Exception e) {
                log.error("Error processing message on {}", msg.getSubject(), e);
                msg.nak();
            }
        });
        dispatcher.subscribe(subject);
        log.debug("Subscribed to {}", subject);
        return new NatsSubscription(subject, conn, dispatcher);
    }

    @Override
    public QuarkPublisher createPublisher(String systemName, String namespace,
                                           String nodeName, Set<String> allowedEvents) {
        return new NatsQuarkPublisher(
                connectionManager.getConnection(),
                systemName, namespace, nodeName, allowedEvents);
    }

    @Override
    public boolean isConnected() {
        try {
            Connection conn = connectionManager.getConnection();
            return conn.getStatus() == Connection.Status.CONNECTED;
        } catch (Exception e) {
            return false;
        }
    }

    @Override
    public void connect() {
        connectionManager.connect();
    }

    @Override
    @PreDestroy
    public void close() {
        // Connection lifecycle owned by NatsConnectionManager
    }

    private static QuarkMessage toQuarkMessage(Message natsMsg) {
        String subject = natsMsg.getSubject();
        String[] parts = subject.split("\\.");
        // Subject format: {namespace}.{system}.{node}.{event}
        String namespace = parts.length > 0 ? parts[0] : "";
        String systemName = parts.length > 1 ? parts[1] : "";
        String nodeName = parts.length > 2 ? parts[2] : "";
        return new NatsQuarkMessage(natsMsg, systemName, namespace, nodeName);
    }

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
