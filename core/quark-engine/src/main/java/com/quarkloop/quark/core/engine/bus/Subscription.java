package com.quarkloop.quark.core.engine.bus;

public interface Subscription extends AutoCloseable {
    String subject();
    @Override void close();
}
