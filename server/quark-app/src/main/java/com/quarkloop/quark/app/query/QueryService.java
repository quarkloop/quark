package com.quarkloop.quark.app.query;

import com.quarkloop.quark.runtime.domain.identity.Namespace;
import com.quarkloop.quark.runtime.domain.node.Node;
import com.quarkloop.quark.runtime.domain.state.HealthStatus;
import com.quarkloop.quark.runtime.domain.state.NodeState;
import com.quarkloop.quark.runtime.domain.system.NodeDefinition;
import com.quarkloop.quark.runtime.engine.lifecycle.RuntimeNode;
import com.quarkloop.quark.runtime.engine.lifecycle.RuntimeSystem;
import com.quarkloop.quark.runtime.engine.runtime.RuntimeContext;
import com.quarkloop.quark.runtime.engine.store.SystemRecord;
import com.quarkloop.quark.runtime.engine.store.SystemRepository;
import com.quarkloop.quark.runtime.engine.store.NodeRepository;
import com.quarkloop.quark.runtime.engine.store.NodeRecord;
import jakarta.enterprise.context.ApplicationScoped;
import jakarta.inject.Inject;

import java.time.Instant;
import java.util.ArrayList;
import java.util.Comparator;
import java.util.List;
import java.util.Map;
import java.util.Optional;

/**
 * Read-only queries over the runtime registry and persistent store.
 *
 * <p>In control-plane mode, system/node listings are read from the Catalog
 * persistent store (the control plane's RuntimeContext is empty because
 * systems execute in data-plane processes). In data-plane mode, listings
 * are read from the in-memory RuntimeContext (the actual runtime).
 *
 * <p>All methods are scoped to a single namespace — cross-namespace
 * enumeration is intentionally NOT supported here.
 */
@ApplicationScoped
public class QueryService {

    private final RuntimeContext runtimeContext;
    private final SystemRepository systemRepository;
    private final NodeRepository nodeRepository;

    @Inject
    public QueryService(RuntimeContext runtimeContext,
                        SystemRepository systemRepository,
                        NodeRepository nodeRepository) {
        this.runtimeContext = runtimeContext;
        this.systemRepository = systemRepository;
        this.nodeRepository = nodeRepository;
    }

    public List<SystemSummary> listSystems(String namespace) {
        // Try the in-memory runtime first (data-plane mode). If empty,
        // fall back to the persistent store (control-plane mode).
        List<RuntimeSystem> runtime = runtimeContext.getSystemsByNamespace(Namespace.of(namespace));
        if (!runtime.isEmpty()) {
            List<SystemSummary> out = new ArrayList<>();
            for (RuntimeSystem rs : runtime) {
                out.add(toSummary(rs));
            }
            out.sort(Comparator.comparing(SystemSummary::name));
            return out;
        }
        // Control-plane mode: read from the Catalog
        List<SystemSummary> out = new ArrayList<>();
        for (SystemRecord sr : systemRepository.findByNamespace(namespace)) {
            int nodeCount = nodeRepository.findBySystem(namespace, sr.name()).size();
            out.add(new SystemSummary(
                    sr.name(), sr.namespace(), nodeCount,
                    sr.state(), sr.health(), nodeCount, 0, 0, nodeCount));
        }
        out.sort(Comparator.comparing(SystemSummary::name));
        return out;
    }

    public Optional<SystemDetail> getSystem(String namespace, String systemName) {
        // Try in-memory first (data-plane mode)
        Optional<RuntimeSystem> runtime = runtimeContext.getSystem(Namespace.of(namespace), systemName);
        if (runtime.isPresent()) {
            return runtime.map(this::toDetail);
        }
        // Control-plane mode: read from the Catalog
        return systemRepository.findByNamespaceAndName(namespace, systemName)
                .map(sr -> {
                    List<NodeSummary> nodes = listNodes(namespace, systemName);
                    return new SystemDetail(
                            sr.name(), sr.namespace(), sr.state(), sr.health(),
                            sr.version(), sr.createdAt().toString(), sr.updatedAt().toString(),
                            nodes);
                });
    }

    public List<NodeSummary> listNodes(String namespace, String systemName) {
        // Try in-memory first (data-plane mode)
        Optional<RuntimeSystem> runtime = runtimeContext.getSystem(Namespace.of(namespace), systemName);
        if (runtime.isPresent()) {
            return runtime.get().nodes().stream()
                    .map(rn -> toNodeSummary(runtime.get(), rn))
                    .sorted(Comparator.comparing(NodeSummary::name))
                    .toList();
        }
        // Control-plane mode: read from the Catalog
        List<NodeSummary> out = new ArrayList<>();
        for (NodeRecord nr : nodeRepository.findBySystem(namespace, systemName)) {
            out.add(new NodeSummary(
                    nr.name(), systemName, namespace, nr.uri(), 
                    nr.state(), nr.health(), nr.version()));
        }
        out.sort(Comparator.comparing(NodeSummary::name));
        return out;
    }

    public List<NodeSummary> listAllNodes(String namespace) {
        // Try in-memory first (data-plane mode)
        List<RuntimeSystem> runtime = runtimeContext.getSystemsByNamespace(Namespace.of(namespace));
        if (!runtime.isEmpty()) {
            List<NodeSummary> out = new ArrayList<>();
            for (RuntimeSystem rs : runtime) {
                for (RuntimeNode rn : rs.nodes()) {
                    out.add(toNodeSummary(rs, rn));
                }
            }
            out.sort(Comparator.comparing(NodeSummary::name));
            return out;
        }
        // Control-plane mode: read from the Catalog
        List<NodeSummary> out = new ArrayList<>();
        for (NodeRecord nr : nodeRepository.findNodesByNamespace(namespace)) {
            out.add(new NodeSummary(
                    nr.name(), nr.systemName(), namespace, nr.uri(), 
                    nr.state(), nr.health(), nr.version()));
        }
        out.sort(Comparator.comparing(NodeSummary::name));
        return out;
    }

    public Optional<NodeDetail> getNode(String namespace, String systemName, String nodeName) {
        return runtimeContext.getNode(Namespace.of(namespace), systemName, nodeName)
                .map(rn -> {
                    RuntimeSystem rs = runtimeContext.getSystem(Namespace.of(namespace), systemName).orElseThrow();
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
        // Get creation timestamp from first node's metadata (all nodes in a system
        // are deployed at the same time)
        String createdAt = rs.nodes().stream().findFirst()
                .map(rn -> rn.definition().metadata().createdAt().toString())
                .orElse(Instant.now().toString());
        String updatedAt = rs.nodes().stream().findFirst()
                .map(rn -> rn.definition().metadata().updatedAt().toString())
                .orElse(createdAt);
        return new SystemDetail(
                rs.name(),
                rs.namespace().value(),
                overallState(rs),
                rs.overallHealth().name(),
                1L,
                createdAt,
                updatedAt,
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
                
                rn.state().name(),
                rn.health().name(),
                rn.version()
        );
    }

    private NodeDetail toNodeDetail(RuntimeSystem rs, RuntimeNode rn) {
        Node def = rn.definition();
        // Look up the NodeDefinition from the system definition to get
        // listens/events (the domain Node interface doesn't expose them).
        NodeDefinition nodeDef = rs.definition().nodes().get(def.name());
        List<String> listens = (nodeDef != null) ? nodeDef.listens() : List.of();
        List<String> events = (nodeDef != null) ? nodeDef.events() : List.of();

        // Filter out engine-internal config keys (prefixed with _quark_)
        // so they don't leak to clients.
        Map<String, Object> filteredConfig = new java.util.HashMap<>();
        for (var entry : def.config().asMap().entrySet()) {
            if (!entry.getKey().startsWith("_quark_")) {
                filteredConfig.put(entry.getKey(), entry.getValue());
            }
        }

        return new NodeDetail(
                def.name(),
                rs.name(),
                rs.namespace().value(),
                def.uri().toString(),
                
                rn.state().name(),
                rn.health().name(),
                rn.version(),
                rn.errorMessage(),
                def.metadata().createdAt().toString(),
                def.metadata().updatedAt().toString(),
                filteredConfig,
                def.metadata().labels().values(),
                def.metadata().annotations().values(),
                listens,
                events,
                listens,
                events
        );
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
            String createdAt,
            String updatedAt,
            List<NodeSummary> nodes
    ) {}

    public record NodeSummary(
            String name,
            String systemName,
            String namespace,
            String uri,
            String state,
            String health,
            long version
    ) {}

    public record NodeDetail(
            String name,
            String systemName,
            String namespace,
            String uri,
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
