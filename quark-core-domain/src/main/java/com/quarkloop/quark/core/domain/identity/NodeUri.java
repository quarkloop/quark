package com.quarkloop.quark.core.domain.identity;

import com.quarkloop.quark.core.domain.category.NodeCategory;

import java.util.Objects;
import java.util.Optional;
import java.util.regex.Matcher;
import java.util.regex.Pattern;

/**
 * Represents a parsed and validated Node URI.
 * Format: [quark://instance/namespace/]<category>/<implementation>:<version>
 */
public record NodeUri(
        NodeCategory category,
        String implementation,
        String version,
        String rawUri,
        String instance,
        String namespace
) {
    // Regex to parse the URI. Handles optional remote prefix.
    // Group 1: Optional remote prefix (quark://instance/namespace/)
    // Group 2: Category
    // Group 3: Implementation
    // Group 4: Version
    private static final Pattern URI_PATTERN = Pattern.compile("^(quark://([^/]+)/([^/]+)/)?([^/]+)/(.+):([^:]+)$");

    public NodeUri {
        Objects.requireNonNull(category, "category must not be null");
        if (implementation == null || implementation.isBlank()) {
            throw new IllegalArgumentException("implementation must not be null or blank");
        }
        if (version == null || version.isBlank()) {
            throw new IllegalArgumentException("version must not be null or blank");
        }
        Objects.requireNonNull(rawUri, "rawUri must not be null");
    }

    /**
     * Parses a raw URI string into a NodeUri.
     */
    public static NodeUri parse(String raw) {
        if (raw == null || raw.isBlank()) {
            throw new IllegalArgumentException("URI cannot be null or blank");
        }

        Matcher matcher = URI_PATTERN.matcher(raw);
        if (!matcher.matches()) {
            throw new IllegalArgumentException("Invalid URI format: " + raw);
        }

        String instance = matcher.group(2);
        String namespace = matcher.group(3);
        String categoryStr = matcher.group(4);
        String implementation = matcher.group(5);
        String version = matcher.group(6);

        NodeCategory category = NodeCategory.fromLabel(categoryStr);

        return new NodeUri(category, implementation, version, raw, instance, namespace);
    }

    public boolean isRemote() {
        return instance != null && !instance.isBlank();
    }

    public Optional<String> getInstance() {
        return Optional.ofNullable(instance);
    }

    public Optional<String> getNamespace() {
        return Optional.ofNullable(namespace);
    }

    @Override
    public String toString() {
        return rawUri;
    }
}
