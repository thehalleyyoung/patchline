package certdiff

import (
	"fmt"
	"sort"
	"strings"

	"github.com/thehalleyyoung/patchline/internal/certlang"
)

const ReportVersion = "patchline.certificate-semantic-diff/v1"

type Report struct {
	Version                  string             `json:"version"`
	ObligationLattice        string             `json:"obligation_lattice"`
	OldVersion               string             `json:"old_version"`
	NewVersion               string             `json:"new_version"`
	OldCertificateID         string             `json:"old_certificate_id"`
	NewCertificateID         string             `json:"new_certificate_id"`
	OldCanonicalSHA256       string             `json:"old_canonical_sha256"`
	NewCanonicalSHA256       string             `json:"new_canonical_sha256"`
	OldNormalizedSHA256      string             `json:"old_normalized_sha256"`
	NewNormalizedSHA256      string             `json:"new_normalized_sha256"`
	OldVerdict               string             `json:"old_verdict"`
	NewVerdict               string             `json:"new_verdict"`
	OldRiskBPS               int                `json:"old_risk_bps"`
	NewRiskBPS               int                `json:"new_risk_bps"`
	Summary                  map[string]int     `json:"summary"`
	Obligations              []ObligationChange `json:"obligations"`
	AllChangesClassified     bool               `json:"all_changes_classified"`
	NormalizedIdentityStable bool               `json:"normalized_identity_stable"`
}

type ObligationChange struct {
	ID          string   `json:"id"`
	Kind        string   `json:"kind,omitempty"`
	OldKind     string   `json:"old_kind,omitempty"`
	NewKind     string   `json:"new_kind,omitempty"`
	OldStatus   string   `json:"old_status,omitempty"`
	NewStatus   string   `json:"new_status,omitempty"`
	OldFormula  string   `json:"old_formula,omitempty"`
	NewFormula  string   `json:"new_formula,omitempty"`
	OldEvidence []string `json:"old_evidence,omitempty"`
	NewEvidence []string `json:"new_evidence,omitempty"`
	Change      string   `json:"change"`
	Reason      string   `json:"reason"`
}

type normalizedInput struct {
	sourceVersion string
	normalized    certlang.NormalizationResult
	certificate   certlang.Certificate
}

func CompareBytes(oldData, newData []byte, opts certlang.Options) (Report, error) {
	oldInput, err := normalizeInput(oldData, opts)
	if err != nil {
		return Report{}, fmt.Errorf("old certificate: %w", err)
	}
	newInput, err := normalizeInput(newData, opts)
	if err != nil {
		return Report{}, fmt.Errorf("new certificate: %w", err)
	}
	return CompareCertificates(oldInput, newInput), nil
}

func CompareCertificates(oldInput, newInput normalizedInput) Report {
	oldCert := certlang.NormalizeCertificate(oldInput.certificate)
	newCert := certlang.NormalizeCertificate(newInput.certificate)
	report := Report{
		Version:                  ReportVersion,
		ObligationLattice:        "checked > assumed > unsupported; refuted is a counterexample state, not a confidence point",
		OldVersion:               oldInput.sourceVersion,
		NewVersion:               newInput.sourceVersion,
		OldCertificateID:         oldCert.CertificateID,
		NewCertificateID:         newCert.CertificateID,
		OldCanonicalSHA256:       oldInput.normalized.NormalizedCanonicalSHA256,
		NewCanonicalSHA256:       newInput.normalized.NormalizedCanonicalSHA256,
		OldNormalizedSHA256:      oldInput.normalized.NormalizedSHA256,
		NewNormalizedSHA256:      newInput.normalized.NormalizedSHA256,
		OldVerdict:               oldCert.Verdict,
		NewVerdict:               newCert.Verdict,
		OldRiskBPS:               oldCert.RiskBPS,
		NewRiskBPS:               newCert.RiskBPS,
		Summary:                  map[string]int{},
		AllChangesClassified:     true,
		NormalizedIdentityStable: oldInput.normalized.NormalizedCanonicalSHA256 == newInput.normalized.NormalizedCanonicalSHA256,
	}
	oldObligations := obligationMap(oldCert.Obligations)
	newObligations := obligationMap(newCert.Obligations)
	ids := make([]string, 0, len(oldObligations)+len(newObligations))
	seen := map[string]bool{}
	for id := range oldObligations {
		ids = append(ids, id)
		seen[id] = true
	}
	for id := range newObligations {
		if !seen[id] {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	for _, id := range ids {
		change := classify(id, oldObligations[id], newObligations[id])
		report.Summary[change.Change]++
		report.Obligations = append(report.Obligations, change)
	}
	return report
}

func normalizeInput(data []byte, opts certlang.Options) (normalizedInput, error) {
	migrated, err := certlang.MigrateToCurrent(data, opts)
	if err != nil {
		return normalizedInput{}, err
	}
	normalized, err := certlang.Normalize(migrated.Migrated, opts)
	if err != nil {
		return normalizedInput{}, err
	}
	return normalizedInput{
		sourceVersion: migrated.SourceVersion,
		normalized:    normalized,
		certificate:   *normalized.Certificate,
	}, nil
}

func obligationMap(obligations []certlang.Obligation) map[string]certlang.Obligation {
	out := map[string]certlang.Obligation{}
	for _, obligation := range obligations {
		obligation.Evidence = append([]string(nil), obligation.Evidence...)
		sort.Strings(obligation.Evidence)
		out[obligation.ID] = obligation
	}
	return out
}

func classify(id string, oldObligation, newObligation certlang.Obligation) ObligationChange {
	_, oldOK := nonEmptyObligation(oldObligation)
	_, newOK := nonEmptyObligation(newObligation)
	change := ObligationChange{ID: id}
	if oldOK {
		change.OldKind = oldObligation.Kind
		change.OldStatus = oldObligation.Status
		change.OldFormula = oldObligation.Formula
		change.OldEvidence = append([]string(nil), oldObligation.Evidence...)
	}
	if newOK {
		change.NewKind = newObligation.Kind
		change.NewStatus = newObligation.Status
		change.NewFormula = newObligation.Formula
		change.NewEvidence = append([]string(nil), newObligation.Evidence...)
	}
	if oldOK && newOK && oldObligation.Kind == newObligation.Kind {
		change.Kind = newObligation.Kind
	}
	switch {
	case !oldOK && newOK:
		change.Kind = newObligation.Kind
		change.Change = "strengthened"
		change.Reason = "new obligation added to the certificate witness"
	case oldOK && !newOK:
		change.Kind = oldObligation.Kind
		change.Change = "weakened"
		change.Reason = "previous obligation removed from the certificate witness"
	case !sameStructure(oldObligation, newObligation):
		change.Change = "changed"
		change.Reason = "kind, formula, or evidence references changed; logical implication is not inferred"
	case oldObligation.Status == newObligation.Status:
		change.Change = "unchanged"
		change.Reason = "obligation kind, evidence, formula, and status are unchanged"
	case newObligation.Status == "refuted":
		change.Change = "refuted"
		change.Reason = "new certificate carries a counterexample for this obligation"
	case oldObligation.Status == "refuted":
		change.Change = "repaired"
		change.Reason = "new certificate no longer carries the previous counterexample state"
	default:
		oldRank := confidenceRank(oldObligation.Status)
		newRank := confidenceRank(newObligation.Status)
		if newRank > oldRank {
			change.Change = "strengthened"
			change.Reason = fmt.Sprintf("confidence status moved from %s to %s", oldObligation.Status, newObligation.Status)
		} else if newRank < oldRank {
			change.Change = "weakened"
			change.Reason = fmt.Sprintf("confidence status moved from %s to %s", oldObligation.Status, newObligation.Status)
		} else {
			change.Change = "changed"
			change.Reason = "status changed outside the confidence lattice"
		}
	}
	return change
}

func nonEmptyObligation(obligation certlang.Obligation) (certlang.Obligation, bool) {
	return obligation, obligation.ID != ""
}

func sameStructure(oldObligation, newObligation certlang.Obligation) bool {
	return oldObligation.Kind == newObligation.Kind &&
		oldObligation.Formula == newObligation.Formula &&
		strings.Join(oldObligation.Evidence, "\x00") == strings.Join(newObligation.Evidence, "\x00")
}

func confidenceRank(status string) int {
	switch status {
	case "checked":
		return 3
	case "assumed":
		return 2
	case "unsupported":
		return 1
	default:
		return 0
	}
}
