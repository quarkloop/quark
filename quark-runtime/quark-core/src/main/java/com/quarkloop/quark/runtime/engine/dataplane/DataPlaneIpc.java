package com.quarkloop.quark.runtime.engine.dataplane;

/**
 * NATS subject conventions for control-plane ↔ data-plane IPC.
 *
 * <p>The control plane sends commands to data-plane processes on
 * namespace-scoped subjects. Data-plane processes respond with status
 * and send periodic heartbeats and event forwards.
 *
 * <p>Subject layout (NATS wildcards: {@code *} matches one token,
 * {@code >} matches one or more tokens at the END):
 * <pre>
 *   quark.control.&lt;runtimeId&gt;.deploy      — control → data: deploy a system
 *   quark.control.&lt;runtimeId&gt;.undeploy    — control → data: undeploy a system
 *   quark.data.status.&lt;runtimeId&gt;         — data → control: deploy/undeploy result
 *   quark.data.heartbeat.&lt;runtimeId&gt;      — data → control: periodic metrics
 *   quark.data.event.&lt;runtimeId&gt;          — data → control: lifecycle event forward
 * </pre>
 *
 * <p>Wildcards for control-plane subscription:
 * <pre>
 *   quark.data.event.>       — receive events from ALL data planes
 *   quark.data.heartbeat.>   — receive heartbeats from ALL data planes
 * </pre>
 *
 * <p>{@code runtimeId} identifies which data-plane process should handle the
 * command:
 * <ul>
 *   <li>For shared namespaces: {@code "shared"} — all non-isolated namespaces
 *       run in the same data-plane process.</li>
 *   <li>For isolated namespaces: {@code "ns-&lt;namespace&gt;"} — a dedicated
 *       data-plane process per namespace.</li>
 * </ul>
 */
public final class DataPlaneIpc {

    private DataPlaneIpc() {}

    /** The runtimeId for the shared data-plane process. */
    public static final String SHARED_RUNTIME_ID = "shared";

    /** Subject prefix for control → data commands. */
    public static final String CONTROL_PREFIX = "quark.control.";

    /** Subject prefix for data → control responses/events/heartbeats. */
    public static final String DATA_PREFIX = "quark.data.";

    /**
     * Build the runtimeId for a namespace.
     *
     * @param namespace   the namespace name
     * @param isIsolated  true if the namespace runs in an isolated data-plane process
     * @return "shared" for non-isolated namespaces, "ns-&lt;namespace&gt;" for isolated
     */
    public static String runtimeId(String namespace, boolean isIsolated) {
        return isIsolated ? "ns-" + namespace : SHARED_RUNTIME_ID;
    }

    /**
     * Control → data: deploy a system.
     * Payload: JSON {@code {"namespace":"alice","systemName":"monitor","source":"..."}}
     */
    public static String deploySubject(String runtimeId) {
        return CONTROL_PREFIX + runtimeId + ".deploy";
    }

    /**
     * Control → data: undeploy a system.
     * Payload: JSON {@code {"namespace":"alice","systemName":"monitor"}}
     */
    public static String undeploySubject(String runtimeId) {
        return CONTROL_PREFIX + runtimeId + ".undeploy";
    }

    /**
     * Data → control: status response for a deploy/undeploy command.
     * Payload: JSON {@code {"success":true,"error":"...","systemName":"monitor","namespace":"alice"}}
     */
    public static String statusSubject(String runtimeId) {
        return DATA_PREFIX + "status." + runtimeId;
    }

    /**
     * Data → control: periodic heartbeat with metrics.
     * Payload: JSON map of namespace → Snapshot
     */
    public static String heartbeatSubject(String runtimeId) {
        return DATA_PREFIX + "heartbeat." + runtimeId;
    }

    /**
     * Data → control: lifecycle event forwarding.
     * Payload: JSON-serialized {@link com.quarkloop.quark.runtime.domain.event.NodeEvent}.
     */
    public static String eventSubject(String runtimeId) {
        return DATA_PREFIX + "event." + runtimeId;
    }

    /**
     * Wildcard subject for subscribing to events from ALL data-plane processes.
     * NATS {@code >} matches one or more tokens at the END of the subject.
     */
    public static final String EVENT_WILDCARD = DATA_PREFIX + "event.>";

    /**
     * Wildcard subject for subscribing to heartbeats from ALL data-plane processes.
     */
    public static final String HEARTBEAT_WILDCARD = DATA_PREFIX + "heartbeat.>";
}
