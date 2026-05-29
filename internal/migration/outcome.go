package migration

import (
	"fmt"
	"sort"
	"strings"

	"github.com/patchline/patchline/internal/canonical"
	"github.com/patchline/patchline/internal/provenance"
)

const MigrationOutcomeVersion = "patchline.migration-outcomes/v1"

type OutcomeOptions struct {
	EvidenceHash        string   `json:"evidence_hash,omitempty"`
	RepairID            string   `json:"repair_id,omitempty"`
	RepairHash          string   `json:"repair_hash,omitempty"`
	RepairOperations    int      `json:"repair_operations,omitempty"`
	RollbackStrategy    string   `json:"rollback_strategy,omitempty"`
	PolicyFailures      []string `json:"policy_failures,omitempty"`
	BenchmarkHash       string   `json:"benchmark_hash,omitempty"`
	SourceSQLHash       string   `json:"source_sql_hash,omitempty"`
	InvariantCandidates []string `json:"invariant_candidates,omitempty"`
}

type MigrationOutcomeReport struct {
	Version             string                     `json:"version"`
	MigrationSource     string                     `json:"migration_source"`
	MigrationReportHash string                     `json:"migration_report_hash"`
	EvidenceHash        string                     `json:"evidence_hash,omitempty"`
	Outcomes            []MigrationOutcome         `json:"outcomes"`
	Changelog           MigrationSemanticChangelog `json:"changelog"`
	Hash                string                     `json:"hash"`
}

type MigrationOutcome struct {
	MigrationID    string               `json:"migration_id"`
	MigrationName  string               `json:"migration_name,omitempty"`
	Traces         []string             `json:"traces,omitempty"`
	SQLMutations   []SQLMutationOutcome `json:"sql_mutations,omitempty"`
	Records        []string             `json:"records,omitempty"`
	DerivedReports []string             `json:"derived_reports,omitempty"`
	Repairs        []RepairOutcome      `json:"repairs,omitempty"`
	PolicyFailures []string             `json:"policy_failures,omitempty"`
	Hash           string               `json:"hash"`
}

type SQLMutationOutcome struct {
	ID          string `json:"id"`
	Fingerprint string `json:"fingerprint,omitempty"`
	Table       string `json:"table,omitempty"`
	Risk        Risk   `json:"risk,omitempty"`
	Effect      string `json:"effect,omitempty"`
}

type RepairOutcome struct {
	RepairID         string `json:"repair_id"`
	RepairHash       string `json:"repair_hash"`
	OperationCount   int    `json:"operation_count"`
	RollbackStrategy string `json:"rollback_strategy,omitempty"`
}

type MigrationSemanticChangelog struct {
	ChangedTables       []TableChange        `json:"changed_tables,omitempty"`
	BroadEffects        []BroadEffect        `json:"broad_effects,omitempty"`
	ObservedOutcomes    ObservedOutcomeStats `json:"observed_outcomes"`
	PolicyFailures      []string             `json:"policy_failures,omitempty"`
	InvariantCandidates []string             `json:"invariant_candidates,omitempty"`
	BenchmarkHash       string               `json:"benchmark_hash,omitempty"`
	SourceSQLHash       string               `json:"source_sql_hash,omitempty"`
	Hash                string               `json:"hash"`
}

type TableChange struct {
	Table       string   `json:"table"`
	Operation   string   `json:"operation"`
	Risk        Risk     `json:"risk"`
	Effect      string   `json:"effect"`
	Reasons     []string `json:"reasons,omitempty"`
	Fingerprint string   `json:"fingerprint"`
}

type BroadEffect struct {
	Table       string `json:"table,omitempty"`
	Operation   string `json:"operation"`
	Risk        Risk   `json:"risk"`
	Effect      string `json:"effect"`
	Fingerprint string `json:"fingerprint"`
	Reason      string `json:"reason"`
}

type ObservedOutcomeStats struct {
	Migrations   int `json:"migrations"`
	Traces       int `json:"traces"`
	SQLMutations int `json:"sql_mutations"`
	Records      int `json:"records"`
	Reports      int `json:"reports"`
	Repairs      int `json:"repairs"`
}

func BuildMigrationOutcomeReport(source string, report Report, entities []provenance.Entity, edges []provenance.Edge, opts OutcomeOptions) MigrationOutcomeReport {
	entityMap := map[string]provenance.Entity{}
	for _, entity := range entities {
		entityMap[entity.ID] = entity
	}
	outgoing := map[string][]provenance.Edge{}
	for _, edge := range edges {
		outgoing[edge.From] = append(outgoing[edge.From], edge)
	}
	for id := range outgoing {
		sort.Slice(outgoing[id], func(i, j int) bool {
			if outgoing[id][i].To != outgoing[id][j].To {
				return outgoing[id][i].To < outgoing[id][j].To
			}
			return outgoing[id][i].Kind < outgoing[id][j].Kind
		})
	}
	statementByFingerprint := map[string]Statement{}
	for _, statement := range report.Statements {
		statementByFingerprint[statement.Fingerprint] = statement
	}

	var outcomes []MigrationOutcome
	for _, entity := range entities {
		if entity.Kind != provenance.KindMigration {
			continue
		}
		outcome := MigrationOutcome{MigrationID: entity.ID, MigrationName: entity.Name}
		traceSet, sqlSet, recordSet, reportSet := map[string]bool{}, map[string]bool{}, map[string]bool{}, map[string]bool{}
		for _, traceEdge := range outgoing[entity.ID] {
			if traceEdge.Kind != provenance.EdgeObserved {
				continue
			}
			traceSet[traceEdge.To] = true
			for _, sqlEdge := range outgoing[traceEdge.To] {
				if sqlEdge.Kind != provenance.EdgeCaused {
					continue
				}
				sqlSet[sqlEdge.To] = true
				for _, rowEdge := range outgoing[sqlEdge.To] {
					if rowEdge.Kind != provenance.EdgeMutated {
						continue
					}
					recordSet[rowEdge.To] = true
					collectDerivedReports(rowEdge.To, outgoing, entityMap, recordSet, reportSet)
				}
			}
		}
		outcome.Traces = sortedKeys(traceSet)
		outcome.Records = sortedKeys(recordSet)
		outcome.DerivedReports = sortedKeys(reportSet)
		for _, sqlID := range sortedKeys(sqlSet) {
			sqlEntity := entityMap[sqlID]
			fingerprint := sqlEntity.Attributes["fingerprint"]
			statement := statementByFingerprint[fingerprint]
			outcome.SQLMutations = append(outcome.SQLMutations, SQLMutationOutcome{
				ID:          sqlID,
				Fingerprint: fingerprint,
				Table:       statement.Table,
				Risk:        statement.Risk,
				Effect:      statement.Effect,
			})
		}
		if opts.RepairID != "" {
			outcome.Repairs = append(outcome.Repairs, RepairOutcome{
				RepairID:         opts.RepairID,
				RepairHash:       opts.RepairHash,
				OperationCount:   opts.RepairOperations,
				RollbackStrategy: opts.RollbackStrategy,
			})
		}
		outcome.PolicyFailures = append([]string(nil), opts.PolicyFailures...)
		sort.Strings(outcome.PolicyFailures)
		outcome.Hash = canonical.Hash(struct {
			MigrationID    string               `json:"migration_id"`
			Traces         []string             `json:"traces,omitempty"`
			SQLMutations   []SQLMutationOutcome `json:"sql_mutations,omitempty"`
			Records        []string             `json:"records,omitempty"`
			DerivedReports []string             `json:"derived_reports,omitempty"`
			Repairs        []RepairOutcome      `json:"repairs,omitempty"`
			PolicyFailures []string             `json:"policy_failures,omitempty"`
		}{outcome.MigrationID, outcome.Traces, outcome.SQLMutations, outcome.Records, outcome.DerivedReports, outcome.Repairs, outcome.PolicyFailures})
		outcomes = append(outcomes, outcome)
	}
	sort.Slice(outcomes, func(i, j int) bool { return outcomes[i].MigrationID < outcomes[j].MigrationID })
	result := MigrationOutcomeReport{
		Version:             MigrationOutcomeVersion,
		MigrationSource:     source,
		MigrationReportHash: report.Summary.ReportHash,
		EvidenceHash:        opts.EvidenceHash,
		Outcomes:            outcomes,
	}
	result.Changelog = BuildMigrationChangelog(report, outcomes, opts)
	result.Hash = canonical.Hash(struct {
		Version             string                     `json:"version"`
		MigrationSource     string                     `json:"migration_source"`
		MigrationReportHash string                     `json:"migration_report_hash"`
		EvidenceHash        string                     `json:"evidence_hash,omitempty"`
		Outcomes            []MigrationOutcome         `json:"outcomes"`
		Changelog           MigrationSemanticChangelog `json:"changelog"`
	}{result.Version, result.MigrationSource, result.MigrationReportHash, result.EvidenceHash, result.Outcomes, result.Changelog})
	return result
}

func BuildMigrationChangelog(report Report, outcomes []MigrationOutcome, opts OutcomeOptions) MigrationSemanticChangelog {
	changelog := MigrationSemanticChangelog{
		BenchmarkHash:       opts.BenchmarkHash,
		SourceSQLHash:       opts.SourceSQLHash,
		PolicyFailures:      append([]string(nil), opts.PolicyFailures...),
		InvariantCandidates: append([]string(nil), opts.InvariantCandidates...),
	}
	sort.Strings(changelog.PolicyFailures)
	sort.Strings(changelog.InvariantCandidates)
	seenChanges := map[string]bool{}
	for _, statement := range report.Statements {
		if statement.Table != "" {
			change := TableChange{
				Table:       statement.Table,
				Operation:   statement.Kind,
				Risk:        statement.Risk,
				Effect:      statement.Effect,
				Reasons:     append([]string(nil), statement.Reasons...),
				Fingerprint: statement.Fingerprint,
			}
			key := fmt.Sprintf("%s:%s:%s", change.Table, change.Operation, change.Fingerprint)
			if !seenChanges[key] {
				seenChanges[key] = true
				changelog.ChangedTables = append(changelog.ChangedTables, change)
			}
		}
		if statement.Risk == RiskHigh || isBroadStatement(statement) {
			reason := "high-risk migration effect"
			if isBroadStatement(statement) {
				reason = "potential broad effect over table"
			}
			changelog.BroadEffects = append(changelog.BroadEffects, BroadEffect{
				Table:       statement.Table,
				Operation:   statement.Kind,
				Risk:        statement.Risk,
				Effect:      statement.Effect,
				Fingerprint: statement.Fingerprint,
				Reason:      reason,
			})
		}
	}
	sort.Slice(changelog.ChangedTables, func(i, j int) bool {
		if changelog.ChangedTables[i].Table != changelog.ChangedTables[j].Table {
			return changelog.ChangedTables[i].Table < changelog.ChangedTables[j].Table
		}
		return changelog.ChangedTables[i].Fingerprint < changelog.ChangedTables[j].Fingerprint
	})
	sort.Slice(changelog.BroadEffects, func(i, j int) bool {
		if changelog.BroadEffects[i].Table != changelog.BroadEffects[j].Table {
			return changelog.BroadEffects[i].Table < changelog.BroadEffects[j].Table
		}
		return changelog.BroadEffects[i].Fingerprint < changelog.BroadEffects[j].Fingerprint
	})
	stats := ObservedOutcomeStats{Migrations: len(outcomes)}
	traceSet, sqlSet, recordSet, reportSet := map[string]bool{}, map[string]bool{}, map[string]bool{}, map[string]bool{}
	for _, outcome := range outcomes {
		for _, id := range outcome.Traces {
			traceSet[id] = true
		}
		for _, sql := range outcome.SQLMutations {
			sqlSet[sql.ID] = true
		}
		for _, id := range outcome.Records {
			recordSet[id] = true
		}
		for _, id := range outcome.DerivedReports {
			reportSet[id] = true
		}
		stats.Repairs += len(outcome.Repairs)
	}
	stats.Traces = len(traceSet)
	stats.SQLMutations = len(sqlSet)
	stats.Records = len(recordSet)
	stats.Reports = len(reportSet)
	changelog.ObservedOutcomes = stats
	changelog.Hash = canonical.Hash(struct {
		ChangedTables       []TableChange        `json:"changed_tables,omitempty"`
		BroadEffects        []BroadEffect        `json:"broad_effects,omitempty"`
		ObservedOutcomes    ObservedOutcomeStats `json:"observed_outcomes"`
		PolicyFailures      []string             `json:"policy_failures,omitempty"`
		InvariantCandidates []string             `json:"invariant_candidates,omitempty"`
		BenchmarkHash       string               `json:"benchmark_hash,omitempty"`
		SourceSQLHash       string               `json:"source_sql_hash,omitempty"`
	}{changelog.ChangedTables, changelog.BroadEffects, changelog.ObservedOutcomes, changelog.PolicyFailures, changelog.InvariantCandidates, changelog.BenchmarkHash, changelog.SourceSQLHash})
	return changelog
}

func collectDerivedReports(start string, outgoing map[string][]provenance.Edge, entities map[string]provenance.Entity, records, reports map[string]bool) {
	queue := []string{start}
	seen := map[string]bool{start: true}
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		for _, edge := range outgoing[current] {
			if edge.Kind != provenance.EdgeDerivedInto {
				continue
			}
			if seen[edge.To] {
				continue
			}
			seen[edge.To] = true
			entity := entities[edge.To]
			if entity.Kind == provenance.KindReport {
				reports[edge.To] = true
				continue
			}
			records[edge.To] = true
			queue = append(queue, edge.To)
		}
	}
}

func isBroadStatement(statement Statement) bool {
	switch statement.Kind {
	case "update", "delete":
		return !statement.HasWhere || statement.Risk == RiskHigh
	case "drop", "truncate":
		return true
	default:
		return false
	}
}

func sortedKeys(values map[string]bool) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		if strings.TrimSpace(key) != "" {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	return keys
}
