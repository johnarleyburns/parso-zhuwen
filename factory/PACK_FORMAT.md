# Zhuwen Pack Format (v1, FROZEN at CP-02)

Authoritative spec for `.zpack` content packs. Refs: handoff §3, requirements §9.
Packs are **immutable**; any fix ships as a new semver. The on-device app verifies a
pack before install and rejects unsigned / tampered / unknown-lexicon / imageless packs.

## Container

```
zhuwen-pack-<id>-v<semver>.zpack        # zip container
├── manifest.json      # id, semver, lexicon_version, created_at, schema_version, file hashes
├── manifest.sig       # minisign detached signature over manifest.json (see below)
├── content.sqlite     # schema below (PRAGMA user_version = schema_version)
├── images/<image_id>@{480,1200}.heic
└── audio/<story_id>.opus                # 24 kbps mono, -16 LUFS (added in CP-06)
```

Zip entries are written **STORED** (no compression): still a standard zip, but this lets
the on-device reader extract entries without an inflate dependency. Manifest hashes and
the signature are over file *contents* (not the zip encoding), so the compression method
is verification-neutral.

`manifest.json`:

```json
{
  "id": "a2",
  "semver": "0.0.0",
  "lexicon_version": "fixture-hsk3.0-v0",
  "created_at": "2026-07-04T00:00:00Z",
  "schema_version": 1,
  "files": { "content.sqlite": "<sha256>", "images/…@480.heic": "<sha256>" }
}
```

## Signature (minisign, legacy `Ed` / pure ed25519)

`manifest.sig` is a standard minisign signature file over the exact bytes of
`manifest.json`:

```
untrusted comment: signature from zhuwen minisign key
base64( "Ed"(2) || key_id(8) || ed25519_sig(64) )
trusted comment: pack <id> v<semver> lexicon <lexicon_version>
base64( ed25519(secret, ed25519_sig(64) || trusted_comment_bytes) )
```

The app bakes in the publisher public key (`"Ed"(2) || key_id(8) || pubkey(32)` base64).
Verification: check `key_id`, verify the signature over `manifest.json`, verify the
trusted-comment global signature, then confirm every `files[]` hash. The legacy pure
variant (no BLAKE2b prehash) is verifiable by the upstream `minisign` CLI.

## content.sqlite schema (v1)

Frozen DDL: `internal/pack/schema.sql` (sha256 pinned by `TestSchemaFrozen`).
`PRAGMA user_version` mirrors `schema_version`. A `meta(key,value)` table carries
`schema_version`, `lexicon_version`, `pack_id`, `semver`.

Tables: `meta`, `lexicon`, `character`, `image`, `story`, `question`,
`sentence_translation`, `citation`, `alignment`, `foundations_card`.

**I6 at the schema level:** `story.cover_image_id` is `NOT NULL`. The reference verifier
additionally enforces, per story, that `cover_image_id` is non-empty and resolves to an
`image` row with a complete provenance record (license, license_url, author, source_url,
retrieved_at). AI-categorized images are excluded upstream at curation (§8A).

## Verifier reject matrix (reference implementation: `pack.Verify`)

| Condition | Result |
|-----------|--------|
| `manifest.sig` missing | reject (unsigned) |
| signature invalid / wrong key_id | reject |
| trusted comment altered | reject |
| any file hash mismatch | reject (tampered) |
| `lexicon_version` not in app's known set | reject |
| story with empty/missing/unprovenanced cover image | reject (I6) |

## Changing the format

Bump `pack.SchemaVersion`, update `schema.sql`, update `frozenSchemaSHA256` in
`schema_test.go`, and update this doc. Old packs remain valid under their own
`schema_version`; the app keys behavior off it.
