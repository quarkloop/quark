package com.quarkloop.quark.runtime.registry;

import com.quarkloop.quark.runtime.domain.identity.NodeUri;

import java.util.List;
import java.util.Optional;

/**
 * The central registry of all available node implementations.
 */
public interface NodeRegistry {

    Optional<NodeDescriptor> lookup(NodeUri uri);

    Optional<NodeImplementationFactory> lookupFactory(NodeUri uri);

    void register(NodeImplementationFactory factory);

    List<NodeDescriptor> search(String keyword);

    List<NodeDescriptor> listAll();

    boolean isRegistered(NodeUri uri);
}
