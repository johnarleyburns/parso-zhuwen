# Blockers

Open items that require a human decision, a secret, or license-cleared data that an agent
cannot supply without violating an invariant (handoff §0.7). Work around these by using the
fixture data; do not fabricate the real inputs.

---

## B-1 — HSK-3.0 word list is not license-cleared for redistribution (MC-2.1)

**Status:** OPEN — owner action required.

**What.** MC-2.1 calls for ingesting the real HSK-3.0 vocabulary lists (~11k words) into
`lexicon.sqlite`. The `zhuwenctl lexicon ingest` command and its input format are implemented
and tested, and `factory/data/README.md` documents the expected source layout. **The raw HSK
lists themselves are not committed**, because the redistribution terms of the official
《国际中文教育中文水平等级标准》(HSK 3.0 / CLPS, MoE / CLEC, 2021) list are unclear — it is a
government-published standard without an explicit open-content licence.

**Why this is a blocker, not a workaround.** Handoff §0.7 forbids shipping content whose
licensing is unclear; §0.4's license floor (Apache-2.0/MIT/BSD/MPL for code, per §8A for
content) does not obviously cover this list. Shipping it would create the exact App
Store/relicensing friction MC-0 just removed.

**Owner action.** Provide one of:
1. A written confirmation that the specific list file(s) may be redistributed under the
   project licence (or a compatible content licence), plus the file(s); or
2. A permissively-licensed equivalent frequency/level list (e.g. a CC-BY corpus-derived list)
   to ingest instead; or
3. A decision to keep the real list *out of the repo* and have operators supply it locally at
   build time (the ingest command already supports `--src <dir>`), shipping only derived,
   non-copyrightable artifacts (stable integer IDs + attributes) if that is cleared.

**Until then.** The fixture lexicon (`internal/assets/lexicon.tsv`,
`fixture-hsk3.0-v0`, 32 words) remains the tested lexicon for the whole pipeline; the real
`lexicon_version` ships as a new version once cleared (the pack format already supports it).

---

## B-2 — Live LLM run for the MC-2 content spike needs an API key (MC-2.3/2.5)

**Status:** OPEN — owner action required.

**What.** The MC-2 spike's core question — can a real Chinese-strong LLM retell canon beats
inside a real band lexicon at acceptable pass-rates — requires a live model call. The
`LLMProvider` (OpenAI-compatible; DeepSeek per house pattern) and the repair loop are
implemented and hermetically tested (prompt construction + response parsing), but the actual
generation is gated behind an explicit `--live` flag and reads its key from the environment
(`ZHUWEN_LLM_API_KEY`); **no key is committed** (I2), and CI never takes the network path.

**Owner action.** Run the spike locally with a key set, or provide a key/endpoint the
maintainer can use, then fill in the live metrics in `plans/mc-2-spike-report.md`
(pass rate @ iter 0, mean repair iterations, discard rate, token cost/story, failure-code
histogram) and record the go/adjust decision. **Do not loosen gate budgets to hit the
numbers (I1).**

**Until then.** `plans/mc-2-spike-report.md` records the harness-readiness result and the
mechanics validated with the deterministic fixture provider; the live-LLM section is marked
BLOCKED on this item.
