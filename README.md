# Zhuwen (朱文) — Chinese by Reading

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
| CP-08a…CP-10 | images/Foundations, scale-up, polish | ⏳ pending |

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
stage; real CosyVoice render is CP-09) and a `ZhuwenAudio` karaoke engine whose
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
retelling is CP-09); pack audio bytes are fixture stubs until the CosyVoice render (CP-09). The
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
./bin/zhuwenctl segment eval                        # FMM coverage + ambiguity report
./bin/zhuwenctl spike --n 5                         # MC-2: canon->gen->gate->repair harness

make test         # unit + integration + e2e (go test ./...)
make vet
make fmt
make ci           # fmt + vet + test
make fixtures     # regenerate ios/Fixtures (DEV-signed, reproducible)
```

Command reference: `zhuwenctl {lexicon | lexicon ingest | segment eval | spike | build | verify | keygen}` —
run `zhuwenctl` with no args for usage. The `lexicon ingest` command expects an
**operator-supplied, license-cleared** HSK-3.0 TSV (the raw lists are not distributed
with the repo — see `plans/blockers.md` B-1 and `factory/data/README.md`). The `spike
--live` path requires `ZHUWEN_LLM_API_KEY` and is never exercised in CI. Pack format is
documented in [`factory/PACK_FORMAT.md`](factory/PACK_FORMAT.md).

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

- **GitHub Actions** ([`.github/workflows/ci.yml`](.github/workflows/ci.yml)) runs the **unit
  tests only** on every push and PR — `factory` Go units (`make test-unit` = `go test
  ./internal/...`, plus `gofmt`/`vet`) on Linux, and iOS Swift units (`make test-unit`, skipping
  the simulation/drift acceptance suites) plus the network-surface audit on macOS. Fast signal.
- **Pre-push hook** ([`.githooks/pre-push`](.githooks/pre-push)) guards the heavier
  **integration/e2e** suites, the NFR-2 benchmark, and the audit locally, so a broken build can
  never be pushed: it runs `ios` full `swift test` + `make bench` + `make audit` and factory
  `make ci` (incl. the `cmd/` e2e). Enable once per clone:

  ```sh
  git config core.hooksPath .githooks
  ```

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
