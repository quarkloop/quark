package com.quarkloop.quark.core.script;

import com.quarkloop.quark.core.domain.category.NodeCategory;
import com.quarkloop.quark.core.domain.config.NodeConfig;
import com.quarkloop.quark.core.domain.identity.Namespace;
import com.quarkloop.quark.core.domain.identity.NodeUri;
import com.quarkloop.quark.core.domain.metadata.Annotations;
import com.quarkloop.quark.core.domain.metadata.Labels;
import com.quarkloop.quark.core.domain.system.NodeDefinition;
import com.quarkloop.quark.core.domain.system.OnFailureConfig;
import com.quarkloop.quark.core.domain.system.SystemDefinition;
import jakarta.enterprise.context.ApplicationScoped;
import org.graalvm.polyglot.Context;
import org.graalvm.polyglot.Engine;
import org.graalvm.polyglot.HostAccess;
import org.graalvm.polyglot.Value;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import java.util.ArrayList;
import java.util.HashMap;
import java.util.List;
import java.util.Map;

/**
 * GraalJS-based parser for .quark.ts files.
 *
 * <p>Evaluates the user's TypeScript/JavaScript source code in a sandboxed
 * GraalJS context. The user's code exports a default object (the system
 * configuration). The parser extracts this object via GraalJS's polyglot
 * API and converts it to a Java {@link SystemDefinition}.
 *
 * <p>The sandbox:
 * <ul>
 *   <li>No host access (can't call Java methods)</li>
 *   <li>No file I/O, no threads, no processes, no native access</li>
 * </ul>
 *
 * <p>TypeScript transpilation: the parser strips TypeScript syntax (type
 * annotations, interfaces, imports, exports) using a simple regex-based
 * transpiler. The stripped JS is then evaluated by GraalJS.
 */
@ApplicationScoped
public class GraalJsSystemParser implements SystemParser {

    private static final Logger log = LoggerFactory.getLogger(GraalJsSystemParser.class);

    /**
     * Force Quarkus's build-time bytecode analysis to detect direct references
     * to classes in {@code truffle-api.jar} and {@code regex.jar}. Without
     * these references, Quarkus prunes those jars from the runtime, causing
     * {@code NoClassDefFoundError: org/graalvm/polyglot/impl/AbstractPolyglotImpl}
     * when {@link Engine#newBuilder()} tries to load the polyglot implementation
     * via ServiceLoader.
     */
    static {
        //noinspection ResultOfMethodCallIgnored
        GraalRuntimeReferences.TRUFFLE_LANGUAGE.getName();
        //noinspection ResultOfMethodCallIgnored
        GraalRuntimeReferences.POLYGLOT_IMPL.getName();
        //noinspection ResultOfMethodCallIgnored
        GraalRuntimeReferences.REGEX_LANGUAGE.getName();
    }

    /**
     * The GraalJS {@link Engine} is created lazily (not in a field initializer)
     * to control the thread's context classloader at creation time.
     *
     * <p>Quarkus's fast-jar layout splits GraalVM jars across two classloaders:
     * <ul>
     *   <li>{@code lib/boot/} (system classloader) — {@code truffle-api.jar}
     *       containing {@code com.oracle.truffle.polyglot.PolyglotImpl}</li>
     *   <li>{@code lib/main/} (Quarkus classloader) — {@code graal-sdk.jar}
     *       containing {@code org.graalvm.polyglot.impl.AbstractPolyglotImpl}
     *       (the superclass of {@code PolyglotImpl})</li>
     * </ul>
     *
     * <p>When {@link Engine#newBuilder()}.build() uses {@link java.util.ServiceLoader}
     * to discover the polyglot implementation, it may use a classloader that
     * can see {@code PolyglotImpl} but NOT its superclass {@code AbstractPolyglotImpl},
     * causing {@code NoClassDefFoundError}.
     *
     * <p>Setting the thread's context classloader to the Quarkus application
     * classloader (which can see BOTH {@code lib/main/} directly AND
     * {@code lib/boot/} via parent delegation) before creating the Engine
     * resolves the split-classloader issue.
     */
    private volatile Engine engine;

    private Engine getEngine() {
        Engine local = engine;
        if (local != null) {
            return local;
        }
        synchronized (this) {
            if (engine == null) {
                Thread current = Thread.currentThread();
                ClassLoader original = current.getContextClassLoader();
                // Use this class's classloader (Quarkus app classloader) which
                // can see both lib/main/ (graal-sdk) and lib/boot/ (truffle-api).
                current.setContextClassLoader(GraalJsSystemParser.class.getClassLoader());
                try {
                    engine = Engine.newBuilder()
                            .option("engine.WarnInterpreterOnly", "false")
                            .build();
                } finally {
                    current.setContextClassLoader(original);
                }
            }
            return engine;
        }
    }

    @Override
    @SuppressWarnings("unchecked")
    public SystemParseResult parse(String sourceCode) {
        if (sourceCode == null || sourceCode.isBlank()) {
            return new SystemParseResult.Failure(List.of("Source code is empty"));
        }

        List<String> errors = new ArrayList<>();

        // 1. Strip TypeScript syntax
        String jsCode = stripTypeScript(sourceCode);

        // 2. Create sandboxed GraalJS context
        try (Context ctx = Context.newBuilder("js")
                .engine(getEngine())
                .allowHostAccess(HostAccess.NONE)
                .allowHostClassLookup(name -> false)
                .allowIO(false)
                .allowCreateThread(false)
                .allowCreateProcess(false)
                .allowNativeAccess(false)
                .allowEnvironmentAccess(org.graalvm.polyglot.EnvironmentAccess.NONE)
                .build()) {

            // 3. Evaluate the user's code — capture the exported default object
            // We wrap the code to capture `module.exports` or `export default` result
            String wrappedCode = """
                var __exported = null;
                var module = { exports: null };
                """ + jsCode + """
                ;if (module.exports !== null) { __exported = module.exports; }
                ;if (typeof __exported === 'undefined' || __exported === null) {
                    // Try to find the last expression
                    __exported = eval('(function() { ' + arguments[0] + ' })()');
                }
                """;

            // Simpler approach: just eval the stripped JS, which should end with
            // an expression (the exported object). The stripTypeScript function
            // converts `export default {...}` to just `{...}` which is a valid
            // JS expression.
            Value result = ctx.eval("js", jsCode);

            // 4. If result is an object, use it directly
            Value captured;
            if (result != null && !result.isNull() && result.hasMembers()) {
                captured = result;
            } else {
                errors.add("The .quark.ts file must export a default object with name, namespace, and nodes fields.");
                return new SystemParseResult.Failure(errors);
            }

            // 5. Convert the GraalJS Value to a Java Map
            Map<String, Object> systemMap = valueToMap(captured);

            // 6. Validate and build SystemDefinition
            return buildSystemDefinition(systemMap, errors);

        } catch (Exception e) {
            log.error("Failed to evaluate .quark.ts file", e);
            errors.add("Evaluation error: " + e.getMessage());
            return new SystemParseResult.Failure(errors);
        }
    }

    /**
     * Convert a GraalJS Value to a Java Map.
     */
    @SuppressWarnings("unchecked")
    private Map<String, Object> valueToMap(Value value) {
        if (value == null || value.isNull()) {
            return Map.of();
        }
        if (value.hasMembers()) {
            Map<String, Object> result = new HashMap<>();
            for (String key : value.getMemberKeys()) {
                Value memberValue = value.getMember(key);
                result.put(key, valueToObject(memberValue));
            }
            return result;
        }
        return Map.of();
    }

    /**
     * Convert a GraalJS Value to a Java Object (recursively).
     *
     * <p>Note: array check happens BEFORE object check because GraalJS
     * arrays also report {@code hasMembers()==true} (for index-based access).
     * Checking {@code hasArrayElements()} first ensures arrays become
     * {@link List}s, not {@link java.util.Map}s with integer-string keys.
     */
    private Object valueToObject(Value value) {
        if (value == null || value.isNull()) {
            return null;
        }
        if (value.isString()) {
            return value.asString();
        }
        if (value.isNumber()) {
            // Preserve integer values when they fit, otherwise use double.
            // GraalJS returns long/double based on the source literal; we
            // try int first because Jackson/the SPI expects ints for
            // things like retry counts and maxSize.
            try {
                return value.asInt();
            } catch (Exception ignored) {
                return value.asDouble();
            }
        }
        if (value.isBoolean()) {
            return value.asBoolean();
        }
        if (value.hasArrayElements()) {
            List<Object> list = new ArrayList<>();
            for (long i = 0; i < value.getArraySize(); i++) {
                list.add(valueToObject(value.getArrayElement(i)));
            }
            return list;
        }
        if (value.hasMembers()) {
            return valueToMap(value);
        }
        return value.toString();
    }

    /**
     * Build a SystemDefinition from the parsed JavaScript object.
     */
    @SuppressWarnings("unchecked")
    private SystemParseResult buildSystemDefinition(Map<String, Object> map, List<String> errors) {
        String name = (String) map.get("name");
        if (name == null || name.isBlank()) {
            errors.add("Missing required field: name");
        }

        String namespaceStr = (String) map.get("namespace");
        if (namespaceStr == null || namespaceStr.isBlank()) {
            errors.add("Missing required field: namespace");
        }

        Map<String, Object> nodesMap = (Map<String, Object>) map.get("nodes");
        if (nodesMap == null || nodesMap.isEmpty()) {
            errors.add("Missing or empty field: nodes");
        }

        if (!errors.isEmpty()) {
            return new SystemParseResult.Failure(errors);
        }

        Namespace namespace = Namespace.of(namespaceStr);
        Map<String, NodeDefinition> nodes = new HashMap<>();

        for (Map.Entry<String, Object> entry : nodesMap.entrySet()) {
            String nodeName = entry.getKey();
            Map<String, Object> nodeMap = (Map<String, Object>) entry.getValue();

            String uses = (String) nodeMap.get("uses");
            if (uses == null || uses.isBlank()) {
                errors.add("Node '" + nodeName + "' is missing required field: uses");
                continue;
            }

            NodeUri uri;
            try {
                uri = NodeUri.parse(uses);
            } catch (Exception e) {
                errors.add("Node '" + nodeName + "' has invalid URI: " + uses + " — " + e.getMessage());
                continue;
            }

            Map<String, Object> configMap = new HashMap<>();
            List<String> listens = List.of();
            List<String> events = List.of();
            OnFailureConfig onFailure = null;
            String timeout = null;

            for (Map.Entry<String, Object> prop : nodeMap.entrySet()) {
                switch (prop.getKey()) {
                    case "uses" -> {}
                    case "listens" -> {
                        List<Object> listensList = (List<Object>) prop.getValue();
                        listens = listensList.stream().map(Object::toString).toList();
                    }
                    case "events" -> {
                        List<Object> eventsList = (List<Object>) prop.getValue();
                        events = eventsList.stream().map(Object::toString).toList();
                    }
                    case "onFailure" -> {
                        Map<String, Object> failureMap = (Map<String, Object>) prop.getValue();
                        int retry = ((Number) failureMap.get("retry")).intValue();
                        String routeTo = (String) failureMap.get("routeTo");
                        onFailure = new OnFailureConfig(retry, routeTo);
                    }
                    case "timeout" -> timeout = (String) prop.getValue();
                    case "labels", "annotations" -> {}
                    default -> configMap.put(prop.getKey(), prop.getValue());
                }
            }

            nodes.put(nodeName, new NodeDefinition(
                    nodeName, uri, NodeConfig.of(configMap),
                    listens, events, onFailure, timeout,
                    Labels.empty(), Annotations.empty()
            ));
        }

        if (!errors.isEmpty()) {
            return new SystemParseResult.Failure(errors);
        }

        String runtime = (String) map.get("runtime");
        if (runtime != null && !runtime.isBlank()
                && !runtime.equalsIgnoreCase("shared")
                && !runtime.equalsIgnoreCase("isolated")) {
            errors.add("Invalid runtime value: " + runtime + " (must be 'shared' or 'isolated')");
        }

        if (!errors.isEmpty()) {
            return new SystemParseResult.Failure(errors);
        }

        return new SystemParseResult.Success(new SystemDefinition(name, namespace, nodes, runtime));
    }

    /**
     * Strip TypeScript-specific syntax to produce valid JavaScript.
     *
     * <p>Handles: import removal, export removal, type annotation stripping,
     * interface/type declaration removal, `as` assertions, generics.
     *
     * <p>The key transformation: {@code export default {...};} becomes
     * {@code ({...})} — a parenthesized object literal, which is a valid
     * JS expression that evaluates to the object. Without the parens, a
     * leading `{` at statement position would be parsed as a block
     * statement (syntax error). Also handles the variant
     * {@code export default({...});} where the object is wrapped in
     * parens (common in TS codebases).
     */
    private String stripTypeScript(String ts) {
        String js = ts;

        // Remove import statements
        js = js.replaceAll("(?m)^import\\s+.*?;$", "");

        // Remove `export default ` — leaves the object literal expression.
        // Matches `export default ` (the canonical form) and
        // `export default(` (the wrapped form). We strip just the
        // `export default` keyword + whitespace, leaving the `{` or `(`.
        js = js.replaceAll("(?m)^export\\s+default\\s+", "");

        // Remove other `export ` keywords (named exports, re-exports).
        js = js.replaceAll("(?m)^export\\s+", "");

        // Remove interface declarations
        js = js.replaceAll("(?s)interface\\s+\\w+\\s*\\{.*?\\}", "");

        // Remove type declarations
        js = js.replaceAll("(?s)type\\s+\\w+\\s*=\\s*.*?;", "");

        // Remove type annotations on variables
        js = js.replaceAll("(\\w+)\\s*:\\s*[A-Za-z_<>,\\[\\]\\s|]+\\s*=", "$1 =");

        // Remove 'as Type' assertions
        js = js.replaceAll("\\s+as\\s+[A-Za-z_<>,\\[\\]\\s|]+", "");

        // Remove generic type parameters
        js = js.replaceAll("<[A-Za-z_<>,\\[\\]\\s|]+>", "");

        // Trim and strip any trailing semicolons (left over from
        // `export default {...};`) so wrapping in parens doesn't produce
        // `({...};)` which is a syntax error.
        js = js.trim();
        while (js.endsWith(";")) {
            js = js.substring(0, js.length() - 1).trim();
        }

        // Wrap the whole thing in parens so a leading `{...}` is parsed as
        // an object expression, not a block statement.
        js = "(" + js + ")";

        return js;
    }
}
