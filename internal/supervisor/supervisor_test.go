package supervisor_test

import (
	"context"
	"os/exec"
	"testing"
	"time"

	"github.com/janhuddel/metrics-agent/internal/supervisor"
)

// TestSupervisor_StartAndStopDemoWorker tests starting and stopping a demo worker.
func TestSupervisor_StartAndStopDemoWorker(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sup := supervisor.New(false) // Use subprocess mode for testing

	if err := sup.Start(ctx, supervisor.VendorSpec{Name: "demo"}); err != nil {
		t.Fatalf("failed to start demo worker: %v", err)
	}

	// Wait for an event (Start / Exit)
	select {
	case ev := <-sup.Events():
		t.Logf("got event: %s", ev)
	case <-time.After(5 * time.Second):
		t.Fatal("no event from supervisor within 5s")
	}

	// Stop the supervisor
	stopCtx, stopCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer stopCancel()
	sup.StopAll(stopCtx)
}

// TestSupervisor_UnknownModuleFails tests that unknown modules fail to start.
func TestSupervisor_UnknownModuleFails(t *testing.T) {
	// Simulate calling an unknown module by directly running the worker
	cmd := exec.Command("go", "run", "./cmd/metrics-agent", "-worker", "-module", "unknown")
	if err := cmd.Run(); err == nil {
		t.Fatal("expected error for unknown module")
	}
}
