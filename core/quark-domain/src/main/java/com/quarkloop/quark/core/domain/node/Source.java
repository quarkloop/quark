package com.quarkloop.quark.core.domain.node;

import com.quarkloop.quark.core.domain.category.NodeCategory;
import com.quarkloop.quark.core.domain.config.NodeConfig;
import com.quarkloop.quark.core.domain.identity.NodeUri;
import com.quarkloop.quark.core.domain.metadata.NodeMetadata;

import java.util.Objects;

/**
 * An origin of information.
 */
public record Source(
        String name,
        NodeUri uri,
        NodeConfig config,
        NodeMetadata metadata
) implements PassiveNode {
    public Source {
        Objects.requireNonNull(name, "name cannot be null");
        Objects.requireNonNull(uri, "uri cannot be null");
        Objects.requireNonNull(config, "config cannot be null");
        Objects.requireNonNull(metadata, "metadata cannot be null");
    }

    @Override
    public NodeCategory category() {
        return NodeCategory.SOURCE;
    }
}
