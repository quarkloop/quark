package com.quarkloop.quark.runtime;

import com.quarkloop.quark.core.domain.event.NodeEvent;
import com.quarkloop.quark.core.domain.event.NodeEventKind;
import com.quarkloop.quark.core.domain.identity.Namespace;
import com.quarkloop.quark.core.domain.system.NodeDefinition;
import com.quarkloop.quark.core.domain.system.SystemDefinition;
import com.quarkloop.quark.core.engine.lifecycle.RuntimeSystem;
import com.quarkloop.quark.core.engine.lifecycle.SystemDeployer;
import com.quarkloop.quark.core.event.EventBus;
import com.quarkloop.quark.core.script.SystemParseResult;
import com.quarkloop.quark.core.script.SystemParser;
import com.quarkloop.quark.runtime.script.GraalJsSystemParser;
import jakarta.enterprise.context.ApplicationScoped;
import jakarta.inject.Inject;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import java.util.Map;

/**
 * Data-plane deploy service — receives deploy commands from the control plane
 * and executes them locally via SystemDeployer.
 *
 * This is the runtime-side counterpart to the server's DeployService.
 * It only has the deployLocally() logic (no NATS, no persistence).
 */
@ApplicationScoped
public class RuntimeDeployService {

    private static final Logger log = LoggerFactory.getLogger(RuntimeDeployService.class);

    private final SystemDeployer systemDeployer;
    private final EventBus eventBus;
    private final SystemParser parser;

    @Inject
    public RuntimeDeployService(SystemDeployer systemDeployer, EventBus eventBus,
                                GraalJsSystemParser parser) {
        this.systemDeployer = systemDeployer;
        this.eventBus = eventBus;
        this.parser = parser;
    }

    public RuntimeSystem deploy(String source, String namespaceOverride) {
        SystemParseResult parseResult = parser.parse(source);
        if (parseResult instanceof SystemParseResult.Failure failure) {
            throw new RuntimeException("Parse failed: " + String.join("; ", failure.errors()));
        }
        SystemParseResult.Success success = (SystemParseResult.Success) parseResult;
        SystemDefinition systemDef = success.system();

        if (namespaceOverride != null && !namespaceOverride.isBlank()
                && !namespaceOverride.equals(systemDef.namespace().value())) {
            systemDef = new SystemDefinition(
                    systemDef.name(), Namespace.of(namespaceOverride),
                    systemDef.nodes(), systemDef.runtime());
        }

        RuntimeSystem runtime = systemDeployer.deploy(systemDef);

        for (NodeDefinition nodeDef : systemDef.nodes().values()) {
            NodeEvent created = NodeEvent.of(
                    NodeEventKind.NODE_CREATED, nodeDef.name(),
                    systemDef.name(), systemDef.namespace().value(),
                    Map.of("uri", nodeDef.uri().toString(),
                            "listens", nodeDef.listens(),
                            "events", nodeDef.events()));
            eventBus.publish(created);
        }

        log.info("Deployed system {}/{} ({} nodes) in data-plane mode",
                systemDef.namespace().value(), systemDef.name(), systemDef.nodes().size());
        return runtime;
    }

    public void undeploy(String namespace, String systemName) {
        systemDeployer.undeploy(namespace, systemName);
        log.info("Undeployed system {}/{} (data-plane mode)", namespace, systemName);
    }
}
