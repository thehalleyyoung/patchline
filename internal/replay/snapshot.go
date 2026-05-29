package replay

import (
	"github.com/patchline/patchline/internal/canonical"
	"github.com/patchline/patchline/internal/provenance"
	"github.com/patchline/patchline/internal/repair"
)

const SnapshotComparisonVersion = "patchline.snapshot-comparison/v1"

type SnapshotComparison struct {
	Version            string           `json:"version"`
	Manifest           string           `json:"manifest"`
	Incident           string           `json:"incident"`
	BeforeSnapshotHash string           `json:"before_snapshot_hash"`
	AfterSnapshotHash  string           `json:"after_snapshot_hash"`
	BeforeReplayHash   string           `json:"before_replay_hash"`
	AfterReplayHash    string           `json:"after_replay_hash"`
	BeforeFinalHash    string           `json:"before_final_hash"`
	AfterFinalHash     string           `json:"after_final_hash"`
	Stable             bool             `json:"stable"`
	OperationDrift     []OperationDrift `json:"operation_drift,omitempty"`
	Hash               string           `json:"hash"`
}

type OperationDrift struct {
	OperationID       string `json:"operation_id"`
	BeforeMatchedRows int    `json:"before_matched_rows"`
	AfterMatchedRows  int    `json:"after_matched_rows"`
	BeforeDiffHash    string `json:"before_diff_hash"`
	AfterDiffHash     string `json:"after_diff_hash"`
	Reason            string `json:"reason"`
}

func CompareSnapshots(manifest repair.Manifest, graph *provenance.Graph, before, after Store) (SnapshotComparison, error) {
	beforeReport, beforeFinal, err := Apply(manifest, graph, before)
	if err != nil {
		return SnapshotComparison{}, err
	}
	afterReport, afterFinal, err := Apply(manifest, graph, after)
	if err != nil {
		return SnapshotComparison{}, err
	}
	comparison := SnapshotComparison{
		Version:            SnapshotComparisonVersion,
		Manifest:           manifest.Name,
		Incident:           manifest.Incident,
		BeforeSnapshotHash: before.Hash(),
		AfterSnapshotHash:  after.Hash(),
		BeforeReplayHash:   beforeReport.Hash(),
		AfterReplayHash:    afterReport.Hash(),
		BeforeFinalHash:    beforeFinal.Hash(),
		AfterFinalHash:     afterFinal.Hash(),
	}
	comparison.OperationDrift = operationDrift(beforeReport, afterReport)
	comparison.Stable = len(comparison.OperationDrift) == 0
	comparison.Hash = snapshotComparisonHash(comparison)
	return comparison, nil
}

func operationDrift(before, after Report) []OperationDrift {
	beforeByID := map[string]OperationReport{}
	for _, operation := range before.Operations {
		beforeByID[operation.OperationID] = operation
	}
	var drift []OperationDrift
	for _, afterOperation := range after.Operations {
		beforeOperation := beforeByID[afterOperation.OperationID]
		beforeHash := canonical.Hash(beforeOperation.Diffs)
		afterHash := canonical.Hash(afterOperation.Diffs)
		if beforeOperation.MatchedRows == afterOperation.MatchedRows && beforeHash == afterHash {
			continue
		}
		reason := "row diffs changed across historical snapshots"
		if beforeOperation.MatchedRows != afterOperation.MatchedRows {
			reason = "matched row count changed across historical snapshots"
		}
		drift = append(drift, OperationDrift{
			OperationID:       afterOperation.OperationID,
			BeforeMatchedRows: beforeOperation.MatchedRows,
			AfterMatchedRows:  afterOperation.MatchedRows,
			BeforeDiffHash:    beforeHash,
			AfterDiffHash:     afterHash,
			Reason:            reason,
		})
	}
	return drift
}

func snapshotComparisonHash(comparison SnapshotComparison) string {
	comparison.Hash = ""
	return canonical.Hash(comparison)
}
