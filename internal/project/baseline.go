package project

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/thehalleyyoung/patchline/internal/canonical"
	"github.com/thehalleyyoung/patchline/internal/intake"
	"github.com/thehalleyyoung/patchline/internal/migration"
)

const BaselineVersion = "patchline.repo-baseline/v1"

type BaselineReport struct {
	Version        string            `json:"version"`
	InventoryRoot  string            `json:"inventory_root"`
	IntakeSource   string            `json:"intake_source"`
	Summary        BaselineSummary   `json:"summary"`
	Risks          []BaselineRisk    `json:"risks,omitempty"`
	EvidenceLinks  []EvidenceLink    `json:"evidence_links,omitempty"`
	CauseClusters  []EvidenceCluster `json:"cause_clusters,omitempty"`
	RepairClusters []EvidenceCluster `json:"repair_clusters,omitempty"`
	NativeChecks   []Command         `json:"native_checks,omitempty"`
	Hash           string            `json:"hash"`
	Markdown       string            `json:"markdown,omitempty"`
}

type BaselineSummary struct {
	RankedRisks         int `json:"ranked_risks"`
	EvidenceLinks       int `json:"evidence_links"`
	CauseClusters       int `json:"cause_clusters"`
	RepairClusters      int `json:"repair_clusters"`
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

func Baseline(inv Inventory, facts []Fact, intakeReport intake.Report) BaselineReport {
	report := BaselineReport{Version: BaselineVersion, InventoryRoot: inv.Root, IntakeSource: intakeReport.Source.Input}
	factIndex := indexFacts(facts)
	report.Risks = rankRisks(intakeReport, factIndex)
	report.EvidenceLinks = linkRisks(report.Risks, factIndex)
	report.CauseClusters = clusterCandidates("cause", intakeReport.Causes, factIndex)
	report.RepairClusters = clusterRepairCandidates(intakeReport.RepairCandidates, factIndex)
	report.NativeChecks = uniqueCommands(append([]Command(nil), inv.TestCommands...))
	report.Summary = BaselineSummary{
		RankedRisks:         len(report.Risks),
		EvidenceLinks:       len(report.EvidenceLinks),
		CauseClusters:       len(report.CauseClusters),
		RepairClusters:      len(report.RepairClusters),
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

func rankRisks(report intake.Report, facts factIndex) []BaselineRisk {
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
	fmt.Fprintf(&b, "| evidence links | %d |\n", report.Summary.EvidenceLinks)
	fmt.Fprintf(&b, "| cause clusters | %d |\n", report.Summary.CauseClusters)
	fmt.Fprintf(&b, "| repair clusters | %d |\n", report.Summary.RepairClusters)
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
