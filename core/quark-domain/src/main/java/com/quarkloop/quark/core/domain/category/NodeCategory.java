package com.quarkloop.quark.core.domain.category;

import java.util.Arrays;

/**
 * The primary classification of a Node in the Quark platform.
 */
public enum NodeCategory {
    SOURCE("source", false),
    FUNCTION("function", true),
    STORE("store", false),
    ENDPOINT("endpoint", false),
    POLICY("policy", false);

    private final String label;
    private final boolean active;

    NodeCategory(String label, boolean active) {
        this.label = label;
        this.active = active;
    }

    public String label() {
        return label;
    }

    /**
     * @return true if nodes of this category are active (perform behavior/execute).
     */
    public boolean isActive() {
        return active;
    }

    /**
     * @return true if nodes of this category are passive (state/description only).
     */
    public boolean isPassive() {
        return !active;
    }

    public static NodeCategory fromLabel(String label) {
        if (label == null || label.isBlank()) {
            throw new IllegalArgumentException("Category label cannot be null or blank");
        }
        return Arrays.stream(values())
                .filter(c -> c.label.equals(label))
                .findFirst()
                .orElseThrow(() -> new IllegalArgumentException("Unknown node category: " + label));
    }
}
