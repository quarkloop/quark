package com.quarkloop.quark.core.engine.lifecycle;

import com.quarkloop.quark.core.domain.event.NodeEvent;
import com.quarkloop.quark.core.domain.event.NodeEventKind;
import com.quarkloop.quark.core.domain.identity.Namespace;
import com.quarkloop.quark.core.domain.node.Node;
import com.quarkloop.quark.core.domain.state.HealthStatus;
import com.quarkloop.quark.core.domain.state.NodeState;
import com.quarkloop.quark.core.domain.state.LifecycleStateMachine;
import com.quarkloop.quark.core.domain.state.StateTransition;
import com.quarkloop.quark.core.event.EventBus;
import jakarta.enterprise.context.ApplicationScoped;
import jakarta.inject.Inject;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import java.util.Map;

/**
 * Manages lifecycle state transitions for runtime nodes.
 */
@ApplicationScoped
public class LifecycleManager {

    private static final Logger log = LoggerFactory.getLogger(LifecycleManager.class);

    private final EventBus eventBus;
    private final SystemRuntimeRegistry registry;

    @Inject
    public LifecycleManager(EventBus eventBus, SystemRuntimeRegistry registry) {
        this.eventBus = eventBus;
        this.registry = registry;
    }

    public void startAll(RuntimeSystem system) {
        for (RuntimeNode rn : system.nodes()) {
            Node def = rn.definition();
            log.info("Starting node {} in system {}", def.name(), system.name());
            try {
                transitionTo(system.namespace(), system.name(), rn, NodeState.ACTIVE, "deploy");
                rn.setHealth(HealthStatus.HEALTHY);
            } catch (Exception e) {
                log.error("Failed to start node {} in system {}", def.name(), system.name(), e);
                rn.setErrorMessage(e.getMessage());
                transitionTo(system.namespace(), system.name(), rn, NodeState.ERROR, "start-failure");
                rn.setHealth(HealthStatus.UNHEALTHY);
            }
        }
    }

    public void stopAll(RuntimeSystem system) {
        for (RuntimeNode rn : system.nodes()) {
            try {
                if (rn.state() != NodeState.ERROR) {
                    transitionTo(system.namespace(), system.name(), rn, NodeState.DRAINING, "undeploy");
                    transitionTo(system.namespace(), system.name(), rn, NodeState.ARCHIVED, "drain-complete");
                }
            } catch (Exception e) {
                log.error("Failed to stop node {} in system {}", rn.definition().name(), system.name(), e);
            }
        }
    }

    public StateTransition transitionTo(
            Namespace namespace, String systemName,
            RuntimeNode rn, NodeState target, String trigger) {
        // Synchronize on the runtime node so the read-validate-write
        // sequence is atomic. Without this, two concurrent lifecycle
        // operations on the same node could both read the same `from`
        // state, both validate, both write — producing duplicate events
        // and a double version increment.
        synchronized (rn) {
            NodeState from = rn.state();
            if (from == target) {
                log.debug("No-op transition for {} (already {})", rn.definition().name(), from);
                return new StateTransition(from, target, trigger, java.time.Instant.now());
            }
            if (!LifecycleStateMachine.isValidTransition(from, target)) {
                throw new IllegalStateException(String.format(
                        "Invalid state transition for node %s in system %s/%s: %s -> %s",
                        rn.definition().name(), namespace.value(), systemName, from, target));
            }
            rn.setState(target);
            StateTransition t = new StateTransition(from, target, trigger, java.time.Instant.now());
            NodeEvent event = NodeEvent.of(
                    NodeEventKind.NODE_STATE_CHANGED,
                    rn.definition().name(),
                    systemName,
                    namespace.value(),
                    Map.of("from", from.name(), "to", target.name(), "trigger", trigger));
            eventBus.publish(event);
            log.debug("Transitioned {}/{} {} : {} -> {}",
                    namespace.value(), systemName, rn.definition().name(), from, target);
            return t;
        }
    }

    public RuntimeNode getNode(Namespace namespace, String systemName, String nodeName) {
        return registry.getNode(namespace, systemName, nodeName).orElse(null);
    }
}
