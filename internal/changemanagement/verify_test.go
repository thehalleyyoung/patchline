package changemanagement

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildReportBindsPatchlineGatesToApprovals(t *testing.T) {
	root := t.TempDir()
	prepareChangeManagementEvidence(t, root)

	report, err := BuildReport(validChangeManagementSpec(t, root), root)
	if err != nil {
		t.Fatal(err)
	}
	if !report.OK || report.Summary.Workflows != 2 || report.Summary.PassedBlockingGates != 2 || report.Summary.ApprovedSteps != 4 {
		t.Fatalf("expected clean change-management report, got ok=%t summary=%#v counterexamples=%#v", report.OK, report.Summary, report.Counterexamples)
	}
	if report.Summary.EvidenceFiles < 6 || report.Summary.EmergencyWorkflows != 1 {
		t.Fatalf("expected real evidence and emergency workflow coverage, got %#v", report.Summary)
	}
	for _, workflow := range report.Workflows {
		for _, gate := range workflow.Gates {
			if gate.BlocksChange && !gate.HashMatches {
				t.Fatalf("expected blocking gate hash to match: %#v", gate)
			}
		}
	}
	markdown := RenderMarkdown(report)
	if !strings.Contains(markdown, "Change-management integration") || !strings.Contains(markdown, "bypassing") {
		t.Fatalf("expected useful markdown, got:\n%s", markdown)
	}
}

func TestBuildReportRejectsApprovalBypassingBlockingGate(t *testing.T) {
	root := t.TempDir()
	prepareChangeManagementEvidence(t, root)
	spec := validChangeManagementSpec(t, root)
	spec.Workflows[0].Approvals[0].GateIDs = []string{"missing-gate"}

	report, err := BuildReport(spec, root)
	if err != nil {
		t.Fatal(err)
	}
	if report.OK || !hasChangeCounterexample(report, "approval_references_unknown_gate") || !hasChangeCounterexample(report, "approval_without_passed_blocking_gate") {
		t.Fatalf("expected approval bypass counterexamples, got ok=%t counterexamples=%#v", report.OK, report.Counterexamples)
	}
}

func TestBuildReportRejectsFailedBlockingGateWithApproval(t *testing.T) {
	root := t.TempDir()
	prepareChangeManagementEvidence(t, root)
	spec := validChangeManagementSpec(t, root)
	spec.Workflows[0].Gates[0].Status = "fail"

	report, err := BuildReport(spec, root)
	if err != nil {
		t.Fatal(err)
	}
	if report.OK || !hasChangeCounterexample(report, "blocking_gate_not_passed") || !hasChangeCounterexample(report, "failed_blocking_gate_with_approval") {
		t.Fatalf("expected failed blocking gate counterexamples, got ok=%t counterexamples=%#v", report.OK, report.Counterexamples)
	}
}

func TestBuildReportRejectsHashMismatchAndEscapingPath(t *testing.T) {
	root := t.TempDir()
	prepareChangeManagementEvidence(t, root)
	spec := validChangeManagementSpec(t, root)
	spec.Workflows[0].Gates[0].ReportSHA256 = strings.Repeat("0", 64)
	spec.Workflows[0].Approvals[0].EvidencePath = "../approval.txt"

	report, err := BuildReport(spec, root)
	if err != nil {
		t.Fatal(err)
	}
	if report.OK || !hasChangeCounterexample(report, "gate_report_hash_mismatch") || !hasChangeCounterexample(report, "invalid_evidence_path") {
		t.Fatalf("expected hash and path counterexamples, got ok=%t counterexamples=%#v", report.OK, report.Counterexamples)
	}
}

func TestBuildReportRequiresDeterministicEmergencyExpiry(t *testing.T) {
	root := t.TempDir()
	prepareChangeManagementEvidence(t, root)
	spec := validChangeManagementSpec(t, root)
	spec.Workflows[1].DeploymentControls.EmergencyUntil = ""

	report, err := BuildReport(spec, root)
	if err != nil {
		t.Fatal(err)
	}
	if report.OK || !hasChangeCounterexample(report, "emergency_without_expiry") {
		t.Fatalf("expected emergency expiry counterexample, got ok=%t counterexamples=%#v", report.OK, report.Counterexamples)
	}
}

func TestReadSpecRejectsUnknownFields(t *testing.T) {
	_, err := ReadSpec(strings.NewReader(`{"version":"patchline.change-management/v1","name":"x","criteria":{"min_approval_steps":1},"workflows":[],"extra":true}`))
	if err == nil {
		t.Fatal("expected unknown field error")
	}
}

func validChangeManagementSpec(t *testing.T, root string) Spec {
	t.Helper()
	return Spec{
		Version: SpecVersion,
		Name:    "change-management integration fixture",
		Criteria: Criteria{
			MinApprovalSteps:             2,
			RequireDistinctApprovers:     true,
			RequireEvidenceHashes:        true,
			RequirePatchlineGateBinding:  true,
			RequireChangeTicket:          true,
			RequireRollbackPlan:          true,
			RequireEmergencyExpiry:       true,
			RequireWorkflowEvidencePaths: true,
		},
		Workflows: []Workflow{
			{
				WorkflowID:        "chg-2026-001-expand-contract",
				Title:             "Customers expand-contract migration",
				ChangeTicket:      "CHG-2026-001",
				RiskLevel:         "high",
				PatchlineFindings: []string{"PL-DB-001", "PL-DB-002"},
				Gates: []Gate{{
					GateID:       "patchline-reviewer-fairness",
					Command:      "make reviewer-fairness-audit-gate",
					Status:       "pass",
					ReportPath:   "evidence/gate-report.json",
					ReportSHA256: testFileHash(t, filepath.Join(root, "evidence/gate-report.json")),
					BlocksChange: true,
				}},
				Approvals: []Approval{
					approval("database-review", "database reliability", "robin-db", "evidence/cab-approval.txt", "patchline-reviewer-fairness"),
					approval("service-owner", "service owner", "sam-service", "evidence/service-approval.txt", "patchline-reviewer-fairness"),
				},
				DeploymentControls: DeploymentControls{ChangeWindow: "2026-01-15T22:00:00Z/2026-01-15T23:00:00Z", RollbackPlanPath: "evidence/rollback-plan.md"},
				EvidencePaths:      []string{"evidence/change-ticket.md"},
			},
			{
				WorkflowID:        "chg-2026-002-emergency-guard",
				Title:             "Emergency guard before customer repair",
				ChangeTicket:      "CHG-2026-002",
				RiskLevel:         "emergency",
				PatchlineFindings: []string{"PL-DB-009"},
				Gates: []Gate{{
					GateID:       "patchline-emergency-blocker",
					Command:      "go run ./cmd/patchline reviewer-fairness-audit --spec examples/reviewer-fairness-audit.json --root . --out results/generated/reviewer-fairness-audit --json",
					Status:       "pass",
					ReportPath:   "evidence/emergency-gate-report.json",
					ReportSHA256: testFileHash(t, filepath.Join(root, "evidence/emergency-gate-report.json")),
					BlocksChange: true,
				}},
				Approvals: []Approval{
					approval("incident-commander", "incident commander", "ivy-incident", "evidence/incident-commander-approval.txt", "patchline-emergency-blocker"),
					approval("security-owner", "security owner", "sky-security", "evidence/security-approval.txt", "patchline-emergency-blocker"),
				},
				DeploymentControls: DeploymentControls{ChangeWindow: "2026-01-16T10:00:00Z/2026-01-16T11:00:00Z", RollbackPlanPath: "evidence/emergency-rollback-plan.md", EmergencyUntil: "2026-01-16T18:00:00Z"},
				EvidencePaths:      []string{"evidence/emergency-ticket.md"},
			},
		},
	}
}

func approval(stepID, role, approver, evidencePath, gateID string) Approval {
	return Approval{
		StepID:       stepID,
		Role:         role,
		Approver:     approver,
		ApprovedAt:   "2026-01-15T13:00:00Z",
		Decision:     "approved",
		EvidencePath: evidencePath,
		GateIDs:      []string{gateID},
	}
}

func prepareChangeManagementEvidence(t *testing.T, root string) {
	t.Helper()
	files := map[string]string{
		"evidence/gate-report.json":                `{"version":"patchline.gate-report/v1","gate_id":"patchline-reviewer-fairness","status":"pass","finding_ids":["PL-DB-001","PL-DB-002"],"checked_at":"2026-01-15T12:00:00Z"}` + "\n",
		"evidence/emergency-gate-report.json":      `{"version":"patchline.gate-report/v1","gate_id":"patchline-emergency-blocker","status":"pass","finding_ids":["PL-DB-009"],"checked_at":"2026-01-16T09:30:00Z"}` + "\n",
		"evidence/cab-approval.txt":                "CAB approval: database reliability approved the passed Patchline blocking gate.\n",
		"evidence/service-approval.txt":            "Service owner approval: reviewed the same Patchline gate report and accepted rollback obligations.\n",
		"evidence/incident-commander-approval.txt": "Incident commander approval: emergency path is temporary and gate-bound.\n",
		"evidence/security-approval.txt":           "Security owner approval: emergency safety gate reviewed before rollout.\n",
		"evidence/rollback-plan.md":                "Rollback plan: restore the pre-migration snapshot and revert the expand-contract PR.\n",
		"evidence/emergency-rollback-plan.md":      "Emergency rollback plan: disable the guard, restore affected rows from snapshot, and page owners.\n",
		"evidence/change-ticket.md":                "CHG-2026-001 records the approval workflow and Patchline evidence bundle.\n",
		"evidence/emergency-ticket.md":             "CHG-2026-002 records the emergency expiry and manual approvals.\n",
	}
	for rel, contents := range files {
		writeChangeFile(t, root, rel, contents)
	}
}

func writeChangeFile(t *testing.T, root, relPath, contents string) {
	t.Helper()
	path := filepath.Join(root, relPath)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
}

func testFileHash(t *testing.T, path string) string {
	t.Helper()
	bytes, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(bytes)
	return hex.EncodeToString(sum[:])
}

func hasChangeCounterexample(report Report, kind string) bool {
	for _, counterexample := range report.Counterexamples {
		if counterexample.Kind == kind {
			return true
		}
	}
	return false
}
