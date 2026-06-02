package project

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/thehalleyyoung/patchline/internal/canonical"
	"github.com/thehalleyyoung/patchline/internal/effects"
	"github.com/thehalleyyoung/patchline/internal/intake"
	"github.com/thehalleyyoung/patchline/internal/migration"
)

const BaselineVersion = "patchline.repo-baseline/v1"

var (
	blastFKInlinePattern = regexp.MustCompile(`(?is)\b(?:foreign\s+key\s*\([^)]+\)\s*)?references\s+["'\[]?([A-Za-z_][A-Za-z0-9_.$-]*)`)
	blastFKAlterPattern  = regexp.MustCompile(`(?is)\balter\s+table\s+(?:if\s+exists\s+)?["'\[]?([A-Za-z_][A-Za-z0-9_.$-]*)[^;]*?\breferences\s+["'\[]?([A-Za-z_][A-Za-z0-9_.$-]*)`)
	blastCreateTablePat  = regexp.MustCompile(`(?is)\bcreate\s+table\s+(?:if\s+not\s+exists\s+)?["'\[]?([A-Za-z_][A-Za-z0-9_.$-]*)`)
	blastQueryTablePat   = regexp.MustCompile(`(?is)\b(?:from|join|update|into|delete\s+from)\s+["'\[]?([A-Za-z_][A-Za-z0-9_.$-]*)`)
)

type BaselineReport struct {
	Version         string                    `json:"version"`
	InventoryRoot   string                    `json:"inventory_root"`
	IntakeSource    string                    `json:"intake_source"`
	Summary         BaselineSummary           `json:"summary"`
	Risks           []BaselineRisk            `json:"risks,omitempty"`
	Rankings        []RankingExplanation      `json:"ranking_explanations,omitempty"`
	EvidenceLinks   []EvidenceLink            `json:"evidence_links,omitempty"`
	CauseClusters   []EvidenceCluster         `json:"cause_clusters,omitempty"`
	RepairClusters  []EvidenceCluster         `json:"repair_clusters,omitempty"`
	Provenance      []ProvenanceSlice         `json:"provenance_slices,omitempty"`
	DatalogQueries  []DatalogQuery            `json:"datalog_queries,omitempty"`
	AbstractEffects []effects.AbstractSummary `json:"abstract_effects,omitempty"`
	SymbolicChecks  []SymbolicCheck           `json:"symbolic_checks,omitempty"`
	TemporalWindows []TemporalWindow          `json:"temporal_windows,omitempty"`
	Recurrences     []RecurrencePattern       `json:"recurrences,omitempty"`
	Transactions    []TransactionBoundary     `json:"transaction_boundaries,omitempty"`
	Idempotency     []IdempotencyClass        `json:"idempotency_classifications,omitempty"`
	LockHazards     []LockHazard              `json:"lock_concurrency_hazards,omitempty"`
	PrivacyHazards  []PrivacyHazard           `json:"data_retention_privacy_hazards,omitempty"`
	Invariants      []InvariantCandidate      `json:"invariant_candidates,omitempty"`
	TraceLinks      []TraceCodeLink           `json:"trace_code_links,omitempty"`
	BlastRadius     []BlastRadiusEstimate     `json:"blast_radius_estimates,omitempty"`
	ProofMinimizers []ProofHoleMinimization   `json:"proof_hole_minimizations,omitempty"`
	PolicyChecks    []PolicyCheck             `json:"policy_checks,omitempty"`
	RepairProofs    []RepairProofSummary      `json:"repair_proof_summaries,omitempty"`
	NativeChecks    []Command                 `json:"native_checks,omitempty"`
	Hash            string                    `json:"hash"`
	Markdown        string                    `json:"markdown,omitempty"`
}

type BaselineSummary struct {
	RankedRisks         int `json:"ranked_risks"`
	CodePathRankedRisks int `json:"code_path_ranked_risks"`
	RankingExplanations int `json:"ranking_explanations"`
	RankingFeatures     int `json:"ranking_features"`
	AblationSensitive   int `json:"ablation_sensitive_risks"`
	EvidenceLinks       int `json:"evidence_links"`
	CauseClusters       int `json:"cause_clusters"`
	RepairClusters      int `json:"repair_clusters"`
	ProvenanceSlices    int `json:"provenance_slices"`
	DatalogQueries      int `json:"datalog_queries"`
	DatalogRows         int `json:"datalog_rows"`
	AbstractEffects     int `json:"abstract_effects"`
	AbstractOperations  int `json:"abstract_operations"`
	AbstractProofHoles  int `json:"abstract_proof_holes"`
	SymbolicChecks      int `json:"symbolic_checks"`
	SymbolicPassed      int `json:"symbolic_passed"`
	SymbolicWarnings    int `json:"symbolic_warnings"`
	SymbolicFailed      int `json:"symbolic_failed"`
	TemporalWindows     int `json:"temporal_windows"`
	TemporalSignals     int `json:"temporal_signals"`
	Recurrences         int `json:"recurrences"`
	RecurringRisks      int `json:"recurring_risks"`
	Transactions        int `json:"transaction_boundaries"`
	TransactionExplicit int `json:"transaction_explicit"`
	TransactionMissing  int `json:"transaction_missing"`
	TransactionPartial  int `json:"transaction_partial"`
	IdempotencyClasses  int `json:"idempotency_classifications"`
	IdempotencyProven   int `json:"idempotency_proven"`
	IdempotencyGuarded  int `json:"idempotency_guarded"`
	IdempotencyUnknown  int `json:"idempotency_unknown"`
	IdempotencyUnsafe   int `json:"idempotency_non_idempotent"`
	LockHazards         int `json:"lock_concurrency_hazards"`
	LockHazardCritical  int `json:"lock_hazard_critical"`
	LockHazardHigh      int `json:"lock_hazard_high"`
	LockHazardMedium    int `json:"lock_hazard_medium"`
	LockHazardLow       int `json:"lock_hazard_low"`
	PrivacyHazards      int `json:"data_retention_privacy_hazards"`
	PrivacyCritical     int `json:"privacy_hazard_critical"`
	PrivacyHigh         int `json:"privacy_hazard_high"`
	PrivacyMedium       int `json:"privacy_hazard_medium"`
	PrivacyLow          int `json:"privacy_hazard_low"`
	Invariants          int `json:"invariant_candidates"`
	InvariantSchema     int `json:"invariants_from_schema"`
	InvariantTests      int `json:"invariants_from_tests"`
	InvariantValidation int `json:"invariants_from_validations"`
	InvariantFixtures   int `json:"invariants_from_fixtures"`
	TraceCodeLinks      int `json:"trace_code_links"`
	TraceLinkExact      int `json:"trace_links_exact"`
	TraceLinkCausal     int `json:"trace_links_causal"`
	TraceLinkTemporal   int `json:"trace_links_temporal"`
	TraceLinkInferred   int `json:"trace_links_inferred"`
	BlastRadius         int `json:"blast_radius_estimates"`
	BlastRadiusHigh     int `json:"blast_radius_high"`
	BlastRadiusMedium   int `json:"blast_radius_medium"`
	BlastRadiusLow      int `json:"blast_radius_low"`
	ProofMinimizations  int `json:"proof_hole_minimizations"`
	ProofMinCritical    int `json:"proof_hole_min_critical"`
	ProofMinHigh        int `json:"proof_hole_min_high"`
	ProofMinMedium      int `json:"proof_hole_min_medium"`
	ProofMinLow         int `json:"proof_hole_min_low"`
	PolicyChecks        int `json:"policy_checks"`
	PolicyPassed        int `json:"policy_passed"`
	PolicyWarnings      int `json:"policy_warnings"`
	PolicyFailed        int `json:"policy_failed"`
	RepairProofs        int `json:"repair_proof_summaries"`
	RepairProofChecked  int `json:"repair_proof_checked"`
	RepairProofCond     int `json:"repair_proof_conditional"`
	RepairProofOpen     int `json:"repair_proof_open"`
	RepairProofRefuted  int `json:"repair_proof_refuted"`
	GrepOnlyMatches     int `json:"grep_only_matches"`
	SQLOnlyRankedRisks  int `json:"sql_only_ranked_risks"`
	IdentifierOnlyLinks int `json:"identifier_only_links"`
	DateOnlyLinks       int `json:"date_only_links"`
}

type BaselineRisk struct {
	ID           string        `json:"id"`
	StableID     string        `json:"stable_id,omitempty"`
	EvidenceHash string        `json:"evidence_hash,omitempty"`
	Path         string        `json:"path"`
	Statement    int           `json:"statement,omitempty"`
	Kind         string        `json:"kind"`
	Table        string        `json:"table,omitempty"`
	Severity     string        `json:"severity"`
	Score        int           `json:"score"`
	Factors      []ScoreFactor `json:"factors,omitempty"`
	Identifiers  []Identifier  `json:"identifiers,omitempty"`
	Rationale    string        `json:"rationale"`
	NextCommand  string        `json:"next_command,omitempty"`
}

type TransactionBoundary struct {
	ID          string       `json:"id"`
	RiskID      string       `json:"risk_id,omitempty"`
	Path        string       `json:"path"`
	Line        int          `json:"line,omitempty"`
	Table       string       `json:"table,omitempty"`
	Surface     string       `json:"surface"`
	Operation   string       `json:"operation,omitempty"`
	Status      string       `json:"status"`
	Confidence  string       `json:"confidence"`
	Markers     []string     `json:"markers,omitempty"`
	Evidence    []string     `json:"evidence,omitempty"`
	Identifiers []Identifier `json:"identifiers,omitempty"`
	Rationale   string       `json:"rationale"`
}

type IdempotencyClass struct {
	ID          string       `json:"id"`
	RiskID      string       `json:"risk_id,omitempty"`
	Path        string       `json:"path"`
	Line        int          `json:"line,omitempty"`
	Table       string       `json:"table,omitempty"`
	Surface     string       `json:"surface"`
	Operation   string       `json:"operation,omitempty"`
	Status      string       `json:"status"`
	Confidence  string       `json:"confidence"`
	Markers     []string     `json:"markers,omitempty"`
	Evidence    []string     `json:"evidence,omitempty"`
	Identifiers []Identifier `json:"identifiers,omitempty"`
	Rationale   string       `json:"rationale"`
}

type LockHazard struct {
	ID          string       `json:"id"`
	RiskID      string       `json:"risk_id,omitempty"`
	Path        string       `json:"path"`
	Line        int          `json:"line,omitempty"`
	Table       string       `json:"table,omitempty"`
	Surface     string       `json:"surface"`
	Operation   string       `json:"operation,omitempty"`
	Severity    string       `json:"severity"`
	Confidence  string       `json:"confidence"`
	Markers     []string     `json:"markers,omitempty"`
	Mitigations []string     `json:"mitigations,omitempty"`
	Evidence    []string     `json:"evidence,omitempty"`
	Identifiers []Identifier `json:"identifiers,omitempty"`
	Rationale   string       `json:"rationale"`
}

type PrivacyHazard struct {
	ID          string       `json:"id"`
	RiskID      string       `json:"risk_id,omitempty"`
	Path        string       `json:"path"`
	Line        int          `json:"line,omitempty"`
	Table       string       `json:"table,omitempty"`
	Surface     string       `json:"surface"`
	Operation   string       `json:"operation,omitempty"`
	Severity    string       `json:"severity"`
	Confidence  string       `json:"confidence"`
	Markers     []string     `json:"markers,omitempty"`
	Mitigations []string     `json:"mitigations,omitempty"`
	Evidence    []string     `json:"evidence,omitempty"`
	Identifiers []Identifier `json:"identifiers,omitempty"`
	Rationale   string       `json:"rationale"`
}

type InvariantCandidate struct {
	ID          string       `json:"id"`
	Source      string       `json:"source"`
	Kind        string       `json:"kind"`
	Path        string       `json:"path"`
	Line        int          `json:"line,omitempty"`
	Table       string       `json:"table,omitempty"`
	Column      string       `json:"column,omitempty"`
	Expression  string       `json:"expression"`
	Confidence  string       `json:"confidence"`
	Evidence    []string     `json:"evidence,omitempty"`
	Identifiers []Identifier `json:"identifiers,omitempty"`
	Rationale   string       `json:"rationale"`
}

type TraceCodeLink struct {
	ID          string       `json:"id"`
	SourcePath  string       `json:"source_path"`
	CodePath    string       `json:"code_path,omitempty"`
	RiskID      string       `json:"risk_id,omitempty"`
	Kind        string       `json:"kind"`
	Relation    string       `json:"relation"`
	Confidence  string       `json:"confidence"`
	Signals     []string     `json:"signals,omitempty"`
	Identifiers []Identifier `json:"identifiers,omitempty"`
	Time        string       `json:"time,omitempty"`
	Evidence    []string     `json:"evidence,omitempty"`
	Rationale   string       `json:"rationale"`
}

type BlastRadiusEstimate struct {
	ID              string       `json:"id"`
	RiskID          string       `json:"risk_id"`
	Table           string       `json:"table"`
	Level           string       `json:"level"`
	Score           int          `json:"score"`
	TableCentrality int          `json:"table_centrality"`
	FKReachability  int          `json:"foreign_key_reachability"`
	CodePathFanout  int          `json:"code_path_fanout"`
	QueryUsage      int          `json:"query_usage"`
	AffectedTables  []string     `json:"affected_tables,omitempty"`
	SourcePaths     []string     `json:"source_paths,omitempty"`
	QueryPaths      []string     `json:"query_paths,omitempty"`
	Evidence        []string     `json:"evidence,omitempty"`
	Identifiers     []Identifier `json:"identifiers,omitempty"`
	Rationale       string       `json:"rationale"`
}

type ProofHoleMinimization struct {
	ID               string       `json:"id"`
	RiskID           string       `json:"risk_id"`
	Table            string       `json:"table,omitempty"`
	Source           string       `json:"source"`
	Hole             string       `json:"hole"`
	MissingEvidence  string       `json:"missing_evidence"`
	UpgradeFrom      string       `json:"upgrade_from"`
	UpgradeTo        string       `json:"upgrade_to"`
	MinimalArtifacts []string     `json:"minimal_artifacts"`
	CandidatePaths   []string     `json:"candidate_paths,omitempty"`
	Effort           int          `json:"effort"`
	Priority         string       `json:"priority"`
	Evidence         []string     `json:"evidence,omitempty"`
	Identifiers      []Identifier `json:"identifiers,omitempty"`
	Rationale        string       `json:"rationale"`
}

type ScoreFactor struct {
	Name   string `json:"name"`
	Weight int    `json:"weight"`
	Reason string `json:"reason"`
}

type RankingExplanation struct {
	RiskID        string                `json:"risk_id"`
	Score         int                   `json:"score"`
	Severity      string                `json:"severity"`
	TopFeature    string                `json:"top_feature,omitempty"`
	Contributions []FeatureContribution `json:"contributions"`
	Ablations     []FeatureAblation     `json:"ablations"`
	Rationale     string                `json:"rationale"`
}

type FeatureContribution struct {
	Feature string  `json:"feature"`
	Weight  int     `json:"weight"`
	Share   float64 `json:"share"`
	Reason  string  `json:"reason"`
}

type FeatureAblation struct {
	Feature         string `json:"feature"`
	ScoreWithout    int    `json:"score_without"`
	SeverityWithout string `json:"severity_without"`
	ChangesSeverity bool   `json:"changes_severity"`
}

type EvidenceLink struct {
	RiskID      string       `json:"risk_id,omitempty"`
	FromID      string       `json:"from_id,omitempty"`
	FactID      string       `json:"fact_id"`
	FactKind    string       `json:"fact_kind"`
	Path        string       `json:"path,omitempty"`
	Identifiers []Identifier `json:"identifiers,omitempty"`
	Confidence  string       `json:"confidence"`
}

type EvidenceCluster struct {
	ID          string         `json:"id"`
	Kind        string         `json:"kind"`
	SourceID    string         `json:"source_id"`
	Path        string         `json:"path,omitempty"`
	Identifiers []Identifier   `json:"identifiers,omitempty"`
	Links       []EvidenceLink `json:"links,omitempty"`
	Rationale   string         `json:"rationale"`
}

type ProvenanceSlice struct {
	ID             string         `json:"id"`
	RiskID         string         `json:"risk_id"`
	Table          string         `json:"table,omitempty"`
	MigrationPath  string         `json:"migration_path,omitempty"`
	SourcePaths    []string       `json:"source_paths,omitempty"`
	TestCommands   []Command      `json:"test_commands,omitempty"`
	NativeCommands []Command      `json:"native_commands,omitempty"`
	IncidentPaths  []string       `json:"incident_paths,omitempty"`
	RepairPaths    []string       `json:"repair_paths,omitempty"`
	Identifiers    []Identifier   `json:"identifiers,omitempty"`
	StagesPresent  []string       `json:"stages_present"`
	Links          []EvidenceLink `json:"links,omitempty"`
	Confidence     string         `json:"confidence"`
	Rationale      string         `json:"rationale"`
}

type DatalogQuery struct {
	Name string       `json:"name"`
	Rule string       `json:"rule"`
	Rows []DatalogRow `json:"rows,omitempty"`
}

type DatalogRow struct {
	Bindings   map[string]string `json:"bindings"`
	Evidence   []string          `json:"evidence,omitempty"`
	Confidence string            `json:"confidence"`
}

type SymbolicCheck struct {
	ID         string   `json:"id"`
	RiskID     string   `json:"risk_id"`
	Property   string   `json:"property"`
	Status     string   `json:"status"`
	Table      string   `json:"table,omitempty"`
	Expression string   `json:"expression"`
	Evidence   []string `json:"evidence,omitempty"`
	Reason     string   `json:"reason"`
}

type TemporalWindow struct {
	ID            string       `json:"id"`
	RiskID        string       `json:"risk_id,omitempty"`
	Table         string       `json:"table,omitempty"`
	Start         string       `json:"start"`
	End           string       `json:"end"`
	Anchor        string       `json:"anchor"`
	Signals       []TimeSignal `json:"signals"`
	RelatedPaths  []string     `json:"related_paths,omitempty"`
	StagesPresent []string     `json:"stages_present,omitempty"`
	Confidence    string       `json:"confidence"`
	Rationale     string       `json:"rationale"`
}

type TimeSignal struct {
	Timestamp   string       `json:"timestamp"`
	Source      string       `json:"source"`
	Path        string       `json:"path"`
	Stage       string       `json:"stage"`
	Identifiers []Identifier `json:"identifiers,omitempty"`
}

type RecurrencePattern struct {
	ID          string   `json:"id"`
	Key         string   `json:"key"`
	Table       string   `json:"table,omitempty"`
	Kind        string   `json:"kind"`
	Effect      string   `json:"effect,omitempty"`
	RiskIDs     []string `json:"risk_ids"`
	Paths       []string `json:"paths"`
	RepairPaths []string `json:"repair_paths,omitempty"`
	Count       int      `json:"count"`
	Confidence  string   `json:"confidence"`
	Rationale   string   `json:"rationale"`
	NextCommand string   `json:"next_command,omitempty"`
}

type PolicyCheck struct {
	ID          string   `json:"id"`
	RiskID      string   `json:"risk_id"`
	Policy      string   `json:"policy"`
	Status      string   `json:"status"`
	RiskClass   string   `json:"risk_class"`
	Required    []string `json:"required"`
	Satisfied   []string `json:"satisfied,omitempty"`
	Missing     []string `json:"missing,omitempty"`
	Evidence    []string `json:"evidence,omitempty"`
	ReviewLevel string   `json:"review_level"`
	Rationale   string   `json:"rationale"`
}

type RepairProofSummary struct {
	ID           string   `json:"id"`
	RiskID       string   `json:"risk_id"`
	RepairSource string   `json:"repair_source"`
	RepairPaths  []string `json:"repair_paths,omitempty"`
	Table        string   `json:"table"`
	Status       string   `json:"status"`
	ScopeStatus  string   `json:"scope_status"`
	FrameStatus  string   `json:"frame_status"`
	Obligations  []string `json:"obligations"`
	ProofHoles   []string `json:"proof_holes,omitempty"`
	Evidence     []string `json:"evidence,omitempty"`
	NextCommand  string   `json:"next_command,omitempty"`
	Rationale    string   `json:"rationale"`
}

func Baseline(inv Inventory, facts []Fact, intakeReport intake.Report) BaselineReport {
	report := BaselineReport{Version: BaselineVersion, InventoryRoot: inv.Root, IntakeSource: intakeReport.Source.Input}
	factIndex := indexFacts(facts)
	report.Risks = rankRisks(inv, intakeReport, factIndex)
	report.Rankings = buildRankingExplanations(report.Risks)
	report.EvidenceLinks = linkRisks(report.Risks, factIndex)
	report.CauseClusters = clusterCandidates("cause", intakeReport.Causes, factIndex)
	report.RepairClusters = clusterRepairCandidates(intakeReport.RepairCandidates, factIndex)
	report.NativeChecks = uniqueCommands(append([]Command(nil), inv.TestCommands...))
	report.Provenance = buildProvenanceSlices(inv, report.Risks, intakeReport, factIndex)
	assignStableRiskIDs(report.Risks, report.Provenance)
	report.DatalogQueries = buildDatalogQueries(report.Risks, report.Provenance)
	report.AbstractEffects = buildAbstractEffectSummaries(report.Risks, report.Provenance)
	report.SymbolicChecks = buildSymbolicChecks(report.Risks, report.AbstractEffects, report.Provenance)
	report.TemporalWindows = buildTemporalWindows(report.Risks, report.Provenance, intakeReport)
	report.Recurrences = buildRecurrences(report.Risks, report.AbstractEffects, report.Provenance)
	report.Transactions = buildTransactionBoundaries(report.Risks, report.Provenance, intakeReport)
	report.Idempotency = buildIdempotencyClasses(report.Risks, report.Provenance, report.SymbolicChecks, facts, intakeReport)
	report.LockHazards = buildLockHazards(report.Risks, report.Provenance, facts, intakeReport)
	report.PrivacyHazards = buildPrivacyHazards(report.Risks, report.Provenance, facts, intakeReport)
	report.Invariants = buildInvariantCandidates(inv.Root, facts, intakeReport)
	report.TraceLinks = buildTraceCodeLinks(inv.Root, report.Risks, report.Provenance, facts, intakeReport)
	report.BlastRadius = buildBlastRadiusEstimates(inv.Root, report.Risks, report.Provenance, facts, intakeReport)
	report.PolicyChecks = buildPolicyChecks(report.Risks, report.Provenance, report.SymbolicChecks)
	report.RepairProofs = buildRepairProofSummaries(report.Risks, report.Provenance, report.AbstractEffects, report.SymbolicChecks)
	report.ProofMinimizers = buildProofHoleMinimizations(inv.Root, report.Risks, report.Provenance, report.SymbolicChecks, report.PolicyChecks, report.RepairProofs, report.AbstractEffects, facts, intakeReport)
	report.Summary = BaselineSummary{
		RankedRisks:         len(report.Risks),
		CodePathRankedRisks: countCodePathRisks(report.Risks),
		RankingExplanations: len(report.Rankings),
		RankingFeatures:     countRankingFeatures(report.Rankings),
		AblationSensitive:   countAblationSensitive(report.Rankings),
		EvidenceLinks:       len(report.EvidenceLinks),
		CauseClusters:       len(report.CauseClusters),
		RepairClusters:      len(report.RepairClusters),
		ProvenanceSlices:    len(report.Provenance),
		DatalogQueries:      len(report.DatalogQueries),
		DatalogRows:         countDatalogRows(report.DatalogQueries),
		AbstractEffects:     len(report.AbstractEffects),
		AbstractOperations:  countAbstractOperations(report.AbstractEffects),
		AbstractProofHoles:  countAbstractProofHoles(report.AbstractEffects),
		SymbolicChecks:      len(report.SymbolicChecks),
		SymbolicPassed:      countSymbolicStatus(report.SymbolicChecks, "pass"),
		SymbolicWarnings:    countSymbolicStatus(report.SymbolicChecks, "warn"),
		SymbolicFailed:      countSymbolicStatus(report.SymbolicChecks, "fail"),
		TemporalWindows:     len(report.TemporalWindows),
		TemporalSignals:     countTemporalSignals(report.TemporalWindows),
		Recurrences:         len(report.Recurrences),
		RecurringRisks:      countRecurringRisks(report.Recurrences),
		Transactions:        len(report.Transactions),
		TransactionExplicit: countTransactionStatus(report.Transactions, "explicit"),
		TransactionMissing:  countTransactionStatus(report.Transactions, "missing"),
		TransactionPartial:  countTransactionStatus(report.Transactions, "partial"),
		IdempotencyClasses:  len(report.Idempotency),
		IdempotencyProven:   countIdempotencyStatus(report.Idempotency, "proven"),
		IdempotencyGuarded:  countIdempotencyStatus(report.Idempotency, "guarded"),
		IdempotencyUnknown:  countIdempotencyStatus(report.Idempotency, "unknown"),
		IdempotencyUnsafe:   countIdempotencyStatus(report.Idempotency, "non_idempotent"),
		LockHazards:         len(report.LockHazards),
		LockHazardCritical:  countLockHazardSeverity(report.LockHazards, "critical"),
		LockHazardHigh:      countLockHazardSeverity(report.LockHazards, "high"),
		LockHazardMedium:    countLockHazardSeverity(report.LockHazards, "medium"),
		LockHazardLow:       countLockHazardSeverity(report.LockHazards, "low"),
		PrivacyHazards:      len(report.PrivacyHazards),
		PrivacyCritical:     countPrivacyHazardSeverity(report.PrivacyHazards, "critical"),
		PrivacyHigh:         countPrivacyHazardSeverity(report.PrivacyHazards, "high"),
		PrivacyMedium:       countPrivacyHazardSeverity(report.PrivacyHazards, "medium"),
		PrivacyLow:          countPrivacyHazardSeverity(report.PrivacyHazards, "low"),
		Invariants:          len(report.Invariants),
		InvariantSchema:     countInvariantSource(report.Invariants, "schema"),
		InvariantTests:      countInvariantSource(report.Invariants, "test"),
		InvariantValidation: countInvariantSource(report.Invariants, "validation"),
		InvariantFixtures:   countInvariantSource(report.Invariants, "fixture"),
		TraceCodeLinks:      len(report.TraceLinks),
		TraceLinkExact:      countTraceLinkConfidence(report.TraceLinks, "exact"),
		TraceLinkCausal:     countTraceLinkConfidence(report.TraceLinks, "causal"),
		TraceLinkTemporal:   countTraceLinkConfidence(report.TraceLinks, "temporal"),
		TraceLinkInferred:   countTraceLinkConfidence(report.TraceLinks, "inferred"),
		BlastRadius:         len(report.BlastRadius),
		BlastRadiusHigh:     countBlastRadiusLevel(report.BlastRadius, "high"),
		BlastRadiusMedium:   countBlastRadiusLevel(report.BlastRadius, "medium"),
		BlastRadiusLow:      countBlastRadiusLevel(report.BlastRadius, "low"),
		ProofMinimizations:  len(report.ProofMinimizers),
		ProofMinCritical:    countProofMinimizationPriority(report.ProofMinimizers, "critical"),
		ProofMinHigh:        countProofMinimizationPriority(report.ProofMinimizers, "high"),
		ProofMinMedium:      countProofMinimizationPriority(report.ProofMinimizers, "medium"),
		ProofMinLow:         countProofMinimizationPriority(report.ProofMinimizers, "low"),
		PolicyChecks:        len(report.PolicyChecks),
		PolicyPassed:        countPolicyStatus(report.PolicyChecks, "pass"),
		PolicyWarnings:      countPolicyStatus(report.PolicyChecks, "warn"),
		PolicyFailed:        countPolicyStatus(report.PolicyChecks, "fail"),
		RepairProofs:        len(report.RepairProofs),
		RepairProofChecked:  countRepairProofStatus(report.RepairProofs, "checked"),
		RepairProofCond:     countRepairProofStatus(report.RepairProofs, "conditional"),
		RepairProofOpen:     countRepairProofStatus(report.RepairProofs, "open"),
		RepairProofRefuted:  countRepairProofStatus(report.RepairProofs, "refuted"),
		GrepOnlyMatches:     grepOnlyMatches(inv.Root),
		SQLOnlyRankedRisks:  sqlOnlyRankedRisks(intakeReport),
		IdentifierOnlyLinks: countLinksByIdentifierKind(report.EvidenceLinks, false),
		DateOnlyLinks:       countLinksByIdentifierKind(report.EvidenceLinks, true),
	}
	report.Hash = baselineHash(report)
	report.Markdown = renderBaselineMarkdown(report)
	return report
}

func WriteBaseline(outDir string, report BaselineReport) error {
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}
	copy := report
	copy.Markdown = ""
	data, err := json.MarshalIndent(copy, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(outDir, "baseline.json"), append(data, '\n'), 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(outDir, "baseline.md"), []byte(report.Markdown), 0o644); err != nil {
		return err
	}
	sarif, err := json.MarshalIndent(renderBaselineSARIF(report), "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(outDir, "baseline.sarif"), append(sarif, '\n'), 0o644)
}

func LoadInventory(path string) (Inventory, string, error) {
	inventoryPath, baseDir := resolveInventoryPath(path)
	data, err := os.ReadFile(inventoryPath)
	if err != nil {
		return Inventory{}, "", err
	}
	var inv Inventory
	if err := json.Unmarshal(data, &inv); err != nil {
		return Inventory{}, "", err
	}
	facts, err := LoadFacts(filepath.Join(baseDir, "facts.jsonl"))
	if err != nil {
		return Inventory{}, "", err
	}
	inv.Facts = facts
	return inv, baseDir, nil
}

func LoadFacts(path string) ([]Fact, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	var facts []Fact
	scanner := bufio.NewScanner(file)
	buffer := make([]byte, 0, 64*1024)
	scanner.Buffer(buffer, 4*1024*1024)
	for scanner.Scan() {
		var fact Fact
		if err := json.Unmarshal(scanner.Bytes(), &fact); err != nil {
			return nil, err
		}
		facts = append(facts, fact)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return facts, nil
}

func LoadIntakeReport(path string) (intake.Report, error) {
	reportPath := path
	if info, err := os.Stat(path); err == nil && info.IsDir() {
		reportPath = filepath.Join(path, "summary.json")
	}
	data, err := os.ReadFile(reportPath)
	if err != nil {
		return intake.Report{}, err
	}
	var report intake.Report
	if err := json.Unmarshal(data, &report); err != nil {
		return intake.Report{}, err
	}
	return report, nil
}

func resolveInventoryPath(path string) (string, string) {
	if info, err := os.Stat(path); err == nil && info.IsDir() {
		return filepath.Join(path, "inventory.json"), path
	}
	return path, filepath.Dir(path)
}

type factIndex struct {
	byIdentifier map[string][]Fact
}

func indexFacts(facts []Fact) factIndex {
	idx := factIndex{byIdentifier: map[string][]Fact{}}
	for _, fact := range facts {
		for _, id := range fact.Identifiers {
			key := canonicalIdentifier(id.Kind, id.Value)
			if key != "" {
				idx.byIdentifier[key] = append(idx.byIdentifier[key], fact)
			}
		}
	}
	for key := range idx.byIdentifier {
		sort.Slice(idx.byIdentifier[key], func(i, j int) bool {
			if idx.byIdentifier[key][i].Kind != idx.byIdentifier[key][j].Kind {
				return idx.byIdentifier[key][i].Kind < idx.byIdentifier[key][j].Kind
			}
			if idx.byIdentifier[key][i].Path != idx.byIdentifier[key][j].Path {
				return idx.byIdentifier[key][i].Path < idx.byIdentifier[key][j].Path
			}
			return idx.byIdentifier[key][i].ID < idx.byIdentifier[key][j].ID
		})
	}
	return idx
}

func rankRisks(inv Inventory, report intake.Report, facts factIndex) []BaselineRisk {
	var risks []BaselineRisk
	for _, finding := range report.SQL {
		for _, statement := range finding.Statements {
			if statement.Risk != migration.RiskHigh && statement.Risk != migration.RiskMedium {
				continue
			}
			if finding.SourceKind == "loose_text" && isSQLIdentifierStopword(statement.Table) {
				continue
			}
			risk := riskFromStatement(finding.Path, finding.SourceKind, statement)
			addEvidenceFactors(&risk, facts)
			risks = append(risks, risk)
		}
	}
	for _, problem := range report.Problems {
		if problem.Severity != "high" {
			continue
		}
		if problem.Kind == "high-risk-sql" && isSQLIdentifierStopword(problem.Table) {
			continue
		}
		risk := BaselineRisk{
			ID:          "risk:" + canonical.Hash("problem\x00" + problem.ID)[:16],
			Path:        problem.Path,
			Kind:        problem.Kind,
			Table:       problem.Table,
			Severity:    problem.Severity,
			Identifiers: identifiersFromIntake(problem.Identifiers, problem.Table),
			Rationale:   problem.Rationale,
		}
		addFactor(&risk, "intake-problem", 80, "intake produced a high-severity problem candidate")
		addEvidenceFactors(&risk, facts)
		risks = append(risks, risk)
	}
	risks = append(risks, rankSourceCodePathRisks(inv.Root, report.SourceSQL, facts)...)
	risks = append(risks, rankSchemaEvolutionCodeRisks(inv.Facts, facts)...)
	risks = uniqueRisks(risks)
	sort.Slice(risks, func(i, j int) bool {
		if risks[i].Score != risks[j].Score {
			return risks[i].Score > risks[j].Score
		}
		if risks[i].Path != risks[j].Path {
			return risks[i].Path < risks[j].Path
		}
		return risks[i].ID < risks[j].ID
	})
	return risks
}

func assignStableRiskIDs(risks []BaselineRisk, provenance []ProvenanceSlice) {
	provenanceByRisk := map[string]ProvenanceSlice{}
	for _, slice := range provenance {
		provenanceByRisk[slice.RiskID] = slice
	}
	for i := range risks {
		risks[i].StableID = stableRiskID(risks[i], provenanceByRisk[risks[i].ID])
		risks[i].EvidenceHash = riskEvidenceHash(risks[i])
	}
}

func stableRiskID(risk BaselineRisk, slice ProvenanceSlice) string {
	ids := make([]string, 0, len(risk.Identifiers)+len(slice.Identifiers))
	for _, id := range append(append([]Identifier(nil), risk.Identifiers...), slice.Identifiers...) {
		if key := canonicalIdentifier(id.Kind, id.Value); key != "" && !strings.HasPrefix(key, "path:") {
			ids = append(ids, key)
		}
	}
	sort.Strings(ids)
	ids = uniqueStringsForStableID(ids)
	factors := make([]string, 0, len(risk.Factors))
	for _, factor := range risk.Factors {
		if factor.Name != "" {
			factors = append(factors, strings.ToLower(strings.TrimSpace(factor.Name)))
		}
	}
	sort.Strings(factors)
	stages := append([]string(nil), slice.StagesPresent...)
	sort.Strings(stages)
	payload := strings.Join([]string{
		"stable-risk-v1",
		operationFamily(risk.Kind),
		strings.ToLower(strings.TrimSpace(risk.Table)),
		strings.Join(ids, ","),
		strings.Join(uniqueStringsForStableID(factors), ","),
		strings.Join(uniqueStringsForStableID(stages), ","),
	}, "\x00")
	return "stable-risk:" + canonical.Hash(payload)[:16]
}

func operationFamily(kind string) string {
	kind = strings.ToLower(strings.TrimSpace(kind))
	switch {
	case strings.Contains(kind, "delete"), strings.Contains(kind, "drop"), strings.Contains(kind, "truncate"):
		return "destructive"
	case strings.Contains(kind, "update"), strings.Contains(kind, "backfill"), strings.Contains(kind, "write"):
		return "write"
	case strings.Contains(kind, "schema"), strings.Contains(kind, "migration"):
		return "schema"
	case strings.Contains(kind, "sql"):
		return "sql"
	default:
		return kind
	}
}

func riskEvidenceHash(risk BaselineRisk) string {
	return "sha256:" + canonical.Hash(strings.Join([]string{
		risk.StableID,
		strings.ToLower(strings.TrimSpace(risk.Table)),
		operationFamily(risk.Kind),
		risk.Severity,
	}, "\x00"))
}

func uniqueStringsForStableID(values []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}

func riskFromStatement(path, sourceKind string, statement migration.Statement) BaselineRisk {
	risk := BaselineRisk{
		ID:          "risk:" + canonical.Hash(fmt.Sprintf("%s\x00%d\x00%s", path, statement.Index, statement.Fingerprint))[:16],
		Path:        path,
		Statement:   statement.Index,
		Kind:        statement.Kind,
		Table:       statement.Table,
		Severity:    string(statement.Risk),
		Identifiers: identifiersFromStatement(statement),
		Rationale:   strings.Join(statement.Reasons, "; "),
		NextCommand: fmt.Sprintf("patchline analyze-migration %s --json", shellPath(path)),
	}
	switch statement.Risk {
	case migration.RiskHigh:
		addFactor(&risk, "high-risk-sql", 100, "SQL analyzer classified this statement as high risk")
	case migration.RiskMedium:
		addFactor(&risk, "medium-risk-sql", 30, "SQL analyzer classified this statement as medium risk")
	}
	if sourceKind == "loose_text" {
		addFactor(&risk, "loose-sql", 10, "SQL was found in a non-SQL file and should be inspected in context")
	}
	for _, reason := range statement.Reasons {
		lower := strings.ToLower(reason)
		switch {
		case strings.Contains(lower, "unbounded") || strings.Contains(lower, "broad"):
			addFactor(&risk, "broad-write", 20, reason)
		case strings.Contains(lower, "destructive") || strings.Contains(lower, "delete") || strings.Contains(lower, "drop"):
			addFactor(&risk, "destructive-effect", 20, reason)
		}
	}
	return risk
}

func identifiersFromStatement(statement migration.Statement) []Identifier {
	var ids []Identifier
	if statement.Table != "" {
		ids = append(ids, Identifier{Kind: "table", Value: statement.Table})
	}
	return uniqueIdentifiers(ids)
}

func identifiersFromIntake(raw []string, table string) []Identifier {
	var ids []Identifier
	if table != "" {
		ids = append(ids, Identifier{Kind: "table", Value: table})
	}
	for _, value := range raw {
		kind, val, ok := strings.Cut(value, ":")
		if ok {
			ids = append(ids, Identifier{Kind: kind, Value: val})
		}
	}
	return uniqueIdentifiers(ids)
}

func rankSourceCodePathRisks(inventoryRoot string, sourceSQL migration.SourceSQLReport, facts factIndex) []BaselineRisk {
	root := firstNonEmpty(sourceSQL.Root, inventoryRoot)
	var risks []BaselineRisk
	for _, obs := range sourceSQL.Observations {
		if !isPersistentCodeObservation(obs) {
			continue
		}
		window := sourceWindowForObservation(root, obs)
		risk := codePathRiskFromObservation(obs, window)
		addEvidenceFactors(&risk, facts)
		risks = append(risks, risk)
	}
	return risks
}

func isPersistentCodeObservation(obs migration.SourceSQLObservation) bool {
	switch obs.Kind {
	case "orm_query":
		return isPersistentWriteOperation(obs.Operation)
	case "migration_framework":
		return isSchemaWriteOperation(obs.Operation)
	default:
		return false
	}
}

func isPersistentWriteOperation(operation string) bool {
	switch strings.ToLower(operation) {
	case "update", "delete", "insert", "save", "merge", "persist", "upsert", "update_or_create", "bulk_create", "bulk_update":
		return true
	default:
		return false
	}
}

func isSchemaWriteOperation(operation string) bool {
	switch strings.ToLower(operation) {
	case "create_table", "drop_table", "add_column", "remove_column", "drop_column", "create", "alter", "drop":
		return true
	default:
		return false
	}
}

func codePathRiskFromObservation(obs migration.SourceSQLObservation, window string) BaselineRisk {
	operation := strings.ToLower(obs.Operation)
	kind := "code-path:" + operation
	risk := BaselineRisk{
		ID:          "risk:" + canonical.Hash(fmt.Sprintf("code-path\x00%s\x00%d\x00%s\x00%s", obs.Path, obs.Line, operation, obs.SnippetHash))[:16],
		Path:        obs.Path,
		Statement:   obs.Line,
		Kind:        kind,
		Table:       obs.Table,
		Severity:    "medium",
		Identifiers: identifiersFromSourceObservation(obs),
		Rationale:   "project-native source path performs a persistent write; review breadth, idempotency, transaction, retry, and rollback behavior",
		NextCommand: fmt.Sprintf("patchline extract-sql %s --json", shellPath(obs.Path)),
	}
	addFactor(&risk, "persistent-write-code-path", 45, "source or migration framework observation writes persistent data")
	switch operation {
	case "delete", "drop_table", "remove_column", "drop_column", "drop":
		addFactor(&risk, "destructive-code-path", 45, "operation can delete rows or remove schema/data surfaces")
	case "update", "alter", "add_column":
		addFactor(&risk, "write-breadth-unknown", 35, "operation mutates existing persistent records or schema")
	case "insert", "create_table", "create":
		addFactor(&risk, "persistent-create-path", 20, "operation creates persistent records or schema")
	}
	lower := strings.ToLower(window)
	if window == "" {
		addFactor(&risk, "source-window-unavailable", 5, "source window was unavailable; risk remains based on extracted operation metadata")
	} else {
		if (operation == "update" || operation == "delete") && !hasScopedWriteMarker(lower) {
			addFactor(&risk, "broad-write", 20, "write path lacks an obvious where/filter/id/limit scope marker in nearby source")
		}
		if !containsAny(lower, "transaction", "atomic", "begin", "commit") {
			addFactor(&risk, "missing-transaction-boundary", 15, "nearby source lacks an obvious transaction boundary")
		}
		if !containsAny(lower, "idempot", "upsert", "on conflict", "unique", "retry-safe", "retry_safe") {
			addFactor(&risk, "missing-idempotency", 10, "nearby source lacks an obvious idempotency or uniqueness marker")
		}
		if !containsAny(lower, "rollback", "revert", "dry_run", "dry-run", "dryrun") {
			addFactor(&risk, "weak-rollback-signal", 10, "nearby source lacks an obvious rollback, revert, or dry-run marker")
		}
		if containsAny(lower, "worker", "job", "retry", "cron", "background") && !containsAny(lower, "idempot", "upsert", "unique", "retry-safe", "retry_safe") {
			addFactor(&risk, "retry-hazard", 10, "write appears near retry/background execution signals without obvious idempotency")
		}
	}
	risk.Severity = severityForScore(risk.Score)
	return risk
}

func identifiersFromSourceObservation(obs migration.SourceSQLObservation) []Identifier {
	var ids []Identifier
	if obs.Table != "" {
		ids = append(ids, Identifier{Kind: "table", Value: obs.Table}, Identifier{Kind: "model", Value: obs.Table})
	}
	if obs.Framework != "" {
		ids = append(ids, Identifier{Kind: "framework", Value: obs.Framework})
	}
	if obs.Operation != "" {
		ids = append(ids, Identifier{Kind: "operation", Value: obs.Operation})
	}
	return uniqueIdentifiers(ids)
}

func sourceWindowForObservation(root string, obs migration.SourceSQLObservation) string {
	if root == "" || obs.Path == "" {
		return ""
	}
	path := obs.Path
	if !filepath.IsAbs(path) {
		path = filepath.Join(root, filepath.FromSlash(obs.Path))
	}
	text, err := readTextPrefix(path, factContentLimit)
	if err != nil || text == "" {
		return ""
	}
	lines := strings.Split(text, "\n")
	if obs.Line <= 0 || obs.Line > len(lines) {
		return ""
	}
	start := obs.Line - 4
	if start < 0 {
		start = 0
	}
	end := obs.Line + 4
	if end > len(lines) {
		end = len(lines)
	}
	return strings.Join(lines[start:end], "\n")
}

func hasScopedWriteMarker(lower string) bool {
	for _, marker := range []string{".where", "where(", "filter(", "find_by", "limit(", "where ", " id ", "_id", ".id"} {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}

func rankSchemaEvolutionCodeRisks(facts []Fact, index factIndex) []BaselineRisk {
	var risks []BaselineRisk
	for _, fact := range facts {
		if fact.Kind != "schema_evolution" {
			continue
		}
		operation := strings.ToLower(fact.Properties["operation"])
		if !isSchemaWriteOperation(operation) && operation != "create_model" && operation != "add_field" && operation != "model" && operation != "field" {
			continue
		}
		table := firstNonEmpty(fact.Properties["table"], firstIdentifierValue(fact.Identifiers, "table"), firstIdentifierValue(fact.Identifiers, "model"))
		risk := BaselineRisk{
			ID:          "risk:" + canonical.Hash("schema-code-path\x00" + fact.ID)[:16],
			Path:        fact.Path,
			Kind:        "code-path:schema:" + operation,
			Table:       table,
			Severity:    "medium",
			Identifiers: fact.Identifiers,
			Rationale:   "project-native migration or ORM declaration changes persistent schema without requiring a pre-authored schema",
			NextCommand: fmt.Sprintf("patchline repo inventory %s --json", shellPath(fact.Path)),
		}
		addFactor(&risk, "schema-code-path", 35, "project-native migration or ORM declaration changes persistent schema")
		switch operation {
		case "drop_table", "remove_column", "drop_column", "drop":
			addFactor(&risk, "destructive-schema-change", 45, "schema change removes persistent data surface")
		case "alter_table", "add_column", "add_field", "field":
			addFactor(&risk, "schema-write-breadth", 20, "schema change can affect existing records or code paths")
		default:
			addFactor(&risk, "schema-create-path", 10, "schema declaration creates a persistent data surface")
		}
		addEvidenceFactors(&risk, index)
		risk.Severity = severityForScore(risk.Score)
		risks = append(risks, risk)
	}
	return risks
}

func firstIdentifierValue(ids []Identifier, kind string) string {
	for _, id := range ids {
		if id.Kind == kind {
			return id.Value
		}
	}
	return ""
}

func severityForScore(score int) string {
	switch {
	case score >= 90:
		return "high"
	case score >= 50:
		return "medium"
	default:
		return "low"
	}
}

func addEvidenceFactors(risk *BaselineRisk, facts factIndex) {
	matches := matchingFacts(risk.Identifiers, facts)
	if len(matches) == 0 {
		return
	}
	addFactor(risk, "linked-project-evidence", minInt(len(matches), 5)*2, "project facts share identifiers with this risk")
	for _, fact := range matches {
		switch fact.Kind {
		case "operational_doc", "evidence_export":
			addFactor(risk, "operational-context", 10, "operational evidence shares identifiers with this risk")
			return
		case "test_command":
			addFactor(risk, "native-check-available", 5, "native project check is available")
			return
		}
	}
}

func addFactor(risk *BaselineRisk, name string, weight int, reason string) {
	risk.Factors = append(risk.Factors, ScoreFactor{Name: name, Weight: weight, Reason: reason})
	risk.Score += weight
}

func buildRankingExplanations(risks []BaselineRisk) []RankingExplanation {
	explanations := make([]RankingExplanation, 0, len(risks))
	for _, risk := range risks {
		if len(risk.Factors) == 0 {
			continue
		}
		contributions := make([]FeatureContribution, 0, len(risk.Factors))
		ablations := make([]FeatureAblation, 0, len(risk.Factors))
		score := risk.Score
		if score < 0 {
			score = 0
		}
		for _, factor := range risk.Factors {
			share := 0.0
			if score > 0 {
				share = float64(factor.Weight) / float64(score)
			}
			scoreWithout := score - factor.Weight
			if scoreWithout < 0 {
				scoreWithout = 0
			}
			severityWithout := severityForScore(scoreWithout)
			contributions = append(contributions, FeatureContribution{
				Feature: factor.Name,
				Weight:  factor.Weight,
				Share:   share,
				Reason:  factor.Reason,
			})
			ablations = append(ablations, FeatureAblation{
				Feature:         factor.Name,
				ScoreWithout:    scoreWithout,
				SeverityWithout: severityWithout,
				ChangesSeverity: severityWithout != risk.Severity,
			})
		}
		sort.Slice(contributions, func(i, j int) bool {
			if contributions[i].Weight != contributions[j].Weight {
				return contributions[i].Weight > contributions[j].Weight
			}
			return contributions[i].Feature < contributions[j].Feature
		})
		sort.Slice(ablations, func(i, j int) bool {
			if ablations[i].ChangesSeverity != ablations[j].ChangesSeverity {
				return ablations[i].ChangesSeverity
			}
			if ablations[i].ScoreWithout != ablations[j].ScoreWithout {
				return ablations[i].ScoreWithout < ablations[j].ScoreWithout
			}
			return ablations[i].Feature < ablations[j].Feature
		})
		topFeature := ""
		if len(contributions) > 0 {
			topFeature = contributions[0].Feature
		}
		explanations = append(explanations, RankingExplanation{
			RiskID:        risk.ID,
			Score:         score,
			Severity:      risk.Severity,
			TopFeature:    topFeature,
			Contributions: contributions,
			Ablations:     ablations,
			Rationale:     "ranking explanation expands score factors into per-feature contributions and leave-one-feature ablations",
		})
	}
	sort.Slice(explanations, func(i, j int) bool {
		if explanations[i].Score != explanations[j].Score {
			return explanations[i].Score > explanations[j].Score
		}
		return explanations[i].RiskID < explanations[j].RiskID
	})
	if len(explanations) > 400 {
		explanations = explanations[:400]
	}
	return explanations
}

func countRankingFeatures(explanations []RankingExplanation) int {
	seen := map[string]bool{}
	for _, explanation := range explanations {
		for _, contribution := range explanation.Contributions {
			seen[contribution.Feature] = true
		}
	}
	return len(seen)
}

func countAblationSensitive(explanations []RankingExplanation) int {
	count := 0
	for _, explanation := range explanations {
		for _, ablation := range explanation.Ablations {
			if ablation.ChangesSeverity {
				count++
				break
			}
		}
	}
	return count
}

func countExplanationSeverityChanges(explanation RankingExplanation) int {
	count := 0
	for _, ablation := range explanation.Ablations {
		if ablation.ChangesSeverity {
			count++
		}
	}
	return count
}

func matchingFacts(ids []Identifier, facts factIndex) []Fact {
	seen := map[string]bool{}
	var out []Fact
	for _, id := range ids {
		for _, fact := range facts.byIdentifier[canonicalIdentifier(id.Kind, id.Value)] {
			if seen[fact.ID] {
				continue
			}
			seen[fact.ID] = true
			out = append(out, fact)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if factKindPriority(out[i].Kind) != factKindPriority(out[j].Kind) {
			return factKindPriority(out[i].Kind) < factKindPriority(out[j].Kind)
		}
		if out[i].Path != out[j].Path {
			return out[i].Path < out[j].Path
		}
		return out[i].ID < out[j].ID
	})
	return out
}

func capFacts(in []Fact, n int) []Fact {
	if len(in) <= n {
		return in
	}
	return in[:n]
}

func factKindPriority(kind string) int {
	switch kind {
	case "operational_doc", "evidence_export":
		return 0
	case "repair_candidate", "cause":
		return 1
	case "migration_root", "migration_system", "source_sql_hint":
		return 2
	case "file":
		return 3
	default:
		return 4
	}
}

func linkRisks(risks []BaselineRisk, facts factIndex) []EvidenceLink {
	var links []EvidenceLink
	for _, risk := range risks {
		for _, fact := range capFacts(matchingFacts(risk.Identifiers, facts), 5) {
			links = append(links, EvidenceLink{RiskID: risk.ID, FactID: fact.ID, FactKind: fact.Kind, Path: fact.Path, Identifiers: sharedIdentifiers(risk.Identifiers, fact.Identifiers), Confidence: "identifier"})
		}
	}
	return uniqueLinks(links)
}

type candidateLike struct {
	ID          string
	Path        string
	Identifiers []string
	Rationale   string
}

func clusterCandidates(kind string, candidates []intake.CauseCandidate, facts factIndex) []EvidenceCluster {
	var clusters []EvidenceCluster
	for _, candidate := range candidates {
		clusters = append(clusters, clusterForCandidate(kind, candidateLike{ID: candidate.ID, Path: candidate.Path, Identifiers: candidate.Identifiers, Rationale: candidate.Rationale}, facts))
	}
	return nonEmptyClusters(clusters)
}

func clusterRepairCandidates(candidates []intake.RepairCandidate, facts factIndex) []EvidenceCluster {
	var clusters []EvidenceCluster
	for _, candidate := range candidates {
		clusters = append(clusters, clusterForCandidate("repair", candidateLike{ID: candidate.ID, Path: candidate.Path, Identifiers: candidate.Identifiers, Rationale: candidate.Rationale}, facts))
	}
	return nonEmptyClusters(clusters)
}

func clusterForCandidate(kind string, candidate candidateLike, facts factIndex) EvidenceCluster {
	ids := identifiersFromIntake(candidate.Identifiers, "")
	cluster := EvidenceCluster{ID: "cluster:" + canonical.Hash(kind + "\x00" + candidate.ID)[:16], Kind: kind, SourceID: candidate.ID, Path: candidate.Path, Identifiers: ids, Rationale: candidate.Rationale}
	for _, fact := range capFacts(matchingFacts(ids, facts), 5) {
		cluster.Links = append(cluster.Links, EvidenceLink{FromID: candidate.ID, FactID: fact.ID, FactKind: fact.Kind, Path: fact.Path, Identifiers: sharedIdentifiers(ids, fact.Identifiers), Confidence: "identifier"})
	}
	cluster.Links = uniqueLinks(cluster.Links)
	return cluster
}

func nonEmptyClusters(in []EvidenceCluster) []EvidenceCluster {
	var out []EvidenceCluster
	for _, cluster := range in {
		if len(cluster.Links) > 0 {
			out = append(out, cluster)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if len(out[i].Links) != len(out[j].Links) {
			return len(out[i].Links) > len(out[j].Links)
		}
		return out[i].ID < out[j].ID
	})
	return out
}

func buildProvenanceSlices(inv Inventory, risks []BaselineRisk, report intake.Report, facts factIndex) []ProvenanceSlice {
	var slices []ProvenanceSlice
	for _, risk := range risks {
		ids := provenanceSeedIdentifiers(risk)
		if len(ids) == 0 {
			continue
		}
		matches := matchingFacts(ids, facts)
		slice := ProvenanceSlice{
			ID:             "slice:" + canonical.Hash("provenance\x00" + risk.ID)[:16],
			RiskID:         risk.ID,
			Table:          firstNonEmpty(risk.Table, firstIdentifierValue(ids, "table"), firstIdentifierValue(ids, "model")),
			MigrationPath:  provenanceMigrationPath(risk, matches),
			SourcePaths:    provenanceSourcePaths(risk, matches),
			TestCommands:   uniqueCommands(inv.TestCommands),
			NativeCommands: uniqueCommands(inv.NativeCommands),
			Identifiers:    ids,
			Links:          provenanceLinks(risk, ids, matches),
			Confidence:     "identifier-shared",
			Rationale:      "slice groups repo evidence that co-references the same table or strong identifier; it is a navigation aid, not proof of causality",
		}
		slice.IncidentPaths = provenanceIncidentPaths(ids, report.Causes, matches)
		slice.RepairPaths = provenanceRepairPaths(ids, report.RepairCandidates, matches)
		slice.StagesPresent = provenanceStages(slice)
		if len(slice.StagesPresent) < 2 || slice.Table == "" {
			continue
		}
		slices = append(slices, slice)
	}
	slices = uniqueProvenanceSlices(slices)
	sort.Slice(slices, func(i, j int) bool {
		if len(slices[i].StagesPresent) != len(slices[j].StagesPresent) {
			return len(slices[i].StagesPresent) > len(slices[j].StagesPresent)
		}
		return slices[i].ID < slices[j].ID
	})
	if len(slices) > 100 {
		slices = slices[:100]
	}
	return slices
}

func provenanceSeedIdentifiers(risk BaselineRisk) []Identifier {
	ids := append([]Identifier(nil), risk.Identifiers...)
	if risk.Table != "" {
		ids = append(ids, Identifier{Kind: "table", Value: risk.Table}, Identifier{Kind: "model", Value: risk.Table})
	}
	return strongIdentifiers(uniqueIdentifiers(ids))
}

func strongIdentifiers(ids []Identifier) []Identifier {
	var out []Identifier
	for _, id := range ids {
		switch id.Kind {
		case "date", "timestamp", "field":
			continue
		default:
			out = append(out, id)
		}
	}
	return uniqueIdentifiers(out)
}

func provenanceMigrationPath(risk BaselineRisk, facts []Fact) string {
	if isMigrationLikePath(risk.Path) || strings.HasPrefix(risk.Kind, "code-path:schema:") || risk.Kind == "migration_framework" {
		return risk.Path
	}
	for _, fact := range facts {
		if fact.Path == "" {
			continue
		}
		if fact.Kind == "schema_evolution" || fact.Kind == "migration_root" || fact.Kind == "migration_system" || isMigrationLikePath(fact.Path) {
			return fact.Path
		}
	}
	return ""
}

func provenanceSourcePaths(risk BaselineRisk, facts []Fact) []string {
	var paths []string
	if risk.Path != "" {
		paths = append(paths, risk.Path)
	}
	for _, fact := range facts {
		if fact.Path == "" {
			continue
		}
		switch fact.Kind {
		case "file", "source_sql_hint", "schema_evolution":
			paths = append(paths, fact.Path)
		}
	}
	return capStrings(uniqueSortedStrings(paths), 8)
}

func provenanceIncidentPaths(ids []Identifier, causes []intake.CauseCandidate, facts []Fact) []string {
	var paths []string
	for _, cause := range causes {
		causeIDs := strongIdentifiers(identifiersFromIntake(cause.Identifiers, ""))
		if cause.Path != "" && (containsAny(strings.ToLower(cause.Kind), "incident", "postmortem", "outage", "deploy", "trace") || len(sharedIdentifiers(ids, causeIDs)) > 0) {
			paths = append(paths, cause.Path)
		}
	}
	for _, fact := range facts {
		if fact.Path == "" || len(sharedIdentifiers(ids, strongIdentifiers(fact.Identifiers))) == 0 {
			continue
		}
		switch fact.Kind {
		case "operational_doc", "evidence_export", "field_evidence":
			if containsAny(strings.ToLower(fact.Path+" "+fact.Rationale), "incident", "postmortem", "outage", "deploy", "trace", "error") {
				paths = append(paths, fact.Path)
			}
		}
	}
	return capStrings(uniqueSortedStrings(paths), 8)
}

func provenanceRepairPaths(ids []Identifier, repairs []intake.RepairCandidate, facts []Fact) []string {
	var paths []string
	for _, repair := range repairs {
		repairIDs := strongIdentifiers(identifiersFromIntake(repair.Identifiers, repair.Table))
		if repair.Path != "" && len(sharedIdentifiers(ids, repairIDs)) > 0 {
			paths = append(paths, repair.Path)
		}
	}
	for _, fact := range facts {
		if fact.Path == "" || len(sharedIdentifiers(ids, strongIdentifiers(fact.Identifiers))) == 0 {
			continue
		}
		if fact.Kind == "file" && containsAny(strings.ToLower(fact.Path+" "+fact.Rationale), "repair", "rollback", "revert", "backfill", "reconcile", "fix") {
			paths = append(paths, fact.Path)
		}
	}
	return capStrings(uniqueSortedStrings(paths), 8)
}

func provenanceLinks(risk BaselineRisk, ids []Identifier, facts []Fact) []EvidenceLink {
	var links []EvidenceLink
	for _, fact := range capFacts(facts, 12) {
		shared := sharedIdentifiers(ids, fact.Identifiers)
		if len(shared) == 0 {
			continue
		}
		links = append(links, EvidenceLink{RiskID: risk.ID, FactID: fact.ID, FactKind: fact.Kind, Path: fact.Path, Identifiers: shared, Confidence: "identifier"})
	}
	return uniqueLinks(links)
}

func provenanceStages(slice ProvenanceSlice) []string {
	var stages []string
	if slice.MigrationPath != "" {
		stages = append(stages, "migration")
	}
	if slice.Table != "" {
		stages = append(stages, "table")
	}
	if len(slice.SourcePaths) > 0 {
		stages = append(stages, "source")
	}
	if len(slice.TestCommands) > 0 {
		stages = append(stages, "test")
	}
	if len(slice.NativeCommands) > 0 {
		stages = append(stages, "native-check")
	}
	if len(slice.IncidentPaths) > 0 {
		stages = append(stages, "incident")
	}
	if len(slice.RepairPaths) > 0 {
		stages = append(stages, "repair")
	}
	return stages
}

func uniqueProvenanceSlices(in []ProvenanceSlice) []ProvenanceSlice {
	seen := map[string]bool{}
	var out []ProvenanceSlice
	for _, slice := range in {
		if seen[slice.ID] {
			continue
		}
		seen[slice.ID] = true
		out = append(out, slice)
	}
	return out
}

func isMigrationLikePath(path string) bool {
	lower := strings.ToLower(filepath.ToSlash(path))
	return containsAny(lower, "migration", "migrations", "db/migrate", ".sql", ".psql", ".ddl")
}

func uniqueSortedStrings(in []string) []string {
	sort.Strings(in)
	var out []string
	var last string
	for _, value := range in {
		if value == "" || value == last {
			continue
		}
		out = append(out, value)
		last = value
	}
	return out
}

func capStrings(in []string, n int) []string {
	if len(in) <= n {
		return in
	}
	return in[:n]
}

func buildDatalogQueries(risks []BaselineRisk, slices []ProvenanceSlice) []DatalogQuery {
	return []DatalogQuery{
		{
			Name: "minimal_cause_sets",
			Rule: "minimal_cause_set(Risk, Table, Incident) :- risk(Risk, Table), provenance(Risk, Table, Incident), not smaller_identifier_set(Risk, Incident).",
			Rows: datalogMinimalCauseRows(slices),
		},
		{
			Name: "shared_ancestors",
			Rule: "shared_ancestor(Table, RiskA, RiskB) :- risk(RiskA, Table), risk(RiskB, Table), RiskA != RiskB.",
			Rows: datalogSharedAncestorRows(slices),
		},
		{
			Name: "repair_lineage",
			Rule: "repair_lineage(Risk, Table, Repair) :- risk(Risk, Table), provenance_repair(Risk, Repair), shares_identifier(Table, Repair).",
			Rows: datalogRepairLineageRows(slices),
		},
		{
			Name: "affected_outputs",
			Rule: "affected_output(Risk, Output) :- risk(Risk, Table), output(Table); affected_output(Risk, SourcePath) :- provenance_source(Risk, SourcePath).",
			Rows: datalogAffectedOutputRows(risks, slices),
		},
	}
}

func datalogMinimalCauseRows(slices []ProvenanceSlice) []DatalogRow {
	var rows []DatalogRow
	for _, slice := range slices {
		for _, incident := range slice.IncidentPaths {
			rows = append(rows, DatalogRow{
				Bindings: map[string]string{
					"risk":      slice.RiskID,
					"table":     slice.Table,
					"incident":  incident,
					"migration": slice.MigrationPath,
				},
				Evidence:   provenanceEvidence(slice),
				Confidence: "identifier-shared",
			})
		}
	}
	return capDatalogRows(rows, 100)
}

func datalogSharedAncestorRows(slices []ProvenanceSlice) []DatalogRow {
	byTable := map[string][]ProvenanceSlice{}
	for _, slice := range slices {
		if slice.Table == "" {
			continue
		}
		byTable[slice.Table] = append(byTable[slice.Table], slice)
	}
	var rows []DatalogRow
	for table, group := range byTable {
		sort.Slice(group, func(i, j int) bool { return group[i].RiskID < group[j].RiskID })
		riskIDs := make([]string, 0, len(group))
		paths := make([]string, 0, len(group))
		for _, slice := range group {
			riskIDs = append(riskIDs, slice.RiskID)
			paths = append(paths, slice.MigrationPath)
			paths = append(paths, slice.SourcePaths...)
		}
		rows = append(rows, DatalogRow{
			Bindings: map[string]string{
				"ancestor":      "table:" + table,
				"table":         table,
				"risk_count":    fmt.Sprintf("%d", len(uniqueSortedStrings(riskIDs))),
				"sample_risks":  strings.Join(capStrings(uniqueSortedStrings(riskIDs), 5), ","),
				"sample_paths":  strings.Join(capStrings(uniqueSortedStrings(paths), 5), ","),
				"shared_by":     "risk",
				"minimal_basis": "table",
			},
			Confidence: "identifier-shared",
		})
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Bindings["risk_count"] != rows[j].Bindings["risk_count"] {
			return rows[i].Bindings["risk_count"] > rows[j].Bindings["risk_count"]
		}
		return rows[i].Bindings["table"] < rows[j].Bindings["table"]
	})
	return capDatalogRows(rows, 100)
}

func datalogRepairLineageRows(slices []ProvenanceSlice) []DatalogRow {
	var rows []DatalogRow
	for _, slice := range slices {
		for _, repair := range slice.RepairPaths {
			rows = append(rows, DatalogRow{
				Bindings: map[string]string{
					"risk":      slice.RiskID,
					"table":     slice.Table,
					"migration": slice.MigrationPath,
					"repair":    repair,
				},
				Evidence:   provenanceEvidence(slice),
				Confidence: "identifier-shared",
			})
		}
	}
	return capDatalogRows(rows, 100)
}

func datalogAffectedOutputRows(risks []BaselineRisk, slices []ProvenanceSlice) []DatalogRow {
	riskByID := map[string]BaselineRisk{}
	for _, risk := range risks {
		riskByID[risk.ID] = risk
	}
	var rows []DatalogRow
	for _, slice := range slices {
		risk := riskByID[slice.RiskID]
		output := slice.Table
		if output == "" {
			output = risk.Table
		}
		rows = append(rows, DatalogRow{
			Bindings: map[string]string{
				"risk":             slice.RiskID,
				"output":           output,
				"severity":         risk.Severity,
				"source_path_cnt":  fmt.Sprintf("%d", len(slice.SourcePaths)),
				"native_check_cnt": fmt.Sprintf("%d", len(slice.NativeCommands)+len(slice.TestCommands)),
			},
			Evidence:   provenanceEvidence(slice),
			Confidence: "identifier-shared",
		})
	}
	return capDatalogRows(rows, 100)
}

func provenanceEvidence(slice ProvenanceSlice) []string {
	var evidence []string
	if slice.MigrationPath != "" {
		evidence = append(evidence, "migration:"+slice.MigrationPath)
	}
	for _, path := range slice.SourcePaths {
		evidence = append(evidence, "source:"+path)
	}
	for _, path := range slice.IncidentPaths {
		evidence = append(evidence, "incident:"+path)
	}
	for _, path := range slice.RepairPaths {
		evidence = append(evidence, "repair:"+path)
	}
	return capStrings(uniqueSortedStrings(evidence), 12)
}

func capDatalogRows(rows []DatalogRow, limit int) []DatalogRow {
	sort.Slice(rows, func(i, j int) bool {
		left := canonical.Hash(rows[i])
		right := canonical.Hash(rows[j])
		return left < right
	})
	if len(rows) <= limit {
		return rows
	}
	return rows[:limit]
}

func countDatalogRows(queries []DatalogQuery) int {
	var count int
	for _, query := range queries {
		count += len(query.Rows)
	}
	return count
}

func buildAbstractEffectSummaries(risks []BaselineRisk, slices []ProvenanceSlice) []effects.AbstractSummary {
	sliceByRisk := map[string]ProvenanceSlice{}
	for _, slice := range slices {
		sliceByRisk[slice.RiskID] = slice
	}
	var summaries []effects.AbstractSummary
	for _, risk := range risks {
		table := firstNonEmpty(risk.Table, firstIdentifierValue(risk.Identifiers, "table"), firstIdentifierValue(risk.Identifiers, "model"))
		if table == "" {
			continue
		}
		slice := sliceByRisk[risk.ID]
		observation := effects.OperationObservation{
			OperationID:         risk.ID,
			Table:               table,
			Effect:              abstractEffectForRisk(risk),
			MatchedRows:         -1,
			ChangedColumns:      changedColumnsForRisk(risk),
			DownstreamEntities:  len(slice.SourcePaths) + len(slice.IncidentPaths) + len(slice.RepairPaths),
			HasSnapshotRollback: riskHasFactor(risk, "weak-rollback-signal") == false && len(slice.RepairPaths) > 0,
			Reasons:             abstractReasonsForRisk(risk),
		}
		summary := effects.Summarize(firstNonEmpty(slice.MigrationPath, risk.Path), "", []effects.OperationObservation{observation})
		summaries = append(summaries, summary)
	}
	sort.Slice(summaries, func(i, j int) bool {
		if summaries[i].Join != summaries[j].Join {
			return effectRank(summaries[i].Join) > effectRank(summaries[j].Join)
		}
		return summaries[i].Hash < summaries[j].Hash
	})
	if len(summaries) > 100 {
		summaries = summaries[:100]
	}
	return summaries
}

func abstractEffectForRisk(risk BaselineRisk) effects.Effect {
	kind := strings.ToLower(risk.Kind)
	switch {
	case strings.Contains(kind, "delete"), strings.Contains(kind, "drop"), strings.Contains(kind, "truncate"):
		return effects.EffectDestructive
	case riskHasFactor(risk, "destructive-code-path"), riskHasFactor(risk, "destructive-schema-change"), riskHasFactor(risk, "broad-write"):
		return effects.EffectDestructive
	case strings.Contains(kind, "insert"), strings.Contains(kind, "create"):
		return effects.EffectIdempotentUpdate
	case strings.Contains(kind, "update"), strings.Contains(kind, "alter"), strings.Contains(kind, "add_column"), strings.Contains(kind, "add_field"):
		if riskHasFactor(risk, "weak-rollback-signal") {
			return effects.EffectIdempotentUpdate
		}
		return effects.EffectReversibleUpdate
	default:
		return effects.EffectUnknown
	}
}

func changedColumnsForRisk(risk BaselineRisk) []string {
	var columns []string
	for _, id := range risk.Identifiers {
		if id.Kind == "column" {
			columns = append(columns, id.Value)
		}
	}
	return uniqueSortedStrings(columns)
}

func abstractReasonsForRisk(risk BaselineRisk) []string {
	var reasons []string
	for _, factor := range risk.Factors {
		reasons = append(reasons, factor.Name+": "+factor.Reason)
	}
	return capStrings(uniqueSortedStrings(reasons), 8)
}

func riskHasFactor(risk BaselineRisk, name string) bool {
	for _, factor := range risk.Factors {
		if factor.Name == name {
			return true
		}
	}
	return false
}

func effectRank(effect effects.Effect) int {
	for _, item := range effects.Lattice() {
		if item.Effect == effect {
			return item.Rank
		}
	}
	return 7
}

func countAbstractOperations(summaries []effects.AbstractSummary) int {
	var count int
	for _, summary := range summaries {
		count += len(summary.Operations)
	}
	return count
}

func countAbstractProofHoles(summaries []effects.AbstractSummary) int {
	var count int
	for _, summary := range summaries {
		count += len(summary.Concretization.UnsupportedFacts)
	}
	return count
}

func buildSymbolicChecks(risks []BaselineRisk, summaries []effects.AbstractSummary, slices []ProvenanceSlice) []SymbolicCheck {
	riskByID := map[string]BaselineRisk{}
	for _, risk := range risks {
		riskByID[risk.ID] = risk
	}
	sliceByRisk := map[string]ProvenanceSlice{}
	for _, slice := range slices {
		sliceByRisk[slice.RiskID] = slice
	}
	var checks []SymbolicCheck
	for _, summary := range summaries {
		for _, op := range summary.Operations {
			risk := riskByID[op.OperationID]
			slice := sliceByRisk[op.OperationID]
			checks = append(checks,
				symbolicIdempotencyCheck(risk, op, slice),
				symbolicReversibilityCheck(risk, op, slice),
				symbolicFrameCheck(risk, op, slice),
				symbolicScopeCheck(risk, op, slice),
			)
		}
	}
	sort.Slice(checks, func(i, j int) bool {
		if checks[i].Status != checks[j].Status {
			return symbolicStatusRank(checks[i].Status) > symbolicStatusRank(checks[j].Status)
		}
		if checks[i].RiskID != checks[j].RiskID {
			return checks[i].RiskID < checks[j].RiskID
		}
		return checks[i].Property < checks[j].Property
	})
	if len(checks) > 400 {
		checks = checks[:400]
	}
	return checks
}

func symbolicIdempotencyCheck(risk BaselineRisk, op effects.AbstractOperation, slice ProvenanceSlice) SymbolicCheck {
	status := "pass"
	reason := "abstract transfer is stable under repeated application"
	if !op.Idempotent {
		status = "warn"
		reason = "idempotency is not proven by the static transfer function"
	}
	if op.Destructive {
		status = "fail"
		reason = "destructive or unknown effect cannot be considered idempotent without stronger evidence"
	}
	return symbolicCheck(risk, op, slice, "idempotency", status, "T(T(state)) == T(state)", reason)
}

func symbolicReversibilityCheck(risk BaselineRisk, op effects.AbstractOperation, slice ProvenanceSlice) SymbolicCheck {
	status := "pass"
	reason := "abstract transfer has a rollback or inverse witness"
	if !op.Reversible {
		status = "warn"
		reason = "reversibility is not proven by the static transfer function"
	}
	if op.Destructive {
		status = "fail"
		reason = "destructive or unknown effect lacks a safe inverse without snapshot or repair evidence"
	}
	return symbolicCheck(risk, op, slice, "reversibility", status, "exists inverse T^-1 such that T^-1(T(state)) == state", reason)
}

func symbolicFrameCheck(risk BaselineRisk, op effects.AbstractOperation, slice ProvenanceSlice) SymbolicCheck {
	status := "pass"
	reason := "changed columns define an explicit frame for untouched fields"
	expression := "forall c notin changed_columns: post[c] == pre[c]"
	if op.Table == "" {
		status = "fail"
		reason = "frame condition cannot be scoped without a table"
	} else if len(op.ChangedColumns) == 0 {
		status = "warn"
		reason = "changed-column frame is unknown from static evidence"
	}
	if op.Destructive {
		status = "fail"
		reason = "destructive or unknown effect invalidates a row-preservation frame"
		expression = "rows_outside_scope are preserved"
	}
	return symbolicCheck(risk, op, slice, "frame_condition", status, expression, reason)
}

func symbolicScopeCheck(risk BaselineRisk, op effects.AbstractOperation, slice ProvenanceSlice) SymbolicCheck {
	status := "pass"
	reason := "risk has table scope and no broad-write factor"
	expression := "affected_rows subset scope(table, predicates, provenance)"
	if op.Table == "" {
		status = "fail"
		reason = "scope preservation cannot be checked without a table"
	} else if riskHasFactor(risk, "broad-write") || riskHasFactor(risk, "write-breadth-unknown") || riskHasFactor(risk, "source-window-unavailable") {
		status = "warn"
		reason = "scope is table-known but row breadth is not statically bounded"
	}
	if op.Destructive {
		status = "fail"
		reason = "destructive or unknown effect must be reviewed as potentially escaping the intended scope"
	}
	return symbolicCheck(risk, op, slice, "scope_preservation", status, expression, reason)
}

func symbolicCheck(risk BaselineRisk, op effects.AbstractOperation, slice ProvenanceSlice, property, status, expression, reason string) SymbolicCheck {
	evidence := provenanceEvidence(slice)
	for _, hole := range op.ProofHoles {
		evidence = append(evidence, "proof-hole:"+hole)
	}
	evidence = capStrings(uniqueSortedStrings(evidence), 12)
	return SymbolicCheck{
		ID:         "sym:" + canonical.Hash(op.OperationID + "\x00" + property)[:16],
		RiskID:     op.OperationID,
		Property:   property,
		Status:     status,
		Table:      firstNonEmpty(op.Table, risk.Table),
		Expression: expression,
		Evidence:   evidence,
		Reason:     reason,
	}
}

func symbolicStatusRank(status string) int {
	switch status {
	case "fail":
		return 3
	case "warn":
		return 2
	case "pass":
		return 1
	default:
		return 0
	}
}

func countSymbolicStatus(checks []SymbolicCheck, status string) int {
	var count int
	for _, check := range checks {
		if check.Status == status {
			count++
		}
	}
	return count
}

func buildTemporalWindows(risks []BaselineRisk, slices []ProvenanceSlice, report intake.Report) []TemporalWindow {
	signalsByPath := map[string][]TimeSignal{}
	for _, signal := range report.TimeSignals {
		parsed, err := time.Parse("2006-01-02", signal.Timestamp)
		if err != nil {
			continue
		}
		ts := TimeSignal{
			Timestamp:   parsed.Format("2006-01-02"),
			Source:      signal.Source,
			Path:        signal.Path,
			Stage:       temporalStageForPath(signal.Path),
			Identifiers: identifiersFromIntake(signal.Identifiers, ""),
		}
		signalsByPath[signal.Path] = append(signalsByPath[signal.Path], ts)
	}
	for path := range signalsByPath {
		sort.Slice(signalsByPath[path], func(i, j int) bool {
			if signalsByPath[path][i].Timestamp != signalsByPath[path][j].Timestamp {
				return signalsByPath[path][i].Timestamp < signalsByPath[path][j].Timestamp
			}
			return signalsByPath[path][i].Source < signalsByPath[path][j].Source
		})
	}
	riskByID := map[string]BaselineRisk{}
	for _, risk := range risks {
		riskByID[risk.ID] = risk
	}
	var windows []TemporalWindow
	for _, slice := range slices {
		signals := temporalSignalsForSlice(slice, signalsByPath)
		if len(signals) == 0 {
			signals = temporalSignalsForRisk(riskByID[slice.RiskID], signalsByPath)
		}
		if len(signals) == 0 {
			if window, ok := logicalTemporalWindow(slice); ok {
				windows = append(windows, window)
			}
			continue
		}
		window := temporalWindowForSignals(slice, signals)
		windows = append(windows, window)
	}
	windows = uniqueTemporalWindows(windows)
	sort.Slice(windows, func(i, j int) bool {
		if windows[i].Start != windows[j].Start {
			return windows[i].Start < windows[j].Start
		}
		if windows[i].End != windows[j].End {
			return windows[i].End < windows[j].End
		}
		return windows[i].ID < windows[j].ID
	})
	if len(windows) > 100 {
		windows = windows[:100]
	}
	return windows
}

func temporalSignalsForSlice(slice ProvenanceSlice, byPath map[string][]TimeSignal) []TimeSignal {
	var paths []string
	paths = append(paths, slice.MigrationPath)
	paths = append(paths, slice.SourcePaths...)
	paths = append(paths, slice.IncidentPaths...)
	paths = append(paths, slice.RepairPaths...)
	var signals []TimeSignal
	for _, path := range uniqueSortedStrings(paths) {
		signals = append(signals, byPath[path]...)
	}
	return capTimeSignals(uniqueTimeSignals(signals), 12)
}

func temporalSignalsForRisk(risk BaselineRisk, byPath map[string][]TimeSignal) []TimeSignal {
	return capTimeSignals(uniqueTimeSignals(byPath[risk.Path]), 12)
}

func temporalWindowForSignals(slice ProvenanceSlice, signals []TimeSignal) TemporalWindow {
	minTime := signals[0].Timestamp
	maxTime := signals[0].Timestamp
	for _, signal := range signals[1:] {
		if signal.Timestamp < minTime {
			minTime = signal.Timestamp
		}
		if signal.Timestamp > maxTime {
			maxTime = signal.Timestamp
		}
	}
	start := addDays(minTime, -3)
	end := addDays(maxTime, 3)
	return TemporalWindow{
		ID:            "timewin:" + canonical.Hash(slice.RiskID + "\x00" + start + "\x00" + end)[:16],
		RiskID:        slice.RiskID,
		Table:         slice.Table,
		Start:         start,
		End:           end,
		Anchor:        minTime + ".." + maxTime,
		Signals:       signals,
		RelatedPaths:  temporalRelatedPaths(slice),
		StagesPresent: slice.StagesPresent,
		Confidence:    temporalConfidence(signals),
		Rationale:     "window is derived from dates in migration, source, incident, release, or repair evidence around the same provenance slice",
	}
}

func temporalRelatedPaths(slice ProvenanceSlice) []string {
	var paths []string
	paths = append(paths, slice.MigrationPath)
	paths = append(paths, slice.SourcePaths...)
	paths = append(paths, slice.IncidentPaths...)
	paths = append(paths, slice.RepairPaths...)
	return capStrings(uniqueSortedStrings(paths), 12)
}

func logicalTemporalWindow(slice ProvenanceSlice) (TemporalWindow, bool) {
	if slice.MigrationPath == "" {
		return TemporalWindow{}, false
	}
	anchor := migrationOrderAnchor(slice.MigrationPath)
	if anchor == "" {
		return TemporalWindow{}, false
	}
	return TemporalWindow{
		ID:            "timewin:" + canonical.Hash(slice.RiskID + "\x00" + anchor)[:16],
		RiskID:        slice.RiskID,
		Table:         slice.Table,
		Start:         anchor,
		End:           anchor,
		Anchor:        anchor,
		RelatedPaths:  temporalRelatedPaths(slice),
		StagesPresent: slice.StagesPresent,
		Confidence:    "migration-order-temporal",
		Rationale:     "window is derived from migration filename ordering because no calendar date was present in this repo slice",
	}, true
}

func migrationOrderAnchor(path string) string {
	base := filepath.Base(path)
	var digits strings.Builder
	for _, r := range base {
		if r < '0' || r > '9' {
			break
		}
		digits.WriteRune(r)
	}
	if digits.Len() == 0 {
		return ""
	}
	return "migration-order:" + digits.String()
}

func temporalStageForPath(path string) string {
	lower := strings.ToLower(path)
	switch {
	case containsAny(lower, "incident", "postmortem", "outage", "sev"):
		return "incident"
	case containsAny(lower, "release", "changelog", "deploy"):
		return "release"
	case containsAny(lower, "rollback", "repair", "revert", "backfill", "reconcile", "fix"):
		return "repair"
	case isMigrationLikePath(lower):
		return "migration"
	default:
		return "source"
	}
}

func temporalConfidence(signals []TimeSignal) string {
	stages := map[string]bool{}
	for _, signal := range signals {
		stages[signal.Stage] = true
	}
	if len(stages) >= 2 {
		return "multi-stage-temporal"
	}
	return "single-stage-temporal"
}

func addDays(date string, days int) string {
	parsed, err := time.Parse("2006-01-02", date)
	if err != nil {
		return date
	}
	return parsed.AddDate(0, 0, days).Format("2006-01-02")
}

func uniqueTimeSignals(in []TimeSignal) []TimeSignal {
	seen := map[string]bool{}
	var out []TimeSignal
	for _, signal := range in {
		key := signal.Timestamp + "\x00" + signal.Path + "\x00" + signal.Source + "\x00" + signal.Stage
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, signal)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Timestamp != out[j].Timestamp {
			return out[i].Timestamp < out[j].Timestamp
		}
		if out[i].Path != out[j].Path {
			return out[i].Path < out[j].Path
		}
		return out[i].Source < out[j].Source
	})
	return out
}

func capTimeSignals(in []TimeSignal, n int) []TimeSignal {
	if len(in) <= n {
		return in
	}
	return in[:n]
}

func uniqueTemporalWindows(in []TemporalWindow) []TemporalWindow {
	seen := map[string]bool{}
	var out []TemporalWindow
	for _, window := range in {
		if seen[window.ID] {
			continue
		}
		seen[window.ID] = true
		out = append(out, window)
	}
	return out
}

func countTemporalSignals(windows []TemporalWindow) int {
	var count int
	for _, window := range windows {
		count += len(window.Signals)
	}
	return count
}

func buildRecurrences(risks []BaselineRisk, summaries []effects.AbstractSummary, slices []ProvenanceSlice) []RecurrencePattern {
	effectByRisk := map[string]string{}
	for _, summary := range summaries {
		for _, op := range summary.Operations {
			effectByRisk[op.OperationID] = string(op.Effect)
		}
	}
	sliceByRisk := map[string]ProvenanceSlice{}
	for _, slice := range slices {
		sliceByRisk[slice.RiskID] = slice
	}
	type bucket struct {
		table       string
		kind        string
		effect      string
		riskIDs     []string
		paths       []string
		repairPaths []string
	}
	buckets := map[string]*bucket{}
	for _, risk := range risks {
		table := firstNonEmpty(risk.Table, firstIdentifierValue(risk.Identifiers, "table"), firstIdentifierValue(risk.Identifiers, "model"))
		if table == "" {
			continue
		}
		kind := recurrenceKind(risk)
		effect := effectByRisk[risk.ID]
		key := table + "\x00" + kind + "\x00" + effect
		b := buckets[key]
		if b == nil {
			b = &bucket{table: table, kind: kind, effect: effect}
			buckets[key] = b
		}
		b.riskIDs = append(b.riskIDs, risk.ID)
		b.paths = append(b.paths, risk.Path)
		b.repairPaths = append(b.repairPaths, sliceByRisk[risk.ID].RepairPaths...)
	}
	var patterns []RecurrencePattern
	for key, b := range buckets {
		riskIDs := uniqueSortedStrings(b.riskIDs)
		if len(riskIDs) < 2 {
			continue
		}
		paths := capStrings(uniqueSortedStrings(b.paths), 12)
		repairs := capStrings(uniqueSortedStrings(b.repairPaths), 12)
		patterns = append(patterns, RecurrencePattern{
			ID:          "recurrence:" + canonical.Hash(key)[:16],
			Key:         strings.ReplaceAll(key, "\x00", "|"),
			Table:       b.table,
			Kind:        b.kind,
			Effect:      b.effect,
			RiskIDs:     capStrings(riskIDs, 20),
			Paths:       paths,
			RepairPaths: repairs,
			Count:       len(riskIDs),
			Confidence:  recurrenceConfidence(len(riskIDs), len(repairs)),
			Rationale:   "multiple ranked risks in this repo share table, operation family, and abstract effect; inspect as a recurring data-change pattern",
			NextCommand: fmt.Sprintf("patchline repo baseline %s --json", shellPath(firstNonEmpty(firstString(paths), "."))),
		})
	}
	sort.Slice(patterns, func(i, j int) bool {
		if patterns[i].Count != patterns[j].Count {
			return patterns[i].Count > patterns[j].Count
		}
		return patterns[i].ID < patterns[j].ID
	})
	if len(patterns) > 100 {
		patterns = patterns[:100]
	}
	return patterns
}

func recurrenceKind(risk BaselineRisk) string {
	kind := strings.ToLower(risk.Kind)
	switch {
	case strings.Contains(kind, "delete"), strings.Contains(kind, "drop"), strings.Contains(kind, "truncate"):
		return "destructive"
	case strings.Contains(kind, "update"):
		if riskHasFactor(risk, "broad-write") || riskHasFactor(risk, "write-breadth-unknown") {
			return "broad-update"
		}
		return "update"
	case strings.Contains(kind, "insert"):
		return "insert"
	case strings.Contains(kind, "schema"):
		return "schema-change"
	default:
		return kind
	}
}

func recurrenceConfidence(count, repairs int) string {
	switch {
	case count >= 5 && repairs > 0:
		return "recurring-with-repair-evidence"
	case count >= 5:
		return "recurring"
	default:
		return "repeated"
	}
}

func firstString(values []string) string {
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

func countRecurringRisks(patterns []RecurrencePattern) int {
	seen := map[string]bool{}
	for _, pattern := range patterns {
		for _, riskID := range pattern.RiskIDs {
			seen[riskID] = true
		}
	}
	return len(seen)
}

func buildTransactionBoundaries(risks []BaselineRisk, slices []ProvenanceSlice, intakeReport intake.Report) []TransactionBoundary {
	root := firstNonEmpty(intakeReport.Source.ScannedRoot, intakeReport.Source.Input)
	sliceByRisk := map[string]ProvenanceSlice{}
	for _, slice := range slices {
		sliceByRisk[slice.RiskID] = slice
	}
	seen := map[string]bool{}
	var out []TransactionBoundary
	for _, risk := range risks {
		if !riskNeedsTransactionBoundary(risk) {
			continue
		}
		text := textForBoundary(root, risk.Path)
		window := text
		if risk.Statement > 0 {
			window = lineWindow(text, risk.Statement, 8)
		}
		status, markers := classifyTransactionBoundary(window)
		if status == "missing" && riskHasFactor(risk, "missing-transaction-boundary") == false {
			status, markers = classifyTransactionBoundary(text)
		}
		boundary := TransactionBoundary{
			ID:          "tx:" + canonical.Hash(fmt.Sprintf("risk\x00%s\x00%s\x00%d", risk.ID, risk.Path, risk.Statement))[:16],
			RiskID:      risk.ID,
			Path:        risk.Path,
			Line:        risk.Statement,
			Table:       risk.Table,
			Surface:     transactionSurface(risk.Kind, risk.Path),
			Operation:   transactionOperation(risk.Kind),
			Status:      status,
			Confidence:  transactionConfidence(status, text),
			Markers:     markers,
			Evidence:    transactionEvidence(risk, sliceByRisk[risk.ID], markers),
			Identifiers: risk.Identifiers,
			Rationale:   transactionRationale(status, transactionSurface(risk.Kind, risk.Path)),
		}
		if addTransactionBoundary(&out, seen, boundary) {
			continue
		}
	}
	for _, slice := range slices {
		for _, path := range slice.RepairPaths {
			text := textForBoundary(root, path)
			status, markers := classifyTransactionBoundary(text)
			surface := "repair"
			if isGeneratedPath(path) {
				surface = "generated_repair"
			}
			addTransactionBoundary(&out, seen, TransactionBoundary{
				ID:          "tx:" + canonical.Hash(fmt.Sprintf("repair\x00%s\x00%s", slice.RiskID, path))[:16],
				RiskID:      slice.RiskID,
				Path:        path,
				Table:       slice.Table,
				Surface:     surface,
				Operation:   "repair",
				Status:      status,
				Confidence:  transactionConfidence(status, text),
				Markers:     markers,
				Evidence:    []string{"repair path linked by provenance slice"},
				Identifiers: slice.Identifiers,
				Rationale:   transactionRationale(status, surface),
			})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Status != out[j].Status {
			return transactionStatusRank(out[i].Status) > transactionStatusRank(out[j].Status)
		}
		if out[i].Path != out[j].Path {
			return out[i].Path < out[j].Path
		}
		return out[i].ID < out[j].ID
	})
	return out
}

func addTransactionBoundary(out *[]TransactionBoundary, seen map[string]bool, boundary TransactionBoundary) bool {
	key := boundary.RiskID + "\x00" + boundary.Path + "\x00" + boundary.Surface + "\x00" + boundary.Operation
	if seen[key] {
		return false
	}
	seen[key] = true
	*out = append(*out, boundary)
	return true
}

func riskNeedsTransactionBoundary(risk BaselineRisk) bool {
	kind := strings.ToLower(risk.Kind)
	if containsAny(kind, "update", "delete", "insert", "drop", "truncate", "alter", "create", "merge", "schema", "code-path") {
		return true
	}
	for _, factor := range risk.Factors {
		if containsAny(strings.ToLower(factor.Name), "persistent", "destructive", "write", "schema") {
			return true
		}
	}
	return false
}

func textForBoundary(root, path string) string {
	if path == "" {
		return ""
	}
	candidate := path
	if !filepath.IsAbs(candidate) && root != "" {
		candidate = filepath.Join(root, filepath.FromSlash(path))
	}
	text, err := readTextPrefix(candidate, factContentLimit)
	if err == nil {
		return text
	}
	return ""
}

func lineWindow(text string, line, radius int) string {
	if text == "" || line <= 0 {
		return text
	}
	lines := strings.Split(text, "\n")
	if line > len(lines) {
		return text
	}
	start := line - radius - 1
	if start < 0 {
		start = 0
	}
	end := line + radius
	if end > len(lines) {
		end = len(lines)
	}
	return strings.Join(lines[start:end], "\n")
}

func classifyTransactionBoundary(text string) (string, []string) {
	lower := strings.ToLower(text)
	var markers []string
	markerChecks := []struct {
		Name string
		Need []string
	}{
		{"transaction-block", []string{"transaction"}},
		{"atomic-block", []string{"atomic"}},
		{"transactional-annotation", []string{"@transactional"}},
		{"sql-begin", []string{"begin"}},
		{"sql-commit", []string{"commit"}},
		{"rollback", []string{"rollback"}},
		{"savepoint", []string{"savepoint"}},
		{"gorm-transaction", []string{".transaction("}},
		{"django-atomic", []string{"transaction.atomic"}},
		{"rails-transaction", []string{"activerecord::base.transaction", ".transaction do"}},
		{"sqlalchemy-begin", []string{"session.begin"}},
		{"typeorm-transaction", []string{"manager.transaction", "queryrunner.starttransaction"}},
		{"prisma-transaction", []string{"$transaction"}},
	}
	for _, check := range markerChecks {
		for _, token := range check.Need {
			if strings.Contains(lower, token) {
				markers = append(markers, check.Name)
				break
			}
		}
	}
	markers = uniqueStrings(markers)
	hasExplicit := containsAny(lower, "transaction", "atomic", "@transactional", ".transaction(", "$transaction", "session.begin", "starttransaction")
	hasBegin := containsAny(lower, "begin", "start transaction")
	hasCommit := strings.Contains(lower, "commit")
	hasRollback := strings.Contains(lower, "rollback")
	if hasExplicit || (hasBegin && (hasCommit || hasRollback)) {
		return "explicit", markers
	}
	if hasBegin || hasCommit || hasRollback || strings.Contains(lower, "savepoint") || strings.Contains(lower, " tx.") || strings.Contains(lower, "tx.") {
		return "partial", markers
	}
	return "missing", markers
}

func transactionSurface(kind, path string) string {
	lower := strings.ToLower(kind + " " + path)
	switch {
	case strings.Contains(lower, "code-path"):
		return "app_code"
	case strings.Contains(lower, "schema"):
		return "migration_dsl"
	case strings.HasSuffix(lower, ".sql") || strings.Contains(lower, "sql"):
		return "raw_sql"
	case isGeneratedPath(path):
		return "generated_repair"
	case containsAny(lower, "job", "worker", "task", "cron"):
		return "job"
	default:
		return "project_file"
	}
}

func transactionOperation(kind string) string {
	parts := strings.Split(kind, ":")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return kind
}

func transactionConfidence(status, text string) string {
	if text == "" {
		return "low"
	}
	switch status {
	case "explicit":
		return "high"
	case "partial":
		return "medium"
	default:
		return "derived"
	}
}

func transactionEvidence(risk BaselineRisk, slice ProvenanceSlice, markers []string) []string {
	var evidence []string
	if len(markers) > 0 {
		evidence = append(evidence, "transaction markers: "+strings.Join(markers, ", "))
	}
	for _, factor := range risk.Factors {
		if factor.Name == "missing-transaction-boundary" {
			evidence = append(evidence, factor.Reason)
		}
	}
	if len(slice.StagesPresent) > 0 {
		evidence = append(evidence, "provenance stages: "+strings.Join(slice.StagesPresent, ", "))
	}
	return uniqueStrings(evidence)
}

func transactionRationale(status, surface string) string {
	switch status {
	case "explicit":
		return surface + " has an explicit transaction boundary marker near the write"
	case "partial":
		return surface + " has partial transaction evidence but no complete begin/commit or framework transaction marker"
	default:
		return surface + " write has no obvious transaction boundary in the scanned context"
	}
}

func isGeneratedPath(path string) bool {
	lower := strings.ToLower(path)
	return containsAny(lower, "generated", "proposal", "patchline", "repair")
}

func transactionStatusRank(status string) int {
	switch status {
	case "missing":
		return 3
	case "partial":
		return 2
	case "explicit":
		return 1
	default:
		return 0
	}
}

func countTransactionStatus(boundaries []TransactionBoundary, status string) int {
	var count int
	for _, boundary := range boundaries {
		if boundary.Status == status {
			count++
		}
	}
	return count
}

func buildIdempotencyClasses(risks []BaselineRisk, slices []ProvenanceSlice, symbolic []SymbolicCheck, facts []Fact, intakeReport intake.Report) []IdempotencyClass {
	root := firstNonEmpty(intakeReport.Source.ScannedRoot, intakeReport.Source.Input)
	sliceByRisk := map[string]ProvenanceSlice{}
	for _, slice := range slices {
		sliceByRisk[slice.RiskID] = slice
	}
	symbolicByRisk := map[string][]SymbolicCheck{}
	for _, check := range symbolic {
		if check.Property == "idempotency" {
			symbolicByRisk[check.RiskID] = append(symbolicByRisk[check.RiskID], check)
		}
	}
	seen := map[string]bool{}
	var out []IdempotencyClass
	for _, fact := range facts {
		if !isRunbookOrSupportFact(fact) {
			continue
		}
		text := textForBoundary(root, fact.Path)
		status, markers := classifyIdempotency(text, BaselineRisk{Kind: fact.Kind, Path: fact.Path}, nil)
		addIdempotencyClass(&out, seen, IdempotencyClass{
			ID:          "idem:" + canonical.Hash(fmt.Sprintf("fact\x00%s\x00%s", fact.ID, fact.Path))[:16],
			Path:        fact.Path,
			Table:       firstIdentifierValue(fact.Identifiers, "table"),
			Surface:     "runbook_command",
			Operation:   "support",
			Status:      status,
			Confidence:  idempotencyConfidence(status, text),
			Markers:     markers,
			Evidence:    []string{"project runbook or support file discovered by inventory"},
			Identifiers: fact.Identifiers,
			Rationale:   idempotencyRationale(status, "runbook_command"),
		})
	}
	for _, risk := range risks {
		if !riskNeedsIdempotencyClassification(risk) {
			continue
		}
		text := textForBoundary(root, risk.Path)
		window := text
		if risk.Statement > 0 {
			window = lineWindow(text, risk.Statement, 8)
		}
		status, markers := classifyIdempotency(window, risk, symbolicByRisk[risk.ID])
		if status == "unknown" && window != text {
			fullStatus, fullMarkers := classifyIdempotency(text, risk, symbolicByRisk[risk.ID])
			if idempotencyStatusRank(fullStatus) > idempotencyStatusRank(status) {
				status, markers = fullStatus, fullMarkers
			}
		}
		addIdempotencyClass(&out, seen, IdempotencyClass{
			ID:          "idem:" + canonical.Hash(fmt.Sprintf("risk\x00%s\x00%s\x00%d", risk.ID, risk.Path, risk.Statement))[:16],
			RiskID:      risk.ID,
			Path:        risk.Path,
			Line:        risk.Statement,
			Table:       risk.Table,
			Surface:     idempotencySurface(risk.Kind, risk.Path),
			Operation:   transactionOperation(risk.Kind),
			Status:      status,
			Confidence:  idempotencyConfidence(status, text),
			Markers:     markers,
			Evidence:    idempotencyEvidence(risk, sliceByRisk[risk.ID], symbolicByRisk[risk.ID], markers),
			Identifiers: risk.Identifiers,
			Rationale:   idempotencyRationale(status, idempotencySurface(risk.Kind, risk.Path)),
		})
	}
	for _, slice := range slices {
		for _, path := range append(append([]string{}, slice.RepairPaths...), slice.IncidentPaths...) {
			if !isIdempotencySupportPath(path) {
				continue
			}
			text := textForBoundary(root, path)
			status, markers := classifyIdempotency(text, BaselineRisk{Kind: "runbook", Table: slice.Table}, symbolicByRisk[slice.RiskID])
			surface := "runbook_command"
			if isGeneratedPath(path) {
				surface = "generated_script"
			} else if containsAny(strings.ToLower(path), "repair", "rollback", "backfill", "reconcile", "fix") {
				surface = "repair_job"
			}
			addIdempotencyClass(&out, seen, IdempotencyClass{
				ID:          "idem:" + canonical.Hash(fmt.Sprintf("support\x00%s\x00%s", slice.RiskID, path))[:16],
				RiskID:      slice.RiskID,
				Path:        path,
				Table:       slice.Table,
				Surface:     surface,
				Operation:   "support",
				Status:      status,
				Confidence:  idempotencyConfidence(status, text),
				Markers:     markers,
				Evidence:    []string{"support path linked by provenance slice"},
				Identifiers: slice.Identifiers,
				Rationale:   idempotencyRationale(status, surface),
			})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Status != out[j].Status {
			return idempotencyStatusRank(out[i].Status) > idempotencyStatusRank(out[j].Status)
		}
		if out[i].Path != out[j].Path {
			return out[i].Path < out[j].Path
		}
		return out[i].ID < out[j].ID
	})
	return out
}

func addIdempotencyClass(out *[]IdempotencyClass, seen map[string]bool, item IdempotencyClass) bool {
	key := item.RiskID + "\x00" + item.Path + "\x00" + item.Surface + "\x00" + item.Operation
	if seen[key] {
		return false
	}
	seen[key] = true
	*out = append(*out, item)
	return true
}

func isRunbookOrSupportFact(fact Fact) bool {
	lower := strings.ToLower(fact.Kind + " " + fact.Path + " " + fact.Rationale)
	return containsAny(lower, "runbook", "rollback", "repair", "backfill", "reconcile", "fix", "script")
}

func riskNeedsIdempotencyClassification(risk BaselineRisk) bool {
	if riskNeedsTransactionBoundary(risk) {
		return true
	}
	kind := strings.ToLower(risk.Kind)
	return containsAny(kind, "backfill", "repair", "runbook", "job", "script")
}

func classifyIdempotency(text string, risk BaselineRisk, symbolic []SymbolicCheck) (string, []string) {
	lower := strings.ToLower(text)
	var markers []string
	markerChecks := []struct {
		Name   string
		Tokens []string
	}{
		{"explicit-idempotency-note", []string{"idempotent", "idempotency"}},
		{"upsert", []string{"upsert", "update_or_create", "find_or_create", "insert_all", "upsert_all"}},
		{"conflict-guard", []string{"on conflict", "on duplicate key", "insert ignore", "merge into"}},
		{"uniqueness-guard", []string{"unique", "primary key", "dedupe", "deduplicate"}},
		{"existence-guard", []string{"if not exists", "where not exists", "unless exists", "find_or_initialize"}},
		{"checkpoint", []string{"checkpoint", "cursor", "last_processed", "resume", "processed_at"}},
		{"dry-run", []string{"dry_run", "dry-run", "dryrun"}},
		{"scoped-key", []string{" where id", " where ", "_id", ".where", "find_by", "primary_key"}},
	}
	for _, check := range markerChecks {
		for _, token := range check.Tokens {
			if strings.Contains(lower, token) {
				markers = append(markers, check.Name)
				break
			}
		}
	}
	markers = uniqueStrings(markers)
	for _, check := range symbolic {
		if check.Status == "pass" {
			markers = append(markers, "symbolic-idempotency-pass")
			return "proven", uniqueStrings(markers)
		}
	}
	kind := strings.ToLower(risk.Kind)
	if containsAny(kind, "drop", "truncate", "delete") || riskHasFactor(risk, "destructive-code-path") || riskHasFactor(risk, "destructive-schema-change") || riskHasFactor(risk, "broad-write") {
		if containsAny(lower, "idempotent", "if exists", "where id", "where ", "limit", "primary key", "dry_run", "dry-run") {
			return "guarded", markers
		}
		return "non_idempotent", markers
	}
	if containsAny(lower, "idempotent", "idempotency", "upsert", "on conflict", "on duplicate key", "insert ignore", "merge into", "if not exists", "where not exists") {
		return "proven", markers
	}
	if containsAny(lower, "unique", "primary key", "dedupe", "deduplicate", "checkpoint", "cursor", "resume", "dry_run", "dry-run", "where id", "find_by", ".where") {
		return "guarded", markers
	}
	if riskHasFactor(risk, "missing-idempotency") || riskHasFactor(risk, "retry-hazard") {
		return "unknown", markers
	}
	if containsAny(kind, "insert", "create") {
		return "guarded", markers
	}
	return "unknown", markers
}

func idempotencySurface(kind, path string) string {
	lower := strings.ToLower(kind + " " + path)
	switch {
	case isGeneratedPath(path):
		return "generated_script"
	case containsAny(lower, "runbook", "rollback", "repair", "backfill", "reconcile", "fix"):
		return "repair_job"
	case strings.Contains(lower, "code-path"):
		if containsAny(lower, "job", "worker", "task", "cron") {
			return "backfill_job"
		}
		return "app_code"
	case strings.Contains(lower, "schema"):
		return "migration_dsl"
	case strings.HasSuffix(lower, ".sql") || strings.Contains(lower, "sql"):
		return "migration_sql"
	default:
		return "project_file"
	}
}

func isIdempotencySupportPath(path string) bool {
	lower := strings.ToLower(path)
	return containsAny(lower, "runbook", "rollback", "repair", "backfill", "reconcile", "fix", "script", ".sh", ".sql", ".md")
}

func idempotencyConfidence(status, text string) string {
	if text == "" {
		return "low"
	}
	switch status {
	case "proven":
		return "high"
	case "guarded", "non_idempotent":
		return "medium"
	default:
		return "derived"
	}
}

func idempotencyEvidence(risk BaselineRisk, slice ProvenanceSlice, symbolic []SymbolicCheck, markers []string) []string {
	var evidence []string
	if len(markers) > 0 {
		evidence = append(evidence, "idempotency markers: "+strings.Join(markers, ", "))
	}
	for _, check := range symbolic {
		evidence = append(evidence, "symbolic:idempotency:"+check.Status)
	}
	for _, factor := range risk.Factors {
		if factor.Name == "missing-idempotency" || factor.Name == "retry-hazard" || factor.Name == "broad-write" {
			evidence = append(evidence, factor.Name+": "+factor.Reason)
		}
	}
	if len(slice.StagesPresent) > 0 {
		evidence = append(evidence, "provenance stages: "+strings.Join(slice.StagesPresent, ", "))
	}
	return uniqueStrings(evidence)
}

func idempotencyRationale(status, surface string) string {
	switch status {
	case "proven":
		return surface + " has explicit idempotency semantics or a symbolic idempotency pass"
	case "guarded":
		return surface + " has scope, uniqueness, checkpoint, dry-run, or existence guards but is not fully proven"
	case "non_idempotent":
		return surface + " appears destructive or broad without repeat-safe guards"
	default:
		return surface + " lacks enough evidence to prove repeat-safe execution"
	}
}

func idempotencyStatusRank(status string) int {
	switch status {
	case "non_idempotent":
		return 4
	case "unknown":
		return 3
	case "guarded":
		return 2
	case "proven":
		return 1
	default:
		return 0
	}
}

func countIdempotencyStatus(classes []IdempotencyClass, status string) int {
	var count int
	for _, item := range classes {
		if item.Status == status {
			count++
		}
	}
	return count
}

func buildLockHazards(risks []BaselineRisk, slices []ProvenanceSlice, facts []Fact, intakeReport intake.Report) []LockHazard {
	root := firstNonEmpty(intakeReport.Source.ScannedRoot, intakeReport.Source.Input)
	sliceByRisk := map[string]ProvenanceSlice{}
	for _, slice := range slices {
		sliceByRisk[slice.RiskID] = slice
	}
	seen := map[string]bool{}
	var out []LockHazard
	for _, risk := range risks {
		if !riskNeedsLockHazardAnalysis(risk) {
			continue
		}
		text := textForBoundary(root, risk.Path)
		window := text
		if risk.Statement > 0 {
			window = lineWindow(text, risk.Statement, 10)
		}
		severity, markers, mitigations := classifyLockHazard(window, risk)
		if severity == "low" && window != text {
			fullSeverity, fullMarkers, fullMitigations := classifyLockHazard(text, risk)
			if lockHazardSeverityRank(fullSeverity) > lockHazardSeverityRank(severity) {
				severity, markers, mitigations = fullSeverity, fullMarkers, fullMitigations
			}
		}
		addLockHazard(&out, seen, LockHazard{
			ID:          "lock:" + canonical.Hash(fmt.Sprintf("risk\x00%s\x00%s\x00%d", risk.ID, risk.Path, risk.Statement))[:16],
			RiskID:      risk.ID,
			Path:        risk.Path,
			Line:        risk.Statement,
			Table:       risk.Table,
			Surface:     lockHazardSurface(risk.Kind, risk.Path),
			Operation:   transactionOperation(risk.Kind),
			Severity:    severity,
			Confidence:  lockHazardConfidence(severity, text),
			Markers:     markers,
			Mitigations: mitigations,
			Evidence:    lockHazardEvidence(risk, sliceByRisk[risk.ID], markers, mitigations),
			Identifiers: risk.Identifiers,
			Rationale:   lockHazardRationale(severity, lockHazardSurface(risk.Kind, risk.Path)),
		})
	}
	for _, fact := range facts {
		if !isConcurrencySupportFact(fact) {
			continue
		}
		text := textForBoundary(root, fact.Path)
		severity, markers, mitigations := classifyLockHazard(text, BaselineRisk{Kind: fact.Kind, Path: fact.Path})
		addLockHazard(&out, seen, LockHazard{
			ID:          "lock:" + canonical.Hash(fmt.Sprintf("fact\x00%s\x00%s", fact.ID, fact.Path))[:16],
			Path:        fact.Path,
			Table:       firstIdentifierValue(fact.Identifiers, "table"),
			Surface:     "job_or_runbook",
			Operation:   "support",
			Severity:    severity,
			Confidence:  lockHazardConfidence(severity, text),
			Markers:     markers,
			Mitigations: mitigations,
			Evidence:    []string{"project job, worker, runbook, or support file discovered by inventory"},
			Identifiers: fact.Identifiers,
			Rationale:   lockHazardRationale(severity, "job_or_runbook"),
		})
	}
	for _, slice := range slices {
		for _, path := range append(append([]string{}, slice.RepairPaths...), slice.SourcePaths...) {
			if !isLockHazardSupportPath(path) {
				continue
			}
			text := textForBoundary(root, path)
			severity, markers, mitigations := classifyLockHazard(text, BaselineRisk{Kind: "support", Path: path, Table: slice.Table})
			surface := "job_or_runbook"
			if isGeneratedPath(path) {
				surface = "generated_script"
			} else if strings.HasSuffix(strings.ToLower(path), ".sql") {
				surface = "migration_sql"
			}
			addLockHazard(&out, seen, LockHazard{
				ID:          "lock:" + canonical.Hash(fmt.Sprintf("support\x00%s\x00%s", slice.RiskID, path))[:16],
				RiskID:      slice.RiskID,
				Path:        path,
				Table:       slice.Table,
				Surface:     surface,
				Operation:   "support",
				Severity:    severity,
				Confidence:  lockHazardConfidence(severity, text),
				Markers:     markers,
				Mitigations: mitigations,
				Evidence:    []string{"support path linked by provenance slice"},
				Identifiers: slice.Identifiers,
				Rationale:   lockHazardRationale(severity, surface),
			})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Severity != out[j].Severity {
			return lockHazardSeverityRank(out[i].Severity) > lockHazardSeverityRank(out[j].Severity)
		}
		if out[i].Path != out[j].Path {
			return out[i].Path < out[j].Path
		}
		return out[i].ID < out[j].ID
	})
	return out
}

func addLockHazard(out *[]LockHazard, seen map[string]bool, item LockHazard) bool {
	key := item.RiskID + "\x00" + item.Path + "\x00" + item.Surface + "\x00" + item.Operation
	if seen[key] {
		return false
	}
	seen[key] = true
	*out = append(*out, item)
	return true
}

func riskNeedsLockHazardAnalysis(risk BaselineRisk) bool {
	kind := strings.ToLower(risk.Kind)
	if containsAny(kind, "alter", "create", "drop", "truncate", "update", "delete", "insert", "merge", "schema", "migration", "code-path") {
		return true
	}
	for _, factor := range risk.Factors {
		if containsAny(strings.ToLower(factor.Name), "write", "schema", "destructive", "retry", "persistent") {
			return true
		}
	}
	return false
}

func classifyLockHazard(text string, risk BaselineRisk) (string, []string, []string) {
	lower := strings.ToLower(text)
	kind := strings.ToLower(risk.Kind)
	var markers []string
	var mitigations []string
	markerChecks := []struct {
		Name   string
		Tokens []string
	}{
		{"explicit-lock", []string{"lock table", "for update", "with (tablock", "with(tablock", "lock in share mode", "advisory_lock", "get_lock("}},
		{"blocking-ddl", []string{"alter table", "drop table", "truncate table", "rename table", "vacuum full", "cluster "}},
		{"index-build", []string{"create index", "add index", "add_index", "create_index"}},
		{"non-online-index", []string{"create index ", "add index", "add_index", "create_index"}},
		{"table-rewrite", []string{"algorithm=copy", "type: string", "default now()", "default: ->", "set not null", "not null", "modify ", "change column", "drop column"}},
		{"broad-write", []string{"update ", "delete from", "merge into", "bulk_update", "update_all", "delete_all"}},
		{"job-contention", []string{"worker", "background", "cron", "sidekiq", "celery", "queue", "retry", "concurrent", "parallel"}},
		{"blast-radius-read", []string{"select count", "candidate_rows", "limit 1"}},
	}
	for _, check := range markerChecks {
		for _, token := range check.Tokens {
			if strings.Contains(lower, token) {
				markers = append(markers, check.Name)
				break
			}
		}
	}
	mitigationChecks := []struct {
		Name   string
		Tokens []string
	}{
		{"concurrent-index", []string{"create index concurrently", "algorithm=concurrently", "concurrently: true"}},
		{"online-ddl", []string{"algorithm=inplace", "algorithm=instant", "lock=none", "online", "without blocking"}},
		{"batching", []string{"batch", "in_batches", "find_each", "limit ", "chunk", "sleep", "throttle"}},
		{"skip-locked", []string{"skip locked", "nowait"}},
		{"transaction-boundary", []string{"transaction", "begin", "commit", "atomic"}},
		{"advisory-lock", []string{"advisory_lock", "get_lock("}},
		{"lock-timeout", []string{"lock_timeout", "statement_timeout", "innodb_lock_wait_timeout"}},
	}
	for _, check := range mitigationChecks {
		for _, token := range check.Tokens {
			if strings.Contains(lower, token) {
				mitigations = append(mitigations, check.Name)
				break
			}
		}
	}
	markers = uniqueStrings(markers)
	mitigations = uniqueStrings(mitigations)
	hasHazard := len(markers) > 0 || riskHasFactor(risk, "retry-hazard") || riskHasFactor(risk, "broad-write") || riskHasFactor(risk, "write-breadth-unknown")
	if !hasHazard && containsAny(kind, "alter", "drop", "truncate", "delete", "update") {
		markers = append(markers, "operation-lock-risk")
		hasHazard = true
	}
	if containsAny(lower, "lock table", "truncate table", "drop table", "vacuum full", "algorithm=copy") {
		if len(mitigations) == 0 || !containsAny(strings.Join(mitigations, " "), "online-ddl", "lock-timeout") {
			return "critical", uniqueStrings(markers), mitigations
		}
		return "high", uniqueStrings(markers), mitigations
	}
	if containsAny(lower, "alter table", "set not null", "drop column", "modify ", "change column", "default now()") || containsAny(kind, "schema", "alter") {
		if containsAny(strings.Join(mitigations, " "), "online-ddl", "concurrent-index") {
			return "medium", markers, mitigations
		}
		return "high", markers, mitigations
	}
	if containsAny(lower, "create index", "add index", "add_index", "create_index") && !containsAny(lower, "concurrently", "algorithm=concurrently", "algorithm=inplace", "algorithm=instant", "lock=none") {
		return "high", markers, mitigations
	}
	if riskHasFactor(risk, "retry-hazard") || (containsAny(lower, "worker", "background", "cron", "sidekiq", "celery", "queue", "retry") && containsAny(lower, "update ", "delete ", "insert ", "bulk_", "save!")) {
		if containsAny(strings.Join(mitigations, " "), "batching", "skip-locked", "advisory-lock") {
			return "medium", markers, mitigations
		}
		return "high", markers, mitigations
	}
	if riskHasFactor(risk, "broad-write") || riskHasFactor(risk, "write-breadth-unknown") || containsAny(lower, "update ", "delete from", "merge into", "bulk_update", "update_all", "delete_all") {
		if containsAny(strings.Join(mitigations, " "), "batching", "lock-timeout", "transaction-boundary") {
			return "medium", markers, mitigations
		}
		return "high", markers, mitigations
	}
	if hasHazard {
		if len(mitigations) > 0 {
			return "medium", markers, mitigations
		}
		return "high", markers, mitigations
	}
	return "low", markers, mitigations
}

func lockHazardSurface(kind, path string) string {
	lower := strings.ToLower(kind + " " + path)
	switch {
	case isGeneratedPath(path):
		return "generated_script"
	case containsAny(lower, "job", "worker", "task", "cron", "sidekiq", "celery", "queue"):
		return "background_job"
	case strings.Contains(lower, "code-path"):
		return "app_code"
	case strings.Contains(lower, "schema"):
		return "migration_dsl"
	case strings.HasSuffix(lower, ".sql") || strings.Contains(lower, "sql"):
		return "migration_sql"
	default:
		return "project_file"
	}
}

func isConcurrencySupportFact(fact Fact) bool {
	lower := strings.ToLower(fact.Kind + " " + fact.Path + " " + fact.Rationale)
	return containsAny(lower, "job", "worker", "cron", "queue", "sidekiq", "celery", "concurrency", "lock", "backfill", "migration", "repair")
}

func isLockHazardSupportPath(path string) bool {
	lower := strings.ToLower(path)
	return containsAny(lower, "job", "worker", "cron", "queue", "backfill", "repair", "migration", ".sql", ".rb", ".py", ".go", ".js", ".ts", ".md", ".sh")
}

func lockHazardConfidence(severity, text string) string {
	if text == "" {
		return "low"
	}
	switch severity {
	case "critical", "high":
		return "high"
	case "medium":
		return "medium"
	default:
		return "derived"
	}
}

func lockHazardEvidence(risk BaselineRisk, slice ProvenanceSlice, markers, mitigations []string) []string {
	var evidence []string
	if len(markers) > 0 {
		evidence = append(evidence, "lock/concurrency markers: "+strings.Join(markers, ", "))
	}
	if len(mitigations) > 0 {
		evidence = append(evidence, "mitigations: "+strings.Join(mitigations, ", "))
	}
	for _, factor := range risk.Factors {
		if containsAny(factor.Name, "broad-write", "write-breadth-unknown", "retry-hazard", "missing-transaction-boundary", "destructive") {
			evidence = append(evidence, factor.Name+": "+factor.Reason)
		}
	}
	if len(slice.StagesPresent) > 0 {
		evidence = append(evidence, "provenance stages: "+strings.Join(slice.StagesPresent, ", "))
	}
	return uniqueStrings(evidence)
}

func lockHazardRationale(severity, surface string) string {
	switch severity {
	case "critical":
		return surface + " contains blocking DDL or explicit table-lock evidence likely to block deploys or jobs"
	case "high":
		return surface + " contains schema/write/job patterns with material lock or contention risk and insufficient online/batching mitigation"
	case "medium":
		return surface + " contains lock/contention risk with partial mitigation such as batching, online DDL, timeouts, or skip-locked behavior"
	default:
		return surface + " has no strong lock/concurrency hazard in the scanned context"
	}
}

func lockHazardSeverityRank(severity string) int {
	switch severity {
	case "critical":
		return 4
	case "high":
		return 3
	case "medium":
		return 2
	case "low":
		return 1
	default:
		return 0
	}
}

func countLockHazardSeverity(hazards []LockHazard, severity string) int {
	var count int
	for _, hazard := range hazards {
		if hazard.Severity == severity {
			count++
		}
	}
	return count
}

func buildPrivacyHazards(risks []BaselineRisk, slices []ProvenanceSlice, facts []Fact, intakeReport intake.Report) []PrivacyHazard {
	root := firstNonEmpty(intakeReport.Source.ScannedRoot, intakeReport.Source.Input)
	sliceByRisk := map[string]ProvenanceSlice{}
	for _, slice := range slices {
		sliceByRisk[slice.RiskID] = slice
	}
	seen := map[string]bool{}
	var out []PrivacyHazard
	for _, risk := range risks {
		if !riskNeedsPrivacyHazardAnalysis(risk) {
			continue
		}
		text := textForBoundary(root, risk.Path)
		window := text
		if risk.Statement > 0 {
			window = lineWindow(text, risk.Statement, 10)
		}
		severity, markers, mitigations := classifyPrivacyHazard(window, risk, sliceByRisk[risk.ID])
		if severity == "low" && window != text {
			fullSeverity, fullMarkers, fullMitigations := classifyPrivacyHazard(text, risk, sliceByRisk[risk.ID])
			if privacyHazardSeverityRank(fullSeverity) > privacyHazardSeverityRank(severity) {
				severity, markers, mitigations = fullSeverity, fullMarkers, fullMitigations
			}
		}
		addPrivacyHazard(&out, seen, PrivacyHazard{
			ID:          "privacy:" + canonical.Hash(fmt.Sprintf("risk\x00%s\x00%s\x00%d", risk.ID, risk.Path, risk.Statement))[:16],
			RiskID:      risk.ID,
			Path:        risk.Path,
			Line:        risk.Statement,
			Table:       risk.Table,
			Surface:     privacyHazardSurface(risk.Kind, risk.Path),
			Operation:   transactionOperation(risk.Kind),
			Severity:    severity,
			Confidence:  privacyHazardConfidence(severity, text),
			Markers:     markers,
			Mitigations: mitigations,
			Evidence:    privacyHazardEvidence(risk, sliceByRisk[risk.ID], markers, mitigations),
			Identifiers: risk.Identifiers,
			Rationale:   privacyHazardRationale(severity, privacyHazardSurface(risk.Kind, risk.Path)),
		})
	}
	for _, fact := range facts {
		if !isPrivacySupportFact(fact) {
			continue
		}
		text := textForBoundary(root, fact.Path)
		severity, markers, mitigations := classifyPrivacyHazard(text, BaselineRisk{Kind: fact.Kind, Path: fact.Path, Table: firstIdentifierValue(fact.Identifiers, "table")}, ProvenanceSlice{})
		addPrivacyHazard(&out, seen, PrivacyHazard{
			ID:          "privacy:" + canonical.Hash(fmt.Sprintf("fact\x00%s\x00%s", fact.ID, fact.Path))[:16],
			Path:        fact.Path,
			Table:       firstIdentifierValue(fact.Identifiers, "table"),
			Surface:     privacyHazardSurface(fact.Kind, fact.Path),
			Operation:   "support",
			Severity:    severity,
			Confidence:  privacyHazardConfidence(severity, text),
			Markers:     markers,
			Mitigations: mitigations,
			Evidence:    []string{"privacy, retention, export, anonymization, or rollback-related file discovered by inventory"},
			Identifiers: fact.Identifiers,
			Rationale:   privacyHazardRationale(severity, privacyHazardSurface(fact.Kind, fact.Path)),
		})
	}
	for _, slice := range slices {
		for _, path := range append(append([]string{}, slice.RepairPaths...), slice.SourcePaths...) {
			if !isPrivacySupportPath(path) {
				continue
			}
			text := textForBoundary(root, path)
			severity, markers, mitigations := classifyPrivacyHazard(text, BaselineRisk{Kind: "support", Path: path, Table: slice.Table}, slice)
			addPrivacyHazard(&out, seen, PrivacyHazard{
				ID:          "privacy:" + canonical.Hash(fmt.Sprintf("support\x00%s\x00%s", slice.RiskID, path))[:16],
				RiskID:      slice.RiskID,
				Path:        path,
				Table:       slice.Table,
				Surface:     privacyHazardSurface("support", path),
				Operation:   "support",
				Severity:    severity,
				Confidence:  privacyHazardConfidence(severity, text),
				Markers:     markers,
				Mitigations: mitigations,
				Evidence:    []string{"privacy-relevant support path linked by provenance slice"},
				Identifiers: slice.Identifiers,
				Rationale:   privacyHazardRationale(severity, privacyHazardSurface("support", path)),
			})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Severity != out[j].Severity {
			return privacyHazardSeverityRank(out[i].Severity) > privacyHazardSeverityRank(out[j].Severity)
		}
		if out[i].Path != out[j].Path {
			return out[i].Path < out[j].Path
		}
		return out[i].ID < out[j].ID
	})
	return out
}

func addPrivacyHazard(out *[]PrivacyHazard, seen map[string]bool, item PrivacyHazard) bool {
	key := item.RiskID + "\x00" + item.Path + "\x00" + item.Surface + "\x00" + item.Operation
	if seen[key] {
		return false
	}
	seen[key] = true
	*out = append(*out, item)
	return true
}

func riskNeedsPrivacyHazardAnalysis(risk BaselineRisk) bool {
	kind := strings.ToLower(risk.Kind)
	if containsAny(kind, "delete", "drop", "truncate", "update", "export", "anonym", "privacy", "retention", "backfill", "repair", "code-path") {
		return true
	}
	if privacyTableOrIdentifierSignal(risk.Table, risk.Identifiers) {
		return true
	}
	for _, factor := range risk.Factors {
		if containsAny(strings.ToLower(factor.Name+" "+factor.Reason), "broad-write", "destructive", "rollback", "privacy", "retention", "delete") {
			return true
		}
	}
	return false
}

func classifyPrivacyHazard(text string, risk BaselineRisk, slice ProvenanceSlice) (string, []string, []string) {
	lower := strings.ToLower(text)
	kind := strings.ToLower(risk.Kind)
	var markers []string
	var mitigations []string
	markerChecks := []struct {
		Name   string
		Tokens []string
	}{
		{"broad-delete", []string{"delete from", "truncate ", "drop table", "destroy_all", "delete_all", "remove_all"}},
		{"anonymization-change", []string{"anonym", "pseudonym", "mask", "redact", "scrub", "sanitize", "hashed_email", "email_hash"}},
		{"export-script", []string{"copy ", "select *", "dump", "export", "csv", "parquet", "s3://", "gs://", "write.csv", "to_csv", "bulk insert"}},
		{"sensitive-identifier", []string{"email", "phone", "address", "ssn", "social_security", "dob", "birth", "token", "secret", "password", "api_key", "ip_address", "user_agent"}},
		{"retention-policy", []string{"retention", "ttl", "expires_at", "purge", "prune", "cleanup", "right to be forgotten", "gdpr", "ccpa"}},
		{"rollback-gap", []string{"irreversible", "no rollback", "cannot rollback", "disable_ddl_transaction", "raise activerecord::irreversiblemigration"}},
		{"broad-update", []string{"update ", "update_all", "bulk_update"}},
	}
	for _, check := range markerChecks {
		for _, token := range check.Tokens {
			if strings.Contains(lower, token) {
				markers = append(markers, check.Name)
				break
			}
		}
	}
	if privacyTableOrIdentifierSignal(risk.Table, risk.Identifiers) {
		markers = append(markers, "sensitive-table-or-identifier")
	}
	mitigationChecks := []struct {
		Name   string
		Tokens []string
	}{
		{"scoped-predicate", []string{" where ", "where id", ".where", "find_by", "limit ", "tenant_id", "account_id", "user_id"}},
		{"snapshot-backup", []string{"backup", "snapshot", "restore", "archive", "copy before", "create table backup"}},
		{"dry-run", []string{"dry_run", "dry-run", "dryrun", "preview", "explain"}},
		{"anonymized-output", []string{"anonymized", "pseudonymized", "redacted", "masked", "hashed", "digest("}},
		{"retention-window", []string{"older than", "created_at <", "expires_at", "ttl", "retention_days", "interval"}},
		{"approval-or-audit", []string{"approved", "audit", "review", "ticket", "compliance"}},
		{"rollback-evidence", []string{"rollback", "revert", "restore", "down do", "def down"}},
	}
	for _, check := range mitigationChecks {
		for _, token := range check.Tokens {
			if strings.Contains(lower, token) {
				mitigations = append(mitigations, check.Name)
				break
			}
		}
	}
	if len(slice.RepairPaths) > 0 {
		mitigations = append(mitigations, "linked-repair-evidence")
	}
	if containsAny(lower, "irreversible", "no rollback", "cannot rollback", "raise activerecord::irreversiblemigration") {
		mitigations = removeString(mitigations, "rollback-evidence")
	}
	markers = uniqueStrings(markers)
	mitigations = uniqueStrings(mitigations)
	rollbackGap := len(slice.RepairPaths) == 0 && (riskHasFactor(risk, "weak-rollback-signal") || containsAny(lower, "irreversible", "no rollback", "cannot rollback"))
	broadDestructive := containsAny(kind, "delete", "drop", "truncate") || riskHasFactor(risk, "destructive-code-path") || riskHasFactor(risk, "destructive-schema-change") || containsAny(lower, "delete from", "truncate ", "drop table", "destroy_all", "delete_all")
	broadWrite := riskHasFactor(risk, "broad-write") || riskHasFactor(risk, "write-breadth-unknown") || containsAny(lower, "update_all", "delete_all", "select *")
	sensitive := containsAny(strings.Join(markers, " "), "sensitive", "anonymization", "export", "retention")
	hasScope := containsAny(strings.Join(mitigations, " "), "scoped-predicate", "retention-window")
	hasRollback := containsAny(strings.Join(mitigations, " "), "rollback-evidence", "snapshot-backup", "linked-repair-evidence")
	hasPrivacyMitigation := containsAny(strings.Join(mitigations, " "), "anonymized-output", "approval-or-audit")
	switch {
	case broadDestructive && sensitive && rollbackGap:
		return "critical", markers, mitigations
	case broadDestructive && (sensitive || !hasScope):
		if hasRollback && (hasScope || hasPrivacyMitigation) {
			return "medium", markers, mitigations
		}
		return "high", markers, mitigations
	case containsAny(strings.Join(markers, " "), "export-script") && !hasPrivacyMitigation:
		if hasScope {
			return "medium", markers, mitigations
		}
		return "high", markers, mitigations
	case containsAny(strings.Join(markers, " "), "anonymization-change") && !hasRollback:
		return "high", markers, mitigations
	case rollbackGap && (broadWrite || sensitive):
		return "high", markers, mitigations
	case broadWrite || sensitive || len(markers) > 0:
		if hasScope || hasRollback || hasPrivacyMitigation {
			return "medium", markers, mitigations
		}
		return "high", markers, mitigations
	default:
		return "low", markers, mitigations
	}
}

func privacyTableOrIdentifierSignal(table string, ids []Identifier) bool {
	lower := strings.ToLower(table)
	if containsAny(lower, "user", "account", "customer", "person", "profile", "email", "address", "phone", "session", "token", "credential", "privacy", "consent") {
		return true
	}
	for _, id := range ids {
		value := strings.ToLower(id.Kind + " " + id.Value)
		if containsAny(value, "user", "account", "customer", "person", "profile", "email", "address", "phone", "session", "token", "credential", "privacy", "consent") {
			return true
		}
	}
	return false
}

func privacyHazardSurface(kind, path string) string {
	lower := strings.ToLower(kind + " " + path)
	switch {
	case isGeneratedPath(path):
		return "generated_script"
	case containsAny(lower, "export", "dump", ".csv", ".parquet"):
		return "export_script"
	case containsAny(lower, "runbook", "rollback", "repair", "backfill", "reconcile", "fix"):
		return "repair_or_runbook"
	case strings.Contains(lower, "code-path"):
		return "app_code"
	case strings.Contains(lower, "schema"):
		return "migration_dsl"
	case strings.HasSuffix(lower, ".sql") || strings.Contains(lower, "sql"):
		return "migration_sql"
	default:
		return "project_file"
	}
}

func isPrivacySupportFact(fact Fact) bool {
	lower := strings.ToLower(fact.Kind + " " + fact.Path + " " + fact.Rationale)
	return containsAny(lower, "privacy", "retention", "gdpr", "ccpa", "anonym", "redact", "export", "dump", "delete", "purge", "cleanup", "rollback", "repair", "backfill")
}

func isPrivacySupportPath(path string) bool {
	lower := strings.ToLower(path)
	return containsAny(lower, "privacy", "retention", "gdpr", "ccpa", "anonym", "redact", "export", "dump", "delete", "purge", "cleanup", "rollback", "repair", "backfill", ".sql", ".rb", ".py", ".go", ".js", ".ts", ".sh")
}

func privacyHazardConfidence(severity, text string) string {
	if text == "" {
		return "low"
	}
	switch severity {
	case "critical", "high":
		return "high"
	case "medium":
		return "medium"
	default:
		return "derived"
	}
}

func privacyHazardEvidence(risk BaselineRisk, slice ProvenanceSlice, markers, mitigations []string) []string {
	var evidence []string
	if len(markers) > 0 {
		evidence = append(evidence, "privacy/retention markers: "+strings.Join(markers, ", "))
	}
	if len(mitigations) > 0 {
		evidence = append(evidence, "mitigations: "+strings.Join(mitigations, ", "))
	}
	for _, factor := range risk.Factors {
		if containsAny(strings.ToLower(factor.Name+" "+factor.Reason), "broad-write", "destructive", "rollback", "delete", "privacy", "retention") {
			evidence = append(evidence, factor.Name+": "+factor.Reason)
		}
	}
	if len(slice.StagesPresent) > 0 {
		evidence = append(evidence, "provenance stages: "+strings.Join(slice.StagesPresent, ", "))
	}
	return uniqueStrings(evidence)
}

func privacyHazardRationale(severity, surface string) string {
	switch severity {
	case "critical":
		return surface + " combines broad/destructive data change, sensitive or retention-related data, and a rollback gap"
	case "high":
		return surface + " contains privacy, retention, export, anonymization, or broad-delete risk without enough scope, audit, anonymization, or rollback evidence"
	case "medium":
		return surface + " contains privacy/retention risk with partial mitigation such as scope predicates, anonymized output, snapshots, audits, or rollback evidence"
	default:
		return surface + " has no strong privacy or retention hazard in the scanned context"
	}
}

func privacyHazardSeverityRank(severity string) int {
	switch severity {
	case "critical":
		return 4
	case "high":
		return 3
	case "medium":
		return 2
	case "low":
		return 1
	default:
		return 0
	}
}

func countPrivacyHazardSeverity(hazards []PrivacyHazard, severity string) int {
	var count int
	for _, hazard := range hazards {
		if hazard.Severity == severity {
			count++
		}
	}
	return count
}

func removeString(values []string, target string) []string {
	var out []string
	for _, value := range values {
		if value != target {
			out = append(out, value)
		}
	}
	return out
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func buildInvariantCandidates(inventoryRoot string, facts []Fact, intakeReport intake.Report) []InvariantCandidate {
	root := firstNonEmpty(inventoryRoot, intakeReport.Source.ScannedRoot, intakeReport.Source.Input)
	seen := map[string]bool{}
	var out []InvariantCandidate
	for _, fact := range facts {
		if fact.Kind == "schema_evolution" {
			text := textForBoundary(root, fact.Path)
			for _, inv := range schemaInvariantsFromFact(fact, text) {
				addInvariantCandidate(&out, seen, inv)
			}
		}
		if !isInvariantMiningPath(fact.Path, fact.Kind) {
			continue
		}
		text := textForBoundary(root, fact.Path)
		if text == "" {
			continue
		}
		for _, inv := range invariantsFromTextPath(fact, text) {
			addInvariantCandidate(&out, seen, inv)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Confidence != out[j].Confidence {
			return invariantConfidenceRank(out[i].Confidence) > invariantConfidenceRank(out[j].Confidence)
		}
		if out[i].Source != out[j].Source {
			return out[i].Source < out[j].Source
		}
		if out[i].Path != out[j].Path {
			return out[i].Path < out[j].Path
		}
		return out[i].ID < out[j].ID
	})
	if len(out) > 500 {
		out = out[:500]
	}
	return out
}

func schemaInvariantsFromFact(fact Fact, text string) []InvariantCandidate {
	table := firstNonEmpty(fact.Properties["table"], firstIdentifierValue(fact.Identifiers, "table"), firstIdentifierValue(fact.Identifiers, "model"))
	column := firstNonEmpty(fact.Properties["column"], firstIdentifierValue(fact.Identifiers, "column"))
	columnType := fact.Properties["column_type"]
	line, context := invariantLineForColumn(text, column)
	base := InvariantCandidate{
		Source:      "schema",
		Path:        fact.Path,
		Line:        line,
		Table:       table,
		Column:      column,
		Confidence:  "derived",
		Evidence:    []string{"schema_evolution fact: " + fact.ID},
		Identifiers: fact.Identifiers,
		Rationale:   "candidate invariant mined from schema evolution or ORM schema declarations",
	}
	var out []InvariantCandidate
	if table != "" {
		item := base
		item.Kind = "table-exists"
		item.Expression = "table(" + table + ") exists"
		out = append(out, finalizeInvariant(item))
	}
	if column != "" {
		item := base
		item.Kind = "column-exists"
		item.Expression = invariantColumnExpr(table, column, "exists")
		out = append(out, finalizeInvariant(item))
	}
	if columnType != "" {
		item := base
		item.Kind = "column-type"
		item.Expression = invariantColumnExpr(table, column, "type="+columnType)
		out = append(out, finalizeInvariant(item))
	}
	lower := strings.ToLower(context)
	if containsAny(lower, "not null", "null: false", "nullable=false", "nullable: false", "blank=false", "blank: false", "@notnull", "required") {
		item := base
		item.Kind = "not-null"
		item.Expression = invariantColumnExpr(table, column, "not_null")
		item.Confidence = "high"
		item.Evidence = append(item.Evidence, strings.TrimSpace(context))
		out = append(out, finalizeInvariant(item))
	}
	if containsAny(lower, "primary key", "primary_key", "@id", "primarykey") {
		item := base
		item.Kind = "primary-key"
		item.Expression = invariantColumnExpr(table, column, "primary_key")
		item.Confidence = "high"
		item.Evidence = append(item.Evidence, strings.TrimSpace(context))
		out = append(out, finalizeInvariant(item))
	}
	if containsAny(lower, "unique", "@unique", "uniqueindex", "unique_together") {
		item := base
		item.Kind = "unique"
		item.Expression = invariantColumnExpr(table, column, "unique")
		item.Confidence = "high"
		item.Evidence = append(item.Evidence, strings.TrimSpace(context))
		out = append(out, finalizeInvariant(item))
	}
	if containsAny(lower, "check ", "check(", "constraint", "validate") {
		item := base
		item.Kind = "check-constraint"
		item.Expression = "check(" + compactInvariantExpression(context) + ")"
		item.Confidence = "high"
		item.Evidence = append(item.Evidence, strings.TrimSpace(context))
		out = append(out, finalizeInvariant(item))
	}
	return out
}

func invariantsFromTextPath(fact Fact, text string) []InvariantCandidate {
	lowerPath := strings.ToLower(filepath.ToSlash(fact.Path))
	switch {
	case isTestPath(lowerPath):
		return testInvariantsFromText(fact, text)
	case isValidationPath(lowerPath, text):
		return validationInvariantsFromText(fact, text)
	case isFixturePath(lowerPath) || isLikelyFixtureText(lowerPath, text) || isFactoryExampleText(text):
		return fixtureInvariantsFromText(fact, text)
	default:
		return nil
	}
}

func testInvariantsFromText(fact Fact, text string) []InvariantCandidate {
	var out []InvariantCandidate
	for lineNo, line := range strings.Split(text, "\n") {
		lower := strings.ToLower(line)
		if !containsAny(lower, "assert", "expect(", "expect ", "should", "require.", "equal", "not_to", "to be", "to eq", "not_nil", "not nil", "valid") {
			continue
		}
		table, column := invariantTableColumnFromText(line, fact)
		kind := "assertion"
		if containsAny(lower, "not_nil", "not nil", "not_to be_nil", "not null", "present", "presence") {
			kind = "not-null"
		} else if containsAny(lower, "unique", "duplicate") {
			kind = "unique"
		} else if containsAny(lower, "valid", "invalid", "error") {
			kind = "validation-behavior"
		}
		out = append(out, finalizeInvariant(InvariantCandidate{
			Source:      "test",
			Kind:        kind,
			Path:        fact.Path,
			Line:        lineNo + 1,
			Table:       table,
			Column:      column,
			Expression:  compactInvariantExpression(line),
			Confidence:  "medium",
			Evidence:    []string{strings.TrimSpace(line)},
			Identifiers: uniqueIdentifiers(append(append([]Identifier{}, fact.Identifiers...), identifiersFromText(line)...)),
			Rationale:   "candidate invariant mined from test assertion text",
		}))
		if len(out) >= 40 {
			break
		}
	}
	return out
}

func validationInvariantsFromText(fact Fact, text string) []InvariantCandidate {
	var out []InvariantCandidate
	for lineNo, line := range strings.Split(text, "\n") {
		lower := strings.ToLower(line)
		if !containsAny(lower, "validates", "presence:", "uniqueness:", "length:", "format:", "nullable=false", "null=false", "null: false", "blank=false", "blank: false", "@notnull", "@size", "@pattern", "required", "joi.", "z.string", "constraint") {
			continue
		}
		table, column := invariantTableColumnFromText(line, fact)
		kind := "validation"
		switch {
		case containsAny(lower, "presence:", "required", "@notnull", "nullable=false", "null=false", "null: false", "blank=false", "blank: false"):
			kind = "not-null"
		case containsAny(lower, "uniqueness:", "unique"):
			kind = "unique"
		case containsAny(lower, "length:", "@size", "max_length", "min_length"):
			kind = "length"
		case containsAny(lower, "format:", "@pattern", "regex", "email"):
			kind = "format"
		}
		out = append(out, finalizeInvariant(InvariantCandidate{
			Source:      "validation",
			Kind:        kind,
			Path:        fact.Path,
			Line:        lineNo + 1,
			Table:       table,
			Column:      column,
			Expression:  compactInvariantExpression(line),
			Confidence:  "high",
			Evidence:    []string{strings.TrimSpace(line)},
			Identifiers: uniqueIdentifiers(append(append([]Identifier{}, fact.Identifiers...), identifiersFromText(line)...)),
			Rationale:   "candidate invariant mined from application validation declarations",
		}))
		if len(out) >= 40 {
			break
		}
	}
	return out
}

func fixtureInvariantsFromText(fact Fact, text string) []InvariantCandidate {
	columns := fixtureColumns(text)
	if len(columns) == 0 {
		return nil
	}
	table := firstNonEmpty(firstIdentifierValue(fact.Identifiers, "table"), tableFromFixturePath(fact.Path))
	var out []InvariantCandidate
	for _, column := range capStrings(columns, 12) {
		out = append(out, finalizeInvariant(InvariantCandidate{
			Source:      "fixture",
			Kind:        "example-non-null",
			Path:        fact.Path,
			Table:       table,
			Column:      column,
			Expression:  invariantColumnExpr(table, column, "observed_non_null_in_example_data"),
			Confidence:  "example",
			Evidence:    []string{"field observed in fixture or production-like example data"},
			Identifiers: uniqueIdentifiers(append(append([]Identifier{}, fact.Identifiers...), Identifier{Kind: "column", Value: column})),
			Rationale:   "candidate invariant mined from fixtures, factories, seeds, or production-like example data",
		}))
	}
	return out
}

func addInvariantCandidate(out *[]InvariantCandidate, seen map[string]bool, item InvariantCandidate) bool {
	if item.Expression == "" {
		return false
	}
	key := item.Source + "\x00" + item.Kind + "\x00" + item.Path + "\x00" + item.Table + "\x00" + item.Column + "\x00" + item.Expression
	if seen[key] {
		return false
	}
	seen[key] = true
	*out = append(*out, finalizeInvariant(item))
	return true
}

func finalizeInvariant(item InvariantCandidate) InvariantCandidate {
	item.Table = normalizeIdentifierValue(item.Table)
	item.Column = normalizeIdentifierValue(item.Column)
	item.Identifiers = uniqueIdentifiers(item.Identifiers)
	item.Evidence = capStrings(uniqueSortedStrings(item.Evidence), 8)
	if item.ID == "" {
		item.ID = "invariant:" + canonical.Hash(strings.Join([]string{item.Source, item.Kind, item.Path, item.Table, item.Column, item.Expression}, "\x00"))[:16]
	}
	return item
}

func isInvariantMiningPath(path, kind string) bool {
	lowerPath := strings.ToLower(filepath.ToSlash(path))
	lower := lowerPath + " " + strings.ToLower(kind)
	return isTestPath(lower) || isValidationPath(lower, "") || isFixturePath(lower) || strings.Contains(lower, "schema_evolution") || hasInvariantMiningExtension(lowerPath)
}

func isTestPath(lowerPath string) bool {
	return containsAny(lowerPath, "/test/", "/tests/", "/spec/", "__tests__", "_test.", ".test.", "_spec.", ".spec.")
}

func isValidationPath(lowerPath, text string) bool {
	lower := strings.ToLower(lowerPath + " " + text)
	return containsAny(lower, "models/", "model/", "validators", "validation", "schema", "serializer", "forms.py", "validates", "nullable=false", "null: false", "presence:", "uniqueness:", "@notnull")
}

func isFixturePath(lowerPath string) bool {
	return containsAny(lowerPath, "fixture", "fixtures", "factory", "factories", "seed", "seeds", "sample", "example", "examples", "testdata", "golden")
}

func hasInvariantMiningExtension(lowerPath string) bool {
	for _, suffix := range []string{".rb", ".py", ".go", ".java", ".kt", ".js", ".ts", ".tsx", ".jsx", ".php", ".cs", ".sql", ".prisma", ".yml", ".yaml", ".json", ".jsonl", ".csv", ".toml"} {
		if strings.HasSuffix(lowerPath, suffix) {
			return true
		}
	}
	return false
}

func isLikelyFixtureText(lowerPath, text string) bool {
	if !containsAny(lowerPath, ".yml", ".yaml", ".json", ".jsonl", ".csv", ".toml") {
		return false
	}
	lower := strings.ToLower(text)
	return containsAny(lower, "email", "user", "account", "id:", "\"id\"", "created_at", "updated_at") || strings.Contains(firstNonEmpty(firstCSVLine(text), ""), ",")
}

func isFactoryExampleText(text string) bool {
	lower := strings.ToLower(text)
	return strings.Contains(lower, "factorybot.define") || strings.Contains(lower, "factory :") || strings.Contains(lower, "factory(")
}

func invariantLineForColumn(text, column string) (int, string) {
	if text == "" {
		return 0, ""
	}
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		if column == "" || strings.Contains(strings.ToLower(line), strings.ToLower(column)) {
			return i + 1, strings.TrimSpace(line)
		}
	}
	return 0, ""
}

func invariantTableColumnFromText(line string, fact Fact) (string, string) {
	ids := uniqueIdentifiers(append(append([]Identifier{}, fact.Identifiers...), identifiersFromText(line)...))
	table := firstNonEmpty(firstIdentifierValue(ids, "table"), firstIdentifierValue(ids, "model"), tableFromFixturePath(fact.Path))
	column := firstIdentifierValue(ids, "column")
	if column == "" {
		column = firstColumnLikeToken(line)
	}
	return table, column
}

func fixtureColumns(text string) []string {
	factoryColumns := factoryBotColumns(text)
	if len(factoryColumns) > 0 {
		return factoryColumns
	}
	lines := strings.Split(text, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, "//") {
			continue
		}
		if strings.Contains(trimmed, ",") && !strings.Contains(trimmed, "{") {
			return normalizeColumnList(strings.Split(trimmed, ","))
		}
		break
	}
	keyPattern := regexp.MustCompile(`(?m)["']?([A-Za-z_][A-Za-z0-9_]*)["']?\s*[:=]`)
	var columns []string
	for _, match := range keyPattern.FindAllStringSubmatch(text, 60) {
		key := normalizeIdentifierValue(match[1])
		if key == "" || isInvariantStopword(key) {
			continue
		}
		columns = append(columns, key)
	}
	return uniqueSortedStrings(columns)
}

func factoryBotColumns(text string) []string {
	var columns []string
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`(?m)^\s*([A-Za-z_][A-Za-z0-9_!?]*)\s*\{`),
		regexp.MustCompile(`(?m)^\s*sequence\(:([A-Za-z_][A-Za-z0-9_]*)\)`),
		regexp.MustCompile(`(?m)^\s*association\(:([A-Za-z_][A-Za-z0-9_]*)\)`),
	}
	for _, pattern := range patterns {
		for _, match := range pattern.FindAllStringSubmatch(text, -1) {
			column := normalizeIdentifierValue(strings.TrimSuffix(match[1], "!"))
			if column == "" || isInvariantStopword(column) || column == "factory" || column == "trait" || column == "transient" {
				continue
			}
			columns = append(columns, column)
		}
	}
	return uniqueSortedStrings(columns)
}

func firstCSVLine(text string) string {
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "#") && strings.Contains(line, ",") {
			return line
		}
	}
	return ""
}

func normalizeColumnList(values []string) []string {
	var columns []string
	for _, value := range values {
		value = normalizeIdentifierValue(strings.Trim(value, " \t\"'`"))
		if value == "" || isInvariantStopword(value) {
			continue
		}
		columns = append(columns, value)
	}
	return uniqueSortedStrings(columns)
}

func tableFromFixturePath(path string) string {
	base := strings.TrimSuffix(strings.ToLower(filepath.Base(path)), filepath.Ext(path))
	base = strings.TrimSuffix(base, "_fixture")
	base = strings.TrimSuffix(base, "_fixtures")
	base = strings.TrimSuffix(base, "_factory")
	base = strings.TrimSuffix(base, "_factories")
	base = strings.TrimSuffix(base, "_seed")
	base = strings.TrimSuffix(base, "_seeds")
	return normalizeIdentifierValue(base)
}

func firstColumnLikeToken(line string) string {
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`(?i)validates\s+:([A-Za-z_][A-Za-z0-9_]*)`),
		regexp.MustCompile(`(?i)["']([A-Za-z_][A-Za-z0-9_]*)["']\s*(?:=>|:)`),
		regexp.MustCompile(`(?i)\.([A-Za-z_][A-Za-z0-9_]*)\s*(?:=|==|to|not_to)`),
	}
	for _, pattern := range patterns {
		if match := pattern.FindStringSubmatch(line); len(match) > 1 {
			if value := normalizeIdentifierValue(match[1]); value != "" && !isInvariantStopword(value) {
				return value
			}
		}
	}
	return ""
}

func invariantColumnExpr(table, column, predicate string) string {
	if table != "" && column != "" {
		return table + "." + column + " " + predicate
	}
	if column != "" {
		return column + " " + predicate
	}
	if table != "" {
		return table + " " + predicate
	}
	return predicate
}

func compactInvariantExpression(value string) string {
	value = strings.Join(strings.Fields(strings.TrimSpace(value)), " ")
	if len(value) > 180 {
		value = value[:180]
	}
	return value
}

func isInvariantStopword(value string) bool {
	switch value {
	case "true", "false", "null", "nil", "none", "type", "class", "def", "end", "do", "if", "else", "return", "let", "var", "const", "expect", "assert", "should", "describe", "context", "it":
		return true
	default:
		return false
	}
}

func invariantConfidenceRank(confidence string) int {
	switch confidence {
	case "high":
		return 4
	case "medium":
		return 3
	case "derived":
		return 2
	case "example":
		return 1
	default:
		return 0
	}
}

func countInvariantSource(items []InvariantCandidate, source string) int {
	var count int
	for _, item := range items {
		if item.Source == source {
			count++
		}
	}
	return count
}

func buildTraceCodeLinks(inventoryRoot string, risks []BaselineRisk, slices []ProvenanceSlice, facts []Fact, intakeReport intake.Report) []TraceCodeLink {
	root := firstNonEmpty(inventoryRoot, intakeReport.Source.ScannedRoot, intakeReport.Source.Input)
	codeFacts := traceCodeFacts(facts)
	traceFacts := traceEvidenceFacts(facts)
	sliceByPath := provenanceByCodePath(slices)
	riskByID := map[string]BaselineRisk{}
	for _, risk := range risks {
		riskByID[risk.ID] = risk
	}
	seen := map[string]bool{}
	var out []TraceCodeLink
	for _, traceFact := range traceFacts {
		traceText := traceFactText(root, traceFact)
		traceIDs := uniqueIdentifiers(append(append([]Identifier{}, traceFact.Identifiers...), identifiersFromText(traceText)...))
		kind, signals := traceEvidenceKind(traceFact, traceText)
		for _, codeFact := range codeFacts {
			codeText := traceFactText(root, codeFact)
			codeIDs := uniqueIdentifiers(append(append([]Identifier{}, codeFact.Identifiers...), identifiersFromText(codeText)...))
			shared := sharedIdentifiers(traceIDs, codeIDs)
			pathSignals := tracePathSignals(traceText, codeFact.Path)
			if len(shared) == 0 && len(pathSignals) == 0 {
				continue
			}
			confidence := traceLinkConfidence(kind, shared, pathSignals, traceFact, codeFact)
			riskID := firstRiskForCodePath(sliceByPath[codeFact.Path], riskByID)
			signalsOut := uniqueStrings(append(append([]string{}, signals...), pathSignals...))
			link := TraceCodeLink{
				ID:          "trace-link:" + canonical.Hash(strings.Join([]string{traceFact.ID, codeFact.ID, codeFact.Path, confidence}, "\x00"))[:16],
				SourcePath:  traceFact.Path,
				CodePath:    codeFact.Path,
				RiskID:      riskID,
				Kind:        kind,
				Relation:    "observability-evidence-to-code",
				Confidence:  confidence,
				Signals:     signalsOut,
				Identifiers: shared,
				Time:        traceTimeSignal(traceFact, traceText),
				Evidence:    traceLinkEvidence(traceFact, codeFact, shared, signalsOut),
				Rationale:   traceLinkRationale(kind, confidence),
			}
			addTraceCodeLink(&out, seen, link)
		}
		if len(out) > 700 {
			break
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Confidence != out[j].Confidence {
			return traceLinkConfidenceRank(out[i].Confidence) > traceLinkConfidenceRank(out[j].Confidence)
		}
		if out[i].Kind != out[j].Kind {
			return out[i].Kind < out[j].Kind
		}
		if out[i].SourcePath != out[j].SourcePath {
			return out[i].SourcePath < out[j].SourcePath
		}
		return out[i].CodePath < out[j].CodePath
	})
	if len(out) > 500 {
		out = out[:500]
	}
	return out
}

func addTraceCodeLink(out *[]TraceCodeLink, seen map[string]bool, link TraceCodeLink) bool {
	key := link.SourcePath + "\x00" + link.CodePath + "\x00" + link.Kind + "\x00" + strings.Join(identifierKeys(link.Identifiers), ",")
	if seen[key] {
		return false
	}
	seen[key] = true
	*out = append(*out, link)
	return true
}

func traceCodeFacts(facts []Fact) []Fact {
	var out []Fact
	for _, fact := range facts {
		lower := strings.ToLower(fact.Kind + " " + fact.Path)
		if fact.Path == "" {
			continue
		}
		if fact.Kind == "file" || fact.Kind == "source_sql_hint" || fact.Kind == "schema_evolution" || containsAny(lower, "source", "migration", "model", "job", "worker") {
			if hasCodeLikeExtension(fact.Path) || fact.Kind == "schema_evolution" || fact.Kind == "source_sql_hint" {
				out = append(out, fact)
			}
		}
	}
	return out
}

func traceEvidenceFacts(facts []Fact) []Fact {
	var out []Fact
	for _, fact := range facts {
		if fact.Path == "" {
			continue
		}
		lower := strings.ToLower(fact.Kind + " " + fact.Path + " " + fact.Rationale + " " + fact.Properties["field"] + " " + fact.Properties["value_preview"])
		if fact.Kind == "evidence_export" || fact.Kind == "operational_doc" || fact.Kind == "field_evidence" || fact.Kind == "file" {
			if containsAny(lower, "trace", "span", "otel", "opentelemetry", "datadog", "dd.", "deploy", "deployment", "incident", "timeline", "log", "structured", "service", "resource", "operation_name", "trace_id", "span_id", "commit") {
				out = append(out, fact)
			}
		}
	}
	return out
}

func traceFactText(root string, fact Fact) string {
	var parts []string
	parts = append(parts, fact.Kind, fact.Path, fact.Rationale)
	for key, value := range fact.Properties {
		parts = append(parts, key, value)
	}
	if text := textForBoundary(root, fact.Path); text != "" {
		parts = append(parts, text)
	}
	return strings.Join(parts, "\n")
}

func traceEvidenceKind(fact Fact, text string) (string, []string) {
	lower := strings.ToLower(fact.Path + " " + fact.Kind + " " + fact.Rationale + " " + text)
	checks := []struct {
		Kind   string
		Tokens []string
	}{
		{"opentelemetry", []string{"opentelemetry", "otel", "resource_spans", "resourcespans", "scope_spans", "scopespans", "resource.attributes"}},
		{"datadog", []string{"datadog", "dd.", "ddtrace", "\"traces\"", "resource_name"}},
		{"structured_log", []string{".log", "level=", "msg=", "logger", "structured", "jsonl", "trace_id", "request_id"}},
		{"incident_timeline", []string{"incident", "postmortem", "timeline", "outage", "sev", "error"}},
		{"deploy_marker", []string{"deploy", "deployment", "commit", "release", "sha"}},
	}
	var signals []string
	for _, check := range checks {
		for _, token := range check.Tokens {
			if strings.Contains(lower, token) {
				if !containsString(signals, check.Kind) {
					signals = append(signals, check.Kind)
				}
				break
			}
		}
	}
	if len(signals) == 0 {
		return "observability_evidence", nil
	}
	return signals[0], signals
}

func tracePathSignals(text, codePath string) []string {
	if text == "" || codePath == "" {
		return nil
	}
	lower := strings.ToLower(text)
	path := strings.ToLower(filepath.ToSlash(codePath))
	base := strings.ToLower(strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)))
	var signals []string
	if path != "" && strings.Contains(lower, path) {
		signals = append(signals, "code-path-exact")
	}
	if base != "" && len(base) > 3 && strings.Contains(lower, base) {
		signals = append(signals, "code-symbol-or-file")
	}
	return uniqueStrings(signals)
}

func traceLinkConfidence(kind string, shared []Identifier, pathSignals []string, traceFact, codeFact Fact) string {
	if containsAny(strings.Join(pathSignals, " "), "code-path-exact") {
		return "exact"
	}
	for _, id := range shared {
		if id.Kind == "commit" || id.Kind == "endpoint" || id.Kind == "job" || id.Kind == "queue" {
			return "causal"
		}
	}
	if len(shared) > 0 {
		return "inferred"
	}
	if kind == "deploy_marker" || kind == "incident_timeline" || traceFact.Path == codeFact.Path {
		return "temporal"
	}
	return "inferred"
}

func traceTimeSignal(fact Fact, text string) string {
	for _, id := range append(append([]Identifier{}, fact.Identifiers...), identifiersFromText(text)...) {
		if id.Kind == "timestamp" || id.Kind == "date" {
			return id.Value
		}
	}
	return ""
}

func traceLinkEvidence(traceFact, codeFact Fact, shared []Identifier, signals []string) []string {
	var evidence []string
	evidence = append(evidence, "observability fact: "+traceFact.ID)
	evidence = append(evidence, "code fact: "+codeFact.ID)
	if len(shared) > 0 {
		var ids []string
		for _, id := range shared {
			ids = append(ids, id.Kind+":"+id.Value)
		}
		evidence = append(evidence, "shared identifiers: "+strings.Join(uniqueSortedStrings(ids), ", "))
	}
	if len(signals) > 0 {
		evidence = append(evidence, "signals: "+strings.Join(signals, ", "))
	}
	return capStrings(uniqueSortedStrings(evidence), 8)
}

func traceLinkRationale(kind, confidence string) string {
	return "links " + kind + " evidence to code using deterministic shared identifiers, path mentions, deploy markers, incident timelines, or structured log fields at " + confidence + " confidence"
}

func provenanceByCodePath(slices []ProvenanceSlice) map[string][]ProvenanceSlice {
	out := map[string][]ProvenanceSlice{}
	for _, slice := range slices {
		if slice.MigrationPath != "" {
			out[slice.MigrationPath] = append(out[slice.MigrationPath], slice)
		}
		for _, path := range slice.SourcePaths {
			out[path] = append(out[path], slice)
		}
		for _, path := range slice.IncidentPaths {
			out[path] = append(out[path], slice)
		}
	}
	return out
}

func firstRiskForCodePath(slices []ProvenanceSlice, risks map[string]BaselineRisk) string {
	for _, slice := range slices {
		if _, ok := risks[slice.RiskID]; ok {
			return slice.RiskID
		}
	}
	return ""
}

func hasCodeLikeExtension(path string) bool {
	lower := strings.ToLower(path)
	for _, suffix := range []string{".go", ".py", ".rb", ".js", ".ts", ".tsx", ".jsx", ".java", ".kt", ".cs", ".php", ".rs", ".sql", ".prisma"} {
		if strings.HasSuffix(lower, suffix) {
			return true
		}
	}
	return false
}

func identifierKeys(ids []Identifier) []string {
	var keys []string
	for _, id := range ids {
		if key := canonicalIdentifier(id.Kind, id.Value); key != "" {
			keys = append(keys, key)
		}
	}
	return uniqueSortedStrings(keys)
}

func traceLinkConfidenceRank(confidence string) int {
	switch confidence {
	case "exact":
		return 4
	case "causal":
		return 3
	case "temporal":
		return 2
	case "inferred":
		return 1
	default:
		return 0
	}
}

func countTraceLinkConfidence(links []TraceCodeLink, confidence string) int {
	var count int
	for _, link := range links {
		if link.Confidence == confidence {
			count++
		}
	}
	return count
}

func buildBlastRadiusEstimates(inventoryRoot string, risks []BaselineRisk, slices []ProvenanceSlice, facts []Fact, intakeReport intake.Report) []BlastRadiusEstimate {
	root := firstNonEmpty(inventoryRoot, intakeReport.Source.ScannedRoot, intakeReport.Source.Input)
	tableFacts := blastFactsByTable(root, facts)
	queryPaths := blastQueryPathsByTable(root, facts)
	fkGraph := blastForeignKeyGraph(root, facts)
	sliceByRisk := map[string]ProvenanceSlice{}
	for _, slice := range slices {
		sliceByRisk[slice.RiskID] = slice
	}
	var out []BlastRadiusEstimate
	seen := map[string]bool{}
	for _, risk := range risks {
		table := normalizeBlastTable(firstNonEmpty(risk.Table, firstIdentifierValue(risk.Identifiers, "table"), firstIdentifierValue(risk.Identifiers, "model")))
		if table == "" {
			continue
		}
		slice := sliceByRisk[risk.ID]
		affected := blastReachableTables(table, fkGraph, 2)
		centrality := len(uniqueStrings(append(tableFacts[table], queryPaths[table]...)))
		sourcePaths := blastSourcePathsForRisk(table, risk, slice, tableFacts)
		queryUsage := len(queryPaths[table])
		fanout := len(sourcePaths)
		score := blastRadiusScore(risk, centrality, len(affected), fanout, queryUsage)
		estimate := BlastRadiusEstimate{
			ID:              "blast:" + canonical.Hash(strings.Join([]string{risk.ID, table, strings.Join(affected, ",")}, "\x00"))[:16],
			RiskID:          risk.ID,
			Table:           table,
			Level:           blastRadiusLevel(score),
			Score:           score,
			TableCentrality: centrality,
			FKReachability:  len(affected),
			CodePathFanout:  fanout,
			QueryUsage:      queryUsage,
			AffectedTables:  affected,
			SourcePaths:     sourcePaths,
			QueryPaths:      capStrings(queryPaths[table], 12),
			Evidence:        blastRadiusEvidence(risk, slice, affected, sourcePaths, queryPaths[table]),
			Identifiers:     uniqueIdentifiers(append(risk.Identifiers, Identifier{Kind: "table", Value: table})),
			Rationale:       "approximate blast radius combines table centrality, foreign-key reachability, code-path fanout, and observed query usage; it is a prioritization estimate, not a concrete row-count claim",
		}
		key := estimate.RiskID + "\x00" + estimate.Table
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, estimate)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Score != out[j].Score {
			return out[i].Score > out[j].Score
		}
		if out[i].Table != out[j].Table {
			return out[i].Table < out[j].Table
		}
		return out[i].RiskID < out[j].RiskID
	})
	if len(out) > 200 {
		out = out[:200]
	}
	return out
}

func blastFactsByTable(root string, facts []Fact) map[string][]string {
	out := map[string][]string{}
	for _, fact := range facts {
		if fact.Path == "" {
			continue
		}
		text := fact.Path + "\n" + fact.Rationale + "\n" + fact.Properties["kind"] + "\n" + fact.Properties["table"] + "\n" + fact.Properties["value_preview"]
		if fact.Kind == "file" || fact.Kind == "source_sql_hint" || fact.Kind == "schema_evolution" || fact.Kind == "field_evidence" {
			text += "\n" + textForBoundary(root, fact.Path)
		}
		for _, table := range blastTablesFromText(text, fact.Identifiers) {
			out[table] = append(out[table], fact.Path)
		}
	}
	for table, paths := range out {
		out[table] = uniqueSortedStrings(paths)
	}
	return out
}

func blastQueryPathsByTable(root string, facts []Fact) map[string][]string {
	out := map[string][]string{}
	for _, fact := range facts {
		if fact.Path == "" {
			continue
		}
		text := fact.Path + "\n" + fact.Rationale + "\n" + fact.Properties["value_preview"]
		if fact.Kind == "file" || fact.Kind == "source_sql_hint" || fact.Kind == "field_evidence" || fact.Kind == "schema_evolution" {
			text += "\n" + textForBoundary(root, fact.Path)
		}
		if !containsAny(strings.ToLower(text), "select ", " join ", " update ", " delete from ", " insert into ", " from ") {
			continue
		}
		for _, match := range blastQueryTablePat.FindAllStringSubmatch(text, -1) {
			if len(match) > 1 {
				table := normalizeBlastTable(match[1])
				if table != "" {
					out[table] = append(out[table], fact.Path)
				}
			}
		}
		for _, id := range fact.Identifiers {
			if id.Kind == "table" {
				table := normalizeBlastTable(id.Value)
				if table != "" {
					out[table] = append(out[table], fact.Path)
				}
			}
		}
	}
	for table, paths := range out {
		out[table] = uniqueSortedStrings(paths)
	}
	return out
}

func blastForeignKeyGraph(root string, facts []Fact) map[string][]string {
	graph := map[string][]string{}
	for _, fact := range facts {
		if fact.Path == "" {
			continue
		}
		text := textForBoundary(root, fact.Path)
		if text == "" || !containsAny(strings.ToLower(text), "foreign key", "references") {
			continue
		}
		currentTable := ""
		if table := normalizeBlastTable(fact.Properties["table"]); table != "" {
			currentTable = table
		}
		if currentTable == "" {
			for _, match := range blastCreateTablePat.FindAllStringSubmatch(text, -1) {
				if len(match) > 1 {
					currentTable = normalizeBlastTable(match[1])
					break
				}
			}
		}
		for _, match := range blastFKAlterPattern.FindAllStringSubmatch(text, -1) {
			if len(match) > 2 {
				blastAddUndirectedEdge(graph, normalizeBlastTable(match[1]), normalizeBlastTable(match[2]))
			}
		}
		if currentTable != "" {
			for _, match := range blastFKInlinePattern.FindAllStringSubmatch(text, -1) {
				if len(match) > 1 {
					blastAddUndirectedEdge(graph, currentTable, normalizeBlastTable(match[1]))
				}
			}
		}
	}
	for table, related := range graph {
		graph[table] = uniqueSortedStrings(related)
	}
	return graph
}

func blastAddUndirectedEdge(graph map[string][]string, left, right string) {
	if left == "" || right == "" || left == right {
		return
	}
	graph[left] = append(graph[left], right)
	graph[right] = append(graph[right], left)
}

func blastReachableTables(table string, graph map[string][]string, depth int) []string {
	table = normalizeBlastTable(table)
	if table == "" || depth <= 0 {
		return nil
	}
	seen := map[string]bool{table: true}
	frontier := []string{table}
	var out []string
	for level := 0; level < depth && len(frontier) > 0; level++ {
		var next []string
		for _, current := range frontier {
			for _, related := range graph[current] {
				if seen[related] {
					continue
				}
				seen[related] = true
				out = append(out, related)
				next = append(next, related)
			}
		}
		frontier = next
	}
	return capStrings(uniqueSortedStrings(out), 20)
}

func blastTablesFromText(text string, ids []Identifier) []string {
	var tables []string
	for _, id := range ids {
		if id.Kind == "table" || id.Kind == "model" {
			table := normalizeBlastTable(id.Value)
			if table != "" {
				tables = append(tables, table)
			}
		}
	}
	for _, match := range identifierSQLTablePattern.FindAllStringSubmatch(text, -1) {
		if len(match) > 1 {
			table := normalizeBlastTable(match[1])
			if table != "" {
				tables = append(tables, table)
			}
		}
	}
	for _, match := range blastQueryTablePat.FindAllStringSubmatch(text, -1) {
		if len(match) > 1 {
			table := normalizeBlastTable(match[1])
			if table != "" {
				tables = append(tables, table)
			}
		}
	}
	return uniqueSortedStrings(tables)
}

func blastSourcePathsForRisk(table string, risk BaselineRisk, slice ProvenanceSlice, tableFacts map[string][]string) []string {
	var paths []string
	if risk.Path != "" {
		paths = append(paths, risk.Path)
	}
	paths = append(paths, slice.SourcePaths...)
	paths = append(paths, tableFacts[table]...)
	var codePaths []string
	for _, path := range uniqueSortedStrings(paths) {
		if hasCodeLikeExtension(path) || isMigrationLikePath(path) {
			codePaths = append(codePaths, path)
		}
	}
	return capStrings(uniqueSortedStrings(codePaths), 20)
}

func blastRadiusScore(risk BaselineRisk, centrality, reachability, fanout, usage int) int {
	score := 10
	switch risk.Severity {
	case "critical":
		score += 35
	case "high":
		score += 25
	case "medium":
		score += 15
	default:
		score += 5
	}
	score += minInt(centrality*3, 24)
	score += minInt(reachability*8, 24)
	score += minInt(fanout*4, 20)
	score += minInt(usage*3, 18)
	if riskHasFactor(risk, "broad-write") || riskHasFactor(risk, "destructive-effect") || riskHasFactor(risk, "destructive-code-path") {
		score += 10
	}
	if score > 100 {
		return 100
	}
	return score
}

func blastRadiusLevel(score int) string {
	switch {
	case score >= 70:
		return "high"
	case score >= 40:
		return "medium"
	default:
		return "low"
	}
}

func blastRadiusEvidence(risk BaselineRisk, slice ProvenanceSlice, affected, sourcePaths, queryPaths []string) []string {
	var evidence []string
	if risk.Path != "" {
		evidence = append(evidence, "risk path: "+risk.Path)
	}
	if slice.ID != "" {
		evidence = append(evidence, "provenance slice: "+slice.ID)
	}
	if len(affected) > 0 {
		evidence = append(evidence, "foreign-key reachable tables: "+strings.Join(affected, ", "))
	}
	if len(sourcePaths) > 0 {
		evidence = append(evidence, fmt.Sprintf("code/source paths: %d", len(sourcePaths)))
	}
	if len(queryPaths) > 0 {
		evidence = append(evidence, fmt.Sprintf("query usage paths: %d", len(queryPaths)))
	}
	return capStrings(uniqueSortedStrings(evidence), 10)
}

func normalizeBlastTable(value string) string {
	value = normalizeProjectIdentifierValue("table", value)
	value = strings.Trim(value, "`")
	value = strings.Trim(value, `"'[]`)
	if strings.Contains(value, ".") {
		parts := strings.Split(value, ".")
		value = parts[len(parts)-1]
	}
	if isSQLIdentifierStopword(value) {
		return ""
	}
	return value
}

func countBlastRadiusLevel(estimates []BlastRadiusEstimate, level string) int {
	var count int
	for _, estimate := range estimates {
		if estimate.Level == level {
			count++
		}
	}
	return count
}

func buildPolicyChecks(risks []BaselineRisk, slices []ProvenanceSlice, symbolic []SymbolicCheck) []PolicyCheck {
	sliceByRisk := map[string]ProvenanceSlice{}
	for _, slice := range slices {
		sliceByRisk[slice.RiskID] = slice
	}
	symbolicByRisk := map[string][]SymbolicCheck{}
	for _, check := range symbolic {
		symbolicByRisk[check.RiskID] = append(symbolicByRisk[check.RiskID], check)
	}
	var checks []PolicyCheck
	for _, risk := range risks {
		riskClass, required := policyRequirementForRisk(risk, symbolicByRisk[risk.ID])
		if len(required) == 0 {
			continue
		}
		slice := sliceByRisk[risk.ID]
		satisfied := satisfiedPolicyObligations(risk, slice, symbolicByRisk[risk.ID])
		missing := missingStrings(required, satisfied)
		status := "pass"
		if len(missing) > 0 {
			status = "warn"
		}
		if policyMissingCritical(missing) {
			status = "fail"
		}
		checks = append(checks, PolicyCheck{
			ID:          "policy:" + canonical.Hash(risk.ID + "\x00" + riskClass)[:16],
			RiskID:      risk.ID,
			Policy:      "guard-rollback-approval-dryrun-test",
			Status:      status,
			RiskClass:   riskClass,
			Required:    required,
			Satisfied:   intersectStrings(required, satisfied),
			Missing:     missing,
			Evidence:    policyEvidence(slice, symbolicByRisk[risk.ID]),
			ReviewLevel: policyReviewLevel(riskClass, status),
			Rationale:   "selected risk classes must carry explicit guard, rollback, approval, dry-run, or test evidence before generated or manual repairs are trusted",
		})
	}
	sort.Slice(checks, func(i, j int) bool {
		if checks[i].Status != checks[j].Status {
			return policyStatusRank(checks[i].Status) > policyStatusRank(checks[j].Status)
		}
		if len(checks[i].Missing) != len(checks[j].Missing) {
			return len(checks[i].Missing) > len(checks[j].Missing)
		}
		return checks[i].ID < checks[j].ID
	})
	if len(checks) > 200 {
		checks = checks[:200]
	}
	return checks
}

func policyRequirementForRisk(risk BaselineRisk, symbolic []SymbolicCheck) (string, []string) {
	kind := strings.ToLower(risk.Kind)
	hasSymbolicFail := false
	for _, check := range symbolic {
		if check.Status == "fail" {
			hasSymbolicFail = true
			break
		}
	}
	switch {
	case risk.Severity == "high" || hasSymbolicFail || riskHasFactor(risk, "destructive-effect") || riskHasFactor(risk, "destructive-code-path") || strings.Contains(kind, "delete") || strings.Contains(kind, "drop") || strings.Contains(kind, "truncate"):
		return "destructive-or-unproven", []string{"guard", "rollback", "approval", "dry-run", "test"}
	case riskHasFactor(risk, "broad-write") || riskHasFactor(risk, "write-breadth-unknown"):
		return "broad-write", []string{"guard", "dry-run", "test"}
	case strings.HasPrefix(kind, "code-path:"):
		return "persistent-code-path", []string{"test", "approval"}
	default:
		return "", nil
	}
}

func satisfiedPolicyObligations(risk BaselineRisk, slice ProvenanceSlice, symbolic []SymbolicCheck) []string {
	var out []string
	if !riskHasFactor(risk, "broad-write") && !riskHasFactor(risk, "write-breadth-unknown") && !riskHasFactor(risk, "source-window-unavailable") {
		out = append(out, "guard")
	}
	if len(slice.RepairPaths) > 0 || !riskHasFactor(risk, "weak-rollback-signal") {
		out = append(out, "rollback")
	}
	if len(slice.TestCommands) > 0 || len(slice.NativeCommands) > 0 {
		out = append(out, "test")
	}
	for _, check := range symbolic {
		if check.Property == "scope_preservation" && check.Status == "pass" {
			out = append(out, "guard")
		}
		if check.Property == "reversibility" && check.Status == "pass" {
			out = append(out, "rollback")
		}
	}
	if risk.NextCommand != "" || slice.MigrationPath != "" {
		out = append(out, "dry-run")
	}
	return uniqueSortedStrings(out)
}

func missingStrings(required, satisfied []string) []string {
	sat := map[string]bool{}
	for _, value := range satisfied {
		sat[value] = true
	}
	var missing []string
	for _, value := range required {
		if !sat[value] {
			missing = append(missing, value)
		}
	}
	return uniqueSortedStrings(missing)
}

func intersectStrings(required, satisfied []string) []string {
	req := map[string]bool{}
	for _, value := range required {
		req[value] = true
	}
	var out []string
	for _, value := range satisfied {
		if req[value] {
			out = append(out, value)
		}
	}
	return uniqueSortedStrings(out)
}

func policyMissingCritical(missing []string) bool {
	for _, value := range missing {
		if value == "guard" || value == "rollback" || value == "approval" {
			return true
		}
	}
	return false
}

func policyEvidence(slice ProvenanceSlice, symbolic []SymbolicCheck) []string {
	evidence := provenanceEvidence(slice)
	for _, check := range symbolic {
		evidence = append(evidence, "symbolic:"+check.Property+":"+check.Status)
	}
	return capStrings(uniqueSortedStrings(evidence), 12)
}

func policyReviewLevel(riskClass, status string) string {
	if status == "fail" || riskClass == "destructive-or-unproven" {
		return "manual-approval-required"
	}
	if status == "warn" {
		return "maintainer-review"
	}
	return "standard-review"
}

func policyStatusRank(status string) int {
	switch status {
	case "fail":
		return 3
	case "warn":
		return 2
	case "pass":
		return 1
	default:
		return 0
	}
}

func countPolicyStatus(checks []PolicyCheck, status string) int {
	var count int
	for _, check := range checks {
		if check.Status == status {
			count++
		}
	}
	return count
}

func buildRepairProofSummaries(risks []BaselineRisk, slices []ProvenanceSlice, summaries []effects.AbstractSummary, symbolic []SymbolicCheck) []RepairProofSummary {
	riskByID := map[string]BaselineRisk{}
	for _, risk := range risks {
		riskByID[risk.ID] = risk
	}
	sliceByRisk := map[string]ProvenanceSlice{}
	for _, slice := range slices {
		sliceByRisk[slice.RiskID] = slice
	}
	checksByRisk := map[string]map[string]SymbolicCheck{}
	for _, check := range symbolic {
		if check.Property != "scope_preservation" && check.Property != "frame_condition" {
			continue
		}
		if checksByRisk[check.RiskID] == nil {
			checksByRisk[check.RiskID] = map[string]SymbolicCheck{}
		}
		checksByRisk[check.RiskID][check.Property] = check
	}
	var proofs []RepairProofSummary
	for _, summary := range summaries {
		for _, op := range summary.Operations {
			checks := checksByRisk[op.OperationID]
			scope, hasScope := checks["scope_preservation"]
			frame, hasFrame := checks["frame_condition"]
			if !hasScope || !hasFrame || firstNonEmpty(op.Table, scope.Table, frame.Table) == "" {
				continue
			}
			risk := riskByID[op.OperationID]
			slice := sliceByRisk[op.OperationID]
			holes := repairProofHoles(op, summary)
			status := repairProofStatus(scope.Status, frame.Status, holes)
			repairPaths := capStrings(uniqueSortedStrings(slice.RepairPaths), 8)
			source := "candidate-obligation"
			if len(repairPaths) > 0 {
				source = "repo-evidence"
			}
			proofs = append(proofs, RepairProofSummary{
				ID:           "repair-proof:" + canonical.Hash(op.OperationID + "\x00" + scope.ID + "\x00" + frame.ID)[:16],
				RiskID:       op.OperationID,
				RepairSource: source,
				RepairPaths:  repairPaths,
				Table:        firstNonEmpty(op.Table, scope.Table, frame.Table, risk.Table),
				Status:       status,
				ScopeStatus:  scope.Status,
				FrameStatus:  frame.Status,
				Obligations:  repairProofObligations(scope, frame),
				ProofHoles:   holes,
				Evidence:     capStrings(uniqueSortedStrings(append(provenanceEvidence(slice), append(scope.Evidence, frame.Evidence...)...)), 12),
				NextCommand:  risk.NextCommand,
				Rationale:    "summary records the scope and frame obligations a repair must preserve; conditional and open statuses retain explicit proof holes instead of claiming full proof",
			})
		}
	}
	sort.Slice(proofs, func(i, j int) bool {
		if proofs[i].Status != proofs[j].Status {
			return repairProofStatusRank(proofs[i].Status) > repairProofStatusRank(proofs[j].Status)
		}
		if len(proofs[i].ProofHoles) != len(proofs[j].ProofHoles) {
			return len(proofs[i].ProofHoles) > len(proofs[j].ProofHoles)
		}
		return proofs[i].ID < proofs[j].ID
	})
	if len(proofs) > 200 {
		proofs = proofs[:200]
	}
	return proofs
}

func repairProofHoles(op effects.AbstractOperation, summary effects.AbstractSummary) []string {
	holes := append([]string(nil), op.ProofHoles...)
	for _, unsupported := range summary.Concretization.UnsupportedFacts {
		if strings.Contains(unsupported, op.OperationID) || strings.Contains(unsupported, op.Table) {
			holes = append(holes, unsupported)
		}
	}
	if len(holes) == 0 && !op.BoundedRows {
		holes = append(holes, "row bound unavailable")
	}
	return capStrings(uniqueSortedStrings(holes), 12)
}

func repairProofStatus(scopeStatus, frameStatus string, holes []string) string {
	if scopeStatus == "fail" || frameStatus == "fail" {
		return "refuted"
	}
	if scopeStatus == "warn" || frameStatus == "warn" {
		return "open"
	}
	if len(holes) > 0 {
		return "conditional"
	}
	return "checked"
}

func repairProofObligations(scope, frame SymbolicCheck) []string {
	return uniqueSortedStrings([]string{
		"scope_preservation:" + scope.Status + ":" + scope.Expression,
		"frame_condition:" + frame.Status + ":" + frame.Expression,
	})
}

func repairProofStatusRank(status string) int {
	switch status {
	case "refuted":
		return 4
	case "open":
		return 3
	case "conditional":
		return 2
	case "checked":
		return 1
	default:
		return 0
	}
}

func countRepairProofStatus(proofs []RepairProofSummary, status string) int {
	var count int
	for _, proof := range proofs {
		if proof.Status == status {
			count++
		}
	}
	return count
}

func buildProofHoleMinimizations(inventoryRoot string, risks []BaselineRisk, slices []ProvenanceSlice, symbolic []SymbolicCheck, policies []PolicyCheck, proofs []RepairProofSummary, summaries []effects.AbstractSummary, facts []Fact, intakeReport intake.Report) []ProofHoleMinimization {
	root := firstNonEmpty(inventoryRoot, intakeReport.Source.ScannedRoot, intakeReport.Source.Input)
	riskByID := map[string]BaselineRisk{}
	for _, risk := range risks {
		riskByID[risk.ID] = risk
	}
	sliceByRisk := map[string]ProvenanceSlice{}
	for _, slice := range slices {
		sliceByRisk[slice.RiskID] = slice
	}
	factPaths := proofCandidatePathsByKind(root, facts)
	seen := map[string]bool{}
	var out []ProofHoleMinimization
	add := func(item ProofHoleMinimization) {
		if item.RiskID == "" || item.Hole == "" || item.MissingEvidence == "" {
			return
		}
		key := item.RiskID + "\x00" + item.Source + "\x00" + item.Hole + "\x00" + item.MissingEvidence
		if seen[key] {
			return
		}
		seen[key] = true
		item.ID = "proof-min:" + canonical.Hash(key)[:16]
		item.Priority = proofMinimizationPriority(item, riskByID[item.RiskID])
		if item.UpgradeTo == "" {
			item.UpgradeTo = "checked"
		}
		if item.Rationale == "" {
			item.Rationale = "ranks the smallest concrete evidence artifact likely to upgrade an open proof obligation into a checked claim without treating absent evidence as proof"
		}
		out = append(out, item)
	}
	for _, check := range policies {
		if check.Status == "pass" {
			continue
		}
		risk := riskByID[check.RiskID]
		slice := sliceByRisk[check.RiskID]
		for _, missing := range check.Missing {
			spec := proofEvidenceSpec(missing)
			add(ProofHoleMinimization{
				RiskID:           check.RiskID,
				Table:            firstNonEmpty(risk.Table, slice.Table),
				Source:           "policy",
				Hole:             "policy missing: " + missing,
				MissingEvidence:  spec.Kind,
				UpgradeFrom:      check.Status,
				UpgradeTo:        "pass",
				MinimalArtifacts: spec.Artifacts,
				CandidatePaths:   proofCandidatePaths(spec.Kind, slice, factPaths),
				Effort:           spec.Effort,
				Evidence:         capStrings(uniqueSortedStrings(append(check.Evidence, "review:"+check.ReviewLevel)), 10),
				Identifiers:      proofMinimizationIdentifiers(risk, slice),
			})
		}
	}
	for _, check := range symbolic {
		if check.Status == "pass" {
			continue
		}
		risk := riskByID[check.RiskID]
		slice := sliceByRisk[check.RiskID]
		spec := proofEvidenceSpec(check.Property + ":" + check.Status + ":" + check.Reason)
		add(ProofHoleMinimization{
			RiskID:           check.RiskID,
			Table:            firstNonEmpty(check.Table, risk.Table, slice.Table),
			Source:           "symbolic",
			Hole:             check.Property + ": " + check.Reason,
			MissingEvidence:  spec.Kind,
			UpgradeFrom:      check.Status,
			UpgradeTo:        "pass",
			MinimalArtifacts: spec.Artifacts,
			CandidatePaths:   proofCandidatePaths(spec.Kind, slice, factPaths),
			Effort:           spec.Effort,
			Evidence:         capStrings(uniqueSortedStrings(check.Evidence), 10),
			Identifiers:      proofMinimizationIdentifiers(risk, slice),
		})
	}
	for _, proof := range proofs {
		if proof.Status == "checked" {
			continue
		}
		risk := riskByID[proof.RiskID]
		slice := sliceByRisk[proof.RiskID]
		if len(proof.ProofHoles) == 0 {
			spec := proofEvidenceSpec(proof.Status)
			add(ProofHoleMinimization{
				RiskID:           proof.RiskID,
				Table:            firstNonEmpty(proof.Table, risk.Table, slice.Table),
				Source:           "repair_proof",
				Hole:             "repair proof status: " + proof.Status,
				MissingEvidence:  spec.Kind,
				UpgradeFrom:      proof.Status,
				UpgradeTo:        "checked",
				MinimalArtifacts: spec.Artifacts,
				CandidatePaths:   proofCandidatePaths(spec.Kind, slice, factPaths),
				Effort:           spec.Effort,
				Evidence:         proof.Evidence,
				Identifiers:      proofMinimizationIdentifiers(risk, slice),
			})
			continue
		}
		for _, hole := range proof.ProofHoles {
			spec := proofEvidenceSpec(hole)
			add(ProofHoleMinimization{
				RiskID:           proof.RiskID,
				Table:            firstNonEmpty(proof.Table, risk.Table, slice.Table),
				Source:           "repair_proof",
				Hole:             hole,
				MissingEvidence:  spec.Kind,
				UpgradeFrom:      proof.Status,
				UpgradeTo:        "checked",
				MinimalArtifacts: spec.Artifacts,
				CandidatePaths:   proofCandidatePaths(spec.Kind, slice, factPaths),
				Effort:           spec.Effort,
				Evidence:         proof.Evidence,
				Identifiers:      proofMinimizationIdentifiers(risk, slice),
			})
		}
	}
	for _, summary := range summaries {
		for _, hole := range summary.Concretization.UnsupportedFacts {
			riskID := proofRiskIDFromHole(hole, risks)
			if riskID == "" {
				continue
			}
			risk := riskByID[riskID]
			slice := sliceByRisk[riskID]
			spec := proofEvidenceSpec(hole)
			add(ProofHoleMinimization{
				RiskID:           riskID,
				Table:            firstNonEmpty(risk.Table, slice.Table),
				Source:           "abstract_effect",
				Hole:             hole,
				MissingEvidence:  spec.Kind,
				UpgradeFrom:      "unsupported",
				UpgradeTo:        "checked",
				MinimalArtifacts: spec.Artifacts,
				CandidatePaths:   proofCandidatePaths(spec.Kind, slice, factPaths),
				Effort:           spec.Effort,
				Evidence:         []string{"abstract summary: " + summary.Hash, "join: " + string(summary.Join)},
				Identifiers:      proofMinimizationIdentifiers(risk, slice),
			})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Effort != out[j].Effort {
			return out[i].Effort < out[j].Effort
		}
		if out[i].Priority != out[j].Priority {
			return proofMinPriorityRank(out[i].Priority) > proofMinPriorityRank(out[j].Priority)
		}
		if out[i].RiskID != out[j].RiskID {
			return out[i].RiskID < out[j].RiskID
		}
		return out[i].Hole < out[j].Hole
	})
	if len(out) > 250 {
		out = out[:250]
	}
	return out
}

type proofEvidenceRequirement struct {
	Kind      string
	Artifacts []string
	Effort    int
}

func proofEvidenceSpec(hole string) proofEvidenceRequirement {
	lower := strings.ToLower(hole)
	switch {
	case containsAny(lower, "approval"):
		return proofEvidenceRequirement{"approval-record", []string{"approval comment or change ticket referencing the risk/table"}, 1}
	case containsAny(lower, "dry-run", "dry run"):
		return proofEvidenceRequirement{"dry-run-result", []string{"captured dry-run command", "row-count or generated SQL preview output"}, 1}
	case containsAny(lower, "test", "frame_condition", "changed-column", "changed column", "frame"):
		return proofEvidenceRequirement{"focused-test-or-frame-witness", []string{"focused test command covering the changed table/path", "changed-column list or frame assertion"}, 2}
	case containsAny(lower, "guard", "scope", "bounded", "row bound", "row count", "concrete row count", "scope_preservation"):
		return proofEvidenceRequirement{"scope-bound", []string{"WHERE predicate or tenant/key guard", "SELECT COUNT(*) dry-run bound for affected rows"}, 2}
	case containsAny(lower, "rollback", "reversibility", "inverse", "snapshot", "repair"):
		return proofEvidenceRequirement{"rollback-witness", []string{"rollback SQL or compensating repair path", "snapshot/backup evidence before mutation"}, 3}
	case containsAny(lower, "compensating", "append-only"):
		return proofEvidenceRequirement{"compensating-action", []string{"operator-supplied compensating action for appended event/log/queue records"}, 3}
	case containsAny(lower, "transfer function", "external operation", "system-specific", "unknown"):
		return proofEvidenceRequirement{"transfer-function-witness", []string{"project-specific transfer description", "replay/rebuild invariant or contract test"}, 4}
	default:
		return proofEvidenceRequirement{"supporting-evidence", []string{"smallest repo-local artifact that proves the missing obligation"}, 3}
	}
}

func proofCandidatePathsByKind(root string, facts []Fact) map[string][]string {
	out := map[string][]string{}
	for _, fact := range facts {
		if fact.Path == "" {
			continue
		}
		text := strings.ToLower(fact.Kind + " " + fact.Path + " " + fact.Rationale + " " + fact.Properties["kind"] + " " + fact.Properties["field"] + " " + fact.Properties["value_preview"])
		if fact.Kind == "file" || fact.Kind == "source_sql_hint" || fact.Kind == "schema_evolution" || fact.Kind == "field_evidence" {
			text += "\n" + strings.ToLower(textForBoundary(root, fact.Path))
		}
		addPath := func(kind string) {
			out[kind] = append(out[kind], fact.Path)
		}
		if containsAny(text, "approval", "approved", "reviewed", "ticket", "jira", "linear", "issue") {
			addPath("approval-record")
		}
		if containsAny(text, "dry-run", "dry run", "explain", "select count", "count(*)", "candidate_rows") {
			addPath("dry-run-result")
		}
		if containsAny(text, "where ", "tenant", "account_id", "user_id", "limit ", "scope", "bounded") {
			addPath("scope-bound")
		}
		if containsAny(text, "rollback", "restore", "snapshot", "backup", "revert", "down do", "def down") {
			addPath("rollback-witness")
		}
		if containsAny(text, "test", "spec", "assert", "expect(", "pytest", "go test") || isTestPath(strings.ToLower(fact.Path)) {
			addPath("focused-test-or-frame-witness")
		}
		if containsAny(text, "compensat", "dead letter", "dlq", "replay", "queue", "event") {
			addPath("compensating-action")
		}
		if containsAny(text, "transfer", "rebuild", "reconcile", "replay", "invariant", "contract") {
			addPath("transfer-function-witness")
		}
	}
	for kind, paths := range out {
		out[kind] = capStrings(uniqueSortedStrings(paths), 20)
	}
	return out
}

func proofCandidatePaths(kind string, slice ProvenanceSlice, factPaths map[string][]string) []string {
	var paths []string
	paths = append(paths, factPaths[kind]...)
	switch kind {
	case "rollback-witness", "compensating-action":
		paths = append(paths, slice.RepairPaths...)
	case "focused-test-or-frame-witness":
		for _, command := range append(slice.TestCommands, slice.NativeCommands...) {
			if command.Command != "" {
				paths = append(paths, "command:"+command.Command)
			}
		}
		paths = append(paths, slice.SourcePaths...)
	case "scope-bound", "dry-run-result", "transfer-function-witness":
		paths = append(paths, slice.MigrationPath)
		paths = append(paths, slice.SourcePaths...)
	case "approval-record":
		paths = append(paths, slice.IncidentPaths...)
	}
	return capStrings(uniqueSortedStrings(paths), 12)
}

func proofRiskIDFromHole(hole string, risks []BaselineRisk) string {
	lower := strings.ToLower(hole)
	for _, risk := range risks {
		if strings.Contains(hole, risk.ID) || (risk.Table != "" && strings.Contains(lower, strings.ToLower(risk.Table))) {
			return risk.ID
		}
	}
	return ""
}

func proofMinimizationIdentifiers(risk BaselineRisk, slice ProvenanceSlice) []Identifier {
	ids := append([]Identifier{}, risk.Identifiers...)
	ids = append(ids, slice.Identifiers...)
	if risk.Table != "" {
		ids = append(ids, Identifier{Kind: "table", Value: risk.Table})
	}
	if slice.Table != "" {
		ids = append(ids, Identifier{Kind: "table", Value: slice.Table})
	}
	return uniqueIdentifiers(ids)
}

func proofMinimizationPriority(item ProofHoleMinimization, risk BaselineRisk) string {
	lower := strings.ToLower(item.Hole + " " + item.MissingEvidence)
	switch {
	case risk.Severity == "high" && containsAny(lower, "rollback", "guard", "scope", "approval", "destructive"):
		return "critical"
	case risk.Severity == "high" || containsAny(lower, "rollback", "transfer", "unknown", "compensating"):
		return "high"
	case containsAny(lower, "test", "dry-run", "row count", "scope"):
		return "medium"
	default:
		return "low"
	}
}

func proofMinPriorityRank(priority string) int {
	switch priority {
	case "critical":
		return 4
	case "high":
		return 3
	case "medium":
		return 2
	case "low":
		return 1
	default:
		return 0
	}
}

func countProofMinimizationPriority(items []ProofHoleMinimization, priority string) int {
	var count int
	for _, item := range items {
		if item.Priority == priority {
			count++
		}
	}
	return count
}

func sharedIdentifiers(left, right []Identifier) []Identifier {
	rightSet := map[string]Identifier{}
	for _, id := range right {
		key := canonicalIdentifier(id.Kind, id.Value)
		if key != "" {
			rightSet[key] = id
		}
	}
	var out []Identifier
	for _, id := range left {
		key := canonicalIdentifier(id.Kind, id.Value)
		if _, ok := rightSet[key]; ok {
			out = append(out, Identifier{Kind: id.Kind, Value: normalizeIdentifierValue(id.Value)})
		}
	}
	return uniqueIdentifiers(out)
}

func uniqueLinks(in []EvidenceLink) []EvidenceLink {
	sort.Slice(in, func(i, j int) bool {
		if in[i].RiskID != in[j].RiskID {
			return in[i].RiskID < in[j].RiskID
		}
		if in[i].FromID != in[j].FromID {
			return in[i].FromID < in[j].FromID
		}
		if in[i].FactKind != in[j].FactKind {
			return in[i].FactKind < in[j].FactKind
		}
		if in[i].Path != in[j].Path {
			return in[i].Path < in[j].Path
		}
		return in[i].FactID < in[j].FactID
	})
	seen := map[string]bool{}
	var out []EvidenceLink
	for _, link := range in {
		key := link.RiskID + "\x00" + link.FromID + "\x00" + link.FactID
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, link)
	}
	return out
}

func canonicalIdentifier(kind, value string) string {
	kind = strings.ToLower(strings.TrimSpace(kind))
	value = normalizeIdentifierValue(value)
	if kind == "" || value == "" {
		return ""
	}
	return kind + ":" + value
}

func normalizeIdentifierValue(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.Trim(value, `"'[]`)
	return value
}

func uniqueRisks(in []BaselineRisk) []BaselineRisk {
	seen := map[string]bool{}
	var out []BaselineRisk
	for _, risk := range in {
		if seen[risk.ID] {
			continue
		}
		seen[risk.ID] = true
		sort.Slice(risk.Factors, func(i, j int) bool {
			if risk.Factors[i].Weight != risk.Factors[j].Weight {
				return risk.Factors[i].Weight > risk.Factors[j].Weight
			}
			return risk.Factors[i].Name < risk.Factors[j].Name
		})
		out = append(out, risk)
	}
	return out
}

func countCodePathRisks(risks []BaselineRisk) int {
	var count int
	for _, risk := range risks {
		if strings.HasPrefix(risk.Kind, "code-path:") {
			count++
		}
	}
	return count
}

func sqlOnlyRankedRisks(report intake.Report) int {
	var count int
	for _, finding := range report.SQL {
		if finding.Summary.HighRisk > 0 || finding.Summary.MediumRisk > 0 {
			count++
		}
	}
	return count
}

func grepOnlyMatches(root string) int {
	files, _, err := discoverFiles(root, false)
	if err != nil {
		return 0
	}
	var count int
	for _, file := range files {
		if file.Size > factContentLimit {
			continue
		}
		text, err := readTextPrefix(file.Abs, factContentLimit)
		if err != nil || text == "" {
			continue
		}
		count += len(identifierSQLTablePattern.FindAllString(text, -1))
	}
	return count
}

func countLinksByIdentifierKind(links []EvidenceLink, dateOnly bool) int {
	var count int
	for _, link := range links {
		for _, id := range link.Identifiers {
			if dateOnly && id.Kind == "date" {
				count++
				break
			}
			if !dateOnly && id.Kind != "date" {
				count++
				break
			}
		}
	}
	return count
}

func baselineHash(report BaselineReport) string {
	copy := report
	copy.Hash = ""
	copy.Markdown = ""
	return canonical.Hash(copy)
}

func renderBaselineMarkdown(report BaselineReport) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Patchline repo baseline\n\n")
	fmt.Fprintf(&b, "- inventory_root: `%s`\n", report.InventoryRoot)
	fmt.Fprintf(&b, "- intake_source: `%s`\n", report.IntakeSource)
	fmt.Fprintf(&b, "- hash: `%s`\n\n", report.Hash)
	fmt.Fprintf(&b, "## Summary\n\n| area | count |\n| --- | ---: |\n")
	fmt.Fprintf(&b, "| ranked risks | %d |\n", report.Summary.RankedRisks)
	fmt.Fprintf(&b, "| code-path ranked risks | %d |\n", report.Summary.CodePathRankedRisks)
	fmt.Fprintf(&b, "| ranking explanations | %d |\n", report.Summary.RankingExplanations)
	fmt.Fprintf(&b, "| ranking features | %d |\n", report.Summary.RankingFeatures)
	fmt.Fprintf(&b, "| ablation-sensitive risks | %d |\n", report.Summary.AblationSensitive)
	fmt.Fprintf(&b, "| evidence links | %d |\n", report.Summary.EvidenceLinks)
	fmt.Fprintf(&b, "| cause clusters | %d |\n", report.Summary.CauseClusters)
	fmt.Fprintf(&b, "| repair clusters | %d |\n", report.Summary.RepairClusters)
	fmt.Fprintf(&b, "| provenance slices | %d |\n", report.Summary.ProvenanceSlices)
	fmt.Fprintf(&b, "| datalog-style queries | %d |\n", report.Summary.DatalogQueries)
	fmt.Fprintf(&b, "| datalog-style rows | %d |\n", report.Summary.DatalogRows)
	fmt.Fprintf(&b, "| abstract effects | %d |\n", report.Summary.AbstractEffects)
	fmt.Fprintf(&b, "| abstract operations | %d |\n", report.Summary.AbstractOperations)
	fmt.Fprintf(&b, "| abstract proof holes | %d |\n", report.Summary.AbstractProofHoles)
	fmt.Fprintf(&b, "| symbolic checks | %d |\n", report.Summary.SymbolicChecks)
	fmt.Fprintf(&b, "| symbolic passed | %d |\n", report.Summary.SymbolicPassed)
	fmt.Fprintf(&b, "| symbolic warnings | %d |\n", report.Summary.SymbolicWarnings)
	fmt.Fprintf(&b, "| symbolic failed | %d |\n", report.Summary.SymbolicFailed)
	fmt.Fprintf(&b, "| temporal windows | %d |\n", report.Summary.TemporalWindows)
	fmt.Fprintf(&b, "| temporal signals | %d |\n", report.Summary.TemporalSignals)
	fmt.Fprintf(&b, "| recurrence patterns | %d |\n", report.Summary.Recurrences)
	fmt.Fprintf(&b, "| recurring risks | %d |\n", report.Summary.RecurringRisks)
	fmt.Fprintf(&b, "| transaction boundaries | %d |\n", report.Summary.Transactions)
	fmt.Fprintf(&b, "| transaction explicit | %d |\n", report.Summary.TransactionExplicit)
	fmt.Fprintf(&b, "| transaction partial | %d |\n", report.Summary.TransactionPartial)
	fmt.Fprintf(&b, "| transaction missing | %d |\n", report.Summary.TransactionMissing)
	fmt.Fprintf(&b, "| idempotency classifications | %d |\n", report.Summary.IdempotencyClasses)
	fmt.Fprintf(&b, "| idempotency proven | %d |\n", report.Summary.IdempotencyProven)
	fmt.Fprintf(&b, "| idempotency guarded | %d |\n", report.Summary.IdempotencyGuarded)
	fmt.Fprintf(&b, "| idempotency unknown | %d |\n", report.Summary.IdempotencyUnknown)
	fmt.Fprintf(&b, "| idempotency non-idempotent | %d |\n", report.Summary.IdempotencyUnsafe)
	fmt.Fprintf(&b, "| lock/concurrency hazards | %d |\n", report.Summary.LockHazards)
	fmt.Fprintf(&b, "| lock hazard critical | %d |\n", report.Summary.LockHazardCritical)
	fmt.Fprintf(&b, "| lock hazard high | %d |\n", report.Summary.LockHazardHigh)
	fmt.Fprintf(&b, "| lock hazard medium | %d |\n", report.Summary.LockHazardMedium)
	fmt.Fprintf(&b, "| lock hazard low | %d |\n", report.Summary.LockHazardLow)
	fmt.Fprintf(&b, "| data-retention/privacy hazards | %d |\n", report.Summary.PrivacyHazards)
	fmt.Fprintf(&b, "| privacy hazard critical | %d |\n", report.Summary.PrivacyCritical)
	fmt.Fprintf(&b, "| privacy hazard high | %d |\n", report.Summary.PrivacyHigh)
	fmt.Fprintf(&b, "| privacy hazard medium | %d |\n", report.Summary.PrivacyMedium)
	fmt.Fprintf(&b, "| privacy hazard low | %d |\n", report.Summary.PrivacyLow)
	fmt.Fprintf(&b, "| invariant candidates | %d |\n", report.Summary.Invariants)
	fmt.Fprintf(&b, "| invariants from schema | %d |\n", report.Summary.InvariantSchema)
	fmt.Fprintf(&b, "| invariants from tests | %d |\n", report.Summary.InvariantTests)
	fmt.Fprintf(&b, "| invariants from validations | %d |\n", report.Summary.InvariantValidation)
	fmt.Fprintf(&b, "| invariants from fixtures | %d |\n", report.Summary.InvariantFixtures)
	fmt.Fprintf(&b, "| trace-to-code links | %d |\n", report.Summary.TraceCodeLinks)
	fmt.Fprintf(&b, "| trace links exact | %d |\n", report.Summary.TraceLinkExact)
	fmt.Fprintf(&b, "| trace links causal | %d |\n", report.Summary.TraceLinkCausal)
	fmt.Fprintf(&b, "| trace links temporal | %d |\n", report.Summary.TraceLinkTemporal)
	fmt.Fprintf(&b, "| trace links inferred | %d |\n", report.Summary.TraceLinkInferred)
	fmt.Fprintf(&b, "| blast-radius estimates | %d |\n", report.Summary.BlastRadius)
	fmt.Fprintf(&b, "| blast radius high | %d |\n", report.Summary.BlastRadiusHigh)
	fmt.Fprintf(&b, "| blast radius medium | %d |\n", report.Summary.BlastRadiusMedium)
	fmt.Fprintf(&b, "| blast radius low | %d |\n", report.Summary.BlastRadiusLow)
	fmt.Fprintf(&b, "| proof-hole minimizations | %d |\n", report.Summary.ProofMinimizations)
	fmt.Fprintf(&b, "| proof minimizations critical | %d |\n", report.Summary.ProofMinCritical)
	fmt.Fprintf(&b, "| proof minimizations high | %d |\n", report.Summary.ProofMinHigh)
	fmt.Fprintf(&b, "| proof minimizations medium | %d |\n", report.Summary.ProofMinMedium)
	fmt.Fprintf(&b, "| proof minimizations low | %d |\n", report.Summary.ProofMinLow)
	fmt.Fprintf(&b, "| policy checks | %d |\n", report.Summary.PolicyChecks)
	fmt.Fprintf(&b, "| policy passed | %d |\n", report.Summary.PolicyPassed)
	fmt.Fprintf(&b, "| policy warnings | %d |\n", report.Summary.PolicyWarnings)
	fmt.Fprintf(&b, "| policy failed | %d |\n", report.Summary.PolicyFailed)
	fmt.Fprintf(&b, "| repair proof summaries | %d |\n", report.Summary.RepairProofs)
	fmt.Fprintf(&b, "| repair proof checked | %d |\n", report.Summary.RepairProofChecked)
	fmt.Fprintf(&b, "| repair proof conditional | %d |\n", report.Summary.RepairProofCond)
	fmt.Fprintf(&b, "| repair proof open | %d |\n", report.Summary.RepairProofOpen)
	fmt.Fprintf(&b, "| repair proof refuted | %d |\n", report.Summary.RepairProofRefuted)
	fmt.Fprintf(&b, "| grep-only matches | %d |\n", report.Summary.GrepOnlyMatches)
	fmt.Fprintf(&b, "| SQL-only ranked risks | %d |\n", report.Summary.SQLOnlyRankedRisks)
	fmt.Fprintf(&b, "| identifier-only links | %d |\n", report.Summary.IdentifierOnlyLinks)
	fmt.Fprintf(&b, "| date-only links | %d |\n\n", report.Summary.DateOnlyLinks)
	if len(report.Risks) > 0 {
		fmt.Fprintf(&b, "## Top risks\n\n| stable id | score | severity | path | kind | table | rationale |\n| --- | ---: | --- | --- | --- | --- | --- |\n")
		limit := minInt(len(report.Risks), 25)
		for _, risk := range report.Risks[:limit] {
			fmt.Fprintf(&b, "| `%s` | %d | %s | %s | %s | %s | %s |\n", risk.StableID, risk.Score, risk.Severity, risk.Path, risk.Kind, risk.Table, risk.Rationale)
		}
		fmt.Fprintf(&b, "\n")
	}
	if len(report.Rankings) > 0 {
		fmt.Fprintf(&b, "## Ranking explanations\n\n| risk | score | severity | top feature | features | severity-changing ablations |\n| --- | ---: | --- | --- | ---: | ---: |\n")
		limit := minInt(len(report.Rankings), 25)
		for _, explanation := range report.Rankings[:limit] {
			fmt.Fprintf(&b, "| `%s` | %d | %s | %s | %d | %d |\n", explanation.RiskID, explanation.Score, explanation.Severity, explanation.TopFeature, len(explanation.Contributions), countExplanationSeverityChanges(explanation))
		}
		fmt.Fprintf(&b, "\n")
	}
	if len(report.Provenance) > 0 {
		fmt.Fprintf(&b, "## Provenance slices\n\n| risk | table | stages | migration | source paths | incidents | repairs |\n| --- | --- | --- | --- | ---: | ---: | ---: |\n")
		limit := minInt(len(report.Provenance), 20)
		for _, slice := range report.Provenance[:limit] {
			fmt.Fprintf(&b, "| `%s` | %s | %s | %s | %d | %d | %d |\n", slice.RiskID, slice.Table, strings.Join(slice.StagesPresent, ", "), slice.MigrationPath, len(slice.SourcePaths), len(slice.IncidentPaths), len(slice.RepairPaths))
		}
		fmt.Fprintf(&b, "\n")
	}
	if len(report.DatalogQueries) > 0 {
		fmt.Fprintf(&b, "## Datalog-style query results\n\n| query | rows | rule |\n| --- | ---: | --- |\n")
		for _, query := range report.DatalogQueries {
			fmt.Fprintf(&b, "| %s | %d | `%s` |\n", query.Name, len(query.Rows), query.Rule)
		}
		fmt.Fprintf(&b, "\n")
	}
	if len(report.AbstractEffects) > 0 {
		fmt.Fprintf(&b, "## Abstract effects\n\n| join | operations | proof holes | manifest |\n| --- | ---: | ---: | --- |\n")
		limit := minInt(len(report.AbstractEffects), 20)
		for _, summary := range report.AbstractEffects[:limit] {
			fmt.Fprintf(&b, "| %s | %d | %d | %s |\n", summary.Join, len(summary.Operations), len(summary.Concretization.UnsupportedFacts), summary.Manifest)
		}
		fmt.Fprintf(&b, "\n")
	}
	if len(report.SymbolicChecks) > 0 {
		fmt.Fprintf(&b, "## Symbolic checks\n\n| status | property | risk | table | reason |\n| --- | --- | --- | --- | --- |\n")
		limit := minInt(len(report.SymbolicChecks), 25)
		for _, check := range report.SymbolicChecks[:limit] {
			fmt.Fprintf(&b, "| %s | %s | `%s` | %s | %s |\n", check.Status, check.Property, check.RiskID, check.Table, check.Reason)
		}
		fmt.Fprintf(&b, "\n")
	}
	if len(report.TemporalWindows) > 0 {
		fmt.Fprintf(&b, "## Temporal windows\n\n| window | risk | table | signals | confidence |\n| --- | --- | --- | ---: | --- |\n")
		limit := minInt(len(report.TemporalWindows), 20)
		for _, window := range report.TemporalWindows[:limit] {
			fmt.Fprintf(&b, "| %s..%s | `%s` | %s | %d | %s |\n", window.Start, window.End, window.RiskID, window.Table, len(window.Signals), window.Confidence)
		}
		fmt.Fprintf(&b, "\n")
	}
	if len(report.Recurrences) > 0 {
		fmt.Fprintf(&b, "## Recurrence patterns\n\n| count | table | kind | effect | paths | confidence |\n| ---: | --- | --- | --- | ---: | --- |\n")
		limit := minInt(len(report.Recurrences), 20)
		for _, pattern := range report.Recurrences[:limit] {
			fmt.Fprintf(&b, "| %d | %s | %s | %s | %d | %s |\n", pattern.Count, pattern.Table, pattern.Kind, pattern.Effect, len(pattern.Paths), pattern.Confidence)
		}
		fmt.Fprintf(&b, "\n")
	}
	if len(report.Transactions) > 0 {
		fmt.Fprintf(&b, "## Transaction boundaries\n\n| status | surface | risk | table | path | markers |\n| --- | --- | --- | --- | --- | --- |\n")
		limit := minInt(len(report.Transactions), 25)
		for _, boundary := range report.Transactions[:limit] {
			fmt.Fprintf(&b, "| %s | %s | `%s` | %s | %s | %s |\n", boundary.Status, boundary.Surface, boundary.RiskID, boundary.Table, boundary.Path, strings.Join(boundary.Markers, ", "))
		}
		fmt.Fprintf(&b, "\n")
	}
	if len(report.Idempotency) > 0 {
		fmt.Fprintf(&b, "## Idempotency classifications\n\n| status | surface | risk | table | path | markers |\n| --- | --- | --- | --- | --- | --- |\n")
		limit := minInt(len(report.Idempotency), 25)
		for _, item := range report.Idempotency[:limit] {
			fmt.Fprintf(&b, "| %s | %s | `%s` | %s | %s | %s |\n", item.Status, item.Surface, item.RiskID, item.Table, item.Path, strings.Join(item.Markers, ", "))
		}
		fmt.Fprintf(&b, "\n")
	}
	if len(report.LockHazards) > 0 {
		fmt.Fprintf(&b, "## Lock and concurrency hazards\n\n| severity | surface | risk | table | path | markers | mitigations |\n| --- | --- | --- | --- | --- | --- | --- |\n")
		limit := minInt(len(report.LockHazards), 25)
		for _, item := range report.LockHazards[:limit] {
			fmt.Fprintf(&b, "| %s | %s | `%s` | %s | %s | %s | %s |\n", item.Severity, item.Surface, item.RiskID, item.Table, item.Path, strings.Join(item.Markers, ", "), strings.Join(item.Mitigations, ", "))
		}
		fmt.Fprintf(&b, "\n")
	}
	if len(report.PrivacyHazards) > 0 {
		fmt.Fprintf(&b, "## Data-retention and privacy hazards\n\n| severity | surface | risk | table | path | markers | mitigations |\n| --- | --- | --- | --- | --- | --- | --- |\n")
		limit := minInt(len(report.PrivacyHazards), 25)
		for _, item := range report.PrivacyHazards[:limit] {
			fmt.Fprintf(&b, "| %s | %s | `%s` | %s | %s | %s | %s |\n", item.Severity, item.Surface, item.RiskID, item.Table, item.Path, strings.Join(item.Markers, ", "), strings.Join(item.Mitigations, ", "))
		}
		fmt.Fprintf(&b, "\n")
	}
	if len(report.Invariants) > 0 {
		fmt.Fprintf(&b, "## Invariant candidates\n\n| source | kind | table | column | path | expression |\n| --- | --- | --- | --- | --- | --- |\n")
		limit := minInt(len(report.Invariants), 25)
		for _, item := range report.Invariants[:limit] {
			fmt.Fprintf(&b, "| %s | %s | %s | %s | %s | `%s` |\n", item.Source, item.Kind, item.Table, item.Column, item.Path, item.Expression)
		}
		fmt.Fprintf(&b, "\n")
	}
	if len(report.TraceLinks) > 0 {
		fmt.Fprintf(&b, "## Trace-to-code links\n\n| confidence | kind | source | code | risk | signals |\n| --- | --- | --- | --- | --- | --- |\n")
		limit := minInt(len(report.TraceLinks), 25)
		for _, item := range report.TraceLinks[:limit] {
			fmt.Fprintf(&b, "| %s | %s | %s | %s | `%s` | %s |\n", item.Confidence, item.Kind, item.SourcePath, item.CodePath, item.RiskID, strings.Join(item.Signals, ", "))
		}
		fmt.Fprintf(&b, "\n")
	}
	if len(report.BlastRadius) > 0 {
		fmt.Fprintf(&b, "## Blast-radius estimates\n\n| level | score | risk | table | centrality | fk reach | fanout | query usage |\n| --- | ---: | --- | --- | ---: | ---: | ---: | ---: |\n")
		limit := minInt(len(report.BlastRadius), 25)
		for _, item := range report.BlastRadius[:limit] {
			fmt.Fprintf(&b, "| %s | %d | `%s` | %s | %d | %d | %d | %d |\n", item.Level, item.Score, item.RiskID, item.Table, item.TableCentrality, item.FKReachability, item.CodePathFanout, item.QueryUsage)
		}
		fmt.Fprintf(&b, "\n")
	}
	if len(report.ProofMinimizers) > 0 {
		fmt.Fprintf(&b, "## Proof-hole minimizations\n\n| priority | effort | risk | source | missing evidence | upgrade | candidate paths |\n| --- | ---: | --- | --- | --- | --- | ---: |\n")
		limit := minInt(len(report.ProofMinimizers), 25)
		for _, item := range report.ProofMinimizers[:limit] {
			fmt.Fprintf(&b, "| %s | %d | `%s` | %s | %s | %s -> %s | %d |\n", item.Priority, item.Effort, item.RiskID, item.Source, item.MissingEvidence, item.UpgradeFrom, item.UpgradeTo, len(item.CandidatePaths))
		}
		fmt.Fprintf(&b, "\n")
	}
	if len(report.PolicyChecks) > 0 {
		fmt.Fprintf(&b, "## Policy checks\n\n| status | risk | class | missing | review |\n| --- | --- | --- | --- | --- |\n")
		limit := minInt(len(report.PolicyChecks), 25)
		for _, check := range report.PolicyChecks[:limit] {
			fmt.Fprintf(&b, "| %s | `%s` | %s | %s | %s |\n", check.Status, check.RiskID, check.RiskClass, strings.Join(check.Missing, ", "), check.ReviewLevel)
		}
		fmt.Fprintf(&b, "\n")
	}
	if len(report.RepairProofs) > 0 {
		fmt.Fprintf(&b, "## Repair proof summaries\n\n| status | risk | table | source | scope | frame | holes |\n| --- | --- | --- | --- | --- | --- | ---: |\n")
		limit := minInt(len(report.RepairProofs), 25)
		for _, proof := range report.RepairProofs[:limit] {
			fmt.Fprintf(&b, "| %s | `%s` | %s | %s | %s | %s | %d |\n", proof.Status, proof.RiskID, proof.Table, proof.RepairSource, proof.ScopeStatus, proof.FrameStatus, len(proof.ProofHoles))
		}
		fmt.Fprintf(&b, "\n")
	}
	if len(report.NativeChecks) > 0 {
		fmt.Fprintf(&b, "## Native checks\n\n")
		for _, command := range report.NativeChecks {
			fmt.Fprintf(&b, "- `%s` — %s\n", command.Command, command.Reason)
		}
		fmt.Fprintf(&b, "\n")
	}
	return b.String()
}

type baselineSARIFLog struct {
	Version string             `json:"version"`
	Schema  string             `json:"$schema"`
	Runs    []baselineSARIFRun `json:"runs"`
}

type baselineSARIFRun struct {
	Tool    baselineSARIFTool     `json:"tool"`
	Results []baselineSARIFResult `json:"results"`
}

type baselineSARIFTool struct {
	Driver baselineSARIFDriver `json:"driver"`
}

type baselineSARIFDriver struct {
	Name  string              `json:"name"`
	Rules []baselineSARIFRule `json:"rules"`
}

type baselineSARIFRule struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type baselineSARIFResult struct {
	RuleID    string                  `json:"ruleId"`
	Level     string                  `json:"level"`
	Message   baselineSARIFMessage    `json:"message"`
	Locations []baselineSARIFLocation `json:"locations,omitempty"`
}

type baselineSARIFMessage struct {
	Text string `json:"text"`
}

type baselineSARIFLocation struct {
	PhysicalLocation baselineSARIFPhysicalLocation `json:"physicalLocation"`
}

type baselineSARIFPhysicalLocation struct {
	ArtifactLocation baselineSARIFArtifactLocation `json:"artifactLocation"`
}

type baselineSARIFArtifactLocation struct {
	URI string `json:"uri"`
}

func renderBaselineSARIF(report BaselineReport) baselineSARIFLog {
	results := make([]baselineSARIFResult, 0, len(report.Risks))
	for _, risk := range report.Risks {
		level := "warning"
		if risk.Severity == "high" {
			level = "error"
		}
		results = append(results, baselineSARIFResult{
			RuleID:  "patchline.repo-baseline.risk",
			Level:   level,
			Message: baselineSARIFMessage{Text: fmt.Sprintf("%s risk score %d: %s", risk.Severity, risk.Score, risk.Rationale)},
			Locations: []baselineSARIFLocation{{
				PhysicalLocation: baselineSARIFPhysicalLocation{ArtifactLocation: baselineSARIFArtifactLocation{URI: risk.Path}},
			}},
		})
	}
	return baselineSARIFLog{
		Version: "2.1.0",
		Schema:  "https://json.schemastore.org/sarif-2.1.0.json",
		Runs: []baselineSARIFRun{{
			Tool:    baselineSARIFTool{Driver: baselineSARIFDriver{Name: "patchline repo baseline", Rules: []baselineSARIFRule{{ID: "patchline.repo-baseline.risk", Name: "Ranked repo-native data-change risk"}}}},
			Results: results,
		}},
	}
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
