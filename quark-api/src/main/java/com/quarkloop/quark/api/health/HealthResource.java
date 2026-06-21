package com.quarkloop.quark.api.health;

import com.quarkloop.quark.api.dto.ResponseHelpers;
import com.quarkloop.quark.app.health.HealthService;
import jakarta.enterprise.context.ApplicationScoped;
import jakarta.inject.Inject;
import jakarta.ws.rs.Consumes;
import jakarta.ws.rs.GET;
import jakarta.ws.rs.Path;
import jakarta.ws.rs.PathParam;
import jakarta.ws.rs.Produces;
import jakarta.ws.rs.QueryParam;
import jakarta.ws.rs.core.MediaType;
import jakarta.ws.rs.core.Response;

/**
 * REST resource for application-level health (NOT the SmallRye Health
 * Kubernetes probe, which lives at {@code /q/health}).
 */
@Path("/health")
@ApplicationScoped
@Produces(MediaType.APPLICATION_JSON)
@Consumes(MediaType.APPLICATION_JSON)
public class HealthResource {

    private final HealthService healthService;

    @Inject
    public HealthResource(HealthService healthService) {
        this.healthService = healthService;
    }

    @GET
    public Response platform() {
        return Response.ok(healthService.platformHealth()).build();
    }

    @GET
    @Path("/namespaces/{ns}")
    public Response namespace(@PathParam("ns") String namespace) {
        return Response.ok(healthService.namespaceHealth(namespace)).build();
    }

    @GET
    @Path("/systems/{name}")
    public Response system(@PathParam("name") String name,
                           @QueryParam("namespace") String namespace) {
        if (namespace == null || namespace.isBlank()) {
            return Response.status(Response.Status.BAD_REQUEST).build();
        }
        return ResponseHelpers.okOr404(healthService.systemHealth(namespace, name));
    }

    @GET
    @Path("/nodes/{name}")
    public Response node(@PathParam("name") String name,
                         @QueryParam("namespace") String namespace,
                         @QueryParam("system") String system) {
        if (namespace == null || namespace.isBlank() || system == null || system.isBlank()) {
            return Response.status(Response.Status.BAD_REQUEST).build();
        }
        return ResponseHelpers.okOr404(healthService.nodeHealth(namespace, system, name));
    }
}
