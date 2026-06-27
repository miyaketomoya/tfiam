package cli

import (
	"github.com/spf13/cobra"
)

// GlobalFlags holds flags shared across subcommands.
type GlobalFlags struct {
	Principal        string
	PermissionSource string
	PermissionsFile  string
	DeepCheck        bool
	Format           string
	NoCloud          bool
}

func NewRootCmd() *cobra.Command {
	gf := &GlobalFlags{}

	root := &cobra.Command{
		Use:   "tfiam",
		Short: "Terraform-aware IAM Policy Simulator",
		Long: `tfiam checks whether the executing principal has the IAM permissions
required by a Terraform plan before running terraform apply.

Workflow:
  terraform plan -out=plan.tfplan
  tfiam check plan.tfplan      # exit 0 = OK, 1 = missing permissions
  terraform apply plan.tfplan`,
		SilenceUsage: true,
	}

	root.PersistentFlags().StringVar(&gf.Principal, "principal", "", "IAM principal ARN to check (default: auto-detect via STS)")
	root.PersistentFlags().StringVar(&gf.PermissionSource, "permission-source", "api", "Permission source: api or file")
	root.PersistentFlags().StringVar(&gf.PermissionsFile, "permissions-file", "", "Path to perms.yaml (required when --permission-source=file)")
	root.PersistentFlags().BoolVar(&gf.DeepCheck, "deep-check", false, "Enable cloud-based naming conflict checks (requires AWS access)")
	root.PersistentFlags().StringVar(&gf.Format, "format", "text", "Output format: text or json")
	root.PersistentFlags().BoolVar(&gf.NoCloud, "no-cloud", false, "Disable all cloud access; only static checks run")

	root.AddCommand(newCheckCmd(gf))

	return root
}
