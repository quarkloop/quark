package com.quarkloop.quark.runtime.engine.lifecycle;

/**
 * Thrown when a system cannot be deployed.
 *
 * <p>Carries a human-readable message describing the root cause: missing
 * node URI, factory failure, configuration error, etc.
 */
public class DeploymentException extends RuntimeException {

    public DeploymentException(String message) {
        super(message);
    }

    public DeploymentException(String message, Throwable cause) {
        super(message, cause);
    }
}
