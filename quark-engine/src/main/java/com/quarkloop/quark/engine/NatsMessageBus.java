package com.quarkloop.quark.engine;

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
        Connection conn = connectionManager.getConnection();
        if (conn == null) { log.debug("publish skipped (bus disconnected): {}", subject); return; }
        conn.publish(subject, payload);
    }

    @Override
    public Subscription subscribe(String subject, MessageHandler handler) {
        Connection conn = connectionManager.getConnection();
        if (conn == null) {
            log.warn("subscribe skipped (bus disconnected): {}", subject);
            return new NoopSubscription(subject);
        }
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
    public QuarkPublisher createPublisher(String systemName, String namespace, String nodeName, Set<String> allowedEvents) {
        Connection conn = connectionManager.getConnection();
        if (conn == null) {
            return (event, payload) -> log.debug("publish skipped (bus disconnected): {}.{}.{}.{}", systemName, namespace, nodeName, event);
        }
        return new NatsQuarkPublisher(conn, systemName, namespace, nodeName, allowedEvents);
    }

    @Override
    public boolean isConnected() {
        Connection conn = connectionManager.getConnection();
        return conn != null && conn.getStatus() == Connection.Status.CONNECTED;
    }

    @Override
    public void connect() { connectionManager.getConnection(); }

    @Override
    @PreDestroy
    public void close() { /* Connection lifecycle owned by NatsConnectionManager */ }

    private static QuarkMessage toQuarkMessage(Message natsMsg) {
        String subject = natsMsg.getSubject();
        String[] parts = subject.split("\\.");
        String systemName = parts.length > 0 ? parts[0] : "";
        String namespace = parts.length > 1 ? parts[1] : "";
        String nodeName = parts.length > 2 ? parts[2] : "";
        return new NatsQuarkMessage(natsMsg, systemName, namespace, nodeName);
    }

    private static final class NoopSubscription implements Subscription {
        private final String subject;
        NoopSubscription(String subject) { this.subject = subject; }
        @Override public String subject() { return subject; }
        @Override public void close() {}
    }

    private static final class NatsSubscription implements Subscription {
        private final String subject;
        private final Connection connection;
        private final Dispatcher dispatcher;
        private volatile boolean closed;

        NatsSubscription(String subject, Connection connection, Dispatcher dispatcher) {
            this.subject = subject; this.connection = connection; this.dispatcher = dispatcher;
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
