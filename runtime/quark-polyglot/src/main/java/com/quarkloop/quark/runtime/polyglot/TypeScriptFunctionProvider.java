package com.quarkloop.quark.runtime.polyglot;

import com.quarkloop.quark.core.domain.config.NodeConfig;
import com.quarkloop.quark.core.domain.spi.FunctionProvider;
import com.quarkloop.quark.core.domain.spi.QuarkMessage;
import com.quarkloop.quark.core.domain.spi.QuarkPublisher;
import org.graalvm.polyglot.Context;
import org.graalvm.polyglot.Value;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import java.util.HashMap;
import java.util.Map;

/**
 * Wraps a TypeScript Function node (has onMessage, no onStart/onStop) in the
 * Java {@link FunctionProvider} interface.
 */
class TypeScriptFunctionProvider implements FunctionProvider {

    private static final Logger log = LoggerFactory.getLogger(TypeScriptFunctionProvider.class);

    private final Value exported;
    private final Context ctx;

    TypeScriptFunctionProvider(Value exported, Context ctx, NodeConfig config) {
        this.exported = exported;
        this.ctx = ctx;
        // Inject config for potential use in onMessage
        ctx.getBindings("js").putMember("__config", new JsConfig(config));
    }

    @Override
    public void onMessage(QuarkMessage message, QuarkPublisher publisher) {
        try {
            ctx.getBindings("js").putMember("__publisher", new JsPublisher(publisher));
            Value onMessage = exported.getMember("onMessage");
            if (onMessage != null && onMessage.canExecute()) {
                JsMessage jsMsg = new JsMessage(message);
                JsPublisher jsPub = new JsPublisher(publisher);
                onMessage.execute(exported, jsMsg, jsPub);
            }
        } catch (Exception e) {
            log.error("TypeScript function onMessage failed", e);
        }
    }
}
