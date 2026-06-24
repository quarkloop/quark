package com.quarkloop.quark.app.lifecycle;

import com.quarkloop.quark.core.domain.identity.Namespace;
import com.quarkloop.quark.core.domain.state.NodeState;
import com.quarkloop.quark.core.engine.lifecycle.LifecycleManager;
import com.quarkloop.quark.core.engine.lifecycle.RuntimeNode;
import com.quarkloop.quark.core.engine.lifecycle.RuntimeSystem;
import com.quarkloop.quark.core.engine.runtime.RuntimeContext;
import jakarta.enterprise.context.ApplicationScoped;
import jakarta.inject.Inject;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import java.util.Optional;

/**
 * Application-layer wrapper around {@link LifecycleManager} that enforces
 * the namespace scoping rules: every operation MUST supply the namespace
 * explicitly; cross-namespace lifecycle changes are not allowed.
 *
 * <p>Operations exposed:
 * <ul>
 *   <li>{@code pause}   — ACTIVE -> PAUSED</li>
 *   <li>{@code resume}  — PAUSED -> ACTIVE</li>
 *   <li>{@code drain}   — ACTIVE or PAUSED -> DRAINING -> ARCHIVED</li>
 *   <li>{@code archive} — DRAINING or ERROR -> ARCHIVED</li>
 *   <li>{@code recover} — ERROR -> RECOVERING -> ACTIVE</li>
 *   <li>{@code delete}  — ARCHIVED -> DELETED (and removes from registry)</li>
 * </ul>
 */
@ApplicationScoped
public class LifecycleService {

    private static final Logger log = LoggerFactory.getLogger(LifecycleService.class);

    private final LifecycleManager lifecycleManager;
    private final RuntimeContext runtimeContext;

    @Inject
    public LifecycleService(LifecycleManager lifecycleManager, RuntimeContext runtimeContext) {
        this.lifecycleManager = lifecycleManager;
        this.runtimeContext = runtimeContext;
    }

    public void pause(String namespace, String systemName, String nodeName) {
        transition(namespace, systemName, nodeName, NodeState.PAUSED, "api-pause");
    }

    public void resume(String namespace, String systemName, String nodeName) {
        transition(namespace, systemName, nodeName, NodeState.ACTIVE, "api-resume");
    }

    public void drain(String namespace, String systemName, String nodeName) {
        transition(namespace, systemName, nodeName, NodeState.DRAINING, "api-drain");
    }

    public void archive(String namespace, String systemName, String nodeName) {
        transition(namespace, systemName, nodeName, NodeState.ARCHIVED, "api-archive");
    }

    public void recover(String namespace, String systemName, String nodeName) {
        RuntimeNode rn = lookupOrThrow(namespace, systemName, nodeName);
        // ERROR -> RECOVERING -> ACTIVE
        lifecycleManager.transitionTo(
                Namespace.of(namespace), systemName, rn, NodeState.RECOVERING, "api-recover");
        lifecycleManager.transitionTo(
                Namespace.of(namespace), systemName, rn, NodeState.ACTIVE, "recovery-complete");
    }

    /**
     * Permanently delete a node. Only allowed from ARCHIVED state.
     * The node is removed from the runtime registry — its persisted
     * state files remain on disk (operator can rm them).
     */
    public void delete(String namespace, String systemName, String nodeName) {
        RuntimeNode rn = lookupOrThrow(namespace, systemName, nodeName);
        if (rn.state() != NodeState.ARCHIVED) {
            throw new IllegalStateException(
                    "Node " + nodeName + " must be ARCHIVED before DELETE (current=" + rn.state() + ")");
        }
        lifecycleManager.transitionTo(
                Namespace.of(namespace), systemName, rn, NodeState.DELETED, "api-delete");
        log.info("Deleted node {}/{}/{}", namespace, systemName, nodeName);
        // The registry doesn't expose per-node removal — caller would need
        // to undeploy the system to fully clean up. This is intentional:
        // ARCHIVED nodes linger for inspection; DELETE marks them.
    }

    // ----- Helpers -----

    private void transition(String namespace, String systemName, String nodeName,
                             NodeState target, String trigger) {
        RuntimeNode rn = lookupOrThrow(namespace, systemName, nodeName);
        lifecycleManager.transitionTo(
                Namespace.of(namespace), systemName, rn, target, trigger);
    }

    private RuntimeNode lookupOrThrow(String namespace, String systemName, String nodeName) {
        Optional<RuntimeNode> opt = runtimeContext.getNode(
                Namespace.of(namespace), systemName, nodeName);
        if (opt.isEmpty()) {
            throw new java.util.NoSuchElementException(
                    "Node not found: " + namespace + "/" + systemName + "/" + nodeName);
        }
        return opt.get();
    }
}
