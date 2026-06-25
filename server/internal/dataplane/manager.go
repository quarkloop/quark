// Package dataplane manages the lifecycle of data-plane JVM processes.
//
// The control plane delegates system execution to data-plane processes
// (one shared process for non-isolated namespaces + one per isolated
// namespace). This package is responsible for spawning them,
// health-checking them, and shutting them down.
//
// The runtime binary lives at runtime/quark-runtime/target/ and is
// either a JVM JAR (default) or a native executable. The
// ProcessManager.findBinary function searches the same 6 candidate
// paths the Java ProcessManager does.
package dataplane

import (
        "context"
        "errors"
        "fmt"
        "os"
        "os/exec"
        "path/filepath"
        "strconv"
        "sync"
        "syscall"
        "time"

        "go.uber.org/zap"
)

// ProcessManager spawns and tracks data-plane processes.
//
// runtimeId → *DataPlaneProcess map. The "shared" runtimeId hosts
// non-isolated namespaces; "ns-<namespace>" hosts isolated ones.
//
// Port assignment: a counter from 9100 increments for each new process.
// Same convention as the Java ProcessManager.
type ProcessManager struct {
        log       *zap.Logger
        stateRoot string
        natsURL   string
        javaExe   string
        binary    string // optional override; if "", findBinary() is used
        portBase  int

        mu        sync.Mutex
        processes map[string]*DataPlaneProcess
        nextPort  int
}

// NewProcessManager constructs a ProcessManager.
//
//   - stateRoot: filesystem path used for $STATE_ROOT/dataplane-logs/
//   - natsURL: passed to the data plane via -Dquark.nats.url=...
//   - binaryOverride: optional explicit path to the runtime JAR/native
//     binary. If empty, findBinary() is used.
//   - portBase: starting port for data-plane HTTP servers (default 9100).
func NewProcessManager(log *zap.Logger, stateRoot, natsURL, javaExe, binaryOverride string, portBase int) *ProcessManager {
        return &ProcessManager{
                log:        log,
                stateRoot:  stateRoot,
                natsURL:    natsURL,
                javaExe:    javaExe,
                binary:     binaryOverride,
                portBase:   portBase,
                processes:  make(map[string]*DataPlaneProcess),
                nextPort:   portBase,
        }
}

// EnsureProcess returns the running data-plane process for runtimeId,
// spawning a new one if none exists (or if the existing one died).
//
// Spawns the process, then polls its /q/health/live HTTP endpoint for
// up to 30 seconds. Returns an error if the process fails to become
// ready.
func (pm *ProcessManager) EnsureProcess(ctx context.Context, runtimeId string) (*DataPlaneProcess, error) {
        pm.mu.Lock()
        defer pm.mu.Unlock()

        if existing, ok := pm.processes[runtimeId]; ok && existing.IsAlive() {
                return existing, nil
        }
        if existing, ok := pm.processes[runtimeId]; ok && !existing.IsAlive() {
                pm.log.Info("data-plane process is dead — spawning replacement", zap.String("runtimeId", runtimeId))
                delete(pm.processes, runtimeId)
        }

        binary := pm.binary
        if binary == "" {
                b, err := findBinary(pm.stateRoot)
                if err != nil {
                        return nil, fmt.Errorf("find data-plane binary: %w", err)
                }
                binary = b
        }

        port := pm.nextPort
        pm.nextPort++

        proc, err := startProcess(pm.log, runtimeId, binary, pm.stateRoot, pm.natsURL, pm.javaExe, port)
        if err != nil {
                return nil, fmt.Errorf("start data-plane process %s: %w", runtimeId, err)
        }

        if !proc.waitForReady(30 * time.Second) {
                proc.Stop()
                return nil, fmt.Errorf("data-plane process %s did not become ready in 30s", runtimeId)
        }

        pm.processes[runtimeId] = proc
        pm.log.Info("data-plane process ready",
                zap.String("runtimeId", runtimeId),
                zap.Int("pid", proc.PID()),
                zap.Int("port", port))
        return proc, nil
}

// StopProcess stops a specific data-plane process by runtimeId.
// Used when an isolated namespace's last system is undeployed.
func (pm *ProcessManager) StopProcess(runtimeId string) {
        pm.mu.Lock()
        defer pm.mu.Unlock()
        if proc, ok := pm.processes[runtimeId]; ok {
                proc.Stop()
                delete(pm.processes, runtimeId)
        }
}

// IsProcessAlive reports whether a process exists AND is alive for the
// given runtimeId.
func (pm *ProcessManager) IsProcessAlive(runtimeId string) bool {
        pm.mu.Lock()
        defer pm.mu.Unlock()
        proc, ok := pm.processes[runtimeId]
        return ok && proc.IsAlive()
}

// StopAll stops every running data-plane process. Called on graceful
// shutdown of the control plane.
func (pm *ProcessManager) StopAll() {
        pm.mu.Lock()
        defer pm.mu.Unlock()
        for id, proc := range pm.processes {
                pm.log.Info("stopping data-plane process", zap.String("runtimeId", id))
                proc.Stop()
        }
        pm.processes = make(map[string]*DataPlaneProcess)
}

// findBinary locates the data-plane binary by searching the same 6
// candidate paths the Java ProcessManager does (see Pitfall 8 in
// AGENTS.md). First match wins:
//  1. Native binary at runtime/quark-runtime/target/quark-runtime-runner-runner
//  2. Same path, relative to stateRoot's parent
//  3. JVM JAR at runtime/quark-runtime/target/quark-runtime-runner-runner.jar
//  4. Same JAR path, relative to stateRoot's parent
//  5. Legacy quark-server native binary (kept for backwards compat)
//  6. Legacy quark-server JAR
func findBinary(stateRoot string) (string, error) {
        stateAbs, _ := filepath.Abs(stateRoot)
        parent := filepath.Dir(stateAbs)

        nativeMaven := filepath.Join("runtime", "quark-runtime", "target", "quark-runtime-runner-runner")
        if isExecutable(nativeMaven) {
                return nativeMaven, nil
        }
        nativeState := filepath.Join(parent, "runtime", "quark-runtime", "target", "quark-runtime-runner-runner")
        if isExecutable(nativeState) {
                return nativeState, nil
        }

        jarMaven := filepath.Join("runtime", "quark-runtime", "target", "quark-runtime-runner-runner.jar")
        if fileExists(jarMaven) {
                return jarMaven, nil
        }
        jarState := filepath.Join(parent, "runtime", "quark-runtime", "target", "quark-runtime-runner-runner.jar")
        if fileExists(jarState) {
                return jarState, nil
        }

        legacyNative := filepath.Join("server", "quark-server", "target", "quark-server-0.1.0-SNAPSHOT-runner")
        if isExecutable(legacyNative) {
                return legacyNative, nil
        }
        legacyJar := filepath.Join("server", "quark-server", "target", "quark-server-0.1.0-SNAPSHOT-runner.jar")
        if fileExists(legacyJar) {
                return legacyJar, nil
        }

        return "", errors.New("no data-plane binary found (searched runtime/quark-runtime/target/ and server/quark-server/target/)")
}

// fileExists / isExecutable are tiny helpers — kept inline to avoid
// pulling in additional imports for one-shot os.Stat calls.
func fileExists(p string) bool {
        st, err := os.Stat(p)
        return err == nil && !st.IsDir()
}

func isExecutable(p string) bool {
        st, err := os.Stat(p)
        if err != nil || st.IsDir() {
                return false
        }
        // On POSIX, check any executable bit. On Windows, presence + .exe
        // suffix is good enough (this code path is rarely hit on Windows).
        return st.Mode()&0o111 != 0
}

// resolveJavaExe returns the path to the java executable. Falls back
// to $JAVA_HOME/bin/java, then "java" on PATH. Same convention as the
// Java ProcessManager.
//
// The override parameter is intended for tests / explicit config; in
// production it's typically empty (and we read JAVA_HOME from the env).
func resolveJavaExe(override string) string {
        if override != "" {
                // If the override is a directory (e.g. JAVA_HOME), append /bin/java
                if st, err := os.Stat(override); err == nil && st.IsDir() {
                        p := filepath.Join(override, "bin", "java")
                        if isExecutable(p) {
                                return p
                        }
                }
                return override
        }
        if javaHome := os.Getenv("JAVA_HOME"); javaHome != "" {
                p := filepath.Join(javaHome, "bin", "java")
                if isExecutable(p) {
                        return p
                }
        }
        return "java"
}

// startProcess spawns a single data-plane process. The process's
// stdout+stderr are redirected to $STATE_ROOT/dataplane-logs/dataplane-<runtimeId>.log.
//
// JVM mode (binary ends with .jar):
//
//      java -Dquark.mode=data -Dquark.dataplane.runtimeId=<id> \
//           -Dquark.state.root=<stateRoot> -Dquark.nats.url=<url> \
//           -Dquarkus.http.port=<port> -Dquarkus.http.host=127.0.0.1 \
//           -jar <binary>
//
// Native mode (binary is an executable):
//
//      <binary> -Dquark.mode=data -Dquark.dataplane.runtimeId=<id> ...
//
// Same command-line as the Java ProcessManager.
func startProcess(log *zap.Logger, runtimeId, binary, stateRoot, natsURL, javaExeOverride string, httpPort int) (*DataPlaneProcess, error) {
        isJar := filepath.Ext(binary) == ".jar"

        args := make([]string, 0, 12)
        if isJar {
                args = append(args, resolveJavaExe(javaExeOverride))
        }
        args = append(args,
                "-Dquark.mode=data",
                "-Dquark.dataplane.runtimeId="+runtimeId,
                "-Dquark.state.root="+stateRoot,
                "-Dquark.nats.url="+natsURL,
                "-Dquarkus.http.port="+strconv.Itoa(httpPort),
                "-Dquarkus.http.host=127.0.0.1",
                "-Dquarkus.swagger-ui.always-include=false",
        )
        if isJar {
                args = append(args, "-jar", binary)
        } else {
                // Native binary: prepend the binary path itself
                args = append([]string{binary}, args...)
                args = append(args, "-Dquark.native=true")
        }

        logDir := filepath.Join(stateRoot, "dataplane-logs")
        if err := os.MkdirAll(logDir, 0o755); err != nil {
                return nil, fmt.Errorf("create log dir: %w", err)
        }
        logFile := filepath.Join(logDir, "dataplane-"+runtimeId+".log")

        logf, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
        if err != nil {
                return nil, fmt.Errorf("open log file %s: %w", logFile, err)
        }

        log.Info("starting data-plane process",
                zap.String("runtimeId", runtimeId),
                zap.String("mode", modeLabel(isJar)),
                zap.Int("port", httpPort),
                zap.String("log", logFile))

        cmd := exec.Command(args[0], args[1:]...)
        cmd.Stdout = logf
        cmd.Stderr = logf
        cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true} // detach from control-plane process group
        if err := cmd.Start(); err != nil {
                logf.Close()
                return nil, fmt.Errorf("spawn: %w", err)
        }

        return &DataPlaneProcess{
                runtimeId: runtimeId,
                cmd:       cmd,
                logFile:   logf,
                httpPort:  httpPort,
                log:       log,
        }, nil
}

// modeLabel returns "jvm" or "native" for logging.
func modeLabel(isJar bool) string {
        if isJar {
                return "jvm"
        }
        return "native"
}
