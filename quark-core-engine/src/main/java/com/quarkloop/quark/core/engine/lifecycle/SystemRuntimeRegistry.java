package com.quarkloop.quark.core.engine.lifecycle;

import com.quarkloop.quark.core.domain.identity.Namespace;
import jakarta.enterprise.context.ApplicationScoped;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import java.util.Collection;
import java.util.List;
import java.util.Optional;
import java.util.concurrent.ConcurrentHashMap;
import java.util.concurrent.ConcurrentMap;

/**
 * Registry of currently-deployed {@link RuntimeSystem} instances.
 *
 * <p>Keyed by {@code <namespace>/<systemName>} so systems with the same
 * name in different namespaces don't collide.
 *
 * <p><b>Multi-tenancy</b>: this registry enforces namespace isolation at the
 * runtime level. There is deliberately NO cross-namespace lookup method.
 * Callers that need to find a system or node MUST supply the namespace
 * explicitly. The only exception is {@link #all()} which is reserved for
 * platform-level admin endpoints (and those endpoints are responsible for
 * filtering by namespace before returning data to clients).
 *
 * <p>This is an in-memory registry — systems are lost on restart. The
 * persistent state is maintained by {@code quark-adapter-state}; on startup
 * the platform re-deploys any previously-deployed system found on disk.
 */
@ApplicationScoped
public class SystemRuntimeRegistry {

    private static final Logger log = LoggerFactory.getLogger(SystemRuntimeRegistry.class);

    private final ConcurrentMap<String, RuntimeSystem> systems = new ConcurrentHashMap<>();

    public void register(RuntimeSystem system) {
        String key = key(system.namespace(), system.name());
        if (systems.containsKey(key)) {
            log.warn("Overwriting existing runtime system at {}", key);
        }
        systems.put(key, system);
    }

    public Optional<RuntimeSystem> get(Namespace namespace, String systemName) {
        return Optional.ofNullable(systems.get(key(namespace, systemName)));
    }

    /**
     * Remove a system from the runtime registry. Does NOT stop its nodes —
     * callers must invoke {@link LifecycleManager#stopAll} first.
     */
    public void remove(Namespace namespace, String systemName) {
        systems.remove(key(namespace, systemName));
    }

    /**
     * Returns all deployed systems across ALL namespaces.
     *
     * <p><b>Security note</b>: this method is intended for platform-level
     * admin operations only. REST endpoints that expose system data to
     * clients MUST filter by namespace before returning — never expose the
     * raw collection.
     */
    public Collection<RuntimeSystem> all() {
        return systems.values();
    }

    /**
     * Returns all deployed systems within a single namespace.
     */
    public List<RuntimeSystem> listByNamespace(Namespace namespace) {
        return systems.values().stream()
                .filter(t -> t.namespace().equals(namespace))
                .toList();
    }

    public void clear() {
        systems.clear();
    }

    /**
     * Look up a runtime node within a SPECIFIC system. Cross-namespace
     * lookups are impossible — the caller must supply the namespace.
     *
     * @return the node, or empty if the system doesn't exist in this
     *         namespace OR the node doesn't exist within that system.
     */
    public Optional<RuntimeNode> getNode(Namespace namespace, String systemName, String nodeName) {
        return get(namespace, systemName).map(t -> t.getNode(nodeName));
    }

    /**
     * Returns true if a system with this name exists in this namespace.
     */
    public boolean exists(Namespace namespace, String systemName) {
        return systems.containsKey(key(namespace, systemName));
    }

    private static String key(Namespace ns, String name) {
        return ns.value() + "/" + name;
    }
}
