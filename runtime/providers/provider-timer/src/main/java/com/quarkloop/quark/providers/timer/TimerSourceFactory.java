package com.quarkloop.quark.providers.timer;

import com.quarkloop.quark.core.domain.category.NodeCategory;
import com.quarkloop.quark.core.domain.config.NodeConfig;
import com.quarkloop.quark.core.domain.identity.NodeUri;
import com.quarkloop.quark.core.domain.spi.QuarkPublisher;
import com.quarkloop.quark.core.domain.spi.SourceProvider;
import com.quarkloop.quark.core.registry.NodeDescriptor;
import com.quarkloop.quark.core.registry.NodeImplementationFactory;
import jakarta.enterprise.context.ApplicationScoped;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import java.time.Duration;
import java.time.Instant;
import java.util.HashMap;
import java.util.Map;
import java.util.concurrent.Executors;
import java.util.concurrent.ScheduledExecutorService;
import java.util.concurrent.ThreadFactory;
import java.util.concurrent.TimeUnit;
import java.util.concurrent.atomic.AtomicLong;

/**
 * Source node that emits a {@code tick} envelope at a fixed interval.
 *
 * <p>URI: {@code source/timer:v1}. Config:
 * <ul>
 *   <li>{@code interval} (string, default {@code "1s"}) — duration like "1s", "500ms", "10s"</li>
 * </ul>
 *
 * <p>The provider publishes a {@code tick} event whose payload contains the
 * tick number (monotonic per-instance counter) and an ISO-8601 timestamp.
 */
@ApplicationScoped
public class TimerSourceFactory implements NodeImplementationFactory<SourceProvider> {

    private static final Logger log = LoggerFactory.getLogger(TimerSourceFactory.class);

    @Override
    public String uriPattern() {
        return "source/timer";
    }

    @Override
    public SourceProvider create(NodeConfig config) {
        return new TimerSource(config);
    }

    @Override
    public NodeDescriptor descriptor() {
        return new NodeDescriptor(
                NodeUri.parse("source/timer:v1"),
                NodeCategory.SOURCE,
                false,
                "Emits a tick envelope at a fixed interval (default 1s)."
        );
    }

    @Override
    public NodeCategory category() {
        return NodeCategory.SOURCE;
    }

    static final class TimerSource implements SourceProvider {

        private final Duration interval;
        private ScheduledExecutorService scheduler;
        private final AtomicLong tickCounter = new AtomicLong(0);

        TimerSource(NodeConfig config) {
            String intervalStr = config.getString("interval", "1s");
            this.interval = parseDuration(intervalStr);
        }

        @Override
        public void start(QuarkPublisher publisher, NodeConfig config) {
            // Use platform threads in native image (Truffle JIT doesn't support
            // virtual threads). Use virtual threads in JVM mode for efficiency.
            // Detection: check if running inside a native image by testing
            // whether the imagecodekey system property is set (GraalVM sets it
            // at build time). Also check quark.native as a fallback.
            boolean isNative = System.getProperty("org.graalvm.nativeimage.imagecodekey") != null
                    || "true".equals(System.getProperty("quark.native"));
            ThreadFactory factory = isNative
                    ? Thread.ofPlatform().name("quark-timer-", 0).factory()
                    : Thread.ofVirtual().name("quark-timer-", 0).factory();
            scheduler = Executors.newSingleThreadScheduledExecutor(factory);
            log.info("Starting timer source (interval={}, threads={})", interval, isNative ? "platform" : "virtual");
            scheduler.scheduleAtFixedRate(() -> {
                try {
                    long n = tickCounter.incrementAndGet();
                    Map<String, Object> payload = new HashMap<>();
                    payload.put("tick", n);
                    payload.put("timestamp", Instant.now().toString());
                    publisher.publish("tick", payload);
                } catch (Exception e) {
                    log.warn("Timer tick publish failed", e);
                }
            }, interval.toMillis(), interval.toMillis(), TimeUnit.MILLISECONDS);
        }

        @Override
        public void stop() {
            if (scheduler != null) {
                log.info("Stopping timer source (emitted {} ticks)", tickCounter.get());
                scheduler.shutdownNow();
                scheduler = null;
            }
        }

        /** Parse simple duration strings: "1s", "500ms", "10s", "1m". */
        private static Duration parseDuration(String s) {
            if (s == null || s.isBlank()) return Duration.ofSeconds(1);
            String t = s.trim().toLowerCase();
            try {
                if (t.endsWith("ms")) return Duration.ofMillis(Long.parseLong(t.substring(0, t.length() - 2)));
                if (t.endsWith("s"))  return Duration.ofSeconds(Long.parseLong(t.substring(0, t.length() - 1)));
                if (t.endsWith("m"))  return Duration.ofMinutes(Long.parseLong(t.substring(0, t.length() - 1)));
                if (t.endsWith("h"))  return Duration.ofHours(Long.parseLong(t.substring(0, t.length() - 1)));
                // Plain number → seconds
                return Duration.ofSeconds(Long.parseLong(t));
            } catch (NumberFormatException e) {
                log.warn("Invalid duration '{}', defaulting to 1s", s);
                return Duration.ofSeconds(1);
            }
        }
    }
}
