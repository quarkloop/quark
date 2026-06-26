package com.quarkloop.quark.runtime.engine.lifecycle;

import com.quarkloop.quark.runtime.domain.node.Node;
import com.quarkloop.quark.runtime.domain.state.HealthStatus;
import com.quarkloop.quark.runtime.domain.state.NodeState;

import java.util.concurrent.atomic.AtomicLong;

/**
 * Mutable runtime wrapper for a single node instance within a deployed system.
 *
 * <p>Holds the immutable {@link Node} definition plus the runtime SPI
 * provider instance and the node's current lifecycle state. State
 * transitions are guarded by the {@link LifecycleManager} (which enforces
 * the {@link com.quarkloop.quark.runtime.domain.state.LifecycleStateMachine}).
 *
 * <p><b>Thread-safety</b>: all mutable fields are {@code volatile} or atomic.
 * The {@code version} counter is an {@link AtomicLong} so concurrent state
 * transitions cannot lose increments. The {@code errorMessage} write is
 * paired with the state write under the node's monitor (synchronized in
 * {@link LifecycleManager#transitionTo}). Reads are lock-free.
 */
public final class RuntimeNode {

    private final Node definition;
    private final Object spiProvider;
    private volatile NodeState state;
    private volatile HealthStatus health;
    private volatile String errorMessage;
    private final AtomicLong version;

    public RuntimeNode(Node definition, Object spiProvider) {
        this.definition = definition;
        this.spiProvider = spiProvider;
        this.state = NodeState.CREATING;
        this.health = HealthStatus.UNKNOWN;
        this.version = new AtomicLong(1L);
    }

    public Node definition() {
        return definition;
    }

    public Object spiProvider() {
        return spiProvider;
    }

    public NodeState state() {
        return state;
    }

    /** Package-private — only {@link LifecycleManager} should call this. */
    void setState(NodeState newState) {
        this.state = newState;
        this.version.incrementAndGet();
    }

    public HealthStatus health() {
        return health;
    }

    public void setHealth(HealthStatus newHealth) {
        this.health = newHealth;
    }

    public String errorMessage() {
        return errorMessage;
    }

    public void setErrorMessage(String msg) {
        this.errorMessage = msg;
    }

    public long version() {
        return version.get();
    }

    @Override
    public String toString() {
        return "RuntimeNode{" + definition.name() + " state=" + state + " health=" + health + "}";
    }
}
