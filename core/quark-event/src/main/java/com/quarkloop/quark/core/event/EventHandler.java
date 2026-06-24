package com.quarkloop.quark.core.event;

import com.quarkloop.quark.core.domain.event.NodeEvent;

/**
 * Functional interface for handling events.
 */
@FunctionalInterface
public interface EventHandler {
    void onEvent(NodeEvent event);
}
