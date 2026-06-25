package com.quarkloop.quark.runtime.event;

import java.time.Instant;
import java.util.Collections;
import java.util.Set;

import com.quarkloop.quark.runtime.domain.event.NodeEventKind;

/**
 * Filter for querying events from the store.
 */
public record EventFilter(
        String nodeName,
        String systemName,
        String namespace,
        Set<NodeEventKind> kinds,
        Instant since,
        Instant until,
        int limit
) {
    public EventFilter {
        if (kinds == null) {
            kinds = Collections.emptySet();
        } else {
            kinds = Set.copyOf(kinds);
        }
        if (limit <= 0) {
            limit = 1000;
        }
    }

    public static Builder builder() {
        return new Builder();
    }

    public static class Builder {
        private String nodeName;
        private String systemName;
        private String namespace;
        private Set<NodeEventKind> kinds;
        private Instant since;
        private Instant until;
        private int limit = 1000;

        public Builder nodeName(String nodeName) {
            this.nodeName = nodeName;
            return this;
        }

        public Builder systemName(String systemName) {
            this.systemName = systemName;
            return this;
        }

        public Builder namespace(String namespace) {
            this.namespace = namespace;
            return this;
        }

        public Builder kinds(Set<NodeEventKind> kinds) {
            this.kinds = kinds;
            return this;
        }

        public Builder since(Instant since) {
            this.since = since;
            return this;
        }

        public Builder until(Instant until) {
            this.until = until;
            return this;
        }

        public Builder limit(int limit) {
            this.limit = limit;
            return this;
        }

        public EventFilter build() {
            return new EventFilter(nodeName, systemName, namespace, kinds, since, until, limit);
        }
    }
}
