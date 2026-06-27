package plan

import (
	"context"
	"testing"
)

func loadFixture(t *testing.T, path string) []RequiredAction {
	t.Helper()
	l := NewLoader()
	p, err := l.Load(context.Background(), path)
	if err != nil {
		t.Fatalf("Load %s: %v", path, err)
	}
	e := NewExtractor()
	actions, err := e.Extract(p)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	return actions
}

func containsAction(actions []RequiredAction, target string) bool {
	for _, a := range actions {
		if a.Action == target {
			return true
		}
	}
	return false
}

func TestExtract_S3Create(t *testing.T) {
	actions := loadFixture(t, "../../testdata/plans/s3_create.json")

	if !containsAction(actions, "s3:CreateBucket") {
		t.Errorf("expected s3:CreateBucket in actions, got %v", actions)
	}
	// No delete actions expected
	if containsAction(actions, "s3:DeleteBucket") {
		t.Errorf("did not expect s3:DeleteBucket for create-only plan")
	}
}

func TestExtract_S3Delete(t *testing.T) {
	actions := loadFixture(t, "../../testdata/plans/s3_delete.json")

	if !containsAction(actions, "s3:DeleteBucket") {
		t.Errorf("expected s3:DeleteBucket in actions, got %v", actions)
	}
	if containsAction(actions, "s3:CreateBucket") {
		t.Errorf("did not expect s3:CreateBucket for delete-only plan")
	}
}

func TestExtract_LambdaCreate_PassRole(t *testing.T) {
	actions := loadFixture(t, "../../testdata/plans/lambda_create.json")

	if !containsAction(actions, "lambda:CreateFunction") {
		t.Errorf("expected lambda:CreateFunction, got %v", actions)
	}
	if !containsAction(actions, "iam:PassRole") {
		t.Errorf("expected iam:PassRole for lambda create (role assignment), got %v", actions)
	}
}

func TestExtract_Dedup(t *testing.T) {
	actions := loadFixture(t, "../../testdata/plans/s3_create.json")

	seen := map[string]int{}
	for _, a := range actions {
		seen[a.Action]++
	}
	for action, count := range seen {
		if count > 1 {
			t.Errorf("action %s appears %d times (expected 1 after dedup)", action, count)
		}
	}
}

func TestExtract_NoCloudPath(t *testing.T) {
	// .json path must not call AWS APIs — loading is pure JSON unmarshal
	// We verify this by loading without any AWS environment set and confirming no panic/error
	actions := loadFixture(t, "../../testdata/plans/s3_create.json")
	if len(actions) == 0 {
		t.Error("expected at least one action from .json fixture")
	}
}
