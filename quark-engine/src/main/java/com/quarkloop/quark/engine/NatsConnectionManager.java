package com.quarkloop.quark.engine;

import io.nats.client.Connection;
import io.nats.client.Nats;
import io.nats.client.Options;
import jakarta.annotation.PreDestroy;
import jakarta.enterprise.context.ApplicationScoped;
import jakarta.enterprise.inject.Produces;
import org.eclipse.microprofile.config.inject.ConfigProperty;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import java.time.Duration;

/**
 * CDI bean that manages the NATS connection.
 *
 * <p>The NATS server can be embedded (running in-process) or external.
 * Configuration:
 * <ul>
 *   <li>{@code quark.nats.url} — NATS server URL (default: {@code nats://localhost:4222})</li>
 * </ul>
 *
 * <p>For embedded NATS, the server module starts the NATS server before
 * this bean is created. This bean just connects to it.
 */
@ApplicationScoped
public class NatsConnectionManager {

    private static final Logger log = LoggerFactory.getLogger(NatsConnectionManager.class);

    @ConfigProperty(name = "quark.nats.url", defaultValue = "nats://localhost:4222")
    String natsUrl;

    private Connection connection;

    @Produces
    @ApplicationScoped
    public Connection produceConnection() throws Exception {
        if (connection != null) {
            return connection;
        }

        log.info("Connecting to NATS at {}", natsUrl);

        Options options = Options.builder()
                .server(natsUrl)
                .connectionTimeout(Duration.ofSeconds(5))
                .reconnectWait(Duration.ofSeconds(1))
                .maxReconnects(-1) // infinite reconnects
                .build();

        connection = Nats.connect(options);
        log.info("Connected to NATS: {}", connection.getServerInfo().getServerId());

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
