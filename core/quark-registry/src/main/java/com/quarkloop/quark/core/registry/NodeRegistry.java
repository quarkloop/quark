package com.quarkloop.quark.core.registry;

import com.quarkloop.quark.core.domain.category.NodeCategory;
import com.quarkloop.quark.core.domain.identity.NodeUri;

import java.util.List;
import java.util.Optional;

/**
 * The central registry of all available node implementations.
 */
public interface NodeRegistry {

    Optional<NodeDescriptor> lookup(NodeUri uri);

    Optional<NodeImplementationFactory<?>> lookupFactory(NodeUri uri);

    void register(NodeImplementationFactory<?> factory);

    List<NodeDescriptor> search(String keyword);

    List<NodeDescriptor> listAll();

    List<NodeDescriptor> listByCategory(NodeCategory category);

    boolean isRegistered(NodeUri uri);
}
