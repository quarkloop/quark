package com.quarkloop.quark.core.domain.system;

import com.quarkloop.quark.core.domain.config.NodeConfig;
import com.quarkloop.quark.core.domain.identity.NodeUri;
import com.quarkloop.quark.core.domain.metadata.Annotations;
import com.quarkloop.quark.core.domain.metadata.Labels;

import java.util.List;
import java.util.Objects;

/**
 * Parsed definition of a single node from a .quark.ts file.
 *
 * <p>Contains the node's URI, configuration, and NATS communication patterns
 * (listens, events, onFailure). This is the declarative contract the engine
 * uses to create NATS consumers and publish ACLs.
 */
public record NodeDefinition(
        String name,
        NodeUri uri,
        NodeConfig config,
        List<String> listens,
        List<String> events,
        OnFailureConfig onFailure,
        String timeout,
        Labels labels,
        Annotations annotations
) {
    public NodeDefinition {
        Objects.requireNonNull(name, "name cannot be null");
        Objects.requireNonNull(uri, "uri cannot be null");
        if (config == null) config = NodeConfig.empty();
        if (listens == null) listens = List.of(); else listens = List.copyOf(listens);
        if (events == null) events = List.of(); else events = List.copyOf(events);
        if (labels == null) labels = Labels.empty();
        if (annotations == null) annotations = Annotations.empty();
    }
}
