package naming

import "regexp"

// Rule defines a naming constraint for a specific resource type.
type Rule struct {
	ResourceType string
	// NameField is the JSON key in change.After that holds the resource name.
	// Empty string means the rule applies to the resource address itself.
	NameField  string
	MinLength  int
	MaxLength  int
	Pattern    *regexp.Regexp
	PatternDoc string
}

// awsRules is the static table of naming rules for supported AWS resources.
var awsRules = []Rule{
	{
		ResourceType: "aws_s3_bucket",
		NameField:    "bucket",
		MinLength:    3,
		MaxLength:    63,
		Pattern:      regexp.MustCompile(`^[a-z0-9][a-z0-9.\-]*[a-z0-9]$`),
		PatternDoc:   "lowercase letters, numbers, hyphens and dots; must start and end with alphanumeric",
	},
	{
		ResourceType: "aws_iam_role",
		NameField:    "name",
		MinLength:    1,
		MaxLength:    64,
		Pattern:      regexp.MustCompile(`^[\w+=,.@-]+$`),
		PatternDoc:   "alphanumeric and +=,.@-",
	},
	{
		ResourceType: "aws_iam_policy",
		NameField:    "name",
		MinLength:    1,
		MaxLength:    128,
		Pattern:      regexp.MustCompile(`^[\w+=,.@-]+$`),
		PatternDoc:   "alphanumeric and +=,.@-",
	},
	{
		ResourceType: "aws_lambda_function",
		NameField:    "function_name",
		MinLength:    1,
		MaxLength:    64,
		Pattern:      regexp.MustCompile(`^[a-zA-Z0-9_-]+$`),
		PatternDoc:   "letters, numbers, hyphens and underscores",
	},
	{
		ResourceType: "aws_dynamodb_table",
		NameField:    "name",
		MinLength:    3,
		MaxLength:    255,
		Pattern:      regexp.MustCompile(`^[a-zA-Z0-9_.-]+$`),
		PatternDoc:   "letters, numbers, underscores, hyphens and dots",
	},
	{
		ResourceType: "aws_sqs_queue",
		NameField:    "name",
		MinLength:    1,
		MaxLength:    80,
		Pattern:      regexp.MustCompile(`^[a-zA-Z0-9_-]+$`),
		PatternDoc:   "letters, numbers, hyphens and underscores",
	},
	{
		ResourceType: "aws_sns_topic",
		NameField:    "name",
		MinLength:    1,
		MaxLength:    256,
		Pattern:      regexp.MustCompile(`^[a-zA-Z0-9_-]+$`),
		PatternDoc:   "letters, numbers, hyphens and underscores",
	},
	{
		ResourceType: "aws_cloudwatch_log_group",
		NameField:    "name",
		MinLength:    1,
		MaxLength:    512,
	},
	{
		ResourceType: "aws_ecr_repository",
		NameField:    "name",
		MinLength:    2,
		MaxLength:    256,
		Pattern:      regexp.MustCompile(`^[a-z0-9][a-z0-9/_-]*$`),
		PatternDoc:   "lowercase letters, numbers, hyphens, underscores and forward slashes",
	},
}

// RulesFor returns the naming rules for a given resource type.
func RulesFor(resourceType string) []Rule {
	var out []Rule
	for _, r := range awsRules {
		if r.ResourceType == resourceType {
			out = append(out, r)
		}
	}
	return out
}
