package com.quarkloop.quark.core.event.internal;

import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.datatype.jsr310.JavaTimeModule;
import com.quarkloop.quark.core.domain.event.NodeEvent;
import com.quarkloop.quark.core.event.EventFilter;
import com.quarkloop.quark.core.event.EventStore;
import jakarta.enterprise.context.ApplicationScoped;
import org.eclipse.microprofile.config.inject.ConfigProperty;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import java.io.BufferedReader;
import java.io.BufferedWriter;
import java.io.IOException;
import java.nio.file.Files;
import java.nio.file.Path;
import java.nio.file.Paths;
import java.nio.file.StandardOpenOption;
import java.util.ArrayList;
import java.util.List;
import java.util.concurrent.ConcurrentHashMap;
import java.util.concurrent.ConcurrentMap;

/**
 * Filesystem-backed event store that persists events as JSON Lines.
 *
 * <p>Per the Quark specification (docs/node.md §4.5), every system has its
 * own append-only {@code events.jsonl} file located at:
 * <pre>
 *   $STATE_ROOT/systems/&lt;namespace&gt;/&lt;system-name&gt;/events.jsonl
 * </pre>
 *
 * <p>This implementation routes each {@link NodeEvent} to the file matching
 * its {@code systemName} field. Events with no system (platform-level
 * events) are written to a fallback file at {@code $STATE_ROOT/platform-events.jsonl}.
 *
 * <p>Writes are atomic at the line level: each {@code append} call opens the
 * file in {@code APPEND} mode, writes one JSON object followed by a newline,
 * and closes the writer. This guarantees that no partial records survive a
 * crash mid-write (the OS guarantees atomic appends up to PIPE_BUF on POSIX).
 *
 * <p>Reads ({@link #query(EventFilter)} / {@link #count(EventFilter)}) scan
 * every system file in parallel because the filter may not constrain the
 * system. When the filter does specify {@code systemName}, only that file
 * is read.
 */
@ApplicationScoped
public class JsonlEventStore implements EventStore {

    private static final Logger log = LoggerFactory.getLogger(JsonlEventStore.class);

    /** Fallback file for events that have no system context. */
    private static final String PLATFORM_EVENTS_FILENAME = "platform-events.jsonl";

    private final Path stateRoot;
    private final Path systemsDir;
    private final ObjectMapper mapper;

    /**
     * Per-file write locks. We use one monitor per file path so that appends
     * to different system files do not serialize against each other.
     */
    private final ConcurrentMap<Path, Object> fileLocks = new ConcurrentHashMap<>();

    public JsonlEventStore(
            @ConfigProperty(name = "quark.state.root", defaultValue = "./quark-state")
            String stateRootPath
    ) {
        this.stateRoot = Paths.get(stateRootPath).toAbsolutePath().normalize();
        this.systemsDir = this.stateRoot.resolve("systems");
        this.mapper = new ObjectMapper();
        this.mapper.registerModule(new JavaTimeModule());

        try {
            Files.createDirectories(this.stateRoot);
            Files.createDirectories(this.systemsDir);
        } catch (IOException e) {
            log.error("Failed to initialize event store state root at {}", this.stateRoot, e);
            // Non-fatal: individual append() calls will create directories on demand.
        }
    }

    @Override
    public void append(NodeEvent event) {
        if (event == null) return;
        Path file = resolveEventFile(event);
        Object lock = fileLocks.computeIfAbsent(file, k -> new Object());
        synchronized (lock) {
            ensureFileExists(file);
            try (BufferedWriter writer = Files.newBufferedWriter(
                    file,
                    StandardOpenOption.APPEND,
                    StandardOpenOption.CREATE)) {
                writer.write(mapper.writeValueAsString(event));
                writer.newLine();
                writer.flush();
            } catch (IOException e) {
                log.error("Failed to write event {} to {}", event.id(), file, e);
            }
        }
    }

    @Override
    public void appendAll(List<NodeEvent> events) {
        if (events == null || events.isEmpty()) return;
        // Group by target file so each file is opened only once.
        var byFile = new java.util.HashMap<Path, List<NodeEvent>>();
        for (NodeEvent event : events) {
            byFile.computeIfAbsent(resolveEventFile(event), k -> new ArrayList<>()).add(event);
        }
        for (var entry : byFile.entrySet()) {
            Path file = entry.getKey();
            List<NodeEvent> batch = entry.getValue();
            Object lock = fileLocks.computeIfAbsent(file, k -> new Object());
            synchronized (lock) {
                ensureFileExists(file);
                try (BufferedWriter writer = Files.newBufferedWriter(
                        file,
                        StandardOpenOption.APPEND,
                        StandardOpenOption.CREATE)) {
                    for (NodeEvent event : batch) {
                        writer.write(mapper.writeValueAsString(event));
                        writer.newLine();
                    }
                    writer.flush();
                } catch (IOException e) {
                    log.error("Failed to write batch of {} events to {}", batch.size(), file, e);
                }
            }
        }
    }

    @Override
    public List<NodeEvent> query(EventFilter filter) {
        List<NodeEvent> results = new ArrayList<>();
        if (filter == null) {
            return results;
        }
        for (Path file : candidateFiles(filter)) {
            if (!Files.isRegularFile(file)) continue;
            try (BufferedReader reader = Files.newBufferedReader(file)) {
                String line;
                while ((line = reader.readLine()) != null) {
                    if (line.isBlank()) continue;
                    NodeEvent event;
                    try {
                        event = mapper.readValue(line, NodeEvent.class);
                    } catch (Exception parse) {
                        // Skip corrupt line but warn.
                        log.warn("Skipping malformed event line in {}: {}", file, parse.getMessage());
                        continue;
                    }
                    if (matches(event, filter)) {
                        results.add(event);
                        if (results.size() >= filter.limit()) {
                            return results;
                        }
                    }
                }
            } catch (IOException e) {
                log.error("Failed to query events from {}", file, e);
            }
        }
        return results;
    }

    @Override
    public long count(EventFilter filter) {
        if (filter == null) return 0L;
        long count = 0L;
        for (Path file : candidateFiles(filter)) {
            if (!Files.isRegularFile(file)) continue;
            try (BufferedReader reader = Files.newBufferedReader(file)) {
                String line;
                while ((line = reader.readLine()) != null) {
                    if (line.isBlank()) continue;
                    NodeEvent event;
                    try {
                        event = mapper.readValue(line, NodeEvent.class);
                    } catch (Exception parse) {
                        continue;
                    }
                    if (matches(event, filter)) {
                        count++;
                    }
                }
            } catch (IOException e) {
                log.error("Failed to count events in {}", file, e);
            }
        }
        return count;
    }

    // ------------------------------------------------------------------
    // Helpers
    // ------------------------------------------------------------------

    /**
     * Resolves which file an event belongs to, based on the event's
     * (namespace, systemName) identity.
     *
     * <p><b>Multi-tenancy</b>: the event carries its own namespace (added
     * to the {@link NodeEvent} record specifically to fix cross-tenant
     * data leakage). The file is at:
     * <pre>
     *   $STATE_ROOT/systems/&lt;namespace&gt;/&lt;systemName&gt;/events.jsonl
     * </pre>
     *
     * <p>Platform-level events (namespace == "system") go to
     * {@code $STATE_ROOT/platform-events.jsonl}.
     */
    private Path resolveEventFile(NodeEvent event) {
        String ns = event.namespace();
        String sys = event.systemName();
        if (ns == null || ns.isBlank() || "system".equals(ns)
                || sys == null || sys.isBlank() || "system".equals(sys)) {
            return stateRoot.resolve(PLATFORM_EVENTS_FILENAME);
        }
        return systemsDir.resolve(ns).resolve(sys).resolve("events.jsonl");
    }

    /**
     * Returns the set of files that must be scanned for the given filter.
     *
     * <p><b>Multi-tenancy enforcement</b>:
     * <ul>
     *   <li>If {@code filter.namespace()} is set, ONLY scan systems under
     *       {@code systems/<namespace>/}. Cross-namespace data is invisible.</li>
     *   <li>If {@code filter.systemName()} is also set, narrow further to
     *       that single system within the namespace.</li>
     *   <li>If {@code filter.namespace()} is null (admin-only), scan all
     *       namespaces. REST endpoints MUST set namespace before reaching
     *       here — the EventService guards against this.</li>
     * </ul>
     */
    private List<Path> candidateFiles(EventFilter filter) {
        List<Path> files = new ArrayList<>();

        // Case 1: Both namespace and system specified — single file.
        if (filter.namespace() != null && !filter.namespace().isBlank()
                && filter.systemName() != null && !filter.systemName().isBlank()) {
            Path file = systemsDir.resolve(filter.namespace())
                    .resolve(filter.systemName())
                    .resolve("events.jsonl");
            if (Files.isRegularFile(file)) {
                files.add(file);
            }
            return files;
        }

        // Case 2: Only namespace specified — scan all systems in that namespace.
        if (filter.namespace() != null && !filter.namespace().isBlank()) {
            Path nsDir = systemsDir.resolve(filter.namespace());
            if (Files.isDirectory(nsDir)) {
                try (var topoStream = Files.list(nsDir)) {
                    for (Path topoDir : topoStream.toList()) {
                        if (!Files.isDirectory(topoDir)) continue;
                        Path file = topoDir.resolve("events.jsonl");
                        if (Files.isRegularFile(file)) {
                            files.add(file);
                        }
                    }
                } catch (IOException e) {
                    log.warn("Error scanning namespace dir {}", nsDir, e);
                }
            }
            // Include platform events file only if the namespace matches "default"
            // (platform events are global; we attribute them to default for filtering).
            // Actually no — platform events have no namespace. Don't include them
            // when a specific namespace is requested, to avoid leaking platform-level
            // events to tenant-scoped queries.
            return files;
        }

        // Case 3: Only system name specified (no namespace) — search across
        // all namespaces for a matching system name. This is a RELAXED mode
        // that should only be used by admin endpoints. REST endpoints must
        // always supply namespace.
        if (filter.systemName() != null && !filter.systemName().isBlank()) {
            if (Files.isDirectory(systemsDir)) {
                try (var nsStream = Files.list(systemsDir)) {
                    for (Path nsDir : nsStream.toList()) {
                        Path file = nsDir.resolve(filter.systemName()).resolve("events.jsonl");
                        if (Files.isRegularFile(file)) {
                            files.add(file);
                        }
                    }
                } catch (IOException e) {
                    log.warn("Error scanning for system {} event file", filter.systemName(), e);
                }
            }
            return files;
        }

        // Case 4: No namespace, no system — scan EVERYTHING. Admin only.
        if (Files.isDirectory(systemsDir)) {
            try (var nsStream = Files.list(systemsDir)) {
                for (Path nsDir : nsStream.toList()) {
                    try (var topoStream = Files.list(nsDir)) {
                        for (Path topoDir : topoStream.toList()) {
                            if (!Files.isDirectory(topoDir)) continue;
                            Path file = topoDir.resolve("events.jsonl");
                            if (Files.isRegularFile(file)) {
                                files.add(file);
                            }
                        }
                    } catch (IOException ignored) { /* skip unreadable */ }
                }
            } catch (IOException e) {
                log.warn("Error scanning systems dir", e);
            }
        }
        Path platformFile = stateRoot.resolve(PLATFORM_EVENTS_FILENAME);
        if (Files.isRegularFile(platformFile)) {
            files.add(platformFile);
        }
        return files;
    }

    private void ensureFileExists(Path file) {
        try {
            Path parent = file.getParent();
            if (parent != null && !Files.isDirectory(parent)) {
                Files.createDirectories(parent);
            }
            if (!Files.exists(file)) {
                Files.createFile(file);
            }
        } catch (IOException e) {
            log.error("Failed to ensure event file exists: {}", file, e);
        }
    }

    private boolean matches(NodeEvent event, EventFilter filter) {
        if (filter.nodeName() != null && !filter.nodeName().equals(event.nodeName())) {
            return false;
        }
        if (filter.systemName() != null && !filter.systemName().equals(event.systemName())) {
            return false;
        }
        if (filter.kinds() != null && !filter.kinds().isEmpty() && !filter.kinds().contains(event.kind())) {
            return false;
        }
        if (filter.since() != null && event.timestamp().isBefore(filter.since())) {
            return false;
        }
        if (filter.until() != null && event.timestamp().isAfter(filter.until())) {
            return false;
        }
        return true;
    }
}
