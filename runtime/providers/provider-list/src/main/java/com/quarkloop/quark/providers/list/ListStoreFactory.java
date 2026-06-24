package com.quarkloop.quark.providers.list;

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

import java.util.HashMap;
import java.util.LinkedList;
import java.util.List;
import java.util.Map;
import java.util.concurrent.locks.ReentrantLock;

/**
 * Store node that accumulates incoming payloads in an in-memory bounded list
 * and publishes an {@code updated} event after each append.
 *
 * <p>URI: {@code store/list:v1}. Config:
 * <ul>
 *   <li>{@code maxSize} (int, default 1000) — max entries before oldest is evicted</li>
 * </ul>
 *
 * <p>The {@code updated} payload shape:
 * <pre>
 *   { "size": 42, "maxSize": 100, "droppedCount": 0, "latest": { ... } }
 * </pre>
 */
@ApplicationScoped
public class ListStoreFactory implements NodeImplementationFactory<StoreProvider> {

    private static final Logger log = LoggerFactory.getLogger(ListStoreFactory.class);

    @Override
    public String uriPattern() {
        return "store/list";
    }

    @Override
    public StoreProvider create(NodeConfig config) {
        return new ListStore(config);
    }

    @Override
    public NodeDescriptor descriptor() {
        return new NodeDescriptor(
                NodeUri.parse("store/list:v1"),
                NodeCategory.STORE,
                false,
                "In-memory bounded list store; publishes 'updated' on each append."
        );
    }

    @Override
    public NodeCategory category() {
        return NodeCategory.STORE;
    }

    static final class ListStore implements StoreProvider {

        private final int maxSize;
        private final LinkedList<Map<String, Object>> items = new LinkedList<>();
        private final ReentrantLock lock = new ReentrantLock();
        private long droppedCount = 0;

        ListStore(NodeConfig config) {
            this.maxSize = config.getInt("maxSize", 1000);
        }

        @Override
        public void onMessage(QuarkMessage message, QuarkPublisher publisher) {
            Map<String, Object> latest;
            int size;
            lock.lock();
            try {
                items.addLast(message.payload());
                while (items.size() > maxSize) {
                    items.removeFirst();
                    droppedCount++;
                }
                latest = items.getLast();
                size = items.size();
            } finally {
                lock.unlock();
            }

            Map<String, Object> payload = new HashMap<>();
            payload.put("size", size);
            payload.put("maxSize", maxSize);
            payload.put("droppedCount", droppedCount);
            payload.put("latest", latest);

            log.debug("List updated: size={}, dropped={}", size, droppedCount);
            publisher.publish("updated", payload);
        }
    }
}
