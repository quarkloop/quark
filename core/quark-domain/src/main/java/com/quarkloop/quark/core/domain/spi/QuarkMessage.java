package com.quarkloop.quark.core.domain.spi;

import java.time.Instant;
import java.util.Map;

/**
 * A message received from NATS JetStream.
 *
 * <p>Providers receive this in {@code onMessage()}. It wraps the NATS message
 * with typed accessors. Providers never see the raw NATS API.
 */
public interface QuarkMessage {

    /** Full NATS subject (e.g., "monitor.alice.timer.tick") */
    String subject();

    /** Message payload as a map */
    Map<String, Object> payload();

    /** Message headers/metadata */
    Map<String, String> headers();

    /** When NATS received the message */
    Instant timestamp();

    /** System name (e.g., "monitor") */
    String systemName();

    /** Namespace (e.g., "alice") */
    String namespace();

    /** Node name (e.g., "cpu") */
    String nodeName();
}
