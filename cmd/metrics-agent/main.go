// Package main implements the metrics-agent application.
// It can run in two modes:
// 1. Supervisor mode: Manages and monitors multiple metric collection modules
// 2. Worker mode: Executes a specific metric collection module
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/janhuddel/metrics-agent/internal/metrics"
	"github.com/janhuddel/metrics-agent/internal/modules"
	"github.com/janhuddel/metrics-agent/internal/supervisor"
)

var (
	// flagWorker indicates whether the program runs as a worker subprocess (started by supervisor)
	flagWorker = flag.Bool("worker", false, "Run as worker subprocess")
	// flagModule specifies the module name to run in worker mode
	flagModule = flag.String("module", "", "Module name to run in worker mode")
	// flagVersion prints the version and exits
	flagVersion = flag.Bool("version", false, "Print version and exit")
	// flagInProcess starts modules in-process instead of as subprocesses (for debugging)
	flagInProcess = flag.Bool("inprocess", false, "Start workers in-process instead of as subprocesses")
)

// version can be overridden at build time with -ldflags
const version = "0.1.0"

// main is the entry point of the metrics-agent application.
// It initializes logging, parses command-line flags, and delegates to either
// supervisor or worker mode based on the flags.
func main() {
	// Configure logging to stderr since stdout is reserved for metrics (Line Protocol)
	log.SetOutput(os.Stderr)
	log.SetFlags(log.LstdFlags | log.Lmicroseconds | log.Lmsgprefix)
	log.SetPrefix("[metric-agent] ")

	flag.Parse()

	// Handle version flag
	if *flagVersion {
		fmt.Fprintf(os.Stderr, "metric-agent %s (%s %s)\n", version, runtime.GOOS, runtime.GOARCH)
		return
	}

	// Determine execution mode
	if *flagWorker {
		runWorker(*flagModule)
		return
	}

	// Default to supervisor mode
	runSupervisor()
}

// runSupervisor starts the main supervisor process that manages and monitors modules.
// It creates a supervisor instance, starts all registered modules, and handles
// system signals for graceful shutdown and restart operations.
func runSupervisor() {
	// Create module specifications based on registered modules
	var specs []supervisor.VendorSpec
	for _, moduleName := range modules.Global.List() {
		specs = append(specs, supervisor.VendorSpec{
			Name: moduleName,
		})
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sup := supervisor.New(*flagInProcess)

	// Set up signal handling: TERM/INT → Shutdown; HUP → Restart
	sigCh := make(chan os.Signal, 2)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT, syscall.SIGHUP)

	// Start all modules
	for _, spec := range specs {
		if err := sup.Start(ctx, spec); err != nil {
			log.Printf("[supervisor] failed to start module %q: %v", spec.Name, err)
		}
	}

	// Main event loop
	shuttingDown := false
	for !shuttingDown {
		select {
		case sig := <-sigCh:
			switch sig {
			case syscall.SIGHUP:
				log.Printf("[supervisor] received SIGHUP: restarting all modules...")
				sup.RestartAll(ctx)
			case syscall.SIGINT, syscall.SIGTERM:
				log.Printf("[supervisor] received %s: shutting down...", sig)
				shuttingDown = true
			}
		case ev := <-sup.Events():
			log.Printf("[supervisor] event: %s", ev)
		}
	}

	// Graceful shutdown with timeout
	shCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	sup.StopAll(shCtx)
	log.Printf("[supervisor] exit")
}

// runWorker starts a specific module directly.
// It is called by the supervisor process as a subprocess.
func runWorker(moduleName string) {
	if moduleName == "" {
		log.Fatalf("[worker] missing -module flag")
	}

	// Create buffered channel for metrics
	metricCh := make(chan metrics.Metric, 100)

	// Set up context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle shutdown signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		<-sigCh
		cancel()
	}()

	// Start metric serializer goroutine
	go func() {
		for m := range metricCh {
			line, err := m.ToLineProtocol()
			if err != nil {
				log.Printf("[worker] serialization error: %v", err)
				continue
			}
			fmt.Println(line) // Write directly to stdout
		}
	}()

	// Run the specified module
	if err := modules.Global.Run(ctx, moduleName, metricCh); err != nil {
		log.Fatalf("[worker] module %s error: %v", moduleName, err)
	}

	// Close channel when module finishes
	close(metricCh)
}
