package report

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestSuggestPolicy_WithMissingActions(t *testing.T) {
	r := New(3)
	r.AddPermFindings([]PermFinding{
		{Action: "iam:CreateRole", Decision: FindingMissing, Confidence: "high", Source: "test"},
		{Action: "s3:CreateBucket", Decision: FindingMissing, Confidence: "high", Source: "test"},
		{Action: "s3:PutBucketTagging", Decision: FindingMissing, Confidence: "best-effort", Source: "test"},
	})

	var buf bytes.Buffer
	r.SuggestPolicy(&buf)
	out := buf.String()

	if !strings.Contains(out, "Suggested IAM policy") {
		t.Errorf("expected header in output, got: %s", out)
	}

	// Extract just the JSON part
	jsonStart := strings.Index(out, "{")
	if jsonStart == -1 {
		t.Fatal("no JSON found in output")
	}

	var policy struct {
		Version   string `json:"Version"`
		Statement []struct {
			Action   []string `json:"Action"`
			Resource string   `json:"Resource"`
		} `json:"Statement"`
	}
	if err := json.Unmarshal([]byte(out[jsonStart:]), &policy); err != nil {
		t.Fatalf("invalid JSON: %v\noutput: %s", err, out)
	}

	if policy.Version != "2012-10-17" {
		t.Errorf("expected Version 2012-10-17, got %s", policy.Version)
	}
	if len(policy.Statement) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(policy.Statement))
	}

	actionSet := make(map[string]bool)
	for _, a := range policy.Statement[0].Action {
		actionSet[a] = true
	}
	// Only high-confidence actions should appear
	if !actionSet["iam:CreateRole"] || !actionSet["s3:CreateBucket"] {
		t.Errorf("expected high-confidence actions, got %v", policy.Statement[0].Action)
	}
	// best-effort should be excluded
	if actionSet["s3:PutBucketTagging"] {
		t.Errorf("best-effort action should not appear in suggested policy")
	}
	// Actions should be sorted
	if policy.Statement[0].Action[0] != "iam:CreateRole" {
		t.Errorf("expected sorted actions, got %v", policy.Statement[0].Action)
	}
}

func TestSuggestPolicy_NoMissingActions(t *testing.T) {
	r := New(2)
	r.AddPermFindings([]PermFinding{
		{Action: "s3:CreateBucket", Decision: FindingUnknown, Confidence: "high", Source: "test"},
	})

	var buf bytes.Buffer
	r.SuggestPolicy(&buf)
	if buf.Len() != 0 {
		t.Errorf("expected no output when no missing actions, got: %s", buf.String())
	}
}
