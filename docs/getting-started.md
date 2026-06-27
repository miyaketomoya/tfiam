# Getting Started

## Prerequisites

- Go 1.21+
- AWS credentials configured (`aws configure` or environment variables)
- Terraform installed (only needed for `.tfplan` binary files)

## Installation

```bash
go install github.com/tfiam-dev/tfiam/cmd/tfiam@latest
```

## Basic Usage

### Step 1: Generate a Terraform plan

```bash
terraform plan -out=plan.tfplan
```

### Step 2: Convert to JSON (recommended)

```bash
terraform show -json plan.tfplan > plan.json
```

Using the `.json` format avoids shelling out to `terraform` at check time and is the fastest path.

### Step 3: Run tfiam

```bash
tfiam check plan.json
```

tfiam will auto-detect your AWS identity via `sts:GetCallerIdentity` and check all required IAM actions.

---

## Modes

### API mode (default)

Calls AWS IAM `SimulatePrincipalPolicy` in real time.

```bash
tfiam check plan.json
tfiam check --principal arn:aws:iam::123456789012:role/deployer plan.json
```

### File mode (no live AWS calls at check time)

Export your permissions once, then check offline. Useful in air-gapped environments or CI pipelines where you don't want extra AWS calls.

```bash
# Export permissions to a file
tfiam check --permission-source file --permissions-file perms.yaml plan.json
```

See [CLI Reference](cli-reference.md) for the `perms.yaml` format.

### No-cloud mode

Static checks only — zero AWS API calls.

```bash
tfiam check --no-cloud plan.json
```

Only naming violations and statically-known permission gaps are reported.

---

## Reading the Output

```
MISSING: iam:CreateRole on aws_iam_role.deployer (confidence: high, source: aws-iam-api)
NAMING:  aws_s3_bucket.logs — name "my_logs_bucket" exceeds 63 chars (got 67)
WARNING: 3/10 actions were not evaluated (unknown resource type)
```

| Prefix | Meaning |
|--------|---------|
| `MISSING` | IAM action your principal cannot perform |
| `NAMING` | Naming rule violation |
| `NAMING-SKIPPED` | Name is computed at apply time — cannot check statically |
| `WARNING` | Non-fatal issue (e.g. unknown resource type) |

## Exit Codes

| Code | Meaning |
|------|---------|
| `0` | No issues found |
| `1` | Missing permission or naming violation detected |
| `2` | Tool error (file not found, AWS API failure, etc.) |

Exit code `1` is designed for CI gates:

```yaml
# GitHub Actions example
- run: tfiam check plan.json
  # Fails the step if permissions are missing
```
