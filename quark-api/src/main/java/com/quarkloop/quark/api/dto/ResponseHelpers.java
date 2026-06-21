package com.quarkloop.quark.api.dto;

import jakarta.ws.rs.core.Response;

import java.util.Optional;
import java.util.function.Supplier;

/**
 * Helpers for building JAX-RS {@link Response}s from {@link Optional}s.
 */
public final class ResponseHelpers {

    private ResponseHelpers() {}

    /**
     * If the optional has a value, return {@code 200 OK} with that value
     * as the JSON body. Otherwise return {@code 404 Not Found}.
     */
    public static Response okOr404(Optional<?> opt) {
        return opt
                .map(v -> Response.ok(v).build())
                .orElseGet(() -> Response.status(Response.Status.NOT_FOUND).build());
    }

    /**
     * Like {@link #okOr404(Optional)} but lets the caller build the
     * response body via a supplier on the optional's value.
     */
    public static <T> Response okOr404(Optional<T> opt, java.util.function.Function<T, Object> bodyFn) {
        return opt.<Response>map(v -> Response.ok(bodyFn.apply(v)).build())
                .orElseGet(() -> Response.status(Response.Status.NOT_FOUND).build());
    }
}
