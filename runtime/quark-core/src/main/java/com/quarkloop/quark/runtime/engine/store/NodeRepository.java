package com.quarkloop.quark.runtime.engine.store;

import java.util.List;
import java.util.Optional;

public interface NodeRepository {
    void save(NodeRecord node);
    void saveAll(List<NodeRecord> nodes);
    Optional<NodeRecord> find(String namespace, String systemName, String nodeName);
    List<NodeRecord> findBySystem(String namespace, String systemName);
    List<NodeRecord> findNodesByNamespace(String namespace);
    void delete(String namespace, String systemName, String nodeName);
    void deleteBySystem(String namespace, String systemName);
    void updateState(String namespace, String systemName, String nodeName,
                     String state, String health, long version, String errorMessage);
}
