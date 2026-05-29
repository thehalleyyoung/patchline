package artifact

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestGeneratePaperTables(t *testing.T) {
	report, err := GeneratePaperTables(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("GeneratePaperTables: %v", err)
	}
	if report.Version != PaperTablesVersion {
		t.Fatalf("version = %q, want %q", report.Version, PaperTablesVersion)
	}
	if len(report.Tables) != 5 {
		t.Fatalf("tables = %d, want 5", len(report.Tables))
	}
	if report.Hash == "" {
		t.Fatal("hash is empty")
	}
	if report.SourceHashes["public-archive"] == "" {
		t.Fatal("public archive source hash is missing")
	}
	ids := map[string]bool{}
	for _, table := range report.Tables {
		ids[table.ID] = true
		if len(table.Columns) == 0 {
			t.Fatalf("%s has no columns", table.ID)
		}
		if len(table.Rows) == 0 {
			t.Fatalf("%s has no rows", table.ID)
		}
		for _, row := range table.Rows {
			if len(row) != len(table.Columns) {
				t.Fatalf("%s row has %d cells, want %d: %#v", table.ID, len(row), len(table.Columns), row)
			}
		}
	}
	for _, id := range []string{"table-1-corpus", "table-2-detection-actionability", "table-3-ablation", "table-4-historical-counterfactuals", "table-5-scale"} {
		if !ids[id] {
			t.Fatalf("missing table %s", id)
		}
	}
	if !strings.Contains(report.Markdown, "public-postmortem-derived") {
		t.Fatal("markdown does not state the public-postmortem-derived boundary")
	}
	if !strings.Contains(report.Markdown, "cannot_prove") {
		t.Fatal("markdown does not expose cannot_prove outcomes")
	}
}
