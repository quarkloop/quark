package com.quarkloop.quark.runtime.script;

import com.quarkloop.quark.runtime.domain.config.NodeConfig;
import com.quarkloop.quark.runtime.domain.identity.Namespace;
import com.quarkloop.quark.runtime.domain.identity.NodeUri;
import com.quarkloop.quark.runtime.domain.metadata.Annotations;
import com.quarkloop.quark.runtime.domain.metadata.Labels;
import com.quarkloop.quark.runtime.domain.system.NodeDefinition;
import com.quarkloop.quark.runtime.domain.system.OnFailureConfig;
import com.quarkloop.quark.runtime.domain.system.SystemDefinition;
import com.quarkloop.quark.runtime.script.SystemParseResult;
import com.quarkloop.quark.runtime.script.SystemParser;
import jakarta.enterprise.context.ApplicationScoped;
import org.graalvm.polyglot.Context;
import org.graalvm.polyglot.Engine;
import org.graalvm.polyglot.HostAccess;
import org.graalvm.polyglot.Source;
import org.graalvm.polyglot.Value;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import java.util.ArrayList;
import java.util.HashMap;
import java.util.List;
import java.util.Map;

/**
 * GraalJS-based parser for {@code .quark.ts} files.
 *
 * <p>Evaluates the user's TypeScript/JavaScript source code as an
 * <em>ECMAScript Module</em> in a sandboxed GraalJS context. The user's
 * code uses {@code export default { ... }} to expose the system
 * configuration object. The parser extracts this object via GraalJS's
 * polyglot API and converts it to a Java {@link SystemDefinition}.
 *
 * <p>This parser is the default in JVM mode. In native-image mode, the
 * {@code SimpleSystemParser} (in {@code core/quark-script}) is used
 * instead because GraalJS/Truffle is not compatible with GraalVM
 * native-image's closed-world analysis.
 *
 * <h2>TypeScript handling</h2>
 *
 * <p>GraalJS Community Edition (24.1.x) does not natively parse
 * TypeScript — there is no {@code js.typescript} option (see
 * <a href="https://github.com/oracle/graaljs/issues/784">graaljs#784</a>).
 * The previous hand-rolled regex-based {@code stripTypeScript()} was
 * fundamentally broken (it would, e.g., eat the words {@code "as JSON"}
 * from comments, and clobber {@code export default} inside
 * {@code __quark_export}). See {@link TypeScriptNodeFactory} for full
 * background.
 *
 * <p>The current approach evaluates the source directly as an ESM module
 * using GraalJS's native module parser. This works because the
 * {@code .quark.ts} files used in this platform are valid ECMAScript
 * modules — they use {@code export default { ... }} without any actual
 * TypeScript type annotations.
 *
 * <p>The sandbox:
 * <ul>
 *   <li>No host access (can't call Java methods)</li>
 *   <li>No file I/O, no threads, no processes, no native access</li>
 *   <li>No environment variable access</li>
 * </ul>
 */
@ApplicationScoped
public class GraalJsSystemParser implements SystemParser {

    private static final Logger log = LoggerFactory.getLogger(GraalJsSystemParser.class);

    /**
     * MIME type that triggers GraalJS's ECMAScript Module parser.
     */
    static final String ESM_MIME_TYPE = "application/javascript+module";

    /**
     * Force Quarkus's build-time bytecode analysis to detect direct references
     * to classes in {@code truffle-api.jar} and {@code regex.jar}. Without
     * these references, Quarkus prunes those jars from the runtime, causing
     * {@code NoClassDefFoundError: org/graalvm/polyglot/impl/AbstractPolyglotImpl}
     * when {@link Engine#newBuilder()} tries to load the polyglot implementation
     * via ServiceLoader.
     */
    private static void ensureGraalReferences() {
        //noinspection ResultOfMethodCallIgnored
        GraalRuntimeReferences.TRUFFLE_LANGUAGE.getName();
        //noinspection ResultOfMethodCallIgnored
        GraalRuntimeReferences.POLYGLOT_IMPL.getName();
        //noinspection ResultOfMethodCallIgnored
        GraalRuntimeReferences.REGEX_LANGUAGE.getName();
    }

    private volatile Engine engine;

    private Engine getEngine() {
        Engine local = engine;
        if (local != null) {
            return local;
        }
        synchronized (this) {
            if (engine == null) {
                ensureGraalReferences();
                Thread current = Thread.currentThread();
                ClassLoader original = current.getContextClassLoader();
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

        // Create sandboxed GraalJS context.
        // js.esm-eval-returns-exports=true causes Context.eval(Source) of an
        // ESM Source to return the module namespace object, whose 'default'
        // member is the value of `export default { ... }`.
        try (Context ctx = Context.newBuilder("js")
                .engine(getEngine())
                .allowHostAccess(HostAccess.NONE)
                .allowHostClassLookup(name -> false)
                .allowIO(false)
                .allowCreateThread(false)
                .allowCreateProcess(false)
                .allowNativeAccess(false)
                .allowEnvironmentAccess(org.graalvm.polyglot.EnvironmentAccess.NONE)
                .allowExperimentalOptions(true)
                .option("js.esm-eval-returns-exports", "true")
                .build()) {

            // Evaluate as an ECMAScript Module. The Source's MIME type tells
            // GraalJS to use the module parser, which natively understands
            // `export default { ... }`. No regex stripping needed.
            Source src = Source.newBuilder("js", sourceCode, "system.quark.mjs")
                    .mimeType(ESM_MIME_TYPE)
                    .buildLiteral();

            Value moduleNamespace;
            try {
                moduleNamespace = ctx.eval(src);
            } catch (org.graalvm.polyglot.PolyglotException pe) {
                log.error("Failed to evaluate .quark.ts file as ESM module", pe);
                errors.add("Evaluation error: " + pe.getMessage());
                return new SystemParseResult.Failure(errors);
            }

            if (moduleNamespace == null || moduleNamespace.isNull()) {
                errors.add("GraalJS returned a null module namespace. "
                        + "The .quark.ts file must end with `export default { ... };`.");
                return new SystemParseResult.Failure(errors);
            }

            Value captured = moduleNamespace.getMember("default");
            if (captured == null || captured.isNull() || !captured.hasMembers()) {
                errors.add("The .quark.ts file must export a default object with "
                        + "name, namespace, and nodes fields, e.g. "
                        + "`export default { name: \"...\", namespace: \"...\", nodes: { ... } };`.");
                return new SystemParseResult.Failure(errors);
            }

            // Convert the GraalJS Value to a Java Map
            Map<String, Object> systemMap = valueToMap(captured);

            // Validate and build SystemDefinition
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
}
