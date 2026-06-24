package com.quarkloop.quark.core.engine.store;

public record RegistryRecord(
        String uri, String pattern, String category, boolean active, String description
) {
    public RegistryRecord {
        java.util.Objects.requireNonNull(uri);
        java.util.Objects.requireNonNull(pattern);
        java.util.Objects.requireNonNull(category);
        java.util.Objects.requireNonNull(description);
    }
}
