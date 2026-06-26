package com.quarkloop.quark.runtime.event;

import com.quarkloop.quark.runtime.domain.event.NodeEvent;

import java.util.List;

/**
 * SPI interface for event persistence.
 */
public interface EventStore {

    void append(NodeEvent event);
    
    void appendAll(List<NodeEvent> events);
    
    List<NodeEvent> query(EventFilter filter);
    
    long count(EventFilter filter);
}
