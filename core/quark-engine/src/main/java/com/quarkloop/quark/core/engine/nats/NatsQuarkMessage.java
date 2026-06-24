package com.quarkloop.quark.core.engine.nats;

import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.datatype.jsr310.JavaTimeModule;
import com.quarkloop.quark.core.domain.spi.QuarkMessage;
import io.nats.client.Message;

import java.time.Instant;
import java.nio.charset.StandardCharsets;
import java.util.HashMap;
import java.util.Map;

/**
 * Implementation of {@link QuarkMessage} that wraps a NATS {@link Message}.
 *
 * <p>Providers receive this in {@code onMessage()}. They never see the raw
 * NATS API — only this typed interface.
 */
public final class NatsQuarkMessage implements QuarkMessage {

    private static final ObjectMapper mapper = new ObjectMapper();
    static {
        mapper.registerModule(new JavaTimeModule());
    }

    private final String subject;
    private final Map<String, Object> payload;
    private final Map<String, String> headers;
    private final Instant timestamp;
    private final String systemName;
    private final String namespace;
    private final String nodeName;

    /**
     * Create a QuarkMessage from a NATS message.
     *
     * @param natsMsg    the NATS message
     * @param systemName the system name (extracted from subject)
     * @param namespace  the namespace (extracted from subject)
     * @param nodeName   the node name (for whom this message is destined)
     */
    @SuppressWarnings("unchecked")
    public NatsQuarkMessage(Message natsMsg, String systemName, String namespace, String nodeName) {
        this.subject = natsMsg.getSubject();
        this.systemName = systemName;
        this.namespace = namespace;
        this.nodeName = nodeName;
        this.timestamp = Instant.now();

        // Parse payload from JSON
        Map<String, Object> parsedPayload;
        try {
            byte[] data = natsMsg.getData();
            if (data != null && data.length > 0) {
                parsedPayload = mapper.readValue(data, Map.class);
            } else {
                parsedPayload = Map.of();
            }
        } catch (Exception e) {
            parsedPayload = Map.of("__raw__", new String(natsMsg.getData(), StandardCharsets.UTF_8));
        }
        this.payload = parsedPayload;

        // Extract headers from NATS metadata
        this.headers = new HashMap<>();
        if (natsMsg.getHeaders() != null) {
            for (String key : natsMsg.getHeaders().keySet()) {
                this.headers.put(key, natsMsg.getHeaders().getFirst(key));
            }
        }
    }

    @Override
    public String subject() { return subject; }

    @Override
    public Map<String, Object> payload() { return payload; }

    @Override
    public Map<String, String> headers() { return headers; }

    @Override
    public Instant timestamp() { return timestamp; }

    @Override
    public String systemName() { return systemName; }

    @Override
    public String namespace() { return namespace; }

    @Override
    public String nodeName() { return nodeName; }
}
