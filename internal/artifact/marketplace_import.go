package artifact

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/thehalleyyoung/patchline/internal/canonical"
	"github.com/thehalleyyoung/patchline/internal/evidencemarketplace"
)

const MarketplaceBenchmarkImportVersion = "patchline.marketplace-benchmark-import/v1"

type MarketplaceBenchmarkImportOptions struct {
	RegistryPath string
	OutDir       string
	DatasetID    string
}

type MarketplaceBenchmarkImportReport struct {
	Version               string                               `json:"version"`
	OK                    bool                                 `json:"ok"`
	DatasetID             string                               `json:"dataset_id"`
	Registry              string                               `json:"registry"`
	RegistryHash          string                               `json:"registry_hash"`
	MarketplaceReportHash string                               `json:"marketplace_report_hash"`
	Manifest              string                               `json:"manifest"`
	Summary               MarketplaceBenchmarkImportSummary    `json:"summary"`
	Cases                 []MarketplaceBenchmarkImportedCase   `json:"cases"`
	Rejected              []MarketplaceBenchmarkRejectedImport `json:"rejected,omitempty"`
	Hash                  string                               `json:"hash"`
	Markdown              string                               `json:"markdown,omitempty"`
}

type MarketplaceBenchmarkImportSummary struct {
	Published          int `json:"published"`
	Imported           int `json:"imported"`
	Rejected           int `json:"rejected"`
	ArtifactsVerified  int `json:"artifacts_verified"`
	LabelDisagreements int `json:"label_disagreements"`
}

type MarketplaceBenchmarkImportedCase struct {
	ExampleID               string                     `json:"example_id"`
	CaseID                  string                     `json:"case_id"`
	ClaimedHazardClass      string                     `json:"claimed_hazard_class"`
	ArtifactHazardClass     string                     `json:"artifact_hazard_class,omitempty"`
	DerivedHazardClass      string                     `json:"derived_hazard_class"`
	SubmitterLabelsTrusted  bool                       `json:"submitter_labels_trusted"`
	LabelSource             string                     `json:"label_source"`
	CueID                   string                     `json:"cue_id"`
	Source                  evidencemarketplace.Source `json:"source"`
	CertificateSubjectHash  string                     `json:"certificate_subject_hash"`
	EvidenceHash            string                     `json:"evidence_hash"`
	MarketplaceArtifact     string                     `json:"marketplace_artifact"`
	MarketplaceArtifactHash string                     `json:"marketplace_artifact_hash"`
	Fixture                 string                     `json:"fixture"`
	FixtureSHA256           string                     `json:"fixture_sha256"`
	GroundTruth             string                     `json:"ground_truth"`
	GroundTruthSHA256       string                     `json:"ground_truth_sha256"`
}

type MarketplaceBenchmarkRejectedImport struct {
	ID      string   `json:"id"`
	Reasons []string `json:"reasons"`
}

type marketplaceHazardArtifact struct {
	Version     string                      `json:"version"`
	Summary     string                      `json:"summary"`
	Finding     string                      `json:"finding"`
	HazardClass string                      `json:"hazard_class"`
	Evidence    []marketplaceHazardEvidence `json:"evidence"`
}

type marketplaceHazardEvidence struct {
	Path      string `json:"path"`
	LineRange string `json:"line_range"`
	Snippet   string `json:"snippet"`
}

type marketplaceImportCue struct {
	ID                 string
	DerivedHazardClass string
	Risk               string
	SQL                string
	Match              func(string) bool
}

var marketplaceImportCues = []marketplaceImportCue{
	{
		ID:                 "rails-find-each-update-backfill",
		DerivedHazardClass: "broad-backfill-without-guard",
		Risk:               "high",
		SQL:                "UPDATE marketplace_accounts SET repaired = true;\n",
		Match: func(text string) bool {
			return strings.Contains(text, "backfill") &&
				strings.Contains(text, "find_each") &&
				(strings.Contains(text, "update!") || strings.Contains(text, "update("))
		},
	},
	{
		ID:                 "orm-nullability-tightening-before-backfill-proof",
		DerivedHazardClass: "constraint-tightening-before-complete-backfill",
		Risk:               "high",
		SQL:                "UPDATE marketplace_accounts SET required_code = 'pending';\n",
		Match: func(text string) bool {
			compact := strings.ReplaceAll(text, " ", "")
			return strings.Contains(text, "backfill") &&
				(strings.Contains(compact, "null=false") || strings.Contains(text, "set not null")) &&
				(strings.Contains(text, "__isnull=true") || strings.Contains(text, "is null"))
		},
	},
}

func ImportMarketplaceBenchmark(options MarketplaceBenchmarkImportOptions) (MarketplaceBenchmarkImportReport, error) {
	if strings.TrimSpace(options.RegistryPath) == "" {
		return MarketplaceBenchmarkImportReport{}, fmt.Errorf("registry path is required")
	}
	if strings.TrimSpace(options.OutDir) == "" {
		return MarketplaceBenchmarkImportReport{}, fmt.Errorf("out dir is required")
	}
	datasetID := strings.TrimSpace(options.DatasetID)
	if datasetID == "" {
		datasetID = "patchline-marketplace-import-v1"
	}
	registryPath, err := filepath.Abs(options.RegistryPath)
	if err != nil {
		return MarketplaceBenchmarkImportReport{}, err
	}
	outDir, err := filepath.Abs(options.OutDir)
	if err != nil {
		return MarketplaceBenchmarkImportReport{}, err
	}
	registryRoot := filepath.Dir(registryPath)
	published, err := evidencemarketplace.PublishRegistryFile(registryPath)
	if err != nil {
		return MarketplaceBenchmarkImportReport{}, err
	}
	if err := os.MkdirAll(filepath.Join(outDir, "manifests"), 0o755); err != nil {
		return MarketplaceBenchmarkImportReport{}, err
	}
	if err := os.MkdirAll(filepath.Join(outDir, "fixtures", "marketplace"), 0o755); err != nil {
		return MarketplaceBenchmarkImportReport{}, err
	}
	if err := os.MkdirAll(filepath.Join(outDir, "ground_truth", "marketplace"), 0o755); err != nil {
		return MarketplaceBenchmarkImportReport{}, err
	}

	report := MarketplaceBenchmarkImportReport{
		Version:               MarketplaceBenchmarkImportVersion,
		DatasetID:             datasetID,
		Registry:              filepath.ToSlash(options.RegistryPath),
		RegistryHash:          published.RegistryHash,
		MarketplaceReportHash: published.Hash,
		Summary: MarketplaceBenchmarkImportSummary{
			Published:         published.Summary.Published,
			ArtifactsVerified: published.Summary.ArtifactsVerified,
		},
	}
	for _, rejected := range published.Rejected {
		report.Rejected = append(report.Rejected, MarketplaceBenchmarkRejectedImport{ID: rejected.ID, Reasons: rejected.Reasons})
	}

	manifest := Manifest{
		Version:     "patchline.artifact-benchmark/v1",
		DatasetID:   datasetID,
		Description: "Marketplace-derived benchmark cases with labels derived from redacted evidence cues, not submitter hazard labels.",
	}
	seenCaseIDs := map[string]bool{}
	for _, example := range published.Examples {
		caseID := "marketplace-" + slugForBenchmarkCase(example.ID)
		if seenCaseIDs[caseID] {
			report.Rejected = append(report.Rejected, MarketplaceBenchmarkRejectedImport{ID: example.ID, Reasons: []string{"duplicate imported case_id " + caseID}})
			continue
		}
		seenCaseIDs[caseID] = true
		imported, manifestCase, reasons := importMarketplaceExample(registryRoot, outDir, example)
		if len(reasons) > 0 {
			report.Rejected = append(report.Rejected, MarketplaceBenchmarkRejectedImport{ID: example.ID, Reasons: reasons})
			continue
		}
		report.Cases = append(report.Cases, imported)
		manifest.Cases = append(manifest.Cases, manifestCase)
		if imported.ClaimedHazardClass != imported.DerivedHazardClass || (imported.ArtifactHazardClass != "" && imported.ArtifactHazardClass != imported.DerivedHazardClass) {
			report.Summary.LabelDisagreements++
		}
	}
	sort.Slice(report.Cases, func(i, j int) bool { return report.Cases[i].CaseID < report.Cases[j].CaseID })
	sort.Slice(manifest.Cases, func(i, j int) bool { return manifest.Cases[i].CaseID < manifest.Cases[j].CaseID })
	sort.Slice(report.Rejected, func(i, j int) bool {
		if report.Rejected[i].ID != report.Rejected[j].ID {
			return report.Rejected[i].ID < report.Rejected[j].ID
		}
		return strings.Join(report.Rejected[i].Reasons, "\n") < strings.Join(report.Rejected[j].Reasons, "\n")
	})

	manifestPath := filepath.Join(outDir, "manifests", "marketplace-import.json")
	if err := writeArtifactJSON(manifestPath, manifest); err != nil {
		return MarketplaceBenchmarkImportReport{}, err
	}
	report.Manifest = filepath.ToSlash(filepath.Join("manifests", "marketplace-import.json"))
	report.Summary.Imported = len(report.Cases)
	report.Summary.Rejected = len(report.Rejected)
	report.OK = report.Summary.Imported > 0 && report.Summary.Rejected == 0
	report.Hash = marketplaceBenchmarkImportHash(report)
	report.Markdown = renderMarketplaceBenchmarkImportMarkdown(report)
	if err := WriteMarketplaceBenchmarkImportReport(outDir, report); err != nil {
		return MarketplaceBenchmarkImportReport{}, err
	}
	return report, nil
}

func importMarketplaceExample(registryRoot, outDir string, example evidencemarketplace.PublishedExample) (MarketplaceBenchmarkImportedCase, ManifestCase, []string) {
	var reasons []string
	artifact, artifactReasons := marketplaceHazardArtifactSummary(example.Artifacts)
	reasons = append(reasons, artifactReasons...)
	if len(reasons) > 0 {
		sort.Strings(reasons)
		return MarketplaceBenchmarkImportedCase{}, ManifestCase{}, reasons
	}
	path, err := marketplaceArtifactPath(registryRoot, artifact.Path)
	if err != nil {
		return MarketplaceBenchmarkImportedCase{}, ManifestCase{}, []string{"artifact " + artifact.Path + ": " + err.Error()}
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return MarketplaceBenchmarkImportedCase{}, ManifestCase{}, []string{"artifact " + artifact.Path + ": " + err.Error()}
	}
	actualArtifactHash := "sha256:" + sha256Hex(content)
	if actualArtifactHash != artifact.SHA256 {
		return MarketplaceBenchmarkImportedCase{}, ManifestCase{}, []string{"artifact " + artifact.Path + " sha256 mismatch during import"}
	}
	var hazard marketplaceHazardArtifact
	if err := json.Unmarshal(content, &hazard); err != nil {
		return MarketplaceBenchmarkImportedCase{}, ManifestCase{}, []string{"artifact " + artifact.Path + " is not a redacted hazard JSON object: " + err.Error()}
	}
	cue, cueReasons := deriveMarketplaceImportCue(hazard)
	if len(cueReasons) > 0 {
		return MarketplaceBenchmarkImportedCase{}, ManifestCase{}, cueReasons
	}

	caseID := "marketplace-" + slugForBenchmarkCase(example.ID)
	fixtureRel := filepath.ToSlash(filepath.Join("fixtures", "marketplace", caseID+".sql"))
	groundTruthRel := filepath.ToSlash(filepath.Join("ground_truth", "marketplace", caseID+".json"))
	fixturePath := filepath.Join(outDir, filepath.FromSlash(fixtureRel))
	groundTruthPath := filepath.Join(outDir, filepath.FromSlash(groundTruthRel))
	if err := os.WriteFile(fixturePath, []byte(cue.SQL), 0o644); err != nil {
		return MarketplaceBenchmarkImportedCase{}, ManifestCase{}, []string{err.Error()}
	}
	gt := marketplaceImportGroundTruth(caseID, fixtureRel, example, artifact, actualArtifactHash, cue)
	if err := writeArtifactJSON(groundTruthPath, gt); err != nil {
		return MarketplaceBenchmarkImportedCase{}, ManifestCase{}, []string{err.Error()}
	}
	fixtureBytes, err := os.ReadFile(fixturePath)
	if err != nil {
		return MarketplaceBenchmarkImportedCase{}, ManifestCase{}, []string{err.Error()}
	}
	groundTruthBytes, err := os.ReadFile(groundTruthPath)
	if err != nil {
		return MarketplaceBenchmarkImportedCase{}, ManifestCase{}, []string{err.Error()}
	}
	imported := MarketplaceBenchmarkImportedCase{
		ExampleID:               example.ID,
		CaseID:                  caseID,
		ClaimedHazardClass:      example.HazardClass,
		ArtifactHazardClass:     strings.TrimSpace(hazard.HazardClass),
		DerivedHazardClass:      cue.DerivedHazardClass,
		SubmitterLabelsTrusted:  false,
		LabelSource:             "artifact-evidence-cue",
		CueID:                   cue.ID,
		Source:                  example.Source,
		CertificateSubjectHash:  example.CertificateSubjectHash,
		EvidenceHash:            example.EvidenceHash,
		MarketplaceArtifact:     artifact.Path,
		MarketplaceArtifactHash: actualArtifactHash,
		Fixture:                 fixtureRel,
		FixtureSHA256:           "sha256:" + sha256Hex(fixtureBytes),
		GroundTruth:             groundTruthRel,
		GroundTruthSHA256:       "sha256:" + sha256Hex(groundTruthBytes),
	}
	manifestCase := ManifestCase{
		CaseID:      caseID,
		CaseType:    "migration",
		AvailableAt: "pre_deploy",
		Fixture:     "../" + fixtureRel,
		GroundTruth: "../" + groundTruthRel,
	}
	return imported, manifestCase, nil
}

func marketplaceHazardArtifactSummary(artifacts []evidencemarketplace.ArtifactSummary) (evidencemarketplace.ArtifactSummary, []string) {
	var matches []evidencemarketplace.ArtifactSummary
	for _, artifact := range artifacts {
		if artifact.Role == "redacted-hazard-example" {
			matches = append(matches, artifact)
		}
	}
	switch len(matches) {
	case 0:
		return evidencemarketplace.ArtifactSummary{}, []string{"missing redacted-hazard-example artifact"}
	case 1:
		return matches[0], nil
	default:
		return evidencemarketplace.ArtifactSummary{}, []string{"multiple redacted-hazard-example artifacts are ambiguous"}
	}
}

func deriveMarketplaceImportCue(hazard marketplaceHazardArtifact) (marketplaceImportCue, []string) {
	text := strings.ToLower(marketplaceHazardCueText(hazard))
	var matches []marketplaceImportCue
	for _, cue := range marketplaceImportCues {
		if cue.Match(text) {
			matches = append(matches, cue)
		}
	}
	switch len(matches) {
	case 0:
		return marketplaceImportCue{}, []string{"unsupported redacted evidence cue; no benchmark label imported"}
	case 1:
		return matches[0], nil
	default:
		ids := make([]string, 0, len(matches))
		for _, match := range matches {
			ids = append(ids, match.ID)
		}
		sort.Strings(ids)
		return marketplaceImportCue{}, []string{"ambiguous redacted evidence cues: " + strings.Join(ids, ",")}
	}
}

func marketplaceHazardCueText(hazard marketplaceHazardArtifact) string {
	var b strings.Builder
	for _, evidence := range hazard.Evidence {
		fmt.Fprintf(&b, "%s\n%s\n%s\n", evidence.Path, evidence.LineRange, evidence.Snippet)
	}
	return b.String()
}

func marketplaceImportGroundTruth(caseID, fixtureRel string, example evidencemarketplace.PublishedExample, artifact evidencemarketplace.ArtifactSummary, artifactHash string, cue marketplaceImportCue) GroundTruthCase {
	return GroundTruthCase{
		CaseID:   caseID,
		CaseType: "migration",
		Phase:    "pre_deploy",
		Labels: GroundTruthLabel{
			ExpectedResult: ResultFlag,
			Risk:           cue.Risk,
		},
		Evidence: []Evidence{
			{
				Kind:      "marketplace_artifact",
				Locator:   artifact.Path,
				Rationale: "Hash-verified redacted-hazard-example artifact from marketplace example " + example.ID + ".",
			},
			{
				Kind:      "sha256",
				Locator:   artifactHash,
				Rationale: "The importer rehashes the marketplace artifact bytes before deriving any benchmark case.",
			},
			{
				Kind:      "certificate",
				Locator:   example.CertificateSubjectHash,
				Rationale: "The marketplace certificate binds source, artifact, license, redaction, obligation, and reproduction metadata.",
			},
			{
				Kind:      "independent_cue",
				Locator:   cue.ID,
				Rationale: "The benchmark label is derived from redacted evidence snippets; registry and artifact hazard_class labels are recorded but not trusted.",
			},
			{
				Kind:      "synthetic_fixture",
				Locator:   fixtureRel,
				Rationale: "Deterministic SQL fixture exercises Patchline's existing migration analyzer for the derived hazard.",
			},
		},
		AllowedInputs:  []string{"migration_text", "schema", "policy", "prior_archive"},
		ExcludedInputs: []string{"postmortem_text"},
	}
}

func marketplaceArtifactPath(root, rel string) (string, error) {
	if filepath.IsAbs(rel) {
		return "", fmt.Errorf("absolute paths are not allowed")
	}
	clean := filepath.Clean(filepath.FromSlash(rel))
	if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("path escapes registry root")
	}
	joined := filepath.Join(root, clean)
	realRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", err
	}
	realPath, err := filepath.EvalSymlinks(joined)
	if err != nil {
		return "", err
	}
	relative, err := filepath.Rel(realRoot, realPath)
	if err != nil {
		return "", err
	}
	if relative == ".." || strings.HasPrefix(relative, ".."+string(os.PathSeparator)) || filepath.IsAbs(relative) {
		return "", fmt.Errorf("path escapes registry root")
	}
	return realPath, nil
}

func WriteMarketplaceBenchmarkImportReport(outDir string, report MarketplaceBenchmarkImportReport) error {
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}
	if err := writeArtifactJSON(filepath.Join(outDir, "marketplace-import.json"), report); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(outDir, "marketplace-import.md"), []byte(report.Markdown), 0o644)
}

func writeArtifactJSON(path string, value any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()
	return canonical.WriteJSON(file, value)
}

func marketplaceBenchmarkImportHash(report MarketplaceBenchmarkImportReport) string {
	copy := report
	copy.Hash = ""
	copy.Markdown = ""
	return "sha256:" + canonical.Hash(copy)
}

func renderMarketplaceBenchmarkImportMarkdown(report MarketplaceBenchmarkImportReport) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Marketplace benchmark import\n\n")
	fmt.Fprintf(&b, "Patchline imported `%d` marketplace examples into runnable benchmark cases while recording, but not trusting, submitter hazard labels.\n\n", report.Summary.Imported)
	fmt.Fprintf(&b, "| Metric | Count |\n| --- | ---: |\n")
	fmt.Fprintf(&b, "| Published examples | %d |\n", report.Summary.Published)
	fmt.Fprintf(&b, "| Imported cases | %d |\n", report.Summary.Imported)
	fmt.Fprintf(&b, "| Rejected imports | %d |\n", report.Summary.Rejected)
	fmt.Fprintf(&b, "| Label disagreements preserved | %d |\n", report.Summary.LabelDisagreements)
	fmt.Fprintf(&b, "\n| Case | Claimed label | Derived label | Cue | Trusted submitter labels |\n")
	fmt.Fprintf(&b, "| --- | --- | --- | --- | ---: |\n")
	for _, c := range report.Cases {
		fmt.Fprintf(&b, "| `%s` | `%s` | `%s` | `%s` | `%t` |\n", c.CaseID, c.ClaimedHazardClass, c.DerivedHazardClass, c.CueID, c.SubmitterLabelsTrusted)
	}
	if len(report.Rejected) > 0 {
		fmt.Fprintf(&b, "\n## Rejected imports\n\n")
		fmt.Fprintf(&b, "| ID | Reasons |\n| --- | --- |\n")
		for _, rejected := range report.Rejected {
			fmt.Fprintf(&b, "| `%s` | %s |\n", rejected.ID, strings.Join(rejected.Reasons, "; "))
		}
	}
	return b.String()
}

func slugForBenchmarkCase(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var b strings.Builder
	lastDash := false
	for _, r := range value {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash {
			b.WriteByte('-')
			lastDash = true
		}
	}
	slug := strings.Trim(b.String(), "-")
	if slug == "" {
		return "example"
	}
	return slug
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
