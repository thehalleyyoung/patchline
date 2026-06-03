package education

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildLocalizedTeachingReportValidatesTranslations(t *testing.T) {
	root := localizedTeachingRoot(t)
	report, err := BuildLocalizedTeachingReport(validLocalizedTeachingSpec(), root)
	if err != nil {
		t.Fatal(err)
	}
	if !report.OK {
		t.Fatalf("expected clean localized teaching report, got counterexamples %#v", report.Counterexamples)
	}
	if report.Summary.Examples != 2 || report.Summary.Translations != 4 || report.Summary.LocalesCovered != 2 || report.Summary.AudiencesCovered != 2 {
		t.Fatalf("unexpected summary: %#v", report.Summary)
	}
	if report.Summary.TechnicalTerms != 12 || report.Summary.EquivalenceChecks != 8 || report.Summary.AccessibilityChecks != 12 || report.Summary.EvidenceArtifacts != 20 {
		t.Fatalf("expected technical, equivalence, accessibility, and evidence coverage, got %#v", report.Summary)
	}
	for _, example := range report.Examples {
		if !example.GateBacked || !example.ReproducibleCommandOK {
			t.Fatalf("expected gate-backed reproducible example: %#v", example)
		}
		for _, translation := range example.Translations {
			if len(translation.Evidence) != 4 || len(translation.Evidence[0].SHA256) != 64 {
				t.Fatalf("translation missing evidence hashes: %#v", translation)
			}
			if strings.Join(translation.RequiredAccessibilityCovered, ",") != "alt-text,plain-language,reading-order" {
				t.Fatalf("unexpected accessibility coverage: %#v", translation.RequiredAccessibilityCovered)
			}
		}
	}
	markdown := RenderLocalizedTeachingMarkdown(report)
	if !strings.Contains(markdown, "Localized teaching examples") || !strings.Contains(markdown, "Translation coverage") {
		t.Fatalf("expected useful markdown, got:\n%s", markdown)
	}
}

func TestBuildLocalizedTeachingReportFindsBrokenTranslations(t *testing.T) {
	root := localizedTeachingRoot(t)
	spec := validLocalizedTeachingSpec()
	spec.Examples = spec.Examples[:1]
	spec.Examples[0].Concepts = spec.Examples[0].Concepts[:1]
	spec.Examples[0].ReproduceCommands = nil
	spec.Examples[0].EvidencePaths = []string{"docs/missing-localized-evidence.md"}
	spec.Examples[0].Translations = spec.Examples[0].Translations[:1]
	spec.Examples[0].Translations[0].TechnicalTerms = []LocalizedTechnicalTerm{{
		ID: "risk-token", Source: "risk_id", Translation: "riesgo_id", MustPreserve: true,
	}}
	spec.Examples[0].Translations[0].EquivalenceChecks = []LocalizedEquivalenceCheck{{
		ID: "broken-equivalence", Kind: "identifier-preservation", SourceQuote: "missing source quote", TranslatedQuote: "missing translated quote", PreservedTokens: []string{"missing_token"},
	}}
	spec.Examples[0].Translations[0].AccessibilityChecks = nil
	spec.Examples[0].NegativeControls = nil
	report, err := BuildLocalizedTeachingReport(spec, root)
	if err != nil {
		t.Fatal(err)
	}
	if report.OK {
		t.Fatalf("expected deficient localized teaching report to fail: %#v", report)
	}
	for _, kind := range []string{
		"insufficient_examples",
		"missing_required_audience",
		"missing_required_locale",
		"insufficient_concepts",
		"missing_reproducible_command",
		"missing_evidence",
		"insufficient_translations",
		"missing_technical_terms",
		"missing_preserved_technical_token",
		"missing_equivalence_check",
		"missing_equivalence_quote",
		"missing_preserved_equivalence_token",
		"insufficient_accessibility_checks",
		"missing_required_accessibility_check",
		"missing_negative_control",
	} {
		if !hasLocalizedTeachingCountercase(report, kind) {
			t.Fatalf("expected %s counterexample, got %#v", kind, report.Counterexamples)
		}
	}
}

func TestReadLocalizedTeachingSpecRejectsUnknownFields(t *testing.T) {
	_, err := ReadLocalizedTeachingSpec(strings.NewReader(`{"version":"patchline.localized-teaching/v1","name":"x","criteria":{"required_locales":["es"],"required_audiences":["dba"],"required_accessibility_checks":["plain-language"],"min_examples":1,"min_translations_per_example":1,"min_concepts_per_example":1,"min_technical_terms_per_translation":1,"min_equivalence_checks_per_translation":1,"min_accessibility_checks_per_translation":1,"require_technical_terms":true,"require_equivalence_checks":true,"require_accessibility_checks":true,"require_reproducible_command":true,"require_negative_control":true},"examples":[],"extra":true}`))
	if err == nil {
		t.Fatal("expected unknown field error")
	}
}

func TestWriteLocalizedTeachingArtifactsIsDeterministic(t *testing.T) {
	root := localizedTeachingRoot(t)
	report, err := BuildLocalizedTeachingReport(validLocalizedTeachingSpec(), root)
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "localized-teaching")
	if err := WriteLocalizedTeachingArtifacts(out, report); err != nil {
		t.Fatal(err)
	}
	var reread LocalizedTeachingReport
	file, err := os.Open(filepath.Join(out, "localized-teaching-examples.json"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	if err := json.NewDecoder(file).Decode(&reread); err != nil {
		t.Fatal(err)
	}
	if reread.Hash != report.Hash {
		t.Fatalf("report hash changed after write/read: got %s want %s", reread.Hash, report.Hash)
	}
}

func validLocalizedTeachingSpec() LocalizedTeachingSpec {
	return LocalizedTeachingSpec{
		Version: LocalizedTeachingSpecVersion,
		Name:    "Patchline localized teaching examples",
		Claim:   "Patchline localized teaching examples preserve technical tokens, hash real evidence, check accessibility, and include negative controls so translated migration-safety lessons stay equivalent.",
		Criteria: LocalizedTeachingCriteria{
			RequiredLocales:                      []string{"es", "fr"},
			RequiredAudiences:                    []string{"app-developer", "dba"},
			RequiredAccessibilityChecks:          []string{"plain-language", "alt-text", "reading-order"},
			MinExamples:                          2,
			MinTranslationsPerExample:            2,
			MinConceptsPerExample:                2,
			MinTechnicalTermsPerTranslation:      3,
			MinEquivalenceChecksPerTranslation:   2,
			MinAccessibilityChecksPerTranslation: 3,
			RequireTechnicalTerms:                true,
			RequireEquivalenceChecks:             true,
			RequireAccessibilityChecks:           true,
			RequireReproducibleCommand:           true,
			RequireNegativeControl:               true,
		},
		Examples: []LocalizedExample{
			localizedExample("app", "app-developer", "make classroom-lab-kits-gate", "risk_id", "evidence_hash", "docs/classroom-lab-kits.md"),
			localizedExample("dba", "dba", "make db-semantics-reproducibility-gate", "engine_version", "rollback_feasibility", "docs/db-semantics-reproducibility.md"),
		},
	}
}

func localizedExample(id, audience, command, firstIdentifier, secondIdentifier, evidencePath string) LocalizedExample {
	sourceText := "Run `" + command + "` and keep `" + firstIdentifier + "` plus `" + secondIdentifier + "` visible in the Patchline lesson."
	return LocalizedExample{
		ID:                id,
		Audience:          audience,
		SourceLocale:      "en",
		Title:             id + " localized lesson",
		SourceText:        sourceText,
		Concepts:          []string{"technical equivalence", "accessible teaching"},
		EvidencePaths:     []string{evidencePath, "docs/a11y-i18n-output.md"},
		ReproduceCommands: []string{"make localized-teaching-examples-gate", "make " + strings.TrimPrefix(command, "make ")},
		Translations: []LocalizedTranslation{
			localizedTranslation("es", command, firstIdentifier, secondIdentifier, sourceText, evidencePath),
			localizedTranslation("fr", command, firstIdentifier, secondIdentifier, sourceText, evidencePath),
		},
		NegativeControls: []LocalizedControl{{
			ID:                     "translate-token",
			Mutation:               "translate the preserved identifier",
			ExpectedCounterexample: "missing preserved technical token",
		}},
	}
}

func localizedTranslation(locale, command, firstIdentifier, secondIdentifier, sourceText, evidencePath string) LocalizedTranslation {
	joiner := "y"
	if locale == "fr" {
		joiner = "et"
	}
	text := "Ejecute `" + command + "` y mantenga `" + firstIdentifier + "` " + joiner + " `" + secondIdentifier + "` visibles en la leccion Patchline."
	if locale == "fr" {
		text = "Executez `" + command + "` et gardez `" + firstIdentifier + "` " + joiner + " `" + secondIdentifier + "` visibles dans la lecon Patchline."
	}
	return LocalizedTranslation{
		Locale: locale,
		Title:  locale + " localized lesson",
		Text:   text,
		TechnicalTerms: []LocalizedTechnicalTerm{{
			ID: "patchline", Source: "Patchline", Translation: "Patchline", MustPreserve: true,
		}, {
			ID: "first-identifier", Source: firstIdentifier, Translation: firstIdentifier, MustPreserve: true,
		}, {
			ID: "second-identifier", Source: secondIdentifier, Translation: secondIdentifier, MustPreserve: true,
		}},
		EquivalenceChecks: []LocalizedEquivalenceCheck{{
			ID: "command-preserved", Kind: "command-preservation", SourceQuote: "`" + command + "`", TranslatedQuote: "`" + command + "`", PreservedTokens: []string{command},
		}, {
			ID: "identifiers-preserved", Kind: "identifier-preservation", SourceQuote: "`" + firstIdentifier + "` plus `" + secondIdentifier + "`", TranslatedQuote: "`" + firstIdentifier + "` " + joiner + " `" + secondIdentifier + "`", PreservedTokens: []string{firstIdentifier, secondIdentifier},
		}},
		AccessibilityChecks: []LocalizedAccessibilityCheck{{
			ID: "plain-language", Type: "plain-language", Requirement: "plain language", EvidencePaths: []string{"docs/a11y-i18n-output.md"},
		}, {
			ID: "alt-text", Type: "alt-text", Requirement: "alt text names the command", EvidencePaths: []string{"docs/accessibility-conformance.md"},
		}, {
			ID: "reading-order", Type: "reading-order", Requirement: "command appears before decision", EvidencePaths: []string{"docs/localized-teaching-examples.md"},
		}},
		EvidencePaths: []string{evidencePath},
	}
}

func localizedTeachingRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeLocalizedTeachingFile(t, root, "Makefile", "localized-teaching-examples-gate:\n\tbash scripts/localized-teaching-examples-gate.sh\n\n")
	writeLocalizedTeachingFile(t, root, "scripts/localized-teaching-examples-gate.sh", "#!/usr/bin/env bash\nset -euo pipefail\n")
	writeLocalizedTeachingFile(t, root, "docs/classroom-lab-kits.md", "Classroom lab kit evidence.\n")
	writeLocalizedTeachingFile(t, root, "docs/db-semantics-reproducibility.md", "Database semantics evidence.\n")
	writeLocalizedTeachingFile(t, root, "docs/a11y-i18n-output.md", "Accessibility and i18n evidence.\n")
	writeLocalizedTeachingFile(t, root, "docs/accessibility-conformance.md", "WCAG conformance evidence.\n")
	writeLocalizedTeachingFile(t, root, "docs/localized-teaching-examples.md", "Localized teaching examples evidence.\n")
	return root
}

func writeLocalizedTeachingFile(t *testing.T, root, relPath, contents string) {
	t.Helper()
	path := filepath.Join(root, relPath)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
}

func hasLocalizedTeachingCountercase(report LocalizedTeachingReport, kind string) bool {
	for _, counterexample := range report.Counterexamples {
		if counterexample.Kind == kind {
			return true
		}
	}
	return false
}
