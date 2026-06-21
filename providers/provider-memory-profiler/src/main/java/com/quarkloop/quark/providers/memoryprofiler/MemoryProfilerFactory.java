package com.quarkloop.quark.providers.memoryprofiler;

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
import java.lang.management.MemoryMXBean;
import java.lang.management.MemoryUsage;
import java.time.Instant;
import java.util.HashMap;
import java.util.Map;

/**
 * Function node that reads JVM heap/non-heap memory usage on receipt of a
 * trigger message and publishes a {@code data} event.
 *
 * <p>URI: {@code function/memory-profiler:v1}. No config required.
 *
 * <p>Payload shape:
 * <pre>
 *   {
 *     "heapUsed": 12345678, "heapCommitted": 33554432, "heapMax": 536870912,
 *     "nonHeapUsed": 9876543, "nonHeapCommitted": 16777216,
 *     "timestamp": "2026-...", "trigger": "timer.tick"
 *   }
 * </pre>
 */
@ApplicationScoped
public class MemoryProfilerFactory implements NodeImplementationFactory<FunctionProvider> {

    private static final Logger log = LoggerFactory.getLogger(MemoryProfilerFactory.class);

    @Override
    public String uriPattern() {
        return "function/memory-profiler";
    }

    @Override
    public FunctionProvider create(NodeConfig config) {
        return new MemoryProfiler();
    }

    @Override
    public NodeDescriptor descriptor() {
        return new NodeDescriptor(
                NodeUri.parse("function/memory-profiler:v1"),
                NodeCategory.FUNCTION,
                true,
                "Reads JVM heap/non-heap memory usage on receipt of a trigger."
        );
    }

    @Override
    public NodeCategory category() {
        return NodeCategory.FUNCTION;
    }

    static final class MemoryProfiler implements FunctionProvider {

        private final MemoryMXBean memory = ManagementFactory.getMemoryMXBean();

        @Override
        public void onMessage(QuarkMessage message, QuarkPublisher publisher) {
            MemoryUsage heap = memory.getHeapMemoryUsage();
            MemoryUsage nonHeap = memory.getNonHeapMemoryUsage();

            Map<String, Object> payload = new HashMap<>();
            payload.put("heapUsed", heap.getUsed());
            payload.put("heapCommitted", heap.getCommitted());
            payload.put("heapMax", heap.getMax() < 0 ? 0 : heap.getMax());
            payload.put("nonHeapUsed", nonHeap.getUsed());
            payload.put("nonHeapCommitted", nonHeap.getCommitted());
            payload.put("objectPendingFinalizationCount", memory.getObjectPendingFinalizationCount());
            payload.put("timestamp", Instant.now().toString());
            payload.put("trigger", message.subject());

            log.debug("Memory read: heapUsed={}, nonHeapUsed={}", heap.getUsed(), nonHeap.getUsed());
            publisher.publish("data", payload);
        }
    }
}
