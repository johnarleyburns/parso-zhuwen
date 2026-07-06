# MC-2 — Content-reality spike report

**Status: RUN COMPLETE (real HSK lexicon + live DeepSeek). Decision = ADJUST.**

Both MC-2 blockers cleared: the real HSK-3.0 lexicon is ingested (`hsk3.0-v1`, 12,283 forms,
B-1) and the live DeepSeek key was used (B-2). This is the real content-bet measurement, not a
fixture harness — no metrics are fabricated, and the gate budgets (I1) were not touched.

## Setup
- **Lexicon:** `hsk3.0-v1` — 12,283 written forms mapped exactly to the official per-level
  standard (vocabulary + recognition characters); see `factory/data/README.md`.
- **Model:** `deepseek-chat` (OpenAI-compatible), temperature 0.3, N=1/iteration, max 4 repairs.
- **Pipeline:** canon (5 entries) → brief → gen → FMM segment → I1 gate → repair loop, with the
  repair loop feeding the prior candidate + a violation-naming rewrite prompt back to the model.
- **Bands tried:** A2 (known HSK 1–2 = 638 words, frontier HSK 3) and B1 (known HSK 1–4 = 1942,
  frontier HSK 5).

## Results (live)
| Band | pass@0 | passed | discarded | mean repair iters | tokens |
|------|--------|--------|-----------|-------------------|--------|
| A2 (known ≤2, frontier 3) | 0/5 | 0/5 | **5/5 (100%)** | — | ~58k |
| B1 (known ≤4, frontier 5) | 0/5 | 0/5 | **5/5 (100%)** | — | ~108k |

Failure-code histogram (A2, 25 attempts = 5 briefs × up to 5 iterations):

```
frontier                 25   (used words outside the frontier HSK level)
recurrence               25   (new words appear <3×)
token_budget             25   (>2% of tokens are new)
type_budget              25   (>8 new word types)
literal_out_of_lexicon   10   (proper nouns/names — harness gap, see below)
```

At B1 `type_budget` eased (bigger known set) but `token_budget`/`recurrence`/`frontier` still
fail 25/25. The repair loop, even with real feedback, does not converge within 4 iterations.

## What this means (honest read)
The core content bet — a general LLM, prompted with a brief, restricting itself to a fixed
band vocabulary tightly enough to pass the I1 98%-coverage gate — **does not hold with naive
prompting.** The gate is not broken: the fixture-band harness passes 100%, and a cooperative
generator would pass. The model simply writes natural Chinese that overshoots the budgets, and
prose-level repair instructions are insufficient to pull a whole draft into a ~600–2000-word
whitelist. This is precisely the risk MC-2 existed to surface **before** paying for CP-09 scale.

## Go/adjust decision — **ADJUST** (do not loosen I1)
CP-09 must NOT proceed as "prompt + retry." Recommended changes, in priority order:

1. **Vocabulary-constrained generation**, not prompt-only: constrain decoding to the
   known+frontier token set (logit masking / grammar-constrained decoding), or use a
   band-vocabulary-aware model. Prompting alone cannot hold the whitelist.
2. **Smarter repair**: name each *specific* out-of-band word and supply an in-band synonym to
   substitute (the loop currently names counts/types; it should name and replace tokens).
3. **Brief redesign for the budget math**: at 400 tokens the 2% budget affords ≈2 new types at
   3× each — so briefs must target longer texts and/or explicitly plan ≤2 frontier words with
   heavy repetition, rather than free retelling.
4. **Proper-noun handling** (harness gap): the spike segments with no proper-noun dictionary, so
   names become `literal_out_of_lexicon` (10/25). CP-09's pipeline must pass per-story
   proper-noun glosses (the segmenter already supports them) so names are excluded from coverage.
5. **Consider band strategy**: hand-author or heavily curate the lowest bands (A1–A2), where the
   known vocabulary is too small for free generation, and lean on the LLM at B1+.

## Harness improvements delivered by this spike
- Complete real lexicon (vocabulary + recognition characters), exact level mapping (B-1).
- Grammar whitelist aligned to the detector's standard patterns (了/的/不/在/过/吗/把/被).
- Repair loop now actually feeds failures back to the model (`gen.RepairProvider`).
- `zhuwenctl spike --live --lexicon … --known-max … --frontier-level … [--verbose]`.

## Reproduce
```sh
go run ./cmd/zhuwenctl lexicon ingest --src data/hsk3.0 --out /tmp/lexicon.sqlite --version hsk3.0-v1
ZHUWEN_LLM_API_KEY=$(cat ~/.deepseek-api-key) \
  go run ./cmd/zhuwenctl spike --live --lexicon /tmp/lexicon.sqlite --known-max 2 --frontier-level 3 --n 5 --verbose
```
CI never takes this path (no key on the runner); the fixture harness remains the hermetic test.
