package com.quarkloop.quark.core.script;

import com.quarkloop.quark.core.domain.system.SystemDefinition;

/**
 * Interface for parsing .quark.ts files into {@link SystemDefinition} objects.
 *
 * <p>Implementations use GraalJS to evaluate the TypeScript/JavaScript source
 * in a sandboxed context. The user's code exports a default object which
 * the parser extracts and converts to a Java {@link SystemDefinition}.
 */
public interface SystemParser {

    /**
     * Parse a .quark.ts file's source code into a SystemDefinition.
     *
     * @param sourceCode the TypeScript/JavaScript source code
     * @return parse result (Success or Failure)
     */
    SystemParseResult parse(String sourceCode);
}
