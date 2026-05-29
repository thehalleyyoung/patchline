package replay

import (
	"fmt"
	"sort"
	"strings"

	"github.com/thehalleyyoung/patchline/internal/canonical"
	"github.com/thehalleyyoung/patchline/internal/provenance"
	"github.com/thehalleyyoung/patchline/internal/repair"
)

const SemanticsVersion = "patchline.replay-semantics/v1"

type AnalysisReport struct {
	Version         string                 `json:"version"`
	Manifest        string                 `json:"manifest"`
	Incident        string                 `json:"incident"`
	OK              bool                   `json:"ok"`
	InitialHash     string                 `json:"initial_hash"`
	FinalHash       string                 `json:"final_hash"`
	StepTrace       []StepEvent            `json:"step_trace"`
	Footprints      []OperationFootprint   `json:"footprints"`
	PairChecks      []PairCheck            `json:"pair_checks"`
	Confluence      ConfluenceCheck        `json:"confluence"`
	Isolation       IsolationReport        `json:"isolation"`
	Compensation    []CompensatingAction   `json:"compensation,omitempty"`
	Counterexamples []ReplayCounterexample `json:"counterexamples,omitempty"`
	Hash            string                 `json:"hash"`
}

type StepEvent struct {
	Index       int      `json:"index"`
	OperationID string   `json:"operation_id"`
	Rule        string   `json:"rule"`
	State       string   `json:"state"`
	PreHash     string   `json:"pre_hash"`
	PostHash    string   `json:"post_hash"`
	MatchedRows int      `json:"matched_rows"`
	TouchedRows []string `json:"touched_rows,omitempty"`
	Error       string   `json:"error,omitempty"`
	Explanation string   `json:"explanation"`
}

type OperationFootprint struct {
	OperationID  string            `json:"operation_id"`
	Kind         string            `json:"kind"`
	Table        string            `json:"table"`
	ReadColumns  []string          `json:"read_columns,omitempty"`
	WriteColumns []string          `json:"write_columns,omitempty"`
	ReadKey      string            `json:"read_key,omitempty"`
	WriteKey     string            `json:"write_key,omitempty"`
	Predicate    map[string]string `json:"predicate,omitempty"`
}

type PairCheck struct {
	Left               string                `json:"left"`
	Right              string                `json:"right"`
	SyntacticVerdict   string                `json:"syntactic_verdict"`
	Reason             string                `json:"reason"`
	ObservedEquivalent bool                  `json:"observed_equivalent"`
	ObservationStatus  string                `json:"observation_status"`
	LeftThenRightHash  string                `json:"left_then_right_hash,omitempty"`
	RightThenLeftHash  string                `json:"right_then_left_hash,omitempty"`
	Counterexample     *ReplayCounterexample `json:"counterexample,omitempty"`
}

type ConfluenceCheck struct {
	Status        string   `json:"status"`
	Reason        string   `json:"reason"`
	OrdersChecked int      `json:"orders_checked"`
	FinalHashes   []string `json:"final_hashes,omitempty"`
	Bound         int      `json:"bound"`
}

type IsolationReport struct {
	Levels  []IsolationLevelReport `json:"levels"`
	Hazards []IsolationHazard      `json:"hazards,omitempty"`
}

type IsolationLevelReport struct {
	Level  string `json:"level"`
	Status string `json:"status"`
	Reason string `json:"reason"`
}

type IsolationHazard struct {
	Level    string               `json:"level"`
	Kind     string               `json:"kind"`
	Left     string               `json:"left"`
	Right    string               `json:"right"`
	Severity string               `json:"severity"`
	Witness  ReplayCounterexample `json:"witness"`
}

type CompensatingAction struct {
	OperationID string `json:"operation_id"`
	Kind        string `json:"kind"`
	Target      string `json:"target,omitempty"`
	Status      string `json:"status"`
	Action      string `json:"action"`
	Reason      string `json:"reason"`
}

type ReplayCounterexample struct {
	Ref           string             `json:"ref"`
	Message       string             `json:"message"`
	StoreFragment Store              `json:"store_fragment"`
	Operations    []repair.Operation `json:"operations"`
	BeforeHash    string             `json:"before_hash"`
	AfterHashes   map[string]string  `json:"after_hashes,omitempty"`
}

func Analyze(manifest repair.Manifest, graph *provenance.Graph, store Store) AnalysisReport {
	report := AnalysisReport{
		Version:     SemanticsVersion,
		Manifest:    manifest.Name,
		Incident:    manifest.Incident,
		OK:          true,
		InitialHash: store.Hash(),
	}
	stepTrace, finalStore, counterexamples := Trace(manifest, graph, store)
	report.StepTrace = stepTrace
	report.FinalHash = finalStore.Hash()
	report.Counterexamples = append(report.Counterexamples, counterexamples...)
	report.Footprints = footprints(manifest.Operations)
	report.PairChecks = pairChecks(manifest, graph, store, report.Footprints)
	for _, pair := range report.PairChecks {
		if pair.Counterexample != nil {
			report.Counterexamples = append(report.Counterexamples, *pair.Counterexample)
		}
	}
	report.Confluence = confluenceCheck(manifest, graph, store, 720)
	if report.Confluence.Status == "refuted" {
		report.Counterexamples = append(report.Counterexamples, ReplayCounterexample{
			Ref:           "confluence.refuted",
			Message:       report.Confluence.Reason,
			StoreFragment: operationStoreFragment(store, manifest.Operations),
			Operations:    append([]repair.Operation(nil), manifest.Operations...),
			BeforeHash:    store.Hash(),
		})
	}
	report.Isolation = isolationReport(report.PairChecks, store, manifest.Operations)
	report.Compensation = compensationSemantics(manifest.Operations)
	for _, hazard := range report.Isolation.Hazards {
		report.Counterexamples = append(report.Counterexamples, hazard.Witness)
	}
	report.OK = len(report.Counterexamples) == 0
	report.Hash = analysisHash(report)
	return report
}

func Trace(manifest repair.Manifest, graph *provenance.Graph, store Store) ([]StepEvent, Store, []ReplayCounterexample) {
	working := store.Clone()
	var events []StepEvent
	var counterexamples []ReplayCounterexample
	for index, op := range manifest.Operations {
		preHash := working.Hash()
		event := StepEvent{
			Index:       index,
			OperationID: op.ID,
			Rule:        "eval-" + op.Kind,
			PreHash:     preHash,
		}
		single := manifest
		single.Operations = []repair.Operation{op}
		opReport, next, err := Apply(single, graph, working)
		if err != nil {
			event.State = "error"
			event.PostHash = preHash
			event.Error = err.Error()
			event.Explanation = "a transition rule fired but replay returned an explicit error"
			counterexamples = append(counterexamples, ReplayCounterexample{
				Ref:           "step.error." + op.ID,
				Message:       err.Error(),
				StoreFragment: operationStoreFragment(working, []repair.Operation{op}),
				Operations:    []repair.Operation{op},
				BeforeHash:    preHash,
			})
			events = append(events, event)
			if manifest.Rollback.Strategy == "snapshot" && manifest.Rollback.SnapshotRequired {
				events = append(events, StepEvent{
					Index:       index,
					OperationID: op.ID,
					Rule:        "rollback-snapshot",
					State:       "rollback",
					PreHash:     preHash,
					PostHash:    preHash,
					Explanation: "snapshot rollback leaves the replay store at the pre-error state",
				})
			}
			continue
		}
		opResult := opReport.Operations[0]
		event.MatchedRows = opResult.MatchedRows
		event.TouchedRows = touchedRows(opResult.Diffs)
		if isExternalNoopKind(op.Kind) {
			event.State = "unknown"
			event.Explanation = "operation has a declared external semantic effect but no row-level transition in the replay store"
		} else if opResult.MatchedRows == 0 && (op.Kind == "update" || op.Kind == "delete") {
			event.State = "stuck"
			event.Explanation = "no row matched the operation predicate, so no concrete small-step rule applied"
			counterexamples = append(counterexamples, ReplayCounterexample{
				Ref:           "step.stuck." + op.ID,
				Message:       "operation matched zero rows",
				StoreFragment: operationStoreFragment(working, []repair.Operation{op}),
				Operations:    []repair.Operation{op},
				BeforeHash:    preHash,
				AfterHashes:   map[string]string{"attempt": next.Hash()},
			})
		} else {
			event.State = "normal"
			event.Explanation = "operation rewrote the replay store under a deterministic transition rule"
		}
		working = next
		event.PostHash = working.Hash()
		events = append(events, event)
	}
	return events, working, counterexamples
}

func compensationSemantics(ops []repair.Operation) []CompensatingAction {
	var out []CompensatingAction
	for _, op := range ops {
		switch op.Kind {
		case "append-log":
			out = append(out, compensation(op, "append compensating log entry that references the original append and marks it superseded", "append-only log effects cannot be deleted; recovery is a later semantic entry"))
		case "emit-event":
			out = append(out, compensation(op, "emit inverse or tombstone event keyed by the original event identifier", "event streams are repaired by publishing a causally linked compensating event"))
		case "enqueue":
			out = append(out, compensation(op, "enqueue cancellation or correction message with deterministic idempotency key", "queues are repaired by a follow-up message because delivery may already have occurred"))
		case "replay":
			out = append(out, compensation(op, "record replay cursor and replay window so the external system can be advanced or rewound by contract", "logical replays require system-specific cursor semantics"))
		case "rebuild-index":
			out = append(out, compensation(op, "rebuild derived state from the pinned source snapshot and publish a replacement checksum", "derived rebuilds are compensating recomputations over source-of-truth records"))
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].OperationID < out[j].OperationID })
	return out
}

func compensation(op repair.Operation, action, reason string) CompensatingAction {
	status := "checked"
	target := op.Table
	if target == "" {
		target = op.Set["target"]
	}
	if target == "" {
		status = "proof_hole"
	}
	return CompensatingAction{
		OperationID: op.ID,
		Kind:        op.Kind,
		Target:      target,
		Status:      status,
		Action:      action,
		Reason:      reason,
	}
}

func isExternalNoopKind(kind string) bool {
	switch kind {
	case "replay", "rebuild-index", "append-log", "emit-event", "enqueue":
		return true
	default:
		return false
	}
}

func (r AnalysisReport) CanonicalBytes() []byte {
	return canonical.MustBytes(r)
}

func footprints(ops []repair.Operation) []OperationFootprint {
	out := make([]OperationFootprint, 0, len(ops))
	for _, op := range ops {
		fp := OperationFootprint{
			OperationID:  op.ID,
			Kind:         op.Kind,
			Table:        op.Table,
			ReadColumns:  keys(op.Where),
			Predicate:    cloneMap(op.Where),
			WriteColumns: keys(op.Set),
		}
		if op.Kind == "delete" {
			fp.WriteColumns = []string{"*"}
		}
		if op.Kind == "insert" && !contains(fp.WriteColumns, "id") {
			fp.WriteColumns = append(fp.WriteColumns, "id")
			sort.Strings(fp.WriteColumns)
		}
		fp.ReadKey = op.Where["id"]
		if op.Kind == "insert" {
			fp.WriteKey = op.Set["id"]
		} else {
			fp.WriteKey = op.Where["id"]
		}
		out = append(out, fp)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].OperationID < out[j].OperationID })
	return out
}

func pairChecks(manifest repair.Manifest, graph *provenance.Graph, store Store, fps []OperationFootprint) []PairCheck {
	byID := map[string]OperationFootprint{}
	opByID := map[string]repair.Operation{}
	for _, fp := range fps {
		byID[fp.OperationID] = fp
	}
	for _, op := range manifest.Operations {
		opByID[op.ID] = op
	}
	var checks []PairCheck
	for i := 0; i < len(manifest.Operations); i++ {
		for j := i + 1; j < len(manifest.Operations); j++ {
			left := manifest.Operations[i]
			right := manifest.Operations[j]
			verdict, reason := independence(byID[left.ID], byID[right.ID], left, right)
			check := PairCheck{Left: left.ID, Right: right.ID, SyntacticVerdict: verdict, Reason: reason}
			lrHash, lrErr := applyOrder(manifest, graph, store, []repair.Operation{left, right})
			rlHash, rlErr := applyOrder(manifest, graph, store, []repair.Operation{right, left})
			check.LeftThenRightHash = lrHash
			check.RightThenLeftHash = rlHash
			switch {
			case lrErr != nil || rlErr != nil:
				check.ObservationStatus = "error"
				check.Counterexample = &ReplayCounterexample{
					Ref:           "pair.observation." + left.ID + "." + right.ID,
					Message:       joinErrors(lrErr, rlErr),
					StoreFragment: operationStoreFragment(store, []repair.Operation{left, right}),
					Operations:    []repair.Operation{left, right},
					BeforeHash:    store.Hash(),
					AfterHashes:   map[string]string{"left_then_right": lrHash, "right_then_left": rlHash},
				}
			case lrHash == rlHash:
				check.ObservedEquivalent = true
				check.ObservationStatus = "equivalent_on_fixture"
			default:
				check.ObservationStatus = "different_on_fixture"
				check.Counterexample = &ReplayCounterexample{
					Ref:           "pair.non_commutative." + left.ID + "." + right.ID,
					Message:       "operation order produced distinct final store hashes on the replay fixture",
					StoreFragment: operationStoreFragment(store, []repair.Operation{left, right}),
					Operations:    []repair.Operation{left, right},
					BeforeHash:    store.Hash(),
					AfterHashes:   map[string]string{"left_then_right": lrHash, "right_then_left": rlHash},
				}
			}
			checks = append(checks, check)
			_ = opByID
		}
	}
	return checks
}

func independence(left, right OperationFootprint, leftOp, rightOp repair.Operation) (string, string) {
	if dependsOn(leftOp, rightOp.ID) || dependsOn(rightOp, leftOp.ID) {
		return "ordered", "manifest dependency imposes an execution order"
	}
	if left.Table != right.Table {
		return "independent", "operations target disjoint tables"
	}
	if left.WriteKey != "" && right.WriteKey != "" && left.WriteKey != right.WriteKey {
		return "independent", "operations write distinct row keys in the same table"
	}
	if left.WriteKey == right.WriteKey && left.WriteKey != "" {
		return "conflicting", "operations may write the same row key"
	}
	if intersects(left.WriteColumns, right.WriteColumns) {
		return "conflicting", "write sets overlap"
	}
	if intersects(left.WriteColumns, right.ReadColumns) || intersects(right.WriteColumns, left.ReadColumns) {
		return "conflicting", "one operation writes a column read by the other predicate"
	}
	return "unknown", "same-table operations lack enough key information to prove disjointness"
}

func confluenceCheck(manifest repair.Manifest, graph *provenance.Graph, store Store, bound int) ConfluenceCheck {
	if len(manifest.Operations) <= 1 {
		return ConfluenceCheck{Status: "checked", Reason: "single-operation repair has only one execution order", OrdersChecked: 1, Bound: bound}
	}
	orders := validPermutations(manifest.Operations, bound)
	if len(orders) == 0 {
		return ConfluenceCheck{Status: "unknown", Reason: "permutation search bound exceeded or dependencies left no valid order", Bound: bound}
	}
	hashes := map[string]bool{}
	for _, order := range orders {
		hash, err := applyOrder(manifest, graph, store, order)
		if err != nil {
			return ConfluenceCheck{Status: "refuted", Reason: "valid operation order failed replay: " + err.Error(), OrdersChecked: len(hashes), Bound: bound}
		}
		hashes[hash] = true
	}
	finalHashes := stringSetBool(hashes)
	if len(finalHashes) == 1 {
		return ConfluenceCheck{Status: "checked", Reason: "all dependency-valid operation orders reached the same final store hash on the replay fixture", OrdersChecked: len(orders), FinalHashes: finalHashes, Bound: bound}
	}
	return ConfluenceCheck{Status: "refuted", Reason: "dependency-valid operation orders reached different final store hashes on the replay fixture", OrdersChecked: len(orders), FinalHashes: finalHashes, Bound: bound}
}

func isolationReport(pairs []PairCheck, store Store, ops []repair.Operation) IsolationReport {
	levels := []string{"read_committed", "repeatable_read", "snapshot", "serializable"}
	report := IsolationReport{}
	for _, level := range levels {
		report.Levels = append(report.Levels, IsolationLevelReport{Level: level, Status: "checked", Reason: "no modeled write/write or predicate read/write hazard found"})
	}
	byID := map[string]repair.Operation{}
	for _, op := range ops {
		byID[op.ID] = op
	}
	for _, pair := range pairs {
		if pair.SyntacticVerdict != "conflicting" {
			continue
		}
		left, right := byID[pair.Left], byID[pair.Right]
		kind := "predicate_read_write"
		levelsForHazard := []string{"repeatable_read", "snapshot", "serializable"}
		severity := "medium"
		if sameWriteKey(left, right) || pair.Reason == "write sets overlap" {
			kind = "write_write"
			levelsForHazard = levels
			severity = "high"
		}
		for _, level := range levelsForHazard {
			report.Hazards = append(report.Hazards, IsolationHazard{
				Level:    level,
				Kind:     kind,
				Left:     pair.Left,
				Right:    pair.Right,
				Severity: severity,
				Witness: ReplayCounterexample{
					Ref:           "isolation." + level + "." + kind + "." + pair.Left + "." + pair.Right,
					Message:       pair.Reason,
					StoreFragment: operationStoreFragment(store, []repair.Operation{left, right}),
					Operations:    []repair.Operation{left, right},
					BeforeHash:    store.Hash(),
					AfterHashes:   map[string]string{"left_then_right": pair.LeftThenRightHash, "right_then_left": pair.RightThenLeftHash},
				},
			})
		}
	}
	for i := range report.Levels {
		for _, hazard := range report.Hazards {
			if hazard.Level == report.Levels[i].Level {
				report.Levels[i].Status = "refuted"
				report.Levels[i].Reason = "modeled hazards require ordering or stronger transaction discipline"
				break
			}
		}
	}
	return report
}

func applyOrder(manifest repair.Manifest, graph *provenance.Graph, store Store, ops []repair.Operation) (string, error) {
	ordered := manifest
	ordered.Operations = append([]repair.Operation(nil), ops...)
	_, finalStore, err := Apply(ordered, graph, store)
	if err != nil {
		return "", err
	}
	return finalStore.Hash(), nil
}

func validPermutations(ops []repair.Operation, bound int) [][]repair.Operation {
	if len(ops) == 0 {
		return nil
	}
	var out [][]repair.Operation
	used := make([]bool, len(ops))
	var current []repair.Operation
	var visit func()
	visit = func() {
		if len(out) > bound {
			return
		}
		if len(current) == len(ops) {
			order := append([]repair.Operation(nil), current...)
			out = append(out, order)
			return
		}
		for i, op := range ops {
			if used[i] || !depsSatisfied(op, current) {
				continue
			}
			used[i] = true
			current = append(current, op)
			visit()
			current = current[:len(current)-1]
			used[i] = false
		}
	}
	visit()
	if len(out) > bound {
		return nil
	}
	return out
}

func depsSatisfied(op repair.Operation, current []repair.Operation) bool {
	seen := map[string]bool{}
	for _, done := range current {
		seen[done.ID] = true
	}
	for _, dep := range op.DependsOn {
		if !seen[dep] {
			return false
		}
	}
	return true
}

func operationStoreFragment(store Store, ops []repair.Operation) Store {
	fragment := Store{Tables: map[string]map[string]Row{}}
	for _, op := range ops {
		rows := store.Tables[op.Table]
		if _, ok := fragment.Tables[op.Table]; !ok {
			fragment.Tables[op.Table] = map[string]Row{}
		}
		for id, row := range rows {
			if rowRelevant(row, op) {
				fragment.Tables[op.Table][id] = cloneRow(row)
			}
		}
		if op.Kind == "insert" && op.Set["id"] != "" {
			if _, exists := fragment.Tables[op.Table][op.Set["id"]]; !exists {
				fragment.Tables[op.Table][op.Set["id"]] = Row{}
			}
		}
	}
	return fragment
}

func rowRelevant(row Row, op repair.Operation) bool {
	if len(op.Where) == 0 {
		return true
	}
	if matches(row, op.Where) {
		return true
	}
	if id := op.Where["id"]; id != "" && row["id"] == id {
		return true
	}
	return false
}

func touchedRows(diffs []RowDiff) []string {
	out := make([]string, 0, len(diffs))
	for _, diff := range diffs {
		out = append(out, diff.Table+"/"+diff.ID)
	}
	sort.Strings(out)
	return out
}

func dependsOn(op repair.Operation, id string) bool {
	for _, dep := range op.DependsOn {
		if dep == id {
			return true
		}
	}
	return false
}

func sameWriteKey(left, right repair.Operation) bool {
	leftKey := left.Where["id"]
	if left.Kind == "insert" {
		leftKey = left.Set["id"]
	}
	rightKey := right.Where["id"]
	if right.Kind == "insert" {
		rightKey = right.Set["id"]
	}
	return left.Table == right.Table && leftKey != "" && leftKey == rightKey
}

func intersects(left, right []string) bool {
	leftSet := map[string]bool{}
	for _, value := range left {
		leftSet[value] = true
	}
	for _, value := range right {
		if value == "*" || leftSet["*"] || leftSet[value] {
			return true
		}
	}
	return false
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func cloneMap(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func stringSetBool(in map[string]bool) []string {
	out := make([]string, 0, len(in))
	for value := range in {
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func joinErrors(left, right error) string {
	var parts []string
	if left != nil {
		parts = append(parts, "left_then_right: "+left.Error())
	}
	if right != nil {
		parts = append(parts, "right_then_left: "+right.Error())
	}
	return strings.Join(parts, "; ")
}

func analysisHash(report AnalysisReport) string {
	report.Hash = ""
	return canonical.Hash(report)
}

func (r AnalysisReport) Summary() string {
	status := "ok"
	if !r.OK {
		status = "counterexamples"
	}
	return fmt.Sprintf("replay semantics %s steps=%d pairs=%d confluence=%s hazards=%d hash=%s",
		status, len(r.StepTrace), len(r.PairChecks), r.Confluence.Status, len(r.Isolation.Hazards), r.Hash)
}
