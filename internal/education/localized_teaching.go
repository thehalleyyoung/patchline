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

const LocalizedTeachingSpecVersion = "patchline.localized-teaching/v1"
const LocalizedTeachingReportVersion = "patchline.localized-teaching-report/v1"

const localizedTeachingGateName = "localized-teaching-examples-gate"

type LocalizedTeachingSpec struct {
	Version  string                    `json:"version"`
	Name     string                    `json:"name"`
	Claim    string                    `json:"claim,omitempty"`
	Criteria LocalizedTeachingCriteria `json:"criteria"`
	Examples []LocalizedExample        `json:"examples"`
}

type LocalizedTeachingCriteria struct {
	RequiredLocales                      []string `json:"required_locales"`
	RequiredAudiences                    []string `json:"required_audiences"`
	RequiredAccessibilityChecks          []string `json:"required_accessibility_checks"`
	MinExamples                          int      `json:"min_examples"`
	MinTranslationsPerExample            int      `json:"min_translations_per_example"`
	MinConceptsPerExample                int      `json:"min_concepts_per_example"`
	MinTechnicalTermsPerTranslation      int      `json:"min_technical_terms_per_translation"`
	MinEquivalenceChecksPerTranslation   int      `json:"min_equivalence_checks_per_translation"`
	MinAccessibilityChecksPerTranslation int      `json:"min_accessibility_checks_per_translation"`
	RequireTechnicalTerms                bool     `json:"require_technical_terms"`
	RequireEquivalenceChecks             bool     `json:"require_equivalence_checks"`
	RequireAccessibilityChecks           bool     `json:"require_accessibility_checks"`
	RequireReproducibleCommand           bool     `json:"require_reproducible_command"`
	RequireNegativeControl               bool     `json:"require_negative_control"`
}

type LocalizedExample struct {
	ID                string                 `json:"id"`
	Audience          string                 `json:"audience"`
	SourceLocale      string                 `json:"source_locale"`
	Title             string                 `json:"title"`
	SourceText        string                 `json:"source_text"`
	Concepts          []string               `json:"concepts"`
	EvidencePaths     []string               `json:"evidence_paths"`
	ReproduceCommands []string               `json:"reproduce_commands"`
	Translations      []LocalizedTranslation `json:"translations"`
	NegativeControls  []LocalizedControl     `json:"negative_controls"`
}

type LocalizedTranslation struct {
	Locale              string                        `json:"locale"`
	Title               string                        `json:"title"`
	Text                string                        `json:"text"`
	TechnicalTerms      []LocalizedTechnicalTerm      `json:"technical_terms"`
	EquivalenceChecks   []LocalizedEquivalenceCheck   `json:"equivalence_checks"`
	AccessibilityChecks []LocalizedAccessibilityCheck `json:"accessibility_checks"`
	EvidencePaths       []string                      `json:"evidence_paths"`
}

type LocalizedTechnicalTerm struct {
	ID           string `json:"id"`
	Source       string `json:"source"`
	Translation  string `json:"translation"`
	MustPreserve bool   `json:"must_preserve"`
}

type LocalizedEquivalenceCheck struct {
	ID              string   `json:"id"`
	Kind            string   `json:"kind"`
	SourceQuote     string   `json:"source_quote"`
	TranslatedQuote string   `json:"translated_quote"`
	PreservedTokens []string `json:"preserved_tokens"`
}

type LocalizedAccessibilityCheck struct {
	ID            string   `json:"id"`
	Type          string   `json:"type"`
	Requirement   string   `json:"requirement"`
	EvidencePaths []string `json:"evidence_paths"`
}

type LocalizedControl struct {
	ID                     string `json:"id"`
	Mutation               string `json:"mutation"`
	ExpectedCounterexample string `json:"expected_counterexample"`
}

type LocalizedTeachingReport struct {
	Version         string                         `json:"version"`
	Name            string                         `json:"name"`
	OK              bool                           `json:"ok"`
	Criteria        LocalizedTeachingCriteria      `json:"criteria"`
	Summary         LocalizedTeachingSummary       `json:"summary"`
	Examples        []LocalizedExampleReport       `json:"examples"`
	Counterexamples []LocalizedTeachingCountercase `json:"counterexamples,omitempty"`
	Hash            string                         `json:"hash"`
}

type LocalizedTeachingSummary struct {
	Examples             int `json:"examples"`
	Translations         int `json:"translations"`
	LocalesCovered       int `json:"locales_covered"`
	AudiencesCovered     int `json:"audiences_covered"`
	ReproducibleExamples int `json:"reproducible_examples"`
	TechnicalTerms       int `json:"technical_terms"`
	EquivalenceChecks    int `json:"equivalence_checks"`
	AccessibilityChecks  int `json:"accessibility_checks"`
	EvidenceArtifacts    int `json:"evidence_artifacts"`
	NegativeControls     int `json:"negative_controls"`
	Counterexamples      int `json:"counterexamples"`
}

type LocalizedExampleReport struct {
	ID                    string                       `json:"id"`
	Audience              string                       `json:"audience"`
	SourceLocale          string                       `json:"source_locale"`
	Title                 string                       `json:"title"`
	Concepts              int                          `json:"concepts"`
	GateBacked            bool                         `json:"gate_backed"`
	ReproducibleCommandOK bool                         `json:"reproducible_command_ok"`
	Evidence              []ArtifactEvidence           `json:"evidence"`
	Translations          []LocalizedTranslationReport `json:"translations"`
	NegativeControls      int                          `json:"negative_controls"`
}

type LocalizedTranslationReport struct {
	Locale                       string             `json:"locale"`
	Title                        string             `json:"title"`
	TechnicalTerms               int                `json:"technical_terms"`
	EquivalenceChecks            int                `json:"equivalence_checks"`
	AccessibilityChecks          int                `json:"accessibility_checks"`
	RequiredAccessibilityCovered []string           `json:"required_accessibility_covered"`
	Evidence                     []ArtifactEvidence `json:"evidence"`
}

type LocalizedTeachingCountercase struct {
	ID      string   `json:"id"`
	Kind    string   `json:"kind"`
	Subject string   `json:"subject,omitempty"`
	Message string   `json:"message"`
	Witness []string `json:"witness,omitempty"`
}

func ReadLocalizedTeachingSpec(reader io.Reader) (LocalizedTeachingSpec, error) {
	decoder := json.NewDecoder(reader)
	decoder.DisallowUnknownFields()
	var spec LocalizedTeachingSpec
	if err := decoder.Decode(&spec); err != nil {
		return LocalizedTeachingSpec{}, err
	}
	if spec.Version != LocalizedTeachingSpecVersion {
		return LocalizedTeachingSpec{}, fmt.Errorf("localized teaching spec version must be %s", LocalizedTeachingSpecVersion)
	}
	return spec, nil
}

func BuildLocalizedTeachingReport(spec LocalizedTeachingSpec, root string) (LocalizedTeachingReport, error) {
	if err := validateLocalizedTeachingSpec(spec); err != nil {
		return LocalizedTeachingReport{}, err
	}
	if root == "" {
		root = "."
	}
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return LocalizedTeachingReport{}, err
	}
	criteria := normalizeLocalizedTeachingCriteria(spec.Criteria)
	report := LocalizedTeachingReport{
		Version:  LocalizedTeachingReportVersion,
		Name:     spec.Name,
		OK:       true,
		Criteria: criteria,
	}
	if len(spec.Examples) < criteria.MinExamples {
		report.Counterexamples = append(report.Counterexamples, LocalizedTeachingCountercase{
			ID:      "criteria.min_examples",
			Kind:    "insufficient_examples",
			Message: fmt.Sprintf("localized teaching examples %d below required %d", len(spec.Examples), criteria.MinExamples),
		})
	}

	locales := map[string]struct{}{}
	audiences := map[string]struct{}{}
	for _, example := range sortedLocalizedExamples(spec.Examples) {
		exampleReport, counterexamples := buildLocalizedExampleReport(rootAbs, example, criteria)
		report.Examples = append(report.Examples, exampleReport)
		report.Counterexamples = append(report.Counterexamples, counterexamples...)
		report.Summary.Translations += len(exampleReport.Translations)
		report.Summary.TechnicalTerms += localizedExampleTechnicalTerms(exampleReport)
		report.Summary.EquivalenceChecks += localizedExampleEquivalenceChecks(exampleReport)
		report.Summary.AccessibilityChecks += localizedExampleAccessibilityChecks(exampleReport)
		report.Summary.EvidenceArtifacts += len(exampleReport.Evidence) + localizedExampleTranslationEvidence(exampleReport)
		report.Summary.NegativeControls += exampleReport.NegativeControls
		if exampleReport.GateBacked && exampleReport.ReproducibleCommandOK {
			report.Summary.ReproducibleExamples++
		}
		audience := normalizeToken(example.Audience)
		if audience != "" {
			audiences[audience] = struct{}{}
		}
		for _, translation := range exampleReport.Translations {
			locale := normalizeToken(translation.Locale)
			if locale != "" {
				locales[locale] = struct{}{}
			}
		}
	}
	for _, audience := range criteria.RequiredAudiences {
		if _, ok := audiences[audience]; !ok {
			report.Counterexamples = append(report.Counterexamples, LocalizedTeachingCountercase{
				ID:      "audience." + stableID(audience) + ".missing",
				Kind:    "missing_required_audience",
				Subject: audience,
				Message: "required teaching audience is not covered by any localized example",
			})
		}
	}
	for _, locale := range criteria.RequiredLocales {
		if _, ok := locales[locale]; !ok {
			report.Counterexamples = append(report.Counterexamples, LocalizedTeachingCountercase{
				ID:      "locale." + stableID(locale) + ".missing",
				Kind:    "missing_required_locale",
				Subject: locale,
				Message: "required target locale is not covered by any checked translation",
			})
		}
	}
	sortLocalizedTeachingCountercases(report.Counterexamples)
	report.Summary.Examples = len(report.Examples)
	report.Summary.LocalesCovered = len(locales)
	report.Summary.AudiencesCovered = len(audiences)
	report.Summary.Counterexamples = len(report.Counterexamples)
	report.OK = len(report.Counterexamples) == 0
	report.Hash = localizedTeachingReportHash(report)
	return report, nil
}

func WriteLocalizedTeachingArtifacts(outDir string, report LocalizedTeachingReport) error {
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}
	file, err := os.Create(filepath.Join(outDir, "localized-teaching-examples.json"))
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
	return os.WriteFile(filepath.Join(outDir, "localized-teaching-examples.md"), []byte(RenderLocalizedTeachingMarkdown(report)), 0o644)
}

func RenderLocalizedTeachingMarkdown(report LocalizedTeachingReport) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Localized teaching examples\n\n")
	fmt.Fprintf(&b, "Patchline validates localized teaching examples by preserving byte-identical technical tokens across translations, hashing real evidence files, and requiring accessibility checks plus negative controls.\n\n")
	fmt.Fprintf(&b, "## Summary\n\n")
	fmt.Fprintf(&b, "| Metric | Value |\n| --- | ---: |\n")
	fmt.Fprintf(&b, "| OK | `%t` |\n", report.OK)
	fmt.Fprintf(&b, "| Examples | %d |\n", report.Summary.Examples)
	fmt.Fprintf(&b, "| Translations | %d |\n", report.Summary.Translations)
	fmt.Fprintf(&b, "| Locales covered | %d |\n", report.Summary.LocalesCovered)
	fmt.Fprintf(&b, "| Audiences covered | %d |\n", report.Summary.AudiencesCovered)
	fmt.Fprintf(&b, "| Reproducible examples | %d |\n", report.Summary.ReproducibleExamples)
	fmt.Fprintf(&b, "| Technical terms | %d |\n", report.Summary.TechnicalTerms)
	fmt.Fprintf(&b, "| Equivalence checks | %d |\n", report.Summary.EquivalenceChecks)
	fmt.Fprintf(&b, "| Accessibility checks | %d |\n", report.Summary.AccessibilityChecks)
	fmt.Fprintf(&b, "| Evidence artifacts | %d |\n", report.Summary.EvidenceArtifacts)
	fmt.Fprintf(&b, "| Negative controls | %d |\n", report.Summary.NegativeControls)
	fmt.Fprintf(&b, "| Counterexamples | %d |\n\n", report.Summary.Counterexamples)
	fmt.Fprintf(&b, "## Translation coverage\n\n")
	fmt.Fprintf(&b, "| Example | Audience | Locale | Terms | Equivalence checks | Accessibility checks | Evidence hashes |\n| --- | --- | --- | ---: | ---: | ---: | --- |\n")
	for _, example := range report.Examples {
		for _, translation := range example.Translations {
			hashes := make([]string, 0, len(translation.Evidence))
			for _, evidence := range translation.Evidence {
				hash := evidence.SHA256
				if len(hash) > 16 {
					hash = hash[:16]
				}
				hashes = append(hashes, evidence.Path+":"+hash)
			}
			fmt.Fprintf(&b, "| `%s` | `%s` | `%s` | %d | %d | %d | %s |\n",
				escapeTable(example.ID),
				escapeTable(example.Audience),
				escapeTable(translation.Locale),
				translation.TechnicalTerms,
				translation.EquivalenceChecks,
				translation.AccessibilityChecks,
				escapeTable(strings.Join(hashes, "; ")),
			)
		}
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

func buildLocalizedExampleReport(root string, example LocalizedExample, criteria LocalizedTeachingCriteria) (LocalizedExampleReport, []LocalizedTeachingCountercase) {
	subject := example.ID
	evidence, counterexamples := collectLocalizedTeachingArtifacts(root, example.EvidencePaths, subject)
	gateBacked := gateExists(root, localizedTeachingGateName)
	reproducible := containsCommand(example.ReproduceCommands, requiredGateCommand(localizedTeachingGateName))
	report := LocalizedExampleReport{
		ID:                    example.ID,
		Audience:              normalizeToken(example.Audience),
		SourceLocale:          normalizeToken(example.SourceLocale),
		Title:                 example.Title,
		Concepts:              len(normalizedStrings(example.Concepts)),
		GateBacked:            gateBacked,
		ReproducibleCommandOK: reproducible,
		Evidence:              evidence,
		NegativeControls:      len(example.NegativeControls),
	}
	if !gateBacked {
		counterexamples = append(counterexamples, LocalizedTeachingCountercase{
			ID:      "example." + stableID(subject, "gate") + ".missing",
			Kind:    "missing_gate",
			Subject: subject,
			Message: "localized teaching examples gate is not present as a Make target backed by a script",
			Witness: []string{localizedTeachingGateName},
		})
	}
	if criteria.RequireReproducibleCommand && !reproducible {
		counterexamples = append(counterexamples, LocalizedTeachingCountercase{
			ID:      "example." + stableID(subject, "command") + ".missing",
			Kind:    "missing_reproducible_command",
			Subject: subject,
			Message: "localized example does not include the exact reproducing gate command",
			Witness: []string{requiredGateCommand(localizedTeachingGateName)},
		})
	}
	if report.Concepts < criteria.MinConceptsPerExample {
		counterexamples = append(counterexamples, LocalizedTeachingCountercase{
			ID:      "example." + stableID(subject, "concepts") + ".insufficient",
			Kind:    "insufficient_concepts",
			Subject: subject,
			Message: fmt.Sprintf("example has %d concepts below required %d", report.Concepts, criteria.MinConceptsPerExample),
		})
	}
	if len(example.Translations) < criteria.MinTranslationsPerExample {
		counterexamples = append(counterexamples, LocalizedTeachingCountercase{
			ID:      "example." + stableID(subject, "translations") + ".insufficient",
			Kind:    "insufficient_translations",
			Subject: subject,
			Message: fmt.Sprintf("example has %d translations below required %d", len(example.Translations), criteria.MinTranslationsPerExample),
		})
	}
	if criteria.RequireNegativeControl && len(example.NegativeControls) == 0 {
		counterexamples = append(counterexamples, LocalizedTeachingCountercase{
			ID:      "example." + stableID(subject, "negative-control") + ".missing",
			Kind:    "missing_negative_control",
			Subject: subject,
			Message: "localized example does not include a failing translation or accessibility mutation",
		})
	}
	for _, control := range example.NegativeControls {
		if strings.TrimSpace(control.ID) == "" || strings.TrimSpace(control.Mutation) == "" || strings.TrimSpace(control.ExpectedCounterexample) == "" {
			counterexamples = append(counterexamples, LocalizedTeachingCountercase{
				ID:      "example." + stableID(subject, control.ID, "negative-control") + ".incomplete",
				Kind:    "incomplete_negative_control",
				Subject: subject,
				Message: "negative control must include id, mutation, and expected counterexample",
			})
		}
	}
	translationLocales := map[string]struct{}{}
	for _, translation := range sortedLocalizedTranslations(example.Translations) {
		translationReport, translationCounterexamples := buildLocalizedTranslationReport(root, example, translation, criteria)
		report.Translations = append(report.Translations, translationReport)
		counterexamples = append(counterexamples, translationCounterexamples...)
		if translationReport.Locale != "" {
			translationLocales[translationReport.Locale] = struct{}{}
		}
	}
	for _, requiredLocale := range criteria.RequiredLocales {
		if _, ok := translationLocales[requiredLocale]; !ok {
			counterexamples = append(counterexamples, LocalizedTeachingCountercase{
				ID:      "example." + stableID(subject, requiredLocale, "locale") + ".missing",
				Kind:    "missing_required_locale",
				Subject: subject,
				Message: "localized example does not include required target locale " + requiredLocale,
				Witness: []string{requiredLocale},
			})
		}
	}
	sortLocalizedTeachingCountercases(counterexamples)
	return report, counterexamples
}

func buildLocalizedTranslationReport(root string, example LocalizedExample, translation LocalizedTranslation, criteria LocalizedTeachingCriteria) (LocalizedTranslationReport, []LocalizedTeachingCountercase) {
	subject := example.ID + "/" + normalizeToken(translation.Locale)
	paths := append([]string{}, translation.EvidencePaths...)
	for _, check := range translation.AccessibilityChecks {
		paths = append(paths, check.EvidencePaths...)
	}
	evidence, counterexamples := collectLocalizedTeachingArtifacts(root, paths, subject)
	report := LocalizedTranslationReport{
		Locale:              normalizeToken(translation.Locale),
		Title:               translation.Title,
		TechnicalTerms:      len(translation.TechnicalTerms),
		EquivalenceChecks:   len(translation.EquivalenceChecks),
		AccessibilityChecks: len(translation.AccessibilityChecks),
		Evidence:            evidence,
	}
	if criteria.RequireTechnicalTerms && len(translation.TechnicalTerms) < criteria.MinTechnicalTermsPerTranslation {
		counterexamples = append(counterexamples, LocalizedTeachingCountercase{
			ID:      "translation." + stableID(subject, "terms") + ".insufficient",
			Kind:    "missing_technical_terms",
			Subject: subject,
			Message: fmt.Sprintf("translation has %d technical terms below required %d", len(translation.TechnicalTerms), criteria.MinTechnicalTermsPerTranslation),
		})
	}
	for _, term := range sortedLocalizedTechnicalTerms(translation.TechnicalTerms) {
		counterexamples = append(counterexamples, validateLocalizedTechnicalTerm(example, translation, term, subject)...)
	}
	if criteria.RequireEquivalenceChecks && len(translation.EquivalenceChecks) < criteria.MinEquivalenceChecksPerTranslation {
		counterexamples = append(counterexamples, LocalizedTeachingCountercase{
			ID:      "translation." + stableID(subject, "equivalence") + ".insufficient",
			Kind:    "missing_equivalence_check",
			Subject: subject,
			Message: fmt.Sprintf("translation has %d equivalence checks below required %d", len(translation.EquivalenceChecks), criteria.MinEquivalenceChecksPerTranslation),
		})
	}
	for _, check := range sortedLocalizedEquivalenceChecks(translation.EquivalenceChecks) {
		counterexamples = append(counterexamples, validateLocalizedEquivalenceCheck(example, translation, check, subject)...)
	}
	covered := map[string]struct{}{}
	for _, check := range sortedLocalizedAccessibilityChecks(translation.AccessibilityChecks) {
		checkType := normalizeToken(check.Type)
		if checkType != "" {
			covered[checkType] = struct{}{}
		}
		if strings.TrimSpace(check.ID) == "" || checkType == "" || strings.TrimSpace(check.Requirement) == "" || len(check.EvidencePaths) == 0 {
			counterexamples = append(counterexamples, LocalizedTeachingCountercase{
				ID:      "translation." + stableID(subject, check.ID, "accessibility") + ".incomplete",
				Kind:    "incomplete_accessibility_check",
				Subject: subject,
				Message: "accessibility check must include id, type, requirement, and evidence paths",
			})
		}
	}
	report.RequiredAccessibilityCovered = sortedStrings(mapKeys(covered))
	if criteria.RequireAccessibilityChecks && len(translation.AccessibilityChecks) < criteria.MinAccessibilityChecksPerTranslation {
		counterexamples = append(counterexamples, LocalizedTeachingCountercase{
			ID:      "translation." + stableID(subject, "accessibility") + ".insufficient",
			Kind:    "insufficient_accessibility_checks",
			Subject: subject,
			Message: fmt.Sprintf("translation has %d accessibility checks below required %d", len(translation.AccessibilityChecks), criteria.MinAccessibilityChecksPerTranslation),
		})
	}
	if criteria.RequireAccessibilityChecks {
		for _, required := range criteria.RequiredAccessibilityChecks {
			if _, ok := covered[required]; !ok {
				counterexamples = append(counterexamples, LocalizedTeachingCountercase{
					ID:      "translation." + stableID(subject, required, "accessibility") + ".missing",
					Kind:    "missing_required_accessibility_check",
					Subject: subject,
					Message: "translation does not cover required accessibility check " + required,
					Witness: []string{required},
				})
			}
		}
	}
	sortLocalizedTeachingCountercases(counterexamples)
	return report, counterexamples
}

func validateLocalizedTechnicalTerm(example LocalizedExample, translation LocalizedTranslation, term LocalizedTechnicalTerm, subject string) []LocalizedTeachingCountercase {
	var counterexamples []LocalizedTeachingCountercase
	if strings.TrimSpace(term.ID) == "" || strings.TrimSpace(term.Source) == "" || strings.TrimSpace(term.Translation) == "" {
		counterexamples = append(counterexamples, LocalizedTeachingCountercase{
			ID:      "term." + stableID(subject, term.ID, "detail") + ".incomplete",
			Kind:    "incomplete_technical_term",
			Subject: subject,
			Message: "technical term must include id, source, and translation",
		})
		return counterexamples
	}
	if !strings.Contains(example.SourceText, term.Source) {
		counterexamples = append(counterexamples, LocalizedTeachingCountercase{
			ID:      "term." + stableID(subject, term.ID, "source") + ".missing",
			Kind:    "missing_source_technical_token",
			Subject: subject,
			Message: "technical source token is not present in the source teaching text",
			Witness: []string{term.Source},
		})
	}
	if !strings.Contains(translation.Text, term.Translation) {
		counterexamples = append(counterexamples, LocalizedTeachingCountercase{
			ID:      "term." + stableID(subject, term.ID, "translation") + ".missing",
			Kind:    "missing_translated_technical_token",
			Subject: subject,
			Message: "technical translation token is not present in the localized teaching text",
			Witness: []string{term.Translation},
		})
	}
	if term.MustPreserve && (term.Source != term.Translation || !strings.Contains(translation.Text, term.Source)) {
		counterexamples = append(counterexamples, LocalizedTeachingCountercase{
			ID:      "term." + stableID(subject, term.ID, "preserve") + ".missing",
			Kind:    "missing_preserved_technical_token",
			Subject: subject,
			Message: "byte-identical command or identifier token was translated, removed, or altered",
			Witness: []string{term.Source},
		})
	}
	return counterexamples
}

func validateLocalizedEquivalenceCheck(example LocalizedExample, translation LocalizedTranslation, check LocalizedEquivalenceCheck, subject string) []LocalizedTeachingCountercase {
	var counterexamples []LocalizedTeachingCountercase
	if strings.TrimSpace(check.ID) == "" || strings.TrimSpace(check.Kind) == "" {
		counterexamples = append(counterexamples, LocalizedTeachingCountercase{
			ID:      "equivalence." + stableID(subject, check.ID, "detail") + ".incomplete",
			Kind:    "incomplete_equivalence_check",
			Subject: subject,
			Message: "equivalence check must include id and kind",
		})
	}
	if strings.TrimSpace(check.SourceQuote) == "" || strings.TrimSpace(check.TranslatedQuote) == "" || !strings.Contains(example.SourceText, check.SourceQuote) || !strings.Contains(translation.Text, check.TranslatedQuote) {
		counterexamples = append(counterexamples, LocalizedTeachingCountercase{
			ID:      "equivalence." + stableID(subject, check.ID, "quote") + ".missing",
			Kind:    "missing_equivalence_quote",
			Subject: subject,
			Message: "equivalence check quotes must be present in the source and localized teaching text",
			Witness: []string{check.SourceQuote, check.TranslatedQuote},
		})
	}
	if len(check.PreservedTokens) == 0 {
		counterexamples = append(counterexamples, LocalizedTeachingCountercase{
			ID:      "equivalence." + stableID(subject, check.ID, "token") + ".missing",
			Kind:    "missing_preserved_equivalence_token",
			Subject: subject,
			Message: "equivalence check must name at least one byte-identical command, identifier, or numeric token",
		})
	}
	for _, token := range uniqueSorted(check.PreservedTokens) {
		if !strings.Contains(example.SourceText, token) || !strings.Contains(translation.Text, token) {
			counterexamples = append(counterexamples, LocalizedTeachingCountercase{
				ID:      "equivalence." + stableID(subject, check.ID, token) + ".missing",
				Kind:    "missing_preserved_equivalence_token",
				Subject: subject,
				Message: "preserved equivalence token is not byte-identical across source and localized text",
				Witness: []string{token},
			})
		}
	}
	return counterexamples
}

func validateLocalizedTeachingSpec(spec LocalizedTeachingSpec) error {
	if strings.TrimSpace(spec.Name) == "" {
		return fmt.Errorf("localized teaching spec name is required")
	}
	criteria := spec.Criteria
	if len(criteria.RequiredLocales) == 0 {
		return fmt.Errorf("criteria.required_locales is required")
	}
	if len(criteria.RequiredAudiences) == 0 {
		return fmt.Errorf("criteria.required_audiences is required")
	}
	if len(criteria.RequiredAccessibilityChecks) == 0 {
		return fmt.Errorf("criteria.required_accessibility_checks is required")
	}
	if criteria.MinExamples <= 0 {
		return fmt.Errorf("criteria.min_examples must be positive")
	}
	if criteria.MinTranslationsPerExample <= 0 {
		return fmt.Errorf("criteria.min_translations_per_example must be positive")
	}
	if criteria.MinConceptsPerExample <= 0 {
		return fmt.Errorf("criteria.min_concepts_per_example must be positive")
	}
	if criteria.MinTechnicalTermsPerTranslation <= 0 {
		return fmt.Errorf("criteria.min_technical_terms_per_translation must be positive")
	}
	if criteria.MinEquivalenceChecksPerTranslation <= 0 {
		return fmt.Errorf("criteria.min_equivalence_checks_per_translation must be positive")
	}
	if criteria.MinAccessibilityChecksPerTranslation <= 0 {
		return fmt.Errorf("criteria.min_accessibility_checks_per_translation must be positive")
	}
	seenExamples := map[string]struct{}{}
	for _, example := range spec.Examples {
		if strings.TrimSpace(example.ID) == "" {
			return fmt.Errorf("localized example id is required")
		}
		if _, ok := seenExamples[example.ID]; ok {
			return fmt.Errorf("duplicate localized example id %q", example.ID)
		}
		seenExamples[example.ID] = struct{}{}
		if strings.TrimSpace(example.Audience) == "" || strings.TrimSpace(example.SourceLocale) == "" || strings.TrimSpace(example.Title) == "" || strings.TrimSpace(example.SourceText) == "" {
			return fmt.Errorf("localized example %q requires audience, source_locale, title, and source_text", example.ID)
		}
		for _, path := range example.EvidencePaths {
			if err := validateRelativePath(path); err != nil {
				return fmt.Errorf("localized example %q evidence path: %w", example.ID, err)
			}
		}
		seenTranslations := map[string]struct{}{}
		for _, translation := range example.Translations {
			locale := normalizeToken(translation.Locale)
			if locale == "" {
				return fmt.Errorf("localized example %q translation locale is required", example.ID)
			}
			if _, ok := seenTranslations[locale]; ok {
				return fmt.Errorf("localized example %q contains duplicate translation locale %q", example.ID, translation.Locale)
			}
			seenTranslations[locale] = struct{}{}
			if strings.TrimSpace(translation.Title) == "" || strings.TrimSpace(translation.Text) == "" {
				return fmt.Errorf("localized example %q translation %q requires title and text", example.ID, translation.Locale)
			}
			for _, path := range translation.EvidencePaths {
				if err := validateRelativePath(path); err != nil {
					return fmt.Errorf("localized example %q translation %q evidence path: %w", example.ID, translation.Locale, err)
				}
			}
			for _, check := range translation.AccessibilityChecks {
				for _, path := range check.EvidencePaths {
					if err := validateRelativePath(path); err != nil {
						return fmt.Errorf("localized example %q translation %q accessibility evidence path: %w", example.ID, translation.Locale, err)
					}
				}
			}
		}
	}
	return nil
}

func collectLocalizedTeachingArtifacts(root string, paths []string, subject string) ([]ArtifactEvidence, []LocalizedTeachingCountercase) {
	var artifacts []ArtifactEvidence
	var counterexamples []LocalizedTeachingCountercase
	for _, relPath := range uniqueSorted(paths) {
		fullPath, err := safeJoin(root, relPath)
		if err != nil {
			counterexamples = append(counterexamples, LocalizedTeachingCountercase{
				ID:      "localized." + stableID(subject, relPath, "evidence-path") + ".invalid",
				Kind:    "invalid_evidence_path",
				Subject: subject,
				Message: err.Error(),
				Witness: []string{relPath},
			})
			continue
		}
		data, err := os.ReadFile(fullPath)
		if err != nil {
			counterexamples = append(counterexamples, LocalizedTeachingCountercase{
				ID:      "localized." + stableID(subject, relPath, "evidence") + ".missing",
				Kind:    "missing_evidence",
				Subject: subject,
				Message: "localized teaching evidence could not be read",
				Witness: []string{relPath},
			})
			continue
		}
		if len(data) == 0 {
			counterexamples = append(counterexamples, LocalizedTeachingCountercase{
				ID:      "localized." + stableID(subject, relPath, "evidence") + ".empty",
				Kind:    "empty_evidence",
				Subject: subject,
				Message: "localized teaching evidence is empty",
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

func normalizeLocalizedTeachingCriteria(criteria LocalizedTeachingCriteria) LocalizedTeachingCriteria {
	criteria.RequiredLocales = sortedStrings(normalizedStrings(criteria.RequiredLocales))
	criteria.RequiredAudiences = sortedStrings(normalizedStrings(criteria.RequiredAudiences))
	criteria.RequiredAccessibilityChecks = sortedStrings(normalizedStrings(criteria.RequiredAccessibilityChecks))
	return criteria
}

func sortedLocalizedExamples(examples []LocalizedExample) []LocalizedExample {
	out := append([]LocalizedExample(nil), examples...)
	sort.SliceStable(out, func(i, j int) bool {
		left := normalizeToken(out[i].Audience) + "\x00" + out[i].ID
		right := normalizeToken(out[j].Audience) + "\x00" + out[j].ID
		return left < right
	})
	return out
}

func sortedLocalizedTranslations(translations []LocalizedTranslation) []LocalizedTranslation {
	out := append([]LocalizedTranslation(nil), translations...)
	sort.SliceStable(out, func(i, j int) bool { return normalizeToken(out[i].Locale) < normalizeToken(out[j].Locale) })
	return out
}

func sortedLocalizedTechnicalTerms(terms []LocalizedTechnicalTerm) []LocalizedTechnicalTerm {
	out := append([]LocalizedTechnicalTerm(nil), terms...)
	sort.SliceStable(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func sortedLocalizedEquivalenceChecks(checks []LocalizedEquivalenceCheck) []LocalizedEquivalenceCheck {
	out := append([]LocalizedEquivalenceCheck(nil), checks...)
	sort.SliceStable(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func sortedLocalizedAccessibilityChecks(checks []LocalizedAccessibilityCheck) []LocalizedAccessibilityCheck {
	out := append([]LocalizedAccessibilityCheck(nil), checks...)
	sort.SliceStable(out, func(i, j int) bool {
		return normalizeToken(out[i].Type)+"\x00"+out[i].ID < normalizeToken(out[j].Type)+"\x00"+out[j].ID
	})
	return out
}

func localizedExampleTechnicalTerms(example LocalizedExampleReport) int {
	total := 0
	for _, translation := range example.Translations {
		total += translation.TechnicalTerms
	}
	return total
}

func localizedExampleEquivalenceChecks(example LocalizedExampleReport) int {
	total := 0
	for _, translation := range example.Translations {
		total += translation.EquivalenceChecks
	}
	return total
}

func localizedExampleAccessibilityChecks(example LocalizedExampleReport) int {
	total := 0
	for _, translation := range example.Translations {
		total += translation.AccessibilityChecks
	}
	return total
}

func localizedExampleTranslationEvidence(example LocalizedExampleReport) int {
	total := 0
	for _, translation := range example.Translations {
		total += len(translation.Evidence)
	}
	return total
}

func mapKeys(values map[string]struct{}) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	return keys
}

func sortLocalizedTeachingCountercases(counterexamples []LocalizedTeachingCountercase) {
	sort.SliceStable(counterexamples, func(i, j int) bool { return counterexamples[i].ID < counterexamples[j].ID })
}

func localizedTeachingReportHash(report LocalizedTeachingReport) string {
	report.Hash = ""
	return canonical.Hash(report)
}
