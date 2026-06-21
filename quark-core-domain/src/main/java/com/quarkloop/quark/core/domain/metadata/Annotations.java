package com.quarkloop.quark.core.domain.metadata;

import java.util.Collections;
import java.util.Map;
import java.util.Objects;
import java.util.Set;

/**
 * Immutable wrapper over a map of annotations for attaching non-identifying metadata.
 */
public record Annotations(Map<String, String> values) {

    private static final Annotations EMPTY = new Annotations(Collections.emptyMap());

    public Annotations {
        Objects.requireNonNull(values, "values map cannot be null");
        values.forEach((k, v) -> {
            Objects.requireNonNull(k, "Annotation key cannot be null");
            Objects.requireNonNull(v, "Annotation value cannot be null");
        });
        values = Map.copyOf(values);
    }

    public static Annotations empty() {
        return EMPTY;
    }

    public static Annotations of(Map<String, String> values) {
        if (values == null || values.isEmpty()) {
            return EMPTY;
        }
        return new Annotations(values);
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
