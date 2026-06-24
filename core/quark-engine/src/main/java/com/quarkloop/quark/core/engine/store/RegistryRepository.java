package com.quarkloop.quark.core.engine.store;

import java.util.List;
import java.util.Optional;

public interface RegistryRepository {
    void save(RegistryRecord record);
    Optional<RegistryRecord> findByUri(String uri);
    List<RegistryRecord> findAllRegistry();
    List<RegistryRecord> search(String keyword);
    boolean existsByUri(String uri);
}
