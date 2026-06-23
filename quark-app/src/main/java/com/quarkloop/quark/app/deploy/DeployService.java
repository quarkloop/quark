package com.quarkloop.quark.app.deploy;

import com.quarkloop.quark.core.domain.event.NodeEvent;
import com.quarkloop.quark.core.domain.event.NodeEventKind;
import com.quarkloop.quark.core.domain.identity.Namespace;
import com.quarkloop.quark.core.domain.system.NodeDefinition;
import com.quarkloop.quark.core.domain.system.SystemDefinition;
import com.quarkloop.quark.core.engine.lifecycle.DeploymentException;
import com.quarkloop.quark.core.engine.lifecycle.RuntimeSystem;
import com.quarkloop.quark.core.engine.lifecycle.SystemDeployer;
import com.quarkloop.quark.core.engine.metrics.NamespaceMetrics;
import com.quarkloop.quark.core.engine.runtime.RuntimeContext;
import com.quarkloop.quark.core.engine.store.SourceRepository;
import com.quarkloop.quark.core.engine.store.SystemRecord;
import com.quarkloop.quark.core.engine.store.SystemRepository;
import com.quarkloop.quark.core.event.EventBus;
import com.quarkloop.quark.core.script.SystemParseResult;
import com.quarkloop.quark.core.script.SystemParser;
import io.quarkus.runtime.StartupEvent;
import jakarta.annotation.Priority;
import jakarta.enterprise.context.ApplicationScoped;
import jakarta.enterprise.event.Observes;
import jakarta.inject.Inject;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import java.util.ArrayList;
import java.util.List;
import java.util.Map;

/**
 * Application-layer orchestration for system deployment.
 *
 * <p>Chains together the concerns that must run on every deploy:
 * <ol>
 *   <li><b>Parse</b> — {@link SystemParser} evaluates the {@code .quark.ts}
 *       source and produces a {@link SystemDefinition}.</li>
 *   <li><b>Execute</b> — {@link SystemDeployer} wires NATS subscriptions,
 *       publishers, and starts Source/Endpoint providers.</li>
 *   <li><b>Persist</b> — {@link SystemRepository} stores a {@link SystemRecord}
 *       (containing the original source) in DuckDB so the system can be
 *       recovered on server restart. The {@link SourceRepository} SPI is
 *       also invoked for implementations that decouple source storage from
 *       system records.</li>
 *   <li><b>Emit</b> — fires {@code NODE_CREATED} events for each node.</li>
 * </ol>
 *
 * <p>Persistence is delegated to DuckDB (via {@link SystemRepository} and
 * {@link SourceRepository}). The legacy filesystem-based {@code StateRoot}
 * adapter has been removed; all source-of-truth state lives in DuckDB.
 *
 * <p>The CLI's deploy request body carries the source AND a namespace
 * argument. If the {@code .ts} file's own {@code namespace} field differs,
 * the request's namespace wins — this is what enables the multi-tenant
 * example to deploy the same file under "alice" and "bob" in turn.
 */
@ApplicationScoped
public class DeployService {

    private static final Logger log = LoggerFactory.getLogger(DeployService.class);

    private final SystemParser parser;
    private final SystemDeployer systemDeployer;
    private final EventBus eventBus;
    private final SystemRepository systemRepository;
    private final SourceRepository sourceRepository;
    private final NamespaceMetrics namespaceMetrics;
    private final RuntimeContext runtimeContext;

    @Inject
    public DeployService(SystemParser parser,
                         SystemDeployer systemDeployer,
                         EventBus eventBus,
                         SystemRepository systemRepository,
                         SourceRepository sourceRepository,
                         NamespaceMetrics namespaceMetrics,
                         RuntimeContext runtimeContext) {
        this.parser = parser;
        this.systemDeployer = systemDeployer;
        this.eventBus = eventBus;
        this.systemRepository = systemRepository;
        this.sourceRepository = sourceRepository;
        this.namespaceMetrics = namespaceMetrics;
        this.runtimeContext = runtimeContext;
    }

    /**
     * Parse, validate, deploy, persist, and emit NODE_CREATED events.
     *
     * @param source      the {@code .quark.ts} source
     * @param namespaceOverride if non-null/non-blank, overrides the namespace
     *                          declared in the source file (enables multi-tenant
     *                          deployment of the same file under different tenants)
     * @return the resulting {@link RuntimeSystem}
     * @throws DeploymentException if parsing fails or a referenced URI is not registered
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

        // 3. Deploy (single call — SystemDeployer handles bus wiring + lifecycle)
        RuntimeSystem runtime = systemDeployer.deploy(systemDef);

        // 4. Persist the system record (with source) to DuckDB so we can
        //    recover on restart. SystemRecord stores the original .quark.ts
        //    source in the `source` column of the `systems` table.
        String ns = systemDef.namespace().value();
        String name = systemDef.name();
        try {
            systemRepository.save(SystemRecord.creating(ns, name, source));
            systemRepository.updateState(ns, name, "ACTIVE", "HEALTHY", 1);
            log.debug("Persisted system record {}/{} to DuckDB", ns, name);
        } catch (Exception e) {
            log.warn("Failed to persist system record for {}/{} — deploy will continue",
                    ns, name, e);
        }

        // 5. Also invoke SourceRepository.saveSource() for SPI compliance.
        //    In DuckDBStore this is a no-op (source is stored via the systems
        //    table above), but future repository implementations may decouple
        //    source storage from system records.
        try {
            sourceRepository.saveSource(ns, name, source);
        } catch (Exception e) {
            log.warn("Failed to save source via SourceRepository for {}/{} — deploy will continue",
                    ns, name, e);
        }

        // 6. Emit NODE_CREATED events
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

        log.info("Deployed system {}/{} ({} nodes)", ns, name, systemDef.nodes().size());
        return runtime;
    }

    /**
     * Undeploy a single system: stop sources/endpoints, close NATS
     * dispatchers, remove from runtime registry, delete the persisted system
     * record, and emit a final NODE_STATE_CHANGED -> ARCHIVED event per node.
     */
    public void undeploy(String namespace, String systemName) {
        systemDeployer.undeploy(namespace, systemName);
        try {
            systemRepository.delete(namespace, systemName);
            log.debug("Deleted system record {}/{} from DuckDB", namespace, systemName);
        } catch (Exception e) {
            log.warn("Failed to delete system record for {}/{} — undeploy continues",
                    namespace, systemName, e);
        }
        // If no systems remain in this namespace, remove the namespace's
        // metrics counters so stale entries don't linger in the stats output.
        if (runtimeContext.getSystemsByNamespace(namespace).isEmpty()) {
            namespaceMetrics.remove(namespace);
            log.debug("Removed metrics counters for namespace {} (no systems remaining)", namespace);
        }
        log.info("Undeployed system {}/{}", namespace, systemName);
    }

    /**
     * Validate-only entry point: parse and return either the resulting
     * SystemDefinition (success) or throw with the list of parse errors.
     * Used by REST endpoints that have a separate validate path.
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
     * source repository. DuckDB stores the original {@code .quark.ts} source
     * for every system record; re-deploying each restores the platform to
     * its pre-restart state.
     *
     * <p>Runs at {@link Priority#MIN_PRIORITY} + 10 so that the
     * {@code RegistryInitializer} (priority 1) has already populated the
     * node registry before recovery attempts to look up node URIs.
     */
    void onStart(@Observes @Priority(10) StartupEvent event) {
        List<String> recovered = recoverFromDisk();
        if (!recovered.isEmpty()) {
            log.info("Recovered {} system(s) from source repository: {}", recovered.size(), recovered);
        }
    }

    /**
     * Recover previously-deployed systems on server startup. Lists every
     * (namespace, name) entry in the source repository, reads back the
     * original source, and re-deploys it.
     *
     * <p>Recovery is best-effort: a failure on one system is logged and
     * skipped so that one bad system cannot block recovery of the others.
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
}
