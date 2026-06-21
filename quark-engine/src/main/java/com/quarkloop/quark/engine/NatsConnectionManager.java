package com.quarkloop.quark.engine;

import io.nats.client.Connection;
import io.nats.client.Nats;
import io.nats.client.Options;
import jakarta.annotation.PreDestroy;
import jakarta.enterprise.context.ApplicationScoped;
import org.eclipse.microprofile.config.inject.ConfigProperty;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import java.time.Duration;

/**
 * CDI bean that manages the NATS connection.
 *
 * <p>Connects to an external NATS server at {@code quark.nats.url}
 * (default: {@code nats://localhost:4222}). If the server is unavailable
 * at startup, the platform operates in "degraded" mode — systems can be
 * deployed and tracked, but message flow is disabled until a NATS server
 * becomes available.
 *
 * <p>To enable full message flow, start a NATS server before deploying:
 * <pre>
 *   # Using Docker:
 *   docker run -p 4222:4222 nats:latest
 *
 *   # Using Homebrew (macOS):
 *   brew install nats-server && nats-server
 * </pre>
 */
@ApplicationScoped
public class NatsConnectionManager {

    private static final Logger log = LoggerFactory.getLogger(NatsConnectionManager.class);

    @ConfigProperty(name = "quark.nats.url", defaultValue = "nats://localhost:4222")
    String natsUrl;

    private volatile Connection connection;

    /**
     * Returns the NATS {@link Connection}, or {@code null} if the server
     * is unavailable (degraded mode).
     *
     * <p>We do NOT use a {@code @Produces} method because CDI forbids
     * {@code @ApplicationScoped} producers from returning null. Instead,
     * callers inject {@link NatsConnectionManager} and call this method
     * directly.
     */
    public Connection getConnection() {
        if (connection != null) {
            return connection;
        }
        return tryConnect();
    }

    private synchronized Connection tryConnect() {
        if (connection != null) {
            return connection;
        }

        log.info("Connecting to NATS at {}", natsUrl);

        Options options = Options.builder()
                .server(natsUrl)
                .connectionTimeout(Duration.ofSeconds(2))
                .reconnectWait(Duration.ofSeconds(1))
                .maxReconnects(-1) // infinite reconnects after initial connection
                .build();

        try {
            connection = Nats.connect(options);
            log.info("Connected to NATS: {}", connection.getServerInfo().getServerId());
        } catch (Exception e) {
            log.warn("NATS server not available at {} — platform will operate in degraded mode. " +
                    "Systems can be deployed and tracked, but message flow is disabled. " +
                    "Start a NATS server and redeploy to enable full functionality.", natsUrl);
            connection = null;
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
