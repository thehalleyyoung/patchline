package archive

import (
	"sort"

	"github.com/thehalleyyoung/patchline/internal/canonical"
)

const Version = "patchline.archive-index/v1"

type Spec struct {
	Version   string      `json:"version"`
	Name      string      `json:"name"`
	Incidents []InputSpec `json:"incidents"`
}

type InputSpec struct {
	ID        string `json:"id"`
	Evidence  string `json:"evidence"`
	Migration string `json:"migration"`
	Repair    string `json:"repair"`
	Policy    string `json:"policy"`
	Benchmark string `json:"benchmark"`
}

type Report struct {
	Version             string          `json:"version"`
	Name                string          `json:"name"`
	Incidents           []Entry         `json:"incidents"`
	ByShape             []Bucket        `json:"by_shape"`
	ByMigrationTable    []Bucket        `json:"by_migration_table"`
	ByMigrationRisk     []Bucket        `json:"by_migration_risk"`
	ByRepairEffect      []Bucket        `json:"by_repair_effect"`
	ByPolicyDecision    []Bucket        `json:"by_policy_decision"`
	ByBenchmarkDecision []Bucket        `json:"by_benchmark_decision"`
	RepairOutcomes      []RepairOutcome `json:"repair_outcomes"`
	HistoricalQueries   Queries         `json:"historical_queries"`
	Hash                string          `json:"hash"`
}

type Entry struct {
	ID                       string               `json:"id"`
	EvidencePath             string               `json:"evidence_path"`
	MigrationPath            string               `json:"migration_path"`
	RepairPath               string               `json:"repair_path"`
	PolicyPath               string               `json:"policy_path"`
	BenchmarkPath            string               `json:"benchmark_path"`
	EvidenceHash             string               `json:"evidence_hash"`
	ShapeHash                string               `json:"shape_hash"`
	MigrationHash            string               `json:"migration_hash"`
	MigrationTables          []string             `json:"migration_tables,omitempty"`
	MigrationMaxRisk         string               `json:"migration_max_risk"`
	MigrationBroadUpdates    []MigrationStatement `json:"migration_broad_updates,omitempty"`
	RepairHash               string               `json:"repair_hash"`
	RepairEffect             string               `json:"repair_effect"`
	RepairDryRunHash         string               `json:"repair_dry_run_hash,omitempty"`
	RepairAppliedSQLHash     string               `json:"repair_applied_sql_hash,omitempty"`
	RepairRollbackAvailable  bool                 `json:"repair_rollback_available"`
	RepairVerificationResult string               `json:"repair_verification_result,omitempty"`
	RepairVerificationHash   string               `json:"repair_verification_hash,omitempty"`
	PolicyAllowed            bool                 `json:"policy_allowed"`
	PolicyFailures           []string             `json:"policy_failures,omitempty"`
	PolicyHash               string               `json:"policy_hash"`
	BenchmarkOK              bool                 `json:"benchmark_ok"`
	BenchmarkHash            string               `json:"benchmark_hash"`
	DamagedEntities          int                  `json:"damaged_entities"`
	DamagedEntityIDs         []string             `json:"damaged_entity_ids,omitempty"`
	DerivedReports           int                  `json:"derived_reports"`
	DerivedReportIDs         []string             `json:"derived_report_ids,omitempty"`
	ProofBundleReady         bool                 `json:"proof_bundle_ready"`
}

type RepairOutcome struct {
	IncidentID         string   `json:"incident_id"`
	RepairPath         string   `json:"repair_path"`
	RepairHash         string   `json:"repair_hash"`
	DryRunHash         string   `json:"dry_run_hash"`
	AppliedSQLHash     string   `json:"applied_sql_hash"`
	VerificationResult string   `json:"verification_result"`
	VerificationHash   string   `json:"verification_hash"`
	RollbackAvailable  bool     `json:"rollback_available"`
	LaterRecurrences   []string `json:"later_recurrences,omitempty"`
	Hash               string   `json:"hash"`
}

type Bucket struct {
	Key       string   `json:"key"`
	Count     int      `json:"count"`
	Incidents []string `json:"incidents"`
}

type MigrationStatement struct {
	Table       string `json:"table"`
	Operation   string `json:"operation"`
	Risk        string `json:"risk"`
	Effect      string `json:"effect"`
	Fingerprint string `json:"fingerprint"`
	Reason      string `json:"reason"`
}

type Queries struct {
	BroadUpdateMigrations  []BroadUpdateResult     `json:"broad_update_migrations"`
	DamagedDerivedReports  []DerivedReportResult   `json:"damaged_derived_reports"`
	RepairsLackingRollback []MissingRollbackResult `json:"repairs_lacking_rollback"`
	RepairOutcomeHistory   []RepairOutcome         `json:"repair_outcome_history"`
	Hash                   string                  `json:"hash"`
}

type BroadUpdateResult struct {
	IncidentID    string `json:"incident_id"`
	MigrationPath string `json:"migration_path"`
	MigrationHash string `json:"migration_hash"`
	Table         string `json:"table"`
	Operation     string `json:"operation"`
	Risk          string `json:"risk"`
	Effect        string `json:"effect"`
	Fingerprint   string `json:"fingerprint"`
	Reason        string `json:"reason"`
}

type DerivedReportResult struct {
	ReportID  string   `json:"report_id"`
	Count     int      `json:"count"`
	Incidents []string `json:"incidents"`
}

type MissingRollbackResult struct {
	IncidentID    string `json:"incident_id"`
	RepairPath    string `json:"repair_path"`
	RepairHash    string `json:"repair_hash"`
	RepairEffect  string `json:"repair_effect"`
	PolicyAllowed bool   `json:"policy_allowed"`
}

func Build(spec Spec, entries []Entry) Report {
	orderByID := map[string]int{}
	for index, entry := range entries {
		if _, exists := orderByID[entry.ID]; !exists {
			orderByID[entry.ID] = index
		}
	}
	entries = append([]Entry(nil), entries...)
	sort.Slice(entries, func(i, j int) bool { return entries[i].ID < entries[j].ID })
	queries := buildQueries(entries, orderByID)
	repairOutcomes := repairOutcomes(entries, orderByID)
	report := Report{
		Version:             Version,
		Name:                spec.Name,
		Incidents:           entries,
		ByShape:             bucket(entries, func(e Entry) []string { return []string{e.ShapeHash} }),
		ByMigrationTable:    bucket(entries, func(e Entry) []string { return e.MigrationTables }),
		ByMigrationRisk:     bucket(entries, func(e Entry) []string { return []string{e.MigrationMaxRisk} }),
		ByRepairEffect:      bucket(entries, func(e Entry) []string { return []string{e.RepairEffect} }),
		ByPolicyDecision:    bucket(entries, func(e Entry) []string { return []string{boolKey(e.PolicyAllowed)} }),
		ByBenchmarkDecision: bucket(entries, func(e Entry) []string { return []string{boolKey(e.BenchmarkOK)} }),
		RepairOutcomes:      repairOutcomes,
		HistoricalQueries:   queries,
	}
	report.Hash = canonical.Hash(struct {
		Version             string          `json:"version"`
		Name                string          `json:"name"`
		Incidents           []Entry         `json:"incidents"`
		ByShape             []Bucket        `json:"by_shape"`
		ByMigrationTable    []Bucket        `json:"by_migration_table"`
		ByMigrationRisk     []Bucket        `json:"by_migration_risk"`
		ByRepairEffect      []Bucket        `json:"by_repair_effect"`
		ByPolicyDecision    []Bucket        `json:"by_policy_decision"`
		ByBenchmarkDecision []Bucket        `json:"by_benchmark_decision"`
		RepairOutcomes      []RepairOutcome `json:"repair_outcomes"`
		HistoricalQueries   Queries         `json:"historical_queries"`
	}{report.Version, report.Name, report.Incidents, report.ByShape, report.ByMigrationTable, report.ByMigrationRisk, report.ByRepairEffect, report.ByPolicyDecision, report.ByBenchmarkDecision, report.RepairOutcomes, report.HistoricalQueries})
	return report
}

func buildQueries(entries []Entry, orderByID map[string]int) Queries {
	queries := Queries{
		BroadUpdateMigrations:  broadUpdateMigrations(entries),
		DamagedDerivedReports:  damagedDerivedReports(entries),
		RepairsLackingRollback: repairsLackingRollback(entries),
		RepairOutcomeHistory:   repairOutcomes(entries, orderByID),
	}
	queries.Hash = canonical.Hash(struct {
		BroadUpdateMigrations  []BroadUpdateResult     `json:"broad_update_migrations"`
		DamagedDerivedReports  []DerivedReportResult   `json:"damaged_derived_reports"`
		RepairsLackingRollback []MissingRollbackResult `json:"repairs_lacking_rollback"`
		RepairOutcomeHistory   []RepairOutcome         `json:"repair_outcome_history"`
	}{queries.BroadUpdateMigrations, queries.DamagedDerivedReports, queries.RepairsLackingRollback, queries.RepairOutcomeHistory})
	return queries
}

func repairOutcomes(entries []Entry, orderByID map[string]int) []RepairOutcome {
	out := make([]RepairOutcome, 0, len(entries))
	for _, entry := range entries {
		outcome := RepairOutcome{
			IncidentID:         entry.ID,
			RepairPath:         entry.RepairPath,
			RepairHash:         entry.RepairHash,
			DryRunHash:         entry.RepairDryRunHash,
			AppliedSQLHash:     entry.RepairAppliedSQLHash,
			VerificationResult: entry.RepairVerificationResult,
			VerificationHash:   entry.RepairVerificationHash,
			RollbackAvailable:  entry.RepairRollbackAvailable,
			LaterRecurrences:   laterRecurrences(entries, entry, orderByID),
		}
		outcome.Hash = canonical.Hash(struct {
			IncidentID         string   `json:"incident_id"`
			RepairPath         string   `json:"repair_path"`
			RepairHash         string   `json:"repair_hash"`
			DryRunHash         string   `json:"dry_run_hash"`
			AppliedSQLHash     string   `json:"applied_sql_hash"`
			VerificationResult string   `json:"verification_result"`
			VerificationHash   string   `json:"verification_hash"`
			RollbackAvailable  bool     `json:"rollback_available"`
			LaterRecurrences   []string `json:"later_recurrences,omitempty"`
		}{outcome.IncidentID, outcome.RepairPath, outcome.RepairHash, outcome.DryRunHash, outcome.AppliedSQLHash, outcome.VerificationResult, outcome.VerificationHash, outcome.RollbackAvailable, outcome.LaterRecurrences})
		out = append(out, outcome)
	}
	return out
}

func laterRecurrences(entries []Entry, current Entry, orderByID map[string]int) []string {
	var candidates []Entry
	currentTables := stringSet(current.MigrationTables)
	currentOrder := orderByID[current.ID]
	for _, candidate := range entries {
		if candidate.ID == current.ID || orderByID[candidate.ID] <= currentOrder {
			continue
		}
		if candidate.ShapeHash == current.ShapeHash || intersects(currentTables, candidate.MigrationTables) {
			candidates = append(candidates, candidate)
		}
	}
	sort.Slice(candidates, func(i, j int) bool {
		left := orderByID[candidates[i].ID]
		right := orderByID[candidates[j].ID]
		if left == right {
			return candidates[i].ID < candidates[j].ID
		}
		return left < right
	})
	out := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		out = append(out, candidate.ID)
	}
	return out
}

func broadUpdateMigrations(entries []Entry) []BroadUpdateResult {
	out := []BroadUpdateResult{}
	for _, entry := range entries {
		for _, statement := range entry.MigrationBroadUpdates {
			out = append(out, BroadUpdateResult{
				IncidentID:    entry.ID,
				MigrationPath: entry.MigrationPath,
				MigrationHash: entry.MigrationHash,
				Table:         statement.Table,
				Operation:     statement.Operation,
				Risk:          statement.Risk,
				Effect:        statement.Effect,
				Fingerprint:   statement.Fingerprint,
				Reason:        statement.Reason,
			})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Table != out[j].Table {
			return out[i].Table < out[j].Table
		}
		if out[i].IncidentID != out[j].IncidentID {
			return out[i].IncidentID < out[j].IncidentID
		}
		return out[i].Fingerprint < out[j].Fingerprint
	})
	return out
}

func damagedDerivedReports(entries []Entry) []DerivedReportResult {
	byReport := map[string]map[string]struct{}{}
	for _, entry := range entries {
		for _, reportID := range entry.DerivedReportIDs {
			if byReport[reportID] == nil {
				byReport[reportID] = map[string]struct{}{}
			}
			byReport[reportID][entry.ID] = struct{}{}
		}
	}
	reportIDs := make([]string, 0, len(byReport))
	for reportID := range byReport {
		reportIDs = append(reportIDs, reportID)
	}
	sort.Strings(reportIDs)
	out := make([]DerivedReportResult, 0, len(reportIDs))
	for _, reportID := range reportIDs {
		incidents := sortedSet(byReport[reportID])
		out = append(out, DerivedReportResult{ReportID: reportID, Count: len(incidents), Incidents: incidents})
	}
	return out
}

func repairsLackingRollback(entries []Entry) []MissingRollbackResult {
	out := []MissingRollbackResult{}
	for _, entry := range entries {
		if entry.RepairRollbackAvailable {
			continue
		}
		out = append(out, MissingRollbackResult{
			IncidentID:    entry.ID,
			RepairPath:    entry.RepairPath,
			RepairHash:    entry.RepairHash,
			RepairEffect:  entry.RepairEffect,
			PolicyAllowed: entry.PolicyAllowed,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].IncidentID < out[j].IncidentID })
	return out
}

func bucket(entries []Entry, keyFunc func(Entry) []string) []Bucket {
	byKey := map[string]map[string]struct{}{}
	for _, entry := range entries {
		for _, key := range keyFunc(entry) {
			if key == "" {
				key = "unknown"
			}

			if byKey[key] == nil {
				byKey[key] = map[string]struct{}{}
			}
			byKey[key][entry.ID] = struct{}{}
		}
	}
	keys := make([]string, 0, len(byKey))
	for key := range byKey {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out := make([]Bucket, 0, len(keys))
	for _, key := range keys {
		ids := make([]string, 0, len(byKey[key]))
		for id := range byKey[key] {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		out = append(out, Bucket{Key: key, Count: len(ids), Incidents: ids})
	}
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

func boolKey(value bool) string {
	if value {
		return "true"
	}
	return "false"
}

func stringSet(values []string) map[string]struct{} {
	out := map[string]struct{}{}
	for _, value := range values {
		out[value] = struct{}{}
	}
	return out
}

func intersects(set map[string]struct{}, values []string) bool {
	for _, value := range values {
		if _, ok := set[value]; ok {
			return true
		}
	}
	return false
}
