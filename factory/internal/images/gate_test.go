package images

import (
	"testing"
)

func TestClassifyLicense(t *testing.T) {
	tests := []struct {
		lic  string
		want LicenseKind
	}{
		// Accepted licenses
		{"Public Domain", LicensePD},
		{"pd", LicensePD},
		{"CC0", LicenseCC0},
		{"Cc-zero", LicenseCC0},
		{"CC-BY 4.0", LicenseCCBY},
		{"CC BY 3.0", LicenseCCBY},
		{"CC BY-SA 4.0", LicenseCCBYSA},
		{"cc-by-sa-2.0", LicenseCCBYSA},
		{"CC BY SA", LicenseCCBYSA},

		// Rejected licenses
		{"CC BY-NC 4.0", LicenseNC},
		{"CC BY-NC-ND 2.0", LicenseNC},
		{"CC BY-ND 4.0", LicenseND},
		{"GFDL", LicenseGFDL},
		{"GNU Free Documentation License", LicenseGFDL},
		{"all rights reserved", LicenseOther},
		{"fair use", LicenseOther},
		{"", LicenseUnknown},
		{"some random text", LicenseOther},
	}
	for _, tt := range tests {
		got := ClassifyLicense(tt.lic)
		if got != tt.want {
			t.Errorf("ClassifyLicense(%q) = %d, want %d", tt.lic, got, tt.want)
		}
	}
}

func TestLicenseIsAcceptable(t *testing.T) {
	tests := []struct {
		kind       LicenseKind
		acceptable bool
	}{
		{LicensePD, true},
		{LicenseCC0, true},
		{LicenseCCBY, true},
		{LicenseCCBYSA, true},
		{LicenseNC, false},
		{LicenseND, false},
		{LicenseGFDL, false},
		{LicenseOther, false},
		{LicenseUnknown, false},
	}
	for _, tt := range tests {
		if tt.kind.IsAcceptable() != tt.acceptable {
			t.Errorf("IsAcceptable(%d) = %v, want %v", tt.kind, tt.kind.IsAcceptable(), tt.acceptable)
		}
	}
}

func TestGateAcceptsValidCandidates(t *testing.T) {
	tests := []struct {
		name string
		c    Candidate
	}{
		{"CC-BY large", Candidate{Title: "File:Foo.jpg", DescURL: "https://commons.example/Foo", Author: "A Photographer", License: "CC-BY 4.0", LicenseURL: "https://creativecommons.org/licenses/by/4.0/", W: 2000, H: 1500}},
		{"CC0 large", Candidate{Title: "File:Bar.jpg", DescURL: "https://commons.example/Bar", Author: "B Photographer", License: "CC0", LicenseURL: "https://creativecommons.org/publicdomain/zero/1.0/", W: 3000, H: 2000}},
		{"PD large", Candidate{Title: "File:Baz.jpg", DescURL: "https://commons.example/Baz", Author: "C Photographer", License: "Public Domain", LicenseURL: "https://creativecommons.org/publicdomain/mark/1.0/", W: 4000, H: 3000}},
		{"CC-BY-SA exact", Candidate{Title: "File:Qux.jpg", DescURL: "https://commons.example/Qux", Author: "D Photographer", License: "CC BY-SA 4.0", LicenseURL: "https://creativecommons.org/licenses/by-sa/4.0/", W: 1200, H: 1200}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gr := Gate(tt.c)
			if !gr.Accepted {
				t.Errorf("Gate accepted=false, reject=%s: %s", gr.RejectCode, gr.RejectWhy)
			}
		})
	}
}

func TestGateRejectsInvalidCandidates(t *testing.T) {
	tests := []struct {
		name     string
		c        Candidate
		wantCode GateRejectCode
	}{
		{"AI-generated", Candidate{Title: "File:AI.jpg", DescURL: "https://c.example/AI", Author: "Bot", License: "CC0", LicenseURL: "https://creativecommons.org/publicdomain/zero/1.0/", W: 2000, H: 1500, Categories: []string{"Category:AI-generated images"}}, RejectAICategory},
		{"NC license", Candidate{Title: "File:NC.jpg", DescURL: "https://c.example/NC", Author: "Someone", License: "CC BY-NC 4.0", LicenseURL: "https://creativecommons.org/licenses/by-nc/4.0/", W: 2000, H: 1500}, RejectLicense},
		{"ND license", Candidate{Title: "File:ND.jpg", DescURL: "https://c.example/ND", Author: "Someone", License: "CC BY-ND 4.0", LicenseURL: "https://creativecommons.org/licenses/by-nd/4.0/", W: 2000, H: 1500}, RejectLicense},
		{"GFDL-only", Candidate{Title: "File:GFDL.jpg", DescURL: "https://c.example/GFDL", Author: "Someone", License: "GFDL", LicenseURL: "https://gnu.org/fdl", W: 2000, H: 1500}, RejectLicense},
		{"empty license", Candidate{Title: "File:Empty.jpg", DescURL: "https://c.example/Empty", Author: "Someone", License: "", LicenseURL: "", W: 2000, H: 1500}, RejectLicense},
		{"low resolution", Candidate{Title: "File:Small.jpg", DescURL: "https://c.example/Small", Author: "Someone", License: "CC0", LicenseURL: "https://creativecommons.org/publicdomain/zero/1.0/", W: 640, H: 480}, RejectLowRes},
		{"below floor", Candidate{Title: "File:Below.jpg", DescURL: "https://c.example/Below", Author: "Someone", License: "CC0", LicenseURL: "https://creativecommons.org/publicdomain/zero/1.0/", W: 1199, H: 2000}, RejectLowRes},
		{"fair use", Candidate{Title: "File:Fair.jpg", DescURL: "https://c.example/Fair", Author: "Someone", License: "fair use", LicenseURL: "", W: 2000, H: 1500}, RejectLicense},
		{"all rights reserved", Candidate{Title: "File:ARR.jpg", DescURL: "https://c.example/ARR", Author: "Someone", License: "all rights reserved", LicenseURL: "", W: 2000, H: 1500}, RejectLicense},
		{"missing desc URL", Candidate{Title: "File:NoURL.jpg", DescURL: "", Author: "Someone", License: "CC0", LicenseURL: "https://creativecommons.org/publicdomain/zero/1.0/", W: 2000, H: 1500}, RejectMissingURL},
		{"missing author", Candidate{Title: "File:NoAuthor.jpg", DescURL: "https://c.example/NoAuthor", Author: "", License: "CC0", LicenseURL: "https://creativecommons.org/publicdomain/zero/1.0/", W: 2000, H: 1500}, RejectMissingAuthor},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gr := Gate(tt.c)
			if gr.Accepted {
				t.Errorf("Gate accepted=true, expected reject %s", tt.wantCode)
			}
			if gr.RejectCode != tt.wantCode {
				t.Errorf("Gate reject code = %s, want %s; why=%s", gr.RejectCode, tt.wantCode, gr.RejectWhy)
			}
		})
	}
}

func TestGateScoring(t *testing.T) {
	// PD should score higher than CC-BY which should score higher than CC-BY-SA, all else equal.
	pd := Candidate{Title: "PD", DescURL: "https://c.example/PD", Author: "X", License: "Public Domain", LicenseURL: "https://creativecommons.org/publicdomain/mark/1.0/", W: 2000, H: 1500}
	ccby := Candidate{Title: "BY", DescURL: "https://c.example/BY", Author: "X", License: "CC-BY 4.0", LicenseURL: "https://creativecommons.org/licenses/by/4.0/", W: 2000, H: 1500}
	ccbysa := Candidate{Title: "SA", DescURL: "https://c.example/SA", Author: "X", License: "CC BY-SA 4.0", LicenseURL: "https://creativecommons.org/licenses/by-sa/4.0/", W: 2000, H: 1500}
	cc0 := Candidate{Title: "CC0", DescURL: "https://c.example/CC0", Author: "X", License: "CC0", LicenseURL: "https://creativecommons.org/publicdomain/zero/1.0/", W: 2000, H: 1500}

	_, _, _, scores := GateCandidates([]Candidate{pd, ccby, ccbysa, cc0})
	if scores["PD"] <= scores["BY"] {
		t.Errorf("PD score %f <= CC-BY score %f", scores["PD"], scores["BY"])
	}
	if scores["BY"] <= scores["SA"] {
		t.Errorf("CC-BY score %f <= CC-BY-SA score %f", scores["BY"], scores["SA"])
	}
	if scores["CC0"] <= scores["BY"] {
		t.Errorf("CC0 score %f <= CC-BY score %f", scores["CC0"], scores["BY"])
	}
}

func TestGateCandidatesPartitionsCorrectly(t *testing.T) {
	cands := []Candidate{
		{Title: "Good1", DescURL: "https://c.example/1", Author: "A", License: "CC0", LicenseURL: "https://creativecommons.org/publicdomain/zero/1.0/", W: 3000, H: 2000},
		{Title: "Good2", DescURL: "https://c.example/2", Author: "A", License: "CC-BY 4.0", LicenseURL: "https://creativecommons.org/licenses/by/4.0/", W: 2000, H: 1500},
		{Title: "Bad1", DescURL: "https://c.example/3", Author: "A", License: "CC BY-NC 4.0", LicenseURL: "https://creativecommons.org/licenses/by-nc/4.0/", W: 2000, H: 1500},
		{Title: "Bad2", DescURL: "https://c.example/4", Author: "A", License: "CC0", LicenseURL: "https://creativecommons.org/publicdomain/zero/1.0/", W: 640, H: 480},
		{Title: "Good3", DescURL: "https://c.example/5", Author: "A", License: "Public Domain", LicenseURL: "https://creativecommons.org/publicdomain/mark/1.0/", W: 4000, H: 3000},
	}
	best, alts, rejects, _ := GateCandidates(cands)

	if best == nil {
		t.Fatal("expected a best pick")
	}
	if best.Title != "Good3" { // PD + highest resolution = highest score
		t.Errorf("best = %s, want Good3 (PD + 4000x3000)", best.Title)
	}
	if len(alts) != 2 {
		t.Errorf("got %d alternates, want 2", len(alts))
	}
	if len(rejects) != 2 {
		t.Errorf("got %d rejects, want 2", len(rejects))
	}
}

func TestGateCandidatesEmpty(t *testing.T) {
	best, alts, rejects, _ := GateCandidates(nil)
	if best != nil {
		t.Error("expected nil best for empty input")
	}
	if len(alts) != 0 || len(rejects) != 0 {
		t.Error("expected empty alts/rejects for empty input")
	}
}

func TestGateCandidatesAllRejected(t *testing.T) {
	cands := []Candidate{
		{Title: "Bad1", DescURL: "https://c.example/1", Author: "A", License: "CC BY-NC 4.0", LicenseURL: "https://creativecommons.org/licenses/by-nc/4.0/", W: 2000, H: 1500},
		{Title: "Bad2", DescURL: "https://c.example/2", Author: "A", License: "GFDL", LicenseURL: "https://gnu.org/fdl", W: 2000, H: 1500},
	}
	best, alts, rejects, _ := GateCandidates(cands)
	if best != nil {
		t.Error("expected nil best when all rejected")
	}
	if len(alts) != 0 {
		t.Errorf("expected 0 alternates, got %d", len(alts))
	}
	if len(rejects) != 2 {
		t.Errorf("expected 2 rejects, got %d", len(rejects))
	}
}
