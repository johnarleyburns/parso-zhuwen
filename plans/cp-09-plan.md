# CP-09 — Content scale-up (canon → ~200, A1–B1 packs, CosyVoice render + Qwen A/B, audit workflow, license re-verification)

**Refs:** handoff §6 (CP-09 acceptance: *audit pass-rates recorded in manifests; pack sizes within
NFR-3/4*), §4 (pipeline stages), §4.4 (repair loop), §4.7 (TTS/alignment), `00` §6 (I1 gate budgets
PED-001..003), §8/§8A (canon + image reuse), NFR-3 (≤90 MB download), NFR-4 (A1+A2 ≤250 MB on disk).
Depends on: CP-01 pipeline (`internal/{pipeline,gen,repair,segment,canon,brief,gate}`), CP-02 pack
format (frozen — **no schema change**), CP-06 alignment stage (`alignment` rows; stub audio today),
CP-08a images (`internal/images`, §8A join; B-4 curation), MC-2 (real `hsk3.0-v1` lexicon; live
DeepSeek harness), MC-3 (`internal/workq` resumable queue; hosted CI).

## 0. This plan opens by importing the MC-2 spike decision (required, `02` §MC-2)

**MC-2 outcome = ADJUST (`plans/mc-2-spike-report.md`).** The live run (real HSK-3.0 lexicon + live
DeepSeek, gate budgets untouched) passed **0/5 at A2 and 0/5 at B1** — naive "prompt + retry" LLM
generation does **not** hold the I1 98%-coverage gate; the repair loop did not converge in 4 iters.
The gate is not broken (the fixture-band harness passes 100%). **CP-09 therefore does NOT proceed as
"prompt + retry at scale," and MUST NOT loosen I1.** The report's five recommendations are the spine
of Part A below:

1. **Vocabulary-constrained generation**, not prompt-only (logit masking / grammar-constrained
   decoding, or a band-vocabulary-aware model).
2. **Token-level repair** — name each *specific* out-of-band word and supply an in-band substitute
   (today's loop names counts/types only).
3. **Brief redesign for the budget math** — at 400 tokens the 2% budget affords ≈2 new types at 3×;
   briefs must target longer texts and plan ≤2 frontier words with heavy repetition.
4. **Proper-noun handling** — pass per-story proper-noun glosses so names are excluded from coverage
   (segmenter already supports them; harness gap caused 10/25 `literal_out_of_lexicon`).
5. **Band strategy** — hand-author / heavily curate the lowest bands (A1–A2); lean on the LLM at B1+.

**Acceptance (handoff §6):** ✅ audit pass-rates recorded in manifests; ✅ pack sizes within NFR-3/4.
Plus a **de-risking re-run gate**: the constrained pipeline must clear the MC-2 bar it failed —
**≥1 story passes I1 at A2 and B1 on the real lexicon** (hermetic fixture-replayed in CI; live behind
`--live`). Plus standing exit criteria (EC-1 READMEs, EC-2 `make build`→`bin/` incl. new subcommands,
EC-3 `make ci` / `swift test` green).

## Invariants in play
- **I1 — coverage gate unchanged.** No budget loosening at any band. Every scaled story passes the
  same `gate.Evaluate`; the shared Go/Swift gate-vector suite stays green. Scale-up changes the
  *generator*, never the gate.
- **I6 — every story has a human-made, non-AI Commons image.** ~200 canon entries + A1–B1 packs each
  need a provenanced cover through the §8A join; this is the **B-4** curation long pole. No
  placeholder-art escape hatch; a story blocked on imagery does not ship.
- **I4 — evidence-gated claims.** Audit pass-rates + the Qwen-vs-DeepSeek A/B are recorded honestly in
  the pack manifest and a report; no fabricated metrics (the MC-2 discipline).
- **I2 — app network surface unchanged.** All generation/TTS/audit is factory build-time (behind
  `--live`); CI is hermetic (fixture snapshots). `make audit` stays green.
- **NFR-3/NFR-4 — size budgets.** Real CosyVoice Opus audio + real HEIC covers must keep the app
  download ≤90 MB and A1+A2-on-disk ≤250 MB; measured and asserted, not assumed.

## Part A — Constrained generation + convergent repair (`internal/gen`, `internal/repair`)

The MC-2 fix. Retire "prompt + retry"; make the generator hold the whitelist.

1. **Vocabulary-constrained decoding.** Add a `ConstrainedProvider` that restricts output to the
   known+frontier token set. Two implementable paths (pick per owner decision, see Dependencies):
   (a) **logit-bias / grammar-constrained** decoding against a model that exposes it, or (b) a
   **candidate-rerank** loop: oversample N candidates and select the gate-passing one. The interface
   stays `gen.Provider`; CI uses a deterministic constrained fixture provider (no network).
2. **Token-level repair (`internal/repair`).** Extend the checker to emit, per failed story, the
   *specific* offending tokens (out-of-band word IDs / under-recurring types) and a candidate in-band
   substitute drawn from the known set; feed a **name-and-replace** rewrite prompt (not the current
   count/type summary). Bounded iterations; record convergence stats.
3. **Proper-noun glosses.** Thread per-story proper-noun dictionaries through brief → gen → segment so
   names segment as `properNoun` (excluded from the coverage denominator, already supported by
   `segment`/`gate`), closing the MC-2 `literal_out_of_lexicon` harness gap.

## Part B — Brief redesign + band strategy (`internal/brief`, `internal/canon`)

4. **Budget-aware briefs.** Briefs must encode the budget math: target length band, an explicit plan
   of ≤`maxNewTypes` frontier words each recurring ≥`minRecurrence`, and topic/grammar whitelist. The
   brief becomes a coverage *contract* the generator is scored against, not free-retelling.
5. **Canon → ~200 entries.** Grow `internal/assets/canon.seed.json` from 10 → ~200 curated public-domain
   source entries with PD rationale + topic/canon_id keys (feeds §8A image reuse rules). Deterministic,
   reviewed.
6. **Hand-authored low bands (A1–A2).** Where the known set is too small for free generation, ship
   hand-authored / heavily-curated stories through the *same* I1 gate (no gate fork). These + the
   Foundations F2 micro-stories (CP-08a) are the A1 backbone; the LLM carries B1+.

## Part C — CosyVoice render + alignment (external build-time stage, `internal/assets`/pipeline)

7. **Real CosyVoice render.** Replace the CP-06 stub audio bytes with a real render as an **external
   build-time stage** (same pattern as the aligner/HEIC encoder, handoff §1): shell out behind a config
   flag; deterministic stub encoder for CI. Emits per-story Opus + real word-level `alignment` rows.
8. **Alignment re-verification.** Re-run the CP-06 drift check against real audio (Swift
   `KaraokeDriftTests` budget <120 ms over a 3-min story) on a sampled story.

## Part D — Generator A/B + audit workflow (`zhuwenctl audit`, manifest fields)

9. **Qwen vs DeepSeek A/B.** Run both Chinese-strong models through the constrained pipeline on a fixed
   brief set; record pass@0, post-repair pass-rate, mean iters, tokens, and a human-quality sample per
   model. Report in `plans/cp-09-ab-report.md` (the MC-2 report format).
10. **Audit workflow + manifest pass-rates.** A per-pack audit stage samples stories for human review
    (quality/naturalness/cultural-fit flag) and **records the audit pass-rate + generator + model into
    the pack manifest** (handoff §6 acceptance). No schema change to `content.sqlite`; manifest is JSON.
11. **License re-verification memo.** Re-verify every canon source's PD/CC status + every shipped image
    license at scale; record in `factory/data/README.md` + `plans/cp-09-license-memo.md`.

## Part E — Size budgets (NFR-3/NFR-4)

12. **Measure + assert.** With real Opus audio + real HEIC covers, measure the app download size
    (≤90 MB, NFR-3) and A1+A2-on-disk (≤250 MB, NFR-4); add a factory size-budget test that fails the
    build if a pack breaches. Recheck the CP-08a image-weight estimate against reality.

## Tasks
- A: `internal/gen` `ConstrainedProvider` (+ fixture); `internal/repair` token-level name-and-replace;
  proper-noun glosses through brief→gen→segment.
- B: budget-aware `internal/brief`; canon.seed.json → ~200 (reviewed); hand-authored A1–A2 set.
- C: CosyVoice external render stage + stub encoder; real `alignment` rows; drift re-verify.
- D: `zhuwenctl audit` + manifest pass-rate/generator/model fields; Qwen/DeepSeek A/B; reports.
- E: size-budget measurement + failing test; NFR-3/4 recheck note.
- Docs: root README + `factory/data/README.md` + `ios/README.md` (CP-09 status, new subcommands,
  CosyVoice run), `plans/cp-09-done.md`, `plans/cp-09-ab-report.md`, `plans/cp-09-license-memo.md`.
- Blockers: file/track CosyVoice tooling access, constrained-decoding model access, Qwen key, and
  **B-4** (real Commons curation) in `plans/blockers.md`; ship fixture stand-ins until cleared.

## Tests (unit / integration / e2e)
- **Constrained gen (Go, hermetic):** the constrained fixture provider produces gate-passing stories
  at A2 and B1 on the real lexicon slice — the **MC-2 re-run gate** (≥1 pass each), replayed from
  snapshots so CI never hits the network.
- **Token-level repair (Go):** a story with named out-of-band tokens converges after name-and-replace;
  convergence stats asserted; non-convergence still discards (never loosens the gate).
- **Proper-noun (Go):** names segment as proper nouns and are excluded from the coverage denominator
  (no `literal_out_of_lexicon`); first-occurrence gloss enforced (existing `properNounGloss` code).
- **I1 regression:** shared gate-vector suite stays green (Go `gatevec` + Swift `swift test`); scaled
  packs verify (I6 + sig + hashes) via the existing golden suite.
- **Audit/manifest (Go):** manifest carries audit pass-rate + generator + model; verifier still passes;
  a fabricated/malformed audit field is rejected.
- **CosyVoice/alignment:** stub-encoder path deterministic in CI; sampled real render re-verifies the
  Swift <120 ms karaoke drift budget (pre-push, like CP-06).
- **Size budget (Go):** a pack exceeding NFR-3/NFR-4 fails the build.
- **CI gate:** `make ci` (factory) + `swift test` green; `make audit` green (no new app network
  surface); `make build` → new subcommands runnable from `bin/` (EC-2).

## Dependencies (handoff §0.4) — confirm with owner before wiring
- **CosyVoice** (TTS render) — external build-time stage (not linked into the app); CI uses the
  deterministic stub encoder. Same treatment as the aligner/HEIC encoder.
- **Constrained-decoding access** — either a model/endpoint exposing logit bias / grammar-constrained
  decoding, or the candidate-rerank fallback (no special access, higher token cost). **Owner decision.**
- **Qwen API access** (for the A/B) — behind `--live`; DeepSeek key already available (MC-2).
- No new **shipped app** dependency (generation/TTS/audit are all factory-side).

## Blockers filed / carried with this plan
- **B-4 — real Commons image curation + license verification** (from CP-08a) — ⏳ owner-in-the-loop; the
  ~200-canon + A1–B1 covers can't ship without it. Fixture stand-ins until cleared.
- **B-5 (new) — CosyVoice tooling + voice licensing** — record the render tool + voice-model license
  before wiring the real stage.
- **B-6 (new) — constrained-decoding path decision** — logit-bias model vs candidate-rerank; blocks
  Part A's chosen implementation (fixture provider unblocks CI meanwhile).

## Follow-ons (not CP-09)
- CP-10 ship polish: accessibility (Dynamic Type XXL, VoiceOver), App Store assets, HSK-3.0 launch
  messaging, methodology page with citations (I4), full-device QA checklist (`plans/cp-10-qa.md`).
- Broader B2 content; multi-voice CosyVoice; automated (non-sampled) audit models.
