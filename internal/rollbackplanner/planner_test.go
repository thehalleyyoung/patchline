package rollbackplanner

import (
	"strings"
	"testing"
)

func TestBuildReportVerifiesMultiServiceRollbackOrderAndBounds(t *testing.T) {
	report, err := BuildReport(validSpec())
	if err != nil {
		t.Fatal(err)
	}
	if !report.OK || report.DependencyProof.Status != "checked" || report.DataLossProof.Status != "checked" || report.Hash == "" {
		t.Fatalf("expected checked rollback report, got %#v", report)
	}
	if got, want := strings.Join(report.DependencyProof.ForwardOrder, ","), "billing-expand,ledger-dual-write,api-read-shift,billing-contract"; got != want {
		t.Fatalf("unexpected forward order: got %s want %s", got, want)
	}
	if got, want := strings.Join(report.DependencyProof.RollbackOrder, ","), "billing-contract,api-read-shift,ledger-dual-write,billing-expand"; got != want {
		t.Fatalf("unexpected rollback order: got %s want %s", got, want)
	}
	if report.DependencyProof.MaxDepth != 4 || report.DependencyProof.MaxFanout != 1 || report.Summary.RollbackWaves != 4 {
		t.Fatalf("unexpected dependency bounds: %#v", report.DependencyProof)
	}
	if report.Summary.DataLossRows != 0 || report.Summary.CriticalDataLossRows != 0 || report.Summary.AffectedServices != 0 {
		t.Fatalf("expected zero data loss, got %#v", report.Summary)
	}
	if len(report.DependencyProof.CrossServiceEdges) != 3 {
		t.Fatalf("expected three cross-service dependency witnesses, got %#v", report.DependencyProof.CrossServiceEdges)
	}
	for _, edge := range report.DependencyProof.CrossServiceEdges {
		if !edge.Declared {
			t.Fatalf("expected declared cross-service edge: %#v", edge)
		}
	}
	markdown := RenderMarkdown(report)
	if !strings.Contains(markdown, "Verified multi-service rollback planner") || !strings.Contains(markdown, "rollback order") {
		t.Fatalf("expected useful markdown, got:\n%s", markdown)
	}
}

func TestBuildReportRefutesUnverifiedRollbackAndDataLossBounds(t *testing.T) {
	spec := validSpec()
	spec.DataLossBound = DataLossBound{MaxRows: 10, MaxCriticalRows: 1, MaxAffectedServices: 0}
	spec.Migrations[2].RollbackVerified = false
	spec.Migrations[2].RollbackAction = ""
	spec.Migrations[2].EstimatedRows = 45
	spec.Migrations[2].CriticalRows = 3
	report, err := BuildReport(spec)
	if err != nil {
		t.Fatal(err)
	}
	if report.OK || report.DataLossProof.Status != "refuted" {
		t.Fatalf("expected data-loss refutation, got %#v", report.DataLossProof)
	}
	statuses := obligationStatuses(report.Obligations)
	for _, id := range []string{"data_loss.rollback_verified", "data_loss.bounds.rows", "data_loss.bounds.critical_rows", "data_loss.bounds.services"} {
		if statuses[id] != "refuted" {
			t.Fatalf("expected %s to be refuted, got statuses %#v", id, statuses)
		}
	}
	if report.DataLossProof.Rows != 45 || report.DataLossProof.CriticalRows != 3 || strings.Join(report.DataLossProof.AffectedServices, ",") != "api" {
		t.Fatalf("unexpected data-loss proof: %#v", report.DataLossProof)
	}
}

func TestBuildReportRefutesCycleAndUnknownDependenciesWithoutReturningError(t *testing.T) {
	spec := validSpec()
	spec.DependencyBound = DependencyBound{MaxDepth: 4, MaxFanout: 2}
	spec.Migrations = []Migration{{
		ID: "a", ServiceID: "billing", RollbackVerified: true, RollbackAction: "restore a", DependsOn: []string{"b", "missing"},
	}, {
		ID: "b", ServiceID: "ledger", RollbackVerified: true, RollbackAction: "restore b", DependsOn: []string{"a"}, UpstreamServices: []string{"billing"},
	}}
	report, err := BuildReport(spec)
	if err != nil {
		t.Fatal(err)
	}
	if report.OK || report.DependencyProof.Status != "refuted" {
		t.Fatalf("expected dependency refutation, got %#v", report.DependencyProof)
	}
	ids := counterexampleIDs(report.Counterexamples)
	if !strings.Contains(ids, "dependency.cycle") || !strings.Contains(ids, "dependency.unknown.a.missing") {
		t.Fatalf("expected cycle and unknown dependency counterexamples, got %s", ids)
	}
}

func TestBuildReportTieBreaksIndependentMigrationsDeterministically(t *testing.T) {
	spec := Spec{
		Version:         SpecVersion,
		Name:            "tie break",
		DependencyBound: DependencyBound{MaxDepth: 1, MaxFanout: 0},
		DataLossBound:   DataLossBound{MaxRows: 0, MaxCriticalRows: 0, MaxAffectedServices: 0},
		Services:        []Service{{ID: "svc"}},
		Migrations: []Migration{{
			ID: "b", ServiceID: "svc", RollbackVerified: true, RollbackAction: "restore b",
		}, {
			ID: "a", ServiceID: "svc", RollbackVerified: true, RollbackAction: "restore a",
		}},
	}
	first, err := BuildReport(spec)
	if err != nil {
		t.Fatal(err)
	}
	second, err := BuildReport(spec)
	if err != nil {
		t.Fatal(err)
	}
	if first.Hash == "" || first.Hash != second.Hash {
		t.Fatalf("expected deterministic hash, first=%q second=%q", first.Hash, second.Hash)
	}
	if got, want := strings.Join(first.DependencyProof.ForwardOrder, ","), "a,b"; got != want {
		t.Fatalf("unexpected deterministic forward order: got %s want %s", got, want)
	}
	if got, want := strings.Join(first.DependencyProof.RollbackOrder, ","), "b,a"; got != want {
		t.Fatalf("unexpected deterministic rollback order: got %s want %s", got, want)
	}
}

func TestReadSpecRejectsUnknownFields(t *testing.T) {
	_, err := ReadSpec(strings.NewReader(`{"version":"patchline.multi-service-rollback-plan/v1","name":"x","dependency_bound":{"max_depth":1,"max_fanout":1},"data_loss_bound":{"max_rows":0,"max_critical_rows":0,"max_affected_services":0},"services":[],"migrations":[],"extra":true}`))
	if err == nil {
		t.Fatal("expected unknown field error")
	}
}

func TestBuildReportRejectsInvalidCriticalRowEstimate(t *testing.T) {
	spec := validSpec()
	spec.Migrations[0].CriticalRows = spec.Migrations[0].EstimatedRows + 1
	_, err := BuildReport(spec)
	if err == nil || !strings.Contains(err.Error(), "critical_rows cannot exceed estimated_rows") {
		t.Fatalf("expected invalid critical row estimate error, got %v", err)
	}
}

func validSpec() Spec {
	return Spec{
		Version:         SpecVersion,
		Name:            "billing multi-service rollback",
		DependencyBound: DependencyBound{MaxDepth: 4, MaxFanout: 2, MaxWaves: 4},
		DataLossBound:   DataLossBound{MaxRows: 0, MaxCriticalRows: 0, MaxAffectedServices: 0},
		Services: []Service{{
			ID: "api", Name: "public API", Owners: []string{"@org/api"}, Criticality: "high", UpstreamServices: []string{"ledger"}, DownstreamServices: []string{"billing"},
		}, {
			ID: "billing", Name: "billing writer", Owners: []string{"@org/billing"}, Criticality: "critical", UpstreamServices: []string{"api"}, DownstreamServices: []string{"ledger"},
		}, {
			ID: "ledger", Name: "ledger projector", Owners: []string{"@org/ledger"}, Criticality: "high", UpstreamServices: []string{"billing"}, DownstreamServices: []string{"api"},
		}},
		Migrations: []Migration{{
			ID: "billing-expand", ServiceID: "billing", Stage: "expand", Kind: "schema", Operation: "add nullable external_id", DataClasses: []string{"invoice"}, RollbackAction: "drop external_id while nullable before writers use it", RollbackVerified: true,
		}, {
			ID: "ledger-dual-write", ServiceID: "ledger", Stage: "dual-write", Kind: "application", Operation: "write ledger external_id shadow column", DataClasses: []string{"invoice", "ledger"}, EstimatedRows: 2500, DependsOn: []string{"billing-expand"}, RollbackAction: "disable dual-write flag and replay from billing snapshot", RollbackVerified: true,
		}, {
			ID: "api-read-shift", ServiceID: "api", Stage: "read-shift", Kind: "application", Operation: "read external_id from ledger view", DataClasses: []string{"invoice"}, EstimatedRows: 2500, DependsOn: []string{"ledger-dual-write"}, RollbackAction: "restore API read flag to legacy_external_id", RollbackVerified: true,
		}, {
			ID: "billing-contract", ServiceID: "billing", Stage: "contract", Kind: "schema", Operation: "set external_id not null and remove legacy id", DataClasses: []string{"invoice"}, EstimatedRows: 2500, DependsOn: []string{"api-read-shift"}, RollbackAction: "restore legacy column from verified backfill snapshot before re-enabling legacy reads", RollbackVerified: true,
		}},
	}
}

func obligationStatuses(obligations []Obligation) map[string]string {
	out := map[string]string{}
	for _, obligation := range obligations {
		out[obligation.ID] = obligation.Status
	}
	return out
}

func counterexampleIDs(counterexamples []Counterexample) string {
	var ids []string
	for _, counterexample := range counterexamples {
		ids = append(ids, counterexample.ID)
	}
	return strings.Join(ids, ",")
}
