package com.quarkloop.quark.providers.timer;

import com.quarkloop.quark.core.domain.config.NodeConfig;
import com.quarkloop.quark.core.domain.identity.NodeUri;
import com.quarkloop.quark.core.domain.spi.NodeProvider;
import com.quarkloop.quark.core.domain.spi.QuarkPublisher;
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
 * Timer node — emits a {@code tick} event at a fixed interval.
 *
 * <p>URI: {@code quark/time/schedule/timer:v1}
 * Config: {@code interval} (string, default "1s")
 */
@ApplicationScoped
class TimerSourceFactory implements NodeImplementationFactory {

    private static final Logger log = LoggerFactory.getLogger(TimerSourceFactory.class);

    @Override
    public String uriPattern() {
        return "quark/time/schedule/timer";
    }

    @Override
    public NodeProvider create(NodeConfig config) {
        return new TimerNode();
    }

    @Override
    public NodeDescriptor descriptor() {
        return new NodeDescriptor(
                NodeUri.parse("quark/time/schedule/timer:v1"),
                "Emits a tick event at a fixed interval (default 1s)."
        );
    }

    static final class TimerNode implements NodeProvider {

        private Duration interval;
        private ScheduledExecutorService scheduler;
        private final AtomicLong tickCounter = new AtomicLong(0);

        @Override
        public void init(NodeConfig config) {
            String intervalStr = config.getString("interval", "1s");
            this.interval = parseDuration(intervalStr);
        }

        @Override
        public void start(QuarkPublisher publisher, NodeConfig config) {
            boolean isNative = System.getProperty("org.graalvm.nativeimage.imagecodekey") != null
                    || "true".equals(System.getProperty("quark.native"));
            ThreadFactory factory = isNative
                    ? Thread.ofPlatform().name("quark-timer-", 0).factory()
                    : Thread.ofVirtual().name("quark-timer-", 0).factory();
            scheduler = Executors.newSingleThreadScheduledExecutor(factory);
            log.info("Starting timer (interval={}, threads={})", interval, isNative ? "platform" : "virtual");
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
        public void close() {
            if (scheduler != null) {
                log.info("Stopping timer (emitted {} ticks)", tickCounter.get());
                scheduler.shutdownNow();
                scheduler = null;
            }
        }

        private static Duration parseDuration(String s) {
            if (s == null || s.isBlank()) return Duration.ofSeconds(1);
            String t = s.trim().toLowerCase();
            try {
                if (t.endsWith("ms")) return Duration.ofMillis(Long.parseLong(t.substring(0, t.length() - 2)));
                if (t.endsWith("s"))  return Duration.ofSeconds(Long.parseLong(t.substring(0, t.length() - 1)));
                if (t.endsWith("m"))  return Duration.ofMinutes(Long.parseLong(t.substring(0, t.length() - 1)));
                if (t.endsWith("h"))  return Duration.ofHours(Long.parseLong(t.substring(0, t.length() - 1)));
                return Duration.ofSeconds(Long.parseLong(t));
            } catch (NumberFormatException e) {
                log.warn("Invalid duration '{}', defaulting to 1s", s);
                return Duration.ofSeconds(1);
            }
        }
    }
}
