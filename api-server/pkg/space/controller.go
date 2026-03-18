package space

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/quarkloop/agent/pkg/infra/idgen"
)

// restartPolicy mirrors the Quarkfile restart field.
type restartPolicy string

const (
	policyOnFailure restartPolicy = "on-failure"
	policyAlways    restartPolicy = "always"
	policyNever     restartPolicy = "never"

	maxRestarts     = 5
	restartCooldown = 10 * time.Second
	reconcileEvery  = 15 * time.Second
)

// processRecord tracks a live agent supervisor OS process.
type processRecord struct {
	spaceID      string
	cmd          *exec.Cmd
	restartCount int
	lastRestart  time.Time
	policy       restartPolicy
	// logBuf buffers recent stdout/stderr lines for `quark logs` streaming.
	logBuf *ringBuf
}

// Controller supervises agent supervisor OS processes.
//
// One Controller exists per api-server and drives the full process lifecycle:
//  1. Launch — allocate port, write StatusStarting, exec agent supervisor.
//  2. startProcess — goroutine: wait for exit, persist LastLogs + final status.
//  3. reconcile — 15 s ticker: call maybeRestart for failed spaces.
//  4. Stop/Kill — send SIGINT or SIGKILL via live record or PID fallback.
//
// Port allocation is serialised by mu. Ports reserved in previous runs are
// re-hydrated from the store on startup to avoid collisions.
// Controller launches, monitors, and restarts agent supervisor OS processes.
// One Controller is created per api-server instance. It owns port allocation
// and an in-memory process registry. All persisted state is written through
// the Store interface so it survives api-server restarts.
type Controller struct {
	store      Store
	runtimeBin string
	apiAddr    string
	portStart  int
	portEnd    int

	mu        sync.Mutex
	usedPorts map[int]bool
	processes map[string]*processRecord // spaceID → live process
}

// NewController creates a Controller. runtimeBin is the path to the
// agent supervisor binary. portStart/portEnd are the inclusive port range
// (e.g. 7100–7999) used to assign a unique port to each running space.
// NewController creates a Controller backed by store, launching runtimeBin for
// each new space. Ports in [portStart, portEnd] are allocated sequentially.
// Ports for non-stopped spaces that are already listening are reserved
// immediately so a restarted api-server does not double-assign them.
// apiAddr is the base URL of this api-server (e.g. "http://127.0.0.1:7070"),
// forwarded to each agent supervisor so it can POST health reports back.
func NewController(store Store, runtimeBin, apiAddr string, portStart, portEnd int) *Controller {
	if apiAddr == "" {
		apiAddr = "http://127.0.0.1:7070"
	}
	c := &Controller{
		store:      store,
		runtimeBin: runtimeBin,
		apiAddr:    apiAddr,
		portStart:  portStart,
		portEnd:    portEnd,
		usedPorts:  map[int]bool{},
		processes:  map[string]*processRecord{},
	}
	c.hydratePortsFromStore()
	return c
}

// hydratePortsFromStore marks ports for all non-stopped spaces as in-use.
func (c *Controller) hydratePortsFromStore() {
	spaces, err := c.store.List()
	if err != nil {
		log.Printf("controller: port hydration failed: %v", err)
		return
	}
	count := 0
	for _, sp := range spaces {
		if sp.Port > 0 && sp.Status != StatusStopped && sp.Status != StatusFailed {
			if portIsListening(sp.Port) {
				c.usedPorts[sp.Port] = true
				count++
			} else {
				sp.Status = StatusFailed
				c.store.Save(sp)
			}
		}
	}
	if count > 0 {
		log.Printf("controller: reserved %d ports from previous run", count)
	}
}

// portIsListening returns true if something is already bound to 127.0.0.1:<port>.
func portIsListening(port int) bool {
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 200*time.Millisecond)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// Run starts the reconciliation loop, blocking until ctx is cancelled.
// Launch this in a goroutine alongside the HTTP server.
func (c *Controller) Run(ctx context.Context) {
	tick := time.NewTicker(reconcileEvery)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			c.reconcile()
		}
	}
}

// reconcile enforces the restart policy for every failed space.
// It only acts on spaces that have no live process record — meaning the
// process already exited and startProcess already saved the final status.
func (c *Controller) reconcile() {
	spaces, err := c.store.List()
	if err != nil {
		log.Printf("controller reconcile: %v", err)
		return
	}
	for _, sp := range spaces {
		if sp.Status != StatusFailed {
			continue
		}
		policy := restartPolicy(sp.RestartPolicy)
		if policy == "" {
			policy = policyOnFailure
		}
		if policy == policyNever {
			continue
		}

		c.mu.Lock()
		rec, alive := c.processes[sp.ID]
		c.mu.Unlock()

		if !alive {
			c.maybeRestart(sp, rec)
		}
	}
}

// maybeRestart restarts a failed space respecting maxRestarts and cooldown.
// Restart counters are read from and written to the persisted store record so
// the limits survive api-server restarts.
func (c *Controller) maybeRestart(sp *Space, rec *processRecord) {
	// Prefer in-memory counter (more up-to-date within a single run) but fall
	// back to the persisted value after an api-server restart.
	restartCount := sp.RestartCount
	var lastRestart time.Time
	if rec != nil && rec.restartCount > restartCount {
		restartCount = rec.restartCount
	}
	if rec != nil {
		lastRestart = rec.lastRestart
	} else if sp.LastRestartAt != nil {
		lastRestart = *sp.LastRestartAt
	}

	if restartCount >= maxRestarts {
		log.Printf("controller: space %s exceeded max restarts (%d) — leaving failed", sp.ID, maxRestarts)
		return
	}
	if !lastRestart.IsZero() && time.Since(lastRestart) < restartCooldown {
		return
	}
	log.Printf("controller: restarting space %s (attempt %d/%d)", sp.ID, restartCount+1, maxRestarts)
	port, err := c.allocatePort()
	if err != nil {
		log.Printf("controller: restart %s: no port available: %v", sp.ID, err)
		return
	}

	now := time.Now()
	sp.Port = port
	sp.Status = StatusStarting
	sp.RestartCount = restartCount + 1
	sp.LastRestartAt = &now
	c.store.Save(sp)
	req := &RunRequest{Name: sp.Name, Dir: sp.Dir, Env: sp.LastEnv}
	go c.startProcess(sp, req, restartCount+1)
}

// Launch allocates a port, persists a Space record (StatusStarting), and
// starts the agent supervisor in a background goroutine.
// It returns the initial record immediately — poll GetSpace for the final status.
// Launch allocates a port, saves a StatusStarting Space record, and spawns
// startProcess in a goroutine. Returns immediately; callers should poll
// GetSpace or stream logs to observe StatusRunning.
func (c *Controller) Launch(req *RunRequest) (*Space, error) {
	port, err := c.allocatePort()
	if err != nil {
		return nil, err
	}
	sp := &Space{
		ID:            idgen.SpaceID(),
		Name:          req.Name,
		Dir:           req.Dir,
		Status:        StatusStarting,
		Port:          port,
		RestartPolicy: string(policyOnFailure),
		LastEnv:       req.Env,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}
	if req.RestartPolicy != "" {
		sp.RestartPolicy = req.RestartPolicy
	}
	if err := c.store.Save(sp); err != nil {
		c.releasePort(port)
		return nil, fmt.Errorf("saving space: %w", err)
	}
	go c.startProcess(sp, req, 0)
	return sp, nil
}

// Stop signals a space to shut down. It writes StatusStopping immediately so
// callers see an accurate status while the process is winding down.
// force=true delivers SIGKILL; false delivers SIGINT.
func (c *Controller) Stop(id string, force bool) error {
	sp, err := c.store.Get(id)
	if err != nil {
		return err
	}
	if sp.Status == StatusStopped || sp.Status == StatusFailed {
		return fmt.Errorf("space %s is already %s", id, sp.Status)
	}

	// Write stopping status immediately so ps/inspect reflect reality.
	sp.Status = StatusStopping
	c.store.Save(sp)

	c.mu.Lock()
	rec, ok := c.processes[id]
	c.mu.Unlock()

	if ok && rec.cmd != nil && rec.cmd.Process != nil {
		if force {
			return rec.cmd.Process.Kill()
		}
		return rec.cmd.Process.Signal(os.Interrupt)
	}

	// Fall back to PID from store.
	if sp.PID == 0 {
		return fmt.Errorf("space %s: no live process found", id)
	}
	proc, err := os.FindProcess(sp.PID)
	if err != nil {
		return err
	}
	if force {
		return proc.Kill()
	}
	return proc.Signal(os.Interrupt)
}

// GetLogBuf returns the live log ring buffer for a running space, or nil
// if the space has no active process record (already exited or not started).
func (c *Controller) GetLogBuf(id string) *ringBuf {
	c.mu.Lock()
	defer c.mu.Unlock()
	if rec, ok := c.processes[id]; ok {
		return rec.logBuf
	}
	return nil
}

// startProcess launches the agent binary in supervisor mode and waits for it to exit.
func (c *Controller) startProcess(sp *Space, req *RunRequest, restartCount int) {
	buf := newRingBuf(512)
	ipcSocket := ipcSocketPath(sp.ID)
	cmd := exec.Command(c.runtimeBin,
		"supervisor",
		"--id", sp.ID,
		"--dir", req.Dir,
		"--port", fmt.Sprintf("%d", sp.Port),
		"--api-server", c.apiAddr,
		"--ipc-socket", ipcSocket,
	)
	// Tee stdout/stderr to both the ring buffer and the parent process output.
	cmd.Stdout = newTeeWriter(os.Stdout, buf)
	cmd.Stderr = newTeeWriter(os.Stderr, buf)
	cmd.Env = os.Environ()
	for k, v := range req.Env {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
	}

	if err := cmd.Start(); err != nil {
		log.Printf("controller: launch space %s: %v", sp.ID, err)
		sp.Status = StatusFailed
		c.store.Save(sp)
		c.releasePort(sp.Port)
		return
	}

	sp.PID = cmd.Process.Pid
	sp.Status = StatusRunning
	c.store.Save(sp)

	c.mu.Lock()
	c.processes[sp.ID] = &processRecord{
		spaceID:      sp.ID,
		cmd:          cmd,
		restartCount: restartCount,
		lastRestart:  time.Now(),
		policy:       restartPolicy(sp.RestartPolicy),
		logBuf:       buf,
	}
	c.mu.Unlock()

	err := cmd.Wait()

	c.mu.Lock()
	delete(c.processes, sp.ID)
	c.mu.Unlock()
	c.releasePort(sp.Port)

	if err != nil {
		log.Printf("controller: space %s exited with error: %v", sp.ID, err)
		sp.Status = StatusFailed
	} else {
		sp.Status = StatusStopped
	}
	sp.PID = 0

	// Persist the tail of log output so "quark logs" works after process exit.
	if lines := buf.Lines(); len(lines) > 0 {
		tail := make([]string, len(lines))
		for i, l := range lines {
			tail[i] = l.t.Format("15:04:05") + " " + l.text
		}
		sp.LastLogs = tail
	}
	c.store.Save(sp)
}

func (c *Controller) allocatePort() (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for p := c.portStart; p <= c.portEnd; p++ {
		if !c.usedPorts[p] {
			c.usedPorts[p] = true
			return p, nil
		}
	}
	return 0, fmt.Errorf("no ports available in range %d–%d", c.portStart, c.portEnd)
}

func (c *Controller) releasePort(port int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.usedPorts, port)
}

// ipcSocketPath returns the Unix domain socket path for a supervisor's IPC server.
func ipcSocketPath(spaceID string) string {
	home, _ := os.UserHomeDir()
	return fmt.Sprintf("%s/.quark/agents/%s/ipc.sock", home, spaceID)
}
