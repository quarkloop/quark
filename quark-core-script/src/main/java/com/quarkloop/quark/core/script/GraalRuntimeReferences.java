package com.quarkloop.quark.core.script;

/**
 * Force Quarkus to include the GraalJS runtime jars in the deployment.
 *
 * <p>Quarkus's build-time dependency analysis strips jars whose classes
 * are not directly referenced in application bytecode. The GraalJS
 * Engine discovers its implementation via {@link java.util.ServiceLoader}
 * at runtime, so no application code directly references
 * {@code com.oracle.truffle.*} or {@code com.oracle.truffle.regex.*}
 * classes — and Quarkus prunes {@code truffle-api.jar} and
 * {@code regex.jar}.
 *
 * <p>Without those jars, the Engine fails with:
 * <pre>
 *   NoClassDefFoundError: org/graalvm/polyglot/impl/AbstractPolyglotImpl
 * </pre>
 * because the ServiceLoader file
 * {@code META-INF/services/org.graalvm.polyglot.impl.AbstractPolyglotImpl}
 * (which points to {@code com.oracle.truffle.polyglot.PolyglotImpl}) lives
 * inside {@code truffle-api.jar}.
 *
 * <p>This class holds direct compile-time references to one class from
 * each required jar. Quarkus's bytecode analysis detects these references
 * and marks the jars as non-removable.
 */
final class GraalRuntimeReferences {

    private GraalRuntimeReferences() {}

    /** From {@code truffle-api.jar} — the Truffle language SPI. */
    static final Class<?> TRUFFLE_LANGUAGE = com.oracle.truffle.api.TruffleLanguage.class;

    /** From {@code truffle-api.jar} — the concrete polyglot implementation. */
    static final Class<?> POLYGLOT_IMPL = com.oracle.truffle.polyglot.PolyglotImpl.class;

    /** From {@code regex.jar} — the Truffle regex language. */
    static final Class<?> REGEX_LANGUAGE = com.oracle.truffle.regex.RegexLanguage.class;
}
