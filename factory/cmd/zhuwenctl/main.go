// Command zhuwenctl is the factory CLI/TUI entry (handoff §4). CP-01 wires the walking
// skeleton: lexicon ingest, a full fixture-pack build, and pack verification.
package main

import (
	"fmt"
	"os"

	"github.com/parso/zhuwen-factory/internal/assets"
	"github.com/parso/zhuwen-factory/internal/fixtures"
	"github.com/parso/zhuwen-factory/internal/minisign"
	"github.com/parso/zhuwen-factory/internal/pack"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	var err error
	switch os.Args[1] {
	case "lexicon":
		err = cmdLexicon()
	case "build":
		err = cmdBuild(os.Args[2:])
	case "verify":
		err = cmdVerify(os.Args[2:])
	case "keygen":
		err = cmdKeygen(os.Args[2:])
	default:
		usage()
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprint(os.Stderr, `zhuwenctl — Zhuwen content factory (CP-01)

usage:
  zhuwenctl lexicon                 ingest the fixture lexicon and report
  zhuwenctl build --out <pack>      run the pipeline and emit a signed .zpack
                    [--pub <file>]  also write the verify pubkey (default <pack>.pub)
                    [--key <file>]  sign with a minisign secret key
                    [--devkey]      sign with the reproducible DEV fixture key
  zhuwenctl verify <pack> --pub <f> verify signature + hashes + I6 + lexicon_version
  zhuwenctl keygen --out <prefix>   write <prefix>.pub / <prefix>.key (minisign)
`)
}

func cmdLexicon() error {
	lex, err := assets.Lexicon()
	if err != nil {
		return err
	}
	fmt.Printf("lexicon %s: %d words, max id %d\n", lex.Version(), lex.Len(), lex.MaxID())
	return nil
}

func flagValue(args []string, name string) string {
	for i := 0; i < len(args); i++ {
		if args[i] == name && i+1 < len(args) {
			return args[i+1]
		}
	}
	return ""
}

func hasFlag(args []string, name string) bool {
	for _, a := range args {
		if a == name {
			return true
		}
	}
	return false
}

// runFixturePipeline builds the CP-01 fixture pack (shared with genfixtures).
func runFixturePipeline() (*pack.Pack, error) {
	p, rejected, err := fixtures.BuildFixturePack()
	if err != nil {
		return nil, err
	}
	fmt.Printf("pipeline: %d stories packed, %d rejected\n", len(p.Stories), rejected)
	return p, nil
}

func cmdBuild(args []string) error {
	out := flagValue(args, "--out")
	if out == "" {
		return fmt.Errorf("build: --out <pack> required")
	}
	pubPath := flagValue(args, "--pub")
	if pubPath == "" {
		pubPath = out + ".pub"
	}
	pk, err := runFixturePipeline()
	if err != nil {
		return err
	}

	// Sign with a supplied minisign secret key, or a fresh ephemeral key.
	var pub minisign.PublicKey
	var priv minisign.PrivateKey
	if hasFlag(args, "--devkey") {
		// DEV-ONLY reproducible key from a public seed — signs vendored test fixtures
		// only, never production packs. Lets ios/Fixtures/ be regenerated deterministically.
		pub, priv = fixtures.DevKey()
	} else if keyPath := flagValue(args, "--key"); keyPath != "" {
		raw, err := os.ReadFile(keyPath)
		if err != nil {
			return err
		}
		if priv, err = minisign.ParseSecret(string(raw)); err != nil {
			return err
		}
		pub = priv.Public()
	} else {
		if pub, priv, err = minisign.GenerateKey(); err != nil {
			return err
		}
	}
	if err := pack.Build(pk, out, priv); err != nil {
		return err
	}
	if err := os.WriteFile(pubPath, []byte(pub.Encode()), 0o644); err != nil {
		return err
	}
	fmt.Printf("wrote %s (%d stories) + pubkey %s\n", out, len(pk.Stories), pubPath)
	return nil
}

func cmdVerify(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("verify: <pack> required")
	}
	packPath := args[0]
	pubPath := flagValue(args, "--pub")
	if pubPath == "" {
		return fmt.Errorf("verify: --pub <file> required")
	}
	raw, err := os.ReadFile(pubPath)
	if err != nil {
		return err
	}
	pub, err := minisign.ParsePublicKey(string(raw))
	if err != nil {
		return err
	}
	man, err := pack.Verify(packPath, pub, nil)
	if err != nil {
		return err
	}
	fmt.Printf("OK: pack %s v%s, lexicon %s, schema %d, %d files\n",
		man.ID, man.Semver, man.LexiconVersion, man.SchemaVersion, len(man.Files))
	return nil
}

func cmdKeygen(args []string) error {
	prefix := flagValue(args, "--out")
	if prefix == "" {
		return fmt.Errorf("keygen: --out <prefix> required")
	}
	pub, priv, err := minisign.GenerateKey()
	if err != nil {
		return err
	}
	if err := os.WriteFile(prefix+".pub", []byte(pub.Encode()), 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(prefix+".key", []byte(priv.EncodeSecret()), 0o600); err != nil {
		return err
	}
	fmt.Printf("wrote %s.pub / %s.key\n", prefix, prefix)
	return nil
}
