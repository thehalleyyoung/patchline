package archive

import (
	"sort"

	"github.com/patchline/patchline/internal/canonical"
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
	Version             string   `json:"version"`
	Name                string   `json:"name"`
	Incidents           []Entry  `json:"incidents"`
	ByShape             []Bucket `json:"by_shape"`
	ByMigrationTable    []Bucket `json:"by_migration_table"`
	ByMigrationRisk     []Bucket `json:"by_migration_risk"`
	ByRepairEffect      []Bucket `json:"by_repair_effect"`
	ByPolicyDecision    []Bucket `json:"by_policy_decision"`
	ByBenchmarkDecision []Bucket `json:"by_benchmark_decision"`
	Hash                string   `json:"hash"`
}

type Entry struct {
	ID               string   `json:"id"`
	EvidencePath     string   `json:"evidence_path"`
	MigrationPath    string   `json:"migration_path"`
	RepairPath       string   `json:"repair_path"`
	PolicyPath       string   `json:"policy_path"`
	BenchmarkPath    string   `json:"benchmark_path"`
	EvidenceHash     string   `json:"evidence_hash"`
	ShapeHash        string   `json:"shape_hash"`
	MigrationHash    string   `json:"migration_hash"`
	MigrationTables  []string `json:"migration_tables,omitempty"`
	MigrationMaxRisk string   `json:"migration_max_risk"`
	RepairHash       string   `json:"repair_hash"`
	RepairEffect     string   `json:"repair_effect"`
	PolicyAllowed    bool     `json:"policy_allowed"`
	PolicyFailures   []string `json:"policy_failures,omitempty"`
	PolicyHash       string   `json:"policy_hash"`
	BenchmarkOK      bool     `json:"benchmark_ok"`
	BenchmarkHash    string   `json:"benchmark_hash"`
	DamagedEntities  int      `json:"damaged_entities"`
	DerivedReports   int      `json:"derived_reports"`
	ProofBundleReady bool     `json:"proof_bundle_ready"`
}

type Bucket struct {
	Key       string   `json:"key"`
	Count     int      `json:"count"`
	Incidents []string `json:"incidents"`
}

func Build(spec Spec, entries []Entry) Report {
	entries = append([]Entry(nil), entries...)
	sort.Slice(entries, func(i, j int) bool { return entries[i].ID < entries[j].ID })
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
	}
	report.Hash = canonical.Hash(struct {
		Version             string   `json:"version"`
		Name                string   `json:"name"`
		Incidents           []Entry  `json:"incidents"`
		ByShape             []Bucket `json:"by_shape"`
		ByMigrationTable    []Bucket `json:"by_migration_table"`
		ByMigrationRisk     []Bucket `json:"by_migration_risk"`
		ByRepairEffect      []Bucket `json:"by_repair_effect"`
		ByPolicyDecision    []Bucket `json:"by_policy_decision"`
		ByBenchmarkDecision []Bucket `json:"by_benchmark_decision"`
	}{report.Version, report.Name, report.Incidents, report.ByShape, report.ByMigrationTable, report.ByMigrationRisk, report.ByRepairEffect, report.ByPolicyDecision, report.ByBenchmarkDecision})
	return report
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

func boolKey(value bool) string {
	if value {
		return "true"
	}
	return "false"
}
