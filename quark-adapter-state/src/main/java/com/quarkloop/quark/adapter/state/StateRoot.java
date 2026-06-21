package com.quarkloop.quark.adapter.state;

import org.eclipse.microprofile.config.inject.ConfigProperty;

import jakarta.enterprise.context.ApplicationScoped;
import java.nio.file.Path;
import java.nio.file.Paths;

/**
 * Resolves the filesystem root under which all platform state is persisted.
 */
@ApplicationScoped
public class StateRoot {

    private final Path root;

    public StateRoot(
            @ConfigProperty(name = "quark.state.root", defaultValue = "./quark-state")
            String rootPath
    ) {
        this.root = Paths.get(rootPath).toAbsolutePath().normalize();
    }

    public Path path() {
        return root;
    }

    public Path systemsDir() {
        return root.resolve("systems");
    }

    public Path systemDir(String namespace, String systemName) {
        return systemsDir().resolve(namespace).resolve(systemName);
    }

    public Path systemStateFile(String namespace, String systemName) {
        return systemDir(namespace, systemName).resolve("state.json");
    }

    public Path systemSourceFile(String namespace, String systemName) {
        return systemDir(namespace, systemName).resolve("source.ts");
    }

    public Path systemEventsFile(String namespace, String systemName) {
        return systemDir(namespace, systemName).resolve("events.jsonl");
    }

    public Path platformEventsFile() {
        return root.resolve("platform-events.jsonl");
    }

    @Override
    public String toString() {
        return "StateRoot{" + root + "}";
    }
}
