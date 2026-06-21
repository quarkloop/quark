package com.quarkloop.quark.core.domain.node;

/**
 * A node that describes something but does not execute behavior.
 */
public sealed interface PassiveNode extends Node permits Source, Store, Endpoint, Policy {
}
