package com.quarkloop.quark.app.polyglot;

import com.fasterxml.jackson.databind.ObjectMapper;
import com.quarkloop.quark.core.domain.category.NodeCategory;
import com.quarkloop.quark.core.domain.identity.NodeUri;
import com.quarkloop.quark.core.engine.dataplane.DataPlaneIpc;
import com.quarkloop.quark.core.registry.NodeDescriptor;
import com.quarkloop.quark.core.registry.NodeImplementationFactory;
import com.quarkloop.quark.engine.NatsConnectionManager;
import io.nats.client.Connection;
import io.nats.client.Message;
import jakarta.enterprise.context.ApplicationScoped;
import jakarta.inject.Inject;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import java.nio.charset.StandardCharsets;
import java.time.Duration;
import java.util.*;

/**
 * Retrieves node packages from the Catalog service and creates
 * {@link TypeScriptNodeFactory} instances for TypeScript nodes.
 *
 * <p>When a {@code .quark.ts} file references a URI that is not in the built-in
 * Java {@code NodeRegistry}, the {@code SystemDeployer} calls this registry
 * to fetch the node implementation from the Catalog.
 *
 * <p>The Catalog stores node packages with a {@code content_type} field:
 * <ul>
 *   <li>{@code "typescript"} — source code, evaluated by GraalJS</li>
 *   <li>{@code "shared-library"} — .so file, loaded via JNI (future)</li>
 *   <li>{@code "python"} — source code, evaluated by GraalPy (future)</li>
 * </ul>
 *
 * <p>This class handles {@code "typescript"} content. Java built-in nodes are
 * handled by the existing {@code InMemoryNodeRegistry}.
 */
@ApplicationScoped
public class PolyglotNodeRegistry implements com.quarkloop.quark.core.engine.polyglot.PolyglotNodeLookup {

    private static final Logger log = LoggerFactory.getLogger(PolyglotNodeRegistry.class);
    private static final Duration REQUEST_TIMEOUT = Duration.ofSeconds(5);

    private final ObjectMapper mapper = new ObjectMapper();
    private final NatsConnectionManager natsConnectionManager;

    /** Cache of URI → factory, to avoid re-downloading on every deploy. */
    private final Map<String, NodeImplementationFactory<?>> cache = new HashMap<>();

    @Inject
    public PolyglotNodeRegistry(NatsConnectionManager natsConnectionManager) {
        this.natsConnectionManager = natsConnectionManager;
    }

    /**
     * Look up a node implementation by URI from the Catalog.
     *
     * @param uri the full node URI (e.g., "function/my-fn:v1")
     * @return the factory, or empty if not found in the catalog
     */
    public Optional<NodeImplementationFactory<?>> lookupFactory(NodeUri uri) {
        String uriStr = uri.rawUri();

        // Check cache first
        if (cache.containsKey(uriStr)) {
            return Optional.of(cache.get(uriStr));
        }

        // Query the catalog
        try {
            Connection conn = natsConnectionManager.getConnection();
            byte[] req = mapper.writeValueAsBytes(Map.of("uri", uriStr));
            Message reply = conn.request("registry.node.pull", req, REQUEST_TIMEOUT);

            if (reply == null) {
                log.warn("Catalog did not respond to registry.node.pull for {} within {}s", uriStr, REQUEST_TIMEOUT.getSeconds());
                return Optional.empty();
            }

            var pkg = mapper.readTree(reply.getData());
            if (pkg.has("error") || !pkg.has("content")) {
                return Optional.empty();
            }

            String contentType = pkg.get("contentType").asText();
            String manifest = pkg.get("manifest").asText();
            byte[] content = pkg.get("content").binaryValue();

            NodeImplementationFactory<?> factory = createFactory(uriStr, contentType, content, manifest);
            if (factory != null) {
                cache.put(uriStr, factory);
                log.info("Loaded node {} from catalog (type={}, {} bytes)", uriStr, contentType, content.length);
                return Optional.of(factory);
            }
        } catch (Exception e) {
            log.error("Failed to look up node {} from catalog", uriStr, e);
        }

        return Optional.empty();
    }

    /**
     * Create the appropriate factory based on content type.
     */
    private NodeImplementationFactory<?> createFactory(String uri, String contentType, byte[] content, String manifestJson) {
        try {
            var manifest = mapper.readTree(manifestJson);
            String categoryStr = manifest.has("category") ? manifest.get("category").asText() : "function";
            NodeCategory category = NodeCategory.valueOf(categoryStr.toUpperCase());
            String description = manifest.has("description") ? manifest.get("description").asText() : "";

            return switch (contentType) {
                case "typescript" -> {
                    String source = new String(content, StandardCharsets.UTF_8);
                    yield new TypeScriptNodeFactory(uri, source, category, description);
                }
                case "shared-library" -> {
                    log.warn("Shared-library node loading not yet implemented for {}", uri);
                    yield null;
                }
                case "python" -> {
                    log.warn("Python node loading not yet implemented for {}", uri);
                    yield null;
                }
                default -> {
                    log.warn("Unknown content type '{}' for node {}", contentType, uri);
                    yield null;
                }
            };
        } catch (Exception e) {
            log.error("Failed to create factory for {} from manifest", uri, e);
            return null;
        }
    }

    /**
     * Check if a node package exists in the catalog.
     */
    public boolean existsInCatalog(String uri) {
        try {
            Connection conn = natsConnectionManager.getConnection();
            byte[] req = mapper.writeValueAsBytes(Map.of("uri", uri));
            Message reply = conn.request("registry.node.exists", req, REQUEST_TIMEOUT);
            if (reply == null) return false;
            var resp = mapper.readTree(reply.getData());
            return resp.has("exists") && resp.get("exists").asBoolean();
        } catch (Exception e) {
            return false;
        }
    }

    /**
     * Clear the cache (called when a system is undeployed).
     */
    public void clearCache() {
        cache.clear();
    }
}
