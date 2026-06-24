package com.quarkloop.quark.runtime.polyglot;

import com.quarkloop.quark.core.domain.config.NodeConfig;
import com.quarkloop.quark.core.domain.spi.QuarkMessage;
import com.quarkloop.quark.core.domain.spi.QuarkPublisher;
import com.quarkloop.quark.core.domain.spi.StoreProvider;
import org.graalvm.polyglot.Context;
import org.graalvm.polyglot.Value;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

/**
 * Wraps a TypeScript Store node (same interface as Function, but category=STORE).
 */
class TypeScriptStoreProvider implements StoreProvider {

    private static final Logger log = LoggerFactory.getLogger(TypeScriptStoreProvider.class);

    private final Value exported;
    private final Context ctx;

    TypeScriptStoreProvider(Value exported, Context ctx, NodeConfig config) {
        this.exported = exported;
        this.ctx = ctx;
        ctx.getBindings("js").putMember("__config", new JsConfig(config));
    }

    @Override
    public void onMessage(QuarkMessage message, QuarkPublisher publisher) {
        try {
            ctx.getBindings("js").putMember("__publisher", new JsPublisher(publisher));
            Value onMessage = exported.getMember("onMessage");
            if (onMessage != null && onMessage.canExecute()) {
                onMessage.execute(exported, new JsMessage(message), new JsPublisher(publisher));
            }
        } catch (Exception e) {
            log.error("TypeScript store onMessage failed", e);
        }
    }
}
