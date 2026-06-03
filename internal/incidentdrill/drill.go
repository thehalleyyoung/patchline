package incidentdrill

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/thehalleyyoung/patchline/internal/canonical"
)

const SpecVersion = "patchline.incident-response-drill/v1"
const ReportVersion = "patchline.incident-response-drill-report/v1"

type Spec struct {
	Version  string   `json:"version"`
	Name     string   `json:"name"`
	Criteria Criteria `json:"criteria"`
	Drill    Drill    `json:"drill"`
}

type Criteria struct {
	MaxDetectionMinutes            int  `json:"max_detection_minutes"`
	MaxPublicDisclosureHours       int  `json:"max_public_disclosure_hours"`
	MaxMitigationHours             int  `json:"max_mitigation_hours"`
	MaxRemediationHours            int  `json:"max_remediation_hours"`
	MinDistinctRoles               int  `json:"min_distinct_roles"`
	RequirePublicDisclosure        bool `json:"require_public_disclosure"`
	RequireCustomerImpactStatement bool `json:"require_customer_impact_statement"`
	RequireRegressionGate          bool `json:"require_regression_gate"`
	RequirePostmortem              bool `json:"require_postmortem"`
}

type Drill struct {
	DrillID       string           `json:"drill_id"`
	Title         string           `json:"title"`
	Scenario      string           `json:"scenario"`
	Severity      string           `json:"severity"`
	FalseNegative FalseNegative    `json:"false_negative"`
	Timeline      []TimelineEvent  `json:"timeline"`
	Disclosures   []Disclosure     `json:"disclosures"`
	Remediations  []Remediation    `json:"remediations"`
	Roles         []RoleAssignment `json:"roles"`
	EvidencePaths []string         `json:"evidence_paths"`
}

type FalseNegative struct {
	DetectorID               string   `json:"detector_id"`
	MissedSignalID           string   `json:"missed_signal_id"`
	OriginalPatchlineCommand string   `json:"original_patchline_command"`
	PublicReportAt           string   `json:"public_report_at"`
	DiscoveredAt             string   `json:"discovered_at"`
	AffectedSystems          []string `json:"affected_systems"`
	CustomerImpact           string   `json:"customer_impact"`
}

type TimelineEvent struct {
	ID           string `json:"id"`
	Phase        string `json:"phase"`
	At           string `json:"at"`
	Owner        string `json:"owner"`
	Summary      string `json:"summary"`
	EvidencePath string `json:"evidence_path"`
}

type Disclosure struct {
	ID           string `json:"id"`
	Audience     string `json:"audience"`
	Channel      string `json:"channel"`
	PlannedAt    string `json:"planned_at"`
	PublishedAt  string `json:"published_at"`
	Summary      string `json:"summary"`
	EvidencePath string `json:"evidence_path"`
}

type Remediation struct {
	ID               string `json:"id"`
	Kind             string `json:"kind"`
	Owner            string `json:"owner"`
	DueAt            string `json:"due_at"`
	CompletedAt      string `json:"completed_at"`
	Command          string `json:"command"`
	GateReportPath   string `json:"gate_report_path,omitempty"`
	GateReportSHA256 string `json:"gate_report_sha256,omitempty"`
	EvidencePath     string `json:"evidence_path"`
}

type RoleAssignment struct {
	Role         string `json:"role"`
	Owner        string `json:"owner"`
	Backup       string `json:"backup"`
	EvidencePath string `json:"evidence_path"`
}

type Report struct {
	Version         string           `json:"version"`
	Name            string           `json:"name"`
	OK              bool             `json:"ok"`
	Criteria        Criteria         `json:"criteria"`
	Summary         Summary          `json:"summary"`
	Drill           DrillReport      `json:"drill"`
	Counterexamples []Counterexample `json:"counterexamples,omitempty"`
	Hash            string           `json:"hash"`
}

type Summary struct {
	TimelineEvents        int     `json:"timeline_events"`
	Disclosures           int     `json:"disclosures"`
	Remediations          int     `json:"remediations"`
	RegressionGates       int     `json:"regression_gates"`
	Roles                 int     `json:"roles"`
	DistinctRoles         int     `json:"distinct_roles"`
	DistinctOwners        int     `json:"distinct_owners"`
	EvidenceFiles         int     `json:"evidence_files"`
	DetectionMinutes      float64 `json:"detection_minutes"`
	PublicDisclosureHours float64 `json:"public_disclosure_hours"`
	MitigationHours       float64 `json:"mitigation_hours"`
	RemediationHours      float64 `json:"remediation_hours"`
	Counterexamples       int     `json:"counterexamples"`
}

type DrillReport struct {
	DrillID       string              `json:"drill_id"`
	Title         string              `json:"title"`
	Scenario      string              `json:"scenario"`
	Severity      string              `json:"severity"`
	FalseNegative FalseNegative       `json:"false_negative"`
	Timeline      []TimelineReport    `json:"timeline"`
	Disclosures   []DisclosureReport  `json:"disclosures"`
	Remediations  []RemediationReport `json:"remediations"`
	Roles         []RoleReport        `json:"roles"`
	Evidence      []ArtifactEvidence  `json:"evidence"`
}

type TimelineReport struct {
	TimelineEvent
	Evidence *ArtifactEvidence `json:"evidence,omitempty"`
}

type DisclosureReport struct {
	Disclosure
	Evidence *ArtifactEvidence `json:"evidence,omitempty"`
}

type RemediationReport struct {
	Remediation
	Evidence     *ArtifactEvidence `json:"evidence,omitempty"`
	GateReport   *ArtifactEvidence `json:"gate_report,omitempty"`
	ExpectedHash string            `json:"expected_hash,omitempty"`
	HashMatches  bool              `json:"hash_matches"`
}

type RoleReport struct {
	RoleAssignment
	Evidence *ArtifactEvidence `json:"evidence,omitempty"`
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
		return Spec{}, fmt.Errorf("incident-response drill spec version must be %s", SpecVersion)
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

	evidenceSeen := map[string]ArtifactEvidence{}
	report := Report{
		Version:  ReportVersion,
		Name:     spec.Name,
		OK:       true,
		Criteria: spec.Criteria,
		Drill: DrillReport{
			DrillID:       spec.Drill.DrillID,
			Title:         spec.Drill.Title,
			Scenario:      spec.Drill.Scenario,
			Severity:      spec.Drill.Severity,
			FalseNegative: spec.Drill.FalseNegative,
		},
	}

	var counterexamples []Counterexample
	for _, relPath := range sortedStrings(spec.Drill.EvidencePaths) {
		evidence, fileCounterexamples := resolveFileUnderRoot(rootAbs, relPath, spec.Drill.DrillID, "drill_evidence")
		counterexamples = append(counterexamples, fileCounterexamples...)
		if evidence != nil {
			report.Drill.Evidence = append(report.Drill.Evidence, *evidence)
			evidenceSeen[evidence.Path] = *evidence
		}
	}

	timelineReports, timelineCounterexamples := evaluateTimeline(spec.Drill, rootAbs, evidenceSeen)
	report.Drill.Timeline = timelineReports
	counterexamples = append(counterexamples, timelineCounterexamples...)

	disclosureReports, disclosureCounterexamples := evaluateDisclosures(spec.Drill.Disclosures, rootAbs, evidenceSeen)
	report.Drill.Disclosures = disclosureReports
	counterexamples = append(counterexamples, disclosureCounterexamples...)

	remediationReports, remediationCounterexamples := evaluateRemediations(spec.Drill.Remediations, rootAbs, evidenceSeen)
	report.Drill.Remediations = remediationReports
	counterexamples = append(counterexamples, remediationCounterexamples...)

	roleReports, roleCounterexamples := evaluateRoles(spec.Drill.Roles, rootAbs, evidenceSeen)
	report.Drill.Roles = roleReports
	counterexamples = append(counterexamples, roleCounterexamples...)

	report.Summary = summarize(spec, report.Drill)
	counterexamples = append(counterexamples, criteriaCounterexamples(spec, report.Summary)...)
	sortCounterexamples(counterexamples)
	report.Counterexamples = counterexamples
	report.Summary.Counterexamples = len(counterexamples)
	report.Summary.EvidenceFiles = len(evidenceSeen)
	report.OK = len(counterexamples) == 0
	report.Hash = reportHash(report)
	return report, nil
}

func WriteArtifacts(outDir string, report Report) error {
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}
	jsonFile, err := os.Create(filepath.Join(outDir, "incident-response-drill.json"))
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
	return os.WriteFile(filepath.Join(outDir, "incident-response-drill.md"), []byte(RenderMarkdown(report)), 0o644)
}

func RenderMarkdown(report Report) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Incident-response drill\n\n")
	fmt.Fprintf(&b, "Patchline rehearses a hypothetical false negative as a public incident: detection, disclosure, mitigation, remediation, regression-gate closure, and postmortem publication are all timestamped and evidence-hashed.\n\n")
	fmt.Fprintf(&b, "## Summary\n\n")
	fmt.Fprintf(&b, "| Metric | Value |\n| --- | ---: |\n")
	fmt.Fprintf(&b, "| OK | `%t` |\n", report.OK)
	fmt.Fprintf(&b, "| Timeline events | %d |\n", report.Summary.TimelineEvents)
	fmt.Fprintf(&b, "| Public disclosures | %d |\n", report.Summary.Disclosures)
	fmt.Fprintf(&b, "| Remediations | %d |\n", report.Summary.Remediations)
	fmt.Fprintf(&b, "| Regression gates | %d |\n", report.Summary.RegressionGates)
	fmt.Fprintf(&b, "| Distinct roles | %d |\n", report.Summary.DistinctRoles)
	fmt.Fprintf(&b, "| Distinct owners | %d |\n", report.Summary.DistinctOwners)
	fmt.Fprintf(&b, "| Evidence files | %d |\n", report.Summary.EvidenceFiles)
	fmt.Fprintf(&b, "| Detection minutes | %.2f |\n", report.Summary.DetectionMinutes)
	fmt.Fprintf(&b, "| Public disclosure hours | %.2f |\n", report.Summary.PublicDisclosureHours)
	fmt.Fprintf(&b, "| Mitigation hours | %.2f |\n", report.Summary.MitigationHours)
	fmt.Fprintf(&b, "| Remediation hours | %.2f |\n", report.Summary.RemediationHours)
	fmt.Fprintf(&b, "| Counterexamples | %d |\n\n", report.Summary.Counterexamples)

	fmt.Fprintf(&b, "Policy: detection within `%d` minutes of the public report, public disclosure within `%d` hours of discovery, mitigation within `%d` hours, remediation within `%d` hours, at least `%d` distinct public-response roles, public disclosure `%t`, customer-impact statement `%t`, regression gate `%t`, postmortem `%t`.\n\n", report.Criteria.MaxDetectionMinutes, report.Criteria.MaxPublicDisclosureHours, report.Criteria.MaxMitigationHours, report.Criteria.MaxRemediationHours, report.Criteria.MinDistinctRoles, report.Criteria.RequirePublicDisclosure, report.Criteria.RequireCustomerImpactStatement, report.Criteria.RequireRegressionGate, report.Criteria.RequirePostmortem)

	fmt.Fprintf(&b, "## False negative under drill\n\n")
	fmt.Fprintf(&b, "| Field | Value |\n| --- | --- |\n")
	fmt.Fprintf(&b, "| Drill | `%s` |\n", report.Drill.DrillID)
	fmt.Fprintf(&b, "| Detector | `%s` |\n", report.Drill.FalseNegative.DetectorID)
	fmt.Fprintf(&b, "| Missed signal | `%s` |\n", report.Drill.FalseNegative.MissedSignalID)
	fmt.Fprintf(&b, "| Severity | `%s` |\n", report.Drill.Severity)
	fmt.Fprintf(&b, "| Affected systems | `%s` |\n\n", strings.Join(sortedStrings(report.Drill.FalseNegative.AffectedSystems), ", "))

	fmt.Fprintf(&b, "## Timeline\n\n")
	fmt.Fprintf(&b, "| Phase | At | Owner | Evidence |\n| --- | --- | --- | ---: |\n")
	for _, event := range report.Drill.Timeline {
		evidence := 0
		if event.Evidence != nil {
			evidence = 1
		}
		fmt.Fprintf(&b, "| `%s` | `%s` | `%s` | %d |\n", event.Phase, event.At, event.Owner, evidence)
	}

	fmt.Fprintf(&b, "\n## Public disclosure and remediation\n\n")
	fmt.Fprintf(&b, "| Item | Owner/audience | Completed/published | Evidence |\n| --- | --- | --- | ---: |\n")
	for _, disclosure := range report.Drill.Disclosures {
		evidence := 0
		if disclosure.Evidence != nil {
			evidence = 1
		}
		fmt.Fprintf(&b, "| disclosure `%s` | `%s` | `%s` | %d |\n", disclosure.ID, disclosure.Audience, disclosure.PublishedAt, evidence)
	}
	for _, remediation := range report.Drill.Remediations {
		evidence := 0
		if remediation.Evidence != nil {
			evidence++
		}
		if remediation.GateReport != nil {
			evidence++
		}
		fmt.Fprintf(&b, "| remediation `%s` | `%s` | `%s` | %d |\n", remediation.ID, remediation.Owner, remediation.CompletedAt, evidence)
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

func evaluateTimeline(drill Drill, rootAbs string, evidenceSeen map[string]ArtifactEvidence) ([]TimelineReport, []Counterexample) {
	var reports []TimelineReport
	var counterexamples []Counterexample
	for _, event := range sortedTimeline(drill.Timeline) {
		report := TimelineReport{TimelineEvent: event}
		if containsPrivateMarker(event.Summary) {
			counterexamples = append(counterexamples, Counterexample{
				ID:      fmt.Sprintf("timeline-private-marker-%s", safeID(event.ID)),
				Kind:    "public_summary_private_marker",
				Subject: event.ID,
				Message: "timeline public summary contains a private marker and cannot be published",
				Witness: []string{event.Summary},
			})
		}
		if strings.TrimSpace(event.EvidencePath) == "" {
			counterexamples = append(counterexamples, Counterexample{
				ID:      fmt.Sprintf("missing-timeline-evidence-%s", safeID(event.ID)),
				Kind:    "missing_evidence",
				Subject: event.ID,
				Message: "timeline event must preserve evidence for the public drill",
			})
		} else {
			evidence, fileCounterexamples := resolveFileUnderRoot(rootAbs, event.EvidencePath, event.ID, "timeline_evidence")
			counterexamples = append(counterexamples, fileCounterexamples...)
			if evidence != nil {
				report.Evidence = evidence
				evidenceSeen[evidence.Path] = *evidence
			}
		}
		reports = append(reports, report)
	}
	return reports, counterexamples
}

func evaluateDisclosures(disclosures []Disclosure, rootAbs string, evidenceSeen map[string]ArtifactEvidence) ([]DisclosureReport, []Counterexample) {
	var reports []DisclosureReport
	var counterexamples []Counterexample
	for _, disclosure := range sortedDisclosures(disclosures) {
		report := DisclosureReport{Disclosure: disclosure}
		if containsPrivateMarker(disclosure.Summary) {
			counterexamples = append(counterexamples, Counterexample{
				ID:      fmt.Sprintf("disclosure-private-marker-%s", safeID(disclosure.ID)),
				Kind:    "public_summary_private_marker",
				Subject: disclosure.ID,
				Message: "public disclosure summary contains a private marker and cannot be published",
				Witness: []string{disclosure.Summary},
			})
		}
		if planned, published, ok := parsedPair(disclosure.PlannedAt, disclosure.PublishedAt); ok && published.Before(planned) {
			counterexamples = append(counterexamples, Counterexample{
				ID:      fmt.Sprintf("disclosure-published-before-plan-%s", safeID(disclosure.ID)),
				Kind:    "timeline_order_violation",
				Subject: disclosure.ID,
				Message: "disclosure published_at must not be before planned_at",
				Witness: []string{disclosure.PlannedAt, disclosure.PublishedAt},
			})
		}
		if strings.TrimSpace(disclosure.EvidencePath) == "" {
			counterexamples = append(counterexamples, Counterexample{
				ID:      fmt.Sprintf("missing-disclosure-evidence-%s", safeID(disclosure.ID)),
				Kind:    "missing_evidence",
				Subject: disclosure.ID,
				Message: "public disclosure must preserve the published artifact",
			})
		} else {
			evidence, fileCounterexamples := resolveFileUnderRoot(rootAbs, disclosure.EvidencePath, disclosure.ID, "disclosure_evidence")
			counterexamples = append(counterexamples, fileCounterexamples...)
			if evidence != nil {
				report.Evidence = evidence
				evidenceSeen[evidence.Path] = *evidence
			}
		}
		reports = append(reports, report)
	}
	return reports, counterexamples
}

func evaluateRemediations(remediations []Remediation, rootAbs string, evidenceSeen map[string]ArtifactEvidence) ([]RemediationReport, []Counterexample) {
	var reports []RemediationReport
	var counterexamples []Counterexample
	for _, remediation := range sortedRemediations(remediations) {
		report := RemediationReport{Remediation: remediation, ExpectedHash: normalizeSHA256(remediation.GateReportSHA256)}
		if due, completed, ok := parsedPair(remediation.DueAt, remediation.CompletedAt); ok && completed.After(due) {
			counterexamples = append(counterexamples, Counterexample{
				ID:      fmt.Sprintf("remediation-after-due-%s", safeID(remediation.ID)),
				Kind:    "remediation_completed_after_due",
				Subject: remediation.ID,
				Message: "remediation completed_at is after its committed due_at",
				Witness: []string{remediation.DueAt, remediation.CompletedAt},
			})
		}
		if strings.TrimSpace(remediation.Command) == "" {
			counterexamples = append(counterexamples, Counterexample{
				ID:      fmt.Sprintf("missing-remediation-command-%s", safeID(remediation.ID)),
				Kind:    "missing_remediation_command",
				Subject: remediation.ID,
				Message: "remediation must name the command or gate that proves closure",
			})
		}
		if remediation.Kind == "regression_gate" && !isAllowedGateCommand(remediation.Command) {
			counterexamples = append(counterexamples, Counterexample{
				ID:      fmt.Sprintf("regression-gate-command-not-allowlisted-%s", safeID(remediation.ID)),
				Kind:    "regression_gate_command_not_allowlisted",
				Subject: remediation.ID,
				Message: "regression remediation must use an allowlisted Patchline or gate command",
				Witness: []string{remediation.Command},
			})
		}
		if strings.TrimSpace(remediation.EvidencePath) == "" {
			counterexamples = append(counterexamples, Counterexample{
				ID:      fmt.Sprintf("missing-remediation-evidence-%s", safeID(remediation.ID)),
				Kind:    "missing_evidence",
				Subject: remediation.ID,
				Message: "remediation must preserve implementation evidence",
			})
		} else {
			evidence, fileCounterexamples := resolveFileUnderRoot(rootAbs, remediation.EvidencePath, remediation.ID, "remediation_evidence")
			counterexamples = append(counterexamples, fileCounterexamples...)
			if evidence != nil {
				report.Evidence = evidence
				evidenceSeen[evidence.Path] = *evidence
			}
		}
		if remediation.Kind == "regression_gate" || strings.TrimSpace(remediation.GateReportPath) != "" {
			if strings.TrimSpace(remediation.GateReportPath) == "" {
				counterexamples = append(counterexamples, Counterexample{
					ID:      fmt.Sprintf("missing-regression-gate-report-%s", safeID(remediation.ID)),
					Kind:    "missing_regression_gate_report",
					Subject: remediation.ID,
					Message: "regression gate remediation must preserve the gate report artifact",
				})
			} else {
				gateReport, fileCounterexamples := resolveFileUnderRoot(rootAbs, remediation.GateReportPath, remediation.ID, "remediation_gate_report")
				counterexamples = append(counterexamples, fileCounterexamples...)
				if gateReport != nil {
					report.GateReport = gateReport
					evidenceSeen[gateReport.Path] = *gateReport
					if report.ExpectedHash == "" {
						counterexamples = append(counterexamples, Counterexample{
							ID:      fmt.Sprintf("missing-remediation-gate-hash-%s", safeID(remediation.ID)),
							Kind:    "missing_remediation_gate_hash",
							Subject: remediation.ID,
							Message: "regression gate report hash must be pinned",
							Witness: []string{remediation.GateReportPath},
						})
					} else {
						report.HashMatches = strings.EqualFold(report.ExpectedHash, gateReport.SHA256)
						if !report.HashMatches {
							counterexamples = append(counterexamples, Counterexample{
								ID:      fmt.Sprintf("remediation-gate-hash-mismatch-%s", safeID(remediation.ID)),
								Kind:    "remediation_gate_hash_mismatch",
								Subject: remediation.ID,
								Message: "remediation gate report hash does not match the preserved artifact",
								Witness: []string{remediation.GateReportPath, gateReport.SHA256, report.ExpectedHash},
							})
						}
					}
				}
			}
		}
		reports = append(reports, report)
	}
	return reports, counterexamples
}

func evaluateRoles(roles []RoleAssignment, rootAbs string, evidenceSeen map[string]ArtifactEvidence) ([]RoleReport, []Counterexample) {
	var reports []RoleReport
	var counterexamples []Counterexample
	for _, role := range sortedRoles(roles) {
		report := RoleReport{RoleAssignment: role}
		if strings.TrimSpace(role.EvidencePath) == "" {
			counterexamples = append(counterexamples, Counterexample{
				ID:      fmt.Sprintf("missing-role-evidence-%s", safeID(role.Role)),
				Kind:    "missing_evidence",
				Subject: role.Role,
				Message: "incident role assignment must preserve ownership evidence",
			})
		} else {
			evidence, fileCounterexamples := resolveFileUnderRoot(rootAbs, role.EvidencePath, role.Role, "role_evidence")
			counterexamples = append(counterexamples, fileCounterexamples...)
			if evidence != nil {
				report.Evidence = evidence
				evidenceSeen[evidence.Path] = *evidence
			}
		}
		reports = append(reports, report)
	}
	return reports, counterexamples
}

func summarize(spec Spec, drill DrillReport) Summary {
	summary := Summary{
		TimelineEvents: len(drill.Timeline),
		Disclosures:    len(drill.Disclosures),
		Remediations:   len(drill.Remediations),
		Roles:          len(drill.Roles),
	}
	roleSet := map[string]bool{}
	ownerSet := map[string]bool{}
	for _, role := range drill.Roles {
		roleSet[strings.ToLower(strings.TrimSpace(role.Role))] = true
		ownerSet[strings.ToLower(strings.TrimSpace(role.Owner))] = true
	}
	summary.DistinctRoles = len(roleSet)
	summary.DistinctOwners = len(ownerSet)
	for _, remediation := range drill.Remediations {
		if remediation.Kind == "regression_gate" {
			summary.RegressionGates++
		}
	}

	publicReport := mustParseTime(spec.Drill.FalseNegative.PublicReportAt)
	discovered := mustParseTime(spec.Drill.FalseNegative.DiscoveredAt)
	if detected, ok := phaseTime(drill.Timeline, "detected"); ok && !detected.Before(publicReport) {
		summary.DetectionMinutes = round4(detected.Sub(publicReport).Minutes())
	}
	if disclosure, ok := earliestDisclosurePublished(drill.Disclosures); ok && !disclosure.Before(discovered) {
		summary.PublicDisclosureHours = round4(disclosure.Sub(discovered).Hours())
	}
	if mitigated, ok := phaseTime(drill.Timeline, "mitigated"); ok && !mitigated.Before(discovered) {
		summary.MitigationHours = round4(mitigated.Sub(discovered).Hours())
	}
	if remediated, ok := latestRemediationCompleted(drill.Remediations); ok && !remediated.Before(discovered) {
		summary.RemediationHours = round4(remediated.Sub(discovered).Hours())
	}
	return summary
}

func criteriaCounterexamples(spec Spec, summary Summary) []Counterexample {
	var counterexamples []Counterexample
	drill := spec.Drill
	criteria := spec.Criteria
	phaseTimes := map[string]time.Time{}
	for _, event := range sortedTimeline(drill.Timeline) {
		if _, exists := phaseTimes[event.Phase]; !exists {
			phaseTimes[event.Phase] = mustParseTime(event.At)
		}
	}
	for _, phase := range []string{"detected", "triaged", "public_disclosure", "mitigated", "remediated", "regression_added", "postmortem_published"} {
		if _, ok := phaseTimes[phase]; !ok {
			counterexamples = append(counterexamples, Counterexample{
				ID:      fmt.Sprintf("missing-timeline-phase-%s", safeID(phase)),
				Kind:    "missing_timeline_phase",
				Subject: phase,
				Message: "incident-response drill must include every required public timeline phase",
			})
		}
	}

	publicReport := mustParseTime(drill.FalseNegative.PublicReportAt)
	discovered := mustParseTime(drill.FalseNegative.DiscoveredAt)
	if discovered.Before(publicReport) {
		counterexamples = append(counterexamples, Counterexample{
			ID:      "false-negative-discovered-before-public-report",
			Kind:    "timeline_order_violation",
			Subject: "false_negative",
			Message: "discovered_at must not be before public_report_at",
			Witness: []string{drill.FalseNegative.PublicReportAt, drill.FalseNegative.DiscoveredAt},
		})
	}
	if detected, ok := phaseTimes["detected"]; ok {
		counterexamples = append(counterexamples, durationCounterexample("detection", "detected", publicReport, detected, float64(criteria.MaxDetectionMinutes), "minutes")...)
	}
	if disclosure, ok := earliestDisclosurePublishedFromSpec(drill.Disclosures); ok {
		counterexamples = append(counterexamples, durationCounterexample("public_disclosure", "disclosure", discovered, disclosure, float64(criteria.MaxPublicDisclosureHours), "hours")...)
	}
	if mitigated, ok := phaseTimes["mitigated"]; ok {
		counterexamples = append(counterexamples, durationCounterexample("mitigation", "mitigated", discovered, mitigated, float64(criteria.MaxMitigationHours), "hours")...)
	}
	if remediated, ok := latestRemediationCompletedFromSpec(drill.Remediations); ok {
		counterexamples = append(counterexamples, durationCounterexample("remediation", "remediation", discovered, remediated, float64(criteria.MaxRemediationHours), "hours")...)
	}
	if criteria.RequirePublicDisclosure && len(drill.Disclosures) == 0 {
		counterexamples = append(counterexamples, Counterexample{
			ID:      "missing-public-disclosure",
			Kind:    "missing_public_disclosure",
			Subject: drill.DrillID,
			Message: "public incident drill must include at least one public disclosure artifact",
		})
	}
	if criteria.RequireCustomerImpactStatement && strings.TrimSpace(drill.FalseNegative.CustomerImpact) == "" {
		counterexamples = append(counterexamples, Counterexample{
			ID:      "missing-customer-impact-statement",
			Kind:    "missing_customer_impact_statement",
			Subject: "false_negative",
			Message: "false negative must include a customer-impact statement",
		})
	}
	if containsPrivateMarker(drill.FalseNegative.CustomerImpact) {
		counterexamples = append(counterexamples, Counterexample{
			ID:      "customer-impact-private-marker",
			Kind:    "public_summary_private_marker",
			Subject: "false_negative",
			Message: "customer-impact statement contains a private marker and cannot be published",
			Witness: []string{drill.FalseNegative.CustomerImpact},
		})
	}
	if criteria.RequireRegressionGate && summary.RegressionGates == 0 {
		counterexamples = append(counterexamples, Counterexample{
			ID:      "missing-regression-gate",
			Kind:    "missing_regression_gate",
			Subject: drill.DrillID,
			Message: "false-negative drill must close with a regression-gate remediation",
		})
	}
	if criteria.RequirePostmortem {
		if _, ok := phaseTimes["postmortem_published"]; !ok {
			counterexamples = append(counterexamples, Counterexample{
				ID:      "missing-postmortem",
				Kind:    "missing_postmortem",
				Subject: drill.DrillID,
				Message: "public drill must publish a postmortem phase",
			})
		}
	}
	if summary.DistinctRoles < criteria.MinDistinctRoles {
		counterexamples = append(counterexamples, Counterexample{
			ID:      "insufficient-distinct-roles",
			Kind:    "insufficient_distinct_roles",
			Subject: drill.DrillID,
			Message: fmt.Sprintf("drill has %d distinct roles, below required %d", summary.DistinctRoles, criteria.MinDistinctRoles),
		})
	}
	if summary.DistinctOwners < criteria.MinDistinctRoles {
		counterexamples = append(counterexamples, Counterexample{
			ID:      "insufficient-distinct-owners",
			Kind:    "insufficient_distinct_owners",
			Subject: drill.DrillID,
			Message: fmt.Sprintf("drill has %d distinct role owners, below required %d", summary.DistinctOwners, criteria.MinDistinctRoles),
		})
	}
	return counterexamples
}

func durationCounterexample(metric, subject string, start, end time.Time, limit float64, unit string) []Counterexample {
	if end.Before(start) {
		return []Counterexample{{
			ID:      fmt.Sprintf("%s-order-violation", strings.ReplaceAll(metric, "_", "-")),
			Kind:    "timeline_order_violation",
			Subject: subject,
			Message: fmt.Sprintf("%s endpoint occurs before its start timestamp", strings.ReplaceAll(metric, "_", " ")),
			Witness: []string{start.Format(time.RFC3339), end.Format(time.RFC3339)},
		}}
	}
	var observed float64
	if unit == "minutes" {
		observed = end.Sub(start).Minutes()
	} else {
		observed = end.Sub(start).Hours()
	}
	if observed <= limit {
		return nil
	}
	return []Counterexample{{
		ID:      fmt.Sprintf("%s-deadline-exceeded", strings.ReplaceAll(metric, "_", "-")),
		Kind:    metric + "_deadline_exceeded",
		Subject: subject,
		Message: fmt.Sprintf("%s took %.2f %s, above allowed %.2f", strings.ReplaceAll(metric, "_", " "), observed, unit, limit),
		Witness: []string{start.Format(time.RFC3339), end.Format(time.RFC3339)},
	}}
}

func validateSpec(spec Spec) error {
	if spec.Version != SpecVersion {
		return fmt.Errorf("incident-response drill spec version must be %s", SpecVersion)
	}
	if strings.TrimSpace(spec.Name) == "" {
		return fmt.Errorf("incident-response drill spec name is required")
	}
	if spec.Criteria.MaxDetectionMinutes < 1 {
		return fmt.Errorf("criteria.max_detection_minutes must be at least 1")
	}
	if spec.Criteria.MaxPublicDisclosureHours < 1 || spec.Criteria.MaxMitigationHours < 1 || spec.Criteria.MaxRemediationHours < 1 {
		return fmt.Errorf("criteria disclosure, mitigation, and remediation windows must be at least 1 hour")
	}
	if spec.Criteria.MinDistinctRoles < 2 {
		return fmt.Errorf("criteria.min_distinct_roles must be at least 2")
	}
	if strings.TrimSpace(spec.Drill.DrillID) == "" || strings.TrimSpace(spec.Drill.Title) == "" || strings.TrimSpace(spec.Drill.Scenario) == "" || strings.TrimSpace(spec.Drill.Severity) == "" {
		return fmt.Errorf("drill must include drill_id, title, scenario, and severity")
	}
	if strings.TrimSpace(spec.Drill.FalseNegative.DetectorID) == "" || strings.TrimSpace(spec.Drill.FalseNegative.MissedSignalID) == "" || strings.TrimSpace(spec.Drill.FalseNegative.OriginalPatchlineCommand) == "" {
		return fmt.Errorf("false_negative must include detector_id, missed_signal_id, and original_patchline_command")
	}
	if len(spec.Drill.FalseNegative.AffectedSystems) == 0 {
		return fmt.Errorf("false_negative.affected_systems must not be empty")
	}
	if _, err := time.Parse(time.RFC3339, spec.Drill.FalseNegative.PublicReportAt); err != nil {
		return fmt.Errorf("false_negative.public_report_at must be RFC3339: %w", err)
	}
	if _, err := time.Parse(time.RFC3339, spec.Drill.FalseNegative.DiscoveredAt); err != nil {
		return fmt.Errorf("false_negative.discovered_at must be RFC3339: %w", err)
	}
	if err := validateTimeline(spec.Drill.Timeline); err != nil {
		return err
	}
	if err := validateDisclosures(spec.Drill.Disclosures); err != nil {
		return err
	}
	if err := validateRemediations(spec.Drill.Remediations); err != nil {
		return err
	}
	if err := validateRoles(spec.Drill.Roles); err != nil {
		return err
	}
	return nil
}

func validateTimeline(events []TimelineEvent) error {
	if len(events) == 0 {
		return fmt.Errorf("drill.timeline must not be empty")
	}
	seen := map[string]bool{}
	for _, event := range events {
		if strings.TrimSpace(event.ID) == "" || strings.TrimSpace(event.Phase) == "" || strings.TrimSpace(event.Owner) == "" {
			return fmt.Errorf("timeline event must include id, phase, and owner")
		}
		if seen[event.ID] {
			return fmt.Errorf("duplicate timeline event id %q", event.ID)
		}
		seen[event.ID] = true
		if _, err := time.Parse(time.RFC3339, event.At); err != nil {
			return fmt.Errorf("timeline event %q at must be RFC3339: %w", event.ID, err)
		}
	}
	return nil
}

func validateDisclosures(disclosures []Disclosure) error {
	seen := map[string]bool{}
	for _, disclosure := range disclosures {
		if strings.TrimSpace(disclosure.ID) == "" || strings.TrimSpace(disclosure.Audience) == "" || strings.TrimSpace(disclosure.Channel) == "" {
			return fmt.Errorf("disclosure must include id, audience, and channel")
		}
		if seen[disclosure.ID] {
			return fmt.Errorf("duplicate disclosure id %q", disclosure.ID)
		}
		seen[disclosure.ID] = true
		if _, err := time.Parse(time.RFC3339, disclosure.PlannedAt); err != nil {
			return fmt.Errorf("disclosure %q planned_at must be RFC3339: %w", disclosure.ID, err)
		}
		if _, err := time.Parse(time.RFC3339, disclosure.PublishedAt); err != nil {
			return fmt.Errorf("disclosure %q published_at must be RFC3339: %w", disclosure.ID, err)
		}
	}
	return nil
}

func validateRemediations(remediations []Remediation) error {
	if len(remediations) == 0 {
		return fmt.Errorf("drill.remediations must not be empty")
	}
	seen := map[string]bool{}
	for _, remediation := range remediations {
		if strings.TrimSpace(remediation.ID) == "" || strings.TrimSpace(remediation.Kind) == "" || strings.TrimSpace(remediation.Owner) == "" {
			return fmt.Errorf("remediation must include id, kind, and owner")
		}
		if seen[remediation.ID] {
			return fmt.Errorf("duplicate remediation id %q", remediation.ID)
		}
		seen[remediation.ID] = true
		if _, err := time.Parse(time.RFC3339, remediation.DueAt); err != nil {
			return fmt.Errorf("remediation %q due_at must be RFC3339: %w", remediation.ID, err)
		}
		if _, err := time.Parse(time.RFC3339, remediation.CompletedAt); err != nil {
			return fmt.Errorf("remediation %q completed_at must be RFC3339: %w", remediation.ID, err)
		}
	}
	return nil
}

func validateRoles(roles []RoleAssignment) error {
	if len(roles) == 0 {
		return fmt.Errorf("drill.roles must not be empty")
	}
	seen := map[string]bool{}
	for _, role := range roles {
		if strings.TrimSpace(role.Role) == "" || strings.TrimSpace(role.Owner) == "" || strings.TrimSpace(role.Backup) == "" {
			return fmt.Errorf("role assignment must include role, owner, and backup")
		}
		normalized := strings.ToLower(strings.TrimSpace(role.Role))
		if seen[normalized] {
			return fmt.Errorf("duplicate role assignment %q", role.Role)
		}
		seen[normalized] = true
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
			Message: fmt.Sprintf("%s path %q must be a relative file below the drill root", strings.ReplaceAll(kind, "_", " "), relPath),
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
			Message: fmt.Sprintf("%s file %q must be a regular file under the drill root", strings.ReplaceAll(kind, "_", " "), clean),
			Witness: []string{clean},
		}}
	}
	rootReal, err := filepath.EvalSymlinks(rootAbs)
	if err != nil {
		return nil, []Counterexample{{
			ID:      fmt.Sprintf("invalid-%s-root-%s", strings.ReplaceAll(kind, "_", "-"), safeID(subject)),
			Kind:    "invalid_evidence_root",
			Subject: subject,
			Message: fmt.Sprintf("drill root %q could not be resolved without symlinks: %v", rootAbs, err),
			Witness: []string{rootAbs},
		}}
	}
	artifactReal, err := filepath.EvalSymlinks(artifactPath)
	if err != nil {
		return nil, []Counterexample{{
			ID:      fmt.Sprintf("unreadable-%s-%s-%s", strings.ReplaceAll(kind, "_", "-"), safeID(subject), safeID(clean)),
			Kind:    "unreadable_evidence",
			Subject: subject,
			Message: fmt.Sprintf("%s file %q could not be resolved without symlinks: %v", strings.ReplaceAll(kind, "_", " "), clean, err),
			Witness: []string{clean},
		}}
	}
	relToRoot, err := filepath.Rel(rootReal, artifactReal)
	if err != nil || relToRoot == ".." || strings.HasPrefix(relToRoot, ".."+string(filepath.Separator)) || filepath.IsAbs(relToRoot) {
		return nil, []Counterexample{{
			ID:      fmt.Sprintf("escaped-%s-file-%s-%s", strings.ReplaceAll(kind, "_", "-"), safeID(subject), safeID(clean)),
			Kind:    "invalid_evidence_file",
			Subject: subject,
			Message: fmt.Sprintf("%s file %q resolves outside the drill root", strings.ReplaceAll(kind, "_", " "), clean),
			Witness: []string{clean, artifactReal, rootReal},
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

func phaseTime(timeline []TimelineReport, phase string) (time.Time, bool) {
	var selected time.Time
	found := false
	for _, event := range timeline {
		if event.Phase != phase {
			continue
		}
		at := mustParseTime(event.At)
		if !found || at.Before(selected) {
			selected = at
			found = true
		}
	}
	return selected, found
}

func earliestDisclosurePublished(disclosures []DisclosureReport) (time.Time, bool) {
	var selected time.Time
	found := false
	for _, disclosure := range disclosures {
		at := mustParseTime(disclosure.PublishedAt)
		if !found || at.Before(selected) {
			selected = at
			found = true
		}
	}
	return selected, found
}

func earliestDisclosurePublishedFromSpec(disclosures []Disclosure) (time.Time, bool) {
	var selected time.Time
	found := false
	for _, disclosure := range disclosures {
		at := mustParseTime(disclosure.PublishedAt)
		if !found || at.Before(selected) {
			selected = at
			found = true
		}
	}
	return selected, found
}

func latestRemediationCompleted(remediations []RemediationReport) (time.Time, bool) {
	var selected time.Time
	found := false
	for _, remediation := range remediations {
		at := mustParseTime(remediation.CompletedAt)
		if !found || at.After(selected) {
			selected = at
			found = true
		}
	}
	return selected, found
}

func latestRemediationCompletedFromSpec(remediations []Remediation) (time.Time, bool) {
	var selected time.Time
	found := false
	for _, remediation := range remediations {
		at := mustParseTime(remediation.CompletedAt)
		if !found || at.After(selected) {
			selected = at
			found = true
		}
	}
	return selected, found
}

func parsedPair(left, right string) (time.Time, time.Time, bool) {
	leftAt, errLeft := time.Parse(time.RFC3339, left)
	rightAt, errRight := time.Parse(time.RFC3339, right)
	if errLeft != nil || errRight != nil {
		return time.Time{}, time.Time{}, false
	}
	return leftAt, rightAt, true
}

func mustParseTime(value string) time.Time {
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		panic(err)
	}
	return parsed
}

func normalizeSHA256(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	return strings.TrimPrefix(value, "sha256:")
}

func isAllowedGateCommand(command string) bool {
	command = strings.TrimSpace(command)
	if strings.HasPrefix(command, "patchline ") {
		return true
	}
	if strings.HasPrefix(command, "go run ./cmd/patchline ") {
		return true
	}
	if strings.HasPrefix(command, "make ") && strings.Contains(command, "-gate") {
		return true
	}
	if strings.HasPrefix(command, "bash scripts/") && strings.Contains(command, "-gate.sh") {
		return true
	}
	return false
}

func containsPrivateMarker(value string) bool {
	lower := strings.ToLower(value)
	for _, marker := range []string{"token=", "secret=", "password=", "passwd=", "apikey", "api_key", "aws_secret", "bearer "} {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}

func reportHash(report Report) string {
	copyReport := report
	copyReport.Hash = ""
	return canonical.Hash(copyReport)
}

func sortedTimeline(events []TimelineEvent) []TimelineEvent {
	sorted := append([]TimelineEvent(nil), events...)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].At != sorted[j].At {
			return sorted[i].At < sorted[j].At
		}
		if sorted[i].Phase != sorted[j].Phase {
			return sorted[i].Phase < sorted[j].Phase
		}
		return sorted[i].ID < sorted[j].ID
	})
	return sorted
}

func sortedDisclosures(disclosures []Disclosure) []Disclosure {
	sorted := append([]Disclosure(nil), disclosures...)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].ID < sorted[j].ID
	})
	return sorted
}

func sortedRemediations(remediations []Remediation) []Remediation {
	sorted := append([]Remediation(nil), remediations...)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].ID < sorted[j].ID
	})
	return sorted
}

func sortedRoles(roles []RoleAssignment) []RoleAssignment {
	sorted := append([]RoleAssignment(nil), roles...)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Role < sorted[j].Role
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

func round4(value float64) float64 {
	return math.Round(value*10000) / 10000
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
