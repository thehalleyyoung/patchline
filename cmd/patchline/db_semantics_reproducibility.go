package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/thehalleyyoung/patchline/internal/dbsemantics"
)

func dbSemanticsReproducibilityCommand(args []string) error {
	reportPaths := flagValues(args, "--report")
	if reportsDir, ok := flagValue(args, "--reports"); ok {
		if strings.TrimSpace(reportsDir) == "" {
			return errors.New("--reports requires a directory containing db-semantics JSON reports")
		}
		paths, err := dbSemanticsReportPaths(reportsDir)
		if err != nil {
			return err
		}
		reportPaths = append(reportPaths, paths...)
	}
	if len(reportPaths) == 0 {
		return errors.New("usage: patchline db-semantics-reproducibility --report report.json [--report report2.json ...] [--reports dir] [--out report.json] [--markdown report.md] [--json]")
	}
	reports, err := readDBSemanticsReports(reportPaths)
	if err != nil {
		return err
	}
	report, err := dbsemantics.BuildReproducibilityReport(reports)
	if err != nil {
		return err
	}
	if outPath, ok := flagValue(args, "--out"); ok && outPath != "" {
		if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
			return err
		}
		if err := writeJSONArtifact(outPath, report); err != nil {
			return err
		}
	}
	if markdownPath, ok := flagValue(args, "--markdown"); ok && markdownPath != "" {
		if err := os.MkdirAll(filepath.Dir(markdownPath), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(markdownPath, []byte(renderDBSemanticsReproducibilityMarkdown(report)), 0o644); err != nil {
			return err
		}
	}
	if hasFlag(args, "--json") {
		return writeJSON(os.Stdout, report)
	}
	fmt.Printf("db-semantics reproducibility engines=%d reports=%d images=%d observations=%d hash=%s\n",
		report.Summary.Engines,
		report.Summary.Reports,
		report.Summary.ContainerImages,
		report.Summary.Observations,
		report.Hash,
	)
	return nil
}

func dbSemanticsReportPaths(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read db-semantics reports dir %q: %w", dir, err)
	}
	var paths []string
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") || isGeneratedDBSemanticsCompanion(entry.Name()) {
			continue
		}
		paths = append(paths, filepath.Join(dir, entry.Name()))
	}
	sort.Strings(paths)
	if len(paths) == 0 {
		return nil, fmt.Errorf("reports dir %q contains no JSON reports", dir)
	}
	return paths, nil
}

func isGeneratedDBSemanticsCompanion(name string) bool {
	switch name {
	case "gate-summary.json", "reproducibility-report.json", "reproducibility-report.stdout.json":
		return true
	default:
		return strings.HasSuffix(name, ".stdout.json")
	}
}

func readDBSemanticsReports(paths []string) ([]dbsemantics.Report, error) {
	unique := map[string]bool{}
	var reports []dbsemantics.Report
	for _, path := range paths {
		path = strings.TrimSpace(path)
		if path == "" {
			return nil, errors.New("--report requires a non-empty JSON path")
		}
		clean := filepath.Clean(path)
		if unique[clean] {
			continue
		}
		unique[clean] = true
		content, err := os.ReadFile(clean)
		if err != nil {
			return nil, fmt.Errorf("read db-semantics report %q: %w", clean, err)
		}
		var report dbsemantics.Report
		if err := json.Unmarshal(content, &report); err != nil {
			return nil, fmt.Errorf("parse db-semantics report %q: %w", clean, err)
		}
		reports = append(reports, report)
	}
	return reports, nil
}

func renderDBSemanticsReproducibilityMarkdown(report dbsemantics.ReproducibilityReport) string {
	var b strings.Builder
	b.WriteString("# Database-semantics reproducibility report\n\n")
	b.WriteString(fmt.Sprintf("Hash: `%s`\n\n", report.Hash))
	b.WriteString(fmt.Sprintf("Covers %d engines, %d engine/version pins, %d container image pins, and %d observed behavior rows.\n\n",
		report.Summary.Engines,
		report.Summary.EngineVersionPins,
		report.Summary.ContainerImages,
		report.Summary.Observations,
	))
	b.WriteString("| Engine | Version | Runtime pin | Status |\n")
	b.WriteString("|---|---|---|---|\n")
	for _, pin := range report.EnginePins {
		image := pin.ContainerImage
		if image == "" {
			image = pin.RuntimeKind
		}
		b.WriteString(fmt.Sprintf("| %s | %s | `%s` | %s |\n", pin.Engine, pin.ResolvedVersion, image, pin.VerificationStatus))
	}
	b.WriteString("\n| Observation kind | Count |\n")
	b.WriteString("|---|---:|\n")
	counts := map[string]int{}
	for _, observation := range report.Observations {
		counts[observation.ObservationKind]++
	}
	var kinds []string
	for kind := range counts {
		kinds = append(kinds, kind)
	}
	sort.Strings(kinds)
	for _, kind := range kinds {
		b.WriteString(fmt.Sprintf("| %s | %d |\n", kind, counts[kind]))
	}
	return b.String()
}
