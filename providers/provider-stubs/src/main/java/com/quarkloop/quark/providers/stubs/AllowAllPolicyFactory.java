package com.quarkloop.quark.providers.stubs;

import com.quarkloop.quark.core.domain.category.NodeCategory;
import com.quarkloop.quark.core.domain.config.NodeConfig;
import com.quarkloop.quark.core.domain.identity.NodeUri;
import com.quarkloop.quark.core.domain.spi.PolicyProvider;
import com.quarkloop.quark.core.domain.spi.QuarkMessage;
import com.quarkloop.quark.core.domain.spi.QuarkPublisher;
import com.quarkloop.quark.core.registry.NodeDescriptor;
import com.quarkloop.quark.core.registry.NodeImplementationFactory;
import jakarta.enterprise.context.ApplicationScoped;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

/**
 * Policy stub that allows every message. The message is silently forwarded
 * (no-op since the engine has already delivered it). Useful for wiring tests.
 *
 * <p>URI: {@code policy/allow-all:v1}.
 */
@ApplicationScoped
public class AllowAllPolicyFactory implements NodeImplementationFactory<PolicyProvider> {

    private static final Logger log = LoggerFactory.getLogger(AllowAllPolicyFactory.class);

    @Override
    public String uriPattern() {
        return "policy/allow-all";
    }

    @Override
    public PolicyProvider create(NodeConfig config) {
        return new AllowAllPolicy();
    }

    @Override
    public NodeDescriptor descriptor() {
        return new NodeDescriptor(
                NodeUri.parse("policy/allow-all:v1"),
                NodeCategory.POLICY,
                false,
                "Allow-all policy — permits every message."
        );
    }

    @Override
    public NodeCategory category() {
        return NodeCategory.POLICY;
    }

    static final class AllowAllPolicy implements PolicyProvider {
        @Override
        public void onMessage(QuarkMessage message, QuarkPublisher publisher) {
            log.debug("allow-all permitted message on {}", message.subject());
            // No forwarding — the engine already routed the message to this
            // node's subscriptions. Policy nodes typically re-publish to a
            // downstream subject; the stub just logs.
        }
    }
}
