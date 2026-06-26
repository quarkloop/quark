package com.quarkloop.quark.runtime.domain.identity;

import java.util.Objects;
import java.util.Optional;
import java.util.regex.Matcher;
import java.util.regex.Pattern;

/**
 * Represents a parsed and validated Node URI.
 *
 * <p>Format: {@code <namespace>/<domain>/<subdomain>/<node>:<version>}
 *
 * <p>Example: {@code quark/time/schedule/timer:v1}
 * <ul>
 *   <li>namespace = "quark"</li>
 *   <li>domain = "time"</li>
 *   <li>subdomain = "schedule"</li>
 *   <li>node = "timer"</li>
 *   <li>version = "v1"</li>
 * </ul>
 *
 * <p>Optional remote prefix: {@code quark://<instance>/<namespace>/<domain>/<subdomain>/<node>:<version>}
 */
public record NodeUri(
        String namespace,
        String domain,
        String subdomain,
        String node,
        String version,
        String rawUri,
        String instance
) {
    // Regex: optional quark://instance/ prefix, then 4 path segments, then :version
    // Group 1: Optional remote prefix (quark://instance/)
    // Group 2: instance
    // Group 3: namespace
    // Group 4: domain
    // Group 5: subdomain
    // Group 6: node (before colon)
    // Group 7: version (after colon)
    private static final Pattern URI_PATTERN = Pattern.compile(
            "^(?:quark://([^/]+)/)?" +           // optional instance prefix
            "([^/]+)/([^/]+)/([^/]+)/([^:]+):(.+)$"  // namespace/domain/subdomain/node:version
    );

    public NodeUri {
        Objects.requireNonNull(namespace, "namespace must not be null");
        Objects.requireNonNull(domain, "domain must not be null");
        Objects.requireNonNull(subdomain, "subdomain must not be null");
        Objects.requireNonNull(node, "node must not be null");
        Objects.requireNonNull(version, "version must not be null");
        Objects.requireNonNull(rawUri, "rawUri must not be null");
    }

    /**
     * Parses a raw URI string into a NodeUri.
     *
     * @param raw the raw URI string (e.g., "quark/time/schedule/timer:v1")
     * @return the parsed NodeUri
     * @throws IllegalArgumentException if the URI is null, blank, or malformed
     */
    public static NodeUri parse(String raw) {
        if (raw == null || raw.isBlank()) {
            throw new IllegalArgumentException("URI cannot be null or blank");
        }

        Matcher matcher = URI_PATTERN.matcher(raw);
        if (!matcher.matches()) {
            throw new IllegalArgumentException("Invalid URI format: " + raw +
                    " — expected <namespace>/<domain>/<subdomain>/<node>:<version>");
        }

        String instance = matcher.group(1);
        String namespace = matcher.group(2);
        String domain = matcher.group(3);
        String subdomain = matcher.group(4);
        String node = matcher.group(5);
        String version = matcher.group(6);

        return new NodeUri(namespace, domain, subdomain, node, version, raw, instance);
    }

    /**
     * Returns the URI pattern without the version, used for registry lookups.
     * Example: "quark/time/schedule/timer"
     */
    public String pattern() {
        return namespace + "/" + domain + "/" + subdomain + "/" + node;
    }

    /**
     * Returns the Java package derived from this URI.
     * Example: "quark/time/schedule/timer:v1" → "quark.time.schedule.timer"
     */
    public String packageName() {
        return namespace + "." + domain + "." + subdomain + "." + node.replace('-', '_');
    }

    public boolean isRemote() {
        return instance != null && !instance.isBlank();
    }

    public Optional<String> getInstance() {
        return Optional.ofNullable(instance);
    }

    @Override
    public String toString() {
        return rawUri;
    }
}
