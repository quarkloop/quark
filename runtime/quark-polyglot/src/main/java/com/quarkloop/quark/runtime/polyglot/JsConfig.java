package com.quarkloop.quark.runtime.polyglot;

import com.quarkloop.quark.core.domain.config.NodeConfig;

/**
 * Host-access bridge that lets TypeScript nodes read configuration.
 *
 * <p>Injected into the GraalJS context as {@code config}. TypeScript code calls:
 * <pre>
 *   const interval = config.getString("interval", "1000");
 *   const maxSize = config.getInt("maxSize", 100);
 * </pre>
 */
public class JsConfig {
    private final NodeConfig delegate;

    JsConfig(NodeConfig delegate) {
        this.delegate = delegate;
    }

    public String getString(String key, String defaultValue) {
        return delegate.getString(key, defaultValue);
    }

    public int getInt(String key, int defaultValue) {
        return delegate.getInt(key, defaultValue);
    }

    public boolean getBoolean(String key, boolean defaultValue) {
        return delegate.getBoolean(key, defaultValue);
    }

    public Object get(String key) {
        return delegate.get(key).orElse(null);
    }
}
