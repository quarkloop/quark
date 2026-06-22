package com.quarkloop.quark.adapter.store.duckdb;

import com.fasterxml.jackson.core.JsonProcessingException;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.datatype.jsr310.JavaTimeModule;
import com.quarkloop.quark.core.domain.event.NodeEvent;
import com.quarkloop.quark.core.domain.event.NodeEventKind;
import com.quarkloop.quark.core.engine.store.*;
import com.quarkloop.quark.core.event.EventFilter;
import jakarta.annotation.PostConstruct;
import jakarta.annotation.PreDestroy;
import jakarta.enterprise.context.ApplicationScoped;
import org.eclipse.microprofile.config.inject.ConfigProperty;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import java.nio.file.Files;
import java.nio.file.Path;
import java.nio.file.Paths;
import java.sql.*;
import java.time.Instant;
import java.util.*;

@ApplicationScoped
public class DuckDBStore implements SystemRepository, NodeRepository, EventRepository,
        SourceRepository, RegistryRepository {

    private static final Logger log = LoggerFactory.getLogger(DuckDBStore.class);
    private final ObjectMapper mapper = new ObjectMapper();
    { mapper.registerModule(new JavaTimeModule()); }

    @ConfigProperty(name = "quark.state.root", defaultValue = "./quark-state")
    String stateRootPath;

    private Connection connection;

    @PostConstruct
    void init() {
        try {
            Path dbDir = Paths.get(stateRootPath).toAbsolutePath().normalize();
            Files.createDirectories(dbDir);
            Path dbFile = dbDir.resolve("quark.db");
            log.info("Opening DuckDB database at {}", dbFile);
            connection = DriverManager.getConnection("jdbc:duckdb:" + dbFile);
            createSchema();
            log.info("DuckDB schema initialized");
        } catch (Exception e) {
            throw new IllegalStateException("Failed to initialize DuckDB store", e);
        }
    }

    @PreDestroy
    void close() {
        if (connection != null) {
            try { connection.close(); log.info("DuckDB connection closed"); }
            catch (SQLException e) { log.warn("Failed to close DuckDB connection", e); }
        }
    }

    private void createSchema() throws SQLException {
        try (Statement stmt = connection.createStatement()) {
            stmt.execute("""
                CREATE TABLE IF NOT EXISTS systems (
                    namespace VARCHAR NOT NULL, name VARCHAR NOT NULL, source VARCHAR NOT NULL,
                    state VARCHAR NOT NULL DEFAULT 'ACTIVE', health VARCHAR NOT NULL DEFAULT 'HEALTHY',
                    version BIGINT NOT NULL DEFAULT 1,
                    created_at TIMESTAMP NOT NULL DEFAULT now(), updated_at TIMESTAMP NOT NULL DEFAULT now(),
                    PRIMARY KEY (namespace, name))""");
            stmt.execute("""
                CREATE TABLE IF NOT EXISTS nodes (
                    namespace VARCHAR NOT NULL, system_name VARCHAR NOT NULL, name VARCHAR NOT NULL,
                    uri VARCHAR NOT NULL, category VARCHAR NOT NULL,
                    state VARCHAR NOT NULL DEFAULT 'CREATING', health VARCHAR NOT NULL DEFAULT 'UNKNOWN',
                    version BIGINT NOT NULL DEFAULT 1, error_message VARCHAR,
                    listens VARCHAR, events VARCHAR, config VARCHAR, labels VARCHAR, annotations VARCHAR,
                    on_failure VARCHAR, timeout VARCHAR,
                    created_at TIMESTAMP NOT NULL DEFAULT now(), updated_at TIMESTAMP NOT NULL DEFAULT now(),
                    PRIMARY KEY (namespace, system_name, name))""");
            stmt.execute("""
                CREATE TABLE IF NOT EXISTS events (
                    id UUID NOT NULL, kind VARCHAR NOT NULL, node_name VARCHAR, system_name VARCHAR,
                    namespace VARCHAR, timestamp TIMESTAMP NOT NULL, payload JSON, PRIMARY KEY (id))""");
            stmt.execute("CREATE INDEX IF NOT EXISTS idx_events_ns ON events(namespace)");
            stmt.execute("CREATE INDEX IF NOT EXISTS idx_events_sys ON events(namespace, system_name)");
            stmt.execute("CREATE INDEX IF NOT EXISTS idx_events_node ON events(namespace, system_name, node_name)");
            stmt.execute("CREATE INDEX IF NOT EXISTS idx_events_kind ON events(kind)");
            stmt.execute("CREATE INDEX IF NOT EXISTS idx_events_ts ON events(timestamp)");
            stmt.execute("CREATE INDEX IF NOT EXISTS idx_events_ns_ts ON events(namespace, timestamp DESC)");
            stmt.execute("CREATE INDEX IF NOT EXISTS idx_events_sys_ts ON events(namespace, system_name, timestamp DESC)");
            stmt.execute("""
                CREATE TABLE IF NOT EXISTS registry (
                    uri VARCHAR NOT NULL, pattern VARCHAR NOT NULL, category VARCHAR NOT NULL,
                    active BOOLEAN NOT NULL, description VARCHAR NOT NULL, PRIMARY KEY (uri))""");
            stmt.execute("""
                CREATE TABLE IF NOT EXISTS duckdb_meta (key VARCHAR NOT NULL, value VARCHAR NOT NULL, PRIMARY KEY (key))""");
            stmt.execute("INSERT INTO duckdb_meta VALUES ('schema_version', '1') ON CONFLICT DO NOTHING");
        }
    }

    // --- SystemRepository ---
    @Override public void save(SystemRecord system) {
        try (PreparedStatement ps = connection.prepareStatement(
                "INSERT OR REPLACE INTO systems (namespace, name, source, state, health, version, created_at, updated_at) VALUES (?,?,?,?,?,?,?,?)")) {
            ps.setString(1, system.namespace()); ps.setString(2, system.name()); ps.setString(3, system.source());
            ps.setString(4, system.state()); ps.setString(5, system.health()); ps.setLong(6, system.version());
            ps.setTimestamp(7, Timestamp.from(system.createdAt())); ps.setTimestamp(8, Timestamp.from(system.updatedAt()));
            ps.executeUpdate();
        } catch (SQLException e) { throw new RuntimeException("Failed to save system", e); }
    }

    @Override public Optional<SystemRecord> findByNamespaceAndName(String namespace, String name) {
        try (PreparedStatement ps = connection.prepareStatement("SELECT * FROM systems WHERE namespace=? AND name=?")) {
            ps.setString(1, namespace); ps.setString(2, name);
            try (ResultSet rs = ps.executeQuery()) { if (rs.next()) return Optional.of(mapSystem(rs)); }
        } catch (SQLException e) { throw new RuntimeException("Failed to find system", e); }
        return Optional.empty();
    }

    @Override public List<SystemRecord> findByNamespace(String namespace) {
        List<SystemRecord> out = new ArrayList<>();
        try (PreparedStatement ps = connection.prepareStatement("SELECT * FROM systems WHERE namespace=? ORDER BY name")) {
            ps.setString(1, namespace);
            try (ResultSet rs = ps.executeQuery()) { while (rs.next()) out.add(mapSystem(rs)); }
        } catch (SQLException e) { throw new RuntimeException("Failed to list systems", e); }
        return out;
    }

    @Override public List<SystemRecord> findAllSystems() {
        List<SystemRecord> out = new ArrayList<>();
        try (Statement stmt = connection.createStatement(); ResultSet rs = stmt.executeQuery("SELECT * FROM systems ORDER BY namespace, name")) {
            while (rs.next()) out.add(mapSystem(rs));
        } catch (SQLException e) { throw new RuntimeException("Failed to list all systems", e); }
        return out;
    }

    @Override public void delete(String namespace, String name) {
        try (PreparedStatement ps = connection.prepareStatement("DELETE FROM systems WHERE namespace=? AND name=?")) {
            ps.setString(1, namespace); ps.setString(2, name); ps.executeUpdate();
        } catch (SQLException e) { throw new RuntimeException("Failed to delete system", e); }
    }

    @Override public void updateState(String namespace, String name, String state, String health, long version) {
        try (PreparedStatement ps = connection.prepareStatement("UPDATE systems SET state=?, health=?, version=?, updated_at=now() WHERE namespace=? AND name=?")) {
            ps.setString(1, state); ps.setString(2, health); ps.setLong(3, version); ps.setString(4, namespace); ps.setString(5, name);
            ps.executeUpdate();
        } catch (SQLException e) { throw new RuntimeException("Failed to update system state", e); }
    }

    // --- NodeRepository ---
    @Override public void save(NodeRecord node) {
        try (PreparedStatement ps = connection.prepareStatement(
                "INSERT OR REPLACE INTO nodes (namespace, system_name, name, uri, category, state, health, version, error_message, listens, events, config, labels, annotations, on_failure, timeout, created_at, updated_at) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)")) {
            setNodeParams(ps, node); ps.executeUpdate();
        } catch (SQLException e) { throw new RuntimeException("Failed to save node", e); }
    }

    @Override public void saveAll(List<NodeRecord> nodes) {
        if (nodes.isEmpty()) return;
        try (PreparedStatement ps = connection.prepareStatement(
                "INSERT OR REPLACE INTO nodes (namespace, system_name, name, uri, category, state, health, version, error_message, listens, events, config, labels, annotations, on_failure, timeout, created_at, updated_at) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)")) {
            for (NodeRecord node : nodes) { setNodeParams(ps, node); ps.addBatch(); }
            ps.executeBatch();
        } catch (SQLException e) { throw new RuntimeException("Failed to save nodes batch", e); }
    }

    @Override public Optional<NodeRecord> find(String namespace, String systemName, String nodeName) {
        try (PreparedStatement ps = connection.prepareStatement("SELECT * FROM nodes WHERE namespace=? AND system_name=? AND name=?")) {
            ps.setString(1, namespace); ps.setString(2, systemName); ps.setString(3, nodeName);
            try (ResultSet rs = ps.executeQuery()) { if (rs.next()) return Optional.of(mapNode(rs)); }
        } catch (SQLException e) { throw new RuntimeException("Failed to find node", e); }
        return Optional.empty();
    }

    @Override public List<NodeRecord> findBySystem(String namespace, String systemName) {
        List<NodeRecord> out = new ArrayList<>();
        try (PreparedStatement ps = connection.prepareStatement("SELECT * FROM nodes WHERE namespace=? AND system_name=? ORDER BY name")) {
            ps.setString(1, namespace); ps.setString(2, systemName);
            try (ResultSet rs = ps.executeQuery()) { while (rs.next()) out.add(mapNode(rs)); }
        } catch (SQLException e) { throw new RuntimeException("Failed to list nodes", e); }
        return out;
    }

    @Override public List<NodeRecord> findNodesByNamespace(String namespace) {
        List<NodeRecord> out = new ArrayList<>();
        try (PreparedStatement ps = connection.prepareStatement("SELECT * FROM nodes WHERE namespace=? ORDER BY system_name, name")) {
            ps.setString(1, namespace);
            try (ResultSet rs = ps.executeQuery()) { while (rs.next()) out.add(mapNode(rs)); }
        } catch (SQLException e) { throw new RuntimeException("Failed to list nodes by namespace", e); }
        return out;
    }

    @Override public void delete(String namespace, String systemName, String nodeName) {
        try (PreparedStatement ps = connection.prepareStatement("DELETE FROM nodes WHERE namespace=? AND system_name=? AND name=?")) {
            ps.setString(1, namespace); ps.setString(2, systemName); ps.setString(3, nodeName); ps.executeUpdate();
        } catch (SQLException e) { throw new RuntimeException("Failed to delete node", e); }
    }

    @Override public void deleteBySystem(String namespace, String systemName) {
        try (PreparedStatement ps = connection.prepareStatement("DELETE FROM nodes WHERE namespace=? AND system_name=?")) {
            ps.setString(1, namespace); ps.setString(2, systemName); ps.executeUpdate();
        } catch (SQLException e) { throw new RuntimeException("Failed to delete nodes by system", e); }
    }

    @Override public void updateState(String namespace, String systemName, String nodeName, String state, String health, long version, String errorMessage) {
        try (PreparedStatement ps = connection.prepareStatement("UPDATE nodes SET state=?, health=?, version=?, error_message=?, updated_at=now() WHERE namespace=? AND system_name=? AND name=?")) {
            ps.setString(1, state); ps.setString(2, health); ps.setLong(3, version);
            if (errorMessage != null) ps.setString(4, errorMessage); else ps.setNull(4, Types.VARCHAR);
            ps.setString(5, namespace); ps.setString(6, systemName); ps.setString(7, nodeName);
            ps.executeUpdate();
        } catch (SQLException e) { throw new RuntimeException("Failed to update node state", e); }
    }

    // --- EventRepository (extends EventStore) ---
    @Override public void append(NodeEvent event) {
        try (PreparedStatement ps = connection.prepareStatement("INSERT INTO events (id, kind, node_name, system_name, namespace, timestamp, payload) VALUES (?,?,?,?,?,?,?)")) {
            setEventParams(ps, event); ps.executeUpdate();
        } catch (SQLException e) { log.error("Failed to append event {}", event.id(), e); }
    }

    @Override public void appendAll(List<NodeEvent> events) {
        if (events.isEmpty()) return;
        try (PreparedStatement ps = connection.prepareStatement("INSERT INTO events (id, kind, node_name, system_name, namespace, timestamp, payload) VALUES (?,?,?,?,?,?,?)")) {
            for (NodeEvent event : events) { setEventParams(ps, event); ps.addBatch(); }
            ps.executeBatch();
        } catch (SQLException e) { log.error("Failed to append batch of {} events", events.size(), e); }
    }

    @Override public List<NodeEvent> query(EventFilter filter) {
        StringBuilder sql = new StringBuilder("SELECT * FROM events WHERE 1=1");
        List<Object> params = new ArrayList<>();
        addFilterConditions(sql, params, filter);
        sql.append(" ORDER BY timestamp DESC LIMIT ?");
        params.add(filter.limit());
        List<NodeEvent> out = new ArrayList<>();
        try (PreparedStatement ps = connection.prepareStatement(sql.toString())) {
            for (int i = 0; i < params.size(); i++) ps.setObject(i + 1, params.get(i));
            try (ResultSet rs = ps.executeQuery()) { while (rs.next()) out.add(mapEvent(rs)); }
        } catch (SQLException e) { throw new RuntimeException("Failed to query events", e); }
        return out;
    }

    @Override public long count(EventFilter filter) {
        StringBuilder sql = new StringBuilder("SELECT COUNT(*) FROM events WHERE 1=1");
        List<Object> params = new ArrayList<>();
        addFilterConditions(sql, params, filter);
        try (PreparedStatement ps = connection.prepareStatement(sql.toString())) {
            for (int i = 0; i < params.size(); i++) ps.setObject(i + 1, params.get(i));
            try (ResultSet rs = ps.executeQuery()) { if (rs.next()) return rs.getLong(1); }
        } catch (SQLException e) { throw new RuntimeException("Failed to count events", e); }
        return 0;
    }

    @Override public int deleteOlderThan(Instant cutoff, int limit) {
        try (PreparedStatement ps = connection.prepareStatement("DELETE FROM events WHERE id IN (SELECT id FROM events WHERE timestamp < ? LIMIT ?)")) {
            ps.setTimestamp(1, Timestamp.from(cutoff)); ps.setInt(2, limit);
            return ps.executeUpdate();
        } catch (SQLException e) { throw new RuntimeException("Failed to delete old events", e); }
    }

    private void addFilterConditions(StringBuilder sql, List<Object> params, EventFilter filter) {
        if (filter.namespace() != null && !filter.namespace().isBlank()) { sql.append(" AND namespace=?"); params.add(filter.namespace()); }
        if (filter.systemName() != null && !filter.systemName().isBlank()) { sql.append(" AND system_name=?"); params.add(filter.systemName()); }
        if (filter.nodeName() != null && !filter.nodeName().isBlank()) { sql.append(" AND node_name=?"); params.add(filter.nodeName()); }
        if (filter.kinds() != null && !filter.kinds().isEmpty()) {
            sql.append(" AND kind IN (").append(String.join(",", filter.kinds().stream().map(k -> "?").toList())).append(")");
            for (NodeEventKind k : filter.kinds()) params.add(k.name());
        }
        if (filter.since() != null) { sql.append(" AND timestamp >= ?"); params.add(Timestamp.from(filter.since())); }
        if (filter.until() != null) { sql.append(" AND timestamp <= ?"); params.add(Timestamp.from(filter.until())); }
    }

    // --- SourceRepository ---
    @Override public void saveSource(String namespace, String name, String source) { /* stored via systems table */ }
    @Override public Optional<String> getSource(String namespace, String name) {
        return findByNamespaceAndName(namespace, name).map(SystemRecord::source);
    }
    @Override public List<SourceEntry> listSources() {
        List<SourceEntry> out = new ArrayList<>();
        try (Statement stmt = connection.createStatement(); ResultSet rs = stmt.executeQuery("SELECT namespace, name FROM systems ORDER BY namespace, name")) {
            while (rs.next()) out.add(new SourceEntry(rs.getString(1), rs.getString(2)));
        } catch (SQLException e) { throw new RuntimeException("Failed to list sources", e); }
        return out;
    }

    // --- RegistryRepository ---
    @Override public void save(RegistryRecord record) {
        try (PreparedStatement ps = connection.prepareStatement("INSERT OR REPLACE INTO registry (uri, pattern, category, active, description) VALUES (?,?,?,?,?)")) {
            ps.setString(1, record.uri()); ps.setString(2, record.pattern()); ps.setString(3, record.category());
            ps.setBoolean(4, record.active()); ps.setString(5, record.description()); ps.executeUpdate();
        } catch (SQLException e) { throw new RuntimeException("Failed to save registry record", e); }
    }
    @Override public Optional<RegistryRecord> findByUri(String uri) {
        try (PreparedStatement ps = connection.prepareStatement("SELECT * FROM registry WHERE uri=?")) {
            ps.setString(1, uri);
            try (ResultSet rs = ps.executeQuery()) { if (rs.next()) return Optional.of(mapRegistry(rs)); }
        } catch (SQLException e) { throw new RuntimeException("Failed to find registry record", e); }
        return Optional.empty();
    }
    @Override public List<RegistryRecord> findAllRegistry() {
        List<RegistryRecord> out = new ArrayList<>();
        try (Statement stmt = connection.createStatement(); ResultSet rs = stmt.executeQuery("SELECT * FROM registry ORDER BY uri")) {
            while (rs.next()) out.add(mapRegistry(rs));
        } catch (SQLException e) { throw new RuntimeException("Failed to list registry", e); }
        return out;
    }
    @Override public List<RegistryRecord> findByCategory(String category) {
        List<RegistryRecord> out = new ArrayList<>();
        try (PreparedStatement ps = connection.prepareStatement("SELECT * FROM registry WHERE category=? ORDER BY uri")) {
            ps.setString(1, category);
            try (ResultSet rs = ps.executeQuery()) { while (rs.next()) out.add(mapRegistry(rs)); }
        } catch (SQLException e) { throw new RuntimeException("Failed to list registry by category", e); }
        return out;
    }
    @Override public List<RegistryRecord> search(String keyword) {
        List<RegistryRecord> out = new ArrayList<>();
        try (PreparedStatement ps = connection.prepareStatement("SELECT * FROM registry WHERE uri LIKE ? OR description LIKE ? ORDER BY uri")) {
            String pattern = "%" + keyword + "%"; ps.setString(1, pattern); ps.setString(2, pattern);
            try (ResultSet rs = ps.executeQuery()) { while (rs.next()) out.add(mapRegistry(rs)); }
        } catch (SQLException e) { throw new RuntimeException("Failed to search registry", e); }
        return out;
    }
    @Override public boolean existsByUri(String uri) {
        try (PreparedStatement ps = connection.prepareStatement("SELECT 1 FROM registry WHERE uri=?")) {
            ps.setString(1, uri);
            try (ResultSet rs = ps.executeQuery()) { return rs.next(); }
        } catch (SQLException e) { throw new RuntimeException("Failed to check registry existence", e); }
    }

    // --- Mappers ---
    private SystemRecord mapSystem(ResultSet rs) throws SQLException {
        return new SystemRecord(rs.getString("namespace"), rs.getString("name"), rs.getString("source"),
                rs.getString("state"), rs.getString("health"), rs.getLong("version"),
                rs.getTimestamp("created_at").toInstant(), rs.getTimestamp("updated_at").toInstant());
    }

    @SuppressWarnings("unchecked")
    private NodeRecord mapNode(ResultSet rs) throws SQLException {
        String onFailureJson = rs.getString("on_failure");
        String onFailureRetry = null, onFailureRouteTo = null;
        if (onFailureJson != null && !onFailureJson.isBlank()) {
            try { Map<String, Object> onFailure = mapper.readValue(onFailureJson, Map.class);
                onFailureRetry = String.valueOf(onFailure.get("retry")); onFailureRouteTo = (String) onFailure.get("routeTo");
            } catch (JsonProcessingException ignored) {}
        }
        return new NodeRecord(rs.getString("namespace"), rs.getString("system_name"), rs.getString("name"),
                rs.getString("uri"), rs.getString("category"), rs.getString("state"), rs.getString("health"),
                rs.getLong("version"), rs.getString("error_message"),
                parseStringList(rs.getString("listens")), parseStringList(rs.getString("events")),
                parseMap(rs.getString("config")), parseStringMap(rs.getString("labels")), parseStringMap(rs.getString("annotations")),
                onFailureRetry, onFailureRouteTo, rs.getString("timeout"),
                rs.getTimestamp("created_at").toInstant(), rs.getTimestamp("updated_at").toInstant());
    }

    @SuppressWarnings("unchecked")
    private NodeEvent mapEvent(ResultSet rs) throws SQLException {
        String payloadJson = rs.getString("payload");
        Map<String, Object> payload = Map.of();
        if (payloadJson != null && !payloadJson.isBlank()) {
            try { payload = mapper.readValue(payloadJson, Map.class); } catch (JsonProcessingException ignored) {}
        }
        return new NodeEvent(UUID.fromString(rs.getString("id")), NodeEventKind.valueOf(rs.getString("kind")),
                rs.getString("node_name"), rs.getString("system_name"), rs.getString("namespace"),
                rs.getTimestamp("timestamp").toInstant(), payload);
    }

    private RegistryRecord mapRegistry(ResultSet rs) throws SQLException {
        return new RegistryRecord(rs.getString("uri"), rs.getString("pattern"), rs.getString("category"),
                rs.getBoolean("active"), rs.getString("description"));
    }

    // --- Param setters ---
    private void setNodeParams(PreparedStatement ps, NodeRecord node) throws SQLException {
        ps.setString(1, node.namespace()); ps.setString(2, node.systemName()); ps.setString(3, node.name());
        ps.setString(4, node.uri()); ps.setString(5, node.category()); ps.setString(6, node.state()); ps.setString(7, node.health());
        ps.setLong(8, node.version());
        if (node.errorMessage() != null) ps.setString(9, node.errorMessage()); else ps.setNull(9, Types.VARCHAR);
        ps.setString(10, toJson(node.listens())); ps.setString(11, toJson(node.events()));
        ps.setString(12, toJson(node.config())); ps.setString(13, toJson(node.labels())); ps.setString(14, toJson(node.annotations()));
        if (node.onFailureRetry() != null || node.onFailureRouteTo() != null) {
            Map<String, Object> onFailure = new HashMap<>();
            if (node.onFailureRetry() != null) onFailure.put("retry", node.onFailureRetry());
            if (node.onFailureRouteTo() != null) onFailure.put("routeTo", node.onFailureRouteTo());
            ps.setString(15, toJson(onFailure));
        } else ps.setNull(15, Types.VARCHAR);
        if (node.timeout() != null) ps.setString(16, node.timeout()); else ps.setNull(16, Types.VARCHAR);
        ps.setTimestamp(17, Timestamp.from(node.createdAt())); ps.setTimestamp(18, Timestamp.from(node.updatedAt()));
    }

    private void setEventParams(PreparedStatement ps, NodeEvent event) throws SQLException {
        ps.setObject(1, event.id()); ps.setString(2, event.kind().name());
        if (event.nodeName() != null) ps.setString(3, event.nodeName()); else ps.setNull(3, Types.VARCHAR);
        if (event.systemName() != null) ps.setString(4, event.systemName()); else ps.setNull(4, Types.VARCHAR);
        if (event.namespace() != null) ps.setString(5, event.namespace()); else ps.setNull(5, Types.VARCHAR);
        ps.setTimestamp(6, Timestamp.from(event.timestamp()));
        if (event.payload() != null && !event.payload().isEmpty()) ps.setString(7, toJson(event.payload()));
        else ps.setNull(7, Types.OTHER);
    }

    // --- JSON helpers ---
    private String toJson(Object obj) {
        if (obj == null) return null;
        try { return mapper.writeValueAsString(obj); } catch (JsonProcessingException e) { return null; }
    }
    @SuppressWarnings("unchecked")
    private List<String> parseStringList(String json) {
        if (json == null || json.isBlank()) return List.of();
        try { return mapper.readValue(json, List.class); } catch (JsonProcessingException e) { return List.of(); }
    }
    @SuppressWarnings("unchecked")
    private Map<String, Object> parseMap(String json) {
        if (json == null || json.isBlank()) return Map.of();
        try { return mapper.readValue(json, Map.class); } catch (JsonProcessingException e) { return Map.of(); }
    }
    @SuppressWarnings("unchecked")
    private Map<String, String> parseStringMap(String json) {
        if (json == null || json.isBlank()) return Map.of();
        try { Map<String, Object> raw = mapper.readValue(json, Map.class); Map<String, String> out = new HashMap<>();
            for (var entry : raw.entrySet()) out.put(entry.getKey(), String.valueOf(entry.getValue()));
            return out;
        } catch (JsonProcessingException e) { return Map.of(); }
    }
}
