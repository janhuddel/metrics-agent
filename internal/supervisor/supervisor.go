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
	"github.com/janhuddel/metrics-agent/internal/modules/demo"
)

// VendorSpec beschreibt ein Modul, das überwacht werden soll.
type VendorSpec struct {
	Name      string
	Args      []string // Platzhalter für spätere Parameter
	InProcess bool     // Wenn true, wird das Modul in-process gestartet
}

// procState hält den Zustand eines gestarteten Moduls.
type procState struct {
	spec         VendorSpec
	cmd          *exec.Cmd
	stdout       io.ReadCloser
	stderr       io.ReadCloser
	backoff      time.Duration
	startedAt    time.Time
	restarts     int
	stopping     bool // finaler Stopp
	restarting   bool // Neustart (SIGHUP)
	stdoutPrefix string
	stderrPrefix string
	// In-process worker state
	inProcessCtx    context.Context
	inProcessCancel context.CancelFunc
	inProcessDone   chan struct{}
}

// Supervisor verwaltet mehrere Module (Subprozesse).
// Er startet sie, überwacht sie und startet sie ggf. neu.
type Supervisor struct {
	mu     sync.Mutex
	procs  map[string]*procState
	events chan string // Events nach außen
}

func New() *Supervisor {
	return &Supervisor{
		procs:  make(map[string]*procState),
		events: make(chan string, 64),
	}
}

func (s *Supervisor) Events() <-chan string { return s.events }

// Start startet ein Modul (als Subprozess).
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

	// runLoop startet den Prozess und respawnt bei Abstürzen.
	go s.runLoop(ctx, ps)
	return nil
}

// RestartAll beendet und startet alle Module neu.
func (s *Supervisor) RestartAll(ctx context.Context) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, ps := range s.procs {
		go s.stop(ctx, ps, true) // restart=true
	}
}

// StopAll beendet alle Module.
func (s *Supervisor) StopAll(ctx context.Context) {
	var wg sync.WaitGroup
	s.mu.Lock()
	for _, ps := range s.procs {
		wg.Add(1)
		go func(ps *procState) {
			defer wg.Done()
			s.stop(ctx, ps, false) // restart=false
		}(ps)
	}
	s.mu.Unlock()
	wg.Wait()
}

// runLoop startet ein Modul, überwacht es und startet es nach Absturz neu.
func (s *Supervisor) runLoop(ctx context.Context, ps *procState) {
	for {
		if err := s.spawn(ctx, ps); err != nil {
			s.events <- fmt.Sprintf("%s spawn error: %v", ps.spec.Name, err)
		}

		var err error
		var uptime time.Duration

		if ps.spec.InProcess {
			// In-process worker: warten auf done channel
			select {
			case <-ps.inProcessDone:
				uptime = time.Since(ps.startedAt)
				// In-process worker ist beendet
			case <-ctx.Done():
				return
			}
		} else {
			// Subprocess: warten auf cmd.Wait()
			err = ps.cmd.Wait()
			uptime = time.Since(ps.startedAt)
		}

		if ps.stopping {
			// endgültiger Stopp
			s.events <- fmt.Sprintf("%s stopped", ps.spec.Name)
			return
		}

		if ps.restarting {
			// Restart: Flag zurücksetzen, dann sofort neu starten (kein Backoff)
			ps.restarting = false
			s.events <- fmt.Sprintf("%s restarting (uptime=%s)", ps.spec.Name, uptime)
			continue
		}

		// regulärer Exit (z. B. Crash)
		if err != nil {
			s.events <- fmt.Sprintf("%s exited: %v (uptime=%s)", ps.spec.Name, err, uptime)
		} else {
			s.events <- fmt.Sprintf("%s exited normally (uptime=%s)", ps.spec.Name, uptime)
		}

		// Backoff-Strategie
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

// spawn startet das eigentliche Modul als Subprozess oder in-process.
func (s *Supervisor) spawn(ctx context.Context, ps *procState) error {
	if ps.spec.InProcess {
		return s.spawnInProcess(ctx, ps)
	}
	return s.spawnSubprocess(ctx, ps)
}

// spawnSubprocess startet das Modul als Subprozess.
func (s *Supervisor) spawnSubprocess(ctx context.Context, ps *procState) error {
	// Wir starten das gleiche Binary mit -worker und -module.
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

	// Logs des Subprozesses → STDERR mit Prefix.
	go prefixCopy(ps.stderrPrefix, os.Stderr, stderr)
	// Metriken (STDOUT) → transparent durchreichen.
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

// spawnInProcess startet das Modul in-process.
func (s *Supervisor) spawnInProcess(ctx context.Context, ps *procState) error {
	// Context für das in-process Modul
	ps.inProcessCtx, ps.inProcessCancel = context.WithCancel(ctx)
	ps.inProcessDone = make(chan struct{})

	// Channel für Metriken
	metricCh := make(chan metrics.Metric, 100)

	// Metriken-Serializer
	go func() {
		for m := range metricCh {
			line, err := m.ToLineProtocol()
			if err != nil {
				log.Printf("%sserialization error: %v", ps.stderrPrefix, err)
				continue
			}
			fmt.Println(line) // direkt STDOUT
		}
	}()

	// Modul in separater Goroutine starten
	go func() {
		defer close(ps.inProcessDone)
		defer close(metricCh)

		// Modul-Dispatch
		switch ps.spec.Name {
		case "demo":
			if err := demo.Run(ps.inProcessCtx, metricCh); err != nil {
				log.Printf("%smodule error: %v", ps.stderrPrefix, err)
			}
		default:
			log.Printf("%sunknown module: %s", ps.stderrPrefix, ps.spec.Name)
		}
	}()

	ps.startedAt = time.Now()
	ps.stopping = false
	ps.restarts = 0
	return nil
}

// stop beendet ein Modul sanft (erst Interrupt, dann notfalls Kill).
func (s *Supervisor) stop(ctx context.Context, ps *procState, restart bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if ps == nil {
		return
	}

	if restart {
		ps.restarting = true
	} else {
		ps.stopping = true
	}

	if ps.spec.InProcess {
		// In-process worker: cancel context
		if ps.inProcessCancel != nil {
			ps.inProcessCancel()
		}

		// Warten auf Beendigung
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
			// Timeout - in-process worker sollte sich beenden
		}
	} else {
		// Subprocess: Signal senden
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
		}
	}
}

// prefixCopy liest Zeilen aus src und schreibt sie mit Prefix nach dst.
// Wird für STDERR der Module genutzt.
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

// forwardLines leitet Zeilen aus src direkt an dst weiter.
// Wird für STDOUT der Module genutzt (Metriken).
func forwardLines(dst io.Writer, src io.Reader) {
	sc := bufio.NewScanner(src)
	bw := bufio.NewWriter(dst)
	for sc.Scan() {
		fmt.Fprintln(bw, sc.Text())
		bw.Flush()
	}
	_ = bw.Flush()
}

func isBrokenPipe(err error) bool {
	return err != nil && strings.Contains(strings.ToLower(err.Error()), "broken pipe")
}
