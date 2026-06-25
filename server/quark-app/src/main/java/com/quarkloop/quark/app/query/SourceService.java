package com.quarkloop.quark.app.query;

import com.quarkloop.quark.runtime.engine.store.SourceRepository;
import jakarta.enterprise.context.ApplicationScoped;
import jakarta.inject.Inject;

import java.util.Optional;

/**
 * Read the persisted {@code .quark.ts} source for a system.
 *
 * <p>Delegates to {@link SourceRepository} (backed by the Catalog service's
 * {@code systems} table). Source is now read directly from the durable store.
 */
@ApplicationScoped
public class SourceService {

    private final SourceRepository sourceRepository;

    @Inject
    public SourceService(SourceRepository sourceRepository) {
        this.sourceRepository = sourceRepository;
    }

    public Optional<String> getSource(String namespace, String systemName) {
        return sourceRepository.getSource(namespace, systemName);
    }
}
