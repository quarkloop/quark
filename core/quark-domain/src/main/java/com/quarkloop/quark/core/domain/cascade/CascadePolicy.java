package com.quarkloop.quark.core.domain.cascade;

/**
 * Governs behavior when a parent node changes state.
 */
public enum CascadePolicy {
    CASCADE("Dependents follow the parent into the same state"),
    ORPHAN("Dependents are left running (with a warning logged)"),
    REJECT("The parent's state change is blocked until dependents are removed"),
    NOTIFY("Dependents are notified but not affected");

    private final String description;

    CascadePolicy(String description) {
        this.description = description;
    }

    public String description() {
        return description;
    }
}
