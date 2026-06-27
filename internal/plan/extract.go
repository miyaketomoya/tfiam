package plan

import (
	tfjson "github.com/hashicorp/terraform-json"
)

// RequiredAction represents an IAM action required by a plan change.
type RequiredAction struct {
	ResourceType    string
	ResourceAddress string
	Action          string
	ResourceArn     string   // "*" when unknown (known after apply)
	Confidence      string   // "high" or "best-effort"
	ChangeActions   []string // create / update / delete / replace
}

// Extractor pulls required IAM actions from plan resource changes.
type Extractor struct {
	mapper *Mapper
}

func NewExtractor() *Extractor {
	return &Extractor{mapper: NewMapper()}
}

// Extract walks resource_changes and returns all required IAM actions.
// no-op changes (Actions == ["no-op"]) are skipped.
func (e *Extractor) Extract(p *tfjson.Plan) ([]RequiredAction, error) {
	var result []RequiredAction

	for _, rc := range p.ResourceChanges {
		if rc.Change == nil {
			continue
		}
		actions := rc.Change.Actions
		if isNoOp(actions) {
			continue
		}

		required, err := e.mapper.RequiredActions(rc.Type, rc.Change, rc.Address)
		if err != nil {
			// Unknown resource type: caller records a warning, not a fatal error
			continue
		}
		result = append(result, required...)
	}

	return dedup(result), nil
}

func isNoOp(actions tfjson.Actions) bool {
	return len(actions) == 1 && actions[0] == tfjson.ActionNoop
}

// dedup removes duplicate (action, resourceArn) pairs.
func dedup(in []RequiredAction) []RequiredAction {
	seen := make(map[string]bool)
	var out []RequiredAction
	for _, ra := range in {
		key := ra.Action + "\x00" + ra.ResourceArn
		if !seen[key] {
			seen[key] = true
			out = append(out, ra)
		}
	}
	return out
}
