package com.quarkloop.quark.providers.stubs;

import com.quarkloop.quark.core.domain.category.NodeCategory;
import com.quarkloop.quark.core.domain.config.NodeConfig;
import com.quarkloop.quark.core.domain.identity.NodeUri;
import com.quarkloop.quark.core.domain.spi.QuarkMessage;
import com.quarkloop.quark.core.domain.spi.QuarkPublisher;
import com.quarkloop.quark.core.domain.spi.StoreProvider;
import com.quarkloop.quark.core.registry.NodeDescriptor;
import com.quarkloop.quark.core.registry.NodeImplementationFactory;
import jakarta.enterprise.context.ApplicationScoped;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import java.util.LinkedList;
import java.util.List;
import java.util.concurrent.locks.ReentrantLock;

/**
 * In-memory store stub. Keeps the last N messages (default 100) in memory.
 * Publishes no events.
 *
 * <p>URI: {@code store/memory:v1}. Config:
 * <ul>
 *   <li>{@code maxSize} (int, default 100)</li>
 * </ul>
 */
@ApplicationScoped
public class MemoryStoreFactory implements NodeImplementationFactory<StoreProvider> {

    private static final Logger log = LoggerFactory.getLogger(MemoryStoreFactory.class);

    @Override
    public String uriPattern() {
        return "store/memory";
    }

    @Override
    public StoreProvider create(NodeConfig config) {
        return new MemoryStore(config);
    }

    @Override
    public NodeDescriptor descriptor() {
        return new NodeDescriptor(
                NodeUri.parse("store/memory:v1"),
                NodeCategory.STORE,
                false,
                "In-memory bounded store (publishes no events)."
        );
    }

    @Override
    public NodeCategory category() {
        return NodeCategory.STORE;
    }

    static final class MemoryStore implements StoreProvider {
        private final int maxSize;
        private final List<QuarkMessage> items = new LinkedList<>();
        private final ReentrantLock lock = new ReentrantLock();

        MemoryStore(NodeConfig config) {
            this.maxSize = config.getInt("maxSize", 100);
        }

        @Override
        public void onMessage(QuarkMessage message, QuarkPublisher publisher) {
            lock.lock();
            try {
                items.addLast(message);
                while (items.size() > maxSize) {
                    items.removeFirst();
                }
            } finally {
                lock.unlock();
            }
            log.debug("memory-store captured 1 (size<= {})", maxSize);
        }
    }
}
