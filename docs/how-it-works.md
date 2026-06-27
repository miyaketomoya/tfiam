# How It Works

## Overview

```
plan.json
    │
    ▼
[Loader]          Parse JSON or run `terraform show -json`
    │
    ▼
[Extractor]       For each resource change → look up required IAM actions
    │               (reads internal/plan/mappings/aws.yaml)
    ▼
[IAM Checker]     Ask AWS: can this principal perform these actions?
    │               (SimulatePrincipalPolicy or perms.yaml)
    ▼
[Naming Validator] Check resource names against cloud provider rules
    │
    ▼
[Reporter]        Print findings, return exit code
```

---

## IAM Action Mapping

The core of tfiam is `internal/plan/mappings/aws.yaml` — a mapping from Terraform resource types to the IAM actions they require.

```yaml
resources:
  aws_iam_role:
    create:
      - action: iam:CreateRole
        confidence: high
      - action: iam:PassRole
        confidence: high
        passrole: true
    delete:
      - action: iam:DeleteRole
        confidence: high
```

### Confidence levels

| Level | Meaning |
|-------|---------|
| `high` | Core operation — nearly certain to be required |
| `best-effort` | Optional or conditional (tagging, logging, etc.) |

Only `high`-confidence missing actions trigger exit code `1`. `best-effort` gaps are reported as warnings.

### Where the mapping comes from

The mapping is generated monthly from [AWS CloudFormation Resource Provider Schemas](https://github.com/aws-cloudformation/cloudformation-resource-schema), which list the exact IAM permissions each resource handler requires.

The generator lives in `cmd/gen-mappings` and runs automatically via GitHub Actions.

---

## Permission Check

tfiam calls [AWS IAM Policy Simulator](https://docs.aws.amazon.com/IAM/latest/UserGuide/access_policies_testing-policies.html) (`iam:SimulatePrincipalPolicy`) with:

- The principal ARN (from `--principal` or auto-detected via `sts:GetCallerIdentity`)
- All required actions extracted from the plan
- `ResourceArns: ["*"]` (resource-level scoping is not yet supported)

The simulator evaluates the principal's attached policies, inline policies, and permission boundaries.

!!! note "SCPs are not evaluated"
    AWS Service Control Policies (SCPs) are not reflected in `SimulatePrincipalPolicy` when called from within the account. If your organization uses SCPs, add them to your review checklist manually.

---

## Naming Validation

tfiam statically checks resource name fields against provider-defined rules, for example:

| Resource | Field | Rule |
|----------|-------|------|
| `aws_s3_bucket` | `bucket` | 3–63 chars, lowercase, no underscores |
| `aws_iam_role` | `name` | 1–64 chars |
| `aws_lambda_function` | `function_name` | 1–64 chars |

Names that are computed at apply time (`(known after apply)`) are skipped automatically.

---

## Extending the Mapping

To add a resource type not yet in the mapping, edit `internal/plan/mappings/aws.yaml`:

```yaml
resources:
  aws_my_custom_resource:
    create:
      - action: myservice:CreateResource
        confidence: high
    delete:
      - action: myservice:DeleteResource
        confidence: high
```

Or run the generator to pull fresh data from CloudFormation schemas:

```bash
go run ./cmd/gen-mappings --out internal/plan/mappings/aws.yaml
```
