package com.quarkloop.quark.runtime.polyglot;

import com.quarkloop.quark.core.domain.spi.QuarkPublisher;

import java.util.HashMap;
import java.util.Map;

/**
 * Host-access bridge that lets TypeScript nodes publish events.
 *
 * <p>Injected into the GraalJS context as {@code publisher}. TypeScript code calls:
 * <pre>
 *   publisher.publish("tick", { count: 1, timestamp: new Date().toISOString() });
 * </pre>
 */
public class JsPublisher {
    private final QuarkPublisher delegate;

    JsPublisher(QuarkPublisher delegate) {
        this.delegate = delegate;
    }

    /** Called from JavaScript: publisher.publish("event", { key: value }) */
    public void publish(String event, Object payload) {
        Map<String, Object> payloadMap;
        if (payload instanceof Map) {
            @SuppressWarnings("unchecked")
            Map<String, Object> m = (Map<String, Object>) payload;
            payloadMap = m;
        } else if (payload instanceof org.graalvm.polyglot.Value v) {
            payloadMap = valueToMap(v);
        } else {
            payloadMap = new HashMap<>();
            if (payload != null) payloadMap.put("data", payload);
        }
        delegate.publish(event, payloadMap);
    }

    @SuppressWarnings("unchecked")
    private static Map<String, Object> valueToMap(org.graalvm.polyglot.Value v) {
        if (v == null || v.isNull()) return Map.of();
        Map<String, Object> map = new HashMap<>();
        if (v.hasMembers()) {
            for (String key : v.getMemberKeys()) {
                map.put(key, valueToObject(v.getMember(key)));
            }
        }
        return map;
    }

    private static Object valueToObject(org.graalvm.polyglot.Value v) {
        if (v == null || v.isNull()) return null;
        if (v.isString()) return v.asString();
        if (v.isNumber()) {
            try { return v.asInt(); }
            catch (Exception e) { return v.asDouble(); }
        }
        if (v.isBoolean()) return v.asBoolean();
        if (v.hasMembers()) return valueToMap(v);
        return v.toString();
    }
}
