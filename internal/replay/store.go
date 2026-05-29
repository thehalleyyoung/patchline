package replay

import (
	"fmt"
	"sort"

	"github.com/patchline/patchline/internal/canonical"
	"github.com/patchline/patchline/internal/effects"
	"github.com/patchline/patchline/internal/provenance"
	"github.com/patchline/patchline/internal/repair"
)

type Row map[string]string

type Store struct {
	Tables map[string]map[string]Row `json:"tables"`
}

type Report struct {
	Manifest           string            `json:"manifest"`
	Incident           string            `json:"incident"`
	Operations         []OperationReport `json:"operations"`
	DownstreamEntities []string          `json:"downstream_entities"`
}

type OperationReport struct {
	OperationID string    `json:"operation_id"`
	Table       string    `json:"table"`
	MatchedRows int       `json:"matched_rows"`
	Effect      string    `json:"effect"`
	Diffs       []RowDiff `json:"diffs"`
}

type RowDiff struct {
	Table      string                 `json:"table"`
	ID         string                 `json:"id"`
	BeforeHash string                 `json:"before_hash"`
	AfterHash  string                 `json:"after_hash"`
	Changes    map[string]ValueChange `json:"changes"`
}

type ValueChange struct {
	Before string `json:"before"`
	After  string `json:"after"`
}

func (s Store) Clone() Store {
	out := Store{Tables: map[string]map[string]Row{}}
	for table, rows := range s.Tables {
		out.Tables[table] = map[string]Row{}
		for id, row := range rows {
			out.Tables[table][id] = cloneRow(row)
		}
	}
	return out
}

func DryRun(manifest repair.Manifest, graph *provenance.Graph, store Store) (Report, error) {
	report, _, err := Apply(manifest, graph, store)
	return report, err
}

func Apply(manifest repair.Manifest, graph *provenance.Graph, store Store) (Report, Store, error) {
	working := store.Clone()
	report := Report{
		Manifest:           manifest.Name,
		Incident:           manifest.Incident,
		DownstreamEntities: downstreamEntities(manifest, graph),
	}

	for _, op := range manifest.Operations {
		classification := effects.Infer(effects.Mutation{
			Kind:                op.Kind,
			Table:               op.Table,
			WhereKeys:           keys(op.Where),
			SetKeys:             keys(op.Set),
			HasSnapshotRollback: manifest.Rollback.Strategy == "snapshot" && manifest.Rollback.SnapshotRequired,
		})
		opReport := OperationReport{
			OperationID: op.ID,
			Table:       op.Table,
			Effect:      string(classification.Effect),
		}

		switch op.Kind {
		case "insert":
			diffs, err := dryRunInsert(working, op)
			if err != nil {
				return Report{}, Store{}, err
			}
			opReport.MatchedRows = len(diffs)
			opReport.Diffs = diffs
		case "update":
			diffs, err := dryRunUpdate(working, op)
			if err != nil {
				return Report{}, Store{}, err
			}
			opReport.MatchedRows = len(diffs)
			opReport.Diffs = diffs
		case "delete":
			diffs, err := dryRunDelete(working, op)
			if err != nil {
				return Report{}, Store{}, err
			}
			opReport.MatchedRows = len(diffs)
			opReport.Diffs = diffs
		case "replay", "rebuild-index", "append-log", "emit-event", "enqueue":
			opReport.MatchedRows = 0
		default:
			return Report{}, Store{}, fmt.Errorf("dry-run does not support operation kind %q", op.Kind)
		}
		report.Operations = append(report.Operations, opReport)
	}

	return report, working, nil
}

func (r Report) CanonicalBytes() []byte {
	return canonical.MustBytes(r)
}

func (r Report) Hash() string {
	return canonical.HashBytes(r.CanonicalBytes())
}

func (s Store) Hash() string {
	return canonical.Hash(s)
}

func dryRunInsert(store Store, op repair.Operation) ([]RowDiff, error) {
	rows, ok := store.Tables[op.Table]
	if !ok {
		return nil, fmt.Errorf("table %s does not exist in replay store", op.Table)
	}
	id := op.Set["id"]
	if id == "" {
		return nil, fmt.Errorf("insert operation %s requires an id value", op.ID)
	}
	if _, exists := rows[id]; exists {
		return nil, fmt.Errorf("insert operation %s would overwrite existing row %s", op.ID, id)
	}
	row := cloneRow(op.Set)
	rows[id] = row
	return []RowDiff{{
		Table:      op.Table,
		ID:         id,
		BeforeHash: hashRow(Row{}),
		AfterHash:  hashRow(row),
		Changes:    changes(Row{}, row),
	}}, nil
}

func dryRunUpdate(store Store, op repair.Operation) ([]RowDiff, error) {
	rows, ok := store.Tables[op.Table]
	if !ok {
		return nil, fmt.Errorf("table %s does not exist in replay store", op.Table)
	}
	rowIDs := make([]string, 0, len(rows))
	for id := range rows {
		rowIDs = append(rowIDs, id)
	}
	sort.Strings(rowIDs)

	var diffs []RowDiff
	for _, id := range rowIDs {
		row := rows[id]
		if !matches(row, op.Where) {
			continue
		}
		before := cloneRow(row)
		for key, value := range op.Set {
			row[key] = value
		}
		diff := RowDiff{
			Table:      op.Table,
			ID:         id,
			BeforeHash: hashRow(before),
			AfterHash:  hashRow(row),
			Changes:    changes(before, row),
		}
		diffs = append(diffs, diff)
	}
	return diffs, nil
}

func dryRunDelete(store Store, op repair.Operation) ([]RowDiff, error) {
	rows, ok := store.Tables[op.Table]
	if !ok {
		return nil, fmt.Errorf("table %s does not exist in replay store", op.Table)
	}
	rowIDs := make([]string, 0, len(rows))
	for id := range rows {
		rowIDs = append(rowIDs, id)
	}
	sort.Strings(rowIDs)

	var diffs []RowDiff
	for _, id := range rowIDs {
		row := rows[id]
		if !matches(row, op.Where) {
			continue
		}
		before := cloneRow(row)
		delete(rows, id)
		diffs = append(diffs, RowDiff{
			Table:      op.Table,
			ID:         id,
			BeforeHash: hashRow(before),
			AfterHash:  hashRow(Row{}),
			Changes:    changes(before, Row{}),
		})
	}
	return diffs, nil
}

func downstreamEntities(manifest repair.Manifest, graph *provenance.Graph) []string {
	if graph == nil {
		return nil
	}
	seen := map[string]bool{}
	for _, entityID := range manifest.Scope.Entities {
		reachable, err := graph.ReachableFrom(entityID, nil)
		if err != nil {
			continue
		}
		for _, entity := range reachable {
			seen[entity.ID] = true
		}
	}
	out := make([]string, 0, len(seen))
	for entityID := range seen {
		out = append(out, entityID)
	}
	sort.Strings(out)
	return out
}

func matches(row Row, predicate map[string]string) bool {
	for key, want := range predicate {
		if row[key] != want {
			return false
		}
	}
	return true
}

func changes(before, after Row) map[string]ValueChange {
	keysSeen := map[string]bool{}
	for key := range before {
		keysSeen[key] = true
	}
	for key := range after {
		keysSeen[key] = true
	}
	out := map[string]ValueChange{}
	for key := range keysSeen {
		if before[key] != after[key] {
			out[key] = ValueChange{Before: before[key], After: after[key]}
		}
	}
	return out
}

func hashRow(row Row) string {
	return canonical.Hash(row)
}

func cloneRow(row Row) Row {
	out := make(Row, len(row))
	for key, value := range row {
		out[key] = value
	}
	return out
}

func keys(in map[string]string) []string {
	out := make([]string, 0, len(in))
	for key := range in {
		out = append(out, key)
	}
	sort.Strings(out)
	return out
}
