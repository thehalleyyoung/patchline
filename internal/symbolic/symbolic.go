package symbolic

import (
	"fmt"
	"sort"
	"strings"

	"github.com/thehalleyyoung/patchline/internal/canonical"
	"github.com/thehalleyyoung/patchline/internal/repair"
	"github.com/thehalleyyoung/patchline/internal/replay"
)

const Version = "patchline.symbolic-execution/v1"

type Report struct {
	Version   string  `json:"version"`
	Manifest  string  `json:"manifest"`
	StoreHash string  `json:"store_hash"`
	Steps     []Step  `json:"steps"`
	Summary   Summary `json:"summary"`
	Hash      string  `json:"hash"`
}

type Step struct {
	Index       int       `json:"index"`
	OperationID string    `json:"operation_id"`
	Kind        string    `json:"kind"`
	Table       string    `json:"table"`
	PreHash     string    `json:"pre_hash"`
	PostHash    string    `json:"post_hash,omitempty"`
	Status      string    `json:"status"`
	Rows        []RowPath `json:"rows,omitempty"`
	Error       string    `json:"error,omitempty"`
}

type RowPath struct {
	RowID       string       `json:"row_id"`
	Guard       []Constraint `json:"guard"`
	GuardStatus string       `json:"guard_status"`
	Assignments []Assignment `json:"assignments,omitempty"`
}

type Constraint struct {
	Expression string `json:"expression"`
	Status     string `json:"status"`
}

type Assignment struct {
	Column string `json:"column"`
	Before string `json:"before_symbol"`
	After  string `json:"after_expression"`
}

type Summary struct {
	Steps          int `json:"steps"`
	RowsExplored   int `json:"rows_explored"`
	RowsSatisfying int `json:"rows_satisfying"`
	Assignments    int `json:"assignments"`
	Errors         int `json:"errors"`
}

func Execute(manifest repair.Manifest, store replay.Store) Report {
	working := store.Clone()
	report := Report{
		Version:   Version,
		Manifest:  manifest.Name,
		StoreHash: store.Hash(),
	}
	for index, op := range manifest.Operations {
		step := Step{
			Index:       index,
			OperationID: op.ID,
			Kind:        op.Kind,
			Table:       op.Table,
			PreHash:     working.Hash(),
			Status:      "normal",
		}
		step.Rows = symbolicRows(op, working)
		one := manifest
		one.Operations = []repair.Operation{op}
		_, next, err := replay.Apply(one, nil, working)
		if err != nil {
			step.Status = "stuck"
			step.Error = err.Error()
			report.Summary.Errors++
		} else {
			working = next
			step.PostHash = working.Hash()
		}
		for _, row := range step.Rows {
			report.Summary.RowsExplored++
			if row.GuardStatus == "satisfied" {
				report.Summary.RowsSatisfying++
			}
			report.Summary.Assignments += len(row.Assignments)
		}
		report.Steps = append(report.Steps, step)
	}
	report.Summary.Steps = len(report.Steps)
	report.Hash = canonical.Hash(struct {
		Version   string  `json:"version"`
		Manifest  string  `json:"manifest"`
		StoreHash string  `json:"store_hash"`
		Steps     []Step  `json:"steps"`
		Summary   Summary `json:"summary"`
	}{report.Version, report.Manifest, report.StoreHash, report.Steps, report.Summary})
	return report
}

func symbolicRows(op repair.Operation, store replay.Store) []RowPath {
	switch op.Kind {
	case "insert":
		return []RowPath{insertPath(op)}
	case "update", "delete":
		rows := store.Tables[op.Table]
		out := make([]RowPath, 0, len(rows))
		for _, rowID := range sortedRowIDs(rows) {
			out = append(out, rowPath(op, rowID, rows[rowID]))
		}
		return out
	default:
		return nil
	}
}

func rowPath(op repair.Operation, rowID string, row replay.Row) RowPath {
	path := RowPath{
		RowID:       rowID,
		Guard:       guardConstraints(op, rowID, row),
		GuardStatus: "satisfied",
	}
	for _, constraint := range path.Guard {
		if constraint.Status != "satisfied" {
			path.GuardStatus = "unsatisfied"
			break
		}
	}
	if path.GuardStatus != "satisfied" {
		return path
	}
	switch op.Kind {
	case "update":
		for _, column := range sortedKeys(op.Set) {
			path.Assignments = append(path.Assignments, Assignment{
				Column: column,
				Before: symbol(op.Table, rowID, column),
				After:  fmt.Sprintf("%q", op.Set[column]),
			})
		}
	case "delete":
		path.Assignments = append(path.Assignments, Assignment{
			Column: "*",
			Before: symbol(op.Table, rowID, "*"),
			After:  "deleted",
		})
	}
	return path
}

func insertPath(op repair.Operation) RowPath {
	rowID := op.Set["id"]
	if rowID == "" {
		rowID = "<missing-id>"
	}
	path := RowPath{
		RowID:       rowID,
		GuardStatus: "satisfied",
		Guard: []Constraint{{
			Expression: fmt.Sprintf("not exists %s.%s", op.Table, rowID),
			Status:     "assumed",
		}},
	}
	for _, column := range sortedKeys(op.Set) {
		path.Assignments = append(path.Assignments, Assignment{
			Column: column,
			Before: "undefined",
			After:  fmt.Sprintf("%q", op.Set[column]),
		})
	}
	return path
}

func guardConstraints(op repair.Operation, rowID string, row replay.Row) []Constraint {
	var out []Constraint
	for _, column := range sortedKeys(op.Where) {
		status := "satisfied"
		if row[column] != op.Where[column] {
			status = "unsatisfied"
		}
		out = append(out, Constraint{
			Expression: fmt.Sprintf("%s == %q", symbol(op.Table, rowID, column), op.Where[column]),
			Status:     status,
		})
	}
	return out
}

func symbol(table, rowID, column string) string {
	parts := []string{table, rowID, column}
	for i, part := range parts {
		part = strings.ReplaceAll(part, "-", "_")
		part = strings.ReplaceAll(part, ":", "_")
		part = strings.ReplaceAll(part, "/", "_")
		parts[i] = part
	}
	return strings.Join(parts, ".")
}

func sortedRowIDs(rows map[string]replay.Row) []string {
	ids := make([]string, 0, len(rows))
	for id := range rows {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

func sortedKeys(in map[string]string) []string {
	keys := make([]string, 0, len(in))
	for key := range in {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
