package com.quarkloop.quark.core.domain.validation;

import java.util.ArrayList;
import java.util.List;
import java.util.Objects;

/**
 * Aggregates validation errors.
 */
public record ValidationResult(List<ValidationError> errors) {

    public ValidationResult {
        if (errors == null) {
            errors = List.of();
        } else {
            errors = List.copyOf(errors);
        }
    }

    public static ValidationResult success() {
        return new ValidationResult(List.of());
    }

    public static ValidationResult failure(String path, String message) {
        return new ValidationResult(List.of(new ValidationError(path, message, ValidationSeverity.ERROR)));
    }

    public static ValidationResult failure(List<ValidationError> errors) {
        return new ValidationResult(errors);
    }

    public boolean isValid() {
        return errors.stream().noneMatch(e -> e.severity() == ValidationSeverity.ERROR);
    }

    public boolean hasWarnings() {
        return errors.stream().anyMatch(e -> e.severity() == ValidationSeverity.WARNING);
    }

    public ValidationResult merge(ValidationResult other) {
        if (other == null || other.errors().isEmpty()) {
            return this;
        }
        if (this.errors.isEmpty()) {
            return other;
        }
        List<ValidationError> combined = new ArrayList<>(this.errors);
        combined.addAll(other.errors());
        return new ValidationResult(combined);
    }
}
