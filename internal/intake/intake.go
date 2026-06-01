package intake

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/thehalleyyoung/patchline/internal/canonical"
	"github.com/thehalleyyoung/patchline/internal/evidence"
	"github.com/thehalleyyoung/patchline/internal/migration"
	"github.com/thehalleyyoung/patchline/internal/repair"
)

const Version = "patchline.intake/v1"

type Options struct {
	Path         string
	GitHub       string
	Ref          string
	Subpath      string
	DownloadDir  string
	MaxFileBytes int64
}

type Report struct {
	Version          string                    `json:"version"`
	Source           Source                    `json:"source"`
	Summary          Summary                   `json:"summary"`
	SQL              []SQLFinding              `json:"sql_findings,omitempty"`
	SourceSQL        migration.SourceSQLReport `json:"source_sql,omitempty"`
	Evidence         []EvidenceFinding         `json:"evidence_findings,omitempty"`
	Repairs          []RepairFinding           `json:"repair_findings,omitempty"`
	Problems         []ProblemCandidate        `json:"problem_candidates,omitempty"`
	Causes           []CauseCandidate          `json:"cause_candidates,omitempty"`
	RepairCandidates []RepairCandidate         `json:"repair_candidates,omitempty"`
	Links            []CandidateLink           `json:"candidate_links,omitempty"`
	TimeSignals      []TimeSignal              `json:"time_signals,omitempty"`
	UnknownInputs    []UnknownInput            `json:"unknown_inputs,omitempty"`
	Suggestions      []Suggestion              `json:"suggestions,omitempty"`
	Warnings         []string                  `json:"warnings,omitempty"`
	Hash             string                    `json:"hash"`
	Markdown         string                    `json:"markdown,omitempty"`
}

type Source struct {
	Mode        string `json:"mode"`
	Input       string `json:"input"`
	Ref         string `json:"ref,omitempty"`
	Subpath     string `json:"subpath,omitempty"`
	ScannedRoot string `json:"scanned_root"`
	Downloaded  string `json:"downloaded,omitempty"`
}

type Summary struct {
	FilesScanned            int   `json:"files_scanned"`
	BytesScanned            int64 `json:"bytes_scanned"`
	SQLFiles                int   `json:"sql_files"`
	LooseSQLSnippets        int   `json:"loose_sql_snippets"`
	HighRiskSQLStatements   int   `json:"high_risk_sql_statements"`
	MediumRiskSQLStatements int   `json:"medium_risk_sql_statements"`
	SourceSQLObservations   int   `json:"source_sql_observations"`
	EvidenceFiles           int   `json:"evidence_files"`
	EvidenceEvents          int   `json:"evidence_events"`
	GenericEvidenceSignals  int   `json:"generic_evidence_signals"`
	RepairManifests         int   `json:"repair_manifests"`
	ProblemCandidates       int   `json:"problem_candidates"`
	CauseCandidates         int   `json:"cause_candidates"`
	RepairCandidates        int   `json:"repair_candidates"`
	LinkedCandidates        int   `json:"linked_candidates"`
	TimeSignals             int   `json:"time_signals"`
	UnknownJSONFiles        int   `json:"unknown_json_files"`
}

type SQLFinding struct {
	Path       string                `json:"path"`
	SourceKind string                `json:"source_kind"`
	Dialect    migration.Dialect     `json:"dialect,omitempty"`
	Line       int                   `json:"line,omitempty"`
	Statements []migration.Statement `json:"statements"`
	Summary    migration.Summary     `json:"summary"`
	Hash       string                `json:"hash"`
}

type EvidenceFinding struct {
	Path       string         `json:"path"`
	Format     string         `json:"format"`
	Adapter    string         `json:"adapter,omitempty"`
	OK         bool           `json:"ok"`
	EventCount int            `json:"event_count"`
	Warnings   []string       `json:"warnings,omitempty"`
	Signals    map[string]int `json:"signals,omitempty"`
	InputHash  string         `json:"input_hash,omitempty"`
}

type RepairFinding struct {
	Path        string              `json:"path"`
	Name        string              `json:"name,omitempty"`
	Incident    string              `json:"incident,omitempty"`
	Operations  int                 `json:"operations"`
	Errors      int                 `json:"errors"`
	Warnings    int                 `json:"warnings"`
	Diagnostics []repair.Diagnostic `json:"diagnostics,omitempty"`
}

type ProblemCandidate struct {
	ID          string   `json:"id"`
	Path        string   `json:"path"`
	Kind        string   `json:"kind"`
	Severity    string   `json:"severity"`
	Table       string   `json:"table,omitempty"`
	Fingerprint string   `json:"fingerprint,omitempty"`
	Identifiers []string `json:"identifiers,omitempty"`
	Confidence  string   `json:"confidence"`
	Rationale   string   `json:"rationale"`
}

type CauseCandidate struct {
	ID          string   `json:"id"`
	Path        string   `json:"path"`
	Kind        string   `json:"kind"`
	Identifiers []string `json:"identifiers,omitempty"`
	Confidence  string   `json:"confidence"`
	Rationale   string   `json:"rationale"`
}

type RepairCandidate struct {
	ID          string   `json:"id"`
	Path        string   `json:"path"`
	Kind        string   `json:"kind"`
	Table       string   `json:"table,omitempty"`
	Identifiers []string `json:"identifiers,omitempty"`
	Confidence  string   `json:"confidence"`
	Rationale   string   `json:"rationale"`
}

type CandidateLink struct {
	From        string   `json:"from"`
	To          string   `json:"to"`
	Kind        string   `json:"kind"`
	Identifiers []string `json:"identifiers"`
	Confidence  string   `json:"confidence"`
}

type TimeSignal struct {
	ID          string   `json:"id"`
	Path        string   `json:"path"`
	Timestamp   string   `json:"timestamp"`
	Source      string   `json:"source"`
	Confidence  string   `json:"confidence"`
	Identifiers []string `json:"identifiers,omitempty"`
}

type UnknownInput struct {
	Path   string `json:"path"`
	Kind   string `json:"kind"`
	Reason string `json:"reason"`
}

type Suggestion struct {
	Command string `json:"command"`
	Reason  string `json:"reason"`
}

type scanFile struct {
	Abs  string
	Rel  string
	Size int64
}

func Run(ctx context.Context, opts Options) (Report, error) {
	if opts.MaxFileBytes == 0 {
		opts.MaxFileBytes = 2 << 20
	}
	root, source, err := resolveSource(ctx, opts)
	if err != nil {
		return Report{}, err
	}
	report := Report{Version: Version, Source: source}
	files, err := discoverFiles(root)
	if err != nil {
		return Report{}, err
	}
	report.Summary.FilesScanned = len(files)
	for _, file := range files {
		report.Summary.BytesScanned += file.Size
	}
	if sourceSQL, err := migration.ExtractSourceSQL(root); err == nil {
		report.SourceSQL = sourceSQL
		report.Summary.SourceSQLObservations = len(sourceSQL.Observations)
		if len(sourceSQL.Observations) > 0 {
			report.Suggestions = append(report.Suggestions, Suggestion{Command: fmt.Sprintf("patchline extract-sql %s --json", shellPath(source.ScannedRoot)), Reason: "embedded/source SQL is present and can be inspected without labels"})
		}
	} else {
		report.Warnings = append(report.Warnings, "source SQL extraction failed: "+err.Error())
	}
	for _, file := range files {
		if file.Size > opts.MaxFileBytes {
			report.UnknownInputs = append(report.UnknownInputs, UnknownInput{Path: file.Rel, Kind: "oversized", Reason: fmt.Sprintf("skipped file larger than %d bytes", opts.MaxFileBytes)})
			continue
		}
		content, err := os.ReadFile(file.Abs)
		if err != nil {
			report.Warnings = append(report.Warnings, fmt.Sprintf("%s: %v", file.Rel, err))
			continue
		}
		report.scanSQLFile(file, content)
		report.scanLooseSQL(file, content)
		report.scanEvidence(file, content)
		report.scanRepairManifest(file, content)
		report.scanTextCandidates(file, content)
		report.scanTimeSignals(file, content)
	}
	report.finalize()
	return report, nil
}

func WriteReport(outDir string, report Report) error {
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
	if err := os.WriteFile(filepath.Join(outDir, "summary.md"), []byte(report.Markdown), 0o644); err != nil {
		return err
	}
	sarifData, err := json.MarshalIndent(renderSARIF(report), "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(outDir, "summary.sarif"), append(sarifData, '\n'), 0o644)
}

func resolveSource(ctx context.Context, opts Options) (string, Source, error) {
	if opts.GitHub != "" {
		root, downloaded, ref, err := downloadGitHub(ctx, opts)
		if err != nil {
			return "", Source{}, err
		}
		scanRoot := root
		if opts.Subpath != "" {
			scanRoot = filepath.Join(root, filepath.FromSlash(opts.Subpath))
		}
		if _, err := os.Stat(scanRoot); err != nil {
			return "", Source{}, err
		}
		return scanRoot, Source{Mode: "github", Input: opts.GitHub, Ref: ref, Subpath: filepath.ToSlash(opts.Subpath), ScannedRoot: filepath.ToSlash(scanRoot), Downloaded: filepath.ToSlash(downloaded)}, nil
	}
	if opts.Path == "" {
		opts.Path = "."
	}
	root, err := filepath.Abs(opts.Path)
	if err != nil {
		return "", Source{}, err
	}
	return root, Source{Mode: "local", Input: filepath.ToSlash(opts.Path), ScannedRoot: filepath.ToSlash(root)}, nil
}

func downloadGitHub(ctx context.Context, opts Options) (root, downloaded, ref string, err error) {
	owner, repo, err := parseGitHubRepo(opts.GitHub)
	if err != nil {
		return "", "", "", err
	}
	ref = strings.TrimSpace(opts.Ref)
	if ref == "" {
		ref = "HEAD"
	}
	downloadDir := opts.DownloadDir
	if downloadDir == "" {
		downloadDir = filepath.Join("results", "generated", "intake", "downloads")
	}
	if err := os.MkdirAll(downloadDir, 0o755); err != nil {
		return "", "", "", err
	}
	target, err := os.MkdirTemp(downloadDir, owner+"-"+repo+"-")
	if err != nil {
		return "", "", "", err
	}
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/tarball/%s", owner, repo, ref)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", "", "", err
	}
	req.Header.Set("User-Agent", "patchline-intake")
	client := &http.Client{Timeout: 2 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return "", "", "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", "", "", fmt.Errorf("download github repo %s: %s", opts.GitHub, resp.Status)
	}
	extractedRoot, err := extractTarGz(resp.Body, target)
	if err != nil {
		return "", "", "", err
	}
	return extractedRoot, target, ref, nil
}

func parseGitHubRepo(value string) (string, string, error) {
	value = strings.TrimSpace(strings.TrimPrefix(value, "https://github.com/"))
	value = strings.TrimSuffix(value, ".git")
	parts := strings.Split(value, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" || strings.Contains(value, "..") {
		return "", "", fmt.Errorf("github repo must be owner/repo, got %q", value)
	}
	return parts[0], parts[1], nil
}

func extractTarGz(reader io.Reader, target string) (string, error) {
	gz, err := gzip.NewReader(reader)
	if err != nil {
		return "", err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	var top string
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}
		if header.Typeflag == tar.TypeXGlobalHeader || header.Typeflag == tar.TypeXHeader || header.Typeflag == tar.TypeGNULongName || header.Typeflag == tar.TypeGNULongLink {
			continue
		}
		name := filepath.Clean(header.Name)
		if strings.HasPrefix(name, "..") || filepath.IsAbs(name) {
			return "", fmt.Errorf("unsafe archive path %q", header.Name)
		}
		parts := strings.Split(name, string(filepath.Separator))
		if len(parts) == 0 || parts[0] == "." {
			continue
		}
		out := filepath.Join(target, name)
		if !strings.HasPrefix(out, filepath.Clean(target)+string(filepath.Separator)) {
			return "", fmt.Errorf("unsafe archive path %q", header.Name)
		}
		switch header.Typeflag {
		case tar.TypeDir:
			if top == "" {
				top = parts[0]
			}
			if err := os.MkdirAll(out, 0o755); err != nil {
				return "", err
			}
		case tar.TypeReg:
			if top == "" {
				top = parts[0]
			}
			if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
				return "", err
			}
			f, err := os.OpenFile(out, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, fs.FileMode(header.Mode)&0o644)
			if err != nil {
				return "", err
			}
			if _, err := io.Copy(f, tr); err != nil {
				_ = f.Close()
				return "", err
			}
			if err := f.Close(); err != nil {
				return "", err
			}
		}
	}
	if top == "" {
		return "", fmt.Errorf("github archive was empty")
	}
	return filepath.Join(target, top), nil
}

func discoverFiles(root string) ([]scanFile, error) {
	var files []scanFile
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if shouldSkipDir(entry.Name()) && path != root {
				return filepath.SkipDir
			}
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		files = append(files, scanFile{Abs: path, Rel: filepath.ToSlash(rel), Size: info.Size()})
		return nil
	})
	sort.Slice(files, func(i, j int) bool { return files[i].Rel < files[j].Rel })
	return files, err
}

func shouldSkipDir(name string) bool {
	switch name {
	case ".git", "node_modules", "vendor", "target", "dist", "build", ".next", ".venv", "__pycache__":
		return true
	default:
		return false
	}
}

func (report *Report) scanSQLFile(file scanFile, content []byte) {
	if !isSQLPath(file.Rel) {
		return
	}
	dialect := inferDialect(file.Rel, content)
	sqlReport, err := migration.AnalyzeBytesWithDialect(file.Rel, content, dialect)
	if err != nil || len(sqlReport.Statements) == 0 {
		return
	}
	report.Summary.SQLFiles++
	report.Summary.HighRiskSQLStatements += sqlReport.Summary.HighRisk
	report.Summary.MediumRiskSQLStatements += sqlReport.Summary.MediumRisk
	report.SQL = append(report.SQL, SQLFinding{Path: file.Rel, SourceKind: "sql_file", Dialect: dialect, Statements: sqlReport.Statements, Summary: sqlReport.Summary, Hash: sqlReport.Summary.ReportHash})
	report.addSQLCandidates(file.Rel, "sql_file", sqlReport.Statements)
	if sqlReport.Summary.HighRisk > 0 {
		report.Suggestions = append(report.Suggestions, Suggestion{Command: fmt.Sprintf("patchline analyze-migration %s --json", shellPath(file.Abs)), Reason: "plain SQL file contains high-risk data-change statements"})
	}
}

func isSQLPath(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return ext == ".sql" || ext == ".psql" || ext == ".ddl"
}

func inferDialect(path string, content []byte) migration.Dialect {
	lower := strings.ToLower(path + "\n" + string(content[:min(len(content), 4096)]))
	switch {
	case strings.Contains(lower, "postgres"), strings.Contains(lower, "plpgsql"), strings.Contains(lower, "create index concurrently"):
		return migration.DialectPostgres
	case strings.Contains(lower, "mysql"), strings.Contains(lower, "algorithm=copy"), strings.Contains(lower, "`"):
		return migration.DialectMySQL
	case strings.Contains(lower, "sqlite"), strings.Contains(lower, "pragma foreign_keys"):
		return migration.DialectSQLite
	case strings.Contains(lower, "sqlserver"), strings.Contains(lower, "nvarchar"), strings.Contains(lower, " top ("):
		return migration.DialectSQLServer
	default:
		return migration.DialectGeneric
	}
}

var looseSQLPattern = regexp.MustCompile(`(?i)\b(update|delete\s+from|insert\s+into|alter\s+table|drop\s+table|truncate\s+table|create\s+table)\b[^;\n]*(;)?`)

func (report *Report) scanLooseSQL(file scanFile, content []byte) {
	if isSQLPath(file.Rel) || !isTextLike(file.Rel, content) {
		return
	}
	text := string(content)
	matches := looseSQLPattern.FindAllStringIndex(text, 8)
	for _, match := range matches {
		sql := strings.TrimSpace(text[match[0]:match[1]])
		if sql == "" {
			continue
		}
		sqlReport, err := migration.AnalyzeBytes(file.Rel, []byte(sql))
		if err != nil || len(sqlReport.Statements) == 0 {
			continue
		}
		line := 1 + strings.Count(text[:match[0]], "\n")
		report.Summary.LooseSQLSnippets++
		report.Summary.HighRiskSQLStatements += sqlReport.Summary.HighRisk
		report.Summary.MediumRiskSQLStatements += sqlReport.Summary.MediumRisk
		report.SQL = append(report.SQL, SQLFinding{Path: file.Rel, SourceKind: "loose_text", Line: line, Statements: sqlReport.Statements, Summary: sqlReport.Summary, Hash: sqlReport.Summary.ReportHash})
		report.addSQLCandidates(file.Rel, "loose_text", sqlReport.Statements)
	}
}

func isTextLike(path string, content []byte) bool {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".go", ".py", ".rb", ".js", ".ts", ".java", ".cs", ".php", ".rs", ".md", ".txt", ".log", ".json", ".jsonl", ".yaml", ".yml", ".toml", ".xml":
		return true
	}
	for _, b := range content[:min(len(content), 512)] {
		if b == 0 {
			return false
		}
	}
	return true
}

func (report *Report) scanEvidence(file scanFile, content []byte) {
	ext := strings.ToLower(filepath.Ext(file.Rel))
	if ext == ".jsonl" {
		result, err := evidence.IngestJSONL(strings.NewReader(string(content)))
		if err == nil && result.EventCount > 0 {
			report.Summary.EvidenceFiles++
			report.Summary.EvidenceEvents += result.EventCount
			report.Evidence = append(report.Evidence, EvidenceFinding{Path: file.Rel, Format: "patchline-jsonl", OK: result.OK, EventCount: result.EventCount, InputHash: result.InputHash, Warnings: result.Errors})
			report.Suggestions = append(report.Suggestions, Suggestion{Command: fmt.Sprintf("patchline ingest-evidence %s --json", shellPath(file.Abs)), Reason: "JSONL already contains typed evidence events"})
			return
		}
		signals := genericJSONLinesSignals(content)
		if sumSignals(signals) > 0 {
			report.Summary.GenericEvidenceSignals += sumSignals(signals)
			report.Evidence = append(report.Evidence, EvidenceFinding{Path: file.Rel, Format: "generic-jsonl-signals", OK: true, Signals: signals})
		}
		return
	}
	if ext != ".json" {
		return
	}
	best := EvidenceFinding{}
	for _, adapter := range []string{"otlp", "datadog", "postgres", "github", "migration-runner"} {
		result, err := evidence.AdaptJSON(strings.NewReader(string(content)), adapter)
		if err != nil {
			continue
		}
		if result.EventCount > best.EventCount {
			best = EvidenceFinding{Path: file.Rel, Format: "known-export", Adapter: adapter, OK: result.OK, EventCount: result.EventCount, Warnings: result.Warnings, InputHash: result.InputHash}
		}
	}
	if best.EventCount > 0 {
		report.Summary.EvidenceFiles++
		report.Summary.EvidenceEvents += best.EventCount
		report.Evidence = append(report.Evidence, best)
		report.addCauseCandidate(file.Rel, "known-"+best.Adapter+"-export", []string{best.Adapter}, "inferred", fmt.Sprintf("existing export adapted to %d normalized evidence events", best.EventCount))
		report.Suggestions = append(report.Suggestions, Suggestion{Command: fmt.Sprintf("patchline adapt-evidence %s %s --json", best.Adapter, shellPath(file.Abs)), Reason: "existing JSON export matches a built-in adapter"})
		return
	}
	signals := genericJSONSignals(content)
	if sumSignals(signals) > 0 {
		report.Summary.EvidenceFiles++
		report.Summary.GenericEvidenceSignals += sumSignals(signals)
		report.Evidence = append(report.Evidence, EvidenceFinding{Path: file.Rel, Format: "generic-json-signals", OK: true, Signals: signals, InputHash: canonical.HashBytes(content)})
		report.addCauseCandidate(file.Rel, "generic-export-signals", signalIdentifiers(signals), "inferred", "unknown JSON contains deploy/trace/commit/migration/SQL fields")
		return
	}
	report.Summary.UnknownJSONFiles++
	report.UnknownInputs = append(report.UnknownInputs, UnknownInput{Path: file.Rel, Kind: "json", Reason: "valid or invalid JSON with no recognized SQL/deploy/trace/repair signals"})
}

func (report *Report) scanRepairManifest(file scanFile, content []byte) {
	if strings.ToLower(filepath.Ext(file.Rel)) != ".json" || !strings.Contains(string(content), "patchline.repair/v1") {
		return
	}
	manifest, err := repair.ReadManifest(strings.NewReader(string(content)))
	if err != nil {
		return
	}
	diagnostics := repair.Validate(manifest, nil)
	finding := RepairFinding{Path: file.Rel, Name: manifest.Name, Incident: manifest.Incident, Operations: len(manifest.Operations), Diagnostics: diagnostics}
	for _, d := range diagnostics {
		if d.Level == "error" {
			finding.Errors++
		}
		if d.Level == "warning" {
			finding.Warnings++
		}
	}
	report.Summary.RepairManifests++
	report.Repairs = append(report.Repairs, finding)
	ids := []string{manifest.Incident}
	if manifest.Scope.Table != "" {
		ids = append(ids, "table:"+manifest.Scope.Table)
	}
	report.addRepairCandidate(file.Rel, "patchline-repair-manifest", manifest.Scope.Table, ids, "exact", "existing repair manifest can be linted and replayed when stores are present")
	report.Suggestions = append(report.Suggestions, Suggestion{Command: fmt.Sprintf("patchline lint-repair %s --json", shellPath(file.Abs)), Reason: "repair manifest is present and can be linted immediately"})
}

func (report *Report) addSQLCandidates(path, sourceKind string, statements []migration.Statement) {
	for _, statement := range statements {
		if statement.Risk != migration.RiskHigh {
			continue
		}
		ids := identifiersForStatement(statement)
		kind := "high-risk-sql"
		if repairishPath(path) || repairishSQL(statement) {
			report.addRepairCandidate(path, "repair-like-sql", statement.Table, ids, "inferred", "SQL appears in a repair/rollback/reconcile context")
			continue
		}
		report.Problems = append(report.Problems, ProblemCandidate{
			ID:          candidateID("problem", path, statement.Fingerprint),
			Path:        path,
			Kind:        kind,
			Severity:    "high",
			Table:       statement.Table,
			Fingerprint: statement.Fingerprint,
			Identifiers: ids,
			Confidence:  "inferred",
			Rationale:   fmt.Sprintf("%s analyzer classified %s on %s as high risk", sourceKind, statement.Kind, firstNonEmpty(statement.Table, "unknown table")),
		})
		report.addCauseCandidate(path, "risky-migration-or-query", ids, "inferred", "same file contains a high-risk data-changing SQL statement")
	}
}

func (report *Report) scanTextCandidates(file scanFile, content []byte) {
	if !isTextLike(file.Rel, content) {
		return
	}
	text := string(content)
	incidentScore := incidentSignalScore(text)
	repairScore := repairSignalScore(file.Rel, text)
	ids := textIdentifiers(text)
	if incidentScore >= 2 {
		report.addCauseCandidate(file.Rel, "incident-or-postmortem-text", ids, "inferred", "text contains multiple incident/root-cause/outage signals")
	}
	if repairScore >= 2 && (incidentScore > 0 || looseSQLPattern.MatchString(text) || len(ids) > 0) {
		table := firstTableIdentifier(ids)
		report.addRepairCandidate(file.Rel, "repair-or-rollback-text", table, ids, "inferred", "text contains repair/rollback/reconcile language near operational or SQL evidence")
	}
}

func (report *Report) scanTimeSignals(file scanFile, content []byte) {
	ids := textIdentifiers(file.Rel + "\n" + string(content[:min(len(content), 4096)]))
	for _, signal := range timeSignalsFromText(file.Rel, file.Rel, "path", ids, 4) {
		report.TimeSignals = append(report.TimeSignals, signal)
	}
	if isTextLike(file.Rel, content) {
		for _, signal := range timeSignalsFromText(file.Rel, string(content), "content", ids, 4) {
			report.TimeSignals = append(report.TimeSignals, signal)
		}
	}
}

var (
	dateDashPattern  = regexp.MustCompile(`(?:^|[^0-9])((20[0-9]{2})[-_/](0[1-9]|1[0-2])[-_/]([0-2][0-9]|3[0-1]))(?:$|[^0-9])`)
	datePlainPattern = regexp.MustCompile(`(?:^|[^0-9])((20[0-9]{2})(0[1-9]|1[0-2])([0-2][0-9]|3[0-1])(?:[0-2][0-9][0-5][0-9][0-5][0-9])?)(?:$|[^0-9])`)
)

func timeSignalsFromText(path, text, source string, ids []string, limit int) []TimeSignal {
	seen := map[string]bool{}
	var out []TimeSignal
	add := func(raw, year, month, day string) {
		if len(out) >= limit {
			return
		}
		normalized := year + "-" + month + "-" + day
		if _, err := time.Parse("2006-01-02", normalized); err != nil {
			return
		}
		key := source + "\x00" + normalized
		if seen[key] {
			return
		}
		seen[key] = true
		out = append(out, TimeSignal{
			ID:          candidateID("time", path, source+raw),
			Path:        path,
			Timestamp:   normalized,
			Source:      source,
			Confidence:  "temporal",
			Identifiers: ids,
		})
	}
	for _, match := range dateDashPattern.FindAllStringSubmatch(text, limit) {
		add(match[1], match[2], match[3], match[4])
	}
	for _, match := range datePlainPattern.FindAllStringSubmatch(text, limit) {
		add(match[1], match[2], match[3], match[4])
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Timestamp != out[j].Timestamp {
			return out[i].Timestamp < out[j].Timestamp
		}
		return out[i].Source < out[j].Source
	})
	return out
}

func identifiersForStatement(statement migration.Statement) []string {
	var ids []string
	if statement.Table != "" {
		ids = append(ids, "table:"+statement.Table)
	}
	if statement.Fingerprint != "" {
		ids = append(ids, "sql:"+statement.Fingerprint)
	}
	return stableStrings(ids)
}

func textIdentifiers(text string) []string {
	var ids []string
	for _, match := range commitPattern.FindAllStringSubmatch(text, 8) {
		ids = append(ids, "commit:"+match[1])
	}
	for _, match := range tablePattern.FindAllStringSubmatch(text, 12) {
		ids = append(ids, "table:"+strings.ToLower(match[1]))
	}
	for _, match := range incidentPattern.FindAllStringSubmatch(text, 8) {
		ids = append(ids, "incident:"+strings.ToLower(match[1]))
	}
	return stableStrings(ids)
}

var (
	commitPattern   = regexp.MustCompile(`(?i)\b(?:commit|sha|revision|rev)[\s:=#-]+([a-f0-9]{7,40})\b`)
	tablePattern    = regexp.MustCompile(`(?i)\b(?:table|relation|collection)[\s:=#-]+([a-zA-Z_][a-zA-Z0-9_.]*)\b`)
	incidentPattern = regexp.MustCompile(`(?i)\b(?:incident|outage|sev)[\s:=#-]+([a-zA-Z0-9_.-]+)\b`)
	repairWords     = regexp.MustCompile(`(?i)\b(rollback|roll back|revert|restore|repair|fix|backfill|reconcile|compensat(?:e|ing)|recover|remediate)\b`)
	incidentWords   = regexp.MustCompile(`(?i)\b(incident|outage|root cause|postmortem|data loss|corrupt(?:ed|ion)?|bad migration|failed deploy|rollback|remediation)\b`)
)

func incidentSignalScore(text string) int {
	return len(incidentWords.FindAllStringIndex(text, 5))
}

func repairSignalScore(path, text string) int {
	score := len(repairWords.FindAllStringIndex(text, 5))
	if repairishPath(path) {
		score++
	}
	return score
}

func repairishPath(path string) bool {
	lower := strings.ToLower(path)
	return strings.Contains(lower, "repair") || strings.Contains(lower, "rollback") || strings.Contains(lower, "revert") || strings.Contains(lower, "restore") || strings.Contains(lower, "fix") || strings.Contains(lower, "reconcile")
}

func repairishSQL(statement migration.Statement) bool {
	lower := strings.ToLower(strings.Join(statement.Reasons, " ") + " " + statement.Fingerprint)
	return strings.Contains(lower, "restore") || strings.Contains(lower, "revert") || strings.Contains(lower, "repair") || strings.Contains(lower, "backfill")
}

func signalIdentifiers(signals map[string]int) []string {
	var ids []string
	for key, count := range signals {
		if count > 0 {
			ids = append(ids, "signal:"+key)
		}
	}
	return stableStrings(ids)
}

func firstTableIdentifier(ids []string) string {
	for _, id := range ids {
		if strings.HasPrefix(id, "table:") {
			return strings.TrimPrefix(id, "table:")
		}
	}
	return ""
}

func (report *Report) addCauseCandidate(path, kind string, ids []string, confidence, rationale string) {
	ids = stableStrings(ids)
	report.Causes = append(report.Causes, CauseCandidate{
		ID:          candidateID("cause", path, kind+strings.Join(ids, ",")),
		Path:        path,
		Kind:        kind,
		Identifiers: ids,
		Confidence:  confidence,
		Rationale:   rationale,
	})
}

func (report *Report) addRepairCandidate(path, kind, table string, ids []string, confidence, rationale string) {
	ids = stableStrings(ids)
	report.RepairCandidates = append(report.RepairCandidates, RepairCandidate{
		ID:          candidateID("repair", path, kind+strings.Join(ids, ",")),
		Path:        path,
		Kind:        kind,
		Table:       table,
		Identifiers: ids,
		Confidence:  confidence,
		Rationale:   rationale,
	})
}

func candidateID(prefix, path, seed string) string {
	return prefix + ":" + canonical.Hash(filepath.ToSlash(path) + "\x00" + seed)[:16]
}

func stableStrings(values []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func genericJSONLinesSignals(content []byte) map[string]int {
	signals := map[string]int{}
	for _, line := range strings.Split(string(content), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var value any
		if json.Unmarshal([]byte(line), &value) == nil {
			collectSignals(value, signals)
		}
	}
	return signals
}

func genericJSONSignals(content []byte) map[string]int {
	var value any
	if json.Unmarshal(content, &value) != nil {
		return nil
	}
	signals := map[string]int{}
	collectSignals(value, signals)
	return signals
}

func collectSignals(value any, signals map[string]int) {
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			classifyKeyValueSignal(key, child, signals)
			collectSignals(child, signals)
		}
	case []any:
		for _, child := range typed {
			collectSignals(child, signals)
		}
	case string:
		if looksLikeSQLString(typed) {
			signals["sql_strings"]++
		}
	}
}

func classifyKeyValueSignal(key string, value any, signals map[string]int) {
	lower := strings.ToLower(key)
	text := fmt.Sprint(value)
	if strings.Contains(lower, "sql") || strings.Contains(lower, "query") || strings.Contains(lower, "statement") || looksLikeSQLString(text) {
		signals["sql_fields"]++
	}
	if strings.Contains(lower, "trace") || strings.Contains(lower, "span") {
		signals["trace_fields"]++
	}
	if strings.Contains(lower, "deploy") || strings.Contains(lower, "release") {
		signals["deploy_fields"]++
	}
	if strings.Contains(lower, "commit") || lower == "sha" {
		signals["commit_fields"]++
	}
	if strings.Contains(lower, "migration") {
		signals["migration_fields"]++
	}
	if strings.Contains(lower, "record") || strings.Contains(lower, "row") || strings.Contains(lower, "entity") {
		signals["record_fields"]++
	}
}

func looksLikeSQLString(value string) bool {
	return looseSQLPattern.MatchString(value)
}

func sumSignals(signals map[string]int) int {
	var total int
	for _, n := range signals {
		total += n
	}
	return total
}

func (report *Report) finalize() {
	report.Problems = uniqueProblems(report.Problems)
	report.Causes = uniqueCauses(report.Causes)
	report.RepairCandidates = uniqueRepairCandidates(report.RepairCandidates)
	report.TimeSignals = uniqueTimeSignals(report.TimeSignals)
	report.Links = buildCandidateLinks(report.Problems, report.Causes, report.RepairCandidates)
	report.Summary.ProblemCandidates = len(report.Problems)
	report.Summary.CauseCandidates = len(report.Causes)
	report.Summary.RepairCandidates = len(report.RepairCandidates)
	report.Summary.LinkedCandidates = len(report.Links)
	report.Summary.TimeSignals = len(report.TimeSignals)
	sort.Slice(report.SQL, func(i, j int) bool {
		if report.SQL[i].Path != report.SQL[j].Path {
			return report.SQL[i].Path < report.SQL[j].Path
		}
		return report.SQL[i].SourceKind < report.SQL[j].SourceKind
	})
	sort.Slice(report.Evidence, func(i, j int) bool { return report.Evidence[i].Path < report.Evidence[j].Path })
	sort.Slice(report.Repairs, func(i, j int) bool { return report.Repairs[i].Path < report.Repairs[j].Path })
	sort.Slice(report.TimeSignals, func(i, j int) bool {
		if report.TimeSignals[i].Timestamp != report.TimeSignals[j].Timestamp {
			return report.TimeSignals[i].Timestamp < report.TimeSignals[j].Timestamp
		}
		if report.TimeSignals[i].Path != report.TimeSignals[j].Path {
			return report.TimeSignals[i].Path < report.TimeSignals[j].Path
		}
		return report.TimeSignals[i].Source < report.TimeSignals[j].Source
	})
	report.Suggestions = uniqueSuggestions(report.Suggestions)
	report.Hash = hashReport(*report)
	report.Markdown = renderMarkdown(*report)
}

func uniqueProblems(in []ProblemCandidate) []ProblemCandidate {
	seen := map[string]bool{}
	var out []ProblemCandidate
	for _, item := range in {
		if item.ID == "" || seen[item.ID] {
			continue
		}
		seen[item.ID] = true
		out = append(out, item)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Path != out[j].Path {
			return out[i].Path < out[j].Path
		}
		return out[i].ID < out[j].ID
	})
	return out
}

func uniqueCauses(in []CauseCandidate) []CauseCandidate {
	seen := map[string]bool{}
	var out []CauseCandidate
	for _, item := range in {
		if item.ID == "" || seen[item.ID] {
			continue
		}
		seen[item.ID] = true
		out = append(out, item)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Path != out[j].Path {
			return out[i].Path < out[j].Path
		}
		return out[i].ID < out[j].ID
	})
	return out
}

func uniqueRepairCandidates(in []RepairCandidate) []RepairCandidate {
	seen := map[string]bool{}
	var out []RepairCandidate
	for _, item := range in {
		if item.ID == "" || seen[item.ID] {
			continue
		}
		seen[item.ID] = true
		out = append(out, item)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Path != out[j].Path {
			return out[i].Path < out[j].Path
		}
		return out[i].ID < out[j].ID
	})
	return out
}

func uniqueTimeSignals(in []TimeSignal) []TimeSignal {
	seen := map[string]bool{}
	var out []TimeSignal
	for _, item := range in {
		if item.ID == "" || seen[item.ID] {
			continue
		}
		seen[item.ID] = true
		sort.Strings(item.Identifiers)
		out = append(out, item)
	}
	return out
}

func buildCandidateLinks(problems []ProblemCandidate, causes []CauseCandidate, repairs []RepairCandidate) []CandidateLink {
	var links []CandidateLink
	for _, cause := range causes {
		for _, problem := range problems {
			ids := sharedIdentifiers(cause.Identifiers, problem.Identifiers)
			if len(ids) == 0 {
				continue
			}
			links = append(links, CandidateLink{From: cause.ID, To: problem.ID, Kind: "possible-cause-for-problem", Identifiers: ids, Confidence: linkConfidence(ids)})
		}
	}
	for _, repair := range repairs {
		for _, problem := range problems {
			ids := sharedIdentifiers(repair.Identifiers, problem.Identifiers)
			if len(ids) == 0 {
				continue
			}
			links = append(links, CandidateLink{From: repair.ID, To: problem.ID, Kind: "possible-repair-for-problem", Identifiers: ids, Confidence: linkConfidence(ids)})
		}
	}
	for _, repair := range repairs {
		for _, cause := range causes {
			ids := sharedIdentifiers(repair.Identifiers, cause.Identifiers)
			if len(ids) == 0 {
				continue
			}
			links = append(links, CandidateLink{From: cause.ID, To: repair.ID, Kind: "cause-repair-shared-evidence", Identifiers: ids, Confidence: linkConfidence(ids)})
		}
	}
	sort.Slice(links, func(i, j int) bool {
		left := links[i].Kind + "\x00" + links[i].From + "\x00" + links[i].To
		right := links[j].Kind + "\x00" + links[j].From + "\x00" + links[j].To
		return left < right
	})
	return links
}

func sharedIdentifiers(left, right []string) []string {
	rightSet := map[string]bool{}
	for _, value := range right {
		if strings.HasPrefix(value, "signal:") || strings.HasPrefix(value, "sql:") {
			continue
		}
		rightSet[value] = true
	}
	var shared []string
	for _, value := range left {
		if strings.HasPrefix(value, "signal:") || strings.HasPrefix(value, "sql:") {
			continue
		}
		if rightSet[value] {
			shared = append(shared, value)
		}
	}
	return stableStrings(shared)
}

func linkConfidence(ids []string) string {
	for _, id := range ids {
		if strings.HasPrefix(id, "table:") || strings.HasPrefix(id, "incident:") {
			return "causal"
		}
	}
	return "inferred"
}

func uniqueSuggestions(in []Suggestion) []Suggestion {
	seen := map[string]bool{}
	var out []Suggestion
	for _, s := range in {
		if s.Command == "" || seen[s.Command] {
			continue
		}
		seen[s.Command] = true
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Command < out[j].Command })
	return out
}

func hashReport(report Report) string {
	copy := report
	copy.Hash = ""
	copy.Markdown = ""
	return canonical.Hash(copy)
}

func renderMarkdown(report Report) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Patchline intake\n\n")
	fmt.Fprintf(&b, "- source: `%s`\n", report.Source.Input)
	fmt.Fprintf(&b, "- scanned_root: `%s`\n", report.Source.ScannedRoot)
	fmt.Fprintf(&b, "- hash: `%s`\n\n", report.Hash)
	fmt.Fprintf(&b, "## What can run now\n\n")
	fmt.Fprintf(&b, "| area | count |\n| --- | ---: |\n")
	fmt.Fprintf(&b, "| files scanned | %d |\n", report.Summary.FilesScanned)
	fmt.Fprintf(&b, "| SQL files | %d |\n", report.Summary.SQLFiles)
	fmt.Fprintf(&b, "| loose SQL snippets | %d |\n", report.Summary.LooseSQLSnippets)
	fmt.Fprintf(&b, "| high-risk SQL statements | %d |\n", report.Summary.HighRiskSQLStatements)
	fmt.Fprintf(&b, "| source SQL observations | %d |\n", report.Summary.SourceSQLObservations)
	fmt.Fprintf(&b, "| evidence/export files | %d |\n", report.Summary.EvidenceFiles)
	fmt.Fprintf(&b, "| generic evidence signals | %d |\n", report.Summary.GenericEvidenceSignals)
	fmt.Fprintf(&b, "| repair manifests | %d |\n", report.Summary.RepairManifests)
	fmt.Fprintf(&b, "| problem candidates | %d |\n", report.Summary.ProblemCandidates)
	fmt.Fprintf(&b, "| cause candidates | %d |\n", report.Summary.CauseCandidates)
	fmt.Fprintf(&b, "| repair candidates | %d |\n", report.Summary.RepairCandidates)
	fmt.Fprintf(&b, "| linked candidates | %d |\n", report.Summary.LinkedCandidates)
	fmt.Fprintf(&b, "| time signals | %d |\n\n", report.Summary.TimeSignals)
	if len(report.Suggestions) > 0 {
		fmt.Fprintf(&b, "## Commands that should run on this checkout/export\n\n")
		for _, suggestion := range report.Suggestions {
			fmt.Fprintf(&b, "- `%s` — %s\n", suggestion.Command, suggestion.Reason)
		}
		fmt.Fprintf(&b, "\n")
	}
	if len(report.SQL) > 0 {
		fmt.Fprintf(&b, "## SQL findings\n\n| path | kind | high | medium | statements |\n| --- | --- | ---: | ---: | ---: |\n")
		for _, finding := range report.SQL {
			fmt.Fprintf(&b, "| %s | %s | %d | %d | %d |\n", finding.Path, finding.SourceKind, finding.Summary.HighRisk, finding.Summary.MediumRisk, finding.Summary.TotalStatements)
		}
		fmt.Fprintf(&b, "\n")
	}
	if len(report.Problems) > 0 {
		fmt.Fprintf(&b, "## Problem candidates\n\n| id | path | severity | table | confidence | rationale |\n| --- | --- | --- | --- | --- | --- |\n")
		for _, problem := range report.Problems {
			fmt.Fprintf(&b, "| `%s` | %s | %s | %s | %s | %s |\n", problem.ID, problem.Path, problem.Severity, problem.Table, problem.Confidence, problem.Rationale)
		}
		fmt.Fprintf(&b, "\n")
	}
	if len(report.Causes) > 0 {
		fmt.Fprintf(&b, "## Cause candidates\n\n| id | path | kind | confidence | rationale |\n| --- | --- | --- | --- | --- |\n")
		for _, cause := range report.Causes {
			fmt.Fprintf(&b, "| `%s` | %s | %s | %s | %s |\n", cause.ID, cause.Path, cause.Kind, cause.Confidence, cause.Rationale)
		}
		fmt.Fprintf(&b, "\n")
	}
	if len(report.RepairCandidates) > 0 {
		fmt.Fprintf(&b, "## Repair candidates\n\n| id | path | kind | table | confidence | rationale |\n| --- | --- | --- | --- | --- | --- |\n")
		for _, repair := range report.RepairCandidates {
			fmt.Fprintf(&b, "| `%s` | %s | %s | %s | %s | %s |\n", repair.ID, repair.Path, repair.Kind, repair.Table, repair.Confidence, repair.Rationale)
		}
		fmt.Fprintf(&b, "\n")
	}
	if len(report.Links) > 0 {
		fmt.Fprintf(&b, "## Candidate links\n\nThese are identifier-grounded links, not proof of causality.\n\n| kind | from | to | identifiers | confidence |\n| --- | --- | --- | --- | --- |\n")
		for _, link := range report.Links {
			fmt.Fprintf(&b, "| %s | `%s` | `%s` | %s | %s |\n", link.Kind, link.From, link.To, strings.Join(link.Identifiers, ", "), link.Confidence)
		}
		fmt.Fprintf(&b, "\n")
	}
	if len(report.TimeSignals) > 0 {
		fmt.Fprintf(&b, "## Time signals\n\nDates found in existing filenames and text can help slice a checkout/export around likely migration, incident, deploy, or repair windows.\n\n| date | path | source | identifiers |\n| --- | --- | --- | --- |\n")
		for _, signal := range report.TimeSignals {
			fmt.Fprintf(&b, "| %s | %s | %s | %s |\n", signal.Timestamp, signal.Path, signal.Source, strings.Join(signal.Identifiers, ", "))
		}
		fmt.Fprintf(&b, "\n")
	}
	if len(report.Evidence) > 0 {
		fmt.Fprintf(&b, "## Existing evidence/export signals\n\n| path | format | adapter | events | generic signals |\n| --- | --- | --- | ---: | ---: |\n")
		for _, finding := range report.Evidence {
			fmt.Fprintf(&b, "| %s | %s | %s | %d | %d |\n", finding.Path, finding.Format, finding.Adapter, finding.EventCount, sumSignals(finding.Signals))
		}
	}
	return b.String()
}

type sarifLog struct {
	Version string     `json:"version"`
	Schema  string     `json:"$schema"`
	Runs    []sarifRun `json:"runs"`
}

type sarifRun struct {
	Tool    sarifTool     `json:"tool"`
	Results []sarifResult `json:"results"`
}

type sarifTool struct {
	Driver sarifDriver `json:"driver"`
}

type sarifDriver struct {
	Name           string      `json:"name"`
	InformationURI string      `json:"informationUri,omitempty"`
	Rules          []sarifRule `json:"rules"`
}

type sarifRule struct {
	ID               string       `json:"id"`
	Name             string       `json:"name"`
	ShortDescription sarifMessage `json:"shortDescription"`
	FullDescription  sarifMessage `json:"fullDescription"`
}

type sarifResult struct {
	RuleID              string            `json:"ruleId"`
	Level               string            `json:"level"`
	Message             sarifMessage      `json:"message"`
	Locations           []sarifLocation   `json:"locations,omitempty"`
	PartialFingerprints map[string]string `json:"partialFingerprints,omitempty"`
	Properties          map[string]any    `json:"properties,omitempty"`
}

type sarifMessage struct {
	Text string `json:"text"`
}

type sarifLocation struct {
	PhysicalLocation sarifPhysicalLocation `json:"physicalLocation"`
}

type sarifPhysicalLocation struct {
	ArtifactLocation sarifArtifactLocation `json:"artifactLocation"`
}

type sarifArtifactLocation struct {
	URI string `json:"uri"`
}

func renderSARIF(report Report) sarifLog {
	results := make([]sarifResult, 0, len(report.Problems)+len(report.Causes)+len(report.RepairCandidates))
	for _, problem := range report.Problems {
		results = append(results, sarifResult{
			RuleID:              "patchline.problem.high-risk-sql",
			Level:               "warning",
			Message:             sarifMessage{Text: problem.Rationale},
			Locations:           []sarifLocation{sarifFileLocation(problem.Path)},
			PartialFingerprints: map[string]string{"patchlineCandidateId": problem.ID},
			Properties: map[string]any{
				"kind":        problem.Kind,
				"severity":    problem.Severity,
				"table":       problem.Table,
				"confidence":  problem.Confidence,
				"identifiers": problem.Identifiers,
			},
		})
	}
	for _, cause := range report.Causes {
		results = append(results, sarifResult{
			RuleID:              "patchline.cause.candidate",
			Level:               "note",
			Message:             sarifMessage{Text: cause.Rationale},
			Locations:           []sarifLocation{sarifFileLocation(cause.Path)},
			PartialFingerprints: map[string]string{"patchlineCandidateId": cause.ID},
			Properties: map[string]any{
				"kind":        cause.Kind,
				"confidence":  cause.Confidence,
				"identifiers": cause.Identifiers,
			},
		})
	}
	for _, repair := range report.RepairCandidates {
		results = append(results, sarifResult{
			RuleID:              "patchline.repair.candidate",
			Level:               "note",
			Message:             sarifMessage{Text: repair.Rationale},
			Locations:           []sarifLocation{sarifFileLocation(repair.Path)},
			PartialFingerprints: map[string]string{"patchlineCandidateId": repair.ID},
			Properties: map[string]any{
				"kind":        repair.Kind,
				"table":       repair.Table,
				"confidence":  repair.Confidence,
				"identifiers": repair.Identifiers,
			},
		})
	}
	return sarifLog{
		Version: "2.1.0",
		Schema:  "https://json.schemastore.org/sarif-2.1.0.json",
		Runs: []sarifRun{{
			Tool: sarifTool{Driver: sarifDriver{
				Name: "Patchline",
				Rules: []sarifRule{
					{ID: "patchline.problem.high-risk-sql", Name: "High-risk data-changing SQL", ShortDescription: sarifMessage{Text: "Patchline found high-risk SQL in existing project data."}, FullDescription: sarifMessage{Text: "High-risk SQL findings are derived from deterministic migration/source SQL analysis over the scanned project."}},
					{ID: "patchline.cause.candidate", Name: "Cause candidate", ShortDescription: sarifMessage{Text: "Patchline found a possible cause signal."}, FullDescription: sarifMessage{Text: "Cause candidates are leads from risky migrations, deploy/trace/commit/migration signals, or incident-like text; they are not proof of causality."}},
					{ID: "patchline.repair.candidate", Name: "Repair candidate", ShortDescription: sarifMessage{Text: "Patchline found a possible repair or rollback signal."}, FullDescription: sarifMessage{Text: "Repair candidates are leads from manifests, SQL, scripts, or docs with repair/rollback/reconcile evidence."}},
				},
			}},
			Results: results,
		}},
	}
}

func sarifFileLocation(path string) sarifLocation {
	return sarifLocation{PhysicalLocation: sarifPhysicalLocation{ArtifactLocation: sarifArtifactLocation{URI: filepath.ToSlash(path)}}}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func shellPath(path string) string {
	if path == "" {
		return "."
	}
	if strings.ContainsAny(path, " \t\n'\"") {
		return strconvQuote(path)
	}
	return path
}

func strconvQuote(path string) string {
	escaped := strings.ReplaceAll(path, `'`, `'\''`)
	return "'" + escaped + "'"
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
