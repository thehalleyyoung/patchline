package project

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/thehalleyyoung/patchline/internal/canonical"
	"github.com/thehalleyyoung/patchline/internal/effects"
	"github.com/thehalleyyoung/patchline/internal/intake"
	"github.com/thehalleyyoung/patchline/internal/migration"
)

const BaselineVersion = "patchline.repo-baseline/v1"

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
	ID          string        `json:"id"`
	Path        string        `json:"path"`
	Statement   int           `json:"statement,omitempty"`
	Kind        string        `json:"kind"`
	Table       string        `json:"table,omitempty"`
	Severity    string        `json:"severity"`
	Score       int           `json:"score"`
	Factors     []ScoreFactor `json:"factors,omitempty"`
	Identifiers []Identifier  `json:"identifiers,omitempty"`
	Rationale   string        `json:"rationale"`
	NextCommand string        `json:"next_command,omitempty"`
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
	report.DatalogQueries = buildDatalogQueries(report.Risks, report.Provenance)
	report.AbstractEffects = buildAbstractEffectSummaries(report.Risks, report.Provenance)
	report.SymbolicChecks = buildSymbolicChecks(report.Risks, report.AbstractEffects, report.Provenance)
	report.TemporalWindows = buildTemporalWindows(report.Risks, report.Provenance, intakeReport)
	report.Recurrences = buildRecurrences(report.Risks, report.AbstractEffects, report.Provenance)
	report.PolicyChecks = buildPolicyChecks(report.Risks, report.Provenance, report.SymbolicChecks)
	report.RepairProofs = buildRepairProofSummaries(report.Risks, report.Provenance, report.AbstractEffects, report.SymbolicChecks)
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
	case "update", "delete", "insert":
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
		fmt.Fprintf(&b, "## Top risks\n\n| score | severity | path | kind | table | rationale |\n| ---: | --- | --- | --- | --- | --- |\n")
		limit := minInt(len(report.Risks), 25)
		for _, risk := range report.Risks[:limit] {
			fmt.Fprintf(&b, "| %d | %s | %s | %s | %s | %s |\n", risk.Score, risk.Severity, risk.Path, risk.Kind, risk.Table, risk.Rationale)
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
