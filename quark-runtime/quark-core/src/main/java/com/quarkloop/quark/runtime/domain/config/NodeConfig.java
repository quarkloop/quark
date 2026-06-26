package com.quarkloop.quark.runtime.domain.config;

import java.util.Collections;
import java.util.Map;
import java.util.Objects;
import java.util.Optional;

/**
 * Immutable configuration properties for a Node.
 */
public record NodeConfig(Map<String, Object> properties) {

    private static final NodeConfig EMPTY = new NodeConfig(Collections.emptyMap());

    public NodeConfig {
        Objects.requireNonNull(properties, "properties cannot be null");
        properties = Map.copyOf(properties);
    }

    public static NodeConfig empty() {
        return EMPTY;
    }

    public static NodeConfig of(Map<String, Object> properties) {
        if (properties == null || properties.isEmpty()) {
            return EMPTY;
        }
        return new NodeConfig(properties);
    }

    public Optional<Object> get(String key) {
        return Optional.ofNullable(properties.get(key));
    }

    public String getString(String key, String defaultValue) {
        Object val = properties.get(key);
        return val != null ? val.toString() : defaultValue;
    }

    public int getInt(String key, int defaultValue) {
        Object val = properties.get(key);
        if (val instanceof Number n) {
            return n.intValue();
        }
        if (val instanceof String s) {
            try {
                return Integer.parseInt(s);
            } catch (NumberFormatException e) {
                return defaultValue;
            }
        }
        return defaultValue;
    }

    public boolean getBoolean(String key, boolean defaultValue) {
        Object val = properties.get(key);
        if (val instanceof Boolean b) {
            return b;
        }
        if (val instanceof String s) {
            return Boolean.parseBoolean(s);
        }
        return defaultValue;
    }

    public Map<String, Object> asMap() {
        return properties; // Already unmodifiable via constructor
    }
}
