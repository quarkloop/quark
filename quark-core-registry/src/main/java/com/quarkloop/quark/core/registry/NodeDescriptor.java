package com.quarkloop.quark.core.registry;

import com.fasterxml.jackson.annotation.JsonGetter;
import com.fasterxml.jackson.annotation.JsonIgnore;
import com.quarkloop.quark.core.domain.category.NodeCategory;
import com.quarkloop.quark.core.domain.identity.NodeUri;

import java.util.Objects;

/**
 * Describes a registered node implementation.
 *
 * <p>The {@code uri} field is serialized as its raw string form (e.g.
 * {@code "source/timer:v1"}) rather than the full NodeUri object, so
 * CLI clients can parse it as a plain string.
 */
public record NodeDescriptor(
        NodeUri uri,
        NodeCategory category,
        boolean active,
        String description
) {
    public NodeDescriptor {
        Objects.requireNonNull(uri, "uri cannot be null");
        Objects.requireNonNull(category, "category cannot be null");
        Objects.requireNonNull(description, "description cannot be null");
    }

    /**
     * Jackson uses this getter for the "uri" JSON field instead of
     * serializing the full NodeUri object. Returns the raw URI string.
     */
    @JsonGetter("uri")
    public String getUriString() {
        return uri.rawUri();
    }

    /**
     * Hide the record accessor from Jackson to avoid conflict with {@link #getUriString()}.
     */
    @JsonIgnore
    public NodeUri uri() {
        return uri;
    }
}
