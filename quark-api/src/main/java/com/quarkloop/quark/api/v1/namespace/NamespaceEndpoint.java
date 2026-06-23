package com.quarkloop.quark.api.v1.namespace;

import com.quarkloop.quark.app.metrics.NamespaceMetricsCollector;
import com.quarkloop.quark.core.engine.runtime.RuntimeContext;
import com.quarkloop.quark.core.engine.store.NodeRepository;
import com.quarkloop.quark.core.engine.store.SystemRecord;
import com.quarkloop.quark.core.engine.store.SystemRepository;
import jakarta.enterprise.context.ApplicationScoped;
import jakarta.inject.Inject;
import jakarta.ws.rs.GET;
import jakarta.ws.rs.Path;
import jakarta.ws.rs.PathParam;
import jakarta.ws.rs.Produces;
import jakarta.ws.rs.core.MediaType;
import jakarta.ws.rs.core.Response;

import java.lang.management.ManagementFactory;
import java.lang.management.MemoryMXBean;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import java.util.stream.Collectors;

/**
 * Namespace listing and detail endpoints.
 *
 * <p>The detail endpoint ({@code GET /api/v1/namespaces/{ns}}) returns
 * per-namespace metrics:
 * <ul>
 *   <li><b>CPU</b> — for shared namespaces, the CPU % attributed to message
 *       handlers for this namespace (measured via
 *       {@link java.lang.management.ThreadMXBean#getCurrentThreadCpuTime()}
 *       inside the handler path). For isolated namespaces, the entire
 *       process CPU is attributed to this namespace.</li>
 *   <li><b>Memory</b> — JVM-level heap/non-heap usage. For shared namespaces,
 *       memory is shared at the JVM level; for isolated namespaces, it's
 *       exact at the process level.</li>
 *   <li><b>Throughput</b> — messages published/received per second over the
 *       last 2-second interval.</li>
 *   <li><b>Error rate</b> — errors per second over the last 2-second interval.</li>
 * </ul>
 */
@Path("/api/v1/namespaces")
@ApplicationScoped
@Produces(MediaType.APPLICATION_JSON)
public class NamespaceEndpoint {

    private final RuntimeContext runtimeContext;
    private final NamespaceMetricsCollector metricsCollector;
    private final SystemRepository systemRepository;
    private final NodeRepository nodeRepository;

    @Inject
    public NamespaceEndpoint(RuntimeContext runtimeContext,
                             NamespaceMetricsCollector metricsCollector,
                             SystemRepository systemRepository,
                             NodeRepository nodeRepository) {
        this.runtimeContext = runtimeContext;
        this.metricsCollector = metricsCollector;
        this.systemRepository = systemRepository;
        this.nodeRepository = nodeRepository;
    }

    @GET
    public Response list() {
        // In control-plane mode, RuntimeContext is empty — read from DuckDB.
        // In data-plane mode, RuntimeContext has the live systems.
        java.util.Set<String> namespaces = new java.util.TreeSet<>();
        for (var rs : runtimeContext.getAllSystems()) {
            namespaces.add(rs.namespace().value());
        }
        // Also include namespaces from DuckDB (control-plane mode)
        for (SystemRecord sr : systemRepository.findAllSystems()) {
            namespaces.add(sr.namespace());
        }

        List<Map<String, Object>> out = namespaces.stream()
                .map(ns -> {
                    var runtimeSystems = runtimeContext.getSystemsByNamespace(ns);
                    int systemCount, totalNodes;
                    long healthy, unhealthy;
                    if (!runtimeSystems.isEmpty()) {
                        systemCount = runtimeSystems.size();
                        totalNodes = runtimeSystems.stream().mapToInt(s -> s.nodes().size()).sum();
                        healthy = runtimeSystems.stream().flatMap(s -> s.nodes().stream())
                                .filter(n -> n.health().name().equals("HEALTHY")).count();
                        unhealthy = runtimeSystems.stream().flatMap(s -> s.nodes().stream())
                                .filter(n -> n.health().name().equals("UNHEALTHY")).count();
                    } else {
                        List<SystemRecord> sysRecs = systemRepository.findByNamespace(ns);
                        systemCount = sysRecs.size();
                        // Count actual nodes from the nodes table
                        totalNodes = nodeRepository.findNodesByNamespace(ns).size();
                        healthy = nodeRepository.findNodesByNamespace(ns).stream()
                                .filter(n -> "HEALTHY".equals(n.health())).count();
                        unhealthy = nodeRepository.findNodesByNamespace(ns).stream()
                                .filter(n -> "UNHEALTHY".equals(n.health())).count();
                    }
                    Map<String, Object> entry = new LinkedHashMap<>();
                    entry.put("namespace", ns);
                    entry.put("systemCount", systemCount);
                    entry.put("nodeCount", totalNodes);
                    entry.put("healthyNodes", healthy);
                    entry.put("unhealthyNodes", unhealthy);
                    return entry;
                })
                .collect(Collectors.toList());
        return Response.ok(out).build();
    }

    @GET
    @Path("/{ns}")
    public Response get(@PathParam("ns") String namespace) {
        var systems = runtimeContext.getSystemsByNamespace(namespace);
        int totalNodes;
        long healthy, unhealthy;
        List<SystemRecord> sysRecs;
        List<Map<String, Object>> systemList;

        if (!systems.isEmpty()) {
            // Data-plane mode: read from RuntimeContext
            totalNodes = systems.stream().mapToInt(s -> s.nodes().size()).sum();
            healthy = systems.stream().flatMap(s -> s.nodes().stream())
                    .filter(n -> n.health().name().equals("HEALTHY")).count();
            unhealthy = systems.stream().flatMap(s -> s.nodes().stream())
                    .filter(n -> n.health().name().equals("UNHEALTHY")).count();
            sysRecs = List.of();
            systemList = systems.stream().map(s -> {
                Map<String, Object> m = new LinkedHashMap<>();
                m.put("name", s.name());
                m.put("nodeCount", s.nodes().size());
                m.put("state", s.nodes().stream().anyMatch(n -> n.state().name().equals("ERROR")) ? "ERROR" : "ACTIVE");
                m.put("health", s.overallHealth().name());
                return m;
            }).toList();
        } else {
            // Control-plane mode: read from DuckDB
            sysRecs = systemRepository.findByNamespace(namespace);
            if (sysRecs.isEmpty()) {
                return Response.status(Response.Status.NOT_FOUND).build();
            }
            // Count actual nodes from the nodes table
            var allNodes = nodeRepository.findNodesByNamespace(namespace);
            totalNodes = allNodes.size();
            healthy = allNodes.stream().filter(n -> "HEALTHY".equals(n.health())).count();
            unhealthy = allNodes.stream().filter(n -> "UNHEALTHY".equals(n.health())).count();
            systemList = sysRecs.stream().map(s -> {
                int sysNodes = nodeRepository.findBySystem(namespace, s.name()).size();
                Map<String, Object> m = new LinkedHashMap<>();
                m.put("name", s.name());
                m.put("nodeCount", sysNodes);
                m.put("state", s.state());
                m.put("health", s.health());
                return m;
            }).toList();
        }

        // JVM-wide memory — shared across all namespaces in this JVM.
        // For shared namespaces, this is the JVM-level memory. For isolated
        // namespaces (data-plane process), this is the process-level memory.
        MemoryMXBean mem = ManagementFactory.getMemoryMXBean();

        // Per-namespace CPU attribution + throughput + error rate.
        NamespaceMetricsCollector.NamespaceRate rate = metricsCollector.getRate(namespace);

        Map<String, Object> metricsBody = new LinkedHashMap<>();
        if (rate != null) {
            metricsBody.put("cpu", Map.of(
                    "namespacePercent", rate.cpuPercent(),
                    "availableProcessors", Runtime.getRuntime().availableProcessors()
            ));
            metricsBody.put("throughput", Map.of(
                    "messagesPublishedPerSec", rate.publishRate(),
                    "messagesReceivedPerSec", rate.receiveRate(),
                    "errorsPerSec", rate.errorRate(),
                    "totalPublished", rate.messagesPublished(),
                    "totalReceived", rate.messagesReceived(),
                    "totalErrors", rate.errors(),
                    "cpuTimeNanos", rate.cpuTimeNanos()
            ));
        } else {
            metricsBody.put("cpu", Map.of(
                    "namespacePercent", 0.0,
                    "availableProcessors", Runtime.getRuntime().availableProcessors()
            ));
            metricsBody.put("throughput", Map.of(
                    "messagesPublishedPerSec", 0.0,
                    "messagesReceivedPerSec", 0.0,
                    "errorsPerSec", 0.0,
                    "totalPublished", 0L,
                    "totalReceived", 0L,
                    "totalErrors", 0L,
                    "cpuTimeNanos", 0L
            ));
        }
        metricsBody.put("memory", Map.of(
                "heapUsed", mem.getHeapMemoryUsage().getUsed(),
                "heapMax", mem.getHeapMemoryUsage().getMax(),
                "heapCommitted", mem.getHeapMemoryUsage().getCommitted(),
                "nonHeapUsed", mem.getNonHeapMemoryUsage().getUsed(),
                "nonHeapCommitted", mem.getNonHeapMemoryUsage().getCommitted()
        ));

        Map<String, Object> body = new LinkedHashMap<>();
        body.put("namespace", namespace);
        body.put("systemCount", !systems.isEmpty() ? systems.size() : sysRecs.size());
        body.put("nodeCount", totalNodes);
        body.put("healthyNodes", healthy);
        body.put("unhealthyNodes", unhealthy);
        body.put("metrics", metricsBody);
        body.put("systems", systemList);
        return Response.ok(body).build();
    }
}
