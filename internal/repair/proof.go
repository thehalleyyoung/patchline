package repair

import (
	"fmt"
	"sort"
	"strings"

	"github.com/thehalleyyoung/patchline/internal/canonical"
)

const ProofVersion = "patchline.repair-proof/v1"

type ProofStatus string

const (
	ProofChecked     ProofStatus = "checked"
	ProofAssumed     ProofStatus = "assumed"
	ProofUnsupported ProofStatus = "unsupported"
	ProofRefuted     ProofStatus = "refuted"
)

type ProofReport struct {
	Version              string                `json:"version"`
	OK                   bool                  `json:"ok"`
	Manifest             string                `json:"manifest"`
	Incident             string                `json:"incident"`
	HoareTriple          HoareTriple           `json:"hoare_triple"`
	WeakestPreconditions []ProofObligation     `json:"weakest_preconditions"`
	FrameConditions      []FrameCondition      `json:"frame_conditions"`
	RefinementChecks     []RefinementCheck     `json:"refinement_checks"`
	Counterexamples      []ProofCounterexample `json:"counterexamples,omitempty"`
	Findings             []Finding             `json:"findings,omitempty"`
	Hash                 string                `json:"hash"`
}

type HoareTriple struct {
	Precondition  []string          `json:"precondition"`
	Command       []string          `json:"command"`
	Postcondition []string          `json:"postcondition"`
	Obligations   []ProofObligation `json:"obligations"`
	Notation      string            `json:"notation"`
}

type ProofObligation struct {
	Ref     string      `json:"ref"`
	Status  ProofStatus `json:"status"`
	Formula string      `json:"formula"`
	Reason  string      `json:"reason"`
}

type FrameCondition struct {
	Ref             string      `json:"ref"`
	Status          ProofStatus `json:"status"`
	Table           string      `json:"table"`
	MayWriteRows    []string    `json:"may_write_rows,omitempty"`
	MayWriteColumns []string    `json:"may_write_columns,omitempty"`
	MustNotTouch    []string    `json:"must_not_touch,omitempty"`
	Reason          string      `json:"reason"`
}

type RefinementCheck struct {
	OperationID string      `json:"operation_id"`
	Status      ProofStatus `json:"status"`
	Abstract    string      `json:"abstract"`
	Concrete    string      `json:"concrete"`
	Reason      string      `json:"reason"`
}

type ProofCounterexample struct {
	Ref     string `json:"ref"`
	Message string `json:"message"`
	Witness string `json:"witness,omitempty"`
}

func BuildProof(manifest Manifest) ProofReport {
	lint := Lint(manifest)
	plan, sqlErr := GenerateSQL(manifest)
	report := ProofReport{
		Version:  ProofVersion,
		OK:       lint.OK && sqlErr == nil,
		Manifest: manifest.Name,
		Incident: manifest.Incident,
		Findings: lint.Findings,
	}
	report.HoareTriple = buildHoareTriple(manifest)
	report.WeakestPreconditions = weakestPreconditions(manifest)
	report.FrameConditions = frameConditions(manifest)
	if sqlErr != nil {
		report.Counterexamples = append(report.Counterexamples, ProofCounterexample{
			Ref:     "sql.generate",
			Message: sqlErr.Error(),
		})
		report.RefinementChecks = refinementFailures(manifest, sqlErr)
	} else {
		report.RefinementChecks = refinementChecks(manifest, plan)
	}
	report.Counterexamples = append(report.Counterexamples, proofCounterexamples(report.WeakestPreconditions, report.FrameConditions, report.RefinementChecks)...)
	report.OK = report.OK && len(report.Counterexamples) == 0
	report.Hash = canonical.Hash(struct {
		Version              string                `json:"version"`
		OK                   bool                  `json:"ok"`
		Manifest             string                `json:"manifest"`
		Incident             string                `json:"incident"`
		HoareTriple          HoareTriple           `json:"hoare_triple"`
		WeakestPreconditions []ProofObligation     `json:"weakest_preconditions"`
		FrameConditions      []FrameCondition      `json:"frame_conditions"`
		RefinementChecks     []RefinementCheck     `json:"refinement_checks"`
		Counterexamples      []ProofCounterexample `json:"counterexamples,omitempty"`
	}{
		Version:              report.Version,
		OK:                   report.OK,
		Manifest:             report.Manifest,
		Incident:             report.Incident,
		HoareTriple:          report.HoareTriple,
		WeakestPreconditions: report.WeakestPreconditions,
		FrameConditions:      report.FrameConditions,
		RefinementChecks:     report.RefinementChecks,
		Counterexamples:      report.Counterexamples,
	})
	return report
}

func buildHoareTriple(manifest Manifest) HoareTriple {
	pre := []string{"evidence_graph_scope(" + manifest.Incident + ")"}
	for _, check := range manifest.Preconditions {
		pre = append(pre, check.Kind+":"+check.Expr+" == "+check.Expect)
	}
	var command []string
	for _, op := range manifest.Operations {
		command = append(command, op.ID+":"+op.Kind+"("+op.Table+")")
	}
	post := []string{"frame_condition_preserved"}
	for _, check := range manifest.Postconditions {
		post = append(post, check.Kind+":"+check.Expr+" == "+check.Expect)
	}
	sort.Strings(pre)
	sort.Strings(command)
	sort.Strings(post)
	return HoareTriple{
		Precondition:  pre,
		Command:       command,
		Postcondition: post,
		Obligations: []ProofObligation{{
			Ref:     "hoare.scope",
			Status:  ProofChecked,
			Formula: "operations respect declared manifest scope",
			Reason:  "manifest validation rejects operations that escape table scope predicates",
		}, {
			Ref:     "hoare.postconditions",
			Status:  assumedIf(len(manifest.Postconditions) > 0),
			Formula: "postconditions are checked after execution",
			Reason:  "SQL and graph postconditions are external obligations until replay/invariant engines discharge them",
		}},
		Notation: "{historical evidence + preconditions} repair {postconditions + frame obligations}",
	}
}

func weakestPreconditions(manifest Manifest) []ProofObligation {
	var out []ProofObligation
	if len(manifest.Preconditions) == 0 {
		out = append(out, ProofObligation{
			Ref:     "wp.preconditions.declared",
			Status:  ProofUnsupported,
			Formula: "len(preconditions) > 0",
			Reason:  "repair has no explicit guard proving the historical state still matches the incident",
		})
	}
	for _, op := range manifest.Operations {
		base := "wp." + op.ID
		switch op.Kind {
		case "update":
			out = append(out,
				obligation(base+".row_exists", ProofAssumed, rowPredicateFormula(op), "operator/database must prove the scoped rows exist before update"),
				obligation(base+".has_assignment", checked(len(op.Set) > 0), fmt.Sprintf("set_keys=%s", strings.Join(sortedKeys(op.Set), ",")), "update has at least one deterministic assignment"),
				obligation(base+".scoped", checked(len(op.Where) > 0), predicateFormula(op.Where), "update has a deterministic predicate"),
			)
		case "insert":
			out = append(out,
				obligation(base+".row_absent", ProofAssumed, uniquenessFormula(op), "insert is safe only if the inserted identifier is not already present"),
				obligation(base+".has_values", checked(len(op.Set) > 0), fmt.Sprintf("set_keys=%s", strings.Join(sortedKeys(op.Set), ",")), "insert carries explicit row values"),
			)
		case "delete":
			out = append(out,
				obligation(base+".row_exists", ProofAssumed, rowPredicateFormula(op), "operator/database must prove the scoped rows exist before delete"),
				obligation(base+".scoped", checked(len(op.Where) > 0), predicateFormula(op.Where), "delete has a deterministic predicate"),
			)
		case "replay", "rebuild-index":
			out = append(out, obligation(base+".external_semantics", ProofAssumed, op.Kind+" has replay/idempotence contract", "non-row mutation operation requires external system-specific semantics"))
		case "append-log", "emit-event", "enqueue":
			out = append(out, obligation(base+".compensating_semantics", ProofAssumed, op.Kind+" has a causally linked compensating action", "append-only external operation requires compensating-action semantics"))
		default:
			out = append(out, obligation(base+".supported", ProofRefuted, "kind in supported_operations", "unsupported operation cannot be proven"))
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Ref < out[j].Ref
	})
	return out
}

func frameConditions(manifest Manifest) []FrameCondition {
	var out []FrameCondition
	for _, op := range manifest.Operations {
		frame := FrameCondition{
			Ref:          "frame." + op.ID,
			Status:       checked(op.Table != ""),
			Table:        op.Table,
			MayWriteRows: predicateParts(op.Where),
			Reason:       "frame is derived syntactically from operation table, predicate, and assigned columns",
		}
		switch op.Kind {
		case "update":
			frame.MayWriteColumns = sortedKeys(op.Set)
			frame.MustNotTouch = mustNotTouchColumns(op.Where, op.Set)
		case "insert", "delete":
			frame.MayWriteColumns = []string{"*"}
			frame.MustNotTouch = []string{"all tables except " + op.Table}
		case "replay", "rebuild-index", "append-log", "emit-event", "enqueue":
			frame.Status = ProofAssumed
			frame.MustNotTouch = []string{"external system frame supplied by operator"}
		default:
			frame.Status = ProofRefuted
			frame.MustNotTouch = []string{"unknown"}
		}
		if len(frame.MayWriteRows) == 0 && op.Kind != "insert" {
			frame.Status = ProofRefuted
			frame.Reason = "operation has no row predicate, so the frame cannot bound affected rows"
		}
		out = append(out, frame)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Ref < out[j].Ref
	})
	return out
}

func refinementChecks(manifest Manifest, plan SQLPlan) []RefinementCheck {
	statementByOp := map[string]SQLStatement{}
	for _, statement := range plan.Statements {
		statementByOp[statement.OperationID] = statement
	}
	var out []RefinementCheck
	for _, op := range manifest.Operations {
		statement, ok := statementByOp[op.ID]
		check := RefinementCheck{
			OperationID: op.ID,
			Abstract:    abstractOperation(op),
			Concrete:    statement.SQL,
		}
		switch {
		case !ok:
			check.Status = ProofRefuted
			check.Reason = "no generated SQL statement for operation"
		case statement.Kind != op.Kind:
			check.Status = ProofRefuted
			check.Reason = "generated SQL statement kind differs from manifest operation kind"
		case !sqlContainsOperationShape(statement.SQL, op):
			check.Status = ProofRefuted
			check.Reason = "generated SQL omits table, predicate, or assignment from abstract operation"
		default:
			check.Status = ProofChecked
			check.Reason = "generated SQL preserves operation id, kind, table, predicate, and assignments"
		}
		out = append(out, check)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].OperationID < out[j].OperationID
	})
	return out
}

func refinementFailures(manifest Manifest, err error) []RefinementCheck {
	var out []RefinementCheck
	for _, op := range manifest.Operations {
		out = append(out, RefinementCheck{
			OperationID: op.ID,
			Status:      ProofRefuted,
			Abstract:    abstractOperation(op),
			Reason:      err.Error(),
		})
	}
	return out
}

func proofCounterexamples(obligations []ProofObligation, frames []FrameCondition, refinements []RefinementCheck) []ProofCounterexample {
	var out []ProofCounterexample
	for _, item := range obligations {
		if item.Status == ProofRefuted {
			out = append(out, ProofCounterexample{Ref: item.Ref, Message: item.Reason, Witness: item.Formula})
		}
	}
	for _, item := range frames {
		if item.Status == ProofRefuted {
			out = append(out, ProofCounterexample{Ref: item.Ref, Message: item.Reason, Witness: item.Table})
		}
	}
	for _, item := range refinements {
		if item.Status == ProofRefuted {
			out = append(out, ProofCounterexample{Ref: "refinement." + item.OperationID, Message: item.Reason, Witness: item.Concrete})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Ref < out[j].Ref
	})
	return out
}

func obligation(ref string, status ProofStatus, formula, reason string) ProofObligation {
	return ProofObligation{Ref: ref, Status: status, Formula: formula, Reason: reason}
}

func checked(ok bool) ProofStatus {
	if ok {
		return ProofChecked
	}
	return ProofRefuted
}

func assumedIf(ok bool) ProofStatus {
	if ok {
		return ProofAssumed
	}
	return ProofUnsupported
}

func rowPredicateFormula(op Operation) string {
	return fmt.Sprintf("exists row in %s where %s", op.Table, predicateFormula(op.Where))
}

func uniquenessFormula(op Operation) string {
	if id := op.Set["id"]; id != "" {
		return fmt.Sprintf("not exists row in %s where id=%q", op.Table, id)
	}
	return fmt.Sprintf("not exists duplicate row in %s matching inserted key", op.Table)
}

func predicateFormula(where map[string]string) string {
	parts := predicateParts(where)
	if len(parts) == 0 {
		return "true"
	}
	return strings.Join(parts, " && ")
}

func predicateParts(where map[string]string) []string {
	keys := sortedKeys(where)
	out := make([]string, 0, len(keys))
	for _, key := range keys {
		out = append(out, key+"="+where[key])
	}
	return out
}

func mustNotTouchColumns(where, set map[string]string) []string {
	untouched := []string{"all tables except operation table"}
	for _, key := range sortedKeys(where) {
		if _, ok := set[key]; !ok {
			untouched = append(untouched, "predicate column "+key)
		}
	}
	return untouched
}

func abstractOperation(op Operation) string {
	return fmt.Sprintf("%s table=%s where={%s} set={%s}", op.Kind, op.Table, predicateFormula(op.Where), predicateFormula(op.Set))
}

func sqlContainsOperationShape(sql string, op Operation) bool {
	if !strings.Contains(sql, quoteIdent(op.Table)) {
		return false
	}
	for _, key := range sortedKeys(op.Set) {
		if !strings.Contains(sql, quoteIdent(key)) || !strings.Contains(sql, quoteLiteral(op.Set[key])) {
			return false
		}
	}
	for _, key := range sortedKeys(op.Where) {
		if !strings.Contains(sql, quoteIdent(key)) || !strings.Contains(sql, quoteLiteral(op.Where[key])) {
			return false
		}
	}
	if (op.Kind == "update" || op.Kind == "delete") && !strings.Contains(sql, " WHERE ") {
		return false
	}
	return true
}
