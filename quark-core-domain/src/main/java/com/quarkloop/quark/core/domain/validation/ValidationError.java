package com.quarkloop.quark.core.domain.validation;

import java.util.Objects;

/**
 * Represents a validation error in a configuration or system.
 */
public record ValidationError(String path, String message, ValidationSeverity severity) {
    public ValidationError {
        Objects.requireNonNull(path, "path cannot be null");
        Objects.requireNonNull(message, "message cannot be null");
        Objects.requireNonNull(severity, "severity cannot be null");
    }
}
