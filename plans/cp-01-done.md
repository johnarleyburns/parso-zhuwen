# CP-01 — Done note (Factory Walking Skeleton)

**Status:** complete. `go test ./...`, `go vet ./...`, `gofmt -l` all green.
42 test functions across unit / integration / e2e.

## What shipped
`factory/` — Go module `github.com/parso/zhuwen-factory`.

Pipeline (handoff §4): lexicon ingest → 10-entry canon registry → beat-sheet briefs →
retell (deterministic fixture provider) → FMM segmentation → **coverage gate (I1)** →
signed `.zpack` (SQLite + manifest + ed25519), with **I6** hard-fail on imageless /
unprovenanced / AI images.

### Packages
- `internal/bitset` — coverage bitmap primitive.
- `internal/lexicon` — TSV ingest, stable word IDs, segmenter dict.
- `internal/canon` — registry loader + FR-12.2 `pd_rationale` validation; 10 seeds.
- `internal/brief` — canon entry → constrained brief (lexicon slice, band, register).
- `internal/gen` — `Provider` interface + `FixtureProvider` (I3: no runtime gen in app).
- `internal/segment` — forward-maximum-matching + proper-noun tagging + sentence split.
- `internal/grammar` — rule-based grammar-pattern detector for the gate whitelist.
- `internal/gate` — **I1**: `Evaluate` + `StoryCandidate` (unexported fields; only the
  gate can produce a populated candidate). Implements the §4.3 reference algorithm.
- `internal/pack` — SQLite builder (`schema.sql`), manifest, ed25519 sign + `Verify`.
- `internal/pipeline` — wires all stages; reuse rule §8A.1(a) (one image per canon).
- `internal/assets` — embedded fixtures (lexicon + canon seeds).
- `cmd/zhuwenctl` — `lexicon` / `build` / `verify` / `keygen`.

## Test pyramid
- **Unit:** bitset, lexicon (incl. dup/bad-row rejection), segment (FMM/proper/literal),
  gate (PASS + every FAIL branch: type budget, **97.9% token budget**, recurrence,
  frontier discipline, grammar whitelist, proper-noun gloss, out-of-lexicon literal),
  gate external test (I1 private-init), grammar, canon (pd_rationale required).
- **Integration:** `pipeline_test` — all 10 seeds pass the gate; a gate-violating provider
  is rejected with reasons.
- **E2E:** `pack_test` — build→verify round trip; I6 imageless/missing/unprovenanced/AI
  builds fail; wrong-key / tampered-content / unsigned packs rejected.
  `cmd/zhuwenctl/e2e_test` — compiles the real binary and drives lexicon→build→verify,
  plus tampered-pack rejection at the CLI boundary.

## Demo
```
$ zhuwenctl lexicon
lexicon fixture-hsk3.0-v0: 30 words, max id 30
$ zhuwenctl build --out fixture-a2-v0.zpack
pipeline: 10 stories packed, 0 rejected
wrote fixture-a2-v0.zpack (10 stories) + pubkey fixture-a2-v0.zpack.pub
$ zhuwenctl verify fixture-a2-v0.zpack --pub fixture-a2-v0.zpack.pub
OK: pack a2 v0.0.0, lexicon fixture-hsk3.0-v0, 11 files
```

## Deviations / notes (for the next agent)
- CP-01 stories use the deterministic `FixtureProvider` (flagged `fixture:true`); real
  LLM retelling is CP-09. Stub images carry complete (fixture) provenance and pass I6.
- Canon `source_urls` use real Wikipedia article URLs (verifiable); revision-permalink
  pinning (handoff §4.2) is deferred to the content build.
- Signature is raw ed25519 over `manifest.json`; **minisign wire-format compatibility is
  a CP-02 task** (handoff §3).
- Canon registry is JSON (stdlib) rather than YAML — no new dependency.

## Next: CP-02
Freeze `schema.sql`, adopt the minisign detached-signature format, and produce the golden
pack suite — unsigned / tampered / **imageless (I6)** — that the reference verifier must
reject, then vendor `Fixtures/` into the iOS repo.
