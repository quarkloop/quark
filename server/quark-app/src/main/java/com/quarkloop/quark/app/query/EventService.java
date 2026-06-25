package com.quarkloop.quark.app.query;

import com.quarkloop.quark.runtime.domain.event.NodeEvent;
import com.quarkloop.quark.runtime.domain.event.NodeEventKind;
import com.quarkloop.quark.runtime.event.EventFilter;
import com.quarkloop.quark.runtime.event.EventStore;
import jakarta.enterprise.context.ApplicationScoped;
import jakarta.inject.Inject;

import java.time.Instant;
import java.util.Arrays;
import java.util.List;
import java.util.Optional;
import java.util.Set;
import java.util.stream.Collectors;

/**
 * Read-only event queries. Multi-tenancy is enforced by requiring a
 * non-blank {@code namespace} on every call unless the caller explicitly
 * opts into admin mode with {@code all=true}.
 */
@ApplicationScoped
public class EventService {

    private final EventStore eventStore;

    @Inject
    public EventService(EventStore eventStore) {
        this.eventStore = eventStore;
    }

    public List<NodeEvent> query(String namespace, String systemName, String nodeName,
                                  String kindsCsv, Instant since, Instant until,
                                  int limit, boolean all) {
        EventFilter filter = buildFilter(namespace, systemName, nodeName, kindsCsv, since, until, limit, all);
        return eventStore.query(filter);
    }

    public long count(String namespace, String systemName, String nodeName,
                       String kindsCsv, Instant since, Instant until,
                       boolean all) {
        EventFilter filter = buildFilter(namespace, systemName, nodeName, kindsCsv, since, until, 100_000, all);
        return eventStore.count(filter);
    }

    private EventFilter buildFilter(String namespace, String systemName, String nodeName,
                                     String kindsCsv, Instant since, Instant until,
                                     int limit, boolean all) {
        EventFilter.Builder b = EventFilter.builder();
        if (!all) {
            if (namespace == null || namespace.isBlank()) {
                throw new IllegalArgumentException(
                        "namespace query parameter is required (use ?all=true for admin mode)");
            }
            b.namespace(namespace);
        } else if (namespace != null && !namespace.isBlank()) {
            b.namespace(namespace);
        }
        if (systemName != null && !systemName.isBlank()) b.systemName(systemName);
        if (nodeName != null && !nodeName.isBlank()) b.nodeName(nodeName);
        if (kindsCsv != null && !kindsCsv.isBlank()) {
            Set<NodeEventKind> kinds = Arrays.stream(kindsCsv.split(","))
                    .map(String::trim)
                    .map(String::toUpperCase)
                    .map(NodeEventKind::valueOf)
                    .collect(Collectors.toSet());
            b.kinds(kinds);
        }
        if (since != null) b.since(since);
        if (until != null) b.until(until);
        if (limit > 0) b.limit(limit); else b.limit(100);
        return b.build();
    }
}
