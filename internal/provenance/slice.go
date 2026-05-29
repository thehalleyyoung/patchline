package provenance

import (
	"fmt"
	"sort"

	"github.com/patchline/patchline/internal/canonical"
)

type Direction string

const (
	DirectionBackward Direction = "backward"
	DirectionForward  Direction = "forward"
	DirectionBoth     Direction = "both"
)

type SliceOptions struct {
	Starts      []string      `json:"starts"`
	Direction   Direction     `json:"direction"`
	MaxDepth    int           `json:"max_depth"`
	MinEvidence EvidenceLevel `json:"min_evidence"`
}

type Slice struct {
	Version    string       `json:"version"`
	Options    SliceOptions `json:"options"`
	Entities   []Entity     `json:"entities"`
	Edges      []Edge       `json:"edges"`
	EntityHash string       `json:"entity_hash"`
	EdgeHash   string       `json:"edge_hash"`
	SliceHash  string       `json:"slice_hash"`
}

func (g *Graph) Slice(opts SliceOptions) (Slice, error) {
	if len(opts.Starts) == 0 {
		return Slice{}, fmt.Errorf("at least one start entity is required")
	}
	if opts.Direction == "" {
		opts.Direction = DirectionBoth
	}
	if opts.MaxDepth < 0 {
		return Slice{}, fmt.Errorf("max depth must be non-negative")
	}
	if opts.MaxDepth == 0 {
		opts.MaxDepth = 4
	}
	opts.Starts = append([]string(nil), opts.Starts...)
	sort.Strings(opts.Starts)

	entities := map[string]Entity{}
	edges := map[string]Edge{}
	queue := make([]sliceNode, 0, len(opts.Starts))
	for _, start := range opts.Starts {
		entity, ok := g.Entity(start)
		if !ok {
			return Slice{}, fmt.Errorf("start entity %s does not exist", start)
		}
		entities[start] = entity
		queue = append(queue, sliceNode{ID: start})
	}

	visitedAtDepth := map[string]int{}
	for _, start := range opts.Starts {
		visitedAtDepth[start] = 0
	}

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		if current.Depth >= opts.MaxDepth {
			continue
		}
		for _, edge := range g.sliceEdges(current.ID, opts.Direction) {
			if !meetsEvidence(edge.Evidence, opts.MinEvidence) {
				continue
			}
			nextID := edge.To
			if edge.To == current.ID {
				nextID = edge.From
			}
			nextEntity, ok := g.Entity(nextID)
			if !ok {
				continue
			}
			entities[nextID] = nextEntity
			edges[edgeKey(edge)] = edge
			nextDepth := current.Depth + 1
			seenDepth, seen := visitedAtDepth[nextID]
			if seen && seenDepth <= nextDepth {
				continue
			}
			visitedAtDepth[nextID] = nextDepth
			queue = append(queue, sliceNode{ID: nextID, Depth: nextDepth})
		}
	}

	slice := Slice{
		Version:  "patchline.provenance-slice/v1",
		Options:  opts,
		Entities: sortedEntities(entities),
		Edges:    sortedEdgeMap(edges),
	}
	slice.EntityHash = canonical.Hash(slice.Entities)
	slice.EdgeHash = canonical.Hash(slice.Edges)
	slice.SliceHash = canonical.Hash(struct {
		Version    string       `json:"version"`
		Options    SliceOptions `json:"options"`
		EntityHash string       `json:"entity_hash"`
		EdgeHash   string       `json:"edge_hash"`
	}{
		Version:    slice.Version,
		Options:    slice.Options,
		EntityHash: slice.EntityHash,
		EdgeHash:   slice.EdgeHash,
	})
	return slice, nil
}

type sliceNode struct {
	ID    string
	Depth int
}

func (g *Graph) sliceEdges(id string, direction Direction) []Edge {
	var edges []Edge
	if direction == DirectionBackward || direction == DirectionBoth {
		edges = append(edges, g.Incoming(id)...)
	}
	if direction == DirectionForward || direction == DirectionBoth {
		edges = append(edges, g.Outgoing(id)...)
	}
	sortEdges(edges)
	return edges
}

func edgeKey(edge Edge) string {
	return string(edge.Kind) + "\x00" + edge.From + "\x00" + edge.To + "\x00" + string(edge.Evidence) + "\x00" + edge.Description
}

func sortedEntities(in map[string]Entity) []Entity {
	out := make([]Entity, 0, len(in))
	for _, entity := range in {
		out = append(out, entity)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].ID < out[j].ID
	})
	return out
}

func sortedEdgeMap(in map[string]Edge) []Edge {
	out := make([]Edge, 0, len(in))
	for _, edge := range in {
		out = append(out, edge)
	}
	sortEdges(out)
	return out
}
