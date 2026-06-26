package com.quarkloop.quark.runtime.polyglot;

import com.quarkloop.quark.runtime.domain.spi.QuarkMessage;

import java.time.Instant;
import java.util.Map;

/**
 * Host-access bridge that lets TypeScript nodes read incoming messages.
 *
 * <p>Passed to the TypeScript node's {@code onMessage(message, publisher)} method.
 * TypeScript code accesses:
 * <pre>
 *   const value = message.payload.value;
 *   const subject = message.subject;
 *   const nodeName = message.nodeName;
 * </pre>
 */
public class JsMessage {
    private final QuarkMessage delegate;

    JsMessage(QuarkMessage delegate) {
        this.delegate = delegate;
    }

    public String getSubject() { return delegate.subject(); }
    public Map<String, Object> getPayload() { return delegate.payload(); }
    public Map<String, String> getHeaders() { return delegate.headers(); }
    public String getTimestamp() { return delegate.timestamp().toString(); }
    public String getSystemName() { return delegate.systemName(); }
    public String getNamespace() { return delegate.namespace(); }
    public String getNodeName() { return delegate.nodeName(); }
}
