/**
 * JSON Pipeline Example
 *
 * Demonstrates: timer → JSON parse → field mapping → stdout logging
 *
 * This pipeline simulates receiving raw JSON strings, parsing them,
 * remapping the fields, and logging the result.
 *
 * Deploy: quarkctl apply -f system.quark.ts -n demo
 */

export default {
    name: "json-pipeline",
    namespace: "demo",

    nodes: {
        // 1-second timer
        timer: {
            uses: "quark/time/schedule/timer:v1",
            interval: "1s",
            events: ["tick"],
        },

        // CPU profiler produces data on each tick
        cpu: {
            uses: "quark/system/cpu/profile:v1",
            listens: ["timer.tick"],
            events: ["data"],
        },

        // Map the CPU data fields to a cleaner shape
        shaper: {
            uses: "quark/data/shape/map:v1",
            mapping: {
                "cpu": "cpuUsage",
                "processCpu": "processCpuUsage",
                "availableProcessors": "cores",
                "timestamp": "measuredAt"
            },
            listens: ["cpu.data"],
            events: ["mapped"],
        },

        // Log the remapped data to stdout
        logger: {
            uses: "quark/log/console/stdout:v1",
            listens: ["shaper.mapped"],
        },

        // Also write the raw CPU data to a file
        writer: {
            uses: "quark/io/file/write:v1",
            path: "./example/json-pipeline/output.jsonl",
            mode: "append",
            listens: ["cpu.data"],
        },
    },
};
