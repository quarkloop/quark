package com.quarkloop.quark.app.deploy;

import com.quarkloop.quark.adapter.state.StateRoot;
import com.quarkloop.quark.core.domain.event.NodeEvent;
import com.quarkloop.quark.core.domain.event.NodeEventKind;
import com.quarkloop.quark.core.domain.identity.Namespace;
import com.quarkloop.quark.core.domain.system.NodeDefinition;
import com.quarkloop.quark.core.domain.system.SystemDefinition;
import com.quarkloop.quark.core.engine.lifecycle.DeploymentException;
import com.quarkloop.quark.core.engine.lifecycle.RuntimeSystem;
import com.quarkloop.quark.core.engine.lifecycle.SystemDeployer;
import com.quarkloop.quark.core.event.EventBus;
import com.quarkloop.quark.core.script.SystemParseResult;
import com.quarkloop.quark.core.script.SystemParser;
import com.quarkloop.quark.engine.SystemRunner;
import jakarta.enterprise.context.ApplicationScoped;
import jakarta.inject.Inject;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import java.io.IOException;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.ArrayList;
import java.util.List;
import java.util.Map;

/**
 * Application-layer orchestration for system deployment.
 *
 * <p>Chains together the three concerns that must run on every deploy:
 * <ol>
 *   <li><b>Parse</b> — {@link SystemParser} evaluates the {@code .quark.ts}
 *       source and produces a {@link SystemDefinition}.</li>
 *   <li><b>Execute</b> — {@link SystemRunner} wires NATS subscriptions,
 *       publishers, and starts Source/Endpoint providers.</li>
 *   <li><b>Track</b> — {@link SystemDeployer} records the runtime in the
 *       {@code SystemRuntimeRegistry} for lifecycle/health queries.</li>
 *   <li><b>Persist</b> — {@link StateRoot} writes the original source to
 *       {@code $STATE_ROOT/systems/<ns>/<sys>/source.ts} so the system
 *       can be recovered on server restart.</li>
 * </ol>
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
    private final SystemRunner systemRunner;
    private final SystemDeployer systemDeployer;
    private final EventBus eventBus;
    private final StateRoot stateRoot;

    @Inject
    public DeployService(SystemParser parser,
                         SystemRunner systemRunner,
                         SystemDeployer systemDeployer,
                         EventBus eventBus,
                         StateRoot stateRoot) {
        this.parser = parser;
        this.systemRunner = systemRunner;
        this.systemDeployer = systemDeployer;
        this.eventBus = eventBus;
        this.stateRoot = stateRoot;
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

        // 2. Apply namespace override
        if (namespaceOverride != null && !namespaceOverride.isBlank()
                && !namespaceOverride.equals(systemDef.namespace().value())) {
            log.info("Overriding namespace: source='{}' -> request='{}'",
                    systemDef.namespace().value(), namespaceOverride);
            systemDef = new SystemDefinition(
                    systemDef.name(),
                    Namespace.of(namespaceOverride),
                    systemDef.nodes()
            );
        }

        // 3. Execute on NATS (wires subscriptions, starts sources/endpoints)
        systemRunner.deploy(systemDef);

        // 4. Track in runtime registry + start lifecycle (CREATING -> ACTIVE)
        RuntimeSystem runtime = systemDeployer.deploy(systemDef);

        // 5. Persist the original source so we can recover on restart
        try {
            persistSource(systemDef, source);
        } catch (IOException e) {
            log.warn("Failed to persist source for system {}/{} — deploy will continue",
                    systemDef.namespace().value(), systemDef.name(), e);
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

        log.info("Deployed system {}/{} ({} nodes)",
                systemDef.namespace().value(), systemDef.name(), systemDef.nodes().size());
        return runtime;
    }

    /**
     * Undeploy a single system: stop sources/endpoints, close NATS
     * dispatchers, remove from runtime registry, and emit a final
     * NODE_STATE_CHANGED -> ARCHIVED event per node.
     */
    public void undeploy(String namespace, String systemName) {
        Namespace ns = Namespace.of(namespace);
        // Stop NATS wiring + provider resources
        systemRunner.undeploy(namespace, systemName);
        // Remove from runtime registry + stop lifecycle
        systemDeployer.undeploy(ns, systemName);
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

    private void persistSource(SystemDefinition systemDef, String source) throws IOException {
        Path sourceFile = stateRoot.systemSourceFile(
                systemDef.namespace().value(), systemDef.name());
        Files.createDirectories(sourceFile.getParent());
        Files.writeString(sourceFile, source, StandardCharsets.UTF_8);
        log.debug("Persisted source to {}", sourceFile);
    }

    /**
     * Recover previously-deployed systems on server startup. Scans the
     * state root for {@code source.ts} files and re-deploys them.
     */
    public List<String> recoverFromDisk() {
        List<String> recovered = new ArrayList<>();
        Path systemsDir = stateRoot.systemsDir();
        if (!Files.isDirectory(systemsDir)) {
            return recovered;
        }
        try (var nsStream = Files.list(systemsDir)) {
            for (Path nsDir : nsStream.toList()) {
                if (!Files.isDirectory(nsDir)) continue;
                String namespace = nsDir.getFileName().toString();
                try (var sysStream = Files.list(nsDir)) {
                    for (Path sysDir : sysStream.toList()) {
                        if (!Files.isDirectory(sysDir)) continue;
                        Path sourceFile = sysDir.resolve("source.ts");
                        if (!Files.isRegularFile(sourceFile)) continue;
                        String systemName = sysDir.getFileName().toString();
                        try {
                            String source = Files.readString(sourceFile, StandardCharsets.UTF_8);
                            deploy(source, namespace);
                            recovered.add(namespace + "/" + systemName);
                            log.info("Recovered system {}/{}", namespace, systemName);
                        } catch (Exception e) {
                            log.warn("Failed to recover system {}/{}", namespace, systemName, e);
                        }
                    }
                } catch (IOException e) {
                    log.warn("Error scanning namespace dir {}", nsDir, e);
                }
            }
        } catch (IOException e) {
            log.warn("Error scanning systems dir for recovery", e);
        }
        return recovered;
    }
}
