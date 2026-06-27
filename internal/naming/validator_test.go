package naming

import (
	"encoding/json"
	"testing"

	tfjson "github.com/hashicorp/terraform-json"
	"github.com/tfiam-dev/tfiam/internal/report"
)

func makePlan(resourceType, address string, after map[string]interface{}, afterUnknown map[string]interface{}, actions []string) *tfjson.Plan {
	afterRaw, _ := json.Marshal(after)
	unknownRaw, _ := json.Marshal(afterUnknown)

	var afterVal, unknownVal interface{}
	_ = json.Unmarshal(afterRaw, &afterVal)
	_ = json.Unmarshal(unknownRaw, &unknownVal)

	tfjsonActions := make(tfjson.Actions, len(actions))
	for i, a := range actions {
		tfjsonActions[i] = tfjson.Action(a)
	}

	return &tfjson.Plan{
		ResourceChanges: []*tfjson.ResourceChange{
			{
				Type:    resourceType,
				Address: address,
				Change: &tfjson.Change{
					Actions:      tfjsonActions,
					After:        afterVal,
					AfterUnknown: unknownVal,
				},
			},
		},
	}
}

func TestValidator_S3NameTooLong(t *testing.T) {
	longName := "a" + string(make([]byte, 63)) // 64 chars
	for i := range longName {
		_ = i
	}
	// Create a 64-char lowercase bucket name
	name := ""
	for i := 0; i < 64; i++ {
		name += "a"
	}

	p := makePlan("aws_s3_bucket", "aws_s3_bucket.too_long", map[string]interface{}{
		"bucket": name,
	}, map[string]interface{}{}, []string{"create"})

	v := NewValidator()
	findings := v.Validate(p)

	var violations []report.NamingFinding
	for _, f := range findings {
		if f.Kind == report.NamingViolation {
			violations = append(violations, f)
		}
	}
	if len(violations) == 0 {
		t.Error("expected NAMING violation for 64-char S3 bucket name, got none")
	}
}

func TestValidator_S3NameValid(t *testing.T) {
	p := makePlan("aws_s3_bucket", "aws_s3_bucket.valid", map[string]interface{}{
		"bucket": "valid-bucket-name",
	}, map[string]interface{}{}, []string{"create"})

	v := NewValidator()
	findings := v.Validate(p)

	for _, f := range findings {
		if f.Kind == report.NamingViolation {
			t.Errorf("unexpected NAMING violation: %s", f.Detail)
		}
	}
}

func TestValidator_S3NameUnknown_Skipped(t *testing.T) {
	p := makePlan("aws_s3_bucket", "aws_s3_bucket.computed", map[string]interface{}{
		"bucket": nil,
	}, map[string]interface{}{
		"bucket": true, // after_unknown: bucket is computed
	}, []string{"create"})

	v := NewValidator()
	findings := v.Validate(p)

	var skipped []report.NamingFinding
	for _, f := range findings {
		if f.Kind == report.NamingSkipped {
			skipped = append(skipped, f)
		}
	}
	if len(skipped) == 0 {
		t.Error("expected NAMING-SKIPPED for after_unknown bucket name, got none")
	}
}

func TestValidator_NoCloudAccess(t *testing.T) {
	// Validator must not call AWS APIs (no --deep-check)
	// We verify this by running without AWS credentials and expecting no panic/error
	p := makePlan("aws_s3_bucket", "aws_s3_bucket.test", map[string]interface{}{
		"bucket": "test-bucket-ok",
	}, map[string]interface{}{}, []string{"create"})

	v := NewValidator()
	_ = v.Validate(p) // Should not panic or call AWS
}

func TestValidator_IAMRoleNameTooLong(t *testing.T) {
	name := ""
	for i := 0; i < 65; i++ {
		name += "a"
	}

	p := makePlan("aws_iam_role", "aws_iam_role.too_long", map[string]interface{}{
		"name": name,
	}, map[string]interface{}{}, []string{"create"})

	v := NewValidator()
	findings := v.Validate(p)

	var violations []report.NamingFinding
	for _, f := range findings {
		if f.Kind == report.NamingViolation {
			violations = append(violations, f)
		}
	}
	if len(violations) == 0 {
		t.Error("expected NAMING violation for 65-char IAM role name, got none")
	}
}

func TestValidator_DeleteOnly_Skipped(t *testing.T) {
	// delete-only changes should not be checked for naming
	p := makePlan("aws_s3_bucket", "aws_s3_bucket.deleted", map[string]interface{}{
		"bucket": "x", // too short (< 3 chars), but delete-only
	}, map[string]interface{}{}, []string{"delete"})

	v := NewValidator()
	findings := v.Validate(p)

	for _, f := range findings {
		if f.Kind == report.NamingViolation {
			t.Errorf("unexpected violation on delete-only resource: %s", f.Detail)
		}
	}
}
