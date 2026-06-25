package com.quarkloop.quark.runtime.polyglot;

import com.quarkloop.quark.core.domain.config.NodeConfig;
import com.quarkloop.quark.core.domain.identity.NodeUri;
import com.quarkloop.quark.core.domain.spi.NodeProvider;
import com.quarkloop.quark.core.registry.NodeDescriptor;
import com.quarkloop.quark.core.registry.NodeImplementationFactory;
import org.graalvm.polyglot.Context;
import org.graalvm.polyglot.HostAccess;
import org.graalvm.polyglot.Source;
import org.graalvm.polyglot.Value;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import java.util.Objects;

/**
 * Factory that creates {@link NodeProvider} instances from TypeScript source
 * code fetched from the Catalog.
 *
 * <h2>TypeScript handling</h2>
 *
 * <p>GraalJS Community Edition (24.1.x) does <strong>not</strong> natively
 * parse TypeScript — there is no {@code js.typescript} option (see
 * <a href="https://github.com/oracle/graaljs/issues/784">graaljs#784</a>
 * and the maintainer comment in
 * <a href="https://github.com/oracle/graaljs/issues/709">graaljs#709</a>).
 * The previously used hand-rolled regex-based
 * {@code stripTypeScript()} was fundamentally broken: TypeScript syntax is
 * not a regular language, so a regex cannot distinguish a TS keyword from
 * the same substring inside an identifier, comment, or string literal.
 *
 * <p>Specific bugs in the old regex stripper:
 * <ul>
 *   <li>{@code js.replace("export ", "")} also matched {@code "export "}
 *       inside {@code __quark_export = }, producing {@code __quark_= },
 *       so the binding lookup always failed.</li>
 *   <li>{@code \bas\s+Identifier} ate the words {@code "as JSON"} from
 *       comments such as {@code // ... as JSON.}.</li>
 * </ul>
 *
 * <p><strong>Current approach.</strong> The node source files in this
 * platform are <em>ECMAScript modules</em> using the standard
 * {@code export default { ... }} syntax — they do not contain TypeScript
 * type annotations, interfaces, or generics. We therefore evaluate them
 * directly as ESM using GraalJS's native module support:
 * <ul>
 *   <li>{@link Source} MIME type is set to
 *       {@code application/javascript+module}.</li>
 *   <li>Context option {@code js.esm-eval-returns-exports=true} causes
 *       {@link Context#eval(Source)} to return the module namespace
 *       object.</li>
 *   <li>The namespace's {@code default} member is the value of
 *       {@code export default { ... }}.</li>
 * </ul>
 *
 * <p>This is the approach recommended by the GraalJS maintainers in
 * <a href="https://www.graalvm.org/latest/reference-manual/js/Modules">the
 * GraalJS Modules reference</a> and matches what
 * <a href="https://github.com/reactiverse/es4x">es4x</a> does at runtime.
 *
 * <p>If real TypeScript (with type annotations) needs to be supported in
 * the future, the source should be transpiled to JavaScript at catalog
 * push time using {@code tsc}, {@code esbuild}, or {@code swc} — not by
 * regex stripping at runtime.
 */
public class TypeScriptNodeFactory implements NodeImplementationFactory {

    private static final Logger log = LoggerFactory.getLogger(TypeScriptNodeFactory.class);

    /**
     * MIME type that triggers GraalJS's ECMAScript Module parser.
     * Equivalent to using a {@code .mjs} file extension.
     */
    static final String ESM_MIME_TYPE = "application/javascript+module";

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
                // Required so that Context.eval(Source) of an ESM Source
                // returns the module namespace object (rather than
                // undefined). See GraalJS Modules reference.
                .option("js.esm-eval-returns-exports", "true")
                .build();

        try {
            // Provide a `console` global so node scripts that call
            // console.log/error/warn don't throw ReferenceError. GraalJS
            // does not provide console in sandboxed contexts.
            ctx.getBindings("js").putMember("console", new JsConsole(uri));

            // Inject config global BEFORE evaluating the module so that
            // module code can read it via the global scope chain.
            ctx.getBindings("js").putMember("config", new JsConfig(config));

            // Evaluate as an ECMAScript Module. The Source's MIME type
            // tells GraalJS to use the module parser, which natively
            // understands `export default { ... }`.
            Source src = Source.newBuilder("js", source, uriToFilename(uri))
                    .mimeType(ESM_MIME_TYPE)
                    .buildLiteral();
            Value moduleNamespace = ctx.eval(src);

            if (moduleNamespace == null || moduleNamespace.isNull()) {
                throw new RuntimeException(
                        "GraalJS returned null module namespace for " + uri);
            }

            Value exported = moduleNamespace.getMember("default");
            if (exported == null || exported.isNull()) {
                throw new RuntimeException(
                        "No 'export default { ... }' found in TypeScript node "
                                + uri + " — the source must end with "
                                + "`export default { onMessage: function(...) { ... } ... };`");
            }

            return new TypeScriptNodeProvider(exported, ctx, config);
        } catch (Exception e) {
            try {
                ctx.close();
            } catch (Exception closeEx) {
                log.trace("Failed to close GraalJS context after create() failure", closeEx);
            }
            throw new RuntimeException(
                    "Failed to evaluate TypeScript node " + uri + ": " + e.getMessage(), e);
        }
    }

    @Override
    public NodeDescriptor descriptor() {
        return new NodeDescriptor(NodeUri.parse(uri), description);
    }

    /**
     * Convert a node URI (e.g. {@code quark/log/console/stdout:v1}) to a
     * filename with {@code .mjs} extension. The filename is used by GraalJS
     * for stack traces and module-source identification only — it is not
     * read from disk because the source text is provided inline.
     */
    static String uriToFilename(String uri) {
        String safe = uri.replace(':', '_').replace('/', '_');
        return safe + ".mjs";
    }
}
