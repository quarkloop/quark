package com.quarkloop.quark.api.node;

import com.quarkloop.quark.api.dto.ResponseHelpers;
import com.quarkloop.quark.app.lifecycle.LifecycleService;
import com.quarkloop.quark.app.query.QueryService;
import jakarta.enterprise.context.ApplicationScoped;
import jakarta.inject.Inject;
import jakarta.ws.rs.BadRequestException;
import jakarta.ws.rs.Consumes;
import jakarta.ws.rs.GET;
import jakarta.ws.rs.POST;
import jakarta.ws.rs.Path;
import jakarta.ws.rs.PathParam;
import jakarta.ws.rs.Produces;
import jakarta.ws.rs.QueryParam;
import jakarta.ws.rs.core.MediaType;
import jakarta.ws.rs.core.Response;

import java.util.List;
import java.util.NoSuchElementException;

/**
 * REST resource for node queries and lifecycle control.
 */
@Path("/nodes")
@ApplicationScoped
@Produces(MediaType.APPLICATION_JSON)
@Consumes(MediaType.APPLICATION_JSON)
public class NodeResource {

    private final QueryService queryService;
    private final LifecycleService lifecycleService;

    @Inject
    public NodeResource(QueryService queryService, LifecycleService lifecycleService) {
        this.queryService = queryService;
        this.lifecycleService = lifecycleService;
    }

    /**
     * List nodes. {@code namespace} is required; {@code system} is optional.
     */
    @GET
    public Response list(@QueryParam("namespace") String namespace,
                         @QueryParam("system") String system) {
        requireNamespace(namespace);
        List<QueryService.NodeSummary> nodes = (system == null || system.isBlank())
                ? queryService.listAllNodes(namespace)
                : queryService.listNodes(namespace, system);
        return Response.ok(nodes).build();
    }

    @GET
    @Path("/{name}")
    public Response get(@PathParam("name") String name,
                        @QueryParam("namespace") String namespace,
                        @QueryParam("system") String system) {
        requireNamespace(namespace);
        requireSystem(system);
        return ResponseHelpers.okOr404(queryService.getNode(namespace, system, name));
    }

    @POST
    @Path("/{name}/pause")
    public Response pause(@PathParam("name") String name,
                          @QueryParam("namespace") String namespace,
                          @QueryParam("system") String system) {
        return transition(namespace, system, name, "pause");
    }

    @POST
    @Path("/{name}/resume")
    public Response resume(@PathParam("name") String name,
                           @QueryParam("namespace") String namespace,
                           @QueryParam("system") String system) {
        return transition(namespace, system, name, "resume");
    }

    @POST
    @Path("/{name}/drain")
    public Response drain(@PathParam("name") String name,
                          @QueryParam("namespace") String namespace,
                          @QueryParam("system") String system) {
        return transition(namespace, system, name, "drain");
    }

    @POST
    @Path("/{name}/archive")
    public Response archive(@PathParam("name") String name,
                            @QueryParam("namespace") String namespace,
                            @QueryParam("system") String system) {
        return transition(namespace, system, name, "archive");
    }

    @POST
    @Path("/{name}/recover")
    public Response recover(@PathParam("name") String name,
                            @QueryParam("namespace") String namespace,
                            @QueryParam("system") String system) {
        return transition(namespace, system, name, "recover");
    }

    @POST
    @Path("/{name}/delete")
    public Response delete(@PathParam("name") String name,
                           @QueryParam("namespace") String namespace,
                           @QueryParam("system") String system) {
        return transition(namespace, system, name, "delete");
    }

    private Response transition(String namespace, String system, String name, String op) {
        requireNamespace(namespace);
        requireSystem(system);
        try {
            switch (op) {
                case "pause"   -> lifecycleService.pause(namespace, system, name);
                case "resume"  -> lifecycleService.resume(namespace, system, name);
                case "drain"   -> lifecycleService.drain(namespace, system, name);
                case "archive" -> lifecycleService.archive(namespace, system, name);
                case "recover" -> lifecycleService.recover(namespace, system, name);
                case "delete"  -> lifecycleService.delete(namespace, system, name);
                default -> throw new IllegalArgumentException("Unknown lifecycle op: " + op);
            }
            return Response.noContent().build();
        } catch (NoSuchElementException e) {
            return Response.status(Response.Status.NOT_FOUND).build();
        } catch (IllegalStateException e) {
            return Response.status(Response.Status.CONFLICT)
                    .entity(java.util.Map.of("message", e.getMessage()))
                    .build();
        }
    }

    private void requireNamespace(String namespace) {
        if (namespace == null || namespace.isBlank()) {
            throw new BadRequestException("namespace query parameter is required");
        }
    }

    private void requireSystem(String system) {
        if (system == null || system.isBlank()) {
            throw new BadRequestException("system query parameter is required");
        }
    }
}
