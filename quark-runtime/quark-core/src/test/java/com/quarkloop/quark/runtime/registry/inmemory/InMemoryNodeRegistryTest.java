package com.quarkloop.quark.runtime.registry.inmemory;

import com.quarkloop.quark.runtime.domain.config.NodeConfig;
import com.quarkloop.quark.runtime.domain.identity.NodeUri;
import com.quarkloop.quark.runtime.domain.spi.NodeProvider;
import com.quarkloop.quark.runtime.registry.NodeDescriptor;
import com.quarkloop.quark.runtime.registry.NodeImplementationFactory;
import org.junit.jupiter.api.Test;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;

class InMemoryNodeRegistryTest {

    private final InMemoryNodeRegistry registry = new InMemoryNodeRegistry();

    private NodeImplementationFactory createFactory(String pattern, String desc) {
        return new NodeImplementationFactory() {
            @Override
            public String uriPattern() { return pattern; }
            @Override
            public NodeProvider create(NodeConfig config) { return new NodeProvider() {}; }
            @Override
            public NodeDescriptor descriptor() {
                return new NodeDescriptor(NodeUri.parse(pattern + ":latest"), desc);
            }
        };
    }

    @Test
    void testRegisterAndLookup() {
        var factory = createFactory("quark/time/schedule/timer", "A simple scheduler");
        registry.register(factory);

        NodeUri searchUri = NodeUri.parse("quark/time/schedule/timer:v1.2.3");
        var desc = registry.lookup(searchUri);

        assertThat(desc).isPresent();
        assertThat(desc.get().description()).isEqualTo("A simple scheduler");

        assertThat(registry.lookupFactory(searchUri)).isPresent().contains(factory);
        assertThat(registry.isRegistered(searchUri)).isTrue();
    }

    @Test
    void testDuplicateRegistration() {
        var factory1 = createFactory("quark/io/file/write", "File Writer 1");
        var factory2 = createFactory("quark/io/file/write", "File Writer 2");

        registry.register(factory1);

        assertThatThrownBy(() -> registry.register(factory2))
                .isInstanceOf(com.quarkloop.quark.runtime.registry.RegistryException.class)
                .hasMessageContaining("already registered");
    }

    @Test
    void testSearch() {
        registry.register(createFactory("quark/time/schedule/timer", "Emits timer ticks"));
        registry.register(createFactory("quark/ai/openai/inference", "Calls LLM API"));

        var results1 = registry.search("timer");
        assertThat(results1).hasSize(1);
        assertThat(results1.getFirst().uri().rawUri()).contains("timer");

        var results2 = registry.search("LLM");
        assertThat(results2).hasSize(1);

        var results3 = registry.search("");
        assertThat(results3).hasSize(2);
    }

    @Test
    void testListAll() {
        registry.register(createFactory("quark/time/schedule/timer", "Emits timer ticks"));
        registry.register(createFactory("quark/io/file/write", "Writes files"));
        registry.register(createFactory("quark/ai/openai/inference", "Calls LLM API"));

        var all = registry.listAll();
        assertThat(all).hasSize(3);
    }
}
