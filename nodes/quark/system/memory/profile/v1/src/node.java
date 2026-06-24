package com.quarkloop.quark.providers.memoryprofiler;

import com.quarkloop.quark.core.domain.config.NodeConfig;
import com.quarkloop.quark.core.domain.identity.NodeUri;
import com.quarkloop.quark.core.domain.spi.NodeProvider;
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
 * Memory profiler node — reads JVM heap/non-heap memory usage on receipt of a trigger.
 *
 * <p>URI: {@code quark/system/memory/profile:v1}
 */
@ApplicationScoped
public class MemoryProfilerFactory implements NodeImplementationFactory {

    private static final Logger log = LoggerFactory.getLogger(MemoryProfilerFactory.class);

    @Override
    public String uriPattern() {
        return "quark/system/memory/profile";
    }

    @Override
    public NodeProvider create(NodeConfig config) {
        return new MemoryProfilerNode();
    }

    @Override
    public NodeDescriptor descriptor() {
        return new NodeDescriptor(
                NodeUri.parse("quark/system/memory/profile:v1"),
                "Reads JVM heap/non-heap memory usage on receipt of a trigger."
        );
    }

    static final class MemoryProfilerNode implements NodeProvider {

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
