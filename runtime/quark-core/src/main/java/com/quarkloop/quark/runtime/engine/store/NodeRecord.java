package com.quarkloop.quark.runtime.engine.store;

import java.time.Instant;
import java.util.List;
import java.util.Map;

public record NodeRecord(
        String namespace, String systemName, String name,
        String uri, String state, String health,
        long version, String errorMessage,
        List<String> listens, List<String> events,
        Map<String, Object> config, Map<String, String> labels, Map<String, String> annotations,
        String onFailureRetry, String onFailureRouteTo, String timeout,
        Instant createdAt, Instant updatedAt
) {
    public NodeRecord {
        java.util.Objects.requireNonNull(namespace);
        java.util.Objects.requireNonNull(systemName);
        java.util.Objects.requireNonNull(name);
        java.util.Objects.requireNonNull(uri);
        java.util.Objects.requireNonNull(state);
        java.util.Objects.requireNonNull(health);
        if (version < 1) version = 1;
        if (listens == null) listens = List.of();
        if (events == null) events = List.of();
        if (config == null) config = Map.of();
        if (labels == null) labels = Map.of();
        if (annotations == null) annotations = Map.of();
        Instant now = Instant.now();
        if (createdAt == null) createdAt = now;
        if (updatedAt == null) updatedAt = now;
    }
}
