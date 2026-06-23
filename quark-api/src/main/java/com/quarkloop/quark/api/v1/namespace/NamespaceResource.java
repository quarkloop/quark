package com.quarkloop.quark.api.v1.namespace;

import com.quarkloop.quark.app.query.QueryService;
import com.quarkloop.quark.core.engine.runtime.RuntimeContext;
import jakarta.enterprise.context.ApplicationScoped;
import jakarta.inject.Inject;
import jakarta.ws.rs.GET;
import jakarta.ws.rs.Path;
import jakarta.ws.rs.PathParam;
import jakarta.ws.rs.Produces;
import jakarta.ws.rs.core.MediaType;
import jakarta.ws.rs.core.Response;

import java.util.List;
import java.util.stream.Collectors;

/**
 * Namespace-scoped operations.
 *
 * <p>Kubernetes-style: namespace is in the path, not the query string.
 * Namespaces are the highest-level container — systems live inside namespaces.
 */
@Path("/api/v1/namespaces")
@ApplicationScoped
@Produces(MediaType.APPLICATION_JSON)
public class NamespaceResource {

    private final RuntimeContext runtimeContext;

    @Inject
    public NamespaceResource(RuntimeContext runtimeContext) {
        this.runtimeContext = runtimeContext;
    }

    /**
     * List all active namespaces.
     * GET /api/v1/namespaces
     */
    @GET
    public Response list() {
        List<String> namespaces = runtimeContext.getAllSystems().stream()
                .map(rs -> rs.namespace().value())
                .distinct()
                .sorted()
                .collect(Collectors.toList());
        return Response.ok(namespaces).build();
    }

    /**
     * Get namespace details.
     * GET /api/v1/namespaces/{ns}
     */
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

        return Response.ok(java.util.Map.of(
                "namespace", namespace,
                "systemCount", systems.size(),
                "nodeCount", totalNodes,
                "healthyNodes", healthy,
                "unhealthyNodes", unhealthy,
                "systems", systems.stream().map(s -> java.util.Map.of(
                        "name", s.name(),
                        "nodeCount", s.nodes().size(),
                        "state", s.nodes().stream().anyMatch(n -> n.state().name().equals("ERROR")) ? "ERROR" : "ACTIVE",
                        "health", s.overallHealth().name()
                )).toList()
        )).build();
    }
}
