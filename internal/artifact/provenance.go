package artifact

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/thehalleyyoung/patchline/internal/canonical"
)

const ArtifactProvenanceVersion = "patchline.artifact-provenance/v1"

type ArtifactProvenanceReport struct {
	Version  string                    `json:"version"`
	Root     string                    `json:"root"`
	Files    []ArtifactProvenanceFile  `json:"files"`
	Checks   []ArtifactProvenanceCheck `json:"checks"`
	Hash     string                    `json:"hash"`
	Markdown string                    `json:"markdown,omitempty"`
}

type ArtifactProvenanceFile struct {
	Role   string `json:"role"`
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
	Bytes  int64  `json:"bytes"`
}

type ArtifactProvenanceCheck struct {
	ID      string            `json:"id"`
	OK      bool              `json:"ok"`
	Summary string            `json:"summary"`
	Details map[string]string `json:"details,omitempty"`
}

type publicCorpusSourceManifest struct {
	Version string                     `json:"version"`
	Name    string                     `json:"name"`
	Sources []publicCorpusSourceRecord `json:"sources"`
}

type publicCorpusSourceRecord struct {
	ID     string `json:"id"`
	URL    string `json:"url"`
	SHA256 string `json:"sha256"`
	Out    string `json:"out"`
}

type publicCorpusFetchReport struct {
	Version              string                         `json:"version"`
	SourceManifest       string                         `json:"source_manifest"`
	SourceManifestSHA256 string                         `json:"source_manifest_sha256"`
	OutputDir            string                         `json:"output_dir"`
	Offline              bool                           `json:"offline"`
	Files                []publicCorpusFetchReportEntry `json:"files"`
	OK                   bool                           `json:"ok"`
}

type publicCorpusFetchReportEntry struct {
	ID             string `json:"id"`
	URL            string `json:"url"`
	Path           string `json:"path"`
	Status         string `json:"status"`
	ExpectedSHA256 string `json:"expected_sha256"`
	ActualSHA256   string `json:"actual_sha256"`
	OK             bool   `json:"ok"`
}

type demoBundleManifest struct {
	Files     []demoBundleFile `json:"files"`
	FileCount int              `json:"file_count"`
}

type demoBundleFile struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
	Bytes  int64  `json:"bytes"`
}

func GenerateArtifactProvenance(root string) (ArtifactProvenanceReport, error) {
	if root == "" {
		root = "."
	}
	files, err := collectArtifactProvenanceFiles(root)
	if err != nil {
		return ArtifactProvenanceReport{}, err
	}
	checks, err := runArtifactProvenanceChecks(root)
	if err != nil {
		return ArtifactProvenanceReport{}, err
	}
	report := ArtifactProvenanceReport{
		Version: ArtifactProvenanceVersion,
		Root:    filepath.ToSlash(root),
		Files:   files,
		Checks:  checks,
	}
	report.Hash = artifactProvenanceHash(report)
	report.Markdown = renderArtifactProvenanceMarkdown(report)
	return report, nil
}

func WriteArtifactProvenanceReport(outDir string, report ArtifactProvenanceReport) error {
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(outDir, "summary.json"), append(data, '\n'), 0o644); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(outDir, "summary.md"), []byte(report.Markdown), 0o644)
}

func collectArtifactProvenanceFiles(root string) ([]ArtifactProvenanceFile, error) {
	var files []ArtifactProvenanceFile
	for _, rel := range []string{
		filepath.Join("examples", "public-corpus", "sources.json"),
		filepath.Join("results", "generated", "public-corpus", "fetch-report.json"),
		filepath.Join("results", "generated", "artifact-demo", "bundle-manifest.json"),
		filepath.Join("results", "generated", "artifact-tables", "summary.json"),
		filepath.Join("results", "generated", "artifact-numbers", "summary.json"),
		filepath.Join("results", "generated", "artifact-subtasks", "summary.json"),
		filepath.Join("results", "generated", "artifact-corpus-audit", "summary.json"),
	} {
		file, err := artifactProvenanceFile(root, rel)
		if err != nil {
			return nil, err
		}
		files = append(files, file)
	}
	for _, rel := range []string{
		filepath.Join("benchmarks", "manifests"),
		filepath.Join("benchmarks", "ground_truth"),
		filepath.Join("benchmarks", "expected"),
		filepath.Join("benchmarks", "corpus_protocol.json"),
		filepath.Join("results", "generated"),
	} {
		walked, err := walkArtifactProvenanceFiles(root, rel)
		if err != nil {
			return nil, err
		}
		files = append(files, walked...)
	}
	files = dedupeArtifactProvenanceFiles(files)
	sort.Slice(files, func(i, j int) bool {
		if files[i].Role != files[j].Role {
			return files[i].Role < files[j].Role
		}
		return files[i].Path < files[j].Path
	})
	return files, nil
}

func walkArtifactProvenanceFiles(root, rel string) ([]ArtifactProvenanceFile, error) {
	base := filepath.Join(root, rel)
	var files []ArtifactProvenanceFile
	if _, err := os.Stat(base); err != nil {
		return nil, err
	}
	err := filepath.WalkDir(base, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if filepath.ToSlash(relativePath(root, path)) == "results/generated/artifact-provenance" {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.Name() == ".DS_Store" {
			return nil
		}
		file, err := artifactProvenanceFile(root, relativePath(root, path))
		if err != nil {
			return err
		}
		files = append(files, file)
		return nil
	})
	return files, err
}

func artifactProvenanceFile(root, rel string) (ArtifactProvenanceFile, error) {
	path := filepath.Join(root, filepath.FromSlash(rel))
	sha, bytes, err := sha256File(path)
	if err != nil {
		return ArtifactProvenanceFile{}, err
	}
	rel = filepath.ToSlash(relativePath(root, path))
	return ArtifactProvenanceFile{
		Role:   artifactProvenanceRole(rel),
		Path:   rel,
		SHA256: sha,
		Bytes:  bytes,
	}, nil
}

func artifactProvenanceRole(rel string) string {
	switch {
	case strings.HasPrefix(rel, "benchmarks/manifests/"):
		return "benchmark-manifest"
	case strings.HasPrefix(rel, "benchmarks/ground_truth/"):
		return "ground-truth"
	case strings.HasPrefix(rel, "benchmarks/expected/"):
		return "frozen-expected"
	case rel == "examples/public-corpus/sources.json":
		return "source-manifest"
	case strings.HasPrefix(rel, "results/generated/public-corpus/"):
		return "generated-public-corpus-provenance"
	case strings.HasPrefix(rel, "results/generated/artifact-demo/"):
		return "generated-demo"
	case strings.HasPrefix(rel, "results/generated/"):
		return "generated-artifact"
	default:
		return "artifact-input"
	}
}

func dedupeArtifactProvenanceFiles(files []ArtifactProvenanceFile) []ArtifactProvenanceFile {
	seen := map[string]ArtifactProvenanceFile{}
	for _, file := range files {
		seen[file.Path] = file
	}
	paths := make([]string, 0, len(seen))
	for path := range seen {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	out := make([]ArtifactProvenanceFile, 0, len(paths))
	for _, path := range paths {
		out = append(out, seen[path])
	}
	return out
}

func runArtifactProvenanceChecks(root string) ([]ArtifactProvenanceCheck, error) {
	checks := []ArtifactProvenanceCheck{}
	fetchCheck, err := validatePublicCorpusFetch(root)
	if err != nil {
		return nil, err
	}
	checks = append(checks, fetchCheck)
	demoCheck, err := validateDemoBundleManifest(root)
	if err != nil {
		return nil, err
	}
	checks = append(checks, demoCheck)
	numbers, err := GenerateExperimentNumbers(root)
	if err != nil {
		return nil, fmt.Errorf("experiment-number provenance requires generated benchmark/study reports; run make artifact-numbers first: %w", err)
	}
	checks = append(checks, ArtifactProvenanceCheck{
		ID:      "benchmark-expected-matches",
		OK:      true,
		Summary: "generated benchmark reports match frozen expected report hashes through the experiment-number ledger",
		Details: map[string]string{
			"experiment_numbers_hash": numbers.Hash,
			"benchmarks":              fmt.Sprint(len(numbers.Benchmarks)),
			"studies":                 fmt.Sprint(len(numbers.Studies)),
			"inputs":                  fmt.Sprint(len(numbers.Inputs)),
		},
	})
	requiredCheck, err := validateRequiredGeneratedReports(root)
	if err != nil {
		return nil, err
	}
	checks = append(checks, requiredCheck)
	return checks, nil
}

func validatePublicCorpusFetch(root string) (ArtifactProvenanceCheck, error) {
	manifestPath := filepath.Join(root, "examples", "public-corpus", "sources.json")
	reportPath := filepath.Join(root, "results", "generated", "public-corpus", "fetch-report.json")
	var manifest publicCorpusSourceManifest
	if err := readJSONFile(manifestPath, &manifest); err != nil {
		return ArtifactProvenanceCheck{}, err
	}
	var report publicCorpusFetchReport
	if err := readJSONFile(reportPath, &report); err != nil {
		return ArtifactProvenanceCheck{}, err
	}
	manifestSHA, _, err := sha256File(manifestPath)
	if err != nil {
		return ArtifactProvenanceCheck{}, err
	}
	if report.SourceManifestSHA256 != manifestSHA {
		return ArtifactProvenanceCheck{}, fmt.Errorf("public corpus fetch report manifest hash %s does not match current sources.json %s", report.SourceManifestSHA256, manifestSHA)
	}
	if !report.OK {
		return ArtifactProvenanceCheck{}, fmt.Errorf("public corpus fetch report is not ok")
	}
	sources := map[string]publicCorpusSourceRecord{}
	for _, source := range manifest.Sources {
		if source.ID == "" {
			return ArtifactProvenanceCheck{}, fmt.Errorf("public corpus source has empty id")
		}
		sources[source.ID] = source
	}
	seen := map[string]struct{}{}
	for _, file := range report.Files {
		source, ok := sources[file.ID]
		if !ok {
			return ArtifactProvenanceCheck{}, fmt.Errorf("public corpus fetch report contains unexpected source %q", file.ID)
		}
		seen[file.ID] = struct{}{}
		if !file.OK {
			return ArtifactProvenanceCheck{}, fmt.Errorf("public corpus file %s is not ok", file.ID)
		}
		if file.URL != source.URL {
			return ArtifactProvenanceCheck{}, fmt.Errorf("public corpus file %s URL drifted", file.ID)
		}
		expectedPath, err := publicCorpusCachePath(root, source)
		if err != nil {
			return ArtifactProvenanceCheck{}, err
		}
		expectedRel := filepath.ToSlash(relativePath(root, expectedPath))
		if filepath.ToSlash(file.Path) != expectedRel {
			return ArtifactProvenanceCheck{}, fmt.Errorf("public corpus file %s report path %s does not match manifest-derived cache path %s", file.ID, file.Path, expectedRel)
		}
		if file.ExpectedSHA256 != source.SHA256 {
			return ArtifactProvenanceCheck{}, fmt.Errorf("public corpus file %s expected hash %s does not match manifest %s", file.ID, file.ExpectedSHA256, source.SHA256)
		}
		actualSHA, _, err := sha256File(expectedPath)
		if err != nil {
			return ArtifactProvenanceCheck{}, err
		}
		if actualSHA != source.SHA256 {
			return ArtifactProvenanceCheck{}, fmt.Errorf("public corpus file %s on-disk hash %s does not match manifest %s", file.ID, actualSHA, source.SHA256)
		}

		if file.ActualSHA256 != actualSHA {
			return ArtifactProvenanceCheck{}, fmt.Errorf("public corpus file %s report hash %s does not match on-disk hash %s", file.ID, file.ActualSHA256, actualSHA)
		}
	}
	for id := range sources {
		if _, ok := seen[id]; !ok {
			return ArtifactProvenanceCheck{}, fmt.Errorf("public corpus fetch report is missing source %q", id)
		}
	}
	return ArtifactProvenanceCheck{
		ID:      "public-corpus-fetch-ok",
		OK:      true,
		Summary: "public corpus manifest, fetch report, and on-disk cache hashes match",
		Details: map[string]string{
			"source_manifest_sha256": manifestSHA,
			"files":                  fmt.Sprint(len(report.Files)),
			"offline":                fmt.Sprint(report.Offline),
		},
	}, nil
}

func publicCorpusCachePath(root string, source publicCorpusSourceRecord) (string, error) {
	out := filepath.Clean(filepath.FromSlash(source.Out))
	if source.Out == "" || filepath.IsAbs(out) || out == "." || strings.HasPrefix(out, ".."+string(filepath.Separator)) || out == ".." {
		return "", fmt.Errorf("public corpus source %s has unsafe output path %q", source.ID, source.Out)
	}
	base := filepath.Join(root, "examples", "public-corpus", "downloads")
	path := filepath.Join(base, out)
	rel, err := filepath.Rel(base, path)
	if err != nil {
		return "", err
	}
	if strings.HasPrefix(rel, ".."+string(filepath.Separator)) || rel == ".." || filepath.IsAbs(rel) {
		return "", fmt.Errorf("public corpus source %s escapes cache directory: %q", source.ID, source.Out)
	}
	return path, nil
}

func validateDemoBundleManifest(root string) (ArtifactProvenanceCheck, error) {
	demoDir := filepath.Join(root, "results", "generated", "artifact-demo")
	manifestPath := filepath.Join(demoDir, "bundle-manifest.json")
	var manifest demoBundleManifest
	if err := readJSONFile(manifestPath, &manifest); err != nil {
		return ArtifactProvenanceCheck{}, err
	}
	actualFiles, err := demoBundleFilesOnDisk(demoDir)
	if err != nil {
		return ArtifactProvenanceCheck{}, err
	}
	if manifest.FileCount != len(manifest.Files) {
		return ArtifactProvenanceCheck{}, fmt.Errorf("demo bundle manifest file_count %d does not match listed files %d", manifest.FileCount, len(manifest.Files))
	}
	if manifest.FileCount != len(actualFiles) {
		return ArtifactProvenanceCheck{}, fmt.Errorf("demo bundle manifest file_count %d does not match on-disk stage files %d", manifest.FileCount, len(actualFiles))
	}
	listed := map[string]demoBundleFile{}
	for _, file := range manifest.Files {
		if file.Path == "" || filepath.IsAbs(file.Path) || strings.Contains(filepath.ToSlash(file.Path), "../") {
			return ArtifactProvenanceCheck{}, fmt.Errorf("demo bundle manifest contains unsafe path %q", file.Path)
		}
		listed[filepath.ToSlash(file.Path)] = file
	}
	for rel, actual := range actualFiles {
		listedFile, ok := listed[rel]
		if !ok {
			return ArtifactProvenanceCheck{}, fmt.Errorf("demo bundle manifest missing on-disk file %s", rel)
		}
		if listedFile.SHA256 != actual.SHA256 || listedFile.Bytes != actual.Bytes {
			return ArtifactProvenanceCheck{}, fmt.Errorf("demo bundle manifest entry %s does not match on-disk sha/bytes", rel)
		}
	}
	for rel := range listed {
		if _, ok := actualFiles[rel]; !ok {
			return ArtifactProvenanceCheck{}, fmt.Errorf("demo bundle manifest lists missing file %s", rel)
		}
	}
	manifestSHA, _, err := sha256File(manifestPath)
	if err != nil {
		return ArtifactProvenanceCheck{}, err
	}
	return ArtifactProvenanceCheck{
		ID:      "demo-bundle-manifest-ok",
		OK:      true,
		Summary: "artifact-demo bundle manifest exactly matches generated stage output files",
		Details: map[string]string{
			"bundle_manifest_sha256": manifestSHA,
			"files":                  fmt.Sprint(manifest.FileCount),
		},
	}, nil
}

func demoBundleFilesOnDisk(demoDir string) (map[string]demoBundleFile, error) {
	files := map[string]demoBundleFile{}
	err := filepath.WalkDir(demoDir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		name := filepath.Base(path)
		if name == "summary.json" || name == "summary.md" || name == "bundle-manifest.json" {
			return nil
		}
		rel, err := filepath.Rel(demoDir, path)
		if err != nil {
			return err
		}
		sha, bytes, err := sha256File(path)
		if err != nil {
			return err
		}
		files[filepath.ToSlash(rel)] = demoBundleFile{Path: filepath.ToSlash(rel), SHA256: sha, Bytes: bytes}
		return nil
	})
	return files, err
}

func validateRequiredGeneratedReports(root string) (ArtifactProvenanceCheck, error) {
	required := []string{
		filepath.Join("results", "generated", "artifact-demo", "summary.json"),
		filepath.Join("results", "generated", "artifact-tables", "summary.json"),
		filepath.Join("results", "generated", "artifact-numbers", "summary.json"),
		filepath.Join("results", "generated", "artifact-subtasks", "summary.json"),
		filepath.Join("results", "generated", "artifact-corpus-audit", "summary.json"),
		filepath.Join("results", "generated", "public-corpus", "fetch-report.json"),
	}
	details := map[string]string{}
	for _, rel := range required {
		sha, _, err := sha256File(filepath.Join(root, rel))
		if err != nil {
			return ArtifactProvenanceCheck{}, err
		}
		details[filepath.ToSlash(rel)] = sha
	}
	return ArtifactProvenanceCheck{
		ID:      "required-generated-reports-present",
		OK:      true,
		Summary: "reviewer-facing generated reports are present and hashed",
		Details: details,
	}, nil
}

func sha256File(path string) (string, int64, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", 0, err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), int64(len(data)), nil
}

func artifactProvenanceHash(report ArtifactProvenanceReport) string {
	return canonical.Hash(struct {
		Version string                    `json:"version"`
		Root    string                    `json:"root"`
		Files   []ArtifactProvenanceFile  `json:"files"`
		Checks  []ArtifactProvenanceCheck `json:"checks"`
	}{report.Version, report.Root, report.Files, report.Checks})
}

func renderArtifactProvenanceMarkdown(report ArtifactProvenanceReport) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Patchline artifact provenance\n\n")
	fmt.Fprintf(&b, "- version: `%s`\n- hash: `%s`\n- files: `%d`\n- checks: `%d`\n\n", report.Version, report.Hash, len(report.Files), len(report.Checks))
	fmt.Fprintf(&b, "## Checks\n\n")
	fmt.Fprintf(&b, "| check | result | summary |\n")
	fmt.Fprintf(&b, "| --- | --- | --- |\n")
	for _, check := range report.Checks {
		result := "pass"
		if !check.OK {
			result = "fail"
		}
		fmt.Fprintf(&b, "| %s | %s | %s |\n", check.ID, result, strings.ReplaceAll(check.Summary, "|", "\\|"))
	}
	fmt.Fprintf(&b, "\n## File roles\n\n")
	fmt.Fprintf(&b, "| role | files | bytes |\n")
	fmt.Fprintf(&b, "| --- | ---: | ---: |\n")
	for _, row := range artifactProvenanceRoleRows(report.Files) {
		fmt.Fprintf(&b, "| %s | %s | %s |\n", row[0], row[1], row[2])
	}
	fmt.Fprintf(&b, "\n## Hashed files\n\n")
	fmt.Fprintf(&b, "| role | path | sha256 | bytes |\n")
	fmt.Fprintf(&b, "| --- | --- | --- | ---: |\n")
	for _, file := range report.Files {
		fmt.Fprintf(&b, "| %s | `%s` | `%s` | %d |\n", file.Role, file.Path, file.SHA256, file.Bytes)
	}
	return b.String()
}

func artifactProvenanceRoleRows(files []ArtifactProvenanceFile) [][]string {
	type totals struct {
		files int
		bytes int64
	}
	byRole := map[string]totals{}
	for _, file := range files {
		total := byRole[file.Role]
		total.files++
		total.bytes += file.Bytes
		byRole[file.Role] = total
	}
	roles := make([]string, 0, len(byRole))
	for role := range byRole {
		roles = append(roles, role)
	}
	sort.Strings(roles)
	rows := make([][]string, 0, len(roles))
	for _, role := range roles {
		total := byRole[role]
		rows = append(rows, []string{role, fmt.Sprint(total.files), fmt.Sprint(total.bytes)})
	}
	return rows
}
