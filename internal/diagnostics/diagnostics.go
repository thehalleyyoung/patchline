package diagnostics

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/thehalleyyoung/patchline/internal/canonical"
)

const Version = "patchline.diagnostics/v1"

type Recorder struct {
	enabled bool
	outDir  string
	events  []Event
	start   time.Time
	mu      sync.Mutex
	nextID  int
}

type Event struct {
	Version    string         `json:"version"`
	Type       string         `json:"type"`
	TraceID    string         `json:"trace_id"`
	SpanID     string         `json:"span_id,omitempty"`
	ParentID   string         `json:"parent_id,omitempty"`
	Name       string         `json:"name"`
	Status     string         `json:"status,omitempty"`
	Message    string         `json:"message,omitempty"`
	StartedAt  string         `json:"started_at,omitempty"`
	EndedAt    string         `json:"ended_at,omitempty"`
	DurationMS int64          `json:"duration_ms,omitempty"`
	ElapsedMS  int64          `json:"elapsed_ms,omitempty"`
	Attributes map[string]any `json:"attributes,omitempty"`
	Error      string         `json:"error,omitempty"`
	Sequence   int            `json:"sequence"`
}

type Summary struct {
	Version     string `json:"version"`
	TraceID     string `json:"trace_id"`
	Events      int    `json:"events"`
	Spans       int    `json:"spans"`
	Logs        int    `json:"logs"`
	FailedSpans int    `json:"failed_spans"`
	DurationMS  int64  `json:"duration_ms"`
	EventsPath  string `json:"events_path,omitempty"`
	SummaryPath string `json:"summary_path,omitempty"`
	Hash        string `json:"hash"`
}

type Span struct {
	rec      *Recorder
	id       string
	parentID string
	name     string
	start    time.Time
	attrs    map[string]any
	ended    bool
}

func New(outDir string, enabled bool) *Recorder {
	return &Recorder{
		enabled: enabled,
		outDir:  outDir,
		start:   time.Now().UTC(),
	}
}

func (r *Recorder) Enabled() bool {
	return r != nil && r.enabled
}

func (r *Recorder) StartSpan(name string, attrs map[string]any) *Span {
	if !r.Enabled() {
		return &Span{}
	}
	r.mu.Lock()
	r.nextID++
	id := fmt.Sprintf("span:%04d", r.nextID)
	start := time.Now().UTC()
	r.mu.Unlock()
	return &Span{rec: r, id: id, name: name, start: start, attrs: normalizeAttrs(attrs)}
}

func (s *Span) Child(name string, attrs map[string]any) *Span {
	if s == nil || !s.rec.Enabled() {
		return &Span{}
	}
	child := s.rec.StartSpan(name, attrs)
	child.parentID = s.id
	return child
}

func (s *Span) End(err error, attrs map[string]any) {
	if s == nil || s.rec == nil || !s.rec.Enabled() || s.ended {
		return
	}
	s.ended = true
	end := time.Now().UTC()
	status := "ok"
	errText := ""
	if err != nil {
		status = "error"
		errText = err.Error()
	}
	merged := normalizeAttrs(s.attrs)
	finalAttrs := normalizeAttrs(attrs)
	if len(finalAttrs) > 0 && merged == nil {
		merged = map[string]any{}
	}
	for key, value := range finalAttrs {
		merged[key] = value
	}
	s.rec.append(Event{
		Version:    Version,
		Type:       "span",
		TraceID:    s.rec.traceID(),
		SpanID:     s.id,
		ParentID:   s.parentID,
		Name:       s.name,
		Status:     status,
		StartedAt:  s.start.Format(time.RFC3339Nano),
		EndedAt:    end.Format(time.RFC3339Nano),
		DurationMS: end.Sub(s.start).Milliseconds(),
		ElapsedMS:  end.Sub(s.rec.start).Milliseconds(),
		Attributes: merged,
		Error:      errText,
	})
}

func (r *Recorder) Log(name, message string, attrs map[string]any) {
	if !r.Enabled() {
		return
	}
	now := time.Now().UTC()
	r.append(Event{
		Version:    Version,
		Type:       "log",
		TraceID:    r.traceID(),
		Name:       name,
		Status:     "info",
		Message:    message,
		ElapsedMS:  now.Sub(r.start).Milliseconds(),
		Attributes: normalizeAttrs(attrs),
	})
}

func (r *Recorder) Write() (Summary, error) {
	if !r.Enabled() {
		return Summary{}, nil
	}
	if err := os.MkdirAll(r.outDir, 0o755); err != nil {
		return Summary{}, err
	}
	eventsPath := filepath.Join(r.outDir, "events.jsonl")
	file, err := os.OpenFile(eventsPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return Summary{}, err
	}
	enc := json.NewEncoder(file)
	for _, event := range r.snapshot() {
		if err := enc.Encode(event); err != nil {
			_ = file.Close()
			return Summary{}, err
		}
	}
	if err := file.Close(); err != nil {
		return Summary{}, err
	}
	summary := r.Summary()
	summary.EventsPath = filepath.ToSlash(eventsPath)
	summary.SummaryPath = filepath.ToSlash(filepath.Join(r.outDir, "summary.json"))
	summary.Hash = canonical.Hash(struct {
		Version string  `json:"version"`
		TraceID string  `json:"trace_id"`
		Events  []Event `json:"events"`
	}{summary.Version, summary.TraceID, r.snapshot()})
	data, err := json.MarshalIndent(summary, "", "  ")
	if err != nil {
		return Summary{}, err
	}
	if err := os.WriteFile(filepath.Join(r.outDir, "summary.json"), append(data, '\n'), 0o644); err != nil {
		return Summary{}, err
	}
	return summary, nil
}

func (r *Recorder) Summary() Summary {
	if !r.Enabled() {
		return Summary{}
	}
	summary := Summary{Version: Version, TraceID: r.traceID(), DurationMS: time.Since(r.start).Milliseconds()}
	for _, event := range r.snapshot() {
		summary.Events++
		switch event.Type {
		case "span":
			summary.Spans++
			if event.Status == "error" {
				summary.FailedSpans++
			}
		case "log":
			summary.Logs++
		}
	}
	return summary
}

func (r *Recorder) append(event Event) {
	r.mu.Lock()
	defer r.mu.Unlock()
	event.Sequence = len(r.events) + 1
	r.events = append(r.events, event)
}

func (r *Recorder) snapshot() []Event {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]Event, len(r.events))
	copy(out, r.events)
	return out
}

func (r *Recorder) traceID() string {
	return "trace:" + canonical.Hash(struct {
		OutDir string `json:"out_dir"`
		Start  string `json:"start"`
	}{r.outDir, r.start.Format(time.RFC3339Nano)})[:20]
}

func normalizeAttrs(attrs map[string]any) map[string]any {
	if len(attrs) == 0 {
		return nil
	}
	out := make(map[string]any, len(attrs))
	keys := make([]string, 0, len(attrs))
	for key := range attrs {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		value := attrs[key]
		switch v := value.(type) {
		case nil:
			continue
		case string, bool, int, int64, float64:
			out[key] = v
		default:
			out[key] = fmt.Sprintf("%v", v)
		}
	}
	return out
}
