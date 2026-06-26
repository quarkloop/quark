package com.quarkloop.quark.runtime.domain.event;

import com.fasterxml.jackson.annotation.JsonInclude;

import java.time.Instant;
import java.util.Collections;
import java.util.Map;
import java.util.Objects;
import java.util.UUID;

/**
 * Immutable record of a significant occurrence in a node's lifecycle.
 *
 * <p>Every event carries its {@code namespace} for multi-tenant isolation.
 */
@JsonInclude(JsonInclude.Include.NON_NULL)
public record NodeEvent(
        UUID id,
        NodeEventKind kind,
        String nodeName,
        String systemName,
        String namespace,
        Instant timestamp,
        Map<String, Object> payload
) {
    public NodeEvent {
        Objects.requireNonNull(id, "id cannot be null");
        Objects.requireNonNull(kind, "kind cannot be null");
        Objects.requireNonNull(nodeName, "nodeName cannot be null");
        Objects.requireNonNull(systemName, "systemName cannot be null");
        Objects.requireNonNull(namespace, "namespace cannot be null");
        Objects.requireNonNull(timestamp, "timestamp cannot be null");
        if (payload == null) {
            payload = Collections.emptyMap();
        } else {
            payload = Map.copyOf(payload);
        }
    }

    public static NodeEvent of(NodeEventKind kind, String nodeName, String systemName, String namespace) {
        return new NodeEvent(UUID.randomUUID(), kind, nodeName, systemName, namespace, Instant.now(), Collections.emptyMap());
    }

    public static NodeEvent of(NodeEventKind kind, String nodeName, String systemName, String namespace, Map<String, Object> payload) {
        return new NodeEvent(UUID.randomUUID(), kind, nodeName, systemName, namespace, Instant.now(), payload);
    }

    public static NodeEvent system(NodeEventKind kind, String nodeName, String systemName) {
        return new NodeEvent(UUID.randomUUID(), kind, nodeName, systemName, "system", Instant.now(), Collections.emptyMap());
    }

    public static NodeEvent system(NodeEventKind kind, String nodeName, String systemName, Map<String, Object> payload) {
        return new NodeEvent(UUID.randomUUID(), kind, nodeName, systemName, "system", Instant.now(), payload);
    }
}
