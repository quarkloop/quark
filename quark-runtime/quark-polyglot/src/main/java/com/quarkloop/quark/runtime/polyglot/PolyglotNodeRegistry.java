package com.quarkloop.quark.runtime.polyglot;

import com.fasterxml.jackson.databind.ObjectMapper;
import com.quarkloop.quark.runtime.domain.identity.NodeUri;
import com.quarkloop.quark.runtime.engine.nats.NatsConnectionManager;
import com.quarkloop.quark.runtime.registry.NodeImplementationFactory;
import io.nats.client.Connection;
import io.nats.client.Message;
import jakarta.enterprise.context.ApplicationScoped;
import jakarta.inject.Inject;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import java.io.ByteArrayInputStream;
import java.io.IOException;
import java.net.URL;
import java.net.URLClassLoader;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.time.Duration;
import java.util.*;
import java.util.concurrent.ConcurrentHashMap;
import java.util.jar.JarEntry;
import java.util.jar.JarInputStream;

/**
 * Retrieves node packages from the Catalog service and instantiates the
 * appropriate {@link NodeImplementationFactory} for each one.
 *
 * <p>The runtime NEVER compiles or stores node implementations directly.
 * Every node — Java or TypeScript — is fetched from the Catalog on first
 * use via the {@code registry.node.pull} NATS subject and cached for the
 * rest of the process lifetime. This keeps the runtime binary small,
 * generic, and decoupled from any specific node implementation: adding
 * a new node only requires pushing it to the Catalog, not rebuilding
 * the runtime.
 *
 * <h2>Supported content types</h2>
 * <ul>
 *   <li><strong>typescript</strong> — content is the UTF-8 source of an
 *       ECMAScript module ({@code export default { ... }}). A
 *       {@link TypeScriptNodeFactory} evaluates it via GraalJS's native
 *       ESM module support.</li>
 *   <li><strong>shared-library</strong> — content is the bytes of a
 *       {@code .jar} file containing one or more classes that implement
 *       {@link NodeImplementationFactory}. The jar is written to a
 *       temporary file, loaded via a child {@link URLClassLoader} whose
 *       parent is the runtime's classloader (so the node code can see
 *       {@code com.quarkloop.quark.runtime.*} and {@code org.slf4j}), and
 *       the first matching class is instantiated via its no-arg
 *       constructor.</li>
 * </ul>
 *
 * <p>The catalog pull is best-effort: on any failure (catalog down,
 * package missing, jar parse error, class load error), the lookup
 * returns {@link Optional#empty()} and the caller
 * ({@link com.quarkloop.quark.runtime.engine.lifecycle.SystemDeployer})
 * fails the deploy with a clear "Unknown node URIs" error. There are
 * no code-level fallbacks — see AGENTS.md pitfall 6.
 *
 * <p>Loaded factories are cached by URI so subsequent deploys of the
 * same URI in the same process don't re-pull. Use {@link #clearCache()}
 * to force a re-pull (e.g. after a node has been re-pushed).
 */
@ApplicationScoped
public class PolyglotNodeRegistry implements com.quarkloop.quark.runtime.engine.polyglot.PolyglotNodeLookup {

    private static final Logger log = LoggerFactory.getLogger(PolyglotNodeRegistry.class);
    private static final Duration REQUEST_TIMEOUT = Duration.ofSeconds(5);

    private final ObjectMapper mapper = new ObjectMapper();
    private final NatsConnectionManager natsConnectionManager;

    /** Cache of URI → factory, thread-safe. */
    private final ConcurrentHashMap<String, NodeImplementationFactory> cache = new ConcurrentHashMap<>();

    /** Tracks temp jar files so we can delete them on shutdown. */
    private final List<Path> tempJars = Collections.synchronizedList(new ArrayList<>());

    @Inject
    public PolyglotNodeRegistry(NatsConnectionManager natsConnectionManager) {
        this.natsConnectionManager = natsConnectionManager;
    }

    @Override
    public Optional<NodeImplementationFactory> lookupFactory(NodeUri uri) {
        String uriStr = uri.rawUri();

        // Check cache first
        NodeImplementationFactory cached = cache.get(uriStr);
        if (cached != null) {
            return Optional.of(cached);
        }

        // Query the catalog
        try {
            Connection conn = natsConnectionManager.getConnection();
            byte[] req = mapper.writeValueAsBytes(Map.of("uri", uriStr));
            Message reply = conn.request("registry.node.pull", req, REQUEST_TIMEOUT);

            if (reply == null) {
                log.warn("Catalog did not respond to registry.node.pull for {} within {}s",
                        uriStr, REQUEST_TIMEOUT.getSeconds());
                return Optional.empty();
            }

            var pkg = mapper.readTree(reply.getData());
            if (pkg.has("error") || !pkg.has("content")) {
                log.warn("Catalog returned no content for node {} (response: {})",
                        uriStr, pkg.has("error") ? pkg.get("error").asText() : "missing content");
                return Optional.empty();
            }

            String contentType = pkg.get("contentType").asText();
            String manifest = pkg.get("manifest").asText();
            // Catalog serialises content as base64-encoded string (NATS JSON
            // wire format). Jackson's binaryValue() returns null for non-binary
            // nodes, so we fall back to base64 decoding.
            byte[] content = pkg.get("content").binaryValue();
            if (content == null && pkg.get("content").isTextual()) {
                content = java.util.Base64.getDecoder().decode(pkg.get("content").asText());
            }
            if (content == null) {
                log.error("Catalog returned null content for node {}", uriStr);
                return Optional.empty();
            }

            NodeImplementationFactory factory = createFactory(uriStr, contentType, content, manifest);
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

    private NodeImplementationFactory createFactory(String uri, String contentType, byte[] content, String manifestJson) {
        try {
            var manifest = mapper.readTree(manifestJson);
            String description = manifest.has("description") ? manifest.get("description").asText() : "";

            return switch (contentType) {
                case "typescript" -> {
                    // Content is a zip containing manifest.json + node.ts.
                    // Extract the .ts source — for TypeScript nodes there's
                    // exactly one .ts file in the package.
                    String source = extractTypeScriptSource(uri, content);
                    if (source == null) {
                        log.error("No .ts file found in package for node {}", uri);
                        yield null;
                    }
                    yield new TypeScriptNodeFactory(uri, source, description);
                }
                case "shared-library" -> {
                    // Content is a zip containing manifest.json + <node>.jar.
                    // Extract the .jar bytes.
                    byte[] jarBytes = extractJarBytes(uri, content);
                    if (jarBytes == null) {
                        log.error("No .jar file found in package for node {}", uri);
                        yield null;
                    }
                    yield loadSharedLibrary(uri, jarBytes, description);
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
     * Extract the first {@code .ts} file from a node package zip.
     * Returns null if the zip doesn't contain a TypeScript source file.
     */
    private String extractTypeScriptSource(String uri, byte[] zipBytes) {
        try (var jar = new java.util.zip.ZipInputStream(new ByteArrayInputStream(zipBytes))) {
            java.util.zip.ZipEntry entry;
            while ((entry = jar.getNextEntry()) != null) {
                if (entry.getName().endsWith(".ts")) {
                    return new String(readAll(jar), StandardCharsets.UTF_8);
                }
            }
        } catch (IOException e) {
            log.error("Failed to extract .ts from package for node {}: {}", uri, e.getMessage());
        }
        return null;
    }

    /**
     * Extract the first {@code .jar} file from a node package zip.
     * Returns null if the zip doesn't contain a .jar file.
     */
    private byte[] extractJarBytes(String uri, byte[] zipBytes) {
        try (var jar = new java.util.zip.ZipInputStream(new ByteArrayInputStream(zipBytes))) {
            java.util.zip.ZipEntry entry;
            while ((entry = jar.getNextEntry()) != null) {
                if (entry.getName().endsWith(".jar")) {
                    return readAll(jar);
                }
            }
        } catch (IOException e) {
            log.error("Failed to extract .jar from package for node {}: {}", uri, e.getMessage());
        }
        return null;
    }

    private static byte[] readAll(java.io.InputStream in) throws IOException {
        java.io.ByteArrayOutputStream out = new java.io.ByteArrayOutputStream();
        byte[] buf = new byte[8192];
        int n;
        while ((n = in.read(buf)) > 0) {
            out.write(buf, 0, n);
        }
        return out.toByteArray();
    }

    /**
     * Load a Java shared-library node package.
     *
     * <p>The content is a {@code .jar} file containing one or more classes
     * that implement {@link NodeImplementationFactory}. The jar is
     * materialised to a temp file (URLClassLoader needs a URL; in-memory
     * classloaders are possible but more fragile), loaded via a child
     * URLClassLoader whose parent is the current thread's context
     * classloader (which, in Quarkus, is the application classloader
     * that sees {@code com.quarkloop.quark.runtime.*}), and the first
     * matching class is instantiated.
     *
     * <p>The temp file is kept for the lifetime of this registry (the
     * classloader holds a handle to it). It is deleted on JVM shutdown
     * via a shutdown hook.
     */
    private NodeImplementationFactory loadSharedLibrary(String uri, byte[] jarBytes, String description) {
        Path jarFile;
        try {
            jarFile = Files.createTempFile("quark-node-" + sanitizeFileName(uri) + "-", ".jar");
            Files.write(jarFile, jarBytes);
            tempJars.add(jarFile);
            log.debug("Wrote {} ({} bytes) to {}", uri, jarBytes.length, jarFile);
        } catch (IOException e) {
            log.error("Failed to materialise jar for node {}: {}", uri, e.getMessage());
            return null;
        }

        // Use the current thread's context classloader as parent — in Quarkus
        // this is the application classloader that sees all of core/*. Without
        // the right parent, the loaded Factory class can't see the
        // NodeImplementationFactory interface it implements, and we'd get
        // ClassNotFoundException at load time.
        ClassLoader parent = Thread.currentThread().getContextClassLoader();
        if (parent == null) {
            parent = getClass().getClassLoader();
        }

        // Keep the classloader open for the lifetime of the loaded factory
        // (otherwise the loaded classes get unloaded and the factory stops
        // working). We can't use try-with-resources here.
        URLClassLoader loader;
        try {
            loader = new URLClassLoader(new URL[]{jarFile.toUri().toURL()}, parent);
        } catch (java.net.MalformedURLException e) {
            log.error("Failed to build jar URL for node {}: {}", uri, e.getMessage());
            return null;
        }

        try {
            // Walk every .class entry in the jar, attempt to load it, and
            // check whether it implements NodeImplementationFactory. We can't
            // use ServiceLoader because the node.java sources don't declare
            // META-INF/services/ entries — they rely on CDI's @ApplicationScoped
            // discovery in the legacy in-process layout.
            //
            // Loading every class is more expensive than a bytecode scan, but
            // it's robust against all class-file versions and constant-pool
            // tag combinations. Node jars are small (a handful of classes),
            // so the cost is negligible.
            String factoryClassName = null;
            try (JarInputStream jar = new JarInputStream(new ByteArrayInputStream(jarBytes))) {
                JarEntry entry;
                while ((entry = jar.getNextJarEntry()) != null) {
                    String name = entry.getName();
                    if (!name.endsWith(".class")) continue;
                    // Skip inner classes — only top-level Factory classes are
                    // what we want. Inner classes (Foo$Bar.class) are loaded
                    // as part of their enclosing class.
                    if (name.contains("$")) continue;
                    // Convert com/foo/Bar.class → com.foo.Bar
                    String binaryName = name.substring(0, name.length() - ".class".length())
                            .replace('/', '.');

                    Class<?> clazz;
                    try {
                        clazz = Class.forName(binaryName, false, loader);
                    } catch (Throwable t) {
                        log.debug("Skipping class {} for node {}: {}", binaryName, uri, t.toString());
                        continue;
                    }
                    if (NodeImplementationFactory.class.isAssignableFrom(clazz)
                            && !clazz.isInterface()
                            && !java.lang.reflect.Modifier.isAbstract(clazz.getModifiers())) {
                        factoryClassName = binaryName;
                        log.debug("Found factory class {} for node {}", factoryClassName, uri);
                        break;
                    } else {
                        log.debug("Class {} for node {} is not a factory (assignable={}, interface={}, abstract={})",
                                binaryName, uri,
                                NodeImplementationFactory.class.isAssignableFrom(clazz),
                                clazz.isInterface(),
                                java.lang.reflect.Modifier.isAbstract(clazz.getModifiers()));
                    }
                }
            }

            if (factoryClassName == null) {
                log.error("No class implementing NodeImplementationFactory found in jar for node {}", uri);
                loader.close();
                return null;
            }

            log.debug("Loading factory class {} for node {}", factoryClassName, uri);
            Class<?> clazz = loader.loadClass(factoryClassName);
            @SuppressWarnings("unchecked")
            Class<? extends NodeImplementationFactory> factoryClass =
                    (Class<? extends NodeImplementationFactory>) clazz;
            // The factory class may be package-private (nodes/ convention uses
            // package-private classes so the file can be named node.java
            // instead of matching the public class name). setAccessible(true)
            // bypasses the access check.
            var ctor = factoryClass.getDeclaredConstructor();
            ctor.setAccessible(true);
            NodeImplementationFactory factory = ctor.newInstance();

            // Wrap in a descriptor override so the URI the runtime sees is the
            // URI the user requested, not whatever the Factory hardcodes.
            return new CatalogSourcedFactory(factory, uri, description);
        } catch (Exception e) {
            log.error("Failed to load shared-library node {}: {}", uri, e.getMessage(), e);
            try { loader.close(); } catch (IOException ignored) {}
            return null;
        }
    }

    private static String sanitizeFileName(String uri) {
        return uri == null ? "unknown" : uri.replaceAll("[^A-Za-z0-9._-]", "_");
    }

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

    public void clearCache() {
        cache.clear();
    }

    /**
     * Best-effort cleanup of temp jars on shutdown. Called via a
     * shutdown hook registered in the constructor. We can't delete
     * them eagerly because the URLClassLoader may still hold them
     * open.
     */
    void cleanupTempJars() {
        synchronized (tempJars) {
            for (Path jar : tempJars) {
                try {
                    Files.deleteIfExists(jar);
                } catch (IOException e) {
                    log.debug("Could not delete temp jar {}: {}", jar, e.getMessage());
                }
            }
            tempJars.clear();
        }
    }

    /**
     * Wrapper that overrides the {@link NodeImplementationFactory#descriptor()}
     * URI to use the URI the catalog pull was called with, rather than
     * whatever the Factory class hardcodes.
     *
     * <p>This is important because legacy Factory classes may have been
     * compiled with a different URI pattern (e.g. the old
     * {@code source/timer:v1} pattern from before the v8 URI refactor).
     * When the catalog is asked for {@code quark/time/schedule/timer:v1},
     * we want the runtime to see that URI, not the stale one inside the
     * jar.
     */
    private static final class CatalogSourcedFactory implements NodeImplementationFactory {
        private final NodeImplementationFactory delegate;
        private final String uri;
        private final String description;

        CatalogSourcedFactory(NodeImplementationFactory delegate, String uri, String description) {
            this.delegate = delegate;
            this.uri = uri;
            this.description = description;
        }

        @Override
        public String uriPattern() {
            // Strip the :version suffix so the pattern matches any version
            int colon = uri.indexOf(':');
            return colon > 0 ? uri.substring(0, colon) : uri;
        }

        @Override
        public com.quarkloop.quark.runtime.domain.spi.NodeProvider create(com.quarkloop.quark.runtime.domain.config.NodeConfig config) {
            return delegate.create(config);
        }

        @Override
        public com.quarkloop.quark.runtime.registry.NodeDescriptor descriptor() {
            return new com.quarkloop.quark.runtime.registry.NodeDescriptor(
                    com.quarkloop.quark.runtime.domain.identity.NodeUri.parse(uri),
                    description != null && !description.isBlank() ? description : delegate.descriptor().description()
            );
        }
    }
}
