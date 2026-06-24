package com.quarkloop.quark.core.engine.lifecycle;

import com.quarkloop.quark.core.domain.identity.Namespace;
import com.quarkloop.quark.core.domain.system.SystemDefinition;
import com.quarkloop.quark.core.domain.state.HealthStatus;

import java.util.Collection;
import java.util.Map;
import java.util.Objects;
import java.util.concurrent.ConcurrentHashMap;

/**
 * Runtime state for a single deployed system.
 *
 * <p>Holds the immutable {@link SystemDefinition} and the runtime node instances
 * keyed by node name. In the NATS architecture, there is no in-process traversal —
 * routing is handled by NATS subjects. This class just tracks nodes for the
 * management API (list, health, lifecycle).
 */
public final class RuntimeSystem {

    private final SystemDefinition definition;
    private final Namespace namespace;
    private final Map<String, RuntimeNode> nodesByName = new ConcurrentHashMap<>();

    public RuntimeSystem(SystemDefinition definition, Namespace namespace) {
        this.definition = Objects.requireNonNull(definition);
        this.namespace = Objects.requireNonNull(namespace);
    }

    public SystemDefinition definition() { return definition; }
    public Namespace namespace() { return namespace; }
    public String name() { return definition.name(); }

    public void register(RuntimeNode node) {
        nodesByName.put(node.definition().name(), node);
    }

    public RuntimeNode getNode(String name) {
        return nodesByName.get(name);
    }

    public Collection<RuntimeNode> nodes() {
        return nodesByName.values();
    }

    public HealthStatus overallHealth() {
        HealthStatus worst = HealthStatus.HEALTHY;
        for (RuntimeNode rn : nodesByName.values()) {
            HealthStatus h = rn.health();
            if (h == HealthStatus.UNHEALTHY) return HealthStatus.UNHEALTHY;
            if (h == HealthStatus.DEGRADED && worst != HealthStatus.UNHEALTHY) worst = HealthStatus.DEGRADED;
            if (h == HealthStatus.UNKNOWN && worst == HealthStatus.HEALTHY) worst = HealthStatus.UNKNOWN;
        }
        return worst;
    }

    @Override
    public String toString() {
        return "RuntimeSystem{" + namespace.value() + "/" + definition.name() +
                " nodes=" + nodesByName.size() + "}";
    }
}
