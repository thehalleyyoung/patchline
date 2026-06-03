package feedback

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/thehalleyyoung/patchline/internal/canonical"
)

func TestOnlineEvaluationKeepsFailingDetectorInShadowMode(t *testing.T) {
	report, err := Ingest(strings.NewReader(fixture), Options{})
	if err != nil {
		t.Fatal(err)
	}
	spec := OnlineEvaluationSpec{
		Version: OnlineEvaluationSpecVersion,
		Claim:   "Candidate detectors are evaluated in shadow mode against source-free reviewer outcome groups and can only become review candidates after precision, recall, and burden gates pass.",
		CandidateDetectors: []OnlineDetectorCandidate{
			{Detector: "orm.write-breadth", Release: "v1.0.0", MinEvidence: 3, MinPrecisionBP: 9000, MinRecallBP: 9000, MaxAverageBurdenMinutes: 8, RequiresHumanGate: true},
			{Detector: "sql.destructive-ddl", Release: "v1.0.0", MinEvidence: 3, MinPrecisionBP: 9000, MinRecallBP: 9000, MaxAverageBurdenMinutes: 12, RequiresHumanGate: true},
		},
	}
	evaluation, err := ComputeOnlineEvaluation(report, spec)
	if err != nil {
		t.Fatal(err)
	}
	if !evaluation.OK || evaluation.PolicyMutationAllowed || evaluation.EvidenceBasis != "published_k_anonymous_groups_only" || evaluation.Hash == "" {
		t.Fatalf("unexpected online evaluation header: %#v", evaluation)
	}
	if evaluation.Summary.DetectorsEvaluated != 2 || evaluation.Summary.PromotionCandidates != 1 || evaluation.Summary.ShadowOnly != 1 {
		t.Fatalf("unexpected online evaluation summary: %#v", evaluation.Summary)
	}
	orm := onlineDetectorFor(t, evaluation, "orm.write-breadth")
	if orm.Status != "shadow_only" || orm.Metrics.PrecisionBP != 0 {
		t.Fatalf("false-positive detector should remain shadow-only: %#v", orm)
	}
	sql := onlineDetectorFor(t, evaluation, "sql.destructive-ddl")
	if sql.Status != "candidate_ready_for_gated_review" || sql.Metrics.PrecisionBP != 10000 || sql.Metrics.RecallBP != 10000 {
		t.Fatalf("confirmed detector should clear online gates: %#v", sql)
	}
	assertNoLiveLearningLeak(t, evaluation)
	assertStableHash(t, evaluation.Hash, func() (string, error) {
		next, err := ComputeOnlineEvaluation(report, spec)
		return next.Hash, err
	})
}

func TestActiveLearningQueueSeparatesLocalExamplesFromShareableAggregate(t *testing.T) {
	spec := ActiveLearningSpec{
		Version:          ActiveLearningSpecVersion,
		Claim:            "Adopter-local active learning keeps individual examples and local case identifiers inside the organization while exporting only detector-level uncertainty aggregates for calibration planning.",
		MinUncertaintyBP: 2500,
		MinInfoGainBP:    3000,
		MaxQueueSize:     2,
		Cases: []ActiveLearningCase{
			{LocalCaseID: "case-001", Detector: "sql.destructive-ddl", Release: "v1.0.0", ConfidenceBP: 5100, UncertaintyBP: 4900, ExpectedInformationGainBP: 8100, EstimatedBurdenMinutes: 8},
			{LocalCaseID: "case-002", Detector: "orm.write-breadth", Release: "v1.0.0", ConfidenceBP: 5400, UncertaintyBP: 4600, ExpectedInformationGainBP: 7600, EstimatedBurdenMinutes: 6},
			{LocalCaseID: "case-003", Detector: "migration.guard", Release: "v1.0.0", ConfidenceBP: 9000, UncertaintyBP: 1000, ExpectedInformationGainBP: 1800, EstimatedBurdenMinutes: 4},
			{LocalCaseID: "case-004", Detector: "migration.guard", Release: "v1.0.0", ConfidenceBP: 5000, UncertaintyBP: 5000, ExpectedInformationGainBP: 9000, EstimatedBurdenMinutes: 9, AlreadyLabeled: true},
		},
	}
	report, err := ComputeActiveLearningQueue(spec)
	if err != nil {
		t.Fatal(err)
	}
	if !report.OK || report.Shareable || report.LocalQueue.Shareable || !report.Aggregate.Shareable {
		t.Fatalf("unexpected active-learning sharing model: %#v", report)
	}
	if report.Summary.QueuedCases != 2 || report.Summary.AlreadyLabeledCases != 1 || report.Summary.BelowThresholdCases != 1 {
		t.Fatalf("unexpected active-learning summary: %#v", report.Summary)
	}
	if report.LocalQueue.Items[0].LocalCaseID != "case-001" || report.LocalQueue.Items[1].LocalCaseID != "case-002" {
		t.Fatalf("queue was not ranked by uncertainty/information gain: %#v", report.LocalQueue.Items)
	}
	aggregateBytes, err := json.Marshal(report.Aggregate)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(aggregateBytes), "case-001") || strings.Contains(string(aggregateBytes), "local_case_id") {
		t.Fatalf("shareable aggregate leaked local examples:\n%s", aggregateBytes)
	}
	assertNoLiveLearningLeak(t, report.Aggregate)
	assertStableHash(t, report.Hash, func() (string, error) {
		next, err := ComputeActiveLearningQueue(spec)
		return next.Hash, err
	})
}

func TestActiveLearningReaderRejectsRawEvidence(t *testing.T) {
	raw := `{
	  "version":"patchline.adopter-active-learning/v1",
	  "claim":"bad raw queue",
	  "min_uncertainty_bp":1000,
	  "min_information_gain_bp":1000,
	  "max_queue_size":1,
	  "cases":[{"local_case_id":"case-001","detector":"sql.detector","release":"v1","confidence_bp":5000,"uncertainty_bp":5000,"expected_information_gain_bp":5000,"estimated_burden_minutes":1,"source_code":"DROP TABLE accounts;"}]
	}`
	if _, err := ReadActiveLearningSpec(strings.NewReader(raw)); err == nil {
		t.Fatal("expected active-learning spec with raw evidence to be rejected")
	}
}

func TestPolicyFreezePinsHighStakesIncidentToAuditedRelease(t *testing.T) {
	spec := policyFreezeSpec()
	report, err := ComputePolicyFreeze(spec)
	if err != nil {
		t.Fatal(err)
	}
	if !report.OK || report.Summary.PinnedOrganizations != 1 || report.Summary.AllowedUpdates != 1 {
		t.Fatalf("unexpected policy-freeze report: %#v", report)
	}
	critical := policyDecisionFor(t, report, "critical-payments")
	if critical.PolicyChangeAllowed || critical.PinnedRelease != "v1.0.0" || critical.Reason != "active_high_stakes_incident_pins_audited_release" {
		t.Fatalf("high-stakes incident was not frozen: %#v", critical)
	}
	platform := policyDecisionFor(t, report, "platform-sandbox")
	if !platform.PolicyChangeAllowed || platform.PinnedRelease != "v1.1.0" {
		t.Fatalf("non-incident update should be allowed: %#v", platform)
	}
	assertNoLiveLearningLeak(t, report)
}

func TestPolicyFreezeFailsClosedWhenAuditedReleaseMissing(t *testing.T) {
	spec := policyFreezeSpec()
	spec.AuditedReleases = spec.AuditedReleases[1:]
	report, err := ComputePolicyFreeze(spec)
	if err != nil {
		t.Fatal(err)
	}
	if report.OK || report.Summary.BlockedMissingAudit == 0 {
		t.Fatalf("missing high-stakes audit should fail closed: %#v", report)
	}
}

func TestCalibrationMonitorAlertsOnDecileDrift(t *testing.T) {
	report, err := Ingest(strings.NewReader(fixture), Options{})
	if err != nil {
		t.Fatal(err)
	}
	spec := CalibrationMonitorSpec{
		Version:                  CalibrationMonitorSpecVersion,
		Claim:                    "Live calibration monitoring compares published confidence deciles against observed reviewer confirmation rates and alerts only when pre-registered drift tolerances are exceeded.",
		PreRegisteredToleranceBP: 2000,
		MinEvidence:              3,
		AlertChannels:            []string{"artifact-review"},
	}
	monitor, err := ComputeCalibrationMonitor(report, spec)
	if err != nil {
		t.Fatal(err)
	}
	if !monitor.OK || monitor.Summary.DecilesEvaluated != 2 || monitor.Summary.Alerts != 1 {
		t.Fatalf("unexpected calibration monitor: %#v", monitor.Summary)
	}
	alert := monitor.Alerts[0]
	if alert.Detector != "orm.write-breadth" || alert.DriftBP <= spec.PreRegisteredToleranceBP || alert.Severity != "critical" {
		t.Fatalf("expected orm drift alert, got %#v", alert)
	}
	assertNoLiveLearningLeak(t, monitor)
}

func TestRetentionLifecycleChecksObservedActionsAgainstDeterministicAsOfDate(t *testing.T) {
	spec := retentionSpec()
	report, err := ComputeRetentionLifecycle(spec)
	if err != nil {
		t.Fatal(err)
	}
	if !report.OK || report.Summary.ArtifactsEvaluated != 3 || report.Summary.DeleteRequired != 1 || report.Summary.AnonymizeRequired != 1 || report.Summary.AggregateRetained != 1 {
		t.Fatalf("unexpected retention lifecycle report: %#v", report.Summary)
	}
	expired := retentionDecisionFor(t, report, "raw-old")
	if expired.ExpectedAction != "delete" || !expired.Compliant {
		t.Fatalf("expired raw artifact should require delete: %#v", expired)
	}
	assertNoLiveLearningLeak(t, report)
}

func TestRetentionLifecycleFlagsRetainedExpiredRawFeedback(t *testing.T) {
	spec := retentionSpec()
	spec.Artifacts[0].ObservedAction = "retain_local"
	report, err := ComputeRetentionLifecycle(spec)
	if err != nil {
		t.Fatal(err)
	}
	if report.OK || report.Summary.Violations != 1 {
		t.Fatalf("retained expired raw feedback should violate lifecycle: %#v", report)
	}
}

func TestTrustRegressionFailsOnFaithfulnessAndOverrelianceRegression(t *testing.T) {
	spec := trustSpec()
	report, err := ComputeTrustRegression(spec)
	if err != nil {
		t.Fatal(err)
	}
	if !report.OK || report.Summary.FailedChecks != 0 {
		t.Fatalf("expected trust regression to pass: %#v", report)
	}
	spec.Current.ExplanationFaithfulnessBP = 7800
	spec.Current.OverrelianceRateBP = 2100
	regressed, err := ComputeTrustRegression(spec)
	if err != nil {
		t.Fatal(err)
	}
	if regressed.OK || regressed.Summary.FailedChecks < 2 {
		t.Fatalf("expected trust regression failures: %#v", regressed)
	}
	assertNoLiveLearningLeak(t, report)
}

func TestMethodologyReportRequiresRecallGainWithoutOverrelianceIncrease(t *testing.T) {
	spec := methodologySpec()
	report, err := ComputeMethodologyReport(spec)
	if err != nil {
		t.Fatal(err)
	}
	if !report.OK || report.Summary.RecallImproved != 2 || report.Summary.OverrelianceNotIncreased != 2 || report.Summary.LinkedGateEvidence != 3 {
		t.Fatalf("unexpected methodology report: %#v", report)
	}
	spec.Experiments[0].LiveLearningOverrelianceBP = spec.Experiments[0].BaselineOverrelianceBP + 100
	regressed, err := ComputeMethodologyReport(spec)
	if err != nil {
		t.Fatal(err)
	}
	if regressed.OK {
		t.Fatalf("methodology should fail when over-reliance increases: %#v", regressed)
	}
	assertNoLiveLearningLeak(t, report)
}

func TestDetectorDeprecationPublishesTransparentReadyDecision(t *testing.T) {
	report, err := Ingest(strings.NewReader(fixture), Options{})
	if err != nil {
		t.Fatal(err)
	}
	spec := detectorDeprecationSpec()
	deprecation, err := ComputeDetectorDeprecation(report, spec)
	if err != nil {
		t.Fatal(err)
	}
	if !deprecation.OK || deprecation.Summary.ReadyToDeprecate != 1 || deprecation.Summary.Retained != 1 || deprecation.Summary.ProcessViolations != 0 {
		t.Fatalf("unexpected detector deprecation summary: %#v", deprecation.Summary)
	}
	orm := detectorDeprecationFor(t, deprecation, "orm.write-breadth")
	if orm.Status != "ready_to_deprecate" || orm.Metrics.PrecisionBP != 0 || orm.NoticeAgeDays < spec.MinNoticeDays {
		t.Fatalf("false-positive detector should be transparently ready to deprecate: %#v", orm)
	}
	sql := detectorDeprecationFor(t, deprecation, "sql.destructive-ddl")
	if sql.Status != "retained_thresholds_met" || sql.Metrics.PrecisionBP != 10000 {
		t.Fatalf("passing detector should be retained: %#v", sql)
	}
	assertNoLiveLearningLeak(t, deprecation)
	assertStableHash(t, deprecation.Hash, func() (string, error) {
		next, err := ComputeDetectorDeprecation(report, spec)
		return next.Hash, err
	})
	rehash := deprecation
	if got := hashLiveLearning(rehash); got != deprecation.Hash {
		t.Fatalf("hashLiveLearning should ignore populated hash field, got %q want %q", got, deprecation.Hash)
	}
}

func TestDetectorDeprecationBlocksMissingTransparentProcess(t *testing.T) {
	report, err := Ingest(strings.NewReader(fixture), Options{})
	if err != nil {
		t.Fatal(err)
	}
	spec := detectorDeprecationSpec()
	spec.Detectors[0].PublicNoticeID = ""
	spec.Detectors[0].NoticeOpenedAt = ""
	spec.Detectors[0].ReviewerRoles = []string{"maintainer"}
	spec.Detectors[0].PublicChannels = []string{"release-notes"}
	deprecation, err := ComputeDetectorDeprecation(report, spec)
	if err != nil {
		t.Fatal(err)
	}
	if deprecation.OK || deprecation.Summary.ProcessViolations != 1 {
		t.Fatalf("missing transparent process should fail report: %#v", deprecation.Summary)
	}
	orm := detectorDeprecationFor(t, deprecation, "orm.write-breadth")
	if orm.Status != "blocked_process_violation" || len(orm.ProcessFailures) < 3 || len(orm.MissingPublicChannels) != 1 {
		t.Fatalf("expected missing notice, reviewers, and channel failures: %#v", orm)
	}
}

func TestDetectorDeprecationKeepsFreshCompleteNoticeInReview(t *testing.T) {
	report, err := Ingest(strings.NewReader(fixture), Options{})
	if err != nil {
		t.Fatal(err)
	}
	spec := detectorDeprecationSpec()
	spec.Detectors[0].NoticeOpenedAt = "2026-06-10"
	deprecation, err := ComputeDetectorDeprecation(report, spec)
	if err != nil {
		t.Fatal(err)
	}
	orm := detectorDeprecationFor(t, deprecation, "orm.write-breadth")
	if !deprecation.OK || orm.Status != "notice_open_in_review" || orm.ProcessFailures != nil || orm.NoticeAgeDays >= spec.MinNoticeDays {
		t.Fatalf("complete fresh notice should remain in review without process violation: report=%#v detector=%#v", deprecation.Summary, orm)
	}
}

func TestDetectorDeprecationWaitsForAppealWindow(t *testing.T) {
	report, err := Ingest(strings.NewReader(fixture), Options{})
	if err != nil {
		t.Fatal(err)
	}
	spec := detectorDeprecationSpec()
	spec.MinNoticeDays = 7
	spec.Detectors[0].NoticeOpenedAt = "2026-06-01"
	spec.Detectors[0].AppealWindowDays = 21
	deprecation, err := ComputeDetectorDeprecation(report, spec)
	if err != nil {
		t.Fatal(err)
	}
	orm := detectorDeprecationFor(t, deprecation, "orm.write-breadth")
	if !deprecation.OK || orm.Status != "notice_open_in_review" || orm.NoticeAgeDays != 14 {
		t.Fatalf("notice should wait for longer appeal window before deprecation: report=%#v detector=%#v", deprecation.Summary, orm)
	}
	foundNoticeGate := false
	for _, gate := range orm.TransparencyGates {
		if gate.Name == "notice_age_days" && gate.Required != 21 {
			t.Fatalf("notice age should require appeal window, got %#v", gate)
		}
		if gate.Name == "notice_age_days" {
			foundNoticeGate = true
		}
	}
	if !foundNoticeGate {
		t.Fatalf("missing notice-age transparency gate: %#v", orm.TransparencyGates)
	}
}

func onlineDetectorFor(t *testing.T, report OnlineEvaluationReport, detector string) OnlineDetectorReport {
	t.Helper()
	for _, item := range report.Detectors {
		if item.Detector == detector {
			return item
		}
	}
	t.Fatalf("missing detector %s in %#v", detector, report.Detectors)
	return OnlineDetectorReport{}
}

func detectorDeprecationFor(t *testing.T, report DetectorDeprecationReport, detector string) DetectorDeprecationDecision {
	t.Helper()
	for _, item := range report.Detectors {
		if item.Detector == detector {
			return item
		}
	}
	t.Fatalf("missing detector %s in %#v", detector, report.Detectors)
	return DetectorDeprecationDecision{}
}

func policyFreezeSpec() PolicyFreezeSpec {
	return PolicyFreezeSpec{
		Version:  PolicyFreezeSpecVersion,
		Claim:    "During high-stakes incidents, Patchline pins organizations to audited detector releases and refuses proposed detector changes unless the current and proposed releases carry explicit audit evidence.",
		AsOfDate: "2026-06-02",
		AuditedReleases: []AuditedDetectorRelease{
			{Release: "v1.0.0", AuditHash: "audit-v1-0-0", ApprovedForHighStakes: true},
			{Release: "v1.1.0", AuditHash: "audit-v1-1-0", ApprovedForHighStakes: true},
		},
		Organizations: []PolicyFreezeOrganization{
			{Organization: "critical-payments", HighStakes: true, IncidentActive: true, IncidentID: "inc-2026-06", CurrentRelease: "v1.0.0", ProposedRelease: "v1.1.0"},
			{Organization: "platform-sandbox", CurrentRelease: "v1.0.0", ProposedRelease: "v1.1.0"},
		},
	}
}

func policyDecisionFor(t *testing.T, report PolicyFreezeReport, organization string) PolicyFreezeDecision {
	t.Helper()
	for _, decision := range report.Decisions {
		if decision.Organization == organization {
			return decision
		}
	}
	t.Fatalf("missing organization %s in %#v", organization, report.Decisions)
	return PolicyFreezeDecision{}
}

func retentionSpec() RetentionLifecycleSpec {
	return RetentionLifecycleSpec{
		Version:  RetentionLifecycleSpecVersion,
		Claim:    "Operational feedback must expire, anonymize, or aggregate according to deterministic lifecycle rules, with a fixed as-of date so reviewer outcomes are not retained beyond policy.",
		AsOfDate: "2026-06-02",
		Policies: []RetentionPolicy{{
			Class:                  "live-feedback",
			RawRetentionDays:       30,
			AnonymizeAfterDays:     14,
			AggregateRetentionDays: 365,
		}},
		Artifacts: []RetentionArtifact{
			{ArtifactID: "raw-old", Class: "live-feedback", CreatedAt: "2026-04-01", ContainsRawEvidence: true, ContainsLocalExamples: true, ObservedAction: "delete"},
			{ArtifactID: "raw-mid", Class: "live-feedback", CreatedAt: "2026-05-10", ContainsRawEvidence: true, ContainsLocalExamples: true, ObservedAction: "anonymize"},
			{ArtifactID: "agg-new", Class: "live-feedback", CreatedAt: "2026-05-15", Aggregated: true, Anonymized: true, ObservedAction: "retain_aggregate"},
		},
	}
}

func retentionDecisionFor(t *testing.T, report RetentionLifecycleReport, artifactID string) RetentionDecision {
	t.Helper()
	for _, decision := range report.Artifacts {
		if decision.ArtifactID == artifactID {
			return decision
		}
	}
	t.Fatalf("missing artifact %s in %#v", artifactID, report.Artifacts)
	return RetentionDecision{}
}

func trustSpec() TrustRegressionSpec {
	return TrustRegressionSpec{
		Version: TrustRegressionSpecVersion,
		Claim:   "Human-trust regression checks compare explanations before and after learned components update so faithfulness, citations, uncertainty, over-reliance, and burden cannot silently degrade.",
		Baseline: TrustMetrics{
			Release:                    "v1.0.0",
			ExplanationFaithfulnessBP:  9000,
			EvidenceCitationCoverageBP: 8800,
			UncertaintyDisclosureBP:    8500,
			OverrelianceRateBP:         1200,
			MeanReviewBurdenMinutes:    9,
		},
		Current: TrustMetrics{
			Release:                    "v1.1.0",
			ExplanationFaithfulnessBP:  8950,
			EvidenceCitationCoverageBP: 8800,
			UncertaintyDisclosureBP:    8600,
			OverrelianceRateBP:         1150,
			MeanReviewBurdenMinutes:    9,
		},
		Tolerances: TrustRegressionTolerance{
			MaxFaithfulnessDropBP:     100,
			MaxCitationCoverageDropBP: 100,
			MaxUncertaintyDropBP:      100,
			MaxOverrelianceIncreaseBP: 50,
			MaxBurdenIncreaseMinutes:  1,
		},
	}
}

func methodologySpec() MethodologySpec {
	return MethodologySpec{
		Version: MethodologySpecVersion,
		Claim:   "The live-learning methodology report ties shadow evaluation, calibration monitoring, local active learning, and human-trust regression to recall gains that do not increase reviewer over-reliance.",
		Experiments: []MethodologyExperiment{
			{Name: "shadow-detectors", Population: "public-slices", BaselineRecallBP: 7100, LiveLearningRecallBP: 7900, BaselineOverrelianceBP: 1300, LiveLearningOverrelianceBP: 1200, BaselineBurdenMinutes: 12, LiveLearningBurdenMinutes: 11, Evidence: []string{"safe-online-evaluation-gate", "live-calibration-monitor-gate"}},
			{Name: "local-queue", Population: "adopter-aggregates", BaselineRecallBP: 6900, LiveLearningRecallBP: 7600, BaselineOverrelianceBP: 1400, LiveLearningOverrelianceBP: 1300, BaselineBurdenMinutes: 10, LiveLearningBurdenMinutes: 9, Evidence: []string{"adopter-active-learning-gate", "human-trust-regression-gate"}},
		},
		GateEvidence: []MethodologyGateEvidence{
			{Gate: "safe-online-evaluation-gate", ReportHash: "hash-online", ReproductionCommand: "make safe-online-evaluation-gate"},
			{Gate: "live-calibration-monitor-gate", ReportHash: "hash-calibration", ReproductionCommand: "make live-calibration-monitor-gate"},
			{Gate: "human-trust-regression-gate", ReportHash: "hash-trust", ReproductionCommand: "make human-trust-regression-gate"},
		},
	}
}

func detectorDeprecationSpec() DetectorDeprecationSpec {
	return DetectorDeprecationSpec{
		Version:                 DetectorDeprecationSpecVersion,
		Claim:                   "Detector deprecation is transparent: Patchline evaluates source-free reviewer evidence against precision and burden thresholds, then requires public notice, independent review, appeal time, and migration guidance before retiring a detector.",
		AsOfDate:                "2026-06-15",
		MinEvidence:             3,
		MinPrecisionBP:          9000,
		MaxAverageBurdenMinutes: 12,
		MinNoticeDays:           30,
		MinReviewerRoles:        2,
		MinAppealWindowDays:     14,
		RequiredPublicChannels:  []string{"release-notes", "public-roadmap"},
		Detectors: []DetectorDeprecationTarget{
			{
				Detector:            "orm.write-breadth",
				Release:             "v1.0.0",
				Owner:               "detector-maintainers",
				PublicNoticeID:      "notice-orm-write-breadth-v1",
				NoticeOpenedAt:      "2026-05-01",
				PublicChannels:      []string{"release-notes", "public-roadmap"},
				ReviewerRoles:       []string{"maintainer", "dba"},
				ReplacementDetector: "sql.destructive-ddl",
				MigrationGuide:      "docs/detector-deprecation.md",
				AppealWindowDays:    21,
			},
			{
				Detector:            "sql.destructive-ddl",
				Release:             "v1.0.0",
				Owner:               "detector-maintainers",
				PublicNoticeID:      "notice-sql-destructive-v1",
				NoticeOpenedAt:      "2026-05-01",
				PublicChannels:      []string{"release-notes", "public-roadmap"},
				ReviewerRoles:       []string{"maintainer", "dba"},
				ReplacementDetector: "none",
				MigrationGuide:      "docs/detector-deprecation.md",
				AppealWindowDays:    21,
			},
		},
	}
}

func assertNoLiveLearningLeak(t *testing.T, value any) {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"DROP TABLE", "UPDATE accounts", "diff --git", "finding-001", "ev-001", "team-alpha", "local-secret-feedback-salt-2026", "source_code", "raw_evidence", "evidence_hash"} {
		if strings.Contains(string(data), forbidden) {
			t.Fatalf("live-learning report leaked %q:\n%s", forbidden, data)
		}
	}
}

func assertStableHash(t *testing.T, first string, next func() (string, error)) {
	t.Helper()
	second, err := next()
	if err != nil {
		t.Fatal(err)
	}
	if first == "" || first != second {
		t.Fatalf("expected stable hash, got first=%q second=%q", first, second)
	}
}

func TestLiveLearningCanonicalEncodingIsDeterministic(t *testing.T) {
	spec := methodologySpec()
	first, err := ComputeMethodologyReport(spec)
	if err != nil {
		t.Fatal(err)
	}
	second, err := ComputeMethodologyReport(spec)
	if err != nil {
		t.Fatal(err)
	}
	firstBytes, err := canonical.Bytes(first)
	if err != nil {
		t.Fatal(err)
	}
	secondBytes, err := canonical.Bytes(second)
	if err != nil {
		t.Fatal(err)
	}
	if string(firstBytes) != string(secondBytes) || first.Hash != second.Hash {
		t.Fatalf("expected deterministic methodology report:\n%s\n---\n%s", firstBytes, secondBytes)
	}
}
