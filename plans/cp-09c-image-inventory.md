# CP-09c Image Inventory — B-4 curation at ~570 scale (owner-in-the-loop)

Tracks the §8A Commons image demand for the launch content set and its batch-by-batch
curation status. This is the schedule long pole (`plans/blockers.md` B-4): a canon entry /
pack ships **only** when its cover has a signed-off, gate-passing, provenanced Commons image
(`pack.validateI6`). Fixture stand-ins are not shippable.

**Workflow per batch** (`plans/cp-09c-plan.md` Part D):
`imagespike` (regenerate candidates, Commons anonymous GET — CI never takes this path, I2) →
owner reviews the HTML sheet, overrides picks → export `*-image-decisions.json` →
`zhuwenctl images curate` locks image + provenance into the pack via `DecisionsToImages` →
**per-image license sign-off on the Commons page** recorded (FR-11.2/I4;
`images.DecisionsToImagesSignedOff` rejects an unsigned cover).

```sh
cd factory
go run ./cmd/imagespike --out /tmp/opencode/zhuwen-image-review --n 6   # regenerate (hits Commons)
open /tmp/opencode/zhuwen-image-review/<batch>-review.html               # review + override
# export decisions from the sheet, then:
go run ./cmd/zhuwenctl images curate --decisions <batch>-image-decisions.json \
    --lexicon /tmp/lexicon.sqlite --out <batch>-curated.json
```

## Demand summary (~570 images)

| Segment | Source | Images | Notes |
|---------|--------|--------|-------|
| Foundations F0 | `data/foundations/f0-inventory.tsv` (217 concrete words) | ~217 | word covers; F0 reviewed batch committed (`f0-image-decisions.json`, 219 decisions) |
| Canon covers | `internal/assets/canon.seed.json` (81 entries, **fully Chinese canon** — Aesop/Andersen/Grimm replaced by Chinese legend/myth/history/religion, keeping the 6 school-naturalized fables) | 81 | one cover per canon story (C1–C7 tiers) |
| A1–A2 authored spine | `data/authored/a1-spine.json` | ~40 | one cover per authored story |
| B1 packs | generated (rerank, CP-09b) | ~230 | one cover per generated story across B1 topics |
| **Total** | | **~568** | tracked below |

## Batch plan (bounded owner sittings)

Batches are grouped so each `imagespike` run + review sheet is one sitting. Status:
`todo` → `candidates` (sheet generated) → `reviewed` (owner picked) → `signed-off`
(license verified) → `shipped` (`validateI6` green in the built pack).

### Foundations F0 (word covers)
| Batch | Set(s) | Words | Status |
|-------|--------|-------|--------|
| F0-1 | animals | 26 | signed-off (in `f0-image-decisions.json`) |
| F0-2 | food-drink | 30 | signed-off |
| F0-3 | family + body | 31 | signed-off |
| F0-4 | colors + home | 40 | signed-off |
| F0-5 | kitchen + places + buildings | 30 | signed-off |
| F0-6 | weather + transport | 30 | signed-off |
| F0-7 | clothing + actions + sports-toys | 30 | signed-off |

> F0 was curated in CP-08a and committed. The sign-off flag (`Provenance.SignedOff`) is the
> machine-checkable record; the human verification lives on each Commons page.

### Canon covers (81 entries)
Reviewed via `imagespike` against `data/canon/canon-cover-inventory.tsv`; decisions in
`data/canon/canon-cover-decisions.json`. `reviewed` = a cover picked in the sheet;
`signed-off` = license verified on the Commons page + `Provenance.SignedOff` recorded.

| Batch | Tier | Count | Reviewed | Status |
|-------|------|-------|----------|--------|
| CAN-C1 | C1 | 20 | 20/20 | reviewed |
| CAN-C2 | C2 | 14 | 14/14 | reviewed |
| CAN-C3 | C3 | 5 | 0/5 | todo |
| CAN-C4 | C4 | 14 | 14/14 | reviewed |
| CAN-C5 | C5 | 13 | 6/13 | in progress (6 kept fables done; 7 new Chinese stories to review) |
| CAN-C6 | C6 | 10 | 0/10 | todo |
| CAN-C7 | C7 | 5 | 0/5 | todo |

> **Canon is now fully Chinese** — 12 non-Chinese stories (7 pure Aesop + 5 Andersen/Grimm)
> replaced with Chinese legend/myth/history/religion tales at the same tier; the 6 fables
> naturalized into Chinese schooling (龟兔赛跑, 狼来了, 北风与太阳, 乌鸦喝水, 狐狸和乌鸦, 农夫与蛇)
> were kept. 54/81 covers reviewed; **27 remaining** = C3 (5) + 7 new C5 + C6 (10) + C7 (5),
> resuming at 愚公移山. Continue with `--decided data/canon/canon-cover-decisions.json`.

### Authored spine + B1 packs
| Batch | Scope | Count | Status |
|-------|-------|-------|--------|
| AUT-A1 | A1–A2 authored spine | ~40 | todo |
| B1-* | B1 generated stories (by topic) | ~230 | todo (paced with generation) |

## Ship-readiness gate

A canon entry / pack graduates from fixture stand-in to shippable only when **every** story's
cover is `signed-off` and passes the §8A gate. `pack.validateI6` (builder) and `verifyI6`
(verifier) block imageless / AI / unprovenanced covers; `images.DecisionsToImagesSignedOff`
blocks an unsigned license. This doc is the human tracker of what has cleared; the code is the
enforcement. **B-4 must be closed for the launch pack set before CP-10 submission.**
