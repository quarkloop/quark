package com.quarkloop.quark.core.domain.identity;

import java.util.Objects;
import java.util.regex.Pattern;

/**
 * Represents a validated node name.
 * Must match pattern: [a-z][a-z0-9-]*[a-z0-9] OR single char [a-z]
 */
public record NodeName(String value) {

    private static final Pattern NAME_PATTERN = Pattern.compile("^[a-z]([a-z0-9-]*[a-z0-9])?$");

    public NodeName {
        Objects.requireNonNull(value, "Node name cannot be null");
        if (!NAME_PATTERN.matcher(value).matches()) {
            throw new IllegalArgumentException("Invalid node name format: " + value +
                    ". Must be lowercase alphanumeric, optionally separated by hyphens.");
        }
    }

    public static NodeName of(String name) {
        return new NodeName(name);
    }

    @Override
    public String toString() {
        return value;
    }
}
