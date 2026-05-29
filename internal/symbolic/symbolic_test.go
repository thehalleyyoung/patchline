package symbolic

import (
	"testing"

	"github.com/thehalleyyoung/patchline/internal/demo"
)

func TestExecuteExploresBoundedRepairPaths(t *testing.T) {
	report := Execute(demo.SampleRepair(), demo.BillingStore())
	if report.Summary.Steps != 1 {
		t.Fatalf("expected one symbolic step, got %+v", report.Summary)
	}
	if report.Summary.RowsExplored != 2 || report.Summary.RowsSatisfying != 1 {
		t.Fatalf("expected one satisfying path over two invoice rows, got %+v", report.Summary)
	}
	if report.Summary.Assignments != 2 {
		t.Fatalf("expected two symbolic assignments, got %+v", report.Summary)
	}
	step := report.Steps[0]
	if step.Status != "normal" || step.PreHash == "" || step.PostHash == "" {
		t.Fatalf("expected normal step with pre/post hashes, got %+v", step)
	}
	var satisfying RowPath
	for _, row := range step.Rows {
		if row.GuardStatus == "satisfied" {
			satisfying = row
		}
	}
	if satisfying.RowID != "inv_1002" {
		t.Fatalf("expected inv_1002 satisfying path, got %+v", satisfying)
	}
	if satisfying.Assignments[0].Column != "repair_marker" || satisfying.Assignments[1].Column != "total_cents" {
		t.Fatalf("assignments are not deterministic and sorted: %+v", satisfying.Assignments)
	}
}
