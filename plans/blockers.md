# Blockers

Open items requiring a human decision, a secret, or license-cleared data that an agent cannot
supply without violating an invariant (handoff §0.7). Resolved items are kept for the record.

---

## B-1 — HSK-3.0 word list licensing — ✅ RESOLVED (2026-07-06)

The project owner independently verified that the official HSK-3.0 vocabulary list may be
redistributed as part of a language-learning application. The real lexicon is now committed at
`factory/data/hsk3.0/level-*.tsv` (12,283 unique written forms — vocabulary + recognition
characters — mapped exactly to the official per-level standard) and ingested to `lexicon.sqlite`
(`lexicon_version = hsk3.0-v1`) via `zhuwenctl lexicon ingest`. Provenance, source, retrieval
date, and the ID scheme are recorded in `factory/data/README.md`. The `cmd/hskingest` tool
regenerates the TSVs from source. The fixture lexicon remains the tested lexicon (CI stays
hermetic); the real lexicon ships as its own `lexicon_version`.

---

## B-2 — Live LLM spike — ✅ RESOLVED (2026-07-06); outcome = **ADJUST**

The DeepSeek key (`~/.deepseek-api-key`, read by `LLMConfigFromEnv`) was used to run the live
MC-2 spike against the real HSK lexicon. Results and the go/adjust decision are in
`plans/mc-2-spike-report.md`. **Headline: with naive prompting + a max-4 repair loop, DeepSeek
does not yet produce stories that pass the I1 coverage gate at any tested band (0% pass).** Per
MC-2.6 (discard ≫ 20%), the decision is **ADJUST — do NOT scale CP-09 content on this approach,
and do NOT loosen the gate budgets (I1).** Concrete CP-09 recommendations are in the report.

No key is committed (I2); the LLM path stays behind `--live` and is never exercised in CI.

---

## B-3 — Imageability / concreteness-norm source licensing — ✅ RESOLVED (2026-07-06)

**Resolution (owner instruction): derive imageability from Wikimedia Commons itself** — no external
concreteness-norm dataset, so there is nothing to license. A word's imageability is measured
empirically by whether the §8A pipeline can source a license-clean, on-concept, ≥1200px Commons
photograph for it: the `zhuwenctl images fetch|gate` stages return a candidate count and a best
pick per word, and that **Commons-availability signal is the imageability score** used for Foundations
word ordering (§5A.2). Words that yield no gate-passing image are, by definition, below the
"imageability floor" and are deferred to F1 pattern acquisition or gloss-supported story introduction
later — exactly the §5A.2 rule, now driven by data instead of a purchased norm table. The
`internal/foundations` selection logic consumes `word → (candidates, bestPick)` from the image
pipeline; a small fixture snapshot drives CI hermetically. No new dependency, no licensing exposure.
Validated by the CP-08a curation spike (`factory/cmd/imagespike`, see B-4).

---

## B-4 — Real Commons image curation + per-image license verification — ⏳ AWAITING OWNER REVIEW (2026-07-06)

The §8A pipeline is human-in-the-loop by design (owner is the curator). Best-guess automation is
built and run: **`factory/cmd/imagespike`** queries Wikimedia Commons for a word set, applies the
§8A hard gate (license = PD/CC0/CC-BY/CC-BY-SA only; AI-category exclusion; ≥1200px short side),
ranks candidates by Commons search relevance, auto-picks a best-of-N per word, and writes a
**self-contained HTML review sheet** (no server, no external JS) plus `picks.json`.

**First run: the 11 Foundations F0 seed words (§5A.1).** All 11 produced a license-clean, ≥1200px
best guess with strong alternates; the license/AI/resolution gate and relevance ranking work. The
open judgment is **semantic/cultural fit** — precisely the human step (e.g. `茶`→a Lipton mug and
`米饭`→a Japanese rice bowl are recognizable but culturally off; `水`→an ice-bird photo is off-concept).
The review sheet surfaces alternates so the curator overrides these in seconds.

### How to review (resolves B-4 for this batch)
```sh
cd factory
go run ./cmd/imagespike --out /tmp/opencode/zhuwen-image-review --n 6   # regenerate (hits Commons)
open /tmp/opencode/zhuwen-image-review/f0-review.html                    # macOS
```
In the browser: each word shows the ✓ best guess + alternates + the rejected candidates (with the
gate reason). **Click any card to override the pick**, or choose "reject all / re-query" for a word
with no acceptable option. Click **"Export decisions ▾"** to download `f0-image-decisions.json`, then
hand that file back (or drop it in the repo) — the productionized `images curate` stage will consume
it to lock the per-word image + provenance into packs. Verify each chosen image's license on its
linked Commons page before approving (CC-BY/SA attribution is a legal obligation, FR-11.2).

The full launch inventory (~570 images, §8A.1) is the same workflow at scale (owner runs the TUI /
review sheet per batch). CI never takes the network path (Commons is anonymous, no secret; I2).

### CP-09c status: scaling to ~570 (the schedule long pole)
F0 (11 words) is reviewed; `f0-image-decisions.json` is committed. CP-09c Part D
(`plans/cp-09c-plan.md`) drives the remaining inventory — the 81 canon covers + A1–A2 authored spine +
B1 packs — batch-by-batch through the same `imagespike → decisions → images curate` loop, tracked in
`plans/cp-09c-image-inventory.md`. The **per-image license sign-off is now machine-recorded**:
`images.Provenance{SignedOff,SignedBy,SignedAt}` + `images.DecisionsToImagesSignedOff` reject an
unsigned cover, so a story cannot graduate off its fixture stand-in without a human having verified
the license on the Commons page (I4/FR-11.2). The ~570-image batching plan and per-batch status
ladder are in `plans/cp-09c-image-inventory.md`; F0 batches are marked signed-off (curated in
CP-08a). **A canon entry / pack ships only when its cover is signed off** (`pack.validateI6` blocks
imageless/AI/unprovenanced covers). This blocker must be **closed for the launch pack set** before
CP-10 submission (`plans/cp-10-plan.md` Part 0 precondition).

---

## B-5 — CosyVoice tooling + voice-model licensing — ⏳ OWNER TO RECORD (CP-09c)

The real TTS render (CP-09c Part A, `internal/tts`) runs **CosyVoice 3.0 locally on Apple
Silicon (MPS), build-time only, $0** — not linked into the app (I2/I3), CI uses a deterministic
stub encoder. The render stage is **shipped behind `tts.ModeReal`** (wired into
`pipeline.Config.TTS`); the deterministic stub is the default and keeps CI/dev hermetic. Before
the real stage produces shipped audio, the owner records the **render tool version + voice-model
license** (confirming rendered audio may be redistributed in a paid App Store app, FR-11/§8A
parallel to B-1's HSK sign-off) in `factory/data/README.md` (the provenance table is stubbed
there awaiting the owner's entries).
