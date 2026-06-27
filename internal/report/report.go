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
	requiredTotal  int // total required IAM actions extracted from plan
	permFindings   []PermFinding
	namingFindings []NamingFinding
}

// New creates a Report. requiredTotal is the count of required IAM actions from the plan
// (including those that were allowed and produced no finding). Required to calculate coverage.
func New(requiredTotal int) *Report { return &Report{requiredTotal: requiredTotal} }

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

	// Count perm finding metrics.
	// unknown = actions the source could not evaluate at all.
	// evaluated = r.requiredTotal - unknown (includes allowed + denied).
	var unknown int
	for _, f := range r.permFindings {
		switch f.Decision {
		case FindingMissing:
			if f.Confidence == "high" {
				fmt.Fprintf(w, "MISSING: %s on %s (confidence: %s, source: %s)\n",
					f.Action, f.ResourceAddress, f.Confidence, f.Source)
				exitCode = max(exitCode, 1)
			} else {
				// best-effort denied: source evaluated it but with lower confidence → warn, no exit 1
				fmt.Fprintf(w, "WARNING: %s on %s may be denied (confidence: %s, source: %s)\n",
					f.Action, f.ResourceAddress, f.Confidence, f.Source)
				// Still counts as evaluated (the source returned a decision)
			}
		case FindingUnknown:
			// Source could not evaluate → counts toward unknown, never toward evaluated
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
	total := r.requiredTotal
	evaluated := total - unknown
	if total > 0 && unknown > 0 {
		fmt.Fprintf(w, "WARNING: %d/%d actions were not evaluated (unknown)\n", unknown, total)
	}

	// All-Unknown → exit 2 (source has no usable data at all)
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

	var unknown int
	total := r.requiredTotal
	for _, f := range r.permFindings {
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
				// best-effort denied → warning but still evaluated (not unknown)
				out.Warnings = append(out.Warnings, item)
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
	enc.Encode(out) //nolint:errcheck // write-to-stdout failure is unrecoverable
	return exitCode
}
