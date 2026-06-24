package com.quarkloop.quark.core.domain.spi;

import com.quarkloop.quark.core.domain.config.NodeConfig;

/**
 * SPI for Endpoint nodes.
 *
 * <p>Endpoints are hybrid — they start their own server (HTTP, SSE) AND
 * receive messages from NATS. An SSE endpoint listens for events and
 * pushes them to connected HTTP clients.
 */
public interface EndpointProvider {

    /**
     * Start the endpoint (e.g., open HTTP server, register routes).
     */
    void start(QuarkPublisher publisher, NodeConfig config);

    /**
     * Process an incoming NATS message (e.g., push to SSE clients).
     */
    void onMessage(QuarkMessage message, QuarkPublisher publisher);

    /** Stop the endpoint and release all nodes. */
    void stop();
}
