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

const TextbookCompanionSpecVersion = "patchline.open-textbook-companion/v1"
const TextbookCompanionReportVersion = "patchline.open-textbook-companion-report/v1"

type TextbookCompanionSpec struct {
	Version  string                    `json:"version"`
	Name     string                    `json:"name"`
	Claim    string                    `json:"claim,omitempty"`
	Criteria TextbookCompanionCriteria `json:"criteria"`
	Chapters []TextbookChapter         `json:"chapters"`
}

type TextbookCompanionCriteria struct {
	RequiredChapters                 []string `json:"required_chapters"`
	MinChapters                      int      `json:"min_chapters"`
	MinNotebooksPerChapter           int      `json:"min_notebooks_per_chapter"`
	MinExamplesPerNotebook           int      `json:"min_examples_per_notebook"`
	MinCommandsPerNotebook           int      `json:"min_commands_per_notebook"`
	MinLearningObjectivesPerExample  int      `json:"min_learning_objectives_per_example"`
	MinEvidenceArtifactsPerNotebook  int      `json:"min_evidence_artifacts_per_notebook"`
	MinGeneratedArtifactsPerNotebook int      `json:"min_generated_artifacts_per_notebook"`
	RequireExecutableNotebook        bool     `json:"require_executable_notebook"`
	RequireReproducibleCommands      bool     `json:"require_reproducible_commands"`
	RequireGeneratedArtifacts        bool     `json:"require_generated_artifacts"`
	RequireNegativeControl           bool     `json:"require_negative_control"`
}

type TextbookChapter struct {
	ID        string             `json:"id"`
	Title     string             `json:"title"`
	Audience  string             `json:"audience"`
	Summary   string             `json:"summary"`
	Concepts  []string           `json:"concepts"`
	Notebooks []TextbookNotebook `json:"notebooks"`
}

type TextbookNotebook struct {
	ID                string                    `json:"id"`
	Title             string                    `json:"title"`
	Path              string                    `json:"path"`
	Runtime           string                    `json:"runtime"`
	ExecuteCommands   []string                  `json:"execute_commands"`
	TeachingExamples  []TextbookTeachingExample `json:"teaching_examples"`
	EvidencePaths     []string                  `json:"evidence_paths"`
	ExpectedArtifacts []string                  `json:"expected_artifacts"`
	NegativeControls  []TextbookNegativeControl `json:"negative_controls"`
}

type TextbookTeachingExample struct {
	ID                 string   `json:"id"`
	Title              string   `json:"title"`
	SourceCommand      string   `json:"source_command"`
	LearningObjectives []string `json:"learning_objectives"`
	EvidencePaths      []string `json:"evidence_paths"`
	ExpectedArtifacts  []string `json:"expected_artifacts"`
}

type TextbookNegativeControl struct {
	ID                     string `json:"id"`
	Mutation               string `json:"mutation"`
	ExpectedCounterexample string `json:"expected_counterexample"`
}

type TextbookCompanionReport struct {
	Version         string                         `json:"version"`
	Name            string                         `json:"name"`
	OK              bool                           `json:"ok"`
	Criteria        TextbookCompanionCriteria      `json:"criteria"`
	Summary         TextbookCompanionSummary       `json:"summary"`
	Chapters        []TextbookChapterReport        `json:"chapters"`
	Counterexamples []TextbookCompanionCountercase `json:"counterexamples,omitempty"`
	Hash            string                         `json:"hash"`
}

type TextbookCompanionSummary struct {
	Chapters            int `json:"chapters"`
	Notebooks           int `json:"notebooks"`
	ExecutableNotebooks int `json:"executable_notebooks"`
	TeachingExamples    int `json:"teaching_examples"`
	Commands            int `json:"commands"`
	EvidenceArtifacts   int `json:"evidence_artifacts"`
	GeneratedArtifacts  int `json:"generated_artifacts"`
	NegativeControls    int `json:"negative_controls"`
	Counterexamples     int `json:"counterexamples"`
}

type TextbookChapterReport struct {
	ID              string                   `json:"id"`
	Title           string                   `json:"title"`
	Audience        string                   `json:"audience"`
	Concepts        int                      `json:"concepts"`
	Notebooks       int                      `json:"notebooks"`
	NotebookReports []TextbookNotebookReport `json:"notebook_reports"`
}

type TextbookNotebookReport struct {
	ID                 string             `json:"id"`
	Title              string             `json:"title"`
	Path               string             `json:"path"`
	Runtime            string             `json:"runtime"`
	Executable         bool               `json:"executable"`
	CodeCells          int                `json:"code_cells"`
	Commands           int                `json:"commands"`
	TeachingExamples   int                `json:"teaching_examples"`
	Evidence           []ArtifactEvidence `json:"evidence"`
	GeneratedArtifacts []ArtifactEvidence `json:"generated_artifacts"`
	NegativeControls   int                `json:"negative_controls"`
}

type TextbookCompanionCountercase struct {
	ID      string   `json:"id"`
	Kind    string   `json:"kind"`
	Subject string   `json:"subject,omitempty"`
	Message string   `json:"message"`
	Witness []string `json:"witness,omitempty"`
}

type textbookNotebookDocument struct {
	Cells []textbookNotebookCell `json:"cells"`
}

type textbookNotebookCell struct {
	CellType string          `json:"cell_type"`
	Source   json.RawMessage `json:"source"`
}

func ReadTextbookCompanionSpec(reader io.Reader) (TextbookCompanionSpec, error) {
	decoder := json.NewDecoder(reader)
	decoder.DisallowUnknownFields()
	var spec TextbookCompanionSpec
	if err := decoder.Decode(&spec); err != nil {
		return TextbookCompanionSpec{}, err
	}
	if spec.Version != TextbookCompanionSpecVersion {
		return TextbookCompanionSpec{}, fmt.Errorf("open textbook companion spec version must be %s", TextbookCompanionSpecVersion)
	}
	return spec, nil
}

func BuildTextbookCompanionReport(spec TextbookCompanionSpec, root string) (TextbookCompanionReport, error) {
	if err := validateTextbookCompanionSpec(spec); err != nil {
		return TextbookCompanionReport{}, err
	}
	if root == "" {
		root = "."
	}
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return TextbookCompanionReport{}, err
	}
	criteria := normalizeTextbookCompanionCriteria(spec.Criteria)
	report := TextbookCompanionReport{
		Version:  TextbookCompanionReportVersion,
		Name:     spec.Name,
		OK:       true,
		Criteria: criteria,
	}
	if len(spec.Chapters) < criteria.MinChapters {
		report.Counterexamples = append(report.Counterexamples, TextbookCompanionCountercase{
			ID:      "criteria.min_chapters",
			Kind:    "insufficient_chapters",
			Message: fmt.Sprintf("chapters %d below required %d", len(spec.Chapters), criteria.MinChapters),
		})
	}

	chaptersCovered := map[string]struct{}{}
	for _, chapter := range sortedTextbookChapters(spec.Chapters) {
		chapterID := normalizeToken(chapter.ID)
		if chapterID != "" {
			chaptersCovered[chapterID] = struct{}{}
		}
		cr := TextbookChapterReport{
			ID:       chapter.ID,
			Title:    chapter.Title,
			Audience: normalizeToken(chapter.Audience),
			Concepts: len(normalizedStrings(chapter.Concepts)),
		}
		if len(chapter.Notebooks) < criteria.MinNotebooksPerChapter {
			report.Counterexamples = append(report.Counterexamples, TextbookCompanionCountercase{
				ID:      "chapter." + stableID(chapter.ID, "notebooks") + ".insufficient",
				Kind:    "insufficient_chapter_notebooks",
				Subject: chapter.ID,
				Message: fmt.Sprintf("chapter has %d notebooks below required %d", len(chapter.Notebooks), criteria.MinNotebooksPerChapter),
			})
		}
		for _, notebook := range sortedTextbookNotebooks(chapter.Notebooks) {
			nr, counterexamples := buildTextbookNotebookReport(rootAbs, chapter, notebook, criteria)
			cr.NotebookReports = append(cr.NotebookReports, nr)
			report.Counterexamples = append(report.Counterexamples, counterexamples...)
			report.Summary.Notebooks++
			report.Summary.TeachingExamples += nr.TeachingExamples
			report.Summary.Commands += nr.Commands
			report.Summary.EvidenceArtifacts += len(nr.Evidence)
			report.Summary.GeneratedArtifacts += len(nr.GeneratedArtifacts)
			report.Summary.NegativeControls += nr.NegativeControls
			if nr.Executable {
				report.Summary.ExecutableNotebooks++
			}
		}
		cr.Notebooks = len(cr.NotebookReports)
		report.Chapters = append(report.Chapters, cr)
	}
	for _, chapter := range criteria.RequiredChapters {
		if _, ok := chaptersCovered[chapter]; !ok {
			report.Counterexamples = append(report.Counterexamples, TextbookCompanionCountercase{
				ID:      "chapter." + stableID(chapter, "required") + ".missing",
				Kind:    "missing_required_chapter",
				Subject: chapter,
				Message: "required textbook chapter is not present",
				Witness: []string{chapter},
			})
		}
	}
	sortTextbookCountercases(report.Counterexamples)
	report.Summary.Chapters = len(report.Chapters)
	report.Summary.Counterexamples = len(report.Counterexamples)
	report.OK = len(report.Counterexamples) == 0
	report.Hash = textbookCompanionReportHash(report)
	return report, nil
}

func WriteTextbookCompanionArtifacts(outDir string, report TextbookCompanionReport) error {
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}
	file, err := os.Create(filepath.Join(outDir, "open-textbook-companion.json"))
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
	return os.WriteFile(filepath.Join(outDir, "open-textbook-companion.md"), []byte(RenderTextbookCompanionMarkdown(report)), 0o644)
}

func RenderTextbookCompanionMarkdown(report TextbookCompanionReport) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Open textbook companion\n\n")
	fmt.Fprintf(&b, "Patchline validates an open textbook companion as executable notebooks, not static prose: each chapter maps to teaching examples, exact regeneration commands, notebook code cells, hashed evidence, generated artifacts, and negative controls.\n\n")
	fmt.Fprintf(&b, "## Summary\n\n")
	fmt.Fprintf(&b, "| Metric | Value |\n| --- | ---: |\n")
	fmt.Fprintf(&b, "| OK | `%t` |\n", report.OK)
	fmt.Fprintf(&b, "| Chapters | %d |\n", report.Summary.Chapters)
	fmt.Fprintf(&b, "| Notebooks | %d |\n", report.Summary.Notebooks)
	fmt.Fprintf(&b, "| Executable notebooks | %d |\n", report.Summary.ExecutableNotebooks)
	fmt.Fprintf(&b, "| Teaching examples | %d |\n", report.Summary.TeachingExamples)
	fmt.Fprintf(&b, "| Commands | %d |\n", report.Summary.Commands)
	fmt.Fprintf(&b, "| Evidence artifacts | %d |\n", report.Summary.EvidenceArtifacts)
	fmt.Fprintf(&b, "| Generated artifacts | %d |\n", report.Summary.GeneratedArtifacts)
	fmt.Fprintf(&b, "| Negative controls | %d |\n", report.Summary.NegativeControls)
	fmt.Fprintf(&b, "| Counterexamples | %d |\n\n", report.Summary.Counterexamples)
	fmt.Fprintf(&b, "## Notebook regeneration map\n\n")
	fmt.Fprintf(&b, "| Chapter | Notebook | Runtime | Code cells | Commands | Examples | Generated hashes |\n| --- | --- | --- | ---: | ---: | ---: | --- |\n")
	for _, chapter := range report.Chapters {
		for _, notebook := range chapter.NotebookReports {
			hashes := make([]string, 0, len(notebook.GeneratedArtifacts))
			for _, artifact := range notebook.GeneratedArtifacts {
				hash := artifact.SHA256
				if len(hash) > 16 {
					hash = hash[:16]
				}
				hashes = append(hashes, artifact.Path+":"+hash)
			}
			fmt.Fprintf(&b, "| `%s` | `%s` | `%s` | %d | %d | %d | %s |\n",
				escapeTable(chapter.ID),
				escapeTable(notebook.Path),
				escapeTable(notebook.Runtime),
				notebook.CodeCells,
				notebook.Commands,
				notebook.TeachingExamples,
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

func buildTextbookNotebookReport(root string, chapter TextbookChapter, notebook TextbookNotebook, criteria TextbookCompanionCriteria) (TextbookNotebookReport, []TextbookCompanionCountercase) {
	subject := chapter.ID + "/" + notebook.ID
	staticPaths := []string{notebook.Path}
	staticPaths = append(staticPaths, notebook.EvidencePaths...)
	generatedPaths := append([]string{}, notebook.ExpectedArtifacts...)
	for _, example := range notebook.TeachingExamples {
		staticPaths = append(staticPaths, example.EvidencePaths...)
		generatedPaths = append(generatedPaths, example.ExpectedArtifacts...)
	}
	evidence, counterexamples := collectTextbookArtifacts(root, uniqueSorted(staticPaths), subject, "evidence", "missing_evidence", "empty_evidence", "invalid_evidence_path", "textbook evidence could not be read")
	generated, generatedCounterexamples := collectTextbookArtifacts(root, uniqueSorted(generatedPaths), subject, "generated", "missing_regenerated_artifact", "empty_regenerated_artifact", "invalid_generated_artifact_path", "regenerated textbook artifact could not be read")
	counterexamples = append(counterexamples, generatedCounterexamples...)

	document, codeCells, notebookCounterexamples := readTextbookNotebook(root, notebook.Path, subject)
	counterexamples = append(counterexamples, notebookCounterexamples...)
	commands := uniqueSorted(notebook.ExecuteCommands)
	executable := document != nil && codeCells > 0 && len(commands) >= criteria.MinCommandsPerNotebook
	for _, command := range commands {
		if document == nil || !textbookNotebookHasCommand(*document, command) {
			executable = false
			counterexamples = append(counterexamples, TextbookCompanionCountercase{
				ID:      "notebook." + stableID(subject, command, "code-cell") + ".missing",
				Kind:    "missing_executable_cell",
				Subject: subject,
				Message: "notebook code cells do not contain the exact regeneration command",
				Witness: []string{command},
			})
		}
	}
	if criteria.RequireExecutableNotebook && codeCells == 0 {
		executable = false
		counterexamples = append(counterexamples, TextbookCompanionCountercase{
			ID:      "notebook." + stableID(subject, "code-cells") + ".missing",
			Kind:    "missing_executable_cell",
			Subject: subject,
			Message: "notebook does not contain an executable code cell",
		})
	}
	if len(commands) < criteria.MinCommandsPerNotebook {
		executable = false
		counterexamples = append(counterexamples, TextbookCompanionCountercase{
			ID:      "notebook." + stableID(subject, "commands") + ".insufficient",
			Kind:    "insufficient_commands",
			Subject: subject,
			Message: fmt.Sprintf("notebook has %d commands below required %d", len(commands), criteria.MinCommandsPerNotebook),
		})
	}
	if len(notebook.TeachingExamples) < criteria.MinExamplesPerNotebook {
		counterexamples = append(counterexamples, TextbookCompanionCountercase{
			ID:      "notebook." + stableID(subject, "examples") + ".insufficient",
			Kind:    "insufficient_teaching_examples",
			Subject: subject,
			Message: fmt.Sprintf("notebook has %d teaching examples below required %d", len(notebook.TeachingExamples), criteria.MinExamplesPerNotebook),
		})
	}
	for _, example := range sortedTextbookExamples(notebook.TeachingExamples) {
		counterexamples = append(counterexamples, validateTextbookExample(subject, commands, example, criteria)...)
	}
	if len(evidence) < criteria.MinEvidenceArtifactsPerNotebook {
		counterexamples = append(counterexamples, TextbookCompanionCountercase{
			ID:      "notebook." + stableID(subject, "evidence") + ".insufficient",
			Kind:    "insufficient_evidence_artifacts",
			Subject: subject,
			Message: fmt.Sprintf("notebook has %d evidence artifacts below required %d", len(evidence), criteria.MinEvidenceArtifactsPerNotebook),
		})
	}
	if len(generatedPaths) == 0 && criteria.RequireGeneratedArtifacts {
		counterexamples = append(counterexamples, TextbookCompanionCountercase{
			ID:      "notebook." + stableID(subject, "generated") + ".missing",
			Kind:    "missing_regeneration_target",
			Subject: subject,
			Message: "notebook does not list generated artifacts that prove example regeneration",
		})
	}
	if len(generated) < criteria.MinGeneratedArtifactsPerNotebook {
		counterexamples = append(counterexamples, TextbookCompanionCountercase{
			ID:      "notebook." + stableID(subject, "generated") + ".insufficient",
			Kind:    "insufficient_generated_artifacts",
			Subject: subject,
			Message: fmt.Sprintf("notebook has %d regenerated artifacts below required %d", len(generated), criteria.MinGeneratedArtifactsPerNotebook),
		})
	}
	if criteria.RequireNegativeControl && len(notebook.NegativeControls) == 0 {
		counterexamples = append(counterexamples, TextbookCompanionCountercase{
			ID:      "notebook." + stableID(subject, "negative-control") + ".missing",
			Kind:    "missing_negative_control",
			Subject: subject,
			Message: "notebook does not include a mutation that proves deficient textbook material fails",
		})
	}
	for _, control := range notebook.NegativeControls {
		if strings.TrimSpace(control.ID) == "" || strings.TrimSpace(control.Mutation) == "" || strings.TrimSpace(control.ExpectedCounterexample) == "" {
			counterexamples = append(counterexamples, TextbookCompanionCountercase{
				ID:      "notebook." + stableID(subject, control.ID, "negative-control") + ".incomplete",
				Kind:    "incomplete_negative_control",
				Subject: subject,
				Message: "negative control must include id, mutation, and expected counterexample",
			})
		}
	}
	sortTextbookCountercases(counterexamples)
	return TextbookNotebookReport{
		ID:                 notebook.ID,
		Title:              notebook.Title,
		Path:               filepath.ToSlash(filepath.Clean(notebook.Path)),
		Runtime:            normalizeToken(notebook.Runtime),
		Executable:         executable,
		CodeCells:          codeCells,
		Commands:           len(commands),
		TeachingExamples:   len(notebook.TeachingExamples),
		Evidence:           evidence,
		GeneratedArtifacts: generated,
		NegativeControls:   len(notebook.NegativeControls),
	}, counterexamples
}

func validateTextbookExample(subject string, commands []string, example TextbookTeachingExample, criteria TextbookCompanionCriteria) []TextbookCompanionCountercase {
	var counterexamples []TextbookCompanionCountercase
	exampleSubject := subject + "/" + example.ID
	if criteria.RequireReproducibleCommands && !containsCommand(commands, example.SourceCommand) {
		counterexamples = append(counterexamples, TextbookCompanionCountercase{
			ID:      "example." + stableID(exampleSubject, "command") + ".missing",
			Kind:    "missing_reproducible_command",
			Subject: exampleSubject,
			Message: "teaching example source command is not listed as an executable notebook command",
			Witness: []string{example.SourceCommand},
		})
	}
	if len(example.LearningObjectives) < criteria.MinLearningObjectivesPerExample {
		counterexamples = append(counterexamples, TextbookCompanionCountercase{
			ID:      "example." + stableID(exampleSubject, "objectives") + ".insufficient",
			Kind:    "insufficient_learning_objectives",
			Subject: exampleSubject,
			Message: fmt.Sprintf("teaching example has %d learning objectives below required %d", len(example.LearningObjectives), criteria.MinLearningObjectivesPerExample),
		})
	}
	return counterexamples
}

func validateTextbookCompanionSpec(spec TextbookCompanionSpec) error {
	if strings.TrimSpace(spec.Name) == "" {
		return fmt.Errorf("open textbook companion name is required")
	}
	criteria := spec.Criteria
	if len(criteria.RequiredChapters) == 0 {
		return fmt.Errorf("criteria.required_chapters is required")
	}
	if criteria.MinChapters <= 0 || criteria.MinNotebooksPerChapter <= 0 || criteria.MinExamplesPerNotebook <= 0 || criteria.MinCommandsPerNotebook <= 0 || criteria.MinLearningObjectivesPerExample <= 0 || criteria.MinEvidenceArtifactsPerNotebook <= 0 || criteria.MinGeneratedArtifactsPerNotebook <= 0 {
		return fmt.Errorf("textbook companion minimum criteria must be positive")
	}
	chapterIDs := map[string]struct{}{}
	for _, chapter := range spec.Chapters {
		if strings.TrimSpace(chapter.ID) == "" {
			return fmt.Errorf("textbook chapter id is required")
		}
		if _, ok := chapterIDs[chapter.ID]; ok {
			return fmt.Errorf("duplicate textbook chapter id %q", chapter.ID)
		}
		chapterIDs[chapter.ID] = struct{}{}
		if strings.TrimSpace(chapter.Title) == "" || strings.TrimSpace(chapter.Audience) == "" || strings.TrimSpace(chapter.Summary) == "" {
			return fmt.Errorf("textbook chapter %q requires title, audience, and summary", chapter.ID)
		}
		notebookIDs := map[string]struct{}{}
		for _, notebook := range chapter.Notebooks {
			if strings.TrimSpace(notebook.ID) == "" {
				return fmt.Errorf("textbook chapter %q notebook id is required", chapter.ID)
			}
			if _, ok := notebookIDs[notebook.ID]; ok {
				return fmt.Errorf("textbook chapter %q contains duplicate notebook id %q", chapter.ID, notebook.ID)
			}
			notebookIDs[notebook.ID] = struct{}{}
			if strings.TrimSpace(notebook.Title) == "" || strings.TrimSpace(notebook.Path) == "" || strings.TrimSpace(notebook.Runtime) == "" {
				return fmt.Errorf("textbook notebook %q requires title, path, and runtime", notebook.ID)
			}
			for _, path := range append(append([]string{notebook.Path}, notebook.EvidencePaths...), notebook.ExpectedArtifacts...) {
				if err := validateRelativePath(path); err != nil {
					return fmt.Errorf("textbook notebook %q path: %w", notebook.ID, err)
				}
			}
			exampleIDs := map[string]struct{}{}
			for _, example := range notebook.TeachingExamples {
				if strings.TrimSpace(example.ID) == "" {
					return fmt.Errorf("textbook notebook %q teaching example id is required", notebook.ID)
				}
				if _, ok := exampleIDs[example.ID]; ok {
					return fmt.Errorf("textbook notebook %q contains duplicate teaching example id %q", notebook.ID, example.ID)
				}
				exampleIDs[example.ID] = struct{}{}
				if strings.TrimSpace(example.Title) == "" || strings.TrimSpace(example.SourceCommand) == "" {
					return fmt.Errorf("textbook example %q requires title and source_command", example.ID)
				}
				for _, path := range append(append([]string{}, example.EvidencePaths...), example.ExpectedArtifacts...) {
					if err := validateRelativePath(path); err != nil {
						return fmt.Errorf("textbook example %q path: %w", example.ID, err)
					}
				}
			}
		}
	}
	return nil
}

func collectTextbookArtifacts(root string, paths []string, subject, idPart, missingKind, emptyKind, invalidKind, missingMessage string) ([]ArtifactEvidence, []TextbookCompanionCountercase) {
	var artifacts []ArtifactEvidence
	var counterexamples []TextbookCompanionCountercase
	for _, relPath := range uniqueSorted(paths) {
		fullPath, err := safeJoin(root, relPath)
		if err != nil {
			counterexamples = append(counterexamples, TextbookCompanionCountercase{
				ID:      "textbook." + stableID(subject, relPath, idPart, "path") + ".invalid",
				Kind:    invalidKind,
				Subject: subject,
				Message: err.Error(),
				Witness: []string{relPath},
			})
			continue
		}
		data, err := os.ReadFile(fullPath)
		if err != nil {
			counterexamples = append(counterexamples, TextbookCompanionCountercase{
				ID:      "textbook." + stableID(subject, relPath, idPart) + ".missing",
				Kind:    missingKind,
				Subject: subject,
				Message: missingMessage,
				Witness: []string{relPath},
			})
			continue
		}
		if len(data) == 0 {
			counterexamples = append(counterexamples, TextbookCompanionCountercase{
				ID:      "textbook." + stableID(subject, relPath, idPart) + ".empty",
				Kind:    emptyKind,
				Subject: subject,
				Message: "textbook artifact is empty",
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

func readTextbookNotebook(root, relPath, subject string) (*textbookNotebookDocument, int, []TextbookCompanionCountercase) {
	fullPath, err := safeJoin(root, relPath)
	if err != nil {
		return nil, 0, []TextbookCompanionCountercase{{
			ID:      "notebook." + stableID(subject, relPath, "path") + ".invalid",
			Kind:    "invalid_notebook_path",
			Subject: subject,
			Message: err.Error(),
			Witness: []string{relPath},
		}}
	}
	data, err := os.ReadFile(fullPath)
	if err != nil {
		return nil, 0, []TextbookCompanionCountercase{{
			ID:      "notebook." + stableID(subject, relPath) + ".missing",
			Kind:    "missing_notebook",
			Subject: subject,
			Message: "notebook file could not be read",
			Witness: []string{relPath},
		}}
	}
	var document textbookNotebookDocument
	if err := json.Unmarshal(data, &document); err != nil {
		return nil, 0, []TextbookCompanionCountercase{{
			ID:      "notebook." + stableID(subject, relPath) + ".invalid",
			Kind:    "invalid_notebook",
			Subject: subject,
			Message: "notebook is not valid ipynb JSON: " + err.Error(),
			Witness: []string{relPath},
		}}
	}
	codeCells := 0
	for _, cell := range document.Cells {
		if cell.CellType == "code" && strings.TrimSpace(notebookCellSource(cell.Source)) != "" {
			codeCells++
		}
	}
	return &document, codeCells, nil
}

func textbookNotebookHasCommand(document textbookNotebookDocument, command string) bool {
	command = normalizeCommand(command)
	for _, cell := range document.Cells {
		if cell.CellType != "code" {
			continue
		}
		if strings.Contains(normalizeCommand(notebookCellSource(cell.Source)), command) {
			return true
		}
	}
	return false
}

func notebookCellSource(raw json.RawMessage) string {
	var lines []string
	if err := json.Unmarshal(raw, &lines); err == nil {
		return strings.Join(lines, "")
	}
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		return text
	}
	return ""
}

func normalizeTextbookCompanionCriteria(criteria TextbookCompanionCriteria) TextbookCompanionCriteria {
	criteria.RequiredChapters = sortedStrings(normalizedStrings(criteria.RequiredChapters))
	return criteria
}

func sortedTextbookChapters(chapters []TextbookChapter) []TextbookChapter {
	out := append([]TextbookChapter(nil), chapters...)
	sort.SliceStable(out, func(i, j int) bool { return normalizeToken(out[i].ID) < normalizeToken(out[j].ID) })
	return out
}

func sortedTextbookNotebooks(notebooks []TextbookNotebook) []TextbookNotebook {
	out := append([]TextbookNotebook(nil), notebooks...)
	sort.SliceStable(out, func(i, j int) bool { return normalizeToken(out[i].ID) < normalizeToken(out[j].ID) })
	return out
}

func sortedTextbookExamples(examples []TextbookTeachingExample) []TextbookTeachingExample {
	out := append([]TextbookTeachingExample(nil), examples...)
	sort.SliceStable(out, func(i, j int) bool { return normalizeToken(out[i].ID) < normalizeToken(out[j].ID) })
	return out
}

func sortTextbookCountercases(counterexamples []TextbookCompanionCountercase) {
	sort.SliceStable(counterexamples, func(i, j int) bool { return counterexamples[i].ID < counterexamples[j].ID })
}

func textbookCompanionReportHash(report TextbookCompanionReport) string {
	report.Hash = ""
	return canonical.Hash(report)
}
