package feedback

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"

	"github.com/thehalleyyoung/patchline/internal/canonical"
)

const (
	CounterfactualPolicyHistoryVersion = "patchline.counterfactual-policy-history/v1"
	CounterfactualLogVersion           = "patchline.reviewer-counterfactual-log/v1"
)

type CounterfactualPolicyHistory struct {
	Version  string                         `json:"version"`
	Name     string                         `json:"name"`
	Claim    string                         `json:"claim,omitempty"`
	Policies []CounterfactualPolicySnapshot `json:"policies"`
}

type CounterfactualPolicySnapshot struct {
	Release    string              `json:"release"`
	Thresholds []DetectorThreshold `json:"thresholds"`
}

type CounterfactualLog struct {
	Version           string                         `json:"version"`
	OK                bool                           `json:"ok"`
	FeedbackHash      string                         `json:"feedback_hash"`
	PolicyHistoryHash string                         `json:"policy_history_hash"`
	Hash              string                         `json:"hash"`
	EvidenceBasis     string                         `json:"evidence_basis"`
	ReleaseOrdering   string                         `json:"release_ordering"`
	Recommendation    CounterfactualRecommendation   `json:"recommendation_model"`
	Privacy           CounterfactualPrivacy          `json:"privacy"`
	Summary           CounterfactualSummary          `json:"summary"`
	Releases          []CounterfactualReleaseSummary `json:"releases"`
	Entries           []CounterfactualEntry          `json:"entries"`
	Warnings          []string                       `json:"warnings,omitempty"`
}

type CounterfactualRecommendation struct {
	Kind              string `json:"kind"`
	Comparator        string `json:"comparator"`
	BoundaryTreatment string `json:"boundary_treatment"`
	MissedTreatment   string `json:"missed_treatment"`
}

type CounterfactualPrivacy struct {
	SourceFree              bool     `json:"source_free"`
	RawEvidenceFree         bool     `json:"raw_values_free"`
	IdentifierFree          bool     `json:"identifier_free"`
	SaltEmitted             bool     `json:"salt_emitted"`
	IndividualOutcomesFree  bool     `json:"individual_outcomes_free"`
	AllowedInputEvidence    []string `json:"allowed_input_evidence"`
	ExcludedInputEvidence   []string `json:"excluded_input_evidence"`
	CounterfactualUnit      string   `json:"counterfactual_unit"`
	ConfidenceResolution    string   `json:"confidence_resolution"`
	PreviousReleaseSelector string   `json:"previous_release_selector"`
}

type CounterfactualSummary struct {
	PublishedGroups              int `json:"published_groups"`
	PublishedFeedbackRecords     int `json:"published_feedback_records"`
	ResidualExcludedRecords      int `json:"residual_excluded_records"`
	PolicyReleases               int `json:"policy_releases"`
	PreviousPolicyReleasesUsed   int `json:"previous_policy_releases_used"`
	GroupsWithoutReleasePolicy   int `json:"groups_without_release_policy"`
	GroupsWithoutPreviousPolicy  int `json:"groups_without_previous_policy"`
	CounterfactualGroupsCompared int `json:"counterfactual_groups_compared"`
	CounterfactualCounters
}

type CounterfactualReleaseSummary struct {
	PolicyRelease string `json:"policy_release"`
	CounterfactualCounters
}

type CounterfactualCounters struct {
	ComparedRecords                int `json:"compared_records"`
	WouldBlock                     int `json:"would_block"`
	WouldAllow                     int `json:"would_allow"`
	BoundaryAmbiguous              int `json:"boundary_ambiguous"`
	NoPolicy                       int `json:"no_policy"`
	ConfirmedWouldBlock            int `json:"confirmed_would_block"`
	ConfirmedWouldMiss             int `json:"confirmed_would_miss"`
	ConfirmedBoundaryAmbiguous     int `json:"confirmed_boundary_ambiguous"`
	FalsePositiveWouldBlock        int `json:"false_positive_would_block"`
	FalsePositiveWouldSpare        int `json:"false_positive_would_spare"`
	FalsePositiveBoundaryAmbiguous int `json:"false_positive_boundary_ambiguous"`
	UncertainWouldBlock            int `json:"uncertain_would_block"`
	UncertainWouldAllow            int `json:"uncertain_would_allow"`
	UncertainBoundaryAmbiguous     int `json:"uncertain_boundary_ambiguous"`
	MissedNotEmitted               int `json:"missed_not_emitted"`
}

type CounterfactualEntry struct {
	PolicyRelease          string  `json:"policy_release"`
	ObservedRelease        string  `json:"observed_release"`
	Detector               string  `json:"detector"`
	ConfidenceDecile       string  `json:"confidence_decile"`
	Verdict                string  `json:"verdict"`
	ReviewerAction         string  `json:"reviewer_action"`
	Count                  int     `json:"count"`
	Threshold              float64 `json:"threshold,omitempty"`
	PreviousRecommendation string  `json:"previous_recommendation"`
	Classification         string  `json:"classification"`
}

func ReadCounterfactualPolicyHistory(reader io.Reader) (CounterfactualPolicyHistory, error) {
	content, err := io.ReadAll(reader)
	if err != nil {
		return CounterfactualPolicyHistory{}, err
	}
	trimmed := bytes.TrimSpace(content)
	if len(trimmed) == 0 {
		return CounterfactualPolicyHistory{}, errors.New("counterfactual policy history input is empty")
	}
	if containsBlockedKey(json.RawMessage(trimmed)) {
		return CounterfactualPolicyHistory{}, errors.New("counterfactual policy history contains blocked raw-evidence fields")
	}
	decoder := json.NewDecoder(bytes.NewReader(trimmed))
	decoder.DisallowUnknownFields()
	var history CounterfactualPolicyHistory
	if err := decoder.Decode(&history); err != nil {
		return CounterfactualPolicyHistory{}, err
	}
	if err := ValidateCounterfactualPolicyHistory(history); err != nil {
		return CounterfactualPolicyHistory{}, err
	}
	return history, nil
}

func ValidateCounterfactualPolicyHistory(history CounterfactualPolicyHistory) error {
	if history.Version != CounterfactualPolicyHistoryVersion {
		return fmt.Errorf("counterfactual policy history version must be %s", CounterfactualPolicyHistoryVersion)
	}
	if history.Name == "" || !safeIdentifier(history.Name) || sourceLikeValue(history.Name) {
		return errors.New("counterfactual policy history name must be a source-free identifier")
	}
	if history.Claim != "" && sourceLikeValue(history.Claim) {
		return errors.New("counterfactual policy history claim must not contain source-like values")
	}
	if len(history.Policies) < 2 {
		return errors.New("counterfactual policy history must include at least two ordered releases")
	}
	seenReleases := map[string]struct{}{}
	for _, snapshot := range history.Policies {
		if snapshot.Release == "" || !safeIdentifier(snapshot.Release) || sourceLikeValue(snapshot.Release) {
			return errors.New("counterfactual policy release must be a source-free identifier")
		}
		if _, ok := seenReleases[snapshot.Release]; ok {
			return fmt.Errorf("duplicate counterfactual policy release %s", snapshot.Release)
		}
		seenReleases[snapshot.Release] = struct{}{}
		policy := ThresholdPolicy{
			Version:    ThresholdPolicyVersion,
			Name:       "counterfactual-" + snapshot.Release,
			Thresholds: snapshot.Thresholds,
		}
		if err := ValidateThresholdPolicy(policy); err != nil {
			return fmt.Errorf("release %s: %w", snapshot.Release, err)
		}
	}
	return nil
}

func ComputeCounterfactualLog(report Report, history CounterfactualPolicyHistory) (CounterfactualLog, error) {
	if report.Version != ReportVersion {
		return CounterfactualLog{}, fmt.Errorf("live feedback report version must be %s", ReportVersion)
	}
	if err := ValidateCounterfactualPolicyHistory(history); err != nil {
		return CounterfactualLog{}, err
	}

	releaseIndex := map[string]int{}
	thresholdsByRelease := make([]map[string]float64, len(history.Policies))
	for index, snapshot := range history.Policies {
		releaseIndex[snapshot.Release] = index
		thresholdsByRelease[index] = thresholdMap(ThresholdPolicy{Thresholds: snapshot.Thresholds})
	}

	log := CounterfactualLog{
		Version:           CounterfactualLogVersion,
		OK:                true,
		FeedbackHash:      canonical.Hash(report),
		PolicyHistoryHash: canonical.Hash(history),
		EvidenceBasis:     "published_k_anonymous_groups_only",
		ReleaseOrdering:   "input_order_oldest_to_newest",
		Recommendation: CounterfactualRecommendation{
			Kind:              "threshold_reconstruction",
			Comparator:        "confidence_decile compared to historical detector blocking_threshold using >= when the whole decile is above threshold",
			BoundaryTreatment: "boundary_ambiguous when a threshold falls inside a published confidence decile",
			MissedTreatment:   "missed findings are detector non-emissions and are never reclassified by threshold",
		},
		Privacy: CounterfactualPrivacy{
			SourceFree:             true,
			RawEvidenceFree:        true,
			IdentifierFree:         true,
			SaltEmitted:            false,
			IndividualOutcomesFree: true,
			AllowedInputEvidence: []string{
				"detector", "release", "confidence_decile", "verdict", "action", "count", "total_burden_minutes",
			},
			ExcludedInputEvidence: []string{
				"source code", "diffs", "file paths", "finding IDs", "evidence hashes", "adopter IDs", "salts", "raw confidences",
			},
			CounterfactualUnit:      "published k-anonymous detector/release/confidence/verdict/action group",
			ConfidenceResolution:    "decile",
			PreviousReleaseSelector: "only policy snapshots before the observed group release in the declared release order",
		},
		Summary: CounterfactualSummary{
			PublishedGroups:         len(report.Groups),
			PolicyReleases:          len(history.Policies),
			ResidualExcludedRecords: report.Residual.Count,
		},
	}

	releaseSummaries := make([]CounterfactualReleaseSummary, len(history.Policies))
	for index, snapshot := range history.Policies {
		releaseSummaries[index].PolicyRelease = snapshot.Release
	}

	groups := append([]Group(nil), report.Groups...)
	sort.Slice(groups, func(i, j int) bool { return groupValueLess(groups[i], groups[j]) })
	usedReleases := map[string]struct{}{}
	for _, group := range groups {
		if group.Count <= 0 {
			return CounterfactualLog{}, fmt.Errorf("feedback group for detector %s has non-positive count", group.Detector)
		}
		if _, _, err := confidenceDecileBounds(group.ConfidenceDecile); err != nil {
			return CounterfactualLog{}, err
		}
		log.Summary.PublishedFeedbackRecords += group.Count
		observedIndex, ok := releaseIndex[group.Release]
		if !ok {
			log.Summary.GroupsWithoutReleasePolicy++
			log.Warnings = append(log.Warnings, "feedback group release "+group.Release+" is absent from counterfactual policy history")
			continue
		}
		if observedIndex == 0 {
			log.Summary.GroupsWithoutPreviousPolicy++
			log.Warnings = append(log.Warnings, "feedback group release "+group.Release+" has no previous policy snapshots")
			continue
		}
		for policyIndex := 0; policyIndex < observedIndex; policyIndex++ {
			entry, err := buildCounterfactualEntry(group, history.Policies[policyIndex].Release, thresholdsByRelease[policyIndex])
			if err != nil {
				return CounterfactualLog{}, err
			}
			log.Entries = append(log.Entries, entry)
			log.Summary.CounterfactualGroupsCompared++
			applyCounterfactualCounters(&log.Summary.CounterfactualCounters, entry)
			applyCounterfactualCounters(&releaseSummaries[policyIndex].CounterfactualCounters, entry)
			usedReleases[history.Policies[policyIndex].Release] = struct{}{}
		}
	}
	for _, release := range releaseSummaries {
		if release.ComparedRecords > 0 {
			log.Releases = append(log.Releases, release)
		}
	}
	log.Summary.PreviousPolicyReleasesUsed = len(usedReleases)
	hashable := log
	hashable.Hash = ""
	log.Hash = canonical.Hash(hashable)
	return log, nil
}

func buildCounterfactualEntry(group Group, policyRelease string, thresholds map[string]float64) (CounterfactualEntry, error) {
	entry := CounterfactualEntry{
		PolicyRelease:    policyRelease,
		ObservedRelease:  group.Release,
		Detector:         group.Detector,
		ConfidenceDecile: group.ConfidenceDecile,
		Verdict:          group.Verdict,
		ReviewerAction:   group.Action,
		Count:            group.Count,
	}
	threshold, ok := thresholds[group.Detector]
	if !ok {
		entry.PreviousRecommendation = "no_policy"
		entry.Classification = "no_policy"
		return entry, nil
	}
	entry.Threshold = round2(threshold)
	if group.Verdict == "missed" {
		entry.PreviousRecommendation = "not_emitted"
		entry.Classification = "would_still_be_missed"
		return entry, nil
	}
	low, high, err := confidenceDecileBounds(group.ConfidenceDecile)
	if err != nil {
		return CounterfactualEntry{}, err
	}
	switch {
	case threshold <= low:
		entry.PreviousRecommendation = "block"
	case threshold >= high:
		entry.PreviousRecommendation = "allow"
	default:
		entry.PreviousRecommendation = "boundary_ambiguous"
		entry.Classification = "boundary_ambiguous"
		return entry, nil
	}
	entry.Classification = counterfactualClassification(group.Verdict, entry.PreviousRecommendation)
	return entry, nil
}

func counterfactualClassification(verdict, recommendation string) string {
	switch verdict {
	case "confirmed":
		if recommendation == "block" {
			return "would_block_confirmed"
		}
		return "would_miss_confirmed"
	case "false_positive":
		if recommendation == "block" {
			return "would_block_false_positive"
		}
		return "would_spare_false_positive"
	case "uncertain":
		if recommendation == "block" {
			return "would_block_uncertain"
		}
		return "would_allow_uncertain"
	default:
		return "unclassified"
	}
}

func applyCounterfactualCounters(counters *CounterfactualCounters, entry CounterfactualEntry) {
	counters.ComparedRecords += entry.Count
	switch entry.PreviousRecommendation {
	case "block":
		counters.WouldBlock += entry.Count
	case "allow":
		counters.WouldAllow += entry.Count
	case "boundary_ambiguous":
		counters.BoundaryAmbiguous += entry.Count
	case "no_policy":
		counters.NoPolicy += entry.Count
	}
	switch entry.Classification {
	case "would_block_confirmed":
		counters.ConfirmedWouldBlock += entry.Count
	case "would_miss_confirmed":
		counters.ConfirmedWouldMiss += entry.Count
	case "would_block_false_positive":
		counters.FalsePositiveWouldBlock += entry.Count
	case "would_spare_false_positive":
		counters.FalsePositiveWouldSpare += entry.Count
	case "would_block_uncertain":
		counters.UncertainWouldBlock += entry.Count
	case "would_allow_uncertain":
		counters.UncertainWouldAllow += entry.Count
	case "would_still_be_missed":
		counters.MissedNotEmitted += entry.Count
	case "boundary_ambiguous":
		switch entry.Verdict {
		case "confirmed":
			counters.ConfirmedBoundaryAmbiguous += entry.Count
		case "false_positive":
			counters.FalsePositiveBoundaryAmbiguous += entry.Count
		case "uncertain":
			counters.UncertainBoundaryAmbiguous += entry.Count
		}
	}
}

func groupValueLess(left, right Group) bool {
	if left.Detector != right.Detector {
		return left.Detector < right.Detector
	}
	if left.Release != right.Release {
		return left.Release < right.Release
	}
	if left.ConfidenceDecile != right.ConfidenceDecile {
		return left.ConfidenceDecile < right.ConfidenceDecile
	}
	if left.Verdict != right.Verdict {
		return left.Verdict < right.Verdict
	}
	return left.Action < right.Action
}
