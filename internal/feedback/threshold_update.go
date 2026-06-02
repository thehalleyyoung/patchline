package feedback

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/thehalleyyoung/patchline/internal/canonical"
)

const (
	ThresholdPolicyVersion  = "patchline.threshold-policy/v1"
	ThresholdGateVersion    = "patchline.threshold-policy-gate/v1"
	ThresholdUpdateVersion  = "patchline.threshold-update/v1"
	ThresholdUpdateGateName = "drift-threshold-update-gate"
)

type ThresholdPolicy struct {
	Version    string              `json:"version"`
	Name       string              `json:"name"`
	Thresholds []DetectorThreshold `json:"thresholds"`
}

type DetectorThreshold struct {
	Detector          string  `json:"detector"`
	BlockingThreshold float64 `json:"blocking_threshold"`
}

type ThresholdGateReceipt struct {
	Version                    string `json:"version"`
	Gate                       string `json:"gate"`
	OK                         bool   `json:"ok"`
	PolicyHash                 string `json:"policy_hash"`
	FeedbackHash               string `json:"feedback_hash"`
	AllowsBlockingPolicyChange bool   `json:"allows_blocking_policy_change"`
	Reviewer                   string `json:"reviewer,omitempty"`
	ReproductionCommand        string `json:"reproduction_command,omitempty"`
}

type ThresholdUpdateOptions struct {
	MinEvidence    int
	DriftTolerance float64
	MaxStep        float64
}

type ThresholdUpdateReport struct {
	Version               string                    `json:"version"`
	OK                    bool                      `json:"ok"`
	FeedbackHash          string                    `json:"feedback_hash"`
	PreviousFeedbackHash  string                    `json:"previous_feedback_hash,omitempty"`
	PolicyHash            string                    `json:"policy_hash"`
	EvidenceBasis         string                    `json:"evidence_basis"`
	Summary               ThresholdUpdateSummary    `json:"summary"`
	Gate                  ThresholdGateStatus       `json:"gate"`
	PolicyChangeAllowed   bool                      `json:"policy_change_allowed"`
	BlockingPolicyChanged bool                      `json:"blocking_policy_changed"`
	Recommendations       []ThresholdRecommendation `json:"recommendations"`
	CandidatePolicy       *ThresholdPolicy          `json:"candidate_policy,omitempty"`
	Warnings              []string                  `json:"warnings,omitempty"`
}

type ThresholdUpdateSummary struct {
	DetectorsEvaluated       int     `json:"detectors_evaluated"`
	Recommendations          int     `json:"recommendations"`
	CandidateChanges         int     `json:"candidate_changes"`
	BlockedWithoutGate       int     `json:"blocked_without_gate"`
	InsufficientEvidence     int     `json:"insufficient_evidence"`
	MinEvidence              int     `json:"min_evidence"`
	DriftTolerance           float64 `json:"drift_tolerance"`
	MaxStep                  float64 `json:"max_step"`
	PublishedFeedbackRecords int     `json:"published_feedback_records"`
}

type ThresholdGateStatus struct {
	Required                   bool   `json:"required"`
	Provided                   bool   `json:"provided"`
	OK                         bool   `json:"ok"`
	VersionOK                  bool   `json:"version_ok"`
	GateNamed                  bool   `json:"gate_named"`
	PolicyHashMatches          bool   `json:"policy_hash_matches"`
	FeedbackHashMatches        bool   `json:"feedback_hash_matches"`
	AllowsBlockingPolicyChange bool   `json:"allows_blocking_policy_change"`
	Reason                     string `json:"reason"`
}

type ThresholdRecommendation struct {
	ID                 string                 `json:"id"`
	Detector           string                 `json:"detector"`
	Releases           []string               `json:"releases"`
	CurrentThreshold   float64                `json:"current_threshold"`
	SuggestedThreshold float64                `json:"suggested_threshold"`
	Direction          string                 `json:"direction"`
	Reason             string                 `json:"reason"`
	ApplyStatus        string                 `json:"apply_status"`
	Evidence           ThresholdDriftEvidence `json:"evidence"`
}

type ThresholdDriftEvidence struct {
	PublishedCount        int      `json:"published_count"`
	MeanConfidence        float64  `json:"mean_confidence"`
	ExpectedConfirmedRate float64  `json:"expected_confirmed_rate"`
	ConfirmedRate         float64  `json:"confirmed_rate"`
	FalsePositiveRate     float64  `json:"false_positive_rate"`
	MissedRate            float64  `json:"missed_rate"`
	UncertainRate         float64  `json:"uncertain_rate"`
	PreviousConfirmedRate *float64 `json:"previous_confirmed_rate,omitempty"`
	ConfirmedRateDelta    *float64 `json:"confirmed_rate_delta,omitempty"`
	DriftMagnitude        float64  `json:"drift_magnitude"`
}

type detectorFeedbackStats struct {
	detector           string
	releases           map[string]struct{}
	count              int
	weightedConfidence float64
	confirmed          int
	falsePositive      int
	missed             int
	uncertain          int
}

func ReadReport(reader io.Reader) (Report, error) {
	decoder := json.NewDecoder(reader)
	decoder.DisallowUnknownFields()
	var report Report
	if err := decoder.Decode(&report); err != nil {
		return Report{}, err
	}
	if report.Version != ReportVersion {
		return Report{}, fmt.Errorf("live feedback report version must be %s", ReportVersion)
	}
	return report, nil
}

func ReadThresholdPolicy(reader io.Reader) (ThresholdPolicy, error) {
	decoder := json.NewDecoder(reader)
	decoder.DisallowUnknownFields()
	var policy ThresholdPolicy
	if err := decoder.Decode(&policy); err != nil {
		return ThresholdPolicy{}, err
	}
	if err := ValidateThresholdPolicy(policy); err != nil {
		return ThresholdPolicy{}, err
	}
	return policy, nil
}

func ReadThresholdGate(reader io.Reader) (ThresholdGateReceipt, error) {
	decoder := json.NewDecoder(reader)
	decoder.DisallowUnknownFields()
	var receipt ThresholdGateReceipt
	if err := decoder.Decode(&receipt); err != nil {
		return ThresholdGateReceipt{}, err
	}
	if receipt.Version != ThresholdGateVersion {
		return ThresholdGateReceipt{}, fmt.Errorf("threshold gate version must be %s", ThresholdGateVersion)
	}
	return receipt, nil
}

func ValidateThresholdPolicy(policy ThresholdPolicy) error {
	if policy.Version != ThresholdPolicyVersion {
		return fmt.Errorf("threshold policy version must be %s", ThresholdPolicyVersion)
	}
	if policy.Name == "" || !safeIdentifier(policy.Name) {
		return errors.New("threshold policy name must be a source-free identifier")
	}
	if len(policy.Thresholds) == 0 {
		return errors.New("threshold policy must include at least one detector threshold")
	}
	seen := map[string]struct{}{}
	for _, threshold := range policy.Thresholds {
		if threshold.Detector == "" || !safeIdentifier(threshold.Detector) || sourceLikeValue(threshold.Detector) {
			return errors.New("threshold detector must be a source-free identifier")
		}
		if threshold.BlockingThreshold < 0.01 || threshold.BlockingThreshold > 0.99 || math.IsNaN(threshold.BlockingThreshold) || math.IsInf(threshold.BlockingThreshold, 0) {
			return fmt.Errorf("threshold for %s must be between 0.01 and 0.99", threshold.Detector)
		}
		if _, ok := seen[threshold.Detector]; ok {
			return fmt.Errorf("duplicate threshold for detector %s", threshold.Detector)
		}
		seen[threshold.Detector] = struct{}{}
	}
	return nil
}

func ComputeThresholdUpdate(current Report, policy ThresholdPolicy, previous *Report, gate *ThresholdGateReceipt, opts ThresholdUpdateOptions) (ThresholdUpdateReport, error) {
	if current.Version != ReportVersion {
		return ThresholdUpdateReport{}, fmt.Errorf("live feedback report version must be %s", ReportVersion)
	}
	if err := ValidateThresholdPolicy(policy); err != nil {
		return ThresholdUpdateReport{}, err
	}
	if previous != nil && previous.Version != ReportVersion {
		return ThresholdUpdateReport{}, fmt.Errorf("previous live feedback report version must be %s", ReportVersion)
	}
	opts = normalizeThresholdOptions(opts)
	feedbackHash := canonical.Hash(current)
	policyHash := canonical.Hash(policy)
	report := ThresholdUpdateReport{
		Version:               ThresholdUpdateVersion,
		OK:                    true,
		FeedbackHash:          feedbackHash,
		PolicyHash:            policyHash,
		EvidenceBasis:         "published_k_anonymous_groups_only",
		PolicyChangeAllowed:   false,
		BlockingPolicyChanged: false,
		Gate:                  evaluateThresholdGate(gate, policyHash, feedbackHash),
		Summary: ThresholdUpdateSummary{
			MinEvidence:    opts.MinEvidence,
			DriftTolerance: round3(opts.DriftTolerance),
			MaxStep:        round3(opts.MaxStep),
		},
	}
	if previous != nil {
		report.PreviousFeedbackHash = canonical.Hash(*previous)
	}
	report.PolicyChangeAllowed = report.Gate.OK

	thresholds := thresholdMap(policy)
	currentStats, err := aggregateDetectorStats(current)
	if err != nil {
		return ThresholdUpdateReport{}, err
	}
	previousStats := map[string]detectorFeedbackStats{}
	if previous != nil {
		previousStats, err = aggregateDetectorStats(*previous)
		if err != nil {
			return ThresholdUpdateReport{}, err
		}
	}

	detectors := make([]string, 0, len(currentStats))
	for detector := range currentStats {
		detectors = append(detectors, detector)
	}
	sort.Strings(detectors)
	report.Summary.DetectorsEvaluated = len(detectors)

	for _, detector := range detectors {
		stats := currentStats[detector]
		report.Summary.PublishedFeedbackRecords += stats.count
		currentThreshold, ok := thresholds[detector]
		if !ok {
			report.Warnings = append(report.Warnings, "published feedback for detector "+detector+" has no threshold policy entry")
			continue
		}
		evidence := stats.thresholdEvidence(previousStats[detector], previous != nil)
		if stats.count < opts.MinEvidence {
			report.Summary.InsufficientEvidence++
			continue
		}
		recommendation, ok := buildThresholdRecommendation(detector, currentThreshold, stats, evidence, opts, report.PolicyChangeAllowed)
		if !ok {
			continue
		}
		report.Recommendations = append(report.Recommendations, recommendation)
	}

	report.Summary.Recommendations = len(report.Recommendations)
	if report.PolicyChangeAllowed && len(report.Recommendations) > 0 {
		candidate := applyRecommendations(policy, report.Recommendations)
		report.CandidatePolicy = &candidate
		report.Summary.CandidateChanges = len(report.Recommendations)
	} else {
		report.Summary.BlockedWithoutGate = len(report.Recommendations)
	}
	return report, nil
}

func normalizeThresholdOptions(opts ThresholdUpdateOptions) ThresholdUpdateOptions {
	if opts.MinEvidence <= 0 {
		opts.MinEvidence = MinKFloor
	}
	if opts.DriftTolerance <= 0 {
		opts.DriftTolerance = 0.20
	}
	if opts.MaxStep <= 0 {
		opts.MaxStep = 0.10
	}
	return opts
}

func thresholdMap(policy ThresholdPolicy) map[string]float64 {
	out := map[string]float64{}
	for _, threshold := range policy.Thresholds {
		out[threshold.Detector] = threshold.BlockingThreshold
	}
	return out
}

func aggregateDetectorStats(report Report) (map[string]detectorFeedbackStats, error) {
	out := map[string]detectorFeedbackStats{}
	for _, group := range report.Groups {
		midpoint, err := confidenceDecileMidpoint(group.ConfidenceDecile)
		if err != nil {
			return nil, err
		}
		stats := out[group.Detector]
		if stats.detector == "" {
			stats.detector = group.Detector
			stats.releases = map[string]struct{}{}
		}
		stats.releases[group.Release] = struct{}{}
		stats.count += group.Count
		stats.weightedConfidence += midpoint * float64(group.Count)
		switch group.Verdict {
		case "confirmed":
			stats.confirmed += group.Count
		case "false_positive":
			stats.falsePositive += group.Count
		case "missed":
			stats.missed += group.Count
		case "uncertain":
			stats.uncertain += group.Count
		}
		out[group.Detector] = stats
	}
	return out, nil
}

func confidenceDecileMidpoint(decile string) (float64, error) {
	low, high, err := confidenceDecileBounds(decile)
	if err != nil {
		return 0, err
	}
	return (low + high) / 2, nil
}

func confidenceDecileBounds(decile string) (float64, float64, error) {
	parts := strings.Split(decile, "-")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("invalid confidence decile %q", decile)
	}
	low, err := strconv.ParseFloat(parts[0], 64)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid confidence decile %q", decile)
	}
	high, err := strconv.ParseFloat(parts[1], 64)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid confidence decile %q", decile)
	}
	if low < 0 || high > 1 || low >= high {
		return 0, 0, fmt.Errorf("invalid confidence decile %q", decile)
	}
	return low, high, nil
}

func (stats detectorFeedbackStats) thresholdEvidence(previous detectorFeedbackStats, hasPrevious bool) ThresholdDriftEvidence {
	count := float64(stats.count)
	evidence := ThresholdDriftEvidence{
		PublishedCount:        stats.count,
		MeanConfidence:        round3(stats.weightedConfidence / count),
		ExpectedConfirmedRate: round3(stats.weightedConfidence / count),
		ConfirmedRate:         round3(float64(stats.confirmed) / count),
		FalsePositiveRate:     round3(float64(stats.falsePositive) / count),
		MissedRate:            round3(float64(stats.missed) / count),
		UncertainRate:         round3(float64(stats.uncertain) / count),
	}
	evidence.DriftMagnitude = round3(math.Abs(evidence.ConfirmedRate - evidence.ExpectedConfirmedRate))
	if hasPrevious && previous.count > 0 {
		previousRate := round3(float64(previous.confirmed) / float64(previous.count))
		delta := round3(evidence.ConfirmedRate - previousRate)
		evidence.PreviousConfirmedRate = &previousRate
		evidence.ConfirmedRateDelta = &delta
	}
	return evidence
}

func buildThresholdRecommendation(detector string, currentThreshold float64, stats detectorFeedbackStats, evidence ThresholdDriftEvidence, opts ThresholdUpdateOptions, policyChangeAllowed bool) (ThresholdRecommendation, bool) {
	direction := ""
	reason := ""
	step := math.Min(opts.MaxStep, 0.10)
	if step < 0.05 {
		step = 0.05
	}
	negativeDelta := false
	positiveDelta := false
	if evidence.ConfirmedRateDelta != nil {
		negativeDelta = *evidence.ConfirmedRateDelta <= -opts.DriftTolerance
		positiveDelta = *evidence.ConfirmedRateDelta >= opts.DriftTolerance
	}
	if evidence.FalsePositiveRate >= 0.50 || negativeDelta || (evidence.DriftMagnitude >= opts.DriftTolerance && evidence.ConfirmedRate < evidence.ExpectedConfirmedRate) {
		direction = "raise"
		reason = "published feedback drift shows more false positives or lower confirmation than calibrated confidence"
	} else if evidence.MissedRate >= 0.25 || positiveDelta || (evidence.ConfirmedRate >= 0.75 && evidence.MeanConfidence+0.05 < currentThreshold) {
		direction = "lower"
		reason = "published feedback drift shows missed or under-threshold confirmed hazards"
	}
	if direction == "" {
		return ThresholdRecommendation{}, false
	}
	suggested := currentThreshold
	if direction == "raise" {
		suggested = clampThreshold(currentThreshold + step)
	} else {
		suggested = clampThreshold(currentThreshold - step)
	}
	if suggested == currentThreshold {
		return ThresholdRecommendation{}, false
	}
	applyStatus := "blocked_without_gate"
	if policyChangeAllowed {
		applyStatus = "candidate_requires_review"
	}
	releases := stringSet(stats.releases)
	return ThresholdRecommendation{
		ID:                 thresholdRecommendationID(detector, currentThreshold, suggested),
		Detector:           detector,
		Releases:           releases,
		CurrentThreshold:   round2(currentThreshold),
		SuggestedThreshold: round2(suggested),
		Direction:          direction,
		Reason:             reason,
		ApplyStatus:        applyStatus,
		Evidence:           evidence,
	}, true
}

func thresholdRecommendationID(detector string, current, suggested float64) string {
	return "threshold-" + canonical.Hash(struct {
		Detector  string  `json:"detector"`
		Current   float64 `json:"current"`
		Suggested float64 `json:"suggested"`
	}{Detector: detector, Current: round2(current), Suggested: round2(suggested)})[:12]
}

func evaluateThresholdGate(gate *ThresholdGateReceipt, policyHash, feedbackHash string) ThresholdGateStatus {
	status := ThresholdGateStatus{Required: true}
	if gate == nil {
		status.Reason = "missing_policy_gate"
		return status
	}
	status.Provided = true
	status.VersionOK = gate.Version == ThresholdGateVersion
	status.GateNamed = gate.Gate == ThresholdUpdateGateName
	status.PolicyHashMatches = gate.PolicyHash == policyHash
	status.FeedbackHashMatches = gate.FeedbackHash == feedbackHash
	status.AllowsBlockingPolicyChange = gate.AllowsBlockingPolicyChange
	status.OK = gate.OK && status.VersionOK && status.GateNamed && status.PolicyHashMatches && status.FeedbackHashMatches && status.AllowsBlockingPolicyChange
	if status.OK {
		status.Reason = "gate_bound_to_feedback_and_policy"
	} else {
		status.Reason = "gate_not_bound_to_current_feedback_and_policy"
	}
	return status
}

func applyRecommendations(policy ThresholdPolicy, recommendations []ThresholdRecommendation) ThresholdPolicy {
	candidate := ThresholdPolicy{
		Version:    policy.Version,
		Name:       policy.Name + "-candidate",
		Thresholds: append([]DetectorThreshold(nil), policy.Thresholds...),
	}
	suggested := map[string]float64{}
	for _, recommendation := range recommendations {
		suggested[recommendation.Detector] = recommendation.SuggestedThreshold
	}
	for i := range candidate.Thresholds {
		if next, ok := suggested[candidate.Thresholds[i].Detector]; ok {
			candidate.Thresholds[i].BlockingThreshold = next
		}
	}
	sort.Slice(candidate.Thresholds, func(i, j int) bool {
		return candidate.Thresholds[i].Detector < candidate.Thresholds[j].Detector
	})
	return candidate
}

func stringSet(values map[string]struct{}) []string {
	out := make([]string, 0, len(values))
	for value := range values {
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func clampThreshold(value float64) float64 {
	if value < 0.01 {
		return 0.01
	}
	if value > 0.99 {
		return 0.99
	}
	return round2(value)
}

func round2(value float64) float64 {
	return math.Round(value*100) / 100
}

func round3(value float64) float64 {
	return math.Round(value*1000) / 1000
}
