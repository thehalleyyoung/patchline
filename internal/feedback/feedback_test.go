package feedback

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/thehalleyyoung/patchline/internal/canonical"
)

const fixture = `{
  "version": "patchline.live-feedback-ingestion/v1",
  "adopter_id": "team-alpha",
  "salt": "local-secret-feedback-salt-2026",
  "min_group_size": 1,
  "outcomes": [
    {"finding_id":"finding-001","detector":"sql.destructive-ddl","release":"v1.0.0","confidence":0.93,"verdict":"confirmed","action":"blocked","burden_minutes":9,"evidence_hash":"ev-001","reviewer_role":"maintainer"},
    {"finding_id":"finding-002","detector":"sql.destructive-ddl","release":"v1.0.0","confidence":0.91,"verdict":"confirmed","action":"blocked","burden_minutes":11,"evidence_hash":"ev-002","reviewer_role":"dba"},
    {"finding_id":"finding-003","detector":"sql.destructive-ddl","release":"v1.0.0","confidence":0.95,"verdict":"confirmed","action":"blocked","burden_minutes":10,"evidence_hash":"ev-003","reviewer_role":"sre"},
    {"finding_id":"finding-004","detector":"orm.write-breadth","release":"v1.0.0","confidence":0.73,"verdict":"false_positive","action":"dismissed","burden_minutes":4,"evidence_hash":"ev-004","reviewer_role":"maintainer"},
    {"finding_id":"finding-005","detector":"orm.write-breadth","release":"v1.0.0","confidence":0.78,"verdict":"false_positive","action":"dismissed","burden_minutes":5,"evidence_hash":"ev-005","reviewer_role":"dba"},
    {"finding_id":"finding-006","detector":"orm.write-breadth","release":"v1.0.0","confidence":0.71,"verdict":"false_positive","action":"dismissed","burden_minutes":6,"evidence_hash":"ev-006","reviewer_role":"sre"},
    {"finding_id":"finding-007","detector":"migration.guard","release":"v1.0.0","confidence":0.52,"verdict":"uncertain","action":"deferred","burden_minutes":12,"evidence_hash":"ev-007","reviewer_role":"security"},
    {"finding_id":"finding-007","detector":"migration.guard","release":"v1.0.0","confidence":0.52,"verdict":"uncertain","action":"deferred","burden_minutes":12,"evidence_hash":"ev-007","reviewer_role":"security"},
    {"finding_id":"finding-008","detector":"sql.raw-leak","release":"v1.0.0","confidence":0.8,"verdict":"confirmed","action":"blocked","burden_minutes":3,"evidence_hash":"ev-008","reviewer_role":"maintainer","details":{"source_code":"DROP TABLE accounts;"}},
    {"finding_id":"finding-009","detector":"UPDATE accounts SET plan = 'free'","release":"v1.0.0","confidence":0.8,"verdict":"confirmed","action":"blocked","burden_minutes":3,"evidence_hash":"ev-009","reviewer_role":"maintainer"},
    {"finding_id":"finding-010","detector":"sql.destructive-ddl","release":"v1.0.0","confidence":1.2,"verdict":"confirmed","action":"blocked","burden_minutes":3,"evidence_hash":"ev-010","reviewer_role":"maintainer"}
  ]
}`

func TestIngestProducesSourceFreeAggregates(t *testing.T) {
	report, err := Ingest(strings.NewReader(fixture), Options{})
	if err != nil {
		t.Fatal(err)
	}
	if !report.OK || !report.Shareable || !report.Privacy.SourceFree || !report.Privacy.RawEvidenceFree || !report.Privacy.IdentifierFree || report.Privacy.SaltEmitted {
		t.Fatalf("unexpected privacy report: %#v", report)
	}
	if report.Summary.InputRecords != 11 || report.Summary.AcceptedRecords != 7 || report.Summary.RejectedRecords != 3 || report.Summary.DeduplicatedRecords != 1 {
		t.Fatalf("unexpected summary: %#v", report.Summary)
	}
	if report.Summary.RequestedMinGroupSize != 1 || report.Summary.EffectiveMinGroupSize != MinKFloor {
		t.Fatalf("privacy floor was not enforced: %#v", report.Summary)
	}
	if len(report.Groups) != 2 {
		t.Fatalf("expected two published groups, got %#v", report.Groups)
	}
	for _, group := range report.Groups {
		if group.Count < report.Summary.EffectiveMinGroupSize {
			t.Fatalf("published low-count group: %#v", group)
		}
	}
	if report.Residual.Count != 1 || len(report.Residual.OutcomeCounts) != 1 || report.Residual.OutcomeCounts[0].Verdict != "uncertain" {
		t.Fatalf("expected dimension-free residual for suppressed group, got %#v", report.Residual)
	}
}

func TestIngestDoesNotMarshalRawSourceOrIdentifiers(t *testing.T) {
	report, err := Ingest(strings.NewReader(fixture), Options{})
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	serialized := string(data)
	for _, forbidden := range []string{
		"DROP TABLE", "UPDATE accounts", "source_code", "raw_evidence", "finding_id",
		"evidence_hash", "finding-001", "ev-001", "team-alpha", "local-secret-feedback-salt-2026",
	} {
		if strings.Contains(serialized, forbidden) {
			t.Fatalf("report leaked %q:\n%s", forbidden, serialized)
		}
	}
	for _, rejected := range report.Rejected {
		if strings.Contains(rejected.Reason, "source_code") || strings.Contains(rejected.Reason, "details") {
			t.Fatalf("rejection reason leaked raw field names: %#v", rejected)
		}
	}
}

func TestIngestRejectsNestedRawFieldAndAllowedSourceLikeValue(t *testing.T) {
	input := `{
	  "version":"patchline.live-feedback-ingestion/v1",
	  "adopter_id":"team-beta",
	  "salt":"another-local-secret-salt",
	  "min_group_size":3,
	  "outcomes":[
	    {"finding_id":"f-1","detector":"sql.detector","release":"v1","confidence":0.4,"verdict":"confirmed","action":"blocked","burden_minutes":1,"evidence_hash":"ev-1","reviewer_role":"maintainer","metadata":{"nested":{"diff":"diff --git a/db/migrate/x b/db/migrate/x"}}},
	    {"finding_id":"f-2","detector":"class DangerousMigration","release":"v1","confidence":0.4,"verdict":"confirmed","action":"blocked","burden_minutes":1,"evidence_hash":"ev-2","reviewer_role":"maintainer"}
	  ]
	}`
	report, err := Ingest(strings.NewReader(input), Options{})
	if err != nil {
		t.Fatal(err)
	}
	if report.Summary.AcceptedRecords != 0 || report.Summary.RejectedRecords != 2 {
		t.Fatalf("expected both source-like records rejected: %#v", report.Summary)
	}
	reasons := map[string]bool{}
	for _, rejected := range report.Rejected {
		reasons[rejected.Reason] = true
	}
	if !reasons["blocked_raw_field"] || !reasons["source_like_value"] {
		t.Fatalf("expected both privacy rejection reasons, got %#v", report.Rejected)
	}
}

func TestIngestRejectsShortSaltAndInvalidEnums(t *testing.T) {
	shortSalt := strings.Replace(fixture, "local-secret-feedback-salt-2026", "short", 1)
	if _, err := Ingest(strings.NewReader(shortSalt), Options{}); err == nil {
		t.Fatal("expected short salt rejection")
	}
	invalid := strings.Replace(fixture, `"verdict":"confirmed"`, `"verdict":"maybe"`, 1)
	report, err := Ingest(strings.NewReader(invalid), Options{})
	if err != nil {
		t.Fatal(err)
	}
	if report.Summary.RejectedRecords == 0 {
		t.Fatalf("expected invalid enum rejection: %#v", report)
	}
}

func TestIngestCohortUsesSecretSaltAndOutputIsDeterministic(t *testing.T) {
	first, err := Ingest(strings.NewReader(fixture), Options{})
	if err != nil {
		t.Fatal(err)
	}
	second, err := Ingest(strings.NewReader(fixture), Options{})
	if err != nil {
		t.Fatal(err)
	}
	changedSaltFixture := strings.Replace(fixture, "local-secret-feedback-salt-2026", "different-secret-salt-2026", 1)
	changed, err := Ingest(strings.NewReader(changedSaltFixture), Options{})
	if err != nil {
		t.Fatal(err)
	}
	if first.AdopterCohort == changed.AdopterCohort {
		t.Fatal("expected adopter cohort to change with secret salt")
	}
	firstBytes, err := canonical.Bytes(first)
	if err != nil {
		t.Fatal(err)
	}
	secondBytes, err := canonical.Bytes(second)
	if err != nil {
		t.Fatal(err)
	}
	if string(firstBytes) != string(secondBytes) {
		t.Fatalf("expected deterministic output:\n%s\n---\n%s", firstBytes, secondBytes)
	}
}
