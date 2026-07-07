package pack

import (
	"math"
	"strings"
	"testing"
)

func TestVerifyAuditFieldsFixturePackPasses(t *testing.T) {
	man := &Manifest{}
	if err := verifyAuditFields(man); err != nil {
		t.Fatalf("zero audit fields should pass: %v", err)
	}
}

func TestVerifyAuditFieldsValidMetrics(t *testing.T) {
	man := &Manifest{
		AuditPassRate:   0.85,
		AuditSampleSize: 20,
		Generator:       "deepseek-rerank",
		Model:           "deepseek-chat",
	}
	if err := verifyAuditFields(man); err != nil {
		t.Fatalf("valid audit metrics should pass: %v", err)
	}
}

func TestVerifyAuditFieldsRejectNegativePassRate(t *testing.T) {
	man := &Manifest{AuditPassRate: -0.1}
	err := verifyAuditFields(man)
	if err == nil || !strings.Contains(err.Error(), "audit_pass_rate") {
		t.Fatalf("expected audit_pass_rate rejection for -0.1, got %v", err)
	}
}

func TestVerifyAuditFieldsRejectPassRateAboveOne(t *testing.T) {
	man := &Manifest{AuditPassRate: 1.5}
	err := verifyAuditFields(man)
	if err == nil || !strings.Contains(err.Error(), "audit_pass_rate") {
		t.Fatalf("expected audit_pass_rate rejection for 1.5, got %v", err)
	}
}

func TestVerifyAuditFieldsRejectNaN(t *testing.T) {
	man := &Manifest{AuditPassRate: math.NaN()}
	err := verifyAuditFields(man)
	if err == nil || !strings.Contains(err.Error(), "NaN") {
		t.Fatalf("expected NaN rejection, got %v", err)
	}
}

func TestVerifyAuditFieldsRejectNegativeSampleSize(t *testing.T) {
	man := &Manifest{AuditSampleSize: -1}
	err := verifyAuditFields(man)
	if err == nil || !strings.Contains(err.Error(), "audit_sample_size") {
		t.Fatalf("expected audit_sample_size rejection for -1, got %v", err)
	}
}

func TestVerifyAuditFieldsRejectEmptyGeneratorWithData(t *testing.T) {
	man := &Manifest{AuditPassRate: 0.5}
	err := verifyAuditFields(man)
	if err == nil || !strings.Contains(err.Error(), "generator tag") {
		t.Fatalf("expected empty generator rejection when audit data claimed, got %v", err)
	}
}

func TestVerifyAuditFieldsAllZeroIsFine(t *testing.T) {
	man := &Manifest{
		AuditPassRate:   0,
		AuditSampleSize: 0,
		Generator:       "",
	}
	if err := verifyAuditFields(man); err != nil {
		t.Fatalf("all-zero audit fields (empty sentinel) should pass: %v", err)
	}
}
