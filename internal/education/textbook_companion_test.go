package education

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildTextbookCompanionReportValidatesExecutableNotebooks(t *testing.T) {
	root := textbookCompanionRoot(t)
	report, err := BuildTextbookCompanionReport(validTextbookCompanionSpec(), root)
	if err != nil {
		t.Fatal(err)
	}
	if !report.OK {
		t.Fatalf("expected clean textbook companion report, got counterexamples %#v", report.Counterexamples)
	}
	if report.Summary.Chapters != 3 || report.Summary.Notebooks != 3 || report.Summary.ExecutableNotebooks != 3 || report.Summary.TeachingExamples != 3 {
		t.Fatalf("unexpected summary: %#v", report.Summary)
	}
	if report.Summary.Commands != 3 || report.Summary.EvidenceArtifacts != 9 || report.Summary.GeneratedArtifacts != 6 || report.Summary.NegativeControls != 3 {
		t.Fatalf("expected commands, evidence, generated artifacts, and controls for every notebook, got %#v", report.Summary)
	}
	for _, chapter := range report.Chapters {
		if len(chapter.NotebookReports) != 1 || len(chapter.NotebookReports[0].GeneratedArtifacts) != 2 || len(chapter.NotebookReports[0].GeneratedArtifacts[0].SHA256) != 64 {
			t.Fatalf("chapter report missing generated artifact hashes: %#v", chapter)
		}
	}
	markdown := RenderTextbookCompanionMarkdown(report)
	if !strings.Contains(markdown, "Open textbook companion") || !strings.Contains(markdown, "Notebook regeneration map") {
		t.Fatalf("expected useful markdown, got:\n%s", markdown)
	}
}

func TestBuildTextbookCompanionReportFindsBrokenNotebookRegeneration(t *testing.T) {
	root := textbookCompanionRoot(t)
	spec := validTextbookCompanionSpec()
	spec.Chapters = spec.Chapters[:2]
	spec.Chapters[0].Notebooks[0].Path = "examples/textbook-companion/missing.ipynb"
	spec.Chapters[0].Notebooks[0].ExecuteCommands = []string{"make missing-textbook-command"}
	spec.Chapters[0].Notebooks[0].TeachingExamples = nil
	spec.Chapters[0].Notebooks[0].EvidencePaths = []string{"docs/missing-textbook-evidence.md"}
	spec.Chapters[0].Notebooks[0].ExpectedArtifacts = []string{
		"results/generated/textbook-companion/missing/missing.json",
		"results/generated/textbook-companion/missing/missing.md",
	}
	spec.Chapters[0].Notebooks[0].NegativeControls = nil
	report, err := BuildTextbookCompanionReport(spec, root)
	if err != nil {
		t.Fatal(err)
	}
	if report.OK {
		t.Fatalf("expected deficient textbook companion to fail: %#v", report)
	}
	for _, kind := range []string{
		"missing_required_chapter",
		"missing_notebook",
		"missing_executable_cell",
		"insufficient_teaching_examples",
		"missing_evidence",
		"missing_regenerated_artifact",
		"insufficient_generated_artifacts",
		"missing_negative_control",
	} {
		if !hasTextbookCountercase(report, kind) {
			t.Fatalf("expected %s counterexample, got %#v", kind, report.Counterexamples)
		}
	}
}

func TestReadTextbookCompanionSpecRejectsUnknownFields(t *testing.T) {
	_, err := ReadTextbookCompanionSpec(strings.NewReader(`{"version":"patchline.open-textbook-companion/v1","name":"x","criteria":{"required_chapters":["intro"],"min_chapters":1,"min_notebooks_per_chapter":1,"min_examples_per_notebook":1,"min_commands_per_notebook":1,"min_learning_objectives_per_example":1,"min_evidence_artifacts_per_notebook":1,"min_generated_artifacts_per_notebook":1,"require_executable_notebook":true,"require_reproducible_commands":true,"require_generated_artifacts":true,"require_negative_control":true},"chapters":[],"extra":true}`))
	if err == nil {
		t.Fatal("expected unknown field error")
	}
}

func TestWriteTextbookCompanionArtifactsIsDeterministic(t *testing.T) {
	root := textbookCompanionRoot(t)
	report, err := BuildTextbookCompanionReport(validTextbookCompanionSpec(), root)
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "textbook")
	if err := WriteTextbookCompanionArtifacts(out, report); err != nil {
		t.Fatal(err)
	}
	var reread TextbookCompanionReport
	file, err := os.Open(filepath.Join(out, "open-textbook-companion.json"))
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

func validTextbookCompanionSpec() TextbookCompanionSpec {
	return TextbookCompanionSpec{
		Version: TextbookCompanionSpecVersion,
		Name:    "Patchline open textbook companion",
		Claim:   "Patchline ships an open textbook companion whose executable notebooks regenerate every teaching example, hash the source evidence, verify generated reports, and include negative controls so broken notebooks fail deterministically.",
		Criteria: TextbookCompanionCriteria{
			RequiredChapters:                 []string{"classroom-labs", "reviewer-skills", "localized-lessons"},
			MinChapters:                      3,
			MinNotebooksPerChapter:           1,
			MinExamplesPerNotebook:           1,
			MinCommandsPerNotebook:           1,
			MinLearningObjectivesPerExample:  2,
			MinEvidenceArtifactsPerNotebook:  3,
			MinGeneratedArtifactsPerNotebook: 2,
			RequireExecutableNotebook:        true,
			RequireReproducibleCommands:      true,
			RequireGeneratedArtifacts:        true,
			RequireNegativeControl:           true,
		},
		Chapters: []TextbookChapter{
			textbookChapter("classroom-labs", "Classroom labs", "educator", "examples/textbook-companion/01-classroom-lab-kits.ipynb", "go run ./cmd/patchline classroom-lab-kits --spec examples/classroom-lab-kits.json --root . --out results/generated/textbook-companion/classroom-lab-kits --json", "docs/classroom-lab-kits.md", "examples/classroom-lab-kits.json", "results/generated/textbook-companion/classroom-lab-kits/classroom-lab-kits.json", "results/generated/textbook-companion/classroom-lab-kits/classroom-lab-kits.md"),
			textbookChapter("reviewer-skills", "Reviewer skills", "reviewer", "examples/textbook-companion/02-skills-taxonomy.ipynb", "go run ./cmd/patchline skills-taxonomy --spec examples/skills-taxonomy.json --root . --out results/generated/textbook-companion/skills-taxonomy --json", "docs/skills-taxonomy.md", "examples/skills-taxonomy.json", "results/generated/textbook-companion/skills-taxonomy/skills-taxonomy.json", "results/generated/textbook-companion/skills-taxonomy/skills-taxonomy.md"),
			textbookChapter("localized-lessons", "Localized lessons", "translator", "examples/textbook-companion/03-localized-teaching-examples.ipynb", "go run ./cmd/patchline localized-teaching-examples --spec examples/localized-teaching-examples.json --root . --out results/generated/textbook-companion/localized-teaching-examples --json", "docs/localized-teaching-examples.md", "examples/localized-teaching-examples.json", "results/generated/textbook-companion/localized-teaching-examples/localized-teaching-examples.json", "results/generated/textbook-companion/localized-teaching-examples/localized-teaching-examples.md"),
		},
	}
}

func textbookChapter(id, title, audience, notebookPath, command, docPath, specPath, jsonOut, mdOut string) TextbookChapter {
	return TextbookChapter{
		ID:       id,
		Title:    title,
		Audience: audience,
		Summary:  "Executable notebook chapter for " + title + ".",
		Concepts: []string{"regeneration", "evidence hashing"},
		Notebooks: []TextbookNotebook{{
			ID:              id + "-notebook",
			Title:           title + " notebook",
			Path:            notebookPath,
			Runtime:         "python3",
			ExecuteCommands: []string{command},
			TeachingExamples: []TextbookTeachingExample{{
				ID:                 id + "-example",
				Title:              title + " example",
				SourceCommand:      command,
				LearningObjectives: []string{"run the exact regeneration command", "inspect the generated evidence hashes"},
				EvidencePaths:      []string{docPath, specPath},
				ExpectedArtifacts:  []string{jsonOut, mdOut},
			}},
			EvidencePaths:     []string{docPath, specPath},
			ExpectedArtifacts: []string{jsonOut, mdOut},
			NegativeControls: []TextbookNegativeControl{{
				ID:                     "remove-command",
				Mutation:               "delete the regeneration command from the notebook",
				ExpectedCounterexample: "missing_executable_cell",
			}},
		}},
	}
}

func textbookCompanionRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for _, spec := range validTextbookCompanionSpec().Chapters {
		notebook := spec.Notebooks[0]
		writeTextbookFile(t, root, notebook.Path, textbookNotebookJSON(notebook.ExecuteCommands[0]))
		for _, path := range notebook.EvidencePaths {
			writeTextbookFile(t, root, path, "Evidence for "+path+".\n")
		}
		for _, path := range notebook.ExpectedArtifacts {
			writeTextbookFile(t, root, path, "Generated artifact for "+path+".\n")
		}
	}
	return root
}

func textbookNotebookJSON(command string) string {
	return `{
  "cells": [
    {
      "cell_type": "markdown",
      "metadata": {},
      "source": ["# Patchline textbook notebook\n"]
    },
    {
      "cell_type": "code",
      "execution_count": null,
      "metadata": {},
      "outputs": [],
      "source": ["import subprocess\n", "subprocess.run(\"` + command + `\", shell=True, check=True)\n"]
    }
  ],
  "metadata": {"kernelspec": {"display_name": "Python 3", "language": "python", "name": "python3"}},
  "nbformat": 4,
  "nbformat_minor": 5
}`
}

func writeTextbookFile(t *testing.T, root, relPath, contents string) {
	t.Helper()
	path := filepath.Join(root, relPath)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
}

func hasTextbookCountercase(report TextbookCompanionReport, kind string) bool {
	for _, counterexample := range report.Counterexamples {
		if counterexample.Kind == kind {
			return true
		}
	}
	return false
}
