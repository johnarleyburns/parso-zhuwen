# CP-02 — Done note (Pack Format Freeze)

**Status:** complete. `go test ./...`, `go vet ./...`, `gofmt -l` all green.
55 test functions (was 42 at CP-01).

## What shipped
- **`internal/minisign`** — minisign-compatible detached signatures (legacy pure-ed25519
  `Ed`; stdlib-only). Public-key / secret-key / signature file formats per `PACK_FORMAT.md`;
  trusted-comment global signature; key_id binding.
- **Schema frozen** — `schema.sql` gains a `meta` table; `PRAGMA user_version` = 
  `pack.SchemaVersion` (=1). `TestSchemaFrozen` pins the DDL sha256 (drift guard).
- **Reference verifier hardened** — `pack.Verify` now enforces, in order: minisign
  signature over `manifest.json` → every file hash → `lexicon_version` acceptance
  (`VerifyOptions.KnownLexiconVersions`) → **content-level I6** (opens `content.sqlite`,
  every story's `cover_image_id` resolves to a fully-provenanced image).
- **CLI** — `keygen`/`build --key`/`build --devkey`/`verify` on minisign keys.
- **`PACK_FORMAT.md`** — frozen format spec + verifier reject matrix.
- **`ios/Fixtures/`** vendored — `fixture-a2-v0.zpack` + `zhuwen-dev.pub` + README,
  reproducible via `make fixtures` (DEV key from a public seed; no secret committed).

## Golden negative suite (handoff §6 CP-02 acceptance)
All three rejected by the reference verifier:
- **unsigned** — `manifest.sig` stripped → "manifest.sig missing".
- **tampered** — `content.sqlite` byte flipped → "hash mismatch".
- **imageless (I6)** — SQLite rewritten to null `cover_image_id`, then *re-signed with a
  valid key* so signature+hashes pass and it fails **only** on the content-level I6 audit.
  This proves the verifier (not just the builder) enforces I6.

Plus: wrong-key rejection, tampered-trusted-comment rejection, lexicon_version accept/reject,
and a CLI e2e keygen→build→verify round trip with cross-key rejection.

## Demo
```
$ zhuwenctl keygen --out publisher
$ zhuwenctl build --out a2.zpack --key publisher.key
$ zhuwenctl verify a2.zpack --pub a2.zpack.pub
OK: pack a2 v0.0.0, lexicon fixture-hsk3.0-v0, schema 1, 11 files
$ make fixtures   # regenerate ios/Fixtures deterministically
```

## Deviations / notes
- Legacy pure-ed25519 minisign variant (no BLAKE2b prehash) → stdlib-only, still
  verifiable by the upstream `minisign` CLI. Prehashed `ED` can be added later if needed.
- Factory secret keys stored unencrypted (minisign scrypt KDF omitted); acceptable for
  internal factory custody (handoff §10.3). Production key custody still an open item.

## Next: CP-03 (App skeleton) — or CP-04 model/gate parity
Handoff §7 also calls for one shared invariant suite running the same gate vectors through
Go and Swift. Recommended next: begin the iOS `ZhuwenPacks`/`ZhuwenCore` modules with a
Swift pack verifier + `CoverageGate` that consumes `ios/Fixtures/`, and stand up the
cross-language gate-vector fixture so I1 can't drift between implementations.
