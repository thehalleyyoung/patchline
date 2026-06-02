package certlang

import (
	"fmt"
	"sort"
	"strings"
)

type NormalizationResult struct {
	Version                   string       `json:"version"`
	CertificateID             string       `json:"certificate_id"`
	SourceSHA256              string       `json:"source_sha256"`
	NormalizedSHA256          string       `json:"normalized_sha256"`
	SourceCanonicalSHA256     string       `json:"source_canonical_sha256"`
	NormalizedCanonicalSHA256 string       `json:"normalized_canonical_sha256"`
	Changed                   bool         `json:"changed"`
	EvidenceItems             int          `json:"evidence_items"`
	Obligations               int          `json:"obligations"`
	Normalized                []byte       `json:"-"`
	Certificate               *Certificate `json:"-"`
}

func Normalize(data []byte, opts Options) (NormalizationResult, error) {
	cert, err := Parse(data, opts)
	if err != nil {
		return NormalizationResult{}, err
	}
	normalized := NormalizeCertificate(*cert)
	rendered, renderedCert, err := renderCurrentCertificate(normalized)
	if err != nil {
		return NormalizationResult{}, err
	}
	parsed, err := Parse(rendered, opts)
	if err != nil {
		return NormalizationResult{}, fmt.Errorf("normalized certificate failed validation: %w", err)
	}
	if !sameWitness(*cert, *parsed) {
		return NormalizationResult{}, fmt.Errorf("normalization changed certificate witness semantics")
	}
	return NormalizationResult{
		Version:                   Version,
		CertificateID:             renderedCert.CertificateID,
		SourceSHA256:              sha256Hex(data),
		NormalizedSHA256:          sha256Hex(rendered),
		SourceCanonicalSHA256:     cert.CanonicalHash,
		NormalizedCanonicalSHA256: renderedCert.CanonicalHash,
		Changed:                   string(data) != string(rendered),
		EvidenceItems:             len(renderedCert.Evidence),
		Obligations:               len(renderedCert.Obligations),
		Normalized:                rendered,
		Certificate:               parsed,
	}, nil
}

func NormalizeCertificate(cert Certificate) Certificate {
	normalized := cert
	normalized.Evidence = append([]Evidence(nil), cert.Evidence...)
	sort.Slice(normalized.Evidence, func(i, j int) bool {
		return evidenceKey(normalized.Evidence[i]) < evidenceKey(normalized.Evidence[j])
	})
	normalized.Obligations = append([]Obligation(nil), cert.Obligations...)
	for i := range normalized.Obligations {
		normalized.Obligations[i].Evidence = normalizeRefs(normalized.Obligations[i].Evidence)
	}
	sort.Slice(normalized.Obligations, func(i, j int) bool {
		return obligationKey(normalized.Obligations[i]) < obligationKey(normalized.Obligations[j])
	})
	return normalized
}

func sameWitness(left, right Certificate) bool {
	left = NormalizeCertificate(left)
	right = NormalizeCertificate(right)
	left.CanonicalHash = ""
	right.CanonicalHash = ""
	if left.CertificateID != right.CertificateID ||
		left.SubjectRepo != right.SubjectRepo ||
		left.SubjectRef != right.SubjectRef ||
		left.SubjectPath != right.SubjectPath ||
		left.SubjectSHA256 != right.SubjectSHA256 ||
		left.IssuedAt != right.IssuedAt ||
		left.Producer != right.Producer ||
		left.Verdict != right.Verdict ||
		left.RiskBPS != right.RiskBPS ||
		len(left.Evidence) != len(right.Evidence) ||
		len(left.Obligations) != len(right.Obligations) {
		return false
	}
	for i := range left.Evidence {
		if left.Evidence[i] != right.Evidence[i] {
			return false
		}
	}
	for i := range left.Obligations {
		if left.Obligations[i].ID != right.Obligations[i].ID ||
			left.Obligations[i].Kind != right.Obligations[i].Kind ||
			left.Obligations[i].Status != right.Obligations[i].Status ||
			left.Obligations[i].Formula != right.Obligations[i].Formula ||
			strings.Join(left.Obligations[i].Evidence, "\x00") != strings.Join(right.Obligations[i].Evidence, "\x00") {
			return false
		}
	}
	return true
}

func normalizeRefs(refs []string) []string {
	if len(refs) == 0 {
		return nil
	}
	copied := append([]string(nil), refs...)
	sort.Strings(copied)
	out := copied[:0]
	for _, ref := range copied {
		if len(out) == 0 || out[len(out)-1] != ref {
			out = append(out, ref)
		}
	}
	return out
}

func evidenceKey(item Evidence) string {
	return item.ID + "\x00" + item.Type + "\x00" + item.URI + "\x00" + item.SHA256
}

func obligationKey(item Obligation) string {
	return item.ID + "\x00" + item.Kind + "\x00" + item.Status + "\x00" + strings.Join(item.Evidence, "\x00") + "\x00" + item.Formula
}
