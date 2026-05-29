package solver

import (
	"context"
	"fmt"
	"os/exec"
	"sort"
	"strings"
	"time"

	"github.com/thehalleyyoung/patchline/internal/canonical"
	"github.com/thehalleyyoung/patchline/internal/invariant"
	"github.com/thehalleyyoung/patchline/internal/repair"
	"github.com/thehalleyyoung/patchline/internal/replay"
)

const Version = "patchline.solver-obligations/v1"

type Status string

const (
	StatusProved         Status = "proved"
	StatusChecked        Status = "checked"
	StatusCounterexample Status = "counterexample"
	StatusAssumed        Status = "assumed"
	StatusNotSupported   Status = "not_supported"
)

type Report struct {
	Version           string             `json:"version"`
	Manifest          string             `json:"manifest"`
	StoreHash         string             `json:"store_hash"`
	SolverEngine      string             `json:"solver_engine"`
	SolverVersion     string             `json:"solver_version,omitempty"`
	ScopeImplications []ImplicationCheck `json:"scope_implications"`
	FrameChecks       []FrameCheck       `json:"frame_checks"`
	RowCountChecks    []RowCountCheck    `json:"row_count_checks"`
	InvariantChecks   []InvariantCheck   `json:"invariant_checks,omitempty"`
	Summary           Summary            `json:"summary"`
	Hash              string             `json:"hash"`
}

type ImplicationCheck struct {
	OperationID    string            `json:"operation_id"`
	Status         Status            `json:"status"`
	Antecedent     map[string]string `json:"antecedent,omitempty"`
	Consequent     map[string]string `json:"consequent,omitempty"`
	SMTLIB         string            `json:"smtlib"`
	Solver         string            `json:"solver"`
	SolverResult   string            `json:"solver_result,omitempty"`
	Counterexample map[string]string `json:"counterexample,omitempty"`
	Reason         string            `json:"reason"`
}

type FrameCheck struct {
	OperationID      string   `json:"operation_id"`
	Status           Status   `json:"status"`
	Table            string   `json:"table"`
	MayWriteColumns  []string `json:"may_write_columns,omitempty"`
	ProtectedColumns []string `json:"protected_columns,omitempty"`
	Reason           string   `json:"reason"`
}

type RowCountCheck struct {
	OperationID string `json:"operation_id"`
	Status      Status `json:"status"`
	Table       string `json:"table"`
	MatchedRows int    `json:"matched_rows"`
	UpperBound  int    `json:"upper_bound"`
	SMTLIB      string `json:"smtlib"`
	Reason      string `json:"reason"`
}

type InvariantCheck struct {
	ID           string `json:"id"`
	Status       Status `json:"status"`
	BeforeStatus string `json:"before_status"`
	AfterStatus  string `json:"after_status"`
	Support      int    `json:"support"`
	Witness      string `json:"witness,omitempty"`
	Reason       string `json:"reason"`
}

type Summary struct {
	Proved          int `json:"proved"`
	Checked         int `json:"checked"`
	Counterexamples int `json:"counterexamples"`
	Assumed         int `json:"assumed"`
	NotSupported    int `json:"not_supported"`
}

func Analyze(manifest repair.Manifest, store replay.Store, spec *invariant.Spec) Report {
	solverVersion := z3Version()
	report := Report{
		Version:       Version,
		Manifest:      manifest.Name,
		StoreHash:     store.Hash(),
		SolverEngine:  "z3",
		SolverVersion: solverVersion,
	}
	for _, op := range sortedOperations(manifest.Operations) {
		report.ScopeImplications = append(report.ScopeImplications, proveImplication(op, manifest.Scope))
		report.FrameChecks = append(report.FrameChecks, proveFrame(op, manifest.Scope))
		report.RowCountChecks = append(report.RowCountChecks, checkRowCount(op, store))
	}
	if spec != nil {
		report.InvariantChecks = checkInvariants(manifest, store, *spec)
	}
	report.Summary = summarize(report)
	report.Hash = canonical.Hash(struct {
		Version           string             `json:"version"`
		Manifest          string             `json:"manifest"`
		StoreHash         string             `json:"store_hash"`
		SolverEngine      string             `json:"solver_engine"`
		SolverVersion     string             `json:"solver_version,omitempty"`
		ScopeImplications []ImplicationCheck `json:"scope_implications"`
		FrameChecks       []FrameCheck       `json:"frame_checks"`
		RowCountChecks    []RowCountCheck    `json:"row_count_checks"`
		InvariantChecks   []InvariantCheck   `json:"invariant_checks,omitempty"`
		Summary           Summary            `json:"summary"`
	}{report.Version, report.Manifest, report.StoreHash, report.SolverEngine, report.SolverVersion, report.ScopeImplications, report.FrameChecks, report.RowCountChecks, report.InvariantChecks, report.Summary})
	return report
}

func proveImplication(op repair.Operation, scope repair.Scope) ImplicationCheck {
	check := ImplicationCheck{
		OperationID: op.ID,
		Antecedent:  copyMap(op.Where),
		Consequent:  copyMap(scope.Where),
		SMTLIB:      implicationSMT(op.Where, scope.Where),
		Solver:      "z3",
	}
	if op.Table == "" || scope.Table == "" {
		check.Status = StatusNotSupported
		check.Reason = "operation and scope tables are required for predicate implication"
		return check
	}
	if op.Table != scope.Table {
		check.Status = StatusCounterexample
		check.Counterexample = map[string]string{"operation_table": op.Table, "scope_table": scope.Table}
		check.Reason = "operation table differs from declared scope table"
		return check
	}
	if len(scope.Where) == 0 {
		check.Status = StatusAssumed
		check.Reason = "declared scope has no predicate to prove"
		return check
	}
	result, err := runZ3(check.SMTLIB)
	if err != nil {
		check.Status = StatusAssumed
		check.SolverResult = "unavailable"
		check.Reason = "z3 did not discharge the obligation: " + err.Error()
		return check
	}
	check.SolverResult = result
	switch result {
	case "unsat":
		check.Status = StatusProved
		check.Reason = "z3 proved unsat(operation predicate and not scope predicate) in quantifier-free string equality"
	case "sat":
		check.Status = StatusCounterexample
		check.Counterexample = deterministicImplicationCounterexample(op.Where, scope.Where)
		check.Reason = "z3 found operation predicate does not imply declared scope predicate"
	default:
		check.Status = StatusAssumed
		check.Reason = "z3 returned " + result + " for predicate implication"
	}
	return check
}

func runZ3(smtlib string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "z3", "-in", "-smt2")
	cmd.Stdin = strings.NewReader(smtlib)
	output, err := cmd.CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		return "", fmt.Errorf("timeout after 2s")
	}
	if err != nil {
		return "", fmt.Errorf("%v: %s", err, strings.TrimSpace(string(output)))
	}
	fields := strings.Fields(string(output))
	if len(fields) == 0 {
		return "", fmt.Errorf("empty z3 output")
	}
	switch fields[0] {
	case "sat", "unsat", "unknown":
		return fields[0], nil
	default:
		return "", fmt.Errorf("unexpected z3 output %q", fields[0])
	}
}

func z3Version() string {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	output, err := exec.CommandContext(ctx, "z3", "--version").CombinedOutput()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(output))
}

func deterministicImplicationCounterexample(antecedent, consequent map[string]string) map[string]string {
	out := copyMap(antecedent)
	if out == nil {
		out = map[string]string{}
	}
	for _, key := range sortedKeys(consequent) {
		if got, ok := antecedent[key]; !ok || got != consequent[key] {
			out["violates_scope_"+key] = consequent[key]
			if ok {
				out["actual_"+key] = got
			}
		}
	}
	return out
}

func proveFrame(op repair.Operation, scope repair.Scope) FrameCheck {
	check := FrameCheck{
		OperationID:      op.ID,
		Table:            op.Table,
		MayWriteColumns:  sortedKeys(op.Set),
		ProtectedColumns: protectedColumns(scope.Where, op.Set),
	}
	if op.Kind == "insert" || op.Kind == "delete" {
		check.MayWriteColumns = []string{"*"}
		check.Status = StatusChecked
		check.Reason = "row-level insert/delete frame is bounded by predicate/table checks"
		return check
	}
	if op.Kind != "update" {
		check.Status = StatusNotSupported
		check.Reason = "only row-level insert/update/delete operations have bounded frame checks"
		return check
	}
	if len(op.Set) == 0 {
		check.Status = StatusCounterexample
		check.Reason = "update has no explicit assignment set"
		return check
	}
	for key := range scope.Where {
		if _, writesScopeColumn := op.Set[key]; writesScopeColumn {
			check.Status = StatusCounterexample
			check.Reason = "operation writes a scope predicate column, so row-frame stability is not proved"
			return check
		}
	}
	check.Status = StatusProved
	check.Reason = "assignment set is disjoint from scope predicate columns"
	return check
}

func checkRowCount(op repair.Operation, store replay.Store) RowCountCheck {
	check := RowCountCheck{
		OperationID: op.ID,
		Table:       op.Table,
		UpperBound:  inferredUpperBound(op, store),
		SMTLIB:      rowCountSMT(op),
	}
	rows := store.Tables[op.Table]
	for _, id := range sortedRowIDs(rows) {
		if matches(rows[id], op.Where) {
			check.MatchedRows++
		}
	}
	if check.UpperBound < 0 {
		check.Status = StatusAssumed
		check.Reason = "no finite row-count upper bound inferred beyond concrete store enumeration"
		return check
	}
	if check.MatchedRows <= check.UpperBound {
		check.Status = StatusChecked
		check.Reason = "bounded store enumeration satisfies inferred row-count upper bound"
		return check
	}
	check.Status = StatusCounterexample
	check.Reason = "bounded store enumeration exceeds inferred row-count upper bound"
	return check
}

func checkInvariants(manifest repair.Manifest, store replay.Store, spec invariant.Spec) []InvariantCheck {
	before := invariant.CheckStore(spec, store)
	_, afterStore, err := replay.Apply(manifest, nil, store)
	if err != nil {
		return []InvariantCheck{{
			ID:           "replay.apply",
			Status:       StatusCounterexample,
			BeforeStatus: statusFromOK(before.OK),
			AfterStatus:  "error",
			Witness:      err.Error(),
			Reason:       "repair could not be applied to bounded store",
		}}
	}
	after := invariant.CheckStore(spec, afterStore)
	beforeByID := mapChecks(before.Checks)
	afterByID := mapChecks(after.Checks)
	var out []InvariantCheck
	for _, declaration := range sortedDeclarations(spec.Invariants) {
		b := beforeByID[declaration.ID]
		a := afterByID[declaration.ID]
		check := InvariantCheck{
			ID:           declaration.ID,
			BeforeStatus: b.Status,
			AfterStatus:  a.Status,
			Support:      a.Support,
		}
		switch {
		case b.Status == "checked" && a.Status == "checked":
			check.Status = StatusChecked
			check.Reason = "invariant checked before and after bounded repair replay"
		case b.Status == "checked" && a.Status == "refuted":
			check.Status = StatusCounterexample
			check.Reason = "repair replay refuted an invariant that held before"
			check.Witness = firstInvariantWitness(a)
		case b.Status == "unsupported" || a.Status == "unsupported":
			check.Status = StatusNotSupported
			check.Reason = "invariant kind is outside the bounded checker fragment"
		default:
			check.Status = StatusAssumed
			check.Reason = "invariant was not established before replay, so preservation is not claimed"
			check.Witness = firstInvariantWitness(b)
		}
		out = append(out, check)
	}
	return out
}

func summarize(report Report) Summary {
	var summary Summary
	add := func(status Status) {
		switch status {
		case StatusProved:
			summary.Proved++
		case StatusChecked:
			summary.Checked++
		case StatusCounterexample:
			summary.Counterexamples++
		case StatusAssumed:
			summary.Assumed++
		case StatusNotSupported:
			summary.NotSupported++
		}
	}
	for _, check := range report.ScopeImplications {
		add(check.Status)
	}
	for _, check := range report.FrameChecks {
		add(check.Status)
	}
	for _, check := range report.RowCountChecks {
		add(check.Status)
	}
	for _, check := range report.InvariantChecks {
		add(check.Status)
	}
	return summary
}

func implicationSMT(antecedent, consequent map[string]string) string {
	var b strings.Builder
	keys := sortedUnionKeys(antecedent, consequent)
	vars := map[string]string{}
	for index, key := range keys {
		vars[key] = fmt.Sprintf("v%d", index)
	}
	b.WriteString("(set-logic QF_S)\n")
	b.WriteString("(set-option :timeout 2000)\n")
	for _, key := range keys {
		b.WriteString(fmt.Sprintf("(declare-const %s String)\n", vars[key]))
	}
	b.WriteString("(assert ")
	b.WriteString(smtAndWithVars(antecedent, vars))
	b.WriteString(")\n(assert (not ")
	b.WriteString(smtAndWithVars(consequent, vars))
	b.WriteString("))\n(check-sat)")
	return b.String()
}

func rowCountSMT(op repair.Operation) string {
	return fmt.Sprintf("; finite-store row-count check for %s\n; count rows in %s satisfying %s\n(check-sat)", op.ID, op.Table, smtAnd(op.Where))
}

func smtAnd(pred map[string]string) string {
	return smtAndWithVars(pred, nil)
}

func smtAndWithVars(pred map[string]string, vars map[string]string) string {
	if len(pred) == 0 {
		return "true"
	}
	parts := make([]string, 0, len(pred))
	for _, key := range sortedKeys(pred) {
		ident := smtIdent(key)
		if vars != nil {
			ident = vars[key]
		}
		parts = append(parts, fmt.Sprintf("(= %s %s)", ident, smtString(pred[key])))
	}
	if len(parts) == 1 {
		return parts[0]
	}
	return "(and " + strings.Join(parts, " ") + ")"
}

func smtIdent(value string) string {
	value = strings.ReplaceAll(value, "-", "_")
	return strings.ReplaceAll(value, ".", "_")
}

func smtString(value string) string {
	return `"` + strings.ReplaceAll(value, `"`, `""`) + `"`
}

func inferredUpperBound(op repair.Operation, store replay.Store) int {
	if op.Kind == "insert" {
		return 1
	}
	if id := op.Where["id"]; id != "" {
		if _, ok := store.Tables[op.Table][id]; ok {
			return 1
		}
		return 0
	}
	return -1
}

func matches(row replay.Row, predicate map[string]string) bool {
	for key, want := range predicate {
		if row[key] != want {
			return false
		}
	}
	return true
}

func protectedColumns(scopeWhere, set map[string]string) []string {
	var out []string
	for _, key := range sortedKeys(scopeWhere) {
		if _, written := set[key]; !written {
			out = append(out, key)
		}
	}
	return out
}

func mapChecks(checks []invariant.Check) map[string]invariant.Check {
	out := map[string]invariant.Check{}
	for _, check := range checks {
		out[check.ID] = check
	}
	return out
}

func firstInvariantWitness(check invariant.Check) string {
	if len(check.Counterexamples) == 0 {
		return ""
	}
	witness := check.Counterexamples[0]
	return strings.Join([]string{witness.Table, witness.RowID, witness.Column, witness.Value, witness.Message}, ":")
}

func statusFromOK(ok bool) string {
	if ok {
		return "checked"
	}
	return "refuted"
}

func sortedOperations(ops []repair.Operation) []repair.Operation {
	out := append([]repair.Operation(nil), ops...)
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func sortedDeclarations(declarations []invariant.Declaration) []invariant.Declaration {
	out := append([]invariant.Declaration(nil), declarations...)
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func sortedRowIDs(rows map[string]replay.Row) []string {
	ids := make([]string, 0, len(rows))
	for id := range rows {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

func sortedUnionKeys(left, right map[string]string) []string {
	seen := map[string]bool{}
	for key := range left {
		seen[key] = true
	}
	for key := range right {
		seen[key] = true
	}
	keys := make([]string, 0, len(seen))
	for key := range seen {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func sortedKeys(in map[string]string) []string {
	keys := make([]string, 0, len(in))
	for key := range in {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
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
