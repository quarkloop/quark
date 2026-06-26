package com.quarkloop.quark.runtime.domain.node;

import com.quarkloop.quark.runtime.domain.config.NodeConfig;
import com.quarkloop.quark.runtime.domain.identity.NodeUri;
import com.quarkloop.quark.runtime.domain.metadata.Annotations;
import com.quarkloop.quark.runtime.domain.metadata.Labels;
import com.quarkloop.quark.runtime.domain.metadata.NodeMetadata;

/**
 * The base abstraction for everything in Quark.
 *
 * <p>A Node is identified by its URI, has a name, configuration, and metadata.
 * There are no behavioral categories — the domain (from the URI) is the only
 * organizational axis. The runtime behavior is determined by which methods the
 * node's {@link com.quarkloop.quark.runtime.domain.spi.NodeProvider} implementation
 * overrides.
 */
public interface Node {

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
}
