package com.quarkloop.quark.runtime.script;

import com.quarkloop.quark.core.domain.system.SystemDefinition;
import com.quarkloop.quark.core.script.SystemParseResult;
import com.quarkloop.quark.core.script.SystemParser;
import org.junit.jupiter.api.Test;

import static org.assertj.core.api.Assertions.assertThat;

/**
 * Sanity tests for {@link GraalJsSystemParser}.
 *
 * <p>These tests cover:
 * <ul>
 *   <li>The canonical system shape — verifies ESM module evaluation
 *       produces a valid {@link SystemDefinition}.</li>
 *   <li>A source file with comments containing the substrings
 *       {@code "type"} and {@code "as"} (which previously triggered
 *       false-positive matches in the old regex-based stripper).</li>
 *   <li>An invalid source (no {@code export default}) that must produce
 *       a {@link SystemParseResult.Failure}.</li>
 *   <li>A system that uses an inline TypeScript-style comment block
 *       before the export.</li>
 * </ul>
 */
class GraalJsSystemParserTest {

    private final GraalJsSystemParser parser = new GraalJsSystemParser();

    @Test
    void parseSimpleSystem() {
        String ts = """
            export default {
                name: "monitor",
                namespace: "alice",

                nodes: {
                    timer: {
                        uses: "quark/time/schedule/timer:v1",
                        interval: "1s",
                        events: ["tick"],
                    },
                    cpu: {
                        uses: "quark/system/cpu/profile:v1",
                        listens: ["timer.tick"],
                        events: ["data"],
                        onFailure: { retry: 3, routeTo: "writer" },
                    },
                    writer: {
                        uses: "quark/io/file/write:v1",
                        path: "./out.jsonl",
                        mode: "append",
                        listens: ["cpu.data", "fallback.cpu"],
                    },
                },
            };
            """;

        SystemParseResult result = parser.parse(ts);
        assertThat(result).isInstanceOf(SystemParseResult.Success.class);
        SystemParseResult.Success success = (SystemParseResult.Success) result;
        SystemDefinition def = success.system();
        assertThat(def.name()).isEqualTo("monitor");
        assertThat(def.namespace().value()).isEqualTo("alice");
        assertThat(def.nodes()).containsKeys("timer", "cpu", "writer");
        assertThat(def.nodes().get("timer").uri().node()).isEqualTo("timer");
        assertThat(def.nodes().get("timer").events()).containsExactly("tick");
        assertThat(def.nodes().get("cpu").listens()).containsExactly("timer.tick");
        assertThat(def.nodes().get("cpu").onFailure().retry()).isEqualTo(3);
        assertThat(def.nodes().get("cpu").onFailure().routeTo()).isEqualTo("writer");
    }

    /**
     * Regression test: comments containing the substrings {@code "type"}
     * and {@code "as"} previously caused the old regex-based
     * {@code stripTypeScript()} to corrupt the source (eating the words
     * "type Foo" and "as Bar" out of comments). ESM evaluation has no
     * such issue.
     */
    @Test
    void parseSystemWithCommentsContainingKeywords() {
        String ts = """
            /**
             * This file IS the program. Write it as JSON, not as a type.
             * Treat this as a string. The word type appears many times here
             * but should never be stripped.
             */
            export default {
                name: "json-pipeline",
                namespace: "demo",
                nodes: {
                    timer: {
                        uses: "quark/time/schedule/timer:v1",
                        interval: "1s",
                        events: ["tick"],
                    },
                },
            };
            """;

        SystemParseResult result = parser.parse(ts);
        assertThat(result).isInstanceOf(SystemParseResult.Success.class);
        SystemDefinition def = ((SystemParseResult.Success) result).system();
        assertThat(def.name()).isEqualTo("json-pipeline");
        assertThat(def.namespace().value()).isEqualTo("demo");
        assertThat(def.nodes()).containsOnlyKeys("timer");
    }

    @Test
    void parseInvalidSource() {
        SystemParseResult result = parser.parse("// nothing exported");
        assertThat(result).isInstanceOf(SystemParseResult.Failure.class);
    }

    @Test
    void parseEmptySource() {
        SystemParseResult result = parser.parse("");
        assertThat(result).isInstanceOf(SystemParseResult.Failure.class);
    }

    @Test
    void parseMissingNodes() {
        String ts = """
            export default {
                name: "broken",
                namespace: "demo"
            };
            """;
        SystemParseResult result = parser.parse(ts);
        assertThat(result).isInstanceOf(SystemParseResult.Failure.class);
        assertThat(((SystemParseResult.Failure) result).errors())
                .anyMatch(e -> e.contains("nodes"));
    }
}
