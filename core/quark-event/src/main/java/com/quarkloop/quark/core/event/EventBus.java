package com.quarkloop.quark.core.event;

import com.quarkloop.quark.core.domain.event.NodeEvent;
import com.quarkloop.quark.core.domain.event.NodeEventKind;

import java.util.Set;

/**
 * Interface for in-process publish/subscribe of node events.
 */
public interface EventBus {
    
    void publish(NodeEvent event);
    
    void subscribe(NodeEventKind kind, EventHandler handler);
    
    void subscribe(Set<NodeEventKind> kinds, EventHandler handler);
    
    void subscribeAll(EventHandler handler);
    
    void unsubscribe(EventHandler handler);
}
