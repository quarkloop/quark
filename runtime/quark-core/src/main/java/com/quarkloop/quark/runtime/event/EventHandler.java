package com.quarkloop.quark.runtime.event;

import com.quarkloop.quark.runtime.domain.event.NodeEvent;

/**
 * Functional interface for handling events.
 */
@FunctionalInterface
public interface EventHandler {
    void onEvent(NodeEvent event);
}
