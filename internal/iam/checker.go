package iam

import (
	"context"
	"fmt"

	"github.com/tfiam-dev/tfiam/internal/plan"
	"github.com/tfiam-dev/tfiam/internal/report"
)

// Checker evaluates required actions against a PermissionSource and returns findings.
type Checker struct {
	source PermissionSource
}

func NewChecker(src PermissionSource) *Checker {
	return &Checker{source: src}
}

// Check evaluates each required action and produces permission findings.
func (c *Checker) Check(ctx context.Context, principal Principal, required []plan.RequiredAction) ([]report.PermFinding, error) {
	var findings []report.PermFinding
	var principalMismatchErr error

	for _, ra := range required {
		decision, err := c.source.Evaluate(ctx, principal, ra.Action, ra.ResourceArn)
		if err != nil {
			// Principal mismatch from file source → propagate as fatal
			if principalMismatchErr == nil {
				principalMismatchErr = err
			}
			findings = append(findings, report.PermFinding{
				Action:          ra.Action,
				ResourceAddress: ra.ResourceAddress,
				ResourceArn:     ra.ResourceArn,
				Decision:        report.FindingUnknown,
				Confidence:      ra.Confidence,
				Source:          c.source.Name(),
				Detail:          fmt.Sprintf("evaluation error: %v", err),
			})
			continue
		}

		switch decision {
		case DecisionDenied:
			findings = append(findings, report.PermFinding{
				Action:          ra.Action,
				ResourceAddress: ra.ResourceAddress,
				ResourceArn:     ra.ResourceArn,
				Decision:        report.FindingMissing,
				Confidence:      ra.Confidence,
				Source:          c.source.Name(),
			})
		case DecisionUnknown:
			findings = append(findings, report.PermFinding{
				Action:          ra.Action,
				ResourceAddress: ra.ResourceAddress,
				ResourceArn:     ra.ResourceArn,
				Decision:        report.FindingUnknown,
				Confidence:      "best-effort",
				Source:          c.source.Name(),
			})
		}
		// DecisionAllowed → no finding
	}

	if principalMismatchErr != nil {
		return findings, principalMismatchErr
	}
	return findings, nil
}
