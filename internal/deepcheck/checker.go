// Package deepcheck performs cloud-based naming conflict checks against live AWS APIs.
// It supplements the static naming validator by detecting names already taken in AWS.
package deepcheck

import (
	"context"
	"fmt"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	awsiam "github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	tfjson "github.com/hashicorp/terraform-json"
)

// Finding represents a naming conflict detected via live AWS API calls.
type Finding struct {
	ResourceAddress string
	ResourceType    string
	Name            string
	Detail          string
}

// S3Client is the subset of the S3 API used by Checker.
type S3Client interface {
	HeadBucket(ctx context.Context, in *s3.HeadBucketInput, opts ...func(*s3.Options)) (*s3.HeadBucketOutput, error)
}

// IAMClient is the subset of the IAM API used by Checker.
type IAMClient interface {
	GetRole(ctx context.Context, in *awsiam.GetRoleInput, opts ...func(*awsiam.Options)) (*awsiam.GetRoleOutput, error)
}

// Checker runs naming conflict checks using live AWS APIs.
type Checker struct {
	s3  S3Client
	iam IAMClient
}

// NewChecker creates a Checker using the default AWS configuration.
func NewChecker(ctx context.Context) (*Checker, error) {
	cfg, err := awsconfig.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("loading AWS config: %w", err)
	}
	return &Checker{
		s3:  s3.NewFromConfig(cfg),
		iam: awsiam.NewFromConfig(cfg),
	}, nil
}

// NewCheckerWithClients creates a Checker with injected clients (for tests).
func NewCheckerWithClients(s3c S3Client, iamc IAMClient) *Checker {
	return &Checker{s3: s3c, iam: iamc}
}

// Check inspects all resource changes being created in the plan and returns
// findings for names that already exist in AWS.
func (c *Checker) Check(ctx context.Context, p *tfjson.Plan) []Finding {
	var findings []Finding
	for _, rc := range p.ResourceChanges {
		if rc.Change == nil || !isCreate(rc.Change.Actions) {
			continue
		}
		after, ok := rc.Change.After.(map[string]interface{})
		if !ok {
			continue
		}
		afterUnknown, _ := rc.Change.AfterUnknown.(map[string]interface{})

		switch rc.Type {
		case "aws_s3_bucket":
			if name, ok := staticName(after, afterUnknown, "bucket"); ok {
				if f := c.checkS3Bucket(ctx, rc.Address, name); f != nil {
					findings = append(findings, *f)
				}
			}
		case "aws_iam_role":
			if name, ok := staticName(after, afterUnknown, "name"); ok {
				if f := c.checkIAMRole(ctx, rc.Address, name); f != nil {
					findings = append(findings, *f)
				}
			}
		}
	}
	return findings
}

func (c *Checker) checkS3Bucket(ctx context.Context, address, name string) *Finding {
	_, err := c.s3.HeadBucket(ctx, &s3.HeadBucketInput{Bucket: &name})
	if err == nil {
		// 200 OK — bucket exists under our account but not in this plan's state
		return &Finding{
			ResourceAddress: address,
			ResourceType:    "aws_s3_bucket",
			Name:            name,
			Detail:          fmt.Sprintf("bucket %q already exists in your account (not in Terraform state)", name),
		}
	}
	// 403 → bucket exists but owned by another account
	if isHTTPError(err, 403) {
		return &Finding{
			ResourceAddress: address,
			ResourceType:    "aws_s3_bucket",
			Name:            name,
			Detail:          fmt.Sprintf("bucket %q is already taken by another AWS account", name),
		}
	}
	// 404 / NoSuchBucket → available; other errors → skip
	return nil
}

func (c *Checker) checkIAMRole(ctx context.Context, address, name string) *Finding {
	_, err := c.iam.GetRole(ctx, &awsiam.GetRoleInput{RoleName: &name})
	if err == nil {
		return &Finding{
			ResourceAddress: address,
			ResourceType:    "aws_iam_role",
			Name:            name,
			Detail:          fmt.Sprintf("IAM role %q already exists in this account (not in Terraform state)", name),
		}
	}
	// NoSuchEntityException → available; other errors → skip
	return nil
}

// staticName returns the string value of a field when it is statically known
// (i.e., not marked as unknown after apply).
func staticName(after, afterUnknown map[string]interface{}, field string) (string, bool) {
	if afterUnknown[field] == true {
		return "", false
	}
	v, _ := after[field].(string)
	if v == "" {
		return "", false
	}
	return v, true
}

func isCreate(actions tfjson.Actions) bool {
	for _, a := range actions {
		if a == tfjson.ActionCreate {
			return true
		}
	}
	return false
}

func isHTTPError(err error, code int) bool {
	type httpStatusError interface{ HTTPStatusCode() int }
	if e, ok := err.(httpStatusError); ok {
		return e.HTTPStatusCode() == code
	}
	return false
}
