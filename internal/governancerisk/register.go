package governancerisk

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/thehalleyyoung/patchline/internal/canonical"
)

const SpecVersion = "patchline.governance-risk-register/v1"
const ReportVersion = "patchline.governance-risk-register-report/v1"

var requiredGovernanceDomains = []string{"maintainership", "funding", "infrastructure", "benchmark_control"}

type Spec struct {
	Version  string   `json:"version"`
	Name     string   `json:"name"`
	AsOfDate string   `json:"as_of_date"`
	Criteria Criteria `json:"criteria"`
	Entries  []Entry  `json:"entries"`
}

type Criteria struct {
	RequiredDomains                 []string `json:"required_domains"`
	MaxOwnerShare                   float64  `json:"max_owner_share"`
	MaxOrganizationShare            float64  `json:"max_organization_share"`
	MinIndependentOwnersPerDomain   int      `json:"min_independent_owners_per_domain"`
	MinIndependentOrgsPerDomain     int      `json:"min_independent_orgs_per_domain"`
	MinMitigationsPerHighRiskDomain int      `json:"min_mitigations_per_high_risk_domain"`
	RequireEvidencePaths            bool     `json:"require_evidence_paths"`
	RequireRotationPlan             bool     `json:"require_rotation_plan"`
	ReviewCadenceDays               int      `json:"review_cadence_days"`
}

type Entry struct {
	AssetID       string   `json:"asset_id"`
	Domain        string   `json:"domain"`
	AssetName     string   `json:"asset_name"`
	Owner         string   `json:"owner"`
	Organization  string   `json:"organization"`
	ControlType   string   `json:"control_type"`
	Weight        float64  `json:"weight"`
	LastReviewed  string   `json:"last_reviewed"`
	RotationPlan  string   `json:"rotation_plan,omitempty"`
	Mitigations   []string `json:"mitigations,omitempty"`
	EvidencePaths []string `json:"evidence_paths,omitempty"`
}

type Report struct {
	Version         string           `json:"version"`
	Name            string           `json:"name"`
	AsOfDate        string           `json:"as_of_date"`
	OK              bool             `json:"ok"`
	Criteria        Criteria         `json:"criteria"`
	Summary         Summary          `json:"summary"`
	Domains         []DomainReport   `json:"domains"`
	Counterexamples []Counterexample `json:"counterexamples,omitempty"`
	Hash            string           `json:"hash"`
}

type Summary struct {
	Domains               int     `json:"domains"`
	Entries               int     `json:"entries"`
	TotalWeight           float64 `json:"total_weight"`
	HighRiskDomains       int     `json:"high_risk_domains"`
	MediumRiskDomains     int     `json:"medium_risk_domains"`
	EvidenceFiles         int     `json:"evidence_files"`
	MaxOwnerShare         float64 `json:"max_owner_share"`
	MaxOrganizationShare  float64 `json:"max_organization_share"`
	MinIndependentOwners  int     `json:"min_independent_owners"`
	MinIndependentOrgs    int     `json:"min_independent_orgs"`
	StaleReviews          int     `json:"stale_reviews"`
	MissingRotationPlans  int     `json:"missing_rotation_plans"`
	MissingEvidenceAssets int     `json:"missing_evidence_assets"`
	Counterexamples       int     `json:"counterexamples"`
}

type DomainReport struct {
	Domain                string             `json:"domain"`
	RiskLevel             string             `json:"risk_level"`
	TotalWeight           float64            `json:"total_weight"`
	TopOwner              string             `json:"top_owner"`
	TopOwnerShare         float64            `json:"top_owner_share"`
	TopOrganization       string             `json:"top_organization"`
	TopOrganizationShare  float64            `json:"top_organization_share"`
	DistinctOwners        int                `json:"distinct_owners"`
	DistinctOrganizations int                `json:"distinct_organizations"`
	Mitigations           int                `json:"mitigations"`
	StaleReviews          int                `json:"stale_reviews"`
	MissingRotationPlans  int                `json:"missing_rotation_plans"`
	MissingEvidence       int                `json:"missing_evidence"`
	Evidence              []ArtifactEvidence `json:"evidence"`
	Assets                []EntryReport      `json:"assets"`
}

type EntryReport struct {
	AssetID       string             `json:"asset_id"`
	AssetName     string             `json:"asset_name"`
	Domain        string             `json:"domain"`
	Owner         string             `json:"owner"`
	Organization  string             `json:"organization"`
	ControlType   string             `json:"control_type"`
	Weight        float64            `json:"weight"`
	LastReviewed  string             `json:"last_reviewed"`
	ReviewAgeDays int                `json:"review_age_days"`
	StaleReview   bool               `json:"stale_review"`
	RotationPlan  string             `json:"rotation_plan,omitempty"`
	Mitigations   []string           `json:"mitigations,omitempty"`
	Evidence      []ArtifactEvidence `json:"evidence"`
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

type domainAccumulator struct {
	domain               string
	totalWeight          float64
	ownerWeights         map[string]float64
	ownerNames           map[string]string
	organizationWeights  map[string]float64
	organizationNames    map[string]string
	mitigations          map[string]bool
	evidence             map[string]ArtifactEvidence
	assets               []EntryReport
	staleReviews         int
	missingRotationPlans int
	missingEvidence      int
}

func ReadSpec(reader io.Reader) (Spec, error) {
	decoder := json.NewDecoder(reader)
	decoder.DisallowUnknownFields()
	var spec Spec
	if err := decoder.Decode(&spec); err != nil {
		return Spec{}, err
	}
	if spec.Version != SpecVersion {
		return Spec{}, fmt.Errorf("governance-risk register spec version must be %s", SpecVersion)
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
	asOf := mustParseTime(spec.AsOfDate)
	report := Report{
		Version:  ReportVersion,
		Name:     spec.Name,
		AsOfDate: spec.AsOfDate,
		OK:       true,
		Criteria: spec.Criteria,
	}
	accs := map[string]*domainAccumulator{}
	evidenceSeen := map[string]ArtifactEvidence{}
	var counterexamples []Counterexample

	for _, entry := range sortedEntries(spec.Entries) {
		entry.Domain = normalizeDomain(entry.Domain)
		entryReport, entryCounterexamples := evaluateEntry(spec.Criteria, entry, rootAbs, asOf)
		counterexamples = append(counterexamples, entryCounterexamples...)
		acc := accs[entry.Domain]
		if acc == nil {
			acc = &domainAccumulator{
				domain:              entry.Domain,
				ownerWeights:        map[string]float64{},
				ownerNames:          map[string]string{},
				organizationWeights: map[string]float64{},
				organizationNames:   map[string]string{},
				mitigations:         map[string]bool{},
				evidence:            map[string]ArtifactEvidence{},
			}
			accs[entry.Domain] = acc
		}
		addEntry(acc, entryReport)
		for _, evidence := range entryReport.Evidence {
			evidenceSeen[evidence.Path] = evidence
		}
		report.Summary.Entries++
		report.Summary.TotalWeight += entry.Weight
	}

	report.Domains = finalizeDomains(accs, spec.Criteria)
	report.Summary = summarize(report.Summary, report.Domains, evidenceSeen)
	counterexamples = append(counterexamples, criteriaCounterexamples(spec.Criteria, report.Domains)...)
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
	jsonFile, err := os.Create(filepath.Join(outDir, "governance-risk-register.json"))
	if err != nil {
		return err
	}
	if err := canonical.WriteJSON(jsonFile, report); err != nil {
		_ = jsonFile.Close()
		return err
	}
	if err := jsonFile.Close(); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(outDir, "governance-risk-register.md"), []byte(RenderMarkdown(report)), 0o644)
}

func RenderMarkdown(report Report) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Governance-risk register\n\n")
	fmt.Fprintf(&b, "Patchline tracks concentration risk across maintainership, funding, infrastructure, and benchmark control before treating governance as stable research infrastructure.\n\n")
	fmt.Fprintf(&b, "## Summary\n\n")
	fmt.Fprintf(&b, "| Metric | Value |\n| --- | ---: |\n")
	fmt.Fprintf(&b, "| OK | `%t` |\n", report.OK)
	fmt.Fprintf(&b, "| As of | `%s` |\n", report.AsOfDate)
	fmt.Fprintf(&b, "| Domains | %d |\n", report.Summary.Domains)
	fmt.Fprintf(&b, "| Assets | %d |\n", report.Summary.Entries)
	fmt.Fprintf(&b, "| Evidence files | %d |\n", report.Summary.EvidenceFiles)
	fmt.Fprintf(&b, "| Max owner share | %.4f |\n", report.Summary.MaxOwnerShare)
	fmt.Fprintf(&b, "| Max organization share | %.4f |\n", report.Summary.MaxOrganizationShare)
	fmt.Fprintf(&b, "| High-risk domains | %d |\n", report.Summary.HighRiskDomains)
	fmt.Fprintf(&b, "| Medium-risk domains | %d |\n", report.Summary.MediumRiskDomains)
	fmt.Fprintf(&b, "| Stale reviews | %d |\n", report.Summary.StaleReviews)
	fmt.Fprintf(&b, "| Counterexamples | %d |\n\n", report.Summary.Counterexamples)
	fmt.Fprintf(&b, "Policy: required domains `%s`, owner share at most `%.2f`, organization share at most `%.2f`, at least `%d` independent owners and `%d` independent organizations per domain, review cadence `%d` days.\n\n",
		strings.Join(report.Criteria.RequiredDomains, ", "),
		report.Criteria.MaxOwnerShare,
		report.Criteria.MaxOrganizationShare,
		report.Criteria.MinIndependentOwnersPerDomain,
		report.Criteria.MinIndependentOrgsPerDomain,
		report.Criteria.ReviewCadenceDays,
	)

	fmt.Fprintf(&b, "## Domain concentration\n\n")
	fmt.Fprintf(&b, "| Domain | Risk | Top owner | Owner share | Top organization | Org share | Owners | Orgs | Mitigations | Stale | Evidence |\n")
	fmt.Fprintf(&b, "| --- | --- | --- | ---: | --- | ---: | ---: | ---: | ---: | ---: | ---: |\n")
	for _, domain := range report.Domains {
		fmt.Fprintf(&b, "| `%s` | `%s` | `%s` | %.4f | `%s` | %.4f | %d | %d | %d | %d | %d |\n",
			domain.Domain,
			domain.RiskLevel,
			escapePipes(domain.TopOwner),
			domain.TopOwnerShare,
			escapePipes(domain.TopOrganization),
			domain.TopOrganizationShare,
			domain.DistinctOwners,
			domain.DistinctOrganizations,
			domain.Mitigations,
			domain.StaleReviews,
			len(domain.Evidence),
		)
	}

	fmt.Fprintf(&b, "\n## Controlled assets\n\n")
	fmt.Fprintf(&b, "| Domain | Asset | Owner | Organization | Weight | Age days | Evidence | Mitigations |\n")
	fmt.Fprintf(&b, "| --- | --- | --- | --- | ---: | ---: | ---: | ---: |\n")
	for _, domain := range report.Domains {
		for _, asset := range domain.Assets {
			fmt.Fprintf(&b, "| `%s` | `%s` | `%s` | `%s` | %.2f | %d | %d | %d |\n",
				asset.Domain,
				escapePipes(asset.AssetID),
				escapePipes(asset.Owner),
				escapePipes(asset.Organization),
				asset.Weight,
				asset.ReviewAgeDays,
				len(asset.Evidence),
				len(asset.Mitigations),
			)
		}
	}
	if len(report.Counterexamples) > 0 {
		fmt.Fprintf(&b, "\n## Counterexamples\n\n")
		fmt.Fprintf(&b, "| ID | Kind | Subject | Message |\n| --- | --- | --- | --- |\n")
		for _, counterexample := range report.Counterexamples {
			fmt.Fprintf(&b, "| `%s` | `%s` | `%s` | %s |\n", counterexample.ID, counterexample.Kind, firstNonEmpty(counterexample.Subject, "-"), escapePipes(counterexample.Message))
		}
	}
	return b.String()
}

func evaluateEntry(criteria Criteria, entry Entry, rootAbs string, asOf time.Time) (EntryReport, []Counterexample) {
	lastReviewed := mustParseTime(entry.LastReviewed)
	ageDays := int(math.Floor(asOf.Sub(lastReviewed).Hours() / 24))
	report := EntryReport{
		AssetID:       strings.TrimSpace(entry.AssetID),
		AssetName:     strings.TrimSpace(entry.AssetName),
		Domain:        normalizeDomain(entry.Domain),
		Owner:         strings.TrimSpace(entry.Owner),
		Organization:  strings.TrimSpace(entry.Organization),
		ControlType:   strings.TrimSpace(entry.ControlType),
		Weight:        round4(entry.Weight),
		LastReviewed:  entry.LastReviewed,
		ReviewAgeDays: ageDays,
		RotationPlan:  strings.TrimSpace(entry.RotationPlan),
		Mitigations:   sortedNonEmpty(entry.Mitigations),
	}
	var counterexamples []Counterexample
	if lastReviewed.After(asOf) {
		counterexamples = append(counterexamples, Counterexample{
			ID:      fmt.Sprintf("review-in-future-%s", safeID(entry.AssetID)),
			Kind:    "review_in_future",
			Subject: entry.AssetID,
			Message: "last_reviewed must not be after the register as_of_date",
			Witness: []string{entry.LastReviewed, asOf.Format(time.RFC3339)},
		})
	}
	if criteria.ReviewCadenceDays > 0 && ageDays > criteria.ReviewCadenceDays {
		report.StaleReview = true
		counterexamples = append(counterexamples, Counterexample{
			ID:      fmt.Sprintf("stale-review-%s", safeID(entry.AssetID)),
			Kind:    "stale_review",
			Subject: entry.AssetID,
			Message: fmt.Sprintf("asset review age %d days exceeds cadence %d days", ageDays, criteria.ReviewCadenceDays),
			Witness: []string{entry.LastReviewed, asOf.Format(time.RFC3339)},
		})
	}
	if criteria.RequireRotationPlan && report.RotationPlan == "" {
		counterexamples = append(counterexamples, Counterexample{
			ID:      fmt.Sprintf("missing-rotation-plan-%s", safeID(entry.AssetID)),
			Kind:    "missing_rotation_plan",
			Subject: entry.AssetID,
			Message: "governance-controlled asset must declare a rotation or succession plan",
		})
	}
	seen := map[string]bool{}
	for _, relPath := range sortedStrings(entry.EvidencePaths) {
		clean := filepath.Clean(strings.TrimSpace(relPath))
		if seen[clean] {
			continue
		}
		seen[clean] = true
		evidence, fileCounterexamples := resolveFileUnderRoot(rootAbs, relPath, entry.AssetID, "governance_evidence")
		counterexamples = append(counterexamples, fileCounterexamples...)
		if evidence != nil {
			report.Evidence = append(report.Evidence, *evidence)
		}
	}
	if criteria.RequireEvidencePaths && len(report.Evidence) == 0 {
		counterexamples = append(counterexamples, Counterexample{
			ID:      fmt.Sprintf("missing-entry-evidence-%s", safeID(entry.AssetID)),
			Kind:    "missing_entry_evidence",
			Subject: entry.AssetID,
			Message: "governance-controlled asset must preserve at least one readable evidence file",
		})
	}
	return report, counterexamples
}

func addEntry(acc *domainAccumulator, entry EntryReport) {
	acc.totalWeight += entry.Weight
	ownerKey := normalizeIdentity(entry.Owner)
	orgKey := normalizeIdentity(entry.Organization)
	acc.ownerWeights[ownerKey] += entry.Weight
	acc.organizationWeights[orgKey] += entry.Weight
	if _, ok := acc.ownerNames[ownerKey]; !ok {
		acc.ownerNames[ownerKey] = entry.Owner
	}
	if _, ok := acc.organizationNames[orgKey]; !ok {
		acc.organizationNames[orgKey] = entry.Organization
	}
	for _, mitigation := range entry.Mitigations {
		acc.mitigations[normalizeIdentity(mitigation)] = true
	}
	for _, evidence := range entry.Evidence {
		acc.evidence[evidence.Path] = evidence
	}
	if entry.StaleReview {
		acc.staleReviews++
	}
	if strings.TrimSpace(entry.RotationPlan) == "" {
		acc.missingRotationPlans++
	}
	if len(entry.Evidence) == 0 {
		acc.missingEvidence++
	}
	acc.assets = append(acc.assets, entry)
}

func finalizeDomains(accs map[string]*domainAccumulator, criteria Criteria) []DomainReport {
	var domains []DomainReport
	for _, acc := range accs {
		topOwner, topOwnerShare := topShare(acc.ownerWeights, acc.ownerNames, acc.totalWeight)
		topOrg, topOrgShare := topShare(acc.organizationWeights, acc.organizationNames, acc.totalWeight)
		domain := DomainReport{
			Domain:                acc.domain,
			TotalWeight:           round4(acc.totalWeight),
			TopOwner:              topOwner,
			TopOwnerShare:         topOwnerShare,
			TopOrganization:       topOrg,
			TopOrganizationShare:  topOrgShare,
			DistinctOwners:        len(acc.ownerWeights),
			DistinctOrganizations: len(acc.organizationWeights),
			Mitigations:           len(acc.mitigations),
			StaleReviews:          acc.staleReviews,
			MissingRotationPlans:  acc.missingRotationPlans,
			MissingEvidence:       acc.missingEvidence,
			Evidence:              sortedEvidence(acc.evidence),
			Assets:                sortedEntryReports(acc.assets),
		}
		domain.RiskLevel = classifyRisk(domain, criteria)
		domains = append(domains, domain)
	}
	sort.Slice(domains, func(i, j int) bool {
		return domains[i].Domain < domains[j].Domain
	})
	return domains
}

func summarize(summary Summary, domains []DomainReport, evidenceSeen map[string]ArtifactEvidence) Summary {
	summary.Domains = len(domains)
	summary.TotalWeight = round4(summary.TotalWeight)
	summary.EvidenceFiles = len(evidenceSeen)
	maxInt := int(^uint(0) >> 1)
	summary.MinIndependentOwners = maxInt
	summary.MinIndependentOrgs = maxInt
	for _, domain := range domains {
		if domain.RiskLevel == "high" {
			summary.HighRiskDomains++
		}
		if domain.RiskLevel == "medium" {
			summary.MediumRiskDomains++
		}
		if domain.TopOwnerShare > summary.MaxOwnerShare {
			summary.MaxOwnerShare = domain.TopOwnerShare
		}
		if domain.TopOrganizationShare > summary.MaxOrganizationShare {
			summary.MaxOrganizationShare = domain.TopOrganizationShare
		}
		if domain.DistinctOwners < summary.MinIndependentOwners {
			summary.MinIndependentOwners = domain.DistinctOwners
		}
		if domain.DistinctOrganizations < summary.MinIndependentOrgs {
			summary.MinIndependentOrgs = domain.DistinctOrganizations
		}
		summary.StaleReviews += domain.StaleReviews
		summary.MissingRotationPlans += domain.MissingRotationPlans
		summary.MissingEvidenceAssets += domain.MissingEvidence
	}
	if len(domains) == 0 {
		summary.MinIndependentOwners = 0
		summary.MinIndependentOrgs = 0
	}
	return summary
}

func criteriaCounterexamples(criteria Criteria, domains []DomainReport) []Counterexample {
	var counterexamples []Counterexample
	domainByName := map[string]DomainReport{}
	for _, domain := range domains {
		domainByName[domain.Domain] = domain
	}
	for _, required := range criteria.RequiredDomains {
		required = normalizeDomain(required)
		if _, ok := domainByName[required]; !ok {
			counterexamples = append(counterexamples, Counterexample{
				ID:      fmt.Sprintf("missing-required-domain-%s", safeID(required)),
				Kind:    "missing_required_domain",
				Subject: required,
				Message: "governance-risk register must track every required concentration domain",
			})
		}
	}
	for _, domain := range domains {
		if domain.TopOwnerShare > criteria.MaxOwnerShare {
			counterexamples = append(counterexamples, Counterexample{
				ID:      fmt.Sprintf("owner-share-exceeded-%s", safeID(domain.Domain)),
				Kind:    "owner_share_exceeded",
				Subject: domain.Domain,
				Message: fmt.Sprintf("top owner %q controls %.4f of %s, above limit %.4f", domain.TopOwner, domain.TopOwnerShare, domain.Domain, criteria.MaxOwnerShare),
				Witness: []string{domain.TopOwner, fmt.Sprintf("%.4f", domain.TopOwnerShare), fmt.Sprintf("%.4f", criteria.MaxOwnerShare)},
			})
		}
		if domain.TopOrganizationShare > criteria.MaxOrganizationShare {
			counterexamples = append(counterexamples, Counterexample{
				ID:      fmt.Sprintf("organization-share-exceeded-%s", safeID(domain.Domain)),
				Kind:    "organization_share_exceeded",
				Subject: domain.Domain,
				Message: fmt.Sprintf("top organization %q controls %.4f of %s, above limit %.4f", domain.TopOrganization, domain.TopOrganizationShare, domain.Domain, criteria.MaxOrganizationShare),
				Witness: []string{domain.TopOrganization, fmt.Sprintf("%.4f", domain.TopOrganizationShare), fmt.Sprintf("%.4f", criteria.MaxOrganizationShare)},
			})
		}
		if domain.DistinctOwners < criteria.MinIndependentOwnersPerDomain {
			counterexamples = append(counterexamples, Counterexample{
				ID:      fmt.Sprintf("insufficient-independent-owners-%s", safeID(domain.Domain)),
				Kind:    "insufficient_independent_owners",
				Subject: domain.Domain,
				Message: fmt.Sprintf("%s has %d independent owners, below required %d", domain.Domain, domain.DistinctOwners, criteria.MinIndependentOwnersPerDomain),
			})
		}
		if domain.DistinctOrganizations < criteria.MinIndependentOrgsPerDomain {
			counterexamples = append(counterexamples, Counterexample{
				ID:      fmt.Sprintf("insufficient-independent-orgs-%s", safeID(domain.Domain)),
				Kind:    "insufficient_independent_organizations",
				Subject: domain.Domain,
				Message: fmt.Sprintf("%s has %d independent organizations, below required %d", domain.Domain, domain.DistinctOrganizations, criteria.MinIndependentOrgsPerDomain),
			})
		}
		if domain.RiskLevel == "high" && domain.Mitigations < criteria.MinMitigationsPerHighRiskDomain {
			counterexamples = append(counterexamples, Counterexample{
				ID:      fmt.Sprintf("insufficient-high-risk-mitigations-%s", safeID(domain.Domain)),
				Kind:    "insufficient_high_risk_mitigations",
				Subject: domain.Domain,
				Message: fmt.Sprintf("high-risk domain has %d mitigations, below required %d", domain.Mitigations, criteria.MinMitigationsPerHighRiskDomain),
			})
		}
	}
	return counterexamples
}

func validateSpec(spec Spec) error {
	if spec.Version != SpecVersion {
		return fmt.Errorf("governance-risk register spec version must be %s", SpecVersion)
	}
	if strings.TrimSpace(spec.Name) == "" {
		return fmt.Errorf("governance-risk register name is required")
	}
	if _, err := time.Parse(time.RFC3339, spec.AsOfDate); err != nil {
		return fmt.Errorf("as_of_date must be RFC3339: %v", err)
	}
	if err := validateCriteria(spec.Criteria); err != nil {
		return err
	}
	if len(spec.Entries) == 0 {
		return fmt.Errorf("at least one governance-risk entry is required")
	}
	seenAssets := map[string]bool{}
	for _, entry := range spec.Entries {
		if strings.TrimSpace(entry.AssetID) == "" {
			return fmt.Errorf("entry asset_id is required")
		}
		assetKey := normalizeIdentity(entry.AssetID)
		if seenAssets[assetKey] {
			return fmt.Errorf("duplicate asset_id %q", entry.AssetID)
		}
		seenAssets[assetKey] = true
		domain := normalizeDomain(entry.Domain)
		if !allowedDomain(domain) {
			return fmt.Errorf("entry %q domain must be one of %s", entry.AssetID, strings.Join(requiredGovernanceDomains, ", "))
		}
		if strings.TrimSpace(entry.AssetName) == "" || strings.TrimSpace(entry.Owner) == "" || strings.TrimSpace(entry.Organization) == "" || strings.TrimSpace(entry.ControlType) == "" {
			return fmt.Errorf("entry %q must include asset_name, owner, organization, and control_type", entry.AssetID)
		}
		if entry.Weight <= 0 || !isFinite(entry.Weight) {
			return fmt.Errorf("entry %q weight must be finite and positive", entry.AssetID)
		}
		if _, err := time.Parse(time.RFC3339, entry.LastReviewed); err != nil {
			return fmt.Errorf("entry %q last_reviewed must be RFC3339: %v", entry.AssetID, err)
		}
	}
	return nil
}

func validateCriteria(criteria Criteria) error {
	required := map[string]bool{}
	for _, domain := range criteria.RequiredDomains {
		normalized := normalizeDomain(domain)
		if !allowedDomain(normalized) {
			return fmt.Errorf("criteria.required_domains contains unsupported domain %q", domain)
		}
		required[normalized] = true
	}
	for _, domain := range requiredGovernanceDomains {
		if !required[domain] {
			return fmt.Errorf("criteria.required_domains must include %q", domain)
		}
	}
	if criteria.MaxOwnerShare <= 0 || criteria.MaxOwnerShare > 1 || !isFinite(criteria.MaxOwnerShare) {
		return fmt.Errorf("criteria.max_owner_share must be finite and in (0,1]")
	}
	if criteria.MaxOrganizationShare <= 0 || criteria.MaxOrganizationShare > 1 || !isFinite(criteria.MaxOrganizationShare) {
		return fmt.Errorf("criteria.max_organization_share must be finite and in (0,1]")
	}
	if criteria.MinIndependentOwnersPerDomain < 2 {
		return fmt.Errorf("criteria.min_independent_owners_per_domain must be at least 2")
	}
	if criteria.MinIndependentOrgsPerDomain < 2 {
		return fmt.Errorf("criteria.min_independent_orgs_per_domain must be at least 2")
	}
	if criteria.MinMitigationsPerHighRiskDomain < 1 {
		return fmt.Errorf("criteria.min_mitigations_per_high_risk_domain must be at least 1")
	}
	if criteria.ReviewCadenceDays < 1 {
		return fmt.Errorf("criteria.review_cadence_days must be at least 1")
	}
	return nil
}

func resolveFileUnderRoot(rootAbs, relPath, subject, kind string) (*ArtifactEvidence, []Counterexample) {
	clean := filepath.Clean(strings.TrimSpace(relPath))
	if clean == "" || clean == "." || filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return nil, []Counterexample{{
			ID:      fmt.Sprintf("%s-path-%s-%s", strings.ReplaceAll(kind, "_", "-"), safeID(subject), safeID(relPath)),
			Kind:    "invalid_evidence_path",
			Subject: subject,
			Message: fmt.Sprintf("%s path %q must be a relative file below the register root", strings.ReplaceAll(kind, "_", " "), relPath),
			Witness: []string{relPath},
		}}
	}
	artifactPath := filepath.Join(rootAbs, clean)
	info, err := os.Lstat(artifactPath)
	if err != nil {
		return nil, []Counterexample{{
			ID:      fmt.Sprintf("missing-%s-%s-%s", strings.ReplaceAll(kind, "_", "-"), safeID(subject), safeID(clean)),
			Kind:    "missing_evidence",
			Subject: subject,
			Message: fmt.Sprintf("%s file %q is missing", strings.ReplaceAll(kind, "_", " "), clean),
			Witness: []string{clean},
		}}
	}
	if !info.Mode().IsRegular() {
		return nil, []Counterexample{{
			ID:      fmt.Sprintf("invalid-%s-file-%s-%s", strings.ReplaceAll(kind, "_", "-"), safeID(subject), safeID(clean)),
			Kind:    "invalid_evidence_file",
			Subject: subject,
			Message: fmt.Sprintf("%s file %q must be a regular file under the register root", strings.ReplaceAll(kind, "_", " "), clean),
			Witness: []string{clean},
		}}
	}
	rootReal, err := filepath.EvalSymlinks(rootAbs)
	if err != nil {
		return nil, []Counterexample{{
			ID:      fmt.Sprintf("invalid-%s-root-%s", strings.ReplaceAll(kind, "_", "-"), safeID(subject)),
			Kind:    "invalid_evidence_root",
			Subject: subject,
			Message: fmt.Sprintf("register root %q could not be resolved without symlinks: %v", rootAbs, err),
			Witness: []string{rootAbs},
		}}
	}
	artifactReal, err := filepath.EvalSymlinks(artifactPath)
	if err != nil {
		return nil, []Counterexample{{
			ID:      fmt.Sprintf("unreadable-%s-%s-%s", strings.ReplaceAll(kind, "_", "-"), safeID(subject), safeID(clean)),
			Kind:    "unreadable_evidence",
			Subject: subject,
			Message: fmt.Sprintf("%s file %q could not be resolved without symlinks: %v", strings.ReplaceAll(kind, "_", " "), clean, err),
			Witness: []string{clean},
		}}
	}
	relToRoot, err := filepath.Rel(rootReal, artifactReal)
	if err != nil || relToRoot == ".." || strings.HasPrefix(relToRoot, ".."+string(filepath.Separator)) || filepath.IsAbs(relToRoot) {
		return nil, []Counterexample{{
			ID:      fmt.Sprintf("escaped-%s-file-%s-%s", strings.ReplaceAll(kind, "_", "-"), safeID(subject), safeID(clean)),
			Kind:    "invalid_evidence_file",
			Subject: subject,
			Message: fmt.Sprintf("%s file %q resolves outside the register root", strings.ReplaceAll(kind, "_", " "), clean),
			Witness: []string{clean, artifactReal, rootReal},
		}}
	}
	bytes, err := os.ReadFile(artifactPath)
	if err != nil {
		return nil, []Counterexample{{
			ID:      fmt.Sprintf("unreadable-%s-%s-%s", strings.ReplaceAll(kind, "_", "-"), safeID(subject), safeID(clean)),
			Kind:    "unreadable_evidence",
			Subject: subject,
			Message: fmt.Sprintf("%s file %q could not be read: %v", strings.ReplaceAll(kind, "_", " "), clean, err),
			Witness: []string{clean},
		}}
	}
	sum := sha256.Sum256(bytes)
	return &ArtifactEvidence{Path: filepath.ToSlash(clean), SHA256: hex.EncodeToString(sum[:]), Bytes: info.Size()}, nil
}

func topShare(weights map[string]float64, names map[string]string, total float64) (string, float64) {
	if total <= 0 || len(weights) == 0 {
		return "", 0
	}
	keys := make([]string, 0, len(weights))
	for key := range weights {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	bestKey := keys[0]
	bestWeight := weights[bestKey]
	for _, key := range keys[1:] {
		if weights[key] > bestWeight {
			bestKey = key
			bestWeight = weights[key]
		}
	}
	return names[bestKey], round4(bestWeight / total)
}

func classifyRisk(domain DomainReport, criteria Criteria) string {
	if domain.TopOwnerShare > criteria.MaxOwnerShare ||
		domain.TopOrganizationShare > criteria.MaxOrganizationShare ||
		domain.DistinctOwners < criteria.MinIndependentOwnersPerDomain ||
		domain.DistinctOrganizations < criteria.MinIndependentOrgsPerDomain {
		return "high"
	}
	if domain.TopOwnerShare >= criteria.MaxOwnerShare*0.85 || domain.TopOrganizationShare >= criteria.MaxOrganizationShare*0.85 {
		return "medium"
	}
	return "low"
}

func reportHash(report Report) string {
	copyReport := report
	copyReport.Hash = ""
	return canonical.Hash(copyReport)
}

func sortedEntries(entries []Entry) []Entry {
	sorted := append([]Entry(nil), entries...)
	sort.Slice(sorted, func(i, j int) bool {
		left := sorted[i]
		right := sorted[j]
		if normalizeDomain(left.Domain) != normalizeDomain(right.Domain) {
			return normalizeDomain(left.Domain) < normalizeDomain(right.Domain)
		}
		return left.AssetID < right.AssetID
	})
	return sorted
}

func sortedEntryReports(entries []EntryReport) []EntryReport {
	sorted := append([]EntryReport(nil), entries...)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].Domain != sorted[j].Domain {
			return sorted[i].Domain < sorted[j].Domain
		}
		return sorted[i].AssetID < sorted[j].AssetID
	})
	return sorted
}

func sortedEvidence(evidence map[string]ArtifactEvidence) []ArtifactEvidence {
	var paths []string
	for path := range evidence {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	var sorted []ArtifactEvidence
	for _, path := range paths {
		sorted = append(sorted, evidence[path])
	}
	return sorted
}

func sortedStrings(values []string) []string {
	sorted := append([]string(nil), values...)
	sort.Strings(sorted)
	return sorted
}

func sortedNonEmpty(values []string) []string {
	var out []string
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			out = append(out, value)
		}
	}
	sort.Strings(out)
	return out
}

func sortCounterexamples(counterexamples []Counterexample) {
	sort.Slice(counterexamples, func(i, j int) bool {
		if counterexamples[i].Kind != counterexamples[j].Kind {
			return counterexamples[i].Kind < counterexamples[j].Kind
		}
		if counterexamples[i].Subject != counterexamples[j].Subject {
			return counterexamples[i].Subject < counterexamples[j].Subject
		}
		return counterexamples[i].ID < counterexamples[j].ID
	})
}

func allowedDomain(domain string) bool {
	for _, required := range requiredGovernanceDomains {
		if domain == required {
			return true
		}
	}
	return false
}

func normalizeDomain(domain string) string {
	domain = strings.ToLower(strings.TrimSpace(domain))
	domain = strings.ReplaceAll(domain, "-", "_")
	domain = strings.ReplaceAll(domain, " ", "_")
	return domain
}

func normalizeIdentity(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func mustParseTime(value string) time.Time {
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		panic(err)
	}
	return parsed
}

func round4(value float64) float64 {
	return math.Round(value*10000) / 10000
}

func isFinite(value float64) bool {
	return !math.IsInf(value, 0) && !math.IsNaN(value)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func escapePipes(value string) string {
	return strings.ReplaceAll(value, "|", "\\|")
}

func safeID(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var b strings.Builder
	lastDash := false
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
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
