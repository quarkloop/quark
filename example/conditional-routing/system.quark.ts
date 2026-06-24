/**
 * Conditional Routing Example
 *
 * Demonstrates: timer → memory profiler → conditional router
 *   → "high" → stdout logger (memory > 50%)
 *   → "normal" → stdout logger (everything else)
 *
 * Deploy: quarkctl apply -f system.quark.ts -n demo
 */

export default {
    name: "conditional-routing",
    namespace: "demo",

    nodes: {
        // 2-second timer
        timer: {
            uses: "quark/time/schedule/timer:v1",
            interval: "2s",
            events: ["tick"],
        },

        // Memory profiler
        memory: {
            uses: "quark/system/memory/profile:v1",
            listens: ["timer.tick"],
            events: ["data"],
        },

        // Route based on heap usage percentage
        router: {
            uses: "quark/route/flow/conditional:v1",
            rules: [
                {
                    when: "payload.heapUsed / payload.heapMax > 0.5",
                    emit: "high"
                },
                {
                    when: "true",
                    emit: "normal"
                }
            ],
            listens: ["memory.data"],
            events: ["high", "normal"],
        },

        // Log high memory usage
        highLogger: {
            uses: "quark/log/console/stdout:v1",
            listens: ["router.high"],
        },

        // Log normal memory usage
        normalLogger: {
            uses: "quark/log/console/stdout:v1",
            listens: ["router.normal"],
        },

        // SSE stream for real-time monitoring
        stream: {
            uses: "quark/stream/sse/broadcast:v1",
            listens: ["router.high", "router.normal"],
        },
    },
};
