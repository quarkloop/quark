package com.quarkloop.quark.runtime.engine.bus;

import com.quarkloop.quark.runtime.domain.spi.QuarkPublisher;
import java.util.Set;

public interface MessageBus extends AutoCloseable {
    void publish(String subject, byte[] payload);
    Subscription subscribe(String subject, MessageHandler handler);
    QuarkPublisher createPublisher(String systemName, String namespace, String nodeName, Set<String> allowedEvents);
    boolean isConnected();
    void connect() throws Exception;
    @Override void close();
}
