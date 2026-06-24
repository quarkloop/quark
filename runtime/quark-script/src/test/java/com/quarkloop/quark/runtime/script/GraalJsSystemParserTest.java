package com.quarkloop.quark.runtime.script;

import com.quarkloop.quark.core.domain.system.SystemDefinition;
import com.quarkloop.quark.core.script.SystemParseResult;
import com.quarkloop.quark.core.script.SystemParser;
import org.junit.jupiter.api.Test;

import static org.assertj.core.api.Assertions.assertThat;

/**
 * Sanity test: the {@link GraalJsSystemParser} can parse the canonical
 * example file shape and produce a {@link SystemDefinition}.
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
                        uses: "source/timer:v1",
                        interval: "1s",
                        events: ["tick"],
                    },
                    cpu: {
                        uses: "function/cpu-profiler:v1",
                        listens: ["timer.tick"],
                        events: ["data"],
                        onFailure: { retry: 3, routeTo: "writer" },
                    },
                    writer: {
                        uses: "store/json-writer:v1",
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
        assertThat(def.nodes().get("timer").uri().implementation()).isEqualTo("timer");
        assertThat(def.nodes().get("timer").events()).containsExactly("tick");
        assertThat(def.nodes().get("cpu").listens()).containsExactly("timer.tick");
        assertThat(def.nodes().get("cpu").onFailure().retry()).isEqualTo(3);
        assertThat(def.nodes().get("cpu").onFailure().routeTo()).isEqualTo("writer");
    }

    @Test
    void parseInvalidSource() {
        SystemParseResult result = parser.parse("// nothing exported");
        assertThat(result).isInstanceOf(SystemParseResult.Failure.class);
    }
}
