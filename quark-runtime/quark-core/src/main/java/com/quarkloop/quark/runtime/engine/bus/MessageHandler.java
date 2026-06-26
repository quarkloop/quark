package com.quarkloop.quark.runtime.engine.bus;

import com.quarkloop.quark.runtime.domain.spi.QuarkMessage;

@FunctionalInterface
public interface MessageHandler {
    void onMessage(QuarkMessage message) throws Exception;
}
