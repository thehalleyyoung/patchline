package repair

import (
	"fmt"
	"sort"
	"strings"

	"github.com/thehalleyyoung/patchline/internal/canonical"
)

type TransactionPlan struct {
	Version        string         `json:"version"`
	Manifest       string         `json:"manifest"`
	Incident       string         `json:"incident"`
	LockOrder      []string       `json:"lock_order"`
	OperationOrder []string       `json:"operation_order"`
	Statements     []SQLStatement `json:"statements"`
	Hash           string         `json:"hash"`
}

func GenerateTransactionPlan(manifest Manifest) (TransactionPlan, error) {
	sqlPlan, err := GenerateSQL(manifest)
	if err != nil {
		return TransactionPlan{}, err
	}
	operations, err := orderedOperations(manifest.Operations)
	if err != nil {
		return TransactionPlan{}, err
	}
	statementByOperation := map[string]SQLStatement{}
	for _, statement := range sqlPlan.Statements {
		statementByOperation[statement.OperationID] = statement
	}
	plan := TransactionPlan{
		Version:  "patchline.transaction-plan/v1",
		Manifest: manifest.Name,
		Incident: manifest.Incident,
	}
	tables := tableSet(manifest.Operations)
	plan.LockOrder = tables
	plan.Statements = append(plan.Statements, SQLStatement{OperationID: "transaction", Kind: "begin", SQL: "BEGIN;"})
	if len(tables) > 0 {
		quoted := make([]string, 0, len(tables))
		for _, table := range tables {
			quoted = append(quoted, quoteIdent(table))
		}
		plan.Statements = append(plan.Statements, SQLStatement{
			OperationID: "transaction",
			Kind:        "lock",
			SQL:         "LOCK TABLE " + strings.Join(quoted, ", ") + " IN ROW EXCLUSIVE MODE;",
		})
	}
	for _, op := range operations {
		statement, ok := statementByOperation[op.ID]
		if !ok {
			return TransactionPlan{}, fmt.Errorf("operation %s has no generated SQL statement", op.ID)
		}
		plan.OperationOrder = append(plan.OperationOrder, op.ID)
		plan.Statements = append(plan.Statements, statement)
	}
	plan.Statements = append(plan.Statements, SQLStatement{OperationID: "transaction", Kind: "commit", SQL: "COMMIT;"})
	plan.Hash = canonical.Hash(struct {
		Version        string         `json:"version"`
		Manifest       string         `json:"manifest"`
		Incident       string         `json:"incident"`
		LockOrder      []string       `json:"lock_order"`
		OperationOrder []string       `json:"operation_order"`
		Statements     []SQLStatement `json:"statements"`
	}{plan.Version, plan.Manifest, plan.Incident, plan.LockOrder, plan.OperationOrder, plan.Statements})
	return plan, nil
}

func tableSet(operations []Operation) []string {
	seen := map[string]bool{}
	for _, op := range operations {
		if op.Table != "" {
			seen[op.Table] = true
		}
	}
	tables := make([]string, 0, len(seen))
	for table := range seen {
		tables = append(tables, table)
	}
	sort.Strings(tables)
	return tables
}

func orderedOperations(operations []Operation) ([]Operation, error) {
	byID := map[string]Operation{}
	dependents := map[string][]string{}
	inDegree := map[string]int{}
	for _, op := range operations {
		byID[op.ID] = op
		if _, ok := inDegree[op.ID]; !ok {
			inDegree[op.ID] = 0
		}
		for _, dep := range op.DependsOn {
			dependents[dep] = append(dependents[dep], op.ID)
			inDegree[op.ID]++
		}
	}
	ready := make([]string, 0, len(inDegree))
	for id, degree := range inDegree {
		if degree == 0 {
			ready = append(ready, id)
		}
	}
	sort.Strings(ready)
	var ordered []Operation
	for len(ready) > 0 {
		id := ready[0]
		ready = ready[1:]
		ordered = append(ordered, byID[id])
		sort.Strings(dependents[id])
		for _, next := range dependents[id] {
			inDegree[next]--
			if inDegree[next] == 0 {
				ready = append(ready, next)
				sort.Strings(ready)
			}
		}
	}
	if len(ordered) != len(operations) {
		return nil, fmt.Errorf("operation dependencies contain a cycle")
	}
	return ordered, nil
}
