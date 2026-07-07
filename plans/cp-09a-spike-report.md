# CP-09a — Constrained-generation de-risk spike report

**Status: RUN COMPLETE (real hsk3.0-v1 lexicon + live DeepSeek). Decision = PARTIAL GO / ADJUST.**

This is the go/no-go gate for the Zhuwen content factory (`plans/cp-09-plan.md` Part A, review at
`plans/review-the-current-state-agile-crystal.md`). It answers the one question MC-2 left open:
**can constrained generation hold the I1 98%-coverage gate economically, without loosening it?**
No metrics here are fabricated (I4); the gate budgets (I1) were not touched; the app network
surface is unchanged (I2 — all generation is factory-side behind `--live`).

## Headline

- **B1 (known HSK≤4, frontier 5): GO.** Candidate-rerank on DeepSeek + token-level
  name-and-replace repair **clears I1** — ≥1 story passes the real gate. Cost is low in absolute
  terms (~$0.07 per accepted story) but the accept rate is ~1/3, so cost is discard-dominated.
- **A2 (known HSK≤2, frontier 3): NO-GO for LLM generation.** Candidate-rerank does **not** clear
  I1 at A2 in any configuration tried (0 passes across every run). This is **structural**, not a
  tuning gap: a 638-word known set cannot express the canon fables at 98% coverage. **Recommend the
  documented contingency: hand-author / heavily template A1–A2 through the same gate (CP-09b), and
  lean on DeepSeek rerank for B1+.** Do not scale A2 via the LLM.
- The pipeline itself is proven end-to-end: the **deterministic constrained fixture passes I1 at
  both A2 and B1** hermetically in `make ci` (I2), so the gate accepts cooperative A2 input — i.e.
  hand-authored A2 will pass the identical gate.

## Setup

- **Lexicon:** `hsk3.0-v1` — 12,283 written forms, exact per-level mapping (`factory/data/hsk3.0`).
- **Model:** `deepseek-chat` (OpenAI-compatible), temperature 0.9 (diversity for rerank).
  DeepSeek exposes **no** `logit_bias`/grammar constraint, so the committed approach is
  candidate-rerank (per the review's owner decision), not logit masking.
- **Pipeline (CP-09a):** canon → brief (now threading per-story **proper-noun glosses**) → gen
  (`gen.ConstrainedProvider`: oversample N candidates, segment + `gate.Evaluate` each, keep the
  passing one; hard per-story token ceiling) → FMM segment → I1 gate → **token-level
  name-and-replace** repair loop (names each specific out-of-band word / under-recurring type and
  supplies in-band substitutes drawn from the known set), max 4 iterations, non-convergence
  discards.
- **Bands:** A2 (known HSK 1–2 = 638 words, frontier HSK 3 = 640) and B1 (known HSK 1–4 = 2,487,
  frontier HSK 5 = 1,737). Length target 500–900 chars (headroom for the 2% budget).

## Results (live)

### Constrained (candidate-rerank + name-and-replace + proper nouns)

| Band | Run (N=oversample) | pass | discarded | mean cand/story | mean repair iters (passed) | total tokens (prompt+completion) |
|------|--------------------|------|-----------|-----------------|----------------------------|----------------------------------|
| A2   | n=3, N=6           | 0/3  | 3/3       | 30.0            | —                          | 169,138                          |
| A2   | n=2, N=4           | 0/2  | 2/2       | 20.0            | —                          | 64,516 (49,660 + 14,856)         |
| B1   | n=2, N=6           | **1/2** | 1/2    | 23.0            | 2.0                        | 63,594                           |
| B1   | n=3, N=6           | **1/3** | 2/3    | 28.3            | 4.0                        | 126,921 (83,391 + 43,530)        |
| B1   | n=3, N=4           | 0/3  | 3/3       | 20.0            | —                          | 85,301 (56,956 + 28,345)         |

### Naive single-candidate baseline (same upgraded harness — reproduces MC-2)

| Band | Run  | pass | tokens | note |
|------|------|------|--------|------|
| A2   | n=3  | 0/3  | 28,276 | reproduces MC-2 ADJUST |
| B1   | n=3  | 0/3  | 22,758 | reproduces MC-2 ADJUST |

**Lift from constrained rerank:** naive = 0 at both bands (as MC-2). Rerank + name-and-replace
converts **B1 from 0 → passing**; A2 stays 0.

### Name-and-replace convergence (the mechanism working)

The B1 pass (`C1-bamiaozhuzhang`, n=3 N=6) shows the token-level repair pulling a draft into the
whitelist over iterations — per-iteration **NewTokens** (over-budget count) trace:

```
[54, 6, 4, 3, 5]   → passed at iter 4 (5 new tokens ≈ 1.7% of ~300, ≤ 2%)
```

The first draft was 54 new tokens over; naming each specific offender + supplying in-band
substitutes converged it to a pass. A2 traces plateau far from the budget, e.g.:

```
A2 C1-shouzhudaitu: [62, 45, 45, 44, 42]   (budget ≈ 5–6; never approaches it)
A2 C1-bamiaozhuzhang: [94, 47, 40, 66, 72]
```

### Proper-noun harness gap (MC-2 rec #4) — closed

MC-2 reported `literal_out_of_lexicon` at **10/25**. With per-story proper-noun glosses threaded
brief→gen→segment, it is now **0–1** across all runs (declared names segment as `properNoun` and
are excluded from the coverage denominator). The MC-2 harness gap is closed.

## Economics — $ per accepted story

DeepSeek-chat token usage is measured (above). Priced against DeepSeek's published standard rate
(**assumed** input $0.27 / 1M, output $1.10 / 1M; verify against current pricing):

- **B1, n=3 N=6:** 83,391 prompt + 43,530 completion = **$0.0225 + $0.0479 ≈ $0.070** to yield
  **1 accepted story** → **~$0.07 per accepted story** at a ~33% accept rate.
- Cost is **discard-dominated** (2 of 3 briefs burned tokens without passing). Absolute cost is
  low: even at 33% accept, a few-hundred-story B1+ catalog is on the order of **$20–50** in tokens.
- **A2: not applicable — 0 accepted, so cost per accepted is unbounded.** No amount of
  oversampling closes A2's ~16-point coverage gap (systematic, not variance).

**Verdict on economic viability:** candidate-rerank is **economically viable at B1+** (cheap in
absolute terms; the lever to improve is accept rate via budget-aware briefs, CP-09b). It is **not
viable at A2 at any price** because it does not converge at all.

## What was tried and rejected (honest negatives)

- **Listing the full 638-word A2 known-word palette in the prompt** (strongest prompt-only
  constraint short of logit masking): tripled prompt cost (153k tokens for 2 stories) and still
  produced **0/2** — the model cannot restrict itself to the palette because the fable genuinely
  needs over-level words. Reverted.
- **Raising oversample / length:** helps B1 (accept rate rises with N; N=4→0/3, N=6→1/3) but does
  **not** move A2 off 0.

## Go/adjust decision — **PARTIAL GO (B1+), ADJUST (A2)**

Consistent with the review's "stop/branch here" gate and the design's own risk-table stance
("A1 relies more on scripted Foundations"):

1. **Proceed with DeepSeek candidate-rerank for B1 and above** (CP-09b scale). The remaining lever
   is **budget-aware briefs** (coverage contract: target length + explicit plan of ≤`maxNewTypes`
   frontier words each recurring ≥`minRecurrence`) to raise the accept rate and cut discard cost.
2. **Do NOT generate A1–A2 with the LLM.** Hand-author / heavily template A1–A2 (plus the CP-08a
   Foundations F2 micro-stories) through the **same** I1 gate — no gate fork. The hermetic fixture
   already demonstrates the gate accepts cooperative A2/B1 input.
3. **The local GBNF-constrained Qwen2.5 contingency remains documented but unbuilt** — it is not
   needed for B1 (rerank works) and would not help A2's semantic-coverage wall either; A2 is a
   content-authoring problem, not a decoding-constraint problem.
4. **I1 was never loosened.** Non-convergent stories discard; the shared Go/Swift gate-vector suite
   stays green.

## Reproduce

```sh
go run ./cmd/zhuwenctl lexicon ingest --src data/hsk3.0 --out /tmp/lexicon.sqlite --version hsk3.0-v1

# B1 — clears I1 (rerun; pass is probabilistic in oversample breadth, use N>=6):
ZHUWEN_LLM_API_KEY=$(cat ~/.deepseek-api-key) \
  go run ./cmd/zhuwenctl spike --live --lexicon /tmp/lexicon.sqlite \
    --known-max 4 --frontier-level 5 --n 3 --oversample 6 --verbose

# A2 — does not clear I1 (structural):
ZHUWEN_LLM_API_KEY=$(cat ~/.deepseek-api-key) \
  go run ./cmd/zhuwenctl spike --live --lexicon /tmp/lexicon.sqlite \
    --known-max 2 --frontier-level 3 --n 3 --oversample 6 --verbose

# Naive baseline (MC-2 reproduction): add --naive.
# Hermetic (no network, CI): the constrained fixture passes A2 AND B1 —
#   go test ./internal/spike -run TestConstrainedFixturePassesA2AndB1OnRealLexicon
```

CI never takes the live path (no key on the runner); the constrained **fixture** provider is the
hermetic stand-in and reproduces gate-passing stories at A2 and B1 in `make ci`.
