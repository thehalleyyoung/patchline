package project

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/thehalleyyoung/patchline/internal/canonical"
)

const RemediationPlaybookVersion = "patchline.repo-playbook/v1"

type RemediationPlaybookReport struct {
	Version      string                     `json:"version"`
	BaselineHash string                     `json:"baseline_hash"`
	Summary      RemediationPlaybookSummary `json:"summary"`
	Playbooks    []RemediationPlaybook      `json:"playbooks,omitempty"`
	Hash         string                     `json:"hash"`
	Markdown     string                     `json:"markdown,omitempty"`
}

type RemediationPlaybookSummary struct {
	Risks                int `json:"risks"`
	Playbooks            int `json:"playbooks"`
	HazardClasses        int `json:"hazard_classes"`
	RunbookSteps         int `json:"runbook_steps"`
	RollbackPoints       int `json:"rollback_points"`
	OwnerHandoffs        int `json:"owner_handoffs"`
	ValidationCommands   int `json:"validation_commands"`
	CodeownersHandoffs   int `json:"codeowners_handoffs"`
	FallbackRoleHandoffs int `json:"fallback_role_handoffs"`
}

type RemediationPlaybook struct {
	ID                 string                     `json:"id"`
	RiskID             string                     `json:"risk_id"`
	StableRiskID       string                     `json:"stable_risk_id,omitempty"`
	EvidenceHash       string                     `json:"evidence_hash,omitempty"`
	Path               string                     `json:"path"`
	Table              string                     `json:"table,omitempty"`
	Severity           string                     `json:"severity"`
	Score              int                        `json:"score,omitempty"`
	HazardClasses      []RemediationHazardClass   `json:"hazard_classes"`
	RunbookSteps       []RemediationRunbookStep   `json:"runbook_steps"`
	RollbackPoints     []RemediationRollbackPoint `json:"rollback_points"`
	OwnerHandoffs      []RemediationOwnerHandoff  `json:"owner_handoffs"`
	ValidationCommands []Command                  `json:"validation_commands,omitempty"`
	EvidencePaths      []string                   `json:"evidence_paths,omitempty"`
	NextCommand        string                     `json:"next_command,omitempty"`
	Rationale          string                     `json:"rationale"`
}

type RemediationHazardClass struct {
	ID          string   `json:"id"`
	Class       string   `json:"class"`
	Source      string   `json:"source"`
	Severity    string   `json:"severity,omitempty"`
	Status      string   `json:"status,omitempty"`
	Evidence    []string `json:"evidence,omitempty"`
	Mitigations []string `json:"mitigations,omitempty"`
	Rationale   string   `json:"rationale"`
}

type RemediationRunbookStep struct {
	ID            string   `json:"id"`
	Order         int      `json:"order"`
	Phase         string   `json:"phase"`
	Action        string   `json:"action"`
	OwnerRole     string   `json:"owner_role"`
	HazardClasses []string `json:"hazard_classes"`
	Evidence      []string `json:"evidence,omitempty"`
	ExitCriteria  string   `json:"exit_criteria"`
}

type RemediationRollbackPoint struct {
	ID            string   `json:"id"`
	Stage         string   `json:"stage"`
	Action        string   `json:"action"`
	Trigger       string   `json:"trigger"`
	OwnerRole     string   `json:"owner_role"`
	HazardClasses []string `json:"hazard_classes"`
	Evidence      []string `json:"evidence,omitempty"`
	Limit         string   `json:"limit"`
}

type RemediationOwnerHandoff struct {
	Role       string   `json:"role"`
	Owners     []string `json:"owners"`
	Source     string   `json:"source"`
	Confidence string   `json:"confidence"`
	Rationale  string   `json:"rationale"`
}

type playbookAccumulator struct {
	risk    BaselineRisk
	hazards []RemediationHazardClass
	paths   []string
}

func BuildRemediationPlaybook(baseline BaselineReport) RemediationPlaybookReport {
	accs := map[string]*playbookAccumulator{}
	for _, risk := range baseline.Risks {
		acc := ensurePlaybookAccumulator(accs, risk)
		for _, class := range hazardClassesFromRisk(risk) {
			acc.hazards = append(acc.hazards, class)
		}
	}
	risksByID := map[string]BaselineRisk{}
	for _, risk := range baseline.Risks {
		risksByID[risk.ID] = risk
	}
	for _, item := range baseline.Transactions {
		if shouldIncludeTransactionHazard(item) {
			addHazardForRisk(accs, risksByID, item.RiskID, transactionHazardClass(item), item.Path)
		}
	}
	for _, item := range baseline.Idempotency {
		if shouldIncludeIdempotencyHazard(item) {
			addHazardForRisk(accs, risksByID, item.RiskID, idempotencyHazardClass(item), item.Path)
		}
	}
	for _, item := range baseline.LockHazards {
		addHazardForRisk(accs, risksByID, item.RiskID, lockHazardClass(item), item.Path)
	}
	for _, item := range baseline.PrivacyHazards {
		addHazardForRisk(accs, risksByID, item.RiskID, privacyHazardClass(item), item.Path)
	}
	for _, item := range baseline.BlastRadius {
		if item.Level == "high" || item.Level == "medium" {
			addHazardForRisk(accs, risksByID, item.RiskID, blastRadiusHazardClass(item), "")
		}
	}
	for _, check := range baseline.PolicyChecks {
		if check.Status == "fail" || check.Status == "warn" {
			addHazardForRisk(accs, risksByID, check.RiskID, policyHazardClass(check), "")
		}
	}
	for _, proof := range baseline.RepairProofs {
		if proof.Status != "checked" {
			addHazardForRisk(accs, risksByID, proof.RiskID, repairProofHazardClass(proof), "")
		}
	}
	for _, hole := range baseline.ProofMinimizers {
		addHazardForRisk(accs, risksByID, hole.RiskID, proofHoleHazardClass(hole), "")
	}

	ownerMap := routeOwnersByRisk(baseline.OwnerRoutes)
	var playbooks []RemediationPlaybook
	for _, acc := range sortedPlaybookAccumulators(accs) {
		acc.hazards = uniqueHazardClasses(acc.hazards)
		if len(acc.hazards) == 0 {
			continue
		}
		sortHazardClasses(acc.hazards)
		playbook := RemediationPlaybook{
			ID:                 "playbook:" + canonical.Hash("remediation\x00" + acc.risk.ID)[:16],
			RiskID:             acc.risk.ID,
			StableRiskID:       acc.risk.StableID,
			EvidenceHash:       acc.risk.EvidenceHash,
			Path:               acc.risk.Path,
			Table:              acc.risk.Table,
			Severity:           acc.risk.Severity,
			Score:              acc.risk.Score,
			HazardClasses:      acc.hazards,
			RunbookSteps:       runbookStepsForHazards(acc.risk, acc.hazards),
			RollbackPoints:     rollbackPointsForHazards(acc.risk, acc.hazards),
			OwnerHandoffs:      ownerHandoffsForHazards(acc.risk, acc.hazards, ownerMap[acc.risk.ID]),
			ValidationCommands: validationCommandsForRisk(acc.risk, baseline.NativeChecks),
			EvidencePaths:      evidencePathsForRisk(acc.risk, acc.hazards, acc.paths),
			NextCommand:        acc.risk.NextCommand,
			Rationale:          "remediation playbook maps detected hazard classes to ordered runbook actions, rollback decision points, and owner handoffs using baseline evidence only",
		}
		playbooks = append(playbooks, playbook)
	}
	sort.Slice(playbooks, func(i, j int) bool {
		if playbooks[i].Severity != playbooks[j].Severity {
			return severityRank(playbooks[i].Severity) > severityRank(playbooks[j].Severity)
		}
		if playbooks[i].Score != playbooks[j].Score {
			return playbooks[i].Score > playbooks[j].Score
		}
		return playbooks[i].RiskID < playbooks[j].RiskID
	})
	report := RemediationPlaybookReport{
		Version:      RemediationPlaybookVersion,
		BaselineHash: baseline.Hash,
		Playbooks:    playbooks,
	}
	report.Summary = summarizeRemediationPlaybook(baseline, playbooks)
	report.Hash = remediationPlaybookHash(report)
	report.Markdown = RenderRemediationPlaybookMarkdown(report)
	return report
}

func WriteRemediationPlaybook(outDir string, report RemediationPlaybookReport) error {
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}
	copy := report
	copy.Markdown = ""
	data, err := json.MarshalIndent(copy, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(outDir, "playbook.json"), append(data, '\n'), 0o644); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(outDir, "playbook.md"), []byte(report.Markdown), 0o644)
}

func RenderRemediationPlaybookMarkdown(report RemediationPlaybookReport) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Remediation playbooks\n\n")
	fmt.Fprintf(&b, "Generated from baseline `%s`; report hash `%s`.\n\n", report.BaselineHash, report.Hash)
	fmt.Fprintf(&b, "## Summary\n\n| area | count |\n| --- | ---: |\n")
	fmt.Fprintf(&b, "| risks | %d |\n", report.Summary.Risks)
	fmt.Fprintf(&b, "| playbooks | %d |\n", report.Summary.Playbooks)
	fmt.Fprintf(&b, "| hazard classes | %d |\n", report.Summary.HazardClasses)
	fmt.Fprintf(&b, "| runbook steps | %d |\n", report.Summary.RunbookSteps)
	fmt.Fprintf(&b, "| rollback points | %d |\n", report.Summary.RollbackPoints)
	fmt.Fprintf(&b, "| owner handoffs | %d |\n\n", report.Summary.OwnerHandoffs)
	for _, playbook := range report.Playbooks {
		fmt.Fprintf(&b, "## `%s`\n\n", firstNonEmpty(playbook.StableRiskID, playbook.RiskID))
		fmt.Fprintf(&b, "- path: `%s`\n", playbook.Path)
		fmt.Fprintf(&b, "- table: `%s`\n", playbook.Table)
		fmt.Fprintf(&b, "- severity: `%s`\n", playbook.Severity)
		if len(playbook.OwnerHandoffs) > 0 {
			var handoffs []string
			for _, handoff := range playbook.OwnerHandoffs {
				handoffs = append(handoffs, fmt.Sprintf("%s=%s", handoff.Role, strings.Join(handoff.Owners, ",")))
			}
			fmt.Fprintf(&b, "- owner handoffs: %s\n", strings.Join(handoffs, "; "))
		}
		fmt.Fprintf(&b, "\n### Hazard classes\n\n| class | source | severity/status | rationale |\n| --- | --- | --- | --- |\n")
		for _, hazard := range playbook.HazardClasses {
			status := firstNonEmpty(hazard.Severity, hazard.Status, "-")
			fmt.Fprintf(&b, "| `%s` | %s | %s | %s |\n", hazard.Class, hazard.Source, status, hazard.Rationale)
		}
		fmt.Fprintf(&b, "\n### Runbook steps\n\n| order | phase | owner | action | exit criteria |\n| ---: | --- | --- | --- | --- |\n")
		for _, step := range playbook.RunbookSteps {
			fmt.Fprintf(&b, "| %d | %s | %s | %s | %s |\n", step.Order, step.Phase, step.OwnerRole, step.Action, step.ExitCriteria)
		}
		fmt.Fprintf(&b, "\n### Rollback points\n\n| stage | owner | trigger | action | limit |\n| --- | --- | --- | --- | --- |\n")
		for _, rollback := range playbook.RollbackPoints {
			fmt.Fprintf(&b, "| %s | %s | %s | %s | %s |\n", rollback.Stage, rollback.OwnerRole, rollback.Trigger, rollback.Action, rollback.Limit)
		}
		if len(playbook.ValidationCommands) > 0 {
			fmt.Fprintf(&b, "\n### Validation commands\n\n")
			for _, command := range playbook.ValidationCommands {
				fmt.Fprintf(&b, "- `%s` — %s\n", command.Command, command.Reason)
			}
		}
		fmt.Fprintf(&b, "\n")
	}
	return b.String()
}

func ensurePlaybookAccumulator(accs map[string]*playbookAccumulator, risk BaselineRisk) *playbookAccumulator {
	if acc, ok := accs[risk.ID]; ok {
		return acc
	}
	acc := &playbookAccumulator{risk: risk}
	accs[risk.ID] = acc
	return acc
}

func addHazardForRisk(accs map[string]*playbookAccumulator, risksByID map[string]BaselineRisk, riskID string, hazard RemediationHazardClass, path string) {
	risk, ok := risksByID[riskID]
	if !ok || strings.TrimSpace(risk.ID) == "" || strings.TrimSpace(hazard.Class) == "" {
		return
	}
	acc := ensurePlaybookAccumulator(accs, risk)
	acc.hazards = append(acc.hazards, hazard)
	if path != "" {
		acc.paths = append(acc.paths, path)
	}
}

func sortedPlaybookAccumulators(accs map[string]*playbookAccumulator) []*playbookAccumulator {
	out := make([]*playbookAccumulator, 0, len(accs))
	for _, acc := range accs {
		out = append(out, acc)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].risk.Score != out[j].risk.Score {
			return out[i].risk.Score > out[j].risk.Score
		}
		return out[i].risk.ID < out[j].risk.ID
	})
	return out
}

func hazardClassesFromRisk(risk BaselineRisk) []RemediationHazardClass {
	var hazards []RemediationHazardClass
	for _, factor := range risk.Factors {
		class := hazardClassForFactor(factor.Name)
		if class == "" {
			continue
		}
		hazards = append(hazards, RemediationHazardClass{
			ID:        "hazard:" + canonical.Hash(risk.ID + "\x00factor\x00" + factor.Name)[:16],
			Class:     class,
			Source:    "risk-factor",
			Severity:  risk.Severity,
			Evidence:  []string{factor.Name},
			Rationale: factor.Reason,
		})
	}
	if len(hazards) == 0 && risk.Severity == "high" {
		hazards = append(hazards, RemediationHazardClass{
			ID:        "hazard:" + canonical.Hash(risk.ID + "\x00high-risk")[:16],
			Class:     "high-risk-data-change",
			Source:    "risk",
			Severity:  risk.Severity,
			Rationale: risk.Rationale,
		})
	}
	return hazards
}

func hazardClassForFactor(factor string) string {
	factor = strings.ToLower(strings.TrimSpace(factor))
	switch {
	case strings.Contains(factor, "destructive"):
		return "destructive-change"
	case strings.Contains(factor, "broad-write"):
		return "broad-write"
	case strings.Contains(factor, "transaction"):
		return "transaction-boundary"
	case strings.Contains(factor, "idempot"):
		return "idempotency"
	case strings.Contains(factor, "rollback"):
		return "rollback-readiness"
	case strings.Contains(factor, "retry"):
		return "retry-concurrency"
	case strings.Contains(factor, "infrastructure"):
		return "infrastructure-ordering"
	case strings.Contains(factor, "high-risk"):
		return "high-risk-data-change"
	default:
		return ""
	}
}

func shouldIncludeTransactionHazard(item TransactionBoundary) bool {
	return item.RiskID != "" && (item.Status == "missing" || item.Status == "partial")
}

func transactionHazardClass(item TransactionBoundary) RemediationHazardClass {
	return RemediationHazardClass{
		ID:        firstNonEmpty(item.ID, "hazard:"+canonical.Hash(item.RiskID + "\x00transaction")[:16]),
		Class:     "transaction-boundary",
		Source:    "transaction_boundary",
		Status:    item.Status,
		Evidence:  append([]string(nil), item.Evidence...),
		Rationale: item.Rationale,
	}
}

func shouldIncludeIdempotencyHazard(item IdempotencyClass) bool {
	return item.RiskID != "" && (item.Status == "non_idempotent" || item.Status == "unknown")
}

func idempotencyHazardClass(item IdempotencyClass) RemediationHazardClass {
	return RemediationHazardClass{
		ID:        firstNonEmpty(item.ID, "hazard:"+canonical.Hash(item.RiskID + "\x00idempotency")[:16]),
		Class:     "idempotency",
		Source:    "idempotency_classification",
		Status:    item.Status,
		Evidence:  append([]string(nil), item.Evidence...),
		Rationale: item.Rationale,
	}
}

func lockHazardClass(item LockHazard) RemediationHazardClass {
	return RemediationHazardClass{
		ID:          firstNonEmpty(item.ID, "hazard:"+canonical.Hash(item.RiskID + "\x00lock")[:16]),
		Class:       "lock-concurrency",
		Source:      "lock_concurrency_hazard",
		Severity:    item.Severity,
		Evidence:    append([]string(nil), item.Evidence...),
		Mitigations: append([]string(nil), item.Mitigations...),
		Rationale:   item.Rationale,
	}
}

func privacyHazardClass(item PrivacyHazard) RemediationHazardClass {
	return RemediationHazardClass{
		ID:          firstNonEmpty(item.ID, "hazard:"+canonical.Hash(item.RiskID + "\x00privacy")[:16]),
		Class:       "privacy-retention",
		Source:      "data_retention_privacy_hazard",
		Severity:    item.Severity,
		Evidence:    append([]string(nil), item.Evidence...),
		Mitigations: append([]string(nil), item.Mitigations...),
		Rationale:   item.Rationale,
	}
}

func blastRadiusHazardClass(item BlastRadiusEstimate) RemediationHazardClass {
	return RemediationHazardClass{
		ID:        firstNonEmpty(item.ID, "hazard:"+canonical.Hash(item.RiskID + "\x00blast")[:16]),
		Class:     "blast-radius",
		Source:    "blast_radius_estimate",
		Severity:  item.Level,
		Evidence:  append([]string(nil), item.Evidence...),
		Rationale: item.Rationale,
	}
}

func policyHazardClass(check PolicyCheck) RemediationHazardClass {
	return RemediationHazardClass{
		ID:        firstNonEmpty(check.ID, "hazard:"+canonical.Hash(check.RiskID + "\x00policy")[:16]),
		Class:     "policy-obligation",
		Source:    "policy_check",
		Status:    check.Status,
		Evidence:  append([]string(nil), check.Evidence...),
		Rationale: check.Rationale,
	}
}

func repairProofHazardClass(proof RepairProofSummary) RemediationHazardClass {
	return RemediationHazardClass{
		ID:        firstNonEmpty(proof.ID, "hazard:"+canonical.Hash(proof.RiskID + "\x00repair-proof")[:16]),
		Class:     "repair-proof",
		Source:    "repair_proof_summary",
		Status:    proof.Status,
		Evidence:  append([]string(nil), proof.Evidence...),
		Rationale: proof.Rationale,
	}
}

func proofHoleHazardClass(hole ProofHoleMinimization) RemediationHazardClass {
	return RemediationHazardClass{
		ID:        firstNonEmpty(hole.ID, "hazard:"+canonical.Hash(hole.RiskID + "\x00proof-hole")[:16]),
		Class:     "proof-hole",
		Source:    "proof_hole_minimization",
		Severity:  hole.Priority,
		Evidence:  append([]string(nil), hole.Evidence...),
		Rationale: hole.Rationale,
	}
}

func uniqueHazardClasses(in []RemediationHazardClass) []RemediationHazardClass {
	seen := map[string]bool{}
	var out []RemediationHazardClass
	for _, hazard := range in {
		if hazard.Class == "" {
			continue
		}
		key := hazard.Class + "\x00" + hazard.Source + "\x00" + hazard.ID
		if seen[key] {
			continue
		}
		seen[key] = true
		hazard.Evidence = uniqueSortedStrings(hazard.Evidence)
		hazard.Mitigations = uniqueSortedStrings(hazard.Mitigations)
		out = append(out, hazard)
	}
	return out
}

func sortHazardClasses(hazards []RemediationHazardClass) {
	sort.Slice(hazards, func(i, j int) bool {
		if hazardClassPriority(hazards[i].Class) != hazardClassPriority(hazards[j].Class) {
			return hazardClassPriority(hazards[i].Class) < hazardClassPriority(hazards[j].Class)
		}
		if hazards[i].Class != hazards[j].Class {
			return hazards[i].Class < hazards[j].Class
		}
		return hazards[i].ID < hazards[j].ID
	})
}

func hazardClassPriority(class string) int {
	switch class {
	case "destructive-change":
		return 0
	case "broad-write":
		return 1
	case "transaction-boundary":
		return 2
	case "idempotency":
		return 3
	case "lock-concurrency", "retry-concurrency":
		return 4
	case "privacy-retention":
		return 5
	case "blast-radius":
		return 6
	case "rollback-readiness", "repair-proof", "proof-hole":
		return 7
	case "policy-obligation":
		return 8
	default:
		return 9
	}
}

func runbookStepsForHazards(risk BaselineRisk, hazards []RemediationHazardClass) []RemediationRunbookStep {
	classes := hazardClassNames(hazards)
	steps := []RemediationRunbookStep{{
		ID:            "step:" + canonical.Hash(risk.ID + "\x00scope")[:16],
		Order:         1,
		Phase:         "scope",
		OwnerRole:     "application-owner",
		HazardClasses: classes,
		Action:        fmt.Sprintf("Confirm the target scope for `%s` in `%s` before running or accepting the change.", firstNonEmpty(risk.Table, "affected data"), risk.Path),
		Evidence:      []string{risk.Rationale},
		ExitCriteria:  "affected table, path, risk id, and expected row/schema scope are written in the runbook",
	}}
	order := 2
	if hasHazardClass(hazards, "destructive-change") || hasHazardClass(hazards, "broad-write") || hasHazardClass(hazards, "blast-radius") {
		steps = append(steps, RemediationRunbookStep{
			ID:            "step:" + canonical.Hash(risk.ID + "\x00blast")[:16],
			Order:         order,
			Phase:         "preflight",
			OwnerRole:     "database-owner",
			HazardClasses: intersectionClasses(classes, "destructive-change", "broad-write", "blast-radius", "high-risk-data-change"),
			Action:        "Capture row-count bounds, affected-table list, and a dry-run or explain result before production rollout.",
			Evidence:      hazardEvidence(hazards, "destructive-change", "broad-write", "blast-radius", "high-risk-data-change"),
			ExitCriteria:  "preflight evidence proves the change is bounded or explicitly escalated for manual approval",
		})
		order++
	}
	if hasHazardClass(hazards, "transaction-boundary") || hasHazardClass(hazards, "lock-concurrency") || hasHazardClass(hazards, "retry-concurrency") {
		steps = append(steps, RemediationRunbookStep{
			ID:            "step:" + canonical.Hash(risk.ID + "\x00concurrency")[:16],
			Order:         order,
			Phase:         "coordination",
			OwnerRole:     "sre-owner",
			HazardClasses: intersectionClasses(classes, "transaction-boundary", "lock-concurrency", "retry-concurrency"),
			Action:        "Schedule the change with deploy ordering, lock monitoring, retry suppression, and a clear stop condition.",
			Evidence:      hazardEvidence(hazards, "transaction-boundary", "lock-concurrency", "retry-concurrency"),
			ExitCriteria:  "rollout window, lock threshold, retry policy, and abort owner are recorded",
		})
		order++
	}
	if hasHazardClass(hazards, "idempotency") {
		steps = append(steps, RemediationRunbookStep{
			ID:            "step:" + canonical.Hash(risk.ID + "\x00idempotency")[:16],
			Order:         order,
			Phase:         "execution",
			OwnerRole:     "application-owner",
			HazardClasses: []string{"idempotency"},
			Action:        "Make rerun semantics explicit: either prove idempotency or require a one-shot execution guard with captured completion state.",
			Evidence:      hazardEvidence(hazards, "idempotency"),
			ExitCriteria:  "rerun behavior is proven safe, guarded, or blocked from automatic retry",
		})
		order++
	}
	if hasHazardClass(hazards, "privacy-retention") {
		steps = append(steps, RemediationRunbookStep{
			ID:            "step:" + canonical.Hash(risk.ID + "\x00privacy")[:16],
			Order:         order,
			Phase:         "review",
			OwnerRole:     "privacy-owner",
			HazardClasses: []string{"privacy-retention"},
			Action:        "Review retention, anonymization, export, and rollback obligations before touching protected data.",
			Evidence:      hazardEvidence(hazards, "privacy-retention"),
			ExitCriteria:  "privacy owner accepts the data-retention impact or blocks rollout",
		})
		order++
	}
	steps = append(steps, RemediationRunbookStep{
		ID:            "step:" + canonical.Hash(risk.ID + "\x00verify")[:16],
		Order:         order,
		Phase:         "verification",
		OwnerRole:     "release-owner",
		HazardClasses: classes,
		Action:        "Run the grounded validation commands and attach generated baseline/playbook evidence to the release record.",
		Evidence:      hazardEvidence(hazards, classes...),
		ExitCriteria:  "validation commands, proof holes, and unresolved owner decisions are recorded",
	})
	return steps
}

func rollbackPointsForHazards(risk BaselineRisk, hazards []RemediationHazardClass) []RemediationRollbackPoint {
	classes := hazardClassNames(hazards)
	points := []RemediationRollbackPoint{{
		ID:            "rollback:" + canonical.Hash(risk.ID + "\x00before")[:16],
		Stage:         "before-execution",
		OwnerRole:     "database-owner",
		HazardClasses: classes,
		Trigger:       "preflight scope, owner approval, or validation evidence is missing",
		Action:        "do not start the change; capture missing evidence and rerun the playbook after baseline evidence is updated",
		Evidence:      hazardEvidence(hazards, classes...),
		Limit:         "safe before data or schema mutation begins",
	}}
	if hasHazardClass(hazards, "transaction-boundary") || hasHazardClass(hazards, "lock-concurrency") || hasHazardClass(hazards, "retry-concurrency") {
		points = append(points, RemediationRollbackPoint{
			ID:            "rollback:" + canonical.Hash(risk.ID + "\x00during")[:16],
			Stage:         "during-execution",
			OwnerRole:     "sre-owner",
			HazardClasses: intersectionClasses(classes, "transaction-boundary", "lock-concurrency", "retry-concurrency"),
			Trigger:       "lock wait, retry amplification, or transaction boundary deviates from the approved window",
			Action:        "pause writers or abort the rollout at the transaction/checkpoint boundary defined in the runbook",
			Evidence:      hazardEvidence(hazards, "transaction-boundary", "lock-concurrency", "retry-concurrency"),
			Limit:         "may require manual cleanup if the baseline classifies rollback readiness as weak",
		})
	}
	if hasHazardClass(hazards, "destructive-change") || hasHazardClass(hazards, "privacy-retention") || hasHazardClass(hazards, "broad-write") {
		points = append(points, RemediationRollbackPoint{
			ID:            "rollback:" + canonical.Hash(risk.ID + "\x00after")[:16],
			Stage:         "after-execution",
			OwnerRole:     "incident-owner",
			HazardClasses: intersectionClasses(classes, "destructive-change", "privacy-retention", "broad-write", "rollback-readiness", "repair-proof"),
			Trigger:       "postcondition, privacy obligation, or repair proof fails",
			Action:        "use the approved repair or restore path; if none exists, escalate instead of inventing an unverified reverse patch",
			Evidence:      hazardEvidence(hazards, "destructive-change", "privacy-retention", "broad-write", "rollback-readiness", "repair-proof"),
			Limit:         "destructive or privacy-affecting changes may be irreversible without prior snapshot or repair proof",
		})
	}
	return points
}

func ownerHandoffsForHazards(risk BaselineRisk, hazards []RemediationHazardClass, codeowners []string) []RemediationOwnerHandoff {
	var out []RemediationOwnerHandoff
	if len(codeowners) > 0 {
		out = append(out, RemediationOwnerHandoff{
			Role:       "code-owner",
			Owners:     uniqueSortedStrings(append([]string(nil), codeowners...)),
			Source:     "baseline.owner_routes",
			Confidence: "codeowners",
			Rationale:  "baseline CODEOWNERS route matched the risky path",
		})
	}
	roleSet := map[string]bool{"application-owner": true, "database-owner": true}
	for _, hazard := range hazards {
		switch hazard.Class {
		case "lock-concurrency", "retry-concurrency", "transaction-boundary", "infrastructure-ordering":
			roleSet["sre-owner"] = true
		case "privacy-retention":
			roleSet["privacy-owner"] = true
		case "proof-hole", "repair-proof", "policy-obligation":
			roleSet["release-owner"] = true
		case "destructive-change", "broad-write", "blast-radius":
			roleSet["incident-owner"] = true
		}
	}
	var roles []string
	for role := range roleSet {
		roles = append(roles, role)
	}
	sort.Strings(roles)
	for _, role := range roles {
		out = append(out, RemediationOwnerHandoff{
			Role:       role,
			Owners:     []string{role},
			Source:     "patchline.role-fallback",
			Confidence: "role",
			Rationale:  fmt.Sprintf("fallback %s handoff required for risk %s because the playbook cannot assume a named owner beyond baseline routes", role, risk.ID),
		})
	}
	return out
}

func validationCommandsForRisk(risk BaselineRisk, native []Command) []Command {
	var commands []Command
	if risk.NextCommand != "" {
		commands = append(commands, Command{Command: risk.NextCommand, Reason: "risk-specific Patchline re-analysis command from baseline"})
	}
	for _, command := range native {
		if command.Command == "" {
			continue
		}
		commands = append(commands, command)
	}
	return uniqueCommands(commands)
}

func evidencePathsForRisk(risk BaselineRisk, hazards []RemediationHazardClass, paths []string) []string {
	out := []string{risk.Path}
	out = append(out, paths...)
	for _, hazard := range hazards {
		for _, evidence := range hazard.Evidence {
			if strings.Contains(evidence, "/") || strings.Contains(evidence, ".") {
				out = append(out, evidence)
			}
		}
	}
	return capStrings(uniqueSortedStrings(out), 20)
}

func hazardClassNames(hazards []RemediationHazardClass) []string {
	var classes []string
	for _, hazard := range hazards {
		classes = append(classes, hazard.Class)
	}
	return uniqueSortedStrings(classes)
}

func hasHazardClass(hazards []RemediationHazardClass, class string) bool {
	for _, hazard := range hazards {
		if hazard.Class == class {
			return true
		}
	}
	return false
}

func intersectionClasses(classes []string, wanted ...string) []string {
	wantedSet := map[string]bool{}
	for _, class := range wanted {
		wantedSet[class] = true
	}
	var out []string
	for _, class := range classes {
		if wantedSet[class] {
			out = append(out, class)
		}
	}
	if len(out) == 0 {
		return classes
	}
	return out
}

func hazardEvidence(hazards []RemediationHazardClass, classes ...string) []string {
	classSet := map[string]bool{}
	for _, class := range classes {
		classSet[class] = true
	}
	var evidence []string
	for _, hazard := range hazards {
		if !classSet[hazard.Class] {
			continue
		}
		evidence = append(evidence, hazard.Evidence...)
		evidence = append(evidence, hazard.Mitigations...)
		if hazard.Rationale != "" {
			evidence = append(evidence, hazard.Rationale)
		}
	}
	return capStrings(uniqueSortedStrings(evidence), 8)
}

func summarizeRemediationPlaybook(baseline BaselineReport, playbooks []RemediationPlaybook) RemediationPlaybookSummary {
	classSet := map[string]bool{}
	summary := RemediationPlaybookSummary{Risks: len(baseline.Risks), Playbooks: len(playbooks)}
	for _, playbook := range playbooks {
		summary.RunbookSteps += len(playbook.RunbookSteps)
		summary.RollbackPoints += len(playbook.RollbackPoints)
		summary.OwnerHandoffs += len(playbook.OwnerHandoffs)
		summary.ValidationCommands += len(playbook.ValidationCommands)
		for _, hazard := range playbook.HazardClasses {
			classSet[hazard.Class] = true
		}
		for _, handoff := range playbook.OwnerHandoffs {
			switch handoff.Source {
			case "baseline.owner_routes":
				summary.CodeownersHandoffs++
			case "patchline.role-fallback":
				summary.FallbackRoleHandoffs++
			}
		}
	}
	summary.HazardClasses = len(classSet)
	return summary
}

func remediationPlaybookHash(report RemediationPlaybookReport) string {
	copy := report
	copy.Hash = ""
	copy.Markdown = ""
	return canonical.Hash(copy)
}

func severityRank(severity string) int {
	switch severity {
	case "critical":
		return 4
	case "high":
		return 3
	case "medium":
		return 2
	case "low":
		return 1
	default:
		return 0
	}
}
