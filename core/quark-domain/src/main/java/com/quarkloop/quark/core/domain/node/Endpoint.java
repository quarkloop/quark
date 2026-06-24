package com.quarkloop.quark.core.domain.node;

import com.quarkloop.quark.core.domain.category.NodeCategory;
import com.quarkloop.quark.core.domain.config.NodeConfig;
import com.quarkloop.quark.core.domain.identity.NodeUri;
import com.quarkloop.quark.core.domain.metadata.NodeMetadata;

import java.util.Objects;

/**
 * An external interface.
 */
public record Endpoint(
        String name,
        NodeUri uri,
        NodeConfig config,
        NodeMetadata metadata
) implements PassiveNode {
    public Endpoint {
        Objects.requireNonNull(name, "name cannot be null");
        Objects.requireNonNull(uri, "uri cannot be null");
        Objects.requireNonNull(config, "config cannot be null");
        Objects.requireNonNull(metadata, "metadata cannot be null");
    }

    @Override
    public NodeCategory category() {
        return NodeCategory.ENDPOINT;
    }
}
