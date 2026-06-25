package com.quarkloop.quark.providers.streamingendpoint;

import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.datatype.jsr310.JavaTimeModule;
import com.quarkloop.quark.core.domain.config.NodeConfig;
import com.quarkloop.quark.core.domain.identity.NodeUri;
import com.quarkloop.quark.core.domain.spi.NodeProvider;
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
 * SSE broadcast node — runs an HTTP server and exposes per-node SSE streams.
 *
 * <p>URI: {@code quark/stream/sse/broadcast:v1}
 */
@ApplicationScoped
public class StreamingEndpointFactory implements NodeImplementationFactory {

    private static final Logger log = LoggerFactory.getLogger(StreamingEndpointFactory.class);

    private static final String CONFIG_KEY_SYSTEM    = "_quark_system";
    private static final String CONFIG_KEY_NAMESPACE = "_quark_namespace";
    private static final String CONFIG_KEY_NODE      = "_quark_node";

    @Override
    public String uriPattern() {
        return "quark/stream/sse/broadcast";
    }

    @Override
    public NodeProvider create(NodeConfig config) {
        return new SseBroadcastNode(config);
    }

    @Override
    public NodeDescriptor descriptor() {
        return new NodeDescriptor(
                NodeUri.parse("quark/stream/sse/broadcast:v1"),
                "Exposes an HTTP SSE endpoint that streams incoming NATS messages."
        );
    }

    private static final Map<String, SharedServer> SHARED_SERVERS = new ConcurrentHashMap<>();

    static final class SseBroadcastNode implements NodeProvider {

        private static final ObjectMapper MAPPER = new ObjectMapper();
        static { MAPPER.registerModule(new JavaTimeModule()); }

        private String namespace;
        private String systemName;
        private String nodeName;
        private final List<OutputStream> clients = new CopyOnWriteArrayList<>();
        private final Map<OutputStream, Object> clientLocks = new ConcurrentHashMap<>();
        private SharedServer sharedServer;

        SseBroadcastNode(NodeConfig config) {
            this.namespace = config.getString(CONFIG_KEY_NAMESPACE, "default");
            this.systemName = config.getString(CONFIG_KEY_SYSTEM, "system");
            this.nodeName = config.getString(CONFIG_KEY_NODE, "endpoint");
        }

        @Override
        public void init(NodeConfig config) {
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

            String route = "/" + prefix + "/" + namespace + "/" + systemName + "/" + nodeName;
            server.register(route, this);
            log.info("SSE endpoint registered: {} -> node {}/{}/{}", route, namespace, systemName, nodeName);
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
                    clients.remove(os);
                    clientLocks.remove(os);
                    log.debug("SSE client disconnected (node {}/{}/{})", namespace, systemName, nodeName);
                }
            }
        }

        @Override
        public void close() {
            if (sharedServer != null) {
                sharedServer.unregister(this);
            }
            for (OutputStream os : clients) {
                try { os.close(); } catch (IOException ignored) {}
            }
            clients.clear();
            clientLocks.clear();
            log.info("SSE endpoint stopped: node {}/{}/{}", namespace, systemName, nodeName);
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

        Object getClientLock(OutputStream os) {
            return clientLocks.get(os);
        }

        String namespace() { return namespace; }
        String systemName() { return systemName; }
        String nodeName() { return nodeName; }
    }

    static final class SharedServer {
        private final com.sun.net.httpserver.HttpServer server;
        private final Map<String, SseBroadcastNode> routes = new ConcurrentHashMap<>();

        SharedServer(InetSocketAddress addr) throws IOException {
            this.server = com.sun.net.httpserver.HttpServer.create(addr, 0);
            boolean isNative = System.getProperty("org.graalvm.nativeimage.imagecodekey") != null
                    || "true".equals(System.getProperty("quark.native"));
            java.util.concurrent.ThreadFactory factory = isNative
                    ? Thread.ofPlatform().name("quark-sse-", 0).factory()
                    : Thread.ofVirtual().name("quark-sse-", 0).factory();
            this.server.setExecutor(java.util.concurrent.Executors.newThreadPerTaskExecutor(factory));
            this.server.createContext("/", exchange -> {
                String path = exchange.getRequestURI().getPath();
                SseBroadcastNode endpoint = routes.get(path);
                if (endpoint == null) {
                    String body = "Not Found: " + path + "\n";
                    exchange.sendResponseHeaders(404, body.length());
                    try (OutputStream os = exchange.getResponseBody()) {
                        os.write(body.getBytes(StandardCharsets.UTF_8));
                    }
                    return;
                }
                exchange.getResponseHeaders().set("Content-Type", "text/event-stream");
                exchange.getResponseHeaders().set("Cache-Control", "no-cache");
                exchange.getResponseHeaders().set("Connection", "keep-alive");
                exchange.sendResponseHeaders(200, 0);
                OutputStream os = exchange.getResponseBody();
                endpoint.addClient(os);
                try {
                    while (!Thread.currentThread().isInterrupted()) {
                        Thread.sleep(2000);
                        synchronized (endpoint.getClientLock(os)) {
                            os.write(": keepalive\n\n".getBytes(StandardCharsets.UTF_8));
                            os.flush();
                        }
                    }
                } catch (IOException e) {
                    // Client closed
                } catch (InterruptedException e) {
                    Thread.currentThread().interrupt();
                } finally {
                    endpoint.removeClient(os);
                    try { os.close(); } catch (IOException ignored) {}
                    exchange.close();
                }
            });
        }

        void start() { server.start(); }

        void register(String route, SseBroadcastNode endpoint) {
            routes.put(route, endpoint);
        }

        synchronized void unregister(SseBroadcastNode endpoint) {
            String route = "/" + ConfigProvider.getConfig()
                    .getOptionalValue("quark.streaming.path-prefix", String.class).orElse("stream")
                    + "/" + endpoint.namespace() + "/" + endpoint.systemName() + "/" + endpoint.nodeName();
            routes.remove(route);
            if (routes.isEmpty()) {
                server.stop(0);
                String key = server.getAddress().getHostString() + ":" + server.getAddress().getPort();
                SHARED_SERVERS.remove(key);
                log.info("Stopped streaming HTTP server at {} (no more endpoints registered)", key);
            }
        }
    }
}
