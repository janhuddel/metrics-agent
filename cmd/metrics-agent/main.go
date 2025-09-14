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
	"github.com/janhuddel/metrics-agent/internal/modules/demo"
	"github.com/janhuddel/metrics-agent/internal/supervisor"
)

var (
	// Flag, ob das Programm als Worker-Prozess läuft (von Supervisor gestartet).
	flagWorker = flag.Bool("worker", false, "Run as worker subprocess")
	// Modulname, der im Worker ausgeführt werden soll.
	flagModule = flag.String("module", "", "Module name to run in worker mode")
	// Flag, um die Version anzuzeigen.
	flagVersion = flag.Bool("version", false, "Print version and exit")
	// Flag, um Module in-process zu starten (für Debugging).
	flagInProcess = flag.Bool("inprocess", false, "Start workers in-process instead of as subprocesses")
)

// version kann beim Build mit -ldflags überschrieben werden.
const version = "0.1.0"

func main() {
	// Lognachrichten gehen auf STDERR.
	// Hintergrund: STDOUT ist für Metriken reserviert (Line Protocol).
	log.SetOutput(os.Stderr)
	log.SetFlags(log.LstdFlags | log.Lmicroseconds | log.Lmsgprefix)
	log.SetPrefix("[metric-agent] ")

	flag.Parse()

	// Bei -version nur Version ausgeben und beenden.
	if *flagVersion {
		fmt.Fprintf(os.Stderr, "metric-agent %s (%s %s)\n", version, runtime.GOOS, runtime.GOARCH)
		return
	}

	// Wenn -worker gesetzt: Worker-Modus starten.
	if *flagWorker {
		runWorker(*flagModule)
		return
	}

	// Ansonsten Supervisor-Modus starten.
	runSupervisor()
}

// runSupervisor ist der Hauptprozess. Er startet und überwacht die Module.
func runSupervisor() {
	// Beispielkonfiguration: ein Modul
	specs := []supervisor.VendorSpec{
		{Name: "demo", InProcess: *flagInProcess},
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sup := supervisor.New()

	// Signale: TERM/INT → Shutdown; HUP → Restart
	sigCh := make(chan os.Signal, 2)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT, syscall.SIGHUP)

	// Module starten
	for _, s := range specs {
		if err := sup.Start(ctx, s); err != nil {
			log.Printf("[supervisor] start %q failed: %v", s.Name, err)
		}
	}

	// Kontroll-Flag für die Schleife
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

	// Shutdown-Block → wird garantiert nach Verlassen der Schleife ausgeführt
	shCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	sup.StopAll(shCtx)
	log.Printf("[supervisor] exit")
}

// runWorker startet ein bestimmtes Modul direkt.
// Er wird vom Supervisor-Prozess als Subprozess aufgerufen.
func runWorker(moduleName string) {
	if moduleName == "" {
		log.Fatalf("[worker] missing -module")
	}

	// Channel für Metriken (unbuffered oder leicht gepuffert)
	metricCh := make(chan metrics.Metric, 100)

	// Context für Shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		<-sigCh
		cancel()
	}()

	// Serializer: Metriken auf STDOUT schreiben
	go func() {
		for m := range metricCh {
			line, err := m.ToLineProtocol()
			if err != nil {
				log.Printf("[worker] serialization error: %v", err)
				continue
			}
			fmt.Println(line) // direkt STDOUT
		}
	}()

	// Modul-Dispatch
	switch moduleName {
	case "demo":
		if err := demo.Run(ctx, metricCh); err != nil {
			log.Fatalf("[worker] demo module error: %v", err)
		}
	default:
		log.Fatalf("[worker] unknown module: %s", moduleName)
	}

	// Kanal schließen, wenn Modul beendet ist
	close(metricCh)
}
