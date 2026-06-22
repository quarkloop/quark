package com.quarkloop.quark.core.engine.store;

import java.time.Instant;

public record SystemRecord(
        String namespace, String name, String source,
        String state, String health, long version,
        Instant createdAt, Instant updatedAt
) {
    public SystemRecord {
        java.util.Objects.requireNonNull(namespace);
        java.util.Objects.requireNonNull(name);
        java.util.Objects.requireNonNull(source);
        java.util.Objects.requireNonNull(state);
        java.util.Objects.requireNonNull(health);
        if (version < 1) version = 1;
        Instant now = Instant.now();
        if (createdAt == null) createdAt = now;
        if (updatedAt == null) updatedAt = now;
    }
    public static SystemRecord creating(String namespace, String name, String source) {
        return new SystemRecord(namespace, name, source, "CREATING", "UNKNOWN", 1, null, null);
    }
}
