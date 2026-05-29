package repair

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"

	"github.com/thehalleyyoung/patchline/internal/effects"
	"github.com/thehalleyyoung/patchline/internal/provenance"
)

const Version = "patchline.repair/v1"

type Manifest struct {
	Version        string      `json:"version"`
	Name           string      `json:"name"`
	Incident       string      `json:"incident"`
	Scope          Scope       `json:"scope"`
	Preconditions  []Check     `json:"preconditions,omitempty"`
	Operations     []Operation `json:"operations"`
	Postconditions []Check     `json:"postconditions,omitempty"`
	Rollback       Rollback    `json:"rollback"`
}

type Scope struct {
	Entities []string          `json:"entities,omitempty"`
	Table    string            `json:"table,omitempty"`
	Where    map[string]string `json:"where,omitempty"`
}

type Operation struct {
	ID        string            `json:"id"`
	Kind      string            `json:"kind"`
	Table     string            `json:"table,omitempty"`
	Where     map[string]string `json:"where,omitempty"`
	Set       map[string]string `json:"set,omitempty"`
	DependsOn []string          `json:"depends_on,omitempty"`
}

type Check struct {
	Kind   string `json:"kind"`
	Expr   string `json:"expr"`
	Expect string `json:"expect"`
}

type Rollback struct {
	Strategy         string `json:"strategy"`
	SnapshotRequired bool   `json:"snapshot_required"`
}

type Diagnostic struct {
	Level   string `json:"level"`
	Code    string `json:"code"`
	Message string `json:"message"`
	Ref     string `json:"ref,omitempty"`
}

func ReadManifest(reader io.Reader) (Manifest, error) {
	decoder := json.NewDecoder(reader)
	decoder.DisallowUnknownFields()
	var manifest Manifest
	if err := decoder.Decode(&manifest); err != nil {
		return Manifest{}, err
	}
	return manifest, nil
}

func Validate(manifest Manifest, graph *provenance.Graph) []Diagnostic {
	var diagnostics []Diagnostic
	add := func(level, code, message, ref string) {
		diagnostics = append(diagnostics, Diagnostic{Level: level, Code: code, Message: message, Ref: ref})
	}

	if manifest.Version != Version {
		add("error", "manifest.version", fmt.Sprintf("version must be %s", Version), manifest.Version)
	}
	if manifest.Name == "" {
		add("error", "manifest.name", "name is required", "")
	}
	if manifest.Incident == "" {
		add("error", "manifest.incident", "incident is required", "")
	}
	if len(manifest.Scope.Entities) == 0 && manifest.Scope.Table == "" {
		add("warning", "scope.empty", "scope has no graph entities or table", "")
	}
	if graph != nil {
		for _, entityID := range manifest.Scope.Entities {
			if _, ok := graph.Entity(entityID); !ok {
				add("error", "scope.entity_missing", "scope entity does not exist in provenance graph", entityID)
			}
		}
	}

	operationIDs := map[string]bool{}
	for _, op := range manifest.Operations {
		if op.ID == "" {
			add("error", "operation.id", "operation id is required", "")
			continue
		}
		if operationIDs[op.ID] {
			add("error", "operation.duplicate", "operation id is duplicated", op.ID)
		}
		operationIDs[op.ID] = true
		validateOperation(op, manifest, add)
	}
	if len(manifest.Operations) == 0 {
		add("error", "operations.empty", "at least one operation is required", "")
	}
	for _, op := range manifest.Operations {
		for _, dep := range op.DependsOn {
			if !operationIDs[dep] {
				add("error", "operation.dependency_missing", "operation depends on an unknown operation", op.ID+" -> "+dep)
			}
		}
	}
	if cycle := dependencyCycle(manifest.Operations); len(cycle) > 0 {
		add("error", "operation.dependency_cycle", "operation dependencies contain a cycle", joinCycle(cycle))
	}

	sort.SliceStable(diagnostics, func(i, j int) bool {
		if diagnostics[i].Level != diagnostics[j].Level {
			return diagnostics[i].Level < diagnostics[j].Level
		}
		if diagnostics[i].Code != diagnostics[j].Code {
			return diagnostics[i].Code < diagnostics[j].Code
		}
		return diagnostics[i].Ref < diagnostics[j].Ref
	})
	return diagnostics
}

func HasErrors(diagnostics []Diagnostic) bool {
	for _, diagnostic := range diagnostics {
		if diagnostic.Level == "error" {
			return true
		}
	}
	return false
}

func validateOperation(op Operation, manifest Manifest, add func(string, string, string, string)) {
	classification := effects.Infer(effects.Mutation{
		Kind:                op.Kind,
		Table:               op.Table,
		WhereKeys:           keys(op.Where),
		SetKeys:             keys(op.Set),
		HasSnapshotRollback: manifest.Rollback.Strategy == "snapshot" && manifest.Rollback.SnapshotRequired,
	})
	if classification.Effect == effects.EffectUnknown {
		add("error", "operation.kind", "operation kind is not supported", op.ID)
	}
	if effects.IsRisky(classification.Effect) && manifest.Rollback.Strategy != "snapshot" {
		add("error", "operation.risky_without_snapshot", "risky operation requires snapshot rollback", op.ID)
	}

	switch op.Kind {
	case "insert":
		if op.Table == "" {
			add("error", "operation.table", "insert operation requires a table", op.ID)
		}
		if len(op.Set) == 0 {
			add("error", "operation.set", "insert operation requires row values", op.ID)
		}
	case "update":
		if op.Table == "" {
			add("error", "operation.table", "update operation requires a table", op.ID)
		}
		if len(op.Where) == 0 {
			add("error", "operation.where", "update operation requires a predicate", op.ID)
		}
		if len(op.Set) == 0 {
			add("error", "operation.set", "update operation requires assigned columns", op.ID)
		}
	case "delete":
		if op.Table == "" || len(op.Where) == 0 {
			add("error", "operation.delete_scope", "delete operation requires table and predicate", op.ID)
		}
	case "append-log", "emit-event", "enqueue":
		if op.Table == "" && op.Set["target"] == "" {
			add("error", "operation.external_target", "external operation requires a table/stream target or set.target", op.ID)
		}
	}

	if manifest.Scope.Table != "" && op.Table != "" && manifest.Scope.Table != op.Table {
		add("warning", "operation.scope_table_mismatch", "operation table differs from scope table", op.ID)
	}
	if manifest.Scope.Table != "" && manifest.Scope.Table == op.Table && !operationContainsScope(operationScopeValues(op), manifest.Scope.Where) {
		add("error", "operation.escapes_scope", "operation predicate does not contain the declared scope predicate", op.ID)
	}
}

func operationScopeValues(op Operation) map[string]string {
	values := map[string]string{}
	for key, value := range op.Set {
		values[key] = value
	}
	for key, value := range op.Where {
		values[key] = value
	}
	return values
}

func operationContainsScope(operationWhere, scopeWhere map[string]string) bool {
	for key, value := range scopeWhere {
		if operationWhere[key] != value {
			return false
		}
	}
	return true
}

func dependencyCycle(ops []Operation) []string {
	graph := map[string][]string{}
	for _, op := range ops {
		graph[op.ID] = append([]string(nil), op.DependsOn...)
	}
	visiting := map[string]bool{}
	visited := map[string]bool{}
	var stack []string

	var visit func(string) []string
	visit = func(id string) []string {
		if visiting[id] {
			for i, entry := range stack {
				if entry == id {
					return append(append([]string(nil), stack[i:]...), id)
				}
			}
			return []string{id, id}
		}
		if visited[id] {
			return nil
		}
		visiting[id] = true
		stack = append(stack, id)
		for _, dep := range graph[id] {
			if cycle := visit(dep); len(cycle) > 0 {
				return cycle
			}
		}
		stack = stack[:len(stack)-1]
		visiting[id] = false
		visited[id] = true
		return nil
	}

	ids := make([]string, 0, len(graph))
	for id := range graph {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		if cycle := visit(id); len(cycle) > 0 {
			return cycle
		}
	}
	return nil
}

func keys(in map[string]string) []string {
	out := make([]string, 0, len(in))
	for key := range in {
		out = append(out, key)
	}
	sort.Strings(out)
	return out
}

func joinCycle(cycle []string) string {
	out := ""
	for i, entry := range cycle {
		if i > 0 {
			out += " -> "
		}
		out += entry
	}
	return out
}
