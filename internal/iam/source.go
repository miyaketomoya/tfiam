package iam

import "context"

// Decision represents the IAM evaluation result for a single (action, resource) pair.
type Decision int

const (
	DecisionAllowed Decision = iota
	DecisionDenied            // explicit or implicit deny
	DecisionUnknown           // cannot evaluate (data unavailable)
)

func (d Decision) String() string {
	switch d {
	case DecisionAllowed:
		return "allowed"
	case DecisionDenied:
		return "denied"
	default:
		return "unknown"
	}
}

// Principal identifies the IAM entity performing the apply.
type Principal struct {
	ARN string
}

// PermissionSource is the pluggable backend for permission evaluation.
// Implementations: APISource (SimulatePrincipalPolicy) and FileSource (cached results).
type PermissionSource interface {
	// Evaluate returns whether principal can perform action on resourceArn.
	Evaluate(ctx context.Context, principal Principal, action, resourceArn string) (Decision, error)
	// Name identifies the source for reporting.
	Name() string
}
