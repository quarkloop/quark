package com.quarkloop.quark.providers.jsonwriter;

import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.datatype.jsr310.JavaTimeModule;
import com.quarkloop.quark.core.domain.config.NodeConfig;
import com.quarkloop.quark.core.domain.identity.NodeUri;
import com.quarkloop.quark.core.domain.spi.NodeProvider;
import com.quarkloop.quark.core.domain.spi.QuarkMessage;
import com.quarkloop.quark.core.domain.spi.QuarkPublisher;
import com.quarkloop.quark.core.registry.NodeDescriptor;
import com.quarkloop.quark.core.registry.NodeImplementationFactory;
import jakarta.enterprise.context.ApplicationScoped;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import java.io.IOException;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.nio.file.Paths;
import java.nio.file.StandardOpenOption;
import java.time.Instant;
import java.util.HashMap;
import java.util.Map;
import java.util.concurrent.locks.ReentrantLock;

/**
 * File writer node — appends incoming messages as JSON Lines to a file.
 *
 * <p>URI: {@code quark/io/file/write:v1}
 * Config: {@code path} (string, required), {@code mode} (default "append")
 */
@ApplicationScoped
public class JsonWriterFactory implements NodeImplementationFactory {

    private static final Logger log = LoggerFactory.getLogger(JsonWriterFactory.class);

    @Override
    public String uriPattern() {
        return "quark/io/file/write";
    }

    @Override
    public NodeProvider create(NodeConfig config) {
        return new FileWriteNode();
    }

    @Override
    public NodeDescriptor descriptor() {
        return new NodeDescriptor(
                NodeUri.parse("quark/io/file/write:v1"),
                "Appends incoming messages as JSON Lines to a file on disk."
        );
    }

    static final class FileWriteNode implements NodeProvider {

        private static final ObjectMapper MAPPER = new ObjectMapper();
        static { MAPPER.registerModule(new JavaTimeModule()); }

        private Path path;
        private String mode;
        private final ReentrantLock lock = new ReentrantLock();
        private volatile boolean initialized = false;

        @Override
        public void init(NodeConfig config) {
            String p = config.getString("path", "./quark-output.jsonl");
            this.path = Paths.get(p).toAbsolutePath().normalize();
            this.mode = config.getString("mode", "append");
        }

        private void ensureOpen() {
            if (initialized) return;
            lock.lock();
            try {
                if (initialized) return;
                try {
                    Path parent = path.getParent();
                    if (parent != null && !Files.isDirectory(parent)) {
                        Files.createDirectories(parent);
                    }
                    if (!Files.exists(path) && "append".equalsIgnoreCase(mode)) {
                        Files.createFile(path);
                    }
                } catch (IOException e) {
                    log.error("Failed to open file: {}", path, e);
                }
                initialized = true;
                log.info("File writer initialized at {} (mode={})", path, mode);
            } finally {
                lock.unlock();
            }
        }

        @Override
        public void onMessage(QuarkMessage message, QuarkPublisher publisher) {
            ensureOpen();
            Map<String, Object> record = new HashMap<>();
            record.put("subject", message.subject());
            record.put("systemName", message.systemName());
            record.put("namespace", message.namespace());
            record.put("nodeName", message.nodeName());
            record.put("timestamp", Instant.now().toString());
            record.put("payload", message.payload());

            lock.lock();
            try {
                byte[] line = (MAPPER.writeValueAsString(record) + "\n").getBytes(StandardCharsets.UTF_8);
                Files.write(path, line,
                        StandardOpenOption.CREATE, StandardOpenOption.APPEND, StandardOpenOption.SYNC);
                log.debug("Appended 1 record to {}", path);
            } catch (IOException e) {
                log.error("Failed to append to {}", path, e);
            } finally {
                lock.unlock();
            }
        }
    }
}
