# Zhuwen (朱文) — Chinese by Reading

[![factory-ci](https://github.com/johnarleyburns/parso-zhuwen/actions/workflows/factory-ci.yml/badge.svg)](https://github.com/johnarleyburns/parso-zhuwen/actions/workflows/factory-ci.yml)
[![CI](https://github.com/johnarleyburns/parso-zhuwen/actions/workflows/ci.yml/badge.svg)](https://github.com/johnarleyburns/parso-zhuwen/actions/workflows/ci.yml)

Provably-comprehensible Mandarin reading & listening app. Every story surfaced to a
learner is ≥98% words they already know; the ≤2% new words are deliberately chosen
frontier words (Hu & Nation 98% coverage; Krashen i+1). See
[`00-requirements-and-design.md`](00-requirements-and-design.md) and
[`01-agentic-handoff.md`](01-agentic-handoff.md).

## Repository layout
| Path | What | Toolchain |
|------|------|-----------|
| `factory/` | Go content factory: canon registry → briefs → gate (I1) → signed `.zpack` | Go 1.23+ |
| `ios/` | iOS app + SPM packages (ZhuwenCore/Packs/Audio/UI/Persistence) + vendored `Fixtures/` + in-repo `@main` app (`App/`, XcodeGen) | Xcode 16+/Swift, iOS 17 |
| `plans/` | Per-checkpoint plans, done notes, and standing exit criteria | — |

## Status — where the app is

| CP | Scope | State |
|----|-------|-------|
| CP-01 | Factory walking skeleton: lexicon → canon (10 seeds) → brief → gen (fixture) → segment → **coverage gate I1** → signed pack; **I6** builder hard-fail | ✅ done |
| CP-02 | Pack format freeze: `schema.sql`, **minisign** signatures, reference verifier (sig + hashes + lexicon_version + content-level I6), golden reject suite, `ios/Fixtures/` vendored | ✅ done |
| CP-03 | App skeleton: `PackStore` verifies signatures (Swift, CryptoKit), Reader renders a fixture story with tap-to-gloss; SwiftUI tab shell compiles for iOS 17 | ✅ done |
| CP-04 | On-device model + selector (`ZhuwenCore`): `EventLog` (append-only, I5), `KnownWordModel` (replayable projection), `FrontierQueue`, `CoverageGate`+`StoryCandidate` (I1), `Selector` (bitmap AND + popcount, NFR-2). Shared Go/Swift gate-vector suite | ✅ done |
| CP-05 | Placement (M1–M3): pseudoword foils, logistic fit over frequency rank → probabilistic **seed** of `KnownWordModel` + CEFR/HSK estimate (FR-1.2), reading-passage refinement (FR-1.3), absolute-beginner → Foundations (FR-1.4), **re-placement merge** (FR-1.5) | ✅ done |
| CP-06 | Listening (M7): factory forced-alignment stage → per-word timings in packs; `ZhuwenAudio` karaoke (position→token resolver), speeds 0.6×–1.2×, blind mode, labeled system-TTS fallback (FR-5). Highlight drift <120 ms over a 3-min story | ✅ done |
| CP-07 | Loop completion: comprehension → **seal** (M8), on-device **FSRS** review (M9, sentence-context, 20/day cap), Progress (M10) with **both-skill** reading+listening CEFR estimates. P(known) updates verified for exposure/lookup/grade paths | ✅ done |
| CP-08 | Commerce & data: CDN **`PackClient`** (anonymous GET, verify-before-install, pack manager), **StoreKit 2** SKUs + `FeatureGate` (FR-9.3), paywall (M12), settings (M13), **export/erase/import** (FR-10.3), opt-in private **CloudKit** sync (off by default). Network-surface CI grep green; export→erase→import round-trips | ✅ done |
 | CP-08a | Images + Foundations: §8A Commons image pipeline (`internal/images`: license/quality gate, fetch, curate, join) + `zhuwenctl images`; Foundations F0–F3 (`internal/foundations`) built on the curated inventory; on-device Foundations engine + M14 UI (`FoundationsProgram`/`Session`/`HandoffGate`, attribution/Credits), first-run onboarding gating, methodology page (I4). I6 extended to `foundations_card`; a zero-knowledge learner reaches the F3 handoff in ~300 words | ✅ done |
 | CP-09a | **Constrained-generation de-risk (go/no-go gate):** `gen.ConstrainedProvider` (candidate-rerank — DeepSeek exposes no logit_bias), token-level **name-and-replace** repair (`internal/repair`), per-story proper-noun glosses (closes MC-2's `literal_out_of_lexicon` gap), deterministic constrained fixture. **Finding (`plans/cp-09a-spike-report.md`):** rerank clears I1 at **B1+** (~$0.07/accepted); **A2 is structurally unreachable via LLM → hand-author A1–A2** (CP-09b). I1 never loosened | ✅ done |
 | CP-09b | **Scale content**: budget-aware coverage-contract briefs (`internal/brief` PlanNewTypes), `ConstrainedProvider` wired into `pipeline` + `workq`, hand-authored A1–A2 backbone (`internal/authored` + `zhuwenctl authored check`), canon 10→81 entries, audit workflow (`zhuwenctl audit`), manifest audit fields (`audit_pass_rate`/`generator`/`model`), I4 verifier rejection of fabricated metrics, license re-verification memo | ✅ done |
 | CP-09c | **Real render + measured budgets** (`plans/cp-09c-plan.md`, `plans/cp-09c-done.md`): CosyVoice 3.0 render as a local (Apple Silicon, $0) external build-time stage (`internal/tts`) replacing stub audio + producing real word-level alignment under the same `pack.AlignToken` contract; NFR-3/NFR-4 size budgets measured (`pack.MeasureBudget`) against real Opus/HEIC weights with a **build-failing** size-budget test; karaoke drift <120 ms re-verified with a real-render sample; **B-4** Commons curation tooling scaled toward the ~570-image launch inventory with per-image license sign-off (owner-in-the-loop, `plans/cp-09c-image-inventory.md`) | ✅ done |
 | CP-10 | **Ship polish** (`plans/cp-10-plan.md`): accessibility (Dynamic Type XXL, VoiceOver), full-device QA pass (`plans/cp-10-qa.md`), App Store assets + metadata + privacy label, honest launch messaging (I4), Credits/attribution completeness (I6), App Store Connect submission via the existing TestFlight pipeline | ⏳ pending |

### Mid-course correction (MC series — post-CP-08 audit remediation, `02-midcourse-correction.md`)

| MC | Scope | State |
|----|-------|-------|
| MC-0 | **License** → GPLv3 + App Store §7 additional permission; `NOTICE-APP-STORE.md`, `CONTRIBUTING.md` (DCO) | ✅ done |
| MC-1 | **In-repo `@main` app** (`ios/App`, XcodeGen, `make app`) + **durable SwiftData event log** (`ZhuwenPersistence`); I5 persistence-tested across relaunch + checkpoint | ✅ done |
| MC-2 | **Content-reality harness**: `lexicon ingest`→`lexicon.sqlite`, `segment eval`, LLM `gen` client (DeepSeek), repair loop (§4.4), `spike`. Real **HSK-3.0 lexicon** ingested (`hsk3.0-v1`, 12,283 forms) + **live DeepSeek spike** run — outcome **ADJUST** (`plans/mc-2-spike-report.md`) | ✅ done |
| MC-3 | **Hosted CI** (`factory-ci.yml`, DCO check, artifact) + iOS weekly job; **resumable idempotent work queue** (`internal/workq`, `zhuwenctl run --resume`, kill-9 e2e) | ✅ done |
| MC-4 | **Docs truth-up**: monorepo decision record, CP-01 10-vs-20 note, `internal/brief`/`gen` tests, this table | ✅ done |

**Revised remaining order (post-MC-4):** CP-08a (images + Foundations) → CP-08 (already
landed) → CP-09 (content scale-up, opened by importing `plans/mc-2-spike-report.md`) → CP-10.

**Works today:** the Go factory builds a signed 10-story fixture pack from the embedded
canon registry and verifies it (I1 + I6 enforced and tested). The iOS `ZhuwenPacks`
package verifies that pack in Swift (minisign + hashes + content-level I6), rejects the
golden negatives, and `ReaderModel` renders a story with tap-to-gloss. `ZhuwenCore` now
carries the learner engine: an append-only event log, a replayable `KnownWordModel`, a
`FrontierQueue`, the on-device `CoverageGate` (I1), and a `Selector` that gates + scores the
story lattice by bitmap AND + popcount (NFR-2: 5,000 stories in ~13 ms under `make bench`).
The coverage gate shares **one vector suite** with the Go reference gate
(`ios/Fixtures/gate-vectors.json`) so I1 cannot drift between the two implementations. The
**placement** flow (M1–M3) samples an HSK×frequency word check with pseudoword foils, fits a
logistic knowledge curve (guessing-corrected), and produces a conservative probabilistic
**seed** of `KnownWordModel` plus CEFR/HSK estimates; re-placement merges without destroying
prior state (FR-1.5) and simulated learners recover their HSK band within ±1. The **listening**
layer (CP-06) ships word-level `alignment` in packs (a deterministic factory forced-alignment
stage; the real CosyVoice render + forced aligner landed in CP-09c as a build-time-only
`internal/tts` stage, replacing the stub audio and timings under the same `pack.AlignToken`
contract) and a `ZhuwenAudio` karaoke engine whose
position→token resolver keeps highlight drift **<120 ms over a 3-minute story** (asserted by
`KaraokeDriftTests`), with speeds 0.6×–1.2×, blind mode, and a labeled on-device system-TTS
fallback (FR-5). The **loop-completion** layer (CP-07) closes the read→check→review→track cycle:
a comprehension check (M8) grades the pack's 3 questions and, on ≥2/3, stamps the 读完为证 **seal**
and boosts P(known) for every exposed word; an on-device **FSRS-4.5** scheduler (M9) surfaces due
words as sentence-context cards (capped 20/day) whose grades fold into per-word memory; and a
Progress dashboard (M10) reports **separate** reading and listening CEFR estimates (blind listens
feed the listening band only), lexicon growth, and the HSK-3.0 gap. The known-word model stays a
replayable projection of the append-only log — including FSRS state — and **P(known) updates are
verified for the exposure / lookup / grade paths**. The **commerce & data** layer (CP-08) adds the
app's *only* network surface — a `PackClient` in `ZhuwenPacks` that downloads additional band packs
over an **anonymous, ephemeral** CDN GET and **verifies them (minisign + hashes + I6) before
installing** (a tampered download is rejected, nothing lands on disk), backing a size/delete/
re-download pack manager — plus **StoreKit 2** SKUs ($7.99/mo · $59.99/yr + 30-day trial · $149.99
lifetime, no receipt server) behind a pure `FeatureGate` (free = the full method at one story/day;
Pro removes the throttle and opens the lattice), a single dismissible **paywall** (M12), a
**settings** screen (M13, FR-10.1 toggles + pack manager + privacy page), **export everything
(JSON) / erase / import** (FR-10.3 — the learner archive is just the ordered event log + seed, so
`export→erase→import` round-trips the exact `KnownWordModel`), and an **opt-in, off-by-default**
private-CloudKit sync of learner state only. `URLSession` is confined to `PackClient` and enforced
by a `grep-audit.sh` network-surface gate (no third-party SDKs, no `http://`, no secrets in
UserDefaults — invariant I2). All of
this is covered by `swift test`. Generation uses a deterministic fixture provider (real LLM
retelling is CP-09); pack audio bytes are CosyVoice-rendered Opus with real word-level
alignment (CP-09c `internal/tts`; a deterministic stub keeps CI hermetic), and pack sizes are
asserted against NFR-3/NFR-4 by a build-failing `pack.MeasureBudget` test. The
**`@main` app target is now vendored in-repo** (`ios/App`, generated by XcodeGen via `make app` —
no manual Xcode assembly) and the learner event log is **durably persisted** in SwiftData
(`ZhuwenPersistence.PersistentEventLog`): it is append-only and rebuilt by replay at launch, so
I5 is now *persistence-tested* across a process boundary (`LaunchReplayTests`), with a disposable
projection checkpoint keeping cold launch inside the NFR-1 budget for large histories (MC-1).

## Factory — build, run, test

```sh
cd factory

make build        # compile commands into ./bin  (EC-2)
./bin/zhuwenctl lexicon
./bin/zhuwenctl build  --out /tmp/a2.zpack        # signed fixture pack (+ /tmp/a2.zpack.pub)
./bin/zhuwenctl verify /tmp/a2.zpack --pub /tmp/a2.zpack.pub
./bin/zhuwenctl budget                              # CP-09c: measure NFR-3/NFR-4 pack sizes
./bin/zhuwenctl segment eval                        # FMM coverage + ambiguity report
./bin/zhuwenctl spike --n 5                         # MC-2: canon->gen->gate->repair harness (fixture)
./bin/zhuwenctl spike --lexicon /tmp/lexicon.sqlite --known-max 4 --frontier-level 5 \
    --oversample 6 --n 3 [--live] [--naive]         # CP-09a: constrained candidate-rerank + name-and-replace
./bin/zhuwenctl run --db /tmp/work.db               # MC-3: resumable SQLite work queue
./bin/zhuwenctl run --db /tmp/work.db --resume      #        recover a crashed run (no double-charge)
./bin/zhuwenctl authored check --file data/authored/a1-spine.json  # CP-09b: gate hand-authored A1 stories through I1
./bin/zhuwenctl authored check --file data/authored/a1-spine.json \
    --lexicon /tmp/lexicon.sqlite --known-max 2 --frontier-level 3 --band A1 [--verbose]
./bin/zhuwenctl audit --pack /tmp/a2.zpack --pub /tmp/a2.zpack.pub  # CP-09b: sample stories for audit
./bin/zhuwenctl audit --decisions /tmp/audit-decisions.json [--generator deepseek-rerank] [--model deepseek-chat]

make test         # unit + integration + e2e (go test ./...)
make vet
make fmt
make ci           # fmt + vet + test
make fixtures     # regenerate ios/Fixtures (DEV-signed, reproducible)
```

Command reference: `zhuwenctl {lexicon | lexicon ingest | segment eval | spike | run | build | verify | budget | keygen | authored check | audit}` —
run `zhuwenctl` with no args for usage. The real **HSK-3.0 lexicon** ships in-repo under
`factory/data/hsk3.0/` (12,283 forms, exact per-level mapping; redistribution authorized —
see `factory/data/README.md`) and is built with `zhuwenctl lexicon ingest --src data/hsk3.0
--out lexicon.sqlite --version hsk3.0-v1` (regenerable via `cmd/hskingest`). The `spike --live`
path reads `ZHUWEN_LLM_API_KEY` or `~/.deepseek-api-key` and is never exercised in CI (the
constrained **fixture** provider is the hermetic stand-in — it reproduces gate-passing stories at
A2 and B1 in `make ci`). CP-09a flags: `--oversample <N>` (rerank candidates/story),
`--max-tokens <T>` (per-story ceiling), `--naive` (MC-2 single-candidate baseline). Pack
format is documented in [`factory/PACK_FORMAT.md`](factory/PACK_FORMAT.md).

## iOS — build and test

SPM packages live under `ios/`. Pure-Swift logic (packs, core models) is testable on macOS
without a simulator:

```sh
cd ios
make test         # swift test — ZhuwenPacks + ZhuwenCore + ZhuwenAudio + ZhuwenPersistence (unit / integration / e2e vs Fixtures/)
make bench        # NFR-2 selector benchmark under -c release (the 50 ms gate)
make audit        # network-surface CI gate (I2): URLSession only in PackClient, no SDKs / http:// / secrets
make build-ios    # xcodebuild the SwiftUI shell for the iOS 17 simulator
make app          # XcodeGen-generate + build the in-repo @main app (ios/App) for the iOS 17 simulator
make app-test     # run the app XCUITest smoke (launch → read → lookup → relaunch persists); needs a booted sim
```

The durable learner store lives in `ZhuwenPersistence` (SwiftData; macOS 14+ host for `swift test`).
The `@main` app (`ios/App`, XcodeGen) is agent-buildable from a clean clone with `make app`; the
generated `.xcodeproj` is git-ignored. XcodeGen (MIT) is a build-time tool, not a shipped dependency.

See [`ios/README.md`](ios/README.md) for the package breakdown.

## Continuous integration

- **`factory-ci.yml`** (Go) — runs the **full** factory gate `make ci` (`gofmt` + `vet` +
  `go test ./...`, including the `cmd/` e2e and the MC-3 kill-9 work-queue e2e) on Linux for
  every push and PR, uploads the built `zhuwenctl` binary as an artifact, and enforces the
  **DCO sign-off** (`git commit -s`) on every PR commit (MC-0/MC-3). Target < 3 min.
- **`ci.yml`** (iOS) — runs the Swift **unit** subset (`make test-unit`, which also covers the
  `LaunchReplayTests` I5 durability check) plus the network-surface audit on macOS for every
  push/PR, and re-runs the **full** iOS suite (`make test` incl. integration + 50k-replay perf,
  `make bench`, `make audit`) on a **weekly schedule** — the per-PR macOS unit job is kept
  because it is fast; the weekly job is the hosted signal for the heavy suites.
- **Pre-push hook** ([`.githooks/pre-push`](.githooks/pre-push)) still guards the heavier
  **integration/e2e** suites, the NFR-2 benchmark, and the audit locally, so a broken build can
  never be pushed. Enable once per clone:

  ```sh
  git config core.hooksPath .githooks
  ```
- **`testflight.yml`** (iOS) — builds the in-repo `@main` app (`ios/App`) and uploads it to
  **TestFlight** (`guru.parso.zhuwen`) using App Store Connect API **cloud signing** (no
  pre-made provisioning profile). Manual (`gh workflow run testflight.yml`) or on a `v*` tag.
  See [`plans/testflight-done.md`](plans/testflight-done.md).

## Testing philosophy
Every feature ships with unit, integration, and e2e tests. In the factory these are Go
tests (golden-file pipeline, gate property vectors, pack verifier negative suite). Invariants
I1–I6 are enforced in code and covered by must-fail fixtures. The coverage gate (I1) is
verified by a **single shared vector suite** generated by the Go reference gate and run
through both the Go and Swift implementations (`ios/Fixtures/gate-vectors.json`), so the
invariant cannot drift across languages (handoff §7). The known-word model is an append-only
event log with a **replayable** projection (I5), tested for determinism.

## Exit criteria
Each checkpoint must also satisfy the standing criteria in
[`plans/exit-criteria.md`](plans/exit-criteria.md) (this README kept current; `make build`
→ `bin/`; green `make ci`).

## License
**GPLv3 with an App Store additional permission** (see [`LICENSE`](LICENSE) and
[`NOTICE-APP-STORE.md`](NOTICE-APP-STORE.md)). The project is honestly copyleft under the
GNU GPL version 3; a GPLv3 §7 additional permission explicitly authorizes distribution
through Apple's App Store / TestFlight so it can also ship there. Contributions require a
DCO sign-off (`git commit -s`) and are accepted under GPLv3 + that exception — see
[`CONTRIBUTING.md`](CONTRIBUTING.md).
