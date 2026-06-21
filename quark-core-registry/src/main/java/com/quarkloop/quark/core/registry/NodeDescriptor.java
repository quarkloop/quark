package com.quarkloop.quark.core.registry;

import com.quarkloop.quark.core.domain.category.NodeCategory;
import com.quarkloop.quark.core.domain.identity.NodeUri;

import java.util.Objects;

/**
 * Describes a registered node implementation.
 */
public record NodeDescriptor(
        NodeUri uri,
        NodeCategory category,
        boolean active,
        String description
) {
    public NodeDescriptor {
        Objects.requireNonNull(uri, "uri cannot be null");
        Objects.requireNonNull(category, "category cannot be null");
        Objects.requireNonNull(description, "description cannot be null");
    }
}
