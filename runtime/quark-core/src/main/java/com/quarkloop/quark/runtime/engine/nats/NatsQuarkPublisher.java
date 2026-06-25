package com.quarkloop.quark.runtime.engine.nats;

import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.datatype.jsr310.JavaTimeModule;
import com.quarkloop.quark.runtime.domain.spi.QuarkPublisher;
import com.quarkloop.quark.runtime.engine.metrics.NamespaceMetrics;
import io.nats.client.Connection;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import java.util.Map;
import java.util.Objects;
import java.util.Set;

/**
 * Implementation of {@link QuarkPublisher} that publishes to NATS.
 *
 * <p>The publisher is scoped to a single node within a system. It resolves
 * event names to full NATS subjects: {@code <namespace>.<system>.<nodeName>.<event>}.
 *
 * <p>ACL enforcement: the publisher checks that the event is in the node's
 * declared {@code events} list before publishing. If not, it throws
 * {@link SecurityException}.
 *
 * <p>Metrics: each successful publish increments the namespace's publish
 * counter in {@link NamespaceMetrics}. ACL violations and publish failures
 * increment the namespace's error counter.
 */
public final class NatsQuarkPublisher implements QuarkPublisher {

    private static final Logger log = LoggerFactory.getLogger(NatsQuarkPublisher.class);

    private static final ObjectMapper mapper = new ObjectMapper();
    static {
        mapper.registerModule(new JavaTimeModule());
    }

    private final Connection natsConnection;
    private final String systemName;
    private final String namespace;
    private final String nodeName;
    private final Set<String> allowedEvents;
    private final NamespaceMetrics namespaceMetrics;

    /**
     * @param natsConnection   the NATS connection
     * @param systemName       the system name (e.g., "monitor")
     * @param namespace        the namespace (e.g., "alice")
     * @param nodeName         the node name (e.g., "cpu")
     * @param allowedEvents    the events this node is allowed to publish (from {@code events: [...]} config)
     * @param namespaceMetrics the per-namespace metrics accumulator (may be null in tests)
     */
    public NatsQuarkPublisher(Connection natsConnection, String systemName,
                               String namespace, String nodeName, Set<String> allowedEvents,
                               NamespaceMetrics namespaceMetrics) {
        this.natsConnection = Objects.requireNonNull(natsConnection, "natsConnection cannot be null");
        this.systemName = systemName;
        this.namespace = namespace;
        this.nodeName = nodeName;
        this.allowedEvents = allowedEvents;
        this.namespaceMetrics = namespaceMetrics;
    }

    @Override
    public void publish(String event, Map<String, Object> payload) {
        // ACL check
        if (!allowedEvents.contains(event)) {
            if (namespaceMetrics != null) namespaceMetrics.recordError(namespace);
            throw new SecurityException(
                "Node '" + nodeName + "' is not allowed to publish event '" + event +
                "'. Declared events: " + allowedEvents);
        }

        String subject = SubjectResolver.eventSubject(systemName, namespace, nodeName, event);

        try {
            byte[] data = mapper.writeValueAsBytes(payload);
            natsConnection.publish(subject, data);
            if (namespaceMetrics != null) namespaceMetrics.recordPublish(namespace);
            log.trace("Published to {}: {} bytes", subject, data.length);
        } catch (Exception e) {
            if (namespaceMetrics != null) namespaceMetrics.recordError(namespace);
            log.error("Failed to publish to {}", subject, e);
            throw new RuntimeException("Publish failed: " + e.getMessage(), e);
        }
    }
}
