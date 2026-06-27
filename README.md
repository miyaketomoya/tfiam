# tfiam — Terraform-aware IAM Policy Simulator

> **Pre-flight IAM check between `terraform plan` and `terraform apply`.**

`tfiam` checks whether the executing principal has the IAM permissions required by a Terraform plan *before* you run `apply`, catching permission errors, naming violations, and missing access without touching your infrastructure.

> ⚠️ This project is not affiliated with HashiCorp or Terraform. It is an independent OSS tool.

---

## Workflow

```
terraform plan -out=plan.tfplan
    ↓
tfiam check plan.tfplan      # exit 0 = OK, 1 = missing permissions/naming, 2 = tool error
    ↓
terraform apply plan.tfplan
```

## Quick Start

```bash
# Install
go install github.com/tfiam-dev/tfiam/cmd/tfiam@latest

# 1. Generate a Terraform plan
terraform plan -out=plan.tfplan

# 2. (Recommended) Convert to JSON first — guaranteed no-cloud path
terraform show -json plan.tfplan > plan.json

# 3. Check permissions (uses your default AWS profile)
tfiam check plan.json

# 4. Or pass the binary plan directly (requires terraform in PATH)
tfiam check plan.tfplan

# 5. Specify a principal explicitly
tfiam check --principal arn:aws:iam::123456789012:role/deployer plan.json

# 6. Use a pre-exported permissions file (no live AWS calls at check time)
tfiam check --permission-source file --permissions-file perms.yaml plan.json

# 7. Static checks only — no cloud access at all
tfiam check --no-cloud plan.json
```

## Exit Codes

| Code | Meaning |
|------|---------|
| `0` | No statically-detectable issues found *(does not guarantee apply will succeed)* |
| `1` | High-confidence missing IAM action or naming violation |
| `2` | Tool error (file not found, principal mismatch, all actions unevaluable, etc.) |

## Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--principal` | auto-detect | IAM principal ARN to check (STS `GetCallerIdentity` if omitted) |
| `--permission-source` | `api` | `api` — live AWS IAM Simulate; `file` — cached perms.yaml |
| `--permissions-file` | — | Path to `perms.yaml` (required when `--permission-source=file`) |
| `--no-cloud` | `false` | Disable all cloud access; run static checks only |
| `--deep-check` | `false` | Enable AWS calls to check naming conflicts (e.g. S3 bucket existence) |
| `--format` | `text` | Output format: `text` or `json` |

## Permission Source: file mode

Pre-export your permissions with the AWS CLI:

```bash
# Generate perms.yaml from SimulatePrincipalPolicy
aws iam simulate-principal-policy \
  --policy-source-arn arn:aws:iam::123456789012:role/deployer \
  --action-names s3:CreateBucket iam:CreateRole lambda:CreateFunction \
  > /tmp/sim-out.json

# Convert to perms.yaml format (example structure):
cat > perms.yaml <<'EOF'
principal: "arn:aws:iam::123456789012:role/deployer"
generated_at: "2026-06-27T00:00:00Z"
decisions:
  - action: "s3:CreateBucket"
    resource: "*"
    decision: "allowed"
  - action: "iam:CreateRole"
    resource: "*"
    decision: "denied"
EOF

tfiam check --permission-source file --permissions-file perms.yaml plan.json
```

## Output Examples

```
# Permission gap detected
MISSING: iam:CreateRole on aws_iam_role.deployer (confidence: high, source: aws-iam-api)

# Naming violation
NAMING: aws_s3_bucket.logs bucket — name "my_logs_bucket_very_long_name_2026" exceeds 63 chars (got 36)

# Naming skipped (computed at apply time)
NAMING-SKIPPED: aws_s3_bucket.dynamic (name known after apply)

# Unknown coverage warning
WARNING: 3/10 actions were not evaluated (unknown)

# All clear
No statically-detectable issues found
```

## Limitations

- **Static mapping only (MVP)**: requires actions to be listed in `internal/plan/mappings/aws.yaml`. Unknown resource types produce a warning, not a failure.
- **`ResourceArns=*` in MVP**: resource-scoped IAM conditions (e.g. `"Resource": "arn:aws:s3:::specific-bucket"`) are not evaluated. Findings are labeled `confidence: best-effort`.
- **SCP / Permission Boundary**: not evaluated by `SimulatePrincipalPolicy` in all configurations. See AWS docs.
- **apply-time ARNs**: new resources don't have ARNs until apply — resource-level checks are best-effort.

## Architecture

See [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) for the full design rationale, interface contracts, and extension guide.

## Extending tfiam

### Add a new AWS resource type

Edit `internal/plan/mappings/aws.yaml`:

```yaml
resources:
  aws_my_resource:
    create:
      - action: "myservice:CreateMyResource"
        confidence: high
    delete:
      - action: "myservice:DeleteMyResource"
        confidence: high
```

Record the source in `mappings/MAPPING_COVERAGE.md`.

### Add a new cloud provider

Implement `PermissionSource` from `internal/iam/source.go`:

```go
type PermissionSource interface {
    Evaluate(ctx context.Context, principal Principal, action, resourceArn string) (Decision, error)
    Name() string
}
```

## License

[MIT](LICENSE)
