# CP-09b — Scale content (budget-aware briefs, hand-authored A1–A2, canon → ~200, audit workflow, license memo)

**Refs:** handoff §6 (CP-09 acceptance: *audit pass-rates recorded in manifests; pack sizes within
NFR-3/4*), §4.2 (brief/gen), §4.4 (repair), `00` §6 (I1 budgets PED-001..003), §6A (canon), §8/§8A
(canon + image reuse). Builds directly on **CP-09a** (`plans/cp-09a-spike-report.md`): the go/no-go
gate is passed with a **split verdict** — DeepSeek candidate-rerank clears I1 at **B1+** (~$0.07 per
accepted story, ~1/3 accept rate) but is **structurally unable to generate A1–A2** (a 638-word known
set cannot express canon narrative at 98%). CP-09b turns those findings into shippable content.
Depends on: CP-09a (`gen.ConstrainedProvider`, token-level `internal/repair`, proper-noun threading,
constrained fixture), CP-08a images (`internal/images`, §8A join, B-4 curation) + Foundations F2
spine, CP-02 pack format (**frozen — no `content.sqlite` schema change**), MC-3 (`internal/workq`
resumable queue). This is the **money-and-labor** checkpoint; the R&D risk was retired in 09a.

## 0. This plan opens by importing the CP-09a decision (required)

**CP-09a outcome = PARTIAL GO / ADJUST.** Two commitments flow from it and are non-negotiable here:

1. **B1 and above are LLM-generated** via `gen.ConstrainedProvider` (candidate-rerank on DeepSeek) +
   token-level name-and-replace repair. The one measured weakness is the **~1/3 accept rate** →
   discard-dominated cost. Part A attacks exactly that with **budget-aware briefs**.
2. **A1–A2 are hand-authored / heavily templated**, never LLM-generated, through the **same I1 gate**
   (no gate fork). CP-09a proved the gate accepts cooperative A2 input (the constrained fixture passes
   A2 hermetically); hand-authored A2 will pass identically. Part B delivers this backbone.

Nothing in CP-09b loosens I1, adds an app network surface (I2), or ships a non-provenanced image (I6).

## Invariants in play
- **I1 — coverage gate unchanged.** Every scaled story (LLM B1+ *and* hand-authored A1–A2) passes the
  same `gate.Evaluate`; the shared Go/Swift gate-vector suite stays green. Budget-aware briefs change
  the *generator's target*, not the gate. Non-convergent LLM stories still discard (09a behavior).
- **I6 — every story has a human-made, non-AI Commons image.** ~200 canon entries + A1–B1 packs each
  need a provenanced cover through the §8A join. **B-4 curation is the real schedule long pole**;
  canon growth is *paced to curation throughput*, not the reverse. No placeholder-art escape hatch.
- **I4 — evidence-gated claims.** Audit pass-rate + generator + model are recorded honestly in the
  pack manifest; a fabricated/malformed audit field is rejected by the verifier. The license memo
  cites real sources. (The MC-2 / CP-09a discipline: no invented metrics.)
- **I2 — app network surface unchanged.** All generation/audit is factory build-time behind `--live`;
  CI is hermetic (constrained fixture + snapshot briefs). `make audit` stays green.
- **NFR-3/NFR-4 — size budgets** are *measured* in CP-09c (needs real audio/HEIC weights); CP-09b adds
  the manifest fields + a story-count/text-weight sanity check but defers the hard byte budget to 09c.

## Part A — Budget-aware briefs + productionized generation (`internal/brief`, `internal/pipeline`)

The CP-09a lever to raise the B1 accept rate and cut discard cost, plus wiring the constrained
generator (today only in the `spike` harness) into the real build path.

1. **Coverage-contract brief (`internal/brief`).** Extend `BandSpec`/`Brief` so a brief is a scored
   *contract*, not free retelling (MC-2 rec #3, review 09b): (a) explicit **target length band** sized
   so the 2% budget affords the planned new words (09a showed short drafts starve the budget —
   ~600-char targets); (b) an explicit **plan of ≤`planNewTypes` frontier words** (a small curated
   subset of the band frontier, e.g. 4–6 word IDs) each to recur ≥`minRecurrence`; (c) topic/grammar
   whitelist. `planNewTypes` ≤ the gate's `MaxNewTypes` (never a loosening). The prompt (`gen.BuildMessages`)
   lists *that small chosen set* rather than the whole frontier (09a already caps the dump; this makes
   the choice deliberate and per-story).
2. **Wire `ConstrainedProvider` into `pipeline` + `workq`.** Today `ConstrainedProvider`/name-and-replace
   live only in `internal/spike`. Make the real `pipeline.Run` (and the resumable `zhuwenctl run` queue,
   MC-3) generate B1+ stories through the constrained rerank + repair loop with the per-story token
   ceiling, recording per-story accept/discard/tokens/iters. Idempotent under kill-9 (reuse `workq`
   charge/dedup); a discard is a recorded terminal state, never a silent retry.
3. **Accept-rate measurement.** Re-run the 09a B1 protocol with budget-aware briefs on a fixed brief
   set; record the new accept rate, mean N, mean repair iters, tokens, and **$ per accepted story** vs
   the 09a baseline in `plans/cp-09b-done.md` (I4). Target: materially above 09a's ~1/3 (no fabricated
   claim — report whatever it is). If it does *not* improve, the documented fallback is more
   hand-authoring / the local-GBNF contingency (unchanged from 09a).

## Part B — Hand-authored A1–A2 backbone (`internal/authored`, `internal/canon`)

CP-09a's mandate: A1–A2 do not come from the LLM. Deliver the spine that does.

4. **Authored-story ingest through the same gate.** New `internal/authored` (or `canon` extension):
   load operator-written A1–A2 stories (JSON/TSV, `body` + title + canon_id/topic + band + proper-noun
   glosses) and run them through the **identical** `segment → gate.Evaluate` path. A story that fails
   I1 is rejected with the same token-level diagnostics the repair loop uses (so the human author gets
   "name-and-replace"-style feedback). No gate fork; authored and generated stories are indistinguishable
   to the verifier except by an `origin`/`generator` tag.
5. **A1 spine = authored stories + CP-08a Foundations F2 micro-stories.** The F2 picture-book stories
   (already through the I1 gate at CP-08a) plus a hand-authored A2 set form the A1–A2 backbone; DeepSeek
   rerank carries B1+. Honors the design risk-table stance ("A1 relies more on scripted Foundations").
6. **Author tooling.** A `zhuwenctl authored check <file>` subcommand (EC-2 → `bin/`) that gates a draft
   and prints the token-level failures, so authoring is a fast local loop, not a full pack build.

## Part C — Canon growth (`internal/assets/canon.seed.json`), paced to B-4

7. **Canon 10 → ~200 curated PD entries**, each with `pd_rationale` + topic/canon_id keys (feeds §8A
   image reuse). Deterministic, reviewed. **Paced to B-4 image-curation throughput** — an entry without
   a gate-passing Commons cover (I6) does not ship, so grow canon in batches that curation can cover.
   Ship a **smaller first content pack** rather than blocking on all ~200 covers (review guidance).
8. **Canon validation** stays as today (`canon.Validate`): every entry needs canon_id/tier/title/beats/
   sources/pd_rationale/origin. Add a batch lint that flags entries lacking a curated image join.

## Part D — Audit workflow + manifest fields (`zhuwenctl audit`, `pack.Manifest`)

Handoff §6 core acceptance.

9. **Audit stage.** `zhuwenctl audit` samples stories from a built pack for human quality review
   (naturalness / cultural-fit / register flag), records per-story verdicts, and computes a pack-level
   **audit pass-rate**. Human-in-the-loop like curation; the *sampling + record model* is what CP-09b
   delivers and unit-tests (headless), with a fixture decisions file for CI.
10. **Manifest fields (JSON only — NO schema change).** Add to `pack.Manifest`: `audit_pass_rate`,
    `audit_sample_size`, `generator` (e.g. `deepseek-rerank` / `hand-authored`), `model`
    (e.g. `deepseek-chat`). These live in `manifest.json` (already signed + hashed); **no
    `content.sqlite` DDL change** (SchemaVersion stays 1). The **verifier rejects a malformed/absent/
    out-of-range audit field** (I4: a fabricated audit field must not pass). Extend the golden-reject
    suite accordingly.

## Part E — License re-verification memo (`plans/cp-09-license-memo.md`, `factory/data/README.md`)

11. **License memo.** Re-verify every canon source's PD/CC status **and** every shipped image license
    at scale; record per-source in `plans/cp-09-license-memo.md` + `factory/data/README.md`. Ties to
    B-4 (per-image CC-BY/SA attribution is a legal obligation, FR-11.2) and the CC-BY/SA Credits screen.

## Tasks
- A: coverage-contract fields in `internal/brief` + per-story chosen-frontier prompt; wire
  `ConstrainedProvider` + name-and-replace into `internal/pipeline` and `zhuwenctl run` (workq);
  accept-rate re-measurement vs 09a.
- B: `internal/authored` ingest + gate reuse + token-level author feedback; `zhuwenctl authored check`;
  hand-authored A2 set (+ F2 spine wiring).
- C: `canon.seed.json` 10 → ~200 (reviewed, PD rationale, topic keys), paced to B-4; canon image-join lint.
- D: `zhuwenctl audit` sampling + record model; `audit_pass_rate`/`sample_size`/`generator`/`model`
  in `pack.Manifest`; verifier reject for malformed audit fields; golden negatives.
- E: `plans/cp-09-license-memo.md` + `factory/data/README.md` license section.
- Docs: root README + `factory/data/README.md` + `ios/README.md` (new `audit`/`authored` subcommands,
  budget-aware briefs, split generation strategy), `plans/cp-09b-done.md`.
- Blockers: carry **B-4** (real Commons curation at ~570-image scale) in `plans/blockers.md`; fixture
  stand-ins until cleared.

## Tests (unit / integration / e2e)
- **Budget-aware brief (Go):** the contract compiles to a prompt naming the chosen ≤`planNewTypes`
  frontier words; `planNewTypes ≤ MaxNewTypes` invariant enforced; deterministic prompt (golden).
- **Constrained pipeline (Go, hermetic):** the constrained **fixture** provider produces gate-passing
  B1 (and A2 via authored/fixture) stories through `pipeline.Run` + workq; kill-9 resume records a
  discard/accept once, never double-charges (extends MC-3 e2e). CI never hits the network (I2).
- **Authored ingest (Go):** an authored A2 story passing I1 packs; one that fails I1 is **rejected**
  with token-level diagnostics; authored + generated stories both verify identically (no gate fork).
- **Audit/manifest (Go):** manifest carries `audit_pass_rate`/`sample_size`/`generator`/`model`;
  verifier still passes a well-formed pack; a **fabricated/malformed/out-of-range audit field is
  rejected** (new golden negative). No `content.sqlite` schema change (SchemaVersion == 1 asserted).
- **Canon (Go):** ~200 entries validate; every shipped story has a curated image join (I6) or is held
  back from the pack (no placeholder).
- **I1 regression:** shared gate-vector suite green (Go `gatevec` + Swift `swift test`); scaled packs
  verify (sig + hashes + I6) via the existing golden suite.
- **CI gate:** `make ci` (factory) + `swift test` green; `make audit` green (no new app network
  surface); `make build` → `zhuwenctl audit` / `zhuwenctl authored` runnable from `bin/` (EC-2);
  READMEs updated (EC-1).

## Dependencies (handoff §0.4)
- **DeepSeek** (generation) — behind `--live`; key already available (`~/.deepseek-api-key`). CI uses
  the constrained fixture. No paid Qwen A/B (cut by the review — DeepSeek is the committed generator).
- **No new *shipped* app dependency.** Generation/audit are factory-side.
- **HEIC encoder / Commons** — unchanged from CP-08a (build-time, stubbed in CI).

## Blockers carried with this plan
- **B-4 — real Commons image curation + per-image license verification** — ⏳ owner-in-the-loop; the
  ~200-canon + A1–B1 covers can't ship without it. The `imagespike` review-sheet loop scales per batch;
  canon growth (Part C) is paced to it. Fixture stand-ins until cleared.

## Follow-ons (CP-09c / CP-10)
- **CP-09c:** real CosyVoice render (local, Apple Silicon, $0) + alignment re-verify (Swift
  `KaraokeDriftTests` <120 ms) + **NFR-3/NFR-4 size budgets** measured against real Opus + HEIC, with a
  build-failing size-budget test. (Deferred here because it needs real audio/image weights.)
- **Cut from scope entirely** (review): paid Qwen generator A/B and paid Qwen3-TTS voice A/B.
- Local GBNF-constrained Qwen2.5 remains the documented $0-API contingency; not built (rerank works at
  B1+, and it would not fix A2's semantic-coverage wall — A2 is an authoring problem, not decoding).
