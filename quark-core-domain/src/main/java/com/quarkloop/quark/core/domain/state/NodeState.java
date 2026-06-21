package com.quarkloop.quark.core.domain.state;

/**
 * The lifecycle state of a Node.
 */
public enum NodeState {
    CREATING,
    ACTIVE,
    PAUSED,
    ERROR,
    RECOVERING,
    DRAINING,
    ARCHIVED,
    DELETED
}
