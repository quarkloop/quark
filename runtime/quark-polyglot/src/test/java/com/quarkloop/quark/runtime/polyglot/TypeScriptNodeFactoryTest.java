package com.quarkloop.quark.runtime.polyglot;

import com.quarkloop.quark.runtime.domain.config.NodeConfig;
import com.quarkloop.quark.runtime.domain.spi.NodeProvider;
import com.quarkloop.quark.runtime.domain.spi.QuarkMessage;
import com.quarkloop.quark.runtime.domain.spi.QuarkPublisher;
import org.junit.jupiter.api.Test;

import java.util.HashMap;
import java.util.List;
import java.util.Map;
import java.util.concurrent.atomic.AtomicReference;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;

/**
 * Unit tests for {@link TypeScriptNodeFactory}.
 *
 * <p>These tests verify that the factory can correctly evaluate the
 * platform's TypeScript node sources using GraalJS's native ECMAScript
 * Module support (no regex stripping).
 *
 * <p>The tests cover:
 * <ul>
 *   <li>Each of the production TS node sources (stdout, json-parse,
 *       map, conditional) — verifying they load and execute.</li>
 *   <li>A regression case: comments containing the substrings "type"
 *       and "as" (which the old regex-based stripper would corrupt).</li>
 *   <li>The error case: a source file without {@code export default}.</li>
 *   <li>The error case: a source file with a syntax error.</li>
 * </ul>
 */
class TypeScriptNodeFactoryTest {

    private static final String STDOUT_URI = "quark/log/console/stdout:v1";
    private static final String STDOUT_SRC =
            "// quark/log/console/stdout:v1\n" +
            "// Writes each incoming message payload to standard output as JSON.\n" +
            "\n" +
            "export default {\n" +
            "    onMessage: function(message, publisher) {\n" +
            "        const payload = message.getPayload();\n" +
            "        console.log(JSON.stringify(payload));\n" +
            "    }\n" +
            "};\n";

    private static final String JSON_PARSE_URI = "quark/codec/json/parse:v1";
    private static final String JSON_PARSE_SRC =
            "// quark/codec/json/parse:v1\n" +
            "// Parses a JSON string from the message payload into an object.\n" +
            "\n" +
            "export default {\n" +
            "    onMessage: function(message, publisher) {\n" +
            "        const field = config.getString(\"field\", \"data\");\n" +
            "        const strict = config.getBoolean(\"strict\", false);\n" +
            "        const payload = message.getPayload();\n" +
            "        const raw = payload[field] || payload[\"data\"] || JSON.stringify(payload);\n" +
            "        try {\n" +
            "            const parsed = typeof raw === \"string\" ? JSON.parse(raw) : raw;\n" +
            "            publisher.publish(\"parsed\", { data: parsed, source: message.getSubject() });\n" +
            "        } catch (e) {\n" +
            "            if (strict) throw e;\n" +
            "            publisher.publish(\"error\", { error: e.message, input: raw });\n" +
            "        }\n" +
            "    }\n" +
            "};\n";

    private static final String MAP_URI = "quark/data/shape/map:v1";
    private static final String MAP_SRC =
            "// quark/data/shape/map:v1\n" +
            "// Declaratively maps fields from the input payload to a new output shape.\n" +
            "\n" +
            "export default {\n" +
            "    onMessage: function(message, publisher) {\n" +
            "        const mapping = config.get(\"mapping\") || {};\n" +
            "        const preserve = config.getBoolean(\"preserve\", false);\n" +
            "        const payload = message.getPayload();\n" +
            "        const result = {};\n" +
            "        for (const sourcePath in mapping) {\n" +
            "            const targetField = mapping[sourcePath];\n" +
            "            const value = getNestedValue(payload, sourcePath);\n" +
            "            if (value !== undefined) setNestedValue(result, targetField, value);\n" +
            "        }\n" +
            "        result._source = message.getSubject();\n" +
            "        publisher.publish(\"mapped\", result);\n" +
            "    }\n" +
            "}\n" +
            "\n" +
            "function getNestedValue(obj, path) {\n" +
            "    const parts = path.split(\".\");\n" +
            "    let current = obj;\n" +
            "    for (const part of parts) {\n" +
            "        if (current == null) return undefined;\n" +
            "        current = current[part];\n" +
            "    }\n" +
            "    return current;\n" +
            "}\n" +
            "\n" +
            "function setNestedValue(obj, path, value) {\n" +
            "    const parts = path.split(\".\");\n" +
            "    let current = obj;\n" +
            "    for (let i = 0; i < parts.length - 1; i++) {\n" +
            "        if (!(parts[i] in current)) current[parts[i]] = {};\n" +
            "        current = current[parts[i]];\n" +
            "    }\n" +
            "    current[parts[parts.length - 1]] = value;\n" +
            "}\n";

    private static final String CONDITIONAL_URI = "quark/route/flow/conditional:v1";
    private static final String CONDITIONAL_SRC =
            "// quark/route/flow/conditional:v1\n" +
            "// Routes messages to different events based on content predicates.\n" +
            "\n" +
            "export default {\n" +
            "    onMessage: function(message, publisher) {\n" +
            "        const rules = config.get(\"rules\") || [];\n" +
            "        const payload = message.getPayload();\n" +
            "        for (const rule of rules) {\n" +
            "            if (matchRule(rule.when, payload)) {\n" +
            "                publisher.publish(rule.emit, payload);\n" +
            "                return;\n" +
            "            }\n" +
            "        }\n" +
            "    }\n" +
            "};\n" +
            "\n" +
            "function matchRule(expr, payload) {\n" +
            "    if (!expr) return true;\n" +
            "    try {\n" +
            "        const fn = new Function(\"payload\", '\"use strict\"; return (' + expr + ');');\n" +
            "        return fn(payload) === true;\n" +
            "    } catch (e) { return false; }\n" +
            "}\n";

    @Test
    void loadsStdoutNode() {
        TypeScriptNodeFactory factory = new TypeScriptNodeFactory(
                STDOUT_URI, STDOUT_SRC, "stdout logger");
        NodeProvider provider = factory.create(NodeConfig.empty());

        assertThat(provider).isNotNull();

        // Verify onMessage works — should not throw
        provider.onMessage(testMessage("hello"), noOpPublisher());

        provider.close();
    }

    @Test
    void loadsJsonParseNodeAndPublishesParsed() {
        TypeScriptNodeFactory factory = new TypeScriptNodeFactory(
                JSON_PARSE_URI, JSON_PARSE_SRC, "json parser");

        Map<String, Object> cfg = new HashMap<>();
        cfg.put("field", "data");
        cfg.put("strict", false);
        NodeProvider provider = factory.create(NodeConfig.of(cfg));

        AtomicReference<Map<String, Object>> publishedPayload = new AtomicReference<>();
        AtomicReference<String> publishedEvent = new AtomicReference<>();
        QuarkPublisher capturingPublisher = new CapturingPublisher(publishedEvent, publishedPayload);

        // Message with raw JSON string in `data` field
        provider.onMessage(testMessageWithPayload(Map.of("data", "{\"k\":\"v\"}")),
                capturingPublisher);

        assertThat(publishedEvent.get()).isEqualTo("parsed");
        assertThat(publishedPayload.get()).containsKey("data");
        Object parsedData = publishedPayload.get().get("data");
        assertThat(parsedData).isInstanceOf(Map.class);
        @SuppressWarnings("unchecked")
        Map<String, Object> parsedMap = (Map<String, Object>) parsedData;
        assertThat(parsedMap).containsEntry("k", "v");

        provider.close();
    }

    @Test
    void loadsMapNodeAndAppliesMapping() {
        TypeScriptNodeFactory factory = new TypeScriptNodeFactory(
                MAP_URI, MAP_SRC, "field mapper");

        Map<String, Object> cfg = new HashMap<>();
        // Map "cpu" (source path) -> "cpuUsage" (target field)
        cfg.put("mapping", Map.of("cpu", "cpuUsage"));
        cfg.put("preserve", false);
        NodeProvider provider = factory.create(NodeConfig.of(cfg));

        AtomicReference<Map<String, Object>> publishedPayload = new AtomicReference<>();
        AtomicReference<String> publishedEvent = new AtomicReference<>();
        QuarkPublisher capturingPublisher = new CapturingPublisher(publishedEvent, publishedPayload);

        Map<String, Object> payload = new HashMap<>();
        payload.put("cpu", 0.42);
        payload.put("mem", 1024);
        provider.onMessage(testMessageWithPayload(payload), capturingPublisher);

        assertThat(publishedEvent.get()).isEqualTo("mapped");
        // The mapped field should be present
        assertThat(publishedPayload.get()).containsKey("cpuUsage");
        // Unmapped field should NOT be in result (preserve=false)
        assertThat(publishedPayload.get()).doesNotContainKey("mem");

        provider.close();
    }

    @Test
    void loadsConditionalNodeAndRoutesToMatchingEvent() {
        TypeScriptNodeFactory factory = new TypeScriptNodeFactory(
                CONDITIONAL_URI, CONDITIONAL_SRC, "conditional router");

        Map<String, Object> cfg = new HashMap<>();
        // Two rules: error level -> emit "alert", value > 100 -> emit "high"
        cfg.put("rules", List.of(
                Map.of("when", "payload.level === 'error'", "emit", "alert"),
                Map.of("when", "payload.value > 100", "emit", "high")
        ));
        NodeProvider provider = factory.create(NodeConfig.of(cfg));

        AtomicReference<Map<String, Object>> publishedPayload = new AtomicReference<>();
        AtomicReference<String> publishedEvent = new AtomicReference<>();
        QuarkPublisher capturingPublisher = new CapturingPublisher(publishedEvent, publishedPayload);

        // Payload with level=error → should match first rule → emit "alert"
        Map<String, Object> payload = new HashMap<>();
        payload.put("level", "error");
        payload.put("value", 50);
        provider.onMessage(testMessageWithPayload(payload), capturingPublisher);

        assertThat(publishedEvent.get()).isEqualTo("alert");

        provider.close();
    }

    /**
     * Regression test: comments containing the substrings "type" and "as"
     * previously caused the old regex-based stripTypeScript() to corrupt
     * the source (eating "type Foo" out of comments, etc.). ESM
     * evaluation has no such issue.
     */
    @Test
    void loadsNodeWithCommentContainingKeywords() {
        String src =
                "// This module is type-safe. Treat this as a sample.\n" +
                "// The type of message is implicitly any. Cast as needed.\n" +
                "\n" +
                "export default {\n" +
                "    onMessage: function(message, publisher) {\n" +
                "        publisher.publish(\"ok\", { ok: true });\n" +
                "    }\n" +
                "};\n";

        TypeScriptNodeFactory factory = new TypeScriptNodeFactory(
                "test/keywords/comment:v1", src, "regression test");

        NodeProvider provider = factory.create(NodeConfig.empty());

        AtomicReference<Map<String, Object>> publishedPayload = new AtomicReference<>();
        AtomicReference<String> publishedEvent = new AtomicReference<>();
        QuarkPublisher capturingPublisher = new CapturingPublisher(publishedEvent, publishedPayload);

        provider.onMessage(testMessageWithPayload(Map.of()), capturingPublisher);
        assertThat(publishedEvent.get()).isEqualTo("ok");
        assertThat(publishedPayload.get()).containsEntry("ok", true);

        provider.close();
    }

    @Test
    void rejectsSourceWithoutExportDefault() {
        String src =
                "// Just a comment, no export default\n" +
                "const foo = 42;\n";

        TypeScriptNodeFactory factory = new TypeScriptNodeFactory(
                "test/broken/noexport:v1", src, "broken");

        assertThatThrownBy(() -> factory.create(NodeConfig.empty()))
                .isInstanceOf(RuntimeException.class)
                .hasMessageContaining("export default");
    }

    @Test
    void rejectsSourceWithSyntaxError() {
        String src =
                "export default {\n" +
                "    onMessage: function(message, publisher) {\n" +
                "        // missing closing brace\n" +
                "};\n";

        TypeScriptNodeFactory factory = new TypeScriptNodeFactory(
                "test/broken/syntax:v1", src, "broken");

        assertThatThrownBy(() -> factory.create(NodeConfig.empty()))
                .isInstanceOf(RuntimeException.class);
    }

    // ---------- helpers ----------

    private static QuarkMessage testMessage(String payload) {
        return testMessageWithPayload(Map.of("data", payload));
    }

    private static QuarkMessage testMessageWithPayload(Map<String, Object> payload) {
        return new QuarkMessage() {
            @Override public String subject() { return "test.subject"; }
            @Override public Map<String, Object> payload() { return payload; }
            @Override public Map<String, String> headers() { return Map.of(); }
            @Override public java.time.Instant timestamp() { return java.time.Instant.now(); }
            @Override public String systemName() { return "test-system"; }
            @Override public String namespace() { return "test-ns"; }
            @Override public String nodeName() { return "test-node"; }
        };
    }

    private static QuarkPublisher noOpPublisher() {
        return (event, payload) -> { /* no-op */ };
    }

    /** A publisher that captures the last event/payload pair for assertions. */
    private static final class CapturingPublisher implements QuarkPublisher {
        private final AtomicReference<String> eventRef;
        private final AtomicReference<Map<String, Object>> payloadRef;

        CapturingPublisher(AtomicReference<String> eventRef,
                           AtomicReference<Map<String, Object>> payloadRef) {
            this.eventRef = eventRef;
            this.payloadRef = payloadRef;
        }

        @Override
        public void publish(String event, Map<String, Object> payload) {
            eventRef.set(event);
            payloadRef.set(payload);
        }
    }
}
