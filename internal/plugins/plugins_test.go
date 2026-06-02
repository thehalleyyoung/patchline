package plugins

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultCatalogCoversAllExtensionPoints(t *testing.T) {
	catalog := DefaultCatalog()
	if catalog.Version != CatalogVersion || catalog.Hash == "" {
		t.Fatalf("unexpected catalog metadata: %#v", catalog)
	}
	counts := map[Kind]int{}
	for _, plugin := range catalog.Plugins {
		if plugin.Name == "" || plugin.Version == "" || plugin.Description == "" {
			t.Fatalf("incomplete plugin info: %#v", plugin)
		}
		if !plugin.Deterministic {
			t.Fatalf("default plugin should be deterministic: %#v", plugin)
		}
		counts[plugin.Kind]++
	}
	for _, kind := range []Kind{ParserKind, FactExtractorKind, LinkerKind, RankerKind, ProposalGeneratorKind, CompareCheckKind, ReportRendererKind} {
		if counts[kind] == 0 {
			t.Fatalf("missing plugin kind %s in %#v", kind, counts)
		}
	}
}

func TestDefaultRegistryRunsRepairPipelineOnRealFixtureCode(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "db/migrate/001_backfill_accounts.sql", "UPDATE accounts SET status = 'active';\nDELETE FROM account_events;\n")
	writeFile(t, root, "app/jobs/invoice_job.rb", "Invoice.where(status: 'stale').update_all(status: 'ready')\n")

	report, err := Probe(context.Background(), ProbeOptions{
		Path:   root,
		OutDir: filepath.Join(t.TempDir(), "plugin-probe"),
		Kind:   "all",
		Budget: "files=4,lines=80,tokens=12000,changes=2",
	})
	if err != nil {
		t.Fatal(err)
	}
	if report.Summary.FilesScanned < 2 || report.Summary.Facts == 0 {
		t.Fatalf("expected parser and fact extractor activity, got %#v", report.Summary)
	}
	if report.Summary.RankedRisks == 0 || report.Summary.GeneratedFiles == 0 || report.Summary.GeneratedChecks == 0 {
		t.Fatalf("expected ranker, generator, and compare checks, got %#v", report.Summary)
	}
	if report.Summary.RenderedReports != 4 || len(report.Rendered) != 4 {
		t.Fatalf("expected renderer outputs, got %#v", report.Rendered)
	}
	for _, rendered := range report.Rendered {
		if rendered.Hash == "" || rendered.Bytes == 0 {
			t.Fatalf("unexpected rendered report metadata: %#v", rendered)
		}
	}
	outDir := filepath.Join(t.TempDir(), "written")
	if err := WriteProbe(outDir, report); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"plugin-probe.json", "plugin-probe.md"} {
		if stat, err := os.Stat(filepath.Join(outDir, name)); err != nil || stat.Size() == 0 {
			t.Fatalf("expected %s to be written, stat=%#v err=%v", name, stat, err)
		}
	}
}

func writeFile(t *testing.T, root, rel, content string) {
	t.Helper()
	path := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
