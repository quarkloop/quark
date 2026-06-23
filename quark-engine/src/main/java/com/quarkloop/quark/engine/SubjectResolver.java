package com.quarkloop.quark.engine;

import com.quarkloop.quark.core.domain.identity.Namespace;

/**
 * Resolves relative subject references to full NATS subjects.
 *
 * <p>Subject format: {@code <namespace>.<system>.<node>.<event>}
 *
 * <p>This follows the container hierarchy: namespace contains systems,
 * systems contain nodes, nodes produce events. The subject reads
 * left-to-right from most-general (namespace) to most-specific (event).
 *
 * <p>This enables efficient wildcard subscriptions:
 * <ul>
 *   <li>{@code alice.>} — all events in namespace alice (tenant isolation)</li>
 *   <li>{@code alice.monitor.>} — all events from system "monitor" in alice</li>
 *   <li>{@code alice.monitor.timer.>} — all events from the timer node</li>
 *   <li>{@code alice.monitor.timer.tick} — one specific event</li>
 * </ul>
 */
public final class SubjectResolver {

    private SubjectResolver() {}

    /**
     * Resolve a relative subject to a full NATS subject.
     *
     * @param systemName the system name (e.g., "monitor")
     * @param namespace  the namespace (e.g., "alice")
     * @param relative   the relative subject (e.g., "timer.tick")
     * @return the full subject (e.g., "alice.monitor.timer.tick")
     */
    public static String resolve(String systemName, Namespace namespace, String relative) {
        return namespace.value() + "." + systemName + "." + relative;
    }

    /**
     * Build the subject for a node's event.
     *
     * @param systemName the system name
     * @param namespace  the namespace
     * @param nodeName   the node name
     * @param event      the event type
     * @return the full subject (e.g., "alice.monitor.cpu.data")
     */
    public static String eventSubject(String systemName, String namespace, String nodeName, String event) {
        return namespace + "." + systemName + "." + nodeName + "." + event;
    }

    /**
     * Build the fallback subject for a node.
     *
     * @param systemName the system name
     * @param namespace  the namespace
     * @param nodeName   the node name
     * @return the fallback subject (e.g., "alice.monitor.fallback.cpu")
     */
    public static String fallbackSubject(String systemName, Namespace namespace, String nodeName) {
        return namespace.value() + "." + systemName + ".fallback." + nodeName;
    }

    /**
     * Build the wildcard subject for an entire system.
     *
     * @param systemName the system name
     * @param namespace  the namespace
     * @return the wildcard subject (e.g., "alice.monitor.>")
     */
    public static String systemWildcard(String systemName, Namespace namespace) {
        return namespace.value() + "." + systemName + ".>";
    }
}
