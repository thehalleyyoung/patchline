package changemanagement

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/thehalleyyoung/patchline/internal/canonical"
)

const SpecVersion = "patchline.change-management/v1"
const ReportVersion = "patchline.change-management-report/v1"

type Spec struct {
	Version   string     `json:"version"`
	Name      string     `json:"name"`
	Criteria  Criteria   `json:"criteria"`
	Workflows []Workflow `json:"workflows"`
}

type Criteria struct {
	MinApprovalSteps             int  `json:"min_approval_steps"`
	RequireDistinctApprovers     bool `json:"require_distinct_approvers"`
	RequireEvidenceHashes        bool `json:"require_evidence_hashes"`
	RequirePatchlineGateBinding  bool `json:"require_patchline_gate_binding"`
	RequireChangeTicket          bool `json:"require_change_ticket"`
	RequireRollbackPlan          bool `json:"require_rollback_plan"`
	RequireEmergencyExpiry       bool `json:"require_emergency_expiry"`
	RequireWorkflowEvidencePaths bool `json:"require_workflow_evidence_paths"`
}

type Workflow struct {
	WorkflowID         string             `json:"workflow_id"`
	Title              string             `json:"title"`
	ChangeTicket       string             `json:"change_ticket"`
	RiskLevel          string             `json:"risk_level"`
	PatchlineFindings  []string           `json:"patchline_findings"`
	Gates              []Gate             `json:"gates"`
	Approvals          []Approval         `json:"approvals"`
	DeploymentControls DeploymentControls `json:"deployment_controls"`
	EvidencePaths      []string           `json:"evidence_paths"`
}

type Gate struct {
	GateID       string `json:"gate_id"`
	Command      string `json:"command"`
	Status       string `json:"status"`
	ReportPath   string `json:"report_path"`
	ReportSHA256 string `json:"report_sha256"`
	BlocksChange bool   `json:"blocks_change"`
}

type Approval struct {
	StepID       string   `json:"step_id"`
	Role         string   `json:"role"`
	Approver     string   `json:"approver"`
	ApprovedAt   string   `json:"approved_at"`
	Decision     string   `json:"decision"`
	EvidencePath string   `json:"evidence_path"`
	GateIDs      []string `json:"gate_ids"`
}

type DeploymentControls struct {
	ChangeWindow     string `json:"change_window"`
	RollbackPlanPath string `json:"rollback_plan_path"`
	EmergencyUntil   string `json:"emergency_until,omitempty"`
}

type Report struct {
	Version         string           `json:"version"`
	Name            string           `json:"name"`
	OK              bool             `json:"ok"`
	Criteria        Criteria         `json:"criteria"`
	Summary         Summary          `json:"summary"`
	Workflows       []WorkflowReport `json:"workflows"`
	Counterexamples []Counterexample `json:"counterexamples,omitempty"`
	Hash            string           `json:"hash"`
}

type Summary struct {
	Workflows            int `json:"workflows"`
	ChangeTickets        int `json:"change_tickets"`
	Gates                int `json:"gates"`
	BlockingGates        int `json:"blocking_gates"`
	PassedBlockingGates  int `json:"passed_blocking_gates"`
	Approvals            int `json:"approvals"`
	ApprovedSteps        int `json:"approved_steps"`
	DistinctApprovers    int `json:"distinct_approvers"`
	EvidenceFiles        int `json:"evidence_files"`
	PatchlineFindings    int `json:"patchline_findings"`
	EmergencyWorkflows   int `json:"emergency_workflows"`
	ExpiredEmergencies   int `json:"expired_emergencies"`
	UnverifiedGateHashes int `json:"unverified_gate_hashes"`
	Counterexamples      int `json:"counterexamples"`
}

type WorkflowReport struct {
	WorkflowID         string                   `json:"workflow_id"`
	Title              string                   `json:"title"`
	ChangeTicket       string                   `json:"change_ticket"`
	RiskLevel          string                   `json:"risk_level"`
	PatchlineFindings  []string                 `json:"patchline_findings"`
	Gates              []GateReport             `json:"gates"`
	Approvals          []ApprovalReport         `json:"approvals"`
	DeploymentControls DeploymentControlsReport `json:"deployment_controls"`
	Evidence           []ArtifactEvidence       `json:"evidence"`
}

type GateReport struct {
	GateID       string            `json:"gate_id"`
	Command      string            `json:"command"`
	Status       string            `json:"status"`
	BlocksChange bool              `json:"blocks_change"`
	ExpectedHash string            `json:"expected_hash,omitempty"`
	Report       *ArtifactEvidence `json:"report,omitempty"`
	HashMatches  bool              `json:"hash_matches"`
}

type ApprovalReport struct {
	StepID     string            `json:"step_id"`
	Role       string            `json:"role"`
	Approver   string            `json:"approver"`
	ApprovedAt string            `json:"approved_at"`
	Decision   string            `json:"decision"`
	GateIDs    []string          `json:"gate_ids"`
	Evidence   *ArtifactEvidence `json:"evidence,omitempty"`
}

type DeploymentControlsReport struct {
	ChangeWindow   string            `json:"change_window"`
	EmergencyUntil string            `json:"emergency_until,omitempty"`
	RollbackPlan   *ArtifactEvidence `json:"rollback_plan,omitempty"`
}

type ArtifactEvidence struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
	Bytes  int64  `json:"bytes"`
}

type Counterexample struct {
	ID      string   `json:"id"`
	Kind    string   `json:"kind"`
	Subject string   `json:"subject,omitempty"`
	Message string   `json:"message"`
	Witness []string `json:"witness,omitempty"`
}

func ReadSpec(reader io.Reader) (Spec, error) {
	decoder := json.NewDecoder(reader)
	decoder.DisallowUnknownFields()
	var spec Spec
	if err := decoder.Decode(&spec); err != nil {
		return Spec{}, err
	}
	if spec.Version != SpecVersion {
		return Spec{}, fmt.Errorf("change-management spec version must be %s", SpecVersion)
	}
	return spec, nil
}

func BuildReport(spec Spec, root string) (Report, error) {
	if err := validateSpec(spec); err != nil {
		return Report{}, err
	}
	if root == "" {
		root = "."
	}
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return Report{}, err
	}

	report := Report{
		Version:  ReportVersion,
		Name:     spec.Name,
		OK:       true,
		Criteria: spec.Criteria,
	}
	evidenceSeen := map[string]ArtifactEvidence{}
	approverSeen := map[string]bool{}
	changeTickets := map[string]bool{}

	for _, workflow := range sortedWorkflows(spec.Workflows) {
		workflowReport, counterexamples := evaluateWorkflow(spec.Criteria, workflow, rootAbs, evidenceSeen, approverSeen)
		report.Workflows = append(report.Workflows, workflowReport)
		report.Counterexamples = append(report.Counterexamples, counterexamples...)
		report.Summary.Workflows++
		if strings.TrimSpace(workflow.ChangeTicket) != "" {
			changeTickets[workflow.ChangeTicket] = true
		}
		report.Summary.Gates += len(workflow.Gates)
		report.Summary.Approvals += len(workflow.Approvals)
		report.Summary.PatchlineFindings += len(workflow.PatchlineFindings)
		if isEmergency(workflow) {
			report.Summary.EmergencyWorkflows++
			if strings.TrimSpace(workflow.DeploymentControls.EmergencyUntil) == "" {
				report.Summary.ExpiredEmergencies++
			}
		}
		for _, gate := range workflow.Gates {
			if gate.BlocksChange {
				report.Summary.BlockingGates++
				if gate.Status == "pass" {
					report.Summary.PassedBlockingGates++
				}
			}
			if spec.Criteria.RequireEvidenceHashes && normalizeSHA256(gate.ReportSHA256) == "" {
				report.Summary.UnverifiedGateHashes++
			}
		}
		for _, approval := range workflow.Approvals {
			if approval.Decision == "approved" {
				report.Summary.ApprovedSteps++
			}
		}
	}

	report.Summary.ChangeTickets = len(changeTickets)
	report.Summary.DistinctApprovers = len(approverSeen)
	report.Summary.EvidenceFiles = len(evidenceSeen)
	sortCounterexamples(report.Counterexamples)
	report.Summary.Counterexamples = len(report.Counterexamples)
	report.OK = len(report.Counterexamples) == 0
	report.Hash = reportHash(report)
	return report, nil
}

func WriteArtifacts(outDir string, report Report) error {
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}
	jsonFile, err := os.Create(filepath.Join(outDir, "change-management.json"))
	if err != nil {
		return err
	}
	if err := canonical.WriteJSON(jsonFile, report); err != nil {
		_ = jsonFile.Close()
		return err
	}
	if err := jsonFile.Close(); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(outDir, "change-management.md"), []byte(RenderMarkdown(report)), 0o644)
}

func RenderMarkdown(report Report) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Change-management integration\n\n")
	fmt.Fprintf(&b, "Patchline verifies that blocking safety gates are bound to organizational approval steps, so automation supplies evidence for change control instead of bypassing it.\n\n")
	fmt.Fprintf(&b, "## Summary\n\n")
	fmt.Fprintf(&b, "| Metric | Value |\n| --- | ---: |\n")
	fmt.Fprintf(&b, "| OK | `%t` |\n", report.OK)
	fmt.Fprintf(&b, "| Workflows | %d |\n", report.Summary.Workflows)
	fmt.Fprintf(&b, "| Change tickets | %d |\n", report.Summary.ChangeTickets)
	fmt.Fprintf(&b, "| Gates | %d |\n", report.Summary.Gates)
	fmt.Fprintf(&b, "| Blocking gates | %d |\n", report.Summary.BlockingGates)
	fmt.Fprintf(&b, "| Passed blocking gates | %d |\n", report.Summary.PassedBlockingGates)
	fmt.Fprintf(&b, "| Approved steps | %d |\n", report.Summary.ApprovedSteps)
	fmt.Fprintf(&b, "| Distinct approvers | %d |\n", report.Summary.DistinctApprovers)
	fmt.Fprintf(&b, "| Evidence files | %d |\n", report.Summary.EvidenceFiles)
	fmt.Fprintf(&b, "| Counterexamples | %d |\n\n", report.Summary.Counterexamples)

	fmt.Fprintf(&b, "Policy: at least `%d` approved step(s), distinct approvers `%t`, evidence hashes `%t`, Patchline gate binding `%t`, change tickets `%t`, rollback plans `%t`, emergency expiry `%t`.\n\n", report.Criteria.MinApprovalSteps, report.Criteria.RequireDistinctApprovers, report.Criteria.RequireEvidenceHashes, report.Criteria.RequirePatchlineGateBinding, report.Criteria.RequireChangeTicket, report.Criteria.RequireRollbackPlan, report.Criteria.RequireEmergencyExpiry)
	fmt.Fprintf(&b, "## Workflow bindings\n\n")
	fmt.Fprintf(&b, "| Workflow | Ticket | Risk | Blocking gates passed | Approved steps | Evidence |\n| --- | --- | --- | ---: | ---: | ---: |\n")
	for _, workflow := range report.Workflows {
		passed := 0
		blocking := 0
		approved := 0
		for _, gate := range workflow.Gates {
			if gate.BlocksChange {
				blocking++
				if gate.Status == "pass" {
					passed++
				}
			}
		}
		for _, approval := range workflow.Approvals {
			if approval.Decision == "approved" {
				approved++
			}
		}
		fmt.Fprintf(&b, "| `%s` | `%s` | `%s` | %d/%d | %d | %d |\n", workflow.WorkflowID, workflow.ChangeTicket, workflow.RiskLevel, passed, blocking, approved, len(workflow.Evidence))
	}
	if len(report.Counterexamples) > 0 {
		fmt.Fprintf(&b, "\n## Counterexamples\n\n")
		fmt.Fprintf(&b, "| ID | Kind | Subject | Message |\n| --- | --- | --- | --- |\n")
		for _, counterexample := range report.Counterexamples {
			fmt.Fprintf(&b, "| `%s` | `%s` | `%s` | %s |\n", counterexample.ID, counterexample.Kind, firstNonEmpty(counterexample.Subject, "-"), counterexample.Message)
		}
	}
	return b.String()
}

func evaluateWorkflow(criteria Criteria, workflow Workflow, rootAbs string, evidenceSeen map[string]ArtifactEvidence, approverSeen map[string]bool) (WorkflowReport, []Counterexample) {
	report := WorkflowReport{
		WorkflowID:        workflow.WorkflowID,
		Title:             workflow.Title,
		ChangeTicket:      workflow.ChangeTicket,
		RiskLevel:         workflow.RiskLevel,
		PatchlineFindings: sortedStrings(workflow.PatchlineFindings),
		DeploymentControls: DeploymentControlsReport{
			ChangeWindow:   workflow.DeploymentControls.ChangeWindow,
			EmergencyUntil: workflow.DeploymentControls.EmergencyUntil,
		},
	}
	var counterexamples []Counterexample
	subject := workflow.WorkflowID

	if criteria.RequireChangeTicket && strings.TrimSpace(workflow.ChangeTicket) == "" {
		counterexamples = append(counterexamples, Counterexample{
			ID:      fmt.Sprintf("missing-change-ticket-%s", safeID(subject)),
			Kind:    "missing_change_ticket",
			Subject: subject,
			Message: "workflow must name the organizational change ticket that owns the Patchline gate decision",
		})
	}
	if criteria.RequireWorkflowEvidencePaths && len(workflow.EvidencePaths) == 0 {
		counterexamples = append(counterexamples, Counterexample{
			ID:      fmt.Sprintf("missing-workflow-evidence-%s", safeID(subject)),
			Kind:    "missing_workflow_evidence",
			Subject: subject,
			Message: "workflow must preserve independent change-management evidence paths",
		})
	}
	for _, relPath := range sortedStrings(workflow.EvidencePaths) {
		evidence, fileCounterexamples := resolveFileUnderRoot(rootAbs, relPath, subject, "workflow_evidence")
		counterexamples = append(counterexamples, fileCounterexamples...)
		if evidence != nil {
			report.Evidence = append(report.Evidence, *evidence)
			evidenceSeen[evidence.Path] = *evidence
		}
	}

	if criteria.RequireRollbackPlan && strings.TrimSpace(workflow.DeploymentControls.RollbackPlanPath) == "" {
		counterexamples = append(counterexamples, Counterexample{
			ID:      fmt.Sprintf("missing-rollback-plan-%s", safeID(subject)),
			Kind:    "missing_rollback_plan",
			Subject: subject,
			Message: "workflow must preserve a rollback plan before Patchline gates can bless the change",
		})
	} else if strings.TrimSpace(workflow.DeploymentControls.RollbackPlanPath) != "" {
		evidence, fileCounterexamples := resolveFileUnderRoot(rootAbs, workflow.DeploymentControls.RollbackPlanPath, subject, "rollback_plan")
		counterexamples = append(counterexamples, fileCounterexamples...)
		if evidence != nil {
			report.DeploymentControls.RollbackPlan = evidence
			evidenceSeen[evidence.Path] = *evidence
		}
	}
	if criteria.RequireEmergencyExpiry && isEmergency(workflow) {
		if strings.TrimSpace(workflow.DeploymentControls.EmergencyUntil) == "" {
			counterexamples = append(counterexamples, Counterexample{
				ID:      fmt.Sprintf("emergency-without-expiry-%s", safeID(subject)),
				Kind:    "emergency_without_expiry",
				Subject: subject,
				Message: "emergency change approvals must carry an explicit RFC3339 expiry instead of staying open-ended",
			})
		} else if _, err := time.Parse(time.RFC3339, workflow.DeploymentControls.EmergencyUntil); err != nil {
			counterexamples = append(counterexamples, Counterexample{
				ID:      fmt.Sprintf("invalid-emergency-expiry-%s", safeID(subject)),
				Kind:    "invalid_emergency_expiry",
				Subject: subject,
				Message: "emergency_until must be RFC3339 so reviewers can reproduce expiry checks deterministically",
				Witness: []string{workflow.DeploymentControls.EmergencyUntil},
			})
		}
	}

	gatesByID := map[string]GateReport{}
	for _, gate := range sortedGates(workflow.Gates) {
		gateReport, gateCounterexamples := evaluateGate(criteria, workflow.WorkflowID, gate, rootAbs, evidenceSeen)
		report.Gates = append(report.Gates, gateReport)
		counterexamples = append(counterexamples, gateCounterexamples...)
		gatesByID[gate.GateID] = gateReport
	}

	approvedApprovers := map[string]bool{}
	approvedSteps := 0
	for _, approval := range sortedApprovals(workflow.Approvals) {
		approvalReport, approvalCounterexamples := evaluateApproval(criteria, workflow.WorkflowID, approval, gatesByID, rootAbs, evidenceSeen)
		report.Approvals = append(report.Approvals, approvalReport)
		counterexamples = append(counterexamples, approvalCounterexamples...)
		if approval.Decision == "approved" {
			approvedSteps++
			approvedApprovers[approval.Approver] = true
			approverSeen[approval.Approver] = true
		}
	}
	if approvedSteps < criteria.MinApprovalSteps {
		counterexamples = append(counterexamples, Counterexample{
			ID:      fmt.Sprintf("insufficient-approvals-%s", safeID(subject)),
			Kind:    "insufficient_approvals",
			Subject: subject,
			Message: fmt.Sprintf("workflow has %d approved steps, below required %d", approvedSteps, criteria.MinApprovalSteps),
		})
	}
	if criteria.RequireDistinctApprovers && len(approvedApprovers) < approvedSteps {
		counterexamples = append(counterexamples, Counterexample{
			ID:      fmt.Sprintf("duplicate-approver-%s", safeID(subject)),
			Kind:    "duplicate_approver",
			Subject: subject,
			Message: "approved steps must come from distinct human approvers",
		})
	}
	return report, counterexamples
}

func evaluateGate(criteria Criteria, workflowID string, gate Gate, rootAbs string, evidenceSeen map[string]ArtifactEvidence) (GateReport, []Counterexample) {
	report := GateReport{
		GateID:       gate.GateID,
		Command:      gate.Command,
		Status:       gate.Status,
		BlocksChange: gate.BlocksChange,
		ExpectedHash: normalizeSHA256(gate.ReportSHA256),
	}
	var counterexamples []Counterexample
	subject := workflowID + ":" + gate.GateID

	if criteria.RequirePatchlineGateBinding && !isPatchlineGateCommand(gate.Command) {
		counterexamples = append(counterexamples, Counterexample{
			ID:      fmt.Sprintf("gate-not-patchline-bound-%s", safeID(subject)),
			Kind:    "gate_not_patchline_bound",
			Subject: subject,
			Message: "blocking approval evidence must come from a Patchline command or a gate-backed make target",
			Witness: []string{gate.Command},
		})
	}
	if gate.BlocksChange && gate.Status != "pass" {
		counterexamples = append(counterexamples, Counterexample{
			ID:      fmt.Sprintf("blocking-gate-not-passed-%s", safeID(subject)),
			Kind:    "blocking_gate_not_passed",
			Subject: subject,
			Message: "a gate that blocks the organizational change must pass before approvals can clear the change",
			Witness: []string{gate.Status},
		})
	}
	if strings.TrimSpace(gate.ReportPath) == "" {
		if criteria.RequirePatchlineGateBinding || criteria.RequireEvidenceHashes {
			counterexamples = append(counterexamples, Counterexample{
				ID:      fmt.Sprintf("missing-gate-report-%s", safeID(subject)),
				Kind:    "missing_gate_report",
				Subject: subject,
				Message: "gate must preserve the report artifact reviewed by the approval workflow",
			})
		}
		return report, counterexamples
	}
	evidence, fileCounterexamples := resolveFileUnderRoot(rootAbs, gate.ReportPath, subject, "gate_report")
	counterexamples = append(counterexamples, fileCounterexamples...)
	if evidence == nil {
		return report, counterexamples
	}
	report.Report = evidence
	evidenceSeen[evidence.Path] = *evidence
	if criteria.RequireEvidenceHashes && report.ExpectedHash == "" {
		counterexamples = append(counterexamples, Counterexample{
			ID:      fmt.Sprintf("missing-gate-report-hash-%s", safeID(subject)),
			Kind:    "missing_gate_report_hash",
			Subject: subject,
			Message: "gate report hash must be pinned so approval evidence cannot drift",
			Witness: []string{gate.ReportPath},
		})
		return report, counterexamples
	}
	if report.ExpectedHash != "" {
		report.HashMatches = strings.EqualFold(report.ExpectedHash, evidence.SHA256)
		if !report.HashMatches {
			counterexamples = append(counterexamples, Counterexample{
				ID:      fmt.Sprintf("gate-report-hash-mismatch-%s", safeID(subject)),
				Kind:    "gate_report_hash_mismatch",
				Subject: subject,
				Message: "gate report hash does not match the artifact bound into the approval workflow",
				Witness: []string{gate.ReportPath, evidence.SHA256, report.ExpectedHash},
			})
		}
	}
	return report, counterexamples
}

func evaluateApproval(criteria Criteria, workflowID string, approval Approval, gatesByID map[string]GateReport, rootAbs string, evidenceSeen map[string]ArtifactEvidence) (ApprovalReport, []Counterexample) {
	report := ApprovalReport{
		StepID:     approval.StepID,
		Role:       approval.Role,
		Approver:   approval.Approver,
		ApprovedAt: approval.ApprovedAt,
		Decision:   approval.Decision,
		GateIDs:    sortedStrings(approval.GateIDs),
	}
	var counterexamples []Counterexample
	subject := workflowID + ":" + approval.StepID

	if strings.TrimSpace(approval.EvidencePath) == "" {
		counterexamples = append(counterexamples, Counterexample{
			ID:      fmt.Sprintf("missing-approval-evidence-%s", safeID(subject)),
			Kind:    "missing_approval_evidence",
			Subject: subject,
			Message: "approval step must preserve the human review artifact",
		})
	} else {
		evidence, fileCounterexamples := resolveFileUnderRoot(rootAbs, approval.EvidencePath, subject, "approval_evidence")
		counterexamples = append(counterexamples, fileCounterexamples...)
		if evidence != nil {
			report.Evidence = evidence
			evidenceSeen[evidence.Path] = *evidence
		}
	}

	if approval.Decision != "approved" {
		return report, counterexamples
	}
	if criteria.RequirePatchlineGateBinding && len(approval.GateIDs) == 0 {
		counterexamples = append(counterexamples, Counterexample{
			ID:      fmt.Sprintf("approval-without-gate-binding-%s", safeID(subject)),
			Kind:    "approval_without_gate_binding",
			Subject: subject,
			Message: "approved change-management step must cite the Patchline gate IDs it reviewed",
		})
	}
	linkedPassedBlocking := false
	for _, gateID := range sortedStrings(approval.GateIDs) {
		gate, ok := gatesByID[gateID]
		if !ok {
			counterexamples = append(counterexamples, Counterexample{
				ID:      fmt.Sprintf("approval-references-unknown-gate-%s-%s", safeID(subject), safeID(gateID)),
				Kind:    "approval_references_unknown_gate",
				Subject: subject,
				Message: "approval step references a gate ID that is not in the workflow",
				Witness: []string{gateID},
			})
			continue
		}
		if gate.BlocksChange && gate.Status == "pass" {
			linkedPassedBlocking = true
		}
		if gate.BlocksChange && gate.Status != "pass" {
			counterexamples = append(counterexamples, Counterexample{
				ID:      fmt.Sprintf("failed-blocking-gate-with-approval-%s-%s", safeID(subject), safeID(gateID)),
				Kind:    "failed_blocking_gate_with_approval",
				Subject: subject,
				Message: "approval step cleared a change even though a linked blocking Patchline gate did not pass",
				Witness: []string{gateID, gate.Status},
			})
		}
	}
	if criteria.RequirePatchlineGateBinding && !linkedPassedBlocking {
		counterexamples = append(counterexamples, Counterexample{
			ID:      fmt.Sprintf("approval-without-passed-blocking-gate-%s", safeID(subject)),
			Kind:    "approval_without_passed_blocking_gate",
			Subject: subject,
			Message: "approval step must bind to at least one passed blocking Patchline gate",
			Witness: sortedStrings(approval.GateIDs),
		})
	}
	return report, counterexamples
}

func validateSpec(spec Spec) error {
	if spec.Version != SpecVersion {
		return fmt.Errorf("change-management spec version must be %s", SpecVersion)
	}
	if strings.TrimSpace(spec.Name) == "" {
		return fmt.Errorf("change-management spec name is required")
	}
	if spec.Criteria.MinApprovalSteps < 1 {
		return fmt.Errorf("criteria.min_approval_steps must be at least 1")
	}
	if len(spec.Workflows) == 0 {
		return fmt.Errorf("at least one change-management workflow is required")
	}
	seenWorkflows := map[string]bool{}
	for _, workflow := range spec.Workflows {
		if strings.TrimSpace(workflow.WorkflowID) == "" {
			return fmt.Errorf("workflow_id is required")
		}
		if seenWorkflows[workflow.WorkflowID] {
			return fmt.Errorf("duplicate workflow_id %q", workflow.WorkflowID)
		}
		seenWorkflows[workflow.WorkflowID] = true
		if strings.TrimSpace(workflow.Title) == "" || strings.TrimSpace(workflow.RiskLevel) == "" {
			return fmt.Errorf("workflow %q must include title and risk_level", workflow.WorkflowID)
		}
		if len(workflow.Gates) == 0 {
			return fmt.Errorf("workflow %q must include at least one gate", workflow.WorkflowID)
		}
		if len(workflow.Approvals) == 0 {
			return fmt.Errorf("workflow %q must include at least one approval", workflow.WorkflowID)
		}
		if err := validateGates(workflow); err != nil {
			return err
		}
		if err := validateApprovals(workflow); err != nil {
			return err
		}
	}
	return nil
}

func validateGates(workflow Workflow) error {
	seen := map[string]bool{}
	for _, gate := range workflow.Gates {
		if strings.TrimSpace(gate.GateID) == "" {
			return fmt.Errorf("workflow %q gate_id is required", workflow.WorkflowID)
		}
		if seen[gate.GateID] {
			return fmt.Errorf("workflow %q has duplicate gate_id %q", workflow.WorkflowID, gate.GateID)
		}
		seen[gate.GateID] = true
		if strings.TrimSpace(gate.Command) == "" {
			return fmt.Errorf("workflow %q gate %q command is required", workflow.WorkflowID, gate.GateID)
		}
		switch gate.Status {
		case "pass", "fail", "not_run":
		default:
			return fmt.Errorf("workflow %q gate %q status must be pass, fail, or not_run", workflow.WorkflowID, gate.GateID)
		}
	}
	return nil
}

func validateApprovals(workflow Workflow) error {
	seen := map[string]bool{}
	for _, approval := range workflow.Approvals {
		if strings.TrimSpace(approval.StepID) == "" {
			return fmt.Errorf("workflow %q approval step_id is required", workflow.WorkflowID)
		}
		if seen[approval.StepID] {
			return fmt.Errorf("workflow %q has duplicate approval step_id %q", workflow.WorkflowID, approval.StepID)
		}
		seen[approval.StepID] = true
		if strings.TrimSpace(approval.Role) == "" || strings.TrimSpace(approval.Approver) == "" {
			return fmt.Errorf("workflow %q approval %q must include role and approver", workflow.WorkflowID, approval.StepID)
		}
		switch approval.Decision {
		case "approved", "rejected", "deferred":
		default:
			return fmt.Errorf("workflow %q approval %q decision must be approved, rejected, or deferred", workflow.WorkflowID, approval.StepID)
		}
		if strings.TrimSpace(approval.ApprovedAt) == "" {
			return fmt.Errorf("workflow %q approval %q approved_at is required", workflow.WorkflowID, approval.StepID)
		}
		if _, err := time.Parse(time.RFC3339, approval.ApprovedAt); err != nil {
			return fmt.Errorf("workflow %q approval %q approved_at must be RFC3339: %w", workflow.WorkflowID, approval.StepID, err)
		}
	}
	return nil
}

func resolveFileUnderRoot(rootAbs, relPath, subject, kind string) (*ArtifactEvidence, []Counterexample) {
	clean := filepath.Clean(strings.TrimSpace(relPath))
	if clean == "" || clean == "." || filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return nil, []Counterexample{{
			ID:      fmt.Sprintf("%s-path-%s-%s", strings.ReplaceAll(kind, "_", "-"), safeID(subject), safeID(relPath)),
			Kind:    "invalid_evidence_path",
			Subject: subject,
			Message: fmt.Sprintf("%s path %q must be a relative file below the audit root", strings.ReplaceAll(kind, "_", " "), relPath),
			Witness: []string{relPath},
		}}
	}
	artifactPath := filepath.Join(rootAbs, clean)
	info, err := os.Lstat(artifactPath)
	if err != nil {
		return nil, []Counterexample{{
			ID:      fmt.Sprintf("missing-%s-%s-%s", strings.ReplaceAll(kind, "_", "-"), safeID(subject), safeID(clean)),
			Kind:    "missing_evidence",
			Subject: subject,
			Message: fmt.Sprintf("%s file %q is missing", strings.ReplaceAll(kind, "_", " "), clean),
			Witness: []string{clean},
		}}
	}
	if !info.Mode().IsRegular() {
		return nil, []Counterexample{{
			ID:      fmt.Sprintf("invalid-%s-file-%s-%s", strings.ReplaceAll(kind, "_", "-"), safeID(subject), safeID(clean)),
			Kind:    "invalid_evidence_file",
			Subject: subject,
			Message: fmt.Sprintf("%s file %q must be a regular file under the audit root", strings.ReplaceAll(kind, "_", " "), clean),
			Witness: []string{clean},
		}}
	}
	bytes, err := os.ReadFile(artifactPath)
	if err != nil {
		return nil, []Counterexample{{
			ID:      fmt.Sprintf("unreadable-%s-%s-%s", strings.ReplaceAll(kind, "_", "-"), safeID(subject), safeID(clean)),
			Kind:    "unreadable_evidence",
			Subject: subject,
			Message: fmt.Sprintf("%s file %q could not be read: %v", strings.ReplaceAll(kind, "_", " "), clean, err),
			Witness: []string{clean},
		}}
	}
	sum := sha256.Sum256(bytes)
	return &ArtifactEvidence{Path: clean, SHA256: hex.EncodeToString(sum[:]), Bytes: info.Size()}, nil
}

func isPatchlineGateCommand(command string) bool {
	lower := strings.ToLower(command)
	return strings.Contains(lower, "patchline") || (strings.HasPrefix(lower, "make ") && strings.Contains(lower, "-gate"))
}

func isEmergency(workflow Workflow) bool {
	return strings.EqualFold(workflow.RiskLevel, "emergency") || strings.TrimSpace(workflow.DeploymentControls.EmergencyUntil) != ""
}

func normalizeSHA256(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	return strings.TrimPrefix(value, "sha256:")
}

func reportHash(report Report) string {
	copyReport := report
	copyReport.Hash = ""
	return canonical.Hash(copyReport)
}

func sortedWorkflows(workflows []Workflow) []Workflow {
	sorted := append([]Workflow(nil), workflows...)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].WorkflowID < sorted[j].WorkflowID
	})
	return sorted
}

func sortedGates(gates []Gate) []Gate {
	sorted := append([]Gate(nil), gates...)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].GateID < sorted[j].GateID
	})
	return sorted
}

func sortedApprovals(approvals []Approval) []Approval {
	sorted := append([]Approval(nil), approvals...)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].StepID < sorted[j].StepID
	})
	return sorted
}

func sortedStrings(values []string) []string {
	sorted := append([]string(nil), values...)
	sort.Strings(sorted)
	return sorted
}

func sortCounterexamples(counterexamples []Counterexample) {
	sort.Slice(counterexamples, func(i, j int) bool {
		if counterexamples[i].Kind != counterexamples[j].Kind {
			return counterexamples[i].Kind < counterexamples[j].Kind
		}
		if counterexamples[i].Subject != counterexamples[j].Subject {
			return counterexamples[i].Subject < counterexamples[j].Subject
		}
		return counterexamples[i].ID < counterexamples[j].ID
	})
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func safeID(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var b strings.Builder
	lastDash := false
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash {
			b.WriteByte('-')
			lastDash = true
		}
	}
	return strings.Trim(b.String(), "-")
}
