package project

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/thehalleyyoung/patchline/internal/canonical"
	"github.com/thehalleyyoung/patchline/internal/migration"
)

const CompareVersion = "patchline.repo-compare/v1"

type CompareReport struct {
	Version         string           `json:"version"`
	BaselineHash    string           `json:"baseline_hash"`
	ProposalHash    string           `json:"proposal_hash"`
	Summary         CompareSummary   `json:"summary"`
	RiskDeltas      []RiskDelta      `json:"risk_deltas,omitempty"`
	GeneratedChecks []GeneratedCheck `json:"generated_checks,omitempty"`
	NativeChecks    []Command        `json:"native_checks,omitempty"`
	NativeResults   []NativeResult   `json:"native_results,omitempty"`
	Review          []ReviewItem     `json:"review,omitempty"`
	Hash            string           `json:"hash"`
	Markdown        string           `json:"markdown,omitempty"`
}

type CompareSummary struct {
	BaselineRisks         int `json:"baseline_risks"`
	TargetedRisks         int `json:"targeted_risks"`
	GeneratedFiles        int `json:"generated_files"`
	RisksWithCoverage     int `json:"risks_with_coverage"`
	NewHighRiskSQL        int `json:"new_high_risk_sql"`
	NewMediumRiskSQL      int `json:"new_medium_risk_sql"`
	GuardChecks           int `json:"guard_checks"`
	RepairChecks          int `json:"repair_checks"`
	PatchlineChecksPassed int `json:"patchline_checks_passed"`
	PatchlineChecksFailed int `json:"patchline_checks_failed"`
	NativeChecksRun       int `json:"native_checks_run"`
	NativeChecksPassed    int `json:"native_checks_passed"`
	NativeChecksFailed    int `json:"native_checks_failed"`
	NativeChecksSkipped   int `json:"native_checks_skipped"`
	Rejected              int `json:"rejected"`
	Warnings              int `json:"warnings"`
}

type RiskDelta struct {
	RiskID        string   `json:"risk_id"`
	BaselineScore int      `json:"baseline_score"`
	CoveredBy     []string `json:"covered_by,omitempty"`
	CoverageKinds []string `json:"coverage_kinds,omitempty"`
	Status        string   `json:"status"`
	Rationale     string   `json:"rationale"`
}

type GeneratedCheck struct {
	Path     string   `json:"path"`
	Kind     string   `json:"kind"`
	Status   string   `json:"status"`
	RiskIDs  []string `json:"risk_ids,omitempty"`
	Findings []string `json:"findings,omitempty"`
}

type ReviewItem struct {
	Severity string `json:"severity"`
	Path     string `json:"path,omitempty"`
	Message  string `json:"message"`
}

type CompareOptions struct {
	RunNativeTests    bool
	NativeTestTimeout time.Duration
}

type NativeResult struct {
	Command        string `json:"command"`
	Reason         string `json:"reason,omitempty"`
	Status         string `json:"status"`
	ExitCode       int    `json:"exit_code,omitempty"`
	DurationMillis int64  `json:"duration_millis,omitempty"`
	Log            string `json:"log,omitempty"`
	LogHash        string `json:"log_hash,omitempty"`
	SkippedReason  string `json:"skipped_reason,omitempty"`
}

func Compare(baseline BaselineReport, proposal ProposalReport) CompareReport {
	return CompareWithOptions(baseline, proposal, CompareOptions{})
}

func CompareWithOptions(baseline BaselineReport, proposal ProposalReport, opts CompareOptions) CompareReport {
	report := CompareReport{
		Version:      CompareVersion,
		BaselineHash: baseline.Hash,
		ProposalHash: proposal.OutputHash,
		NativeChecks: baseline.NativeChecks,
	}
	checks := checkGeneratedArtifacts(proposal.Generated)
	report.GeneratedChecks = checks
	report.RiskDeltas = riskDeltas(baseline.Risks, proposal.Generated)
	report.NativeResults = runNativeChecks(baseline.InventoryRoot, baseline.NativeChecks, opts)
	report.Summary = summarizeCompare(baseline, proposal, checks, report.RiskDeltas, report.NativeResults)
	report.Review = reviewCompare(report)
	report.Hash = compareHash(report)
	report.Markdown = renderCompareMarkdown(report)
	return report
}

func WriteCompare(outDir string, report CompareReport) error {
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}
	copy := report
	copy.Markdown = ""
	data, err := json.MarshalIndent(copy, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(outDir, "compare.json"), append(data, '\n'), 0o644); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(outDir, "compare.md"), []byte(report.Markdown), 0o644)
}

func LoadProposal(path string) (ProposalReport, error) {
	baseDir := path
	reportPath := path
	if info, err := os.Stat(path); err == nil && info.IsDir() {
		reportPath = filepath.Join(path, "proposal.json")
	} else {
		baseDir = filepath.Dir(path)
	}
	data, err := os.ReadFile(reportPath)
	if err != nil {
		return ProposalReport{}, err
	}
	var report ProposalReport
	if err := json.Unmarshal(data, &report); err != nil {
		return ProposalReport{}, err
	}
	for _, file := range report.GeneratedFiles {
		content, err := os.ReadFile(filepath.Join(baseDir, filepath.FromSlash(file.Path)))
		if err != nil {
			return ProposalReport{}, err
		}
		report.Generated = append(report.Generated, GeneratedArtifact{Path: file.Path, Kind: file.Kind, Content: string(content), RiskIDs: file.RiskIDs})
	}
	sort.Slice(report.Generated, func(i, j int) bool { return report.Generated[i].Path < report.Generated[j].Path })
	return report, nil
}

func checkGeneratedArtifacts(artifacts []GeneratedArtifact) []GeneratedCheck {
	var checks []GeneratedCheck
	for _, artifact := range artifacts {
		check := GeneratedCheck{Path: artifact.Path, Kind: artifact.Kind, Status: "pass", RiskIDs: artifact.RiskIDs}
		lower := strings.ToLower(artifact.Content)
		if strings.TrimSpace(artifact.Content) == "" {
			check.Status = "fail"
			check.Findings = append(check.Findings, "generated artifact is empty")
		}
		if !strings.Contains(lower, "untrusted generated") && !strings.Contains(lower, "untrusted-generated") {
			check.Status = "warn"
			check.Findings = append(check.Findings, "artifact does not clearly label itself untrusted")
		}
		switch artifact.Kind {
		case "guards":
			if !strings.Contains(lower, "select 1 from") || !strings.Contains(lower, "count(*)") || !strings.Contains(lower, "rollback") {
				check.Status = "fail"
				check.Findings = append(check.Findings, "guard does not fail closed with table existence, row count, and rollback checks")
			}
			sqlReport, err := migration.AnalyzeBytes(artifact.Path, []byte(artifact.Content))
			if err == nil {
				for _, statement := range sqlReport.Statements {
					if generatedMutationRisk(statement) {
						check.Status = "fail"
						check.Findings = append(check.Findings, fmt.Sprintf("generated guard contains %s-risk SQL: %s", statement.Risk, statement.Fingerprint))
					}
				}
			}
		case "explain":
			if !strings.Contains(lower, "explain") || !strings.Contains(lower, "count(*)") {
				check.Status = "fail"
				check.Findings = append(check.Findings, "explain proposal lacks explain and row-count checks")
			}
		case "repair":
			if !strings.Contains(lower, `"scope"`) || !strings.Contains(lower, `"rollback"`) || !strings.Contains(lower, `"postconditions"`) {
				check.Status = "fail"
				check.Findings = append(check.Findings, "repair manifest lacks scope, rollback, or postconditions")
			}
		case "tests":
			if !strings.Contains(lower, "suggested assertions") {
				check.Status = "fail"
				check.Findings = append(check.Findings, "test proposal lacks concrete suggested assertions")
			}
		case "instrumentation":
			if !strings.Contains(lower, "affected_row_count") || !strings.Contains(lower, "rollback_available") {
				check.Status = "fail"
				check.Findings = append(check.Findings, "instrumentation proposal lacks affected-row or rollback signals")
			}
		}
		checks = append(checks, check)
	}
	sort.Slice(checks, func(i, j int) bool { return checks[i].Path < checks[j].Path })
	return checks
}

func riskDeltas(risks []BaselineRisk, artifacts []GeneratedArtifact) []RiskDelta {
	coverage := map[string][]GeneratedArtifact{}
	for _, artifact := range artifacts {
		for _, id := range artifact.RiskIDs {
			coverage[id] = append(coverage[id], artifact)
		}
	}
	var out []RiskDelta
	for _, risk := range risks {
		delta := RiskDelta{RiskID: risk.ID, BaselineScore: risk.Score, Status: "uncovered", Rationale: "no generated artifact targets this baseline risk"}
		for _, artifact := range coverage[risk.ID] {
			delta.CoveredBy = append(delta.CoveredBy, artifact.Path)
			delta.CoverageKinds = append(delta.CoverageKinds, artifact.Kind)
		}
		if len(delta.CoveredBy) > 0 {
			sort.Strings(delta.CoveredBy)
			delta.CoverageKinds = uniqueStrings(delta.CoverageKinds)
			delta.Status = "covered-by-generated-artifacts"
			delta.Rationale = "generated proposal artifacts target this risk; artifacts remain untrusted until applied and re-analyzed"
		}
		out = append(out, delta)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Status != out[j].Status {
			return out[i].Status < out[j].Status
		}
		return out[i].RiskID < out[j].RiskID
	})
	return out
}

func summarizeCompare(baseline BaselineReport, proposal ProposalReport, checks []GeneratedCheck, deltas []RiskDelta, nativeResults []NativeResult) CompareSummary {
	var summary CompareSummary
	summary.BaselineRisks = len(baseline.Risks)
	summary.TargetedRisks = len(proposal.TargetRiskIDs)
	summary.GeneratedFiles = len(proposal.GeneratedFiles)
	for _, delta := range deltas {
		if delta.Status == "covered-by-generated-artifacts" {
			summary.RisksWithCoverage++
		}
	}
	for _, check := range checks {
		switch check.Status {
		case "pass":
			summary.PatchlineChecksPassed++
		case "fail":
			summary.PatchlineChecksFailed++
			summary.Rejected++
		case "warn":
			summary.Warnings++
		}
		switch check.Kind {
		case "guards":
			summary.GuardChecks++
		case "repair":
			summary.RepairChecks++
		}
		if strings.HasSuffix(check.Path, ".sql") {
			sqlReport, err := migration.AnalyzeBytes(check.Path, []byte(generatedContent(proposal.Generated, check.Path)))
			if err == nil {
				for _, statement := range sqlReport.Statements {
					if !generatedMutationRisk(statement) {
						continue
					}
					switch statement.Risk {
					case migration.RiskHigh:
						summary.NewHighRiskSQL++
					case migration.RiskMedium:
						summary.NewMediumRiskSQL++
					}
				}
			}
		}
	}
	for _, result := range nativeResults {
		switch result.Status {
		case "pass":
			summary.NativeChecksRun++
			summary.NativeChecksPassed++
		case "fail", "timeout":
			summary.NativeChecksRun++
			summary.NativeChecksFailed++
			summary.Rejected++
		case "skipped":
			summary.NativeChecksSkipped++
		}
	}
	return summary
}

func runNativeChecks(root string, commands []Command, opts CompareOptions) []NativeResult {
	commands = uniqueCommands(commands)
	if len(commands) == 0 {
		return nil
	}
	timeout := opts.NativeTestTimeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	var results []NativeResult
	for _, command := range commands {
		result := NativeResult{Command: command.Command, Reason: command.Reason, Status: "skipped"}
		args, ok := safeNativeTestArgs(command.Command)
		if !ok {
			result.SkippedReason = "command is not on the safe native-test allowlist"
			results = append(results, result)
			continue
		}
		if root == "" {
			result.SkippedReason = "baseline inventory root is unavailable"
			results = append(results, result)
			continue
		}
		if !opts.RunNativeTests {
			result.SkippedReason = "native tests were discovered but not run; pass --run-native-tests to execute safe allowlisted commands"
			results = append(results, result)
			continue
		}
		start := time.Now()
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		cmd := exec.CommandContext(ctx, args[0], args[1:]...)
		cmd.Dir = root
		var output bytes.Buffer
		cmd.Stdout = &output
		cmd.Stderr = &output
		err := cmd.Run()
		cancel()
		log := output.String()
		result.DurationMillis = time.Since(start).Milliseconds()
		result.LogHash = canonical.Hash(log)
		result.Log = truncateString(log, 64<<10)
		if ctx.Err() == context.DeadlineExceeded {
			result.Status = "timeout"
			result.ExitCode = -1
			results = append(results, result)
			continue
		}
		if err != nil {
			result.Status = "fail"
			if exitErr, ok := err.(*exec.ExitError); ok {
				result.ExitCode = exitErr.ExitCode()
			} else {
				result.ExitCode = -1
				if log == "" {
					result.Log = err.Error()
					result.LogHash = canonical.Hash(result.Log)
				}
			}
			results = append(results, result)
			continue
		}
		result.Status = "pass"
		results = append(results, result)
	}
	return results
}

func safeNativeTestArgs(command string) ([]string, bool) {
	switch strings.TrimSpace(command) {
	case "go test ./...":
		return []string{"go", "test", "./..."}, true
	case "npm test":
		return []string{"npm", "test"}, true
	case "python manage.py test":
		return []string{"python", "manage.py", "test"}, true
	case "pytest":
		return []string{"pytest"}, true
	case "bundle exec rake test":
		return []string{"bundle", "exec", "rake", "test"}, true
	default:
		return nil, false
	}
}

func truncateString(value string, limit int) string {
	if len(value) <= limit {
		return value
	}
	if limit <= 0 {
		return ""
	}
	return value[:limit]
}

func generatedMutationRisk(statement migration.Statement) bool {
	switch statement.Kind {
	case "update", "delete", "alter", "drop", "truncate", "insert", "replace", "pragma":
		return statement.Risk == migration.RiskHigh || statement.Risk == migration.RiskMedium
	default:
		return false
	}
}

func generatedContent(artifacts []GeneratedArtifact, path string) string {
	for _, artifact := range artifacts {
		if artifact.Path == path {
			return artifact.Content
		}
	}
	return ""
}

func reviewCompare(report CompareReport) []ReviewItem {
	var review []ReviewItem
	if report.Summary.PatchlineChecksFailed > 0 {
		review = append(review, ReviewItem{Severity: "error", Message: "generated proposal has failing deterministic checks and should not be applied"})
	}
	if report.Summary.NewHighRiskSQL > 0 {
		review = append(review, ReviewItem{Severity: "error", Message: "generated proposal introduces high-risk SQL"})
	}
	if report.Summary.NativeChecksFailed > 0 {
		review = append(review, ReviewItem{Severity: "error", Message: "project-native tests failed or timed out"})
	}
	if report.Summary.RisksWithCoverage < report.Summary.TargetedRisks {
		review = append(review, ReviewItem{Severity: "warning", Message: "not every targeted risk has generated artifact coverage"})
	}
	if len(review) == 0 {
		review = append(review, ReviewItem{Severity: "info", Message: "generated proposal passed deterministic checks but remains untrusted until applied and re-analyzed"})
	}
	return review
}

func compareHash(report CompareReport) string {
	copy := report
	copy.Hash = ""
	copy.Markdown = ""
	return canonical.Hash(copy)
}

func renderCompareMarkdown(report CompareReport) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Patchline repo compare\n\n")
	fmt.Fprintf(&b, "- baseline_hash: `%s`\n", report.BaselineHash)
	fmt.Fprintf(&b, "- proposal_hash: `%s`\n", report.ProposalHash)
	fmt.Fprintf(&b, "- hash: `%s`\n\n", report.Hash)
	fmt.Fprintf(&b, "## Summary\n\n| area | count |\n| --- | ---: |\n")
	fmt.Fprintf(&b, "| baseline risks | %d |\n", report.Summary.BaselineRisks)
	fmt.Fprintf(&b, "| targeted risks | %d |\n", report.Summary.TargetedRisks)
	fmt.Fprintf(&b, "| generated files | %d |\n", report.Summary.GeneratedFiles)
	fmt.Fprintf(&b, "| risks with coverage | %d |\n", report.Summary.RisksWithCoverage)
	fmt.Fprintf(&b, "| new high-risk SQL | %d |\n", report.Summary.NewHighRiskSQL)
	fmt.Fprintf(&b, "| new medium-risk SQL | %d |\n", report.Summary.NewMediumRiskSQL)
	fmt.Fprintf(&b, "| checks passed | %d |\n", report.Summary.PatchlineChecksPassed)
	fmt.Fprintf(&b, "| checks failed | %d |\n", report.Summary.PatchlineChecksFailed)
	fmt.Fprintf(&b, "| native checks run | %d |\n", report.Summary.NativeChecksRun)
	fmt.Fprintf(&b, "| native checks passed | %d |\n", report.Summary.NativeChecksPassed)
	fmt.Fprintf(&b, "| native checks failed | %d |\n", report.Summary.NativeChecksFailed)
	fmt.Fprintf(&b, "| native checks skipped | %d |\n\n", report.Summary.NativeChecksSkipped)
	if len(report.Review) > 0 {
		fmt.Fprintf(&b, "## Review\n\n")
		for _, item := range report.Review {
			fmt.Fprintf(&b, "- **%s**: %s\n", item.Severity, item.Message)
		}
		fmt.Fprintf(&b, "\n")
	}
	if len(report.NativeResults) > 0 {
		fmt.Fprintf(&b, "## Native checks\n\n| status | command | log hash | reason |\n| --- | --- | --- | --- |\n")
		for _, result := range report.NativeResults {
			reason := result.SkippedReason
			if reason == "" {
				reason = result.Reason
			}
			fmt.Fprintf(&b, "| %s | `%s` | `%s` | %s |\n", result.Status, result.Command, result.LogHash, reason)
		}
		fmt.Fprintf(&b, "\n")
	}
	return b.String()
}

func uniqueStrings(in []string) []string {
	sort.Strings(in)
	var out []string
	var last string
	for _, value := range in {
		if value == "" || value == last {
			continue
		}
		out = append(out, value)
		last = value
	}
	return out
}
