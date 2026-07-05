package minisign

import (
	"strings"
	"testing"
)

func TestSignVerifyRoundTrip(t *testing.T) {
	pub, priv, err := GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	msg := []byte("manifest bytes")
	sig := Sign(priv, msg, "pack a2 v0.0.0")
	if err := Verify(pub, msg, sig); err != nil {
		t.Fatalf("verify: %v", err)
	}
	tc, err := TrustedComment(sig)
	if err != nil || tc != "pack a2 v0.0.0" {
		t.Errorf("trusted comment = %q err=%v", tc, err)
	}
}

func TestVerifyRejectsTamperedMessage(t *testing.T) {
	pub, priv, _ := GenerateKey()
	sig := Sign(priv, []byte("original"), "tc")
	if err := Verify(pub, []byte("modified"), sig); err == nil {
		t.Fatal("expected failure for tampered message")
	}
}

func TestVerifyRejectsWrongKey(t *testing.T) {
	_, priv, _ := GenerateKey()
	otherPub, _, _ := GenerateKey()
	sig := Sign(priv, []byte("m"), "tc")
	err := Verify(otherPub, []byte("m"), sig)
	if err == nil || !strings.Contains(err.Error(), "key_id mismatch") {
		t.Fatalf("expected key_id mismatch, got %v", err)
	}
}

func TestVerifyRejectsTamperedTrustedComment(t *testing.T) {
	pub, priv, _ := GenerateKey()
	sig := Sign(priv, []byte("m"), "genuine comment")
	// Swap the trusted comment text but keep everything else.
	tampered := strings.Replace(sig, "trusted comment: genuine comment", "trusted comment: forged comment", 1)
	if err := Verify(pub, []byte("m"), tampered); err == nil || !strings.Contains(err.Error(), "trusted comment") {
		t.Fatalf("expected trusted-comment failure, got %v", err)
	}
}

func TestPublicKeyEncodeParseRoundTrip(t *testing.T) {
	pub, _, _ := GenerateKey()
	enc := pub.Encode()
	got, err := ParsePublicKey(enc)
	if err != nil {
		t.Fatal(err)
	}
	if got.KeyID != pub.KeyID || string(got.PK) != string(pub.PK) {
		t.Error("public key round trip mismatch")
	}
}

func TestSecretKeyEncodeParseRoundTrip(t *testing.T) {
	_, priv, _ := GenerateKey()
	got, err := ParseSecret(priv.EncodeSecret())
	if err != nil {
		t.Fatal(err)
	}
	if got.KeyID != priv.KeyID || string(got.SK) != string(priv.SK) {
		t.Error("secret key round trip mismatch")
	}
}

func TestDeterministicKeyFromSeed(t *testing.T) {
	var seed [32]byte
	var id [8]byte
	copy(seed[:], "zhuwen-dev-seed-0000000000000000")
	copy(id[:], "zhuwendv")
	pub1, priv1 := KeyFromSeed(seed, id)
	pub2, _ := KeyFromSeed(seed, id)
	if string(pub1.PK) != string(pub2.PK) {
		t.Fatal("KeyFromSeed not deterministic")
	}
	sig := Sign(priv1, []byte("x"), "tc")
	if err := Verify(pub1, []byte("x"), sig); err != nil {
		t.Fatalf("deterministic key verify: %v", err)
	}
}

func TestParseRejectsGarbage(t *testing.T) {
	if _, err := ParsePublicKey("not a key"); err == nil {
		t.Error("expected parse error")
	}
	pub, priv, _ := GenerateKey()
	if err := Verify(pub, []byte("m"), "too\nshort"); err == nil {
		t.Error("expected truncated-file error")
	}
	_ = priv
}
