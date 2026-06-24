package com.quarkloop.quark.runtime.polyglot;

import com.quarkloop.quark.core.domain.config.NodeConfig;
import com.quarkloop.quark.core.domain.identity.NodeUri;
import com.quarkloop.quark.core.domain.spi.NodeProvider;
import com.quarkloop.quark.core.registry.NodeDescriptor;
import com.quarkloop.quark.core.registry.NodeImplementationFactory;
import org.graalvm.polyglot.Context;
import org.graalvm.polyglot.Engine;
import org.graalvm.polyglot.HostAccess;
import org.graalvm.polyglot.Value;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import java.util.Objects;

/**
 * Factory that creates {@link NodeProvider} instances from TypeScript source
 * code fetched from the Catalog.
 *
 * <p>The TypeScript source is evaluated in a GraalJS {@link Context}. The
 * exported object's methods are detected at runtime:
 * <ul>
 *   <li>Has {@code onStart}/{@code start} + {@code onStop}/{@code stop} → autonomous</li>
 *   <li>Has {@code onMessage} → reactive</li>
 *   <li>Has all three → hybrid</li>
 * </ul>
 *
 * <p>There are no behavioral categories — the domain (from the URI) is the
 * only organizational axis. The engine detects which methods are present.
 */
public class TypeScriptNodeFactory implements NodeImplementationFactory {

    private static final Logger log = LoggerFactory.getLogger(TypeScriptNodeFactory.class);

    private static volatile Engine sharedEngine;

    private final String uri;
    private final String source;
    private final String description;

    public TypeScriptNodeFactory(String uri, String source, String description) {
        this.uri = Objects.requireNonNull(uri, "uri cannot be null");
        this.source = Objects.requireNonNull(source, "source cannot be null");
        this.description = description != null ? description : "";
    }

    @Override
    public String uriPattern() {
        // Strip the version from the URI
        int colonIdx = uri.lastIndexOf(':');
        return colonIdx > 0 ? uri.substring(0, colonIdx) : uri;
    }

    @Override
    public NodeProvider create(NodeConfig config) {
        Engine engine = getSharedEngine();

        Context ctx = Context.newBuilder("js")
                .engine(engine)
                .allowHostAccess(HostAccess.ALL)
                .allowHostClassLookup(name -> false)
                .allowIO(false)
                .allowCreateThread(false)
                .allowCreateProcess(false)
                .allowNativeAccess(false)
                .allowExperimentalOptions(true)
                .option("engine.WarnInterpreterOnly", "false")
                .build();

        try {
            // Strip TypeScript syntax (same as GraalJsSystemParser)
            String jsCode = stripTypeScript(source);

            Value exported = ctx.eval("js", jsCode);

            return new TypeScriptNodeProvider(exported, ctx, config);
        } catch (Exception e) {
            ctx.close();
            throw new RuntimeException("Failed to evaluate TypeScript node " + uri + ": " + e.getMessage(), e);
        }
    }

    @Override
    public NodeDescriptor descriptor() {
        return new NodeDescriptor(NodeUri.parse(uri), description);
    }

    private static Engine getSharedEngine() {
        if (sharedEngine == null) {
            synchronized (TypeScriptNodeFactory.class) {
                if (sharedEngine == null) {
                    sharedEngine = Engine.newBuilder("js")
                            .option("engine.WarnInterpreterOnly", "false")
                            .build();
                }
            }
        }
        return sharedEngine;
    }

    /**
     * Strip TypeScript-specific syntax to produce evaluable JavaScript.
     * This is the same regex pipeline used by GraalJsSystemParser.
     */
    static String stripTypeScript(String source) {
        String js = source;

        // Remove import statements
        js = js.replaceAll("import\\s+.*?from\\s+['\"][^'\"]+['\"];?", "");
        js = js.replaceAll("import\\s+['\"][^'\"]+['\"];?", "");

        // Remove export keyword (keep the expression)
        js = js.replace("export default", "(");
        // If we replaced "export default" with "(", we need a closing ")"
        // But only if the original was "export default { ... }" → "( { ... } )"
        // Actually, simpler: just remove "export default " and wrap in parens
        js = source;
        js = js.replaceAll("import\\s+.*?from\\s+['\"][^'\"]+['\"];?", "");
        js = js.replaceAll("import\\s+['\"][^'\"]+['\"];?", "");
        js = js.replace("export default ", "");
        js = js.replace("export ", "");

        // Remove interface declarations
        js = js.replaceAll("interface\\s+\\w+\\s*\\{[^}]*}", "");

        // Remove type declarations
        js = js.replaceAll("type\\s+\\w+\\s*=?\\s*[^;]+;", "");

        // Remove type annotations: : Type (but not in object literals like { key: value })
        // This is tricky — we only want to remove type annotations after parameter names and variable declarations
        js = js.replaceAll("\\)\\s*:\\s*[A-Za-z_][A-Za-z0-9_<>,\\s\\[\\]\\|]*\\s*\\{", ") {");
        js = js.replaceAll("\\)\\s*:\\s*[A-Za-z_][A-Za-z0-9_<>,\\s\\[\\]\\|]*\\s*=>", ") =>");

        // Remove generic type parameters <T>
        js = js.replaceAll("<[A-Z][A-Za-z0-9_,\\s]*>", "");

        // Remove 'as' casts
        js = js.replaceAll("\\bas\\s+[A-Za-z_][A-Za-z0-9_]*", "");

        // Wrap in parens to make it an expression
        js = "(" + js + ")";

        return js;
    }
}
