package com.quarkloop.quark.runtime.domain.system;

import java.util.Objects;

/**
 * Failure handling configuration for a node.
 *
 * <p>When a node fails to process a message:
 * <ol>
 *   <li>NATS retries up to {@code retry} times with exponential backoff</li>
 *   <li>After max retries, the engine publishes the error payload to
 *       {@code <namespace>.<system>.fallback.<nodeName>}</li>
 *   <li>The node specified in {@code routeTo} must listen to that fallback subject</li>
 * </ol>
 *
 * <p>Subject ordering follows the container hierarchy: a namespace contains
 * systems, a system contains nodes, a node produces events. The subject reads
 * left-to-right from most-general (namespace) to most-specific (event), e.g.
 * {@code alice.monitor.fallback.cpu}.
 */
public record OnFailureConfig(
        int retry,
        String routeTo
) {
    public OnFailureConfig {
        if (retry < 0) {
            throw new IllegalArgumentException("retry must be >= 0, got " + retry);
        }
        Objects.requireNonNull(routeTo, "routeTo cannot be null");
        if (routeTo.isBlank()) {
            throw new IllegalArgumentException("routeTo cannot be blank");
        }
    }
}
