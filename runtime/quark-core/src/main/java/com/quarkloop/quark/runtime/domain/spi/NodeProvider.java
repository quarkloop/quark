package com.quarkloop.quark.runtime.domain.spi;

import com.quarkloop.quark.runtime.domain.config.NodeConfig;

/**
 * Unified SPI for all Quark nodes.
 *
 * <p>A single interface with default methods. The engine detects which
 * methods a node overrides and calls them accordingly:
 *
 * <ul>
 *   <li>{@link #init} — called once before any other method</li>
 *   <li>{@link #start} — called after init, for nodes that produce data autonomously</li>
 *   <li>{@link #onMessage} — called per inbound NATS message</li>
 *   <li>{@link #close} — called on undeploy, always</li>
 * </ul>
 *
 * <p>A node does not need to implement all methods. Examples:
 * <ul>
 *   <li>A timer overrides {@code init}, {@code start}, {@code close} (no onMessage).</li>
 *   <li>A JSON parser overrides {@code init}, {@code onMessage}, {@code close} (no start).</li>
 *   <li>An SSE endpoint overrides all four.</li>
 * </ul>
 *
 * <p>There are no behavioral categories — the domain (from the URI) is the
 * only organizational axis. The engine figures out how to run the node by
 * checking which methods are overridden.
 */
public interface NodeProvider {

    /**
     * Initialize the node. Called once before any other method.
     *
     * <p>Use this to validate config, open connections, etc.
     *
     * @param config the node's configuration
     */
    default void init(NodeConfig config) {}

    /**
     * Start the node. Called after {@link #init}, if the node needs to
     * produce data autonomously (e.g., a timer, a file watcher, an HTTP server).
     *
     * <p>The engine calls this method only if the node overrides it
     * (i.e., the method is not the default no-op).
     *
     * @param publisher the publisher for sending events
     * @param config    the node's configuration
     */
    default void start(QuarkPublisher publisher, NodeConfig config) {}

    /**
     * Process an inbound NATS message.
     *
     * <p>Called per message delivered to a subject the node listens to.
     * The engine calls this method only if the node overrides it.
     *
     * @param message  the incoming message
     * @param publisher the publisher for sending results
     */
    default void onMessage(QuarkMessage message, QuarkPublisher publisher) {}

    /**
     * Close the node and release all resources. Called on undeploy.
     *
     * <p>Always called, even if {@link #init} or {@link #start} threw an
     * exception. Implementations must be safe to call even if the node was
     * never fully initialized.
     */
    default void close() {}
}
