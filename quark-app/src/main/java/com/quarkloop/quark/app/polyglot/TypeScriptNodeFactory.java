package com.quarkloop.quark.app.polyglot;

import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.datatype.jsr310.JavaTimeModule;
import com.quarkloop.quark.core.domain.category.NodeCategory;
import com.quarkloop.quark.core.domain.config.NodeConfig;
import com.quarkloop.quark.core.domain.identity.NodeUri;
import com.quarkloop.quark.core.domain.spi.*;
import com.quarkloop.quark.core.registry.NodeDescriptor;
import com.quarkloop.quark.core.registry.NodeImplementationFactory;
import org.graalvm.polyglot.Context;
import org.graalvm.polyglot.HostAccess;
import org.graalvm.polyglot.Value;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import java.util.HashMap;
import java.util.Map;

/**
 * A {@link NodeImplementationFactory} that creates node providers from
 * TypeScript source code evaluated by GraalJS.
 *
 * <p>This enables polyglot node implementations — nodes written in TypeScript
 * that run inside a GraalJS sandbox. The TypeScript source is retrieved from
 * the Catalog service's node package registry and evaluated at deploy time.
 *
 * <p>The TypeScript node must export a default object implementing one of the
 * Quark provider interfaces (SourceProvider, FunctionProvider, StoreProvider,
 * EndpointProvider). The factory inspects the exported object's methods to
 * determine which interface it implements.
 *
 * <h2>TypeScript Node Contract</h2>
 * <pre>
 * // node.ts — a Function node that doubles incoming values
 * export default {
 *   onMessage(message, publisher) {
 *     publisher.publish("data", { value: message.payload.value * 2 });
 *   }
 * };
 *
 * // Or a Source node that emits ticks
 * export default {
 *   onStart(publisher, config) {
 *     this.interval = setInterval(() => {
 *       publisher.publish("tick", { count: ++this.count || 1 });
 *     }, parseInt(config.interval || "1000"));
 *   },
 *   onStop() {
 *     clearInterval(this.interval);
 *   }
 * };
 * </pre>
 *
 * <p>The factory creates a GraalJS {@link Context} with:
 * <ul>
 *   <li>{@link HostAccess#ALL} — so the TS code can call Java methods on the
 *       injected publisher and config objects</li>
 *   <li>No file I/O, no threads, no native access (sandboxed)</li>
 *   <li>The publisher and config injected as global variables</li>
 * </ul>
 */
public class TypeScriptNodeFactory implements NodeImplementationFactory<Object> {

    private static final Logger log = LoggerFactory.getLogger(TypeScriptNodeFactory.class);
    private static final ObjectMapper mapper = new ObjectMapper();
    { mapper.registerModule(new JavaTimeModule()); }

    private final String uri;
    private final String source;
    private final NodeCategory category;
    private final String description;

    /**
     * @param uri         the full node URI (e.g., "function/my-fn:v1")
     * @param source      the TypeScript/JavaScript source code
     * @param category    the node category (determined from the manifest)
     * @param description the node description (from the manifest)
     */
    public TypeScriptNodeFactory(String uri, String source, NodeCategory category, String description) {
        this.uri = uri;
        this.source = source;
        this.category = category;
        this.description = description;
    }

    @Override
    public String uriPattern() {
        // Strip version: "function/my-fn:v1" → "function/my-fn"
        int colonIdx = uri.indexOf(':');
        return colonIdx > 0 ? uri.substring(0, colonIdx) : uri;
    }

    @Override
    public Object create(NodeConfig config) {
        log.info("Creating TypeScript node from source (uri={}, {} bytes)", uri, source.length());

        Context ctx = Context.newBuilder("js")
                .allowHostAccess(HostAccess.ALL)
                .allowHostClassLookup(name -> false)
                .allowIO(false)
                .allowCreateThread(false)
                .allowCreateProcess(false)
                .allowNativeAccess(false)
                .allowEnvironmentAccess(org.graalvm.polyglot.EnvironmentAccess.NONE)
                .option("engine.WarnInterpreterOnly", "false")
                .build();

        try {
            // Strip TypeScript syntax (same as GraalJsSystemParser)
            String jsCode = stripTypeScript(source);

            // Evaluate the node code
            Value exported = ctx.eval("js", jsCode);

            if (exported == null || exported.isNull()) {
                throw new IllegalArgumentException("TypeScript node did not export a default object");
            }

            // Wrap in the appropriate Java SPI interface based on the exported object's methods
            return wrapProvider(exported, ctx, config);

        } catch (Exception e) {
            ctx.close();
            throw new RuntimeException("Failed to create TypeScript node: " + e.getMessage(), e);
        }
    }

    /**
     * Inspect the exported JS object and wrap it in the appropriate Java SPI interface.
     *
     * <p>Detection logic:
     * <ul>
     *   <li>Has <code>onStart</code> and <code>onStop</code> but no <code>onMessage</code> → SourceProvider</li>
     *   <li>Has <code>onMessage</code> only → FunctionProvider or StoreProvider (based on category)</li>
     *   <li>Has <code>onStart</code>, <code>onMessage</code>, and <code>onStop</code> → EndpointProvider</li>
     * </ul>
     */
    private Object wrapProvider(Value exported, Context ctx, NodeConfig config) {
        boolean hasOnStart = exported.hasMember("onStart") || exported.hasMember("start");
        boolean hasOnMessage = exported.hasMember("onMessage");
        boolean hasOnStop = exported.hasMember("onStop") || exported.hasMember("stop");

        if (hasOnStart && hasOnMessage && hasOnStop) {
            return new TypeScriptEndpointProvider(exported, ctx, config);
        } else if (hasOnStart && hasOnStop) {
            return new TypeScriptSourceProvider(exported, ctx, config);
        } else if (hasOnMessage) {
            if (category == NodeCategory.STORE) {
                return new TypeScriptStoreProvider(exported, ctx, config);
            }
            return new TypeScriptFunctionProvider(exported, ctx, config);
        }

        throw new IllegalArgumentException(
                "TypeScript node must implement at least onMessage or onStart/onStop. " +
                "Found: onStart=" + hasOnStart + ", onMessage=" + hasOnMessage + ", onStop=" + hasOnStop);
    }

    @Override
    public NodeDescriptor descriptor() {
        return new NodeDescriptor(NodeUri.parse(uri), category, true, description);
    }

    @Override
    public NodeCategory category() {
        return category;
    }

    /**
     * Strip TypeScript syntax to produce valid JavaScript.
     * Same logic as GraalJsSystemParser.stripTypeScript.
     */
    private static String stripTypeScript(String ts) {
        String js = ts;
        js = js.replaceAll("(?m)^import\\s+.*?;$", "");
        js = js.replaceAll("(?m)^export\\s+default\\s+", "");
        js = js.replaceAll("(?m)^export\\s+", "");
        js = js.replaceAll("(?s)interface\\s+\\w+\\s*\\{.*?\\}", "");
        js = js.replaceAll("(?s)type\\s+\\w+\\s*=\\s*.*?;", "");
        js = js.replaceAll("(\\w+)\\s*:\\s*[A-Za-z_<>,\\[\\]\\s|]+\\s*=", "$1 =");
        js = js.replaceAll("\\s+as\\s+[A-Za-z_<>,\\[\\]\\s|]+", "");
        js = js.replaceAll("<[A-Za-z_<>,\\[\\]\\s|]+>", "");
        js = js.trim();
        while (js.endsWith(";")) {
            js = js.substring(0, js.length() - 1).trim();
        }
        js = "(" + js + ")";
        return js;
    }
}
