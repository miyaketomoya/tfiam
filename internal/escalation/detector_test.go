package escalation

import (
	"encoding/json"
	"testing"

	tfjson "github.com/hashicorp/terraform-json"
)

func makeChange(resourceType, address string, after map[string]interface{}) *tfjson.ResourceChange {
	afterJSON, _ := json.Marshal(after)
	var afterVal interface{}
	json.Unmarshal(afterJSON, &afterVal)
	return &tfjson.ResourceChange{
		Type:    resourceType,
		Address: address,
		Change: &tfjson.Change{
			Actions: tfjson.Actions{tfjson.ActionCreate},
			After:   afterVal,
		},
	}
}

func makePlan(rcs ...*tfjson.ResourceChange) *tfjson.Plan {
	return &tfjson.Plan{ResourceChanges: rcs}
}

func TestDetect_AdministratorAccess(t *testing.T) {
	rc := makeChange("aws_iam_role_policy_attachment", "aws_iam_role_policy_attachment.admin", map[string]interface{}{
		"policy_arn": "arn:aws:iam::aws:policy/AdministratorAccess",
		"role":       "my-role",
	})
	findings := Detect(makePlan(rc))
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].Rule != "dangerous-managed-policy" {
		t.Errorf("expected rule dangerous-managed-policy, got %s", findings[0].Rule)
	}
	if findings[0].Risk != RiskHigh {
		t.Errorf("expected HIGH risk, got %s", findings[0].Risk)
	}
}

func TestDetect_WildcardAction(t *testing.T) {
	policy := `{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Action":"*","Resource":"*"}]}`
	rc := makeChange("aws_iam_role_policy", "aws_iam_role_policy.full", map[string]interface{}{
		"policy": policy,
		"name":   "full-access",
		"role":   "my-role",
	})
	findings := Detect(makePlan(rc))
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].Rule != "wildcard-action" {
		t.Errorf("expected wildcard-action, got %s", findings[0].Rule)
	}
}

func TestDetect_IAMWildcard(t *testing.T) {
	policy := `{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Action":"iam:*","Resource":"*"}]}`
	rc := makeChange("aws_iam_policy", "aws_iam_policy.iam_full", map[string]interface{}{
		"policy": policy,
		"name":   "iam-full",
	})
	findings := Detect(makePlan(rc))
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].Rule != "iam-wildcard" {
		t.Errorf("expected iam-wildcard, got %s", findings[0].Rule)
	}
	if findings[0].Risk != RiskHigh {
		t.Errorf("expected HIGH, got %s", findings[0].Risk)
	}
}

func TestDetect_PrivilegeEscalationCombo(t *testing.T) {
	policy := `{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Action":["iam:CreateRole","iam:AttachRolePolicy","iam:PassRole"],"Resource":"*"}]}`
	rc := makeChange("aws_iam_policy", "aws_iam_policy.escalation", map[string]interface{}{
		"policy": policy,
		"name":   "escalation-policy",
	})
	findings := Detect(makePlan(rc))
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].Rule != "privilege-escalation-combo" {
		t.Errorf("expected privilege-escalation-combo, got %s", findings[0].Rule)
	}
	if findings[0].Risk != RiskMedium {
		t.Errorf("expected MEDIUM, got %s", findings[0].Risk)
	}
}

func TestDetect_SafePolicy(t *testing.T) {
	policy := `{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Action":["s3:GetObject","s3:PutObject"],"Resource":"arn:aws:s3:::my-bucket/*"}]}`
	rc := makeChange("aws_iam_role_policy", "aws_iam_role_policy.safe", map[string]interface{}{
		"policy": policy,
		"name":   "s3-access",
		"role":   "my-role",
	})
	findings := Detect(makePlan(rc))
	if len(findings) != 0 {
		t.Errorf("expected 0 findings for safe policy, got %d: %+v", len(findings), findings)
	}
}

func TestDetect_DenyStatementIgnored(t *testing.T) {
	policy := `{"Version":"2012-10-17","Statement":[{"Effect":"Deny","Action":"*","Resource":"*"}]}`
	rc := makeChange("aws_iam_role_policy", "aws_iam_role_policy.deny", map[string]interface{}{
		"policy": policy,
		"name":   "deny-all",
		"role":   "my-role",
	})
	findings := Detect(makePlan(rc))
	if len(findings) != 0 {
		t.Errorf("expected 0 findings for Deny statement, got %d", len(findings))
	}
}

func TestDetect_DeleteActionIgnored(t *testing.T) {
	rc := &tfjson.ResourceChange{
		Type:    "aws_iam_role_policy_attachment",
		Address: "aws_iam_role_policy_attachment.admin",
		Change: &tfjson.Change{
			Actions: tfjson.Actions{tfjson.ActionDelete},
			After:   map[string]interface{}{"policy_arn": "arn:aws:iam::aws:policy/AdministratorAccess"},
		},
	}
	findings := Detect(makePlan(rc))
	if len(findings) != 0 {
		t.Errorf("expected 0 findings for delete action, got %d", len(findings))
	}
}
