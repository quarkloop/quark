package com.quarkloop.quark.core.domain.identity;

import com.quarkloop.quark.core.domain.category.NodeCategory;

import java.util.Objects;

/**
 * A fully-qualified global address for a Node.
 * Format: quark://<instance>/<namespace>/<category>/<name>
 */
public record NodeAddress(
        String instance,
        Namespace namespace,
        NodeCategory category,
        NodeName name
) {
    public NodeAddress {
        if (instance == null || instance.isBlank()) {
            throw new IllegalArgumentException("instance cannot be null or blank");
        }
        Objects.requireNonNull(namespace, "namespace cannot be null");
        Objects.requireNonNull(category, "category cannot be null");
        Objects.requireNonNull(name, "name cannot be null");
    }

    public static NodeAddress of(String instance, Namespace namespace, NodeCategory category, NodeName name) {
        return new NodeAddress(instance, namespace, category, name);
    }

    @Override
    public String toString() {
        return String.format("quark://%s/%s/%s/%s",
                instance,
                namespace.value(),
                category.label(),
                name.value());
    }
}
