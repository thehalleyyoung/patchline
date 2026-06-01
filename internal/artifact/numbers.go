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

const ExperimentNumbersVersion = "patchline.experiment-numbers/v1"

type ExperimentNumbersReport struct {
	Version    string                   `json:"version"`
	Root       string                   `json:"root"`
	Studies    []ExperimentStudyNumbers `json:"studies"`
	Benchmarks []ExperimentBenchmark    `json:"benchmarks"`
	Inputs     []ExperimentInputDigest  `json:"inputs"`
	Hash       string                   `json:"hash"`
	Markdown   string                   `json:"markdown,omitempty"`
}

type ExperimentInputDigest struct {
	Kind string `json:"kind"`
	Path string `json:"path"`
	Hash string `json:"hash"`
}

type ExperimentStudyNumbers struct {
	Name         string            `json:"name"`
	Directory    string            `json:"directory"`
	Expected     string            `json:"expected"`
	Suite        string            `json:"suite"`
	SuiteHash    string            `json:"suite_hash,omitempty"`
	Patchline    StudyMetrics      `json:"patchline"`
	Baselines    []Baseline        `json:"baselines"`
	Ablations    []AblationMode    `json:"ablations"`
	Scale        ScaleTotals       `json:"scale"`
	ScaleCases   []ScaleCase       `json:"scale_cases"`
	ReportHashes map[string]string `json:"report_hashes"`
	ExpectedHash string            `json:"expected_hash,omitempty"`
}

type ExperimentBenchmark struct {
	Name         string                `json:"name"`
	Path         string                `json:"path"`
	ExpectedPath string                `json:"expected_path"`
	Version      string                `json:"version"`
	Dataset      string                `json:"dataset_id"`
	OK           bool                  `json:"ok"`
	Metrics      BenchmarkRunMetrics   `json:"metrics"`
	Cases        []BenchmarkCaseResult `json:"cases"`
	Hash         string                `json:"hash"`
	ExpectedHash string                `json:"expected_hash"`
}

func GenerateExperimentNumbers(root string) (ExperimentNumbersReport, error) {
	if root == "" {
		root = "."
	}
	studySources := []struct {
		name     string
		dir      string
		expected string
	}{
		{name: "strict-migration-corpus", dir: filepath.Join("results", "generated", "artifact-studies"), expected: filepath.Join("benchmarks", "expected", "studies-strict.json")},
		{name: "public-bytebase-migrations", dir: filepath.Join("results", "generated", "artifact-studies", "public-migrations"), expected: filepath.Join("benchmarks", "expected", "studies-public-migrations.json")},
	}
	report := ExperimentNumbersReport{
		Version: ExperimentNumbersVersion,
		Root:    filepath.ToSlash(root),
	}
	for _, source := range studySources {
		study, inputs, err := readExperimentStudy(root, source.name, source.dir, source.expected)
		if err != nil {
			return ExperimentNumbersReport{}, err
		}
		report.Studies = append(report.Studies, study)
		report.Inputs = append(report.Inputs, inputs...)
	}
	benchmarks, inputs, err := readExperimentBenchmarks(root)
	if err != nil {
		return ExperimentNumbersReport{}, err
	}
	report.Benchmarks = benchmarks
	report.Inputs = append(report.Inputs, inputs...)
	sortInputs(report.Inputs)
	report.Hash = experimentNumbersHash(report)
	report.Markdown = renderExperimentNumbersMarkdown(report)
	return report, nil
}

func WriteExperimentNumbersReport(outDir string, report ExperimentNumbersReport) error {
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

func readExperimentStudy(root, name, dir, expected string) (ExperimentStudyNumbers, []ExperimentInputDigest, error) {
	baseDir := filepath.Join(root, dir)
	baselines, err := readBaselineReport(filepath.Join(baseDir, "baselines.json"))
	if err != nil {
		return ExperimentStudyNumbers{}, nil, err
	}
	ablations, err := readAblationReport(filepath.Join(baseDir, "ablations.json"))
	if err != nil {
		return ExperimentStudyNumbers{}, nil, err
	}
	scale, err := readScaleReport(filepath.Join(baseDir, "scale.json"))
	if err != nil {
		return ExperimentStudyNumbers{}, nil, err
	}
	expectedPath := filepath.Join(root, expected)
	expectedDigest, err := digestFile("study-expected", root, expectedPath)
	if err != nil {
		return ExperimentStudyNumbers{}, nil, err
	}
	inputs := []ExperimentInputDigest{expectedDigest}
	for _, rel := range []string{"baselines.json", "ablations.json", "scale.json"} {
		digest, err := digestFile("study-report", root, filepath.Join(baseDir, rel))
		if err != nil {
			return ExperimentStudyNumbers{}, nil, err
		}
		inputs = append(inputs, digest)
	}
	study := ExperimentStudyNumbers{
		Name:       name,
		Directory:  filepath.ToSlash(dir),
		Expected:   filepath.ToSlash(expected),
		Suite:      baselines.Suite,
		SuiteHash:  baselines.SuiteHash,
		Patchline:  baselines.Patchline,
		Baselines:  baselines.Baselines,
		Ablations:  ablations.Modes,
		Scale:      scale.Totals,
		ScaleCases: scale.Cases,
		ReportHashes: map[string]string{
			"baselines": baselines.Hash,
			"ablations": ablations.Hash,
			"scale":     scale.Hash,
		},
		ExpectedHash: expectedDigest.Hash,
	}
	if baselines.Suite != ablations.Suite || baselines.Suite != scale.Suite {
		return ExperimentStudyNumbers{}, nil, fmt.Errorf("%s: study suite mismatch across generated reports", name)
	}
	return study, inputs, nil
}

func readExperimentBenchmarks(root string) ([]ExperimentBenchmark, []ExperimentInputDigest, error) {
	paths, err := filepath.Glob(filepath.Join(root, "results", "generated", "artifact-benchmark", "*-report.json"))
	if err != nil {
		return nil, nil, err
	}
	if len(paths) == 0 {
		return nil, nil, fmt.Errorf("no generated benchmark reports found under %s", filepath.Join(root, "results", "generated", "artifact-benchmark"))
	}
	sort.Strings(paths)
	var benchmarks []ExperimentBenchmark
	var inputs []ExperimentInputDigest
	for _, path := range paths {
		report, err := readBenchmarkReport(path)
		if err != nil {
			return nil, nil, err
		}
		digest, err := digestFile("benchmark-report", root, path)
		if err != nil {
			return nil, nil, err
		}
		inputs = append(inputs, digest)
		expectedPath := filepath.Join(root, "benchmarks", "expected", filepath.Base(path))
		expected, err := readBenchmarkReport(expectedPath)
		if err != nil {
			return nil, nil, err
		}
		expectedDigest, err := digestFile("benchmark-expected", root, expectedPath)
		if err != nil {
			return nil, nil, err
		}
		inputs = append(inputs, expectedDigest)
		if report.Hash != expected.Hash {
			return nil, nil, fmt.Errorf("%s: actual benchmark hash %s does not match expected hash %s", filepath.Base(path), report.Hash, expected.Hash)
		}
		name := strings.TrimSuffix(filepath.Base(path), ".json")
		benchmarks = append(benchmarks, ExperimentBenchmark{
			Name:         name,
			Path:         filepath.ToSlash(relativePath(root, path)),
			ExpectedPath: filepath.ToSlash(relativePath(root, expectedPath)),
			Version:      report.Version,
			Dataset:      report.DatasetID,
			OK:           report.OK,
			Metrics:      report.Metrics,
			Cases:        report.Cases,
			Hash:         report.Hash,
			ExpectedHash: expected.Hash,
		})
	}
	return benchmarks, inputs, nil
}

func readBaselineReport(path string) (BaselineReport, error) {
	var report BaselineReport
	if err := readJSONFile(path, &report); err != nil {
		return report, err
	}
	recomputed := baselineHash(report)
	if report.Hash != recomputed {
		return report, fmt.Errorf("%s: stored hash %s does not match recomputed hash %s", path, report.Hash, recomputed)
	}
	return report, nil
}

func readAblationReport(path string) (AblationReport, error) {
	var report AblationReport
	if err := readJSONFile(path, &report); err != nil {
		return report, err
	}
	recomputed := ablationHash(report)
	if report.Hash != recomputed {
		return report, fmt.Errorf("%s: stored hash %s does not match recomputed hash %s", path, report.Hash, recomputed)
	}
	return report, nil
}

func readScaleReport(path string) (ScaleReport, error) {
	var report ScaleReport
	if err := readJSONFile(path, &report); err != nil {
		return report, err
	}
	recomputed := scaleHash(report)
	if report.Hash != recomputed {
		return report, fmt.Errorf("%s: stored hash %s does not match recomputed hash %s", path, report.Hash, recomputed)
	}
	return report, nil
}

func readBenchmarkReport(path string) (BenchmarkRunReport, error) {
	var report BenchmarkRunReport
	if err := readJSONFile(path, &report); err != nil {
		return report, err
	}
	recomputed := benchmarkRunHash(report)
	if report.Hash != recomputed {
		return report, fmt.Errorf("%s: stored hash %s does not match recomputed hash %s", path, report.Hash, recomputed)
	}
	return report, nil
}

func readJSONFile(path string, out any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	return nil
}

func digestFile(kind, root, path string) (ExperimentInputDigest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return ExperimentInputDigest{}, err
	}
	return ExperimentInputDigest{
		Kind: kind,
		Path: filepath.ToSlash(relativePath(root, path)),
		Hash: canonical.Hash(json.RawMessage(data)),
	}, nil
}

func relativePath(root, path string) string {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return path
	}
	return rel
}

func sortInputs(inputs []ExperimentInputDigest) {
	sort.Slice(inputs, func(i, j int) bool {
		if inputs[i].Kind != inputs[j].Kind {
			return inputs[i].Kind < inputs[j].Kind
		}
		return inputs[i].Path < inputs[j].Path
	})
}

func experimentNumbersHash(report ExperimentNumbersReport) string {
	return canonical.Hash(struct {
		Version    string                   `json:"version"`
		Root       string                   `json:"root"`
		Studies    []ExperimentStudyNumbers `json:"studies"`
		Benchmarks []ExperimentBenchmark    `json:"benchmarks"`
		Inputs     []ExperimentInputDigest  `json:"inputs"`
	}{report.Version, report.Root, report.Studies, report.Benchmarks, report.Inputs})
}

func renderExperimentNumbersMarkdown(report ExperimentNumbersReport) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Patchline experiment numbers\n\n")
	fmt.Fprintf(&b, "- Schema: `%s`\n", report.Version)
	fmt.Fprintf(&b, "- Hash: `%s`\n", report.Hash)
	fmt.Fprintf(&b, "- Inputs: %d hashed generated/expected reports\n\n", len(report.Inputs))
	for _, study := range report.Studies {
		fmt.Fprintf(&b, "## %s\n\n", study.Name)
		fmt.Fprintf(&b, "- Suite: `%s`\n", study.Suite)
		fmt.Fprintf(&b, "- Cases: %d\n", study.Patchline.Total)
		fmt.Fprintf(&b, "- Scale: %d statements, %d bytes, %d high-risk statements\n\n", study.Scale.Statements, study.Scale.Bytes, study.Scale.HighRiskStatements)
		fmt.Fprintf(&b, "| method | precision | recall | actionability | TP | TN | FP | FN | proof-backed | archive-linked |\n")
		fmt.Fprintf(&b, "| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |\n")
		writeMetricRow(&b, "Patchline", study.Patchline)
		for _, baseline := range study.Baselines {
			writeMetricRow(&b, baseline.Name, baseline.Metrics)
		}
		fmt.Fprintf(&b, "\n| ablation | precision | recall | actionability | proof-backed | archive-linked |\n")
		fmt.Fprintf(&b, "| --- | ---: | ---: | ---: | ---: | ---: |\n")
		for _, mode := range study.Ablations {
			fmt.Fprintf(&b, "| %s | %.3f | %.3f | %.2f | %d | %d |\n", mode.Name, mode.Metrics.Precision, mode.Metrics.Recall, mode.Metrics.MeanActionability, mode.Metrics.ProofBackedCases, mode.Metrics.ArchiveLinkedCases)
		}
		fmt.Fprintf(&b, "\n")
	}
	fmt.Fprintf(&b, "## Benchmark expected reports\n\n")
	fmt.Fprintf(&b, "| report | dataset | total | passed | failed | phase guards | actual hash | expected hash |\n")
	fmt.Fprintf(&b, "| --- | --- | ---: | ---: | ---: | ---: | --- | --- |\n")
	for _, benchmark := range report.Benchmarks {
		fmt.Fprintf(&b, "| %s | %s | %d | %d | %d | %d | `%s` | `%s` |\n", benchmark.Name, benchmark.Dataset, benchmark.Metrics.Total, benchmark.Metrics.Passed, benchmark.Metrics.Failed, benchmark.Metrics.PhaseGuards, benchmark.Hash, benchmark.ExpectedHash)
	}
	fmt.Fprintf(&b, "\nThese are small, deterministic artifact suites; the JSON file carries every per-case baseline, ablation, scale, generated benchmark, expected benchmark, and input-hash detail used by the paper tables.\n")
	return b.String()
}

func writeMetricRow(b *strings.Builder, name string, metrics StudyMetrics) {
	fmt.Fprintf(b, "| %s | %.3f | %.3f | %.2f | %d | %d | %d | %d | %d | %d |\n", name, metrics.Precision, metrics.Recall, metrics.MeanActionability, metrics.TruePositive, metrics.TrueNegative, metrics.FalsePositive, metrics.FalseNegative, metrics.ProofBackedCases, metrics.ArchiveLinkedCases)
}
