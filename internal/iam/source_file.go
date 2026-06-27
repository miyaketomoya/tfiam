package iam

import (
	"context"
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// decisionEntry is one row in perms.yaml.
type decisionEntry struct {
	Action   string `yaml:"action"`
	Resource string `yaml:"resource"`
	Decision string `yaml:"decision"`
}

// permsFile is the full structure of perms.yaml.
type permsFile struct {
	Principal   string          `yaml:"principal"`
	GeneratedAt time.Time       `yaml:"generated_at"`
	Decisions   []decisionEntry `yaml:"decisions"`
}

// FileSource implements PermissionSource using a cached perms.yaml file.
type FileSource struct {
	principal string
	// index: "action\x00resource" → Decision
	index map[string]Decision
	// index: "action\x00*" → Decision (wildcard fallback)
	wildcardIndex map[string]Decision
}

func NewFileSource(path string) (*FileSource, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading permissions file: %w", err)
	}

	var pf permsFile
	if err := yaml.Unmarshal(data, &pf); err != nil {
		return nil, fmt.Errorf("parsing permissions file: %w", err)
	}

	if pf.Principal == "" {
		return nil, fmt.Errorf("permissions file is missing required 'principal' field")
	}

	src := &FileSource{
		principal:     pf.Principal,
		index:         make(map[string]Decision),
		wildcardIndex: make(map[string]Decision),
	}

	for _, e := range pf.Decisions {
		d := parseDecision(e.Decision)
		key := e.Action + "\x00" + e.Resource
		src.index[key] = d
		if e.Resource == "*" {
			src.wildcardIndex[e.Action] = d
		}
	}

	return src, nil
}

func (s *FileSource) Name() string { return "file" }

// Evaluate performs a 2-stage lookup:
//  1. Exact match (action, resourceArn)
//  2. Wildcard fallback (action, "*")
//  3. DecisionUnknown
//
// The file's principal must match the check principal (mismatch → caller must exit 2).
func (s *FileSource) Evaluate(ctx context.Context, principal Principal, action, resourceArn string) (Decision, error) {
	if principal.ARN != s.principal {
		return DecisionUnknown, fmt.Errorf(
			"principal mismatch: permissions file is for %q but checking %q; re-generate perms.yaml for the correct principal",
			s.principal, principal.ARN,
		)
	}

	// Stage 1: exact match
	if d, ok := s.index[action+"\x00"+resourceArn]; ok {
		return d, nil
	}
	// Stage 2: wildcard fallback
	if d, ok := s.wildcardIndex[action]; ok {
		return d, nil
	}
	return DecisionUnknown, nil
}

func parseDecision(s string) Decision {
	switch s {
	case "allowed":
		return DecisionAllowed
	case "denied":
		return DecisionDenied
	default:
		return DecisionUnknown
	}
}
