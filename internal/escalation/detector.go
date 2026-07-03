// Package escalation performs static analysis of Terraform plans to detect
// IAM privilege escalation risks before terraform apply.
package escalation

import (
	"encoding/json"
	"fmt"
	"strings"

	tfjson "github.com/hashicorp/terraform-json"
)

// RiskLevel classifies the severity of an escalation finding.
type RiskLevel string

const (
	RiskHigh   RiskLevel = "HIGH"
	RiskMedium RiskLevel = "MEDIUM"
)

// Finding represents a potential privilege escalation risk in the plan.
type Finding struct {
	ResourceAddress string
	ResourceType    string
	Risk            RiskLevel
	Rule            string
	Detail          string
}

// dangerousManagedPolicies lists AWS managed policies that grant excessive access.
var dangerousManagedPolicies = map[string]string{
	"arn:aws:iam::aws:policy/AdministratorAccess": "grants full AWS access to all services",
	"arn:aws:iam::aws:policy/IAMFullAccess":        "grants full IAM access — enables privilege escalation",
	"arn:aws:iam::aws:policy/PowerUserAccess":      "grants broad service access excluding IAM management",
}

// Detect analyzes a Terraform plan for privilege escalation risks.
// It performs purely static analysis with no cloud API calls.
func Detect(p *tfjson.Plan) []Finding {
	var findings []Finding
	for _, rc := range p.ResourceChanges {
		if rc.Change == nil {
			continue
		}
		if !isCreateOrUpdate(rc.Change.Actions) {
			continue
		}
		after, ok := rc.Change.After.(map[string]interface{})
		if !ok {
			continue
		}

		switch rc.Type {
		case "aws_iam_role_policy_attachment",
			"aws_iam_user_policy_attachment",
			"aws_iam_group_policy_attachment":
			findings = append(findings, checkPolicyAttachment(rc, after)...)

		case "aws_iam_role_policy",
			"aws_iam_user_policy",
			"aws_iam_group_policy",
			"aws_iam_policy":
			findings = append(findings, checkInlinePolicy(rc, after)...)
		}
	}
	return findings
}

func checkPolicyAttachment(rc *tfjson.ResourceChange, after map[string]interface{}) []Finding {
	arn, _ := after["policy_arn"].(string)
	if arn == "" {
		return nil
	}
	if reason, ok := dangerousManagedPolicies[arn]; ok {
		return []Finding{{
			ResourceAddress: rc.Address,
			ResourceType:    rc.Type,
			Risk:            RiskHigh,
			Rule:            "dangerous-managed-policy",
			Detail:          fmt.Sprintf("attaches %s (%s)", arn, reason),
		}}
	}
	return nil
}

func checkInlinePolicy(rc *tfjson.ResourceChange, after map[string]interface{}) []Finding {
	policyRaw, _ := after["policy"].(string)
	if policyRaw == "" {
		return nil
	}

	var doc struct {
		Statement []struct {
			Effect   string          `json:"Effect"`
			Action   json.RawMessage `json:"Action"`
			Resource json.RawMessage `json:"Resource"`
		} `json:"Statement"`
	}
	if err := json.Unmarshal([]byte(policyRaw), &doc); err != nil {
		return nil
	}

	var findings []Finding
	for _, stmt := range doc.Statement {
		if stmt.Effect != "Allow" {
			continue
		}
		actions := parseStringOrSlice(stmt.Action)
		resources := parseStringOrSlice(stmt.Resource)

		for _, action := range actions {
			// Action: "*" — full admin
			if action == "*" {
				findings = append(findings, Finding{
					ResourceAddress: rc.Address,
					ResourceType:    rc.Type,
					Risk:            RiskHigh,
					Rule:            "wildcard-action",
					Detail: fmt.Sprintf(
						`policy allows Action:"*" on [%s] — equivalent to AdministratorAccess`,
						strings.Join(resources, ", "),
					),
				})
				goto nextStatement
			}
			// iam:* — full IAM control enables privilege escalation
			if action == "iam:*" {
				findings = append(findings, Finding{
					ResourceAddress: rc.Address,
					ResourceType:    rc.Type,
					Risk:            RiskHigh,
					Rule:            "iam-wildcard",
					Detail:          `policy allows "iam:*" — full IAM control enables privilege escalation`,
				})
				goto nextStatement
			}
		}

		// Privilege escalation combo: CreateRole/PutRolePolicy + AttachRolePolicy
		if hasAll(actions, "iam:CreateRole", "iam:AttachRolePolicy") ||
			hasAll(actions, "iam:CreateRole", "iam:PutRolePolicy") {
			findings = append(findings, Finding{
				ResourceAddress: rc.Address,
				ResourceType:    rc.Type,
				Risk:            RiskMedium,
				Rule:            "privilege-escalation-combo",
				Detail:          "policy combines iam:CreateRole with policy attachment — classic privilege escalation path",
			})
		}

	nextStatement:
	}
	return findings
}

func parseStringOrSlice(raw json.RawMessage) []string {
	if len(raw) == 0 {
		return nil
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return []string{s}
	}
	var ss []string
	if json.Unmarshal(raw, &ss) == nil {
		return ss
	}
	return nil
}

func hasAll(actions []string, targets ...string) bool {
	set := make(map[string]bool, len(actions))
	for _, a := range actions {
		set[a] = true
	}
	for _, t := range targets {
		if !set[t] {
			return false
		}
	}
	return true
}

func isCreateOrUpdate(actions tfjson.Actions) bool {
	for _, a := range actions {
		if a == tfjson.ActionCreate || a == tfjson.ActionUpdate {
			return true
		}
	}
	return false
}
