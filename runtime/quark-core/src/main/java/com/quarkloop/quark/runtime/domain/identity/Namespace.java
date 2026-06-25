package com.quarkloop.quark.runtime.domain.identity;

import java.util.Objects;
import java.util.regex.Pattern;

/**
 * Represents a validated namespace.
 * Must match: [a-z][a-z0-9-]* (min 1 char)
 */
public record Namespace(String value) {

    private static final Pattern NAMESPACE_PATTERN = Pattern.compile("^[a-z][a-z0-9-]*$");

    public static final Namespace DEFAULT = new Namespace("default");

    public Namespace {
        Objects.requireNonNull(value, "Namespace cannot be null");
        if (!NAMESPACE_PATTERN.matcher(value).matches()) {
            throw new IllegalArgumentException("Invalid namespace format: " + value +
                    ". Must start with a lowercase letter and contain only lowercase alphanumeric and hyphens.");
        }
    }

    public static Namespace of(String ns) {
        if (ns == null || ns.isBlank()) {
            return DEFAULT;
        }
        return new Namespace(ns);
    }

    @Override
    public String toString() {
        return value;
    }
}
