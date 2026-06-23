package com.quarkloop.quark.api.v1.registry;

import com.quarkloop.quark.api.dto.ResponseHelpers;
import com.quarkloop.quark.app.query.RegistryService;
import jakarta.enterprise.context.ApplicationScoped;
import jakarta.inject.Inject;
import jakarta.ws.rs.*;
import jakarta.ws.rs.core.MediaType;
import jakarta.ws.rs.core.Response;

/**
 * Registry operations (platform-level, not namespace-scoped).
 *
 * <p>Lists available node implementations (providers).
 */
@Path("/api/v1/registry")
@ApplicationScoped
@Produces(MediaType.APPLICATION_JSON)
public class RegistryEndpoint {

    private final RegistryService registryService;

    @Inject
    public RegistryEndpoint(RegistryService registryService) {
        this.registryService = registryService;
    }

    @GET
    public Response list(@QueryParam("category") String category,
                         @QueryParam("q") String query) {
        return Response.ok(registryService.list(category, query)).build();
    }

    @GET
    @Path("/{uri: .+}")
    public Response get(@PathParam("uri") String uri) {
        return ResponseHelpers.okOr404(registryService.lookup(uri));
    }
}
