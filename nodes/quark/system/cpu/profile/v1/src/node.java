package com.quarkloop.quark.providers.cpuprofiler;

import com.quarkloop.quark.runtime.domain.config.NodeConfig;
import com.quarkloop.quark.runtime.domain.identity.NodeUri;
import com.quarkloop.quark.runtime.domain.spi.NodeProvider;
import com.quarkloop.quark.runtime.domain.spi.QuarkMessage;
import com.quarkloop.quark.runtime.domain.spi.QuarkPublisher;
import com.quarkloop.quark.runtime.registry.NodeDescriptor;
import com.quarkloop.quark.runtime.registry.NodeImplementationFactory;
import jakarta.enterprise.context.ApplicationScoped;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import java.lang.management.ManagementFactory;
import java.lang.management.OperatingSystemMXBean;
import java.time.Instant;
import java.util.HashMap;
import java.util.Map;

/**
 * CPU profiler node — reads CPU usage on receipt of a trigger message.
 *
 * <p>URI: {@code quark/system/cpu/profile:v1}
 */
@ApplicationScoped
class CpuProfilerFactory implements NodeImplementationFactory {

    private static final Logger log = LoggerFactory.getLogger(CpuProfilerFactory.class);

    @Override
    public String uriPattern() {
        return "quark/system/cpu/profile";
    }

    @Override
    public NodeProvider create(NodeConfig config) {
        return new CpuProfilerNode();
    }

    @Override
    public NodeDescriptor descriptor() {
        return new NodeDescriptor(
                NodeUri.parse("quark/system/cpu/profile:v1"),
                "Reads CPU usage (system + process) on receipt of a trigger."
        );
    }

    static final class CpuProfilerNode implements NodeProvider {

        @Override
        public void onMessage(QuarkMessage message, QuarkPublisher publisher) {
            OperatingSystemMXBean os = ManagementFactory.getOperatingSystemMXBean();
            double systemCpu = os.getSystemLoadAverage();
            double processCpu = -1.0;

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
