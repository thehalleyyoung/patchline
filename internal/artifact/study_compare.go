package artifact

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/thehalleyyoung/patchline/internal/canonical"
)

const StudyExpectedVersion = "patchline.artifact-study-expected/v1"

type StudyExpectedManifest struct {
	Version   string             `json:"version"`
	Suite     string             `json:"suite"`
	SuiteHash string             `json:"suite_hash,omitempty"`
	Reports   []StudyReportEntry `json:"reports"`
	Hash      string             `json:"hash"`
}

type StudyReportEntry struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	Version string `json:"version"`
	Hash    string `json:"hash"`
}

type StudyCompareReport struct {
	Version      string                 `json:"version"`
	OK           bool                   `json:"ok"`
	Suite        string                 `json:"suite"`
	SuiteHash    string                 `json:"suite_hash,omitempty"`
	ActualHash   string                 `json:"actual_hash"`
	ExpectedHash string                 `json:"expected_hash"`
	Mismatches   []StudyCompareMismatch `json:"mismatches,omitempty"`
	Hash         string                 `json:"hash"`
}

type StudyCompareMismatch struct {
	Report   string `json:"report,omitempty"`
	Field    string `json:"field"`
	Actual   string `json:"actual,omitempty"`
	Expected string `json:"expected,omitempty"`
}

func SummarizeStudyReports(dir string) (StudyExpectedManifest, error) {
	entries := make([]StudyReportEntry, 0, len(studyReportFiles()))
	var suite string
	var suiteHash string
	for _, report := range studyReportFiles() {
		entry, reportSuite, reportSuiteHash, err := readStudyReportEntry(dir, report.name, report.path)
		if err != nil {
			return StudyExpectedManifest{}, err
		}
		if suite == "" {
			suite = reportSuite
		} else if reportSuite != suite {
			return StudyExpectedManifest{}, fmt.Errorf("study reports disagree on suite: %q vs %q", suite, reportSuite)
		}
		if reportSuiteHash != "" {
			if suiteHash == "" {
				suiteHash = reportSuiteHash
			} else if reportSuiteHash != suiteHash {
				return StudyExpectedManifest{}, fmt.Errorf("study reports disagree on suite hash: %q vs %q", suiteHash, reportSuiteHash)
			}
		}
		entries = append(entries, entry)
	}
	manifest := StudyExpectedManifest{
		Version:   StudyExpectedVersion,
		Suite:     suite,
		SuiteHash: suiteHash,
		Reports:   entries,
	}
	manifest.Hash = studyExpectedHash(manifest)
	return manifest, nil
}

func CompareStudyReports(dir, expectedPath string) (StudyCompareReport, error) {
	actual, err := SummarizeStudyReports(dir)
	if err != nil {
		return StudyCompareReport{}, err
	}
	expected, err := readStudyExpectedManifest(expectedPath)
	if err != nil {
		return StudyCompareReport{}, err
	}
	actualHash := studyExpectedHash(actual)
	expectedHash := studyExpectedHash(expected)
	report := StudyCompareReport{
		Version:      "patchline.artifact-study-compare/v1",
		OK:           true,
		Suite:        actual.Suite,
		SuiteHash:    actual.SuiteHash,
		ActualHash:   actualHash,
		ExpectedHash: expectedHash,
	}
	if expected.Version != StudyExpectedVersion {
		report.Mismatches = append(report.Mismatches, StudyCompareMismatch{Field: "expected_version", Actual: expected.Version, Expected: StudyExpectedVersion})
	}
	if actual.Hash != actualHash {
		report.Mismatches = append(report.Mismatches, StudyCompareMismatch{Field: "actual_hash_integrity", Actual: actual.Hash, Expected: actualHash})
	}
	if expected.Hash != expectedHash {
		report.Mismatches = append(report.Mismatches, StudyCompareMismatch{Field: "expected_hash_integrity", Actual: expected.Hash, Expected: expectedHash})
	}
	if actual.Suite != expected.Suite {
		report.Mismatches = append(report.Mismatches, StudyCompareMismatch{Field: "suite", Actual: actual.Suite, Expected: expected.Suite})
	}
	if actual.SuiteHash != expected.SuiteHash {
		report.Mismatches = append(report.Mismatches, StudyCompareMismatch{Field: "suite_hash", Actual: actual.SuiteHash, Expected: expected.SuiteHash})
	}
	report.Mismatches = append(report.Mismatches, compareStudyEntries(actual.Reports, expected.Reports)...)
	report.Mismatches = append(report.Mismatches, unexpectedStudyJSONFiles(dir, expected.Reports)...)
	if actualHash != expectedHash {
		report.Mismatches = append(report.Mismatches, StudyCompareMismatch{Field: "hash", Actual: actualHash, Expected: expectedHash})
	}
	if len(report.Mismatches) > 0 {
		report.OK = false
	}
	sort.Slice(report.Mismatches, func(i, j int) bool {
		if report.Mismatches[i].Report != report.Mismatches[j].Report {
			return report.Mismatches[i].Report < report.Mismatches[j].Report
		}
		return report.Mismatches[i].Field < report.Mismatches[j].Field
	})
	report.Hash = canonical.Hash(struct {
		Version      string                 `json:"version"`
		OK           bool                   `json:"ok"`
		Suite        string                 `json:"suite"`
		SuiteHash    string                 `json:"suite_hash,omitempty"`
		ActualHash   string                 `json:"actual_hash"`
		ExpectedHash string                 `json:"expected_hash"`
		Mismatches   []StudyCompareMismatch `json:"mismatches,omitempty"`
	}{report.Version, report.OK, report.Suite, report.SuiteHash, report.ActualHash, report.ExpectedHash, report.Mismatches})
	return report, nil
}

func WriteStudyExpectedManifest(path string, manifest StudyExpectedManifest) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}

func readStudyExpectedManifest(path string) (StudyExpectedManifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return StudyExpectedManifest{}, err
	}
	var manifest StudyExpectedManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return StudyExpectedManifest{}, err
	}
	return manifest, nil
}

type studyReportFile struct {
	name string
	path string
}

func studyReportFiles() []studyReportFile {
	return []studyReportFile{
		{name: "baselines", path: "baselines.json"},
		{name: "ablations", path: "ablations.json"},
		{name: "scale", path: "scale.json"},
	}
}

func readStudyReportEntry(dir, name, relPath string) (StudyReportEntry, string, string, error) {
	path, err := resolveStudyReportPath(dir, relPath)
	if err != nil {
		return StudyReportEntry{}, "", "", err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return StudyReportEntry{}, "", "", err
	}
	switch name {
	case "baselines":
		var report BaselineReport
		if err := json.Unmarshal(data, &report); err != nil {
			return StudyReportEntry{}, "", "", err
		}
		recomputed := baselineHash(report)
		if report.Hash != recomputed {
			return StudyReportEntry{}, "", "", fmt.Errorf("%s: stored hash %s does not match recomputed hash %s", relPath, report.Hash, recomputed)
		}
		return StudyReportEntry{Name: name, Path: relPath, Version: report.Version, Hash: recomputed}, report.Suite, report.SuiteHash, nil
	case "ablations":
		var report AblationReport
		if err := json.Unmarshal(data, &report); err != nil {
			return StudyReportEntry{}, "", "", err
		}
		recomputed := ablationHash(report)
		if report.Hash != recomputed {
			return StudyReportEntry{}, "", "", fmt.Errorf("%s: stored hash %s does not match recomputed hash %s", relPath, report.Hash, recomputed)
		}
		return StudyReportEntry{Name: name, Path: relPath, Version: report.Version, Hash: recomputed}, report.Suite, "", nil
	case "scale":
		var report ScaleReport
		if err := json.Unmarshal(data, &report); err != nil {
			return StudyReportEntry{}, "", "", err
		}
		recomputed := scaleHash(report)
		if report.Hash != recomputed {
			return StudyReportEntry{}, "", "", fmt.Errorf("%s: stored hash %s does not match recomputed hash %s", relPath, report.Hash, recomputed)
		}
		return StudyReportEntry{Name: name, Path: relPath, Version: report.Version, Hash: recomputed}, report.Suite, "", nil
	default:
		return StudyReportEntry{}, "", "", fmt.Errorf("unsupported study report %q", name)
	}
}

func resolveStudyReportPath(dir, relPath string) (string, error) {
	if relPath == "" {
		return "", fmt.Errorf("empty study report path")
	}
	if filepath.IsAbs(relPath) {
		return "", fmt.Errorf("study report path must be relative: %s", relPath)
	}
	cleaned := filepath.Clean(relPath)
	if cleaned == "." || cleaned != filepath.ToSlash(cleaned) || strings.HasPrefix(cleaned, "..") {
		return "", fmt.Errorf("unsafe study report path: %s", relPath)
	}
	return filepath.Join(dir, cleaned), nil
}

func compareStudyEntries(actual, expected []StudyReportEntry) []StudyCompareMismatch {
	actualByName := map[string]StudyReportEntry{}
	expectedByName := map[string]StudyReportEntry{}
	for _, entry := range actual {
		actualByName[entry.Name] = entry
	}
	for _, entry := range expected {
		expectedByName[entry.Name] = entry
	}
	var mismatches []StudyCompareMismatch
	for name, expectedEntry := range expectedByName {
		actualEntry, ok := actualByName[name]
		if !ok {
			mismatches = append(mismatches, StudyCompareMismatch{Report: name, Field: "missing_report", Expected: expectedEntry.Path})
			continue
		}
		if actualEntry.Path != expectedEntry.Path {
			mismatches = append(mismatches, StudyCompareMismatch{Report: name, Field: "path", Actual: actualEntry.Path, Expected: expectedEntry.Path})
		}
		if actualEntry.Version != expectedEntry.Version {
			mismatches = append(mismatches, StudyCompareMismatch{Report: name, Field: "version", Actual: actualEntry.Version, Expected: expectedEntry.Version})
		}
		if actualEntry.Hash != expectedEntry.Hash {
			mismatches = append(mismatches, StudyCompareMismatch{Report: name, Field: "hash", Actual: actualEntry.Hash, Expected: expectedEntry.Hash})
		}
	}
	for name, actualEntry := range actualByName {
		if _, ok := expectedByName[name]; !ok {
			mismatches = append(mismatches, StudyCompareMismatch{Report: name, Field: "unexpected_report", Actual: actualEntry.Path})
		}
	}
	return mismatches
}

func unexpectedStudyJSONFiles(dir string, expected []StudyReportEntry) []StudyCompareMismatch {
	expectedPaths := map[string]bool{}
	for _, entry := range expected {
		expectedPaths[filepath.ToSlash(filepath.Clean(entry.Path))] = true
	}
	matches, err := filepath.Glob(filepath.Join(dir, "*.json"))
	if err != nil {
		return []StudyCompareMismatch{{Field: "actual_dir_glob", Actual: err.Error()}}
	}
	var mismatches []StudyCompareMismatch
	for _, match := range matches {
		rel, err := filepath.Rel(dir, match)
		if err != nil {
			mismatches = append(mismatches, StudyCompareMismatch{Field: "actual_dir_rel", Actual: err.Error()})
			continue
		}
		rel = filepath.ToSlash(rel)
		if !expectedPaths[rel] {
			mismatches = append(mismatches, StudyCompareMismatch{Report: strings.TrimSuffix(rel, ".json"), Field: "unexpected_json_file", Actual: rel})
		}
	}
	return mismatches
}

func studyExpectedHash(manifest StudyExpectedManifest) string {
	return canonical.Hash(struct {
		Version   string             `json:"version"`
		Suite     string             `json:"suite"`
		SuiteHash string             `json:"suite_hash,omitempty"`
		Reports   []StudyReportEntry `json:"reports"`
	}{manifest.Version, manifest.Suite, manifest.SuiteHash, manifest.Reports})
}
