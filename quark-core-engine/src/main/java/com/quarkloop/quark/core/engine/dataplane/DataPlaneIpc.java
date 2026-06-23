package com.quarkloop.quark.core.engine.dataplane;

/**
 * NATS subject conventions for control-plane ↔ data-plane IPC.
 *
 * <p>The control plane sends commands to data-plane processes on
 * namespace-scoped subjects. Data-plane processes respond with status
 * and send periodic heartbeats.
 *
 * <p>Subject layout:
 * <pre>
 *   quark.control.&lt;runtimeId&gt;.deploy      — control → data: deploy a system
 *   quark.control.&lt;runtimeId&gt;.undeploy    — control → data: undeploy a system
 *   quark.data.&lt;runtimeId&gt;.status         — data → control: deploy/undeploy result
 *   quark.data.&lt;runtimeId&gt;.heartbeat      — data → control: periodic health + metrics
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

    /** Subject prefix for data → control responses. */
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
        return DATA_PREFIX + runtimeId + ".status";
    }

    /**
     * Data → control: periodic heartbeat.
     * Payload: JSON {@code {"runtimeId":"shared","pid":12345,"systems":3,"nodes":18,"cpuPercent":12.5,...}}
     */
    public static String heartbeatSubject(String runtimeId) {
        return DATA_PREFIX + runtimeId + ".heartbeat";
    }
}
