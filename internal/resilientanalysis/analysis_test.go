package resilientanalysis

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildReportVerifiesResilientDistributedRun(t *testing.T) {
	root, spec := testResilientSpec(t)
	report, err := BuildReport(spec, root)
	if err != nil {
		t.Fatalf("BuildReport failed: %v", err)
	}
	if !report.OK {
		t.Fatalf("expected resilient report to pass: %#v", report.Counterexamples)
	}
	if report.Summary.CompletedTasks != 3 || report.Summary.RecoveredWorkerLossTasks != 1 || report.Summary.CorruptCaches != 1 || report.Summary.QuarantinedCaches != 1 || report.Summary.RebuiltCaches != 1 || report.Summary.RecoveredPartitions != 1 {
		t.Fatalf("unexpected summary: %#v", report.Summary)
	}
	if report.Summary.PartitionRecoveredTasks < 2 {
		t.Fatalf("expected partition-affected tasks to complete, got %#v", report.Summary)
	}
	if report.Hash == "" {
		t.Fatal("expected deterministic report hash")
	}
	repeat, err := BuildReport(spec, root)
	if err != nil {
		t.Fatalf("repeat BuildReport failed: %v", err)
	}
	if repeat.Hash != report.Hash {
		t.Fatalf("expected deterministic hash, got %s then %s", report.Hash, repeat.Hash)
	}
}

func TestBuildReportRejectsResilienceRegressions(t *testing.T) {
	root, spec := testResilientSpec(t)
	spec.Events = append([]Event(nil), spec.Events...)
	filtered := spec.Events[:0]
	for _, event := range spec.Events {
		if event.Kind == "task_reassigned" || event.Kind == "cache_quarantined" || event.Kind == "cache_rebuilt" {
			continue
		}
		filtered = append(filtered, event)
	}
	spec.Events = append(filtered, Event{
		ID:           "complete-accounts-duplicate",
		Tick:         8,
		Kind:         "task_completed",
		TaskID:       "accounts-backfill-risk",
		WorkerID:     "worker-a",
		Attempt:      1,
		ResultSHA256: spec.Tasks[0].ExpectedResultSHA256,
		Accepted:     true,
	})
	spec.Partitions[0].RecoveredTick = 0
	spec.CacheArtifacts[0].RebuiltPath = ""

	report, err := BuildReport(spec, root)
	if err != nil {
		t.Fatalf("BuildReport failed: %v", err)
	}
	for _, kind := range []string{
		"worker_lost_without_reassignment",
		"corrupt_cache_not_quarantined",
		"corrupt_cache_not_rebuilt",
		"partition_not_recovered",
		"duplicate_accepted_completion",
	} {
		if !hasCounterexample(report, kind) {
			t.Fatalf("expected counterexample %q in %#v", kind, report.Counterexamples)
		}
	}
}

func TestReadSpecRejectsUnknownFields(t *testing.T) {
	_, err := ReadSpec(strings.NewReader(`{"version":"patchline.resilient-analysis/v1","name":"x","criteria":{},"workers":[],"tasks":[],"cache_artifacts":[],"partitions":[],"events":[],"extra":true}`))
	if err == nil {
		t.Fatal("expected unknown field rejection")
	}
}

func testResilientSpec(t *testing.T) (string, Spec) {
	t.Helper()
	root := t.TempDir()
	writeResilientTestFile(t, root, "evidence/runbook.md", "resilient analysis runbook\n")
	writeResilientTestFile(t, root, "evidence/partition-log.jsonl", `{"partition":"az-a-az-c","state":"started"}
{"partition":"az-a-az-c","state":"recovered"}
`)
	writeResilientTestFile(t, root, "evidence/cache-manifest.md", "cache manifest records quarantine and rebuild\n")
	writeResilientTestFile(t, root, "evidence/worker-loss.md", "worker-b lost during tenant shard sweep; task reassigned to worker-c\n")
	writeResilientTestFile(t, root, "results/accounts-backfill-risk.json", `{"task":"accounts-backfill-risk","risk_count":12}
`)
	writeResilientTestFile(t, root, "results/tenant-shard-sweep.json", `{"task":"tenant-shard-sweep","risk_count":9}
`)
	writeResilientTestFile(t, root, "cache/invoice-delete-cache.corrupt.json", `{"task":"invoice-delete-cache","risk_count":0,"stale":true}
`)
	writeResilientTestFile(t, root, "cache/invoice-delete-cache.rebuilt.json", `{"task":"invoice-delete-cache","risk_count":7,"rebuilt":true}
`)
	accountsHash := resilientTestHash(t, root, "results/accounts-backfill-risk.json")
	tenantHash := resilientTestHash(t, root, "results/tenant-shard-sweep.json")
	invoiceHash := resilientTestHash(t, root, "cache/invoice-delete-cache.rebuilt.json")
	return root, Spec{
		Version: SpecVersion,
		Name:    "test resilient distributed analysis",
		Claim:   "Patchline verifies a distributed analysis mode by replaying worker leases, failures, partitions, cache hashes, quarantine events, rebuilds, and accepted completions.",
		Criteria: Criteria{
			MinTasks:                             3,
			MinWorkers:                           3,
			RequireWorkerLossRecovery:            true,
			RequireCacheQuarantine:               true,
			RequireCacheRebuild:                  true,
			RequirePartitionRecovery:             true,
			RequireDeterministicLeases:           true,
			RequireNoDuplicateAcceptedCompletion: true,
			RequireEvidenceHashes:                true,
		},
		Workers: []Worker{
			{ID: "worker-a", Zone: "az-a", InitiallyHealthy: true},
			{ID: "worker-b", Zone: "az-b", InitiallyHealthy: true},
			{ID: "worker-c", Zone: "az-c", InitiallyHealthy: true},
		},
		Tasks: []Task{
			{ID: "accounts-backfill-risk", Repo: "patchline/self", Subpath: "db/migrate", Shard: "00", ExpectedResultSHA256: accountsHash, EvidencePaths: []string{"evidence/runbook.md", "evidence/partition-log.jsonl"}},
			{ID: "tenant-shard-sweep", Repo: "patchline/self", Subpath: "services/tenant", Shard: "01", ExpectedResultSHA256: tenantHash, EvidencePaths: []string{"evidence/worker-loss.md"}},
			{ID: "invoice-delete-cache", Repo: "patchline/self", Subpath: "services/billing", Shard: "02", ExpectedResultSHA256: invoiceHash, EvidencePaths: []string{"evidence/cache-manifest.md"}},
		},
		CacheArtifacts: []CacheArtifact{{
			ID:             "invoice-delete-cache-artifact",
			TaskID:         "invoice-delete-cache",
			Path:           "cache/invoice-delete-cache.corrupt.json",
			ExpectedSHA256: invoiceHash,
			RebuiltPath:    "cache/invoice-delete-cache.rebuilt.json",
			EvidencePaths:  []string{"evidence/cache-manifest.md"},
		}},
		Partitions: []Partition{{
			ID:              "az-a-az-c-partition",
			AffectedWorkers: []string{"worker-a", "worker-c"},
			StartTick:       2,
			RecoveredTick:   5,
			EvidencePaths:   []string{"evidence/partition-log.jsonl"},
		}},
		Events: []Event{
			{ID: "lease-accounts-a1", Tick: 1, Kind: "lease_acquired", TaskID: "accounts-backfill-risk", WorkerID: "worker-a", Attempt: 1, LeaseID: ExpectedLeaseID("accounts-backfill-risk", 1, "worker-a"), EvidencePaths: []string{"evidence/runbook.md"}},
			{ID: "lease-tenant-b1", Tick: 1, Kind: "lease_acquired", TaskID: "tenant-shard-sweep", WorkerID: "worker-b", Attempt: 1, LeaseID: ExpectedLeaseID("tenant-shard-sweep", 1, "worker-b"), EvidencePaths: []string{"evidence/worker-loss.md"}},
			{ID: "lease-invoice-c1", Tick: 2, Kind: "lease_acquired", TaskID: "invoice-delete-cache", WorkerID: "worker-c", Attempt: 1, LeaseID: ExpectedLeaseID("invoice-delete-cache", 1, "worker-c"), EvidencePaths: []string{"evidence/cache-manifest.md"}},
			{ID: "worker-b-lost", Tick: 3, Kind: "worker_lost", WorkerID: "worker-b", EvidencePaths: []string{"evidence/worker-loss.md"}},
			{ID: "tenant-reassigned-c2", Tick: 4, Kind: "task_reassigned", TaskID: "tenant-shard-sweep", WorkerID: "worker-c", Attempt: 2, EvidencePaths: []string{"evidence/worker-loss.md"}},
			{ID: "lease-tenant-c2", Tick: 4, Kind: "lease_acquired", TaskID: "tenant-shard-sweep", WorkerID: "worker-c", Attempt: 2, LeaseID: ExpectedLeaseID("tenant-shard-sweep", 2, "worker-c"), EvidencePaths: []string{"evidence/worker-loss.md"}},
			{ID: "invoice-cache-quarantined", Tick: 5, Kind: "cache_quarantined", ArtifactID: "invoice-delete-cache-artifact", EvidencePaths: []string{"evidence/cache-manifest.md"}},
			{ID: "invoice-cache-rebuilt", Tick: 6, Kind: "cache_rebuilt", ArtifactID: "invoice-delete-cache-artifact", EvidencePaths: []string{"evidence/cache-manifest.md"}},
			{ID: "complete-accounts", Tick: 6, Kind: "task_completed", TaskID: "accounts-backfill-risk", WorkerID: "worker-a", Attempt: 1, ResultSHA256: accountsHash, Accepted: true, EvidencePaths: []string{"results/accounts-backfill-risk.json"}},
			{ID: "complete-tenant", Tick: 7, Kind: "task_completed", TaskID: "tenant-shard-sweep", WorkerID: "worker-c", Attempt: 2, ResultSHA256: tenantHash, Accepted: true, EvidencePaths: []string{"results/tenant-shard-sweep.json"}},
			{ID: "complete-invoice", Tick: 7, Kind: "task_completed", TaskID: "invoice-delete-cache", WorkerID: "worker-c", Attempt: 1, ResultSHA256: invoiceHash, Accepted: true, EvidencePaths: []string{"cache/invoice-delete-cache.rebuilt.json"}},
		},
		EvidencePaths: []string{"evidence/runbook.md", "evidence/partition-log.jsonl"},
	}
}

func writeResilientTestFile(t *testing.T, root, rel, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func resilientTestHash(t *testing.T, root, rel string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func hasCounterexample(report Report, kind string) bool {
	for _, counterexample := range report.Counterexamples {
		if counterexample.Kind == kind {
			return true
		}
	}
	return false
}
