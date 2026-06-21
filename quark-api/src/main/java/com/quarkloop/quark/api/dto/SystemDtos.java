package com.quarkloop.quark.api.dto;

import com.fasterxml.jackson.annotation.JsonInclude;

import java.util.List;

/**
 * Request and response DTOs for the {@code /systems} endpoints.
 *
 * <p>These mirror the {@code model/system.go} structs in the Go CLI so the
 * wire format is identical on both sides.
 */
public final class SystemDtos {

    private SystemDtos() {}

    public record DeploySystemRequest(
            String source,
            String namespace
    ) {
        public DeploySystemRequest {
            if (source == null || source.isBlank()) {
                throw new IllegalArgumentException("source is required");
            }
        }
    }

    @JsonInclude(JsonInclude.Include.NON_NULL)
    public record DeploySystemResponse(
            String name,
            String namespace,
            int nodeCount,
            String state,
            String health,
            List<String> nodes
    ) {}

    public record DeploySystemFailure(
            String message,
            List<ValidationError> errors
    ) {}

    public record ValidationError(
            String path,
            String message,
            String severity
    ) {}
}
