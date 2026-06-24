package com.quarkloop.quark.core.domain.node;

import com.quarkloop.quark.core.domain.config.NodeConfig;
import com.quarkloop.quark.core.domain.identity.NodeUri;
import com.quarkloop.quark.core.domain.metadata.NodeMetadata;

import java.util.Objects;

/**
 * Default implementation of {@link Node}.
 *
 * <p>A simple immutable record. No behavioral category — the node's behavior
 * is determined at runtime by which {@link com.quarkloop.quark.core.domain.spi.NodeProvider}
 * methods it overrides.
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
