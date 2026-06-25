package com.quarkloop.quark.core.script;

import com.quarkloop.quark.core.domain.system.SystemDefinition;
import org.junit.jupiter.api.Test;

import static org.assertj.core.api.Assertions.assertThat;

/**
 * Unit tests for {@link SimpleSystemParser}.
 *
 * <p>Verifies the regex-based parser correctly handles all three example
 * .quark.ts files used in the platform.
 */
class SimpleSystemParserTest {

    private final SimpleSystemParser parser = new SimpleSystemParser();

    @Test
    void parsesSimpleStreamingSystem() {
        String ts = """
                /**
                 * Simple Streaming Monitor — Multi-Tenant Example
                 *
                 * This file IS the program. The user writes only TypeScript.
                 * Deploy: quarkctl system deploy -f system.quark.ts -n alice
                 */

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
                            timeout: "200ms",
                            listens: ["timer.tick"],
                            events: ["data"],
                            onFailure: { retry: 3, routeTo: "writer" },
                        },
                    },
                };
                """;

        SystemParseResult result = parser.parse(ts);
        assertThat(result).isInstanceOf(SystemParseResult.Success.class);
        SystemDefinition def = ((SystemParseResult.Success) result).system();
        assertThat(def.name()).isEqualTo("monitor");
        assertThat(def.namespace().value()).isEqualTo("alice");
        assertThat(def.nodes()).containsKeys("timer", "cpu");
        assertThat(def.nodes().get("cpu").onFailure().retry()).isEqualTo(3);
        assertThat(def.nodes().get("cpu").onFailure().routeTo()).isEqualTo("writer");
    }

    @Test
    void parsesJsonPipelineSystem() {
        String ts = """
                export default {
                    name: "json-pipeline",
                    namespace: "demo",

                    nodes: {
                        timer: {
                            uses: "quark/time/schedule/timer:v1",
                            interval: "1s",
                            events: ["tick"],
                        },
                        shaper: {
                            uses: "quark/data/shape/map:v1",
                            mapping: {
                                "cpu": "cpuUsage",
                                "processCpu": "processCpuUsage"
                            },
                            listens: ["cpu.data"],
                            events: ["mapped"],
                        },
                    },
                };
                """;

        SystemParseResult result = parser.parse(ts);
        assertThat(result).isInstanceOf(SystemParseResult.Success.class);
        SystemDefinition def = ((SystemParseResult.Success) result).system();
        assertThat(def.name()).isEqualTo("json-pipeline");
        assertThat(def.nodes()).containsKeys("timer", "shaper");
        // Verify config preserves the mapping
        assertThat(def.nodes().get("shaper").config().get("mapping")).isPresent();
    }

    @Test
    void rejectsEmptySource() {
        SystemParseResult result = parser.parse("");
        assertThat(result).isInstanceOf(SystemParseResult.Failure.class);
    }

    @Test
    void rejectsMissingNodes() {
        String ts = """
                export default {
                    name: "broken",
                    namespace: "demo"
                };
                """;
        SystemParseResult result = parser.parse(ts);
        assertThat(result).isInstanceOf(SystemParseResult.Failure.class);
    }
}
