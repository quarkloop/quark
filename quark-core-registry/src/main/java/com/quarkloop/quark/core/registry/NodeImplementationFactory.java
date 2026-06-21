package com.quarkloop.quark.core.registry;

import com.quarkloop.quark.core.domain.category.NodeCategory;
import com.quarkloop.quark.core.domain.config.NodeConfig;

/**
 * SPI for node implementation providers.
 */
public interface NodeImplementationFactory<T> {

    /**
     * @return The URI pattern this factory handles, e.g. "source/timer" (without version)
     */
    String uriPattern();

    /**
     * @return Create an instance from the given configuration
     */
    T create(NodeConfig config);

    /**
     * @return The descriptor for registry registration
     */
    NodeDescriptor descriptor();

    /**
     * @return The category this factory produces
     */
    NodeCategory category();
}
