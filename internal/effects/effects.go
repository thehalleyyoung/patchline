package effects

import (
	"sort"

	"github.com/thehalleyyoung/patchline/internal/canonical"
)

type Effect string

const (
	EffectUnknown          Effect = "unknown"
	EffectNoop             Effect = "noop"
	EffectIdempotentUpdate Effect = "idempotent_update"
	EffectReversibleUpdate Effect = "reversible_update"
	EffectDestructive      Effect = "destructive"
	EffectReplay           Effect = "replay"
	EffectDerivedRebuild   Effect = "derived_rebuild"
	EffectAppendOnly       Effect = "append_only_external"
)

type Mutation struct {
	Kind                string
	Table               string
	WhereKeys           []string
	SetKeys             []string
	HasSnapshotRollback bool
}

type Classification struct {
	Effect  Effect   `json:"effect"`
	Reasons []string `json:"reasons"`
}

type LatticeElement struct {
	Effect      Effect   `json:"effect"`
	Rank        int      `json:"rank"`
	Description string   `json:"description"`
	Properties  []string `json:"properties"`
}

type OperationObservation struct {
	OperationID         string   `json:"operation_id"`
	Table               string   `json:"table"`
	Effect              Effect   `json:"effect"`
	MatchedRows         int      `json:"matched_rows"`
	ChangedColumns      []string `json:"changed_columns,omitempty"`
	DownstreamEntities  int      `json:"downstream_entities"`
	HasSnapshotRollback bool     `json:"has_snapshot_rollback"`
	Reasons             []string `json:"reasons,omitempty"`
}

type AbstractSummary struct {
	Version             string                `json:"version"`
	Manifest            string                `json:"manifest"`
	Incident            string                `json:"incident"`
	Join                Effect                `json:"join"`
	Operations          []AbstractOperation   `json:"operations"`
	Concretization      ConcretizationSummary `json:"concretization"`
	AbstractionRelation []string              `json:"abstraction_relation"`
	Hash                string                `json:"hash"`
}

type AbstractOperation struct {
	OperationID      string   `json:"operation_id"`
	Table            string   `json:"table"`
	Effect           Effect   `json:"effect"`
	Rank             int      `json:"rank"`
	BoundedRows      bool     `json:"bounded_rows"`
	MaxRows          int      `json:"max_rows"`
	ChangedColumns   []string `json:"changed_columns,omitempty"`
	Destructive      bool     `json:"destructive"`
	Reversible       bool     `json:"reversible"`
	Idempotent       bool     `json:"idempotent"`
	DownstreamImpact bool     `json:"downstream_impact"`
	ProofHoles       []string `json:"proof_holes,omitempty"`
	Transfer         string   `json:"transfer"`
}

type ConcretizationSummary struct {
	RowsChanged        int      `json:"rows_changed"`
	Tables             []string `json:"tables"`
	Columns            []string `json:"columns"`
	DownstreamEntities int      `json:"downstream_entities"`
	UnsupportedFacts   []string `json:"unsupported_facts,omitempty"`
}

func Infer(m Mutation) Classification {
	whereKeys := sorted(m.WhereKeys)
	setKeys := sorted(m.SetKeys)

	switch m.Kind {
	case "insert":
		if len(setKeys) == 0 {
			return Classification{Effect: EffectNoop, Reasons: []string{"insert has no row values"}}
		}
		if m.HasSnapshotRollback {
			return Classification{Effect: EffectReversibleUpdate, Reasons: []string{"inserted row can be removed under snapshot rollback"}}
		}
		return Classification{Effect: EffectIdempotentUpdate, Reasons: []string{"insert has explicit row values"}}
	case "update":
		if len(setKeys) == 0 {
			return Classification{Effect: EffectNoop, Reasons: []string{"update has no assigned columns"}}
		}
		if len(whereKeys) == 0 {
			return Classification{Effect: EffectDestructive, Reasons: []string{"update without a predicate can rewrite an unbounded table"}}
		}
		if m.HasSnapshotRollback {
			return Classification{Effect: EffectReversibleUpdate, Reasons: []string{"predicate-bounded update has snapshot rollback"}}
		}
		return Classification{Effect: EffectIdempotentUpdate, Reasons: []string{"predicate-bounded constant assignment is idempotent"}}
	case "delete":
		return Classification{Effect: EffectDestructive, Reasons: []string{"delete removes rows and must be reviewed as destructive"}}
	case "replay":
		return Classification{Effect: EffectReplay, Reasons: []string{"operation replays source events instead of editing derived state directly"}}
	case "rebuild-index":
		return Classification{Effect: EffectDerivedRebuild, Reasons: []string{"operation rebuilds derived state from source records"}}
	case "append-log", "emit-event", "enqueue":
		return Classification{Effect: EffectAppendOnly, Reasons: []string{"operation appends to an external log, event stream, or queue and needs compensating-action semantics"}}
	default:
		return Classification{Effect: EffectUnknown, Reasons: []string{"operation kind is not recognized by the effect lattice"}}
	}
}

func IsRisky(effect Effect) bool {
	return effect == EffectUnknown || effect == EffectDestructive
}

func Lattice() []LatticeElement {
	return []LatticeElement{
		{Effect: EffectNoop, Rank: 0, Description: "no concrete row change", Properties: []string{"bounded", "idempotent", "reversible"}},
		{Effect: EffectIdempotentUpdate, Rank: 1, Description: "bounded deterministic write whose repeated application is stable", Properties: []string{"bounded", "idempotent"}},
		{Effect: EffectReversibleUpdate, Rank: 2, Description: "bounded write with declared snapshot rollback", Properties: []string{"bounded", "reversible"}},
		{Effect: EffectReplay, Rank: 3, Description: "external replay operation with system-specific semantics", Properties: []string{"external"}},
		{Effect: EffectDerivedRebuild, Rank: 4, Description: "derived-state rebuild from source records", Properties: []string{"derived", "external"}},
		{Effect: EffectAppendOnly, Rank: 5, Description: "append-only external effect requiring an explicit compensating action", Properties: []string{"external", "irreversible", "compensating"}},
		{Effect: EffectDestructive, Rank: 6, Description: "delete or unbounded write that removes or may rewrite state", Properties: []string{"destructive"}},
		{Effect: EffectUnknown, Rank: 7, Description: "operation outside the known transfer functions", Properties: []string{"unknown"}},
	}
}

func Join(left, right Effect) Effect {
	if rank(left) >= rank(right) {
		return left
	}
	return right
}

func Summarize(manifest, incident string, observations []OperationObservation) AbstractSummary {
	observations = append([]OperationObservation(nil), observations...)
	sort.Slice(observations, func(i, j int) bool {
		return observations[i].OperationID < observations[j].OperationID
	})
	summary := AbstractSummary{
		Version:  "patchline.effect-summary/v1",
		Manifest: manifest,
		Incident: incident,
		Join:     EffectNoop,
		AbstractionRelation: []string{
			"alpha(row diffs) = row-count bounds + changed-column set + operation effect",
			"gamma(summary) = all concrete executions whose changed rows, columns, downstream impact, and effect rank are within the summary bounds",
			"soundness obligation: every concrete replay diff must be represented by exactly one abstract operation summary",
		},
	}
	tableSet := map[string]struct{}{}
	columnSet := map[string]struct{}{}
	for _, observation := range observations {
		abstract := abstractOperation(observation)
		summary.Operations = append(summary.Operations, abstract)
		summary.Join = Join(summary.Join, abstract.Effect)
		if observation.MatchedRows >= 0 {
			summary.Concretization.RowsChanged += observation.MatchedRows
		} else {
			summary.Concretization.UnsupportedFacts = append(summary.Concretization.UnsupportedFacts, observation.OperationID+": concrete row count unavailable")
		}
		if observation.Table != "" {
			tableSet[observation.Table] = struct{}{}
		}
		for _, column := range abstract.ChangedColumns {
			columnSet[column] = struct{}{}
		}
		summary.Concretization.DownstreamEntities += observation.DownstreamEntities
		summary.Concretization.UnsupportedFacts = append(summary.Concretization.UnsupportedFacts, abstract.ProofHoles...)
	}
	summary.Concretization.Tables = sortedSet(tableSet)
	summary.Concretization.Columns = sortedSet(columnSet)
	sort.Strings(summary.Concretization.UnsupportedFacts)
	summary.Hash = canonical.Hash(struct {
		Version             string                `json:"version"`
		Manifest            string                `json:"manifest"`
		Incident            string                `json:"incident"`
		Join                Effect                `json:"join"`
		Operations          []AbstractOperation   `json:"operations"`
		Concretization      ConcretizationSummary `json:"concretization"`
		AbstractionRelation []string              `json:"abstraction_relation"`
	}{
		Version:             summary.Version,
		Manifest:            summary.Manifest,
		Incident:            summary.Incident,
		Join:                summary.Join,
		Operations:          summary.Operations,
		Concretization:      summary.Concretization,
		AbstractionRelation: summary.AbstractionRelation,
	})
	return summary
}

func abstractOperation(observation OperationObservation) AbstractOperation {
	effect := observation.Effect
	if effect == "" {
		effect = EffectUnknown
	}
	abstract := AbstractOperation{
		OperationID:      observation.OperationID,
		Table:            observation.Table,
		Effect:           effect,
		Rank:             rank(effect),
		BoundedRows:      observation.MatchedRows >= 0 && effect != EffectUnknown,
		MaxRows:          observation.MatchedRows,
		ChangedColumns:   sorted(observation.ChangedColumns),
		Destructive:      effect == EffectDestructive || effect == EffectUnknown,
		Reversible:       effect == EffectNoop || effect == EffectReversibleUpdate || observation.HasSnapshotRollback,
		Idempotent:       effect == EffectNoop || effect == EffectIdempotentUpdate,
		DownstreamImpact: observation.DownstreamEntities > 0,
		Transfer:         transferName(effect),
	}
	if effect == EffectReplay || effect == EffectDerivedRebuild {
		abstract.ProofHoles = append(abstract.ProofHoles, observation.OperationID+": external operation needs system-specific transfer function")
	}
	if effect == EffectAppendOnly {
		abstract.ProofHoles = append(abstract.ProofHoles, observation.OperationID+": append-only external effect needs an operator-supplied compensating action")
	}
	if effect == EffectUnknown {
		abstract.ProofHoles = append(abstract.ProofHoles, observation.OperationID+": no abstract transfer function for operation")
	}
	if observation.MatchedRows < 0 {
		abstract.ProofHoles = append(abstract.ProofHoles, observation.OperationID+": concrete row count unavailable")
	}
	return abstract
}

func rank(effect Effect) int {
	for _, item := range Lattice() {
		if item.Effect == effect {
			return item.Rank
		}
	}
	return 7
}

func transferName(effect Effect) string {
	switch effect {
	case EffectNoop:
		return "T_noop"
	case EffectIdempotentUpdate:
		return "T_const_write_idempotent"
	case EffectReversibleUpdate:
		return "T_snapshot_reversible_write"
	case EffectReplay:
		return "T_external_replay"
	case EffectDerivedRebuild:
		return "T_derived_rebuild"
	case EffectAppendOnly:
		return "T_compensating_external_append"
	case EffectDestructive:
		return "T_destructive"
	default:
		return "T_unknown"
	}
}

func sorted(in []string) []string {
	out := append([]string(nil), in...)
	sort.Strings(out)
	return out
}

func sortedSet(values map[string]struct{}) []string {
	out := make([]string, 0, len(values))
	for value := range values {
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}
