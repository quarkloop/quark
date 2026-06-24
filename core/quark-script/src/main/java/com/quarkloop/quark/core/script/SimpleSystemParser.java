package com.quarkloop.quark.core.script;

import com.quarkloop.quark.core.domain.config.NodeConfig;
import com.quarkloop.quark.core.domain.identity.Namespace;
import com.quarkloop.quark.core.domain.identity.NodeUri;
import com.quarkloop.quark.core.domain.metadata.Annotations;
import com.quarkloop.quark.core.domain.metadata.Labels;
import com.quarkloop.quark.core.domain.system.NodeDefinition;
import com.quarkloop.quark.core.domain.system.OnFailureConfig;
import com.quarkloop.quark.core.domain.system.SystemDefinition;
import jakarta.annotation.Priority;
import jakarta.enterprise.context.ApplicationScoped;
import jakarta.enterprise.inject.Alternative;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import java.util.ArrayList;
import java.util.HashMap;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import java.util.Set;
import java.util.regex.Matcher;
import java.util.regex.Pattern;

/**
 * A lightweight TypeScript parser that does NOT use GraalJS.
 *
 * <p>This parser is used in GraalVM native-image mode where GraalJS/Truffle
 * is not available (Truffle languages are incompatible with native-image's
 * closed-world analysis). It parses {@code .quark.ts} files using regex
 * pattern matching to extract the system name, namespace, runtime mode,
 * and node definitions.
 *
 * <p>This bean is marked as an {@link Alternative} with
 * {@link Priority} {@code APPLICATION+5} — lower than the default
 * {@code APPLICATION} priority of {@link GraalJsSystemParser}. This means:
 * <ul>
 *   <li>In JVM mode: {@code GraalJsSystemParser} wins (it's not an
 *       alternative, so it has default priority and is preferred).</li>
 *   <li>In native mode: {@code GraalJsSystemParser} is excluded from the
 *       native image (GraalJS JARs are pruned), so this alternative is
 *       the only available implementation.</li>
 * </ul>
 *
 * <p><b>Limitations:</b> This parser supports the subset of {@code .quark.ts}
 * syntax used by the example systems. It does NOT support arbitrary
 * JavaScript expressions, functions, loops, or conditional logic.
 * For full TypeScript evaluation, use JVM mode with
 * {@link GraalJsSystemParser}.
 */
@ApplicationScoped
@Alternative
@Priority(jakarta.interceptor.Interceptor.Priority.APPLICATION + 5)
public class SimpleSystemParser implements SystemParser {

    private static final Logger log = LoggerFactory.getLogger(SimpleSystemParser.class);

    // Match: name: "value"  or  name: 'value'
    private static final Pattern STRING_FIELD = Pattern.compile(
            "(\\w+)\\s*:\\s*[\"']([^\"']*)[\"']");

    // Match: uses: "category/implementation:version"
    private static final Pattern USES_FIELD = Pattern.compile(
            "uses\\s*:\\s*[\"']([^\"']+)[\"']");

    // Match: interval: "1s" or similar string config values
    private static final Pattern CONFIG_STRING_FIELD = Pattern.compile(
            "(\\w+)\\s*:\\s*[\"']([^\"']*)[\"']");

    // Match: listens: ["a", "b"]
    private static final Pattern STRING_ARRAY = Pattern.compile(
            "(\\w+)\\s*:\\s*\\[([^\\]]*)\\]");

    // Match: onFailure: { retry: 3, routeTo: "writer" }
    private static final Pattern ON_FAILURE = Pattern.compile(
            "onFailure\\s*:\\s*\\{([^}]*)\\}", Pattern.DOTALL);

    // Match: maxSize: 100 (numeric config)
    private static final Pattern NUMERIC_FIELD = Pattern.compile(
            "(\\w+)\\s*:\\s*(\\d+)");

    @Override
    public SystemParseResult parse(String sourceCode) {
        if (sourceCode == null || sourceCode.isBlank()) {
            return new SystemParseResult.Failure(List.of("Source code is empty"));
        }

        List<String> errors = new ArrayList<>();
        String js = stripTypeScript(sourceCode);

        try {
            // Extract top-level fields
            String name = extractStringField(js, "name");
            String namespace = extractStringField(js, "namespace");
            String runtime = extractStringField(js, "runtime");

            if (name == null || name.isBlank()) {
                errors.add("Missing required field: name");
            }
            if (namespace == null || namespace.isBlank()) {
                errors.add("Missing required field: namespace");
            }

            if (!errors.isEmpty()) {
                return new SystemParseResult.Failure(errors);
            }

            // Validate runtime field
            if (runtime != null && !runtime.isBlank()
                    && !runtime.equalsIgnoreCase("shared")
                    && !runtime.equalsIgnoreCase("isolated")) {
                errors.add("Invalid runtime value: " + runtime + " (must be 'shared' or 'isolated')");
                return new SystemParseResult.Failure(errors);
            }

            // Extract nodes block
            Map<String, NodeDefinition> nodes = parseNodes(js, errors);
            if (!errors.isEmpty()) {
                return new SystemParseResult.Failure(errors);
            }

            if (nodes.isEmpty()) {
                errors.add("Missing or empty field: nodes");
                return new SystemParseResult.Failure(errors);
            }

            return new SystemParseResult.Success(
                    new SystemDefinition(name, Namespace.of(namespace), nodes, runtime));

        } catch (Exception e) {
            errors.add("Parse error: " + e.getMessage());
            return new SystemParseResult.Failure(errors);
        }
    }

    /**
     * Extract a string field value from the source.
     * Matches: fieldName: "value" or fieldName: 'value'
     */
    private String extractStringField(String source, String fieldName) {
        Pattern p = Pattern.compile(fieldName + "\\s*:\\s*[\"']([^\"']*)[\"']");
        Matcher m = p.matcher(source);
        if (m.find()) {
            return m.group(1);
        }
        return null;
    }

    /**
     * Parse the nodes block from the source.
     *
     * <p>Only matches node definitions at depth 1 inside the {@code nodes: { ... }}
     * block. Nested blocks like {@code onFailure: { ... }} inside a node
     * definition are NOT treated as separate nodes — they're parsed as part
     * of the containing node.
     */
    private Map<String, NodeDefinition> parseNodes(String source, List<String> errors) {
        Map<String, NodeDefinition> nodes = new LinkedHashMap<>();

        // Find the nodes: { ... } block
        int nodesIdx = source.indexOf("nodes:");
        if (nodesIdx < 0) return nodes;

        // Find the opening brace of the nodes object
        int braceStart = source.indexOf('{', nodesIdx);
        if (braceStart < 0) return nodes;

        // Find the matching closing brace
        int braceEnd = findMatchingBrace(source, braceStart);
        if (braceEnd < 0) return nodes;

        String nodesBlock = source.substring(braceStart + 1, braceEnd);

        // Walk through the nodes block tracking brace depth.
        // Only match `name: {` at depth 0 (direct children of the nodes block).
        List<String> nodeNames = new ArrayList<>();
        List<String> nodeBlocks = new ArrayList<>();

        int depth = 0;
        boolean inString = false;
        char stringChar = 0;
        int i = 0;
        while (i < nodesBlock.length()) {
            char c = nodesBlock.charAt(i);

            if (inString) {
                if (c == stringChar && (i == 0 || nodesBlock.charAt(i - 1) != '\\')) {
                    inString = false;
                }
                i++;
                continue;
            }

            if (c == '"' || c == '\'') {
                inString = true;
                stringChar = c;
                i++;
                continue;
            }

            if (c == '{') {
                if (depth == 0) {
                    // We're at the opening brace of a node definition.
                    // Find the node name by looking backwards from i.
                    String before = nodesBlock.substring(0, i).trim();
                    int colonIdx = before.lastIndexOf(':');
                    if (colonIdx >= 0) {
                        String name = before.substring(colonIdx).replaceAll("[^\\w]", "");
                        // Extract just the identifier after the last non-word char before ':'
                        String[] parts = before.substring(0, colonIdx).trim().split("[^\\w]+");
                        name = parts.length > 0 ? parts[parts.length - 1] : "";
                        if (!name.isEmpty()) {
                            nodeNames.add(name);
                            // Find the matching closing brace
                            int nodeEnd = findMatchingBrace(nodesBlock, i);
                            if (nodeEnd > i) {
                                nodeBlocks.add(nodesBlock.substring(i + 1, nodeEnd));
                                i = nodeEnd + 1;
                                depth = 0;
                                continue;
                            }
                        }
                    }
                }
                depth++;
            } else if (c == '}') {
                depth--;
            }
            i++;
        }

        for (int j = 0; j < nodeNames.size(); j++) {
            NodeDefinition nodeDef = parseNodeDefinition(nodeNames.get(j), nodeBlocks.get(j), errors);
            if (nodeDef != null) {
                nodes.put(nodeNames.get(j), nodeDef);
            }
        }

        return nodes;
    }

    /**
     * Parse a single node definition from its block.
     */
    private NodeDefinition parseNodeDefinition(String nodeName, String block, List<String> errors) {
        // Extract uses
        Matcher usesMatcher = USES_FIELD.matcher(block);
        if (!usesMatcher.find()) {
            errors.add("Node '" + nodeName + "' is missing required field: uses");
            return null;
        }

        String usesStr = usesMatcher.group(1);
        NodeUri uri;
        try {
            uri = NodeUri.parse(usesStr);
        } catch (Exception e) {
            errors.add("Node '" + nodeName + "' has invalid URI: " + usesStr + " — " + e.getMessage());
            return null;
        }

        // Extract listens
        List<String> listens = parseStringArray(block, "listens");

        // Extract events
        List<String> events = parseStringArray(block, "events");

        // Extract onFailure
        OnFailureConfig onFailure = parseOnFailure(block);

        // Extract timeout
        String timeout = extractStringField(block, "timeout");

        // Extract config (all unrecognized string/numeric fields)
        Map<String, Object> config = parseConfig(block);

        return new NodeDefinition(
                nodeName, uri, NodeConfig.of(config),
                listens, events, onFailure, timeout,
                Labels.empty(), Annotations.empty()
        );
    }

    /**
     * Parse a string array field: fieldName: ["a", "b", "c"]
     */
    private List<String> parseStringArray(String source, String fieldName) {
        Pattern p = Pattern.compile(fieldName + "\\s*:\\s*\\[([^\\]]*)\\]");
        Matcher m = p.matcher(source);
        if (!m.find()) return List.of();

        String arrayContent = m.group(1);
        List<String> result = new ArrayList<>();
        Pattern itemPattern = Pattern.compile("[\"']([^\"']+)[\"']");
        Matcher itemMatcher = itemPattern.matcher(arrayContent);
        while (itemMatcher.find()) {
            result.add(itemMatcher.group(1));
        }
        return result;
    }

    /**
     * Parse onFailure: { retry: 3, routeTo: "writer" }
     */
    private OnFailureConfig parseOnFailure(String source) {
        Matcher m = ON_FAILURE.matcher(source);
        if (!m.find()) return null;

        String content = m.group(1);
        String routeTo = null;
        int retry = 0;

        Matcher routeMatcher = STRING_FIELD.matcher(content);
        while (routeMatcher.find()) {
            if (routeMatcher.group(1).equals("routeTo")) {
                routeTo = routeMatcher.group(2);
            }
        }

        Matcher retryMatcher = NUMERIC_FIELD.matcher(content);
        while (retryMatcher.find()) {
            if (retryMatcher.group(1).equals("retry")) {
                retry = Integer.parseInt(retryMatcher.group(2));
            }
        }

        return new OnFailureConfig(retry, routeTo);
    }

    /**
     * Parse config fields (all string/numeric fields that aren't reserved keywords).
     */
    private Map<String, Object> parseConfig(String source) {
        Map<String, Object> config = new HashMap<>();
        Set<String> reserved = Set.of("uses", "listens", "events", "onFailure", "timeout", "labels", "annotations");

        // String fields
        Matcher stringMatcher = CONFIG_STRING_FIELD.matcher(source);
        while (stringMatcher.find()) {
            String key = stringMatcher.group(1);
            if (!reserved.contains(key)) {
                config.put(key, stringMatcher.group(2));
            }
        }

        // Numeric fields
        Matcher numericMatcher = NUMERIC_FIELD.matcher(source);
        while (numericMatcher.find()) {
            String key = numericMatcher.group(1);
            if (!reserved.contains(key) && !config.containsKey(key)) {
                config.put(key, Integer.parseInt(numericMatcher.group(2)));
            }
        }

        return config;
    }

    /**
     * Find the matching closing brace for the opening brace at the given position.
     */
    private int findMatchingBrace(String source, int openPos) {
        int depth = 0;
        boolean inString = false;
        char stringChar = 0;

        for (int i = openPos; i < source.length(); i++) {
            char c = source.charAt(i);

            if (inString) {
                if (c == stringChar && source.charAt(i - 1) != '\\') {
                    inString = false;
                }
                continue;
            }

            if (c == '"' || c == '\'') {
                inString = true;
                stringChar = c;
            } else if (c == '{') {
                depth++;
            } else if (c == '}') {
                depth--;
                if (depth == 0) return i;
            }
        }
        return -1;
    }

    /**
     * Strip TypeScript syntax (same logic as GraalJsSystemParser).
     */
    private String stripTypeScript(String ts) {
        String js = ts;
        js = js.replaceAll("(?m)^import\\s+.*?;$", "");
        js = js.replaceAll("(?m)^export\\s+default\\s+", "");
        js = js.replaceAll("(?m)^export\\s+", "");
        js = js.replaceAll("(?s)interface\\s+\\w+\\s*\\{.*?\\}", "");
        js = js.replaceAll("(?s)type\\s+\\w+\\s*=\\s*.*?;", "");
        js = js.replaceAll("(\\w+)\\s*:\\s*[A-Za-z_<>,\\[\\]\\s|]+\\s*=", "$1 =");
        js = js.replaceAll("\\s+as\\s+[A-Za-z_<>,\\[\\]\\s|]+", "");
        js = js.replaceAll("<[A-Za-z_<>,\\[\\]\\s|]+>", "");
        return js;
    }
}
