package incidentpostmortem

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildReportImportsHistoricalLessonsIntoDetectorRegressions(t *testing.T) {
	specPath := filepath.Join("..", "..", "examples", "incident-postmortem-import.json")
	file, err := os.Open(specPath)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	spec, err := ReadSpec(file)
	if err != nil {
		t.Fatal(err)
	}
	report, err := BuildReport(spec, filepath.Dir(specPath))
	if err != nil {
		t.Fatal(err)
	}
	if !report.OK || report.Summary.Cases != 2 || report.Summary.Regressions < 20 || report.Summary.Failed != 0 || report.Hash == "" {
		t.Fatalf("unexpected postmortem importer report: %#v", report.Summary)
	}
	for _, signalID := range []string{
		"source-established-primary-data-loss",
		"protected-primary-mutation",
		"missing-snapshot-rollback",
		"split-brain-conflicting-writes",
	} {
		if !hasRegressionSignal(report, signalID) {
			t.Fatalf("expected regression for %s", signalID)
		}
	}
	for _, regression := range report.Regressions {
		if !regression.Positive.Detected {
			t.Fatalf("positive fixture did not trigger detector: %#v", regression)
		}
		for _, negative := range regression.Negatives {
			if negative.Detected {
				t.Fatalf("negative fixture triggered detector: regression=%s negative=%#v", regression.ID, negative)
			}
		}
	}
	second, err := BuildReport(spec, filepath.Dir(specPath))
	if err != nil {
		t.Fatal(err)
	}
	if report.Hash != second.Hash {
		t.Fatalf("expected deterministic hash, first=%s second=%s", report.Hash, second.Hash)
	}
}

func TestWriteArtifactsEmitsFixturesAndGeneratedGoTest(t *testing.T) {
	specPath := filepath.Join("..", "..", "examples", "incident-postmortem-import.json")
	file, err := os.Open(specPath)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	spec, err := ReadSpec(file)
	if err != nil {
		t.Fatal(err)
	}
	report, err := BuildReport(spec, filepath.Dir(specPath))
	if err != nil {
		t.Fatal(err)
	}
	out := t.TempDir()
	if err := WriteArtifacts(out, report); err != nil {
		t.Fatal(err)
	}
	for _, rel := range []string{
		"incident-postmortem-import.json",
		"incident-postmortem-import.md",
		"detector-regressions.json",
		"generated-tests/incident_postmortem_regression_test.go",
		report.Regressions[0].Positive.Path,
		report.Regressions[0].Negatives[0].Path,
	} {
		if stat, err := os.Stat(filepath.Join(out, filepath.FromSlash(rel))); err != nil || stat.Size() == 0 {
			t.Fatalf("expected %s to be written, stat=%#v err=%v", rel, stat, err)
		}
	}
	generated, err := os.ReadFile(filepath.Join(out, "generated-tests", "incident_postmortem_regression_test.go"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(generated), "TestIncidentPostmortemDetectorRegressions") || !strings.Contains(string(generated), "incidentpostmortem.DetectSignal") {
		t.Fatalf("generated test does not call importer detector hook:\n%s", generated)
	}
}

func TestDetectSignalNegativeControlsAreDiscriminating(t *testing.T) {
	got, _, err := DetectSignal(InputRepair, "missing-snapshot-rollback", []byte(`{
  "version": "patchline.repair/v1",
  "name": "snapshot backed",
  "incident": "negative",
  "scope": {"table":"projects","where":{"id":"42"}},
  "operations": [{"id":"delete-with-snapshot","kind":"delete","table":"projects","where":{"id":"42"}}],
  "rollback": {"strategy":"snapshot","snapshot_required":true}
}`), nil)
	if err != nil {
		t.Fatal(err)
	}
	if got {
		t.Fatal("snapshot-backed repair should not trigger missing-snapshot detector")
	}
	evidencePrefix := `{"type":"deploy","id":"deploy:negative","commit":"commit:negative","service":"mysql"}
{"type":"migration","id":"migration:negative","deploy":"deploy:negative","name":"negative"}
{"type":"trace","id":"trace:east","migration":"migration:negative"}
{"type":"trace","id":"trace:west","migration":"migration:negative"}
{"type":"sql_mutation","id":"sql:east","trace":"trace:east","fingerprint":"update issues set state = ? where id = ?"}
{"type":"sql_mutation","id":"sql:west","trace":"trace:west","fingerprint":"update issues set state = ? where id = ?"}
`
	sameWriter := []byte(evidencePrefix + `{"type":"row_mutation","record":"record:issues/42","sql":"sql:east","after":{"state":"closed"},"region":"us-east"}
{"type":"row_mutation","record":"record:issues/42","sql":"sql:west","after":{"state":"reopened"},"region":"us-east"}
`)
	got, _, err = DetectSignal(InputEvidence, "split-brain-conflicting-writes", sameWriter, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got {
		t.Fatal("same-writer divergent rows should not trigger split-brain detector")
	}
	sameAfter := []byte(evidencePrefix + `{"type":"row_mutation","record":"record:issues/42","sql":"sql:east","after":{"state":"closed"},"region":"us-east"}
{"type":"row_mutation","record":"record:issues/42","sql":"sql:west","after":{"state":"closed"},"region":"us-west"}
`)
	got, _, err = DetectSignal(InputEvidence, "split-brain-conflicting-writes", sameAfter, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got {
		t.Fatal("same-after multi-region rows should not trigger split-brain detector")
	}
}

func TestReadSpecRejectsUnknownFields(t *testing.T) {
	_, err := ReadSpec(strings.NewReader(`{"version":"patchline.incident-postmortem-import/v1","name":"x","historical_suite":"suite.json","surprise":true}`))
	if err == nil {
		t.Fatal("expected unknown field error")
	}
}

func hasRegressionSignal(report Report, signalID string) bool {
	for _, regression := range report.Regressions {
		if regression.DetectorSignalID == signalID {
			return true
		}
	}
	return false
}
