package images

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/parso/zhuwen-factory/internal/pack"
)

func TestLoadDecisions(t *testing.T) {
	dir := t.TempDir()
	base := filepath.Join(dir, "base.json")
	overrides := filepath.Join(dir, "overrides.json")

	write := func(path string, v any) {
		b, _ := json.Marshal(v)
		os.WriteFile(path, b, 0o644)
	}

	write(base, []ImageDecision{
		{Word: "水", Simp: "水", Decision: "File:Water.jpg", Status: "commons"},
		{Word: "火", Simp: "火", Decision: "File:Fire.jpg", Status: "commons"},
	})
	write(overrides, []ImageDecision{
		{Word: "水", Simp: "水", Decision: "File:BetterWater.jpg", Status: "commons"},
	})

	decisions, err := LoadDecisions(base, overrides)
	if err != nil {
		t.Fatal(err)
	}
	if len(decisions) != 2 {
		t.Fatalf("got %d decisions, want 2", len(decisions))
	}

	var waterDec, fireDec ImageDecision
	for _, d := range decisions {
		switch d.Simp {
		case "水":
			waterDec = d
		case "火":
			fireDec = d
		}
	}

	// Override wins.
	if waterDec.Decision != "File:BetterWater.jpg" {
		t.Errorf("water decision = %q, want 'File:BetterWater.jpg' (override must win)", waterDec.Decision)
	}
	// Unchanged.
	if fireDec.Decision != "File:Fire.jpg" {
		t.Errorf("fire decision = %q, want 'File:Fire.jpg'", fireDec.Decision)
	}
}

func TestLoadDecisionsEmptyFile(t *testing.T) {
	dir := t.TempDir()
	empty := filepath.Join(dir, "empty.json")
	os.WriteFile(empty, []byte("[]"), 0o644)
	decisions, err := LoadDecisions(empty)
	if err != nil {
		t.Fatal(err)
	}
	if len(decisions) != 0 {
		t.Errorf("got %d decisions from empty file, want 0", len(decisions))
	}
}

func TestLoadDecisionsEmptyPath(t *testing.T) {
	decisions, err := LoadDecisions("")
	if err != nil {
		t.Fatal(err)
	}
	if len(decisions) != 0 {
		t.Errorf("got %d decisions from empty path, want 0", len(decisions))
	}
}

func TestCommonsTitle(t *testing.T) {
	tests := []struct {
		decision string
		status   string
		want     string
	}{
		{"File:Foo.jpg", "commons", "File:Foo.jpg"},
		{"https://commons.wikimedia.org/wiki/File:Bar.jpg", "custom", "File:Bar.jpg"},
		{"File:Some Thing.png", "", "File:Some Thing.png"},
	}
	for _, tt := range tests {
		d := ImageDecision{Decision: tt.decision, Status: tt.status}
		if got := d.CommonsTitle(); got != tt.want {
			t.Errorf("CommonsTitle(%q, %q) = %q, want %q", tt.decision, tt.status, got, tt.want)
		}
	}
}

func TestDecisionsToImages(t *testing.T) {
	prov := ProvenanceStore{
		"File:Water.jpg": {File: "File:Water.jpg", License: "CC0", LicenseURL: "https://creativecommons.org/publicdomain/zero/1.0/", Author: "Alice", SourceURL: "https://commons.wikimedia.org/wiki/File:Water.jpg", RetrievedAt: "2026-07-01", W: 2000, H: 1500},
		"File:Fire.jpg":  {File: "File:Fire.jpg", License: "CC-BY 4.0", LicenseURL: "https://creativecommons.org/licenses/by/4.0/", Author: "Bob", SourceURL: "https://commons.wikimedia.org/wiki/File:Fire.jpg", RetrievedAt: "2026-07-01", W: 1500, H: 1500},
	}

	decisions := []ImageDecision{
		{Word: "水", Simp: "水", Decision: "File:Water.jpg", Status: "commons"},
		{Word: "火", Simp: "火", Decision: "File:Fire.jpg", Status: "commons"},
	}

	wordIDs := map[string]int{"水": 1, "火": 2}

	imgs, err := DecisionsToImages(decisions, prov, wordIDs)
	if err != nil {
		t.Fatal(err)
	}
	if len(imgs) != 2 {
		t.Fatalf("got %d images, want 2", len(imgs))
	}

	im1 := imgs[0]
	if *im1.WordID != 1 {
		t.Errorf("水 WordID = %d, want 1", *im1.WordID)
	}
	if im1.License != "CC0" {
		t.Errorf("水 License = %q, want CC0", im1.License)
	}

	im2 := imgs[1]
	if *im2.WordID != 2 {
		t.Errorf("火 WordID = %d, want 2", *im2.WordID)
	}
	if im2.Author != "Bob" {
		t.Errorf("火 Author = %q, want Bob", im2.Author)
	}
}

func TestDecisionsToImagesMissingProvenance(t *testing.T) {
	prov := ProvenanceStore{}
	decisions := []ImageDecision{
		{Word: "水", Simp: "水", Decision: "File:Water.jpg", Status: "commons"},
	}
	wordIDs := map[string]int{"水": 1}
	_, err := DecisionsToImages(decisions, prov, wordIDs)
	if err == nil {
		t.Error("expected error for missing provenance")
	}
}

func TestDecisionsToImagesMissingWord(t *testing.T) {
	prov := ProvenanceStore{
		"File:Water.jpg": {File: "File:Water.jpg", License: "CC0", LicenseURL: "https://creativecommons.org/publicdomain/zero/1.0/", Author: "Alice", SourceURL: "https://commons.wikimedia.org/wiki/File:Water.jpg", RetrievedAt: "2026-07-01", W: 2000, H: 1500},
	}
	decisions := []ImageDecision{
		{Word: "水", Simp: "水", Decision: "File:Water.jpg", Status: "commons"},
	}
	wordIDs := map[string]int{} // empty
	_, err := DecisionsToImages(decisions, prov, wordIDs)
	if err == nil {
		t.Error("expected error for missing word in lexicon")
	}
}

func TestJoinResult(t *testing.T) {
	// Setup: pipeline result with a stub image.
	p := pack.Pack{
		ID: "test-pack",
		Images: []pack.Image{
			{ID: "img-C1-shouzhudaitu", CanonID: "C1-shouzhudaitu", File: "images/img-C1-shouzhudaitu@480.heic", W: 480, H: 480, License: "PD", LicenseURL: "https://creativecommons.org/publicdomain/mark/1.0/", Author: "Stub", SourceURL: "https://example.com/stub", RetrievedAt: "2026-01-01"},
		},
		Stories: []pack.Story{
			{ID: "C1-shouzhudaitu-A2", CanonID: "C1-shouzhudaitu", CoverImageID: "img-C1-shouzhudaitu"},
		},
	}

	curated := []pack.Image{
		{CanonID: "C1-shouzhudaitu", License: "CC0", LicenseURL: "https://creativecommons.org/publicdomain/zero/1.0/", Author: "Real Author", SourceURL: "https://commons.wikimedia.org/wiki/File:Real.jpg", RetrievedAt: "2026-07-01", W: 2000, H: 1500},
	}

	result, err := JoinResult(p, curated, nil)
	if err != nil {
		t.Fatal(err)
	}

	// The stub should be replaced with curated data, ID preserved.
	var found pack.Image
	for _, im := range result.Images {
		if im.ID == "img-C1-shouzhudaitu" {
			found = im
			break
		}
	}
	if found.License != "CC0" {
		t.Errorf("license = %q, want CC0 (curated)", found.License)
	}
	if found.Author != "Real Author" {
		t.Errorf("author = %q, want 'Real Author'", found.Author)
	}
	if found.W != 2000 {
		t.Errorf("W = %d, want 2000", found.W)
	}

	// Story CoverImageID should still point to the same image.
	if result.Stories[0].CoverImageID != "img-C1-shouzhudaitu" {
		t.Errorf("story CoverImageID = %q, want img-C1-shouzhudaitu", result.Stories[0].CoverImageID)
	}
}

func TestJoinResultWithFoundations(t *testing.T) {
	p := pack.Pack{
		ID:     "test-pack",
		Images: nil,
	}

	foundations := []pack.Image{
		{ID: "img-foundations-水", WordID: intPtr(1), File: "images/img-foundations-水@480.heic", License: "CC0", LicenseURL: "https://creativecommons.org/publicdomain/zero/1.0/", Author: "Alice", SourceURL: "https://commons.wikimedia.org/wiki/File:Water.jpg", RetrievedAt: "2026-07-01", W: 2000, H: 1500},
	}

	result, err := JoinResult(p, nil, foundations)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Images) != 1 {
		t.Fatalf("got %d images, want 1", len(result.Images))
	}
	if result.Images[0].ID != "img-foundations-水" {
		t.Errorf("img ID = %q, want img-foundations-水", result.Images[0].ID)
	}
}

func intPtr(i int) *int { return &i }
