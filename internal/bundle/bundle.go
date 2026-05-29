package bundle

import (
	"sort"

	"github.com/patchline/patchline/internal/canonical"
	"github.com/patchline/patchline/internal/ledger"
	"github.com/patchline/patchline/internal/migration"
	"github.com/patchline/patchline/internal/policy"
	"github.com/patchline/patchline/internal/provenance"
	"github.com/patchline/patchline/internal/repair"
	"github.com/patchline/patchline/internal/replay"
)

const Version = "patchline.bundle/v2"

type Entry struct {
	Path   string `json:"path"`
	Hash   string `json:"hash"`
	Bytes  int    `json:"bytes"`
	Kind   string `json:"kind"`
	Inline any    `json:"inline,omitempty"`
}

type Bundle struct {
	Version    string            `json:"version"`
	Name       string            `json:"name"`
	Entries    []Entry           `json:"entries"`
	Checkpoint ledger.Checkpoint `json:"ledger_checkpoint"`
	BundleHash string            `json:"bundle_hash"`
}

type Inputs struct {
	Name              string
	Manifest          repair.Manifest
	Report            replay.Report
	Slice             provenance.Slice
	Migration         migration.Report
	PolicyEvaluation  policy.Evaluation
	LedgerCheckpoint  ledger.Checkpoint
	ReproductionNotes string
	ProofArtifacts    []ProofArtifact
}

type ProofArtifact struct {
	Path   string
	Kind   string
	Inline any
}

func Build(inputs Inputs) Bundle {
	entries := []Entry{
		entry("repair.json", "repair-manifest", inputs.Manifest),
		entry("dry-run.json", "dry-run-report", inputs.Report),
		entry("graph-slice.json", "provenance-slice", inputs.Slice),
		entry("migration-analysis.json", "migration-analysis", inputs.Migration),
		entry("policy-results.json", "policy-evaluation", inputs.PolicyEvaluation),
		entry("ledger-checkpoint.json", "ledger-checkpoint", inputs.LedgerCheckpoint),
		entry("README.txt", "human-readme", readme(inputs)),
	}
	for _, artifact := range inputs.ProofArtifacts {
		if artifact.Path == "" || artifact.Kind == "" || artifact.Inline == nil {
			continue
		}
		entries = append(entries, entry(artifact.Path, artifact.Kind, artifact.Inline))
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Path < entries[j].Path
	})
	b := Bundle{
		Version:    Version,
		Name:       inputs.Name,
		Entries:    entries,
		Checkpoint: inputs.LedgerCheckpoint,
	}
	b.BundleHash = canonical.Hash(struct {
		Version    string            `json:"version"`
		Name       string            `json:"name"`
		Entries    []Entry           `json:"entries"`
		Checkpoint ledger.Checkpoint `json:"ledger_checkpoint"`
	}{
		Version:    b.Version,
		Name:       b.Name,
		Entries:    b.Entries,
		Checkpoint: b.Checkpoint,
	})
	return b
}

func Verify(b Bundle) bool {
	expected := canonical.Hash(struct {
		Version    string            `json:"version"`
		Name       string            `json:"name"`
		Entries    []Entry           `json:"entries"`
		Checkpoint ledger.Checkpoint `json:"ledger_checkpoint"`
	}{
		Version:    b.Version,
		Name:       b.Name,
		Entries:    b.Entries,
		Checkpoint: b.Checkpoint,
	})
	return b.Version == Version && b.BundleHash == expected
}

func entry(path, kind string, value any) Entry {
	bytes := canonical.MustBytes(value)
	return Entry{Path: path, Kind: kind, Hash: canonical.HashBytes(bytes), Bytes: len(bytes), Inline: value}
}

func readme(inputs Inputs) string {
	return "Patchline incident bundle: " + inputs.Name + "\n" +
		"Dry-run hash: " + inputs.Report.Hash() + "\n" +
		"Graph slice hash: " + inputs.Slice.SliceHash + "\n" +
		"Migration analysis hash: " + inputs.Migration.Summary.ReportHash + "\n" +
		"Policy hash: " + inputs.PolicyEvaluation.PolicyHash + "\n" +
		"Ledger tip: " + inputs.LedgerCheckpoint.TipHash + "\n" +
		inputs.ReproductionNotes
}
