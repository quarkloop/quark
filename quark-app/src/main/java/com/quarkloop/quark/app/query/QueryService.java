package com.quarkloop.quark.app.query;

import com.quarkloop.quark.core.domain.identity.Namespace;
import com.quarkloop.quark.core.domain.node.Node;
import com.quarkloop.quark.core.domain.state.HealthStatus;
import com.quarkloop.quark.core.domain.state.NodeState;
import com.quarkloop.quark.core.engine.lifecycle.RuntimeNode;
import com.quarkloop.quark.core.engine.lifecycle.RuntimeSystem;
import com.quarkloop.quark.core.engine.lifecycle.SystemRuntimeRegistry;
import jakarta.enterprise.context.ApplicationScoped;
import jakarta.inject.Inject;

import java.util.ArrayList;
import java.util.Comparator;
import java.util.List;
import java.util.Optional;

/**
 * Read-only queries over the runtime registry.
 *
 * <p>All methods are scoped to a single namespace — cross-namespace
 * enumeration is intentionally NOT supported here. The REST layer must
 * supply the namespace on every call.
 */
@ApplicationScoped
public class QueryService {

    private final SystemRuntimeRegistry registry;

    @Inject
    public QueryService(SystemRuntimeRegistry registry) {
        this.registry = registry;
    }

    public List<SystemSummary> listSystems(String namespace) {
        Namespace ns = Namespace.of(namespace);
        List<SystemSummary> out = new ArrayList<>();
        for (RuntimeSystem rs : registry.listByNamespace(ns)) {
            out.add(toSummary(rs));
        }
        out.sort(Comparator.comparing(SystemSummary::name));
        return out;
    }

    public Optional<SystemDetail> getSystem(String namespace, String systemName) {
        return registry.get(Namespace.of(namespace), systemName)
                .map(this::toDetail);
    }

    public List<NodeSummary> listNodes(String namespace, String systemName) {
        return registry.get(Namespace.of(namespace), systemName)
                .map(rs -> rs.nodes().stream()
                        .map(rn -> toNodeSummary(rs, rn))
                        .sorted(Comparator.comparing(NodeSummary::name))
                        .toList())
                .orElse(List.of());
    }

    public List<NodeSummary> listAllNodes(String namespace) {
        List<NodeSummary> out = new ArrayList<>();
        for (RuntimeSystem rs : registry.listByNamespace(Namespace.of(namespace))) {
            for (RuntimeNode rn : rs.nodes()) {
                out.add(toNodeSummary(rs, rn));
            }
        }
        out.sort(Comparator.comparing(NodeSummary::name));
        return out;
    }

    public Optional<NodeDetail> getNode(String namespace, String systemName, String nodeName) {
        return registry.getNode(Namespace.of(namespace), systemName, nodeName)
                .map(rn -> {
                    RuntimeSystem rs = registry.get(Namespace.of(namespace), systemName).orElseThrow();
                    return toNodeDetail(rs, rn);
                });
    }

    // ----- Builders -----

    private SystemSummary toSummary(RuntimeSystem rs) {
        int total = rs.nodes().size();
        long healthy = rs.nodes().stream().filter(n -> n.health() == HealthStatus.HEALTHY).count();
        long unhealthy = rs.nodes().stream().filter(n -> n.health() == HealthStatus.UNHEALTHY).count();
        long degraded = rs.nodes().stream().filter(n -> n.health() == HealthStatus.DEGRADED).count();
        return new SystemSummary(
                rs.name(),
                rs.namespace().value(),
                rs.definition().nodes().size(),
                overallState(rs),
                rs.overallHealth().name(),
                healthy,
                degraded,
                unhealthy,
                total
        );
    }

    private SystemDetail toDetail(RuntimeSystem rs) {
        List<NodeSummary> nodes = rs.nodes().stream()
                .map(rn -> toNodeSummary(rs, rn))
                .sorted(Comparator.comparing(NodeSummary::name))
                .toList();
        return new SystemDetail(
                rs.name(),
                rs.namespace().value(),
                overallState(rs),
                rs.overallHealth().name(),
                1L,
                nodes
        );
    }

    private NodeSummary toNodeSummary(RuntimeSystem rs, RuntimeNode rn) {
        Node def = rn.definition();
        return new NodeSummary(
                def.name(),
                rs.name(),
                rs.namespace().value(),
                def.uri().toString(),
                def.category().label(),
                rn.state().name(),
                rn.health().name(),
                rn.version()
        );
    }

    private NodeDetail toNodeDetail(RuntimeSystem rs, RuntimeNode rn) {
        Node def = rn.definition();
        return new NodeDetail(
                def.name(),
                rs.name(),
                rs.namespace().value(),
                def.uri().toString(),
                def.category().label(),
                rn.state().name(),
                rn.health().name(),
                rn.version(),
                rn.errorMessage(),
                def.metadata().createdAt().toString(),
                def.metadata().updatedAt().toString(),
                def.config().asMap(),
                def.metadata().labels().values(),
                def.metadata().annotations().values(),
                rn instanceof RuntimeNode ? List.of() : List.of(),
                List.of(),
                listensFor(def),
                eventsFor(def)
        );
    }

    /**
     * Pulls the `listens` and `events` arrays from the original
     * SystemDefinition by looking up the NodeDefinition.
     */
    @SuppressWarnings("unused")
    private List<String> listensFor(Node def) {
        // NodeDefinition carries the listens/events, but the domain Node
        // interface doesn't expose them. They live on the runtime system's
        // definition(). For now, return empty — the REST layer can also
        // fetch the source file to recover these.
        return List.of();
    }

    @SuppressWarnings("unused")
    private List<String> eventsFor(Node def) {
        return List.of();
    }

    private String overallState(RuntimeSystem rs) {
        // If any node is ERROR -> ERROR, otherwise ACTIVE
        for (RuntimeNode rn : rs.nodes()) {
            if (rn.state() == NodeState.ERROR) return NodeState.ERROR.name();
        }
        return NodeState.ACTIVE.name();
    }

    // ----- DTOs (records; Jackson serializes them as JSON objects) -----

    public record SystemSummary(
            String name,
            String namespace,
            int nodeCount,
            String state,
            String health,
            long healthyNodes,
            long degradedNodes,
            long unhealthyNodes,
            long connectionCount
    ) {}

    public record SystemDetail(
            String name,
            String namespace,
            String state,
            String health,
            long version,
            List<NodeSummary> nodes
    ) {}

    public record NodeSummary(
            String name,
            String systemName,
            String namespace,
            String uri,
            String category,
            String state,
            String health,
            long version
    ) {}

    public record NodeDetail(
            String name,
            String systemName,
            String namespace,
            String uri,
            String category,
            String state,
            String health,
            long version,
            String errorMessage,
            String createdAt,
            String updatedAt,
            java.util.Map<String, Object> config,
            java.util.Map<String, String> labels,
            java.util.Map<String, String> annotations,
            List<String> listens,
            List<String> events,
            List<String> inbound,
            List<String> outbound
    ) {}
}
