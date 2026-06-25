package com.quarkloop.quark.runtime.polyglot;

import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import java.util.Arrays;
import java.util.stream.Collectors;

/**
 * Host-access bridge that exposes a {@code console} global to TypeScript
 * nodes running inside GraalJS. GraalJS does not provide a {@code console}
 * object by default in sandboxed contexts, so without this bridge any node
 * that calls {@code console.log(...)} throws {@code ReferenceError: console
 * is not defined}.
 *
 * <p>Methods mirror the standard browser/Node.js console API. All output
 * is forwarded to SLF4J under the logger name
 * {@code quark.node.console.&lt;node-uri&gt;} so it is captured by the
 * data-plane log alongside the rest of the platform's structured logging.
 *
 * <p>Because GraalJS host-access marshalling flattens variadic arguments
 * into a single Java array parameter, every method accepts
 * {@code Object... args}.
 */
public class JsConsole {

    private static final Logger platformLog = LoggerFactory.getLogger("quark.node.console");

    private final String nodeUri;
    private final Logger nodeLog;

    public JsConsole(String nodeUri) {
        this.nodeUri = nodeUri;
        this.nodeLog = LoggerFactory.getLogger("quark.node.console." + sanitize(nodeUri));
    }

    public void log(Object... args) {
        nodeLog.info(format(args));
    }

    public void info(Object... args) {
        nodeLog.info(format(args));
    }

    public void debug(Object... args) {
        nodeLog.debug(format(args));
    }

    public void warn(Object... args) {
        nodeLog.warn(format(args));
    }

    public void error(Object... args) {
        nodeLog.error(format(args));
    }

    public void trace(Object... args) {
        nodeLog.trace(format(args));
    }

    /**
     * Format the variadic args into a single string the same way
     * {@code console.log} does in Node.js: join with a single space, with
     * each arg rendered through {@link String#valueOf} (objects get
     * {@code toString()}). GraalJS proxies values via host access, so
     * primitive JS values arrive as Java primitives; JS objects arrive as
     * {@link org.graalvm.polyglot.Value} whose {@code toString()} yields
     * a readable representation.
     */
    private static String format(Object[] args) {
        if (args == null || args.length == 0) return "";
        return Arrays.stream(args)
                .map(a -> a == null ? "null" : a.toString())
                .collect(Collectors.joining(" "));
    }

    /** Convert a URI like {@code quark/log/console/stdout:v1} to a logger-safe name. */
    private static String sanitize(String uri) {
        return uri == null ? "unknown" : uri.replaceAll("[^A-Za-z0-9._-]", "_");
    }
}
