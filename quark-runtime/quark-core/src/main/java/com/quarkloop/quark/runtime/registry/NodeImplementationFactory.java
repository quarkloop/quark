package com.quarkloop.quark.runtime.registry;

import com.quarkloop.quark.runtime.domain.config.NodeConfig;
import com.quarkloop.quark.runtime.domain.spi.NodeProvider;

/**
 * SPI for node implementation providers.
 *
 * <p>Each factory creates {@link NodeProvider} instances from configuration.
 * The engine detects which methods the provider overrides at runtime
 * (onMessage, start, close, etc.) — there is no behavioral type or
 * category field on the factory.
 */
public interface NodeImplementationFactory {

    /**
     * @return The URI pattern this factory handles, e.g. "quark/time/schedule/timer" (without version)
     */
    String uriPattern();

    /**
     * Create a node provider instance from the given configuration.
     *
     * @param config the node's configuration
     * @return a new NodeProvider instance
     */
    NodeProvider create(NodeConfig config);

    /**
     * @return The descriptor for registry registration
     */
    NodeDescriptor descriptor();
}
