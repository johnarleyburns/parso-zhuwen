package main

import (
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"os"
	"sort"

	"github.com/parso/zhuwen-factory/internal/minisign"
	"github.com/parso/zhuwen-factory/internal/pack"
)

func cmdAudit(args []string) error {
	packPath := flagValue(args, "--pack")
	pubPath := flagValue(args, "--pub")
	sampleSize := 20
	if v := flagValue(args, "--sample-size"); v != "" {
		fmt.Sscanf(v, "%d", &sampleSize)
	}
	decisionsFile := flagValue(args, "--decisions")

	// Headless mode: load fixture decisions from a JSON file for CI.
	if decisionsFile != "" {
		return cmdAuditHeadless(decisionsFile, sampleSize)
	}

	// Normal mode: verify pack first, then sample.
	if packPath == "" {
		return fmt.Errorf("audit: --pack <zpack> required (or use --decisions for CI headless mode)")
	}
	if pubPath == "" {
		return fmt.Errorf("audit: --pub <file> required")
	}
	raw, err := os.ReadFile(pubPath)
	if err != nil {
		return err
	}
	pub, err := minisign.ParsePublicKey(string(raw))
	if err != nil {
		return err
	}
	man, err := pack.Verify(packPath, pub, &pack.VerifyOptions{SkipContentChecks: true})
	if err != nil {
		return fmt.Errorf("audit: verify failed: %w", err)
	}
	fmt.Printf("pack %s v%s (lexicon %s) — schema v%d\n", man.ID, man.Semver, man.LexiconVersion, man.SchemaVersion)

	// Extract story list from the manifest.Files for sampling.
	// The content.sqlite contains stories; we sample from man metadata.
	packInfo, err := pack.ReadPackInfo(packPath)
	if err != nil {
		return fmt.Errorf("audit: read pack info: %w", err)
	}

	if len(packInfo.Stories) == 0 {
		return fmt.Errorf("audit: pack contains no stories")
	}

	// Sample stories for human review.
	sampled := sampleStories(packInfo.Stories, sampleSize)
	report := AuditReport{
		PackID:        man.ID,
		LexiconVer:    man.LexiconVersion,
		SchemaVer:     man.SchemaVersion,
		TotalStories:  len(packInfo.Stories),
		Sampled:       sampleSize,
		SampleStories: sampled,
	}
	reportJSON, _ := json.MarshalIndent(report, "", "  ")

	// Write the audit template — human fills verdicts.
	outPath := packPath + ".audit.json"
	if err := os.WriteFile(outPath, reportJSON, 0644); err != nil {
		return err
	}
	fmt.Printf("audit template written to %s (%d stories, sample %d)\n", outPath, report.Sampled, sampleSize)
	fmt.Println("Fill in each story's verdict (pass/fail) and re-run audit to compute pass_rate.")
	return nil
}

func cmdAuditHeadless(decisionsFile string, sampleSize int) error {
	data, err := os.ReadFile(decisionsFile)
	if err != nil {
		return err
	}
	var report AuditReport
	if err := json.Unmarshal(data, &report); err != nil {
		return fmt.Errorf("audit: parse decisions: %w", err)
	}
	if len(report.SampleStories) == 0 {
		return fmt.Errorf("audit: decisions file contains no sampled stories")
	}
	if report.TotalStories == 0 {
		report.TotalStories = len(report.SampleStories)
	}

	passed := 0
	verified := 0
	for i, s := range report.SampleStories {
		if s.VerdictPass != nil {
			verified++
			if *s.VerdictPass {
				passed++
			}
		}
		if i < sampleSize {
			withVerdict := s
			if withVerdict.VerdictPass == nil {
				v := false
				withVerdict.VerdictPass = &v
			}
			report.SampleAudit = append(report.SampleAudit, withVerdict)
		}
	}

	report.SampleSize = len(report.SampleAudit)
	if verified == 0 {
		return fmt.Errorf("audit: no verdicts found in decisions file")
	}
	report.AuditPassRate = float64(passed) / float64(verified)
	report.AuditPassed = passed
	report.AuditTotal = verified
	report.Generator = flagValue(os.Args, "--generator")
	if report.Generator == "" {
		report.Generator = "unknown"
	}
	report.Model = flagValue(os.Args, "--model")
	if report.Model == "" {
		report.Model = "unknown"
	}

	fmt.Printf("audit result: %d/%d passed (%.1f%%)\n", passed, verified, report.AuditPassRate*100)

	// Write completed report back.
	outPath := decisionsFile + ".result.json"
	reportJSON, _ := json.MarshalIndent(report, "", "  ")
	return os.WriteFile(outPath, reportJSON, 0644)
}

func sampleStories(stories []pack.StoryRef, n int) []AuditStory {
	if n > len(stories) {
		n = len(stories)
	}
	// Deterministic sample: sort by ID, pick evenly spaced.
	sorted := make([]pack.StoryRef, len(stories))
	copy(sorted, stories)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].ID < sorted[j].ID })

	// Use a fixed-seed RNG for reproducible sampling.
	rng := rand.New(rand.NewSource(42))
	indices := rng.Perm(len(sorted))
	out := make([]AuditStory, 0, n)
	for i := 0; i < n && i < len(indices); i++ {
		idx := indices[i]
		out = append(out, AuditStory{
			StoryID: sorted[idx].ID,
			TitleZH: sorted[idx].TitleZH,
			Band:    sorted[idx].Band,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].StoryID < out[j].StoryID })
	return out
}

// --- Audit data types ---

type AuditReport struct {
	PackID        string       `json:"pack_id"`
	LexiconVer    string       `json:"lexicon_version"`
	SchemaVer     int          `json:"schema_version"`
	TotalStories  int          `json:"total_stories"`
	Sampled       int          `json:"sampled"`
	SampleSize    int          `json:"sample_size"`
	AuditPassRate float64      `json:"audit_pass_rate"`
	AuditPassed   int          `json:"audit_passed"`
	AuditTotal    int          `json:"audit_total"`
	Generator     string       `json:"generator"`
	Model         string       `json:"model"`
	SampleStories []AuditStory `json:"sample_stories"`
	SampleAudit   []AuditStory `json:"sample_audit"`
}

type AuditStory struct {
	StoryID     string `json:"story_id"`
	TitleZH     string `json:"title_zh"`
	Band        string `json:"band"`
	VerdictPass *bool  `json:"verdict_pass,omitempty"`
	Notes       string `json:"notes,omitempty"`
}

// --- Non-negative check ---

func (r AuditReport) IsValid() bool {
	if math.IsNaN(r.AuditPassRate) {
		return false
	}
	if r.AuditPassRate < 0.0 || r.AuditPassRate > 1.0 {
		return false
	}
	if r.AuditTotal <= 0 || r.AuditPassed < 0 || r.AuditPassed > r.AuditTotal {
		return false
	}
	return true
}
