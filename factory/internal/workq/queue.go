// Package workq is the SQLite-backed, resumable, idempotent work queue that the factory
// pipeline runs on (handoff §4 preamble: "stages communicate via SQLite work queue; every
// stage is resumable and idempotent"). It was specified for CP-01 but never built; MC-2's
// spike confirmed the real LLM/TTS stages are multi-second, network- and money-attached, so
// resumability is now load-bearing and is back-filled here.
//
// Model:
//   - work(id, stage, ref, state, attempts, last_error, updated_at) — one row per unit of work.
//     States: pending → running → done | failed.
//   - result_cache(ref, payload, tokens, computed_at) — a completed unit's output, so a resume
//     never recomputes it (no duplicate work).
//   - charges(idem_key) — models an upstream API's idempotency-key dedup: a paid call keyed by
//     brief+candidate is recorded at most once even if the unit is retried across a crash, so a
//     kill-9 mid-stage cannot double-charge.
package workq

import (
	"database/sql"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

// State is a work unit's lifecycle state.
type State string

const (
	Pending State = "pending"
	Running State = "running"
	Done    State = "done"
	Failed  State = "failed"
)

// Unit is one row of the work table.
type Unit struct {
	ID       int64
	Stage    string
	Ref      string
	State    State
	Attempts int
	LastErr  string
}

const schema = `
CREATE TABLE IF NOT EXISTS work(
  id         INTEGER PRIMARY KEY AUTOINCREMENT,
  stage      TEXT NOT NULL,
  ref        TEXT NOT NULL,
  state      TEXT NOT NULL DEFAULT 'pending',
  attempts   INTEGER NOT NULL DEFAULT 0,
  last_error TEXT NOT NULL DEFAULT '',
  updated_at TEXT NOT NULL,
  UNIQUE(stage, ref)
);
CREATE TABLE IF NOT EXISTS result_cache(
  ref         TEXT PRIMARY KEY,
  payload     TEXT NOT NULL,
  tokens      INTEGER NOT NULL DEFAULT 0,
  computed_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS charges(
  idem_key    TEXT PRIMARY KEY,
  computed_at TEXT NOT NULL
);
`

// Queue is a handle on a work-queue database.
type Queue struct{ db *sql.DB }

// Open opens (creating if needed) a work-queue database at path.
func Open(path string) (*Queue, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	// Serialize writers; the queue is single-writer by design (one runner at a time).
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("workq schema: %w", err)
	}
	return &Queue{db: db}, nil
}

// Close closes the database.
func (q *Queue) Close() error { return q.db.Close() }

func now() string { return time.Now().UTC().Format(time.RFC3339Nano) }

// Enqueue adds a unit (idempotent: a duplicate stage+ref is ignored).
func (q *Queue) Enqueue(stage, ref string) error {
	_, err := q.db.Exec(
		`INSERT OR IGNORE INTO work(stage,ref,state,updated_at) VALUES(?,?,?,?)`,
		stage, ref, string(Pending), now())
	return err
}

// ResetStale moves any `running` units (left by a crashed runner) back to `pending` so a
// resume retries them. Safe because completed units are cached and charges are deduped.
func (q *Queue) ResetStale() (int64, error) {
	res, err := q.db.Exec(
		`UPDATE work SET state=?, updated_at=? WHERE state=?`,
		string(Pending), now(), string(Running))
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// Claim atomically moves the oldest pending unit of a stage to `running` and returns it.
// Returns (nil, nil) when the stage has no pending work.
func (q *Queue) Claim(stage string) (*Unit, error) {
	tx, err := q.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	var u Unit
	row := tx.QueryRow(
		`SELECT id, ref, attempts FROM work WHERE stage=? AND state=? ORDER BY id LIMIT 1`,
		stage, string(Pending))
	if err := row.Scan(&u.ID, &u.Ref, &u.Attempts); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	u.Stage = stage
	u.State = Running
	u.Attempts++
	if _, err := tx.Exec(
		`UPDATE work SET state=?, attempts=?, updated_at=? WHERE id=?`,
		string(Running), u.Attempts, now(), u.ID); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return &u, nil
}

// Complete marks a unit done.
func (q *Queue) Complete(id int64) error {
	_, err := q.db.Exec(`UPDATE work SET state=?, updated_at=? WHERE id=?`,
		string(Done), now(), id)
	return err
}

// Fail records an error and either re-queues the unit (attempts < maxAttempts) or marks it
// failed (discard-and-log; §4.4 repair discards after 4).
func (q *Queue) Fail(u *Unit, cause error, maxAttempts int) error {
	next := Pending
	if u.Attempts >= maxAttempts {
		next = Failed
	}
	_, err := q.db.Exec(`UPDATE work SET state=?, last_error=?, updated_at=? WHERE id=?`,
		string(next), cause.Error(), now(), u.ID)
	return err
}

// CachedResult returns a completed unit's cached output, if present.
func (q *Queue) CachedResult(ref string) (payload string, tokens int, ok bool, err error) {
	row := q.db.QueryRow(`SELECT payload, tokens FROM result_cache WHERE ref=?`, ref)
	switch err := row.Scan(&payload, &tokens); err {
	case nil:
		return payload, tokens, true, nil
	case sql.ErrNoRows:
		return "", 0, false, nil
	default:
		return "", 0, false, err
	}
}

// StoreResult caches a unit's output (idempotent on ref).
func (q *Queue) StoreResult(ref, payload string, tokens int) error {
	_, err := q.db.Exec(
		`INSERT OR REPLACE INTO result_cache(ref,payload,tokens,computed_at) VALUES(?,?,?,?)`,
		ref, payload, tokens, now())
	return err
}

// Charge records a paid call keyed by an idempotency key, returning whether this was the first
// time (i.e. whether an actual charge would occur). Repeat keys are no-ops — this models the
// upstream API's idempotency-key dedup so a retry after a crash never double-charges.
func (q *Queue) Charge(idemKey string) (charged bool, err error) {
	res, err := q.db.Exec(`INSERT OR IGNORE INTO charges(idem_key,computed_at) VALUES(?,?)`,
		idemKey, now())
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

// ChargeCount returns the number of distinct paid calls (for the no-double-charge assertion).
func (q *Queue) ChargeCount() (int, error) {
	var n int
	err := q.db.QueryRow(`SELECT COUNT(*) FROM charges`).Scan(&n)
	return n, err
}

// Counts returns a state histogram for a stage.
func (q *Queue) Counts(stage string) (map[State]int, error) {
	rows, err := q.db.Query(`SELECT state, COUNT(*) FROM work WHERE stage=? GROUP BY state`, stage)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[State]int{}
	for rows.Next() {
		var s string
		var n int
		if err := rows.Scan(&s, &n); err != nil {
			return nil, err
		}
		out[State(s)] = n
	}
	return out, rows.Err()
}

// AllResults returns every cached ref→payload, sorted by ref (for output-equality checks).
func (q *Queue) AllResults() (map[string]string, error) {
	rows, err := q.db.Query(`SELECT ref, payload FROM result_cache`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]string{}
	for rows.Next() {
		var ref, payload string
		if err := rows.Scan(&ref, &payload); err != nil {
			return nil, err
		}
		out[ref] = payload
	}
	return out, rows.Err()
}
