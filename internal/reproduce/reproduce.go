package reproduce

import (
	"fmt"

	"github.com/thehalleyyoung/patchline/internal/attest"
	"github.com/thehalleyyoung/patchline/internal/ledger"
	"github.com/thehalleyyoung/patchline/internal/provenance"
	"github.com/thehalleyyoung/patchline/internal/repair"
	"github.com/thehalleyyoung/patchline/internal/replay"
)

const Version = "patchline.reproduce/v1"

type Spec struct {
	Version                  string            `json:"version"`
	Name                     string            `json:"name"`
	RepairManifest           string            `json:"repair_manifest"`
	ExpectedReportHash       string            `json:"expected_report_hash,omitempty"`
	ExpectedLedgerCheckpoint ledger.Checkpoint `json:"expected_ledger_checkpoint,omitempty"`
	Checks                   []attest.Check    `json:"checks"`
}

type Result struct {
	Name                     string              `json:"name"`
	OK                       bool                `json:"ok"`
	ReportHash               string              `json:"report_hash"`
	ExpectedReportHash       string              `json:"expected_report_hash,omitempty"`
	LedgerOK                 bool                `json:"ledger_ok"`
	LedgerError              string              `json:"ledger_error,omitempty"`
	LedgerCheckpoint         ledger.Checkpoint   `json:"ledger_checkpoint"`
	ExpectedLedgerCheckpoint ledger.Checkpoint   `json:"expected_ledger_checkpoint,omitempty"`
	RepairDiagnostics        []repair.Diagnostic `json:"repair_diagnostics,omitempty"`
	Attestations             []attest.Result     `json:"attestations"`
}

func Run(spec Spec, manifest repair.Manifest, graph *provenance.Graph, store replay.Store, entries []ledger.Entry, checkpoint ledger.Checkpoint) (Result, error) {
	if spec.Version != Version {
		return Result{}, fmt.Errorf("reproducibility spec version must be %s", Version)
	}
	diagnostics := repair.Validate(manifest, graph)
	report, err := replay.DryRun(manifest, graph, store)
	if err != nil {
		return Result{}, err
	}

	checks := append([]attest.Check(nil), spec.Checks...)
	if spec.ExpectedReportHash != "" {
		checks = append(checks, attest.Check{Kind: "report_hash_equals", Expect: spec.ExpectedReportHash})
	}
	attestations := attest.Verify(report, manifest, checks)
	ledgerErr := ledger.VerifyCheckpoint(entries, checkpoint)
	actualCheckpoint, checkpointErr := ledger.CheckpointFor(entries)
	if checkpointErr != nil {
		return Result{}, checkpointErr
	}
	ledgerOK := ledgerErr == nil
	if spec.ExpectedLedgerCheckpoint.Count != 0 || spec.ExpectedLedgerCheckpoint.TipHash != "" {
		ledgerOK = ledgerOK && actualCheckpoint == spec.ExpectedLedgerCheckpoint
	}

	result := Result{
		Name:                     spec.Name,
		ReportHash:               report.Hash(),
		ExpectedReportHash:       spec.ExpectedReportHash,
		LedgerOK:                 ledgerOK,
		LedgerCheckpoint:         actualCheckpoint,
		ExpectedLedgerCheckpoint: spec.ExpectedLedgerCheckpoint,
		RepairDiagnostics:        diagnostics,
		Attestations:             attestations,
	}
	if ledgerErr != nil {
		result.LedgerError = ledgerErr.Error()
	}
	result.OK = !repair.HasErrors(diagnostics) && attest.OK(attestations) && ledgerOK
	return result, nil
}

func UpdateExpected(spec Spec, result Result) Spec {
	spec.ExpectedReportHash = result.ReportHash
	spec.ExpectedLedgerCheckpoint = result.LedgerCheckpoint
	return spec
}
