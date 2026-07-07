# CP-08a — Commons image pipeline + curation TUI (§8A) · Foundations F0–F3 (FR-11, M14) · first-run onboarding gating

**Refs:** handoff §6 (CP-08a acceptance: *I6 builder tests; a scripted zero-knowledge run reaches
F3 in ~300 words*), §4.6 (image pipeline stage contract), §8A (Commons pipeline stages),
`00` §5A (Foundations spec), FR-11.1–11.6, M14 (Foundations card mockup), FR-1.4 (absolute
beginner → Foundations), FR-1.5 (re-placement), I6 (every story has a human-made Commons image),
I1 (F2 micro-stories reuse the same coverage gate), I5 (Foundations feeds the one KnownWordModel),
I4 (honest-limits methodology note), I2 (Commons fetch is a *factory/build-time* network call — the
app runtime network surface is unchanged). Depends on: CP-01 pipeline (`internal/pipeline`,
`internal/pack` — schema already carries `image` + `foundations_card` + `story.cover_image_id NOT
NULL`), CP-02 pack format (frozen; no schema change), CP-04 `CoverageGate`/`Selector` (reused by the
F3 handoff test), CP-05 placement (`PlacementView`, `PlacementSeed`, `skipAsBeginner`).

**Acceptance (handoff §6):** §8A pipeline end-to-end with curation TUI; Foundations F0–F3 built on
the curated inventory; handoff threshold logic. ✅ **I6 builder tests; a scripted zero-knowledge run
reaches F3 in ~300 words.** Plus standing exit criteria (EC-1 READMEs, EC-2 `make build`→`bin/` incl.
new `images` subcommands, EC-3 `make ci` / `swift test` green).

**De-risking note.** CP-08a is content-and-curation heavy — the same long pole MC-2 flagged. The image
sourcing is validated up front with a real Commons spike (`factory/cmd/imagespike`, run against the F0
seed words) exactly like MC-2's live-LLM spike, and the pipeline is built + tested against **hermetic
fixture snapshots** so CI never touches the network. **B-3 is resolved** by deriving imageability from
Commons availability (no dataset to license); **B-4** (the real ~570-image curation + per-image license
sign-off) is human-in-the-loop and now has a concrete review artifact — the owner reviews best-guess
picks in the generated HTML sheet and exports a decisions JSON (see `plans/blockers.md`).

## Invariants in play
- **I6 — every story has a human-made, fully-provenanced, non-AI Commons image.** Already enforced in
  `pack.Build`/`verify.go` (schema `cover_image_id NOT NULL`; provenance completeness; AI-category
  reject; golden imageless-reject suite). CP-08a makes the provenance *real* (from Commons
  `extmetadata`) instead of the `pipeline.go` stub, and extends the mandate to Foundations card images
  and cover art. **Do not** add a placeholder-art escape hatch; a story/word blocked on imagery does
  not ship. New golden negatives: image with NC/ND/GFDL-only/missing license must fail the license
  gate; image in `Category:AI-generated images` must fail; Foundations card referencing a missing
  image must fail the build.
- **I1 — coverage gate unchanged.** F2 micro-stories go through the *same* `gate.Evaluate` with a
  smaller known set; no budget is loosened. The F3 handoff test reuses `CoverageGate`/`Selector`.
- **I5 — one replayable model.** Foundations is **not** a separate flashcard silo: every F0 recognize/
  read/bind interaction and every F1/F2 pass emits ordinary `Event`s that fold into the single
  `KnownWordModel` (append-only log, replay-projected). No Foundations-only mutable state.
- **I2 — app network surface unchanged.** The Commons API is called only in the **factory** (build
  time), behind `--live`, never from the app and never in CI. `make audit` must stay green (URLSession
  still only in `PackClient`). Curated images ship *inside* packs.
- **I4 — evidence-gated claims.** The §5A.4 honest-limits deviation (one-line English gloss fallback;
  picture-binding degrades for abstraction) is stated on the methodology page with rationale.
- **No new *shipped* dependencies.** Bubble Tea (TUI, already listed handoff §1) and the HEIC/image
  processing tool are **build-time factory** tools, not app dependencies (see "Dependencies" below).

## Part A — Factory: §8A Commons image pipeline (`internal/images` + `zhuwenctl images …`)

Stage contract per handoff §4.6 / `00` §8A. New package `factory/internal/images`; new CLI group
`zhuwenctl images {fetch | gate | curate | process | join}` (EC-2 → `bin/`). Every stage resumable/
idempotent through the MC-3 work queue (`internal/workq`) — image fetches are multi-second network
stages with the same kill-9 requirement as gen/TTS.

1. **`fetch`** — `CommonsClient` queries the Wikimedia Commons API
   (`action=query&generator=search` + Wikidata P18 for the concept item) → top-N candidates with
   `imageinfo`/`extmetadata` (license, author, attribution, categories). **Network is behind `--live`**
   (reads no secret — Commons is anonymous; still gated so CI is hermetic). Offline/CI path replays
   cached JSON snapshots under `factory/testdata/images/` (like the MC-2 fixture provider). Output:
   `candidate(word_or_canon, image_url, extmetadata_json, categories_json, retrieved_at)` rows.
2. **`gate`** — pure, hermetic. **License filter (hard gate):** accept only PD / CC0 / CC-BY /
   CC-BY-SA (any version); reject NC, ND, GFDL-only, "fair use", missing/ambiguous. **Quality gate:**
   short-side < 1200px reject; AI-category reject (`Category:AI-generated images` + descendants);
   OCR/watermark and NSFW passes are stubbed with a documented interface at CP-08a (real OCR/safety
   models are a CP-09 hardening item — recorded, not silently skipped). Prefer PD/CC0/CC-BY over SA on
   ties (§8A.2 note). Emits the full attribution record.
3. **`curate`** — Bubble Tea pick-of-N TUI (parso-pdaudio pattern): one keystroke per candidate, best-
   of-N per word, cultural-mismatch flag (包子≠generic bun), re-query request. Human-in-the-loop by
   design — the *TUI and its selection model* are what CP-08a delivers and unit-tests (model/update
   logic headless); the actual ~570-image curation pass is the human deliverable behind B-4.
4. **`process`** — attention-aware square + 4:3 crops, sRGB, tonal normalization, **HEIC at 480px
   (card) + 1200px (zoom)**. Image encoding runs as an **external build-time stage** (handoff §1
   already treats CosyVoice/aligner this way): shell out to `libheif`/`heif-enc` behind a config flag,
   with a deterministic stub encoder for CI (no libheif on the runner). Precompute FR-11.3 distractor
   sets.
5. **`join`** — replaces the `pipeline.go` stub image record with the real curated `image` rows +
   provenance and joins `story → image` (reuse rules §8A.1: (a) canon-share by `canon_id`, (b) C7/
   original topic pool ~150 keyed by topic w/ one-use-per-topic-per-band, (c) course covers). I6
   builder/verifier already enforce the join; extend to `foundations_card.image_id`.

## Part B — Factory: Foundations content F0–F3 (`internal/foundations`)

Builds the `foundations_card` rows (schema already exists) and the F2 micro-stories, all against the
curated inventory. Word order = HSK-3.0 level-1 membership × corpus frequency × **imageability**,
where imageability is **derived from Wikimedia Commons itself** (B-3 resolution): a word is imageable
iff the §8A pipeline sources a license-clean, on-concept, ≥1200px Commons photo for it; words that
yield no gate-passing image fall below the floor and defer to F1 patterns (§5A.2). No external
concreteness-norm dataset — nothing to license. The `zhuwenctl images fetch|gate` stages supply
`word → (candidateCount, bestPick)`; a fixture snapshot drives CI hermetically.

6. **Selection & sets.** `internal/foundations`: order the ~220 photographable words into semantic
   sets of 6–8 (animals, food/drink, family, numbers, body, colors, home, places, weather, actions);
   words below the imageability floor deferred to F1 patterns (§5A.2).
7. **F0 cards.** Per word emit `foundations_card(word_id, image_id, set_id, stage='F0', distractor_ids)`
   with the 4-step cycle metadata (introduce/recognize/read/bind). Distractors drawn from
   **already-taught words only**, never same minimal-pair set twice in a row (FR-11.3) — property-tested.
8. **F1 patterns.** Picture-anchored copular/demonstrative templates (这是狗 / 这不是猫) carrying
   function words (是的很不这那吗) that are never isolated cards; content words carry meaning (§5A.1).
9. **F2 micro-stories.** 3–6 page picture-book stories through the **same I1 gate** (smaller known
   set); first seals earned here. Reuse `internal/pipeline` gate machinery — no gate fork.
10. **F3 handoff.** Threshold = effective known set can gate **≥20 distinct A1 stories at ≥98%**;
    computed with the existing gate/selector. Emit the handoff marker into the pack/course metadata.

## Part C — iOS: Foundations engine + UI + first-run onboarding (`ZhuwenCore` / `ZhuwenUI`)

11. **`ZhuwenCore/Foundations.swift` — pure engine.** `FoundationsCard`, `FoundationsSet`,
    `FoundationsSession` (F0 introduce→recognize→read→bind; 5–8 min sessions that **end on an F1/F2
    recombination pass, never on isolated cards**, FR-11.4), distractor selection (FR-11.3),
    set-sequencing, and `HandoffGate` (F3: reuse `CoverageGate`/`Selector` to test ≥20 A1 stories
    ≥98%). Every interaction produces an `Event` folded into the existing `KnownWordModel` (I5) — one
    model. FR-11.6: `startingSet(for: PlacementSeed)` lands a partial beginner at the first unmastered
    set rather than at zero.
12. **`ZhuwenUI/FoundationsView.swift` (M14).** Picture-word card UI (photo + audio + hanzi + tone-
    colored pinyin), the 4-step interaction, recognition grids. **Attribution** long-press sheet (author
    / license / source link) + a **Credits** screen listing every image (FR-11.2, CC-BY/SA legal
    obligation). Audio on every interaction (system-TTS layer for taps, pack audio for intros).
13. **Progress integration (FR-11.5).** Words-known counter counts from word #1; CEFR card reads
    **"Pre-A1 · Foundations"** until the F3 handoff fires; then the regular Today/lattice loop activates.
14. **First-run onboarding gating (the piece flagged in this session).** In the `@main` app
    (`ios/App`) + `AppModel`/`LearnerModel`: on first launch with no persisted `PlacementResult`,
    **auto-present** the placement flow (M1 welcome/privacy → M2 → M3) instead of the manual toolbar
    button; **persist** the result via the existing `PlacementResultRecord` (SwiftData) so it never
    re-shows; route **complete/partial beginners → Foundations** at their first unmastered set (FR-1.4/
    11.6); regular loop activates at F3 handoff. Re-run from Settings still merges (FR-1.5). Keep the
    manual "Placement" affordance in Settings only.
15. **Methodology page (I4).** State §5A.4 honest limits (picture-binding degrades for abstraction;
    one-line English gloss is a behind-a-tap fallback, off by default) with rationale.

## Tasks
- A: `internal/images` (`fetch`/`gate`/`curate`/`process`/`join`) + `zhuwenctl images …` + workq
  integration + `factory/testdata/images/` snapshots; retire the `pipeline.go` stub image record.
- B: `internal/foundations` (selection/sets, F0 cards, F1 patterns, F2 micro-stories via I1 gate, F3
  handoff) + fixture imageability table; populate `foundations_card` in the fixture pack.
- C: `ZhuwenCore/Foundations.swift`; `ZhuwenUI/FoundationsView.swift` + attribution/Credits;
  Progress "Pre-A1 · Foundations"; first-run onboarding gating in `ios/App`/`AppModel`; methodology page.
- Docs: root README + `ios/README.md` (CP-08a ✅, new `images` commands, Foundations run/test),
  `factory/data/README.md` (image-source provenance + concreteness-norm provenance once B-3 clears),
  `plans/cp-08a-done.md`.
- Blockers: file **B-3** (imageability/concreteness-norm licensing) and **B-4** (real Commons image
  curation — human + license clearance) in `plans/blockers.md`; ship fixture stand-ins until cleared.

## Tests (unit / integration / e2e)
- **`images` license/quality gate (Go, hermetic):** golden fixtures — CC-BY/CC0/PD/CC-BY-SA pass;
  NC/ND/GFDL-only/missing **reject**; `Category:AI-generated images` **reject**; < 1200px **reject**.
- **`images` curate model (Go, headless):** pick-of-N selection, cultural-flag, re-query transitions.
- **I6 builder/verifier (extends CP-02 suite):** imageless Foundations card fails build; missing
  provenance fails; AI-categorized fails; existing golden imageless-reject stays green.
- **`foundations` (Go):** distractor sets drawn only from taught words + no repeated minimal pair
  (FR-11.3, property test); F2 micro-story passes the I1 gate; **F3 handoff = ≥20 A1 stories ≥98%**.
- **`FoundationsTests` (Swift):** session ends on F1/F2 recombination (FR-11.4); interactions fold into
  `KnownWordModel` (I5); `startingSet(for:)` resumes a partial beginner (FR-11.6); `HandoffGate` fires
  at the threshold.
- **Zero-knowledge e2e (the headline acceptance):** a scripted learner starting from **zero known
  words** runs Foundations F0→F1→F2 and reaches **F3 handoff in ~300 words** (Go pipeline test +/or
  Swift simulation) — the CP-08a acceptance criterion.
- **XCUITest (`ios/App`):** fresh launch → onboarding auto-presents placement → complete-beginner →
  Foundations → answer cards → **relaunch persists** (placement result + Foundations progress; ties to
  MC-1 `LaunchReplayTests`).
- **CI gate:** `make audit` stays green (no new app network surface); `make ci` (factory) + `swift
  test` green; `make build` → new `images` subcommands runnable from `bin/` (EC-2).

## Dependencies (handoff §0.4)
- **Bubble Tea** (`github.com/charmbracelet/bubbletea`, MIT) — curation TUI; already listed in handoff
  §1 toolchain. Build-time factory tool, not shipped in the app.
- **HEIC encoder** (`libheif`/`heif-enc`, LGPL) — invoked as an **external build-time stage** (same
  pattern as CosyVoice/aligner, handoff §1), never linked into the app; CI uses a deterministic stub
  encoder. Decision recorded here; confirm with owner before wiring the real tool.
- **No new app (iOS) dependency.** Foundations is pure Swift + SwiftUI + the existing packages.

## Blockers filed with this plan
- **B-3 — imageability source** — ✅ **RESOLVED**: imageability is derived from Commons availability
  (a word is imageable iff the §8A gate returns a photo), so no concreteness-norm dataset is licensed.
- **B-4 — real Commons image curation + license verification** — ⏳ **awaiting owner review**. Best-guess
  automation is built (`factory/cmd/imagespike`, the seed of `internal/images`): it fetches from
  Commons, applies the §8A gate, auto-picks, and emits an HTML review sheet + `picks.json`. First run
  covers the 11 F0 seed words. Owner reviews/overrides in the sheet and exports a decisions JSON (see
  `plans/blockers.md` B-4 for the review steps); scale to the ~570-image inventory is the same loop.

## Follow-ons (not CP-08a)
Real OCR/watermark + NSFW/safety models (stubbed here → CP-09 hardening); the actual ~570-image
curated inventory and concreteness-norm ingest (B-3/B-4); CosyVoice Foundations card-intro audio
(CP-09); pack-size recheck with real HEIC covers against NFR-3/4 (CP-09).
