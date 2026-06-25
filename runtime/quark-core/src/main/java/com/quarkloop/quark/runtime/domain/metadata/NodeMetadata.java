package com.quarkloop.quark.runtime.domain.metadata;

import java.time.Instant;
import java.util.Objects;

/**
 * Metadata associated with a Node.
 */
public record NodeMetadata(
        Labels labels,
        Annotations annotations,
        Instant createdAt,
        Instant updatedAt,
        long version
) {
    public NodeMetadata {
        Objects.requireNonNull(labels, "labels cannot be null");
        Objects.requireNonNull(annotations, "annotations cannot be null");
        Objects.requireNonNull(createdAt, "createdAt cannot be null");
        Objects.requireNonNull(updatedAt, "updatedAt cannot be null");
        if (version < 1) {
            throw new IllegalArgumentException("version must be >= 1");
        }
    }

    public static NodeMetadata initial() {
        Instant now = Instant.now();
        return new NodeMetadata(Labels.empty(), Annotations.empty(), now, now, 1);
    }

    public NodeMetadata withVersion(long newVersion) {
        return new NodeMetadata(this.labels, this.annotations, this.createdAt, Instant.now(), newVersion);
    }

    public NodeMetadata withLabels(Labels newLabels) {
        return new NodeMetadata(newLabels, this.annotations, this.createdAt, Instant.now(), this.version + 1);
    }

    public NodeMetadata withAnnotations(Annotations newAnnotations) {
        return new NodeMetadata(this.labels, newAnnotations, this.createdAt, Instant.now(), this.version + 1);
    }
}
