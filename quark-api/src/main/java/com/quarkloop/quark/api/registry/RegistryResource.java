package com.quarkloop.quark.api.registry;

import com.quarkloop.quark.api.dto.ResponseHelpers;
import com.quarkloop.quark.app.query.RegistryService;
import com.quarkloop.quark.core.registry.NodeDescriptor;
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

import java.util.List;

/**
 * REST resource for browsing the node registry.
 *
 * <p>Endpoints:
 * <ul>
 *   <li>{@code GET /registry} — list all (or filter by {@code category} /
 *       free-text {@code q})</li>
 *   <li>{@code GET /registry/{uri}} — lookup by URI. The URI is path
 *       param {@code .+} so it can contain slashes (e.g. {@code source/timer:v1}).</li>
 * </ul>
 */
@Path("/registry")
@ApplicationScoped
@Produces(MediaType.APPLICATION_JSON)
@Consumes(MediaType.APPLICATION_JSON)
public class RegistryResource {

    private final RegistryService registryService;

    @Inject
    public RegistryResource(RegistryService registryService) {
        this.registryService = registryService;
    }

    @GET
    public Response list(@QueryParam("category") String category,
                         @QueryParam("q") String query) {
        List<NodeDescriptor> entries = registryService.list(category, query);
        return Response.ok(entries).build();
    }

    /**
     * Path param {@code .+} so the URI can contain slashes.
     */
    @GET
    @Path("/{uri: .+}")
    public Response lookup(@PathParam("uri") String uri) {
        return ResponseHelpers.okOr404(registryService.lookup(uri));
    }
}
