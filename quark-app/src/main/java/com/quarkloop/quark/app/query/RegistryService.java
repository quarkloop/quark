package com.quarkloop.quark.app.query;

import com.quarkloop.quark.core.domain.category.NodeCategory;
import com.quarkloop.quark.core.domain.identity.NodeUri;
import com.quarkloop.quark.core.registry.NodeDescriptor;
import com.quarkloop.quark.core.registry.NodeRegistry;
import jakarta.enterprise.context.ApplicationScoped;
import jakarta.inject.Inject;

import java.util.List;
import java.util.Optional;

/**
 * Read-only queries over the {@link NodeRegistry}.
 */
@ApplicationScoped
public class RegistryService {

    private final NodeRegistry registry;

    @Inject
    public RegistryService(NodeRegistry registry) {
        this.registry = registry;
    }

    public List<NodeDescriptor> list(String categoryLabel, String query) {
        if (categoryLabel != null && !categoryLabel.isBlank()) {
            NodeCategory cat = NodeCategory.fromLabel(categoryLabel.toLowerCase());
            return registry.listByCategory(cat);
        }
        if (query != null && !query.isBlank()) {
            return registry.search(query);
        }
        return registry.listAll();
    }

    public Optional<NodeDescriptor> lookup(String uri) {
        return registry.lookup(NodeUri.parse(uri));
    }
}
