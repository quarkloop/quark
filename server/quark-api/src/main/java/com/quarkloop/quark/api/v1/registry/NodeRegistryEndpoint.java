package com.quarkloop.quark.api.v1.registry;

import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.datatype.jsr310.JavaTimeModule;
import com.quarkloop.quark.runtime.engine.nats.NatsConnectionManager;
import io.nats.client.Connection;
import io.nats.client.Message;
import jakarta.enterprise.context.ApplicationScoped;
import jakarta.inject.Inject;
import jakarta.ws.rs.*;
import jakarta.ws.rs.core.MediaType;
import jakarta.ws.rs.core.Response;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import java.nio.charset.StandardCharsets;
import java.time.Duration;
import java.util.Base64;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;

/**
 * REST API for the node package registry.
 *
 * <p>Proxies requests to the Catalog service via NATS. These endpoints
 * correspond to the {@code registry.node.*} NATS subjects.
 *
 * <p>Used by the CLI's {@code quarkctl node} command group:
 * <ul>
 *   <li>{@code GET /api/v1/registry/nodes} — list all nodes</li>
 *   <li>{@code GET /api/v1/registry/nodes/{uri}} — get node info</li>
 *   <li>{@code GET /api/v1/registry/nodes/{uri}/pull} — download node package</li>
 *   <li>{@code GET /api/v1/registry/nodes/search?keyword=...} — search nodes</li>
 *   <li>{@code POST /api/v1/registry/nodes} — push a node</li>
 * </ul>
 */
@Path("/api/v1/registry/nodes")
@ApplicationScoped
@Produces(MediaType.APPLICATION_JSON)
@Consumes(MediaType.APPLICATION_JSON)
public class NodeRegistryEndpoint {

    private static final Logger log = LoggerFactory.getLogger(NodeRegistryEndpoint.class);
    private static final Duration NATS_TIMEOUT = Duration.ofSeconds(5);

    private final ObjectMapper mapper = new ObjectMapper();
    { mapper.registerModule(new JavaTimeModule()); }

    private final NatsConnectionManager natsConnectionManager;

    @Inject
    public NodeRegistryEndpoint(NatsConnectionManager natsConnectionManager) {
        this.natsConnectionManager = natsConnectionManager;
    }

    @GET
    public Response listNodes() {
        try {
            Map<String, Object> req = new LinkedHashMap<>();

            byte[] resp = natsRequest("registry.node.list", req);
            return Response.ok(new String(resp, StandardCharsets.UTF_8)).build();
        } catch (Exception e) {
            return Response.serverError().entity(Map.of("error", e.getMessage())).build();
        }
    }

    @POST
    @Path("/info")
    public Response getNodeInfo(Map<String, String> body) {
        String uri = body != null ? body.get("uri") : null;
        if (uri == null) return Response.status(Response.Status.BAD_REQUEST).build();
        try {
            byte[] resp = natsRequest("registry.node.info", Map.of("uri", uri));
            String json = new String(resp, StandardCharsets.UTF_8);
            if (json.contains("\"error\"")) {
                return Response.status(Response.Status.NOT_FOUND).entity(json).build();
            }
            return Response.ok(json).build();
        } catch (Exception e) {
            return Response.serverError().entity(Map.of("error", e.getMessage())).build();
        }
    }

    @POST
    @Path("/pull")
    public Response pullNode(Map<String, String> body) {
        String uri = body != null ? body.get("uri") : null;
        if (uri == null) return Response.status(Response.Status.BAD_REQUEST).build();
        try {
            byte[] resp = natsRequest("registry.node.pull", Map.of("uri", uri));
            String json = new String(resp, StandardCharsets.UTF_8);
            if (json.contains("\"error\"")) {
                return Response.status(Response.Status.NOT_FOUND).entity(json).build();
            }
            return Response.ok(json).build();
        } catch (Exception e) {
            return Response.serverError().entity(Map.of("error", e.getMessage())).build();
        }
    }

    @GET
    @Path("/search")
    public Response searchNodes(@QueryParam("keyword") String keyword) {
        try {
            byte[] resp = natsRequest("registry.node.search", Map.of("keyword", keyword != null ? keyword : ""));
            return Response.ok(new String(resp, StandardCharsets.UTF_8)).build();
        } catch (Exception e) {
            return Response.serverError().entity(Map.of("error", e.getMessage())).build();
        }
    }

    @POST
    public Response pushNode(Map<String, Object> body) {
        try {
            // Convert content from base64 if needed
            if (body.containsKey("content") && body.get("content") instanceof String) {
                String b64 = (String) body.get("content");
                body.put("content", Base64.getDecoder().decode(b64));
            }
            byte[] resp = natsRequest("registry.node.push", body);
            return Response.ok(new String(resp, StandardCharsets.UTF_8)).build();
        } catch (Exception e) {
            return Response.serverError().entity(Map.of("error", e.getMessage())).build();
        }
    }

    private byte[] natsRequest(String subject, Object payload) throws Exception {
        Connection conn = natsConnectionManager.getConnection();
        byte[] data = mapper.writeValueAsBytes(payload);
        Message reply = conn.request(subject, data, NATS_TIMEOUT);
        if (reply == null) {
            throw new RuntimeException("Catalog did not respond to " + subject + " within " + NATS_TIMEOUT);
        }
        return reply.getData();
    }
}
