package com.quarkloop.quark.runtime.domain.state;

import java.time.Instant;
import java.util.EnumSet;
import java.util.Set;

/**
 * Pure function state machine governing node lifecycle transitions.
 */
public final class LifecycleStateMachine {

    private LifecycleStateMachine() {
        // Utility class
    }

    public static boolean isValidTransition(NodeState from, NodeState to) {
        return validTargets(from).contains(to);
    }

    public static Set<NodeState> validTargets(NodeState from) {
        if (from == null) {
            return EnumSet.noneOf(NodeState.class);
        }
        return switch (from) {
            case CREATING -> EnumSet.of(NodeState.ACTIVE, NodeState.ERROR);
            case ACTIVE -> EnumSet.of(NodeState.PAUSED, NodeState.ERROR, NodeState.DRAINING);
            case PAUSED -> EnumSet.of(NodeState.ACTIVE, NodeState.ERROR, NodeState.DRAINING);
            case ERROR -> EnumSet.of(NodeState.RECOVERING, NodeState.ARCHIVED);
            case RECOVERING -> EnumSet.of(NodeState.ACTIVE, NodeState.ERROR);
            case DRAINING -> EnumSet.of(NodeState.ARCHIVED, NodeState.ERROR);
            case ARCHIVED -> EnumSet.of(NodeState.DELETED);
            case DELETED -> EnumSet.noneOf(NodeState.class);
        };
    }

    public static StateTransition transition(NodeState from, NodeState to, String trigger) {
        if (!isValidTransition(from, to)) {
            throw new IllegalStateException(String.format("Invalid state transition from %s to %s", from, to));
        }
        return new StateTransition(from, to, trigger, Instant.now());
    }
}
