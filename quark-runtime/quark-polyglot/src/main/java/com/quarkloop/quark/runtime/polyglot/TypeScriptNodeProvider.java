package com.quarkloop.quark.runtime.polyglot;

import com.quarkloop.quark.runtime.domain.config.NodeConfig;
import com.quarkloop.quark.runtime.domain.spi.NodeProvider;
import com.quarkloop.quark.runtime.domain.spi.QuarkMessage;
import com.quarkloop.quark.runtime.domain.spi.QuarkPublisher;
import org.graalvm.polyglot.Context;
import org.graalvm.polyglot.Value;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

/**
 * Unified TypeScript node provider — implements {@link NodeProvider}.
 *
 * <p>Wraps a GraalJS {@link Value} (the default-exported object from the
 * TS module) and delegates to its {@code onStart}, {@code onMessage},
 * {@code onStop} methods if present.
 *
 * <h2>Method invocation</h2>
 *
 * <p>GraalJS provides two ways to call a member function:
 * <ul>
 *   <li>{@code exported.getMember("onMessage").execute(args...)} — calls
 *       the function with the given args, but does <strong>not</strong>
 *       bind {@code this} to {@code exported}. In strict mode (which ESM
 *       modules use), {@code this} is {@code undefined}.</li>
 *   <li>{@code exported.invokeMember("onMessage", args...)} — calls the
 *       method <em>on</em> {@code exported}, so {@code this === exported}.
 *       This is what node authors expect when they write
 *       {@code export default { onMessage(msg, pub) { this... } }}.</li>
 * </ul>
 *
 * <p>We use {@link Value#invokeMember} so that node code which references
 * {@code this} works correctly, and so that the function's formal
 * parameters receive the actual arguments in the right order. (The
 * previous implementation called {@code execute(exported, jsMsg, jsPub)}
 * which incorrectly passed {@code exported} as the first argument,
 * shifting every other argument by one position.)
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

        // Inject config into JS context as a global, so module code can
        // reference `config` via the global scope chain.
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

        String methodName = exported.hasMember("onStart") ? "onStart" : "start";
        try {
            exported.invokeMember(methodName, jsPub, new JsConfig(config));
        } catch (Exception e) {
            throw new RuntimeException("TypeScript " + methodName + " failed: " + e.getMessage(), e);
        }
    }

    @Override
    public void onMessage(QuarkMessage message, QuarkPublisher publisher) {
        if (!hasOnMessage) return;

        var jsMsg = new JsMessage(message);
        var jsPub = new JsPublisher(publisher);
        ctx.getBindings("js").putMember("publisher", jsPub);

        try {
            exported.invokeMember("onMessage", jsMsg, jsPub);
        } catch (Exception e) {
            throw new RuntimeException("TypeScript onMessage failed: " + e.getMessage(), e);
        }
    }

    @Override
    public void close() {
        if (hasStop) {
            String methodName = exported.hasMember("onStop") ? "onStop" : "stop";
            try {
                exported.invokeMember(methodName);
            } catch (Exception e) {
                log.warn("TypeScript {} failed: {}", methodName, e.getMessage());
            }
        }
        try {
            ctx.close();
        } catch (Exception e) {
            log.warn("Failed to close GraalJS context: {}", e.getMessage());
        }
    }
}
