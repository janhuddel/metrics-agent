package supervisor_test

import (
	"context"
	"os/exec"
	"testing"
	"time"

	"github.com/janhuddel/metrics-agent/internal/supervisor"
)

func TestSupervisor_StartAndStopDemoWorker(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sup := supervisor.New()

	if err := sup.Start(ctx, supervisor.VendorSpec{Name: "demo"}); err != nil {
		t.Fatalf("failed to start demo worker: %v", err)
	}

	// Warten auf ein Event (Start / Exit)
	select {
	case ev := <-sup.Events():
		t.Logf("got event: %s", ev)
	case <-time.After(5 * time.Second):
		t.Fatal("no event from supervisor within 5s")
	}

	// Stoppen
	stopCtx, stopCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer stopCancel()
	sup.StopAll(stopCtx)
}

func TestSupervisor_UnknownModuleFails(t *testing.T) {
	// Wir simulieren den Aufruf eines unbekannten Moduls, indem wir direkt spawn aufrufen
	cmd := exec.Command("go", "run", "./cmd/metric-agent", "-worker", "-module", "unknown")
	if err := cmd.Run(); err == nil {
		t.Fatal("expected error for unknown module")
	}
}
