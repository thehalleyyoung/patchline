package evidence

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"sort"

	"github.com/patchline/patchline/internal/canonical"
	"github.com/patchline/patchline/internal/provenance"
)

const Version = "patchline.evidence-ingest/v1"

type Result struct {
	Version           string              `json:"version"`
	OK                bool                `json:"ok"`
	EventCount        int                 `json:"event_count"`
	UnknownFieldCount int                 `json:"unknown_field_count"`
	SourceTypes       []TypeCount         `json:"source_types"`
	Entities          []provenance.Entity `json:"entities"`
	Edges             []provenance.Edge   `json:"edges"`
	DamagedEntities   []string            `json:"damaged_entities"`
	EntityHash        string              `json:"entity_hash"`
	EdgeHash          string              `json:"edge_hash"`
	GraphHash         string              `json:"graph_hash"`
	InputHash         string              `json:"input_hash"`
	Errors            []string            `json:"errors,omitempty"`
}

type TypeCount struct {
	Type  string `json:"type"`
	Count int    `json:"count"`
}

type builder struct {
	entities          map[string]provenance.Entity
	edges             map[string]provenance.Edge
	sourceTypes       map[string]int
	unknownFieldCount int
	errors            []string
}

func IngestJSONL(reader io.Reader) (Result, error) {
	content, err := io.ReadAll(reader)
	if err != nil {
		return Result{}, err
	}
	b := &builder{
		entities:    map[string]provenance.Entity{},
		edges:       map[string]provenance.Edge{},
		sourceTypes: map[string]int{},
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
			b.errors = append(b.errors, fmt.Sprintf("line %d: invalid json: %v", lineNo, err))
			continue
		}
		eventType, ok := stringField(event, "type")
		if !ok {
			b.errors = append(b.errors, fmt.Sprintf("line %d: missing required field type", lineNo))
			continue
		}
		b.sourceTypes[eventType]++
		b.unknownFieldCount += countUnknown(event, allowedFields(eventType))
		if err := b.applyEvent(lineNo, eventType, event); err != nil {
			b.errors = append(b.errors, err.Error())
		}
	}
	if err := scanner.Err(); err != nil {
		return Result{}, err
	}
	b.validateEdges()

	result := Result{
		Version:           Version,
		OK:                len(b.errors) == 0,
		EventCount:        sumTypes(b.sourceTypes),
		UnknownFieldCount: b.unknownFieldCount,
		SourceTypes:       sortedTypeCounts(b.sourceTypes),
		Entities:          sortedEntities(b.entities),
		Edges:             sortedEdges(b.edges),
		DamagedEntities:   damagedEntities(b.edges),
		InputHash:         canonical.HashBytes(normalizeLines(content)),
		Errors:            append([]string(nil), b.errors...),
	}
	result.EntityHash = canonical.Hash(result.Entities)
	result.EdgeHash = canonical.Hash(result.Edges)
	result.GraphHash = canonical.Hash(struct {
		Version    string `json:"version"`
		EntityHash string `json:"entity_hash"`
		EdgeHash   string `json:"edge_hash"`
	}{
		Version:    result.Version,
		EntityHash: result.EntityHash,
		EdgeHash:   result.EdgeHash,
	})
	return result, nil
}

func (b *builder) applyEvent(lineNo int, eventType string, event map[string]json.RawMessage) error {
	line := func(format string, args ...any) error {
		return fmt.Errorf("line %d: %s", lineNo, fmt.Sprintf(format, args...))
	}
	switch eventType {
	case "deploy":
		id, ok := stringField(event, "id")
		if !ok {
			return line("deploy missing id")
		}
		commit, ok := stringField(event, "commit")
		if !ok {
			return line("deploy missing commit")
		}
		service, ok := stringField(event, "service")
		if !ok {
			return line("deploy missing service")
		}
		b.addEntity(provenance.Entity{ID: "service:" + service, Kind: provenance.KindService, Name: service})
		b.addEntity(provenance.Entity{ID: commit, Kind: provenance.KindCommit})
		b.addEntity(provenance.Entity{ID: id, Kind: provenance.KindDeploy, Attributes: map[string]string{"service": service}})
		b.addEdge(provenance.Edge{From: commit, To: id, Kind: provenance.EdgeDeployedCommit, Evidence: provenance.EvidenceExact, Description: "evidence deploy event"})
	case "migration":
		id, ok := stringField(event, "id")
		if !ok {
			return line("migration missing id")
		}
		deploy, ok := stringField(event, "deploy")
		if !ok {
			return line("migration missing deploy")
		}
		name, _ := stringField(event, "name")
		b.addEntity(provenance.Entity{ID: id, Kind: provenance.KindMigration, Name: name})
		b.addEdge(provenance.Edge{From: deploy, To: id, Kind: provenance.EdgeExecuted, Evidence: provenance.EvidenceExact, Description: "evidence migration event"})
	case "trace":
		id, ok := stringField(event, "id")
		if !ok {
			return line("trace missing id")
		}
		migration, ok := stringField(event, "migration")
		if !ok {
			return line("trace missing migration")
		}
		b.addEntity(provenance.Entity{ID: id, Kind: provenance.KindTrace})
		b.addEdge(provenance.Edge{From: migration, To: id, Kind: provenance.EdgeObserved, Evidence: provenance.EvidenceStrong, Description: "evidence trace event"})
	case "sql_mutation":
		id, ok := stringField(event, "id")
		if !ok {
			return line("sql_mutation missing id")
		}
		trace, ok := stringField(event, "trace")
		if !ok {
			return line("sql_mutation missing trace")
		}
		fingerprint, _ := stringField(event, "fingerprint")
		b.addEntity(provenance.Entity{ID: id, Kind: provenance.KindSQLMutation, Attributes: map[string]string{"fingerprint": fingerprint}})
		b.addEdge(provenance.Edge{From: trace, To: id, Kind: provenance.EdgeCaused, Evidence: provenance.EvidenceStrong, Description: "evidence sql mutation event"})
	case "row_mutation":
		record, ok := stringField(event, "record")
		if !ok {
			return line("row_mutation missing record")
		}
		sql, ok := stringField(event, "sql")
		if !ok {
			return line("row_mutation missing sql")
		}
		b.addEntity(provenance.Entity{ID: record, Kind: provenance.KindRecord})
		b.addEdge(provenance.Edge{From: sql, To: record, Kind: provenance.EdgeMutated, Evidence: provenance.EvidenceExact, Description: "evidence row mutation event"})
	case "derived_record", "derived_report":
		from, ok := stringField(event, "from")
		if !ok {
			return line("%s missing from", eventType)
		}
		to, ok := stringField(event, "to")
		if !ok {
			return line("%s missing to", eventType)
		}
		kind := provenance.KindRecord
		if eventType == "derived_report" {
			kind = provenance.KindReport
		}
		b.addEntity(provenance.Entity{ID: to, Kind: kind})
		b.addEdge(provenance.Edge{From: from, To: to, Kind: provenance.EdgeDerivedInto, Evidence: provenance.EvidenceStrong, Description: "evidence derivation event"})
	default:
		return line("unsupported event type %q", eventType)
	}
	return nil
}

func (b *builder) addEntity(entity provenance.Entity) {
	if existing, ok := b.entities[entity.ID]; ok {
		if existing.Kind != entity.Kind {
			b.errors = append(b.errors, fmt.Sprintf("entity %s has conflicting kinds %s and %s", entity.ID, existing.Kind, entity.Kind))
		}
		if existing.Name == "" && entity.Name != "" {
			existing.Name = entity.Name
		}
		if existing.Attributes == nil && entity.Attributes != nil {
			existing.Attributes = copyMap(entity.Attributes)
		}
		b.entities[entity.ID] = existing
		return
	}
	entity.Attributes = copyMap(entity.Attributes)
	b.entities[entity.ID] = entity
}

func (b *builder) addEdge(edge provenance.Edge) {
	b.edges[edgeKey(edge)] = edge
}

func (b *builder) validateEdges() {
	for _, edge := range sortedEdges(b.edges) {
		if _, ok := b.entities[edge.From]; !ok {
			b.errors = append(b.errors, fmt.Sprintf("edge source %s is not defined", edge.From))
		}
		if _, ok := b.entities[edge.To]; !ok {
			b.errors = append(b.errors, fmt.Sprintf("edge target %s is not defined", edge.To))
		}
	}
}

func allowedFields(eventType string) map[string]bool {
	commonTraceFields := []string{
		"source",
		"source_confidence",
		"clock_confidence",
		"event_time",
		"observed_at",
		"timestamp",
		"time",
		"start_time",
		"end_time",
		"window_start",
		"window_end",
	}
	fields := map[string][]string{
		"deploy":         {"type", "id", "commit", "service"},
		"migration":      {"type", "id", "deploy", "name"},
		"trace":          {"type", "id", "migration"},
		"sql_mutation":   {"type", "id", "trace", "fingerprint"},
		"row_mutation":   {"type", "record", "sql", "before", "after"},
		"derived_record": {"type", "from", "to"},
		"derived_report": {"type", "from", "to"},
	}
	allowed := map[string]bool{}
	for _, field := range append(fields[eventType], commonTraceFields...) {
		allowed[field] = true
	}
	return allowed
}

func countUnknown(event map[string]json.RawMessage, allowed map[string]bool) int {
	if len(allowed) == 0 {
		return len(event)
	}
	count := 0
	for field := range event {
		if !allowed[field] {
			count++
		}
	}
	return count
}

func stringField(event map[string]json.RawMessage, field string) (string, bool) {
	raw, ok := event[field]
	if !ok {
		return "", false
	}
	var value string
	if err := json.Unmarshal(raw, &value); err != nil || value == "" {
		return "", false
	}
	return value, true
}

func damagedEntities(edges map[string]provenance.Edge) []string {
	adj := map[string][]string{}
	starts := map[string]bool{}
	for _, edge := range edges {
		if edge.Kind == provenance.EdgeMutated {
			starts[edge.To] = true
		}
		if edge.Kind == provenance.EdgeDerivedInto {
			adj[edge.From] = append(adj[edge.From], edge.To)
		}
	}
	visited := map[string]bool{}
	queue := make([]string, 0, len(starts))
	for id := range starts {
		visited[id] = true
		queue = append(queue, id)
	}
	sort.Strings(queue)
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		next := append([]string(nil), adj[current]...)
		sort.Strings(next)
		for _, id := range next {
			if visited[id] {
				continue
			}
			visited[id] = true
			queue = append(queue, id)
		}
	}
	out := make([]string, 0, len(visited))
	for id := range visited {
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}

func normalizeLines(content []byte) []byte {
	var lines []string
	scanner := bufio.NewScanner(bytes.NewReader(content))
	for scanner.Scan() {
		line := string(bytes.TrimSpace(scanner.Bytes()))
		if line != "" {
			lines = append(lines, line)
		}
	}
	sort.Strings(lines)
	return []byte(fmt.Sprintf("%q", lines))
}

func sortedEntities(in map[string]provenance.Entity) []provenance.Entity {
	out := make([]provenance.Entity, 0, len(in))
	for _, entity := range in {
		entity.Attributes = copyMap(entity.Attributes)
		out = append(out, entity)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func sortedEdges(in map[string]provenance.Edge) []provenance.Edge {
	out := make([]provenance.Edge, 0, len(in))
	for _, edge := range in {
		out = append(out, edge)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].From != out[j].From {
			return out[i].From < out[j].From
		}
		if out[i].To != out[j].To {
			return out[i].To < out[j].To
		}
		if out[i].Kind != out[j].Kind {
			return out[i].Kind < out[j].Kind
		}
		return out[i].Description < out[j].Description
	})
	return out
}

func sortedTypeCounts(in map[string]int) []TypeCount {
	out := make([]TypeCount, 0, len(in))
	for typ, count := range in {
		out = append(out, TypeCount{Type: typ, Count: count})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Type < out[j].Type })
	return out
}

func sumTypes(in map[string]int) int {
	total := 0
	for _, count := range in {
		total += count
	}
	return total
}

func edgeKey(edge provenance.Edge) string {
	return edge.From + "\x00" + edge.To + "\x00" + string(edge.Kind) + "\x00" + string(edge.Evidence) + "\x00" + edge.Description
}

func copyMap(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
