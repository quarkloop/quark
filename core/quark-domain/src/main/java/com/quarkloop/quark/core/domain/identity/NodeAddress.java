package com.quarkloop.quark.core.domain.identity;

import java.util.Objects;

/**
 * A fully-qualified global address for a Node.
 * Format: quark://<instance>/<namespace>/<domain>/<subdomain>/<node>
 */
public record NodeAddress(
        String instance,
        Namespace namespace,
        NodeUri uri
) {
    public NodeAddress {
        if (instance == null || instance.isBlank()) {
            throw new IllegalArgumentException("instance cannot be null or blank");
        }
        Objects.requireNonNull(namespace, "namespace cannot be null");
        Objects.requireNonNull(uri, "uri cannot be null");
    }

    public static NodeAddress of(String instance, Namespace namespace, NodeUri uri) {
        return new NodeAddress(instance, namespace, uri);
    }

    @Override
    public String toString() {
        return String.format("quark://%s/%s/%s",
                instance,
                namespace.value(),
                uri.pattern());
    }
}
