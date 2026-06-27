package plan

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	tfjson "github.com/hashicorp/terraform-json"
)

// Loader loads a Terraform plan from a .tfplan binary or a pre-converted .json file.
type Loader struct{}

func NewLoader() *Loader { return &Loader{} }

// Load reads the plan file and returns a parsed tfjson.Plan.
// .json  → direct unmarshal (guaranteed no-cloud path)
// .tfplan → subprocess: terraform show -json (may require backend init)
func (l *Loader) Load(ctx context.Context, path string) (*tfjson.Plan, error) {
	if _, err := os.Stat(path); err != nil {
		return nil, fmt.Errorf("plan file not found: %s", path)
	}

	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".json":
		return loadJSON(path)
	case ".tfplan":
		return loadViaTerraform(ctx, path)
	default:
		return nil, fmt.Errorf("unsupported plan file extension %q (use .tfplan or .json)", ext)
	}
}

func loadJSON(path string) (*tfjson.Plan, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading plan JSON: %w", err)
	}
	var p tfjson.Plan
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("parsing plan JSON: %w", err)
	}
	return &p, nil
}

func loadViaTerraform(ctx context.Context, path string) (*tfjson.Plan, error) {
	tfBin, err := exec.LookPath("terraform")
	if err != nil {
		return nil, fmt.Errorf("terraform binary not found in PATH; convert the plan first: terraform show -json %s > plan.json", filepath.Base(path))
	}

	cmd := exec.CommandContext(ctx, tfBin, "show", "-json", path)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("terraform show -json failed: %w", err)
	}

	var p tfjson.Plan
	if err := json.Unmarshal(out, &p); err != nil {
		return nil, fmt.Errorf("parsing terraform show output: %w", err)
	}
	return &p, nil
}
