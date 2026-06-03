package education

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/thehalleyyoung/patchline/internal/canonical"
)

const SkillsTaxonomySpecVersion = "patchline.skills-taxonomy/v1"
const SkillsTaxonomyReportVersion = "patchline.skills-taxonomy-report/v1"

type SkillsTaxonomySpec struct {
	Version       string                 `json:"version"`
	Name          string                 `json:"name"`
	Claim         string                 `json:"claim,omitempty"`
	Criteria      SkillsTaxonomyCriteria `json:"criteria"`
	HazardClasses []SkillHazardClass     `json:"hazard_classes"`
}

type SkillsTaxonomyCriteria struct {
	RequiredAudiences             []string `json:"required_audiences"`
	MinHazardClasses              int      `json:"min_hazard_classes"`
	MinConceptsPerHazard          int      `json:"min_concepts_per_hazard"`
	MinPrerequisitesPerConcept    int      `json:"min_prerequisites_per_concept"`
	MinEvidenceArtifactsPerHazard int      `json:"min_evidence_artifacts_per_hazard"`
	RequireGate                   bool     `json:"require_gate"`
	RequireReproducibleCommand    bool     `json:"require_reproducible_command"`
	RequireNegativeControl        bool     `json:"require_negative_control"`
	RequireAssessmentPrompt       bool     `json:"require_assessment_prompt"`
	RequireCrosswalk              bool     `json:"require_crosswalk"`
}

type SkillHazardClass struct {
	HazardClass                   string            `json:"hazard_class"`
	Title                         string            `json:"title"`
	SeverityBand                  string            `json:"severity_band"`
	ReviewerAudiences             []string          `json:"reviewer_audiences"`
	Concepts                      []ReviewerConcept `json:"concepts"`
	Gates                         []TaxonomyGate    `json:"gates"`
	RelatedTutorials              []string          `json:"related_tutorials"`
	RelatedCertificationScenarios []string          `json:"related_certification_scenarios"`
}

type ReviewerConcept struct {
	ID               string   `json:"id"`
	Title            string   `json:"title"`
	Description      string   `json:"description"`
	Prerequisites    []string `json:"prerequisites"`
	AssessmentPrompt string   `json:"assessment_prompt"`
	EvidencePaths    []string `json:"evidence_paths"`
}

type TaxonomyGate struct {
	Name             string                    `json:"name"`
	Commands         []string                  `json:"commands"`
	EvidencePaths    []string                  `json:"evidence_paths"`
	NegativeControls []TaxonomyNegativeControl `json:"negative_controls"`
}

type TaxonomyNegativeControl struct {
	ID                     string `json:"id"`
	Mutation               string `json:"mutation"`
	ExpectedCounterexample string `json:"expected_counterexample"`
}

type SkillsTaxonomyReport struct {
	Version         string                         `json:"version"`
	Name            string                         `json:"name"`
	OK              bool                           `json:"ok"`
	Criteria        SkillsTaxonomyCriteria         `json:"criteria"`
	Summary         SkillsTaxonomySummary          `json:"summary"`
	HazardClasses   []SkillHazardClassReport       `json:"hazard_classes"`
	Counterexamples []SkillsTaxonomyCounterexample `json:"counterexamples,omitempty"`
	Hash            string                         `json:"hash"`
}

type SkillsTaxonomySummary struct {
	HazardClasses     int `json:"hazard_classes"`
	Concepts          int `json:"concepts"`
	ReviewerAudiences int `json:"reviewer_audiences"`
	GateBackedHazards int `json:"gate_backed_hazards"`
	EvidenceArtifacts int `json:"evidence_artifacts"`
	NegativeControls  int `json:"negative_controls"`
	CrosswalkEntries  int `json:"crosswalk_entries"`
	Counterexamples   int `json:"counterexamples"`
}

type SkillHazardClassReport struct {
	HazardClass           string             `json:"hazard_class"`
	Title                 string             `json:"title"`
	SeverityBand          string             `json:"severity_band"`
	ReviewerAudiences     []string           `json:"reviewer_audiences"`
	Concepts              int                `json:"concepts"`
	ConceptIDs            []string           `json:"concept_ids"`
	Gates                 []string           `json:"gates"`
	GateBacked            bool               `json:"gate_backed"`
	ReproducibleCommandOK bool               `json:"reproducible_command_ok"`
	Evidence              []ArtifactEvidence `json:"evidence"`
	NegativeControls      int                `json:"negative_controls"`
	CrosswalkEntries      int                `json:"crosswalk_entries"`
}

type SkillsTaxonomyCounterexample struct {
	ID      string   `json:"id"`
	Kind    string   `json:"kind"`
	Subject string   `json:"subject,omitempty"`
	Message string   `json:"message"`
	Witness []string `json:"witness,omitempty"`
}

func ReadSkillsTaxonomySpec(reader io.Reader) (SkillsTaxonomySpec, error) {
	decoder := json.NewDecoder(reader)
	decoder.DisallowUnknownFields()
	var spec SkillsTaxonomySpec
	if err := decoder.Decode(&spec); err != nil {
		return SkillsTaxonomySpec{}, err
	}
	if spec.Version != SkillsTaxonomySpecVersion {
		return SkillsTaxonomySpec{}, fmt.Errorf("skills taxonomy spec version must be %s", SkillsTaxonomySpecVersion)
	}
	return spec, nil
}

func BuildSkillsTaxonomyReport(spec SkillsTaxonomySpec, root string) (SkillsTaxonomyReport, error) {
	if err := validateSkillsTaxonomySpec(spec); err != nil {
		return SkillsTaxonomyReport{}, err
	}
	if root == "" {
		root = "."
	}
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return SkillsTaxonomyReport{}, err
	}
	criteria := normalizeSkillsTaxonomyCriteria(spec.Criteria)
	report := SkillsTaxonomyReport{
		Version:  SkillsTaxonomyReportVersion,
		Name:     spec.Name,
		OK:       true,
		Criteria: criteria,
	}
	if len(spec.HazardClasses) < criteria.MinHazardClasses {
		report.Counterexamples = append(report.Counterexamples, SkillsTaxonomyCounterexample{
			ID:      "criteria.min_hazard_classes",
			Kind:    "insufficient_hazard_classes",
			Message: fmt.Sprintf("hazard classes %d below required %d", len(spec.HazardClasses), criteria.MinHazardClasses),
		})
	}
	audiences := map[string]struct{}{}
	for _, hazard := range sortedSkillHazardClasses(spec.HazardClasses) {
		hazardReport, counterexamples := buildSkillHazardClassReport(rootAbs, hazard, criteria)
		report.HazardClasses = append(report.HazardClasses, hazardReport)
		report.Counterexamples = append(report.Counterexamples, counterexamples...)
		report.Summary.Concepts += hazardReport.Concepts
		report.Summary.EvidenceArtifacts += len(hazardReport.Evidence)
		report.Summary.NegativeControls += hazardReport.NegativeControls
		report.Summary.CrosswalkEntries += hazardReport.CrosswalkEntries
		if hazardReport.GateBacked {
			report.Summary.GateBackedHazards++
		}
		for _, audience := range hazardReport.ReviewerAudiences {
			audiences[audience] = struct{}{}
		}
	}
	for _, audience := range criteria.RequiredAudiences {
		if _, ok := audiences[audience]; !ok {
			report.Counterexamples = append(report.Counterexamples, SkillsTaxonomyCounterexample{
				ID:      "audience." + stableID(audience) + ".missing",
				Kind:    "missing_required_audience",
				Subject: audience,
				Message: "required reviewer audience is not mapped to any hazard class",
			})
		}
	}
	sortSkillsTaxonomyCounterexamples(report.Counterexamples)
	report.Summary.HazardClasses = len(report.HazardClasses)
	report.Summary.ReviewerAudiences = len(audiences)
	report.Summary.Counterexamples = len(report.Counterexamples)
	report.OK = len(report.Counterexamples) == 0
	report.Hash = skillsTaxonomyReportHash(report)
	return report, nil
}

func WriteSkillsTaxonomyArtifacts(outDir string, report SkillsTaxonomyReport) error {
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}
	file, err := os.Create(filepath.Join(outDir, "skills-taxonomy.json"))
	if err != nil {
		return err
	}
	if err := canonical.WriteJSON(file, report); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(outDir, "skills-taxonomy.md"), []byte(RenderSkillsTaxonomyMarkdown(report)), 0o644)
}

func RenderSkillsTaxonomyMarkdown(report SkillsTaxonomyReport) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Reviewer skills taxonomy\n\n")
	fmt.Fprintf(&b, "Patchline publishes a gate-backed skills taxonomy that maps each declared hazard class to the reviewer concepts, prerequisites, assessment prompts, evidence hashes, and negative controls needed for certification, labs, tutorials, and apprenticeship review.\n\n")
	fmt.Fprintf(&b, "## Summary\n\n")
	fmt.Fprintf(&b, "| Metric | Value |\n| --- | ---: |\n")
	fmt.Fprintf(&b, "| OK | `%t` |\n", report.OK)
	fmt.Fprintf(&b, "| Hazard classes | %d |\n", report.Summary.HazardClasses)
	fmt.Fprintf(&b, "| Concepts | %d |\n", report.Summary.Concepts)
	fmt.Fprintf(&b, "| Reviewer audiences | %d |\n", report.Summary.ReviewerAudiences)
	fmt.Fprintf(&b, "| Gate-backed hazards | %d |\n", report.Summary.GateBackedHazards)
	fmt.Fprintf(&b, "| Evidence artifacts | %d |\n", report.Summary.EvidenceArtifacts)
	fmt.Fprintf(&b, "| Negative controls | %d |\n", report.Summary.NegativeControls)
	fmt.Fprintf(&b, "| Crosswalk entries | %d |\n", report.Summary.CrosswalkEntries)
	fmt.Fprintf(&b, "| Counterexamples | %d |\n\n", report.Summary.Counterexamples)

	fmt.Fprintf(&b, "## Hazard-to-skill map\n\n")
	fmt.Fprintf(&b, "| Hazard class | Audiences | Concepts | Gates | Evidence hashes | Crosswalk entries |\n| --- | --- | ---: | --- | --- | ---: |\n")
	for _, hazard := range report.HazardClasses {
		hashes := make([]string, 0, len(hazard.Evidence))
		for _, evidence := range hazard.Evidence {
			hash := evidence.SHA256
			if len(hash) > 16 {
				hash = hash[:16]
			}
			hashes = append(hashes, evidence.Path+":"+hash)
		}
		fmt.Fprintf(&b, "| `%s` | `%s` | %d | `%s` | %s | %d |\n",
			escapeTable(hazard.HazardClass),
			escapeTable(strings.Join(hazard.ReviewerAudiences, ", ")),
			hazard.Concepts,
			escapeTable(strings.Join(hazard.Gates, ", ")),
			escapeTable(strings.Join(hashes, "; ")),
			hazard.CrosswalkEntries,
		)
	}
	if len(report.Counterexamples) > 0 {
		fmt.Fprintf(&b, "\n## Counterexamples\n\n")
		fmt.Fprintf(&b, "| ID | Kind | Subject | Message |\n| --- | --- | --- | --- |\n")
		for _, counterexample := range report.Counterexamples {
			fmt.Fprintf(&b, "| `%s` | `%s` | `%s` | %s |\n",
				escapeTable(counterexample.ID),
				escapeTable(counterexample.Kind),
				escapeTable(firstNonEmpty(counterexample.Subject, "-")),
				escapeTable(counterexample.Message),
			)
		}
	}
	return b.String()
}

func buildSkillHazardClassReport(root string, hazard SkillHazardClass, criteria SkillsTaxonomyCriteria) (SkillHazardClassReport, []SkillsTaxonomyCounterexample) {
	subject := hazard.HazardClass
	evidence, counterexamples := collectSkillsTaxonomyArtifacts(root, skillsTaxonomyEvidencePaths(hazard), subject)
	audiences := normalizedStrings(hazard.ReviewerAudiences)
	if len(audiences) == 0 {
		counterexamples = append(counterexamples, SkillsTaxonomyCounterexample{
			ID:      "hazard." + stableID(subject, "audience") + ".missing",
			Kind:    "missing_reviewer_audience",
			Subject: subject,
			Message: "hazard class does not name any reviewer audience",
		})
	}

	conceptIDs := make([]string, 0, len(hazard.Concepts))
	if len(hazard.Concepts) < criteria.MinConceptsPerHazard {
		counterexamples = append(counterexamples, SkillsTaxonomyCounterexample{
			ID:      "hazard." + stableID(subject, "concepts") + ".insufficient",
			Kind:    "insufficient_concepts",
			Subject: subject,
			Message: fmt.Sprintf("hazard maps to %d concepts below required %d", len(hazard.Concepts), criteria.MinConceptsPerHazard),
		})
	}
	for _, concept := range sortedReviewerConcepts(hazard.Concepts) {
		conceptIDs = append(conceptIDs, concept.ID)
		if len(concept.EvidencePaths) == 0 {
			counterexamples = append(counterexamples, SkillsTaxonomyCounterexample{
				ID:      "concept." + stableID(subject, concept.ID, "evidence") + ".missing",
				Kind:    "missing_concept_evidence",
				Subject: subject + "/" + concept.ID,
				Message: "reviewer concept does not cite a readable evidence path",
			})
		}
		if len(normalizedStrings(concept.Prerequisites)) < criteria.MinPrerequisitesPerConcept {
			counterexamples = append(counterexamples, SkillsTaxonomyCounterexample{
				ID:      "concept." + stableID(subject, concept.ID, "prerequisites") + ".insufficient",
				Kind:    "insufficient_prerequisites",
				Subject: subject + "/" + concept.ID,
				Message: fmt.Sprintf("concept prerequisites below required %d", criteria.MinPrerequisitesPerConcept),
			})
		}
		if criteria.RequireAssessmentPrompt && strings.TrimSpace(concept.AssessmentPrompt) == "" {
			counterexamples = append(counterexamples, SkillsTaxonomyCounterexample{
				ID:      "concept." + stableID(subject, concept.ID, "assessment") + ".missing",
				Kind:    "missing_assessment_prompt",
				Subject: subject + "/" + concept.ID,
				Message: "reviewer concept does not include an assessment prompt",
			})
		}
	}

	gateNames := make([]string, 0, len(hazard.Gates))
	gateBacked := len(hazard.Gates) > 0
	reproducible := len(hazard.Gates) > 0
	negativeControls := 0
	if criteria.RequireGate && len(hazard.Gates) == 0 {
		counterexamples = append(counterexamples, SkillsTaxonomyCounterexample{
			ID:      "hazard." + stableID(subject, "gate") + ".missing",
			Kind:    "missing_gate",
			Subject: subject,
			Message: "hazard class does not name a gate-backed proof",
		})
	}
	for _, gate := range sortedTaxonomyGates(hazard.Gates) {
		gateNames = append(gateNames, gate.Name)
		if !gateExists(root, gate.Name) {
			gateBacked = false
			if criteria.RequireGate {
				counterexamples = append(counterexamples, SkillsTaxonomyCounterexample{
					ID:      "gate." + stableID(subject, gate.Name, "exists") + ".missing",
					Kind:    "missing_gate",
					Subject: subject,
					Message: "taxonomy gate is not present as a Make target backed by a script",
					Witness: []string{gate.Name},
				})
			}
		}
		if !containsCommand(gate.Commands, requiredGateCommand(gate.Name)) {
			reproducible = false
			if criteria.RequireReproducibleCommand {
				counterexamples = append(counterexamples, SkillsTaxonomyCounterexample{
					ID:      "gate." + stableID(subject, gate.Name, "command") + ".missing",
					Kind:    "missing_reproducible_command",
					Subject: subject,
					Message: "taxonomy gate does not include the exact reproducing make command",
					Witness: []string{requiredGateCommand(gate.Name)},
				})
			}
		}
		negativeControls += len(gate.NegativeControls)
		if criteria.RequireNegativeControl && len(gate.NegativeControls) == 0 {
			counterexamples = append(counterexamples, SkillsTaxonomyCounterexample{
				ID:      "gate." + stableID(subject, gate.Name, "negative-control") + ".missing",
				Kind:    "missing_negative_control",
				Subject: subject,
				Message: "taxonomy gate does not describe a failing mutation and expected counterexample",
			})
		}
		for _, control := range gate.NegativeControls {
			if strings.TrimSpace(control.ID) == "" || strings.TrimSpace(control.Mutation) == "" || strings.TrimSpace(control.ExpectedCounterexample) == "" {
				counterexamples = append(counterexamples, SkillsTaxonomyCounterexample{
					ID:      "gate." + stableID(subject, gate.Name, control.ID, "negative-control") + ".incomplete",
					Kind:    "incomplete_negative_control",
					Subject: subject,
					Message: "negative control must include id, mutation, and expected counterexample",
				})
			}
		}
	}
	if len(evidence) < criteria.MinEvidenceArtifactsPerHazard {
		counterexamples = append(counterexamples, SkillsTaxonomyCounterexample{
			ID:      "hazard." + stableID(subject, "evidence") + ".insufficient",
			Kind:    "insufficient_hazard_evidence",
			Subject: subject,
			Message: fmt.Sprintf("hazard has %d readable evidence artifacts below required %d", len(evidence), criteria.MinEvidenceArtifactsPerHazard),
		})
	}
	crosswalkEntries := len(uniqueSorted(hazard.RelatedTutorials)) + len(uniqueSorted(hazard.RelatedCertificationScenarios))
	if criteria.RequireCrosswalk {
		if len(uniqueSorted(hazard.RelatedTutorials)) == 0 {
			counterexamples = append(counterexamples, SkillsTaxonomyCounterexample{
				ID:      "hazard." + stableID(subject, "tutorial-crosswalk") + ".missing",
				Kind:    "missing_tutorial_crosswalk",
				Subject: subject,
				Message: "hazard class is not linked to a role-specific tutorial or teaching module",
			})
		}
		if len(uniqueSorted(hazard.RelatedCertificationScenarios)) == 0 {
			counterexamples = append(counterexamples, SkillsTaxonomyCounterexample{
				ID:      "hazard." + stableID(subject, "certification-crosswalk") + ".missing",
				Kind:    "missing_certification_crosswalk",
				Subject: subject,
				Message: "hazard class is not linked to a certification scenario or graded exercise",
			})
		}
	}
	sortSkillsTaxonomyCounterexamples(counterexamples)
	return SkillHazardClassReport{
		HazardClass:           normalizeToken(hazard.HazardClass),
		Title:                 hazard.Title,
		SeverityBand:          normalizeToken(hazard.SeverityBand),
		ReviewerAudiences:     audiences,
		Concepts:              len(hazard.Concepts),
		ConceptIDs:            sortedStrings(conceptIDs),
		Gates:                 sortedStrings(gateNames),
		GateBacked:            gateBacked,
		ReproducibleCommandOK: reproducible,
		Evidence:              evidence,
		NegativeControls:      negativeControls,
		CrosswalkEntries:      crosswalkEntries,
	}, counterexamples
}

func validateSkillsTaxonomySpec(spec SkillsTaxonomySpec) error {
	if strings.TrimSpace(spec.Name) == "" {
		return fmt.Errorf("skills taxonomy name is required")
	}
	criteria := spec.Criteria
	if criteria.MinHazardClasses <= 0 {
		return fmt.Errorf("criteria.min_hazard_classes must be positive")
	}
	if criteria.MinConceptsPerHazard <= 0 {
		return fmt.Errorf("criteria.min_concepts_per_hazard must be positive")
	}
	if criteria.MinPrerequisitesPerConcept <= 0 {
		return fmt.Errorf("criteria.min_prerequisites_per_concept must be positive")
	}
	if criteria.MinEvidenceArtifactsPerHazard <= 0 {
		return fmt.Errorf("criteria.min_evidence_artifacts_per_hazard must be positive")
	}
	if len(criteria.RequiredAudiences) == 0 {
		return fmt.Errorf("criteria.required_audiences is required")
	}
	seenHazards := map[string]struct{}{}
	for _, hazard := range spec.HazardClasses {
		hazardID := normalizeToken(hazard.HazardClass)
		if hazardID == "" {
			return fmt.Errorf("hazard_class is required")
		}
		if _, ok := seenHazards[hazardID]; ok {
			return fmt.Errorf("duplicate hazard class %q", hazard.HazardClass)
		}
		seenHazards[hazardID] = struct{}{}
		if strings.TrimSpace(hazard.Title) == "" || strings.TrimSpace(hazard.SeverityBand) == "" {
			return fmt.Errorf("hazard class %q requires title and severity_band", hazard.HazardClass)
		}
		seenConcepts := map[string]struct{}{}
		for _, concept := range hazard.Concepts {
			if strings.TrimSpace(concept.ID) == "" {
				return fmt.Errorf("hazard class %q concept id is required", hazard.HazardClass)
			}
			if _, ok := seenConcepts[concept.ID]; ok {
				return fmt.Errorf("hazard class %q contains duplicate concept id %q", hazard.HazardClass, concept.ID)
			}
			seenConcepts[concept.ID] = struct{}{}
			if strings.TrimSpace(concept.Title) == "" || strings.TrimSpace(concept.Description) == "" {
				return fmt.Errorf("hazard class %q concept %q requires title and description", hazard.HazardClass, concept.ID)
			}
			for _, path := range concept.EvidencePaths {
				if err := validateRelativePath(path); err != nil {
					return fmt.Errorf("hazard class %q concept %q evidence path: %w", hazard.HazardClass, concept.ID, err)
				}
			}
		}
		for _, gate := range hazard.Gates {
			if strings.TrimSpace(gate.Name) == "" {
				return fmt.Errorf("hazard class %q gate name is required", hazard.HazardClass)
			}
			for _, path := range gate.EvidencePaths {
				if err := validateRelativePath(path); err != nil {
					return fmt.Errorf("hazard class %q gate %q evidence path: %w", hazard.HazardClass, gate.Name, err)
				}
			}
		}
	}
	return nil
}

func collectSkillsTaxonomyArtifacts(root string, paths []string, subject string) ([]ArtifactEvidence, []SkillsTaxonomyCounterexample) {
	var artifacts []ArtifactEvidence
	var counterexamples []SkillsTaxonomyCounterexample
	for _, relPath := range uniqueSorted(paths) {
		fullPath, err := safeJoin(root, relPath)
		if err != nil {
			counterexamples = append(counterexamples, SkillsTaxonomyCounterexample{
				ID:      "hazard." + stableID(subject, relPath, "evidence-path") + ".invalid",
				Kind:    "invalid_evidence_path",
				Subject: subject,
				Message: err.Error(),
				Witness: []string{relPath},
			})
			continue
		}
		data, err := os.ReadFile(fullPath)
		if err != nil {
			counterexamples = append(counterexamples, SkillsTaxonomyCounterexample{
				ID:      "hazard." + stableID(subject, relPath, "evidence") + ".missing",
				Kind:    "missing_evidence",
				Subject: subject,
				Message: "taxonomy evidence could not be read",
				Witness: []string{relPath},
			})
			continue
		}
		if len(data) == 0 {
			counterexamples = append(counterexamples, SkillsTaxonomyCounterexample{
				ID:      "hazard." + stableID(subject, relPath, "evidence") + ".empty",
				Kind:    "empty_evidence",
				Subject: subject,
				Message: "taxonomy evidence is empty",
				Witness: []string{relPath},
			})
			continue
		}
		sum := sha256.Sum256(data)
		artifacts = append(artifacts, ArtifactEvidence{
			Path:   filepath.ToSlash(filepath.Clean(relPath)),
			SHA256: hex.EncodeToString(sum[:]),
			Bytes:  int64(len(data)),
		})
	}
	return artifacts, counterexamples
}

func skillsTaxonomyEvidencePaths(hazard SkillHazardClass) []string {
	var paths []string
	for _, concept := range hazard.Concepts {
		paths = append(paths, concept.EvidencePaths...)
	}
	for _, gate := range hazard.Gates {
		paths = append(paths, gate.EvidencePaths...)
	}
	return paths
}

func normalizeSkillsTaxonomyCriteria(criteria SkillsTaxonomyCriteria) SkillsTaxonomyCriteria {
	criteria.RequiredAudiences = sortedStrings(normalizedStrings(criteria.RequiredAudiences))
	return criteria
}

func sortedSkillHazardClasses(hazards []SkillHazardClass) []SkillHazardClass {
	out := append([]SkillHazardClass(nil), hazards...)
	sort.SliceStable(out, func(i, j int) bool { return normalizeToken(out[i].HazardClass) < normalizeToken(out[j].HazardClass) })
	return out
}

func sortedReviewerConcepts(concepts []ReviewerConcept) []ReviewerConcept {
	out := append([]ReviewerConcept(nil), concepts...)
	sort.SliceStable(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func sortedTaxonomyGates(gates []TaxonomyGate) []TaxonomyGate {
	out := append([]TaxonomyGate(nil), gates...)
	sort.SliceStable(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func sortSkillsTaxonomyCounterexamples(counterexamples []SkillsTaxonomyCounterexample) {
	sort.SliceStable(counterexamples, func(i, j int) bool { return counterexamples[i].ID < counterexamples[j].ID })
}

func skillsTaxonomyReportHash(report SkillsTaxonomyReport) string {
	report.Hash = ""
	return canonical.Hash(report)
}
