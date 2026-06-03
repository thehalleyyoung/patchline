package artifact

import (
	"strings"
	"testing"

	"github.com/thehalleyyoung/patchline/internal/attest"
)

func TestFederatedBenchmarkAggregatePublishesOnlySignedThresholdedMetrics(t *testing.T) {
	split, seed := federatedBenchmarkTestSplit(t)
	aggregate, err := RunFederatedBenchmarkAggregate(FederatedBenchmarkRunOptions{
		SplitPath: splitPathForTest(t, split),
		SeedHex:   seed,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !aggregate.OK || aggregate.PrivateCaseCount != 3 || aggregate.PublicCaseCount != 1 {
		t.Fatalf("unexpected aggregate: %#v", aggregate)
	}
	if aggregate.Metrics.Total != 3 || aggregate.Metrics.Buckets["matched"] != 3 || aggregate.Metrics.Buckets["actual:flag"] != 3 {
		t.Fatalf("expected only aggregate metric buckets, got %#v", aggregate.Metrics)
	}
	if strings.Contains(string(federatedAggregatePayloadBytes(aggregate)), "private-broad-update-1") {
		t.Fatalf("signed aggregate payload leaked a private case id: %s", string(federatedAggregatePayloadBytes(aggregate)))
	}
	if err := attest.VerifySignature(aggregate.Signature, federatedAggregatePayloadBytes(aggregate)); err != nil {
		t.Fatal(err)
	}
	verification := VerifyFederatedBenchmarkAggregate(aggregate)
	if !verification.OK {
		t.Fatalf("expected aggregate verification to pass: %#v", verification.Errors)
	}
}

func TestFederatedBenchmarkAggregateRejectsTamperedMetrics(t *testing.T) {
	split, seed := federatedBenchmarkTestSplit(t)
	aggregate, err := RunFederatedBenchmarkAggregate(FederatedBenchmarkRunOptions{
		SplitPath: splitPathForTest(t, split),
		SeedHex:   seed,
	})
	if err != nil {
		t.Fatal(err)
	}
	aggregate.Metrics.Buckets["matched"] = 99
	aggregate.Hash = federatedAggregateHash(aggregate)
	verification := VerifyFederatedBenchmarkAggregate(aggregate)
	if verification.OK {
		t.Fatalf("expected tampered metrics to fail signature verification")
	}
	if !strings.Contains(strings.Join(verification.Errors, "\n"), "artifact hash mismatch") {
		t.Fatalf("expected artifact hash mismatch, got %#v", verification.Errors)
	}
}

func TestFederatedBenchmarkAggregateRejectsLeakyLowCountBucket(t *testing.T) {
	split, seed := federatedBenchmarkTestSplit(t)
	aggregate, err := RunFederatedBenchmarkAggregate(FederatedBenchmarkRunOptions{
		SplitPath: splitPathForTest(t, split),
		SeedHex:   seed,
	})
	if err != nil {
		t.Fatal(err)
	}
	aggregate.Metrics.Buckets["actual:pass"] = 1
	aggregate.Signature, err = attest.Sign(federatedAggregateSubject(aggregate.AdopterID), federatedAggregatePayloadBytes(aggregate), seedBytes(t, seed))
	if err != nil {
		t.Fatal(err)
	}
	aggregate.Hash = federatedAggregateHash(aggregate)
	verification := VerifyFederatedBenchmarkAggregate(aggregate)
	if verification.OK {
		t.Fatalf("expected low-count bucket to fail privacy verification")
	}
	if !strings.Contains(strings.Join(verification.Errors, "\n"), "below min_private_cases") {
		t.Fatalf("expected low-count privacy error, got %#v", verification.Errors)
	}
}

func TestCreateFederatedBenchmarkSplitRejectsTinyPrivateSplit(t *testing.T) {
	manifest := federatedBenchmarkTestManifest(t)
	_, err := CreateFederatedBenchmarkSplit(FederatedBenchmarkSplitOptions{
		ManifestPath:    manifest,
		AdopterID:       "tiny-adopter",
		PrivateCases:    []string{"private-broad-update-1"},
		MinPrivateCases: 3,
		PartitionSalt:   strings.Repeat("0a", 16),
	})
	if err == nil {
		t.Fatal("expected tiny private split to be rejected")
	}
	if !strings.Contains(err.Error(), "below k-anonymity minimum") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func federatedBenchmarkTestSplit(t *testing.T) (FederatedBenchmarkSplit, string) {
	t.Helper()
	manifest := federatedBenchmarkTestManifest(t)
	split, err := CreateFederatedBenchmarkSplit(FederatedBenchmarkSplitOptions{
		ManifestPath: manifest,
		AdopterID:    "adopter-alpha",
		PrivateCases: []string{
			"private-broad-update-1",
			"private-broad-update-2",
			"private-broad-update-3",
		},
		MinPrivateCases: 3,
		PartitionSalt:   strings.Repeat("0b", 16),
	})
	if err != nil {
		t.Fatal(err)
	}
	return split, strings.Repeat("01", 32)
}

func federatedBenchmarkTestManifest(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	writeBenchmarkTestFile(t, dir, "fixtures/private-broad-update-1.sql", "UPDATE accounts SET repaired = true;\n")
	writeBenchmarkTestFile(t, dir, "fixtures/private-broad-update-2.sql", "UPDATE users SET repaired = true;\n")
	writeBenchmarkTestFile(t, dir, "fixtures/private-broad-update-3.sql", "UPDATE invoices SET repaired = true;\n")
	writeBenchmarkTestFile(t, dir, "fixtures/public-safe.sql", "UPDATE invoices SET repaired = true WHERE id = 1;\n")
	for _, id := range []string{"private-broad-update-1", "private-broad-update-2", "private-broad-update-3"} {
		writeBenchmarkTestFile(t, dir, "ground_truth/"+id+".json", `{
  "case_id": "`+id+`",
  "case_type": "migration",
  "phase": "pre_deploy",
  "labels": {"expected_result": "flag", "risk": "high"},
  "evidence": [{"kind": "fixture", "locator": "fixtures/`+id+`.sql", "rationale": "unscoped update should be flagged"}],
  "allowed_inputs": ["migration_text"],
  "excluded_inputs": ["postmortem_text"]
}`)
	}
	writeBenchmarkTestFile(t, dir, "ground_truth/public-safe.json", `{
  "case_id": "public-safe",
  "case_type": "migration",
  "phase": "pre_deploy",
  "labels": {"expected_result": "pass", "risk": "low"},
  "evidence": [{"kind": "fixture", "locator": "fixtures/public-safe.sql", "rationale": "scoped update should pass"}],
  "allowed_inputs": ["migration_text"],
  "excluded_inputs": ["postmortem_text"]
}`)
	writeBenchmarkTestFile(t, dir, "manifests/federated.json", `{
  "version": "patchline.artifact-benchmark/v1",
  "dataset_id": "federated-test",
  "description": "federated benchmark split test",
  "cases": [
    {"case_id": "private-broad-update-1", "case_type": "migration", "available_at": "pre_deploy", "fixture": "../fixtures/private-broad-update-1.sql", "ground_truth": "../ground_truth/private-broad-update-1.json"},
    {"case_id": "private-broad-update-2", "case_type": "migration", "available_at": "pre_deploy", "fixture": "../fixtures/private-broad-update-2.sql", "ground_truth": "../ground_truth/private-broad-update-2.json"},
    {"case_id": "private-broad-update-3", "case_type": "migration", "available_at": "pre_deploy", "fixture": "../fixtures/private-broad-update-3.sql", "ground_truth": "../ground_truth/private-broad-update-3.json"},
    {"case_id": "public-safe", "case_type": "migration", "available_at": "pre_deploy", "fixture": "../fixtures/public-safe.sql", "ground_truth": "../ground_truth/public-safe.json"}
  ]
}`)
	return dir + "/manifests/federated.json"
}

func splitPathForTest(t *testing.T, split FederatedBenchmarkSplit) string {
	t.Helper()
	path := t.TempDir() + "/split.json"
	if err := WriteFederatedBenchmarkSplit(path, split); err != nil {
		t.Fatal(err)
	}
	return path
}

func seedBytes(t *testing.T, value string) []byte {
	t.Helper()
	seed, err := attest.SeedFromHex(value)
	if err != nil {
		t.Fatal(err)
	}
	return seed
}
