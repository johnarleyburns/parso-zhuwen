# iOS Test Fixtures (vendored from the factory)

Golden inputs for the `parso-zhuwen-ios` app tests (handoff §1: "fixture packs produced
by the factory are vendored into the iOS repo under `Fixtures/` and treated as golden
inputs"). Do not hand-edit — regenerate from the factory.

## Contents
- `fixture-a2-v0.zpack` — 10 canon-derived A2 stories, signed with the DEV fixture key.
- `zhuwen-dev.pub` — the minisign public key that verifies the pack above.

## Provenance & signing
Signed with a **DEV-ONLY** reproducible key derived from a public seed
(`zhuwenctl build --devkey`). This key signs test fixtures only and must never sign a
production/CDN pack. The production publisher key is managed separately (handoff §10.3).

## Regenerate
```
cd factory
make fixtures        # or:
go run ./cmd/zhuwenctl build --devkey \
    --out ../ios/Fixtures/fixture-a2-v0.zpack \
    --pub ../ios/Fixtures/zhuwen-dev.pub
```

The iOS `PackStore` negative tests (unsigned / tampered / imageless rejection) construct
their malformed variants at test time from this base pack, mirroring
`factory/internal/pack` golden tests.
