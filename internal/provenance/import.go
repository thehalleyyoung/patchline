package provenance

import "fmt"

func FromSlices(entities []Entity, edges []Edge) (*Graph, error) {
	g := New()
	for _, entity := range entities {
		if err := g.AddEntity(entity); err != nil {
			return nil, fmt.Errorf("add entity %s: %w", entity.ID, err)
		}
	}
	for _, edge := range edges {
		if err := g.AddEdge(edge); err != nil {
			return nil, fmt.Errorf("add edge %s -> %s: %w", edge.From, edge.To, err)
		}
	}
	return g, nil
}
