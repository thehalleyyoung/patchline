package repairescrow

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/thehalleyyoung/patchline/internal/canonical"
)

const SpecVersion = "patchline.repair-escrow/v1"
const ReportVersion = "patchline.repair-escrow-report/v1"

type Spec struct {
	Version      string        `json:"version"`
	Name         string        `json:"name"`
	Thresholds   Thresholds    `json:"thresholds"`
	Repairs      []Repair      `json:"repairs"`
	Evidence     []Evidence    `json:"evidence"`
	Reviews      []Review      `json:"reviews"`
	Certificates []Certificate `json:"certificates"`
}

type Thresholds struct {
	ManualReviews int `json:"manual_reviews"`
	Certificates  int `json:"certificates"`
	Evidence      int `json:"evidence,omitempty"`
}

type Repair struct {
	ID           string `json:"id"`
	Title        string `json:"title"`
	ArtifactHash string `json:"artifact_hash"`
	RiskClass    string `json:"risk_class,omitempty"`
}

type Evidence struct {
	ID           string `json:"id"`
	RepairID     string `json:"repair_id"`
	ArtifactHash string `json:"artifact_hash"`
	Kind         string `json:"kind"`
	Source       string `json:"source,omitempty"`
	Verdict      string `json:"verdict"`
}

type Review struct {
	ID           string `json:"id"`
	RepairID     string `json:"repair_id"`
	ArtifactHash string `json:"artifact_hash"`
	Reviewer     string `json:"reviewer"`
	Decision     string `json:"decision"`
}

type Certificate struct {
	ID           string `json:"id"`
	RepairID     string `json:"repair_id"`
	ArtifactHash string `json:"artifact_hash"`
	Issuer       string `json:"issuer"`
	Status       string `json:"status"`
}

type Report struct {
	Version    string         `json:"version"`
	Name       string         `json:"name"`
	OK         bool           `json:"ok"`
	Thresholds Thresholds     `json:"thresholds"`
	Summary    Summary        `json:"summary"`
	Repairs    []RepairReport `json:"repairs"`
	Hash       string         `json:"hash"`
}

type Summary struct {
	Repairs         int `json:"repairs"`
	Released        int `json:"released"`
	Held            int `json:"held"`
	Rejected        int `json:"rejected"`
	Obligations     int `json:"obligations"`
	Counterexamples int `json:"counterexamples"`
}

type RepairReport struct {
	ID              string           `json:"id"`
	Title           string           `json:"title"`
	ArtifactHash    string           `json:"artifact_hash"`
	RiskClass       string           `json:"risk_class,omitempty"`
	Status          string           `json:"status"`
	ManualReviews   EvidenceCounter  `json:"manual_reviews"`
	Certificates    EvidenceCounter  `json:"certificates"`
	Evidence        EvidenceCounter  `json:"evidence"`
	Obligations     []Obligation     `json:"obligations,omitempty"`
	Counterexamples []Counterexample `json:"counterexamples,omitempty"`
}

type EvidenceCounter struct {
	Required   int      `json:"required"`
	Accepted   int      `json:"accepted"`
	Duplicates int      `json:"duplicates"`
	Subjects   []string `json:"subjects,omitempty"`
}

type Obligation struct {
	ID        string `json:"id"`
	Status    string `json:"status"`
	Required  int    `json:"required"`
	Accepted  int    `json:"accepted"`
	Remaining int    `json:"remaining"`
	Reason    string `json:"reason"`
}

type Counterexample struct {
	ID      string `json:"id"`
	Kind    string `json:"kind"`
	Subject string `json:"subject,omitempty"`
	Reason  string `json:"reason"`
}

func ReadSpec(reader io.Reader) (Spec, error) {
	decoder := json.NewDecoder(reader)
	decoder.DisallowUnknownFields()
	var spec Spec
	if err := decoder.Decode(&spec); err != nil {
		return Spec{}, err
	}
	if spec.Version != SpecVersion {
		return Spec{}, fmt.Errorf("repair escrow spec version must be %s", SpecVersion)
	}
	return spec, nil
}

func BuildReport(spec Spec) (Report, error) {
	if err := validateSpec(spec); err != nil {
		return Report{}, err
	}
	report := Report{
		Version:    ReportVersion,
		Name:       spec.Name,
		OK:         true,
		Thresholds: normalizedThresholds(spec.Thresholds),
	}
	for _, repair := range sortedRepairs(spec.Repairs) {
		repairReport := evaluateRepair(repair, spec, report.Thresholds)
		report.Repairs = append(report.Repairs, repairReport)
		switch repairReport.Status {
		case "released":
			report.Summary.Released++
		case "rejected":
			report.OK = false
			report.Summary.Rejected++
		default:
			report.OK = false
			report.Summary.Held++
		}
		report.Summary.Obligations += len(repairReport.Obligations)
		report.Summary.Counterexamples += len(repairReport.Counterexamples)
	}
	report.Summary.Repairs = len(report.Repairs)
	report.Hash = reportHash(report)
	return report, nil
}

func RenderMarkdown(report Report) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Repair-risk escrow\n\n")
	fmt.Fprintf(&b, "Patchline holds proposed fixes until distinct manual reviewers and repair-bound certificates clear the configured thresholds. Manual rejections, revoked certificates, expired certificates, mismatched repair IDs, or mismatched artifact hashes prevent release.\n\n")
	fmt.Fprintf(&b, "## Summary\n\n")
	fmt.Fprintf(&b, "| Metric | Value |\n| --- | ---: |\n")
	fmt.Fprintf(&b, "| OK | `%t` |\n", report.OK)
	fmt.Fprintf(&b, "| Repairs | %d |\n", report.Summary.Repairs)
	fmt.Fprintf(&b, "| Released | %d |\n", report.Summary.Released)
	fmt.Fprintf(&b, "| Held | %d |\n", report.Summary.Held)
	fmt.Fprintf(&b, "| Rejected | %d |\n", report.Summary.Rejected)
	fmt.Fprintf(&b, "| Obligations | %d |\n", report.Summary.Obligations)
	fmt.Fprintf(&b, "| Counterexamples | %d |\n\n", report.Summary.Counterexamples)
	fmt.Fprintf(&b, "Thresholds: `%d` distinct manual reviewer(s), `%d` distinct certificate(s), `%d` accepted evidence item(s).\n\n", report.Thresholds.ManualReviews, report.Thresholds.Certificates, report.Thresholds.Evidence)

	fmt.Fprintf(&b, "## Repairs\n\n")
	fmt.Fprintf(&b, "| Repair | Status | Reviews | Certificates | Evidence | Outstanding |\n| --- | --- | ---: | ---: | ---: | --- |\n")
	for _, repair := range report.Repairs {
		outstanding := "-"
		if len(repair.Obligations) > 0 {
			parts := make([]string, 0, len(repair.Obligations))
			for _, obligation := range repair.Obligations {
				parts = append(parts, fmt.Sprintf("%s:%d", obligation.ID, obligation.Remaining))
			}
			outstanding = strings.Join(parts, ", ")
		}
		fmt.Fprintf(&b, "| `%s` | `%s` | %d/%d | %d/%d | %d/%d | %s |\n",
			repair.ID,
			repair.Status,
			repair.ManualReviews.Accepted,
			repair.ManualReviews.Required,
			repair.Certificates.Accepted,
			repair.Certificates.Required,
			repair.Evidence.Accepted,
			repair.Evidence.Required,
			outstanding)
	}
	if report.Summary.Counterexamples > 0 {
		fmt.Fprintf(&b, "\n## Blocking counterexamples\n\n")
		fmt.Fprintf(&b, "| Repair | Event | Kind | Subject | Reason |\n| --- | --- | --- | --- | --- |\n")
		for _, repair := range report.Repairs {
			for _, counterexample := range repair.Counterexamples {
				fmt.Fprintf(&b, "| `%s` | `%s` | `%s` | `%s` | %s |\n", repair.ID, counterexample.ID, counterexample.Kind, firstNonEmpty(counterexample.Subject, "-"), counterexample.Reason)
			}
		}
	}
	return b.String()
}

func validateSpec(spec Spec) error {
	if spec.Version != SpecVersion {
		return fmt.Errorf("repair escrow spec version must be %s", SpecVersion)
	}
	if spec.Name == "" {
		return fmt.Errorf("spec name is required")
	}
	if spec.Thresholds.ManualReviews < 0 || spec.Thresholds.Certificates < 0 || spec.Thresholds.Evidence < 0 {
		return fmt.Errorf("thresholds must be non-negative")
	}
	if len(spec.Repairs) == 0 {
		return fmt.Errorf("at least one repair is required")
	}
	ids := map[string]bool{}
	for _, repair := range spec.Repairs {
		if repair.ID == "" {
			return fmt.Errorf("repair id is required")
		}
		if ids[repair.ID] {
			return fmt.Errorf("repair id %q is duplicated", repair.ID)
		}
		ids[repair.ID] = true
		if repair.ArtifactHash == "" {
			return fmt.Errorf("repair %q artifact_hash is required", repair.ID)
		}
	}
	for _, review := range spec.Reviews {
		if err := validateEventBinding("manual review", review.ID, review.RepairID, review.ArtifactHash, ids); err != nil {
			return err
		}
	}
	for _, certificate := range spec.Certificates {
		if err := validateEventBinding("certificate", certificate.ID, certificate.RepairID, certificate.ArtifactHash, ids); err != nil {
			return err
		}
	}
	for _, item := range spec.Evidence {
		if err := validateEventBinding("evidence", item.ID, item.RepairID, item.ArtifactHash, ids); err != nil {
			return err
		}
	}
	return nil
}

func validateEventBinding(kind, id, repairID, artifactHash string, repairs map[string]bool) error {
	if id == "" {
		return fmt.Errorf("%s id is required", kind)
	}
	if repairID == "" {
		return fmt.Errorf("%s %q repair_id is required", kind, id)
	}
	if !repairs[repairID] {
		return fmt.Errorf("%s %q references unknown repair %q", kind, id, repairID)
	}
	if artifactHash == "" {
		return fmt.Errorf("%s %q artifact_hash is required", kind, id)
	}
	return nil
}

func evaluateRepair(repair Repair, spec Spec, thresholds Thresholds) RepairReport {
	report := RepairReport{
		ID:            repair.ID,
		Title:         repair.Title,
		ArtifactHash:  repair.ArtifactHash,
		RiskClass:     repair.RiskClass,
		ManualReviews: EvidenceCounter{Required: thresholds.ManualReviews},
		Certificates:  EvidenceCounter{Required: thresholds.Certificates},
		Evidence:      EvidenceCounter{Required: thresholds.Evidence},
	}

	reviewers := map[string]string{}
	seenReviews := map[string]bool{}
	for _, review := range sortedReviews(spec.Reviews) {
		if review.RepairID != repair.ID {
			continue
		}
		if seenReviews[review.ID] {
			if review.Decision == "rejected" {
				report.Counterexamples = append(report.Counterexamples, Counterexample{ID: review.ID, Kind: "manual_review", Subject: review.Reviewer, Reason: "manual reviewer rejected the proposed repair"})
			}
			report.ManualReviews.Duplicates++
			continue
		}
		seenReviews[review.ID] = true
		if review.ArtifactHash != repair.ArtifactHash {
			report.Counterexamples = append(report.Counterexamples, Counterexample{ID: review.ID, Kind: "manual_review", Subject: review.Reviewer, Reason: "manual review artifact hash does not match repair"})
			continue
		}
		if review.Reviewer == "" {
			report.Counterexamples = append(report.Counterexamples, Counterexample{ID: review.ID, Kind: "manual_review", Reason: "manual review reviewer is required"})
			continue
		}
		switch review.Decision {
		case "approved":
			if _, ok := reviewers[review.Reviewer]; !ok {
				reviewers[review.Reviewer] = review.ID
			} else {
				report.ManualReviews.Duplicates++
			}
		case "rejected":
			report.Counterexamples = append(report.Counterexamples, Counterexample{ID: review.ID, Kind: "manual_review", Subject: review.Reviewer, Reason: "manual reviewer rejected the proposed repair"})
		default:
			report.Counterexamples = append(report.Counterexamples, Counterexample{ID: review.ID, Kind: "manual_review", Subject: review.Reviewer, Reason: "manual review decision must be approved or rejected"})
		}
	}
	report.ManualReviews.Accepted = len(reviewers)
	report.ManualReviews.Subjects = sortedKeys(reviewers)

	certificates := map[string]bool{}
	seenCertificates := map[string]bool{}
	for _, certificate := range sortedCertificates(spec.Certificates) {
		if certificate.RepairID != repair.ID {
			continue
		}
		if seenCertificates[certificate.ID] {
			if certificate.Status == "revoked" || certificate.Status == "expired" {
				report.Counterexamples = append(report.Counterexamples, Counterexample{ID: certificate.ID, Kind: "certificate", Subject: certificate.Issuer, Reason: "duplicate certificate is " + certificate.Status})
			}
			report.Certificates.Duplicates++
			continue
		}
		seenCertificates[certificate.ID] = true
		if certificate.ArtifactHash != repair.ArtifactHash {
			report.Counterexamples = append(report.Counterexamples, Counterexample{ID: certificate.ID, Kind: "certificate", Subject: certificate.Issuer, Reason: "certificate artifact hash does not match repair"})
			continue
		}
		switch certificate.Status {
		case "valid":
			certificates[certificate.ID] = true
		case "revoked":
			report.Counterexamples = append(report.Counterexamples, Counterexample{ID: certificate.ID, Kind: "certificate", Subject: certificate.Issuer, Reason: "certificate is revoked"})
		case "expired":
			report.Counterexamples = append(report.Counterexamples, Counterexample{ID: certificate.ID, Kind: "certificate", Subject: certificate.Issuer, Reason: "certificate is expired"})
		default:
			report.Counterexamples = append(report.Counterexamples, Counterexample{ID: certificate.ID, Kind: "certificate", Subject: certificate.Issuer, Reason: "certificate status must be valid, revoked, or expired"})
		}
	}
	report.Certificates.Accepted = len(certificates)
	report.Certificates.Subjects = sortedBoolKeys(certificates)

	evidence := map[string]bool{}
	seenEvidence := map[string]bool{}
	for _, item := range sortedEvidence(spec.Evidence) {
		if item.RepairID != repair.ID {
			continue
		}
		if seenEvidence[item.ID] {
			if item.Verdict == "fail" {
				report.Counterexamples = append(report.Counterexamples, Counterexample{ID: item.ID, Kind: "evidence", Subject: item.Kind, Reason: "duplicate evidence verdict failed"})
			}
			report.Evidence.Duplicates++
			continue
		}
		seenEvidence[item.ID] = true
		if item.ArtifactHash != repair.ArtifactHash {
			report.Counterexamples = append(report.Counterexamples, Counterexample{ID: item.ID, Kind: "evidence", Subject: item.Kind, Reason: "evidence artifact hash does not match repair"})
			continue
		}
		switch item.Verdict {
		case "pass":
			evidence[item.ID] = true
		case "fail":
			report.Counterexamples = append(report.Counterexamples, Counterexample{ID: item.ID, Kind: "evidence", Subject: item.Kind, Reason: "evidence verdict failed"})
		default:
			report.Counterexamples = append(report.Counterexamples, Counterexample{ID: item.ID, Kind: "evidence", Subject: item.Kind, Reason: "evidence verdict must be pass or fail"})
		}
	}
	report.Evidence.Accepted = len(evidence)
	report.Evidence.Subjects = sortedBoolKeys(evidence)

	report.Obligations = obligations(report)
	sort.SliceStable(report.Counterexamples, func(i, j int) bool {
		if report.Counterexamples[i].Kind != report.Counterexamples[j].Kind {
			return report.Counterexamples[i].Kind < report.Counterexamples[j].Kind
		}
		return report.Counterexamples[i].ID < report.Counterexamples[j].ID
	})
	switch {
	case len(report.Counterexamples) > 0:
		report.Status = "rejected"
	case len(report.Obligations) > 0:
		report.Status = "held"
	default:
		report.Status = "released"
	}
	return report
}

func obligations(report RepairReport) []Obligation {
	var out []Obligation
	add := func(id string, counter EvidenceCounter, reason string) {
		if counter.Accepted >= counter.Required {
			return
		}
		out = append(out, Obligation{
			ID:        id,
			Status:    "open",
			Required:  counter.Required,
			Accepted:  counter.Accepted,
			Remaining: counter.Required - counter.Accepted,
			Reason:    reason,
		})
	}
	add("manual_review.threshold", report.ManualReviews, "repair needs more distinct manual reviewers before release")
	add("certificate.threshold", report.Certificates, "repair needs more distinct valid repair-bound certificates before release")
	add("evidence.threshold", report.Evidence, "repair needs more accepted repair-bound evidence items before release")
	sort.SliceStable(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func normalizedThresholds(thresholds Thresholds) Thresholds {
	if thresholds.ManualReviews == 0 {
		thresholds.ManualReviews = 1
	}
	if thresholds.Certificates == 0 {
		thresholds.Certificates = 1
	}
	if thresholds.Evidence == 0 {
		thresholds.Evidence = 1
	}
	return thresholds
}

func sortedRepairs(repairs []Repair) []Repair {
	out := append([]Repair(nil), repairs...)
	sort.SliceStable(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func sortedReviews(reviews []Review) []Review {
	out := append([]Review(nil), reviews...)
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].ID != out[j].ID {
			return out[i].ID < out[j].ID
		}
		if out[i].Reviewer != out[j].Reviewer {
			return out[i].Reviewer < out[j].Reviewer
		}
		return out[i].Decision < out[j].Decision
	})
	return out
}

func sortedCertificates(certificates []Certificate) []Certificate {
	out := append([]Certificate(nil), certificates...)
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].ID != out[j].ID {
			return out[i].ID < out[j].ID
		}
		if out[i].Issuer != out[j].Issuer {
			return out[i].Issuer < out[j].Issuer
		}
		return out[i].Status < out[j].Status
	})
	return out
}

func sortedEvidence(evidence []Evidence) []Evidence {
	out := append([]Evidence(nil), evidence...)
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].ID != out[j].ID {
			return out[i].ID < out[j].ID
		}
		if out[i].Kind != out[j].Kind {
			return out[i].Kind < out[j].Kind
		}
		return out[i].Verdict < out[j].Verdict
	})
	return out
}

func sortedKeys(values map[string]string) []string {
	out := make([]string, 0, len(values))
	for key := range values {
		out = append(out, key)
	}
	sort.Strings(out)
	return out
}

func sortedBoolKeys(values map[string]bool) []string {
	out := make([]string, 0, len(values))
	for key := range values {
		out = append(out, key)
	}
	sort.Strings(out)
	return out
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func reportHash(report Report) string {
	return canonical.Hash(struct {
		Version    string         `json:"version"`
		Name       string         `json:"name"`
		OK         bool           `json:"ok"`
		Thresholds Thresholds     `json:"thresholds"`
		Summary    Summary        `json:"summary"`
		Repairs    []RepairReport `json:"repairs"`
	}{report.Version, report.Name, report.OK, report.Thresholds, report.Summary, report.Repairs})
}
