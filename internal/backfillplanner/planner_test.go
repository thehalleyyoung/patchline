package backfillplanner

import (
	"strings"
	"testing"

	"github.com/thehalleyyoung/patchline/internal/replay"
)

func TestBuildPlanProvesCompletenessAndGatesContract(t *testing.T) {
	report, err := BuildPlan(validSpec(), replay.Store{Tables: map[string]map[string]replay.Row{
		"invoices": {
			"1": {"id": "1", "legacy_external_id": "inv-1", "external_id": "inv-1"},
			"2": {"id": "2", "legacy_external_id": "inv-2", "external_id": "inv-2"},
			"3": {"id": "3", "legacy_external_id": "inv-3", "external_id": "inv-3"},
		},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if !report.OK || report.Proof.Status != "checked" || report.Summary.RowsChecked != 3 || report.Hash == "" {
		t.Fatalf("expected checked report, got %#v", report)
	}
	if report.Summary.BlockedStages != 0 {
		t.Fatalf("expected no blocked stages, got %#v", report.Stages)
	}
	if got := stageIDs(report.Stages); strings.Join(got, ",") != "backfill,contract,delete-compatibility,expand,validate" {
		t.Fatalf("expected deterministic stage order, got %#v", got)
	}
	if !strings.Contains(RenderSQL(report), `ALTER TABLE "invoices" ALTER COLUMN "external_id" SET NOT NULL`) {
		t.Fatalf("expected contract SQL derived from proof scope:\n%s", RenderSQL(report))
	}
	for _, proof := range report.Proof.RowProofs {
		if proof.TargetHash == "" || proof.SourceHash == "" {
			t.Fatalf("expected hashed row proof without raw values: %#v", proof)
		}
	}
}

func TestBuildPlanRefutesIncompleteBackfillBeforeContract(t *testing.T) {
	spec := validSpec()
	spec.ExpectedRows = 4
	report, err := BuildPlan(spec, replay.Store{Tables: map[string]map[string]replay.Row{
		"invoices": {
			"1": {"id": "1", "legacy_external_id": "inv-1", "external_id": "inv-1"},
			"2": {"id": "2", "legacy_external_id": "inv-2"},
			"3": {"id": "3", "legacy_external_id": "inv-3", "external_id": "stale"},
			"4": {"id": "4", "legacy_external_id": "inv-4", "external_id": ""},
		},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if report.OK || report.Proof.Status != "refuted" {
		t.Fatalf("expected refuted report, got %#v", report)
	}
	got := counterexampleKeys(report.Proof.Counterexamples)
	want := "2:target_missing,3:stale_target,4:target_empty"
	if strings.Join(got, ",") != want {
		t.Fatalf("expected exact counterexamples %s, got %#v", want, got)
	}
	for _, stage := range report.Stages {
		if stage.RequiresCompleteness && stage.Ready {
			t.Fatalf("contract/deletion stage should be blocked before completeness: %#v", stage)
		}
	}
}

func TestBuildPlanRefutesUnsafeStageOrderingAndMissingCompatibilityRefs(t *testing.T) {
	spec := validSpec()
	spec.ExpectedRows = 1
	spec.CompatibilityCodeRefs = nil
	spec.Stages = []StageSpec{{
		ID: "expand", Kind: "expand",
	}, {
		ID: "backfill", Kind: "backfill", DependsOn: []string{"expand"},
	}, {
		ID: "validate", Kind: "validate", DependsOn: []string{"backfill"},
	}, {
		ID: "contract", Kind: "contract", TightensConstraint: true,
	}, {
		ID: "delete-compatibility", Kind: "delete_compatibility", DeletesCompatibility: true, DependsOn: []string{"validate"},
	}}
	report, err := BuildPlan(spec, replay.Store{Tables: map[string]map[string]replay.Row{
		"invoices": {
			"1": {"id": "1", "legacy_external_id": "inv-1", "external_id": "inv-1"},
		},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if report.OK {
		t.Fatalf("expected unsafe stage ordering to fail: %#v", report)
	}
	statuses := obligationStatuses(report.Obligations)
	if statuses["stage.contract.after_validate"] != "refuted" || statuses["compatibility.refs"] != "refuted" {
		t.Fatalf("expected stage and compatibility obligations to fail, got %#v", statuses)
	}
}

func TestBuildPlanTreatsEmptyTableAsInconclusive(t *testing.T) {
	report, err := BuildPlan(validSpec(), replay.Store{Tables: map[string]map[string]replay.Row{"invoices": {}}})
	if err != nil {
		t.Fatal(err)
	}
	if report.OK || report.Proof.Status != "inconclusive" || report.Proof.Counterexamples[0].Code != "table_empty" {
		t.Fatalf("expected empty table to be inconclusive, got %#v", report.Proof)
	}
}

func TestReadSpecRejectsUnknownFields(t *testing.T) {
	_, err := ReadSpec(strings.NewReader(`{"version":"patchline.backfill-plan/v1","name":"x","table":"t","target_column":"c","stages":[],"extra":true}`))
	if err == nil {
		t.Fatal("expected unknown field error")
	}
}

func validSpec() Spec {
	return Spec{
		Version:               SpecVersion,
		Name:                  "invoice external id staged backfill",
		Table:                 "invoices",
		PrimaryKey:            "id",
		SourceColumn:          "legacy_external_id",
		TargetColumn:          "external_id",
		ExpectedRows:          3,
		CompatibilityCodeRefs: []string{"app/models/invoice.rb:dual_write_external_id"},
		Stages: []StageSpec{{
			ID: "expand", Kind: "expand", Command: "add nullable external_id",
		}, {
			ID: "backfill", Kind: "backfill", DependsOn: []string{"expand"}, Command: "copy legacy_external_id into external_id",
		}, {
			ID: "validate", Kind: "validate", DependsOn: []string{"backfill"}, Command: "run generated validation SQL",
		}, {
			ID: "contract", Kind: "contract", DependsOn: []string{"validate"}, TightensConstraint: true, Command: "set external_id NOT NULL",
		}, {
			ID: "delete-compatibility", Kind: "delete_compatibility", DependsOn: []string{"validate"}, DeletesCompatibility: true, Command: "remove dual write fallback",
		}},
	}
}

func stageIDs(stages []Stage) []string {
	out := make([]string, 0, len(stages))
	for _, stage := range stages {
		out = append(out, stage.ID)
	}
	return out
}

func counterexampleKeys(counterexamples []Counterexample) []string {
	out := make([]string, 0, len(counterexamples))
	for _, counterexample := range counterexamples {
		out = append(out, counterexample.RowID+":"+counterexample.Code)
	}
	return out
}

func obligationStatuses(obligations []Obligation) map[string]string {
	out := map[string]string{}
	for _, obligation := range obligations {
		out[obligation.ID] = obligation.Status
	}
	return out
}
