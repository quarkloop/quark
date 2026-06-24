package com.quarkloop.quark.runtime.polyglot;

import com.quarkloop.quark.core.domain.config.NodeConfig;
import com.quarkloop.quark.core.domain.spi.NodeProvider;
import com.quarkloop.quark.core.domain.spi.QuarkMessage;
import com.quarkloop.quark.core.domain.spi.QuarkPublisher;
import org.graalvm.polyglot.Context;
import org.graalvm.polyglot.Value;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

/**
 * Unified TypeScript node provider — implements {@link NodeProvider}.
 *
 * <p>Wraps a GraalJS {@link Value} (the exported object from the TS module)
 * and delegates to its methods if they exist. The engine calls
 * {@code init}, {@code start}, {@code onMessage}, {@code close} — this class
 * checks whether the JS export has the corresponding method and calls it.
 *
 * <p>Exceptions in {@code onMessage} are rethrown (not swallowed) so the
 * engine's metrics and NATS nak semantics work correctly.
 */
class TypeScriptNodeProvider implements NodeProvider {

    private static final Logger log = LoggerFactory.getLogger(TypeScriptNodeProvider.class);

    private final Value exported;
    private final Context ctx;
    private final boolean hasStart;
    private final boolean hasOnMessage;
    private final boolean hasStop;

    TypeScriptNodeProvider(Value exported, Context ctx, NodeConfig config) {
        this.exported = exported;
        this.ctx = ctx;
        this.hasStart = exported.hasMember("onStart") || exported.hasMember("start");
        this.hasOnMessage = exported.hasMember("onMessage");
        this.hasStop = exported.hasMember("onStop") || exported.hasMember("stop");

        // Inject config into JS context
        ctx.getBindings("js").putMember("config", new JsConfig(config));
    }

    @Override
    public void init(NodeConfig config) {
        // Config was already injected in the constructor.
        // This method exists for the engine lifecycle.
    }

    @Override
    public void start(QuarkPublisher publisher, NodeConfig config) {
        if (!hasStart) return;

        var jsPub = new JsPublisher(publisher);
        ctx.getBindings("js").putMember("publisher", jsPub);

        Value startFn = exported.hasMember("onStart") ? exported.getMember("onStart") : exported.getMember("start");
        try {
            startFn.execute(exported, jsPub, new JsConfig(config));
        } catch (Exception e) {
            throw new RuntimeException("TypeScript onStart failed: " + e.getMessage(), e);
        }
    }

    @Override
    public void onMessage(QuarkMessage message, QuarkPublisher publisher) {
        if (!hasOnMessage) return;

        var jsMsg = new JsMessage(message);
        var jsPub = new JsPublisher(publisher);
        ctx.getBindings("js").putMember("publisher", jsPub);

        try {
            exported.getMember("onMessage").execute(exported, jsMsg, jsPub);
        } catch (Exception e) {
            throw new RuntimeException("TypeScript onMessage failed: " + e.getMessage(), e);
        }
    }

    @Override
    public void close() {
        if (hasStop) {
            try {
                Value stopFn = exported.hasMember("onStop") ? exported.getMember("onStop") : exported.getMember("stop");
                stopFn.execute(exported);
            } catch (Exception e) {
                log.warn("TypeScript onStop failed: {}", e.getMessage());
            }
        }
        try {
            ctx.close();
        } catch (Exception e) {
            log.warn("Failed to close GraalJS context: {}", e.getMessage());
        }
    }
}
