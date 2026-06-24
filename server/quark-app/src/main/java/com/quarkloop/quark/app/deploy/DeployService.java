package com.quarkloop.quark.app.deploy;

import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.datatype.jsr310.JavaTimeModule;
import com.quarkloop.quark.app.dataplane.ProcessManager;
import com.quarkloop.quark.core.domain.event.NodeEvent;
import com.quarkloop.quark.core.domain.event.NodeEventKind;
import com.quarkloop.quark.core.domain.identity.Namespace;
import com.quarkloop.quark.core.domain.system.NodeDefinition;
import com.quarkloop.quark.core.domain.system.SystemDefinition;
import com.quarkloop.quark.core.engine.dataplane.DataPlaneIpc;
import com.quarkloop.quark.core.engine.lifecycle.DeploymentException;
import com.quarkloop.quark.core.engine.lifecycle.RuntimeSystem;
import com.quarkloop.quark.core.engine.lifecycle.SystemDeployer;
import com.quarkloop.quark.core.engine.metrics.NamespaceMetrics;
import com.quarkloop.quark.core.engine.runtime.RuntimeContext;
import com.quarkloop.quark.core.engine.store.NodeRecord;
import com.quarkloop.quark.core.engine.store.NodeRepository;
import com.quarkloop.quark.core.engine.store.SourceRepository;
import com.quarkloop.quark.core.engine.store.SystemRecord;
import com.quarkloop.quark.core.engine.store.SystemRepository;
import com.quarkloop.quark.core.event.EventBus;
import com.quarkloop.quark.core.script.SystemParseResult;
import com.quarkloop.quark.core.script.SystemParser;
import com.quarkloop.quark.core.engine.nats.NatsConnectionManager;
import io.nats.client.Connection;
import io.nats.client.Message;
import io.quarkus.runtime.StartupEvent;
import jakarta.annotation.Priority;
import jakarta.enterprise.context.ApplicationScoped;
import jakarta.enterprise.event.Observes;
import jakarta.inject.Inject;
import org.eclipse.microprofile.config.inject.ConfigProperty;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import java.nio.charset.StandardCharsets;
import java.time.Duration;
import java.util.ArrayList;
import java.util.List;
import java.util.Map;

/**
 * Application-layer orchestration for system deployment.
 *
 * <p>Supports two execution modes controlled by {@code quark.mode}:
 * <ul>
 *   <li><b>standalone</b> (default, control plane) — parses the source,
 *       persists the system record to the Catalog, determines the runtime mode
 *       (shared/isolated), ensures the appropriate data-plane process is
 *       running, and sends the deploy command to the data plane via NATS.
 *       Waits for the data-plane's status response (with a 30s timeout).</li>
 *   <li><b>data</b> (data-plane process) — receives deploy commands via NATS
 *       (handled by {@code DataPlaneCommandHandler}) and calls
 *       {@link SystemDeployer#deploy} directly. The deploy/undeploy methods
 *       in this class are NOT called directly in data-plane mode — the
 *       {@code DataPlaneCommandHandler} calls them.</li>
 * </ul>
 *
 * <p>Persistence (Catalog system record + source) is always handled by the
 * control plane, never the data plane. The data plane only executes systems.
 *
 * <p>The CLI's deploy request body carries the source AND a namespace
 * argument. If the {@code .ts} file's own {@code namespace} field differs,
 * the request's namespace wins.
 */
@ApplicationScoped
public class DeployService {

    private static final Logger log = LoggerFactory.getLogger(DeployService.class);

    private final ObjectMapper mapper = new ObjectMapper();
    { mapper.registerModule(new JavaTimeModule()); }

    private final SystemParser parser;
    private final SystemDeployer systemDeployer;
    private final EventBus eventBus;
    private final SystemRepository systemRepository;
    private final NodeRepository nodeRepository;
    private final SourceRepository sourceRepository;
    private final NamespaceMetrics namespaceMetrics;
    private final RuntimeContext runtimeContext;
    private final ProcessManager processManager;
    private final NatsConnectionManager natsConnectionManager;

    @ConfigProperty(name = "quark.mode", defaultValue = "standalone")
    String mode;

    @Inject
    public DeployService(SystemParser parser,
                         SystemDeployer systemDeployer,
                         EventBus eventBus,
                         SystemRepository systemRepository,
                         NodeRepository nodeRepository,
                         SourceRepository sourceRepository,
                         NamespaceMetrics namespaceMetrics,
                         RuntimeContext runtimeContext,
                         ProcessManager processManager,
                         NatsConnectionManager natsConnectionManager) {
        this.parser = parser;
        this.systemDeployer = systemDeployer;
        this.eventBus = eventBus;
        this.systemRepository = systemRepository;
        this.nodeRepository = nodeRepository;
        this.sourceRepository = sourceRepository;
        this.namespaceMetrics = namespaceMetrics;
        this.runtimeContext = runtimeContext;
        this.processManager = processManager;
        this.natsConnectionManager = natsConnectionManager;
    }

    /**
     * Parse, validate, deploy, persist, and emit NODE_CREATED events.
     *
     * <p>In control-plane mode: parses the source, persists the system record,
     * ensures the data-plane process is running, sends the deploy command via
     * NATS, and waits for the status response.
     *
     * <p>In data-plane mode: called by {@code DataPlaneCommandHandler} —
     * parses, deploys locally via {@link SystemDeployer}, and emits events.
     * Does NOT persist (persistence is the control plane's responsibility).
     *
     * @param source      the {@code .quark.ts} source
     * @param namespaceOverride if non-null/non-blank, overrides the namespace
     * @return the resulting {@link RuntimeSystem} (may be a stub in control-plane mode)
     * @throws DeploymentException if parsing fails or deployment fails
     */
    public RuntimeSystem deploy(String source, String namespaceOverride) {
        // 1. Parse
        SystemParseResult parseResult = parser.parse(source);
        if (parseResult instanceof SystemParseResult.Failure failure) {
            throw new DeploymentException("Parse failed: " + String.join("; ", failure.errors()));
        }
        SystemParseResult.Success success = (SystemParseResult.Success) parseResult;
        SystemDefinition systemDef = success.system();

        // 2. Apply namespace override (preserving the runtime field)
        if (namespaceOverride != null && !namespaceOverride.isBlank()
                && !namespaceOverride.equals(systemDef.namespace().value())) {
            log.info("Overriding namespace: source='{}' -> request='{}'",
                    systemDef.namespace().value(), namespaceOverride);
            systemDef = new SystemDefinition(
                    systemDef.name(),
                    Namespace.of(namespaceOverride),
                    systemDef.nodes(),
                    systemDef.runtime()
            );
        }

        String ns = systemDef.namespace().value();
        String name = systemDef.name();
        boolean isIsolated = systemDef.isIsolated();

        // 3. In control-plane mode: persist + route to data plane via NATS.
        //    In data-plane mode: deploy locally.
        if ("standalone".equals(mode)) {
            return deployViaDataPlane(systemDef, source, ns, name, isIsolated);
        } else {
            return deployLocally(systemDef, source, ns, name);
        }
    }

    /**
     * Control-plane deploy: persist the system record, ensure the data-plane
     * process is running, send the deploy command via NATS, and wait for the
     * status response.
     */
    private RuntimeSystem deployViaDataPlane(SystemDefinition systemDef, String source,
                                              String ns, String name, boolean isIsolated) {
        // Persist the system record (with source) to the Catalog
        try {
            systemRepository.save(SystemRecord.creating(ns, name, source));
            systemRepository.updateState(ns, name, "ACTIVE", "HEALTHY", 1);
            log.debug("Persisted system record {}/{} to the Catalog", ns, name);
        } catch (Exception e) {
            log.warn("Failed to persist system record for {}/{} — deploy will continue", ns, name, e);
        }
        try {
            sourceRepository.saveSource(ns, name, source);
        } catch (Exception e) {
            log.warn("Failed to save source via SourceRepository for {}/{}", ns, name, e);
        }

        // Ensure the data-plane process is running
        String runtimeId = DataPlaneIpc.runtimeId(ns, isIsolated);
        try {
            processManager.ensureProcess(runtimeId);
        } catch (Exception e) {
            throw new DeploymentException("Failed to start data-plane process " + runtimeId + ": " + e.getMessage(), e);
        }

        // Send deploy command via NATS and wait for response.
        // Retry up to 5 times with 2s timeout each — the data-plane process
        // may have just been spawned and its NATS subscriptions may not be
        // ready yet even though the HTTP health endpoint is up.
        String deploySubject = DataPlaneIpc.deploySubject(runtimeId);
        try {
            String payload = mapper.writeValueAsString(
                    new DeployCommand(ns, name, source));
            Connection conn = natsConnectionManager.getConnection();
            Message reply = null;
            for (int attempt = 1; attempt <= 5; attempt++) {
                reply = conn.request(deploySubject,
                        payload.getBytes(StandardCharsets.UTF_8),
                        Duration.ofSeconds(3));
                if (reply != null) break;
                log.debug("Deploy attempt {} got no response (data-plane may still be starting up), retrying...", attempt);
                Thread.sleep(1000);
            }
            if (reply == null) {
                throw new DeploymentException("Data-plane did not respond to deploy command for "
                        + ns + "/" + name + " after 5 attempts (15s total)");
            }
            StatusResponse resp = mapper.readValue(reply.getData(), StatusResponse.class);
            if (!resp.success()) {
                throw new DeploymentException("Data-plane deploy failed for "
                        + ns + "/" + name + ": " + resp.error());
            }

            // Persist NodeRecords to the Catalog from the data plane's response.
            // The data plane cannot write to the Catalog (cross-process write
            // conflict), so it reports node info back and the control plane
            // persists it.
            if (resp.nodes() != null && !resp.nodes().isEmpty()) {
                try {
                    int saved = 0;
                    for (NodeInfo ni : resp.nodes()) {
                        NodeRecord record = new NodeRecord(
                                ns, name, ni.name(), ni.uri(), 
                                ni.state(), ni.health(), 1L, null,
                                ni.listens(), ni.events(),
                                Map.of(), Map.of(), Map.of(),
                                null, null, null,
                                null, null);
                        nodeRepository.save(record);
                        saved++;
                    }
                    log.debug("Persisted {} node records for {}/{} to the Catalog", saved, ns, name);
                } catch (Exception e) {
                    log.warn("Failed to persist node records for {}/{}", ns, name, e);
                }
            }

            log.info("Deployed system {}/{} via data-plane {} ({} nodes)",
                    ns, name, runtimeId, systemDef.nodes().size());
        } catch (DeploymentException e) {
            throw e;
        } catch (Exception e) {
            throw new DeploymentException("Failed to send deploy command to data plane: " + e.getMessage(), e);
        }

        // Note: NODE_CREATED events are emitted by the data plane (in
        // deployLocally) and forwarded to the control plane via NATS
        // (DataPlaneEventForwarder → ControlPlaneEventReceiver). The control
        // plane does NOT emit duplicate events here.

        // Return a stub RuntimeSystem (the real runtime lives in the data plane)
        return new RuntimeSystem(systemDef, systemDef.namespace());
    }

    /**
     * Data-plane deploy: deploy locally via SystemDeployer.
     * Does NOT persist (the control plane handles persistence).
     */
    private RuntimeSystem deployLocally(SystemDefinition systemDef, String source,
                                         String ns, String name) {
        RuntimeSystem runtime = systemDeployer.deploy(systemDef);

        // Emit NODE_CREATED events
        for (NodeDefinition nodeDef : systemDef.nodes().values()) {
            NodeEvent created = NodeEvent.of(
                    NodeEventKind.NODE_CREATED,
                    nodeDef.name(),
                    systemDef.name(),
                    systemDef.namespace().value(),
                    Map.of("uri", nodeDef.uri().toString(),
                            "listens", nodeDef.listens(),
                            "events", nodeDef.events())
            );
            eventBus.publish(created);
        }

        log.info("Deployed system {}/{} ({} nodes) in data-plane mode",
                ns, name, systemDef.nodes().size());
        return runtime;
    }

    /**
     * Undeploy a system.
     *
     * <p>In control-plane mode: sends the undeploy command to the data plane
     * via NATS, waits for the response, then deletes the system record from
     * Catalog. If the undeployed system was the last in an isolated namespace,
     * stops the dedicated data-plane process.
     *
     * <p>In data-plane mode: calls {@link SystemDeployer#undeploy} directly.
     */
    public void undeploy(String namespace, String systemName) {
        if ("standalone".equals(mode)) {
            undeployViaDataPlane(namespace, systemName);
        } else {
            undeployLocally(namespace, systemName);
        }
    }

    /**
     * Control-plane undeploy: send undeploy command via NATS, wait for
     * response, delete the system record, and optionally stop the data-plane
     * process for isolated namespaces.
     */
    private void undeployViaDataPlane(String namespace, String systemName) {
        // Determine runtimeId from the persisted system record
        // (we need to know if this was an isolated namespace)
        // For now, check if a dedicated process exists for this namespace
        String isolatedRuntimeId = DataPlaneIpc.runtimeId(namespace, true);
        String sharedRuntimeId = DataPlaneIpc.runtimeId(namespace, false);
        String runtimeId = processManager.isProcessAlive(isolatedRuntimeId)
                ? isolatedRuntimeId : sharedRuntimeId;

        // Send undeploy command via NATS
        String undeploySubject = DataPlaneIpc.undeploySubject(runtimeId);
        try {
            String payload = mapper.writeValueAsString(
                    new UndeployCommand(namespace, systemName));
            Message reply = natsConnectionManager.getConnection().request(undeploySubject,
                    payload.getBytes(StandardCharsets.UTF_8),
                    Duration.ofSeconds(15));
            if (reply == null) {
                log.warn("Data-plane did not respond to undeploy command for {}/{} within 15s",
                        namespace, systemName);
            } else {
                StatusResponse resp = mapper.readValue(reply.getData(), StatusResponse.class);
                if (!resp.success()) {
                    log.warn("Data-plane undeploy failed for {}/{}: {}",
                            namespace, systemName, resp.error());
                }
            }
        } catch (Exception e) {
            log.warn("Failed to send undeploy command to data plane for {}/{}",
                    namespace, systemName, e);
        }

        // Delete the system record and node records from the Catalog
        try {
            nodeRepository.deleteBySystem(namespace, systemName);
            systemRepository.delete(namespace, systemName);
            log.debug("Deleted system + node records for {}/{} from the Catalog", namespace, systemName);
        } catch (Exception e) {
            log.warn("Failed to delete records for {}/{}", namespace, systemName, e);
        }

        // If this was an isolated namespace and no systems remain, stop
        // the dedicated data-plane process
        if (runtimeId.equals(isolatedRuntimeId)) {
            try {
                long remaining = systemRepository.findByNamespace(namespace).size();
                if (remaining == 0) {
                    processManager.stopProcess(isolatedRuntimeId);
                    log.info("Stopped isolated data-plane process for namespace {} (no systems remaining)",
                            namespace);
                }
            } catch (Exception e) {
                log.warn("Failed to check remaining systems for namespace {}", namespace, e);
            }
        }

        // Clean up metrics
        try {
            if (runtimeContext.getSystemsByNamespace(namespace).isEmpty()) {
                namespaceMetrics.remove(namespace);
            }
        } catch (Exception ignored) {
            // RuntimeContext may not have the system if it was in the data plane
        }

        log.info("Undeployed system {}/{}", namespace, systemName);
    }

    /**
     * Data-plane undeploy: call SystemDeployer.undeploy directly.
     */
    private void undeployLocally(String namespace, String systemName) {
        systemDeployer.undeploy(namespace, systemName);
        if (runtimeContext.getSystemsByNamespace(namespace).isEmpty()) {
            namespaceMetrics.remove(namespace);
        }
        log.info("Undeployed system {}/{} (data-plane mode)", namespace, systemName);
    }

    /**
     * Validate-only entry point: parse and return either the resulting
     * SystemDefinition (success) or throw with the list of parse errors.
     */
    public SystemDefinition parseOnly(String source) {
        SystemParseResult result = parser.parse(source);
        if (result instanceof SystemParseResult.Failure failure) {
            throw new DeploymentException("Parse failed: " + String.join("; ", failure.errors()));
        }
        return ((SystemParseResult.Success) result).system();
    }

    /**
     * On server startup, recover any previously-deployed systems from the
     * source repository. Only runs in control-plane mode (the data plane
     * recovers systems received via NATS deploy commands from the control
     * plane).
     *
     * <p>Runs at priority 10 (after registry init at 1, ProcessManager at 2,
     * metrics collector at 5).
     */
    void onStart(@Observes @Priority(10) StartupEvent event) {
        if (!"standalone".equals(mode)) {
            log.debug("Skipping recovery (mode={}, not standalone)", mode);
            return;
        }
        List<String> recovered = recoverFromDisk();
        if (!recovered.isEmpty()) {
            log.info("Recovered {} system(s) from source repository: {}", recovered.size(), recovered);
        }
    }

    /**
     * Recover previously-deployed systems on server startup.
     */
    public List<String> recoverFromDisk() {
        List<String> recovered = new ArrayList<>();
        List<SourceRepository.SourceEntry> entries;
        try {
            entries = sourceRepository.listSources();
        } catch (Exception e) {
            log.warn("Failed to list sources for recovery — platform will start with no systems", e);
            return recovered;
        }
        for (SourceRepository.SourceEntry entry : entries) {
            String namespace = entry.namespace();
            String systemName = entry.name();
            try {
                String source = sourceRepository.getSource(namespace, systemName)
                        .orElseThrow(() -> new IllegalStateException(
                                "Source listed but not found: " + namespace + "/" + systemName));
                deploy(source, namespace);
                recovered.add(namespace + "/" + systemName);
                log.info("Recovered system {}/{}", namespace, systemName);
            } catch (Exception e) {
                log.warn("Failed to recover system {}/{}", namespace, systemName, e);
            }
        }
        return recovered;
    }

    // --- IPC DTOs (shared with DataPlaneCommandHandler) ---

    public record DeployCommand(String namespace, String systemName, String source) {}
    public record UndeployCommand(String namespace, String systemName) {}

    /**
     * Status response from the data plane after a deploy/undeploy command.
     * Includes node info so the control plane can persist NodeRecords to the Catalog
     * (the data plane cannot write to the Catalog because the Catalog doesn't support
     * cross-process write access).
     */
    public record StatusResponse(boolean success, String systemName, String namespace,
                                  String error, List<NodeInfo> nodes) {
        /** Convenience constructor for undeploy (no nodes to report). */
        public StatusResponse(boolean success, String systemName, String namespace, String error) {
            this(success, systemName, namespace, error, List.of());
        }
    }

    /**
     * Node info reported by the data plane after deploy. Contains enough
     * data for the control plane to persist a NodeRecord.
     */
    public record NodeInfo(
            String name, String uri,
            String state, String health,
            List<String> listens, List<String> events
    ) {}
}
