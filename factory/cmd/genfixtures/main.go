// Command genfixtures writes the vendored iOS golden fixtures: the positive pack, its
// public key, and the three golden negatives (unsigned / tampered / imageless) that the
// Swift and Go pack verifiers must reject. Reproducible (DEV key from a public seed).
package main

import (
	"fmt"
	"os"

	"github.com/parso/zhuwen-factory/internal/fixtures"
)

func main() {
	dir := "../ios/Fixtures"
	for i := 1; i < len(os.Args); i++ {
		if os.Args[i] == "--out" && i+1 < len(os.Args) {
			dir = os.Args[i+1]
		}
	}
	if err := fixtures.WriteAll(dir); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
	fmt.Printf("wrote fixtures to %s: fixture-a2-v0.zpack, zhuwen-dev.pub, golden-{unsigned,tampered,imageless}.zpack, gate-vectors.json\n", dir)
}
