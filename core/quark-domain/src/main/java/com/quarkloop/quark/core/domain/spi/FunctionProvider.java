package com.quarkloop.quark.core.domain.spi;

/**
 * SPI for Function nodes.
 *
 * <p>Functions are reactive — they receive messages via {@code onMessage},
 * process them, and publish results via the publisher.
 */
public interface FunctionProvider {

    /**
     * Process an incoming message.
     *
     * @param message  the incoming NATS message
     * @param publisher the publisher for sending results
     */
    void onMessage(QuarkMessage message, QuarkPublisher publisher);
}
