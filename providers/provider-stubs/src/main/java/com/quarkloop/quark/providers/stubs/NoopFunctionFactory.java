package com.quarkloop.quark.providers.stubs;

import com.quarkloop.quark.core.domain.category.NodeCategory;
import com.quarkloop.quark.core.domain.config.NodeConfig;
import com.quarkloop.quark.core.domain.identity.NodeUri;
import com.quarkloop.quark.core.domain.spi.FunctionProvider;
import com.quarkloop.quark.core.domain.spi.QuarkMessage;
import com.quarkloop.quark.core.domain.spi.QuarkPublisher;
import com.quarkloop.quark.core.registry.NodeDescriptor;
import com.quarkloop.quark.core.registry.NodeImplementationFactory;
import jakarta.enterprise.context.ApplicationScoped;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

/**
 * No-op function stub. Drops every incoming message. Useful for wiring tests.
 *
 * <p>URI: {@code function/noop:v1}.
 */
@ApplicationScoped
public class NoopFunctionFactory implements NodeImplementationFactory<FunctionProvider> {

    private static final Logger log = LoggerFactory.getLogger(NoopFunctionFactory.class);

    @Override
    public String uriPattern() {
        return "function/noop";
    }

    @Override
    public FunctionProvider create(NodeConfig config) {
        return new NoopFunction();
    }

    @Override
    public NodeDescriptor descriptor() {
        return new NodeDescriptor(
                NodeUri.parse("function/noop:v1"),
                NodeCategory.FUNCTION,
                true,
                "No-op function — drops every message."
        );
    }

    @Override
    public NodeCategory category() {
        return NodeCategory.FUNCTION;
    }

    static final class NoopFunction implements FunctionProvider {
        @Override
        public void onMessage(QuarkMessage message, QuarkPublisher publisher) {
            log.debug("noop dropping message on {}", message.subject());
        }
    }
}
