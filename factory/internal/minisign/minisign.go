// Package minisign implements minisign-compatible detached signatures (handoff §3:
// "manifest.sig — minisign detached signature (ed25519; pubkey baked into app)").
//
// We use the legacy pure-ed25519 algorithm ("Ed"): the 64-byte signature is
// ed25519(secret, message) — no BLAKE2b prehash — which keeps the implementation
// stdlib-only while remaining verifiable by the reference `minisign` CLI. The signature
// file carries a trusted comment protected by a second (global) ed25519 signature, and a
// key_id ties each signature to a specific key.
//
// File formats (identical to minisign):
//
//	public key file:
//	  untrusted comment: <text>
//	  base64( "Ed"(2) || key_id(8) || ed25519_pub(32) )
//
//	signature file:
//	  untrusted comment: <text>
//	  base64( "Ed"(2) || key_id(8) || signature(64) )
//	  trusted comment: <text>
//	  base64( ed25519(secret, signature(64) || trusted_comment_bytes) )
package minisign

import (
	"bufio"
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
)

// sigAlg is the legacy pure-ed25519 algorithm identifier.
var sigAlg = [2]byte{'E', 'd'}

// PublicKey is a minisign public key.
type PublicKey struct {
	KeyID [8]byte
	PK    ed25519.PublicKey
}

// PrivateKey is a minisign secret key (stored unencrypted for factory use).
type PrivateKey struct {
	KeyID [8]byte
	SK    ed25519.PrivateKey
}

// GenerateKey creates a fresh keypair with a random key_id.
func GenerateKey() (PublicKey, PrivateKey, error) {
	pk, sk, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return PublicKey{}, PrivateKey{}, err
	}
	var id [8]byte
	if _, err := rand.Read(id[:]); err != nil {
		return PublicKey{}, PrivateKey{}, err
	}
	return PublicKey{KeyID: id, PK: pk}, PrivateKey{KeyID: id, SK: sk}, nil
}

// KeyFromSeed derives a deterministic keypair from a 32-byte seed and an 8-byte key_id.
// Used for reproducible dev/test fixtures — never for production keys.
func KeyFromSeed(seed [32]byte, keyID [8]byte) (PublicKey, PrivateKey) {
	sk := ed25519.NewKeyFromSeed(seed[:])
	pk := sk.Public().(ed25519.PublicKey)
	return PublicKey{KeyID: keyID, PK: pk}, PrivateKey{KeyID: keyID, SK: sk}
}

// Public derives the matching public key from a secret key.
func (priv PrivateKey) Public() PublicKey {
	return PublicKey{KeyID: priv.KeyID, PK: priv.SK.Public().(ed25519.PublicKey)}
}

// Sign returns a minisign signature-file string for message with the given trusted comment.
func Sign(priv PrivateKey, message []byte, trustedComment string) string {
	sig := ed25519.Sign(priv.SK, message)

	blob := make([]byte, 0, 2+8+64)
	blob = append(blob, sigAlg[0], sigAlg[1])
	blob = append(blob, priv.KeyID[:]...)
	blob = append(blob, sig...)

	global := ed25519.Sign(priv.SK, append(append([]byte(nil), sig...), []byte(trustedComment)...))

	var b strings.Builder
	fmt.Fprintf(&b, "untrusted comment: signature from zhuwen minisign key\n")
	fmt.Fprintf(&b, "%s\n", base64.StdEncoding.EncodeToString(blob))
	fmt.Fprintf(&b, "trusted comment: %s\n", trustedComment)
	fmt.Fprintf(&b, "%s\n", base64.StdEncoding.EncodeToString(global))
	return b.String()
}

// Verify checks a minisign signature file over message against pub.
func Verify(pub PublicKey, message []byte, sigFile string) error {
	lines := splitLines(sigFile)
	if len(lines) < 4 {
		return errors.New("minisign: signature file truncated")
	}
	blob, err := base64.StdEncoding.DecodeString(strings.TrimSpace(lines[1]))
	if err != nil {
		return fmt.Errorf("minisign: bad signature base64: %w", err)
	}
	if len(blob) != 2+8+64 {
		return fmt.Errorf("minisign: bad signature length %d", len(blob))
	}
	if blob[0] != sigAlg[0] || blob[1] != sigAlg[1] {
		return fmt.Errorf("minisign: unsupported algorithm %q", string(blob[0:2]))
	}
	var keyID [8]byte
	copy(keyID[:], blob[2:10])
	if keyID != pub.KeyID {
		return errors.New("minisign: key_id mismatch (signed by a different key)")
	}
	sig := blob[10:74]
	if !ed25519.Verify(pub.PK, message, sig) {
		return errors.New("minisign: signature invalid (tampered or wrong key)")
	}

	tcPrefix := "trusted comment: "
	if !strings.HasPrefix(lines[2], tcPrefix) {
		return errors.New("minisign: missing trusted comment line")
	}
	trusted := lines[2][len(tcPrefix):]
	global, err := base64.StdEncoding.DecodeString(strings.TrimSpace(lines[3]))
	if err != nil {
		return fmt.Errorf("minisign: bad global signature base64: %w", err)
	}
	if !ed25519.Verify(pub.PK, append(append([]byte(nil), sig...), []byte(trusted)...), global) {
		return errors.New("minisign: trusted comment signature invalid (tampered comment)")
	}
	return nil
}

// TrustedComment extracts the trusted comment from a signature file.
func TrustedComment(sigFile string) (string, error) {
	lines := splitLines(sigFile)
	if len(lines) < 3 || !strings.HasPrefix(lines[2], "trusted comment: ") {
		return "", errors.New("minisign: no trusted comment")
	}
	return lines[2][len("trusted comment: "):], nil
}

// Encode returns the minisign public-key file text.
func (pub PublicKey) Encode() string {
	blob := make([]byte, 0, 2+8+32)
	blob = append(blob, sigAlg[0], sigAlg[1])
	blob = append(blob, pub.KeyID[:]...)
	blob = append(blob, pub.PK...)
	return "untrusted comment: zhuwen minisign public key\n" +
		base64.StdEncoding.EncodeToString(blob) + "\n"
}

// ParsePublicKey parses a minisign public-key file.
func ParsePublicKey(s string) (PublicKey, error) {
	lines := splitLines(s)
	if len(lines) < 2 {
		return PublicKey{}, errors.New("minisign: public key file truncated")
	}
	blob, err := base64.StdEncoding.DecodeString(strings.TrimSpace(lines[1]))
	if err != nil {
		return PublicKey{}, fmt.Errorf("minisign: bad public key base64: %w", err)
	}
	if len(blob) != 2+8+32 {
		return PublicKey{}, fmt.Errorf("minisign: bad public key length %d", len(blob))
	}
	if blob[0] != sigAlg[0] || blob[1] != sigAlg[1] {
		return PublicKey{}, fmt.Errorf("minisign: unsupported public key algorithm %q", string(blob[0:2]))
	}
	var pub PublicKey
	copy(pub.KeyID[:], blob[2:10])
	pub.PK = ed25519.PublicKey(append([]byte(nil), blob[10:42]...))
	return pub, nil
}

// EncodeSecret returns an (unencrypted) secret-key file text for factory use.
func (priv PrivateKey) EncodeSecret() string {
	blob := make([]byte, 0, 2+8+64)
	blob = append(blob, sigAlg[0], sigAlg[1])
	blob = append(blob, priv.KeyID[:]...)
	blob = append(blob, priv.SK...)
	return "untrusted comment: zhuwen minisign SECRET key (unencrypted; keep private)\n" +
		base64.StdEncoding.EncodeToString(blob) + "\n"
}

// ParseSecret parses a secret-key file produced by EncodeSecret.
func ParseSecret(s string) (PrivateKey, error) {
	lines := splitLines(s)
	if len(lines) < 2 {
		return PrivateKey{}, errors.New("minisign: secret key file truncated")
	}
	blob, err := base64.StdEncoding.DecodeString(strings.TrimSpace(lines[1]))
	if err != nil {
		return PrivateKey{}, fmt.Errorf("minisign: bad secret key base64: %w", err)
	}
	if len(blob) != 2+8+64 {
		return PrivateKey{}, fmt.Errorf("minisign: bad secret key length %d", len(blob))
	}
	var priv PrivateKey
	copy(priv.KeyID[:], blob[2:10])
	priv.SK = ed25519.PrivateKey(append([]byte(nil), blob[10:74]...))
	return priv, nil
}

func splitLines(s string) []string {
	var out []string
	sc := bufio.NewScanner(bytes.NewReader([]byte(s)))
	for sc.Scan() {
		out = append(out, sc.Text())
	}
	return out
}
