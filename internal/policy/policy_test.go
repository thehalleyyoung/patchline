package policy

import (
	"strings"
	"testing"

	"github.com/patchline/patchline/internal/demo"
	"github.com/patchline/patchline/internal/migration"
	"github.com/patchline/patchline/internal/replay"
)

func TestReadRejectsUnknownPolicyFields(t *testing.T) {
	_, err := Read(strings.NewReader(`{"version":"patchline.policy/v1","name":"strict","rules":{},"typo":true}`))
	if err == nil {
		t.Fatal("expected unknown field error")
	}
}

func TestEvaluateBlocksHighRiskMigrationWhenNotAllowed(t *testing.T) {
	manifest := demo.SampleRepair()
	report, err := replay.DryRun(manifest, demo.Graph(), demo.BillingStore())
	if err != nil {
		t.Fatal(err)
	}
	migrationReport, err := migration.AnalyzeBytes("bad.sql", []byte("update invoices set total_cents = 0 where status = 'issued';"))
	if err != nil {
		t.Fatal(err)
	}
	_, checkpoint := demo.SampleLedger()
	one := 1
	eval := Evaluate(Policy{
		Version: Version,
		Name:    "strict",
		Rules: Rules{
			RequireSnapshotRollback: true,
			MaxChangedRows:          &one,
			AllowHighRiskMigration:  false,
			RequirePinnedReportHash: true,
			AllowedEffects:          []string{"reversible_update"},
			RequireLedgerCheckpoint: true,
		},
	}, Inputs{
		Manifest:           manifest,
		Report:             report,
		Migration:          migrationReport,
		ExpectedReportHash: report.Hash(),
		LedgerCheckpoint:   checkpoint,
	})
	if eval.OK {
		t.Fatalf("expected policy to fail on high-risk migration: %#v", eval)
	}
}
