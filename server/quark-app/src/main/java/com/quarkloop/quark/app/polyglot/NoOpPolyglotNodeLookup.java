package com.quarkloop.quark.app.polyglot;

import com.quarkloop.quark.runtime.domain.identity.NodeUri;
import com.quarkloop.quark.runtime.engine.polyglot.PolyglotNodeLookup;
import com.quarkloop.quark.runtime.registry.NodeImplementationFactory;
import jakarta.enterprise.context.ApplicationScoped;

import java.util.Optional;

/**
 * No-op implementation of PolyglotNodeLookup for the control plane.
 *
 * <p>The control plane does not execute nodes — it delegates to the data plane
 * via NATS. Node implementations (including polyglot TypeScript nodes) are
 * loaded by the data plane, not the control plane. This no-op implementation
 * satisfies the CDI dependency so the server can start.
 */
@ApplicationScoped
public class NoOpPolyglotNodeLookup implements PolyglotNodeLookup {

    @Override
    public Optional<NodeImplementationFactory> lookupFactory(NodeUri uri) {
        return Optional.empty();
    }
}
