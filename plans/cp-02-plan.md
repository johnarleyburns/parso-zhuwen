# CP-02 — Pack Format Freeze

**Refs:** handoff §3 (pack format), §6 (CP-02 acceptance), §7 (golden-file tests are the
backbone). Depends on CP-01.

**Acceptance (handoff §6):** `schema.sql`, manifest + minisign, golden packs incl.
(a) unsigned, (b) tampered, (c) imageless (I6) — all three rejected by the reference
verifier. Format doc committed; iOS `Fixtures/` vendored.

## Tasks
1. **minisign signatures.** New `internal/minisign` package producing/verifying real
   minisign detached signatures (legacy `Ed` = pure ed25519 over the message; stdlib-only,
   no BLAKE2b dep). minisign-format public key + `.sig` file (untrusted/trusted comment +
   global signature). `manifest.sig` becomes a minisign detached sig over `manifest.json`.
2. **Freeze schema.** Add a `meta(key,value)` table (schema_version, lexicon_version) and
   `PRAGMA user_version`. Expose `pack.SchemaVersion`. Add a drift test pinning the
   embedded `schema.sql` hash so the frozen DDL can't change silently.
3. **Reference verifier hardening.** `Verify` now: (1) minisign-verifies `manifest.json`,
   (2) checks every manifest file hash, (3) opens `content.sqlite` and enforces **I6**
   (every story.cover_image_id non-empty → image row with full provenance), (4) enforces
   the `lexicon_version` acceptance list (handoff §3: "app refuses packs whose
   lexicon_version it doesn't know"). `VerifyOptions` carries the known-version set.
4. **CLI.** `keygen` writes minisign `.pub`/`.key`; `build --key` signs with a supplied
   key (deterministic dev key for vendored fixtures); `verify` reads a minisign pubkey.
5. **Golden negative suite** (`internal/pack`): deterministic dev key (fixed seed → stable
   pubkey), generate the three malformed packs at test time, assert each is rejected with
   the right reason. The imageless golden is re-signed with a valid sig so it passes
   signature/hash and fails *only* on the I6 content check (proves the verifier, not just
   the builder, enforces I6).
6. **Vendor `ios/Fixtures/`.** A positive `fixture-a2-v0.zpack` + `zhuwen-dev.pub` +
   README (provenance + regeneration command) for the future iOS PackStore tests.

## Dependencies
None new — minisign legacy variant uses only `crypto/ed25519`, `crypto/rand`,
`encoding/base64`.

## Test plan
- **Unit:** minisign sign/verify round trip; wrong-key rejection; tampered-message
  rejection; tampered trusted-comment rejection; pubkey encode/parse round trip; malformed
  sig-file parse errors. Schema drift/freeze test.
- **Integration:** build→verify with minisign; lexicon_version accept vs reject.
- **E2E (golden):** unsigned / tampered / imageless packs each rejected by `Verify`;
  CLI `keygen`→`build --key`→`verify` round trip.

## Done criteria
`go test ./...`, `go vet`, `gofmt -l` clean; `plans/cp-02-done.md`; `ios/Fixtures/` present.
