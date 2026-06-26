package com.quarkloop.quark.runtime.domain.system;

import com.quarkloop.quark.runtime.domain.identity.Namespace;

import java.util.Map;
import java.util.Objects;

/**
 * Parsed representation of a complete .quark.ts system definition.
 *
 * <p>Produced by the GraalJS script layer when evaluating a user's TypeScript file.
 * Consumed by the engine layer to create NATS consumers, ACLs, and provider instances.
 *
 * @param runtime the runtime isolation mode: "shared" (default) or "isolated".
 *                "shared" — the system runs in the shared data plane process
 *                alongside other non-isolated namespaces.
 *                "isolated" — the system runs in a dedicated data plane process
 *                for its namespace, providing full process-level isolation.
 */
public record SystemDefinition(
        String name,
        Namespace namespace,
        Map<String, NodeDefinition> nodes,
        String runtime
) {
    public static final String RUNTIME_SHARED = "shared";
    public static final String RUNTIME_ISOLATED = "isolated";

    public SystemDefinition {
        Objects.requireNonNull(name, "system name cannot be null");
        Objects.requireNonNull(namespace, "namespace cannot be null");
        if (name.isBlank()) {
            throw new IllegalArgumentException("system name cannot be blank");
        }
        nodes = nodes == null ? Map.of() : Map.copyOf(nodes);
        runtime = (runtime == null || runtime.isBlank()) ? RUNTIME_SHARED : runtime.toLowerCase();
    }

    /**
     * Convenience constructor defaulting runtime to "shared".
     */
    public SystemDefinition(String name, Namespace namespace, Map<String, NodeDefinition> nodes) {
        this(name, namespace, nodes, RUNTIME_SHARED);
    }

    public boolean isIsolated() {
        return RUNTIME_ISOLATED.equals(runtime);
    }
}
