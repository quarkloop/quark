package com.quarkloop.quark.core.registry.inmemory;

import com.quarkloop.quark.core.domain.category.NodeCategory;
import com.quarkloop.quark.core.domain.identity.NodeUri;
import com.quarkloop.quark.core.registry.NodeDescriptor;
import com.quarkloop.quark.core.registry.NodeImplementationFactory;
import com.quarkloop.quark.core.registry.NodeRegistry;
import jakarta.enterprise.context.ApplicationScoped;

import java.util.List;
import java.util.Optional;
import java.util.concurrent.ConcurrentHashMap;

@ApplicationScoped
public class InMemoryNodeRegistry implements NodeRegistry {

    private record RegistryEntry(NodeImplementationFactory<?> factory, NodeDescriptor descriptor) {}

    private final ConcurrentHashMap<String, RegistryEntry> entries = new ConcurrentHashMap<>();

    private String getPatternKey(NodeUri uri) {
        return uri.category().label() + "/" + uri.implementation();
    }

    @Override
    public Optional<NodeDescriptor> lookup(NodeUri uri) {
        if (uri == null) return Optional.empty();
        RegistryEntry entry = entries.get(getPatternKey(uri));
        return Optional.ofNullable(entry).map(RegistryEntry::descriptor);
    }

    @Override
    public Optional<NodeImplementationFactory<?>> lookupFactory(NodeUri uri) {
        if (uri == null) return Optional.empty();
        RegistryEntry entry = entries.get(getPatternKey(uri));
        return Optional.ofNullable(entry).map(RegistryEntry::factory);
    }

    @Override
    public void register(NodeImplementationFactory<?> factory) {
        if (factory == null) {
            throw new IllegalArgumentException("Factory cannot be null");
        }
        String key = factory.uriPattern();
        if (entries.containsKey(key)) {
            throw new com.quarkloop.quark.core.registry.RegistryException("Factory already registered for pattern: " + key);
        }
        entries.put(key, new RegistryEntry(factory, factory.descriptor()));
    }

    @Override
    public List<NodeDescriptor> search(String keyword) {
        if (keyword == null || keyword.isBlank()) {
            return listAll();
        }
        String lowerKeyword = keyword.toLowerCase();
        return entries.values().stream()
                .map(RegistryEntry::descriptor)
                .filter(desc -> desc.uri().rawUri().toLowerCase().contains(lowerKeyword) ||
                        desc.description().toLowerCase().contains(lowerKeyword))
                .toList();
    }

    @Override
    public List<NodeDescriptor> listAll() {
        return entries.values().stream()
                .map(RegistryEntry::descriptor)
                .toList();
    }

    @Override
    public List<NodeDescriptor> listByCategory(NodeCategory category) {
        if (category == null) return List.of();
        return entries.values().stream()
                .map(RegistryEntry::descriptor)
                .filter(desc -> desc.category() == category)
                .toList();
    }

    @Override
    public boolean isRegistered(NodeUri uri) {
        if (uri == null) return false;
        return entries.containsKey(getPatternKey(uri));
    }
}
