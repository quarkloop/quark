package com.quarkloop.quark.core.registry.inmemory;

import com.quarkloop.quark.core.domain.category.NodeCategory;
import com.quarkloop.quark.core.domain.config.NodeConfig;
import com.quarkloop.quark.core.domain.identity.NodeUri;
import com.quarkloop.quark.core.registry.NodeDescriptor;
import com.quarkloop.quark.core.registry.NodeImplementationFactory;
import org.junit.jupiter.api.Test;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;

class InMemoryNodeRegistryTest {

    private final InMemoryNodeRegistry registry = new InMemoryNodeRegistry();

    private NodeImplementationFactory<Object> createFactory(String pattern, NodeCategory category, String desc) {
        return new NodeImplementationFactory<>() {
            @Override
            public String uriPattern() { return pattern; }
            @Override
            public Object create(NodeConfig config) { return new Object(); }
            @Override
            public NodeDescriptor descriptor() {
                return new NodeDescriptor(NodeUri.parse(pattern + ":latest"), category, category.isActive(), desc);
            }
            @Override
            public NodeCategory category() { return category; }
        };
    }

    @Test
    void testRegisterAndLookup() {
        var factory = createFactory("source/schedule", NodeCategory.SOURCE, "A simple scheduler");
        registry.register(factory);

        NodeUri searchUri = NodeUri.parse("source/schedule:v1.2.3");
        var desc = registry.lookup(searchUri);

        assertThat(desc).isPresent();
        assertThat(desc.get().category()).isEqualTo(NodeCategory.SOURCE);
        assertThat(desc.get().description()).isEqualTo("A simple scheduler");

        assertThat(registry.lookupFactory(searchUri)).isPresent().contains(factory);
        assertThat(registry.isRegistered(searchUri)).isTrue();
    }

    @Test
    void testDuplicateRegistration() {
        var factory1 = createFactory("store/memory", NodeCategory.STORE, "Mem Store 1");
        var factory2 = createFactory("store/memory", NodeCategory.STORE, "Mem Store 2");

        registry.register(factory1);

        assertThatThrownBy(() -> registry.register(factory2))
                .isInstanceOf(com.quarkloop.quark.core.registry.RegistryException.class)
                .hasMessageContaining("already registered");
    }

    @Test
    void testSearch() {
        registry.register(createFactory("source/timer", NodeCategory.SOURCE, "Emits timer ticks"));
        registry.register(createFactory("function/llm", NodeCategory.FUNCTION, "Calls LLM API"));

        var results1 = registry.search("timer");
        assertThat(results1).hasSize(1);
        assertThat(results1.getFirst().uri().implementation()).isEqualTo("timer");

        var results2 = registry.search("LLM");
        assertThat(results2).hasSize(1);

        var results3 = registry.search("");
        assertThat(results3).hasSize(2);
    }

    @Test
    void testListByCategory() {
        registry.register(createFactory("source/timer", NodeCategory.SOURCE, "Emits timer ticks"));
        registry.register(createFactory("source/http", NodeCategory.SOURCE, "HTTP server"));
        registry.register(createFactory("function/llm", NodeCategory.FUNCTION, "Calls LLM API"));

        var sources = registry.listByCategory(NodeCategory.SOURCE);
        assertThat(sources).hasSize(2);

        var functions = registry.listByCategory(NodeCategory.FUNCTION);
        assertThat(functions).hasSize(1);
    }
}
