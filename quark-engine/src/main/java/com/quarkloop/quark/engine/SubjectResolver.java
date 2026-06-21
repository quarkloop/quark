package com.quarkloop.quark.engine;

import com.quarkloop.quark.core.domain.identity.Namespace;

/**
 * Resolves relative subject references to full NATS subjects.
 *
 * <p>Relative subjects in .quark.ts files are short references like
 * {@code "timer.tick"} or {@code "fallback.cpu"}. The engine resolves them
 * to full NATS subjects by prefixing with the system name and namespace:
 * {@code <system>.<namespace>.<relative-subject>}.
 *
 * <p>Example:
 * <pre>
 *   systemName = "monitor"
 *   namespace  = "alice"
 *   relative   = "timer.tick"
 *   full       = "monitor.alice.timer.tick"
 * </pre>
 */
public final class SubjectResolver {

    private SubjectResolver() {}

    /**
     * Resolve a relative subject to a full NATS subject.
     *
     * @param systemName the system name (e.g., "monitor")
     * @param namespace  the namespace (e.g., "alice")
     * @param relative   the relative subject (e.g., "timer.tick")
     * @return the full subject (e.g., "monitor.alice.timer.tick")
     */
    public static String resolve(String systemName, Namespace namespace, String relative) {
        return systemName + "." + namespace.value() + "." + relative;
    }

    /**
     * Build the subject for a node's event.
     *
     * @param systemName the system name
     * @param namespace  the namespace
     * @param nodeName   the node name
     * @param event      the event type
     * @return the full subject (e.g., "monitor.alice.cpu.data")
     */
    public static String eventSubject(String systemName, String namespace, String nodeName, String event) {
        return systemName + "." + namespace + "." + nodeName + "." + event;
    }

    /**
     * Build the fallback subject for a node.
     *
     * @param systemName the system name
     * @param namespace  the namespace
     * @param nodeName   the node name
     * @return the fallback subject (e.g., "monitor.alice.fallback.cpu")
     */
    public static String fallbackSubject(String systemName, Namespace namespace, String nodeName) {
        return systemName + "." + namespace.value() + ".fallback." + nodeName;
    }

    /**
     * Build the wildcard subject for an entire system.
     *
     * @param systemName the system name
     * @param namespace  the namespace
     * @return the wildcard subject (e.g., "monitor.alice.>")
     */
    public static String systemWildcard(String systemName, Namespace namespace) {
        return systemName + "." + namespace.value() + ".>";
    }
}
