package com.quarkloop.quark.runtime.engine.bus;

public interface Subscription extends AutoCloseable {
    String subject();
    @Override void close();
}
