package semantics

import "testing"

func TestDefaultContractIsCanonicalAndGrounded(t *testing.T) {
	contract := DefaultContract()
	if contract.Version != Version {
		t.Fatalf("version = %q, want %q", contract.Version, Version)
	}
	if len(contract.Hash) != 64 {
		t.Fatalf("contract hash = %q", contract.Hash)
	}
	if got := contractHash(contract); got != contract.Hash {
		t.Fatalf("contract hash is not canonical: got %s want %s", got, contract.Hash)
	}
	if len(contract.StateModel) == 0 || len(contract.ObservationModel) == 0 || len(contract.RepairTransformer.Effects) == 0 {
		t.Fatalf("contract is missing semantic registries")
	}
}

func TestAuditFindsEmptyArtifacts(t *testing.T) {
	report := Audit(DefaultContract(), []ArtifactEvidence{{Path: "empty.json", Kind: "empty"}})
	if report.OK {
		t.Fatalf("empty artifact unexpectedly conformed")
	}
	if report.Totals.Counterexamples != 1 {
		t.Fatalf("counterexamples = %d, want 1", report.Totals.Counterexamples)
	}
}
