package com.quarkloop.quark.runtime.script;

import com.quarkloop.quark.runtime.domain.system.SystemDefinition;

import java.util.List;

/**
 * Result of parsing a .quark.ts file.
 *
 * <p>Sealed interface — callers must handle both branches at compile time.
 */
public sealed interface SystemParseResult {

    record Success(SystemDefinition system) implements SystemParseResult {}

    record Failure(List<String> errors) implements SystemParseResult {}
}
