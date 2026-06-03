package dbsemantics

import "testing"

func TestBuildReproducibilityReportPinsAllEnginesAndObservedEvidence(t *testing.T) {
	reports := allEngineSemanticReports(t)
	repro, err := BuildReproducibilityReport(reports)
	if err != nil {
		t.Fatalf("build reproducibility report failed: %v", err)
	}
	if repro.Version != ReproducibilityReportVersion || repro.Hash == "" {
		t.Fatalf("report missing version/hash: %#v", repro)
	}
	if repro.Summary.Engines != len(SupportedEngines()) || repro.Summary.EngineVersionPins != len(SupportedEngines()) {
		t.Fatalf("expected one pin per supported engine, got summary=%#v", repro.Summary)
	}
	if repro.Summary.ContainerImages < 5 || repro.Summary.ManagedOrEmbeddedProfiles < 3 {
		t.Fatalf("expected local image pins plus embedded/managed profiles, got summary=%#v pins=%#v", repro.Summary, repro.EnginePins)
	}
	if repro.Summary.ProfileEvidence < len(SupportedEngines()) || repro.Summary.LockSimulations < len(SupportedEngines()) || repro.Summary.ContainerSmokeFixtures < len(SupportedEngines()) {
		t.Fatalf("expected profile, lock, and smoke observations for every engine, got summary=%#v", repro.Summary)
	}
	if repro.Summary.StatementRules == 0 || repro.Summary.RollbackChecks == 0 || repro.Summary.ReplicationLagRisks == 0 {
		t.Fatalf("expected behavior observations from real db-semantics reports, got summary=%#v", repro.Summary)
	}
	if !hasRuntimePin(repro, EnginePostgres, "postgres:16") || !hasRuntimePin(repro, EngineClickHouse, "clickhouse/clickhouse-server:24.1") {
		t.Fatalf("expected concrete engine image pins, got %#v", repro.EnginePins)
	}
	if !hasObservationKind(repro, "engine_negative_control") || !hasObservationKind(repro, "query_plan_regression") {
		t.Fatalf("expected negative-control and query-plan observations, got %#v", repro.Observations)
	}

	reproAgain, err := BuildReproducibilityReport(reverseReports(reports))
	if err != nil {
		t.Fatalf("build reversed reproducibility report failed: %v", err)
	}
	if repro.Hash != reproAgain.Hash {
		t.Fatalf("hash should be independent of input order: %s != %s", repro.Hash, reproAgain.Hash)
	}
}

func TestBuildReproducibilityReportRejectsMissingEngineAndDuplicateVersion(t *testing.T) {
	reports := allEngineSemanticReports(t)
	if _, err := BuildReproducibilityReport(reports[:len(reports)-1]); err == nil {
		t.Fatal("expected missing supported engine to fail")
	}
	duplicated := append(append([]Report(nil), reports...), reports[0])
	if _, err := BuildReproducibilityReport(duplicated); err == nil {
		t.Fatal("expected duplicate engine/version report to fail")
	}
}

func allEngineSemanticReports(t *testing.T) []Report {
	t.Helper()
	cases := []struct {
		engine  Engine
		version string
		sql     string
	}{
		{EnginePostgres, "16", "CREATE INDEX CONCURRENTLY idx_accounts_status ON accounts(status);"},
		{EngineMySQL, "8.0.34", "ALTER TABLE accounts ADD COLUMN status varchar(20) DEFAULT 'active';"},
		{EngineSQLite, "3.45.1", "PRAGMA foreign_keys = OFF;"},
		{EngineSQLServer, "2022", "CREATE INDEX idx_accounts_status ON accounts(status) WITH (ONLINE=ON);"},
		{EngineOracle, "23", "ALTER TABLE accounts MODIFY status NOT NULL;"},
		{EngineBigQuery, "2024.2", "CREATE OR REPLACE TABLE analytics.daily AS SELECT * FROM analytics.stage;"},
		{EngineSnowflake, "8.20", "CREATE OR REPLACE TABLE analytics.daily AS SELECT * FROM analytics.stage;"},
		{EngineClickHouse, "24.1", "ALTER TABLE events DELETE WHERE created_at < now() - interval 30 day;"},
	}
	reports := make([]Report, 0, len(cases))
	for _, tc := range cases {
		report, err := Evaluate(tc.engine, tc.version, string(tc.engine)+".sql", []byte(tc.sql))
		if err != nil {
			t.Fatalf("%s evaluate failed: %v", tc.engine, err)
		}
		reports = append(reports, report)
	}
	return reports
}

func hasRuntimePin(report ReproducibilityReport, engine Engine, image string) bool {
	for _, pin := range report.EnginePins {
		if pin.Engine == engine && pin.ContainerImage == image {
			return true
		}
	}
	return false
}

func hasObservationKind(report ReproducibilityReport, kind string) bool {
	for _, observation := range report.Observations {
		if observation.ObservationKind == kind {
			return true
		}
	}
	return false
}

func reverseReports(reports []Report) []Report {
	out := append([]Report(nil), reports...)
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out
}
