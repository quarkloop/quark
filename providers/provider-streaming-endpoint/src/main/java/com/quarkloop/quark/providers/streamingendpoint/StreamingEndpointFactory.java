package com.quarkloop.quark.providers.streamingendpoint;

import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.datatype.jsr310.JavaTimeModule;
import com.quarkloop.quark.core.domain.category.NodeCategory;
import com.quarkloop.quark.core.domain.config.NodeConfig;
import com.quarkloop.quark.core.domain.identity.NodeUri;
import com.quarkloop.quark.core.domain.spi.EndpointProvider;
import com.quarkloop.quark.core.domain.spi.QuarkMessage;
import com.quarkloop.quark.core.domain.spi.QuarkPublisher;
import com.quarkloop.quark.core.registry.NodeDescriptor;
import com.quarkloop.quark.core.registry.NodeImplementationFactory;
import jakarta.enterprise.context.ApplicationScoped;
import org.eclipse.microprofile.config.Config;
import org.eclipse.microprofile.config.ConfigProvider;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import java.io.IOException;
import java.io.OutputStream;
import java.net.InetSocketAddress;
import java.nio.charset.StandardCharsets;
import java.util.HashMap;
import java.util.List;
import java.util.Map;
import java.util.concurrent.ConcurrentHashMap;
import java.util.concurrent.CopyOnWriteArrayList;

/**
 * Endpoint node that runs a small HTTP server and exposes per-node SSE
 * streams. Each connected client receives a {@code data: <json>\n\n} line
 * for every NATS message delivered to the endpoint.
 *
 * <p>URI: {@code endpoint/stream:v1}. The engine injects the runtime
 * identity ({@link SystemRunner#CONFIG_KEY_SYSTEM}, {@code _quark_namespace},
 * {@code _quark_node}) into the config so the provider knows its own
 * subject-tuple.
 *
 * <p>HTTP server configuration (read from MicroProfile Config):
 * <ul>
 *   <li>{@code quark.streaming.host} (default {@code 0.0.0.0})</li>
 *   <li>{@code quark.streaming.port} (default {@code 8081})</li>
 *   <li>{@code quark.streaming.path-prefix} (default {@code stream})</li>
 * </ul>
 *
 * <p>URL pattern: {@code /<prefix>/<namespace>/<system>/<node>}.
 * Multiple endpoints in the same JVM share the same HTTP server — the
 * route is keyed by (namespace, system, node).
 */
@ApplicationScoped
public class StreamingEndpointFactory implements NodeImplementationFactory<EndpointProvider> {

    private static final Logger log = LoggerFactory.getLogger(StreamingEndpointFactory.class);

    /**
     * Engine-injected config keys (mirrors {@code SystemRunner.CONFIG_KEY_*}).
     * Kept as string literals here so provider modules don't need to depend
     * on {@code quark-engine}.
     */
    private static final String CONFIG_KEY_SYSTEM    = "_quark_system";
    private static final String CONFIG_KEY_NAMESPACE = "_quark_namespace";
    private static final String CONFIG_KEY_NODE      = "_quark_node";

    @Override
    public String uriPattern() {
        return "endpoint/stream";
    }

    @Override
    public EndpointProvider create(NodeConfig config) {
        return new StreamingEndpoint(config);
    }

    @Override
    public NodeDescriptor descriptor() {
        return new NodeDescriptor(
                NodeUri.parse("endpoint/stream:v1"),
                NodeCategory.ENDPOINT,
                false,
                "Exposes an HTTP SSE endpoint that streams incoming NATS messages."
        );
    }

    @Override
    public NodeCategory category() {
        return NodeCategory.ENDPOINT;
    }

    /**
     * Shared HTTP server registry — one server per (host, port) pair,
     * shared across all endpoint instances in this JVM.
     */
    private static final Map<String, SharedServer> SHARED_SERVERS = new ConcurrentHashMap<>();

    static final class StreamingEndpoint implements EndpointProvider {

        private static final ObjectMapper MAPPER = new ObjectMapper();
        static {
            MAPPER.registerModule(new JavaTimeModule());
        }

        private final String namespace;
        private final String systemName;
        private final String nodeName;

        private final List<OutputStream> clients = new CopyOnWriteArrayList<>();
        private final Map<OutputStream, Object> clientLocks = new ConcurrentHashMap<>();
        private SharedServer sharedServer;

        StreamingEndpoint(NodeConfig config) {
            this.namespace = config.getString(CONFIG_KEY_NAMESPACE, "default");
            this.systemName = config.getString(CONFIG_KEY_SYSTEM, "system");
            this.nodeName = config.getString(CONFIG_KEY_NODE, "endpoint");
        }

        @Override
        public void start(QuarkPublisher publisher, NodeConfig config) {
            Config mpConfig = ConfigProvider.getConfig();
            String host = mpConfig.getOptionalValue("quark.streaming.host", String.class).orElse("0.0.0.0");
            int port = mpConfig.getOptionalValue("quark.streaming.port", Integer.class).orElse(8081);
            String prefix = mpConfig.getOptionalValue("quark.streaming.path-prefix", String.class).orElse("stream");

            String serverKey = host + ":" + port;
            SharedServer server = SHARED_SERVERS.computeIfAbsent(serverKey, k -> {
                try {
                    SharedServer s = new SharedServer(new InetSocketAddress(host, port));
                    s.start();
                    log.info("Streaming HTTP server listening on {}:{}", host, port);
                    return s;
                } catch (IOException e) {
                    throw new IllegalStateException("Failed to start streaming HTTP server on " + serverKey, e);
                }
            });
            this.sharedServer = server;

            // Register this endpoint's route: /<prefix>/<ns>/<sys>/<node>
            String route = "/" + prefix + "/" + namespace + "/" + systemName + "/" + nodeName;
            server.register(route, this);
            log.info("Streaming endpoint registered: {} -> node {}/{}/{}", route, namespace, systemName, nodeName);
        }

        @Override
        public void onMessage(QuarkMessage message, QuarkPublisher publisher) {
            if (clients.isEmpty()) return;
            String json;
            try {
                Map<String, Object> envelope = new HashMap<>();
                envelope.put("subject", message.subject());
                envelope.put("systemName", message.systemName());
                envelope.put("namespace", message.namespace());
                envelope.put("nodeName", message.nodeName());
                envelope.put("timestamp", message.timestamp().toString());
                envelope.put("payload", message.payload());
                json = MAPPER.writeValueAsString(envelope);
            } catch (Exception e) {
                log.warn("Failed to serialize SSE event", e);
                return;
            }
            byte[] frame = ("data: " + json + "\n\n").getBytes(StandardCharsets.UTF_8);

            for (OutputStream os : clients) {
                Object lock = clientLocks.get(os);
                if (lock == null) continue;
                try {
                    synchronized (lock) {
                        os.write(frame);
                        os.flush();
                    }
                } catch (IOException e) {
                    // Client disconnected
                    clients.remove(os);
                    clientLocks.remove(os);
                    log.debug("SSE client disconnected (node {}/{}/{})", namespace, systemName, nodeName);
                }
            }
        }

        @Override
        public void stop() {
            if (sharedServer != null) {
                sharedServer.unregister(this);
            }
            for (OutputStream os : clients) {
                try { os.close(); } catch (IOException ignored) {}
            }
            clients.clear();
            clientLocks.clear();
            log.info("Streaming endpoint stopped: node {}/{}/{}", namespace, systemName, nodeName);
        }

        void addClient(OutputStream os) {
            clients.add(os);
            clientLocks.put(os, new Object());
            log.debug("SSE client connected (node {}/{}/{}, total={})",
                    namespace, systemName, nodeName, clients.size());
        }

        void removeClient(OutputStream os) {
            clients.remove(os);
            clientLocks.remove(os);
        }

        String namespace() { return namespace; }
        String systemName() { return systemName; }
        String nodeName() { return nodeName; }
    }

    /**
     * Wraps a single {@link com.sun.net.httpserver.HttpServer} so multiple
     * endpoint instances can register routes on it.
     */
    static final class SharedServer {
        private final com.sun.net.httpserver.HttpServer server;
        private final Map<String, StreamingEndpoint> routes = new ConcurrentHashMap<>();

        SharedServer(InetSocketAddress addr) throws IOException {
            this.server = com.sun.net.httpserver.HttpServer.create(addr, 0);
            this.server.setExecutor(java.util.concurrent.Executors.newThreadPerTaskExecutor(
                    Thread.ofVirtual().name("quark-sse-", 0).factory()
            ));
            // Catch-all handler — we inspect the path and dispatch.
            this.server.createContext("/", exchange -> {
                String path = exchange.getRequestURI().getPath();
                StreamingEndpoint endpoint = routes.get(path);
                if (endpoint == null) {
                    String body = "Not Found: " + path + "\n";
                    exchange.sendResponseHeaders(404, body.length());
                    try (OutputStream os = exchange.getResponseBody()) {
                        os.write(body.getBytes(StandardCharsets.UTF_8));
                    }
                    return;
                }
                // SSE handshake
                exchange.getResponseHeaders().set("Content-Type", "text/event-stream");
                exchange.getResponseHeaders().set("Cache-Control", "no-cache");
                exchange.getResponseHeaders().set("Connection", "keep-alive");
                exchange.sendResponseHeaders(200, 0);
                OutputStream os = exchange.getResponseBody();
                endpoint.addClient(os);
                // Block until the client disconnects (read returns -1) or
                // the JVM shuts down. The dispatcher thread is virtual so
                // this is cheap.
                try {
                    byte[] buf = new byte[64];
                    //noinspection InfiniteLoopStatement
                    while (true) {
                        int read = exchange.getRequestBody().read(buf);
                        if (read == -1) break;
                    }
                } catch (IOException ignored) {
                    // Client closed the connection
                } finally {
                    endpoint.removeClient(os);
                    try { os.close(); } catch (IOException ignored) {}
                    exchange.close();
                }
            });
        }

        void start() {
            server.start();
        }

        void register(String route, StreamingEndpoint endpoint) {
            routes.put(route, endpoint);
        }

        synchronized void unregister(StreamingEndpoint endpoint) {
            String route = "/" + ConfigProvider.getConfig()
                    .getOptionalValue("quark.streaming.path-prefix", String.class).orElse("stream")
                    + "/" + endpoint.namespace() + "/" + endpoint.systemName() + "/" + endpoint.nodeName();
            routes.remove(route);
            // If no more routes are registered, stop the HTTP server and
            // remove it from the shared map so the port is released.
            if (routes.isEmpty()) {
                server.stop(0);
                String key = server.getAddress().getHostString() + ":" + server.getAddress().getPort();
                SHARED_SERVERS.remove(key);
                log.info("Stopped streaming HTTP server at {} (no more endpoints registered)", key);
            }
        }
    }
}
