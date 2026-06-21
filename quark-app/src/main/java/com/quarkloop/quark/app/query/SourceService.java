package com.quarkloop.quark.app.query;

import com.quarkloop.quark.adapter.state.StateRoot;
import jakarta.enterprise.context.ApplicationScoped;
import jakarta.inject.Inject;

import java.io.IOException;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.util.Optional;

/**
 * Read the persisted {@code source.ts} for a system.
 */
@ApplicationScoped
public class SourceService {

    private final StateRoot stateRoot;

    @Inject
    public SourceService(StateRoot stateRoot) {
        this.stateRoot = stateRoot;
    }

    public Optional<String> getSource(String namespace, String systemName) {
        var file = stateRoot.systemSourceFile(namespace, systemName);
        if (!Files.isRegularFile(file)) return Optional.empty();
        try {
            return Optional.of(Files.readString(file, StandardCharsets.UTF_8));
        } catch (IOException e) {
            throw new IllegalStateException("Failed to read source file: " + file, e);
        }
    }
}
