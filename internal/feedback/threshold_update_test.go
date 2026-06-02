package feedback

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestThresholdUpdateSuggestsButBlocksWithoutBoundGate(t *testing.T) {
	report, err := Ingest(strings.NewReader(fixture), Options{})
	if err != nil {
		t.Fatal(err)
	}
	policy := thresholdTestPolicy()
	update, err := ComputeThresholdUpdate(report, policy, nil, nil, ThresholdUpdateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if update.PolicyChangeAllowed || update.BlockingPolicyChanged || update.CandidatePolicy != nil {
		t.Fatalf("expected advisory-only update without candidate policy: %#v", update)
	}
	if update.FeedbackHash == "" || update.PolicyHash == "" || update.EvidenceBasis != "published_k_anonymous_groups_only" {
		t.Fatalf("missing bound hashes or evidence basis: %#v", update)
	}
	if !update.Gate.Required || update.Gate.Provided || update.Gate.Reason != "missing_policy_gate" {
		t.Fatalf("expected missing gate status, got %#v", update.Gate)
	}
	orm := thresholdRecommendationFor(t, update, "orm.write-breadth")
	if orm.Direction != "raise" || orm.ApplyStatus != "blocked_without_gate" || orm.SuggestedThreshold <= orm.CurrentThreshold {
		t.Fatalf("expected blocked raise recommendation, got %#v", orm)
	}
	if update.Summary.BlockedWithoutGate != len(update.Recommendations) || update.Summary.CandidateChanges != 0 {
		t.Fatalf("expected all recommendations blocked without gate: %#v", update.Summary)
	}
	serialized, err := json.Marshal(update)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"finding-001", "ev-001", "local-secret-feedback-salt-2026", "DROP TABLE", "UPDATE accounts"} {
		if strings.Contains(string(serialized), forbidden) {
			t.Fatalf("threshold update leaked source/raw identifier %q:\n%s", forbidden, serialized)
		}
	}
}

func TestThresholdUpdateRequiresGateBoundToPolicyAndFeedback(t *testing.T) {
	report, err := Ingest(strings.NewReader(fixture), Options{})
	if err != nil {
		t.Fatal(err)
	}
	policy := thresholdTestPolicy()
	noGate, err := ComputeThresholdUpdate(report, policy, nil, nil, ThresholdUpdateOptions{})
	if err != nil {
		t.Fatal(err)
	}

	staleGate := ThresholdGateReceipt{
		Version:                    ThresholdGateVersion,
		Gate:                       "drift-threshold-update-gate",
		OK:                         true,
		PolicyHash:                 "stale-policy",
		FeedbackHash:               noGate.FeedbackHash,
		AllowsBlockingPolicyChange: true,
	}
	staleUpdate, err := ComputeThresholdUpdate(report, policy, nil, &staleGate, ThresholdUpdateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if staleUpdate.PolicyChangeAllowed || staleUpdate.CandidatePolicy != nil || staleUpdate.Gate.PolicyHashMatches {
		t.Fatalf("stale gate should not allow candidate policy: %#v", staleUpdate.Gate)
	}

	wrongGate := ThresholdGateReceipt{
		Version:                    ThresholdGateVersion,
		Gate:                       "unrelated-gate",
		OK:                         true,
		PolicyHash:                 noGate.PolicyHash,
		FeedbackHash:               noGate.FeedbackHash,
		AllowsBlockingPolicyChange: true,
	}
	wrongGateUpdate, err := ComputeThresholdUpdate(report, policy, nil, &wrongGate, ThresholdUpdateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if wrongGateUpdate.PolicyChangeAllowed || wrongGateUpdate.CandidatePolicy != nil || wrongGateUpdate.Gate.GateNamed {
		t.Fatalf("wrong gate name should not allow candidate policy: %#v", wrongGateUpdate.Gate)
	}

	validGate := ThresholdGateReceipt{
		Version:                    ThresholdGateVersion,
		Gate:                       ThresholdUpdateGateName,
		OK:                         true,
		PolicyHash:                 noGate.PolicyHash,
		FeedbackHash:               noGate.FeedbackHash,
		AllowsBlockingPolicyChange: true,
	}
	gatedUpdate, err := ComputeThresholdUpdate(report, policy, nil, &validGate, ThresholdUpdateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if !gatedUpdate.PolicyChangeAllowed || gatedUpdate.BlockingPolicyChanged || gatedUpdate.CandidatePolicy == nil {
		t.Fatalf("expected separate candidate policy with unchanged blocking policy: %#v", gatedUpdate)
	}
	if !gatedUpdate.Gate.OK || !gatedUpdate.Gate.PolicyHashMatches || !gatedUpdate.Gate.FeedbackHashMatches {
		t.Fatalf("expected fully bound gate: %#v", gatedUpdate.Gate)
	}
	orm := thresholdRecommendationFor(t, gatedUpdate, "orm.write-breadth")
	if orm.ApplyStatus != "candidate_requires_review" {
		t.Fatalf("expected gated candidate status, got %#v", orm)
	}
	if policy.Thresholds[1].BlockingThreshold != 0.70 {
		t.Fatalf("input policy was mutated: %#v", policy)
	}
	candidate := thresholdByDetector(*gatedUpdate.CandidatePolicy, "orm.write-breadth")
	if candidate.BlockingThreshold != 0.80 {
		t.Fatalf("expected candidate threshold raised to 0.80, got %#v", candidate)
	}
}

func TestThresholdUpdateUsesPreviousFeedbackDrift(t *testing.T) {
	current := Report{
		Version: ReportVersion,
		Groups: []Group{{
			Detector:         "migration.guard",
			Release:          "v2.0.0",
			ConfidenceDecile: "0.5-0.6",
			Verdict:          "confirmed",
			Action:           "blocked",
			Count:            3,
		}},
	}
	previous := Report{
		Version: ReportVersion,
		Groups: []Group{{
			Detector:         "migration.guard",
			Release:          "v1.9.0",
			ConfidenceDecile: "0.5-0.6",
			Verdict:          "false_positive",
			Action:           "dismissed",
			Count:            3,
		}},
	}
	policy := ThresholdPolicy{
		Version: ThresholdPolicyVersion,
		Name:    "stage63-policy",
		Thresholds: []DetectorThreshold{{
			Detector:          "migration.guard",
			BlockingThreshold: 0.90,
		}},
	}
	update, err := ComputeThresholdUpdate(current, policy, &previous, nil, ThresholdUpdateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	rec := thresholdRecommendationFor(t, update, "migration.guard")
	if rec.Direction != "lower" || rec.SuggestedThreshold >= rec.CurrentThreshold {
		t.Fatalf("expected previous-feedback drift to lower threshold, got %#v", rec)
	}
	if rec.Evidence.PreviousConfirmedRate == nil || rec.Evidence.ConfirmedRateDelta == nil || *rec.Evidence.ConfirmedRateDelta <= 0 {
		t.Fatalf("expected previous drift evidence, got %#v", rec.Evidence)
	}
}

func thresholdTestPolicy() ThresholdPolicy {
	return ThresholdPolicy{
		Version: ThresholdPolicyVersion,
		Name:    "stage63-policy",
		Thresholds: []DetectorThreshold{
			{Detector: "sql.destructive-ddl", BlockingThreshold: 0.90},
			{Detector: "orm.write-breadth", BlockingThreshold: 0.70},
			{Detector: "migration.guard", BlockingThreshold: 0.50},
		},
	}
}

func thresholdRecommendationFor(t *testing.T, update ThresholdUpdateReport, detector string) ThresholdRecommendation {
	t.Helper()
	for _, recommendation := range update.Recommendations {
		if recommendation.Detector == detector {
			return recommendation
		}
	}
	t.Fatalf("missing recommendation for %s in %#v", detector, update.Recommendations)
	return ThresholdRecommendation{}
}

func thresholdByDetector(policy ThresholdPolicy, detector string) DetectorThreshold {
	for _, threshold := range policy.Thresholds {
		if threshold.Detector == detector {
			return threshold
		}
	}
	return DetectorThreshold{}
}
