package com.quarkloop.quark.api.v1.node;

import com.quarkloop.quark.api.dto.ResponseHelpers;
import com.quarkloop.quark.app.lifecycle.LifecycleService;
import com.quarkloop.quark.app.query.QueryService;
import jakarta.enterprise.context.ApplicationScoped;
import jakarta.inject.Inject;
import jakarta.ws.rs.*;
import jakarta.ws.rs.core.MediaType;
import jakarta.ws.rs.core.Response;

import java.util.NoSuchElementException;

/**
 * Node operations, scoped to namespace + system.
 *
 * <p>Namespace and system are path parameters.
 */
@Path("/api/v1/namespaces/{namespace}/systems/{system}/nodes")
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

    @GET
    public Response list(@PathParam("namespace") String namespace,
                         @PathParam("system") String system) {
        return Response.ok(queryService.listNodes(namespace, system)).build();
    }

    @GET
    @Path("/{name}")
    public Response get(@PathParam("namespace") String namespace,
                        @PathParam("system") String system,
                        @PathParam("name") String name) {
        return ResponseHelpers.okOr404(queryService.getNode(namespace, system, name));
    }

    @POST @Path("/{name}/pause")
    public Response pause(@PathParam("namespace") String ns, @PathParam("system") String sys,
                          @PathParam("name") String name) { return transition(ns, sys, name, "pause"); }

    @POST @Path("/{name}/resume")
    public Response resume(@PathParam("namespace") String ns, @PathParam("system") String sys,
                           @PathParam("name") String name) { return transition(ns, sys, name, "resume"); }

    @POST @Path("/{name}/drain")
    public Response drain(@PathParam("namespace") String ns, @PathParam("system") String sys,
                          @PathParam("name") String name) { return transition(ns, sys, name, "drain"); }

    @POST @Path("/{name}/archive")
    public Response archive(@PathParam("namespace") String ns, @PathParam("system") String sys,
                            @PathParam("name") String name) { return transition(ns, sys, name, "archive"); }

    @POST @Path("/{name}/recover")
    public Response recover(@PathParam("namespace") String ns, @PathParam("system") String sys,
                            @PathParam("name") String name) { return transition(ns, sys, name, "recover"); }

    private Response transition(String ns, String sys, String name, String op) {
        try {
            switch (op) {
                case "pause" -> lifecycleService.pause(ns, sys, name);
                case "resume" -> lifecycleService.resume(ns, sys, name);
                case "drain" -> lifecycleService.drain(ns, sys, name);
                case "archive" -> lifecycleService.archive(ns, sys, name);
                case "recover" -> lifecycleService.recover(ns, sys, name);
            }
            return Response.noContent().build();
        } catch (NoSuchElementException e) {
            return Response.status(Response.Status.NOT_FOUND).build();
        } catch (IllegalStateException e) {
            return Response.status(Response.Status.CONFLICT)
                    .entity(java.util.Map.of("message", e.getMessage())).build();
        }
    }
}
