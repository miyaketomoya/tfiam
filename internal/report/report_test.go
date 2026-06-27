package report

import (
	"bytes"
	"strings"
	"testing"
)

func TestReport_NoFindings_Exit0(t *testing.T) {
	r := New(0)
	var buf bytes.Buffer
	code := r.Render(&buf, "text")

	if code != 0 {
		t.Errorf("expected exit 0 with no findings, got %d", code)
	}
	if !strings.Contains(buf.String(), "No statically-detectable issues found") {
		t.Errorf("expected success message, got: %q", buf.String())
	}
}

func TestReport_HighConfidenceMissing_Exit1(t *testing.T) {
	r := New(1) // 1 required action, 1 denied
	r.AddPermFindings([]PermFinding{
		{Action: "s3:CreateBucket", ResourceAddress: "aws_s3_bucket.test", ResourceArn: "*", Decision: FindingMissing, Confidence: "high", Source: "file"},
	})

	var buf bytes.Buffer
	code := r.Render(&buf, "text")

	if code != 1 {
		t.Errorf("expected exit 1 for high-confidence missing, got %d", code)
	}
	if !strings.Contains(buf.String(), "MISSING: s3:CreateBucket") {
		t.Errorf("expected MISSING output, got: %q", buf.String())
	}
}

func TestReport_BestEffortOnly_Exit0WithWarning(t *testing.T) {
	r := New(1) // 1 required, 1 best-effort denied (evaluated, not unknown)
	r.AddPermFindings([]PermFinding{
		{Action: "s3:CreateBucket", ResourceAddress: "aws_s3_bucket.test", ResourceArn: "*", Decision: FindingMissing, Confidence: "best-effort", Source: "file"},
	})

	var buf bytes.Buffer
	code := r.Render(&buf, "text")

	if code != 0 {
		t.Errorf("expected exit 0 for best-effort only, got %d", code)
	}
	out := buf.String()
	if !strings.Contains(out, "WARNING") {
		t.Errorf("expected WARNING for best-effort denied, got: %q", out)
	}
}

func TestReport_UnknownFindings_ShowsWarningCount(t *testing.T) {
	// 2 required actions: 1 allowed (no finding) + 1 unknown → exit 0 + warning
	r := New(2)
	r.AddPermFindings([]PermFinding{
		{Action: "s3:CreateBucket", ResourceAddress: "aws_s3_bucket.test", ResourceArn: "*", Decision: FindingUnknown, Confidence: "best-effort", Source: "file"},
	})

	var buf bytes.Buffer
	code := r.Render(&buf, "text")

	if code != 0 {
		t.Errorf("expected exit 0 for partial unknown (1/2 evaluated), got %d", code)
	}
	out := buf.String()
	if !strings.Contains(out, "were not evaluated") {
		t.Errorf("expected unknown count warning, got: %q", out)
	}
}

func TestReport_AllUnknown_Exit2(t *testing.T) {
	r := New(2) // 2 required, all unknown → exit 2
	r.AddPermFindings([]PermFinding{
		{Action: "s3:CreateBucket", Decision: FindingUnknown, Confidence: "best-effort", Source: "file"},
		{Action: "iam:CreateRole", Decision: FindingUnknown, Confidence: "best-effort", Source: "file"},
	})

	var buf bytes.Buffer
	code := r.Render(&buf, "text")

	if code != 2 {
		t.Errorf("expected exit 2 when all actions unknown, got %d", code)
	}
	if !strings.Contains(buf.String(), "ERROR: 0/") {
		t.Errorf("expected ERROR output for all-unknown, got: %q", buf.String())
	}
}

func TestReport_NamingViolation_Exit1(t *testing.T) {
	r := New(0)
	r.AddNamingFindings([]NamingFinding{
		{ResourceAddress: "aws_s3_bucket.foo", NameField: "bucket", Name: strings.Repeat("a", 64), Kind: NamingViolation, Detail: "name exceeds 63 chars"},
	})

	var buf bytes.Buffer
	code := r.Render(&buf, "text")

	if code != 1 {
		t.Errorf("expected exit 1 for naming violation, got %d", code)
	}
	if !strings.Contains(buf.String(), "NAMING:") {
		t.Errorf("expected NAMING: output, got: %q", buf.String())
	}
}

func TestReport_NamingSkipped_Exit0(t *testing.T) {
	r := New(0)
	r.AddNamingFindings([]NamingFinding{
		{ResourceAddress: "aws_s3_bucket.computed", NameField: "bucket", Kind: NamingSkipped, Detail: "name known after apply"},
	})

	var buf bytes.Buffer
	code := r.Render(&buf, "text")

	if code != 0 {
		t.Errorf("expected exit 0 for naming-skipped, got %d", code)
	}
	if !strings.Contains(buf.String(), "NAMING-SKIPPED:") {
		t.Errorf("expected NAMING-SKIPPED: output, got: %q", buf.String())
	}
}

func TestReport_ExitPriority_DeniedOverUnknown(t *testing.T) {
	// high-confidence denied + unknown → exit 1 (denied takes priority over exit 2)
	r := New(2)
	r.AddPermFindings([]PermFinding{
		{Action: "s3:CreateBucket", Decision: FindingMissing, Confidence: "high", Source: "file"},
		{Action: "iam:CreateRole", Decision: FindingUnknown, Confidence: "best-effort", Source: "file"},
	})

	var buf bytes.Buffer
	code := r.Render(&buf, "text")

	if code != 1 {
		t.Errorf("expected exit 1 (denied takes priority), got %d", code)
	}
}

func TestReport_JSONFormat(t *testing.T) {
	r := New(1)
	r.AddPermFindings([]PermFinding{
		{Action: "s3:CreateBucket", ResourceAddress: "aws_s3_bucket.test", Decision: FindingMissing, Confidence: "high", Source: "file"},
	})

	var buf bytes.Buffer
	code := r.Render(&buf, "json")

	if code != 1 {
		t.Errorf("expected exit 1, got %d", code)
	}
	out := buf.String()
	if !strings.Contains(out, `"exit_code"`) {
		t.Errorf("expected JSON output with exit_code field, got: %q", out)
	}
}
