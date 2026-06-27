# Contributing

## Development Setup

```bash
git clone https://github.com/miyaketomoya/tfiam.git
cd tfiam
go test ./...
```

No external services are required to run the test suite — all AWS calls are stubbed.

## Project Structure

```
cmd/
  tfiam/          # CLI entrypoint
  gen-mappings/   # IAM mapping generator (CloudFormation schemas)
internal/
  cli/            # Command definitions (cobra)
  plan/           # Plan loading, IAM action extraction, mapping
  iam/            # Permission sources (API / file) and checker
  naming/         # Resource naming validation rules
  report/         # Output formatting and exit codes
internal/plan/mappings/
  aws.yaml        # IAM action mapping (auto-generated monthly)
```

## Adding a New AWS Resource Type

Edit `internal/plan/mappings/aws.yaml`:

```yaml
resources:
  aws_example_resource:
    create:
      - action: example:CreateResource
        confidence: high
    delete:
      - action: example:DeleteResource
        confidence: high
```

Alternatively, run the mapping generator with valid AWS credentials:

```bash
go run ./cmd/gen-mappings --out internal/plan/mappings/aws.yaml
```

## Adding a Naming Rule

Edit `internal/naming/rules.go`:

```go
{
    ResourceType: "aws_example_resource",
    NameField:    "name",
    MinLength:    1,
    MaxLength:    64,
    Pattern:      regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_-]*$`),
},
```

## Adding a New Permission Source

Implement the `PermissionSource` interface from `internal/iam/source.go`:

```go
type PermissionSource interface {
    Evaluate(ctx context.Context, principal Principal, action, resourceArn string) (Decision, error)
    Name() string
}
```

Return one of:

- `DecisionAllowed` — principal has this permission
- `DecisionDenied` — principal lacks this permission
- `DecisionUnknown` — could not be determined

## Running Tests

```bash
# All tests
go test ./...

# Specific package with verbose output
go test ./internal/plan/... -v

# With race detector
go test -race ./...
```

## Submitting a Pull Request

1. Fork the repository
2. Create a branch: `git checkout -b feat/my-feature`
3. Make your changes and add tests
4. Verify: `go test ./... && go vet ./...`
5. Open a PR against `main`

## Reporting Issues

Please open an issue on [GitHub](https://github.com/miyaketomoya/tfiam/issues) with:

- The Terraform resource type that's missing or incorrect
- The IAM action that should be required
- A link to AWS documentation if available
