package provenance

import (
	"errors"
	"fmt"
	"sort"
)

type EntityKind string

const (
	KindService     EntityKind = "service"
	KindCommit      EntityKind = "commit"
	KindDeploy      EntityKind = "deploy"
	KindMigration   EntityKind = "migration"
	KindTrace       EntityKind = "trace"
	KindSQLMutation EntityKind = "sql_mutation"
	KindRecord      EntityKind = "record"
	KindJobRun      EntityKind = "job_run"
	KindQueueEvent  EntityKind = "queue_event"
	KindReport      EntityKind = "report"
	KindRepair      EntityKind = "repair"
)

type EdgeKind string

const (
	EdgeDeployedCommit EdgeKind = "deployed_commit"
	EdgeExecuted       EdgeKind = "executed"
	EdgeCaused         EdgeKind = "caused"
	EdgeMutated        EdgeKind = "mutated"
	EdgeDerivedInto    EdgeKind = "derived_into"
	EdgeObserved       EdgeKind = "observed"
	EdgeRepaired       EdgeKind = "repaired"
)

type EvidenceLevel string

const (
	EvidenceWeak   EvidenceLevel = "weak"
	EvidenceMedium EvidenceLevel = "medium"
	EvidenceStrong EvidenceLevel = "strong"
	EvidenceExact  EvidenceLevel = "exact"
)

type Entity struct {
	ID         string            `json:"id"`
	Kind       EntityKind        `json:"kind"`
	Name       string            `json:"name,omitempty"`
	Attributes map[string]string `json:"attributes,omitempty"`
}

type Edge struct {
	From        string        `json:"from"`
	To          string        `json:"to"`
	Kind        EdgeKind      `json:"kind"`
	Evidence    EvidenceLevel `json:"evidence"`
	Description string        `json:"description,omitempty"`
}

type Step struct {
	Entity Entity `json:"entity"`
	Via    *Edge  `json:"via,omitempty"`
}

type Path struct {
	Start string `json:"start"`
	Steps []Step `json:"steps"`
}

type TraceOptions struct {
	StopKinds    []EntityKind
	AllowedEdges []EdgeKind
	MinEvidence  EvidenceLevel
}

type Graph struct {
	entities map[string]Entity
	outgoing map[string][]Edge
	incoming map[string][]Edge
}

func New() *Graph {
	return &Graph{
		entities: map[string]Entity{},
		outgoing: map[string][]Edge{},
		incoming: map[string][]Edge{},
	}
}

func (g *Graph) AddEntity(entity Entity) error {
	if entity.ID == "" {
		return errors.New("entity id is required")
	}
	if entity.Kind == "" {
		return fmt.Errorf("entity %s kind is required", entity.ID)
	}
	if _, exists := g.entities[entity.ID]; exists {
		return fmt.Errorf("entity %s already exists", entity.ID)
	}
	entity.Attributes = copyMap(entity.Attributes)
	g.entities[entity.ID] = entity
	return nil
}

func (g *Graph) AddEdge(edge Edge) error {
	if edge.From == "" || edge.To == "" {
		return errors.New("edge endpoints are required")
	}
	if edge.Kind == "" {
		return fmt.Errorf("edge %s -> %s kind is required", edge.From, edge.To)
	}
	if edge.Evidence == "" {
		edge.Evidence = EvidenceWeak
	}
	if _, exists := g.entities[edge.From]; !exists {
		return fmt.Errorf("edge source %s does not exist", edge.From)
	}
	if _, exists := g.entities[edge.To]; !exists {
		return fmt.Errorf("edge target %s does not exist", edge.To)
	}
	g.outgoing[edge.From] = append(g.outgoing[edge.From], edge)
	g.incoming[edge.To] = append(g.incoming[edge.To], edge)
	sortEdges(g.outgoing[edge.From])
	sortEdges(g.incoming[edge.To])
	return nil
}

func (g *Graph) Entity(id string) (Entity, bool) {
	entity, ok := g.entities[id]
	entity.Attributes = copyMap(entity.Attributes)
	return entity, ok
}

func (g *Graph) Entities() []Entity {
	entities := make([]Entity, 0, len(g.entities))
	for _, entity := range g.entities {
		entity.Attributes = copyMap(entity.Attributes)
		entities = append(entities, entity)
	}
	sort.Slice(entities, func(i, j int) bool {
		return entities[i].ID < entities[j].ID
	})
	return entities
}

func (g *Graph) Edges() []Edge {
	var edges []Edge
	for _, outgoing := range g.outgoing {
		edges = append(edges, outgoing...)
	}
	sortEdges(edges)
	return edges
}

func (g *Graph) Incoming(id string) []Edge {
	edges := append([]Edge(nil), g.incoming[id]...)
	sortEdges(edges)
	return edges
}

func (g *Graph) Outgoing(id string) []Edge {
	edges := append([]Edge(nil), g.outgoing[id]...)
	sortEdges(edges)
	return edges
}

func (g *Graph) Backtrace(start string, opts TraceOptions) ([]Path, error) {
	startEntity, ok := g.Entity(start)
	if !ok {
		return nil, fmt.Errorf("entity %s does not exist", start)
	}

	queue := []Path{{Start: start, Steps: []Step{{Entity: startEntity}}}}
	visited := map[string]bool{start: true}
	var results []Path

	for len(queue) > 0 {
		path := queue[0]
		queue = queue[1:]
		current := path.Steps[len(path.Steps)-1].Entity

		if current.ID != start && containsKind(opts.StopKinds, current.Kind) {
			results = append(results, path)
			continue
		}

		for _, edge := range g.Incoming(current.ID) {
			if !containsEdge(opts.AllowedEdges, edge.Kind) || !meetsEvidence(edge.Evidence, opts.MinEvidence) {
				continue
			}
			if visited[edge.From] {
				continue
			}
			nextEntity, ok := g.Entity(edge.From)
			if !ok {
				continue
			}
			edgeCopy := edge
			next := Path{
				Start: start,
				Steps: append(append([]Step(nil), path.Steps...), Step{
					Entity: nextEntity,
					Via:    &edgeCopy,
				}),
			}
			visited[edge.From] = true
			queue = append(queue, next)
		}
	}

	return results, nil
}

func (g *Graph) ReachableFrom(start string, allowedEdges []EdgeKind) ([]Entity, error) {
	if _, ok := g.Entity(start); !ok {
		return nil, fmt.Errorf("entity %s does not exist", start)
	}

	var out []Entity
	queue := []string{start}
	visited := map[string]bool{start: true}
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		for _, edge := range g.Outgoing(current) {
			if !containsEdge(allowedEdges, edge.Kind) || visited[edge.To] {
				continue
			}
			entity, ok := g.Entity(edge.To)
			if !ok {
				continue
			}
			visited[edge.To] = true
			out = append(out, entity)
			queue = append(queue, edge.To)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].ID < out[j].ID
	})
	return out, nil
}

func (g *Graph) AffectedRecordsFrom(start string) ([]Entity, error) {
	reachable, err := g.ReachableFrom(start, nil)
	if err != nil {
		return nil, err
	}
	records := reachable[:0]
	for _, entity := range reachable {
		if entity.Kind == KindRecord {
			records = append(records, entity)
		}
	}
	return records, nil
}

func containsKind(kinds []EntityKind, kind EntityKind) bool {
	if len(kinds) == 0 {
		return false
	}
	for _, candidate := range kinds {
		if candidate == kind {
			return true
		}
	}
	return false
}

func containsEdge(edges []EdgeKind, edge EdgeKind) bool {
	if len(edges) == 0 {
		return true
	}
	for _, candidate := range edges {
		if candidate == edge {
			return true
		}
	}
	return false
}

func meetsEvidence(actual, minimum EvidenceLevel) bool {
	if minimum == "" {
		minimum = EvidenceWeak
	}
	return evidenceRank(actual) >= evidenceRank(minimum)
}

func evidenceRank(level EvidenceLevel) int {
	switch level {
	case EvidenceExact:
		return 4
	case EvidenceStrong:
		return 3
	case EvidenceMedium:
		return 2
	case EvidenceWeak, "":
		return 1
	default:
		return 0
	}
}

func sortEdges(edges []Edge) {
	sort.Slice(edges, func(i, j int) bool {
		if edges[i].From != edges[j].From {
			return edges[i].From < edges[j].From
		}
		if edges[i].To != edges[j].To {
			return edges[i].To < edges[j].To
		}
		if edges[i].Kind != edges[j].Kind {
			return edges[i].Kind < edges[j].Kind
		}
		if edges[i].Evidence != edges[j].Evidence {
			return evidenceRank(edges[i].Evidence) > evidenceRank(edges[j].Evidence)
		}
		return edges[i].Description < edges[j].Description
	})
}

func copyMap(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}
