package iam

import (
	"context"
	"strings"
	"testing"

	"github.com/tfiam-dev/tfiam/internal/plan"
	"github.com/tfiam-dev/tfiam/internal/report"
)

func singleRequired(action, confidence string) []plan.RequiredAction {
	return []plan.RequiredAction{
		{Action: action, ResourceAddress: "aws_s3_bucket.test", ResourceArn: "*", Confidence: confidence},
	}
}

func TestChecker_Denied_HighConfidence(t *testing.T) {
	src, _ := NewFileSource("../../testdata/permissions/s3_denied.yaml")
	checker := NewChecker(src)
	principal := Principal{ARN: "arn:aws:iam::123456789012:role/deployer"}

	findings, err := checker.Check(context.Background(), principal, singleRequired("s3:CreateBucket", "high"))
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if len(findings) != 1 || findings[0].Decision != report.FindingMissing {
		t.Errorf("expected 1 MISSING finding, got %v", findings)
	}
	if findings[0].Confidence != "high" {
		t.Errorf("expected confidence=high, got %q", findings[0].Confidence)
	}
}

func TestChecker_AllAllowed_NoFindings(t *testing.T) {
	src, _ := NewFileSource("../../testdata/permissions/all_allowed.yaml")
	checker := NewChecker(src)
	principal := Principal{ARN: "arn:aws:iam::123456789012:role/deployer"}

	required := []plan.RequiredAction{
		{Action: "s3:CreateBucket", ResourceAddress: "aws_s3_bucket.test", ResourceArn: "*", Confidence: "high"},
		{Action: "s3:PutBucketTagging", ResourceAddress: "aws_s3_bucket.test", ResourceArn: "*", Confidence: "best-effort"},
	}

	findings, err := checker.Check(context.Background(), principal, required)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if len(findings) != 0 {
		t.Errorf("expected 0 findings for all-allowed, got %v", findings)
	}
}

func TestChecker_Unknown_FindingIsUnknown(t *testing.T) {
	src, _ := NewFileSource("../../testdata/permissions/all_unknown.yaml")
	checker := NewChecker(src)
	principal := Principal{ARN: "arn:aws:iam::123456789012:role/deployer"}

	findings, err := checker.Check(context.Background(), principal, singleRequired("s3:CreateBucket", "high"))
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if len(findings) != 1 || findings[0].Decision != report.FindingUnknown {
		t.Errorf("expected 1 Unknown finding, got %v", findings)
	}
	// Unknown findings should be best-effort regardless of input confidence
	if findings[0].Confidence != "best-effort" {
		t.Errorf("expected confidence=best-effort for Unknown, got %q", findings[0].Confidence)
	}
}

func TestChecker_PrincipalMismatch_ReturnsError(t *testing.T) {
	src, _ := NewFileSource("../../testdata/permissions/principal_mismatch.yaml")
	checker := NewChecker(src)
	// This principal does NOT match the one in principal_mismatch.yaml
	principal := Principal{ARN: "arn:aws:iam::123456789012:role/deployer"}

	_, err := checker.Check(context.Background(), principal, singleRequired("s3:CreateBucket", "high"))
	if err == nil {
		t.Fatal("expected error on principal mismatch, got nil")
	}
	if !strings.Contains(err.Error(), "principal mismatch") {
		t.Errorf("expected 'principal mismatch' in error, got: %v", err)
	}
}
