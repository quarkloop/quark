package com.quarkloop.quark.adapter.catalog;

import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.datatype.jsr310.JavaTimeModule;
import com.quarkloop.quark.core.domain.event.NodeEvent;
import com.quarkloop.quark.core.domain.event.NodeEventKind;
import com.quarkloop.quark.core.engine.store.*;
import com.quarkloop.quark.core.event.EventFilter;
import com.quarkloop.quark.core.engine.nats.NatsConnectionManager;
import io.nats.client.Connection;
import io.nats.client.Message;
import jakarta.annotation.PostConstruct;
import jakarta.enterprise.context.ApplicationScoped;
import jakarta.inject.Inject;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import java.time.Duration;
import java.time.Instant;
import java.util.*;

/**
 * NATS-based implementation of all repository interfaces.
 *
 * <p>Replaces {@code DuckDBStore} — instead of direct JDBC calls, this client
 * sends JSON requests over NATS to the standalone Quark Catalog service
 * (a Go + SQLite process). The catalog service handles all persistence.
 *
 * <p>This eliminates DuckDB (and its JNI incompatibility with native image)
 * from the Java codebase entirely. The catalog service runs as a separate
 * process, communicating via NATS request-reply.
 *
 * <p>Implements: SystemRepository, NodeRepository, EventRepository/EventStore,
 * SourceRepository, RegistryRepository.
 */
@ApplicationScoped
public class NatsCatalogClient implements SystemRepository, NodeRepository,
        EventRepository, SourceRepository, RegistryRepository {

    private static final Logger log = LoggerFactory.getLogger(NatsCatalogClient.class);
    private static final Duration REQUEST_TIMEOUT = Duration.ofSeconds(5);

    private final ObjectMapper mapper = new ObjectMapper();
    { mapper.registerModule(new JavaTimeModule()); }

    private final NatsConnectionManager natsConnectionManager;

    @Inject
    public NatsCatalogClient(NatsConnectionManager natsConnectionManager) {
        this.natsConnectionManager = natsConnectionManager;
    }

    @PostConstruct
    void init() {
        log.info("NatsCatalogClient initialized — using Catalog service via NATS");
    }

    private byte[] natsRequest(String subject, Object payload) {
        try {
            Connection conn = natsConnectionManager.getConnection();
            byte[] data = mapper.writeValueAsBytes(payload);
            Message reply = conn.request(subject, data, REQUEST_TIMEOUT);
            if (reply == null) {
                throw new RuntimeException("Catalog did not respond to " + subject + " within " + REQUEST_TIMEOUT);
            }
            return reply.getData();
        } catch (Exception e) {
            throw new RuntimeException("Catalog request failed: " + subject + " — " + e.getMessage(), e);
        }
    }

    private <T> T natsRequestAndParse(String subject, Object payload, Class<T> type) {
        try {
            return mapper.readValue(natsRequest(subject, payload), type);
        } catch (Exception e) {
            throw new RuntimeException("Failed to parse catalog response for " + subject, e);
        }
    }

    // --- SystemRepository ---
    @Override public void save(SystemRecord system) {
        Map<String,Object> r = new LinkedHashMap<>();
        r.put("namespace", system.namespace()); r.put("name", system.name());
        r.put("source", system.source()); r.put("state", system.state());
        r.put("health", system.health()); r.put("version", system.version());
        natsRequest("catalog.system.save", r);
    }
    @Override public Optional<SystemRecord> findByNamespaceAndName(String ns, String name) {
        try { byte[] d = natsRequest("catalog.system.get", Map.of("namespace",ns,"name",name));
            var n = mapper.readTree(d); if (n.has("error")||!n.has("namespace")) return Optional.empty();
            var r = mapper.readValue(d, SysResp.class);
            return Optional.of(new SystemRecord(r.namespace,r.name,r.source,r.state,r.health,r.version,Instant.parse(r.createdAt),Instant.parse(r.updatedAt)));
        } catch (Exception e) { return Optional.empty(); }
    }
    @Override public List<SystemRecord> findByNamespace(String ns) {
        try { var r = natsRequestAndParse("catalog.system.list", Map.of("namespace",ns), SysListResp.class);
            List<SystemRecord> out = new ArrayList<>();
            for (var s : r.systems) out.add(new SystemRecord(s.namespace,s.name,s.source,s.state,s.health,s.version,Instant.parse(s.createdAt),Instant.parse(s.updatedAt)));
            return out; } catch (Exception e) { return List.of(); }
    }
    @Override public List<SystemRecord> findAllSystems() { return findByNamespace(""); }
    @Override public void delete(String ns, String name) { natsRequest("catalog.system.delete", Map.of("namespace",ns,"name",name)); }
    @Override public void updateState(String ns, String name, String state, String health, long version) {
        Map<String,Object> r = new LinkedHashMap<>();
        r.put("namespace",ns); r.put("name",name); r.put("state",state); r.put("health",health); r.put("version",version);
        natsRequest("catalog.system.updateState", r);
    }

    // --- NodeRepository ---
    @Override public void save(NodeRecord n) { natsRequest("catalog.node.save", nodeToReq(n)); }
    @Override public void saveAll(List<NodeRecord> nodes) {
        List<Map<String,Object>> reqs = new ArrayList<>(); for (NodeRecord n : nodes) reqs.add(nodeToReq(n));
        natsRequest("catalog.node.saveAll", Map.of("nodes", reqs));
    }
    private Map<String,Object> nodeToReq(NodeRecord n) {
        Map<String,Object> r = new LinkedHashMap<>();
        r.put("namespace",n.namespace()); r.put("systemName",n.systemName()); r.put("name",n.name());
        r.put("uri",n.uri()); r.put("category",n.category()); r.put("state",n.state()); r.put("health",n.health());
        r.put("version",n.version()); r.put("listens",n.listens()); r.put("events",n.events());
        if (n.onFailureRetry()!=null) r.put("onFailureRetry",n.onFailureRetry());
        if (n.onFailureRouteTo()!=null) r.put("onFailureRouteTo",n.onFailureRouteTo());
        if (n.timeout()!=null) r.put("timeout",n.timeout());
        return r;
    }
    @Override public Optional<NodeRecord> find(String ns, String sys, String node) {
        return findBySystem(ns,sys).stream().filter(n->n.name().equals(node)).findFirst();
    }
    @Override public List<NodeRecord> findBySystem(String ns, String sys) {
        try { var r = natsRequestAndParse("catalog.node.list", Map.of("namespace",ns,"systemName",sys), NodeListResp.class);
            return parseNodes(r.nodes); } catch (Exception e) { return List.of(); }
    }
    @Override public List<NodeRecord> findNodesByNamespace(String ns) {
        try { var r = natsRequestAndParse("catalog.node.list", Map.of("namespace",ns), NodeListResp.class);
            return parseNodes(r.nodes); } catch (Exception e) { return List.of(); }
    }
    private List<NodeRecord> parseNodes(NodeResp[] nodes) {
        List<NodeRecord> out = new ArrayList<>();
        for (var n : nodes) out.add(new NodeRecord(n.namespace,n.systemName,n.name,n.uri,n.category,
            n.state,n.health,n.version,n.errorMessage,
            n.listens!=null?n.listens:List.of(), n.events!=null?n.events:List.of(),
            n.config!=null?n.config:Map.of(), n.labels!=null?n.labels:Map.of(), n.annotations!=null?n.annotations:Map.of(),
            n.onFailureRetry,n.onFailureRouteTo,n.timeout, Instant.parse(n.createdAt),Instant.parse(n.updatedAt)));
        return out;
    }
    @Override public void delete(String ns, String sys, String node) { log.warn("Individual node delete not yet supported"); }
    @Override public void deleteBySystem(String ns, String sys) { natsRequest("catalog.node.delete", Map.of("namespace",ns,"systemName",sys)); }
    @Override public void updateState(String ns, String sys, String node, String state, String health, long version, String errMsg) {
        find(ns,sys,node).ifPresent(n -> save(new NodeRecord(n.namespace(),n.systemName(),n.name(),n.uri(),n.category(),
            state,health,version,errMsg,n.listens(),n.events(),n.config(),n.labels(),n.annotations(),
            n.onFailureRetry(),n.onFailureRouteTo(),n.timeout(),n.createdAt(),Instant.now())));
    }

    // --- EventRepository / EventStore ---
    @Override public void append(NodeEvent e) {
        if (e==null) return;
        try { natsRequest("catalog.event.append", eventToReq(e)); } catch (Exception ex) { log.error("append failed",ex); }
    }
    @Override public void appendAll(List<NodeEvent> events) {
        if (events==null||events.isEmpty()) return;
        try { List<Map<String,Object>> reqs = new ArrayList<>(); for (NodeEvent e : events) reqs.add(eventToReq(e));
            natsRequest("catalog.event.appendBatch", Map.of("events",reqs)); } catch (Exception ex) { log.error("appendBatch failed",ex); }
    }
    private Map<String,Object> eventToReq(NodeEvent e) {
        Map<String,Object> r = new LinkedHashMap<>();
        r.put("id",e.id().toString()); r.put("kind",e.kind().name()); r.put("nodeName",e.nodeName());
        r.put("systemName",e.systemName()); r.put("namespace",e.namespace()); r.put("timestamp",e.timestamp().toString());
        if (e.payload()!=null&&!e.payload().isEmpty()) r.put("payload",e.payload());
        return r;
    }
    @Override public List<NodeEvent> query(EventFilter f) {
        Map<String,Object> r = new LinkedHashMap<>();
        if (f.namespace()!=null) r.put("namespace",f.namespace());
        if (f.systemName()!=null) r.put("systemName",f.systemName());
        if (f.nodeName()!=null) r.put("nodeName",f.nodeName());
        if (f.kinds()!=null&&!f.kinds().isEmpty()) r.put("kinds",f.kinds().stream().map(NodeEventKind::name).toList());
        r.put("limit", f.limit()>0?f.limit():100);
        try { var resp = natsRequestAndParse("catalog.event.query", r, EventListResp.class);
            List<NodeEvent> out = new ArrayList<>();
            for (var e : resp.events) out.add(new NodeEvent(UUID.fromString(e.id),NodeEventKind.valueOf(e.kind),
                e.nodeName,e.systemName,e.namespace,Instant.parse(e.timestamp),e.payload!=null?e.payload:Map.of()));
            return out; } catch (Exception ex) { return List.of(); }
    }
    @Override public long count(EventFilter f) {
        Map<String,Object> r = new LinkedHashMap<>();
        if (f.namespace()!=null) r.put("namespace",f.namespace());
        if (f.systemName()!=null) r.put("systemName",f.systemName());
        if (f.nodeName()!=null) r.put("nodeName",f.nodeName());
        if (f.kinds()!=null&&!f.kinds().isEmpty()) r.put("kinds",f.kinds().stream().map(NodeEventKind::name).toList());
        try { return natsRequestAndParse("catalog.event.count", r, CountResp.class).count; } catch (Exception e) { return 0; }
    }
    @Override public int deleteOlderThan(Instant cutoff, int limit) { return 0; }

    // --- SourceRepository ---
    @Override public void saveSource(String ns, String name, String source) { natsRequest("catalog.source.save", Map.of("namespace",ns,"name",name,"source",source)); }
    @Override public Optional<String> getSource(String ns, String name) {
        try { byte[] d = natsRequest("catalog.source.get", Map.of("namespace",ns,"name",name));
            var n = mapper.readTree(d); if (n.has("error")) return Optional.empty();
            return Optional.of(mapper.readValue(d, SrcResp.class).source); } catch (Exception e) { return Optional.empty(); }
    }
    @Override public List<SourceEntry> listSources() {
        try { var r = natsRequestAndParse("catalog.source.list", Map.of(), SrcListResp.class);
            List<SourceEntry> out = new ArrayList<>(); for (var s : r.sources) out.add(new SourceEntry(s.namespace,s.name));
            return out; } catch (Exception e) { return List.of(); }
    }

    // --- RegistryRepository ---
    @Override public void save(RegistryRecord rec) {
        Map<String,Object> r = new LinkedHashMap<>();
        r.put("uri",rec.uri()); r.put("pattern",rec.pattern()); r.put("category",rec.category());
        r.put("active",rec.active()); r.put("description",rec.description());
        natsRequest("catalog.registry.save", r);
    }
    @Override public Optional<RegistryRecord> findByUri(String uri) {
        try { byte[] d = natsRequest("catalog.registry.find", Map.of("uri",uri));
            var n = mapper.readTree(d); if (n.has("error")) return Optional.empty();
            var r = mapper.readValue(d, RegResp.class);
            return Optional.of(new RegistryRecord(r.uri,r.pattern,r.category,r.active,r.description)); } catch (Exception e) { return Optional.empty(); }
    }
    @Override public List<RegistryRecord> findAllRegistry() {
        try { var r = natsRequestAndParse("catalog.registry.list", Map.of(), RegListResp.class);
            List<RegistryRecord> out = new ArrayList<>(); for (var rec : r.records)
                out.add(new RegistryRecord(rec.uri,rec.pattern,rec.category,rec.active,rec.description));
            return out; } catch (Exception e) { return List.of(); }
    }
    @Override public List<RegistryRecord> findByCategory(String cat) { return findAllRegistry().stream().filter(r->r.category().equals(cat)).toList(); }
    @Override public List<RegistryRecord> search(String kw) { return findAllRegistry().stream().filter(r->r.uri().contains(kw)||r.description().contains(kw)).toList(); }
    @Override public boolean existsByUri(String uri) {
        try { return natsRequestAndParse("catalog.registry.exists", Map.of("uri",uri), ExistsResp.class).exists; } catch (Exception e) { return false; }
    }

    // --- Response DTOs ---
    private record SysResp(String namespace,String name,String source,String state,String health,long version,String createdAt,String updatedAt) {}
    private record SysListResp(SysResp[] systems) {}
    private record NodeResp(String namespace,String systemName,String name,String uri,String category,String state,String health,long version,String errorMessage,List<String> listens,List<String> events,Map<String,Object> config,Map<String,String> labels,Map<String,String> annotations,String onFailureRetry,String onFailureRouteTo,String timeout,String createdAt,String updatedAt) {}
    private record NodeListResp(NodeResp[] nodes) {}
    private record EventResp(String id,String kind,String nodeName,String systemName,String namespace,String timestamp,Map<String,Object> payload) {}
    private record EventListResp(EventResp[] events) {}
    private record CountResp(long count) {}
    private record SrcResp(String source) {}
    private record SrcEntry(String namespace,String name) {}
    private record SrcListResp(SrcEntry[] sources) {}
    private record RegResp(String uri,String pattern,String category,boolean active,String description) {}
    private record RegListResp(RegResp[] records) {}
    private record ExistsResp(boolean exists) {}
}
