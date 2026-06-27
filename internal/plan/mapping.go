package plan

import (
	_ "embed"
	"fmt"

	tfjson "github.com/hashicorp/terraform-json"
	"gopkg.in/yaml.v3"
)

//go:embed mappings/aws.yaml
var awsYAML []byte

type actionEntry struct {
	Action     string `yaml:"action"`
	Confidence string `yaml:"confidence"`
	PassRole   bool   `yaml:"passrole"`
}

type resourceMapping struct {
	Create []actionEntry `yaml:"create"`
	Update []actionEntry `yaml:"update"`
	Delete []actionEntry `yaml:"delete"`
}

type mappingFile struct {
	Resources map[string]resourceMapping `yaml:"resources"`
}

// Mapper translates resource types + change actions into required IAM actions.
type Mapper struct {
	data mappingFile
}

func NewMapper() *Mapper {
	var mf mappingFile
	if err := yaml.Unmarshal(awsYAML, &mf); err != nil {
		panic(fmt.Sprintf("failed to parse embedded aws.yaml: %v", err))
	}
	return &Mapper{data: mf}
}

// RequiredActions returns the IAM actions required for a given resource type and change.
// Returns an error when the resource type is unknown (caller should emit a warning).
func (m *Mapper) RequiredActions(resourceType string, change *tfjson.Change, address string) ([]RequiredAction, error) {
	rm, ok := m.data.Resources[resourceType]
	if !ok {
		return nil, fmt.Errorf("unknown resource type: %s", resourceType)
	}

	// Terraform represents replace as [create, delete] or [delete, create].
	// We check for each action in the slice individually.
	seen := map[tfjson.Action]bool{}
	for _, a := range change.Actions {
		seen[a] = true
	}

	var entries []actionEntry
	if seen[tfjson.ActionCreate] {
		entries = append(entries, rm.Create...)
	}
	if seen[tfjson.ActionUpdate] {
		entries = append(entries, rm.Update...)
	}
	if seen[tfjson.ActionDelete] {
		entries = append(entries, rm.Delete...)
	}

	var result []RequiredAction
	for _, e := range entries {
		ra := RequiredAction{
			ResourceType:    resourceType,
			ResourceAddress: address,
			Action:          e.Action,
			ResourceArn:     "*",
			Confidence:      e.Confidence,
			ChangeActions:   actionsToStrings(change.Actions),
		}
		result = append(result, ra)
	}
	return result, nil
}

func actionsToStrings(actions tfjson.Actions) []string {
	out := make([]string, len(actions))
	for i, a := range actions {
		out[i] = string(a)
	}
	return out
}
