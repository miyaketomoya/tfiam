# tfiam

> **Pre-flight IAM check between `terraform plan` and `terraform apply`.**

`tfiam` checks whether the executing principal has the IAM permissions required by a Terraform plan *before* you run `apply`, catching permission errors and naming violations without touching your infrastructure.

!!! warning
    This project is not affiliated with HashiCorp or Terraform. It is an independent OSS tool.

---

## The Problem

`terraform plan` succeeds even when the executing role lacks required IAM permissions — because plan is a dry-run that doesn't call the real AWS/GCP APIs. The mismatch only surfaces at `apply` time, often mid-deployment.

```
terraform plan   ✅  # dry-run, no permission checks
terraform apply  ❌  # AccessDenied: iam:CreateRole
```

## The Solution

```
terraform plan -out=plan.tfplan
         ↓
tfiam check plan.tfplan   # exit 0 = OK, 1 = issues found
         ↓
terraform apply plan.tfplan
```

`tfiam` reads the plan, looks up the IAM actions each resource requires, and checks them against your current permissions — all before any infrastructure changes.

## Quick Install

```bash
go install github.com/tfiam-dev/tfiam/cmd/tfiam@latest
```

## What It Catches

- **Missing IAM permissions** — actions your role doesn't have (e.g. `iam:CreateRole`, `s3:CreateBucket`)
- **Naming violations** — resource names that exceed cloud provider limits or violate naming rules
- **PassRole issues** — `iam:PassRole` requirements that are easy to miss

## What It Doesn't Catch

- Resource-scoped conditions (e.g. bucket-specific policies) — evaluated as `best-effort`
- SCPs and permission boundaries in all configurations
- Apply-time ARNs for newly created resources
