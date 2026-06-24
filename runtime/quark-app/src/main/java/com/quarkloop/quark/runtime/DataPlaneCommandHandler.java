package com.quarkloop.quark.runtime;

import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.datatype.jsr310.JavaTimeModule;
import com.quarkloop.quark.core.domain.system.NodeDefinition;
import com.quarkloop.quark.core.engine.dataplane.DataPlaneIpc;
import com.quarkloop.quark.core.engine.lifecycle.RuntimeNode;
import com.quarkloop.quark.core.engine.lifecycle.RuntimeSystem;
import com.quarkloop.quark.core.engine.lifecycle.SystemDeployer;

import com.quarkloop.quark.core.engine.nats.NatsConnectionManager;
import io.nats.client.Connection;
import io.nats.client.Dispatcher;
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
import java.util.ArrayList;
import java.util.List;
import java.util.Map;

/**
 * Data-plane command handler — runs inside each data-plane process.
 *
 * <p>Listens on NATS subjects for deploy/undeploy commands from the control
 * plane and executes them locally via {@link DeployService} /
 * {@link SystemDeployer}.
 *
 * <p>Subjects (scoped by runtimeId):
 * <ul>
 *   <li>{@code quark.control.<runtimeId>.deploy} — payload: JSON DeployCommand</li>
 *   <li>{@code quark.control.<runtimeId>.undeploy} — payload: JSON UndeployCommand</li>
 * </ul>
 *
 * <p>Responses are published on {@code quark.data.<runtimeId>.status}:
 * <pre>
 *   {"success":true,"systemName":"monitor","namespace":"alice"}
 *   {"success":false,"error":"Parse failed: ...","systemName":"monitor","namespace":"alice"}
 * </pre>
 *
 * <p>This bean is only active when {@code quark.mode=data}. In control-plane
 * mode ({@code quark.mode=standalone} or default), it does nothing.
 */
@ApplicationScoped
public class DataPlaneCommandHandler {

    private static final Logger log = LoggerFactory.getLogger(DataPlaneCommandHandler.class);

    private final ObjectMapper mapper = new ObjectMapper();
    { mapper.registerModule(new JavaTimeModule()); }

    @ConfigProperty(name = "quark.mode", defaultValue = "standalone")
    String mode;

    @ConfigProperty(name = "quark.dataplane.runtimeId", defaultValue = "none")
    String runtimeId;

    private final RuntimeDeployService deployService;
    private final NatsConnectionManager natsConnectionManager;

    @Inject
    public DataPlaneCommandHandler(RuntimeDeployService deployService,
                                    NatsConnectionManager natsConnectionManager) {
        this.deployService = deployService;
        this.natsConnectionManager = natsConnectionManager;
    }

    /**
     * On startup, if running in data-plane mode, subscribe to the
     * deploy/undeploy subjects for this process's runtimeId.
     *
     * <p>Runs at priority 20 (after the control plane's ProcessManager at
     * priority 2, and after DeployService recovery at priority 10) so that
     * recovery completes before the data-plane starts accepting commands.
     */
    void onStart(@Observes @Priority(20) StartupEvent event) {
        if (!"data".equals(mode)) {
            log.debug("DataPlaneCommandHandler inactive (mode={}, not data)", mode);
            return;
        }
        if (runtimeId == null || runtimeId.isBlank()) {
            log.error("Data-plane mode active but quark.dataplane.runtimeId is not set");
            return;
        }

        log.info("Data-plane command handler starting (runtimeId={})", runtimeId);

        String deploySubject = DataPlaneIpc.deploySubject(runtimeId);
        String undeploySubject = DataPlaneIpc.undeploySubject(runtimeId);

        Connection conn = natsConnectionManager.getConnection();
        Dispatcher dispatcher = conn.createDispatcher(this::handleMessage);
        dispatcher.subscribe(deploySubject);
        dispatcher.subscribe(undeploySubject);
        log.info("Subscribed to {} and {}", deploySubject, undeploySubject);
    }

    /**
     * NATS message handler — routes to deploy or undeploy based on the subject.
     */
    private void handleMessage(Message natsMsg) {
        String subject = natsMsg.getSubject();
        String body = new String(natsMsg.getData(), StandardCharsets.UTF_8);
        String replyTo = natsMsg.getReplyTo();

        try {
            if (subject.endsWith(".deploy")) {
                handleDeploy(body, replyTo);
            } else if (subject.endsWith(".undeploy")) {
                handleUndeploy(body, replyTo);
            } else {
                log.warn("Unknown command subject: {}", subject);
            }
        } catch (Exception e) {
            log.error("Error handling command on {}", subject, e);
            publishStatus(replyTo, false, null, null, e.getMessage());
        }
    }

    /**
     * Handle a deploy command.
     * Payload: {"namespace":"alice","systemName":"monitor","source":"export default {...}"}
     *
     * <p>After deploying, extracts node info from the {@link RuntimeSystem}
     * and includes it in the {@link StatusResponse} so the
     * control plane can persist {@link com.quarkloop.quark.core.engine.store.NodeRecord}s
     * to the Catalog (the data plane cannot write to the Catalog directly due to
     * cross-process write conflict).
     */
    private void handleDeploy(String body, String replyTo) throws Exception {
        DeployCommand cmd = mapper.readValue(body, DeployCommand.class);
        log.info("Received deploy command for {}/{}", cmd.namespace(), cmd.systemName());
        try {
            RuntimeSystem runtime = deployService.deploy(cmd.source(), cmd.namespace());
            // Extract node info for the control plane to persist
            List<NodeInfo> nodeInfos = new ArrayList<>();
            for (RuntimeNode rn : runtime.nodes()) {
                NodeDefinition def = runtime.definition().nodes().get(rn.definition().name());
                List<String> listens = def != null ? def.listens() : List.of();
                List<String> events = def != null ? def.events() : List.of();
                nodeInfos.add(new NodeInfo(
                        rn.definition().name(),
                        rn.definition().uri().toString(),
                        rn.definition().category().label(),
                        rn.state().name(),
                        rn.health().name(),
                        listens,
                        events
                ));
            }
            publishDeployStatus(replyTo, true, cmd.systemName(), cmd.namespace(), null, nodeInfos);
            log.info("Deploy succeeded for {}/{} ({} nodes reported)", cmd.namespace(), cmd.systemName(), nodeInfos.size());
        } catch (Exception e) {
            publishDeployStatus(replyTo, false, cmd.systemName(), cmd.namespace(), e.getMessage(), List.of());
            log.error("Deploy failed for {}/{}", cmd.namespace(), cmd.systemName(), e);
        }
    }

    /**
     * Handle an undeploy command.
     * Payload: {"namespace":"alice","systemName":"monitor"}
     */
    private void handleUndeploy(String body, String replyTo) throws Exception {
        UndeployCommand cmd = mapper.readValue(body, UndeployCommand.class);
        log.info("Received undeploy command for {}/{}", cmd.namespace(), cmd.systemName());
        try {
            deployService.undeploy(cmd.namespace(), cmd.systemName());
            publishStatus(replyTo, true, cmd.systemName(), cmd.namespace(), null);
            log.info("Undeploy succeeded for {}/{}", cmd.namespace(), cmd.systemName());
        } catch (Exception e) {
            publishStatus(replyTo, false, cmd.systemName(), cmd.namespace(), e.getMessage());
            log.error("Undeploy failed for {}/{}", cmd.namespace(), cmd.systemName(), e);
        }
    }

    /**
     * Publish a status response on the reply-to subject (or the default
     * status subject if no reply-to was provided).
     */
    private void publishStatus(String replyTo, boolean success,
                                String systemName, String namespace, String error) {
        publishDeployStatus(replyTo, success, systemName, namespace, error, List.of());
    }

    /**
     * Publish a deploy status response including node info.
     */
    private void publishDeployStatus(String replyTo, boolean success,
                                      String systemName, String namespace,
                                      String error, List<NodeInfo> nodes) {
        try {
            StatusResponse resp = new StatusResponse(
                    success, systemName, namespace, error, nodes);
            byte[] data = mapper.writeValueAsBytes(resp);
            String subject = replyTo != null && !replyTo.isBlank()
                    ? replyTo
                    : DataPlaneIpc.statusSubject(runtimeId);
            natsConnectionManager.getConnection().publish(subject, data);
        } catch (Exception e) {
            log.error("Failed to publish status response", e);
        }
    }
    // --- IPC DTOs ---
    public record DeployCommand(String namespace, String systemName, String source) {}
    public record UndeployCommand(String namespace, String systemName) {}
    public record StatusResponse(boolean success, String systemName, String namespace,
                                  String error, List<NodeInfo> nodes) {
        public StatusResponse(boolean success, String systemName, String namespace, String error) {
            this(success, systemName, namespace, error, List.of());
        }
    }
    public record NodeInfo(
            String name, String uri, String category,
            String state, String health,
            List<String> listens, List<String> events
    ) {}
}
