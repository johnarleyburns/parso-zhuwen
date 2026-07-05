# CP-01 — Factory Walking Skeleton (Go)

**Refs:** `00-requirements-and-design.md` §8, §14 (CP-01); `01-agentic-handoff.md` §4, §6.
**Goal (handoff §6):** 10-entry canon registry → beat-sheet briefs → retell A2 stories →
segment → coverage gate (I1) → emit a signed fixture pack. *The pipeline is the product.*

## Invariants exercised
- **I1 coverage gate:** `StoryCandidate` has no public constructor; only `gate.Evaluate()`
  can produce a passing candidate. Unit-tested with must-fail fixtures (§4.3 algorithm).
- **I3 pregenerated only:** retell provider is an interface; no runtime generation in the
  emitted pack. CP-01 uses a deterministic fixture provider (`fixture:true`).
- **I6 every story has a Commons image:** pack builder hard-fails on NULL `cover_image_id`
  or an image missing a full provenance record. Golden imageless fixture must fail.

## Module layout (handoff §4)
```
factory/
  go.mod                      module github.com/parso/zhuwen-factory
  cmd/zhuwenctl/              CLI entry (lexicon, build, verify)
  internal/lexicon/           ingest → stable word IDs (§4.1)
  internal/canon/             registry + 10 seed entries (§4.2)
  internal/brief/             beat sheet + lexicon slice (§4.2)
  internal/gen/               retell provider interface + fixture provider (§4.2)
  internal/segment/           FMM segmenter + proper nouns (§4.3)
  internal/gate/              CoverageGate (I1) + StoryCandidate (§4.3)
  internal/pack/              SQLite builder + manifest + ed25519 sign + verifier (§3)
  internal/pipeline/          wires stages canon→brief→gen→segment→gate→pack
  testdata/                   fixture lexicon + canon seeds
```

## Dependencies (handoff §0.4)
| Dep | License | Why |
|-----|---------|-----|
| modernc.org/sqlite | BSD-3-Clause | pure-Go SQLite for `content.sqlite`; no cgo → portable CI |
| (stdlib) crypto/ed25519 | BSD (Go) | detached pack signature; minisign-format upgrade deferred to CP-02 |
| (stdlib) archive/zip | BSD (Go) | `.zpack` container |

## Coverage gate reference (handoff §4.3)
```
types = distinct word_ids excluding proper nouns
new   = types - known(band slice)
FAIL if |new| > 8                                    (type budget)
FAIL if new_token_count / all_token_count > 0.02     (token budget)
FAIL if any w in new with occurrences(w) < 3         (recurrence)
FAIL if any w in new not in frontier_candidates      (frontier discipline)
FAIL if grammar_patterns(text) ⊄ band_whitelist      (grammar gate)
FAIL if proper_noun without first-occurrence gloss
PASS → StoryCandidate{coverage_bitmap, new_type_ids}
```

## Test plan (unit / integration / e2e)

### Unit
- `lexicon`: ingest assigns stable IDs; duplicate/lookup; bitmap size = |lexicon|.
- `segment`: FMM longest-match; proper-noun tagging; sentence splitting; unknown→literal.
- `gate`: PASS baseline; each FAIL branch (type budget, 97.9% token budget, recurrence<3,
  non-frontier new word, grammar out of whitelist, proper noun w/o gloss); bitmap/newtypes
  correctness; `StoryCandidate` cannot be constructed outside the package.
- `canon`/`brief`: 10 seeds load; each has required `pd_rationale` (FR-12.2); brief slice
  = band lexicon ∪ frontier candidates.
- `pack`: builder rejects story with NULL cover image (I6); rejects image missing provenance
  (I6); ed25519 sign/verify round-trip.

### Integration
- `pipeline`: canon→brief→gen(fixture)→segment→gate produces N passing candidates from the
  seed registry against the fixture band; a deliberately over-budget brief is rejected.

### E2E (`cmd/zhuwenctl` + pack on disk)
- `build --pack a2` emits `fixture-a2-v0.zpack`; `verify` accepts it.
- Negative: imageless story → build fails (I6).
- Negative: tampered `content.sqlite` → verify rejects.
- Negative: unsigned/stripped signature → verify rejects.

## Done criteria
- `go test ./...` green; `go vet ./...` clean; `gofmt` clean.
- `plans/cp-01-done.md` demo note with commands + outputs.
