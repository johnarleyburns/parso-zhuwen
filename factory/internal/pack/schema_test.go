package pack

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

// frozenSchemaSHA256 pins the CP-02 content.sqlite DDL. If this test fails you changed
// schema.sql — that is a pack-format change: bump SchemaVersion, update this hash, and
// note it in the format doc (handoff §3: packs are immutable; the schema is frozen).
const frozenSchemaSHA256 = "40f1f7522222a9cf7ed83ebcffc5bec775319e4ac132e6b2126f8c6bf9e2bf21"

func TestSchemaFrozen(t *testing.T) {
	sum := sha256.Sum256([]byte(schemaSQL))
	got := hex.EncodeToString(sum[:])
	if frozenSchemaSHA256 == "" {
		t.Skipf("record frozenSchemaSHA256 = %q to arm the freeze guard", got)
	}
	if got != frozenSchemaSHA256 {
		t.Fatalf("schema.sql changed: got %s want %s (bump SchemaVersion + update hash)", got, frozenSchemaSHA256)
	}
}

func TestSchemaVersionPositive(t *testing.T) {
	if SchemaVersion < 1 {
		t.Fatalf("SchemaVersion must be >= 1, got %d", SchemaVersion)
	}
}
