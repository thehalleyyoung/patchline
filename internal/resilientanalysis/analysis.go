package resilientanalysis

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"

	"github.com/thehalleyyoung/patchline/internal/canonical"
)

const SpecVersion = "patchline.resilient-analysis/v1"
const ReportVersion = "patchline.resilient-analysis-report/v1"

type Spec struct {
	Version        string          `json:"version"`
	Name           string          `json:"name"`
	Claim          string          `json:"claim,omitempty"`
	Criteria       Criteria        `json:"criteria"`
	Workers        []Worker        `json:"workers"`
	Tasks          []Task          `json:"tasks"`
	CacheArtifacts []CacheArtifact `json:"cache_artifacts"`
	Partitions     []Partition     `json:"partitions"`
	Events         []Event         `json:"events"`
	EvidencePaths  []string        `json:"evidence_paths,omitempty"`
}

type Criteria struct {
	MinTasks                             int  `json:"min_tasks"`
	MinWorkers                           int  `json:"min_workers"`
	RequireWorkerLossRecovery            bool `json:"require_worker_loss_recovery"`
	RequireCacheQuarantine               bool `json:"require_cache_quarantine"`
	RequireCacheRebuild                  bool `json:"require_cache_rebuild"`
	RequirePartitionRecovery             bool `json:"require_partition_recovery"`
	RequireDeterministicLeases           bool `json:"require_deterministic_leases"`
	RequireNoDuplicateAcceptedCompletion bool `json:"require_no_duplicate_accepted_completion"`
	RequireEvidenceHashes                bool `json:"require_evidence_hashes"`
}

type Worker struct {
	ID               string `json:"id"`
	Zone             string `json:"zone"`
	InitiallyHealthy bool   `json:"initially_healthy"`
}

type Task struct {
	ID                   string   `json:"id"`
	Repo                 string   `json:"repo"`
	Subpath              string   `json:"subpath"`
	Shard                string   `json:"shard"`
	ExpectedResultSHA256 string   `json:"expected_result_sha256"`
	EvidencePaths        []string `json:"evidence_paths,omitempty"`
}

type CacheArtifact struct {
	ID             string   `json:"id"`
	TaskID         string   `json:"task_id"`
	Path           string   `json:"path"`
	ExpectedSHA256 string   `json:"expected_sha256"`
	RebuiltPath    string   `json:"rebuilt_path,omitempty"`
	EvidencePaths  []string `json:"evidence_paths,omitempty"`
}

type Partition struct {
	ID              string   `json:"id"`
	AffectedWorkers []string `json:"affected_workers"`
	StartTick       int      `json:"start_tick"`
	RecoveredTick   int      `json:"recovered_tick"`
	EvidencePaths   []string `json:"evidence_paths,omitempty"`
}

type Event struct {
	ID            string   `json:"id"`
	Tick          int      `json:"tick"`
	Kind          string   `json:"kind"`
	TaskID        string   `json:"task_id,omitempty"`
	WorkerID      string   `json:"worker_id,omitempty"`
	ArtifactID    string   `json:"artifact_id,omitempty"`
	Attempt       int      `json:"attempt,omitempty"`
	LeaseID       string   `json:"lease_id,omitempty"`
	ResultSHA256  string   `json:"result_sha256,omitempty"`
	Accepted      bool     `json:"accepted,omitempty"`
	EvidencePaths []string `json:"evidence_paths,omitempty"`
}

type Report struct {
	Version         string             `json:"version"`
	Name            string             `json:"name"`
	OK              bool               `json:"ok"`
	Criteria        Criteria           `json:"criteria"`
	Summary         Summary            `json:"summary"`
	Evidence        []ArtifactEvidence `json:"evidence,omitempty"`
	Workers         []WorkerReport     `json:"workers"`
	Tasks           []TaskReport       `json:"tasks"`
	CacheArtifacts  []CacheReport      `json:"cache_artifacts"`
	Partitions      []PartitionReport  `json:"partitions"`
	Counterexamples []Counterexample   `json:"counterexamples,omitempty"`
	Hash            string             `json:"hash"`
}

type Summary struct {
	Workers                      int `json:"workers"`
	Tasks                        int `json:"tasks"`
	CompletedTasks               int `json:"completed_tasks"`
	WorkerLossEvents             int `json:"worker_loss_events"`
	RecoveredWorkerLossTasks     int `json:"recovered_worker_loss_tasks"`
	CacheArtifacts               int `json:"cache_artifacts"`
	CorruptCaches                int `json:"corrupt_caches"`
	QuarantinedCaches            int `json:"quarantined_caches"`
	RebuiltCaches                int `json:"rebuilt_caches"`
	Partitions                   int `json:"partitions"`
	RecoveredPartitions          int `json:"recovered_partitions"`
	PartitionRecoveredTasks      int `json:"partition_recovered_tasks"`
	DeterministicLeases          int `json:"deterministic_leases"`
	DuplicateAcceptedCompletions int `json:"duplicate_accepted_completions"`
	EvidenceArtifacts            int `json:"evidence_artifacts"`
	Counterexamples              int `json:"counterexamples"`
}

type WorkerReport struct {
	ID               string   `json:"id"`
	Zone             string   `json:"zone"`
	InitiallyHealthy bool     `json:"initially_healthy"`
	Lost             bool     `json:"lost"`
	LostTick         int      `json:"lost_tick,omitempty"`
	RecoveredTasks   []string `json:"recovered_tasks,omitempty"`
}

type TaskReport struct {
	ID                         string             `json:"id"`
	Repo                       string             `json:"repo"`
	Subpath                    string             `json:"subpath"`
	Shard                      string             `json:"shard"`
	ExpectedResultSHA256       string             `json:"expected_result_sha256"`
	Attempts                   int                `json:"attempts"`
	AcceptedCompletions        int                `json:"accepted_completions"`
	Completed                  bool               `json:"completed"`
	CompletedWorker            string             `json:"completed_worker,omitempty"`
	CompletionTick             int                `json:"completion_tick,omitempty"`
	ResultSHA256               string             `json:"result_sha256,omitempty"`
	ReassignedAfterWorkerLoss  bool               `json:"reassigned_after_worker_loss"`
	ProgressedThroughPartition bool               `json:"progressed_through_partition"`
	LeaseIDs                   []string           `json:"lease_ids,omitempty"`
	Evidence                   []ArtifactEvidence `json:"evidence,omitempty"`
}

type CacheReport struct {
	ID              string             `json:"id"`
	TaskID          string             `json:"task_id"`
	ExpectedSHA256  string             `json:"expected_sha256"`
	Artifact        ArtifactEvidence   `json:"artifact"`
	Corrupt         bool               `json:"corrupt"`
	Quarantined     bool               `json:"quarantined"`
	Rebuilt         bool               `json:"rebuilt"`
	RebuiltArtifact ArtifactEvidence   `json:"rebuilt_artifact,omitempty"`
	Evidence        []ArtifactEvidence `json:"evidence,omitempty"`
}

type PartitionReport struct {
	ID              string             `json:"id"`
	AffectedWorkers []string           `json:"affected_workers"`
	StartTick       int                `json:"start_tick"`
	RecoveredTick   int                `json:"recovered_tick,omitempty"`
	Recovered       bool               `json:"recovered"`
	CompletedTasks  []string           `json:"completed_tasks,omitempty"`
	Evidence        []ArtifactEvidence `json:"evidence,omitempty"`
}

type ArtifactEvidence struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
	Bytes  int64  `json:"bytes"`
}

type Counterexample struct {
	ID      string   `json:"id"`
	Kind    string   `json:"kind"`
	Subject string   `json:"subject,omitempty"`
	Message string   `json:"message"`
	Witness []string `json:"witness,omitempty"`
}

func ReadSpec(reader io.Reader) (Spec, error) {
	decoder := json.NewDecoder(reader)
	decoder.DisallowUnknownFields()
	var spec Spec
	if err := decoder.Decode(&spec); err != nil {
		return Spec{}, err
	}
	if spec.Version != SpecVersion {
		return Spec{}, fmt.Errorf("resilient analysis spec version must be %s", SpecVersion)
	}
	return spec, nil
}

func BuildReport(spec Spec, root string) (Report, error) {
	if err := validateSpec(spec); err != nil {
		return Report{}, err
	}
	if root == "" {
		root = "."
	}
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return Report{}, err
	}
	criteria := spec.Criteria
	report := Report{
		Version:  ReportVersion,
		Name:     spec.Name,
		OK:       true,
		Criteria: criteria,
	}

	var counterexamples []Counterexample
	report.Evidence, counterexamples = collectArtifacts(rootAbs, spec.EvidencePaths, spec.Name, "run_evidence", "missing_evidence", "empty_evidence", "invalid_evidence_path", "resilient analysis run evidence could not be read")
	report.Summary.EvidenceArtifacts += len(report.Evidence)
	if criteria.RequireEvidenceHashes && len(report.Evidence) == 0 {
		counterexamples = append(counterexamples, Counterexample{
			ID:      "run." + stableID(spec.Name, "evidence") + ".missing",
			Kind:    "missing_evidence",
			Subject: spec.Name,
			Message: "resilient analysis mode does not cite readable run-level evidence",
		})
	}

	events := sortedEvents(spec.Events)
	eventsByTask := map[string][]Event{}
	acceptedByTask := map[string][]Event{}
	lostEvents := make([]Event, 0)
	quarantinedArtifacts := map[string]bool{}
	rebuiltArtifacts := map[string]bool{}
	for _, event := range events {
		if event.TaskID != "" {
			eventsByTask[event.TaskID] = append(eventsByTask[event.TaskID], event)
		}
		switch event.Kind {
		case "task_completed":
			if event.Accepted {
				acceptedByTask[event.TaskID] = append(acceptedByTask[event.TaskID], event)
			}
		case "worker_lost":
			lostEvents = append(lostEvents, event)
		case "cache_quarantined":
			quarantinedArtifacts[event.ArtifactID] = true
		case "cache_rebuilt":
			rebuiltArtifacts[event.ArtifactID] = true
		}
	}
	report.Summary.WorkerLossEvents = len(lostEvents)

	workerReports, recoveredByWorker := buildWorkerReports(spec.Workers, lostEvents)
	report.Workers = workerReports
	report.Summary.Workers = len(workerReports)

	taskReports, taskCounterexamples, taskSummary := buildTaskReports(rootAbs, spec.Tasks, eventsByTask, acceptedByTask, lostEvents, spec.Partitions, criteria)
	report.Tasks = taskReports
	counterexamples = append(counterexamples, taskCounterexamples...)
	report.Summary.Tasks = len(taskReports)
	report.Summary.CompletedTasks = taskSummary.completedTasks
	report.Summary.RecoveredWorkerLossTasks = taskSummary.recoveredWorkerLossTasks
	report.Summary.PartitionRecoveredTasks = taskSummary.partitionRecoveredTasks
	report.Summary.DeterministicLeases = taskSummary.deterministicLeases
	report.Summary.DuplicateAcceptedCompletions = taskSummary.duplicateAcceptedCompletions
	for _, task := range taskReports {
		report.Summary.EvidenceArtifacts += len(task.Evidence)
		if task.ReassignedAfterWorkerLoss {
			for _, event := range lostEvents {
				if taskImpactedByWorkerLoss(eventsByTask[task.ID], event) {
					recoveredByWorker[event.WorkerID] = append(recoveredByWorker[event.WorkerID], task.ID)
				}
			}
		}
	}
	for i := range report.Workers {
		report.Workers[i].RecoveredTasks = sortedStrings(uniqueStrings(recoveredByWorker[report.Workers[i].ID]))
	}

	cacheReports, cacheCounterexamples, cacheSummary := buildCacheReports(rootAbs, spec.CacheArtifacts, quarantinedArtifacts, rebuiltArtifacts, criteria)
	report.CacheArtifacts = cacheReports
	counterexamples = append(counterexamples, cacheCounterexamples...)
	report.Summary.CacheArtifacts = len(cacheReports)
	report.Summary.CorruptCaches = cacheSummary.corrupt
	report.Summary.QuarantinedCaches = cacheSummary.quarantined
	report.Summary.RebuiltCaches = cacheSummary.rebuilt
	for _, cache := range cacheReports {
		if cache.Artifact.Path != "" {
			report.Summary.EvidenceArtifacts++
		}
		if cache.RebuiltArtifact.Path != "" {
			report.Summary.EvidenceArtifacts++
		}
		report.Summary.EvidenceArtifacts += len(cache.Evidence)
	}

	partitionReports, partitionCounterexamples := buildPartitionReports(rootAbs, spec.Partitions, eventsByTask, acceptedByTask, criteria)
	report.Partitions = partitionReports
	counterexamples = append(counterexamples, partitionCounterexamples...)
	report.Summary.Partitions = len(partitionReports)
	for _, partition := range partitionReports {
		if partition.Recovered {
			report.Summary.RecoveredPartitions++
		}
		report.Summary.EvidenceArtifacts += len(partition.Evidence)
	}

	if len(spec.Tasks) < criteria.MinTasks {
		counterexamples = append(counterexamples, Counterexample{
			ID:      "criteria.tasks.insufficient",
			Kind:    "insufficient_tasks",
			Message: fmt.Sprintf("tasks %d below required %d", len(spec.Tasks), criteria.MinTasks),
		})
	}
	if len(spec.Workers) < criteria.MinWorkers {
		counterexamples = append(counterexamples, Counterexample{
			ID:      "criteria.workers.insufficient",
			Kind:    "insufficient_workers",
			Message: fmt.Sprintf("workers %d below required %d", len(spec.Workers), criteria.MinWorkers),
		})
	}
	if criteria.RequireWorkerLossRecovery && len(lostEvents) == 0 {
		counterexamples = append(counterexamples, Counterexample{
			ID:      "criteria.worker-loss.missing",
			Kind:    "missing_worker_loss_scenario",
			Message: "resilient analysis spec does not include a worker-loss scenario",
		})
	}
	if criteria.RequireCacheQuarantine && cacheSummary.corrupt == 0 {
		counterexamples = append(counterexamples, Counterexample{
			ID:      "criteria.cache-corruption.missing",
			Kind:    "missing_cache_corruption_scenario",
			Message: "resilient analysis spec does not include a hash-detected corrupt cache artifact",
		})
	}
	if criteria.RequirePartitionRecovery && len(spec.Partitions) == 0 {
		counterexamples = append(counterexamples, Counterexample{
			ID:      "criteria.partition.missing",
			Kind:    "missing_partition_scenario",
			Message: "resilient analysis spec does not include a partial network partition",
		})
	}

	sortCounterexamples(counterexamples)
	report.Counterexamples = counterexamples
	report.Summary.Counterexamples = len(counterexamples)
	report.OK = len(counterexamples) == 0
	report.Hash = reportHash(report)
	return report, nil
}

func WriteArtifacts(outDir string, report Report) error {
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}
	file, err := os.Create(filepath.Join(outDir, "resilient-analysis.json"))
	if err != nil {
		return err
	}
	if err := canonical.WriteJSON(file, report); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(outDir, "resilient-analysis.md"), []byte(RenderMarkdown(report)), 0o644)
}

func RenderMarkdown(report Report) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Resilient distributed analysis\n\n")
	fmt.Fprintf(&b, "Patchline replays a distributed analysis run and verifies that worker loss, hash-detected cache corruption, and partial network partitions are tolerated without losing tasks, accepting duplicate results, or trusting corrupted cache entries.\n\n")
	fmt.Fprintf(&b, "## Summary\n\n")
	fmt.Fprintf(&b, "| Metric | Value |\n| --- | ---: |\n")
	fmt.Fprintf(&b, "| OK | `%t` |\n", report.OK)
	fmt.Fprintf(&b, "| Workers | %d |\n", report.Summary.Workers)
	fmt.Fprintf(&b, "| Tasks | %d |\n", report.Summary.Tasks)
	fmt.Fprintf(&b, "| Completed tasks | %d |\n", report.Summary.CompletedTasks)
	fmt.Fprintf(&b, "| Worker-loss events | %d |\n", report.Summary.WorkerLossEvents)
	fmt.Fprintf(&b, "| Tasks recovered after worker loss | %d |\n", report.Summary.RecoveredWorkerLossTasks)
	fmt.Fprintf(&b, "| Corrupt caches | %d |\n", report.Summary.CorruptCaches)
	fmt.Fprintf(&b, "| Quarantined caches | %d |\n", report.Summary.QuarantinedCaches)
	fmt.Fprintf(&b, "| Rebuilt caches | %d |\n", report.Summary.RebuiltCaches)
	fmt.Fprintf(&b, "| Recovered partitions | %d |\n", report.Summary.RecoveredPartitions)
	fmt.Fprintf(&b, "| Partition-affected tasks completed | %d |\n", report.Summary.PartitionRecoveredTasks)
	fmt.Fprintf(&b, "| Duplicate accepted completions | %d |\n", report.Summary.DuplicateAcceptedCompletions)
	fmt.Fprintf(&b, "| Evidence artifacts | %d |\n", report.Summary.EvidenceArtifacts)
	fmt.Fprintf(&b, "| Counterexamples | %d |\n\n", report.Summary.Counterexamples)
	fmt.Fprintf(&b, "## Tasks\n\n")
	fmt.Fprintf(&b, "| Task | Attempts | Completed | Worker | Worker-loss recovery | Partition progress |\n| --- | ---: | ---: | --- | ---: | ---: |\n")
	for _, task := range report.Tasks {
		fmt.Fprintf(&b, "| `%s` | %d | `%t` | `%s` | `%t` | `%t` |\n",
			escapeTable(task.ID),
			task.Attempts,
			task.Completed,
			escapeTable(firstNonEmpty(task.CompletedWorker, "-")),
			task.ReassignedAfterWorkerLoss,
			task.ProgressedThroughPartition,
		)
	}
	fmt.Fprintf(&b, "\n## Cache artifacts\n\n")
	fmt.Fprintf(&b, "| Artifact | Task | Corrupt | Quarantined | Rebuilt |\n| --- | --- | ---: | ---: | ---: |\n")
	for _, cache := range report.CacheArtifacts {
		fmt.Fprintf(&b, "| `%s` | `%s` | `%t` | `%t` | `%t` |\n",
			escapeTable(cache.ID),
			escapeTable(cache.TaskID),
			cache.Corrupt,
			cache.Quarantined,
			cache.Rebuilt,
		)
	}
	if len(report.Counterexamples) > 0 {
		fmt.Fprintf(&b, "\n## Counterexamples\n\n")
		fmt.Fprintf(&b, "| ID | Kind | Subject | Message |\n| --- | --- | --- | --- |\n")
		for _, counterexample := range report.Counterexamples {
			fmt.Fprintf(&b, "| `%s` | `%s` | `%s` | %s |\n",
				escapeTable(counterexample.ID),
				escapeTable(counterexample.Kind),
				escapeTable(firstNonEmpty(counterexample.Subject, "-")),
				escapeTable(counterexample.Message),
			)
		}
	}
	return b.String()
}

type taskSummary struct {
	completedTasks               int
	recoveredWorkerLossTasks     int
	partitionRecoveredTasks      int
	deterministicLeases          int
	duplicateAcceptedCompletions int
}

func buildTaskReports(root string, tasks []Task, eventsByTask map[string][]Event, acceptedByTask map[string][]Event, lostEvents []Event, partitions []Partition, criteria Criteria) ([]TaskReport, []Counterexample, taskSummary) {
	var reports []TaskReport
	var counterexamples []Counterexample
	var summary taskSummary
	for _, task := range sortedTasks(tasks) {
		expected := normalizeHash(task.ExpectedResultSHA256)
		taskEvents := sortedEvents(eventsByTask[task.ID])
		accepted := sortedEvents(acceptedByTask[task.ID])
		evidence, evidenceCounterexamples := collectArtifacts(root, task.EvidencePaths, task.ID, "task_evidence", "missing_task_evidence", "empty_task_evidence", "invalid_task_evidence_path", "task evidence could not be read")
		counterexamples = append(counterexamples, evidenceCounterexamples...)
		if criteria.RequireEvidenceHashes && len(evidence) == 0 {
			counterexamples = append(counterexamples, Counterexample{
				ID:      "task." + stableID(task.ID, "evidence") + ".missing",
				Kind:    "missing_task_evidence",
				Subject: task.ID,
				Message: "task does not cite readable analysis evidence",
			})
		}

		attempts := map[int]struct{}{}
		leaseIDs := make([]string, 0)
		for _, event := range taskEvents {
			if event.Attempt > 0 {
				attempts[event.Attempt] = struct{}{}
			}
			if event.Kind == "lease_acquired" {
				leaseIDs = append(leaseIDs, event.LeaseID)
				expectedLease := ExpectedLeaseID(event.TaskID, event.Attempt, event.WorkerID)
				if criteria.RequireDeterministicLeases && event.LeaseID != expectedLease {
					counterexamples = append(counterexamples, Counterexample{
						ID:      "lease." + stableID(task.ID, event.WorkerID, fmt.Sprint(event.Attempt)) + ".nondeterministic",
						Kind:    "nondeterministic_lease",
						Subject: task.ID,
						Message: "lease id does not match the deterministic task/attempt/worker formula",
						Witness: []string{event.LeaseID, expectedLease},
					})
				} else if event.LeaseID == expectedLease {
					summary.deterministicLeases++
				}
			}
		}

		report := TaskReport{
			ID:                   task.ID,
			Repo:                 task.Repo,
			Subpath:              task.Subpath,
			Shard:                task.Shard,
			ExpectedResultSHA256: expected,
			Attempts:             len(attempts),
			AcceptedCompletions:  len(accepted),
			LeaseIDs:             sortedStrings(leaseIDs),
			Evidence:             evidence,
		}
		if len(accepted) == 0 {
			counterexamples = append(counterexamples, Counterexample{
				ID:      "task." + stableID(task.ID, "completion") + ".missing",
				Kind:    "missing_accepted_completion",
				Subject: task.ID,
				Message: "task never reached an accepted completion",
			})
		}
		if len(accepted) > 1 {
			summary.duplicateAcceptedCompletions += len(accepted) - 1
			if criteria.RequireNoDuplicateAcceptedCompletion {
				counterexamples = append(counterexamples, Counterexample{
					ID:      "task." + stableID(task.ID, "completion") + ".duplicate",
					Kind:    "duplicate_accepted_completion",
					Subject: task.ID,
					Message: "task has more than one accepted completion",
					Witness: eventIDs(accepted),
				})
			}
		}
		if len(accepted) > 0 {
			completion := accepted[0]
			report.Completed = true
			report.CompletedWorker = completion.WorkerID
			report.CompletionTick = completion.Tick
			report.ResultSHA256 = normalizeHash(completion.ResultSHA256)
			if expected != "" && report.ResultSHA256 != expected {
				counterexamples = append(counterexamples, Counterexample{
					ID:      "task." + stableID(task.ID, "result") + ".mismatch",
					Kind:    "result_hash_mismatch",
					Subject: task.ID,
					Message: "accepted completion hash does not match the expected result hash",
					Witness: []string{expected, report.ResultSHA256},
				})
			} else if len(accepted) == 1 {
				summary.completedTasks++
			}
		}

		for _, lost := range lostEvents {
			if !taskImpactedByWorkerLoss(taskEvents, lost) {
				continue
			}
			if taskRecoveredAfterWorkerLoss(taskEvents, accepted, lost) {
				report.ReassignedAfterWorkerLoss = true
				summary.recoveredWorkerLossTasks++
			} else if criteria.RequireWorkerLossRecovery {
				counterexamples = append(counterexamples, Counterexample{
					ID:      "task." + stableID(task.ID, lost.WorkerID, "worker-loss") + ".unrecovered",
					Kind:    "worker_lost_without_reassignment",
					Subject: task.ID,
					Message: "task was assigned to a lost worker and did not reach an accepted reassignment",
					Witness: []string{lost.WorkerID, fmt.Sprint(lost.Tick)},
				})
			}
		}

		for _, partition := range partitions {
			if !taskImpactedByPartition(taskEvents, partition) {
				continue
			}
			if taskProgressedThroughPartition(accepted, partition) {
				report.ProgressedThroughPartition = true
				summary.partitionRecoveredTasks++
			} else if criteria.RequirePartitionRecovery {
				counterexamples = append(counterexamples, Counterexample{
					ID:      "task." + stableID(task.ID, partition.ID, "partition") + ".stalled",
					Kind:    "partition_task_not_completed",
					Subject: task.ID,
					Message: "task was affected by a partial partition and did not complete after recovery",
					Witness: []string{partition.ID},
				})
			}
		}

		reports = append(reports, report)
	}
	sortCounterexamples(counterexamples)
	return reports, counterexamples, summary
}

type cacheSummary struct {
	corrupt     int
	quarantined int
	rebuilt     int
}

func buildCacheReports(root string, artifacts []CacheArtifact, quarantinedArtifacts, rebuiltArtifacts map[string]bool, criteria Criteria) ([]CacheReport, []Counterexample, cacheSummary) {
	var reports []CacheReport
	var counterexamples []Counterexample
	var summary cacheSummary
	for _, artifact := range sortedCacheArtifacts(artifacts) {
		expected := normalizeHash(artifact.ExpectedSHA256)
		evidence, evidenceCounterexamples := collectArtifacts(root, artifact.EvidencePaths, artifact.ID, "cache_evidence", "missing_cache_evidence", "empty_cache_evidence", "invalid_cache_evidence_path", "cache evidence could not be read")
		counterexamples = append(counterexamples, evidenceCounterexamples...)
		cacheBytes, artifactCounterexamples := collectArtifact(root, artifact.Path, artifact.ID, "cache", "missing_cache_artifact", "empty_cache_artifact", "invalid_cache_path", "cache artifact could not be read")
		counterexamples = append(counterexamples, artifactCounterexamples...)
		rebuiltBytes, rebuiltCounterexamples := collectOptionalArtifact(root, artifact.RebuiltPath, artifact.ID, "rebuilt_cache", false, "missing_rebuilt_cache")
		counterexamples = append(counterexamples, rebuiltCounterexamples...)
		corrupt := expected != "" && cacheBytes.Path != "" && cacheBytes.SHA256 != expected
		quarantined := quarantinedArtifacts[artifact.ID]
		rebuiltEvent := rebuiltArtifacts[artifact.ID]
		rebuilt := corrupt && rebuiltEvent && rebuiltBytes.Path != "" && rebuiltBytes.SHA256 == expected
		report := CacheReport{
			ID:              artifact.ID,
			TaskID:          artifact.TaskID,
			ExpectedSHA256:  expected,
			Artifact:        cacheBytes,
			Corrupt:         corrupt,
			Quarantined:     quarantined,
			Rebuilt:         rebuilt,
			RebuiltArtifact: rebuiltBytes,
			Evidence:        evidence,
		}
		if corrupt {
			summary.corrupt++
			if quarantined {
				summary.quarantined++
			} else if criteria.RequireCacheQuarantine {
				counterexamples = append(counterexamples, Counterexample{
					ID:      "cache." + stableID(artifact.ID, "quarantine") + ".missing",
					Kind:    "corrupt_cache_not_quarantined",
					Subject: artifact.ID,
					Message: "hash-detected corrupt cache artifact was not quarantined before reuse",
					Witness: []string{artifact.Path, expected, cacheBytes.SHA256},
				})
			}
			if rebuilt {
				summary.rebuilt++
			} else if criteria.RequireCacheRebuild {
				kind := "corrupt_cache_not_rebuilt"
				message := "corrupt cache artifact does not have a successful rebuild event and matching rebuilt bytes"
				witness := []string{artifact.ID}
				if rebuiltEvent && rebuiltBytes.Path != "" {
					kind = "rebuilt_cache_hash_mismatch"
					message = "rebuilt cache bytes do not match the trusted expected hash"
					witness = []string{artifact.RebuiltPath, expected, rebuiltBytes.SHA256}
				}
				counterexamples = append(counterexamples, Counterexample{
					ID:      "cache." + stableID(artifact.ID, "rebuild") + ".missing",
					Kind:    kind,
					Subject: artifact.ID,
					Message: message,
					Witness: witness,
				})
			}
		}
		reports = append(reports, report)
	}
	sortCounterexamples(counterexamples)
	return reports, counterexamples, summary
}

func buildPartitionReports(root string, partitions []Partition, eventsByTask map[string][]Event, acceptedByTask map[string][]Event, criteria Criteria) ([]PartitionReport, []Counterexample) {
	var reports []PartitionReport
	var counterexamples []Counterexample
	for _, partition := range sortedPartitions(partitions) {
		evidence, evidenceCounterexamples := collectArtifacts(root, partition.EvidencePaths, partition.ID, "partition_evidence", "missing_partition_evidence", "empty_partition_evidence", "invalid_partition_evidence_path", "partition evidence could not be read")
		counterexamples = append(counterexamples, evidenceCounterexamples...)
		recovered := partition.RecoveredTick > partition.StartTick
		var completed []string
		for taskID, taskEvents := range eventsByTask {
			if !taskImpactedByPartition(taskEvents, partition) {
				continue
			}
			if taskProgressedThroughPartition(acceptedByTask[taskID], partition) {
				completed = append(completed, taskID)
			}
		}
		sort.Strings(completed)
		if criteria.RequirePartitionRecovery && !recovered {
			counterexamples = append(counterexamples, Counterexample{
				ID:      "partition." + stableID(partition.ID, "recovery") + ".missing",
				Kind:    "partition_not_recovered",
				Subject: partition.ID,
				Message: "partial network partition has no recovery tick after its start tick",
				Witness: []string{fmt.Sprint(partition.StartTick), fmt.Sprint(partition.RecoveredTick)},
			})
		}
		if criteria.RequirePartitionRecovery && recovered && len(completed) == 0 {
			counterexamples = append(counterexamples, Counterexample{
				ID:      "partition." + stableID(partition.ID, "progress") + ".missing",
				Kind:    "partition_no_progress",
				Subject: partition.ID,
				Message: "partial network partition recovered but no affected task made accepted progress",
			})
		}
		reports = append(reports, PartitionReport{
			ID:              partition.ID,
			AffectedWorkers: sortedStrings(normalizedStrings(partition.AffectedWorkers)),
			StartTick:       partition.StartTick,
			RecoveredTick:   partition.RecoveredTick,
			Recovered:       recovered,
			CompletedTasks:  completed,
			Evidence:        evidence,
		})
	}
	sortCounterexamples(counterexamples)
	return reports, counterexamples
}

func buildWorkerReports(workers []Worker, lostEvents []Event) ([]WorkerReport, map[string][]string) {
	recoveredByWorker := map[string][]string{}
	lostByWorker := map[string]int{}
	for _, lost := range lostEvents {
		if current, ok := lostByWorker[lost.WorkerID]; !ok || lost.Tick < current {
			lostByWorker[lost.WorkerID] = lost.Tick
		}
	}
	var reports []WorkerReport
	for _, worker := range sortedWorkers(workers) {
		report := WorkerReport{
			ID:               worker.ID,
			Zone:             worker.Zone,
			InitiallyHealthy: worker.InitiallyHealthy,
		}
		if tick, ok := lostByWorker[worker.ID]; ok {
			report.Lost = true
			report.LostTick = tick
		}
		reports = append(reports, report)
		recoveredByWorker[worker.ID] = nil
	}
	return reports, recoveredByWorker
}

func taskImpactedByWorkerLoss(events []Event, lost Event) bool {
	for _, event := range events {
		if event.WorkerID == lost.WorkerID && event.Tick <= lost.Tick && inSet(event.Kind, "lease_acquired", "attempt_started") {
			return true
		}
	}
	return false
}

func taskRecoveredAfterWorkerLoss(events, accepted []Event, lost Event) bool {
	for _, event := range events {
		if event.Kind != "task_reassigned" || event.WorkerID == lost.WorkerID || event.Tick <= lost.Tick {
			continue
		}
		for _, completion := range accepted {
			if completion.WorkerID == event.WorkerID && completion.Tick >= event.Tick {
				return true
			}
		}
	}
	return false
}

func taskImpactedByPartition(events []Event, partition Partition) bool {
	affected := normalizedSet(partition.AffectedWorkers)
	for _, event := range events {
		if _, ok := affected[normalizeToken(event.WorkerID)]; !ok {
			continue
		}
		if partition.RecoveredTick > partition.StartTick {
			if event.Tick >= partition.StartTick && event.Tick <= partition.RecoveredTick {
				return true
			}
		} else if event.Tick >= partition.StartTick {
			return true
		}
	}
	return false
}

func taskProgressedThroughPartition(accepted []Event, partition Partition) bool {
	if partition.RecoveredTick <= partition.StartTick {
		return false
	}
	for _, event := range accepted {
		if event.Tick >= partition.RecoveredTick {
			return true
		}
	}
	return false
}

func validateSpec(spec Spec) error {
	if strings.TrimSpace(spec.Name) == "" {
		return fmt.Errorf("resilient analysis spec name is required")
	}
	if err := uniqueIDs("worker", workerIDs(spec.Workers)); err != nil {
		return err
	}
	if err := uniqueIDs("task", taskIDs(spec.Tasks)); err != nil {
		return err
	}
	if err := uniqueIDs("cache artifact", cacheIDs(spec.CacheArtifacts)); err != nil {
		return err
	}
	if err := uniqueIDs("partition", partitionIDs(spec.Partitions)); err != nil {
		return err
	}
	if err := uniqueIDs("event", eventIDs(spec.Events)); err != nil {
		return err
	}
	allowedKinds := map[string]bool{
		"lease_acquired":    true,
		"attempt_started":   true,
		"worker_lost":       true,
		"task_reassigned":   true,
		"task_completed":    true,
		"cache_quarantined": true,
		"cache_rebuilt":     true,
	}
	for _, event := range spec.Events {
		if !allowedKinds[event.Kind] {
			return fmt.Errorf("event %q has unsupported kind %q", event.ID, event.Kind)
		}
	}
	return nil
}

func collectArtifacts(root string, paths []string, subject, artifactKind, missingKind, emptyKind, invalidKind, message string) ([]ArtifactEvidence, []Counterexample) {
	var artifacts []ArtifactEvidence
	var counterexamples []Counterexample
	for _, path := range sortedStrings(paths) {
		artifact, artifactCounterexamples := collectArtifact(root, path, subject, artifactKind, missingKind, emptyKind, invalidKind, message)
		if artifact.Path != "" {
			artifacts = append(artifacts, artifact)
		}
		counterexamples = append(counterexamples, artifactCounterexamples...)
	}
	return artifacts, counterexamples
}

func collectArtifact(root, relPath, subject, artifactKind, missingKind, emptyKind, invalidKind, message string) (ArtifactEvidence, []Counterexample) {
	fullPath, err := safeJoin(root, relPath)
	if err != nil {
		return ArtifactEvidence{}, []Counterexample{{
			ID:      artifactKind + "." + stableID(subject, relPath, "path") + ".invalid",
			Kind:    invalidKind,
			Subject: subject,
			Message: err.Error(),
			Witness: []string{relPath},
		}}
	}
	data, err := os.ReadFile(fullPath)
	if err != nil {
		return ArtifactEvidence{}, []Counterexample{{
			ID:      artifactKind + "." + stableID(subject, relPath, "missing") + ".missing",
			Kind:    missingKind,
			Subject: subject,
			Message: message,
			Witness: []string{relPath},
		}}
	}
	if len(data) == 0 {
		return ArtifactEvidence{}, []Counterexample{{
			ID:      artifactKind + "." + stableID(subject, relPath, "empty") + ".empty",
			Kind:    emptyKind,
			Subject: subject,
			Message: "artifact is empty",
			Witness: []string{relPath},
		}}
	}
	sum := sha256.Sum256(data)
	return ArtifactEvidence{
		Path:   filepath.ToSlash(filepath.Clean(relPath)),
		SHA256: "sha256:" + hex.EncodeToString(sum[:]),
		Bytes:  int64(len(data)),
	}, nil
}

func collectOptionalArtifact(root, relPath, subject, artifactKind string, required bool, missingKind string) (ArtifactEvidence, []Counterexample) {
	if strings.TrimSpace(relPath) == "" {
		if required {
			return ArtifactEvidence{}, []Counterexample{{
				ID:      artifactKind + "." + stableID(subject, "missing") + ".missing",
				Kind:    missingKind,
				Subject: subject,
				Message: artifactKind + " artifact path is required",
			}}
		}
		return ArtifactEvidence{}, nil
	}
	return collectArtifact(root, relPath, subject, artifactKind, missingKind, "empty_"+artifactKind, "invalid_"+artifactKind+"_path", artifactKind+" artifact could not be read")
}

func ExpectedLeaseID(taskID string, attempt int, workerID string) string {
	return fmt.Sprintf("lease:%s:%02d:%s", normalizeToken(taskID), attempt, normalizeToken(workerID))
}

func normalizeHash(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.TrimPrefix(value, "sha256:")
	if len(value) != 64 {
		return ""
	}
	for _, r := range value {
		if !((r >= 'a' && r <= 'f') || (r >= '0' && r <= '9')) {
			return ""
		}
	}
	return "sha256:" + value
}

func normalizeToken(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var b strings.Builder
	lastDash := false
	for _, r := range value {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash {
			b.WriteByte('-')
			lastDash = true
		}
	}
	return strings.Trim(b.String(), "-")
}

func normalizedSet(values []string) map[string]struct{} {
	out := map[string]struct{}{}
	for _, value := range values {
		if token := normalizeToken(value); token != "" {
			out[token] = struct{}{}
		}
	}
	return out
}

func normalizedStrings(values []string) []string {
	var out []string
	for _, value := range values {
		if normalized := normalizeToken(value); normalized != "" {
			out = append(out, normalized)
		}
	}
	return sortedStrings(uniqueStrings(out))
}

func sortedStrings(values []string) []string {
	out := append([]string(nil), values...)
	sort.Strings(out)
	return out
}

func uniqueStrings(values []string) []string {
	seen := map[string]struct{}{}
	var out []string
	for _, value := range values {
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func sortedWorkers(workers []Worker) []Worker {
	out := append([]Worker(nil), workers...)
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func sortedTasks(tasks []Task) []Task {
	out := append([]Task(nil), tasks...)
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func sortedCacheArtifacts(artifacts []CacheArtifact) []CacheArtifact {
	out := append([]CacheArtifact(nil), artifacts...)
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func sortedPartitions(partitions []Partition) []Partition {
	out := append([]Partition(nil), partitions...)
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func sortedEvents(events []Event) []Event {
	out := append([]Event(nil), events...)
	sort.Slice(out, func(i, j int) bool {
		if out[i].Tick != out[j].Tick {
			return out[i].Tick < out[j].Tick
		}
		return out[i].ID < out[j].ID
	})
	return out
}

func sortCounterexamples(counterexamples []Counterexample) {
	sort.Slice(counterexamples, func(i, j int) bool {
		if counterexamples[i].Kind != counterexamples[j].Kind {
			return counterexamples[i].Kind < counterexamples[j].Kind
		}
		return counterexamples[i].ID < counterexamples[j].ID
	})
}

func eventIDs(events []Event) []string {
	ids := make([]string, 0, len(events))
	for _, event := range events {
		ids = append(ids, event.ID)
	}
	return sortedStrings(ids)
}

func workerIDs(workers []Worker) []string {
	ids := make([]string, 0, len(workers))
	for _, worker := range workers {
		ids = append(ids, worker.ID)
	}
	return ids
}

func taskIDs(tasks []Task) []string {
	ids := make([]string, 0, len(tasks))
	for _, task := range tasks {
		ids = append(ids, task.ID)
	}
	return ids
}

func cacheIDs(artifacts []CacheArtifact) []string {
	ids := make([]string, 0, len(artifacts))
	for _, artifact := range artifacts {
		ids = append(ids, artifact.ID)
	}
	return ids
}

func partitionIDs(partitions []Partition) []string {
	ids := make([]string, 0, len(partitions))
	for _, partition := range partitions {
		ids = append(ids, partition.ID)
	}
	return ids
}

func uniqueIDs(kind string, ids []string) error {
	seen := map[string]struct{}{}
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" {
			return fmt.Errorf("%s id is required", kind)
		}
		if _, ok := seen[id]; ok {
			return fmt.Errorf("duplicate %s id %q", kind, id)
		}
		seen[id] = struct{}{}
	}
	return nil
}

func safeJoin(root, relPath string) (string, error) {
	if err := validateRelativePath(relPath); err != nil {
		return "", err
	}
	clean := filepath.Clean(filepath.FromSlash(relPath))
	fullPath := filepath.Join(root, clean)
	rel, err := filepath.Rel(root, fullPath)
	if err != nil {
		return "", err
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path %q escapes root", relPath)
	}
	return fullPath, nil
}

func validateRelativePath(path string) error {
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("path is required")
	}
	if filepath.IsAbs(path) {
		return fmt.Errorf("path %q must be relative", path)
	}
	clean := filepath.Clean(filepath.FromSlash(path))
	if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return fmt.Errorf("path %q escapes root", path)
	}
	return nil
}

func inSet(value string, allowed ...string) bool {
	for _, candidate := range allowed {
		if value == candidate {
			return true
		}
	}
	return false
}

func stableID(parts ...string) string {
	joined := strings.Join(parts, "\x00")
	sum := sha256.Sum256([]byte(joined))
	return hex.EncodeToString(sum[:])[:16]
}

func reportHash(report Report) string {
	report.Hash = ""
	return canonical.Hash(report)
}

func escapeTable(value string) string {
	return strings.ReplaceAll(value, "|", "\\|")
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
