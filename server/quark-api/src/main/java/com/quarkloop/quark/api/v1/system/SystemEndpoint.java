package com.quarkloop.quark.api.v1.system;

import com.quarkloop.quark.api.dto.SystemDtos;
import com.quarkloop.quark.api.dto.ResponseHelpers;
import com.quarkloop.quark.app.deploy.ApplyService;
import com.quarkloop.quark.app.deploy.DeployService;
import com.quarkloop.quark.app.query.QueryService;
import com.quarkloop.quark.app.query.SourceService;
import com.quarkloop.quark.core.engine.lifecycle.DeploymentException;
import com.quarkloop.quark.core.engine.lifecycle.RuntimeSystem;
import jakarta.enterprise.context.ApplicationScoped;
import jakarta.inject.Inject;
import jakarta.ws.rs.*;
import jakarta.ws.rs.core.MediaType;
import jakarta.ws.rs.core.Response;

import java.util.List;

/**
 * System operations, scoped to a namespace.
 *
 * <p>Namespace is a path parameter.
 * POST creates (deploy), PUT applies (declarative reconcile), DELETE removes.
 */
@Path("/api/v1/namespaces/{namespace}/systems")
@ApplicationScoped
@Produces(MediaType.APPLICATION_JSON)
@Consumes(MediaType.APPLICATION_JSON)
public class SystemEndpoint {

    private final DeployService deployService;
    private final ApplyService applyService;
    private final QueryService queryService;
    private final SourceService sourceService;

    @Inject
    public SystemEndpoint(DeployService deployService, ApplyService applyService,
                          QueryService queryService, SourceService sourceService) {
        this.deployService = deployService;
        this.applyService = applyService;
        this.queryService = queryService;
        this.sourceService = sourceService;
    }

    /**
     * List systems in a namespace.
     * GET /api/v1/namespaces/{ns}/systems
     */
    @GET
    public Response list(@PathParam("namespace") String namespace) {
        return Response.ok(queryService.listSystems(namespace)).build();
    }

    /**
     * Deploy a system (create).
     * POST /api/v1/namespaces/{ns}/systems
     */
    @POST
    public Response deploy(@PathParam("namespace") String namespace,
                           SystemDtos.DeploySystemRequest request) {
        try {
            RuntimeSystem runtime = deployService.deploy(request.source(), namespace);
            var detail = queryService.getSystem(namespace, runtime.name())
                    .orElseThrow(() -> new IllegalStateException("System just deployed but not found"));
            List<String> nodeNames = detail.nodes().stream()
                    .map(QueryService.NodeSummary::name).toList();
            return Response.status(Response.Status.CREATED).entity(
                    new SystemDtos.DeploySystemResponse(detail.name(), detail.namespace(),
                            detail.nodes().size(), detail.state(), detail.health(), nodeNames)).build();
        } catch (DeploymentException e) {
            return Response.status(Response.Status.BAD_REQUEST).entity(
                    new SystemDtos.DeploySystemFailure(e.getMessage(),
                            List.of(new SystemDtos.ValidationError("system", e.getMessage(), "ERROR")))).build();
        }
    }

    /**
     * Get system details.
     * GET /api/v1/namespaces/{ns}/systems/{name}
     */
    @GET
    @Path("/{name}")
    public Response get(@PathParam("namespace") String namespace,
                        @PathParam("name") String name) {
        return ResponseHelpers.okOr404(queryService.getSystem(namespace, name));
    }

    /**
     * Apply (declarative reconcile).
     * PUT /api/v1/namespaces/{ns}/systems/{name}
     */
    @PUT
    @Path("/{name}")
    public Response apply(@PathParam("namespace") String namespace,
                          @PathParam("name") String name,
                          SystemDtos.DeploySystemRequest request) {
        try {
            ApplyService.ApplyResult result = applyService.apply(request.source(), namespace);
            if (result.created()) return Response.status(Response.Status.CREATED).entity(result).build();
            return Response.ok(result).build();
        } catch (DeploymentException e) {
            return Response.status(Response.Status.BAD_REQUEST).entity(
                    new SystemDtos.DeploySystemFailure(e.getMessage(),
                            List.of(new SystemDtos.ValidationError("system", e.getMessage(), "ERROR")))).build();
        }
    }

    /**
     * Undeploy a system (delete).
     * DELETE /api/v1/namespaces/{ns}/systems/{name}
     */
    @DELETE
    @Path("/{name}")
    public Response delete(@PathParam("namespace") String namespace,
                           @PathParam("name") String name) {
        try { deployService.undeploy(namespace, name); }
        catch (java.util.NoSuchElementException ignored) {}
        return Response.noContent().build();
    }

    /**
     * Get the original .quark.ts source.
     * GET /api/v1/namespaces/{ns}/systems/{name}/source
     */
    @GET
    @Path("/{name}/source")
    @Produces(MediaType.TEXT_PLAIN)
    public Response getSource(@PathParam("namespace") String namespace,
                              @PathParam("name") String name) {
        try {
            return ResponseHelpers.okOr404(sourceService.getSource(namespace, name));
        } catch (IllegalStateException e) {
            return Response.serverError().build();
        }
    }
}
