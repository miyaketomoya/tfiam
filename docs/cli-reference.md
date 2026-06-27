# CLI Reference

## `tfiam check`

Check IAM permissions required by a Terraform plan.

```
tfiam check [flags] <plan-file>
```

`<plan-file>` can be:

- `.json` — output of `terraform show -json plan.tfplan` (no Terraform dependency)
- `.tfplan` — binary plan file (requires `terraform` in `$PATH`)

### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--principal` | auto | IAM principal ARN to check. Defaults to `sts:GetCallerIdentity`. |
| `--permission-source` | `api` | `api` (live AWS IAM Simulate) or `file` (cached perms.yaml). |
| `--permissions-file` | — | Path to `perms.yaml`. Required when `--permission-source=file`. |
| `--no-cloud` | `false` | Disable all AWS API calls. Runs static checks only. |
| `--deep-check` | `false` | Enable additional AWS calls (e.g. check if S3 bucket name is already taken). |
| `--format` | `text` | Output format: `text` or `json`. |

### Exit Codes

| Code | Meaning |
|------|---------|
| `0` | No statically-detectable issues found |
| `1` | High-confidence missing IAM action or naming violation |
| `2` | Tool error |

---

## `perms.yaml` format

Used with `--permission-source file`.

```yaml
principal: "arn:aws:iam::123456789012:role/deployer"
generated_at: "2026-06-27T00:00:00Z"
decisions:
  - action: "s3:CreateBucket"
    resource: "*"
    decision: "allowed"
  - action: "iam:CreateRole"
    resource: "*"
    decision: "denied"
  - action: "lambda:CreateFunction"
    resource: "*"
    decision: "allowed"
```

### decision values

| Value | Meaning |
|-------|---------|
| `allowed` | Principal has permission |
| `denied` | Principal lacks permission — tfiam will flag this |
| `unknown` | Could not be evaluated |

---

## JSON output format

```bash
tfiam check --format json plan.json
```

```json
{
  "summary": {
    "missing": 1,
    "naming_violations": 0,
    "warnings": 2,
    "exit_code": 1
  },
  "findings": [
    {
      "type": "missing",
      "action": "iam:CreateRole",
      "resource_type": "aws_iam_role",
      "resource_address": "aws_iam_role.deployer",
      "confidence": "high",
      "source": "aws-iam-api"
    }
  ]
}
```
