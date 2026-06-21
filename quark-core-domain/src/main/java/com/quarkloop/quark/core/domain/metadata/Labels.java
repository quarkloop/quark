package com.quarkloop.quark.core.domain.metadata;

import java.util.Collections;
import java.util.Map;
import java.util.Objects;
import java.util.Set;

/**
 * Immutable wrapper over a map of labels for identifying and selecting nodes.
 */
public record Labels(Map<String, String> values) {

    private static final Labels EMPTY = new Labels(Collections.emptyMap());

    public Labels {
        Objects.requireNonNull(values, "values map cannot be null");
        values.forEach((k, v) -> {
            Objects.requireNonNull(k, "Label key cannot be null");
            Objects.requireNonNull(v, "Label value cannot be null");
        });
        values = Map.copyOf(values);
    }

    public static Labels empty() {
        return EMPTY;
    }

    public static Labels of(Map<String, String> values) {
        if (values == null || values.isEmpty()) {
            return EMPTY;
        }
        return new Labels(values);
    }

    public boolean matches(Labels selector) {
        if (selector == null || selector.isEmpty()) {
            return true;
        }
        return selector.values().entrySet().stream()
                .allMatch(entry -> Objects.equals(this.values.get(entry.getKey()), entry.getValue()));
    }

    public String get(String key) {
        return values.get(key);
    }

    public boolean containsKey(String key) {
        return values.containsKey(key);
    }

    public Set<Map.Entry<String, String>> entrySet() {
        return values.entrySet();
    }

    public boolean isEmpty() {
        return values.isEmpty();
    }

    public int size() {
        return values.size();
    }
}
