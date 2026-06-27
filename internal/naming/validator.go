package naming

import (
	"fmt"

	tfjson "github.com/hashicorp/terraform-json"
	"github.com/tfiam-dev/tfiam/internal/report"
)

// Validator runs static naming checks against plan resource changes.
type Validator struct{}

func NewValidator() *Validator { return &Validator{} }

// Validate checks all resource changes for naming violations and skips.
func (v *Validator) Validate(p *tfjson.Plan) []report.NamingFinding {
	var findings []report.NamingFinding

	for _, rc := range p.ResourceChanges {
		if rc.Change == nil {
			continue
		}
		// Only check resources that are being created or updated
		if isDestroyOnly(rc.Change.Actions) {
			continue
		}

		rules := RulesFor(rc.Type)
		for _, rule := range rules {
			f := v.checkRule(rc, rule)
			if f != nil {
				findings = append(findings, *f)
			}
		}
	}

	return findings
}

func (v *Validator) checkRule(rc *tfjson.ResourceChange, rule Rule) *report.NamingFinding {
	// Extract the name value from change.After
	after, ok := rc.Change.After.(map[string]interface{})
	if !ok {
		return nil
	}

	// Check if the name field is unknown after apply
	afterUnknown, _ := rc.Change.AfterUnknown.(map[string]interface{})
	if afterUnknown != nil {
		if unknown, _ := afterUnknown[rule.NameField].(bool); unknown {
			return &report.NamingFinding{
				ResourceType:    rc.Type,
				ResourceAddress: rc.Address,
				NameField:       rule.NameField,
				Kind:            report.NamingSkipped,
				Detail:          "name known after apply",
			}
		}
	}

	rawName, exists := after[rule.NameField]
	if !exists {
		return nil
	}

	name, ok := rawName.(string)
	if !ok || name == "" {
		return nil
	}

	// Length checks
	if rule.MinLength > 0 && len(name) < rule.MinLength {
		return &report.NamingFinding{
			ResourceType:    rc.Type,
			ResourceAddress: rc.Address,
			NameField:       rule.NameField,
			Name:            name,
			Kind:            report.NamingViolation,
			Detail:          fmt.Sprintf("name %q is too short (min %d chars, got %d)", name, rule.MinLength, len(name)),
		}
	}
	if rule.MaxLength > 0 && len(name) > rule.MaxLength {
		return &report.NamingFinding{
			ResourceType:    rc.Type,
			ResourceAddress: rc.Address,
			NameField:       rule.NameField,
			Name:            name,
			Kind:            report.NamingViolation,
			Detail:          fmt.Sprintf("name %q exceeds %d chars (got %d)", name, rule.MaxLength, len(name)),
		}
	}

	// Pattern check
	if rule.Pattern != nil && !rule.Pattern.MatchString(name) {
		return &report.NamingFinding{
			ResourceType:    rc.Type,
			ResourceAddress: rc.Address,
			NameField:       rule.NameField,
			Name:            name,
			Kind:            report.NamingViolation,
			Detail:          fmt.Sprintf("name %q does not match required pattern: %s", name, rule.PatternDoc),
		}
	}

	return nil
}

func isDestroyOnly(actions tfjson.Actions) bool {
	if len(actions) != 1 {
		return false
	}
	return actions[0] == tfjson.ActionDelete || actions[0] == tfjson.ActionNoop
}
