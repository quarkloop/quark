package com.quarkloop.quark.core.engine.polyglot;

import com.quarkloop.quark.core.domain.identity.NodeUri;
import com.quarkloop.quark.core.registry.NodeImplementationFactory;

import java.util.Optional;

/**
 * SPI for looking up node implementations from external registries
 * (e.g., the Catalog service's node package registry).
 *
 * <p>When a .quark.ts file references a URI that is not in the built-in
 * Java NodeRegistry, the SystemDeployer calls this interface to check
 * if the node is available from an external source.
 */
public interface PolyglotNodeLookup {
    Optional<NodeImplementationFactory> lookupFactory(NodeUri uri);
}
