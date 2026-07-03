package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/tfiam-dev/tfiam/internal/awsctx"
	"github.com/tfiam-dev/tfiam/internal/deepcheck"
	"github.com/tfiam-dev/tfiam/internal/escalation"
	"github.com/tfiam-dev/tfiam/internal/iam"
	"github.com/tfiam-dev/tfiam/internal/naming"
	"github.com/tfiam-dev/tfiam/internal/plan"
	"github.com/tfiam-dev/tfiam/internal/report"
)

func newCheckCmd(gf *GlobalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "check <plan-file>",
		Short: "Check IAM permissions and naming rules against a Terraform plan",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCheck(cmd.Context(), gf, args[0])
		},
	}
}

func runCheck(ctx context.Context, gf *GlobalFlags, planPath string) error {
	// Validate --no-cloud / --permission-source conflict
	if gf.NoCloud && gf.PermissionSource == "api" {
		fmt.Fprintln(os.Stderr, "WARNING: --no-cloud overrides --permission-source api; falling back to file source")
		gf.PermissionSource = "file"
	}

	// Load plan
	loader := plan.NewLoader()
	tfPlan, err := loader.Load(ctx, planPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: failed to load plan: %v\n", err)
		return ExitError(2)
	}

	// Extract required actions from plan
	extractor := plan.NewExtractor()
	required, err := extractor.Extract(tfPlan)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: failed to extract required actions: %v\n", err)
		return ExitError(2)
	}

	rep := report.New(len(required))

	// Check naming violations
	validator := naming.NewValidator()
	namingFindings := validator.Validate(tfPlan)
	rep.AddNamingFindings(namingFindings)

	// Permission check (skip when --no-cloud and no permissions file)
	if !gf.NoCloud || gf.PermissionsFile != "" {
		principal, err := awsctx.ResolvePrincipal(ctx, gf.Principal)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: failed to resolve principal: %v\n", err)
			return ExitError(2)
		}

		src, err := buildPermissionSource(ctx, gf)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
			return ExitError(2)
		}

		checker := iam.NewChecker(src)
		permFindings, err := checker.Check(ctx, principal, required)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: permission check failed: %v\n", err)
			return ExitError(2)
		}
		rep.AddPermFindings(permFindings)
	}

	// Render report
	exitCode := rep.Render(os.Stdout, gf.Format)

	// B: suggest-policy — print minimal IAM policy covering all missing actions
	if gf.SuggestPolicy {
		rep.SuggestPolicy(os.Stdout)
	}

	// C: deep-check — detect naming conflicts via live AWS API calls
	if gf.DeepCheck {
		dc, err := deepcheck.NewChecker(ctx)
		if err != nil {
			fmt.Fprintf(os.Stderr, "WARNING: deep-check unavailable: %v\n", err)
		} else {
			for _, f := range dc.Check(ctx, tfPlan) {
				fmt.Fprintf(os.Stdout, "CONFLICT: %s — %s\n", f.ResourceAddress, f.Detail)
				exitCode = max(exitCode, 1)
			}
		}
	}

	// D: escalation detection — static analysis, always runs
	for _, f := range escalation.Detect(tfPlan) {
		fmt.Fprintf(os.Stdout, "ESCALATION [%s]: %s — %s\n", f.Risk, f.ResourceAddress, f.Detail)
		if f.Risk == escalation.RiskHigh && gf.EscalationCheck {
			exitCode = max(exitCode, 1)
		}
	}

	if exitCode != 0 {
		return ExitError(exitCode)
	}
	return nil
}

func buildPermissionSource(ctx context.Context, gf *GlobalFlags) (iam.PermissionSource, error) {
	switch gf.PermissionSource {
	case "file":
		if gf.PermissionsFile == "" {
			return nil, fmt.Errorf("--permissions-file is required when --permission-source=file")
		}
		return iam.NewFileSource(gf.PermissionsFile)
	case "api":
		return iam.NewAPISource(ctx)
	default:
		return nil, fmt.Errorf("unknown --permission-source: %q (must be api or file)", gf.PermissionSource)
	}
}

// ExitError is a sentinel error that carries an exit code.
type ExitError int

func (e ExitError) Error() string { return fmt.Sprintf("exit %d", int(e)) }
func (e ExitError) ExitCode() int { return int(e) }
