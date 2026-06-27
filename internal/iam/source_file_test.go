package iam

import (
	"context"
	"testing"
)

var testPrincipal = Principal{ARN: "arn:aws:iam::123456789012:role/deployer"}

func TestFileSource_AllowedWildcard(t *testing.T) {
	src, err := NewFileSource("../../testdata/permissions/all_allowed.yaml")
	if err != nil {
		t.Fatalf("NewFileSource: %v", err)
	}

	d, err := src.Evaluate(context.Background(), testPrincipal, "s3:CreateBucket", "*")
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if d != DecisionAllowed {
		t.Errorf("expected Allowed, got %s", d)
	}
}

func TestFileSource_DeniedWildcard(t *testing.T) {
	src, err := NewFileSource("../../testdata/permissions/s3_denied.yaml")
	if err != nil {
		t.Fatalf("NewFileSource: %v", err)
	}

	d, err := src.Evaluate(context.Background(), testPrincipal, "s3:CreateBucket", "*")
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if d != DecisionDenied {
		t.Errorf("expected Denied, got %s", d)
	}
}

func TestFileSource_UnknownWhenNotInFile(t *testing.T) {
	src, err := NewFileSource("../../testdata/permissions/all_allowed.yaml")
	if err != nil {
		t.Fatalf("NewFileSource: %v", err)
	}

	// iam:CreateRole is not in all_allowed.yaml
	d, err := src.Evaluate(context.Background(), testPrincipal, "iam:CreateRole", "*")
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if d != DecisionUnknown {
		t.Errorf("expected Unknown for unregistered action, got %s", d)
	}
}

func TestFileSource_ExactMatchOverWildcard(t *testing.T) {
	// exact_match.yaml has s3:CreateBucket only for specific ARN
	src, err := NewFileSource("../../testdata/permissions/exact_match.yaml")
	if err != nil {
		t.Fatalf("NewFileSource: %v", err)
	}

	// Stage 1: exact match should succeed
	d, err := src.Evaluate(context.Background(), testPrincipal, "s3:CreateBucket", "arn:aws:s3:::specific-bucket")
	if err != nil {
		t.Fatalf("Evaluate (exact): %v", err)
	}
	if d != DecisionAllowed {
		t.Errorf("expected Allowed on exact match, got %s", d)
	}

	// Stage 2: wildcard fallback → no wildcard entry → Unknown
	d, err = src.Evaluate(context.Background(), testPrincipal, "s3:CreateBucket", "*")
	if err != nil {
		t.Fatalf("Evaluate (wildcard): %v", err)
	}
	if d != DecisionUnknown {
		t.Errorf("expected Unknown when no wildcard entry, got %s", d)
	}

	// Stage 3: completely different ARN → Unknown
	d, err = src.Evaluate(context.Background(), testPrincipal, "s3:CreateBucket", "arn:aws:s3:::other-bucket")
	if err != nil {
		t.Fatalf("Evaluate (miss): %v", err)
	}
	if d != DecisionUnknown {
		t.Errorf("expected Unknown on ARN miss, got %s", d)
	}
}

func TestFileSource_PrincipalMismatch_ReturnsError(t *testing.T) {
	src, err := NewFileSource("../../testdata/permissions/principal_mismatch.yaml")
	if err != nil {
		t.Fatalf("NewFileSource: %v", err)
	}

	_, err = src.Evaluate(context.Background(), testPrincipal, "s3:CreateBucket", "*")
	if err == nil {
		t.Fatal("expected error on principal mismatch, got nil")
	}
}

func TestFileSource_AllUnknown(t *testing.T) {
	src, err := NewFileSource("../../testdata/permissions/all_unknown.yaml")
	if err != nil {
		t.Fatalf("NewFileSource: %v", err)
	}

	d, err := src.Evaluate(context.Background(), testPrincipal, "s3:CreateBucket", "*")
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if d != DecisionUnknown {
		t.Errorf("expected Unknown for empty perms file, got %s", d)
	}
}

func TestFileSource_Name(t *testing.T) {
	src, err := NewFileSource("../../testdata/permissions/all_allowed.yaml")
	if err != nil {
		t.Fatalf("NewFileSource: %v", err)
	}
	if src.Name() != "file" {
		t.Errorf("expected Name()='file', got %q", src.Name())
	}
}
