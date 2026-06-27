# Mapping Coverage

This file records the source and version for each AWS resource type mapping in `internal/plan/mappings/aws.yaml`.

## Generation Policy

Mappings are sourced from `iann0036/iam-dataset` and AWS provider documentation — **not hand-written from scratch**.
Hand-authored entries are kept in an `overrides/` layer (future) to minimize drift risk.

## Coverage Table

| Resource Type | Source | Version / Date | create | update | delete | Notes |
|---------------|--------|---------------|--------|--------|--------|-------|
| `aws_s3_bucket` | AWS docs / iam-dataset | 2026-06-27 | ✅ | ✅ | ✅ | |
| `aws_iam_role` | AWS docs / iam-dataset | 2026-06-27 | ✅ | ✅ | ✅ | |
| `aws_iam_policy` | AWS docs / iam-dataset | 2026-06-27 | ✅ | ✅ | ✅ | |
| `aws_iam_role_policy_attachment` | AWS docs | 2026-06-27 | ✅ | — | ✅ | No update; detach+attach instead |
| `aws_lambda_function` | AWS docs / iam-dataset | 2026-06-27 | ✅ | ✅ | ✅ | PassRole included |
| `aws_dynamodb_table` | AWS docs / iam-dataset | 2026-06-27 | ✅ | ✅ | ✅ | |
| `aws_instance` | AWS docs / iam-dataset | 2026-06-27 | ✅ | ✅ | ✅ | PassRole (instance profile) included |
| `aws_security_group` | AWS docs / iam-dataset | 2026-06-27 | ✅ | ✅ | ✅ | |
| `aws_vpc` | AWS docs / iam-dataset | 2026-06-27 | ✅ | ✅ | ✅ | |
| `aws_subnet` | AWS docs / iam-dataset | 2026-06-27 | ✅ | ✅ | ✅ | |
| `aws_ecr_repository` | AWS docs / iam-dataset | 2026-06-27 | ✅ | ✅ | ✅ | |
| `aws_sqs_queue` | AWS docs / iam-dataset | 2026-06-27 | ✅ | ✅ | ✅ | |
| `aws_sns_topic` | AWS docs / iam-dataset | 2026-06-27 | ✅ | ✅ | ✅ | |
| `aws_cloudwatch_log_group` | AWS docs / iam-dataset | 2026-06-27 | ✅ | ✅ | ✅ | |
| `aws_kms_key` | AWS docs / iam-dataset | 2026-06-27 | ✅ | ✅ | ✅ | |

## Sources

- **iann0036/iam-dataset**: https://github.com/iann0036/iam-dataset — community-maintained dataset of IAM actions per API call
- **AWS Provider docs**: https://registry.terraform.io/providers/hashicorp/aws/latest/docs

## Adding New Entries

1. Check `iann0036/iam-dataset` for the resource type's required IAM actions
2. Cross-reference with the AWS provider resource documentation
3. Add to `internal/plan/mappings/aws.yaml`
4. Add a row here with source and date
5. Write a test fixture in `testdata/plans/`
