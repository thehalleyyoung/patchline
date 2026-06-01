package project

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const Version = "patchline.project/v1"

type FetchOptions struct {
	Input       string
	Ref         string
	Subpath     string
	OutDir      string
	DownloadDir string
	Full        bool
}

type Source struct {
	Version     string   `json:"version"`
	Mode        string   `json:"mode"`
	Input       string   `json:"input"`
	Owner       string   `json:"owner,omitempty"`
	Repo        string   `json:"repo,omitempty"`
	Ref         string   `json:"ref,omitempty"`
	CommitHint  string   `json:"commit_hint,omitempty"`
	Subpath     string   `json:"subpath,omitempty"`
	ArchiveHash string   `json:"archive_hash,omitempty"`
	OutDir      string   `json:"out_dir"`
	ScannedRoot string   `json:"scanned_root"`
	SkippedDirs []string `json:"skipped_dirs,omitempty"`
}

type FetchResult struct {
	Source Source `json:"source"`
}

type InventoryOptions struct {
	Path string
	Full bool
}

type Inventory struct {
	Version           string         `json:"version"`
	Root              string         `json:"root"`
	FilesScanned      int            `json:"files_scanned"`
	BytesScanned      int64          `json:"bytes_scanned"`
	Languages         []Count        `json:"languages,omitempty"`
	Frameworks        []Finding      `json:"frameworks,omitempty"`
	MigrationSystems  []Finding      `json:"migration_systems,omitempty"`
	MigrationRoots    []Finding      `json:"migration_roots,omitempty"`
	CI                []Finding      `json:"ci,omitempty"`
	DeployConfig      []Finding      `json:"deploy_config,omitempty"`
	TestCommands      []Command      `json:"test_commands,omitempty"`
	SourceSQLHints    []Finding      `json:"source_sql_hints,omitempty"`
	OperationalDocs   []Finding      `json:"operational_docs,omitempty"`
	EvidenceExports   []Finding      `json:"evidence_exports,omitempty"`
	NextCommands      []Command      `json:"next_commands,omitempty"`
	SkippedDirs       []string       `json:"skipped_dirs,omitempty"`
	SummaryByCategory map[string]int `json:"summary_by_category,omitempty"`
	Markdown          string         `json:"markdown,omitempty"`
}

type Count struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

type Finding struct {
	Kind       string `json:"kind"`
	Path       string `json:"path"`
	Confidence string `json:"confidence"`
	Rationale  string `json:"rationale"`
}

type Command struct {
	Command string `json:"command"`
	Reason  string `json:"reason"`
}

type scanFile struct {
	Abs  string
	Rel  string
	Size int64
}

func Fetch(ctx context.Context, opts FetchOptions) (FetchResult, error) {
	if strings.TrimSpace(opts.Input) == "" {
		return FetchResult{}, fmt.Errorf("repo fetch input is required")
	}
	outDir := opts.OutDir
	if outDir == "" {
		outDir = filepath.Join("results", "generated", "repo", safeSlug(opts.Input))
	}
	if err := prepareOutDir(outDir); err != nil {
		return FetchResult{}, err
	}
	source := Source{
		Version: Version,
		Input:   opts.Input,
		Ref:     firstNonEmpty(strings.TrimSpace(opts.Ref), "HEAD"),
		Subpath: filepath.ToSlash(strings.TrimSpace(opts.Subpath)),
		OutDir:  filepath.ToSlash(outDir),
	}
	skipped := map[string]bool{}
	input := strings.TrimSpace(opts.Input)
	switch {
	case isGitHubInput(input):
		owner, repo, err := ParseGitHubRepo(input)
		if err != nil {
			return FetchResult{}, err
		}
		source.Mode, source.Owner, source.Repo = "github", owner, repo
		root, archiveHash, commitHint, err := fetchGitHubArchive(ctx, owner, repo, source.Ref, outDir)
		if err != nil {
			return FetchResult{}, err
		}
		source.ArchiveHash = archiveHash
		source.CommitHint = commitHint
		source.ScannedRoot = filepath.ToSlash(root)
	case isHTTPArchive(input):
		source.Mode = "archive-url"
		root, archiveHash, err := fetchArchiveURL(ctx, input, outDir)
		if err != nil {
			return FetchResult{}, err
		}
		source.ArchiveHash = archiveHash
		source.ScannedRoot = filepath.ToSlash(root)
	case isArchivePath(input):
		source.Mode = "archive"
		root, archiveHash, err := extractArchivePath(input, outDir)
		if err != nil {
			return FetchResult{}, err
		}
		source.ArchiveHash = archiveHash
		source.ScannedRoot = filepath.ToSlash(root)
	default:
		root, err := filepath.Abs(input)
		if err != nil {
			return FetchResult{}, err
		}
		info, err := os.Stat(root)
		if err != nil {
			return FetchResult{}, err
		}
		if !info.IsDir() {
			return FetchResult{}, fmt.Errorf("input %q is not a directory or supported archive", input)
		}
		source.Mode = "local"
		if err := copyDir(root, outDir, skipped); err != nil {
			return FetchResult{}, err
		}
		source.ScannedRoot = filepath.ToSlash(outDir)
		source.Ref = ""
	}
	scanRoot, err := containedSubpath(source.ScannedRoot, opts.Subpath)
	if err != nil {
		return FetchResult{}, err
	}
	source.ScannedRoot = filepath.ToSlash(scanRoot)
	source.SkippedDirs = sortedKeys(skipped)
	if err := writeSource(outDir, source); err != nil {
		return FetchResult{}, err
	}
	return FetchResult{Source: source}, nil
}

func InventoryPath(opts InventoryOptions) (Inventory, error) {
	root := opts.Path
	if root == "" {
		root = "."
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return Inventory{}, err
	}
	files, skipped, err := discoverFiles(abs, opts.Full)
	if err != nil {
		return Inventory{}, err
	}
	inv := Inventory{Version: Version, Root: filepath.ToSlash(abs), FilesScanned: len(files), SkippedDirs: sortedKeys(skipped), SummaryByCategory: map[string]int{}}
	inv.inspectRoot(abs)
	languages := map[string]int{}
	for _, file := range files {
		inv.BytesScanned += file.Size
		lang := languageFor(file.Rel)
		if lang != "" {
			languages[lang]++
		}
		inv.inspectFile(file)
	}
	inv.Languages = countsFromMap(languages)
	inv.finalize()
	return inv, nil
}

func (inv *Inventory) inspectRoot(root string) {
	lower := strings.ToLower(filepath.ToSlash(root))
	switch {
	case strings.HasSuffix(lower, "/db/migrate") || strings.Contains(lower, "/db/migrate/"):
		inv.MigrationRoots = append(inv.MigrationRoots, Finding{Kind: "rails-or-generic-db-migrate", Path: ".", Confidence: "path", Rationale: "inventory root is a db/migrate migration path"})
	case strings.HasSuffix(lower, "/migrations") || strings.Contains(lower, "/migrations/"):
		inv.MigrationRoots = append(inv.MigrationRoots, Finding{Kind: "migrations", Path: ".", Confidence: "path", Rationale: "inventory root is a migrations path"})
	case strings.HasSuffix(lower, "/migration") || strings.Contains(lower, "/migration/"):
		inv.MigrationRoots = append(inv.MigrationRoots, Finding{Kind: "migration", Path: ".", Confidence: "path", Rationale: "inventory root is a migration path"})
	}
}

func WriteInventory(outDir string, inv Inventory) error {
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}
	copy := inv
	copy.Markdown = ""
	data, err := json.MarshalIndent(copy, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(outDir, "inventory.json"), append(data, '\n'), 0o644); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(outDir, "inventory.md"), []byte(inv.Markdown), 0o644)
}

func ParseGitHubRepo(value string) (string, string, error) {
	value = strings.TrimSpace(value)
	value = strings.TrimPrefix(value, "https://github.com/")
	value = strings.TrimPrefix(value, "http://github.com/")
	value = strings.TrimSuffix(value, ".git")
	value = strings.Trim(value, "/")
	parts := strings.Split(value, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" || strings.Contains(value, "..") {
		return "", "", fmt.Errorf("github repo must be owner/repo, got %q", value)
	}
	return parts[0], parts[1], nil
}

func isGitHubInput(input string) bool {
	if strings.HasPrefix(input, "https://github.com/") || strings.HasPrefix(input, "http://github.com/") {
		return true
	}
	if _, _, err := ParseGitHubRepo(input); err != nil {
		return false
	}
	if _, err := os.Stat(input); err == nil {
		return false
	}
	return true
}

func fetchGitHubArchive(ctx context.Context, owner, repo, ref, outDir string) (string, string, string, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/tarball/%s", owner, repo, ref)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", "", "", err
	}
	req.Header.Set("User-Agent", "patchline-repo")
	if token := os.Getenv("GITHUB_TOKEN"); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	client := &http.Client{Timeout: 2 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return "", "", "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", "", "", fmt.Errorf("download github repo %s/%s: %s", owner, repo, resp.Status)
	}
	hasher := sha256.New()
	root, top, err := extractTarGz(io.TeeReader(resp.Body, hasher), outDir)
	if err != nil {
		return "", "", "", err
	}
	return root, "sha256:" + hex.EncodeToString(hasher.Sum(nil)), commitHintFromTop(top), nil
}

func fetchArchiveURL(ctx context.Context, url, outDir string) (string, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", "", err
	}
	req.Header.Set("User-Agent", "patchline-repo")
	client := &http.Client{Timeout: 2 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", "", fmt.Errorf("download archive %s: %s", url, resp.Status)
	}
	hasher := sha256.New()
	if strings.HasSuffix(strings.ToLower(url), ".zip") {
		tmp, err := os.CreateTemp("", "patchline-archive-*.zip")
		if err != nil {
			return "", "", err
		}
		tmpPath := tmp.Name()
		defer os.Remove(tmpPath)
		if _, err := io.Copy(tmp, io.TeeReader(resp.Body, hasher)); err != nil {
			_ = tmp.Close()
			return "", "", err
		}
		if err := tmp.Close(); err != nil {
			return "", "", err
		}
		root, err := extractZip(tmpPath, outDir)
		return root, "sha256:" + hex.EncodeToString(hasher.Sum(nil)), err
	}
	root, _, err := extractTarGz(io.TeeReader(resp.Body, hasher), outDir)
	return root, "sha256:" + hex.EncodeToString(hasher.Sum(nil)), err
}

func extractArchivePath(path, outDir string) (string, string, error) {
	sum, err := hashFile(path)
	if err != nil {
		return "", "", err
	}
	lower := strings.ToLower(path)
	if strings.HasSuffix(lower, ".zip") {
		root, err := extractZip(path, outDir)
		return root, sum, err
	}
	file, err := os.Open(path)
	if err != nil {
		return "", "", err
	}
	defer file.Close()
	root, _, err := extractTarGz(file, outDir)
	return root, sum, err
}

func prepareOutDir(outDir string) error {
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}
	entries, err := os.ReadDir(outDir)
	if err != nil {
		return err
	}
	if len(entries) > 0 {
		return fmt.Errorf("output directory %s is not empty", outDir)
	}
	return nil
}

func writeSource(outDir string, source Source) error {
	data, err := json.MarshalIndent(source, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(outDir, "source.json"), append(data, '\n'), 0o644)
}

func extractTarGz(reader io.Reader, target string) (string, string, error) {
	gz, err := gzip.NewReader(reader)
	if err != nil {
		return "", "", err
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
			return "", "", err
		}
		if header.Typeflag == tar.TypeXGlobalHeader || header.Typeflag == tar.TypeXHeader || header.Typeflag == tar.TypeGNULongName || header.Typeflag == tar.TypeGNULongLink {
			continue
		}
		name := filepath.Clean(header.Name)
		if strings.HasPrefix(name, "..") || filepath.IsAbs(name) {
			return "", "", fmt.Errorf("unsafe archive path %q", header.Name)
		}
		parts := strings.Split(name, string(filepath.Separator))
		if len(parts) == 0 || parts[0] == "." {
			continue
		}
		out := filepath.Join(target, name)
		if !contained(target, out) {
			return "", "", fmt.Errorf("unsafe archive path %q", header.Name)
		}
		switch header.Typeflag {
		case tar.TypeDir:
			if top == "" {
				top = parts[0]
			}
			if err := os.MkdirAll(out, 0o755); err != nil {
				return "", "", err
			}
		case tar.TypeReg:
			if top == "" {
				top = parts[0]
			}
			if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
				return "", "", err
			}
			f, err := os.OpenFile(out, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, fs.FileMode(header.Mode)&0o644)
			if err != nil {
				return "", "", err
			}
			if _, err := io.Copy(f, tr); err != nil {
				_ = f.Close()
				return "", "", err
			}
			if err := f.Close(); err != nil {
				return "", "", err
			}
		}
	}
	if top == "" {
		return "", "", fmt.Errorf("archive was empty")
	}
	return filepath.Join(target, top), top, nil
}

func extractZip(path, target string) (string, error) {
	zr, err := zip.OpenReader(path)
	if err != nil {
		return "", err
	}
	defer zr.Close()
	var top string
	for _, file := range zr.File {
		name := filepath.Clean(file.Name)
		if strings.HasPrefix(name, "..") || filepath.IsAbs(name) {
			return "", fmt.Errorf("unsafe archive path %q", file.Name)
		}
		parts := strings.Split(name, string(filepath.Separator))
		if len(parts) == 0 || parts[0] == "." {
			continue
		}
		mode := file.FileInfo().Mode()
		if mode&os.ModeSymlink != 0 {
			continue
		}
		out := filepath.Join(target, name)
		if !contained(target, out) {
			return "", fmt.Errorf("unsafe archive path %q", file.Name)
		}
		if file.FileInfo().IsDir() {
			if top == "" {
				top = parts[0]
			}
			if err := os.MkdirAll(out, 0o755); err != nil {
				return "", err
			}
			continue
		}
		if top == "" {
			top = parts[0]
		}
		if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
			return "", err
		}
		rc, err := file.Open()
		if err != nil {
			return "", err
		}
		f, err := os.OpenFile(out, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode&0o644)
		if err != nil {
			_ = rc.Close()
			return "", err
		}
		if _, err := io.Copy(f, rc); err != nil {
			_ = rc.Close()
			_ = f.Close()
			return "", err
		}
		if err := rc.Close(); err != nil {
			_ = f.Close()
			return "", err
		}
		if err := f.Close(); err != nil {
			return "", err
		}
	}
	if top == "" {
		return "", fmt.Errorf("archive was empty")
	}
	return filepath.Join(target, top), nil
}

func copyDir(src, dst string, skipped map[string]bool) error {
	return filepath.WalkDir(src, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		if entry.IsDir() && shouldSkipDir(entry.Name()) {
			skipped[entry.Name()] = true
			return filepath.SkipDir
		}
		out := filepath.Join(dst, rel)
		if !contained(dst, out) {
			return fmt.Errorf("unsafe copy path %q", rel)
		}
		if entry.IsDir() {
			return os.MkdirAll(out, 0o755)
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return nil
		}
		if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
			return err
		}
		in, err := os.Open(path)
		if err != nil {
			return err
		}
		outFile, err := os.OpenFile(out, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode()&0o644)
		if err != nil {
			_ = in.Close()
			return err
		}
		if _, err := io.Copy(outFile, in); err != nil {
			_ = in.Close()
			_ = outFile.Close()
			return err
		}
		if err := in.Close(); err != nil {
			_ = outFile.Close()
			return err
		}
		return outFile.Close()
	})
}

func discoverFiles(root string, full bool) ([]scanFile, map[string]bool, error) {
	var files []scanFile
	skipped := map[string]bool{}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if !full && shouldSkipDir(entry.Name()) && path != root {
				skipped[entry.Name()] = true
				return filepath.SkipDir
			}
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		files = append(files, scanFile{Abs: path, Rel: filepath.ToSlash(rel), Size: info.Size()})
		return nil
	})
	sort.Slice(files, func(i, j int) bool { return files[i].Rel < files[j].Rel })
	return files, skipped, err
}

func (inv *Inventory) inspectFile(file scanFile) {
	lower := strings.ToLower(file.Rel)
	base := filepath.Base(lower)
	add := func(dst *[]Finding, kind, rationale string) {
		*dst = append(*dst, Finding{Kind: kind, Path: file.Rel, Confidence: "path", Rationale: rationale})
	}
	switch {
	case strings.Contains(lower, ".github/workflows/"):
		add(&inv.CI, "github-actions", "GitHub Actions workflow")
	case base == "circleci.yml" || strings.Contains(lower, ".circleci/"):
		add(&inv.CI, "circleci", "CircleCI config")
	case base == ".gitlab-ci.yml":
		add(&inv.CI, "gitlab-ci", "GitLab CI config")
	}
	switch {
	case base == "dockerfile" || strings.HasSuffix(lower, "/dockerfile"):
		add(&inv.DeployConfig, "docker", "Docker build/deploy config")
	case base == "docker-compose.yml" || base == "docker-compose.yaml":
		add(&inv.DeployConfig, "docker-compose", "Docker Compose deploy config")
	case strings.Contains(lower, "k8s/") || strings.Contains(lower, "kubernetes/") || strings.Contains(lower, "helm/"):
		add(&inv.DeployConfig, "kubernetes", "Kubernetes or Helm deployment config")
	}
	switch {
	case base == "gemfile":
		add(&inv.Frameworks, "ruby", "Ruby dependency file")
		inv.TestCommands = append(inv.TestCommands, Command{Command: "bundle exec rake test", Reason: "Gemfile suggests Ruby tests may be available"})
	case base == "manage.py":
		add(&inv.Frameworks, "django", "Django manage.py entrypoint")
		inv.TestCommands = append(inv.TestCommands, Command{Command: "python manage.py test", Reason: "Django project test command"})
	case base == "package.json":
		add(&inv.Frameworks, "node", "Node package manifest")
		inv.TestCommands = append(inv.TestCommands, Command{Command: "npm test", Reason: "package.json suggests npm tests may be available"})
	case base == "go.mod":
		add(&inv.Frameworks, "go", "Go module")
		inv.TestCommands = append(inv.TestCommands, Command{Command: "go test ./...", Reason: "Go module test command"})
	case base == "pyproject.toml":
		add(&inv.Frameworks, "python", "Python project metadata")
		inv.TestCommands = append(inv.TestCommands, Command{Command: "pytest", Reason: "Python project metadata suggests pytest may be available"})
	}
	switch {
	case strings.Contains(lower, "db/migrate/"):
		add(&inv.MigrationRoots, "rails-or-generic-db-migrate", "db/migrate migration path")
	case strings.Contains(lower, "migrations/"):
		add(&inv.MigrationRoots, "migrations", "migrations path")
	case strings.Contains(lower, "prisma/migrations/"):
		add(&inv.MigrationSystems, "prisma", "Prisma migrations path")
	case base == "alembic.ini":
		add(&inv.MigrationSystems, "alembic", "Alembic config")
	case base == "flyway.conf":
		add(&inv.MigrationSystems, "flyway", "Flyway config")
	case base == "liquibase.properties":
		add(&inv.MigrationSystems, "liquibase", "Liquibase config")
	}
	if isSourceSQLCandidate(lower) {
		add(&inv.SourceSQLHints, "source-sql-candidate", "source file type commonly embeds SQL or ORM persistence calls")
	}
	if isOperationalDoc(lower) {
		add(&inv.OperationalDocs, "operational-doc", "incident, rollback, runbook, or release-note path")
	}
	if isEvidenceExport(lower) {
		add(&inv.EvidenceExports, "evidence-export", "log, trace, JSON/JSONL, CSV, or SARIF-like export")
	}
}

func (inv *Inventory) finalize() {
	inv.Frameworks = uniqueFindings(inv.Frameworks)
	inv.MigrationSystems = uniqueFindings(inv.MigrationSystems)
	inv.MigrationRoots = uniqueFindings(inv.MigrationRoots)
	inv.CI = uniqueFindings(inv.CI)
	inv.DeployConfig = uniqueFindings(inv.DeployConfig)
	inv.SourceSQLHints = capFindings(uniqueFindings(inv.SourceSQLHints), 50)
	inv.OperationalDocs = capFindings(uniqueFindings(inv.OperationalDocs), 50)
	inv.EvidenceExports = capFindings(uniqueFindings(inv.EvidenceExports), 50)
	inv.TestCommands = uniqueCommands(inv.TestCommands)
	inv.NextCommands = append(inv.NextCommands, Command{Command: fmt.Sprintf("patchline intake %s --out results/generated/intake", shellPath(inv.Root)), Reason: "run deterministic data/code repair intake on this project"})
	if len(inv.TestCommands) > 0 {
		inv.NextCommands = append(inv.NextCommands, inv.TestCommands...)
	}
	inv.SummaryByCategory = map[string]int{
		"ci":                len(inv.CI),
		"deploy_config":     len(inv.DeployConfig),
		"evidence_exports":  len(inv.EvidenceExports),
		"frameworks":        len(inv.Frameworks),
		"migration_roots":   len(inv.MigrationRoots),
		"migration_systems": len(inv.MigrationSystems),
		"operational_docs":  len(inv.OperationalDocs),
		"source_sql_hints":  len(inv.SourceSQLHints),
	}
	inv.Markdown = renderMarkdown(*inv)
}

func renderMarkdown(inv Inventory) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Patchline repo inventory\n\n")
	fmt.Fprintf(&b, "- root: `%s`\n", inv.Root)
	fmt.Fprintf(&b, "- files scanned: `%d`\n", inv.FilesScanned)
	fmt.Fprintf(&b, "- bytes scanned: `%d`\n\n", inv.BytesScanned)
	if len(inv.Languages) > 0 {
		fmt.Fprintf(&b, "## Languages\n\n| language | files |\n| --- | ---: |\n")
		for _, c := range inv.Languages {
			fmt.Fprintf(&b, "| %s | %d |\n", c.Name, c.Count)
		}
		fmt.Fprintf(&b, "\n")
	}
	writeFindings := func(title string, findings []Finding) {
		if len(findings) == 0 {
			return
		}
		fmt.Fprintf(&b, "## %s\n\n| kind | path | rationale |\n| --- | --- | --- |\n", title)
		for _, f := range findings {
			fmt.Fprintf(&b, "| %s | %s | %s |\n", f.Kind, f.Path, f.Rationale)
		}
		fmt.Fprintf(&b, "\n")
	}
	writeFindings("Frameworks", inv.Frameworks)
	writeFindings("Migration systems", inv.MigrationSystems)
	writeFindings("Migration roots", inv.MigrationRoots)
	writeFindings("CI", inv.CI)
	writeFindings("Deploy config", inv.DeployConfig)
	writeFindings("Operational docs", inv.OperationalDocs)
	writeFindings("Evidence exports", inv.EvidenceExports)
	if len(inv.NextCommands) > 0 {
		fmt.Fprintf(&b, "## Next commands\n\n")
		for _, c := range inv.NextCommands {
			fmt.Fprintf(&b, "- `%s` — %s\n", c.Command, c.Reason)
		}
	}
	return b.String()
}

func languageFor(path string) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".go":
		return "Go"
	case ".py":
		return "Python"
	case ".rb":
		return "Ruby"
	case ".js", ".jsx", ".mjs", ".cjs":
		return "JavaScript"
	case ".ts", ".tsx":
		return "TypeScript"
	case ".java":
		return "Java"
	case ".cs":
		return "C#"
	case ".sql", ".psql", ".ddl":
		return "SQL"
	case ".yaml", ".yml":
		return "YAML"
	case ".json", ".jsonl":
		return "JSON"
	case ".md", ".markdown":
		return "Markdown"
	default:
		return ""
	}
}

func isSourceSQLCandidate(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".go", ".py", ".rb", ".js", ".ts", ".java", ".cs", ".sh", ".sql", ".psql", ".ddl":
		return true
	default:
		return false
	}
}

func isOperationalDoc(path string) bool {
	return strings.HasSuffix(path, ".md") && containsAny(path, "incident", "postmortem", "runbook", "rollback", "repair", "release", "changelog")
}

func isEvidenceExport(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	if ext == ".json" || ext == ".jsonl" || ext == ".csv" || ext == ".log" || ext == ".sarif" {
		return containsAny(path, "trace", "log", "event", "deploy", "incident", "evidence", "export", "datadog", "otlp", "postgres")
	}
	return false
}

func isArchivePath(path string) bool {
	lower := strings.ToLower(path)
	return strings.HasSuffix(lower, ".zip") || strings.HasSuffix(lower, ".tar.gz") || strings.HasSuffix(lower, ".tgz")
}

func isHTTPArchive(path string) bool {
	return (strings.HasPrefix(path, "https://") || strings.HasPrefix(path, "http://")) && isArchivePath(path)
}

func shouldSkipDir(name string) bool {
	switch name {
	case ".git", "node_modules", "vendor", "target", "dist", "build", ".next", ".venv", "__pycache__", ".terraform":
		return true
	default:
		return false
	}
}

func contained(root, path string) bool {
	cleanRoot := filepath.Clean(root)
	cleanPath := filepath.Clean(path)
	return cleanPath == cleanRoot || strings.HasPrefix(cleanPath, cleanRoot+string(filepath.Separator))
}

func containedSubpath(root, subpath string) (string, error) {
	if strings.TrimSpace(subpath) == "" {
		return root, nil
	}
	candidate := filepath.Join(root, filepath.FromSlash(subpath))
	if !contained(root, candidate) {
		return "", fmt.Errorf("subpath %q escapes repository root", subpath)
	}
	if _, err := os.Stat(candidate); err != nil {
		return "", err
	}
	return candidate, nil
}

func hashFile(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return "", err
	}
	return "sha256:" + hex.EncodeToString(hasher.Sum(nil)), nil
}

func sortedKeys(in map[string]bool) []string {
	out := make([]string, 0, len(in))
	for key := range in {
		out = append(out, key)
	}
	sort.Strings(out)
	return out
}

func countsFromMap(in map[string]int) []Count {
	out := make([]Count, 0, len(in))
	for key, value := range in {
		out = append(out, Count{Name: key, Count: value})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].Name < out[j].Name
	})
	return out
}

func uniqueFindings(in []Finding) []Finding {
	sort.Slice(in, func(i, j int) bool {
		if in[i].Kind != in[j].Kind {
			return in[i].Kind < in[j].Kind
		}
		return in[i].Path < in[j].Path
	})
	seen := map[string]bool{}
	var out []Finding
	for _, item := range in {
		key := item.Kind + "\x00" + item.Path
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, item)
	}
	return out
}

func capFindings(in []Finding, n int) []Finding {
	if len(in) <= n {
		return in
	}
	return in[:n]
}

func uniqueCommands(in []Command) []Command {
	seen := map[string]bool{}
	var out []Command
	for _, item := range in {
		if item.Command == "" || seen[item.Command] {
			continue
		}
		seen[item.Command] = true
		out = append(out, item)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Command < out[j].Command })
	return out
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func safeSlug(value string) string {
	value = strings.TrimSpace(value)
	value = strings.TrimPrefix(value, "https://")
	value = strings.TrimPrefix(value, "http://")
	value = strings.NewReplacer("/", "-", "\\", "-", ":", "-", "?", "-", "&", "-", "=", "-").Replace(value)
	value = strings.Trim(value, "-.")
	if value == "" {
		return "repo"
	}
	return value
}

func commitHintFromTop(top string) string {
	parts := strings.Split(top, "-")
	if len(parts) == 0 {
		return ""
	}
	return parts[len(parts)-1]
}

func containsAny(value string, needles ...string) bool {
	for _, needle := range needles {
		if strings.Contains(value, needle) {
			return true
		}
	}
	return false
}

func shellPath(path string) string {
	if path == "" {
		return "''"
	}
	if strings.ContainsAny(path, " \t\n'\"$`\\") {
		return "'" + strings.ReplaceAll(path, "'", "'\\''") + "'"
	}
	return path
}
