package com.quarkloop.quark.api.v1.namespace;

import com.quarkloop.quark.app.metrics.NamespaceMetricsCollector;
import com.quarkloop.quark.core.engine.runtime.RuntimeContext;
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
import java.lang.management.OperatingSystemMXBean;
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

    @Inject
    public NamespaceEndpoint(RuntimeContext runtimeContext,
                             NamespaceMetricsCollector metricsCollector) {
        this.runtimeContext = runtimeContext;
        this.metricsCollector = metricsCollector;
    }

    @GET
    public Response list() {
        List<Map<String, Object>> namespaces = runtimeContext.getAllSystems().stream()
                .map(rs -> rs.namespace().value())
                .distinct()
                .sorted()
                .map(ns -> {
                    var systems = runtimeContext.getSystemsByNamespace(ns);
                    int totalNodes = systems.stream().mapToInt(s -> s.nodes().size()).sum();
                    long healthy = systems.stream().flatMap(s -> s.nodes().stream())
                            .filter(n -> n.health().name().equals("HEALTHY")).count();
                    long unhealthy = systems.stream().flatMap(s -> s.nodes().stream())
                            .filter(n -> n.health().name().equals("UNHEALTHY")).count();
                    Map<String, Object> entry = new LinkedHashMap<>();
                    entry.put("namespace", ns);
                    entry.put("systemCount", systems.size());
                    entry.put("nodeCount", totalNodes);
                    entry.put("healthyNodes", healthy);
                    entry.put("unhealthyNodes", unhealthy);
                    return entry;
                })
                .collect(Collectors.toList());
        return Response.ok(namespaces).build();
    }

    @GET
    @Path("/{ns}")
    public Response get(@PathParam("ns") String namespace) {
        var systems = runtimeContext.getSystemsByNamespace(namespace);
        if (systems.isEmpty()) {
            return Response.status(Response.Status.NOT_FOUND).build();
        }
        int totalNodes = systems.stream().mapToInt(s -> s.nodes().size()).sum();
        long healthy = systems.stream().flatMap(s -> s.nodes().stream())
                .filter(n -> n.health().name().equals("HEALTHY")).count();
        long unhealthy = systems.stream().flatMap(s -> s.nodes().stream())
                .filter(n -> n.health().name().equals("UNHEALTHY")).count();

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
        body.put("systemCount", systems.size());
        body.put("nodeCount", totalNodes);
        body.put("healthyNodes", healthy);
        body.put("unhealthyNodes", unhealthy);
        body.put("metrics", metricsBody);
        body.put("systems", systems.stream().map(s -> Map.of(
                "name", s.name(),
                "nodeCount", s.nodes().size(),
                "state", s.nodes().stream().anyMatch(n -> n.state().name().equals("ERROR")) ? "ERROR" : "ACTIVE",
                "health", s.overallHealth().name()
        )).toList());
        return Response.ok(body).build();
    }
}
