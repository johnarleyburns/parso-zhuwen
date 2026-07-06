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
