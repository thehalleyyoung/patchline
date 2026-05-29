package semantics

import "github.com/thehalleyyoung/patchline/internal/canonical"

type Claim struct {
	Ref      string      `json:"ref"`
	Status   ClaimStatus `json:"status"`
	Reason   string      `json:"reason"`
	Evidence string      `json:"evidence,omitempty"`
}

type Obligation struct {
	Ref         string      `json:"ref"`
	Description string      `json:"description"`
	Status      ClaimStatus `json:"status"`
}

type Counterexample struct {
	Ref     string `json:"ref"`
	Message string `json:"message"`
	Witness string `json:"witness,omitempty"`
}

type ArtifactEvidence struct {
	Path            string                 `json:"path"`
	Kind            string                 `json:"kind"`
	Facts           []string               `json:"facts,omitempty"`
	Obligations     []Obligation           `json:"obligations,omitempty"`
	Counterexamples []Counterexample       `json:"counterexamples,omitempty"`
	Hashes          map[string]string      `json:"hashes,omitempty"`
	Claims          []Claim                `json:"claims,omitempty"`
	Metadata        map[string]interface{} `json:"metadata,omitempty"`
}

type ConformanceReport struct {
	Version      string             `json:"version"`
	ContractHash string             `json:"contract_hash"`
	OK           bool               `json:"ok"`
	Totals       ConformanceTotals  `json:"totals"`
	Artifacts    []ArtifactEvidence `json:"artifacts"`
	Hash         string             `json:"hash"`
}

type ConformanceTotals struct {
	Artifacts       int `json:"artifacts"`
	Conforming      int `json:"conforming"`
	ProofHoles      int `json:"proof_holes"`
	Counterexamples int `json:"counterexamples"`
	Facts           int `json:"facts"`
	Obligations     int `json:"obligations"`
	Hashes          int `json:"hashes"`
}

func Audit(contract Contract, artifacts []ArtifactEvidence) ConformanceReport {
	report := ConformanceReport{
		Version:      Version,
		ContractHash: contract.Hash,
		OK:           true,
		Artifacts:    append([]ArtifactEvidence(nil), artifacts...),
	}
	report.Totals.Artifacts = len(report.Artifacts)
	for i := range report.Artifacts {
		artifact := &report.Artifacts[i]
		if len(artifact.Facts)+len(artifact.Obligations)+len(artifact.Counterexamples)+len(artifact.Hashes) > 0 {
			report.Totals.Conforming++
		} else {
			report.OK = false
			artifact.Counterexamples = append(artifact.Counterexamples, Counterexample{
				Ref:     "semantic_contract.empty_artifact",
				Message: "artifact emits no facts, obligations, counterexamples, or hashes",
			})
		}
		report.Totals.Facts += len(artifact.Facts)
		report.Totals.Obligations += len(artifact.Obligations)
		report.Totals.Hashes += len(artifact.Hashes)
		for _, obligation := range artifact.Obligations {
			if obligation.Status == ClaimUnsupported || obligation.Status == ClaimAssumed {
				report.Totals.ProofHoles++
			}
		}
		for _, claim := range artifact.Claims {
			if claim.Status == ClaimUnsupported || claim.Status == ClaimAssumed {
				report.Totals.ProofHoles++
			}
			if claim.Status == ClaimRefuted {
				report.OK = false
			}
		}
		report.Totals.Counterexamples += len(artifact.Counterexamples)
		if len(artifact.Counterexamples) > 0 {
			report.OK = false
		}
	}
	report.Hash = reportHash(report)
	return report
}

func reportHash(report ConformanceReport) string {
	report.Hash = ""
	return canonical.Hash(report)
}
