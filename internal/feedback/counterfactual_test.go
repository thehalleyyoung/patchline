package feedback

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/thehalleyyoung/patchline/internal/canonical"
)

const counterfactualHistoryFixture = `{
  "version": "patchline.counterfactual-policy-history/v1",
  "name": "stage63-counterfactual-history",
  "claim": "Ordered source-free policy snapshots let Patchline reconstruct what earlier releases would have recommended for published k-anonymous reviewer-outcome groups.",
  "policies": [
    {
      "release": "v0.8.0",
      "thresholds": [
        {"detector":"orm.write-breadth","blocking_threshold":0.80},
        {"detector":"sql.destructive-ddl","blocking_threshold":0.99}
      ]
    },
    {
      "release": "v0.9.0",
      "thresholds": [
        {"detector":"orm.write-breadth","blocking_threshold":0.70},
        {"detector":"sql.destructive-ddl","blocking_threshold":0.90}
      ]
    },
    {
      "release": "v1.0.0",
      "thresholds": [
        {"detector":"orm.write-breadth","blocking_threshold":0.70},
        {"detector":"sql.destructive-ddl","blocking_threshold":0.90}
      ]
    }
  ]
}`

func TestCounterfactualLogReconstructsPreviousReleaseRecommendations(t *testing.T) {
	report, err := Ingest(strings.NewReader(fixture), Options{})
	if err != nil {
		t.Fatal(err)
	}
	history, err := ReadCounterfactualPolicyHistory(strings.NewReader(counterfactualHistoryFixture))
	if err != nil {
		t.Fatal(err)
	}
	log, err := ComputeCounterfactualLog(report, history)
	if err != nil {
		t.Fatal(err)
	}
	if log.Version != CounterfactualLogVersion || !log.OK || log.Hash == "" || log.EvidenceBasis != "published_k_anonymous_groups_only" {
		t.Fatalf("unexpected counterfactual log header: %#v", log)
	}
	if log.Summary.PublishedGroups != 2 || log.Summary.CounterfactualGroupsCompared != 4 || log.Summary.ComparedRecords != 12 {
		t.Fatalf("unexpected counterfactual comparison summary: %#v", log.Summary)
	}
	if log.Summary.ConfirmedWouldBlock != 3 || log.Summary.FalsePositiveWouldBlock != 3 || log.Summary.FalsePositiveWouldSpare != 3 || log.Summary.ConfirmedBoundaryAmbiguous != 3 {
		t.Fatalf("unexpected reconstructed recommendation counts: %#v", log.Summary.CounterfactualCounters)
	}
	if len(log.Releases) != 2 || log.Releases[0].PolicyRelease != "v0.8.0" || log.Releases[1].PolicyRelease != "v0.9.0" {
		t.Fatalf("expected only previous policy releases in declared order, got %#v", log.Releases)
	}
	if !hasCounterfactualEntry(log, "v0.8.0", "sql.destructive-ddl", "boundary_ambiguous") {
		t.Fatalf("expected threshold-inside-decile ambiguity entry, got %#v", log.Entries)
	}
	if !hasCounterfactualEntry(log, "v0.9.0", "orm.write-breadth", "would_block_false_positive") {
		t.Fatalf("expected prior release false-positive block entry, got %#v", log.Entries)
	}

	second, err := ComputeCounterfactualLog(report, history)
	if err != nil {
		t.Fatal(err)
	}
	firstBytes, err := canonical.Bytes(log)
	if err != nil {
		t.Fatal(err)
	}
	secondBytes, err := canonical.Bytes(second)
	if err != nil {
		t.Fatal(err)
	}
	if string(firstBytes) != string(secondBytes) || log.Hash != second.Hash {
		t.Fatalf("expected deterministic counterfactual log:\n%s\n---\n%s", firstBytes, secondBytes)
	}

	serialized, err := json.Marshal(log)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"finding-001", "ev-001", "local-secret-feedback-salt-2026", "DROP TABLE", "UPDATE accounts", "source_code", "evidence_hash"} {
		if strings.Contains(string(serialized), forbidden) {
			t.Fatalf("counterfactual log leaked source/raw identifier %q:\n%s", forbidden, serialized)
		}
	}
}

func TestCounterfactualLogDoesNotThresholdReclassifyMissedFindings(t *testing.T) {
	report := Report{
		Version: ReportVersion,
		Groups: []Group{{
			Detector:           "migration.guard",
			Release:            "v1.0.0",
			ConfidenceDecile:   "0.4-0.5",
			Verdict:            "missed",
			Action:             "fixed",
			Count:              3,
			TotalBurdenMinutes: 30,
		}},
	}
	history := CounterfactualPolicyHistory{
		Version: CounterfactualPolicyHistoryVersion,
		Name:    "missed-counterfactual-history",
		Policies: []CounterfactualPolicySnapshot{
			{Release: "v0.9.0", Thresholds: []DetectorThreshold{{Detector: "migration.guard", BlockingThreshold: 0.10}}},
			{Release: "v1.0.0", Thresholds: []DetectorThreshold{{Detector: "migration.guard", BlockingThreshold: 0.50}}},
		},
	}
	log, err := ComputeCounterfactualLog(report, history)
	if err != nil {
		t.Fatal(err)
	}
	if log.Summary.MissedNotEmitted != 3 || log.Summary.WouldBlock != 0 || log.Summary.WouldAllow != 0 {
		t.Fatalf("missed detector non-emissions should not be threshold-reclassified: %#v", log.Summary.CounterfactualCounters)
	}
	if len(log.Entries) != 1 || log.Entries[0].PreviousRecommendation != "not_emitted" || log.Entries[0].Classification != "would_still_be_missed" {
		t.Fatalf("unexpected missed counterfactual entry: %#v", log.Entries)
	}
}

func TestCounterfactualPolicyHistoryRejectsRawFields(t *testing.T) {
	raw := `{
	  "version":"patchline.counterfactual-policy-history/v1",
	  "name":"bad-history",
	  "policies":[
	    {"release":"v0.9.0","thresholds":[{"detector":"sql.destructive-ddl","blocking_threshold":0.9,"source_code":"DROP TABLE accounts;"}]},
	    {"release":"v1.0.0","thresholds":[{"detector":"sql.destructive-ddl","blocking_threshold":0.8}]}
	  ]
	}`
	if _, err := ReadCounterfactualPolicyHistory(strings.NewReader(raw)); err == nil {
		t.Fatal("expected counterfactual policy history with raw field to be rejected")
	}
}

func hasCounterfactualEntry(log CounterfactualLog, policyRelease, detector, classification string) bool {
	for _, entry := range log.Entries {
		if entry.PolicyRelease == policyRelease && entry.Detector == detector && entry.Classification == classification {
			return true
		}
	}
	return false
}
