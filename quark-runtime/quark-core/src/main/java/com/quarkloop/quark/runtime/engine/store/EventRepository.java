package com.quarkloop.quark.runtime.engine.store;

import com.quarkloop.quark.runtime.domain.event.NodeEvent;
import com.quarkloop.quark.runtime.event.EventFilter;
import com.quarkloop.quark.runtime.event.EventStore;
import java.time.Instant;
import java.util.List;

public interface EventRepository extends EventStore {
    int deleteOlderThan(Instant cutoff, int limit);
}
