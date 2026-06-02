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
	Version         string                `json:"version"`
	BaselineHash    string                `json:"baseline_hash"`
	ProposalHash    string                `json:"proposal_hash"`
	Summary         CompareSummary        `json:"summary"`
	Intervention    InterventionLoop      `json:"intervention_loop"`
	ReviewBadge     ReviewBadge           `json:"review_badge"`
	RiskDeltas      []RiskDelta           `json:"risk_deltas,omitempty"`
	GeneratedChecks []GeneratedCheck      `json:"generated_checks,omitempty"`
	Transactions    []TransactionBoundary `json:"transaction_boundaries,omitempty"`
	Idempotency     []IdempotencyClass    `json:"idempotency_classifications,omitempty"`
	LockHazards     []LockHazard          `json:"lock_concurrency_hazards,omitempty"`
	PrivacyHazards  []PrivacyHazard       `json:"data_retention_privacy_hazards,omitempty"`
	NativeChecks    []Command             `json:"native_checks,omitempty"`
	NativeResults   []NativeResult        `json:"native_results,omitempty"`
	Review          []ReviewItem          `json:"review,omitempty"`
	Hash            string                `json:"hash"`
	Markdown        string                `json:"markdown,omitempty"`
}

type CompareSummary struct {
	BaselineRisks         int  `json:"baseline_risks"`
	TargetedRisks         int  `json:"targeted_risks"`
	GeneratedFiles        int  `json:"generated_files"`
	RisksWithCoverage     int  `json:"risks_with_coverage"`
	NewHighRiskSQL        int  `json:"new_high_risk_sql"`
	NewMediumRiskSQL      int  `json:"new_medium_risk_sql"`
	RiskBudgetCovered     int  `json:"risk_budget_covered"`
	RiskBudgetAdded       int  `json:"risk_budget_added"`
	RiskBudgetRejected    bool `json:"risk_budget_rejected"`
	GuardChecks           int  `json:"guard_checks"`
	RepairChecks          int  `json:"repair_checks"`
	PatchlineChecksPassed int  `json:"patchline_checks_passed"`
	PatchlineChecksFailed int  `json:"patchline_checks_failed"`
	NativeChecksRun       int  `json:"native_checks_run"`
	NativeChecksPassed    int  `json:"native_checks_passed"`
	NativeChecksFailed    int  `json:"native_checks_failed"`
	NativeChecksSkipped   int  `json:"native_checks_skipped"`
	TransactionBoundaries int  `json:"transaction_boundaries"`
	TransactionExplicit   int  `json:"transaction_explicit"`
	TransactionMissing    int  `json:"transaction_missing"`
	TransactionPartial    int  `json:"transaction_partial"`
	IdempotencyClasses    int  `json:"idempotency_classifications"`
	IdempotencyProven     int  `json:"idempotency_proven"`
	IdempotencyGuarded    int  `json:"idempotency_guarded"`
	IdempotencyUnknown    int  `json:"idempotency_unknown"`
	IdempotencyUnsafe     int  `json:"idempotency_non_idempotent"`
	LockHazards           int  `json:"lock_concurrency_hazards"`
	LockHazardCritical    int  `json:"lock_hazard_critical"`
	LockHazardHigh        int  `json:"lock_hazard_high"`
	LockHazardMedium      int  `json:"lock_hazard_medium"`
	LockHazardLow         int  `json:"lock_hazard_low"`
	PrivacyHazards        int  `json:"data_retention_privacy_hazards"`
	PrivacyCritical       int  `json:"privacy_hazard_critical"`
	PrivacyHigh           int  `json:"privacy_hazard_high"`
	PrivacyMedium         int  `json:"privacy_hazard_medium"`
	PrivacyLow            int  `json:"privacy_hazard_low"`
	InterventionLoops     int  `json:"intervention_loops"`
	InterventionAccepted  int  `json:"intervention_accepted"`
	InterventionRejected  int  `json:"intervention_rejected"`
	Rejected              int  `json:"rejected"`
	Warnings              int  `json:"warnings"`
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

type ReviewBadge struct {
	Label      string   `json:"label"`
	Status     string   `json:"status"`
	Safe       bool     `json:"safe"`
	Reasons    []string `json:"reasons,omitempty"`
	ProofHoles []string `json:"proof_holes"`
}

type InterventionLoop struct {
	ID                  string   `json:"id"`
	Status              string   `json:"status"`
	BaselineHash        string   `json:"baseline_hash"`
	ProposalHash        string   `json:"proposal_hash"`
	ProposalStage       string   `json:"proposal_stage,omitempty"`
	TargetRiskIDs       []string `json:"target_risk_ids,omitempty"`
	GeneratedFiles      int      `json:"generated_files"`
	RisksWithCoverage   int      `json:"risks_with_coverage"`
	ChecksPassed        int      `json:"checks_passed"`
	ChecksFailed        int      `json:"checks_failed"`
	NativeChecksRun     int      `json:"native_checks_run"`
	NativeChecksFailed  int      `json:"native_checks_failed"`
	RequiredNextActions []string `json:"required_next_actions"`
	Rationale           string   `json:"rationale"`
}

type CompareOptions struct {
	RunNativeTests    bool
	NativeTestTimeout time.Duration
}

type NativeResult struct {
	Command        string                `json:"command"`
	Reason         string                `json:"reason,omitempty"`
	Status         string                `json:"status"`
	ExitCode       int                   `json:"exit_code,omitempty"`
	DurationMillis int64                 `json:"duration_millis,omitempty"`
	Log            string                `json:"log,omitempty"`
	LogHash        string                `json:"log_hash,omitempty"`
	SkippedReason  string                `json:"skipped_reason,omitempty"`
	Sandbox        *NativeSandboxProfile `json:"sandbox,omitempty"`
}

type NativeSandboxProfile struct {
	Name            string   `json:"name"`
	Ecosystem       string   `json:"ecosystem"`
	NetworkEnabled  bool     `json:"network_enabled"`
	Filesystem      string   `json:"filesystem"`
	WriteScopes     []string `json:"write_scopes"`
	TimeoutMillis   int64    `json:"timeout_millis"`
	EnvironmentKeys []string `json:"environment_keys,omitempty"`
}

type nativeCommandProfile struct {
	Args    []string
	Sandbox NativeSandboxProfile
	Env     []string
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
	report.Transactions = generatedTransactionBoundaries(proposal.Generated)
	report.Idempotency = generatedIdempotencyClasses(proposal.Generated)
	report.LockHazards = generatedLockHazards(proposal.Generated)
	report.PrivacyHazards = generatedPrivacyHazards(proposal.Generated)
	report.NativeResults = runNativeChecks(baseline.InventoryRoot, baseline.NativeChecks, opts)
	report.Summary = summarizeCompare(baseline, proposal, checks, report.RiskDeltas, report.NativeResults)
	report.Summary.TransactionBoundaries = len(report.Transactions)
	report.Summary.TransactionExplicit = countTransactionStatus(report.Transactions, "explicit")
	report.Summary.TransactionMissing = countTransactionStatus(report.Transactions, "missing")
	report.Summary.TransactionPartial = countTransactionStatus(report.Transactions, "partial")
	report.Summary.IdempotencyClasses = len(report.Idempotency)
	report.Summary.IdempotencyProven = countIdempotencyStatus(report.Idempotency, "proven")
	report.Summary.IdempotencyGuarded = countIdempotencyStatus(report.Idempotency, "guarded")
	report.Summary.IdempotencyUnknown = countIdempotencyStatus(report.Idempotency, "unknown")
	report.Summary.IdempotencyUnsafe = countIdempotencyStatus(report.Idempotency, "non_idempotent")
	report.Summary.LockHazards = len(report.LockHazards)
	report.Summary.LockHazardCritical = countLockHazardSeverity(report.LockHazards, "critical")
	report.Summary.LockHazardHigh = countLockHazardSeverity(report.LockHazards, "high")
	report.Summary.LockHazardMedium = countLockHazardSeverity(report.LockHazards, "medium")
	report.Summary.LockHazardLow = countLockHazardSeverity(report.LockHazards, "low")
	report.Summary.PrivacyHazards = len(report.PrivacyHazards)
	report.Summary.PrivacyCritical = countPrivacyHazardSeverity(report.PrivacyHazards, "critical")
	report.Summary.PrivacyHigh = countPrivacyHazardSeverity(report.PrivacyHazards, "high")
	report.Summary.PrivacyMedium = countPrivacyHazardSeverity(report.PrivacyHazards, "medium")
	report.Summary.PrivacyLow = countPrivacyHazardSeverity(report.PrivacyHazards, "low")
	report.Intervention = buildInterventionLoop(baseline, proposal, report.Summary)
	report.Summary.InterventionLoops = 1
	if report.Intervention.Status == "accepted-for-review" {
		report.Summary.InterventionAccepted = 1
	} else {
		report.Summary.InterventionRejected = 1
	}
	report.ReviewBadge = buildReviewBadge(baseline, report)
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
	if contextData, err := os.ReadFile(filepath.Join(baseDir, "prompt-context.json")); err == nil {
		_ = json.Unmarshal(contextData, &report.Context)
	}
	if promptData, err := os.ReadFile(filepath.Join(baseDir, "prompt.txt")); err == nil {
		report.Prompt = string(promptData)
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
			if !strings.Contains(lower, "select 1 from") || !strings.Contains(lower, "count(*)") || !guardHasRollbackStatement(artifact.Content) {
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
			if findings := validateRepairManifest(artifact.Content); len(findings) > 0 {
				check.Status = "fail"
				check.Findings = append(check.Findings, findings...)
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

type repairManifestCheck struct {
	Version            string                    `json:"version"`
	Trust              string                    `json:"trust"`
	RiskID             string                    `json:"risk_id"`
	Source             string                    `json:"source"`
	Scope              repairManifestScope       `json:"scope"`
	Preconditions      []string                  `json:"preconditions"`
	Postconditions     []string                  `json:"postconditions"`
	Rollback           repairManifestRollback    `json:"rollback"`
	ValidationCommands []Command                 `json:"validation_commands"`
	OwnerReview        repairManifestOwnerReview `json:"owner_review"`
}

type repairManifestScope struct {
	Table string `json:"table"`
	Where string `json:"where"`
}

type repairManifestRollback struct {
	Required bool     `json:"required"`
	Strategy string   `json:"strategy"`
	Steps    []string `json:"steps"`
}

type repairManifestOwnerReview struct {
	Required bool   `json:"required"`
	Status   string `json:"status"`
	Owner    string `json:"owner"`
}

func validateRepairManifest(content string) []string {
	var manifest repairManifestCheck
	if err := json.Unmarshal([]byte(content), &manifest); err != nil {
		return []string{"repair manifest is not valid JSON"}
	}
	var findings []string
	if manifest.Version != "patchline.generated-repair/v1" {
		findings = append(findings, "repair manifest version is missing or unsupported")
	}
	if manifest.Trust != "untrusted-generated-proposal" {
		findings = append(findings, "repair manifest must mark itself untrusted")
	}
	if strings.TrimSpace(manifest.RiskID) == "" || strings.TrimSpace(manifest.Source) == "" {
		findings = append(findings, "repair manifest lacks risk_id or source")
	}
	if strings.TrimSpace(manifest.Scope.Table) == "" || strings.TrimSpace(manifest.Scope.Where) == "" {
		findings = append(findings, "repair manifest scope must include table and where")
	}
	if len(nonEmptyStrings(manifest.Preconditions)) == 0 {
		findings = append(findings, "repair manifest lacks machine-checkable preconditions")
	}
	if len(nonEmptyStrings(manifest.Postconditions)) == 0 {
		findings = append(findings, "repair manifest lacks machine-checkable postconditions")
	}
	if !manifest.Rollback.Required || strings.TrimSpace(manifest.Rollback.Strategy) == "" || len(nonEmptyStrings(manifest.Rollback.Steps)) == 0 {
		findings = append(findings, "repair manifest rollback must be required with strategy and steps")
	}
	if len(manifest.ValidationCommands) == 0 {
		findings = append(findings, "repair manifest lacks validation commands")
	}
	for _, command := range manifest.ValidationCommands {
		if strings.TrimSpace(command.Command) == "" || strings.TrimSpace(command.Reason) == "" {
			findings = append(findings, "repair manifest validation commands need command and reason")
			break
		}
	}
	switch manifest.OwnerReview.Status {
	case "pending", "approved", "rejected":
	default:
		findings = append(findings, "repair manifest owner review status must be pending, approved, or rejected")
	}
	if !manifest.OwnerReview.Required || strings.TrimSpace(manifest.OwnerReview.Owner) == "" {
		findings = append(findings, "repair manifest owner review must be required and name an owner")
	}
	return findings
}

func nonEmptyStrings(values []string) []string {
	var out []string
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			out = append(out, value)
		}
	}
	return out
}

func guardHasRollbackStatement(content string) bool {
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(strings.ToLower(line))
		if line == "" || strings.HasPrefix(line, "--") {
			continue
		}
		if line == "rollback" || strings.HasPrefix(line, "rollback;") {
			return true
		}
	}
	return false
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

func generatedTransactionBoundaries(generated []GeneratedArtifact) []TransactionBoundary {
	var out []TransactionBoundary
	for _, artifact := range generated {
		if artifact.Content == "" || !generatedArtifactNeedsTransactionBoundary(artifact) {
			continue
		}
		status, markers := classifyTransactionBoundary(artifact.Content)
		riskID := ""
		if len(artifact.RiskIDs) > 0 {
			riskID = artifact.RiskIDs[0]
		}
		out = append(out, TransactionBoundary{
			ID:         "tx:" + canonical.Hash("generated\x00" + artifact.Path + "\x00" + canonical.Hash(artifact.Content))[:16],
			RiskID:     riskID,
			Path:       artifact.Path,
			Surface:    "generated_repair",
			Operation:  generatedArtifactOperation(artifact),
			Status:     status,
			Confidence: transactionConfidence(status, artifact.Content),
			Markers:    markers,
			Evidence:   []string{"generated artifact re-scanned by compare"},
			Rationale:  transactionRationale(status, "generated_repair"),
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Status != out[j].Status {
			return transactionStatusRank(out[i].Status) > transactionStatusRank(out[j].Status)
		}
		return out[i].Path < out[j].Path
	})
	return out
}

func generatedArtifactNeedsTransactionBoundary(artifact GeneratedArtifact) bool {
	lower := strings.ToLower(artifact.Kind + " " + artifact.Path + " " + artifact.Content)
	return containsAny(lower, "update ", "delete ", "insert ", "merge ", "drop ", "truncate ", "alter table", "create table", "repair", "rollback", "backfill")
}

func generatedArtifactOperation(artifact GeneratedArtifact) string {
	lower := strings.ToLower(artifact.Content)
	for _, op := range []string{"update", "delete", "insert", "merge", "drop", "truncate", "alter", "create"} {
		if strings.Contains(lower, op+" ") {
			return op
		}
	}
	if artifact.Kind != "" {
		return artifact.Kind
	}
	return "generated"
}

func generatedIdempotencyClasses(generated []GeneratedArtifact) []IdempotencyClass {
	var out []IdempotencyClass
	for _, artifact := range generated {
		if artifact.Content == "" || !generatedArtifactNeedsIdempotency(artifact) {
			continue
		}
		riskID := ""
		if len(artifact.RiskIDs) > 0 {
			riskID = artifact.RiskIDs[0]
		}
		operation := generatedArtifactOperation(artifact)
		status, markers := classifyIdempotency(artifact.Content, BaselineRisk{ID: riskID, Path: artifact.Path, Kind: operation}, nil)
		out = append(out, IdempotencyClass{
			ID:         "idem:" + canonical.Hash("generated\x00" + artifact.Path + "\x00" + canonical.Hash(artifact.Content))[:16],
			RiskID:     riskID,
			Path:       artifact.Path,
			Surface:    "generated_script",
			Operation:  operation,
			Status:     status,
			Confidence: idempotencyConfidence(status, artifact.Content),
			Markers:    markers,
			Evidence:   []string{"generated artifact re-scanned by compare"},
			Rationale:  idempotencyRationale(status, "generated_script"),
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Status != out[j].Status {
			return idempotencyStatusRank(out[i].Status) > idempotencyStatusRank(out[j].Status)
		}
		return out[i].Path < out[j].Path
	})
	return out
}

func generatedArtifactNeedsIdempotency(artifact GeneratedArtifact) bool {
	lower := strings.ToLower(artifact.Kind + " " + artifact.Path + " " + artifact.Content)
	return containsAny(lower, "update ", "delete ", "insert ", "merge ", "drop ", "truncate ", "alter table", "create table", "repair", "rollback", "backfill", "runbook")
}

func generatedLockHazards(generated []GeneratedArtifact) []LockHazard {
	var out []LockHazard
	for _, artifact := range generated {
		if artifact.Content == "" || !generatedArtifactNeedsLockHazard(artifact) {
			continue
		}
		riskID := ""
		if len(artifact.RiskIDs) > 0 {
			riskID = artifact.RiskIDs[0]
		}
		operation := generatedArtifactOperation(artifact)
		severity, markers, mitigations := classifyLockHazard(artifact.Content, BaselineRisk{ID: riskID, Path: artifact.Path, Kind: operation})
		out = append(out, LockHazard{
			ID:          "lock:" + canonical.Hash("generated\x00" + artifact.Path + "\x00" + canonical.Hash(artifact.Content))[:16],
			RiskID:      riskID,
			Path:        artifact.Path,
			Surface:     "generated_script",
			Operation:   operation,
			Severity:    severity,
			Confidence:  lockHazardConfidence(severity, artifact.Content),
			Markers:     markers,
			Mitigations: mitigations,
			Evidence:    []string{"generated artifact re-scanned by compare"},
			Rationale:   lockHazardRationale(severity, "generated_script"),
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Severity != out[j].Severity {
			return lockHazardSeverityRank(out[i].Severity) > lockHazardSeverityRank(out[j].Severity)
		}
		return out[i].Path < out[j].Path
	})
	return out
}

func generatedArtifactNeedsLockHazard(artifact GeneratedArtifact) bool {
	lower := strings.ToLower(artifact.Kind + " " + artifact.Path + " " + artifact.Content)
	return containsAny(lower, "alter table", "create index", "add index", "lock table", "for update", "truncate", "drop ", "update ", "delete ", "merge ", "algorithm=copy", "worker", "background", "backfill", "repair", "guard", "select count", "candidate_rows")
}

func generatedPrivacyHazards(generated []GeneratedArtifact) []PrivacyHazard {
	var out []PrivacyHazard
	for _, artifact := range generated {
		if artifact.Content == "" || !generatedArtifactNeedsPrivacyHazard(artifact) {
			continue
		}
		riskID := ""
		if len(artifact.RiskIDs) > 0 {
			riskID = artifact.RiskIDs[0]
		}
		operation := generatedArtifactOperation(artifact)
		severity, markers, mitigations := classifyPrivacyHazard(artifact.Content, BaselineRisk{ID: riskID, Path: artifact.Path, Kind: operation}, ProvenanceSlice{})
		out = append(out, PrivacyHazard{
			ID:          "privacy:" + canonical.Hash("generated\x00" + artifact.Path + "\x00" + canonical.Hash(artifact.Content))[:16],
			RiskID:      riskID,
			Path:        artifact.Path,
			Surface:     "generated_script",
			Operation:   operation,
			Severity:    severity,
			Confidence:  privacyHazardConfidence(severity, artifact.Content),
			Markers:     markers,
			Mitigations: mitigations,
			Evidence:    []string{"generated artifact re-scanned by compare"},
			Rationale:   privacyHazardRationale(severity, "generated_script"),
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Severity != out[j].Severity {
			return privacyHazardSeverityRank(out[i].Severity) > privacyHazardSeverityRank(out[j].Severity)
		}
		return out[i].Path < out[j].Path
	})
	return out
}

func generatedArtifactNeedsPrivacyHazard(artifact GeneratedArtifact) bool {
	lower := strings.ToLower(artifact.Kind + " " + artifact.Path + " " + artifact.Content)
	return containsAny(lower, "delete ", "delete from", "truncate", "drop ", "update ", "export", "dump", "select *", "copy ", "csv", "parquet", "anonym", "redact", "mask", "scrub", "privacy", "retention", "gdpr", "ccpa", "email", "phone", "address", "user", "customer", "account", "rollback", "repair", "backfill", "guard", "candidate_rows")
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
	summary.RiskBudgetCovered = summary.RisksWithCoverage
	summary.RiskBudgetAdded = summary.NewHighRiskSQL*2 + summary.NewMediumRiskSQL
	if summary.RiskBudgetAdded > summary.RiskBudgetCovered {
		summary.RiskBudgetRejected = true
		summary.Rejected++
	}
	return summary
}

func buildInterventionLoop(baseline BaselineReport, proposal ProposalReport, summary CompareSummary) InterventionLoop {
	status := "accepted-for-review"
	next := []string{
		"review generated artifacts as an untrusted intervention",
		"apply only in an isolated worktree or patch review",
		"rerun baseline and compare after any maintainer edits",
	}
	if summary.PatchlineChecksFailed > 0 || summary.NativeChecksFailed > 0 || summary.NewHighRiskSQL > 0 || summary.RiskBudgetRejected {
		status = "rejected-by-deterministic-checks"
		next = []string{
			"fix or discard generated artifacts that failed deterministic checks",
			"regenerate or edit the intervention before applying it",
			"rerun repo compare after changes",
		}
	}
	riskIDs := append([]string(nil), proposal.TargetRiskIDs...)
	sort.Strings(riskIDs)
	stage := proposal.Intervention.Stage
	if stage == "" {
		stage = "generated-untrusted"
	}
	return InterventionLoop{
		ID:                  "loop:" + canonical.Hash(baseline.Hash + "\x00" + proposal.OutputHash + "\x00" + strings.Join(riskIDs, ","))[:16],
		Status:              status,
		BaselineHash:        baseline.Hash,
		ProposalHash:        proposal.OutputHash,
		ProposalStage:       stage,
		TargetRiskIDs:       riskIDs,
		GeneratedFiles:      summary.GeneratedFiles,
		RisksWithCoverage:   summary.RisksWithCoverage,
		ChecksPassed:        summary.PatchlineChecksPassed,
		ChecksFailed:        summary.PatchlineChecksFailed,
		NativeChecksRun:     summary.NativeChecksRun,
		NativeChecksFailed:  summary.NativeChecksFailed,
		RequiredNextActions: next,
		Rationale:           "generated code is treated as an intervention in a repair-analysis loop, not as trusted completion output",
	}
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
		profile, ok := safeNativeTestProfile(command.Command, timeout)
		if !ok {
			result.SkippedReason = "command is not on the safe native-test allowlist"
			results = append(results, result)
			continue
		}
		sandbox := profile.Sandbox
		result.Sandbox = &sandbox
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
		sandboxRoot, err := os.MkdirTemp("", "patchline-native-sandbox-*")
		if err != nil {
			result.Status = "fail"
			result.ExitCode = -1
			result.Log = fmt.Sprintf("failed to create native-test sandbox: %v", err)
			result.LogHash = canonical.Hash(result.Log)
			results = append(results, result)
			continue
		}
		start := time.Now()
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		cmd := exec.CommandContext(ctx, profile.Args[0], profile.Args[1:]...)
		cmd.Dir = root
		cmd.Env = nativeSandboxEnv(os.Environ(), sandboxRoot, profile.Env)
		var output bytes.Buffer
		cmd.Stdout = &output
		cmd.Stderr = &output
		err = cmd.Run()
		cancel()
		_ = os.RemoveAll(sandboxRoot)
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

func safeNativeTestProfile(command string, timeout time.Duration) (nativeCommandProfile, bool) {
	base := NativeSandboxProfile{
		NetworkEnabled: false,
		Filesystem:     "workspace read/write with isolated HOME, cache, config, and temp directories",
		WriteScopes:    []string{"workspace", "isolated-home", "isolated-cache", "isolated-temp"},
		TimeoutMillis:  timeout.Milliseconds(),
		EnvironmentKeys: []string{
			"HOME",
			"TMPDIR",
			"XDG_CACHE_HOME",
			"XDG_CONFIG_HOME",
			"HTTP_PROXY",
			"HTTPS_PROXY",
			"ALL_PROXY",
			"NO_PROXY",
		},
	}
	switch strings.TrimSpace(command) {
	case "go test ./...":
		base.Name = "go-offline-tests"
		base.Ecosystem = "go"
		base.EnvironmentKeys = append(base.EnvironmentKeys, "GOPROXY", "GOSUMDB")
		return nativeCommandProfile{Args: []string{"go", "test", "./..."}, Sandbox: base, Env: []string{"GOPROXY=off", "GOSUMDB=off"}}, true
	case "npm test":
		base.Name = "node-offline-tests"
		base.Ecosystem = "node"
		base.EnvironmentKeys = append(base.EnvironmentKeys, "npm_config_offline", "npm_config_audit", "npm_config_fund")
		return nativeCommandProfile{Args: []string{"npm", "test"}, Sandbox: base, Env: []string{"npm_config_offline=true", "npm_config_audit=false", "npm_config_fund=false"}}, true
	case "python manage.py test":
		base.Name = "django-offline-tests"
		base.Ecosystem = "python"
		base.EnvironmentKeys = append(base.EnvironmentKeys, "PIP_NO_INDEX", "PYTHONDONTWRITEBYTECODE")
		return nativeCommandProfile{Args: []string{"python", "manage.py", "test"}, Sandbox: base, Env: []string{"PIP_NO_INDEX=1", "PYTHONDONTWRITEBYTECODE=1"}}, true
	case "pytest":
		base.Name = "python-offline-tests"
		base.Ecosystem = "python"
		base.EnvironmentKeys = append(base.EnvironmentKeys, "PIP_NO_INDEX", "PYTHONDONTWRITEBYTECODE")
		return nativeCommandProfile{Args: []string{"pytest"}, Sandbox: base, Env: []string{"PIP_NO_INDEX=1", "PYTHONDONTWRITEBYTECODE=1"}}, true
	case "bundle exec rake test":
		base.Name = "ruby-offline-tests"
		base.Ecosystem = "ruby"
		base.EnvironmentKeys = append(base.EnvironmentKeys, "BUNDLE_DISABLE_VERSION_CHECK", "BUNDLE_FROZEN")
		return nativeCommandProfile{Args: []string{"bundle", "exec", "rake", "test"}, Sandbox: base, Env: []string{"BUNDLE_DISABLE_VERSION_CHECK=true", "BUNDLE_FROZEN=true"}}, true
	default:
		return nativeCommandProfile{}, false
	}
}

func nativeSandboxEnv(base []string, sandboxRoot string, overrides []string) []string {
	env := append([]string{}, base...)
	home := filepath.Join(sandboxRoot, "home")
	tmp := filepath.Join(sandboxRoot, "tmp")
	cache := filepath.Join(sandboxRoot, "cache")
	config := filepath.Join(sandboxRoot, "config")
	_ = os.MkdirAll(home, 0o755)
	_ = os.MkdirAll(tmp, 0o755)
	_ = os.MkdirAll(cache, 0o755)
	_ = os.MkdirAll(config, 0o755)
	env = append(env,
		"HOME="+home,
		"TMPDIR="+tmp,
		"XDG_CACHE_HOME="+cache,
		"XDG_CONFIG_HOME="+config,
		"HTTP_PROXY=",
		"HTTPS_PROXY=",
		"ALL_PROXY=",
		"NO_PROXY=*",
	)
	env = append(env, overrides...)
	return env
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

func buildReviewBadge(baseline BaselineReport, report CompareReport) ReviewBadge {
	badge := ReviewBadge{
		Label:      "NOT SAFE TO REVIEW",
		Status:     "not-safe-to-review",
		ProofHoles: compareProofHoles(baseline, report.Intervention.TargetRiskIDs),
	}
	if report.Summary.PatchlineChecksFailed > 0 {
		badge.Reasons = append(badge.Reasons, "deterministic generated-artifact checks failed")
	}
	if report.Summary.NewHighRiskSQL > 0 {
		badge.Reasons = append(badge.Reasons, "generated proposal introduces high-risk SQL")
	}
	if report.Summary.RiskBudgetRejected {
		badge.Reasons = append(badge.Reasons, "generated SQL risk budget exceeds covered risks")
	}
	nativeOK, nativeReason := nativeChecksReviewable(report)
	if !nativeOK {
		badge.Reasons = append(badge.Reasons, nativeReason)
	}
	if len(badge.ProofHoles) == 0 {
		badge.ProofHoles = []string{"none recorded for targeted risks"}
	}
	if len(badge.Reasons) == 0 {
		badge.Label = "SAFE TO REVIEW"
		badge.Status = "safe-to-review"
		badge.Safe = true
		badge.Reasons = []string{"deterministic checks passed; native checks passed or are explicitly unavailable; proof holes are listed"}
	}
	return badge
}

func nativeChecksReviewable(report CompareReport) (bool, string) {
	if report.Summary.NativeChecksFailed > 0 {
		return false, "native checks failed or timed out"
	}
	if len(report.NativeChecks) == 0 {
		return true, "no native checks discovered"
	}
	for _, result := range report.NativeResults {
		if result.Status == "skipped" && strings.TrimSpace(result.SkippedReason) == "" {
			return false, "native checks were skipped without an explicit reason"
		}
	}
	return true, "native checks passed or were explicitly unavailable"
}

func compareProofHoles(baseline BaselineReport, targetRiskIDs []string) []string {
	targets := map[string]bool{}
	for _, id := range targetRiskIDs {
		targets[id] = true
	}
	if len(targets) == 0 {
		for _, risk := range baseline.Risks {
			targets[risk.ID] = true
		}
	}
	var holes []string
	for _, proof := range baseline.RepairProofs {
		if !targets[proof.RiskID] {
			continue
		}
		for _, hole := range proof.ProofHoles {
			holes = append(holes, fmt.Sprintf("%s repair proof: %s", proof.RiskID, hole))
		}
		if proof.Status != "checked" && len(proof.ProofHoles) == 0 {
			holes = append(holes, fmt.Sprintf("%s repair proof remains %s", proof.RiskID, proof.Status))
		}
	}
	for _, check := range baseline.SymbolicChecks {
		if !targets[check.RiskID] || check.Status == "pass" {
			continue
		}
		holes = append(holes, fmt.Sprintf("%s symbolic %s %s: %s", check.RiskID, check.Property, check.Status, check.Reason))
	}
	for _, check := range baseline.PolicyChecks {
		if !targets[check.RiskID] || check.Status == "pass" {
			continue
		}
		if len(check.Missing) > 0 {
			holes = append(holes, fmt.Sprintf("%s policy missing: %s", check.RiskID, strings.Join(check.Missing, ",")))
		} else {
			holes = append(holes, fmt.Sprintf("%s policy %s: %s", check.RiskID, check.Status, check.Policy))
		}
	}
	holes = uniqueSortedStrings(holes)
	if len(holes) > 20 {
		holes = holes[:20]
	}
	return holes
}

func reviewCompare(report CompareReport) []ReviewItem {
	var review []ReviewItem
	if report.Summary.PatchlineChecksFailed > 0 {
		review = append(review, ReviewItem{Severity: "error", Message: "generated proposal has failing deterministic checks and should not be applied"})
	}
	if report.Summary.NewHighRiskSQL > 0 {
		review = append(review, ReviewItem{Severity: "error", Message: "generated proposal introduces high-risk SQL"})
	}
	if report.Summary.RiskBudgetRejected {
		review = append(review, ReviewItem{Severity: "error", Message: "generated proposal adds more SQL risk budget than it covers"})
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
	fmt.Fprintf(&b, "| risk budget covered | %d |\n", report.Summary.RiskBudgetCovered)
	fmt.Fprintf(&b, "| risk budget added | %d |\n", report.Summary.RiskBudgetAdded)
	fmt.Fprintf(&b, "| checks passed | %d |\n", report.Summary.PatchlineChecksPassed)
	fmt.Fprintf(&b, "| checks failed | %d |\n", report.Summary.PatchlineChecksFailed)
	fmt.Fprintf(&b, "| native checks run | %d |\n", report.Summary.NativeChecksRun)
	fmt.Fprintf(&b, "| native checks passed | %d |\n", report.Summary.NativeChecksPassed)
	fmt.Fprintf(&b, "| native checks failed | %d |\n", report.Summary.NativeChecksFailed)
	fmt.Fprintf(&b, "| native checks skipped | %d |\n", report.Summary.NativeChecksSkipped)
	fmt.Fprintf(&b, "| transaction boundaries | %d |\n", report.Summary.TransactionBoundaries)
	fmt.Fprintf(&b, "| transaction explicit | %d |\n", report.Summary.TransactionExplicit)
	fmt.Fprintf(&b, "| transaction partial | %d |\n", report.Summary.TransactionPartial)
	fmt.Fprintf(&b, "| transaction missing | %d |\n", report.Summary.TransactionMissing)
	fmt.Fprintf(&b, "| idempotency classifications | %d |\n", report.Summary.IdempotencyClasses)
	fmt.Fprintf(&b, "| idempotency proven | %d |\n", report.Summary.IdempotencyProven)
	fmt.Fprintf(&b, "| idempotency guarded | %d |\n", report.Summary.IdempotencyGuarded)
	fmt.Fprintf(&b, "| idempotency unknown | %d |\n", report.Summary.IdempotencyUnknown)
	fmt.Fprintf(&b, "| idempotency non-idempotent | %d |\n", report.Summary.IdempotencyUnsafe)
	fmt.Fprintf(&b, "| lock/concurrency hazards | %d |\n", report.Summary.LockHazards)
	fmt.Fprintf(&b, "| lock hazard critical | %d |\n", report.Summary.LockHazardCritical)
	fmt.Fprintf(&b, "| lock hazard high | %d |\n", report.Summary.LockHazardHigh)
	fmt.Fprintf(&b, "| lock hazard medium | %d |\n", report.Summary.LockHazardMedium)
	fmt.Fprintf(&b, "| lock hazard low | %d |\n", report.Summary.LockHazardLow)
	fmt.Fprintf(&b, "| data-retention/privacy hazards | %d |\n", report.Summary.PrivacyHazards)
	fmt.Fprintf(&b, "| privacy hazard critical | %d |\n", report.Summary.PrivacyCritical)
	fmt.Fprintf(&b, "| privacy hazard high | %d |\n", report.Summary.PrivacyHigh)
	fmt.Fprintf(&b, "| privacy hazard medium | %d |\n", report.Summary.PrivacyMedium)
	fmt.Fprintf(&b, "| privacy hazard low | %d |\n", report.Summary.PrivacyLow)
	fmt.Fprintf(&b, "| intervention loops | %d |\n", report.Summary.InterventionLoops)
	fmt.Fprintf(&b, "| intervention accepted | %d |\n", report.Summary.InterventionAccepted)
	fmt.Fprintf(&b, "| intervention rejected | %d |\n\n", report.Summary.InterventionRejected)
	fmt.Fprintf(&b, "## Review badge\n\n")
	fmt.Fprintf(&b, "- label: `%s`\n", report.ReviewBadge.Label)
	fmt.Fprintf(&b, "- status: `%s`\n", report.ReviewBadge.Status)
	fmt.Fprintf(&b, "- safe: `%t`\n", report.ReviewBadge.Safe)
	if len(report.ReviewBadge.Reasons) > 0 {
		fmt.Fprintf(&b, "- reasons: %s\n", strings.Join(report.ReviewBadge.Reasons, "; "))
	}
	if len(report.ReviewBadge.ProofHoles) > 0 {
		fmt.Fprintf(&b, "\n### Listed proof holes\n\n")
		for _, hole := range report.ReviewBadge.ProofHoles {
			fmt.Fprintf(&b, "- %s\n", hole)
		}
		fmt.Fprintf(&b, "\n")
	}
	if report.Intervention.ID != "" {
		fmt.Fprintf(&b, "## Intervention loop\n\n")
		fmt.Fprintf(&b, "- id: `%s`\n", report.Intervention.ID)
		fmt.Fprintf(&b, "- status: `%s`\n", report.Intervention.Status)
		fmt.Fprintf(&b, "- stage: `%s`\n", report.Intervention.ProposalStage)
		fmt.Fprintf(&b, "- rationale: %s\n\n", report.Intervention.Rationale)
	}
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
	if len(report.Transactions) > 0 {
		fmt.Fprintf(&b, "## Generated transaction boundaries\n\n| status | path | operation | markers |\n| --- | --- | --- | --- |\n")
		for _, boundary := range report.Transactions {
			fmt.Fprintf(&b, "| %s | %s | %s | %s |\n", boundary.Status, boundary.Path, boundary.Operation, strings.Join(boundary.Markers, ", "))
		}
		fmt.Fprintf(&b, "\n")
	}
	if len(report.Idempotency) > 0 {
		fmt.Fprintf(&b, "## Generated idempotency classifications\n\n| status | path | operation | markers |\n| --- | --- | --- | --- |\n")
		for _, item := range report.Idempotency {
			fmt.Fprintf(&b, "| %s | %s | %s | %s |\n", item.Status, item.Path, item.Operation, strings.Join(item.Markers, ", "))
		}
		fmt.Fprintf(&b, "\n")
	}
	if len(report.LockHazards) > 0 {
		fmt.Fprintf(&b, "## Generated lock and concurrency hazards\n\n| severity | path | operation | markers | mitigations |\n| --- | --- | --- | --- | --- |\n")
		for _, item := range report.LockHazards {
			fmt.Fprintf(&b, "| %s | %s | %s | %s | %s |\n", item.Severity, item.Path, item.Operation, strings.Join(item.Markers, ", "), strings.Join(item.Mitigations, ", "))
		}
		fmt.Fprintf(&b, "\n")
	}
	if len(report.PrivacyHazards) > 0 {
		fmt.Fprintf(&b, "## Generated data-retention and privacy hazards\n\n| severity | path | operation | markers | mitigations |\n| --- | --- | --- | --- | --- |\n")
		for _, item := range report.PrivacyHazards {
			fmt.Fprintf(&b, "| %s | %s | %s | %s | %s |\n", item.Severity, item.Path, item.Operation, strings.Join(item.Markers, ", "), strings.Join(item.Mitigations, ", "))
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
