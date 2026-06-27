package iam

import (
	"context"
	"testing"

	awsiam "github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/aws-sdk-go-v2/service/iam/types"
)

// stubIAMClient is a test double for IAMSimulatorClient.
type stubIAMClient struct {
	decision types.PolicyEvaluationDecisionType
	err      error
}

func (s *stubIAMClient) SimulatePrincipalPolicy(_ context.Context, _ *awsiam.SimulatePrincipalPolicyInput, _ ...func(*awsiam.Options)) (*awsiam.SimulatePrincipalPolicyOutput, error) {
	if s.err != nil {
		return nil, s.err
	}
	return &awsiam.SimulatePrincipalPolicyOutput{
		EvaluationResults: []types.EvaluationResult{
			{EvalDecision: s.decision},
		},
	}, nil
}

func TestAPISource_Allowed(t *testing.T) {
	src := NewAPISourceWithClient(&stubIAMClient{decision: types.PolicyEvaluationDecisionTypeAllowed})
	d, err := src.Evaluate(context.Background(), Principal{ARN: "arn:aws:iam::123456789012:role/x"}, "s3:CreateBucket", "*")
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if d != DecisionAllowed {
		t.Errorf("expected Allowed, got %s", d)
	}
}

func TestAPISource_ExplicitDeny(t *testing.T) {
	src := NewAPISourceWithClient(&stubIAMClient{decision: types.PolicyEvaluationDecisionTypeExplicitDeny})
	d, err := src.Evaluate(context.Background(), Principal{ARN: "arn:aws:iam::123456789012:role/x"}, "s3:CreateBucket", "*")
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if d != DecisionDenied {
		t.Errorf("expected Denied for explicitDeny, got %s", d)
	}
}

func TestAPISource_ImplicitDeny(t *testing.T) {
	src := NewAPISourceWithClient(&stubIAMClient{decision: types.PolicyEvaluationDecisionTypeImplicitDeny})
	d, err := src.Evaluate(context.Background(), Principal{ARN: "arn:aws:iam::123456789012:role/x"}, "s3:CreateBucket", "*")
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if d != DecisionDenied {
		t.Errorf("expected Denied for implicitDeny, got %s", d)
	}
}

func TestAPISource_ResourceArnsFixed(t *testing.T) {
	// MVP: ResourceArns must be passed as-is (caller passes "*" for unknown ARNs)
	var captured *awsiam.SimulatePrincipalPolicyInput
	stub := &capturingIAMClient{decision: types.PolicyEvaluationDecisionTypeAllowed, captured: &captured}

	src := NewAPISourceWithClient(stub)
	_, _ = src.Evaluate(context.Background(), Principal{ARN: "arn:aws:iam::123456789012:role/x"}, "s3:CreateBucket", "*")

	if len(captured.ResourceArns) != 1 || captured.ResourceArns[0] != "*" {
		t.Errorf("expected ResourceArns=[*], got %v", captured.ResourceArns)
	}
}

type capturingIAMClient struct {
	decision types.PolicyEvaluationDecisionType
	captured **awsiam.SimulatePrincipalPolicyInput
}

func (c *capturingIAMClient) SimulatePrincipalPolicy(_ context.Context, in *awsiam.SimulatePrincipalPolicyInput, _ ...func(*awsiam.Options)) (*awsiam.SimulatePrincipalPolicyOutput, error) {
	*c.captured = in
	return &awsiam.SimulatePrincipalPolicyOutput{
		EvaluationResults: []types.EvaluationResult{
			{EvalDecision: c.decision},
		},
	}, nil
}
