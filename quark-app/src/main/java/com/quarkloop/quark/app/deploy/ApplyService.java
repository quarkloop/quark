package com.quarkloop.quark.app.deploy;

import com.quarkloop.quark.core.domain.identity.Namespace;
import com.quarkloop.quark.core.domain.system.NodeDefinition;
import com.quarkloop.quark.core.domain.system.SystemDefinition;
import com.quarkloop.quark.core.engine.lifecycle.DeploymentException;
import com.quarkloop.quark.core.engine.runtime.RuntimeContext;
import com.quarkloop.quark.core.script.SystemParseResult;
import com.quarkloop.quark.core.script.SystemParser;
import jakarta.enterprise.context.ApplicationScoped;
import jakarta.inject.Inject;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import java.util.ArrayList;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;

@ApplicationScoped
public class ApplyService {

    private static final Logger log = LoggerFactory.getLogger(ApplyService.class);

    private final SystemParser parser;
    private final DeployService deployService;
    private final RuntimeContext runtimeContext;

    @Inject
    public ApplyService(SystemParser parser, DeployService deployService, RuntimeContext runtimeContext) {
        this.parser = parser;
        this.deployService = deployService;
        this.runtimeContext = runtimeContext;
    }

    public ApplyResult apply(String source, String namespaceOverride) {
        SystemParseResult parseResult = parser.parse(source);
        if (parseResult instanceof SystemParseResult.Failure failure) {
            throw new DeploymentException("Parse failed: " + String.join("; ", failure.errors()));
        }
        SystemDefinition desired = ((SystemParseResult.Success) parseResult).system();

        if (namespaceOverride != null && !namespaceOverride.isBlank()
                && !namespaceOverride.equals(desired.namespace().value())) {
            desired = new SystemDefinition(desired.name(), Namespace.of(namespaceOverride),
                    desired.nodes(), desired.runtime());
        }

        String ns = desired.namespace().value();
        String name = desired.name();
        boolean exists = runtimeContext.getSystem(ns, name).isPresent();

        if (!exists) {
            deployService.deploy(source, namespaceOverride);
            return ApplyResult.created(desired, List.of());
        }

        List<Change> changes = computeDiff(desired);
        if (changes.isEmpty()) return ApplyResult.unchanged(desired);

        log.info("Apply: {} changes for {}/{} — full redeploy", changes.size(), ns, name);
        deployService.deploy(source, namespaceOverride);
        return ApplyResult.updated(desired, changes);
    }

    private List<Change> computeDiff(SystemDefinition desired) {
        var currentOpt = runtimeContext.getSystem(desired.namespace().value(), desired.name());
        if (currentOpt.isEmpty()) {
            List<Change> changes = new ArrayList<>();
            for (String n : desired.nodes().keySet()) changes.add(new Change(ChangeType.ADD_NODE, n, "new node"));
            return changes;
        }
        Map<String, String> currentNodes = new LinkedHashMap<>();
        for (var rn : currentOpt.get().nodes()) currentNodes.put(rn.definition().name(), rn.definition().uri().toString());

        List<Change> changes = new ArrayList<>();
        for (NodeDefinition desiredNode : desired.nodes().values()) {
            String n = desiredNode.name();
            if (!currentNodes.containsKey(n)) changes.add(new Change(ChangeType.ADD_NODE, n, "new node"));
            else if (!currentNodes.get(n).equals(desiredNode.uri().toString())) changes.add(new Change(ChangeType.UPDATE_NODE, n, "uri changed"));
        }
        for (String cn : currentNodes.keySet())
            if (!desired.nodes().containsKey(cn)) changes.add(new Change(ChangeType.REMOVE_NODE, cn, "removed"));
        return changes;
    }

    public enum ChangeType { ADD_NODE, REMOVE_NODE, UPDATE_NODE }
    public record Change(ChangeType type, String node, String details) {}
    public record ApplyResult(String name, String namespace, boolean created, boolean changed, List<Change> changes) {
        public static ApplyResult created(SystemDefinition d, List<Change> c) { return new ApplyResult(d.name(), d.namespace().value(), true, true, c); }
        public static ApplyResult updated(SystemDefinition d, List<Change> c) { return new ApplyResult(d.name(), d.namespace().value(), false, true, c); }
        public static ApplyResult unchanged(SystemDefinition d) { return new ApplyResult(d.name(), d.namespace().value(), false, false, List.of()); }
    }
}
