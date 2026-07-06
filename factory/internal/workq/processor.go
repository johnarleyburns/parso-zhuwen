package workq

import "fmt"

// StageFunc performs the (possibly paid, possibly network) work for one unit, returning the
// payload to cache and the token cost. It must be safe to *retry*: it charges via q.Charge with
// a stable idempotency key so a retry after a crash does not double-charge.
type StageFunc func(q *Queue, ref string) (payload string, tokens int, err error)

// Hook is called after a unit's StageFunc has run (and charged) but before its result is
// committed. A test injects a Hook that os.Exit's here to simulate a kill -9 in the danger
// window — between the paid call and the commit — proving the idempotency-key guard holds.
type Hook func(processed int, ref string)

// Process drains a stage's pending units. For each: if the result is already cached (a prior
// run computed it), it is completed without recomputation; otherwise StageFunc runs, the result
// is cached, and the unit is marked done. On StageFunc error the unit is failed/retried.
// Call ResetStale before Process on a resume to recover units left `running` by a crash.
func (q *Queue) Process(stage string, maxAttempts int, fn StageFunc, hook Hook) error {
	processed := 0
	for {
		u, err := q.Claim(stage)
		if err != nil {
			return err
		}
		if u == nil {
			return nil // stage drained
		}

		if _, _, ok, err := q.CachedResult(u.Ref); err != nil {
			return err
		} else if ok {
			// Already computed in a previous run — idempotent skip, no recompute/charge.
			if err := q.Complete(u.ID); err != nil {
				return err
			}
			continue
		}

		payload, tokens, err := fn(q, u.Ref)
		if err != nil {
			if ferr := q.Fail(u, err, maxAttempts); ferr != nil {
				return ferr
			}
			continue
		}

		// Danger window: the paid call has happened; we have not yet committed the result.
		if hook != nil {
			hook(processed, u.Ref)
		}

		if err := q.StoreResult(u.Ref, payload, tokens); err != nil {
			return err
		}
		if err := q.Complete(u.ID); err != nil {
			return err
		}
		processed++
	}
}

// Summary is a human-readable stage state line.
func (q *Queue) Summary(stage string) (string, error) {
	c, err := q.Counts(stage)
	if err != nil {
		return "", err
	}
	charges, err := q.ChargeCount()
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("stage %s: pending=%d running=%d done=%d failed=%d · charges=%d",
		stage, c[Pending], c[Running], c[Done], c[Failed], charges), nil
}
