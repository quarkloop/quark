package com.quarkloop.quark.core.engine.nats;

import io.nats.client.Connection;
import io.nats.client.Nats;
import io.nats.client.Options;
import jakarta.annotation.PostConstruct;
import jakarta.annotation.PreDestroy;
import jakarta.enterprise.context.ApplicationScoped;
import org.eclipse.microprofile.config.inject.ConfigProperty;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import java.time.Duration;

/**
 * CDI bean that manages the NATS connection.
 *
 * <p>NATS is a HARD dependency. The platform uses a fail-fast approach —
 * if NATS is not available, the server refuses to start. There are no
 * fallbacks, no degraded mode, no in-memory alternatives. See
 * {@code AGENTS.md} §"No fallbacks" for the rationale.
 *
 * <p>Start a NATS server before starting the Quark platform:
 * <pre>
 *   nats-server  # default: nats://localhost:4222
 * </pre>
 */
@ApplicationScoped
public class NatsConnectionManager {

    private static final Logger log = LoggerFactory.getLogger(NatsConnectionManager.class);

    @ConfigProperty(name = "quark.nats.url", defaultValue = "nats://localhost:4222")
    String natsUrl;

    private volatile Connection connection;

    @PostConstruct
    void init() {
        connect();
    }

    /**
     * Connect to NATS. Called at startup. Throws if NATS is unavailable
     * — the platform does NOT start without a message bus.
     */
    public void connect() {
        if (connection != null) return;

        log.info("Connecting to NATS at {}", natsUrl);

        Options options = Options.builder()
                .server(natsUrl)
                .connectionTimeout(Duration.ofSeconds(5))
                .reconnectWait(Duration.ofSeconds(1))
                .maxReconnects(-1)
                .build();

        try {
            connection = Nats.connect(options);
            log.info("Connected to NATS: {}", connection.getServerInfo().getServerId());
        } catch (Exception e) {
            throw new IllegalStateException(
                    "FATAL: Cannot connect to NATS at " + natsUrl + ". " +
                    "NATS is a hard requirement — the platform does not start without it. " +
                    "Start a NATS server and try again. Error: " + e.getMessage(), e);
        }
    }

    /**
     * Returns the NATS {@link Connection}. Never returns null — if the
     * connection was lost, this blocks attempting to reconnect.
     *
     * @throws IllegalStateException if the connection is null (should never
     *         happen after successful {@link #connect()})
     */
    public Connection getConnection() {
        if (connection == null) {
            throw new IllegalStateException(
                    "NATS connection is null. This should not happen after successful startup. " +
                    "The platform requires NATS — no fallbacks are available.");
        }
        return connection;
    }

    @PreDestroy
    void cleanup() {
        if (connection != null) {
            try {
                log.info("Closing NATS connection");
                connection.close();
            } catch (Exception e) {
                log.warn("Failed to close NATS connection", e);
            }
        }
    }
}
