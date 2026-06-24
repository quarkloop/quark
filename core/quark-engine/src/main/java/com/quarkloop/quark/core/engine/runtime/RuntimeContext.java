package com.quarkloop.quark.core.engine.runtime;

import com.quarkloop.quark.core.domain.identity.Namespace;
import com.quarkloop.quark.core.domain.spi.NodeProvider;
import com.quarkloop.quark.core.engine.bus.Subscription;
import com.quarkloop.quark.core.engine.lifecycle.RuntimeNode;
import com.quarkloop.quark.core.engine.lifecycle.RuntimeSystem;
import jakarta.enterprise.context.ApplicationScoped;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import java.util.ArrayList;
import java.util.Collection;
import java.util.List;
import java.util.Optional;
import java.util.concurrent.ConcurrentHashMap;
import java.util.concurrent.ConcurrentMap;
import java.util.concurrent.locks.ReentrantLock;

@ApplicationScoped
public final class RuntimeContext {

    private static final Logger log = LoggerFactory.getLogger(RuntimeContext.class);

    private final ConcurrentMap<String, ReentrantLock> systemLocks = new ConcurrentHashMap<>();
    private final ConcurrentMap<String, RuntimeSystem> systems = new ConcurrentHashMap<>();
    private final ConcurrentMap<String, SystemRuntimeState> systemStates = new ConcurrentHashMap<>();

    public void lock(String namespace, String systemName) {
        systemLocks.computeIfAbsent(key(namespace, systemName), k -> new ReentrantLock()).lock();
    }

    public void unlock(String namespace, String systemName) {
        ReentrantLock lock = systemLocks.get(key(namespace, systemName));
        if (lock != null && lock.isHeldByCurrentThread()) lock.unlock();
    }

    public void registerSystem(RuntimeSystem system) {
        String k = key(system.namespace().value(), system.name());
        RuntimeSystem existing = systems.put(k, system);
        if (existing != null) log.warn("Overwrote existing runtime system at {}", k);
    }

    public Optional<RuntimeSystem> getSystem(String namespace, String systemName) {
        return Optional.ofNullable(systems.get(key(namespace, systemName)));
    }

    public Optional<RuntimeSystem> getSystem(Namespace namespace, String systemName) {
        return getSystem(namespace.value(), systemName);
    }

    public void removeSystem(String namespace, String systemName) {
        systems.remove(key(namespace, systemName));
    }

    public Collection<RuntimeSystem> getAllSystems() { return systems.values(); }

    public List<RuntimeSystem> getSystemsByNamespace(String namespace) {
        return systems.values().stream().filter(rs -> rs.namespace().value().equals(namespace)).toList();
    }

    public List<RuntimeSystem> getSystemsByNamespace(Namespace namespace) {
        return getSystemsByNamespace(namespace.value());
    }

    public Optional<RuntimeNode> getNode(String namespace, String systemName, String nodeName) {
        return getSystem(namespace, systemName).map(rs -> rs.getNode(nodeName));
    }

    public Optional<RuntimeNode> getNode(Namespace namespace, String systemName, String nodeName) {
        return getNode(namespace.value(), systemName, nodeName);
    }

    public void recordStartableProvider(String ns, String sys, String node, NodeProvider provider) {
        systemStates.computeIfAbsent(key(ns, sys), k -> new SystemRuntimeState()).providers.put(node, provider);
    }

    public void recordSubscriptions(String ns, String sys, String node, List<Subscription> subs) {
        systemStates.computeIfAbsent(key(ns, sys), k -> new SystemRuntimeState()).subscriptions.put(node, new ArrayList<>(subs));
    }

    public Collection<NodeProvider> getStartableProviders(String ns, String sys) {
        SystemRuntimeState s = systemStates.get(key(ns, sys));
        return s == null ? List.of() : s.providers.values();
    }

    public Collection<Subscription> getSubscriptions(String ns, String sys) {
        SystemRuntimeState s = systemStates.get(key(ns, sys));
        if (s == null) return List.of();
        List<Subscription> all = new ArrayList<>();
        s.subscriptions.values().forEach(all::addAll);
        return all;
    }

    public void clear(String ns, String sys) {
        systemStates.remove(key(ns, sys));
    }

    private static String key(String ns, String sys) { return ns + "/" + sys; }

    private static final class SystemRuntimeState {
        final ConcurrentMap<String, NodeProvider> providers = new ConcurrentHashMap<>();
        final ConcurrentMap<String, List<Subscription>> subscriptions = new ConcurrentHashMap<>();
    }
}
