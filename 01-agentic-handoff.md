# Zhuwen — Agentic Coding Handoff (01)

**Reads with:** `00-requirements-and-design.md` (v3, FINAL) and `zhuwen-mockups.html`
(M1–M15). Requirement IDs (I*, FR-*, NFR-*, CP-*) refer to that document. Where this
handoff and `00` disagree, `00` wins; file an issue rather than improvising.

---

## 0. How agents must operate on this project

1. **Plan-first.** Each checkpoint begins by writing/updating `plans/cp-XX-plan.md`
   with task breakdown and test list; implementation follows the plan.
2. **Cite requirement IDs** in every commit message (`CP-04: implement CoverageGate (I1, FR-3.1)`).
3. **Invariants are non-negotiable.** If a task appears to require weakening I1–I6,
   stop and write `plans/blockers.md` instead of coding around it.
4. **No new dependencies** without listing them (name, license, why) in the CP plan.
   License floor: Apache-2.0/MIT/BSD/MPL for code; per §8A for content assets.
5. **Tests before merge.** Every acceptance criterion below maps to at least one
   automated test. Golden-file tests are the backbone of the factory.
6. **No telemetry, no accounts, no runtime network** beyond `PackClient` + StoreKit
   (I2). CI greps enforce this (see §8).
7. Do not invent content sources, licenses, or citations. Every canon entry, image,
   and pedagogy claim carries verifiable provenance or it does not exist.

## 1. Repositories & toolchain

| Repo | Contents | Toolchain |
|------|----------|-----------|
| `parso-zhuwen-factory` | Go content factory, canon registry, image pipeline, curation TUI, pack builder | Go 1.23+, Bubble Tea (TUI), SQLite (mattn or modernc), CosyVoice 3.0 + Qwen3-TTS invoked as external stage (Python venv or containerized), forced aligner stage, minisign for pack signing |
| `parso-zhuwen-ios` | SwiftUI app | Xcode 16+, iOS 17 floor, SwiftData, StoreKit 2, no third-party SDKs (NFR-5). Local SPM packages: `ZhuwenCore`, `ZhuwenPacks`, `ZhuwenAudio`, `ZhuwenUI` |

Shared artifacts: fixture packs produced by the factory are vendored into the iOS
repo under `Fixtures/` and treated as golden inputs.

## 2. Invariant → enforcement owner map

| Inv | Factory enforcement | App enforcement | CI |
|-----|--------------------|-----------------|----|
| I1 coverage gate | gate stage refuses stories violating budgets (§4.3) | `StoryCandidate` private init; only `CoverageGate.evaluate()` constructs | unit fixtures incl. 97.9% story that must fail |
| I2 no accounts/server state | n/a | all state SwiftData; network only `PackClient`+StoreKit; optional CloudKit private DB | grep for URLSession outside PackClient; privacy manifest check |
| I3 pregenerated only | everything rendered offline | no generation code paths exist | dependency scan: no LLM/TTS SDKs in app |
| I4 evidence-gated claims | citations table in pack | `PedagogyClaim` private init from registry | DOI validation script |
| I5 every tap teaches | n/a | append-only `Event` log; `WordState` = pure projection | replay test: rebuild WordState from events == stored |
| I6 every story has a Commons image | pack builder hard-fails NULL/AI/unlicensed images | UI assumes cover non-nil (non-optional field) | golden imageless fixture must fail build |

## 3. Pack format (CP-02 deliverable, frozen thereafter)

```
zhuwen-pack-<id>-v<semver>.zpack        # zip container
├── manifest.json     # pack id, semver, lexicon_version, created_at, file hashes
├── manifest.sig      # minisign detached signature (ed25519; pubkey baked into app)
├── content.sqlite    # schema below
├── audio/<story_id>.opus               # 24 kbps mono, loudness-normalized (-16 LUFS)
└── images/<image_id>@{480,1200}.heic
```

`content.sqlite` tables (authoritative DDL to be committed as `schema.sql` in CP-02;
column intent per `00` §9): `story` (incl. `canon_id, tier, origin, source_urls,
pd_rationale, cover_image_id NOT NULL` ← I6 at the schema level), `question`,
`sentence_translation`, `lexicon`, `character`, `image` (full §8A provenance columns,
all NOT NULL), `foundations_card`, `citation` (I4 registry slice), `alignment`
(story_id, token_idx, t0_ms, t1_ms).

Rules: packs are immutable; fixes ship as new semver. `lexicon_version` pins word IDs;
the app refuses packs whose lexicon_version it doesn't know. App verifies
`manifest.sig` before install and rejects tampered/unsigned packs.

## 4. Factory pipeline (Go) — stage contracts

Layout: `cmd/zhuwenctl` (CLI + TUI entry), `internal/{lexicon,canon,brief,gen,segment,
gate,repair,questions,translate,images,tts,align,qa,pack}`. Stages communicate via
SQLite work queue (parso-pdaudio pattern); every stage is resumable and idempotent.

### 4.1 Lexicon ingest
HSK-3.0 word lists + frequency ranks + character decomposition + imageability scores
→ `lexicon.sqlite` with stable integer word IDs. **Word IDs are forever**; levels and
ranks are attributes. Custom segmenter dictionary is generated from this list.

### 4.2 Canon → brief → retell
- Canon registry: YAML/SQLite entries `{canon_id, tier, title_zh, title_en,
  source_urls[] (revision permalinks), pd_rationale (required, FR-12.2), beats[],
  characters[], cultural_notes[], chengyu?}`. Seed 10 entries at CP-01 (§6A.1 lists).
- Brief stage compiles a beat sheet + target lexicon slice (allowed word-ID set =
  band lexicon; frontier candidates marked) + length band + register.
- Retell stage prompts the generation LLM (Chinese-strong; DeepSeek/Qwen via OpenCode
  per house pattern): "retell these beats using only these words; new-word budget
  ≤8 types, each ≥3 occurrences." Temperature low; N candidates per brief.

### 4.3 Coverage gate (I1) — reference algorithm
```
segment(text) -> tokens[] (word_id | literal | proper_noun)
types  = distinct word_ids excluding proper nouns
known  = target band lexicon slice
new    = types - known
FAIL if |new| > 8                                (type budget)
FAIL if token_count(new) / token_count(all) > 0.02   (token budget)
FAIL if any w in new: occurrences(w) < 3         (recurrence)
FAIL if any w in new not in frontier_candidates  (frontier discipline)
FAIL if grammar_patterns(text) ⊄ band_whitelist  (grammar gate)
FAIL if proper_noun without first-occurrence gloss
PASS -> story record with coverage_bitmap, new_type_ids
```
Segmentation: jieba/pkuseg-compatible with the custom lexicon dictionary loaded and
**frozen per lexicon_version**; gate operates only on factory segmentation (risk §13).

### 4.4 Repair loop
Failed candidates get a targeted rewrite prompt naming the exact violations
("replace 摊贩 — out of band; 新鲜 appears once, needs ≥3"). Max 4 iterations, then
discard the brief attempt and log. Track pass-rate per tier; canon retelling is
expected to converge in ≤2 iterations for most briefs.

### 4.5 Questions, translations, fidelity
3 MC questions/story (distractor rules: plausible, in-band, single-key), sentence
translations, and the fidelity pass verifying registered beats (§6A.3). All gated;
question text itself must pass the same lexical band check.

### 4.6 Image pipeline (I6, §8A)
`zhuwenctl images fetch` (Commons/Wikidata P18 query + extmetadata) →
`images gate` (license/resolution/OCR/AI-category exclusion) →
`images curate` (Bubble Tea pick-of-N TUI) → `images process` (crops, HEIC) →
join. Builder refuses any story lacking a curated image (I6). Topic pool + reuse
rules per §8A.1.

### 4.7 TTS + alignment
CosyVoice 3.0 primary, Qwen3-TTS alternate behind a config flag (§7.1). Input =
sandhi-applied pinyin cross-checked against lexicon; output Opus + forced alignment →
`alignment` rows; anomaly detection flags outlier word durations for audit.

### 4.8 QA & audit
`zhuwenctl audit sample --pack A2 --rate 0.1` produces an audit worksheet (TUI):
human reads/listens, flags naturalness, tone errors, cultural issues. A pack ships
only when audit pass-rate ≥ threshold recorded in the pack manifest.

## 5. iOS app — module contracts

- **ZhuwenPacks:** verify signature → attach read-only SQLite → typed queries.
  Owns the only URLSession in the product (I2).
- **ZhuwenCore:** `LexiconStore`, `EventLog` (append-only), `KnownWordModel`
  (projection; replayable), `FrontierQueue`, `CoverageGate` + `StoryCandidate`
  (private init, I1), `Selector` (bitmap AND + popcount scoring per FR-3.2; <50 ms
  over 5k stories, NFR-2), `FSRS` scheduler, `PlacementEstimator` (logistic fit over
  frequency rank; conservative seed per risk table).
- **ZhuwenAudio:** pack playback (word-synced from `alignment`), AVSpeechSynthesizer
  layers 2–3 with enhanced-voice detection (§7), AVAudioSession/NowPlaying (reuse
  Lorewave patterns).
- **ZhuwenUI:** screens M1–M15. Songti story text; cinnabar only for meaning;
  seal-stamp animation with Reduce Motion fade (NFR-6); Liquid-Glass policy per §10.

SwiftData models per `00` §9. `StoryProgress.sealEarned` drives Today's week log.
Free/Pro gating per FR-9 via StoreKit 2 entitlement checks only.

## 6. Checkpoints & acceptance criteria

Each CP ends with: plan updated, tests green, demo note in `plans/cp-XX-done.md`.

- **CP-01 Factory skeleton.** 10-entry canon registry; 20 A2 stories retold, gated,
  packed into `fixture-a2-v0.zpack` (stub audio, stub images allowed ONLY at this CP,
  flagged `fixture:true`). ✅ Gate unit tests incl. must-fail fixtures; pipeline
  resumable after kill -9 mid-stage.
- **CP-02 Pack format freeze.** `schema.sql`, manifest+minisign, golden packs incl.
  (a) unsigned, (b) tampered, (c) imageless (I6) — all three must be rejected by the
  reference verifier. ✅ Format doc committed; iOS Fixtures/ vendored.
- **CP-03 App skeleton.** Tabs, PackStore verifying signatures, Reader renders a
  fixture story with tap-to-gloss from pack data. ✅ Launch <600 ms (NFR-1) measured.
- **CP-04 Model + Selector.** EventLog/KnownWordModel/FrontierQueue/CoverageGate/
  Selector. ✅ Replay test (I5); gate property tests (I1); selector benchmark (NFR-2).
- **CP-05 Placement.** M1–M3 flow; pseudoword foils; logistic seeding; re-placement
  merge (FR-1.5). ✅ Simulated learners at known curves recover level ±1 HSK band.
- **CP-06 Listening.** Pack audio karaoke via alignment; speeds; blind mode; system-TTS
  fallback labeled. ✅ Highlight drift <120 ms across a 3-min story on device.
- **CP-07 Loop completion.** Comprehension→seal (M8), FSRS review (M9), Progress (M10)
  with both-skill estimates. ✅ P(known) updates verified for exposure/lookup/grade paths.
- **CP-08 Commerce & data.** CDN PackClient, StoreKit 2 SKUs (FR-9.3), paywall (M12),
  settings, export/erase, optional CloudKit sync. ✅ Network-surface CI grep green;
  export→erase→import round-trips.
- **CP-08a Images + Foundations.** §8A pipeline end-to-end with curation TUI;
  Foundations F0–F3 (FR-11, M14) on curated inventory; handoff threshold logic.
  ✅ I6 builder tests; a scripted zero-knowledge run reaches F3 in ~300 words.
- **CP-09 Content scale-up.** Canon to ~200 entries; A1–B1 packs; CosyVoice render +
  Qwen A/B; audit workflow; license re-verification memo. ✅ Audit pass-rates recorded
  in manifests; pack sizes within NFR-3/4.
- **CP-10 Ship polish.** Accessibility pass (Dynamic Type XXL, VoiceOver), App Store
  assets, HSK-3.0 launch messaging, methodology page with citations (I4).
  ✅ Full-device QA checklist in `plans/cp-10-qa.md`.

## 7. Testing strategy summary

Factory: golden-file pipeline tests (input brief → expected gated output), gate
property tests (random lexicons/stories), license-gate fixtures (NC/ND/AI-category
images must be rejected), pack verifier negative suite. App: unit (model/gate/
selector/FSRS), snapshot tests for M1–M15 states, replay determinism, StoreKit
sandbox flows. One shared invariant suite runs the same gate vectors through the Go
and Swift implementations to prevent drift.

## 8. CI gates (both repos)

lint/format; license scan of deps; `grep-audit.sh` — app repo: URLSession outside
ZhuwenPacks, analytics SDK names, "http://", localStorage-of-secrets patterns;
factory repo: image records missing provenance columns; DOI validation (I4);
pack-builder I6 test; benchmark regression (NFR-1/2) on device farm or local runner.

## 9. Explicitly out of scope for v1 (do not build)

Speaking/writing/tone-scoring; Android/iPad layouts/watch; runtime generation of any
kind; accounts, social, streaks, notifications beyond an optional daily local
reminder; traditional characters; B2+ content; school/B2B features.

## 10. Open items (owner: J)

1. Record/choose the two voice reference prompts for CosyVoice ("Xiaoyu", "Laohu").
2. App Store Connect name check + trademark knockout for "Zhuwen"; register domains.
3. Choose CDN (existing IONOS+Cloudflare stack presumed) and minisign key custody.
4. Decide launch pack scope: A1+A2 at launch, B1 fast-follow (recommended) vs all three.
5. Curation labor scheduling (~5 evenings for the §8A.1 image inventory + audit passes).
