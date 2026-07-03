package report

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
)

type iamStatement struct {
	Sid      string   `json:"Sid"`
	Effect   string   `json:"Effect"`
	Action   []string `json:"Action"`
	Resource string   `json:"Resource"`
}

type suggestedPolicy struct {
	Version   string         `json:"Version"`
	Statement []iamStatement `json:"Statement"`
}

// SuggestPolicy writes a minimal IAM policy document covering all high-confidence
// missing actions found in the report. No-op when there are no missing actions.
func (r *Report) SuggestPolicy(w io.Writer) {
	seen := map[string]bool{}
	var actions []string
	for _, f := range r.permFindings {
		if f.Decision == FindingMissing && f.Confidence == "high" && !seen[f.Action] {
			seen[f.Action] = true
			actions = append(actions, f.Action)
		}
	}
	if len(actions) == 0 {
		return
	}
	sort.Strings(actions)

	policy := suggestedPolicy{
		Version: "2012-10-17",
		Statement: []iamStatement{
			{
				Sid:      "TfiamSuggestedPermissions",
				Effect:   "Allow",
				Action:   actions,
				Resource: "*",
			},
		},
	}

	fmt.Fprintln(w, "\n--- Suggested IAM policy to add ---")
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.Encode(policy) //nolint:errcheck
}
