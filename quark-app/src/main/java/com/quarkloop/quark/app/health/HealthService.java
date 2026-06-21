package com.quarkloop.quark.app.health;

import com.quarkloop.quark.core.domain.event.NodeEvent;
import com.quarkloop.quark.core.domain.event.NodeEventKind;
import com.quarkloop.quark.core.domain.identity.Namespace;
import com.quarkloop.quark.core.domain.state.HealthStatus;
import com.quarkloop.quark.core.engine.lifecycle.RuntimeNode;
import com.quarkloop.quark.core.engine.lifecycle.RuntimeSystem;
import com.quarkloop.quark.core.engine.lifecycle.SystemRuntimeRegistry;
import com.quarkloop.quark.core.event.EventFilter;
import com.quarkloop.quark.core.event.EventStore;
import jakarta.enterprise.context.ApplicationScoped;
import jakarta.inject.Inject;

import java.util.HashMap;
import java.util.List;
import java.util.Map;
import java.util.Optional;

/**
 * Aggregate health queries across the platform.
 *
 * <p>Three scopes:
 * <ul>
 *   <li>Platform-wide — all systems across all namespaces.</li>
 *   <li>Per-namespace — all systems within one namespace.</li>
 *   <li>Per-system / per-node — drill-down with recent events.</li>
 * </ul>
 */
@ApplicationScoped
public class HealthService {

    private final SystemRuntimeRegistry registry;
    private final EventStore eventStore;

    @Inject
    public HealthService(SystemRuntimeRegistry registry, EventStore eventStore) {
        this.registry = registry;
        this.eventStore = eventStore;
    }

    public HealthSummary platformHealth() {
        return aggregate(registry.all());
    }

    public HealthSummary namespaceHealth(String namespace) {
        return aggregate(registry.listByNamespace(Namespace.of(namespace)));
    }

    public Optional<SystemHealth> systemHealth(String namespace, String systemName) {
        return registry.get(Namespace.of(namespace), systemName)
                .map(rs -> {
                    Map<String, String> perNode = new HashMap<>();
                    for (RuntimeNode rn : rs.nodes()) {
                        perNode.put(rn.definition().name(), rn.health().name());
                    }
                    return new SystemHealth(
                            rs.name(),
                            rs.namespace().value(),
                            rs.overallHealth().name(),
                            rs.nodes().size(),
                            perNode
                    );
                });
    }

    public Optional<NodeHealth> nodeHealth(String namespace, String systemName, String nodeName) {
        return registry.getNode(Namespace.of(namespace), systemName, nodeName)
                .map(rn -> {
                    EventFilter filter = EventFilter.builder()
                            .namespace(namespace)
                            .systemName(systemName)
                            .nodeName(nodeName)
                            .limit(10)
                            .build();
                    List<NodeEvent> recent = eventStore.query(filter);
                    return new NodeHealth(
                            rn.definition().name(),
                            rn.state().name(),
                            rn.health().name(),
                            rn.version(),
                            rn.errorMessage(),
                            recent
                    );
                });
    }

    private HealthSummary aggregate(java.util.Collection<RuntimeSystem> systems) {
        int totalSystems = systems.size();
        int totalNodes = 0;
        int healthy = 0;
        int degraded = 0;
        int unhealthy = 0;
        int unknown = 0;
        String overall = "HEALTHY";
        for (RuntimeSystem rs : systems) {
            for (RuntimeNode rn : rs.nodes()) {
                totalNodes++;
                switch (rn.health()) {
                    case HEALTHY   -> healthy++;
                    case DEGRADED  -> degraded++;
                    case UNHEALTHY -> unhealthy++;
                    case UNKNOWN   -> unknown++;
                }
            }
            if (rs.overallHealth() == HealthStatus.UNHEALTHY) overall = "UNHEALTHY";
            else if (rs.overallHealth() == HealthStatus.DEGRADED && !"UNHEALTHY".equals(overall)) overall = "DEGRADED";
            else if (rs.overallHealth() == HealthStatus.UNKNOWN && "HEALTHY".equals(overall)) overall = "UNKNOWN";
        }
        return new HealthSummary(
                totalSystems,
                totalNodes,
                healthy,
                degraded,
                unhealthy,
                unknown,
                overall
        );
    }

    // ----- DTOs -----

    public record HealthSummary(
            int totalSystems,
            int totalNodes,
            int healthyNodes,
            int degradedNodes,
            int unhealthyNodes,
            int unknownNodes,
            String overall
    ) {}

    public record SystemHealth(
            String systemName,
            String namespace,
            String overall,
            int nodeCount,
            Map<String, String> perNode
    ) {}

    public record NodeHealth(
            String nodeName,
            String state,
            String health,
            long version,
            String errorMessage,
            List<NodeEvent> recentEvents
    ) {}
}
