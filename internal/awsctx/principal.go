package awsctx

import (
	"context"
	"fmt"
	"regexp"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/tfiam-dev/tfiam/internal/iam"
)

// assumedRoleRE matches arn:aws:sts::<account>:assumed-role/<RoleName>/<session>
var assumedRoleRE = regexp.MustCompile(`^arn:aws:sts::(\d+):assumed-role/([^/]+)/`)

// ResolvePrincipal returns the IAM principal to check.
// If explicit is non-empty, it is returned as-is.
// Otherwise, STS GetCallerIdentity is called and the ARN is normalized.
func ResolvePrincipal(ctx context.Context, explicit string) (iam.Principal, error) {
	if explicit != "" {
		return iam.Principal{ARN: explicit}, nil
	}

	cfg, err := awsconfig.LoadDefaultConfig(ctx)
	if err != nil {
		return iam.Principal{}, fmt.Errorf("loading AWS config: %w", err)
	}

	out, err := sts.NewFromConfig(cfg).GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	if err != nil {
		return iam.Principal{}, fmt.Errorf("STS GetCallerIdentity: %w", err)
	}

	arn := aws.ToString(out.Arn)
	return iam.Principal{ARN: normalizeARN(arn)}, nil
}

// normalizeARN converts assumed-role ARNs to role ARNs.
// arn:aws:sts::<account>:assumed-role/<Role>/<session>
//
//	→ arn:aws:iam::<account>:role/<Role>
func normalizeARN(arn string) string {
	m := assumedRoleRE.FindStringSubmatch(arn)
	if m == nil {
		return arn
	}
	account := m[1]
	roleName := m[2]
	return fmt.Sprintf("arn:aws:iam::%s:role/%s", account, roleName)
}
