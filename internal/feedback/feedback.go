package feedback

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"regexp"
	"sort"
	"strings"

	"github.com/thehalleyyoung/patchline/internal/canonical"
)

const (
	InputVersion  = "patchline.live-feedback-ingestion/v1"
	ReportVersion = "patchline.live-feedback/v1"
	MinSaltLength = 16
	MinKFloor     = 3
)

type Options struct {
	DefaultMinGroupSize int
}

type Report struct {
	Version       string           `json:"version"`
	OK            bool             `json:"ok"`
	Shareable     bool             `json:"shareable"`
	AdopterCohort string           `json:"adopter_cohort"`
	AcceptedHash  string           `json:"accepted_hash"`
	Summary       Summary          `json:"summary"`
	Privacy       PrivacySummary   `json:"privacy"`
	Groups        []Group          `json:"groups"`
	Residual      Residual         `json:"residual"`
	Rejected      []RejectedRecord `json:"rejected,omitempty"`
	Warnings      []string         `json:"warnings,omitempty"`
}

type Summary struct {
	InputRecords          int `json:"input_records"`
	AcceptedRecords       int `json:"accepted_records"`
	RejectedRecords       int `json:"rejected_records"`
	DeduplicatedRecords   int `json:"deduplicated_records"`
	GroupsPublished       int `json:"groups_published"`
	GroupsSuppressed      int `json:"groups_suppressed"`
	ResidualRecords       int `json:"residual_records"`
	TotalBurdenMinutes    int `json:"total_burden_minutes"`
	RequestedMinGroupSize int `json:"requested_min_group_size"`
	EffectiveMinGroupSize int `json:"effective_min_group_size"`
}

type PrivacySummary struct {
	SourceFree          bool     `json:"source_free"`
	RawEvidenceFree     bool     `json:"raw_values_free"`
	IdentifierFree      bool     `json:"identifier_free"`
	SaltEmitted         bool     `json:"salt_emitted"`
	UnknownFieldsStored bool     `json:"unknown_fields_stored"`
	AllowedFields       []string `json:"allowed_fields"`
	SuppressionRule     string   `json:"suppression_rule"`
}

type Group struct {
	Detector           string `json:"detector"`
	Release            string `json:"release"`
	ConfidenceDecile   string `json:"confidence_decile"`
	Verdict            string `json:"verdict"`
	Action             string `json:"action"`
	Count              int    `json:"count"`
	TotalBurdenMinutes int    `json:"total_burden_minutes"`
}

type Residual struct {
	Count              int            `json:"count"`
	TotalBurdenMinutes int            `json:"total_burden_minutes"`
	OutcomeCounts      []OutcomeCount `json:"outcome_counts,omitempty"`
}

type OutcomeCount struct {
	Verdict string `json:"verdict"`
	Action  string `json:"action"`
	Count   int    `json:"count"`
}

type RejectedRecord struct {
	Index  int    `json:"index"`
	Reason string `json:"reason"`
}

type inputFile struct {
	Version      string            `json:"version"`
	AdopterID    string            `json:"adopter_id"`
	Salt         string            `json:"salt"`
	MinGroupSize int               `json:"min_group_size"`
	Outcomes     []json.RawMessage `json:"outcomes"`
}

type outcome struct {
	FindingID     string  `json:"finding_id"`
	Detector      string  `json:"detector"`
	Release       string  `json:"release"`
	Confidence    float64 `json:"confidence"`
	Verdict       string  `json:"verdict"`
	Action        string  `json:"action"`
	BurdenMinutes int     `json:"burden_minutes"`
	EvidenceHash  string  `json:"evidence_hash"`
	ReviewerRole  string  `json:"reviewer_role"`
}

type sanitizedOutcome struct {
	Detector      string  `json:"detector"`
	Release       string  `json:"release"`
	Confidence    float64 `json:"confidence"`
	Verdict       string  `json:"verdict"`
	Action        string  `json:"action"`
	BurdenMinutes int     `json:"burden_minutes"`
	ReviewerRole  string  `json:"reviewer_role"`
}

type groupKey struct {
	Detector string
	Release  string
	Decile   string
	Verdict  string
	Action   string
}

type outcomeKey struct {
	Verdict string
	Action  string
}

var (
	identifierPattern = regexp.MustCompile(`^[A-Za-z0-9._:/@=+-]+$`)
	allowedRootKeys   = map[string]struct{}{
		"version": {}, "claim": {}, "adopter_id": {}, "salt": {}, "min_group_size": {}, "outcomes": {},
	}
	allowedOutcomeKeys = map[string]struct{}{
		"finding_id": {}, "detector": {}, "release": {}, "confidence": {}, "verdict": {},
		"action": {}, "burden_minutes": {}, "evidence_hash": {}, "reviewer_role": {},
	}
	allowedVerdicts = map[string]struct{}{"confirmed": {}, "false_positive": {}, "uncertain": {}, "missed": {}}
	allowedActions  = map[string]struct{}{"blocked": {}, "approved": {}, "deferred": {}, "fixed": {}, "dismissed": {}}
	allowedRoles    = map[string]struct{}{"maintainer": {}, "dba": {}, "sre": {}, "security": {}, "researcher": {}, "manager": {}}
	blockedKeys     = map[string]struct{}{
		"source_code": {}, "file_content": {}, "diff": {}, "patch": {}, "snippet": {},
		"raw_evidence": {}, "line_text": {}, "stack_trace": {}, "sql_text": {}, "logs": {},
	}
)

func Ingest(reader io.Reader, opts Options) (Report, error) {
	content, err := io.ReadAll(reader)
	if err != nil {
		return Report{}, err
	}
	trimmed := bytes.TrimSpace(content)
	if len(trimmed) == 0 {
		return Report{}, errors.New("live feedback input is empty")
	}

	var root map[string]json.RawMessage
	decoder := json.NewDecoder(bytes.NewReader(trimmed))
	decoder.UseNumber()
	if err := decoder.Decode(&root); err != nil {
		return Report{}, err
	}
	var input inputFile
	if err := json.Unmarshal(trimmed, &input); err != nil {
		return Report{}, err
	}
	if input.Version != InputVersion {
		return Report{}, fmt.Errorf("unsupported live feedback input version %q", input.Version)
	}
	if input.AdopterID == "" || !safeIdentifier(input.AdopterID) {
		return Report{}, errors.New("adopter_id must be a short source-free identifier")
	}
	if len(input.Salt) < MinSaltLength {
		return Report{}, fmt.Errorf("salt must be at least %d characters and is never emitted", MinSaltLength)
	}

	report := Report{
		Version:       ReportVersion,
		OK:            true,
		Shareable:     true,
		AdopterCohort: keyedHash(input.Salt, "adopter:"+input.AdopterID),
		Privacy: PrivacySummary{
			SourceFree:          true,
			RawEvidenceFree:     true,
			IdentifierFree:      true,
			SaltEmitted:         false,
			UnknownFieldsStored: false,
			AllowedFields: []string{
				"action", "burden_minutes", "confidence", "detector", "local_dedupe_digest",
				"local_finding_key", "release", "reviewer_role", "verdict",
			},
			SuppressionRule: "publish detector/release/confidence/verdict/action groups only when count >= effective_min_group_size; otherwise fold into dimension-free residual outcome totals",
		},
	}

	for key, raw := range root {
		if _, ok := allowedRootKeys[key]; !ok {
			report.Rejected = append(report.Rejected, RejectedRecord{Index: -1, Reason: "unknown_root_field"})
			continue
		}
		if key == "outcomes" {
			continue
		}
		if containsBlockedKey(raw) {
			report.Rejected = append(report.Rejected, RejectedRecord{Index: -1, Reason: "blocked_raw_field"})
		}
	}

	requestedK := input.MinGroupSize
	if requestedK == 0 && opts.DefaultMinGroupSize > 0 {
		requestedK = opts.DefaultMinGroupSize
	}
	effectiveK := requestedK
	if effectiveK < MinKFloor {
		effectiveK = MinKFloor
	}
	if requestedK < effectiveK {
		report.Warnings = append(report.Warnings, "requested_min_group_size_raised_to_privacy_floor")
	}
	report.Summary.RequestedMinGroupSize = requestedK
	report.Summary.EffectiveMinGroupSize = effectiveK
	report.Summary.InputRecords = len(input.Outcomes)

	seen := map[string]struct{}{}
	var accepted []sanitizedOutcome
	groups := map[groupKey]Group{}
	residualCounts := map[outcomeKey]int{}

	for index, raw := range input.Outcomes {
		parsed, reject := parseOutcome(raw)
		if reject != "" {
			report.Rejected = append(report.Rejected, RejectedRecord{Index: index, Reason: reject})
			continue
		}
		dedupe := keyedHash(input.Salt, parsed.FindingID+"|"+parsed.EvidenceHash+"|"+parsed.Detector+"|"+parsed.Release)
		if _, ok := seen[dedupe]; ok {
			report.Summary.DeduplicatedRecords++
			continue
		}
		seen[dedupe] = struct{}{}
		sanitized := sanitizedOutcome{
			Detector:      parsed.Detector,
			Release:       parsed.Release,
			Confidence:    parsed.Confidence,
			Verdict:       parsed.Verdict,
			Action:        parsed.Action,
			BurdenMinutes: parsed.BurdenMinutes,
			ReviewerRole:  parsed.ReviewerRole,
		}
		accepted = append(accepted, sanitized)
		key := groupKey{
			Detector: parsed.Detector,
			Release:  parsed.Release,
			Decile:   confidenceDecile(parsed.Confidence),
			Verdict:  parsed.Verdict,
			Action:   parsed.Action,
		}
		group := groups[key]
		group.Detector = key.Detector
		group.Release = key.Release
		group.ConfidenceDecile = key.Decile
		group.Verdict = key.Verdict
		group.Action = key.Action
		group.Count++
		group.TotalBurdenMinutes += parsed.BurdenMinutes
		groups[key] = group
	}

	sort.Slice(accepted, func(i, j int) bool {
		return acceptedLess(accepted[i], accepted[j])
	})
	report.AcceptedHash = canonical.Hash(accepted)

	groupKeys := make([]groupKey, 0, len(groups))
	for key := range groups {
		groupKeys = append(groupKeys, key)
	}
	sort.Slice(groupKeys, func(i, j int) bool { return groupLess(groupKeys[i], groupKeys[j]) })

	for _, key := range groupKeys {
		group := groups[key]
		report.Summary.AcceptedRecords += group.Count
		report.Summary.TotalBurdenMinutes += group.TotalBurdenMinutes
		if group.Count >= effectiveK {
			report.Groups = append(report.Groups, group)
			continue
		}
		report.Summary.GroupsSuppressed++
		report.Residual.Count += group.Count
		report.Residual.TotalBurdenMinutes += group.TotalBurdenMinutes
		residualCounts[outcomeKey{Verdict: group.Verdict, Action: group.Action}] += group.Count
	}
	report.Summary.GroupsPublished = len(report.Groups)
	report.Summary.ResidualRecords = report.Residual.Count
	report.Summary.RejectedRecords = len(report.Rejected)
	report.Residual.OutcomeCounts = sortedOutcomeCounts(residualCounts)
	return report, nil
}

func parseOutcome(raw json.RawMessage) (outcome, string) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return outcome{}, "invalid_json"
	}
	if containsBlockedKey(raw) {
		return outcome{}, "blocked_raw_field"
	}
	for key := range fields {
		if _, ok := allowedOutcomeKeys[key]; !ok {
			return outcome{}, "unknown_field"
		}
	}
	var parsed outcome
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&parsed); err != nil {
		return outcome{}, "invalid_schema"
	}
	if parsed.FindingID == "" || parsed.Detector == "" || parsed.Release == "" || parsed.Verdict == "" || parsed.Action == "" || parsed.EvidenceHash == "" || parsed.ReviewerRole == "" {
		return outcome{}, "missing_required"
	}
	stringFields := []string{parsed.FindingID, parsed.Detector, parsed.Release, parsed.Verdict, parsed.Action, parsed.EvidenceHash, parsed.ReviewerRole}
	for _, value := range stringFields {
		if !safeIdentifier(value) || sourceLikeValue(value) {
			return outcome{}, "source_like_value"
		}
	}
	if _, ok := allowedVerdicts[parsed.Verdict]; !ok {
		return outcome{}, "invalid_verdict"
	}
	if _, ok := allowedActions[parsed.Action]; !ok {
		return outcome{}, "invalid_action"
	}
	if _, ok := allowedRoles[parsed.ReviewerRole]; !ok {
		return outcome{}, "invalid_reviewer_role"
	}
	if math.IsNaN(parsed.Confidence) || math.IsInf(parsed.Confidence, 0) || parsed.Confidence < 0 || parsed.Confidence > 1 {
		return outcome{}, "invalid_confidence"
	}
	if parsed.BurdenMinutes < 0 || parsed.BurdenMinutes > 480 {
		return outcome{}, "invalid_burden"
	}
	return parsed, ""
}

func containsBlockedKey(raw json.RawMessage) bool {
	var value any
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(&value); err != nil {
		return false
	}
	return containsBlockedKeyValue(value)
}

func containsBlockedKeyValue(value any) bool {
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			if _, ok := blockedKeys[strings.ToLower(key)]; ok {
				return true
			}
			if containsBlockedKeyValue(child) {
				return true
			}
		}
	case []any:
		for _, child := range typed {
			if containsBlockedKeyValue(child) {
				return true
			}
		}
	}
	return false
}

func safeIdentifier(value string) bool {
	if len(value) == 0 || len(value) > 120 || strings.ContainsAny(value, "\r\n\t") {
		return false
	}
	return identifierPattern.MatchString(value)
}

func sourceLikeValue(value string) bool {
	lower := strings.ToLower(value)
	sourceMarkers := []string{
		"select ", "insert into", "update ", "delete from", "create table", "drop table",
		"alter table", "diff --git", "@@ ", "function ", "class ", "exception:", "stack trace",
	}
	for _, marker := range sourceMarkers {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return strings.Count(value, ";") >= 2 || strings.Count(value, "{") >= 2 || strings.Count(value, "}") >= 2
}

func keyedHash(secret, value string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(value))
	return hex.EncodeToString(mac.Sum(nil))
}

func confidenceDecile(confidence float64) string {
	bucket := int(math.Floor(confidence * 10))
	if bucket < 0 {
		bucket = 0
	}
	if bucket > 9 {
		bucket = 9
	}
	return fmt.Sprintf("%.1f-%.1f", float64(bucket)/10, float64(bucket+1)/10)
}

func groupLess(left, right groupKey) bool {
	if left.Detector != right.Detector {
		return left.Detector < right.Detector
	}
	if left.Release != right.Release {
		return left.Release < right.Release
	}
	if left.Decile != right.Decile {
		return left.Decile < right.Decile
	}
	if left.Verdict != right.Verdict {
		return left.Verdict < right.Verdict
	}
	return left.Action < right.Action
}

func acceptedLess(left, right sanitizedOutcome) bool {
	if left.Detector != right.Detector {
		return left.Detector < right.Detector
	}
	if left.Release != right.Release {
		return left.Release < right.Release
	}
	if left.Confidence != right.Confidence {
		return left.Confidence < right.Confidence
	}
	if left.Verdict != right.Verdict {
		return left.Verdict < right.Verdict
	}
	if left.Action != right.Action {
		return left.Action < right.Action
	}
	if left.ReviewerRole != right.ReviewerRole {
		return left.ReviewerRole < right.ReviewerRole
	}
	return left.BurdenMinutes < right.BurdenMinutes
}

func sortedOutcomeCounts(counts map[outcomeKey]int) []OutcomeCount {
	keys := make([]outcomeKey, 0, len(counts))
	for key := range counts {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].Verdict != keys[j].Verdict {
			return keys[i].Verdict < keys[j].Verdict
		}
		return keys[i].Action < keys[j].Action
	})
	out := make([]OutcomeCount, 0, len(keys))
	for _, key := range keys {
		out = append(out, OutcomeCount{Verdict: key.Verdict, Action: key.Action, Count: counts[key]})
	}
	return out
}
