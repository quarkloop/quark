package com.quarkloop.quark.core.domain.node;

/**
 * A node that executes behavior, consumes inputs, and produces outputs.
 */
public sealed interface ActiveNode extends Node permits Function {
}
