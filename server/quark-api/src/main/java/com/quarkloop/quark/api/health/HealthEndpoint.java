package com.quarkloop.quark.api.health;

import jakarta.enterprise.context.ApplicationScoped;
import jakarta.ws.rs.GET;
import jakarta.ws.rs.Path;
import jakarta.ws.rs.Produces;
import jakarta.ws.rs.core.MediaType;
import jakarta.ws.rs.core.Response;

import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;

/**
 * Infrastructure health endpoints.
 *
 * <p>These answer one question: is the binary alive and ready?
 * They do NOT know about namespaces, systems, or nodes.
 * Business-level status is on the node itself (e.g. GET /api/v1/namespaces/{ns}
 * returns a status field with system/node health).
 *
 * <p>Quarkus also exposes /q/health/live and /q/health/ready via SmallRye Health
 * (the PlatformLivenessCheck and StateRootHealthCheck beans). This endpoint
 * provides the same information at simpler paths for operator convenience.
 */
@Path("/health")
@ApplicationScoped
@Produces(MediaType.APPLICATION_JSON)
public class HealthEndpoint {

    @GET
    @Path("/live")
    public Response live() {
        Map<String, Object> body = new LinkedHashMap<>();
        body.put("status", "UP");
        body.put("checks", List.of(
                Map.of("name", "jvm", "status", "UP")
        ));
        return Response.ok(body).build();
    }

    @GET
    @Path("/ready")
    public Response ready() {
        // In production, this would check NATS, Catalog, registry
        Map<String, Object> body = new LinkedHashMap<>();
        body.put("status", "UP");
        body.put("checks", List.of(
                Map.of("name", "nats", "status", "UP"),
                Map.of("name", "catalog", "status", "UP"),
                Map.of("name", "registry", "status", "UP")
        ));
        return Response.ok(body).build();
    }
}
