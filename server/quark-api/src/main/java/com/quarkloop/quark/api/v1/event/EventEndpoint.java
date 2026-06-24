package com.quarkloop.quark.api.v1.event;

import com.quarkloop.quark.app.query.EventService;
import jakarta.enterprise.context.ApplicationScoped;
import jakarta.inject.Inject;
import jakarta.ws.rs.*;
import jakarta.ws.rs.core.MediaType;
import jakarta.ws.rs.core.Response;

import java.time.Instant;

/**
 * Event operations, scoped to a namespace.
 *
 * <p>Namespace is a path parameter.
 */
@Path("/api/v1/namespaces/{namespace}/events")
@ApplicationScoped
@Produces(MediaType.APPLICATION_JSON)
public class EventEndpoint {

    private final EventService eventService;

    @Inject
    public EventEndpoint(EventService eventService) {
        this.eventService = eventService;
    }

    @GET
    public Response list(@PathParam("namespace") String namespace,
                         @QueryParam("system") String system,
                         @QueryParam("node") String node,
                         @QueryParam("kinds") String kinds,
                         @QueryParam("since") String since,
                         @QueryParam("until") String until,
                         @QueryParam("limit") Integer limit) {
        try {
            return Response.ok(eventService.query(namespace, system, node, kinds,
                    parseInstant(since), parseInstant(until),
                    limit == null ? 100 : limit, false)).build();
        } catch (IllegalArgumentException e) {
            return Response.status(Response.Status.BAD_REQUEST)
                    .entity(java.util.Map.of("message", e.getMessage())).build();
        }
    }

    @GET
    @Path("/count")
    public Response count(@PathParam("namespace") String namespace,
                          @QueryParam("system") String system,
                          @QueryParam("node") String node,
                          @QueryParam("kinds") String kinds,
                          @QueryParam("since") String since,
                          @QueryParam("until") String until) {
        try {
            return Response.ok(eventService.count(namespace, system, node, kinds,
                    parseInstant(since), parseInstant(until), false)).build();
        } catch (IllegalArgumentException e) {
            return Response.status(Response.Status.BAD_REQUEST)
                    .entity(java.util.Map.of("message", e.getMessage())).build();
        }
    }

    private Instant parseInstant(String s) {
        if (s == null || s.isBlank()) return null;
        try { return Instant.parse(s); }
        catch (Exception e) { throw new IllegalArgumentException("Invalid ISO-8601 timestamp: " + s); }
    }
}
