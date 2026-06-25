package com.quarkloop.quark.runtime.domain.node;

import com.quarkloop.quark.runtime.domain.config.NodeConfig;
import com.quarkloop.quark.runtime.domain.identity.NodeUri;
import com.quarkloop.quark.runtime.domain.metadata.NodeMetadata;

import java.util.Objects;

/**
 * Default implementation of {@link Node}.
 *
 * <p>A simple immutable record. The node's behavior is determined at
 * runtime by which {@link com.quarkloop.quark.runtime.domain.spi.NodeProvider}
 * methods it overrides (onMessage, start, close, etc.) — there is no
 * behavioral type or category field.
 */
public record SimpleNode(
        String name,
        NodeUri uri,
        NodeConfig config,
        NodeMetadata metadata
) implements Node {
    public SimpleNode {
        Objects.requireNonNull(name, "name cannot be null");
        Objects.requireNonNull(uri, "uri cannot be null");
        if (config == null) config = NodeConfig.empty();
        if (metadata == null) metadata = NodeMetadata.initial();
    }
}
