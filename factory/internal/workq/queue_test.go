package workq

import (
	"fmt"
	"path/filepath"
	"testing"
)

func openTemp(t *testing.T) *Queue {
	t.Helper()
	q, err := Open(filepath.Join(t.TempDir(), "queue.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { q.Close() })
	return q
}

func TestEnqueueIdempotent(t *testing.T) {
	q := openTemp(t)
	for i := 0; i < 3; i++ {
		if err := q.Enqueue("gen", "c1"); err != nil {
			t.Fatal(err)
		}
	}
	u, err := q.Claim("gen")
	if err != nil || u == nil || u.Ref != "c1" {
		t.Fatalf("claim after enqueue: u=%v err=%v", u, err)
	}
	u2, err := q.Claim("gen")
	if err != nil || u2 != nil {
		t.Fatalf("second claim should be nil (only one unit), got %v", u2)
	}
}

func TestResetStaleRecoversCrashed(t *testing.T) {
	q := openTemp(t)
	q.Enqueue("gen", "c1")

	u, _ := q.Claim("gen")
	if u.State != Running {
		t.Fatalf("state = %s, want running", u.State)
	}
	// Simulate crash: the row stays `running`.
	n, err := q.ResetStale()
	if err != nil || n != 1 {
		t.Fatalf("resetStale = %d, err=%v, want 1", n, err)
	}
	u2, _ := q.Claim("gen")
	if u2 == nil || u2.State != Running {
		t.Fatalf("after reset, claim should grab the reset unit, got %v", u2)
	}
}

func TestCompleteAndCacheSkipRecomputation(t *testing.T) {
	q := openTemp(t)
	q.Enqueue("gen", "c1")

	u, _ := q.Claim("gen")
	if err := q.StoreResult(u.Ref, "payload1", 10); err != nil {
		t.Fatal(err)
	}
	if err := q.Complete(u.ID); err != nil {
		t.Fatal(err)
	}
	// Process would skip this (cached).
	payload, tokens, ok, err := q.CachedResult("c1")
	if !ok || err != nil || payload != "payload1" || tokens != 10 {
		t.Fatalf("cached=%v payload=%q tokens=%d err=%v", ok, payload, tokens, err)
	}
}

func TestChargeDedup(t *testing.T) {
	q := openTemp(t)
	ok1, _ := q.Charge("gen:c1:0")
	ok2, _ := q.Charge("gen:c1:0")
	if !ok1 || ok2 {
		t.Fatal("first charge should be new, second IDEMPOTENT-NO-OP")
	}
	ok3, _ := q.Charge("gen:c1:1")
	if !ok3 {
		t.Fatal("different key should be new")
	}
	count, _ := q.ChargeCount()
	if count != 2 {
		t.Fatalf("charges = %d, want 2", count)
	}
}

func TestFailRetryThenExhaust(t *testing.T) {
	q := openTemp(t)
	q.Enqueue("gen", "c1")
	u, _ := q.Claim("gen")
	// Fail with retries left.
	if err := q.Fail(u, fmt.Errorf("boom"), 3); err != nil {
		t.Fatal(err)
	}
	// Reclaim should return the reset unit.
	u2, _ := q.Claim("gen")
	if u2 == nil || u2.Attempts != 2 {
		t.Fatalf("retry claim: u=%v", u2)
	}
	// Exhaust retries (allow 2 max).
	if err := q.Fail(u2, fmt.Errorf("boom2"), 2); err != nil {
		t.Fatal(err)
	}
	c, _ := q.Counts("gen")
	if c[Failed] != 1 {
		t.Fatalf("failed count = %d, want 1 (counts=%v)", c[Failed], c)
	}
}

func TestProcessSkipsCachedUsesHook(t *testing.T) {
	q := openTemp(t)
	q.Enqueue("gen", "c1")
	q.StoreResult("c1", "cached", 0) // pre-cache
	q.Complete(1)

	calls := 0
	err := q.Process("gen", 1, func(q *Queue, ref string) (string, int, error) {
		calls++
		return "new", 1, nil
	}, nil)
	if err != nil || calls != 0 {
		t.Fatalf("process should skip cached unit; calls=%d err=%v", calls, err)
	}
}

func TestProcessRunsAndCaches(t *testing.T) {
	q := openTemp(t)
	q.Enqueue("gen", "c1")

	hookCalled := 0
	err := q.Process("gen", 3, func(q *Queue, ref string) (string, int, error) {
		q.Charge("gen:" + ref)
		return "result-" + ref, 5, nil
	}, func(_ int, _ string) { hookCalled++ })

	if err != nil {
		t.Fatal(err)
	}
	if hookCalled != 1 {
		t.Errorf("hook called %d times, want 1", hookCalled)
	}
	payload, _, ok, _ := q.CachedResult("c1")
	if !ok || payload != "result-c1" {
		t.Fatalf("cached = %q, want result-c1", payload)
	}
	charges, _ := q.ChargeCount()
	if charges != 1 {
		t.Fatalf("charges = %d, want 1", charges)
	}
}
