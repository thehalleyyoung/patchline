package canaryvalidate

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/thehalleyyoung/patchline/internal/replay"
)

func TestBuildReportChecksRedactedPrePostSnapshots(t *testing.T) {
	report, err := BuildReport(validSpec(), beforeSnapshot(), afterSnapshotGood())
	if err != nil {
		t.Fatal(err)
	}
	if !report.OK || report.Summary.Checked != 6 || report.Summary.Refuted != 0 || report.Summary.MatchedRows != 3 {
		t.Fatalf("expected checked canary report, got %#v", report)
	}
	if !report.Privacy.HashOnlyEvidence || report.Privacy.RawValuesEmitted || report.Privacy.RedactionSaltEmitted {
		t.Fatalf("expected hash-only privacy report, got %#v", report.Privacy)
	}
	if report.Sample.RedactionSaltHash == "" || strings.Contains(report.Sample.RedactionSaltHash, "canary-test-salt") {
		t.Fatalf("expected hashed, non-raw salt evidence, got %q", report.Sample.RedactionSaltHash)
	}
	firstHash := report.Hash
	again, err := BuildReport(validSpec(), beforeSnapshot(), afterSnapshotGood())
	if err != nil {
		t.Fatal(err)
	}
	if firstHash == "" || firstHash != again.Hash {
		t.Fatalf("expected deterministic report hash, first=%q second=%q", firstHash, again.Hash)
	}
	bytes, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	for _, raw := range []string{"inv-1", "acct-a", "paid", "canary-test-salt"} {
		if strings.Contains(string(bytes), raw) {
			t.Fatalf("report leaked raw canary value %q: %s", raw, string(bytes))
		}
	}
}

func TestBuildReportRefutesBrokenPostSnapshot(t *testing.T) {
	report, err := BuildReport(validSpec(), beforeSnapshot(), afterSnapshotBad())
	if err != nil {
		t.Fatal(err)
	}
	if report.OK || report.Summary.Checked != 1 || report.Summary.Refuted != 5 {
		t.Fatalf("expected one checked and five refuted invariants, got %#v", report.Summary)
	}
	statuses := invariantStatuses(report.Invariants)
	for _, id := range []string{"external-id-derived", "external-id-not-null", "external-id-unique", "only-external-id-changes", "stable-business-fields"} {
		if statuses[id] != "refuted" {
			t.Fatalf("expected %s to be refuted, got statuses %#v", id, statuses)
		}
	}
	if statuses["invoice-row-count"] != "checked" {
		t.Fatalf("expected row count to stay checked, got statuses %#v", statuses)
	}
	codes := violationCodes(report.Invariants)
	for _, code := range []string{"target_missing", "duplicate_value", "unexpected_change", "disallowed_column_change"} {
		if !codes[code] {
			t.Fatalf("expected violation code %s, got %#v", code, codes)
		}
	}
}

func TestBuildReportRequiresRedactedProductionLikeProtocol(t *testing.T) {
	spec := validSpec()
	spec.SamplePolicy.Redacted = false
	spec.SamplePolicy.ProductionLike = false
	report, err := BuildReport(spec, beforeSnapshot(), afterSnapshotGood())
	if err != nil {
		t.Fatal(err)
	}
	if report.OK || report.Summary.ProtocolRefuted != 2 {
		t.Fatalf("expected redaction and production-like protocol checks to fail, got %#v", report)
	}
}

func TestReadSpecRejectsUnknownFields(t *testing.T) {
	_, err := ReadSpec(strings.NewReader(`{"version":"patchline.canary-validation/v1","name":"x","sample_policy":{"redacted":true,"production_like":true,"redaction_salt":"s"},"invariants":[],"extra":true}`))
	if err == nil {
		t.Fatal("expected unknown field error")
	}
}

func validSpec() Spec {
	return Spec{
		Version: SpecVersion,
		Name:    "invoice external id canary validation",
		SamplePolicy: SamplePolicy{
			Source:         "redacted billing replica sample",
			Redacted:       true,
			ProductionLike: true,
			SamplingBasis:  "deterministic tenant-stratified hash sample",
			ExpectedRows:   3,
			MinRows:        3,
			MinMatchedRows: 3,
			RedactionSalt:  "canary-test-salt",
		},
		Invariants: []InvariantSpec{{
			ID: "invoice-row-count", Kind: "row_count", Table: "invoices", AllowedDelta: 0,
		}, {
			ID: "external-id-not-null", Kind: "not_null", Table: "invoices", Columns: []string{"external_id"},
		}, {
			ID: "external-id-unique", Kind: "unique", Table: "invoices", Columns: []string{"external_id"},
		}, {
			ID: "external-id-derived", Kind: "equals", Table: "invoices", SourceColumn: "legacy_external_id", TargetColumn: "external_id",
		}, {
			ID: "stable-business-fields", Kind: "unchanged", Table: "invoices", Columns: []string{"account_id", "amount_cents", "status"},
		}, {
			ID: "only-external-id-changes", Kind: "changed_only", Table: "invoices", AllowedChangeColumns: []string{"external_id"},
		}},
	}
}

func beforeSnapshot() replay.Store {
	return replay.Store{Tables: map[string]map[string]replay.Row{
		"invoices": {
			"1": {"id": "1", "account_id": "acct-a", "amount_cents": "1000", "status": "paid", "legacy_external_id": "inv-1", "external_id": ""},
			"2": {"id": "2", "account_id": "acct-b", "amount_cents": "2000", "status": "open", "legacy_external_id": "inv-2", "external_id": ""},
			"3": {"id": "3", "account_id": "acct-c", "amount_cents": "3000", "status": "paid", "legacy_external_id": "inv-3", "external_id": ""},
		},
	}}
}

func afterSnapshotGood() replay.Store {
	return replay.Store{Tables: map[string]map[string]replay.Row{
		"invoices": {
			"1": {"id": "1", "account_id": "acct-a", "amount_cents": "1000", "status": "paid", "legacy_external_id": "inv-1", "external_id": "inv-1"},
			"2": {"id": "2", "account_id": "acct-b", "amount_cents": "2000", "status": "open", "legacy_external_id": "inv-2", "external_id": "inv-2"},
			"3": {"id": "3", "account_id": "acct-c", "amount_cents": "3000", "status": "paid", "legacy_external_id": "inv-3", "external_id": "inv-3"},
		},
	}}
}

func afterSnapshotBad() replay.Store {
	return replay.Store{Tables: map[string]map[string]replay.Row{
		"invoices": {
			"1": {"id": "1", "account_id": "acct-a", "amount_cents": "1000", "status": "disabled", "legacy_external_id": "inv-1", "external_id": "inv-1"},
			"2": {"id": "2", "account_id": "acct-b", "amount_cents": "2000", "status": "open", "legacy_external_id": "inv-2", "external_id": "inv-1"},
			"3": {"id": "3", "account_id": "acct-c", "amount_cents": "3000", "status": "paid", "legacy_external_id": "inv-3", "external_id": ""},
		},
	}}
}

func invariantStatuses(results []InvariantResult) map[string]string {
	out := map[string]string{}
	for _, result := range results {
		out[result.ID] = result.Status
	}
	return out
}

func violationCodes(results []InvariantResult) map[string]bool {
	out := map[string]bool{}
	for _, result := range results {
		for _, violation := range result.Violations {
			out[violation.Code] = true
		}
	}
	return out
}
