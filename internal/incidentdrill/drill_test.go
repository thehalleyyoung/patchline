package incidentdrill

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildReportValidatesIncidentResponseTimeline(t *testing.T) {
	root := t.TempDir()
	gateHash := prepareIncidentDrillEvidence(t, root)
	spec := validIncidentDrillSpec(gateHash)

	report, err := BuildReport(spec, root)
	if err != nil {
		t.Fatal(err)
	}
	if !report.OK || report.Summary.Counterexamples != 0 {
		t.Fatalf("expected clean incident drill, got ok=%t counterexamples=%#v", report.OK, report.Counterexamples)
	}
	if report.Summary.DetectionMinutes != 30 || report.Summary.PublicDisclosureHours != 2 || report.Summary.MitigationHours != 4 || report.Summary.RemediationHours != 12 {
		t.Fatalf("unexpected timeline summary: %#v", report.Summary)
	}
	if report.Summary.RegressionGates != 1 || report.Summary.DistinctRoles != 4 || report.Summary.DistinctOwners != 4 || report.Summary.EvidenceFiles < 10 {
		t.Fatalf("unexpected drill coverage: %#v", report.Summary)
	}
	if !hasMatchingRegressionGate(report) {
		t.Fatalf("expected regression gate hash to match: %#v", report.Drill.Remediations)
	}
	second, err := BuildReport(spec, root)
	if err != nil {
		t.Fatal(err)
	}
	if report.Hash == "" || report.Hash != second.Hash {
		t.Fatalf("expected deterministic hash, first=%s second=%s", report.Hash, second.Hash)
	}
	markdown := RenderMarkdown(report)
	if !strings.Contains(markdown, "Incident-response drill") || !strings.Contains(markdown, "false negative") {
		t.Fatalf("expected useful markdown, got:\n%s", markdown)
	}
}

func TestBuildReportRejectsDelayedDisclosureAndRemediation(t *testing.T) {
	root := t.TempDir()
	gateHash := prepareIncidentDrillEvidence(t, root)
	spec := validIncidentDrillSpec(gateHash)
	spec.Drill.Disclosures[0].PublishedAt = "2026-02-03T03:00:00Z"
	spec.Drill.Remediations[0].CompletedAt = "2026-02-05T12:00:00Z"

	report, err := BuildReport(spec, root)
	if err != nil {
		t.Fatal(err)
	}
	if report.OK || !hasDrillCounterexample(report, "public_disclosure_deadline_exceeded") || !hasDrillCounterexample(report, "remediation_deadline_exceeded") || !hasDrillCounterexample(report, "remediation_completed_after_due") {
		t.Fatalf("expected delayed disclosure and remediation counterexamples, got %#v", report.Counterexamples)
	}
}

func TestBuildReportRejectsGateHashMismatchAndPrivateMarker(t *testing.T) {
	root := t.TempDir()
	gateHash := prepareIncidentDrillEvidence(t, root)
	spec := validIncidentDrillSpec(gateHash)
	spec.Drill.Remediations[0].GateReportSHA256 = strings.Repeat("0", 64)
	spec.Drill.Disclosures[0].Summary = "Public update accidentally included Token=not-public."

	report, err := BuildReport(spec, root)
	if err != nil {
		t.Fatal(err)
	}
	if report.OK || !hasDrillCounterexample(report, "remediation_gate_hash_mismatch") || !hasDrillCounterexample(report, "public_summary_private_marker") {
		t.Fatalf("expected hash mismatch and private marker counterexamples, got %#v", report.Counterexamples)
	}
}

func TestBuildReportRejectsReversedTimelineEndpoints(t *testing.T) {
	root := t.TempDir()
	gateHash := prepareIncidentDrillEvidence(t, root)
	spec := validIncidentDrillSpec(gateHash)
	for i := range spec.Drill.Timeline {
		if spec.Drill.Timeline[i].Phase == "detected" {
			spec.Drill.Timeline[i].At = "2026-02-01T11:30:00Z"
		}
	}

	report, err := BuildReport(spec, root)
	if err != nil {
		t.Fatal(err)
	}
	if report.OK || !hasDrillCounterexample(report, "timeline_order_violation") {
		t.Fatalf("expected reversed timeline counterexample, got %#v", report.Counterexamples)
	}
}

func TestBuildReportRejectsSymlinkedEvidenceDirectoryEscape(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "leaked.md"), []byte("outside evidence\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "external")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	gateHash := prepareIncidentDrillEvidence(t, root)
	spec := validIncidentDrillSpec(gateHash)
	spec.Drill.Timeline[0].EvidencePath = "external/leaked.md"

	report, err := BuildReport(spec, root)
	if err != nil {
		t.Fatal(err)
	}
	if report.OK || !hasDrillCounterexample(report, "invalid_evidence_file") {
		t.Fatalf("expected symlinked parent evidence escape to be rejected, got %#v", report.Counterexamples)
	}
}

func TestReadSpecRejectsUnknownFields(t *testing.T) {
	_, err := ReadSpec(strings.NewReader(`{"version":"patchline.incident-response-drill/v1","name":"x","criteria":{"max_detection_minutes":60,"max_public_disclosure_hours":6,"max_mitigation_hours":12,"max_remediation_hours":48,"min_distinct_roles":3},"drill":{"drill_id":"d","title":"t","scenario":"s","severity":"high","false_negative":{"detector_id":"d","missed_signal_id":"s","original_patchline_command":"patchline x","public_report_at":"2026-02-01T12:00:00Z","discovered_at":"2026-02-01T13:00:00Z","affected_systems":["billing"]},"timeline":[],"remediations":[],"roles":[]},"extra":true}`))
	if err == nil {
		t.Fatal("expected unknown field error")
	}
}

func validIncidentDrillSpec(gateHash string) Spec {
	return Spec{
		Version: SpecVersion,
		Name:    "public false-negative incident drill",
		Criteria: Criteria{
			MaxDetectionMinutes:            60,
			MaxPublicDisclosureHours:       6,
			MaxMitigationHours:             12,
			MaxRemediationHours:            48,
			MinDistinctRoles:               4,
			RequirePublicDisclosure:        true,
			RequireCustomerImpactStatement: true,
			RequireRegressionGate:          true,
			RequirePostmortem:              true,
		},
		Drill: Drill{
			DrillID:  "fn-drill-2026-02-billing-nullability",
			Title:    "Billing nullable-column false negative drill",
			Scenario: "A nullable billing migration was under-escalated and caused delayed invoice finalization.",
			Severity: "high",
			FalseNegative: FalseNegative{
				DetectorID:               "db-semantics-nullability",
				MissedSignalID:           "nullable-column-backfill-gap",
				OriginalPatchlineCommand: "patchline repo analyze --github example/billing --subpath db/migrate --no-llm",
				PublicReportAt:           "2026-02-01T12:00:00Z",
				DiscoveredAt:             "2026-02-01T13:00:00Z",
				AffectedSystems:          []string{"billing", "invoice-finalization"},
				CustomerImpact:           "Invoices generated during the drill window could be delayed until a safe backfill guard shipped.",
			},
			Timeline: []TimelineEvent{
				drillEvent("detected", "detected", "2026-02-01T12:30:00Z", "incident-commander", "Public report reproduced against the pinned migration.", "evidence/detection-log.json"),
				drillEvent("triaged", "triaged", "2026-02-01T13:15:00Z", "database-responder", "False negative classified as a missed nullable-column backfill hazard.", "evidence/triage-notes.md"),
				drillEvent("public-disclosure", "public_disclosure", "2026-02-01T15:00:00Z", "communications-owner", "Status page disclosed the hypothetical false negative and mitigation window.", "evidence/status-page.md"),
				drillEvent("mitigated", "mitigated", "2026-02-01T17:00:00Z", "data-repair-owner", "Guarded writes and queued invoice finalization until backfill completed.", "evidence/mitigation-runbook.md"),
				drillEvent("remediated", "remediated", "2026-02-02T01:00:00Z", "detector-owner", "Detector regression and remediation patch completed.", "evidence/remediation-pr.md"),
				drillEvent("regression-added", "regression_added", "2026-02-02T00:30:00Z", "detector-owner", "Focused regression gate added before closure.", "evidence/regression-gate-report.json"),
				drillEvent("postmortem-published", "postmortem_published", "2026-02-02T18:00:00Z", "incident-commander", "Public postmortem published with lessons and follow-up owners.", "evidence/postmortem.md"),
			},
			Disclosures: []Disclosure{{
				ID:           "status-page-update",
				Audience:     "public maintainers and adopters",
				Channel:      "status page",
				PlannedAt:    "2026-02-01T14:30:00Z",
				PublishedAt:  "2026-02-01T15:00:00Z",
				Summary:      "Patchline disclosed a hypothetical false negative, customer impact, mitigation, and expected remediation window.",
				EvidencePath: "evidence/status-page.md",
			}},
			Remediations: []Remediation{
				{
					ID:               "detector-regression",
					Kind:             "regression_gate",
					Owner:            "detector-owner",
					DueAt:            "2026-02-03T13:00:00Z",
					CompletedAt:      "2026-02-02T01:00:00Z",
					Command:          "make incident-postmortem-importer-gate",
					GateReportPath:   "evidence/regression-gate-report.json",
					GateReportSHA256: gateHash,
					EvidencePath:     "evidence/remediation-pr.md",
				},
				{
					ID:           "customer-mitigation",
					Kind:         "customer_repair",
					Owner:        "data-repair-owner",
					DueAt:        "2026-02-02T13:00:00Z",
					CompletedAt:  "2026-02-01T17:00:00Z",
					Command:      "make canary-validation-gate",
					EvidencePath: "evidence/mitigation-runbook.md",
				},
			},
			Roles: []RoleAssignment{
				drillRole("incident commander", "ivy-incident", "sam-backup", "evidence/incident-commander.md"),
				drillRole("database responder", "robin-db", "devon-db", "evidence/database-responder.md"),
				drillRole("communications owner", "casey-comms", "lee-comms", "evidence/communications-owner.md"),
				drillRole("data repair owner", "drew-data", "riley-data", "evidence/data-repair-owner.md"),
			},
			EvidencePaths: []string{"evidence/public-report.md", "evidence/postmortem.md"},
		},
	}
}

func drillEvent(id, phase, at, owner, summary, evidencePath string) TimelineEvent {
	return TimelineEvent{ID: id, Phase: phase, At: at, Owner: owner, Summary: summary, EvidencePath: evidencePath}
}

func drillRole(role, owner, backup, evidencePath string) RoleAssignment {
	return RoleAssignment{Role: role, Owner: owner, Backup: backup, EvidencePath: evidencePath}
}

func prepareIncidentDrillEvidence(t *testing.T, root string) string {
	t.Helper()
	files := map[string]string{
		"evidence/public-report.md":            "Public report: nullable billing migration under-escalated by Patchline.\n",
		"evidence/detection-log.json":          `{"case":"fn-drill","reproduced":true,"migration":"2026020101_nullable_invoice_state.sql"}` + "\n",
		"evidence/triage-notes.md":             "Triage notes: missed nullable-column backfill hazard assigned to detector owner.\n",
		"evidence/status-page.md":              "Status page: hypothetical false negative disclosed with mitigation and remediation ETA.\n",
		"evidence/mitigation-runbook.md":       "Mitigation: guard invoice finalization and queue retries until backfill completes.\n",
		"evidence/remediation-pr.md":           "Remediation PR: detector regression, fixture, and focused gate added.\n",
		"evidence/regression-gate-report.json": `{"version":"patchline.gate-report/v1","gate_id":"incident-postmortem-importer-gate","status":"pass","checked_at":"2026-02-02T00:30:00Z","regressions":3}` + "\n",
		"evidence/postmortem.md":               "Postmortem: root cause, disclosure timeline, remediation timeline, and action items.\n",
		"evidence/incident-commander.md":       "Incident commander: owns timeline, disclosure decision, and postmortem publication.\n",
		"evidence/database-responder.md":       "Database responder: validates migration and backfill semantics.\n",
		"evidence/communications-owner.md":     "Communications owner: publishes public status and adopter update.\n",
		"evidence/data-repair-owner.md":        "Data repair owner: owns mitigation and customer-safe repair runbook.\n",
	}
	for rel, contents := range files {
		writeIncidentDrillFile(t, root, rel, contents)
	}
	return testIncidentDrillHash(t, filepath.Join(root, "evidence/regression-gate-report.json"))
}

func writeIncidentDrillFile(t *testing.T, root, relPath, contents string) {
	t.Helper()
	path := filepath.Join(root, relPath)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
}

func testIncidentDrillHash(t *testing.T, path string) string {
	t.Helper()
	bytes, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(bytes)
	return hex.EncodeToString(sum[:])
}

func hasDrillCounterexample(report Report, kind string) bool {
	for _, counterexample := range report.Counterexamples {
		if counterexample.Kind == kind {
			return true
		}
	}
	return false
}

func hasMatchingRegressionGate(report Report) bool {
	for _, remediation := range report.Drill.Remediations {
		if remediation.Kind == "regression_gate" && remediation.HashMatches {
			return true
		}
	}
	return false
}
