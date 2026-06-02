package goldenfixture

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/thehalleyyoung/patchline/internal/canonical"
	"github.com/thehalleyyoung/patchline/internal/intake"
	"github.com/thehalleyyoung/patchline/internal/project"
)

const Version = "patchline.golden-fixture/v1"

type Options struct {
	Path            string
	GitHub          string
	Ref             string
	Subpath         string
	DownloadDir     string
	OutDir          string
	PackageName     string
	TestName        string
	MaxFiles        int
	MaxFileBytes    int64
	MaxTotalBytes   int64
	MinRankedRisks  int
	KeepFetchOutput bool
}

type Report struct {
	Version       string            `json:"version"`
	ID            string            `json:"id"`
	Input         string            `json:"input"`
	Subpath       string            `json:"subpath,omitempty"`
	Source        project.Source    `json:"source,omitempty"`
	Summary       Summary           `json:"summary"`
	SelectedFiles []SelectedFile    `json:"selected_files"`
	Expectations  Expectations      `json:"expectations"`
	Outputs       map[string]string `json:"outputs"`
	Hash          string            `json:"hash"`
	Markdown      string            `json:"markdown,omitempty"`
}

type Summary struct {
	OriginalFilesScanned int   `json:"original_files_scanned"`
	OriginalRankedRisks  int   `json:"original_ranked_risks"`
	SelectedFiles        int   `json:"selected_files"`
	SelectedBytes        int64 `json:"selected_bytes"`
	ReductionPercent     int   `json:"reduction_percent"`
	GeneratedTestBytes   int64 `json:"generated_test_bytes"`
}

type SelectedFile struct {
	Path        string `json:"path"`
	Bytes       int64  `json:"bytes"`
	ContentHash string `json:"content_hash"`
	Reason      string `json:"reason"`
	Content     string `json:"-"`
}

type Expectations struct {
	FilesScanned        int `json:"files_scanned"`
	Facts               int `json:"facts"`
	IntakeSQLFindings   int `json:"intake_sql_findings"`
	SourceSQLFindings   int `json:"source_sql_observations"`
	ProblemCandidates   int `json:"problem_candidates"`
	CauseCandidates     int `json:"cause_candidates"`
	RepairCandidates    int `json:"repair_candidates"`
	LinkedCandidates    int `json:"linked_candidates"`
	RankedRisks         int `json:"ranked_risks"`
	RankingExplanations int `json:"ranking_explanations"`
	ProvenanceSlices    int `json:"provenance_slices"`
	PolicyChecks        int `json:"policy_checks"`
	RepairProofs        int `json:"repair_proof_summaries"`
}

func Generate(ctx context.Context, opts Options) (Report, error) {
	opts = normalizeOptions(opts)
	if opts.OutDir == "" {
		return Report{}, fmt.Errorf("golden fixture generation requires --out")
	}
	root, source, err := resolveSource(ctx, opts)
	if err != nil {
		return Report{}, err
	}
	originalInv, err := project.InventoryPath(project.InventoryOptions{Path: root})
	if err != nil {
		return Report{}, err
	}
	originalIntake, err := intake.Run(ctx, intake.Options{Path: root})
	if err != nil {
		return Report{}, err
	}
	originalBaseline := project.Baseline(originalInv, originalInv.Facts, originalIntake)
	selected, err := selectFiles(root, originalBaseline, opts)
	if err != nil {
		return Report{}, err
	}
	miniRoot, err := os.MkdirTemp("", "patchline-golden-fixture-*")
	if err != nil {
		return Report{}, err
	}
	defer os.RemoveAll(miniRoot)
	for _, file := range selected {
		if err := writeFixtureFile(miniRoot, file.Path, file.Content); err != nil {
			return Report{}, err
		}
	}
	miniInv, err := project.InventoryPath(project.InventoryOptions{Path: miniRoot})
	if err != nil {
		return Report{}, err
	}
	miniIntake, err := intake.Run(ctx, intake.Options{Path: miniRoot})
	if err != nil {
		return Report{}, err
	}
	miniBaseline := project.Baseline(miniInv, miniInv.Facts, miniIntake)
	if miniBaseline.Summary.RankedRisks < opts.MinRankedRisks {
		return Report{}, fmt.Errorf("minimal fixture produced %d ranked risks, below required %d", miniBaseline.Summary.RankedRisks, opts.MinRankedRisks)
	}

	if err := os.MkdirAll(opts.OutDir, 0o755); err != nil {
		return Report{}, err
	}
	testPath := filepath.Join(opts.OutDir, "generated_golden_test.go")
	testContent := renderGoTest(opts, selected, expectationsFrom(miniInv, miniIntake, miniBaseline))
	if err := os.WriteFile(testPath, []byte(testContent), 0o644); err != nil {
		return Report{}, err
	}
	modulePath := filepath.Join(opts.OutDir, "go.mod")
	moduleContent, err := renderGoMod(fixtureID(opts))
	if err != nil {
		return Report{}, err
	}
	if err := os.WriteFile(modulePath, []byte(moduleContent), 0o644); err != nil {
		return Report{}, err
	}
	stat, err := os.Stat(testPath)
	if err != nil {
		return Report{}, err
	}

	totalBytes := int64(0)
	for _, file := range selected {
		totalBytes += file.Bytes
	}
	report := Report{
		Version:       Version,
		ID:            fixtureID(opts),
		Input:         sourceInput(opts, root),
		Subpath:       opts.Subpath,
		Source:        source,
		SelectedFiles: stripContents(selected),
		Expectations:  expectationsFrom(miniInv, miniIntake, miniBaseline),
		Outputs:       map[string]string{"test": testPath, "go_mod": modulePath},
		Summary: Summary{
			OriginalFilesScanned: originalInv.FilesScanned,
			OriginalRankedRisks:  originalBaseline.Summary.RankedRisks,
			SelectedFiles:        len(selected),
			SelectedBytes:        totalBytes,
			ReductionPercent:     reductionPercent(originalInv.FilesScanned, len(selected)),
			GeneratedTestBytes:   stat.Size(),
		},
	}
	report.Hash = canonical.Hash(struct {
		Version      string         `json:"version"`
		ID           string         `json:"id"`
		Input        string         `json:"input"`
		Subpath      string         `json:"subpath,omitempty"`
		Summary      Summary        `json:"summary"`
		Selected     []SelectedFile `json:"selected_files"`
		Expectations Expectations   `json:"expectations"`
	}{report.Version, report.ID, report.Input, report.Subpath, report.Summary, report.SelectedFiles, report.Expectations})
	report.Markdown = renderMarkdown(report)
	if err := writeReport(opts.OutDir, report); err != nil {
		return Report{}, err
	}
	return report, nil
}

func normalizeOptions(opts Options) Options {
	if opts.PackageName == "" {
		opts.PackageName = "goldenfixture"
	}
	if opts.MaxFiles <= 0 {
		opts.MaxFiles = 3
	}
	if opts.MaxFileBytes <= 0 {
		opts.MaxFileBytes = 24 * 1024
	}
	if opts.MaxTotalBytes <= 0 {
		opts.MaxTotalBytes = 48 * 1024
	}
	if opts.MinRankedRisks <= 0 {
		opts.MinRankedRisks = 1
	}
	return opts
}

func resolveSource(ctx context.Context, opts Options) (string, project.Source, error) {
	if opts.GitHub != "" {
		fetchOut := filepath.Join(opts.OutDir, "fetch")
		fetched, err := project.Fetch(ctx, project.FetchOptions{Input: opts.GitHub, Ref: opts.Ref, Subpath: opts.Subpath, OutDir: fetchOut, DownloadDir: opts.DownloadDir})
		if err != nil {
			return "", project.Source{}, err
		}
		return fetched.Source.ScannedRoot, fetched.Source, nil
	}
	root := opts.Path
	if root == "" {
		root = "."
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return "", project.Source{}, err
	}
	return abs, project.Source{Version: project.Version, Mode: "local", Input: root, Subpath: opts.Subpath, ScannedRoot: filepath.ToSlash(abs)}, nil
}

func selectFiles(root string, baseline project.BaselineReport, opts Options) ([]SelectedFile, error) {
	seen := map[string]bool{}
	var candidates []SelectedFile
	for _, risk := range baseline.Risks {
		if risk.Path == "" || seen[risk.Path] {
			continue
		}
		seen[risk.Path] = true
		file, ok, err := loadCandidate(root, risk.Path, "top-ranked risk "+risk.ID, opts.MaxFileBytes)
		if err != nil {
			return nil, err
		}
		if ok {
			candidates = append(candidates, file)
		}
		if len(candidates) >= opts.MaxFiles {
			break
		}
	}
	if len(candidates) < opts.MaxFiles {
		extras, err := sourceCandidates(root, seen, opts.MaxFileBytes)
		if err != nil {
			return nil, err
		}
		for _, file := range extras {
			candidates = append(candidates, file)
			if len(candidates) >= opts.MaxFiles {
				break
			}
		}
	}
	var selected []SelectedFile
	total := int64(0)
	for _, file := range candidates {
		if total+file.Bytes > opts.MaxTotalBytes && len(selected) > 0 {
			continue
		}
		total += file.Bytes
		selected = append(selected, file)
		if len(selected) >= opts.MaxFiles {
			break
		}
	}
	if len(selected) == 0 {
		return nil, fmt.Errorf("no eligible small source files found under %s", root)
	}
	sort.Slice(selected, func(i, j int) bool { return selected[i].Path < selected[j].Path })
	return selected, nil
}

func sourceCandidates(root string, seen map[string]bool, maxBytes int64) ([]SelectedFile, error) {
	var out []SelectedFile
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			name := entry.Name()
			if strings.HasPrefix(name, ".") || name == "node_modules" || name == "vendor" {
				return filepath.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if seen[rel] || !eligibleExtension(rel) {
			return nil
		}
		file, ok, err := loadCandidate(root, rel, "small analyzer input", maxBytes)
		if err != nil || !ok {
			return err
		}
		out = append(out, file)
		return nil
	})
	sort.Slice(out, func(i, j int) bool {
		if extensionPriority(out[i].Path) == extensionPriority(out[j].Path) {
			return out[i].Path < out[j].Path
		}
		return extensionPriority(out[i].Path) < extensionPriority(out[j].Path)
	})
	return out, err
}

func loadCandidate(root, rel, reason string, maxBytes int64) (SelectedFile, bool, error) {
	clean := filepath.Clean(filepath.FromSlash(rel))
	if strings.HasPrefix(clean, "..") || filepath.IsAbs(clean) {
		return SelectedFile{}, false, nil
	}
	path := filepath.Join(root, clean)
	stat, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return SelectedFile{}, false, nil
		}
		return SelectedFile{}, false, err
	}
	if stat.IsDir() || stat.Size() == 0 || stat.Size() > maxBytes || !eligibleExtension(rel) {
		return SelectedFile{}, false, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return SelectedFile{}, false, err
	}
	if !utf8.Valid(data) {
		return SelectedFile{}, false, nil
	}
	content := strings.ReplaceAll(string(data), "\r\n", "\n")
	return SelectedFile{Path: filepath.ToSlash(clean), Bytes: int64(len(content)), ContentHash: canonical.Hash(content), Reason: reason, Content: content}, true, nil
}

func eligibleExtension(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".sql", ".rb", ".py", ".go", ".js", ".ts", ".yml", ".yaml", ".json":
		return true
	default:
		return false
	}
}

func extensionPriority(path string) int {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".sql":
		return 0
	case ".rb", ".py", ".go":
		return 1
	case ".js", ".ts":
		return 2
	default:
		return 3
	}
}

func expectationsFrom(inv project.Inventory, intakeReport intake.Report, baseline project.BaselineReport) Expectations {
	return Expectations{
		FilesScanned:        inv.FilesScanned,
		Facts:               len(inv.Facts),
		IntakeSQLFindings:   len(intakeReport.SQL),
		SourceSQLFindings:   intakeReport.Summary.SourceSQLObservations,
		ProblemCandidates:   intakeReport.Summary.ProblemCandidates,
		CauseCandidates:     intakeReport.Summary.CauseCandidates,
		RepairCandidates:    intakeReport.Summary.RepairCandidates,
		LinkedCandidates:    intakeReport.Summary.LinkedCandidates,
		RankedRisks:         baseline.Summary.RankedRisks,
		RankingExplanations: baseline.Summary.RankingExplanations,
		ProvenanceSlices:    baseline.Summary.ProvenanceSlices,
		PolicyChecks:        baseline.Summary.PolicyChecks,
		RepairProofs:        baseline.Summary.RepairProofs,
	}
}

func renderGoTest(opts Options, files []SelectedFile, expected Expectations) string {
	testName := opts.TestName
	if testName == "" {
		testName = "TestGoldenFixture" + exportedName(fixtureID(opts))
	}
	var b strings.Builder
	fmt.Fprintf(&b, "package %s\n\n", sanitizePackage(opts.PackageName))
	fmt.Fprintf(&b, "import (\n\t\"context\"\n\t\"os\"\n\t\"path/filepath\"\n\t\"testing\"\n\n\t\"github.com/thehalleyyoung/patchline/internal/intake\"\n\t\"github.com/thehalleyyoung/patchline/internal/project\"\n)\n\n")
	fmt.Fprintf(&b, "func %s(t *testing.T) {\n", testName)
	fmt.Fprintf(&b, "\troot := t.TempDir()\n")
	for _, file := range files {
		fmt.Fprintf(&b, "\twriteGoldenFile(t, root, %s, %s)\n", strconv.Quote(file.Path), strconv.Quote(file.Content))
	}
	fmt.Fprintf(&b, "\tinv, err := project.InventoryPath(project.InventoryOptions{Path: root})\n\tif err != nil {\n\t\tt.Fatal(err)\n\t}\n")
	fmt.Fprintf(&b, "\tintakeReport, err := intake.Run(context.Background(), intake.Options{Path: root})\n\tif err != nil {\n\t\tt.Fatal(err)\n\t}\n")
	fmt.Fprintf(&b, "\tbaseline := project.Baseline(inv, inv.Facts, intakeReport)\n")
	fmt.Fprintf(&b, "\tassertEqual(t, \"files scanned\", inv.FilesScanned, %d)\n", expected.FilesScanned)
	fmt.Fprintf(&b, "\tassertEqual(t, \"facts\", len(inv.Facts), %d)\n", expected.Facts)
	fmt.Fprintf(&b, "\tassertEqual(t, \"intake SQL findings\", len(intakeReport.SQL), %d)\n", expected.IntakeSQLFindings)
	fmt.Fprintf(&b, "\tassertEqual(t, \"source SQL observations\", intakeReport.Summary.SourceSQLObservations, %d)\n", expected.SourceSQLFindings)
	fmt.Fprintf(&b, "\tassertEqual(t, \"problem candidates\", intakeReport.Summary.ProblemCandidates, %d)\n", expected.ProblemCandidates)
	fmt.Fprintf(&b, "\tassertEqual(t, \"cause candidates\", intakeReport.Summary.CauseCandidates, %d)\n", expected.CauseCandidates)
	fmt.Fprintf(&b, "\tassertEqual(t, \"repair candidates\", intakeReport.Summary.RepairCandidates, %d)\n", expected.RepairCandidates)
	fmt.Fprintf(&b, "\tassertEqual(t, \"linked candidates\", intakeReport.Summary.LinkedCandidates, %d)\n", expected.LinkedCandidates)
	fmt.Fprintf(&b, "\tassertEqual(t, \"ranked risks\", baseline.Summary.RankedRisks, %d)\n", expected.RankedRisks)
	fmt.Fprintf(&b, "\tassertEqual(t, \"ranking explanations\", baseline.Summary.RankingExplanations, %d)\n", expected.RankingExplanations)
	fmt.Fprintf(&b, "\tassertEqual(t, \"provenance slices\", baseline.Summary.ProvenanceSlices, %d)\n", expected.ProvenanceSlices)
	fmt.Fprintf(&b, "\tassertEqual(t, \"policy checks\", baseline.Summary.PolicyChecks, %d)\n", expected.PolicyChecks)
	fmt.Fprintf(&b, "\tassertEqual(t, \"repair proof summaries\", baseline.Summary.RepairProofs, %d)\n", expected.RepairProofs)
	fmt.Fprintf(&b, "}\n\n")
	fmt.Fprintf(&b, "func writeGoldenFile(t *testing.T, root, rel, content string) {\n\tt.Helper()\n\tpath := filepath.Join(root, filepath.FromSlash(rel))\n\tif err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {\n\t\tt.Fatal(err)\n\t}\n\tif err := os.WriteFile(path, []byte(content), 0o644); err != nil {\n\t\tt.Fatal(err)\n\t}\n}\n\n")
	fmt.Fprintf(&b, "func assertEqual(t *testing.T, name string, got, want int) {\n\tt.Helper()\n\tif got != want {\n\t\tt.Fatalf(\"%%s = %%d, want %%d\", name, got, want)\n\t}\n}\n")
	return b.String()
}

func renderGoMod(id string) (string, error) {
	root, err := moduleRoot()
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("module github.com/thehalleyyoung/patchline/generated/%s\n\ngo 1.22\n\nrequire github.com/thehalleyyoung/patchline v0.0.0\n\nreplace github.com/thehalleyyoung/patchline => %s\n", sanitizeID(id), filepath.ToSlash(root)), nil
}

func moduleRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("could not find repository go.mod from %s", dir)
		}
		dir = parent
	}
}

func writeReport(outDir string, report Report) error {
	copy := report
	copy.Markdown = ""
	data, err := json.MarshalIndent(copy, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(outDir, "golden-fixture.json"), append(data, '\n'), 0o644); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(outDir, "golden-fixture.md"), []byte(report.Markdown), 0o644)
}

func renderMarkdown(report Report) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Patchline golden fixture\n\n")
	fmt.Fprintf(&b, "- id: `%s`\n", report.ID)
	fmt.Fprintf(&b, "- input: `%s`\n", report.Input)
	fmt.Fprintf(&b, "- selected files: `%d`\n", report.Summary.SelectedFiles)
	fmt.Fprintf(&b, "- reduction: `%d%%`\n", report.Summary.ReductionPercent)
	fmt.Fprintf(&b, "- hash: `%s`\n\n", report.Hash)
	fmt.Fprintf(&b, "## Selected files\n\n| path | bytes | reason |\n| --- | ---: | --- |\n")
	for _, file := range report.SelectedFiles {
		fmt.Fprintf(&b, "| `%s` | %d | %s |\n", file.Path, file.Bytes, file.Reason)
	}
	return b.String()
}

func writeFixtureFile(root, rel, content string) error {
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0o644)
}

func stripContents(files []SelectedFile) []SelectedFile {
	out := make([]SelectedFile, len(files))
	copy(out, files)
	for i := range out {
		out[i].Content = ""
	}
	return out
}

func fixtureID(opts Options) string {
	if opts.GitHub != "" {
		return sanitizeID(opts.GitHub + "-" + opts.Subpath)
	}
	if opts.TestName != "" {
		return sanitizeID(opts.TestName)
	}
	return sanitizeID(filepath.Base(opts.Path))
}

func sourceInput(opts Options, root string) string {
	if opts.GitHub != "" {
		return opts.GitHub
	}
	return root
}

func reductionPercent(original, selected int) int {
	if original <= 0 {
		return 0
	}
	return 100 - (selected * 100 / original)
}

func sanitizeID(value string) string {
	value = strings.ToLower(value)
	value = regexp.MustCompile(`[^a-z0-9]+`).ReplaceAllString(value, "-")
	value = strings.Trim(value, "-")
	if value == "" {
		return "fixture"
	}
	return value
}

func exportedName(value string) string {
	parts := strings.FieldsFunc(sanitizeID(value), func(r rune) bool { return r == '-' })
	var b strings.Builder
	for _, part := range parts {
		if part == "" {
			continue
		}
		b.WriteString(strings.ToUpper(part[:1]))
		if len(part) > 1 {
			b.WriteString(part[1:])
		}
	}
	if b.Len() == 0 {
		return "Fixture"
	}
	return b.String()
}

func sanitizePackage(value string) string {
	value = strings.ToLower(value)
	value = regexp.MustCompile(`[^a-z0-9_]+`).ReplaceAllString(value, "_")
	value = strings.Trim(value, "_")
	if value == "" {
		return "goldenfixture"
	}
	if value[0] >= '0' && value[0] <= '9' {
		value = "pkg_" + value
	}
	return value
}
