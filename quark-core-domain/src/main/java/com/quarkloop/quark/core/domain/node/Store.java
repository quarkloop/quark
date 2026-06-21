package com.quarkloop.quark.core.domain.node;

import com.quarkloop.quark.core.domain.category.NodeCategory;
import com.quarkloop.quark.core.domain.config.NodeConfig;
import com.quarkloop.quark.core.domain.identity.NodeUri;
import com.quarkloop.quark.core.domain.metadata.NodeMetadata;

import java.util.Objects;

/**
 * A persistent, queryable structure for holding data.
 */
public record Store(
        String name,
        NodeUri uri,
        NodeConfig config,
        NodeMetadata metadata
) implements PassiveNode {
    public Store {
        Objects.requireNonNull(name, "name cannot be null");
        Objects.requireNonNull(uri, "uri cannot be null");
        Objects.requireNonNull(config, "config cannot be null");
        Objects.requireNonNull(metadata, "metadata cannot be null");
    }

    @Override
    public NodeCategory category() {
        return NodeCategory.STORE;
    }
}
