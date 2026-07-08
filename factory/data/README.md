# factory/data — external data sources & provenance

License-cleared source data used at build time. Per handoff §0.7, nothing here is committed
unless its redistribution terms are clear.

## HSK 3.0 vocabulary (`hsk3.0/level-*.tsv`) — SHIPPED

The real HSK-3.0 lexicon, mapped **exactly to the official per-level standard**, drives
`lexicon.sqlite` via `zhuwenctl lexicon ingest`.

### Source & licensing
- **Standard:** 《国际中文教育中文水平等级标准》(HSK 3.0 / CLPS, Ministry of Education / CLEC,
  2021), the official vocabulary published per level at
  `https://www.hanzistroke.com/hsk/3.0/level-{1..6,7-9}`.
- **Retrieved:** 2026-07-06.
- **License:** redistribution as part of a language-learning application has been
  **independently verified and authorized by the project owner** (resolves `plans/blockers.md`
  B-1). Only the derived `(id, simplified, pinyin, hsk3_level, freq_rank)` mapping is committed —
  no site markup, definitions, or other prose.

### Contents (12,283 unique written forms)
Both official dimensions are included so the lexicon covers real text completely:
vocabulary words (词汇) and recognition characters (认读字).

| file | HSK level | words |
|------|-----------|-------|
| `level-1.tsv` | 1 | 409 |
| `level-2.tsv` | 2 | 229 |
| `level-3.tsv` | 3 | 640 |
| `level-4.tsv` | 4 | 1209 |
| `level-5.tsv` | 5 | 1737 |
| `level-6.tsv` | 6 | 1931 |
| `level-7-9.tsv` | 7 (advanced band 7–9, not split lexically in the standard) | 6128 |

### ID scheme ("word IDs are forever", §4.1)
- Vocabulary words/characters keep their **official global index** (1 … ~11000) as the word ID.
- Recognition-only characters (not listed as standalone vocabulary) get IDs in a disjoint
  `20000+` range, assigned deterministically in (level, source) order.
- Homographs (same written form, multiple readings/levels) collapse to the **lowest-level**
  occurrence — one stable entry per written form, as the segmenter requires.
- `freq_rank` uses the official global ordering (≈ level/frequency order); an independent
  corpus-frequency source can refine it later without changing IDs.

### Regenerating
```sh
go run ./cmd/hskingest --fetch --out data/hsk3.0        # download + rebuild
# or, from already-downloaded pages:
go run ./cmd/hskingest --src <dir-of-hsk-lN.html> --out data/hsk3.0
go run ./cmd/zhuwenctl lexicon ingest --src data/hsk3.0 --out lexicon.sqlite --version hsk3.0-v1
```

## Ingest input format (for any future list)
Tab-separated, `#` comments/blank lines ignored:
```
# id  simp  pinyin  hsk3_level  freq_rank  [char_ids]
1     爱    ài      1           1
```
`id` and `simp` must be unique and stable across versions. If a future list's licensing is
unclear, **stop** and file/append to `plans/blockers.md` rather than committing it (§0.7).

## Tests still use the fixture lexicon
The pipeline's automated tests run against the vendored fixture lexicon
(`internal/assets/lexicon.tsv`, `fixture-hsk3.0-v0`), so the factory builds and tests with no
external data present. The real `hsk3.0-v1` lexicon ships as a distinct `lexicon_version`.

## Authored stories (`authored/a1-spine.json`)
Hand-authored A1 backbone stories (CP-09b Part B). These are operator-written stories that
pass through the same I1 gate as LLM-generated content. Each story carries:
- `canon_id` — links to a canon registry entry
- `band`/`register` — target proficiency level
- `text` — the Chinese story body

Gate check: `zhuwenctl authored check --file data/authored/a1-spine.json`.

## License re-verification
Every canon source's PD status and every shipped image license is re-verified in
`plans/cp-09-license-memo.md`. The canon registry (`internal/assets/canon.seed.json`) carries
a `pd_rationale` field for every entry (FR-12.2). Image licensing is gated through the §8A
Commons pipeline (`internal/images`). See the license memo for per-source PD rationale and
per-image license attribution records.

## CosyVoice TTS render (CP-09c, `internal/tts`) — BUILD-TIME ONLY

Story narration audio + word-level alignment are produced at build time by `internal/tts`.
The stage is **never linked into the app** (I2/I3): audio + timings are pregenerated and
shipped in the `.zpack`; the app only plays and highlights.

### Two modes
- **Stub (default, hermetic):** deterministic, no network, no venv — the CI/dev path. Timings
  come from the `internal/align` character-rate model; audio bytes are a deterministic
  keystream. This is what `make ci` and `make fixtures` use.
- **Real (`tts.ModeReal`):** shells out to a local **CosyVoice 3.0** render + forced aligner
  on Apple Silicon (MPS), build-time only, **$0** (no hosted API). Emits **24 kbps mono Opus**
  (the NFR-4 budget basis) and real word timings that replace the stub under the identical
  `pack.AlignToken` contract.

```sh
# Real render (owner machine; requires the CosyVoice venv + a licensed voice model — B-5):
#   the pipeline drives it via pipeline.Config.TTS = tts.Config{Mode: tts.ModeReal,
#   PythonBin: "./venv/bin/python", Script: "scripts/cosyvoice_render.py", ...}
# The script reads a JSON job (tokens + sentence indices) and emits {audio_path, duration_ms,
# timings[]} on stdout; timings must satisfy the AlignToken contract (tts.ValidateTimings).
```

### Voice / model provenance (record before the real stage ships — blockers.md B-5)
| field | value |
|-------|-------|
| render tool | CosyVoice 3.0 (local, Apple Silicon MPS) |
| model id | _(owner to record)_ |
| voice id | _(owner to record)_ |
| sample rate / bitrate | 24000 Hz mono / 24 kbps Opus |
| voice-model license | _(owner to verify: rendered audio redistributable in a paid App Store app, parallel to B-1's HSK sign-off)_ |

### Size figures (NFR-3/NFR-4, `zhuwenctl budget`)
Sizes are **asserted, not assumed** (I4): `pack.MeasureBudget` computes the download (NFR-3
≤ 90 MB) and on-disk (NFR-4 ≤ 250 MB) figures from the real file set, and `Build` fails on a
breach. 24 kbps mono Opus ≈ 180 KB/min, so a ~3-minute story ≈ 540 KB of audio. Run
`./bin/zhuwenctl budget` for the current fixture-pack figures.

