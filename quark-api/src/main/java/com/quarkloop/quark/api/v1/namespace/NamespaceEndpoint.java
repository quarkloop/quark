package com.quarkloop.quark.api.v1.namespace;

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

@Path("/api/v1/namespaces")
@ApplicationScoped
@Produces(MediaType.APPLICATION_JSON)
public class NamespaceEndpoint {

    private final RuntimeContext runtimeContext;

    @Inject
    public NamespaceEndpoint(RuntimeContext runtimeContext) {
        this.runtimeContext = runtimeContext;
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

        OperatingSystemMXBean os = ManagementFactory.getOperatingSystemMXBean();
        MemoryMXBean mem = ManagementFactory.getMemoryMXBean();
        double cpuLoad = 0;
        if (os instanceof com.sun.management.OperatingSystemMXBean sunOs) {
            cpuLoad = sunOs.getCpuLoad();
            if (cpuLoad < 0) cpuLoad = 0;
        }

        Map<String, Object> body = new LinkedHashMap<>();
        body.put("namespace", namespace);
        body.put("systemCount", systems.size());
        body.put("nodeCount", totalNodes);
        body.put("healthyNodes", healthy);
        body.put("unhealthyNodes", unhealthy);
        body.put("metrics", Map.of(
                "cpu", Map.of(
                        "systemLoad", cpuLoad,
                        "availableProcessors", os.getAvailableProcessors()
                ),
                "memory", Map.of(
                        "heapUsed", mem.getHeapMemoryUsage().getUsed(),
                        "heapMax", mem.getHeapMemoryUsage().getMax(),
                        "heapCommitted", mem.getHeapMemoryUsage().getCommitted(),
                        "nonHeapUsed", mem.getNonHeapMemoryUsage().getUsed(),
                        "nonHeapCommitted", mem.getNonHeapMemoryUsage().getCommitted()
                )
        ));
        body.put("systems", systems.stream().map(s -> Map.of(
                "name", s.name(),
                "nodeCount", s.nodes().size(),
                "state", s.nodes().stream().anyMatch(n -> n.state().name().equals("ERROR")) ? "ERROR" : "ACTIVE",
                "health", s.overallHealth().name()
        )).toList());
        return Response.ok(body).build();
    }
}
