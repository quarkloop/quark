package com.quarkloop.quark.core.domain.system;

import com.quarkloop.quark.core.domain.identity.Namespace;
import com.quarkloop.quark.core.domain.node.Node;

import java.util.Map;
import java.util.Objects;

/**
 * Parsed representation of a complete .quark.ts system definition.
 *
 * <p>Produced by the GraalJS script layer when evaluating a user's TypeScript file.
 * Consumed by the engine layer to create NATS consumers, ACLs, and provider instances.
 */
public record SystemDefinition(
        String name,
        Namespace namespace,
        Map<String, NodeDefinition> nodes
) {
    public SystemDefinition {
        Objects.requireNonNull(name, "system name cannot be null");
        Objects.requireNonNull(namespace, "namespace cannot be null");
        if (name.isBlank()) {
            throw new IllegalArgumentException("system name cannot be blank");
        }
        nodes = nodes == null ? Map.of() : Map.copyOf(nodes);
    }
}
