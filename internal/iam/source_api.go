package iam

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	awsiam "github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/aws-sdk-go-v2/service/iam/types"
)

// IAMSimulatorClient is the subset of the AWS IAM client used by APISource.
// Defined as an interface to allow stub injection in tests.
type IAMSimulatorClient interface {
	SimulatePrincipalPolicy(ctx context.Context, in *awsiam.SimulatePrincipalPolicyInput, opts ...func(*awsiam.Options)) (*awsiam.SimulatePrincipalPolicyOutput, error)
}

// APISource implements PermissionSource using AWS IAM SimulatePrincipalPolicy.
type APISource struct {
	client IAMSimulatorClient
}

func NewAPISource(ctx context.Context) (*APISource, error) {
	cfg, err := awsconfig.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("loading AWS config: %w", err)
	}
	return &APISource{client: awsiam.NewFromConfig(cfg)}, nil
}

// NewAPISourceWithClient creates an APISource with a custom client (for tests).
func NewAPISourceWithClient(client IAMSimulatorClient) *APISource {
	return &APISource{client: client}
}

func (s *APISource) Name() string { return "aws-iam-api" }

// Evaluate calls SimulatePrincipalPolicy for the given (action, resourceArn) pair.
// ResourceArns is set to [resourceArn]; typically "*" in MVP (R7 in the plan).
func (s *APISource) Evaluate(ctx context.Context, principal Principal, action, resourceArn string) (Decision, error) {
	input := &awsiam.SimulatePrincipalPolicyInput{
		PolicySourceArn: aws.String(principal.ARN),
		ActionNames:     []string{action},
		ResourceArns:    []string{resourceArn},
	}

	out, err := s.client.SimulatePrincipalPolicy(ctx, input)
	if err != nil {
		return DecisionUnknown, fmt.Errorf("SimulatePrincipalPolicy(%s, %s): %w", action, resourceArn, err)
	}

	if len(out.EvaluationResults) == 0 {
		return DecisionUnknown, nil
	}

	switch out.EvaluationResults[0].EvalDecision {
	case types.PolicyEvaluationDecisionTypeAllowed:
		return DecisionAllowed, nil
	default:
		// explicitDeny or implicitDeny
		return DecisionDenied, nil
	}
}
