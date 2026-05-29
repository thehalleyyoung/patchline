package bundle

import (
	"testing"

	"github.com/thehalleyyoung/patchline/internal/demo"
	"github.com/thehalleyyoung/patchline/internal/migration"
	"github.com/thehalleyyoung/patchline/internal/policy"
	"github.com/thehalleyyoung/patchline/internal/provenance"
	"github.com/thehalleyyoung/patchline/internal/replay"
)

func TestBuildBundleIsDeterministic(t *testing.T) {
	inputs := sampleInputs(t)
	first := Build(inputs)
	second := Build(inputs)
	if first.BundleHash != second.BundleHash {
		t.Fatalf("bundle hashes differ: %s != %s", first.BundleHash, second.BundleHash)
	}
	if len(first.Entries) == 0 {
		t.Fatal("expected bundle entries")
	}
}

func sampleInputs(t *testing.T) Inputs {
	t.Helper()
	manifest := demo.SampleRepair()
	report, err := replay.DryRun(manifest, demo.Graph(), demo.BillingStore())
	if err != nil {
		t.Fatal(err)
	}
	slice, err := demo.Graph().Slice(provenance.SliceOptions{
		Starts:      []string{"record:invoices/inv_1002"},
		Direction:   provenance.DirectionBoth,
		MaxDepth:    4,
		MinEvidence: provenance.EvidenceStrong,
	})
	if err != nil {
		t.Fatal(err)
	}
	migrationReport, err := migration.AnalyzeBytes("bad.sql", []byte("update invoices set total_cents = 0 where status = 'issued';"))
	if err != nil {
		t.Fatal(err)
	}
	_, checkpoint := demo.SampleLedger()
	eval := policy.Evaluate(policy.Policy{Version: policy.Version, Name: "review", Rules: policy.Rules{AllowHighRiskMigration: true}}, policy.Inputs{
		Manifest: manifest, Report: report, Migration: migrationReport, ExpectedReportHash: report.Hash(), LedgerCheckpoint: checkpoint,
	})
	return Inputs{
		Name:             "bad-migration-billing",
		Manifest:         manifest,
		Report:           report,
		Slice:            slice,
		Migration:        migrationReport,
		PolicyEvaluation: eval,
		LedgerCheckpoint: checkpoint,
	}
}
