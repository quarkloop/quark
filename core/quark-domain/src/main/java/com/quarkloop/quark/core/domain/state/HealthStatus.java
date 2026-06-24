package com.quarkloop.quark.core.domain.state;

/**
 * The health status of an active node.
 */
public enum HealthStatus {
    HEALTHY,
    DEGRADED,
    UNHEALTHY,
    UNKNOWN
}
