package deepcheck

import (
	"context"
	"errors"
	"testing"

	awsiam "github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	tfjson "github.com/hashicorp/terraform-json"
)

// stubS3 implements S3Client for tests.
type stubS3 struct {
	err error // nil = bucket exists (200), non-nil = bucket not found or error
}

func (s *stubS3) HeadBucket(_ context.Context, in *s3.HeadBucketInput, _ ...func(*s3.Options)) (*s3.HeadBucketOutput, error) {
	return nil, s.err
}

// stubIAM implements IAMClient for tests.
type stubIAM struct {
	err error // nil = role exists, non-nil = not found
}

func (s *stubIAM) GetRole(_ context.Context, in *awsiam.GetRoleInput, _ ...func(*awsiam.Options)) (*awsiam.GetRoleOutput, error) {
	return nil, s.err
}

func makePlanWithCreate(resourceType, address string, after map[string]interface{}) *tfjson.Plan {
	return &tfjson.Plan{
		ResourceChanges: []*tfjson.ResourceChange{
			{
				Type:    resourceType,
				Address: address,
				Change: &tfjson.Change{
					Actions:      tfjson.Actions{tfjson.ActionCreate},
					After:        after,
					AfterUnknown: map[string]interface{}{},
				},
			},
		},
	}
}

func TestCheck_S3BucketConflict(t *testing.T) {
	// stub returns nil error = HeadBucket succeeded = bucket exists
	c := NewCheckerWithClients(&stubS3{err: nil}, &stubIAM{err: errors.New("not found")})
	p := makePlanWithCreate("aws_s3_bucket", "aws_s3_bucket.logs", map[string]interface{}{
		"bucket": "my-company-logs",
	})
	findings := c.Check(context.Background(), p)
	if len(findings) != 1 {
		t.Fatalf("expected 1 conflict finding, got %d", len(findings))
	}
	if findings[0].Name != "my-company-logs" {
		t.Errorf("unexpected name: %s", findings[0].Name)
	}
}

func TestCheck_S3BucketAvailable(t *testing.T) {
	// simulate 404 Not Found
	c := NewCheckerWithClients(&stubS3{err: errors.New("404")}, &stubIAM{err: errors.New("not found")})
	p := makePlanWithCreate("aws_s3_bucket", "aws_s3_bucket.logs", map[string]interface{}{
		"bucket": "my-unique-bucket-xyz",
	})
	findings := c.Check(context.Background(), p)
	if len(findings) != 0 {
		t.Errorf("expected 0 findings for available bucket, got %d", len(findings))
	}
}

func TestCheck_IAMRoleConflict(t *testing.T) {
	// nil error from GetRole = role exists
	c := NewCheckerWithClients(&stubS3{err: errors.New("not found")}, &stubIAM{err: nil})
	p := makePlanWithCreate("aws_iam_role", "aws_iam_role.deployer", map[string]interface{}{
		"name": "my-deployer-role",
	})
	findings := c.Check(context.Background(), p)
	if len(findings) != 1 {
		t.Fatalf("expected 1 conflict finding, got %d", len(findings))
	}
	if findings[0].ResourceType != "aws_iam_role" {
		t.Errorf("unexpected resource type: %s", findings[0].ResourceType)
	}
}

func TestCheck_UnknownNameSkipped(t *testing.T) {
	// bucket name is computed at apply time
	c := NewCheckerWithClients(&stubS3{err: nil}, &stubIAM{err: nil})
	p := &tfjson.Plan{
		ResourceChanges: []*tfjson.ResourceChange{
			{
				Type:    "aws_s3_bucket",
				Address: "aws_s3_bucket.dynamic",
				Change: &tfjson.Change{
					Actions:      tfjson.Actions{tfjson.ActionCreate},
					After:        map[string]interface{}{"bucket": nil},
					AfterUnknown: map[string]interface{}{"bucket": true},
				},
			},
		},
	}
	findings := c.Check(context.Background(), p)
	if len(findings) != 0 {
		t.Errorf("expected 0 findings for unknown name, got %d", len(findings))
	}
}

func TestCheck_DeleteIgnored(t *testing.T) {
	c := NewCheckerWithClients(&stubS3{err: nil}, &stubIAM{err: nil})
	p := &tfjson.Plan{
		ResourceChanges: []*tfjson.ResourceChange{
			{
				Type:    "aws_s3_bucket",
				Address: "aws_s3_bucket.old",
				Change: &tfjson.Change{
					Actions: tfjson.Actions{tfjson.ActionDelete},
					After:   map[string]interface{}{"bucket": "old-bucket"},
				},
			},
		},
	}
	findings := c.Check(context.Background(), p)
	if len(findings) != 0 {
		t.Errorf("expected 0 findings for delete action, got %d", len(findings))
	}
}
