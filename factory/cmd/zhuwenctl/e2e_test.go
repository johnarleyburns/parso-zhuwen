package main

// End-to-end test: compile the real zhuwenctl binary and drive the CP-01 walking
// skeleton through it — lexicon ingest -> build signed fixture pack -> verify. Also
// asserts a tampered pack is rejected at the CLI boundary.

import (
	"archive/zip"
	"bytes"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func buildBinary(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "zhuwenctl")
	cmd := exec.Command("go", "build", "-o", bin, ".")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go build: %v\n%s", err, out)
	}
	return bin
}

func run(t *testing.T, bin string, args ...string) (string, error) {
	t.Helper()
	cmd := exec.Command(bin, args...)
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	err := cmd.Run()
	return buf.String(), err
}

func TestE2E_FullSkeleton(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e in -short")
	}
	bin := buildBinary(t)
	dir := t.TempDir()
	packPath := filepath.Join(dir, "fixture-a2-v0.zpack")
	pubPath := packPath + ".pub"

	if out, err := run(t, bin, "lexicon"); err != nil || !strings.Contains(out, "30 words") {
		t.Fatalf("lexicon: err=%v out=%s", err, out)
	}

	out, err := run(t, bin, "build", "--out", packPath)
	if err != nil || !strings.Contains(out, "stories packed") {
		t.Fatalf("build: err=%v out=%s", err, out)
	}
	if _, err := os.Stat(packPath); err != nil {
		t.Fatalf("pack not written: %v", err)
	}

	out, err = run(t, bin, "verify", packPath, "--pub", pubPath)
	if err != nil || !strings.Contains(out, "OK:") {
		t.Fatalf("verify: err=%v out=%s", err, out)
	}
}

func TestE2E_TamperedPackRejected(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e in -short")
	}
	bin := buildBinary(t)
	dir := t.TempDir()
	packPath := filepath.Join(dir, "p.zpack")
	pubPath := packPath + ".pub"

	if _, err := run(t, bin, "build", "--out", packPath); err != nil {
		t.Fatalf("build failed: %v", err)
	}
	tamperContent(t, packPath)

	out, err := run(t, bin, "verify", packPath, "--pub", pubPath)
	if err == nil {
		t.Fatalf("expected non-zero exit for tampered pack, out=%s", out)
	}
	if !strings.Contains(out, "hash mismatch") && !strings.Contains(out, "signature invalid") {
		t.Fatalf("expected tamper rejection reason, got: %s", out)
	}
}

func TestE2E_KeygenBuildVerifyRoundTrip(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e in -short")
	}
	bin := buildBinary(t)
	dir := t.TempDir()
	keyPrefix := filepath.Join(dir, "publisher")
	packPath := filepath.Join(dir, "p.zpack")

	if out, err := run(t, bin, "keygen", "--out", keyPrefix); err != nil || !strings.Contains(out, ".pub") {
		t.Fatalf("keygen: err=%v out=%s", err, out)
	}
	if _, err := run(t, bin, "build", "--out", packPath, "--key", keyPrefix+".key", "--pub", packPath+".pub"); err != nil {
		t.Fatalf("build --key: %v", err)
	}
	out, err := run(t, bin, "verify", packPath, "--pub", packPath+".pub")
	if err != nil || !strings.Contains(out, "schema 1") {
		t.Fatalf("verify: err=%v out=%s", err, out)
	}
	// A pack signed by the DEV key must NOT verify against the publisher key.
	devPack := filepath.Join(dir, "dev.zpack")
	if _, err := run(t, bin, "build", "--devkey", "--out", devPack, "--pub", devPack+".pub"); err != nil {
		t.Fatalf("build --devkey: %v", err)
	}
	if out, err := run(t, bin, "verify", devPack, "--pub", packPath+".pub"); err == nil {
		t.Fatalf("expected wrong-key rejection, out=%s", out)
	}
}

// tamperContent rewrites the pack, flipping one byte of content.sqlite.
func tamperContent(t *testing.T, path string) {
	t.Helper()
	zr, err := zip.OpenReader(path)
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, f := range zr.File {
		rc, _ := f.Open()
		data, _ := io.ReadAll(rc)
		rc.Close()
		if f.Name == "content.sqlite" {
			data = append(data, 0x7F)
		}
		w, _ := zw.Create(f.Name)
		w.Write(data)
	}
	zw.Close()
	zr.Close()
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
}
