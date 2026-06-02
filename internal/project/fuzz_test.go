package project

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/thehalleyyoung/patchline/internal/intake"
)

func FuzzFactNormalization(f *testing.F) {
	f.Add("table", "Accounts", "AccountID", "db/migrate/001_accounts.sql")
	f.Add("endpoint", "/api/v1/invoices", "commit abc1234", "app/api.py")
	f.Add("column", `"UserName";`, "deploy-42", "service.go")
	f.Fuzz(func(t *testing.T, kind, value, rationale, path string) {
		if len(kind)+len(value)+len(rationale)+len(path) > 4096 {
			t.Skip()
		}
		inv := Inventory{Version: Version}
		inv.addFact(Fact{
			Kind:       kind,
			Path:       filepath.ToSlash(filepath.Clean(path)),
			Confidence: "fuzz",
			Rationale:  rationale,
			Identifiers: []Identifier{
				{Kind: kind, Value: value},
				{Kind: kind, Value: value},
			},
			Properties: map[string]string{"field": value},
		})
		if len(inv.Facts) != 1 {
			t.Fatalf("expected one fact, got %#v", inv.Facts)
		}
		fact := inv.Facts[0]
		if fact.Version != Version || fact.ID == "" || !strings.HasPrefix(fact.ID, "fact:") {
			t.Fatalf("fact not normalized: %#v", fact)
		}
		if len(fact.Identifiers) > 1 && fact.Identifiers[0] == fact.Identifiers[1] {
			t.Fatalf("duplicate identifiers were not normalized: %#v", fact.Identifiers)
		}
		if fact.ID != factID(fact) {
			t.Fatalf("fact id is not stable: got %s recomputed %s", fact.ID, factID(fact))
		}
	})
}

func FuzzArchiveExtraction(f *testing.F) {
	f.Add("zip", "repo/db/migrate/001.sql", "UPDATE accounts SET status = 'active';")
	f.Add("tar", "repo/app/models/user.rb", "User.where(active: false).update_all(active: true)")
	f.Add("zip", "../escape.sql", "DELETE FROM users;")
	f.Fuzz(func(t *testing.T, kind, name, content string) {
		if len(name)+len(content) > 4096 {
			t.Skip()
		}
		target := t.TempDir()
		switch strings.ToLower(kind) {
		case "zip":
			archive := filepath.Join(t.TempDir(), "fixture.zip")
			if err := writeZip(archive, name, content); err != nil {
				t.Fatal(err)
			}
			root, err := extractZip(archive, target)
			if err == nil {
				assertContainedPath(t, target, root)
			}
		default:
			var buf bytes.Buffer
			if err := writeTarGz(&buf, name, content); err != nil {
				t.Fatal(err)
			}
			root, _, err := extractTarGz(bytes.NewReader(buf.Bytes()), target)
			if err == nil {
				assertContainedPath(t, target, root)
			}
		}
	})
}

func FuzzReportLoading(f *testing.F) {
	f.Add(`{"version":"patchline.project/v1","root":"/tmp","files_scanned":1}`+"\n", `{"version":"patchline.project/v1","id":"fact:seed","kind":"file","path":"db/migrate/001.sql"}`+"\n")
	f.Add(`{"version":"patchline.repo-baseline/v1","inventory_root":"/tmp","summary":{"ranked_risks":1},"hash":"abc"}`, "")
	f.Fuzz(func(t *testing.T, reportJSON, factsJSONL string) {
		if len(reportJSON)+len(factsJSONL) > 8192 {
			t.Skip()
		}
		root := t.TempDir()
		_ = os.WriteFile(filepath.Join(root, "inventory.json"), []byte(reportJSON), 0o644)
		_ = os.WriteFile(filepath.Join(root, "facts.jsonl"), []byte(factsJSONL), 0o644)
		if inv, _, err := LoadInventory(root); err == nil {
			if inv.Version == "" && inv.FilesScanned < 0 {
				t.Fatalf("impossible inventory loaded: %#v", inv)
			}
		}
		_ = os.WriteFile(filepath.Join(root, "baseline.json"), []byte(reportJSON), 0o644)
		if baseline, err := LoadBaseline(root); err == nil {
			if baseline.Summary.RankedRisks < 0 {
				t.Fatalf("impossible baseline loaded: %#v", baseline)
			}
		}
		_ = os.WriteFile(filepath.Join(root, "summary.json"), []byte(reportJSON), 0o644)
		if report, err := LoadIntakeReport(root); err == nil {
			if report.Summary.FilesScanned < 0 {
				t.Fatalf("impossible intake report loaded: %#v", report)
			}
		}
		_ = os.WriteFile(filepath.Join(root, "proposal.json"), []byte(reportJSON), 0o644)
		if proposal, err := LoadProposal(root); err == nil {
			if proposal.BudgetRisks < 0 {
				t.Fatalf("impossible proposal loaded: %#v", proposal)
			}
		}
	})
}

func FuzzParserPipeline(f *testing.F) {
	f.Add("db/migrate/001_accounts.sql", "UPDATE accounts SET status = 'active';")
	f.Add("app/jobs/invoice_job.rb", `Invoice.where(status: "stale").update_all(status: "ready")`)
	f.Add("exports/event.json", `{"message":"deploy abc1234 touched accounts","db.statement":"DELETE FROM accounts"}`)
	f.Fuzz(func(t *testing.T, rel, content string) {
		if len(rel)+len(content) > 4096 {
			t.Skip()
		}
		root := t.TempDir()
		rel = safeFuzzRel(rel)
		if err := os.MkdirAll(filepath.Dir(filepath.Join(root, rel)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, rel), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		inv, err := InventoryPath(InventoryOptions{Path: root})
		if err != nil {
			t.Fatalf("InventoryPath returned error: %v", err)
		}
		intakeReport, err := intake.Run(context.Background(), intake.Options{Path: root})
		if err != nil {
			t.Fatalf("intake.Run returned error: %v", err)
		}
		baseline := Baseline(inv, inv.Facts, intakeReport)
		if baseline.Summary.RankedRisks != len(baseline.Risks) {
			t.Fatalf("baseline summary mismatch: %#v", baseline.Summary)
		}
	})
}

func writeZip(path, name, content string) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()
	zw := zip.NewWriter(file)
	w, err := zw.Create(name)
	if err != nil {
		_ = zw.Close()
		return err
	}
	if _, err := w.Write([]byte(content)); err != nil {
		_ = zw.Close()
		return err
	}
	return zw.Close()
}

func writeTarGz(buf *bytes.Buffer, name, content string) error {
	gz := gzip.NewWriter(buf)
	tw := tar.NewWriter(gz)
	if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o644, Size: int64(len(content))}); err != nil {
		return err
	}
	if _, err := tw.Write([]byte(content)); err != nil {
		return err
	}
	if err := tw.Close(); err != nil {
		return err
	}
	return gz.Close()
}

func assertContainedPath(t *testing.T, root, path string) {
	t.Helper()
	rel, err := filepath.Rel(root, path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.HasPrefix(rel, "..") || filepath.IsAbs(rel) {
		t.Fatalf("extracted path escaped target: root=%s path=%s", root, path)
	}
}

func safeFuzzRel(rel string) string {
	rel = strings.Map(func(r rune) rune {
		if r == 0 || r < 32 || r == ':' {
			return '-'
		}
		return r
	}, rel)
	rel = filepath.ToSlash(filepath.Clean(rel))
	rel = strings.TrimPrefix(rel, "/")
	for strings.HasPrefix(rel, "../") || strings.Contains(rel, "/../") || rel == ".." || rel == "." || rel == "" {
		rel = "fuzz.sql"
	}
	if filepath.Ext(rel) == "" {
		rel += ".sql"
	}
	return filepath.FromSlash(rel)
}
