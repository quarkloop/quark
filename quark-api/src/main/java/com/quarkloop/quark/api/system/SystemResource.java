package com.quarkloop.quark.api.system;

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
import jakarta.ws.rs.Consumes;
import jakarta.ws.rs.DELETE;
import jakarta.ws.rs.GET;
import jakarta.ws.rs.POST;
import jakarta.ws.rs.PUT;
import jakarta.ws.rs.Path;
import jakarta.ws.rs.PathParam;
import jakarta.ws.rs.Produces;
import jakarta.ws.rs.QueryParam;
import jakarta.ws.rs.core.MediaType;
import jakarta.ws.rs.core.Response;

import java.util.List;
import java.util.NoSuchElementException;

/**
 * REST resource for system deployment and queries.
 *
 * <p>All endpoints (except {@code POST /systems/deploy}) require a
 * {@code namespace} query parameter for multi-tenant isolation.
 */
@Path("/systems")
@ApplicationScoped
@Produces(MediaType.APPLICATION_JSON)
@Consumes(MediaType.APPLICATION_JSON)
public class SystemResource {

    private final DeployService deployService;
    private final ApplyService applyService;
    private final QueryService queryService;
    private final SourceService sourceService;

    @Inject
    public SystemResource(DeployService deployService,
                          ApplyService applyService,
                          QueryService queryService,
                          SourceService sourceService) {
        this.deployService = deployService;
        this.applyService = applyService;
        this.queryService = queryService;
        this.sourceService = sourceService;
    }

    /**
     * Deploy a system from a {@code .quark.ts} source string.
     *
     * @param request {@code {source: "<ts>", namespace: "<ns>"}}
     * @return 201 with the deployed system summary, or 400 on parse failure.
     */
    @POST
    @Path("/deploy")
    public Response deploy(SystemDtos.DeploySystemRequest request) {
        try {
            RuntimeSystem runtime = deployService.deploy(request.source(), request.namespace());
            QueryService.SystemDetail detail = queryService.getSystem(
                    runtime.namespace().value(), runtime.name())
                    .orElseThrow(() -> new IllegalStateException(
                            "System just deployed but not found in registry"));
            List<String> nodeNames = detail.nodes().stream()
                    .map(QueryService.NodeSummary::name)
                    .toList();
            SystemDtos.DeploySystemResponse resp = new SystemDtos.DeploySystemResponse(
                    detail.name(),
                    detail.namespace(),
                    detail.nodes().size(),
                    detail.state(),
                    detail.health(),
                    nodeNames
            );
            return Response.status(Response.Status.CREATED).entity(resp).build();
        } catch (DeploymentException e) {
            // Parse failure or unknown URI — return structured 400
            SystemDtos.DeploySystemFailure failure = new SystemDtos.DeploySystemFailure(
                    e.getMessage(),
                    List.of(new SystemDtos.ValidationError("system", e.getMessage(), "ERROR"))
            );
            return Response.status(Response.Status.BAD_REQUEST).entity(failure).build();
        } catch (IllegalArgumentException e) {
            SystemDtos.DeploySystemFailure failure = new SystemDtos.DeploySystemFailure(
                    e.getMessage(),
                    List.of(new SystemDtos.ValidationError("request", e.getMessage(), "ERROR"))
            );
            return Response.status(Response.Status.BAD_REQUEST).entity(failure).build();
        }
    }

    /**
     * Declarative apply: reconcile desired state with current state.
     */
    @PUT
    @Path("/{name}")
    public Response apply(@PathParam("name") String name, SystemDtos.DeploySystemRequest request) {
        try {
            ApplyService.ApplyResult result = applyService.apply(request.source(), request.namespace());
            if (result.created()) return Response.status(Response.Status.CREATED).entity(result).build();
            return Response.ok(result).build();
        } catch (DeploymentException e) {
            return Response.status(Response.Status.BAD_REQUEST)
                    .entity(new SystemDtos.DeploySystemFailure(e.getMessage(),
                            List.of(new SystemDtos.ValidationError("system", e.getMessage(), "ERROR")))).build();
        }
    }

    /**
     * List systems in a namespace. {@code namespace} is required.
     */
    @GET
    public Response list(@QueryParam("namespace") String namespace) {
        if (namespace == null || namespace.isBlank()) {
            return Response.status(Response.Status.BAD_REQUEST)
                    .entity(new SystemDtos.DeploySystemFailure(
                            "namespace query parameter is required",
                            List.of()))
                    .build();
        }
        return Response.ok(queryService.listSystems(namespace)).build();
    }

    /**
     * Get a single system with per-node state.
     */
    @GET
    @Path("/{name}")
    public Response get(@PathParam("name") String name,
                        @QueryParam("namespace") String namespace) {
        if (namespace == null || namespace.isBlank()) {
            return Response.status(Response.Status.BAD_REQUEST)
                    .entity(new SystemDtos.DeploySystemFailure(
                            "namespace query parameter is required",
                            List.of()))
                    .build();
        }
        return ResponseHelpers.okOr404(queryService.getSystem(namespace, name));
    }

    /**
     * Get the original {@code .quark.ts} source for a system.
     * Returns {@code text/plain}.
     */
    @GET
    @Path("/{name}/source")
    @Produces(MediaType.TEXT_PLAIN)
    public Response getSource(@PathParam("name") String name,
                              @QueryParam("namespace") String namespace) {
        if (namespace == null || namespace.isBlank()) {
            return Response.status(Response.Status.BAD_REQUEST).build();
        }
        try {
            return ResponseHelpers.okOr404(sourceService.getSource(namespace, name));
        } catch (IllegalStateException e) {
            return Response.serverError().build();
        }
    }

    /**
     * Undeploy a system. Idempotent — returns 204 even if the system
     * doesn't exist.
     */
    @DELETE
    @Path("/{name}")
    public Response delete(@PathParam("name") String name,
                           @QueryParam("namespace") String namespace) {
        if (namespace == null || namespace.isBlank()) {
            return Response.status(Response.Status.BAD_REQUEST).build();
        }
        try {
            deployService.undeploy(namespace, name);
        } catch (NoSuchElementException ignored) {
            // Idempotent
        }
        return Response.noContent().build();
    }
}
