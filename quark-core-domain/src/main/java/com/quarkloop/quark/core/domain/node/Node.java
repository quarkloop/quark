package com.quarkloop.quark.core.domain.node;

import com.quarkloop.quark.core.domain.category.NodeCategory;
import com.quarkloop.quark.core.domain.config.NodeConfig;
import com.quarkloop.quark.core.domain.identity.NodeUri;
import com.quarkloop.quark.core.domain.metadata.Annotations;
import com.quarkloop.quark.core.domain.metadata.Labels;
import com.quarkloop.quark.core.domain.metadata.NodeMetadata;

/**
 * The base abstraction for everything in Quark.
 */
public sealed interface Node permits PassiveNode, ActiveNode {

    String name();

    NodeUri uri();

    NodeConfig config();

    NodeMetadata metadata();

    default Labels labels() {
        return metadata().labels();
    }

    default Annotations annotations() {
        return metadata().annotations();
    }

    NodeCategory category();

    default boolean isPassive() {
        return this instanceof PassiveNode;
    }

    default boolean isActive() {
        return this instanceof ActiveNode;
    }
}
