# MC-2 — Content-reality spike report

**Status: HARNESS READY · live-LLM validation BLOCKED on `plans/blockers.md` B-1 + B-2.**

This report covers what MC-2 could establish without the two owner-supplied inputs (a
license-cleared HSK-3.0 lexicon and a live LLM API key). The content bet itself — real LLM
pass-rates inside a real band lexicon — requires those inputs and is **not fabricated here**
(handoff §0.7; MC-2.6 forbids loosening gate budgets to hit numbers).

## What shipped (all hermetic, in CI)
| Piece | Command / API | Test |
|-------|---------------|------|
| Lexicon ingest → `lexicon.sqlite` (§4.1) | `zhuwenctl lexicon ingest --src <dir> --out … --version …` | `lexicon.WriteSQLite/ReadSQLite` round-trip |
| Segmenter coverage/ambiguity eval (§4.3) | `zhuwenctl segment eval` | `segment.Eval` coverage + FMM hotspot tests |
| Real LLM retell client (§4.2, OpenAI-compatible/DeepSeek) | `gen.LLMProvider` (network behind `--live`) | prompt-construction golden + response-parse + no-key refusal |
| Repair loop (§4.4) | `internal/repair` | prompt-names-every-violation + convergence + discard-after-4 |
| Spike harness | `zhuwenctl spike [--n k] [--live]` | fixture harness all-pass + summary math |

The LLM path never runs in CI: `spike --live` errors immediately when `ZHUWEN_LLM_API_KEY`
is unset, and `gen.Retell` refuses to touch the wire without a key.

## Pipeline mechanics — validated with the deterministic fixture provider
`zhuwenctl spike --n 5` (canon → brief → gen → segment → gate → repair):

```
entries=5  pass@0=5 (100%)  passed=5  discarded=0 (0%)  mean-repair-iters=0.00  tokens=0
```

This confirms the harness wires all six stages correctly end-to-end and the aggregation is
sound. It is **not** a content-quality result: the fixture provider is built to pass the gate
by construction, so a 100% first-try rate here says nothing about a real LLM. The repair loop's
real behaviour is proven separately by unit tests (`repair.TestRepairLoopConverges`,
`…DiscardsAfterMax`) that drive a scripted failing→passing provider through the exact gate.

## Segmenter decision (MC-2.2): keep FMM for now
`zhuwenctl segment eval` over the fixture stories:

```
tokens: 2000 (word 2000, proper 0, literal 0)
distinct in-lexicon types: 11
token coverage: 1.0000   literal (unresolved) rate: 0.0000
ambiguity hotspots: 0
```

The fixture corpus is deliberately clean (single-char known fillers), so it exercises the
*evaluator*, not FMM's weak spots. The evaluator itself is validated on crafted input
(`segment.TestEvalFlagsFMMAmbiguityHotspot`: `研究生命` → FMM greedily takes `研究生` while
`生命` overlaps — the canonical FMM/jieba disagreement). **Decision: FMM stays; the
jieba-parity question is *promoted to CP-09*** and must be re-run via `segment eval` over the
first real LLM corpus (B-1/B-2). If the literal rate or hotspot count is non-trivial there,
CP-09 adds a jieba/pkuseg-compatible segmenter before scaling content. Recorded so §13's
segmentation risk is tracked, not silently closed.

## Live-LLM content validation — BLOCKED
The core spike (5 canon entries × A2 × 3 candidates through a real model) needs:
- a **license-cleared lexicon** so the band is real, not the 32-word fixture (B-1), and
- a **live API key** (B-2).

To run it once the owner supplies both:
```sh
export ZHUWEN_LLM_API_KEY=…        # DeepSeek or any OpenAI-compatible endpoint
export ZHUWEN_LLM_BASE_URL=…       # optional; default https://api.deepseek.com/v1
export ZHUWEN_LLM_MODEL=…          # optional; default deepseek-chat
zhuwenctl spike --live --n 5
```

Then fill in this table and the go/adjust decision (do **not** loosen gate budgets — I1):

| Metric | Target (MC-2.6) | Measured |
|--------|-----------------|----------|
| Pass rate @ iteration 0 | — | _pending_ |
| Mean repair iterations to pass | ≤ 2 | _pending_ |
| Discard rate | ≤ 20% | _pending_ |
| Token cost per shipped story | — | _pending_ |
| Failure-code histogram | — | _pending_ |
| 3 verbatim shipped stories (J's human read) | — | _pending_ |

## Go/adjust decision
**Deferred, harness-ready.** The mechanics, prompt contract, and repair loop are implemented
and tested; the go/adjust call (CP-09 proceeds as planned vs. prompt/brief/budget adjustments)
is made when the live numbers exist. Blockers B-1 and B-2 are filed with the exact owner
actions required.
