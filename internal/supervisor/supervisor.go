// Package supervisor provides process management for metric collection modules.
// It can run modules either as subprocesses or in-process, with automatic
// restart capabilities and graceful shutdown handling.
package supervisor

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/janhuddel/metrics-agent/internal/metrics"
	"github.com/janhuddel/metrics-agent/internal/modules"
)

// VendorSpec describes a module that should be monitored and managed.
type VendorSpec struct {
	Name string   // Module name
	Args []string // Command line arguments (placeholder for future use)
}

// procState holds the state of a running module process.
type procState struct {
	spec         VendorSpec
	cmd          *exec.Cmd
	stdout       io.ReadCloser
	stderr       io.ReadCloser
	backoff      time.Duration
	startedAt    time.Time
	restarts     int
	stopping     bool // indicates final stop (no restart)
	restarting   bool // indicates restart requested (SIGHUP)
	stdoutPrefix string
	stderrPrefix string
	// In-process worker state
	inProcessCtx    context.Context
	inProcessCancel context.CancelFunc
	inProcessDone   chan struct{}
}

// Supervisor manages multiple modules (subprocesses or in-process workers).
// It starts them, monitors them, and restarts them if they crash.
type Supervisor struct {
	mu        sync.RWMutex
	procs     map[string]*procState
	events    chan string // Events sent to external consumers
	inProcess bool        // If true, all modules are started in-process
	ctx       context.Context
	cancel    context.CancelFunc
}

// New creates a new Supervisor instance.
// If inProcess is true, modules will be run in-process instead of as subprocesses.
func New(inProcess bool) *Supervisor {
	ctx, cancel := context.WithCancel(context.Background())
	return &Supervisor{
		procs:     make(map[string]*procState),
		events:    make(chan string, 128), // Increased buffer size
		inProcess: inProcess,
		ctx:       ctx,
		cancel:    cancel,
	}
}

// Events returns a read-only channel for receiving supervisor events.
func (s *Supervisor) Events() <-chan string { return s.events }

// Start starts a module (as subprocess or in-process).
// It returns an error if the module is already running.
func (s *Supervisor) Start(ctx context.Context, spec VendorSpec) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.procs[spec.Name]; exists {
		return fmt.Errorf("module %q already running", spec.Name)
	}

	ps := &procState{
		spec:         spec,
		backoff:      1 * time.Second,
		stdoutPrefix: fmt.Sprintf("[mod:%s][out] ", spec.Name),
		stderrPrefix: fmt.Sprintf("[mod:%s][err] ", spec.Name),
	}
	s.procs[spec.Name] = ps

	// Start the run loop which manages the process and handles restarts
	go s.runLoop(s.ctx, ps)
	return nil
}

// RestartAll stops and restarts all modules.
func (s *Supervisor) RestartAll(ctx context.Context) {
	s.mu.RLock()
	procs := make([]*procState, 0, len(s.procs))
	for _, ps := range s.procs {
		procs = append(procs, ps)
	}
	s.mu.RUnlock()

	for _, ps := range procs {
		go s.stop(ctx, ps, true) // restart=true
	}
}

// StopAll stops all modules gracefully.
func (s *Supervisor) StopAll(ctx context.Context) {
	var wg sync.WaitGroup
	s.mu.RLock()
	procs := make([]*procState, 0, len(s.procs))
	for _, ps := range s.procs {
		procs = append(procs, ps)
	}
	s.mu.RUnlock()

	for _, ps := range procs {
		wg.Add(1)
		go func(ps *procState) {
			defer wg.Done()
			s.stop(ctx, ps, false) // restart=false
		}(ps)
	}
	wg.Wait()

	// Cancel supervisor context to stop all runLoops
	s.cancel()
}

// runLoop starts a module, monitors it, and restarts it after crashes.
// It implements exponential backoff for restart attempts.
func (s *Supervisor) runLoop(ctx context.Context, ps *procState) {
	defer func() {
		if r := recover(); r != nil {
			s.sendEvent(fmt.Sprintf("%s runLoop panic recovered: %v", ps.spec.Name, r))
		}
		// Clean up process state
		s.mu.Lock()
		delete(s.procs, ps.spec.Name)
		s.mu.Unlock()
	}()

	for {
		if err := s.spawn(ctx, ps); err != nil {
			s.sendEvent(fmt.Sprintf("%s spawn error: %v", ps.spec.Name, err))
			// Implement retry with backoff for spawn failures
			select {
			case <-ctx.Done():
				return
			case <-time.After(ps.backoff):
				continue
			}
		}

		var err error
		var uptime time.Duration

		if s.inProcess {
			// In-process worker: wait for done channel
			select {
			case <-ps.inProcessDone:
				uptime = time.Since(ps.startedAt)
				// In-process worker has finished
			case <-ctx.Done():
				return
			}
		} else {
			// Subprocess: wait for cmd.Wait()
			err = ps.cmd.Wait()
			uptime = time.Since(ps.startedAt)
		}

		if ps.stopping {
			// Final stop requested
			s.sendEvent(fmt.Sprintf("%s stopped", ps.spec.Name))
			return
		}

		if ps.restarting {
			// Restart: reset flag and restart immediately (no backoff)
			ps.restarting = false
			s.sendEvent(fmt.Sprintf("%s restarting (uptime=%s)", ps.spec.Name, uptime))
			continue
		}

		// Regular exit (e.g., crash)
		if err != nil {
			s.sendEvent(fmt.Sprintf("%s exited: %v (uptime=%s)", ps.spec.Name, err, uptime))
		} else {
			s.sendEvent(fmt.Sprintf("%s exited normally (uptime=%s)", ps.spec.Name, uptime))
		}

		// Implement exponential backoff strategy
		if uptime > time.Minute {
			ps.backoff = 1 * time.Second
		} else {
			if ps.backoff < 30*time.Second {
				ps.backoff *= 2
				if ps.backoff > 30*time.Second {
					ps.backoff = 30 * time.Second
				}
			}
		}

		select {
		case <-ctx.Done():
			return
		case <-time.After(ps.backoff):
			ps.restarts++
		}
	}
}

// spawn starts the actual module as subprocess or in-process.
func (s *Supervisor) spawn(ctx context.Context, ps *procState) error {
	if s.inProcess {
		return s.spawnInProcess(ctx, ps)
	}
	return s.spawnSubprocess(ctx, ps)
}

// spawnSubprocess starts the module as a subprocess.
func (s *Supervisor) spawnSubprocess(ctx context.Context, ps *procState) error {
	// Start the same binary with -worker and -module flags
	args := []string{"-worker", "-module", ps.spec.Name}
	cmd := exec.Command(os.Args[0], args...)
	cmd.Env = os.Environ()

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}

	// Forward subprocess logs to stderr with prefix
	go prefixCopy(ps.stderrPrefix, os.Stderr, stderr)
	// Forward metrics (stdout) transparently
	go forwardLines(os.Stdout, stdout)

	if err := cmd.Start(); err != nil {
		return err
	}
	ps.cmd = cmd
	ps.stdout = stdout
	ps.stderr = stderr
	ps.startedAt = time.Now()
	ps.stopping = false
	ps.restarts = 0
	return nil
}

// spawnInProcess starts the module in-process.
func (s *Supervisor) spawnInProcess(ctx context.Context, ps *procState) error {
	// Create context for the in-process module
	ps.inProcessCtx, ps.inProcessCancel = context.WithCancel(ctx)
	ps.inProcessDone = make(chan struct{})

	// Create channel for metrics
	metricCh := make(chan metrics.Metric, 100)

	// Start metric serializer goroutine with panic recovery
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("%smetric serializer panic recovered: %v", ps.stderrPrefix, r)
			}
		}()

		for m := range metricCh {
			line, err := m.ToLineProtocolSafe()
			if err != nil {
				log.Printf("%sserialization error: %v", ps.stderrPrefix, err)
				continue
			}
			fmt.Println(line) // Write directly to stdout
		}
	}()

	// Start module in separate goroutine with panic recovery
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("%smodule panic recovered: %v", ps.stderrPrefix, r)
			}
			close(ps.inProcessDone)
			close(metricCh)
		}()

		// Dispatch module through registry
		if err := modules.Global.Run(ps.inProcessCtx, ps.spec.Name, metricCh); err != nil {
			log.Printf("%smodule error: %v", ps.stderrPrefix, err)
		}
	}()

	ps.startedAt = time.Now()
	ps.stopping = false
	ps.restarts = 0
	return nil
}

// stop gracefully stops a module (first interrupt, then kill if necessary).
func (s *Supervisor) stop(ctx context.Context, ps *procState, restart bool) {
	if ps == nil {
		return
	}

	// Set flags atomically
	s.mu.Lock()
	if restart {
		ps.restarting = true
	} else {
		ps.stopping = true
	}
	s.mu.Unlock()

	if s.inProcess {
		// In-process worker: cancel context
		if ps.inProcessCancel != nil {
			ps.inProcessCancel()
		}

		// Wait for termination
		done := make(chan struct{})
		go func() {
			if ps.inProcessDone != nil {
				<-ps.inProcessDone
			}
			close(done)
		}()

		select {
		case <-ctx.Done():
		case <-done:
		case <-time.After(5 * time.Second):
			// Timeout - in-process worker should terminate
			s.sendEvent(fmt.Sprintf("%s stop timeout", ps.spec.Name))
		}
	} else {
		// Subprocess: send signal
		if ps.cmd == nil || ps.cmd.Process == nil {
			return
		}

		_ = ps.cmd.Process.Signal(os.Interrupt)

		done := make(chan struct{})
		go func() {
			ps.cmd.Wait()
			close(done)
		}()

		select {
		case <-ctx.Done():
		case <-done:
		case <-time.After(5 * time.Second):
			_ = ps.cmd.Process.Kill()
			s.sendEvent(fmt.Sprintf("%s force killed", ps.spec.Name))
		}
	}
}

// prefixCopy reads lines from src and writes them with prefix to dst.
// Used for stderr output from modules.
func prefixCopy(prefix string, dst io.Writer, src io.Reader) {
	sc := bufio.NewScanner(src)
	for sc.Scan() {
		line := sc.Text()
		fmt.Fprintln(dst, prefix+line)
	}
	if err := sc.Err(); err != nil && !isBrokenPipe(err) {
		log.Printf("[supervisor] stream error: %v", err)
	}
}

// forwardLines forwards lines from src directly to dst.
// Used for stdout from modules (metrics).
func forwardLines(dst io.Writer, src io.Reader) {
	sc := bufio.NewScanner(src)
	bw := bufio.NewWriter(dst)
	for sc.Scan() {
		fmt.Fprintln(bw, sc.Text())
		bw.Flush()
	}
	_ = bw.Flush()
}

// sendEvent sends an event to the events channel in a non-blocking way.
// If the channel is full, it logs the event instead to prevent deadlocks.
func (s *Supervisor) sendEvent(event string) {
	select {
	case s.events <- event:
		// Event sent successfully
	default:
		// Channel is full, log instead to prevent blocking
		log.Printf("[supervisor] event (channel full): %s", event)
	}
}

// isBrokenPipe checks if an error is a broken pipe error.
func isBrokenPipe(err error) bool {
	return err != nil && strings.Contains(strings.ToLower(err.Error()), "broken pipe")
}
