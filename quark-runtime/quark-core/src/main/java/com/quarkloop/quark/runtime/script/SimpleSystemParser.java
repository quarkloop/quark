package com.quarkloop.quark.runtime.script;

import com.quarkloop.quark.runtime.domain.config.NodeConfig;
import com.quarkloop.quark.runtime.domain.identity.Namespace;
import com.quarkloop.quark.runtime.domain.identity.NodeUri;
import com.quarkloop.quark.runtime.domain.metadata.Annotations;
import com.quarkloop.quark.runtime.domain.metadata.Labels;
import com.quarkloop.quark.runtime.domain.system.NodeDefinition;
import com.quarkloop.quark.runtime.domain.system.OnFailureConfig;
import com.quarkloop.quark.runtime.domain.system.SystemDefinition;
import jakarta.annotation.Priority;
import jakarta.enterprise.context.ApplicationScoped;
import jakarta.enterprise.inject.Alternative;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import java.util.ArrayList;
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
 * <h2>Implementation note</h2>
 *
 * <p>The previous version of this parser used a regex-based
 * {@code stripTypeScript()} pre-pass that tried to remove TypeScript
 * type annotations, interfaces, and generics. That pre-pass was
 * fundamentally broken — TypeScript syntax is not a regular language, so
 * a regex cannot distinguish a TS keyword from the same substring inside
 * a string literal, comment, or identifier (e.g. the
 * {@code (\\w+)\\s*:\\s*[A-Za-z_<>...]+\\s*=} pattern would happily
 * rewrite {@code mapping: \{} as {@code mapping = \{} if the value
 * matched the right shape).
 *
 * <p>The current implementation does <strong>not</strong> strip TypeScript
 * at all. Instead, it operates directly on the source code with brace /
 * string-aware scanners that know how to:
 * <ul>
 *   <li>Skip line comments ({@code // ...}) and block comments
 *       ({@code /* ... *\\/})</li>
 *   <li>Skip string literals (single, double, and template)</li>
 *   <li>Find matching braces for nested object literals</li>
 * </ul>
 *
 * <p>This is sufficient because the platform's {@code .quark.ts} files
 * are valid ECMAScript modules — they don't use TypeScript type
 * annotations on the system-definition object. If a user does add type
 * annotations, the parser will still find the structural fields (name,
 * namespace, nodes) but may mis-parse annotated values; in that case the
 * user should deploy in JVM mode where {@code GraalJsSystemParser}
 * provides full ESM evaluation.
 *
 * <p><b>Limitations:</b> This parser supports the subset of
 * {@code .quark.ts} syntax used by the example systems. It does NOT
 * support arbitrary JavaScript expressions, functions, loops, or
 * conditional logic. For full TypeScript evaluation, use JVM mode with
 * {@link GraalJsSystemParser}.
 */
@ApplicationScoped
@Alternative
@Priority(jakarta.interceptor.Interceptor.Priority.APPLICATION + 5)
public class SimpleSystemParser implements SystemParser {

    private static final Logger log = LoggerFactory.getLogger(SimpleSystemParser.class);

    /** Reserved node-level keys — these are NOT promoted to node config. */
    private static final Set<String> RESERVED_NODE_KEYS =
            Set.of("uses", "listens", "events", "onFailure", "timeout", "labels", "annotations");

    /** Match a top-level string field: {@code fieldName: "value"} or {@code fieldName: 'value'} */
    private static final Pattern STRING_FIELD = Pattern.compile(
            "(\\w+)\\s*:\\s*([\"'])([^\"']*)\\2");

    /** Match a string array: {@code ["a", "b", "c"]} */
    private static final Pattern STRING_ARRAY = Pattern.compile(
            "\\[([^\\]]*)\\]");

    /** Match a numeric value: {@code retry: 3} */
    private static final Pattern NUMERIC_FIELD = Pattern.compile(
            "(\\w+)\\s*:\\s*(\\d+)");

    @Override
    public SystemParseResult parse(String sourceCode) {
        if (sourceCode == null || sourceCode.isBlank()) {
            return new SystemParseResult.Failure(List.of("Source code is empty"));
        }

        List<String> errors = new ArrayList<>();

        // Strip comments before parsing — this simplifies the regexes
        // because we don't have to worry about matching inside comments.
        // We do NOT strip TypeScript type annotations because regex
        // cannot do that correctly (see class javadoc).
        String src = stripComments(sourceCode);

        try {
            // Extract top-level scalar fields
            String name = extractStringField(src, "name");
            String namespace = extractStringField(src, "namespace");
            String runtime = extractStringField(src, "runtime");

            if (name == null || name.isBlank()) {
                errors.add("Missing required field: name");
            }
            if (namespace == null || namespace.isBlank()) {
                errors.add("Missing required field: namespace");
            }
            if (!errors.isEmpty()) {
                return new SystemParseResult.Failure(errors);
            }

            if (runtime != null && !runtime.isBlank()
                    && !runtime.equalsIgnoreCase("shared")
                    && !runtime.equalsIgnoreCase("isolated")) {
                errors.add("Invalid runtime value: " + runtime + " (must be 'shared' or 'isolated')");
                return new SystemParseResult.Failure(errors);
            }

            // Extract nodes block
            Map<String, NodeDefinition> nodes = parseNodes(src, errors);
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
     * Strip line comments ({@code // ...}) and block comments
     * ({@code /* ... *\\/}). String literals are skipped so that
     * {@code //} or {@code /*} inside a string is not treated as a
     * comment marker.
     *
     * <p>The comment text is replaced with whitespace of the same length
     * so that character offsets in the original source are preserved —
     * this keeps error messages and brace-matching logic aligned.
     */
    static String stripComments(String source) {
        StringBuilder out = new StringBuilder(source.length());
        int i = 0;
        int len = source.length();
        boolean inString = false;
        char stringChar = 0;
        boolean inTemplate = false;
        // Template strings can be nested with ${...} expressions; we
        // approximate by treating the whole template as a string for
        // comment-stripping purposes. This is conservative but correct
        // for the .quark.ts system-definition shape.

        while (i < len) {
            char c = source.charAt(i);

            if (inString) {
                out.append(c);
                if (c == '\\' && i + 1 < len) {
                    out.append(source.charAt(i + 1));
                    i += 2;
                    continue;
                }
                if (c == stringChar) {
                    inString = false;
                }
                i++;
                continue;
            }

            if (inTemplate) {
                out.append(c);
                if (c == '\\' && i + 1 < len) {
                    out.append(source.charAt(i + 1));
                    i += 2;
                    continue;
                }
                if (c == '`') {
                    inTemplate = false;
                }
                i++;
                continue;
            }

            // Not in a string/template — check for comment starts
            if (c == '/' && i + 1 < len && source.charAt(i + 1) == '/') {
                // Line comment — skip to end of line (preserving newlines)
                while (i < len && source.charAt(i) != '\n') {
                    out.append(' ');
                    i++;
                }
                continue;
            }
            if (c == '/' && i + 1 < len && source.charAt(i + 1) == '*') {
                // Block comment — skip to */
                out.append(' ').append(' ');
                i += 2;
                while (i + 1 < len && !(source.charAt(i) == '*' && source.charAt(i + 1) == '/')) {
                    out.append(source.charAt(i) == '\n' ? '\n' : ' ');
                    i++;
                }
                if (i + 1 < len) {
                    out.append(' ').append(' ');
                    i += 2;
                } else {
                    // Unterminated block comment — copy rest
                    while (i < len) {
                        out.append(' ');
                        i++;
                    }
                }
                continue;
            }

            // Check for string start
            if (c == '"' || c == '\'') {
                inString = true;
                stringChar = c;
                out.append(c);
                i++;
                continue;
            }
            if (c == '`') {
                inTemplate = true;
                out.append(c);
                i++;
                continue;
            }

            out.append(c);
            i++;
        }
        return out.toString();
    }

    /**
     * Extract a string field value: {@code fieldName: "value"}.
     * Returns the value or {@code null} if not found.
     */
    private String extractStringField(String source, String fieldName) {
        Pattern p = Pattern.compile(fieldName + "\\s*:\\s*([\"'])([^\"']*)\\1");
        Matcher m = p.matcher(source);
        if (m.find()) {
            return m.group(2);
        }
        return null;
    }

    /**
     * Parse the {@code nodes: { ... }} block.
     */
    private Map<String, NodeDefinition> parseNodes(String source, List<String> errors) {
        Map<String, NodeDefinition> nodes = new LinkedHashMap<>();

        int nodesIdx = indexOfTopLevelKey(source, "nodes");
        if (nodesIdx < 0) return nodes;

        int braceStart = source.indexOf('{', nodesIdx);
        if (braceStart < 0) return nodes;

        int braceEnd = findMatchingBrace(source, braceStart);
        if (braceEnd < 0) return nodes;

        String nodesBlock = source.substring(braceStart + 1, braceEnd);

        // Walk through the nodes block tracking brace depth, finding
        // node-name : { ... } pairs at depth 0.
        int i = 0;
        while (i < nodesBlock.length()) {
            // Skip whitespace and commas
            while (i < nodesBlock.length() && (Character.isWhitespace(nodesBlock.charAt(i)) || nodesBlock.charAt(i) == ',')) {
                i++;
            }
            if (i >= nodesBlock.length()) break;

            // Find the next ':' (the key-value separator at depth 0)
            int colonIdx = -1;
            int depth = 0;
            boolean inStr = false;
            char strCh = 0;
            for (int j = i; j < nodesBlock.length(); j++) {
                char c = nodesBlock.charAt(j);
                if (inStr) {
                    if (c == '\\' && j + 1 < nodesBlock.length()) { j++; continue; }
                    if (c == strCh) inStr = false;
                    continue;
                }
                if (c == '"' || c == '\'') { inStr = true; strCh = c; continue; }
                if (c == '{' || c == '[' || c == '(') { depth++; continue; }
                if (c == '}' || c == ']' || c == ')') { depth--; continue; }
                if (c == ':' && depth == 0) {
                    colonIdx = j;
                    break;
                }
            }
            if (colonIdx < 0) break;

            String name = nodesBlock.substring(i, colonIdx).trim();
            // Strip surrounding quotes if the key was a string literal
            if (name.length() >= 2 && ((name.startsWith("\"") && name.endsWith("\""))
                    || (name.startsWith("'") && name.endsWith("'")))) {
                name = name.substring(1, name.length() - 1);
            }

            int valueStart = colonIdx + 1;
            // Skip whitespace
            while (valueStart < nodesBlock.length() && Character.isWhitespace(nodesBlock.charAt(valueStart))) {
                valueStart++;
            }
            if (valueStart >= nodesBlock.length() || nodesBlock.charAt(valueStart) != '{') {
                // Skip to next comma at depth 0
                i = skipToNextTopLevelComma(nodesBlock, valueStart);
                continue;
            }

            int nodeEnd = findMatchingBrace(nodesBlock, valueStart);
            if (nodeEnd < 0) break;
            String nodeBlock = nodesBlock.substring(valueStart + 1, nodeEnd);

            NodeDefinition nodeDef = parseNodeDefinition(name, nodeBlock, errors);
            if (nodeDef != null) {
                nodes.put(name, nodeDef);
            }

            i = nodeEnd + 1;
        }

        return nodes;
    }

    /**
     * Find the index of a top-level identifier key (e.g. {@code nodes} in
     * {@code nodes: \{...\}}). Returns the position of the identifier, or
     * {@code -1} if not found.
     */
    private static int indexOfTopLevelKey(String source, String key) {
        Pattern p = Pattern.compile("\\b" + Pattern.quote(key) + "\\s*:");
        Matcher m = p.matcher(source);
        return m.find() ? m.start() : -1;
    }

    /**
     * Skip from {@code start} to the next comma at depth 0, returning
     * the position just after that comma (or the end of the string).
     */
    private static int skipToNextTopLevelComma(String source, int start) {
        int depth = 0;
        boolean inStr = false;
        char strCh = 0;
        for (int i = start; i < source.length(); i++) {
            char c = source.charAt(i);
            if (inStr) {
                if (c == '\\' && i + 1 < source.length()) { i++; continue; }
                if (c == strCh) inStr = false;
                continue;
            }
            if (c == '"' || c == '\'' || c == '`') { inStr = true; strCh = c; continue; }
            if (c == '{' || c == '[' || c == '(') { depth++; continue; }
            if (c == '}' || c == ']' || c == ')') { depth--; continue; }
            if (c == ',' && depth == 0) return i + 1;
        }
        return source.length();
    }

    /**
     * Parse a single node definition from its block (the text between
     * the outer braces).
     */
    private NodeDefinition parseNodeDefinition(String nodeName, String block, List<String> errors) {
        Matcher usesMatcher = Pattern.compile("uses\\s*:\\s*([\"'])([^\"']+)\\1").matcher(block);
        if (!usesMatcher.find()) {
            errors.add("Node '" + nodeName + "' is missing required field: uses");
            return null;
        }

        String usesStr = usesMatcher.group(2);
        NodeUri uri;
        try {
            uri = NodeUri.parse(usesStr);
        } catch (Exception e) {
            errors.add("Node '" + nodeName + "' has invalid URI: " + usesStr + " — " + e.getMessage());
            return null;
        }

        List<String> listens = parseStringArray(block, "listens");
        List<String> events = parseStringArray(block, "events");
        OnFailureConfig onFailure = parseOnFailure(block);
        String timeout = extractStringField(block, "timeout");
        Map<String, Object> config = parseConfig(block);

        return new NodeDefinition(
                nodeName, uri, NodeConfig.of(config),
                listens, events, onFailure, timeout,
                Labels.empty(), Annotations.empty()
        );
    }

    /**
     * Parse a string array field: {@code fieldName: ["a", "b", "c"]}.
     */
    private List<String> parseStringArray(String source, String fieldName) {
        Pattern p = Pattern.compile(fieldName + "\\s*:\\s*\\[([^\\]]*)\\]");
        Matcher m = p.matcher(source);
        if (!m.find()) return List.of();

        String arrayContent = m.group(1);
        List<String> result = new ArrayList<>();
        Matcher itemMatcher = Pattern.compile("[\"']([^\"']+)[\"']").matcher(arrayContent);
        while (itemMatcher.find()) {
            result.add(itemMatcher.group(1));
        }
        return result;
    }

    /**
     * Parse {@code onFailure: { retry: 3, routeTo: "writer" }}.
     */
    private OnFailureConfig parseOnFailure(String source) {
        Pattern p = Pattern.compile("onFailure\\s*:\\s*\\{([^}]*)\\}", Pattern.DOTALL);
        Matcher m = p.matcher(source);
        if (!m.find()) return null;

        String content = m.group(1);
        String routeTo = null;
        int retry = 0;

        Matcher routeMatcher = STRING_FIELD.matcher(content);
        while (routeMatcher.find()) {
            if (routeMatcher.group(1).equals("routeTo")) {
                routeTo = routeMatcher.group(3);
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
     * Parse all config fields — every string or numeric field that is
     * NOT a reserved keyword ({@code uses}, {@code listens}, etc.).
     *
     * <p>For nested object values (like {@code mapping: \{ "a": "b" \}}),
     * the value is preserved as a string for round-tripping. This is
     * sufficient for the simple key→key mapping shapes used by the
     * example nodes; richer object parsing would require a real
     * TypeScript/JSON parser.
     */
    private Map<String, Object> parseConfig(String source) {
        Map<String, Object> config = new LinkedHashMap<>();

        // String fields
        Matcher stringMatcher = STRING_FIELD.matcher(source);
        while (stringMatcher.find()) {
            String key = stringMatcher.group(1);
            if (!RESERVED_NODE_KEYS.contains(key)) {
                config.put(key, stringMatcher.group(3));
            }
        }

        // Numeric fields
        Matcher numericMatcher = NUMERIC_FIELD.matcher(source);
        while (numericMatcher.find()) {
            String key = numericMatcher.group(1);
            if (!RESERVED_NODE_KEYS.contains(key) && !config.containsKey(key)) {
                config.put(key, Integer.parseInt(numericMatcher.group(2)));
            }
        }

        // Object-valued fields (e.g. mapping: { "cpu": "cpuUsage" })
        // We scan for `key: {` at depth 0 and capture the inner block,
        // then recursively parse its string fields into a nested Map.
        int i = 0;
        while (i < source.length()) {
            // Find next identifier
            while (i < source.length() && !Character.isJavaIdentifierStart(source.charAt(i))) i++;
            if (i >= source.length()) break;
            int idStart = i;
            while (i < source.length() && Character.isJavaIdentifierPart(source.charAt(i))) i++;
            String key = source.substring(idStart, i);
            // Skip whitespace
            while (i < source.length() && Character.isWhitespace(source.charAt(i))) i++;
            if (i >= source.length() || source.charAt(i) != ':') continue;
            i++; // skip ':'
            while (i < source.length() && Character.isWhitespace(source.charAt(i))) i++;
            if (i >= source.length() || source.charAt(i) != '{') continue;
            int objEnd = findMatchingBrace(source, i);
            if (objEnd < 0) break;
            if (!RESERVED_NODE_KEYS.contains(key) && !config.containsKey(key)) {
                String inner = source.substring(i + 1, objEnd);
                Map<String, Object> nested = parseNestedObject(inner);
                if (!nested.isEmpty()) {
                    config.put(key, nested);
                }
            }
            i = objEnd + 1;
        }

        return config;
    }

    /**
     * Parse the inner content of a nested object (text between
     * matching braces) into a {@link Map}. Only string values are
     * extracted — numeric / nested-object values inside the nested
     * object are ignored for simplicity. This is sufficient for the
     * platform's example nodes (e.g. {@code mapping: \{ "cpu": "cpuUsage" \}}).
     */
    private Map<String, Object> parseNestedObject(String inner) {
        Map<String, Object> map = new LinkedHashMap<>();
        Matcher m = Pattern.compile("([\"'])([^\"']+)\\1\\s*:\\s*([\"'])([^\"']*)\\3").matcher(inner);
        while (m.find()) {
            map.put(m.group(2), m.group(4));
        }
        // Also match unquoted-key -> quoted-value
        Matcher m2 = Pattern.compile("(\\w+)\\s*:\\s*([\"'])([^\"']*)\\2").matcher(inner);
        while (m2.find()) {
            map.putIfAbsent(m2.group(1), m2.group(3));
        }
        return map;
    }

    /**
     * Find the matching closing brace for the opening brace at the
     * given position. String literals are skipped so braces inside
     * strings don't affect depth counting.
     */
    private static int findMatchingBrace(String source, int openPos) {
        int depth = 0;
        boolean inString = false;
        char stringChar = 0;
        boolean inTemplate = false;

        for (int i = openPos; i < source.length(); i++) {
            char c = source.charAt(i);

            if (inString) {
                if (c == '\\' && i + 1 < source.length()) { i++; continue; }
                if (c == stringChar) inString = false;
                continue;
            }
            if (inTemplate) {
                if (c == '\\' && i + 1 < source.length()) { i++; continue; }
                if (c == '`') inTemplate = false;
                continue;
            }

            if (c == '"' || c == '\'') { inString = true; stringChar = c; continue; }
            if (c == '`') { inTemplate = true; continue; }
            if (c == '{') depth++;
            else if (c == '}') {
                depth--;
                if (depth == 0) return i;
            }
        }
        return -1;
    }
}
