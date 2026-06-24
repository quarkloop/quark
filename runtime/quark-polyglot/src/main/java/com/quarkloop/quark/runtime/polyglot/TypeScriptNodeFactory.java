package com.quarkloop.quark.runtime.polyglot;

import com.quarkloop.quark.core.domain.config.NodeConfig;
import com.quarkloop.quark.core.domain.identity.NodeUri;
import com.quarkloop.quark.core.domain.spi.NodeProvider;
import com.quarkloop.quark.core.registry.NodeDescriptor;
import com.quarkloop.quark.core.registry.NodeImplementationFactory;
import org.graalvm.polyglot.Context;
import org.graalvm.polyglot.HostAccess;
import org.graalvm.polyglot.Value;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import java.util.Objects;

/**
 * Factory that creates {@link NodeProvider} instances from TypeScript source
 * code fetched from the Catalog.
 */
public class TypeScriptNodeFactory implements NodeImplementationFactory {

    private static final Logger log = LoggerFactory.getLogger(TypeScriptNodeFactory.class);

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
        int colonIdx = uri.lastIndexOf(':');
        return colonIdx > 0 ? uri.substring(0, colonIdx) : uri;
    }

    @Override
    public NodeProvider create(NodeConfig config) {
        Context ctx = Context.newBuilder("js")
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
            String jsCode = stripTypeScript(source);
            ctx.eval("js", jsCode);

            Value exported = ctx.getBindings("js").getMember("__quark_export");
            if (exported == null) {
                throw new RuntimeException("No __quark_export found — the TypeScript source must have 'export default { ... }'");
            }

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

    static String stripTypeScript(String source) {
        String js = source;

        // Remove import statements
        js = js.replaceAll("import\\s+.*?from\\s+['\"][^'\"]+['\"];?", "");
        js = js.replaceAll("import\\s+['\"][^'\"]+['\"];?", "");

        // Replace "export default " with "var __quark_export = "
        js = js.replace("export default ", "var __quark_export = ");

        // Remove any remaining 'export ' keyword
        js = js.replace("export ", "");

        // Remove interface declarations
        js = js.replaceAll("interface\\s+\\w+\\s*\\{[^}]*}", "");

        // Remove type declarations
        js = js.replaceAll("type\\s+\\w+\\s*=?\\s*[^;]+;", "");

        // Remove type annotations after function parameters
        js = js.replaceAll("\\)\\s*:\\s*[A-Za-z_][A-Za-z0-9_<>,\\s\\[\\]\\|]*\\s*\\{", ") {");
        js = js.replaceAll("\\)\\s*:\\s*[A-Za-z_][A-Za-z0-9_<>,\\s\\[\\]\\|]*\\s*=>", ") =>");

        // Remove generic type parameters
        js = js.replaceAll("<[A-Z][A-Za-z0-9_,\\s]*>", "");

        // Remove 'as' casts
        js = js.replaceAll("\\bas\\s+[A-Za-z_][A-Za-z0-9_]*", "");

        return js;
    }
}
