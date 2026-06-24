package com.quarkloop.quark.runtime.polyglot;

import com.quarkloop.quark.core.domain.config.NodeConfig;
import com.quarkloop.quark.core.domain.spi.EndpointProvider;
import com.quarkloop.quark.core.domain.spi.QuarkMessage;
import com.quarkloop.quark.core.domain.spi.QuarkPublisher;
import org.graalvm.polyglot.Context;
import org.graalvm.polyglot.Value;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

/**
 * Wraps a TypeScript Endpoint node (has onStart, onMessage, onStop) in the
 * Java {@link EndpointProvider} interface.
 */
class TypeScriptEndpointProvider implements EndpointProvider {

    private static final Logger log = LoggerFactory.getLogger(TypeScriptEndpointProvider.class);

    private final Value exported;
    private final Context ctx;

    TypeScriptEndpointProvider(Value exported, Context ctx, NodeConfig config) {
        this.exported = exported;
        this.ctx = ctx;
        ctx.getBindings("js").putMember("__config", new JsConfig(config));
    }

    @Override
    public void start(QuarkPublisher publisher, NodeConfig config) {
        try {
            ctx.getBindings("js").putMember("__publisher", new JsPublisher(publisher));
            String methodName = exported.hasMember("onStart") ? "onStart" : "start";
            Value onStart = exported.getMember(methodName);
            if (onStart != null && onStart.canExecute()) {
                onStart.execute(exported,
                        ctx.getBindings("js").getMember("__publisher"),
                        ctx.getBindings("js").getMember("__config"));
            }
        } catch (Exception e) {
            throw new RuntimeException("TypeScript endpoint onStart failed: " + e.getMessage(), e);
        }
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
            log.error("TypeScript endpoint onMessage failed", e);
        }
    }

    @Override
    public void stop() {
        try {
            String methodName = exported.hasMember("onStop") ? "onStop" : "stop";
            Value onStop = exported.getMember(methodName);
            if (onStop != null && onStop.canExecute()) {
                onStop.execute(exported);
            }
        } catch (Exception e) {
            log.warn("TypeScript endpoint onStop failed", e);
        } finally {
            ctx.close();
        }
    }
}
