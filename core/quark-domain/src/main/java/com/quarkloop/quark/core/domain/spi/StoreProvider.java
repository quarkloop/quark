package com.quarkloop.quark.core.domain.spi;

/**
 * SPI for Store nodes.
 *
 * <p>Stores are reactive — same interface as functions. They persist data
 * and optionally publish events (e.g., "updated").
 */
public interface StoreProvider {

    /**
     * Process an incoming message (typically persist it).
     *
     * @param message  the incoming NATS message
     * @param publisher the publisher for sending events
     */
    void onMessage(QuarkMessage message, QuarkPublisher publisher);
}
