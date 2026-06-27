package report

import (
	"encoding/json"
	"fmt"
	"io"
)

// FindingKind classifies permission findings.
type FindingKind int

const (
	FindingMissing FindingKind = iota // high-confidence denied → exit 1
	FindingUnknown                    // could not evaluate → warning, exit 0 (unless all unknown → exit 2)
)

// NamingKind classifies naming findings.
type NamingKind int

const (
	NamingViolation NamingKind = iota // static rule broken → exit 1
	NamingSkipped                     // name unknown after apply → info only
)

// PermFinding is a single permission evaluation result.
type PermFinding struct {
	Action          string
	ResourceAddress string
	ResourceArn     string
	Decision        FindingKind
	Confidence      string // "high" or "best-effort"
	Source          string
	Detail          string
}

// NamingFinding is a single naming validation result.
type NamingFinding struct {
	ResourceType    string
	ResourceAddress string
	NameField       string
	Name            string
	Kind            NamingKind
	Detail          string
}

// Report aggregates all findings from a single tfiam check run.
type Report struct {
	permFindings   []PermFinding
	namingFindings []NamingFinding
}

func New() *Report { return &Report{} }

func (r *Report) AddPermFindings(f []PermFinding)     { r.permFindings = append(r.permFindings, f...) }
func (r *Report) AddNamingFindings(f []NamingFinding) { r.namingFindings = append(r.namingFindings, f...) }

// Render writes the report to w and returns the appropriate exit code.
// Exit code contract:
//  1. high-confidence MISSING or NAMING violation → 1
//  2. all required actions Unknown (evaluated_count == 0, required_total > 0) → 2
//  3. internal errors → 2 (handled by caller before Render)
//  4. Unknown ≥1 but no denied → 0 (with WARNING)
//  5. no findings → 0 ("No statically-detectable issues found")
func (r *Report) Render(w io.Writer, format string) int {
	if format == "json" {
		return r.renderJSON(w)
	}
	return r.renderText(w)
}

func (r *Report) renderText(w io.Writer) int {
	exitCode := 0

	// Count perm finding metrics
	var unknown, total int
	for _, f := range r.permFindings {
		total++
		switch f.Decision {
		case FindingMissing:
			if f.Confidence == "high" {
				fmt.Fprintf(w, "MISSING: %s on %s (confidence: %s, source: %s)\n",
					f.Action, f.ResourceAddress, f.Confidence, f.Source)
				exitCode = max(exitCode, 1)
			} else {
				fmt.Fprintf(w, "WARNING: %s on %s may be denied (confidence: %s, source: %s)\n",
					f.Action, f.ResourceAddress, f.Confidence, f.Source)
				unknown++ // treat best-effort denied as unknown for coverage
			}
		case FindingUnknown:
			unknown++
			if f.Detail != "" {
				fmt.Fprintf(w, "WARNING: %s on %s — %s\n", f.Action, f.ResourceAddress, f.Detail)
			}
		}
	}

	// Naming findings
	for _, nf := range r.namingFindings {
		switch nf.Kind {
		case NamingViolation:
			fmt.Fprintf(w, "NAMING: %s %s — %s\n", nf.ResourceAddress, nf.NameField, nf.Detail)
			if exitCode < 1 {
				exitCode = 1
			}
		case NamingSkipped:
			fmt.Fprintf(w, "NAMING-SKIPPED: %s (%s)\n", nf.ResourceAddress, nf.Detail)
		}
	}

	// Coverage / Unknown summary
	evaluated := total - unknown
	if total > 0 && unknown > 0 {
		fmt.Fprintf(w, "WARNING: %d/%d actions were not evaluated (unknown)\n", unknown, total)
	}

	// All-Unknown → exit 2
	if total > 0 && evaluated == 0 && exitCode == 0 {
		fmt.Fprintf(w, "ERROR: 0/%d actions evaluated — permission source has no usable data (unknown for all)\n", total)
		return 2
	}

	if exitCode == 0 {
		fmt.Fprintln(w, "No statically-detectable issues found")
	}
	return exitCode
}

type jsonOutput struct {
	ExitCode       int              `json:"exit_code"`
	Missing        []jsonPermItem   `json:"missing,omitempty"`
	Warnings       []jsonPermItem   `json:"warnings,omitempty"`
	NamingViolations []jsonNamingItem `json:"naming_violations,omitempty"`
	NamingSkipped  []jsonNamingItem `json:"naming_skipped,omitempty"`
	Coverage       jsonCoverage     `json:"coverage"`
	Message        string           `json:"message,omitempty"`
}

type jsonPermItem struct {
	Action          string `json:"action"`
	ResourceAddress string `json:"resource_address"`
	ResourceArn     string `json:"resource_arn"`
	Confidence      string `json:"confidence"`
	Source          string `json:"source"`
	Detail          string `json:"detail,omitempty"`
}

type jsonNamingItem struct {
	ResourceAddress string `json:"resource_address"`
	NameField       string `json:"name_field"`
	Name            string `json:"name,omitempty"`
	Detail          string `json:"detail"`
}

type jsonCoverage struct {
	EvaluatedCount int `json:"evaluated_count"`
	RequiredTotal  int `json:"required_total"`
	UnknownCount   int `json:"unknown_count"`
}

func (r *Report) renderJSON(w io.Writer) int {
	out := jsonOutput{}

	var unknown, total int
	for _, f := range r.permFindings {
		total++
		item := jsonPermItem{
			Action:          f.Action,
			ResourceAddress: f.ResourceAddress,
			ResourceArn:     f.ResourceArn,
			Confidence:      f.Confidence,
			Source:          f.Source,
			Detail:          f.Detail,
		}
		switch f.Decision {
		case FindingMissing:
			if f.Confidence == "high" {
				out.Missing = append(out.Missing, item)
			} else {
				out.Warnings = append(out.Warnings, item)
				unknown++
			}
		case FindingUnknown:
			out.Warnings = append(out.Warnings, item)
			unknown++
		}
	}

	for _, nf := range r.namingFindings {
		item := jsonNamingItem{
			ResourceAddress: nf.ResourceAddress,
			NameField:       nf.NameField,
			Name:            nf.Name,
			Detail:          nf.Detail,
		}
		if nf.Kind == NamingViolation {
			out.NamingViolations = append(out.NamingViolations, item)
		} else {
			out.NamingSkipped = append(out.NamingSkipped, item)
		}
	}

	evaluated := total - unknown
	out.Coverage = jsonCoverage{
		EvaluatedCount: evaluated,
		RequiredTotal:  total,
		UnknownCount:   unknown,
	}

	exitCode := 0
	if len(out.Missing) > 0 || len(out.NamingViolations) > 0 {
		exitCode = 1
	} else if total > 0 && evaluated == 0 {
		exitCode = 2
		out.Message = fmt.Sprintf("ERROR: 0/%d actions evaluated — permission source has no usable data", total)
	} else {
		out.Message = "No statically-detectable issues found"
	}
	out.ExitCode = exitCode

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(out)
	return exitCode
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
