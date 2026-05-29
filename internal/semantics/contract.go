package semantics

import (
	"sort"

	"github.com/thehalleyyoung/patchline/internal/canonical"
	"github.com/thehalleyyoung/patchline/internal/effects"
	"github.com/thehalleyyoung/patchline/internal/provenance"
)

const Version = "patchline.semantics/v1"

type ClaimStatus string

const (
	ClaimProved      ClaimStatus = "proved"
	ClaimChecked     ClaimStatus = "checked"
	ClaimAssumed     ClaimStatus = "assumed"
	ClaimUnsupported ClaimStatus = "unsupported"
	ClaimRefuted     ClaimStatus = "refuted"
)

type Contract struct {
	Version           string            `json:"version"`
	StateModel        []StateComponent  `json:"state_model"`
	ObservationModel  []ObservationKind `json:"observation_model"`
	RepairTransformer RepairTransformer `json:"repair_transformer"`
	ClaimStatuses     []ClaimStatus     `json:"claim_statuses"`
	ArtifactKinds     []ArtifactKind    `json:"artifact_kinds"`
	CommandContracts  []CommandContract `json:"command_contracts"`
	FailureStates     []FailureState    `json:"failure_states"`
	Hash              string            `json:"hash"`
}

type StateComponent struct {
	Name            string   `json:"name"`
	ExistingKinds   []string `json:"existing_kinds"`
	SemanticRole    string   `json:"semantic_role"`
	CurrentArtifact string   `json:"current_artifact"`
}

type ObservationKind struct {
	Name           string   `json:"name"`
	ExistingEvents []string `json:"existing_events"`
	ExistingEdges  []string `json:"existing_edges,omitempty"`
	SemanticRole   string   `json:"semantic_role"`
}

type RepairTransformer struct {
	Domain      []string `json:"domain"`
	Codomain    []string `json:"codomain"`
	Effects     []string `json:"effects"`
	Obligations []string `json:"obligations"`
	Failures    []string `json:"failures"`
}

type ArtifactKind struct {
	Name        string   `json:"name"`
	MustEmit    []string `json:"must_emit"`
	Examples    []string `json:"examples"`
	Description string   `json:"description"`
}

type CommandContract struct {
	Command     string   `json:"command"`
	Artifacts   []string `json:"artifacts"`
	RequiredAny []string `json:"required_any"`
}

type FailureState struct {
	Name        string   `json:"name"`
	Surfaces    []string `json:"surfaces"`
	Description string   `json:"description"`
}

func DefaultContract() Contract {
	contract := Contract{
		Version: Version,
		StateModel: []StateComponent{
			{"code", stringsOf(provenance.KindCommit, provenance.KindDeploy, provenance.KindService), "versioned program text and rollout state", "provenance graph"},
			{"schema", []string{"migration"}, "relational signature and migration history", "migration reports"},
			{"rows", stringsOf(provenance.KindRecord, provenance.KindSQLMutation), "persistent data state and mutations", "evidence graph + replay store"},
			{"jobs", stringsOf(provenance.KindJobRun, provenance.KindQueueEvent), "asynchronous program state", "provenance graph"},
			{"reports", stringsOf(provenance.KindReport), "customer-visible derived outputs", "provenance graph + attestations"},
			{"traces", stringsOf(provenance.KindTrace), "runtime execution observations", "evidence graph"},
			{"policies", []string{"policy", "gate"}, "review and CI constraints over repairs", "policy/gate reports"},
			{"ledgers", []string{"ledger_entry", "checkpoint"}, "tamper-evident repair history", "ledger checkpoints"},
		},
		ObservationModel: []ObservationKind{
			{"alert", []string{"datadog_span", "otlp_span"}, []string{string(provenance.EdgeObserved)}, "operator-visible signal that motivates reconstruction"},
			{"row_predicate", []string{"row_mutation"}, []string{string(provenance.EdgeMutated)}, "concrete data evidence for damaged state"},
			{"trace", []string{"trace", "sql_mutation"}, []string{string(provenance.EdgeCaused), string(provenance.EdgeObserved)}, "runtime execution linking code/schema to rows"},
			{"report_total", []string{"derived_report"}, []string{string(provenance.EdgeDerivedInto)}, "derived customer-visible output"},
			{"approval", []string{"policy", "ledger"}, []string{string(provenance.EdgeRepaired)}, "review and application evidence"},
		},
		RepairTransformer: RepairTransformer{
			Domain: []string{
				"repair manifest with declared scope",
				"historical evidence graph",
				"current or reconstructed store snapshot",
				"preconditions and policy gates",
			},
			Codomain: []string{
				"post-state replay report",
				"row diffs and downstream entity set",
				"SQL, rollback, and transaction plans",
				"ledger checkpoint and content hashes",
			},
			Effects: stringsOf(
				effects.EffectNoop,
				effects.EffectIdempotentUpdate,
				effects.EffectReversibleUpdate,
				effects.EffectDestructive,
				effects.EffectReplay,
				effects.EffectDerivedRebuild,
				effects.EffectAppendOnly,
				effects.EffectUnknown,
			),
			Obligations: []string{
				"scope containment",
				"precondition evidence",
				"postcondition evidence",
				"rollback availability",
				"frame preservation",
				"predicate implication",
				"symbolic path coverage",
				"workflow temporal properties",
				"snapshot drift stability",
				"compensating external action",
				"counterexample-guided refinement",
				"signed artifact attestation",
				"incident archive retrieval",
				"bounded invariant preservation",
				"hash stability",
			},
			Failures: []string{
				"precondition_unmet",
				"scope_escape",
				"unsupported_operation",
				"stuck_replay",
				"rollback_unavailable",
				"counterexample_found",
			},
		},
		ClaimStatuses: []ClaimStatus{ClaimProved, ClaimChecked, ClaimAssumed, ClaimUnsupported, ClaimRefuted},
		ArtifactKinds: []ArtifactKind{
			{"trace_projection", []string{"facts", "hashes"}, []string{"ingest-evidence result"}, "typed events reconstructed from operational evidence"},
			{"repair_contract", []string{"facts", "obligations", "hashes"}, []string{"validate-repair", "lint-repair"}, "Hoare-style view of a manifest"},
			{"replay_report", []string{"facts", "hashes", "counterexamples"}, []string{"dry-run"}, "concrete execution of a repair over a store"},
			{"replay_semantics", []string{"facts", "hashes", "counterexamples"}, []string{"repair-semantics"}, "small-step trace, commutativity, confluence, and isolation-hazard analysis"},
			{"snapshot_comparison", []string{"facts", "hashes", "counterexamples"}, []string{"snapshot-drift"}, "repair stability comparison across imported historical row snapshots"},
			{"migration_report", []string{"facts", "obligations", "hashes"}, []string{"analyze-migration"}, "SQL statement semantics and risk obligations"},
			{"schema_semantics", []string{"facts", "hashes"}, []string{"schema-diff", "migration-semantics"}, "schema-state diffing and typed relational-signature transformations"},
			{"source_sql_inventory", []string{"facts", "hashes"}, []string{"extract-sql"}, "embedded SQL and ORM/query-builder extraction from application code"},
			{"migration_outcome_history", []string{"facts", "obligations", "hashes"}, []string{"migration-outcomes", "migration-changelog"}, "historical migration-to-trace/row/repair/policy links and semantic changelogs"},
			{"solver_obligations", []string{"facts", "hashes", "counterexamples"}, []string{"solver-obligations", "semantics-audit"}, "Z3-backed scope implication plus bounded frame, row-count, and invariant-preservation checks"},
			{"symbolic_execution", []string{"facts", "hashes", "counterexamples"}, []string{"symbolic-exec", "semantics-audit"}, "bounded symbolic execution of repair programs with row path constraints and symbolic assignments"},
			{"workflow_model_check", []string{"facts", "hashes", "counterexamples"}, []string{"model-check-workflow", "semantics-audit"}, "bounded incident-workflow model checking with temporal properties, proof obligations, and proof holes"},
			{"cegar_refinement", []string{"facts", "obligations", "counterexamples", "hashes"}, []string{"cegar-refine"}, "counterexample-guided reruns that refine coarse repair abstractions with invariants and workflow models"},
			{"signed_attestation", []string{"facts", "hashes"}, []string{"sign-artifact", "verify-artifact"}, "Ed25519 signatures over canonical artifact hashes for CI and incident-review evidence"},
			{"incident_archive", []string{"facts", "hashes"}, []string{"archive-index"}, "searchable historical incident index grouped by evidence shape, migration semantics, repair effect, policy decision, benchmark result, repair outcome, and semantic regression"},
			{"benchmark_report", []string{"facts", "obligations", "counterexamples", "hashes"}, []string{"benchmark-suite", "ci-gate"}, "strict corpus evidence for analyzer quality"},
			{"ledger_checkpoint", []string{"facts", "hashes"}, []string{"ledger-verify"}, "tamper-evident repair history"},
		},
		CommandContracts: []CommandContract{
			{"demo-graph", []string{"trace_projection"}, []string{"facts", "hashes"}},
			{"explain", []string{"causal_certificate"}, []string{"facts"}},
			{"slice", []string{"trace_projection"}, []string{"facts", "hashes"}},
			{"validate-repair", []string{"repair_contract"}, []string{"obligations", "counterexamples", "hashes"}},
			{"lint-repair", []string{"repair_contract"}, []string{"obligations", "counterexamples"}},
			{"solver-obligations", []string{"solver_obligations"}, []string{"facts", "hashes", "counterexamples"}},
			{"symbolic-exec", []string{"symbolic_execution"}, []string{"facts", "hashes", "counterexamples"}},
			{"model-check-workflow", []string{"workflow_model_check"}, []string{"facts", "hashes", "counterexamples"}},
			{"cegar-refine", []string{"cegar_refinement"}, []string{"facts", "obligations", "hashes", "counterexamples"}},
			{"sign-artifact", []string{"signed_attestation"}, []string{"facts", "hashes"}},
			{"verify-artifact", []string{"signed_attestation"}, []string{"facts", "hashes", "counterexamples"}},
			{"archive-index", []string{"incident_archive"}, []string{"facts", "hashes"}},
			{"repair-outcomes", []string{"incident_archive"}, []string{"facts", "hashes"}},
			{"semantic-regressions", []string{"incident_archive"}, []string{"facts", "hashes"}},
			{"generate-sql", []string{"repair_contract"}, []string{"facts", "hashes"}},
			{"rollback-plan", []string{"repair_contract"}, []string{"facts", "hashes"}},
			{"transaction-plan", []string{"repair_contract"}, []string{"facts", "hashes"}},
			{"analyze-migration", []string{"migration_report"}, []string{"facts", "obligations", "hashes"}},
			{"schema-diff", []string{"schema_semantics"}, []string{"facts", "hashes", "counterexamples"}},
			{"migration-semantics", []string{"schema_semantics"}, []string{"facts", "hashes"}},
			{"extract-sql", []string{"source_sql_inventory"}, []string{"facts", "hashes"}},
			{"migration-outcomes", []string{"migration_outcome_history"}, []string{"facts", "obligations", "hashes"}},
			{"migration-changelog", []string{"migration_outcome_history"}, []string{"facts", "obligations", "hashes"}},
			{"dry-run", []string{"replay_report"}, []string{"facts", "hashes", "counterexamples"}},
			{"repair-semantics", []string{"replay_semantics"}, []string{"facts", "hashes", "counterexamples"}},
			{"snapshot-drift", []string{"snapshot_comparison"}, []string{"facts", "hashes", "counterexamples"}},
			{"reproduce", []string{"benchmark_report"}, []string{"obligations", "hashes", "counterexamples"}},
			{"benchmark-suite", []string{"benchmark_report"}, []string{"facts", "obligations", "hashes", "counterexamples"}},
			{"ingest-evidence", []string{"trace_projection"}, []string{"facts", "hashes", "counterexamples"}},
			{"adapt-evidence", []string{"trace_projection"}, []string{"facts", "hashes", "counterexamples"}},
			{"ci-gate", []string{"benchmark_report"}, []string{"obligations", "hashes", "counterexamples"}},
			{"ledger-verify", []string{"ledger_checkpoint"}, []string{"facts", "hashes", "counterexamples"}},
		},
		FailureStates: []FailureState{
			{"precondition_unmet", []string{"repair validation", "policy evaluation"}, "required evidence or declared precondition is absent"},
			{"scope_escape", []string{"repair validation", "SQL generation", "solver obligations"}, "operation may affect state outside the declared frame"},
			{"unsupported_operation", []string{"repair validation", "dry-run", "SQL generation"}, "operation is outside the supported transformer semantics"},
			{"stuck_replay", []string{"dry-run", "symbolic execution"}, "the current store has no transition for the requested operation"},
			{"rollback_unavailable", []string{"rollback-plan", "transaction-plan"}, "the post-state cannot be reversed with available evidence"},
			{"counterexample_found", []string{"benchmark-suite", "ci-gate", "policy evaluation", "solver obligations"}, "a checked claim is refuted by executable evidence"},
		},
	}
	contract.Hash = contractHash(contract)
	return contract
}

func contractHash(contract Contract) string {
	contract.Hash = ""
	return canonical.Hash(contract)
}

func stringsOf[T ~string](values ...T) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		out = append(out, string(value))
	}
	sort.Strings(out)
	return out
}
