package com.quarkloop.quark.providers.cpuprofiler;

import com.quarkloop.quark.core.domain.category.NodeCategory;
import com.quarkloop.quark.core.domain.config.NodeConfig;
import com.quarkloop.quark.core.domain.identity.NodeUri;
import com.quarkloop.quark.core.domain.spi.FunctionProvider;
import com.quarkloop.quark.core.domain.spi.QuarkMessage;
import com.quarkloop.quark.core.domain.spi.QuarkPublisher;
import com.quarkloop.quark.core.registry.NodeDescriptor;
import com.quarkloop.quark.core.registry.NodeImplementationFactory;
import jakarta.enterprise.context.ApplicationScoped;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import java.lang.management.ManagementFactory;
import java.lang.management.OperatingSystemMXBean;
import java.time.Instant;
import java.util.HashMap;
import java.util.Map;

/**
 * Function node that reads CPU usage on receipt of a trigger message and
 * publishes a {@code data} event with the current CPU load.
 *
 * <p>URI: {@code function/cpu-profiler:v1}. No config required.
 *
 * <p>Payload shape:
 * <pre>
 *   { "cpu": 0.42, "processCpu": 0.18, "timestamp": "2026-...", "trigger": "timer.tick" }
 * </pre>
 *
 * <p>Values are fractions in {@code [0.0, 1.0]} (or negative if unavailable
 * on the current JVM/OS).
 */
@ApplicationScoped
public class CpuProfilerFactory implements NodeImplementationFactory<FunctionProvider> {

    private static final Logger log = LoggerFactory.getLogger(CpuProfilerFactory.class);

    @Override
    public String uriPattern() {
        return "function/cpu-profiler";
    }

    @Override
    public FunctionProvider create(NodeConfig config) {
        return new CpuProfiler();
    }

    @Override
    public NodeDescriptor descriptor() {
        return new NodeDescriptor(
                NodeUri.parse("function/cpu-profiler:v1"),
                NodeCategory.FUNCTION,
                true,
                "Reads CPU usage (system + process) on receipt of a trigger."
        );
    }

    @Override
    public NodeCategory category() {
        return NodeCategory.FUNCTION;
    }

    static final class CpuProfiler implements FunctionProvider {

        @Override
        public void onMessage(QuarkMessage message, QuarkPublisher publisher) {
            OperatingSystemMXBean os = ManagementFactory.getOperatingSystemMXBean();
            double systemCpu = os.getSystemLoadAverage();
            double processCpu = -1.0;

            // com.sun.management.OperatingSystemMXBean exposes richer metrics
            if (os instanceof com.sun.management.OperatingSystemMXBean sunOs) {
                systemCpu = sunOs.getCpuLoad();
                processCpu = sunOs.getProcessCpuLoad();
            }

            Map<String, Object> payload = new HashMap<>();
            payload.put("cpu", systemCpu < 0 ? 0.0 : systemCpu);
            payload.put("processCpu", processCpu < 0 ? 0.0 : processCpu);
            payload.put("availableProcessors", os.getAvailableProcessors());
            payload.put("timestamp", Instant.now().toString());
            payload.put("trigger", message.subject());

            log.debug("CPU read: sys={}, proc={}", systemCpu, processCpu);
            publisher.publish("data", payload);
        }
    }
}
