package com.quarkloop.quark.core.engine.store;

public record RegistryRecord(
        String uri, String pattern, String description
) {
    public RegistryRecord {
        java.util.Objects.requireNonNull(uri);
        java.util.Objects.requireNonNull(pattern);
        if (description == null) description = "";
    }
}
