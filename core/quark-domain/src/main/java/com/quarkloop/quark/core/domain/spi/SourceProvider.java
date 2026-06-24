package com.quarkloop.quark.core.domain.spi;

import com.quarkloop.quark.core.domain.config.NodeConfig;

/**
 * SPI for Source nodes.
 *
 * <p>Sources are autonomous — they produce data on their own schedule and
 * publish via the {@link QuarkPublisher}. They do not receive messages.
 */
public interface SourceProvider {

    /**
     * Start the source. The provider should begin producing data and
     * publishing events via the publisher.
     */
    void start(QuarkPublisher publisher, NodeConfig config);

    /** Stop the source and release all nodes. */
    void stop();
}
