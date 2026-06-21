package com.quarkloop.quark.core.domain.spi;

/**
 * SPI for Policy nodes.
 *
 * <p>Policies intercept messages — they receive, evaluate, and either
 * allow (publish to original target) or deny (drop or route to fallback).
 */
public interface PolicyProvider {

    /**
     * Evaluate a policy against an incoming message.
     *
     * @param message  the incoming NATS message
     * @param publisher the publisher for forwarding allowed messages
     */
    void onMessage(QuarkMessage message, QuarkPublisher publisher);
}
