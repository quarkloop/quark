package com.quarkloop.quark.core.engine.store;

import java.util.List;
import java.util.Optional;

public interface SystemRepository {
    void save(SystemRecord system);
    Optional<SystemRecord> findByNamespaceAndName(String namespace, String name);
    List<SystemRecord> findByNamespace(String namespace);
    List<SystemRecord> findAllSystems();
    void delete(String namespace, String name);
    void updateState(String namespace, String name, String state, String health, long version);
}
