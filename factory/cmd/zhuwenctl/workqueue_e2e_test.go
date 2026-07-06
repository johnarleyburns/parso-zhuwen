package main

// MC-3 kill-9 e2e: drive the resumable work queue through the real zhuwenctl binary, SIGKILL it
// mid-stage, resume, and assert the final result set is identical to a clean run with no
// double-charged gen calls. This back-fills CP-01's original resumability acceptance.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/parso/zhuwen-factory/internal/workq"
)

func runEnv(t *testing.T, bin string, env []string, args ...string) (string, error) {
	t.Helper()
	cmd := exec.Command(bin, args...)
	cmd.Env = append(os.Environ(), env...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func TestE2E_WorkQueueResumesAfterKillWithoutDoubleCharge(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e in -short")
	}
	bin := buildBinary(t)

	// --- Reference: a clean, uninterrupted run. ---
	cleanDB := filepath.Join(t.TempDir(), "clean.db")
	if out, err := runEnv(t, bin, nil, "run", "--db", cleanDB); err != nil {
		t.Fatalf("clean run: %v\n%s", err, out)
	}
	cq, err := workq.Open(cleanDB)
	if err != nil {
		t.Fatal(err)
	}
	defer cq.Close()
	want, err := cq.AllResults()
	if err != nil {
		t.Fatal(err)
	}
	if len(want) == 0 {
		t.Fatal("clean run produced no results")
	}
	cleanCharges, _ := cq.ChargeCount()
	if cleanCharges != len(want) {
		t.Fatalf("clean run charged %d for %d units", cleanCharges, len(want))
	}

	// --- Interrupted run: crash mid-stage, then resume. ---
	crashDB := filepath.Join(t.TempDir(), "crash.db")

	// First attempt crashes after committing 3 units (SIGKILL-equivalent os.Exit(137) in the
	// danger window, after the paid call, before commit).
	out, err := runEnv(t, bin, []string{"ZHUWEN_CRASH_AFTER=3"}, "run", "--db", crashDB)
	if err == nil {
		t.Fatalf("expected non-zero exit from simulated crash, out=%s", out)
	}
	if !strings.Contains(out, "simulated crash") {
		t.Fatalf("expected crash message, got: %s", out)
	}

	// Resume: must complete the rest without recomputing the cached units or double-charging.
	if out, err := runEnv(t, bin, nil, "run", "--db", crashDB, "--resume"); err != nil {
		t.Fatalf("resume run: %v\n%s", err, out)
	}

	rq, err := workq.Open(crashDB)
	if err != nil {
		t.Fatal(err)
	}
	defer rq.Close()

	got, err := rq.AllResults()
	if err != nil {
		t.Fatal(err)
	}
	// Identical final output to the clean run.
	if len(got) != len(want) {
		t.Fatalf("resumed run has %d results, clean had %d", len(got), len(want))
	}
	for ref, payload := range want {
		if got[ref] != payload {
			t.Errorf("ref %s differs after resume", ref)
		}
	}
	// No double charge: exactly one charge per unit despite the crash + retry.
	charges, _ := rq.ChargeCount()
	if charges != len(want) {
		t.Fatalf("resumed run charged %d times for %d units — double charge!", charges, len(want))
	}

	// The stage is fully drained (no pending/running/failed left).
	counts, _ := rq.Counts("gen")
	if counts[workq.Done] != len(want) || counts[workq.Pending] != 0 || counts[workq.Running] != 0 {
		t.Fatalf("stage not cleanly drained: %v", counts)
	}
}
