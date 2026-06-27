package plan

import (
	"strings"
	"testing"
)

// TestAWSYAMLMinimumEntries guards against accidentally embedding an empty or
// near-empty aws.yaml (e.g. produced by a failed gen-mappings run).
// A healthy mapping file should contain several hundred entries.
func TestAWSYAMLMinimumEntries(t *testing.T) {
	m := NewMapper()
	count := len(m.data.Resources)
	if count < 50 {
		t.Errorf("aws.yaml has only %d resource entries; expected >=50. "+
			"The file may have been overwritten by a failed gen-mappings run.", count)
	}
}

// TestAWSYAMLCoreResourcesPresent verifies that well-known resource types are
// present in the embedded mapping. If these disappear, the mapping format or
// the gen-mappings output has likely regressed.
func TestAWSYAMLCoreResourcesPresent(t *testing.T) {
	m := NewMapper()
	required := []string{
		"aws_s3_bucket",
		"aws_iam_role",
		"aws_lambda_function",
		"aws_instance",
	}
	for _, rt := range required {
		if _, ok := m.data.Resources[rt]; !ok {
			t.Errorf("core resource type %q missing from aws.yaml", rt)
		}
	}
}

// TestAWSYAMLActionFormat verifies that every action entry in the mapping uses
// the expected "service:Action" format and a known confidence value.
// This catches struct-tag mismatches between gen-mappings and mapping.go.
func TestAWSYAMLActionFormat(t *testing.T) {
	m := NewMapper()
	validConfidence := map[string]bool{"high": true, "best-effort": true}

	for resourceType, rm := range m.data.Resources {
		for _, handler := range []struct {
			name    string
			actions []actionEntry
		}{
			{"create", rm.Create},
			{"update", rm.Update},
			{"delete", rm.Delete},
		} {
			for _, e := range handler.actions {
				if e.Action == "" {
					// Empty actions are a CFn schema artifact; gen-mappings now
					// skips them. Log but don't fail so existing files still pass.
					t.Logf("WARN: %s.%s has empty action (next gen-mappings run will remove it)",
						resourceType, handler.name)
					continue
				}
				if !strings.Contains(e.Action, ":") {
					t.Errorf("%s.%s: action %q is not in service:Action format",
						resourceType, handler.name, e.Action)
				}
				if !validConfidence[e.Confidence] {
					t.Errorf("%s.%s: action %q has unknown confidence %q (want high|best-effort)",
						resourceType, handler.name, e.Action, e.Confidence)
				}
			}
		}
	}
}
