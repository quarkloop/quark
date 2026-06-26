package com.quarkloop.quark.runtime.domain.state;

import java.time.Instant;
import java.util.Objects;

/**
 * Represents a state transition for a Node.
 */
public record StateTransition(
        NodeState from,
        NodeState to,
        String trigger,
        Instant timestamp
) {
    public StateTransition {
        Objects.requireNonNull(from, "from state cannot be null");
        Objects.requireNonNull(to, "to state cannot be null");
        Objects.requireNonNull(timestamp, "timestamp cannot be null");
    }
}
