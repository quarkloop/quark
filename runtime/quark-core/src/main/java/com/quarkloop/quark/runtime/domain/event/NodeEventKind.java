package com.quarkloop.quark.runtime.domain.event;

/**
 * All possible event kinds defined in the spec.
 */
public enum NodeEventKind {
    NODE_CREATED,
    NODE_UPDATED,
    NODE_STATE_CHANGED,
    NODE_DATA_RECEIVED,
    NODE_DATA_PRODUCED,
    NODE_EXECUTION_STARTED,
    NODE_EXECUTION_COMPLETED,
    NODE_EXECUTION_FAILED,
    NODE_QUERY_RECEIVED,
    NODE_QUERY_RESPONDED,
    NODE_POLICY_EVALUATED,
    NODE_POLICY_VIOLATED
}
