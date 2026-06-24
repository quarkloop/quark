package com.quarkloop.quark.runtime.polyglot;

import com.fasterxml.jackson.databind.ObjectMapper;
import com.quarkloop.quark.core.domain.config.NodeConfig;
import com.quarkloop.quark.core.domain.spi.QuarkMessage;
import com.quarkloop.quark.core.domain.spi.QuarkPublisher;
import com.quarkloop.quark.core.domain.spi.SourceProvider;
import org.graalvm.polyglot.Context;
import org.graalvm.polyglot.Value;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import java.util.HashMap;
import java.util.Map;

/**
 * Wraps a TypeScript Source node (has onStart/onStop, no onMessage) in the
 * Java {@link SourceProvider} interface.
 *
 * <p>The TypeScript node's {@code onStart(publisher, config)} method is called
 * when the source starts. The publisher and config are injected as host objects
 * accessible from JavaScript.
 */
class TypeScriptSourceProvider implements SourceProvider {

    private static final Logger log = LoggerFactory.getLogger(TypeScriptSourceProvider.class);
    private static final ObjectMapper mapper = new ObjectMapper();

    private final Value exported;
    private final Context ctx;
    private final NodeConfig config;

    TypeScriptSourceProvider(Value exported, Context ctx, NodeConfig config) {
        this.exported = exported;
        this.ctx = ctx;
        this.config = config;
    }

    @Override
    public void start(QuarkPublisher publisher, NodeConfig config) {
        try {
            // Inject publisher and config as host objects
            ctx.getBindings("js").putMember("__publisher", new JsPublisher(publisher));
            ctx.getBindings("js").putMember("__config", new JsConfig(config));

            // Call onStart
            String methodName = exported.hasMember("onStart") ? "onStart" : "start";
            Value onStart = exported.getMember(methodName);
            if (onStart != null && onStart.canExecute()) {
                onStart.execute(exported,
                        ctx.getBindings("js").getMember("__publisher"),
                        ctx.getBindings("js").getMember("__config"));
            }
        } catch (Exception e) {
            throw new RuntimeException("TypeScript source onStart failed: " + e.getMessage(), e);
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
            log.warn("TypeScript source onStop failed", e);
        } finally {
            ctx.close();
        }
    }
}
