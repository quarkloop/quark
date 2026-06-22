package com.quarkloop.quark.core.engine.bus;

import com.quarkloop.quark.core.domain.spi.QuarkMessage;

@FunctionalInterface
public interface MessageHandler {
    void onMessage(QuarkMessage message) throws Exception;
}
