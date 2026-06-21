package com.quarkloop.quark.api.event;

import com.quarkloop.quark.app.query.EventService;
import com.quarkloop.quark.core.domain.event.NodeEvent;
import jakarta.enterprise.context.ApplicationScoped;
import jakarta.inject.Inject;
import jakarta.ws.rs.BadRequestException;
import jakarta.ws.rs.Consumes;
import jakarta.ws.rs.GET;
import jakarta.ws.rs.Path;
import jakarta.ws.rs.Produces;
import jakarta.ws.rs.QueryParam;
import jakarta.ws.rs.core.MediaType;
import jakarta.ws.rs.core.Response;

import java.time.Instant;
import java.util.List;

/**
 * REST resource for event queries.
 *
 * <p>Multi-tenancy: {@code namespace} is required unless {@code all=true}
 * (admin mode).
 */
@Path("/events")
@ApplicationScoped
@Produces(MediaType.APPLICATION_JSON)
@Consumes(MediaType.APPLICATION_JSON)
public class EventResource {

    private final EventService eventService;

    @Inject
    public EventResource(EventService eventService) {
        this.eventService = eventService;
    }

    @GET
    public Response list(@QueryParam("namespace") String namespace,
                         @QueryParam("system") String system,
                         @QueryParam("node") String node,
                         @QueryParam("kinds") String kinds,
                         @QueryParam("since") String since,
                         @QueryParam("until") String until,
                         @QueryParam("limit") Integer limit,
                         @QueryParam("all") Boolean all) {
        try {
            List<NodeEvent> events = eventService.query(
                    namespace, system, node, kinds,
                    parseInstant(since), parseInstant(until),
                    limit == null ? 100 : limit,
                    Boolean.TRUE.equals(all));
            return Response.ok(events).build();
        } catch (IllegalArgumentException e) {
            return Response.status(Response.Status.BAD_REQUEST)
                    .entity(java.util.Map.of("message", e.getMessage()))
                    .build();
        }
    }

    @GET
    @Path("/count")
    public Response count(@QueryParam("namespace") String namespace,
                          @QueryParam("system") String system,
                          @QueryParam("node") String node,
                          @QueryParam("kinds") String kinds,
                          @QueryParam("since") String since,
                          @QueryParam("until") String until,
                          @QueryParam("all") Boolean all) {
        try {
            long count = eventService.count(
                    namespace, system, node, kinds,
                    parseInstant(since), parseInstant(until),
                    Boolean.TRUE.equals(all));
            return Response.ok(count).build();
        } catch (IllegalArgumentException e) {
            return Response.status(Response.Status.BAD_REQUEST)
                    .entity(java.util.Map.of("message", e.getMessage()))
                    .build();
        }
    }

    private Instant parseInstant(String s) {
        if (s == null || s.isBlank()) return null;
        try {
            return Instant.parse(s);
        } catch (Exception e) {
            throw new BadRequestException("Invalid ISO-8601 timestamp: " + s);
        }
    }
}
