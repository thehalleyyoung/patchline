package evidence

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/patchline/patchline/internal/canonical"
)

const TraceVersion = "patchline.trace-projection/v1"

type SourceConfidence string

const (
	SourceExact       SourceConfidence = "exact"
	SourceCausal      SourceConfidence = "causal"
	SourceTemporal    SourceConfidence = "temporal"
	SourceInferred    SourceConfidence = "inferred"
	SourceConflicting SourceConfidence = "conflicting"
)

type ClockConfidence string

const (
	ClockExact       ClockConfidence = "exact"
	ClockTemporal    ClockConfidence = "temporal"
	ClockInferred    ClockConfidence = "inferred"
	ClockAbsent      ClockConfidence = "absent"
	ClockConflicting ClockConfidence = "conflicting"
)

type TraceProjection struct {
	Version          string             `json:"version"`
	OK               bool               `json:"ok"`
	ObservationCount int                `json:"observation_count"`
	InputHash        string             `json:"input_hash"`
	GraphHash        string             `json:"graph_hash"`
	ProjectionHash   string             `json:"projection_hash"`
	TimeRange        TimeInterval       `json:"time_range,omitempty"`
	SourceSummary    []ConfidenceCount  `json:"source_summary"`
	ClockSummary     []ConfidenceCount  `json:"clock_summary"`
	Observations     []TraceObservation `json:"observations"`
	Errors           []string           `json:"errors,omitempty"`
}

type TraceObservation struct {
	ID               string            `json:"id"`
	Type             string            `json:"type"`
	Relation         string            `json:"relation"`
	Subject          string            `json:"subject"`
	Object           string            `json:"object,omitempty"`
	Source           string            `json:"source"`
	SourceConfidence SourceConfidence  `json:"source_confidence"`
	ClockConfidence  ClockConfidence   `json:"clock_confidence"`
	Time             TimeInterval      `json:"time,omitempty"`
	Attributes       map[string]string `json:"attributes,omitempty"`
	RawHash          string            `json:"raw_hash"`
}

type TimeInterval struct {
	Start     string `json:"start,omitempty"`
	End       string `json:"end,omitempty"`
	Precision string `json:"precision,omitempty"`
}

type ConfidenceCount struct {
	Value string `json:"value"`
	Count int    `json:"count"`
}

type EquivalenceReport struct {
	Version    string   `json:"version"`
	Equivalent bool     `json:"equivalent"`
	LeftPath   string   `json:"left_path"`
	RightPath  string   `json:"right_path"`
	LeftHash   string   `json:"left_hash"`
	RightHash  string   `json:"right_hash"`
	Shared     int      `json:"shared"`
	LeftOnly   []string `json:"left_only,omitempty"`
	RightOnly  []string `json:"right_only,omitempty"`
	ReportHash string   `json:"report_hash"`
}

func ReconstructTraceJSONL(reader io.Reader) (TraceProjection, error) {
	content, err := io.ReadAll(reader)
	if err != nil {
		return TraceProjection{}, err
	}
	ingest, err := IngestJSONL(bytes.NewReader(content))
	if err != nil {
		return TraceProjection{}, err
	}
	projection := TraceProjection{
		Version:   TraceVersion,
		OK:        ingest.OK,
		InputHash: ingest.InputHash,
		GraphHash: ingest.GraphHash,
		Errors:    append([]string(nil), ingest.Errors...),
	}
	scanner := bufio.NewScanner(bytes.NewReader(content))
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}
		var event map[string]json.RawMessage
		if err := json.Unmarshal(line, &event); err != nil {
			projection.Errors = append(projection.Errors, fmt.Sprintf("line %d: invalid json: %v", lineNo, err))
			projection.OK = false
			continue
		}
		observation, err := observationFromEvent(lineNo, event, line)
		if err != nil {
			projection.Errors = append(projection.Errors, err.Error())
			projection.OK = false
			continue
		}
		projection.Observations = append(projection.Observations, observation)
	}
	if err := scanner.Err(); err != nil {
		return TraceProjection{}, err
	}
	sort.Slice(projection.Observations, func(i, j int) bool {
		return observationKey(projection.Observations[i]) < observationKey(projection.Observations[j])
	})
	markConflicts(projection.Observations)
	projection.ObservationCount = len(projection.Observations)
	projection.SourceSummary = confidenceSummary(projection.Observations, true)
	projection.ClockSummary = confidenceSummary(projection.Observations, false)
	projection.TimeRange = projectionRange(projection.Observations)
	projection.ProjectionHash = canonical.Hash(struct {
		Version       string             `json:"version"`
		GraphHash     string             `json:"graph_hash"`
		Observations  []TraceObservation `json:"observations"`
		SourceSummary []ConfidenceCount  `json:"source_summary"`
		ClockSummary  []ConfidenceCount  `json:"clock_summary"`
	}{
		Version:       projection.Version,
		GraphHash:     projection.GraphHash,
		Observations:  semanticObservations(projection.Observations),
		SourceSummary: projection.SourceSummary,
		ClockSummary:  projection.ClockSummary,
	})
	return projection, nil
}

func CompareTraceProjections(leftPath, rightPath string, left, right TraceProjection) EquivalenceReport {
	leftKeys := observationKeys(left.Observations)
	rightKeys := observationKeys(right.Observations)
	report := EquivalenceReport{
		Version:    TraceVersion,
		Equivalent: left.OK && right.OK && left.ProjectionHash == right.ProjectionHash,
		LeftPath:   leftPath,
		RightPath:  rightPath,
		LeftHash:   left.ProjectionHash,
		RightHash:  right.ProjectionHash,
	}
	for key := range leftKeys {
		if rightKeys[key] {
			report.Shared++
		} else {
			report.LeftOnly = append(report.LeftOnly, key)
		}
	}
	for key := range rightKeys {
		if !leftKeys[key] {
			report.RightOnly = append(report.RightOnly, key)
		}
	}
	sort.Strings(report.LeftOnly)
	sort.Strings(report.RightOnly)
	report.ReportHash = canonical.Hash(struct {
		Version    string   `json:"version"`
		Equivalent bool     `json:"equivalent"`
		LeftHash   string   `json:"left_hash"`
		RightHash  string   `json:"right_hash"`
		LeftOnly   []string `json:"left_only,omitempty"`
		RightOnly  []string `json:"right_only,omitempty"`
	}{
		Version:    report.Version,
		Equivalent: report.Equivalent,
		LeftHash:   report.LeftHash,
		RightHash:  report.RightHash,
		LeftOnly:   report.LeftOnly,
		RightOnly:  report.RightOnly,
	})
	return report
}

func observationFromEvent(lineNo int, event map[string]json.RawMessage, raw []byte) (TraceObservation, error) {
	eventType, ok := stringField(event, "type")
	if !ok {
		return TraceObservation{}, fmt.Errorf("line %d: missing required field type", lineNo)
	}
	observation := TraceObservation{
		Type:             eventType,
		Source:           stringValueField(event, "source", "jsonl"),
		SourceConfidence: sourceConfidence(event, eventType),
		ClockConfidence:  ClockAbsent,
		RawHash:          canonical.HashBytes(raw),
	}
	var err error
	observation.Time, observation.ClockConfidence, err = eventTime(event)
	if err != nil {
		return TraceObservation{}, fmt.Errorf("line %d: %v", lineNo, err)
	}
	observation.ID, observation.Relation, observation.Subject, observation.Object, observation.Attributes, err = eventSemantics(eventType, event)
	if err != nil {
		return TraceObservation{}, fmt.Errorf("line %d: %v", lineNo, err)
	}
	return observation, nil
}

func eventSemantics(eventType string, event map[string]json.RawMessage) (string, string, string, string, map[string]string, error) {
	switch eventType {
	case "deploy":
		id, ok := stringField(event, "id")
		if !ok {
			return "", "", "", "", nil, fmt.Errorf("deploy missing id")
		}
		commit, ok := stringField(event, "commit")
		if !ok {
			return "", "", "", "", nil, fmt.Errorf("deploy missing commit")
		}
		service, ok := stringField(event, "service")
		if !ok {
			return "", "", "", "", nil, fmt.Errorf("deploy missing service")
		}
		return id, "deployed_commit", commit, id, map[string]string{"service": service}, nil
	case "migration":
		id, ok := stringField(event, "id")
		if !ok {
			return "", "", "", "", nil, fmt.Errorf("migration missing id")
		}
		deploy, ok := stringField(event, "deploy")
		if !ok {
			return "", "", "", "", nil, fmt.Errorf("migration missing deploy")
		}
		attrs := map[string]string{}
		if name, ok := stringField(event, "name"); ok {
			attrs["name"] = name
		}
		return id, "executed", deploy, id, attrs, nil
	case "trace":
		id, ok := stringField(event, "id")
		if !ok {
			return "", "", "", "", nil, fmt.Errorf("trace missing id")
		}
		migration, ok := stringField(event, "migration")
		if !ok {
			return "", "", "", "", nil, fmt.Errorf("trace missing migration")
		}
		return id, "observed", migration, id, nil, nil
	case "sql_mutation":
		id, ok := stringField(event, "id")
		if !ok {
			return "", "", "", "", nil, fmt.Errorf("sql_mutation missing id")
		}
		trace, ok := stringField(event, "trace")
		if !ok {
			return "", "", "", "", nil, fmt.Errorf("sql_mutation missing trace")
		}
		attrs := map[string]string{}
		if fingerprint, ok := stringField(event, "fingerprint"); ok {
			attrs["fingerprint"] = fingerprint
		}
		return id, "caused", trace, id, attrs, nil
	case "row_mutation":
		record, ok := stringField(event, "record")
		if !ok {
			return "", "", "", "", nil, fmt.Errorf("row_mutation missing record")
		}
		sql, ok := stringField(event, "sql")
		if !ok {
			return "", "", "", "", nil, fmt.Errorf("row_mutation missing sql")
		}
		return "row_mutation:" + record, "mutated", sql, record, nil, nil
	case "derived_record", "derived_report":
		from, ok := stringField(event, "from")
		if !ok {
			return "", "", "", "", nil, fmt.Errorf("%s missing from", eventType)
		}
		to, ok := stringField(event, "to")
		if !ok {
			return "", "", "", "", nil, fmt.Errorf("%s missing to", eventType)
		}
		return eventType + ":" + from + "->" + to, "derived_into", from, to, nil, nil
	default:
		return "", "", "", "", nil, fmt.Errorf("unsupported event type %q", eventType)
	}
}

func sourceConfidence(event map[string]json.RawMessage, eventType string) SourceConfidence {
	if raw, ok := stringField(event, "source_confidence"); ok {
		switch SourceConfidence(strings.ToLower(raw)) {
		case SourceExact, SourceCausal, SourceTemporal, SourceInferred, SourceConflicting:
			return SourceConfidence(strings.ToLower(raw))
		}
	}
	switch eventType {
	case "deploy", "migration", "row_mutation":
		return SourceExact
	case "trace", "sql_mutation", "derived_record", "derived_report":
		return SourceCausal
	default:
		return SourceInferred
	}
}

func eventTime(event map[string]json.RawMessage) (TimeInterval, ClockConfidence, error) {
	if exact := firstStringField(event, "event_time", "observed_at", "timestamp", "time"); exact != "" {
		normalized, err := normalizeTime(exact)
		if err != nil {
			return TimeInterval{}, ClockConflicting, err
		}
		return TimeInterval{Start: normalized, End: normalized, Precision: "instant"}, clockConfidenceOverride(event, ClockExact), nil
	}
	start := firstStringField(event, "start_time", "window_start")
	end := firstStringField(event, "end_time", "window_end")
	if start != "" || end != "" {
		normalizedStart, normalizedEnd, err := normalizeInterval(start, end)
		if err != nil {
			return TimeInterval{}, ClockConflicting, err
		}
		return TimeInterval{Start: normalizedStart, End: normalizedEnd, Precision: "interval"}, clockConfidenceOverride(event, ClockTemporal), nil
	}
	return TimeInterval{}, clockConfidenceOverride(event, ClockAbsent), nil
}

func clockConfidenceOverride(event map[string]json.RawMessage, fallback ClockConfidence) ClockConfidence {
	if raw, ok := stringField(event, "clock_confidence"); ok {
		switch ClockConfidence(strings.ToLower(raw)) {
		case ClockExact, ClockTemporal, ClockInferred, ClockAbsent, ClockConflicting:
			return ClockConfidence(strings.ToLower(raw))
		}
	}
	return fallback
}

func normalizeInterval(start, end string) (string, string, error) {
	var normalizedStart, normalizedEnd string
	var err error
	if start != "" {
		normalizedStart, err = normalizeTime(start)
		if err != nil {
			return "", "", err
		}
	}
	if end != "" {
		normalizedEnd, err = normalizeTime(end)
		if err != nil {
			return "", "", err
		}
	}
	if normalizedStart != "" && normalizedEnd != "" && normalizedStart > normalizedEnd {
		return "", "", fmt.Errorf("time interval start %s is after end %s", normalizedStart, normalizedEnd)
	}
	return normalizedStart, normalizedEnd, nil
}

func normalizeTime(value string) (string, error) {
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return "", fmt.Errorf("invalid event time %q: %w", value, err)
	}
	return parsed.UTC().Format(time.RFC3339Nano), nil
}

func markConflicts(observations []TraceObservation) {
	seen := map[string]int{}
	for i := range observations {
		key := observations[i].Type + "\x00" + observations[i].Relation + "\x00" + observations[i].Subject + "\x00" + observations[i].Object
		if previous, ok := seen[key]; ok {
			if observations[previous].Time != observations[i].Time {
				observations[previous].ClockConfidence = ClockConflicting
				observations[i].ClockConfidence = ClockConflicting
			}
			if observations[previous].Source != observations[i].Source {
				observations[previous].SourceConfidence = SourceConflicting
				observations[i].SourceConfidence = SourceConflicting
			}
			continue
		}
		seen[key] = i
	}
}

func confidenceSummary(observations []TraceObservation, source bool) []ConfidenceCount {
	counts := map[string]int{}
	for _, observation := range observations {
		if source {
			counts[string(observation.SourceConfidence)]++
		} else {
			counts[string(observation.ClockConfidence)]++
		}
	}
	out := make([]ConfidenceCount, 0, len(counts))
	for value, count := range counts {
		out = append(out, ConfidenceCount{Value: value, Count: count})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Value < out[j].Value })
	return out
}

func projectionRange(observations []TraceObservation) TimeInterval {
	var start, end string
	for _, observation := range observations {
		if observation.Time.Start != "" && (start == "" || observation.Time.Start < start) {
			start = observation.Time.Start
		}
		if observation.Time.End != "" && (end == "" || observation.Time.End > end) {
			end = observation.Time.End
		}
	}
	if start == "" && end == "" {
		return TimeInterval{}
	}
	return TimeInterval{Start: start, End: end, Precision: "range"}
}

func observationKeys(observations []TraceObservation) map[string]bool {
	out := map[string]bool{}
	for _, observation := range observations {
		out[observationKey(observation)] = true
	}
	return out
}

func semanticObservations(observations []TraceObservation) []TraceObservation {
	out := make([]TraceObservation, len(observations))
	copy(out, observations)
	for i := range out {
		out[i].RawHash = ""
	}
	return out
}

func observationKey(observation TraceObservation) string {
	return strings.Join([]string{
		observation.Type,
		observation.Relation,
		observation.Subject,
		observation.Object,
		observation.Source,
		string(observation.SourceConfidence),
		string(observation.ClockConfidence),
		observation.Time.Start,
		observation.Time.End,
		canonical.Hash(observation.Attributes),
	}, "\x00")
}

func firstStringField(event map[string]json.RawMessage, fields ...string) string {
	for _, field := range fields {
		if value, ok := stringField(event, field); ok {
			return value
		}
	}
	return ""
}

func stringValueField(event map[string]json.RawMessage, field, fallback string) string {
	if value, ok := stringField(event, field); ok {
		return value
	}
	return fallback
}
