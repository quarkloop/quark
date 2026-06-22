package com.quarkloop.quark.core.engine.store;

import com.quarkloop.quark.core.domain.event.NodeEvent;
import com.quarkloop.quark.core.event.EventFilter;
import com.quarkloop.quark.core.event.EventStore;
import java.time.Instant;
import java.util.List;

public interface EventRepository extends EventStore {
    int deleteOlderThan(Instant cutoff, int limit);
}
