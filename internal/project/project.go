package project

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime/debug"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/thehalleyyoung/patchline/internal/canonical"
	"github.com/thehalleyyoung/patchline/internal/migration"
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
	Version        string   `json:"version"`
	ToolVersion    string   `json:"tool_version"`
	Mode           string   `json:"mode"`
	VCS            string   `json:"vcs,omitempty"`
	Input          string   `json:"input"`
	Owner          string   `json:"owner,omitempty"`
	Repo           string   `json:"repo,omitempty"`
	Ref            string   `json:"ref,omitempty"`
	ResolvedCommit string   `json:"resolved_commit,omitempty"`
	CommitHint     string   `json:"commit_hint,omitempty"`
	Subpath        string   `json:"subpath,omitempty"`
	ArchiveHash    string   `json:"archive_hash,omitempty"`
	FetchedAt      string   `json:"fetched_at"`
	CacheKey       string   `json:"cache_key,omitempty"`
	CachePath      string   `json:"cache_path,omitempty"`
	CacheHit       bool     `json:"cache_hit,omitempty"`
	OutDir         string   `json:"out_dir"`
	ScannedRoot    string   `json:"scanned_root"`
	SkippedDirs    []string `json:"skipped_dirs,omitempty"`
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
	NativeCommands    []Command      `json:"native_commands,omitempty"`
	SourceSQLHints    []Finding      `json:"source_sql_hints,omitempty"`
	SchemaEvolution   []Finding      `json:"schema_evolution,omitempty"`
	FieldEvidence     []Finding      `json:"field_evidence,omitempty"`
	OperationalDocs   []Finding      `json:"operational_docs,omitempty"`
	EvidenceExports   []Finding      `json:"evidence_exports,omitempty"`
	Infrastructure    []Finding      `json:"infrastructure_scans,omitempty"`
	PackageBoundaries []PackageBoundary `json:"package_boundaries,omitempty"`
	NextCommands      []Command      `json:"next_commands,omitempty"`
	SkippedDirs       []string       `json:"skipped_dirs,omitempty"`
	SummaryByCategory map[string]int `json:"summary_by_category,omitempty"`
	Markdown          string         `json:"markdown,omitempty"`
	Facts             []Fact         `json:"-"`
	ProjectMap        string         `json:"-"`
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

// PackageBoundary marks a package root inside a (possibly mono-) repository, identified by the
// build system whose manifest declares it. Boundaries let Patchline attribute data-change risks to
// the owning package rather than to an undifferentiated repository root.
type PackageBoundary struct {
	System   string `json:"system"`
	Path     string `json:"path"`
	Manifest string `json:"manifest"`
}

type Fact struct {
	Version     string            `json:"version"`
	ID          string            `json:"id"`
	Kind        string            `json:"kind"`
	Path        string            `json:"path,omitempty"`
	Confidence  string            `json:"confidence"`
	Rationale   string            `json:"rationale,omitempty"`
	Identifiers []Identifier      `json:"identifiers,omitempty"`
	Properties  map[string]string `json:"properties,omitempty"`
}

type Identifier struct {
	Kind  string `json:"kind"`
	Value string `json:"value"`
}

type scanFile struct {
	Abs  string
	Rel  string
	Size int64
}

type downloadCacheEntry struct {
	Version        string `json:"version"`
	Key            string `json:"key"`
	URL            string `json:"url"`
	ArchiveHash    string `json:"archive_hash"`
	ArchivePath    string `json:"archive_path"`
	Kind           string `json:"kind"`
	Top            string `json:"top,omitempty"`
	FetchedAt      string `json:"fetched_at"`
	ResolvedCommit string `json:"resolved_commit,omitempty"`
}

type downloadCacheResult struct {
	Key         string
	Path        string
	ArchiveHash string
	FetchedAt   string
	Hit         bool
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
		Version:     Version,
		ToolVersion: toolVersion(),
		Input:       opts.Input,
		Ref:         firstNonEmpty(strings.TrimSpace(opts.Ref), "HEAD"),
		Subpath:     filepath.ToSlash(strings.TrimSpace(opts.Subpath)),
		FetchedAt:   time.Now().UTC().Format(time.RFC3339),
		OutDir:      filepath.ToSlash(outDir),
	}
	downloadDir := opts.DownloadDir
	if downloadDir == "" {
		downloadDir = filepath.Join("results", "generated", "repo-downloads")
	}
	skipped := map[string]bool{}
	input := strings.TrimSpace(opts.Input)
	switch {
	case isHostedArchiveInput(input, "gitlab"):
		namespace, repo, err := parsePrefixedRepo(input, "gitlab", true)
		if err != nil {
			return FetchResult{}, err
		}
		source.Mode, source.Owner, source.Repo = "gitlab", namespace, repo
		root, archiveHash, commitHint, cache, err := fetchHostedArchive(ctx, "gitlab", namespace, repo, source.Ref, outDir, downloadDir)
		if err != nil {
			return FetchResult{}, err
		}
		applyCacheResult(&source, cache)
		source.ResolvedCommit = source.Ref
		source.ArchiveHash = archiveHash
		source.CommitHint = commitHint
		source.ScannedRoot = filepath.ToSlash(root)
	case isHostedArchiveInput(input, "bitbucket"):
		namespace, repo, err := parsePrefixedRepo(input, "bitbucket", false)
		if err != nil {
			return FetchResult{}, err
		}
		source.Mode, source.Owner, source.Repo = "bitbucket", namespace, repo
		root, archiveHash, commitHint, cache, err := fetchHostedArchive(ctx, "bitbucket", namespace, repo, source.Ref, outDir, downloadDir)
		if err != nil {
			return FetchResult{}, err
		}
		applyCacheResult(&source, cache)
		source.ResolvedCommit = source.Ref
		source.ArchiveHash = archiveHash
		source.CommitHint = commitHint
		source.ScannedRoot = filepath.ToSlash(root)
	case isHostedArchiveInput(input, "sourcehut"):
		namespace, repo, err := parsePrefixedRepo(input, "sourcehut", false)
		if err != nil {
			return FetchResult{}, err
		}
		source.Mode, source.Owner, source.Repo = "sourcehut", namespace, repo
		root, archiveHash, commitHint, cache, err := fetchHostedArchive(ctx, "sourcehut", namespace, repo, source.Ref, outDir, downloadDir)
		if err != nil {
			return FetchResult{}, err
		}
		applyCacheResult(&source, cache)
		source.ResolvedCommit = source.Ref
		source.ArchiveHash = archiveHash
		source.CommitHint = commitHint
		source.ScannedRoot = filepath.ToSlash(root)
	case isHTTPArchive(input):
		source.Mode = "archive-url"
		root, archiveHash, cache, err := fetchArchiveURL(ctx, input, outDir, downloadDir)
		if err != nil {
			return FetchResult{}, err
		}
		applyCacheResult(&source, cache)
		source.ArchiveHash = archiveHash
		source.ScannedRoot = filepath.ToSlash(root)
	case isGitHubInput(input):
		owner, repo, err := ParseGitHubRepo(input)
		if err != nil {
			return FetchResult{}, err
		}
		resolvedCommit, err := resolveGitHubCommit(ctx, owner, repo, source.Ref)
		if err != nil {
			return FetchResult{}, err
		}
		source.Mode, source.Owner, source.Repo = "github", owner, repo
		source.ResolvedCommit = resolvedCommit
		root, archiveHash, commitHint, cache, err := fetchGitHubArchive(ctx, owner, repo, source.Ref, resolvedCommit, outDir, downloadDir)
		if err != nil {
			return FetchResult{}, err
		}
		applyCacheResult(&source, cache)
		source.ArchiveHash = archiveHash
		source.CommitHint = commitHint
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
		vcs, revision := detectLocalVCS(root)
		source.VCS = vcs
		if revision != "" {
			source.ResolvedCommit = revision
		} else {
			source.ResolvedCommit = localGitCommit(root)
		}
		if treeHash, err := hashTree(outDir); err == nil {
			source.ArchiveHash = treeHash
			source.CacheKey = "tree:" + strings.TrimPrefix(treeHash, "sha256:")
			cachePath := filepath.Join(downloadDir, "trees", "sha256-"+strings.TrimPrefix(treeHash, "sha256:"))
			if _, statErr := os.Stat(cachePath); statErr == nil {
				source.CacheHit = true
			} else if mkErr := os.MkdirAll(filepath.Dir(cachePath), 0o755); mkErr == nil {
				_ = os.WriteFile(cachePath, []byte(treeHash+"\n"), 0o644)
			}
			source.CachePath = filepath.ToSlash(cachePath)
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
		inv.addFileFact(file, lang)
		inv.inspectFile(file)
	}
	inv.Languages = countsFromMap(languages)
	inv.detectPackageBoundaries(files)
	inv.finalize()
	return inv, nil
}

// detectPackageBoundaries scans the discovered files for monorepo build-system manifests and
// records one boundary per package root. It recognizes Bazel, Pants, Nx, Turborepo, Maven,
// Gradle, and Go workspaces. The presence of a workspace-level manifest (WORKSPACE, MODULE.bazel,
// pants.toml, nx.json, turbo.json, go.work) raises confidence that per-directory manifests are
// genuine package boundaries rather than incidental build files.
func (inv *Inventory) detectPackageBoundaries(files []scanFile) {
	type sysInfo struct {
		system     string
		isWorkspce bool
	}
	classify := func(base string) (sysInfo, bool) {
		switch base {
		case "workspace", "workspace.bazel", "workspace.bzlmod", "module.bazel":
			return sysInfo{"bazel", true}, true
		case "build.bazel", "build.bzl":
			return sysInfo{"bazel", false}, true
		case "pants.toml":
			return sysInfo{"pants", true}, true
		case "nx.json":
			return sysInfo{"nx", true}, true
		case "turbo.json":
			return sysInfo{"turborepo", true}, true
		case "go.work":
			return sysInfo{"go-workspace", true}, true
		case "pom.xml":
			return sysInfo{"maven", false}, true
		case "build.gradle", "build.gradle.kts":
			return sysInfo{"gradle", false}, true
		case "settings.gradle", "settings.gradle.kts":
			return sysInfo{"gradle", true}, true
		case "go.mod":
			return sysInfo{"go-workspace", false}, true
		case "project.json", "package.json":
			// Only meaningful as a package boundary when an Nx/Turborepo workspace exists.
			return sysInfo{"", false}, false
		}
		return sysInfo{}, false
	}

	workspaceSystems := map[string]bool{}
	type cand struct {
		system   string
		path     string
		manifest string
	}
	var bazelPlain, otherManifests []cand
	var jsPackages []cand
	for _, f := range files {
		base := strings.ToLower(filepath.Base(f.Rel))
		dir := filepath.ToSlash(filepath.Dir(f.Rel))
		if dir == "." {
			dir = "."
		}
		if base == "project.json" || base == "package.json" {
			jsPackages = append(jsPackages, cand{system: "", path: dir, manifest: filepath.ToSlash(f.Rel)})
			continue
		}
		info, ok := classify(base)
		if !ok {
			continue
		}
		if info.isWorkspce {
			workspaceSystems[info.system] = true
		}
		c := cand{system: info.system, path: dir, manifest: filepath.ToSlash(f.Rel)}
		if info.system == "bazel" && !info.isWorkspce {
			bazelPlain = append(bazelPlain, c)
		} else if !info.isWorkspce {
			otherManifests = append(otherManifests, c)
		}
	}

	seen := map[string]bool{}
	add := func(system, p, manifest string) {
		key := system + "\x00" + p
		if seen[key] {
			return
		}
		seen[key] = true
		boundary := PackageBoundary{System: system, Path: p, Manifest: manifest}
		inv.PackageBoundaries = append(inv.PackageBoundaries, boundary)
		inv.addFindingFact("package_boundary", Finding{Kind: system, Path: p, Confidence: "manifest", Rationale: system + " package boundary declared by " + manifest})
	}

	// Per-directory manifests (Maven modules, Gradle subprojects, Go modules) are always package
	// boundaries.
	for _, c := range otherManifests {
		add(c.system, c.path, c.manifest)
	}
	// Bazel BUILD files are boundaries only when a Bazel workspace marker exists, to avoid treating
	// unrelated files named BUILD as packages.
	if workspaceSystems["bazel"] {
		for _, c := range bazelPlain {
			add("bazel", c.path, c.manifest)
		}
	}
	// JS packages count when an Nx or Turborepo workspace exists.
	if workspaceSystems["nx"] || workspaceSystems["turborepo"] {
		system := "turborepo"
		if workspaceSystems["nx"] {
			system = "nx"
		}
		for _, c := range jsPackages {
			if c.path == "." {
				continue
			}
			add(system, c.path, c.manifest)
		}
	}
	// Record workspace roots themselves so a single-module workspace is still represented.
	for system := range workspaceSystems {
		add(system, ".", system+" workspace root")
	}
	sort.Slice(inv.PackageBoundaries, func(i, j int) bool {
		if inv.PackageBoundaries[i].System != inv.PackageBoundaries[j].System {
			return inv.PackageBoundaries[i].System < inv.PackageBoundaries[j].System
		}
		return inv.PackageBoundaries[i].Path < inv.PackageBoundaries[j].Path
	})
}

func (inv *Inventory) inspectRoot(root string) {
	lower := strings.ToLower(filepath.ToSlash(root))
	switch {
	case strings.HasSuffix(lower, "/db/migrate") || strings.Contains(lower, "/db/migrate/"):
		finding := Finding{Kind: "rails-or-generic-db-migrate", Path: ".", Confidence: "path", Rationale: "inventory root is a db/migrate migration path"}
		inv.MigrationRoots = append(inv.MigrationRoots, finding)
		inv.addFindingFact("migration_root", finding)
		inv.NativeCommands = append(inv.NativeCommands, Command{Command: "bundle exec rails db:migrate", Reason: "db/migrate inventory root suggests Rails-style migration execution from the project root"})
	case strings.HasSuffix(lower, "/migrations") || strings.Contains(lower, "/migrations/"):
		finding := Finding{Kind: "migrations", Path: ".", Confidence: "path", Rationale: "inventory root is a migrations path"}
		inv.MigrationRoots = append(inv.MigrationRoots, finding)
		inv.addFindingFact("migration_root", finding)
		inv.NativeCommands = append(inv.NativeCommands, Command{Command: "python manage.py migrate", Reason: "migrations inventory root may be runnable through Django from the project root"})
	case strings.HasSuffix(lower, "/migration") || strings.Contains(lower, "/migration/"):
		finding := Finding{Kind: "migration", Path: ".", Confidence: "path", Rationale: "inventory root is a migration path"}
		inv.MigrationRoots = append(inv.MigrationRoots, finding)
		inv.addFindingFact("migration_root", finding)
		inv.NativeCommands = append(inv.NativeCommands, Command{Command: "go test ./...", Reason: "singular migration root commonly appears in Go services; run from the project root when available"})
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
	if err := os.WriteFile(filepath.Join(outDir, "inventory.md"), []byte(inv.Markdown), 0o644); err != nil {
		return err
	}
	if err := writeFacts(filepath.Join(outDir, "facts.jsonl"), inv.Facts); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(outDir, "project-map.md"), []byte(inv.ProjectMap), 0o644)
}

func writeFacts(path string, facts []Fact) error {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	defer file.Close()
	encoder := json.NewEncoder(file)
	for _, fact := range facts {
		if err := encoder.Encode(fact); err != nil {
			return err
		}
	}
	return nil
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

func isHostedArchiveInput(input, host string) bool {
	return strings.HasPrefix(strings.TrimSpace(input), host+":")
}

func parsePrefixedRepo(input, host string, allowNestedNamespace bool) (string, string, error) {
	value := strings.TrimSpace(strings.TrimPrefix(input, host+":"))
	value = strings.TrimSuffix(value, ".git")
	value = strings.Trim(value, "/")
	value = strings.TrimPrefix(value, "https://")
	value = strings.TrimPrefix(value, "http://")
	for _, prefix := range []string{"gitlab.com/", "bitbucket.org/", "git.sr.ht/"} {
		value = strings.TrimPrefix(value, prefix)
	}
	parts := strings.Split(value, "/")
	if len(parts) < 2 || strings.Contains(value, "..") {
		return "", "", fmt.Errorf("%s repo must be namespace/repo, got %q", host, input)
	}
	if !allowNestedNamespace && len(parts) != 2 {
		return "", "", fmt.Errorf("%s repo must be owner/repo, got %q", host, input)
	}
	repo := parts[len(parts)-1]
	namespace := strings.Join(parts[:len(parts)-1], "/")
	if namespace == "" || repo == "" {
		return "", "", fmt.Errorf("%s repo must be namespace/repo, got %q", host, input)
	}
	if host == "sourcehut" && !strings.HasPrefix(namespace, "~") {
		namespace = "~" + namespace
	}
	return namespace, repo, nil
}

func resolveGitHubCommit(ctx context.Context, owner, repo, ref string) (string, error) {
	if isFullSHA(ref) {
		return strings.ToLower(ref), nil
	}
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/commits/%s", owner, repo, ref)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "patchline-repo")
	req.Header.Set("Accept", "application/vnd.github+json")
	if token := os.Getenv("GITHUB_TOKEN"); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	client := &http.Client{Timeout: 2 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("resolve github commit %s/%s@%s: %s", owner, repo, ref, resp.Status)
	}
	var payload struct {
		SHA string `json:"sha"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&payload); err != nil {
		return "", err
	}
	if !isFullSHA(payload.SHA) {
		return "", fmt.Errorf("github commit %s/%s@%s resolved to invalid sha %q", owner, repo, ref, payload.SHA)
	}
	return strings.ToLower(payload.SHA), nil
}

func fetchHostedArchive(ctx context.Context, host, namespace, repo, ref, outDir, downloadDir string) (string, string, string, downloadCacheResult, error) {
	if ref == "" {
		ref = "HEAD"
	}
	archiveURL, kind, err := hostedArchiveURL(host, namespace, repo, ref)
	if err != nil {
		return "", "", "", downloadCacheResult{}, err
	}
	key := host + ":" + namespace + "/" + repo + "@" + ref
	root, top, cache, err := fetchCachedArchive(ctx, archiveURL, outDir, downloadDir, key, kind, ref)
	if err != nil {
		return "", "", "", downloadCacheResult{}, err
	}
	commitHint := commitHintFromTop(top)
	if commitHint == "" {
		commitHint = ref
	}
	return root, cache.ArchiveHash, commitHint, cache, nil
}

func hostedArchiveURL(host, namespace, repo, ref string) (string, string, error) {
	switch host {
	case "gitlab":
		repoPath := namespace + "/" + repo
		filename := repo + "-" + ref + ".tar.gz"
		return "https://gitlab.com/" + pathEscapeSegments(repoPath) + "/-/archive/" + url.PathEscape(ref) + "/" + url.PathEscape(filename), "tar.gz", nil
	case "bitbucket":
		return "https://bitbucket.org/" + pathEscapeSegments(namespace+"/"+repo) + "/get/" + url.PathEscape(ref) + ".tar.gz", "tar.gz", nil
	case "sourcehut":
		return "https://git.sr.ht/" + pathEscapeSegments(namespace+"/"+repo) + "/archive/" + url.PathEscape(ref) + ".tar.gz", "tar.gz", nil
	default:
		return "", "", fmt.Errorf("unsupported source host %q", host)
	}
}

func pathEscapeSegments(path string) string {
	parts := strings.Split(path, "/")
	for i, part := range parts {
		parts[i] = url.PathEscape(part)
	}
	return strings.Join(parts, "/")
}

func fetchGitHubArchive(ctx context.Context, owner, repo, ref, resolvedCommit, outDir, downloadDir string) (string, string, string, downloadCacheResult, error) {
	url := fmt.Sprintf("https://codeload.github.com/%s/%s/tar.gz/%s", owner, repo, resolvedCommit)
	key := "github:" + owner + "/" + repo + "@" + resolvedCommit
	root, top, cache, err := fetchCachedArchive(ctx, url, outDir, downloadDir, key, "tar.gz", resolvedCommit)
	if err != nil {
		return "", "", "", downloadCacheResult{}, err
	}
	commitHint := commitHintFromTop(top)
	if commitHint == "" {
		commitHint = ref
	}
	return root, cache.ArchiveHash, commitHint, cache, nil
}

func fetchArchiveURL(ctx context.Context, url, outDir, downloadDir string) (string, string, downloadCacheResult, error) {
	kind := "tar.gz"
	if strings.HasSuffix(strings.ToLower(url), ".zip") {
		kind = "zip"
	}
	key := "archive-url:" + url
	root, _, cache, err := fetchCachedArchive(ctx, url, outDir, downloadDir, key, kind, "")
	if err != nil {
		return "", "", downloadCacheResult{}, err
	}
	return root, cache.ArchiveHash, cache, nil
}

func fetchCachedArchive(ctx context.Context, url, outDir, downloadDir, key, kind, resolvedCommit string) (string, string, downloadCacheResult, error) {
	if downloadDir == "" {
		return fetchUncachedArchive(ctx, url, outDir, kind)
	}
	if entry, ok := readDownloadCache(downloadDir, key); ok {
		if _, err := os.Stat(entry.ArchivePath); err == nil {
			root, top, err := extractCachedArchive(entry.ArchivePath, entry.Kind, outDir)
			if err != nil {
				return "", "", downloadCacheResult{}, err
			}
			return root, firstNonEmpty(entry.Top, top), downloadCacheResult{Key: key, Path: filepath.ToSlash(entry.ArchivePath), ArchiveHash: entry.ArchiveHash, FetchedAt: entry.FetchedAt, Hit: true}, nil
		}
	}
	if err := os.MkdirAll(downloadDir, 0o755); err != nil {
		return "", "", downloadCacheResult{}, err
	}
	tmp, err := os.CreateTemp(downloadDir, "patchline-download-*")
	if err != nil {
		return "", "", downloadCacheResult{}, err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		_ = tmp.Close()
		return "", "", downloadCacheResult{}, err
	}
	req.Header.Set("User-Agent", "patchline-repo")
	if token := os.Getenv("GITHUB_TOKEN"); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	client := &http.Client{Timeout: 2 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		_ = tmp.Close()
		return "", "", downloadCacheResult{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		_ = tmp.Close()
		return "", "", downloadCacheResult{}, fmt.Errorf("download archive %s: %s", url, resp.Status)
	}
	hasher := sha256.New()
	if _, err := io.Copy(tmp, io.TeeReader(resp.Body, hasher)); err != nil {
		_ = tmp.Close()
		return "", "", downloadCacheResult{}, err
	}
	if err := tmp.Close(); err != nil {
		return "", "", downloadCacheResult{}, err
	}
	archiveHash := "sha256:" + hex.EncodeToString(hasher.Sum(nil))
	archivePath := cacheArchivePath(downloadDir, archiveHash, kind)
	if err := os.MkdirAll(filepath.Dir(archivePath), 0o755); err != nil {
		return "", "", downloadCacheResult{}, err
	}
	if _, err := os.Stat(archivePath); os.IsNotExist(err) {
		if err := os.Rename(tmpPath, archivePath); err != nil {
			return "", "", downloadCacheResult{}, err
		}
	}
	root, top, err := extractCachedArchive(archivePath, kind, outDir)
	if err != nil {
		return "", "", downloadCacheResult{}, err
	}
	fetchedAt := time.Now().UTC().Format(time.RFC3339)
	entry := downloadCacheEntry{
		Version:        Version,
		Key:            key,
		URL:            url,
		ArchiveHash:    archiveHash,
		ArchivePath:    filepath.ToSlash(archivePath),
		Kind:           kind,
		Top:            top,
		FetchedAt:      fetchedAt,
		ResolvedCommit: resolvedCommit,
	}
	if err := writeDownloadCache(downloadDir, entry); err != nil {
		return "", "", downloadCacheResult{}, err
	}
	return root, top, downloadCacheResult{Key: key, Path: filepath.ToSlash(archivePath), ArchiveHash: archiveHash, FetchedAt: fetchedAt}, nil
}

func fetchUncachedArchive(ctx context.Context, url, outDir, kind string) (string, string, downloadCacheResult, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", "", downloadCacheResult{}, err
	}
	req.Header.Set("User-Agent", "patchline-repo")
	client := &http.Client{Timeout: 2 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return "", "", downloadCacheResult{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", "", downloadCacheResult{}, fmt.Errorf("download archive %s: %s", url, resp.Status)
	}
	hasher := sha256.New()
	if kind == "zip" {
		tmp, err := os.CreateTemp("", "patchline-archive-*.zip")
		if err != nil {
			return "", "", downloadCacheResult{}, err
		}
		tmpPath := tmp.Name()
		defer os.Remove(tmpPath)
		if _, err := io.Copy(tmp, io.TeeReader(resp.Body, hasher)); err != nil {
			_ = tmp.Close()
			return "", "", downloadCacheResult{}, err
		}
		if err := tmp.Close(); err != nil {
			return "", "", downloadCacheResult{}, err
		}
		root, err := extractZip(tmpPath, outDir)
		archiveHash := "sha256:" + hex.EncodeToString(hasher.Sum(nil))
		return root, "", downloadCacheResult{ArchiveHash: archiveHash, FetchedAt: time.Now().UTC().Format(time.RFC3339)}, err
	}
	root, _, err := extractTarGz(io.TeeReader(resp.Body, hasher), outDir)
	archiveHash := "sha256:" + hex.EncodeToString(hasher.Sum(nil))
	return root, "", downloadCacheResult{ArchiveHash: archiveHash, FetchedAt: time.Now().UTC().Format(time.RFC3339)}, err
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

func extractCachedArchive(path, kind, outDir string) (string, string, error) {
	if kind == "zip" {
		root, err := extractZip(path, outDir)
		return root, "", err
	}
	file, err := os.Open(path)
	if err != nil {
		return "", "", err
	}
	defer file.Close()
	return extractTarGz(file, outDir)
}

type archiveExtractionLimits struct {
	MaxEntries      int
	MaxUncompressed int64
}

var archiveLimits = archiveExtractionLimits{
	MaxEntries:      250000,
	MaxUncompressed: int64(2 << 30),
}

func readDownloadCache(downloadDir, key string) (downloadCacheEntry, bool) {
	path := downloadCacheEntryPath(downloadDir, key)
	data, err := os.ReadFile(path)
	if err != nil {
		return downloadCacheEntry{}, false
	}
	var entry downloadCacheEntry
	if err := json.Unmarshal(data, &entry); err != nil || entry.Key != key || entry.ArchiveHash == "" || entry.ArchivePath == "" {
		return downloadCacheEntry{}, false
	}
	return entry, true
}

func writeDownloadCache(downloadDir string, entry downloadCacheEntry) error {
	path := downloadCacheEntryPath(downloadDir, entry.Key)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(entry, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}

func downloadCacheEntryPath(downloadDir, key string) string {
	sum := sha256.Sum256([]byte(key))
	return filepath.Join(downloadDir, "sources", hex.EncodeToString(sum[:])+".json")
}

func cacheArchivePath(downloadDir, archiveHash, kind string) string {
	name := strings.TrimPrefix(archiveHash, "sha256:")
	name = "sha256-" + name
	switch kind {
	case "zip":
		name += ".zip"
	default:
		name += ".tar.gz"
	}
	return filepath.Join(downloadDir, "archives", name)
}

func applyCacheResult(source *Source, cache downloadCacheResult) {
	source.ArchiveHash = cache.ArchiveHash
	if cache.FetchedAt != "" {
		source.FetchedAt = cache.FetchedAt
	}
	source.CacheKey = cache.Key
	source.CachePath = cache.Path
	source.CacheHit = cache.Hit
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
	var entries int
	var totalBytes int64
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
		entries++
		if entries > archiveLimits.MaxEntries {
			return "", "", fmt.Errorf("archive has too many entries: %d > %d", entries, archiveLimits.MaxEntries)
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
			if header.Size < 0 || totalBytes > archiveLimits.MaxUncompressed-header.Size {
				return "", "", fmt.Errorf("archive uncompressed size exceeds limit %d bytes", archiveLimits.MaxUncompressed)
			}
			totalBytes += header.Size
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
	var totalBytes int64
	for index, file := range zr.File {
		if index+1 > archiveLimits.MaxEntries {
			return "", fmt.Errorf("archive has too many entries: %d > %d", index+1, archiveLimits.MaxEntries)
		}
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
		if !file.FileInfo().IsDir() {
			if file.UncompressedSize64 > uint64(archiveLimits.MaxUncompressed) {
				return "", fmt.Errorf("archive uncompressed size exceeds limit %d bytes", archiveLimits.MaxUncompressed)
			}
			size := int64(file.UncompressedSize64)
			if totalBytes > archiveLimits.MaxUncompressed-size {
				return "", fmt.Errorf("archive uncompressed size exceeds limit %d bytes", archiveLimits.MaxUncompressed)
			}
			totalBytes += size
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
	add := func(category string, dst *[]Finding, kind, rationale string) {
		finding := Finding{Kind: kind, Path: file.Rel, Confidence: "path", Rationale: rationale}
		*dst = append(*dst, finding)
		inv.addFindingFact(category, finding)
	}
	switch {
	case strings.Contains(lower, ".github/workflows/"):
		add("ci", &inv.CI, "github-actions", "GitHub Actions workflow")
	case base == "circleci.yml" || strings.Contains(lower, ".circleci/"):
		add("ci", &inv.CI, "circleci", "CircleCI config")
	case base == ".gitlab-ci.yml":
		add("ci", &inv.CI, "gitlab-ci", "GitLab CI config")
	}
	switch {
	case base == "dockerfile" || strings.HasSuffix(lower, "/dockerfile"):
		add("deploy_config", &inv.DeployConfig, "docker", "Docker build/deploy config")
	case base == "docker-compose.yml" || base == "docker-compose.yaml":
		add("deploy_config", &inv.DeployConfig, "docker-compose", "Docker Compose deploy config")
	case strings.Contains(lower, "k8s/") || strings.Contains(lower, "kubernetes/") || strings.Contains(lower, "helm/"):
		add("deploy_config", &inv.DeployConfig, "kubernetes", "Kubernetes or Helm deployment config")
	}
	switch {
	case base == "gemfile":
		add("framework", &inv.Frameworks, "ruby", "Ruby dependency file")
		inv.TestCommands = append(inv.TestCommands, Command{Command: "bundle exec rake test", Reason: "Gemfile suggests Ruby tests may be available"})
		inv.NativeCommands = append(inv.NativeCommands, Command{Command: "bundle exec rails db:migrate", Reason: "Ruby/Rails dependency file suggests Rails migrations may be runnable from the project root"})
	case base == "manage.py":
		add("framework", &inv.Frameworks, "django", "Django manage.py entrypoint")
		inv.TestCommands = append(inv.TestCommands, Command{Command: "python manage.py test", Reason: "Django project test command"})
		inv.NativeCommands = append(inv.NativeCommands, Command{Command: "python manage.py migrate", Reason: "Django manage.py entrypoint exposes native migration execution"})
	case base == "package.json":
		add("framework", &inv.Frameworks, "node", "Node package manifest")
		inv.TestCommands = append(inv.TestCommands, Command{Command: "npm test", Reason: "package.json suggests npm tests may be available"})
		inv.NativeCommands = append(inv.NativeCommands, Command{Command: "npm test", Reason: "package.json exposes the project-native test command"})
	case base == "go.mod":
		add("framework", &inv.Frameworks, "go", "Go module")
		inv.TestCommands = append(inv.TestCommands, Command{Command: "go test ./...", Reason: "Go module test command"})
	case base == "pyproject.toml":
		add("framework", &inv.Frameworks, "python", "Python project metadata")
		inv.TestCommands = append(inv.TestCommands, Command{Command: "pytest", Reason: "Python project metadata suggests pytest may be available"})
	}
	switch {
	case base == "diesel.toml":
		add("migration_system", &inv.MigrationSystems, "diesel", "Diesel config")
		inv.NativeCommands = append(inv.NativeCommands, Command{Command: "diesel migration run", Reason: "Diesel config exposes native migration execution"})
	case base == "knexfile.js" || base == "knexfile.ts":
		add("migration_system", &inv.MigrationSystems, "knex", "Knex configuration file")
		inv.NativeCommands = append(inv.NativeCommands, Command{Command: "npx knex migrate:latest", Reason: "knexfile exposes native Knex migration execution"})
	case base == ".sequelizerc" || strings.Contains(lower, "/sequelize/migrations/"):
		add("migration_system", &inv.MigrationSystems, "sequelize", "Sequelize migrations configuration")
		inv.NativeCommands = append(inv.NativeCommands, Command{Command: "npx sequelize-cli db:migrate", Reason: "Sequelize configuration exposes native migration execution"})
	case strings.Contains(lower, "database/migrations/"):
		add("migration_system", &inv.MigrationSystems, "laravel", "Laravel database/migrations path")
		inv.NativeCommands = append(inv.NativeCommands, Command{Command: "php artisan migrate", Reason: "Laravel database/migrations path exposes native migration execution"})
	case strings.Contains(lower, "priv/repo/migrations/") || (strings.Contains(lower, "/migrations/") && strings.HasSuffix(lower, ".exs")):
		add("migration_system", &inv.MigrationSystems, "ecto", "Ecto migrations path")
		inv.NativeCommands = append(inv.NativeCommands, Command{Command: "mix ecto.migrate", Reason: "Ecto migrations path exposes native migration execution"})
	case dieselSQLPairPattern.MatchString(lower):
		add("migration_system", &inv.MigrationSystems, "diesel", "Diesel up/down migration pair")
		inv.NativeCommands = append(inv.NativeCommands, Command{Command: "diesel migration run", Reason: "Diesel up/down migration pair exposes native migration execution"})
	case doctrineVersionPattern.MatchString(base) || strings.Contains(lower, "doctrinemigrations/"):
		add("migration_system", &inv.MigrationSystems, "doctrine", "Doctrine Migrations version class")
		inv.NativeCommands = append(inv.NativeCommands, Command{Command: "php bin/console doctrine:migrations:migrate", Reason: "Doctrine Migrations version class exposes native migration execution"})
	case railsMultiDBPattern.MatchString(lower):
		dbName := railsMultiDBName(lower)
		add("migration_system", &inv.MigrationSystems, "rails-multi-db", "Rails multi-database migration path for database "+dbName)
		inv.NativeCommands = append(inv.NativeCommands, Command{Command: "bundle exec rails db:migrate:" + dbName, Reason: "Rails multi-database path exposes a per-database native migration command"})
	case strings.Contains(lower, "db/migrate/"):
		add("migration_root", &inv.MigrationRoots, "rails-or-generic-db-migrate", "db/migrate migration path")
		inv.NativeCommands = append(inv.NativeCommands, Command{Command: "bundle exec rails db:migrate", Reason: "db/migrate path suggests Rails-style migration execution from the project root"})
	case strings.Contains(lower, "prisma/migrations/"):
		add("migration_system", &inv.MigrationSystems, "prisma", "Prisma migrations path")
		inv.NativeCommands = append(inv.NativeCommands, Command{Command: "npx prisma migrate status", Reason: "Prisma migrations path exposes native migration status checks"})
	case strings.Contains(lower, "migrations/"):
		add("migration_root", &inv.MigrationRoots, "migrations", "migrations path")
		if strings.HasSuffix(lower, ".py") {
			inv.NativeCommands = append(inv.NativeCommands, Command{Command: "python manage.py migrate", Reason: "Python migrations path suggests Django-style migration execution from the project root"})
		}
	case base == "alembic.ini":
		add("migration_system", &inv.MigrationSystems, "alembic", "Alembic config")
		inv.NativeCommands = append(inv.NativeCommands, Command{Command: "alembic upgrade head", Reason: "Alembic config exposes native migration execution"})
	case base == "flyway.conf":
		add("migration_system", &inv.MigrationSystems, "flyway", "Flyway config")
		inv.NativeCommands = append(inv.NativeCommands, Command{Command: "flyway migrate", Reason: "Flyway config exposes native migration execution"})
	case base == "liquibase.properties":
		add("migration_system", &inv.MigrationSystems, "liquibase", "Liquibase config")
		inv.NativeCommands = append(inv.NativeCommands, Command{Command: "liquibase update", Reason: "Liquibase config exposes native migration execution"})
	}
	if isSourceSQLCandidate(lower) {
		add("source_sql_hint", &inv.SourceSQLHints, "source-sql-candidate", "source file type commonly embeds SQL or ORM persistence calls")
	}
	if isOperationalDoc(lower) {
		add("operational_doc", &inv.OperationalDocs, "operational-doc", "incident, rollback, runbook, or release-note path")
	}
	if isEvidenceExport(lower) {
		add("evidence_export", &inv.EvidenceExports, "evidence-export", "log, trace, JSON/JSONL, CSV, or SARIF-like export")
	}
}

const factContentLimit = 64 << 10

var (
	identifierDatePattern      = regexp.MustCompile(`\b20[0-9]{2}[-_/]?[01][0-9][-_/]?[0-3][0-9]\b`)
	identifierTimestampPattern = regexp.MustCompile(`\b20[0-9]{2}-[01][0-9]-[0-3][0-9][T ][0-2][0-9]:[0-5][0-9](?::[0-5][0-9](?:\.[0-9]+)?)?(?:Z|[+-][0-9]{2}:?[0-9]{2})?\b`)
	identifierIncidentPattern  = regexp.MustCompile(`(?i)\b(?:incident|inc|sev)[-_:# ]*[0-9]+\b`)
	identifierPRPattern        = regexp.MustCompile(`(?i)\b(?:pr|pull request)[-:# ]*[0-9]+\b|#[0-9]+\b`)
	identifierCommitPattern    = regexp.MustCompile(`\b[0-9a-f]{40}\b`)
	identifierSQLTablePattern  = regexp.MustCompile(`(?i)\b(?:update|delete\s+from|insert\s+into|alter\s+table|create\s+table|drop\s+table|truncate\s+table)\s+(?:if\s+(?:not\s+)?exists\s+)?["'\[]?([A-Za-z_][A-Za-z0-9_.$-]*)`)
	identifierColumnPatterns   = []*regexp.Regexp{
		regexp.MustCompile(`(?i)\b(?:add|drop|rename)\s+column\s+["'\[]?([A-Za-z_][A-Za-z0-9_]*)`),
		regexp.MustCompile(`(?i)\bset\s+["'\[]?([A-Za-z_][A-Za-z0-9_]*)["'\]]?\s*=`),
		regexp.MustCompile(`(?i)\bwhere\s+["'\[]?([A-Za-z_][A-Za-z0-9_]*)["'\]]?\s*(?:=|>|<|in\b|is\b|like\b)`),
	}
	identifierModelPatterns = []*regexp.Regexp{
		regexp.MustCompile(`\bclass\s+([A-Z][A-Za-z0-9_]+)\s*(?:<\s*ApplicationRecord|\([^)]*Model[^)]*\))`),
		regexp.MustCompile(`\btype\s+([A-Z][A-Za-z0-9_]+)\s+struct\b`),
		regexp.MustCompile(`\bmodel\s+([A-Z][A-Za-z0-9_]+)\s*\{`),
	}
	identifierEndpointPatterns = []*regexp.Regexp{
		regexp.MustCompile(`\b(?:GET|POST|PUT|PATCH|DELETE)\s+(/[A-Za-z0-9_./:{}-]+)`),
		regexp.MustCompile(`(?i)\b(?:route|path|endpoint)\s*[:=]\s*["']?(/[A-Za-z0-9_./:{}-]+)`),
	}
	identifierQueuePattern  = regexp.MustCompile(`(?i)\b(?:queue|topic|stream)\s*[:=]\s*["']?([A-Za-z0-9_.:/-]{3,})`)
	identifierJobPattern    = regexp.MustCompile(`(?i)\b(?:job|worker|task|cron)\s*[:=]\s*["']?([A-Za-z0-9_.:/-]{3,})|\b([A-Z][A-Za-z0-9_]*(?:Job|Worker|Task))\b`)
	identifierReportPattern = regexp.MustCompile(`(?i)\b(?:report|dashboard)\s*[:=]\s*["']?([A-Za-z0-9_.:/-]{3,})|([A-Za-z0-9_.-]*report[A-Za-z0-9_.-]*)`)
	identifierDeployPattern = regexp.MustCompile(`(?i)\b(?:deploy(?:ment)?|release|build)\s*[:=#-]\s*["']?([A-Za-z0-9][A-Za-z0-9_.-]{2,})`)
	identifierErrorPattern  = regexp.MustCompile(`(?i)\b([A-Za-z][A-Za-z0-9_]*(?:Error|Exception))\b|\b(?:error|exception|panic)\s*[:=]\s*["']?([A-Za-z_][A-Za-z0-9_.:-]+)`)
	fieldLinePattern        = regexp.MustCompile(`^\s*([A-Za-z_][A-Za-z0-9_.-]*)\s*[:=]\s*(.+?)\s*$`)
	logFieldPattern         = regexp.MustCompile(`\b([A-Za-z_][A-Za-z0-9_.-]*)=("[^"]*"|'[^']*'|[^\s,;]+)`)

	djangoCreateModelPattern = regexp.MustCompile(`(?is)migrations\.CreateModel\(\s*name\s*=\s*['"]([A-Za-z_][A-Za-z0-9_]*)['"]`)
	djangoAddFieldPattern    = regexp.MustCompile(`(?is)migrations\.AddField\([^)]*model_name\s*=\s*['"]([A-Za-z_][A-Za-z0-9_]*)['"][^)]*name\s*=\s*['"]([A-Za-z_][A-Za-z0-9_]*)['"]`)
	railsCreateTablePattern  = regexp.MustCompile(`(?i)\bcreate_table\s+[:'"]([A-Za-z_][A-Za-z0-9_]*)`)
	railsAddColumnPattern    = regexp.MustCompile(`(?i)\badd_column\s+[:'"]([A-Za-z_][A-Za-z0-9_]*)['"]?\s*,\s*[:'"]([A-Za-z_][A-Za-z0-9_]*)`)
	prismaModelPattern       = regexp.MustCompile(`(?s)\bmodel\s+([A-Za-z_][A-Za-z0-9_]*)\s*\{([^}]*)\}`)
	dieselSQLPairPattern     = regexp.MustCompile(`migrations/[^/]+/(up|down)\.sql$`)
	doctrineVersionPattern   = regexp.MustCompile(`^version[0-9]{6,}\.php$`)
	railsMultiDBPattern      = regexp.MustCompile(`(?:^|/)db/([a-z0-9_]+)_migrate/`)
)

func railsMultiDBName(lower string) string {
	if m := railsMultiDBPattern.FindStringSubmatch(lower); len(m) == 2 {
		return m[1]
	}
	return "primary"
}

func (inv *Inventory) addFileFact(file scanFile, language string) {
	props := map[string]string{"size_bytes": fmt.Sprintf("%d", file.Size)}
	if language != "" {
		props["language"] = language
	}
	if sum, err := hashFile(file.Abs); err == nil {
		props["content_hash"] = sum
	} else {
		props["content_hash_error"] = "unreadable"
	}
	prefix, err := readTextPrefix(file.Abs, factContentLimit)
	if err != nil {
		props["read_error"] = "unreadable"
	}
	text := file.Rel
	if prefix != "" {
		text += "\n" + prefix
	}
	inv.addFact(Fact{
		Version:     Version,
		Kind:        "file",
		Path:        file.Rel,
		Confidence:  "observed",
		Rationale:   "file discovered during project inventory",
		Identifiers: identifiersFromText(text),
		Properties:  props,
	})
	if prefix != "" {
		inv.inferSchemaEvolution(file, prefix, language)
		inv.preserveFieldEvidence(file, prefix)
		inv.scanInfrastructure(file, prefix)
	}
}

func (inv *Inventory) addFindingFact(category string, finding Finding) {
	inv.addFact(Fact{
		Version:     Version,
		Kind:        category,
		Path:        finding.Path,
		Confidence:  finding.Confidence,
		Rationale:   finding.Rationale,
		Identifiers: identifiersFromText(finding.Path + "\n" + finding.Kind),
		Properties:  map[string]string{"kind": finding.Kind},
	})
}

func (inv *Inventory) inferSchemaEvolution(file scanFile, text, language string) {
	lower := strings.ToLower(file.Rel)
	if strings.HasSuffix(lower, ".sql") || strings.HasSuffix(lower, ".psql") || strings.HasSuffix(lower, ".ddl") {
		report, err := migration.AnalyzeMigrationSemantics(file.Rel, []byte(text), migration.DialectGeneric, migration.SchemaState{Version: migration.SchemaVersion})
		if err == nil {
			for _, transformation := range report.Transformations {
				inv.addSchemaEvolutionFact(file.Rel, "sql", transformation.Kind, transformation.Table, schemaColumnName(transformation.Column), schemaColumnType(transformation.Column), fmt.Sprintf("%d", transformation.Index))
			}
		}
	}
	switch language {
	case "Python":
		for _, match := range djangoCreateModelPattern.FindAllStringSubmatch(text, -1) {
			inv.addSchemaEvolutionFact(file.Rel, "django-migration", "create_model", match[1], "", "", "")
		}
		for _, match := range djangoAddFieldPattern.FindAllStringSubmatch(text, -1) {
			inv.addSchemaEvolutionFact(file.Rel, "django-migration", "add_field", match[1], match[2], "", "")
		}
	case "Ruby":
		for _, match := range railsCreateTablePattern.FindAllStringSubmatch(text, -1) {
			inv.addSchemaEvolutionFact(file.Rel, "rails-migration", "create_table", match[1], "", "", "")
		}
		for _, match := range railsAddColumnPattern.FindAllStringSubmatch(text, -1) {
			inv.addSchemaEvolutionFact(file.Rel, "rails-migration", "add_column", match[1], match[2], "", "")
		}
	case "Prisma", "JavaScript", "TypeScript":
		for _, match := range prismaModelPattern.FindAllStringSubmatch(text, -1) {
			table := match[1]
			inv.addSchemaEvolutionFact(file.Rel, "prisma-schema", "model", table, "", "", "")
			for _, column := range prismaModelColumns(match[2]) {
				inv.addSchemaEvolutionFact(file.Rel, "prisma-schema", "field", table, column.Name, column.Type, "")
			}
		}
	}
}

func (inv *Inventory) addSchemaEvolutionFact(path, source, operation, table, column, columnType, index string) {
	table = normalizeProjectIdentifierValue("table", table)
	column = normalizeProjectIdentifierValue("column", column)
	if operation == "" || (table == "" && column == "") {
		return
	}
	findingKind := source + ":" + operation
	inv.SchemaEvolution = append(inv.SchemaEvolution, Finding{Kind: findingKind, Path: path, Confidence: "derived", Rationale: "schema evolution inferred from project-native migration or ORM declaration"})
	props := map[string]string{"source": source, "operation": operation}
	var ids []Identifier
	if table != "" {
		props["table"] = table
		ids = append(ids, Identifier{Kind: "table", Value: table}, Identifier{Kind: "model", Value: table})
	}
	if column != "" {
		props["column"] = column
		ids = append(ids, Identifier{Kind: "column", Value: column})
	}
	if columnType != "" {
		props["column_type"] = columnType
	}
	if index != "" {
		props["statement_index"] = index
	}
	inv.addFact(Fact{
		Version:     Version,
		Kind:        "schema_evolution",
		Path:        path,
		Confidence:  "derived",
		Rationale:   "schema evolution inferred without requiring a pre-authored Patchline schema",
		Identifiers: ids,
		Properties:  props,
	})
}

func schemaColumnName(column *migration.SchemaColumn) string {
	if column == nil {
		return ""
	}
	return column.Name
}

func schemaColumnType(column *migration.SchemaColumn) string {
	if column == nil {
		return ""
	}
	return column.Type
}

func prismaModelColumns(body string) []migration.SchemaColumn {
	var columns []migration.SchemaColumn
	for _, line := range strings.Split(body, "\n") {
		fields := strings.Fields(strings.TrimSpace(line))
		if len(fields) < 2 || strings.HasPrefix(fields[0], "//") || strings.HasPrefix(fields[0], "@") {
			continue
		}
		name := normalizeProjectIdentifierValue("column", fields[0])
		if name == "" {
			continue
		}
		columns = append(columns, migration.SchemaColumn{Name: name, Type: strings.ToLower(fields[1])})
	}
	return columns
}

func (inv *Inventory) preserveFieldEvidence(file scanFile, text string) {
	lower := strings.ToLower(file.Rel)
	switch filepath.Ext(lower) {
	case ".json":
		inv.preserveJSONFields(file.Rel, text, "json")
	case ".jsonl":
		for lineNo, line := range strings.Split(text, "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			inv.preserveJSONFieldsAtLine(file.Rel, line, "jsonl", lineNo+1)
		}
	case ".yaml", ".yml":
		inv.preserveLineFields(file.Rel, text, "yaml")
	case ".toml":
		inv.preserveLineFields(file.Rel, text, "toml")
	case ".log":
		inv.preserveLogFields(file.Rel, text, "log")
	}
}

func (inv *Inventory) preserveJSONFields(path, text, format string) {
	inv.preserveJSONFieldsAtLine(path, text, format, 0)
}

func (inv *Inventory) preserveJSONFieldsAtLine(path, text, format string, lineNo int) {
	var value any
	decoder := json.NewDecoder(strings.NewReader(text))
	decoder.UseNumber()
	if err := decoder.Decode(&value); err != nil {
		return
	}
	count := 0
	var walk func(prefix string, current any)
	walk = func(prefix string, current any) {
		if count >= 50 {
			return
		}
		switch v := current.(type) {
		case map[string]any:
			keys := make([]string, 0, len(v))
			for key := range v {
				keys = append(keys, key)
			}
			sort.Strings(keys)
			for _, key := range keys {
				next := key
				if prefix != "" {
					next = prefix + "." + key
				}
				walk(next, v[key])
			}
		case []any:
			for i, item := range v {
				if i >= 10 {
					break
				}
				walk(fmt.Sprintf("%s[]", prefix), item)
			}
		default:
			inv.addFieldEvidenceFact(path, format, prefix, fieldValuePreview(v), lineNo)
			count++
		}
	}
	walk("", value)
}

func (inv *Inventory) preserveLineFields(path, text, format string) {
	for lineNo, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "//") || strings.HasPrefix(line, "[") {
			continue
		}
		match := fieldLinePattern.FindStringSubmatch(line)
		if len(match) < 3 {
			continue
		}
		key := match[1]
		value := strings.Trim(strings.TrimSpace(match[2]), `"'`)
		inv.addFieldEvidenceFact(path, format, key, value, lineNo+1)
	}
}

func (inv *Inventory) preserveLogFields(path, text, format string) {
	for lineNo, line := range strings.Split(text, "\n") {
		for _, match := range logFieldPattern.FindAllStringSubmatch(line, -1) {
			if len(match) < 3 {
				continue
			}
			inv.addFieldEvidenceFact(path, format, match[1], strings.Trim(match[2], `"'`), lineNo+1)
		}
	}
}

func (inv *Inventory) addFieldEvidenceFact(path, format, key, value string, lineNo int) {
	key = normalizeFieldKey(key)
	if key == "" {
		return
	}
	value = strings.TrimSpace(value)
	if value == "" || strings.EqualFold(value, "{}") || strings.EqualFold(value, "[]") {
		return
	}
	props := map[string]string{
		"format":        format,
		"field":         key,
		"value_preview": truncate(value, 160),
	}
	if lineNo > 0 {
		props["line"] = fmt.Sprintf("%d", lineNo)
	}
	inv.FieldEvidence = append(inv.FieldEvidence, Finding{Kind: format + ":" + key, Path: path, Confidence: "observed", Rationale: "unknown structured field preserved as searchable evidence"})
	ids := append([]Identifier{{Kind: "field", Value: key}}, identifiersFromText(key+"\n"+value)...)
	inv.addFact(Fact{
		Version:     Version,
		Kind:        "field_evidence",
		Path:        path,
		Confidence:  "observed",
		Rationale:   "unknown structured/log field preserved without requiring a known schema",
		Identifiers: ids,
		Properties:  props,
	})
}

func (inv *Inventory) scanInfrastructure(file scanFile, text string) {
	lower := strings.ToLower(file.Rel)
	switch {
	case isKubernetesConfigPath(lower, text):
		inv.scanKubernetesConfig(file.Rel, text)
	case strings.HasSuffix(lower, ".tf") || strings.HasSuffix(lower, ".tfvars"):
		inv.scanTerraformConfig(file.Rel, text)
	}
}

func isKubernetesConfigPath(path, text string) bool {
	if !(strings.HasSuffix(path, ".yaml") || strings.HasSuffix(path, ".yml") || strings.HasSuffix(path, ".tpl")) {
		return false
	}
	lowerText := strings.ToLower(text)
	return containsAny(path, "k8s/", "kubernetes/", "helm/", "chart/", "charts/", "deploy/") ||
		(containsAny(lowerText, "apiversion:", "kind:") && containsAny(lowerText, "deployment", "statefulset", "job", "cronjob", "secretkeyref", "initcontainers"))
}

func (inv *Inventory) scanKubernetesConfig(path, text string) {
	lower := strings.ToLower(text)
	resourceKind := firstKubernetesValue(text, "kind")
	resourceName := firstKubernetesNestedName(text)
	schedule := firstKubernetesValue(text, "schedule")
	if resourceName == "" {
		resourceName = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	}
	if containsAny(lower, "secretkeyref", "secretref", "secretname:", "envfrom:", "volumes:") && containsAny(lower, "secret", "password", "token", "credential") {
		inv.addInfrastructureFact(path, "kubernetes_secret_reference", 0, resourceKind, resourceName, schedule, secretNamesFromText(text), []string{"secretKeyRef", "secretName"}, "Kubernetes workload references secrets or secret-derived environment/configuration")
	}
	if containsAny(lower, "initcontainers:", "helm.sh/hook", "argocd.argoproj.io/sync-wave", "depends-on", "post-install", "pre-install", "pre-upgrade", "post-upgrade") {
		inv.addInfrastructureFact(path, "kubernetes_deploy_ordering", 0, resourceKind, resourceName, schedule, nil, deployOrderMarkers(lower), "Kubernetes or Helm manifest encodes deploy ordering that can affect data-change safety")
	}
	if isKubernetesJobKind(resourceKind) || containsAny(lower, "kind: job", "kind: cronjob") {
		if containsAny(lower, "migrate", "migration", "db:migrate", "alembic", "prisma migrate", "flyway", "liquibase") {
			inv.addInfrastructureFact(path, "kubernetes_migration_job", 0, resourceKind, resourceName, schedule, nil, commandMarkers(lower), "Kubernetes job or cron job appears to run database migrations")
		}
		if containsAny(lower, "postgres", "mysql", "mariadb", "mongodb", "redis", "database", "db_") {
			inv.addInfrastructureFact(path, "kubernetes_database_job", 0, resourceKind, resourceName, schedule, nil, commandMarkers(lower), "Kubernetes job or cron job is coupled to database services or credentials")
		}
		if strings.EqualFold(resourceKind, "CronJob") && containsAny(lower, "repair", "rollback", "reconcile", "backfill", "fix") {
			inv.addInfrastructureFact(path, "kubernetes_cron_repair", 0, resourceKind, resourceName, schedule, nil, commandMarkers(lower), "Kubernetes CronJob appears to run recurring repair, rollback, reconcile, or backfill work")
		}
	}
}

func (inv *Inventory) scanTerraformConfig(path, text string) {
	lower := strings.ToLower(text)
	resourceKind, resourceName := firstTerraformResource(text)
	if containsAny(lower, "kubernetes_secret", "secret_name", "secret_key_ref", "password", "token", "credential", "sensitive = true") {
		inv.addInfrastructureFact(path, "terraform_secret_reference", 0, resourceKind, resourceName, "", secretNamesFromText(text), terraformMarkers(lower, "secret"), "Terraform configuration references secrets or secret-valued inputs near deployment resources")
	}
	if containsAny(lower, "depends_on", "wait = true", "atomic = true", "helm_release", "kubernetes_job", "kubernetes_cron_job") {
		inv.addInfrastructureFact(path, "terraform_deploy_ordering", 0, resourceKind, resourceName, "", nil, terraformMarkers(lower, "ordering"), "Terraform configuration encodes deploy ordering or waits that can gate database-affecting rollout")
	}
	if containsAny(lower, "kubernetes_job", "kubernetes_cron_job", "helm_release", "null_resource") && containsAny(lower, "migrate", "migration", "db:migrate", "alembic", "prisma migrate", "flyway", "liquibase") {
		inv.addInfrastructureFact(path, "terraform_migration_job", 0, resourceKind, resourceName, "", nil, commandMarkers(lower), "Terraform-managed resource appears to run database migrations")
	}
	if containsAny(lower, "kubernetes_job", "kubernetes_cron_job", "postgres", "mysql", "database_url", "db_") {
		inv.addInfrastructureFact(path, "terraform_database_job", 0, resourceKind, resourceName, "", nil, commandMarkers(lower), "Terraform-managed resource is coupled to database jobs or database credentials")
	}
	if containsAny(lower, "kubernetes_cron_job", "schedule", "cron") && containsAny(lower, "repair", "rollback", "reconcile", "backfill", "fix") {
		inv.addInfrastructureFact(path, "terraform_cron_repair", 0, resourceKind, resourceName, "", nil, commandMarkers(lower), "Terraform-managed cron resource appears to run recurring repair, rollback, reconcile, or backfill work")
	}
}

func (inv *Inventory) addInfrastructureFact(path, kind string, line int, resourceKind, resourceName, schedule string, secretRefs, markers []string, rationale string) {
	props := map[string]string{"kind": kind}
	if resourceKind != "" {
		props["resource_kind"] = resourceKind
	}
	if resourceName != "" {
		props["resource_name"] = resourceName
	}
	if schedule != "" {
		props["schedule"] = schedule
	}
	if line > 0 {
		props["line"] = fmt.Sprintf("%d", line)
	}
	if len(secretRefs) > 0 {
		props["secret_refs"] = strings.Join(uniqueSortedStrings(secretRefs), ",")
	}
	if len(markers) > 0 {
		props["markers"] = strings.Join(uniqueSortedStrings(markers), ",")
	}
	confidence := "derived"
	inv.Infrastructure = append(inv.Infrastructure, Finding{Kind: kind, Path: path, Confidence: confidence, Rationale: rationale})
	ids := identifiersFromText(strings.Join([]string{path, kind, resourceKind, resourceName, schedule, strings.Join(secretRefs, " "), strings.Join(markers, " ")}, "\n"))
	if resourceName != "" {
		ids = append(ids, Identifier{Kind: "job", Value: normalizeProjectIdentifierValue("job", resourceName)})
	}
	inv.addFact(Fact{
		Version:     Version,
		Kind:        "infrastructure",
		Path:        path,
		Confidence:  confidence,
		Rationale:   rationale,
		Identifiers: ids,
		Properties:  props,
	})
}

func firstKubernetesValue(text, key string) string {
	pattern := regexp.MustCompile(`(?im)^\s*` + regexp.QuoteMeta(key) + `\s*:\s*["']?([^"'\n#]+)`)
	match := pattern.FindStringSubmatch(text)
	if len(match) < 2 {
		return ""
	}
	return strings.TrimSpace(match[1])
}

func firstKubernetesNestedName(text string) string {
	lines := strings.Split(text, "\n")
	inMetadata := false
	metadataIndent := -1
	for _, line := range lines {
		indent := leadingSpaces(line)
		trimmed := strings.TrimSpace(line)
		if trimmed == "metadata:" {
			inMetadata = true
			metadataIndent = indent
			continue
		}
		if inMetadata && indent <= metadataIndent && trimmed != "" {
			inMetadata = false
		}
		if inMetadata && strings.HasPrefix(trimmed, "name:") {
			return strings.Trim(strings.TrimSpace(strings.TrimPrefix(trimmed, "name:")), `"'`)
		}
	}
	return ""
}

func firstTerraformResource(text string) (string, string) {
	pattern := regexp.MustCompile(`(?m)^\s*resource\s+"([^"]+)"\s+"([^"]+)"`)
	match := pattern.FindStringSubmatch(text)
	if len(match) < 3 {
		return "", ""
	}
	return match[1], match[2]
}

func leadingSpaces(line string) int {
	count := 0
	for _, r := range line {
		if r != ' ' {
			break
		}
		count++
	}
	return count
}

func isKubernetesJobKind(kind string) bool {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "job", "cronjob":
		return true
	default:
		return false
	}
}

func secretNamesFromText(text string) []string {
	pattern := regexp.MustCompile(`(?im)\b(?:secretName|name|secret_name)\s*[:=]\s*["']?([A-Za-z0-9_.:/-]+)`)
	matches := pattern.FindAllStringSubmatch(text, -1)
	var names []string
	for _, match := range matches {
		if len(match) > 1 && containsAny(strings.ToLower(match[0]), "secret") {
			names = append(names, strings.Trim(match[1], `"',`))
		}
	}
	blockPattern := regexp.MustCompile(`(?is)(?:secretKeyRef|secretRef|secret)\s*:\s*(?:\n\s+[A-Za-z0-9_.-]+\s*:\s*[^\n#]+)*?\n\s+name\s*:\s*["']?([A-Za-z0-9_.:/-]+)`)
	for _, match := range blockPattern.FindAllStringSubmatch(text, -1) {
		if len(match) > 1 {
			names = append(names, strings.Trim(match[1], `"',`))
		}
	}
	return capStrings(uniqueSortedStrings(names), 8)
}

func deployOrderMarkers(lower string) []string {
	markers := []string{}
	for _, marker := range []string{"initcontainers", "helm.sh/hook", "argocd.argoproj.io/sync-wave", "depends-on", "pre-install", "post-install", "pre-upgrade", "post-upgrade"} {
		if strings.Contains(lower, marker) {
			markers = append(markers, marker)
		}
	}
	return markers
}

func terraformMarkers(lower, category string) []string {
	var candidates []string
	switch category {
	case "secret":
		candidates = []string{"kubernetes_secret", "secret_name", "secret_key_ref", "set_sensitive", "password", "token", "credential", "sensitive = true"}
	default:
		candidates = []string{"depends_on", "wait = true", "atomic = true", "helm_release", "kubernetes_job", "kubernetes_cron_job"}
	}
	var markers []string
	for _, marker := range candidates {
		if strings.Contains(lower, marker) {
			markers = append(markers, marker)
		}
	}
	return markers
}

func commandMarkers(lower string) []string {
	var markers []string
	for _, marker := range []string{"migrate", "migration", "db:migrate", "alembic", "prisma migrate", "flyway", "liquibase", "postgres", "mysql", "database", "repair", "rollback", "reconcile", "backfill", "fix"} {
		if strings.Contains(lower, marker) {
			markers = append(markers, marker)
		}
	}
	return markers
}

func normalizeFieldKey(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	value = strings.Trim(value, `"'[](),;`)
	if len(value) > 120 {
		value = value[:120]
	}
	return value
}

func fieldValuePreview(value any) string {
	switch v := value.(type) {
	case nil:
		return ""
	case string:
		return v
	case json.Number:
		return v.String()
	case bool:
		if v {
			return "true"
		}
		return "false"
	default:
		data, err := json.Marshal(v)
		if err != nil {
			return fmt.Sprintf("%v", v)
		}
		return string(data)
	}
}

func truncate(value string, limit int) string {
	if limit <= 0 || len(value) <= limit {
		return value
	}
	return value[:limit]
}

func (inv *Inventory) addCommandFact(kind string, command Command) {
	inv.addFact(Fact{
		Version:    Version,
		Kind:       kind,
		Confidence: "derived",
		Rationale:  command.Reason,
		Properties: map[string]string{"command": command.Command},
	})
}

func (inv *Inventory) addFact(fact Fact) {
	fact.Version = Version
	fact.Identifiers = uniqueIdentifiers(fact.Identifiers)
	if fact.Properties != nil && len(fact.Properties) == 0 {
		fact.Properties = nil
	}
	fact.ID = factID(fact)
	inv.Facts = append(inv.Facts, fact)
}

func factID(fact Fact) string {
	fact.ID = ""
	return "fact:" + canonical.Hash(fact)[:20]
}

func identifiersFromText(text string) []Identifier {
	var ids []Identifier
	addMatches := func(kind string, matches []string) {
		for _, match := range matches {
			value := normalizeProjectIdentifierValue(kind, match)
			if value != "" {
				ids = append(ids, Identifier{Kind: kind, Value: value})
			}
		}
	}
	addSubmatches := func(kind string, pattern *regexp.Regexp) {
		for _, match := range pattern.FindAllStringSubmatch(text, -1) {
			for _, value := range match[1:] {
				value = normalizeProjectIdentifierValue(kind, value)
				if value != "" {
					ids = append(ids, Identifier{Kind: kind, Value: value})
					break
				}
			}
		}
	}
	addMatches("date", identifierDatePattern.FindAllString(text, -1))
	addMatches("timestamp", identifierTimestampPattern.FindAllString(text, -1))
	addMatches("incident", identifierIncidentPattern.FindAllString(text, -1))
	addMatches("pull_request", identifierPRPattern.FindAllString(text, -1))
	addMatches("commit", identifierCommitPattern.FindAllString(strings.ToLower(text), -1))
	for _, match := range identifierSQLTablePattern.FindAllStringSubmatch(text, -1) {
		if len(match) > 1 {
			value := strings.ToLower(strings.Trim(match[1], `"'[]`))
			if !isSQLIdentifierStopword(value) {
				ids = append(ids, Identifier{Kind: "table", Value: value})
			}
		}
	}
	for _, pattern := range identifierColumnPatterns {
		addSubmatches("column", pattern)
	}
	for _, pattern := range identifierModelPatterns {
		addSubmatches("model", pattern)
	}
	for _, pattern := range identifierEndpointPatterns {
		addSubmatches("endpoint", pattern)
	}
	addSubmatches("queue", identifierQueuePattern)
	addSubmatches("job", identifierJobPattern)
	addSubmatches("report", identifierReportPattern)
	addSubmatches("deploy", identifierDeployPattern)
	addSubmatches("error", identifierErrorPattern)
	return uniqueIdentifiers(ids)
}

func normalizeProjectIdentifierValue(kind, value string) string {
	value = strings.TrimSpace(value)
	value = strings.Trim(value, `"'[](),;`)
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	value = strings.ToLower(value)
	switch kind {
	case "endpoint":
		if !strings.HasPrefix(value, "/") {
			return ""
		}
	case "table", "column", "model", "queue", "job", "report", "deploy", "error":
		if isSQLIdentifierStopword(value) {
			return ""
		}
	}
	return value
}

func isSQLIdentifierStopword(value string) bool {
	switch value {
	case "", "if", "not", "exists", "set", "where", "select", "from", "the":
		return true
	default:
		return false
	}
}

func uniqueIdentifiers(in []Identifier) []Identifier {
	seen := map[string]bool{}
	var out []Identifier
	for _, item := range in {
		if item.Kind == "" || item.Value == "" {
			continue
		}
		key := item.Kind + "\x00" + item.Value
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, item)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Kind != out[j].Kind {
			return out[i].Kind < out[j].Kind
		}
		return out[i].Value < out[j].Value
	})
	return out
}

func uniqueFacts(in []Fact) []Fact {
	sort.Slice(in, func(i, j int) bool {
		if in[i].Kind != in[j].Kind {
			return in[i].Kind < in[j].Kind
		}
		if in[i].Path != in[j].Path {
			return in[i].Path < in[j].Path
		}
		return in[i].ID < in[j].ID
	})
	seen := map[string]bool{}
	var out []Fact
	for _, item := range in {
		if item.ID == "" || seen[item.ID] {
			continue
		}
		seen[item.ID] = true
		out = append(out, item)
	}
	return out
}

func readTextPrefix(path string, limit int64) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, limit))
	if err != nil {
		return "", err
	}
	if bytes.IndexByte(data, 0) >= 0 || !utf8.Valid(data) {
		return "", nil
	}
	return string(data), nil
}

func (inv *Inventory) finalize() {
	inv.Frameworks = uniqueFindings(inv.Frameworks)
	inv.MigrationSystems = uniqueFindings(inv.MigrationSystems)
	inv.MigrationRoots = uniqueFindings(inv.MigrationRoots)
	inv.CI = uniqueFindings(inv.CI)
	inv.DeployConfig = uniqueFindings(inv.DeployConfig)
	inv.SourceSQLHints = capFindings(uniqueFindings(inv.SourceSQLHints), 50)
	inv.SchemaEvolution = capFindings(uniqueFindings(inv.SchemaEvolution), 50)
	inv.OperationalDocs = capFindings(uniqueFindings(inv.OperationalDocs), 50)
	inv.EvidenceExports = capFindings(uniqueFindings(inv.EvidenceExports), 50)
	inv.Infrastructure = capFindings(uniqueFindings(inv.Infrastructure), 100)
	inv.TestCommands = uniqueCommands(inv.TestCommands)
	inv.NativeCommands = uniqueCommands(inv.NativeCommands)
	inv.FieldEvidence = capFindings(uniqueFindings(inv.FieldEvidence), 100)
	inv.NextCommands = append(inv.NextCommands, Command{Command: fmt.Sprintf("patchline intake %s --out results/generated/intake", shellPath(inv.Root)), Reason: "run deterministic data/code repair intake on this project"})
	if len(inv.TestCommands) > 0 {
		inv.NextCommands = append(inv.NextCommands, inv.TestCommands...)
	}
	if len(inv.NativeCommands) > 0 {
		inv.NextCommands = append(inv.NextCommands, inv.NativeCommands...)
	}
	inv.NextCommands = uniqueCommands(inv.NextCommands)
	for _, language := range inv.Languages {
		inv.addFact(Fact{
			Version:    Version,
			Kind:       "language",
			Confidence: "extension",
			Rationale:  "language inferred from file extensions",
			Properties: map[string]string{"language": language.Name, "files": fmt.Sprintf("%d", language.Count)},
		})
	}
	for _, command := range inv.TestCommands {
		inv.addCommandFact("test_command", command)
	}
	for _, command := range inv.NativeCommands {
		inv.addCommandFact("native_command", command)
	}
	for _, command := range inv.NextCommands {
		inv.addCommandFact("next_command", command)
	}
	inv.SummaryByCategory = map[string]int{
		"ci":                len(inv.CI),
		"deploy_config":     len(inv.DeployConfig),
		"evidence_exports":  len(inv.EvidenceExports),
		"field_evidence":    len(inv.FieldEvidence),
		"frameworks":        len(inv.Frameworks),
		"infrastructure":    len(inv.Infrastructure),
		"migration_roots":   len(inv.MigrationRoots),
		"migration_systems": len(inv.MigrationSystems),
		"native_commands":   len(inv.NativeCommands),
		"schema_evolution":  len(inv.SchemaEvolution),
		"operational_docs":  len(inv.OperationalDocs),
		"source_sql_hints":  len(inv.SourceSQLHints),
	}
	inv.Facts = uniqueFacts(inv.Facts)
	inv.Markdown = renderMarkdown(*inv)
	inv.ProjectMap = renderProjectMap(*inv)
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
	writeFindings("Schema evolution", inv.SchemaEvolution)
	writeFindings("Field evidence", inv.FieldEvidence)
	writeFindings("CI", inv.CI)
	writeFindings("Deploy config", inv.DeployConfig)
	writeFindings("Infrastructure scans", inv.Infrastructure)
	writeFindings("Operational docs", inv.OperationalDocs)
	writeFindings("Evidence exports", inv.EvidenceExports)
	if len(inv.NativeCommands) > 0 {
		fmt.Fprintf(&b, "## Native commands\n\n")
		for _, c := range inv.NativeCommands {
			fmt.Fprintf(&b, "- `%s` — %s\n", c.Command, c.Reason)
		}
		fmt.Fprintf(&b, "\n")
	}
	if len(inv.NextCommands) > 0 {
		fmt.Fprintf(&b, "## Next commands\n\n")
		for _, c := range inv.NextCommands {
			fmt.Fprintf(&b, "- `%s` — %s\n", c.Command, c.Reason)
		}
	}
	return b.String()
}

func renderProjectMap(inv Inventory) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Patchline project map\n\n")
	fmt.Fprintf(&b, "This map shows where data-change evidence lives in the scanned project slice.\n\n")
	fmt.Fprintf(&b, "| area | count |\n| --- | ---: |\n")
	fmt.Fprintf(&b, "| files | %d |\n", inv.FilesScanned)
	fmt.Fprintf(&b, "| facts | %d |\n", len(inv.Facts))
	fmt.Fprintf(&b, "| languages | %d |\n", len(inv.Languages))
	fmt.Fprintf(&b, "| migration roots | %d |\n", len(inv.MigrationRoots))
	fmt.Fprintf(&b, "| migration systems | %d |\n", len(inv.MigrationSystems))
	fmt.Fprintf(&b, "| native commands | %d |\n", len(inv.NativeCommands))
	fmt.Fprintf(&b, "| schema evolution | %d |\n", len(inv.SchemaEvolution))
	fmt.Fprintf(&b, "| source SQL hints | %d |\n", len(inv.SourceSQLHints))
	fmt.Fprintf(&b, "| operational docs | %d |\n", len(inv.OperationalDocs))
	fmt.Fprintf(&b, "| field evidence | %d |\n", len(inv.FieldEvidence))
	fmt.Fprintf(&b, "| infrastructure scans | %d |\n", len(inv.Infrastructure))
	fmt.Fprintf(&b, "| evidence exports | %d |\n\n", len(inv.EvidenceExports))
	writePaths := func(title string, findings []Finding) {
		if len(findings) == 0 {
			return
		}
		fmt.Fprintf(&b, "## %s\n\n", title)
		for _, finding := range findings {
			fmt.Fprintf(&b, "- `%s` (%s) — %s\n", finding.Path, finding.Kind, finding.Rationale)
		}
		fmt.Fprintf(&b, "\n")
	}
	writePaths("Migration roots", inv.MigrationRoots)
	writePaths("Migration systems", inv.MigrationSystems)
	writePaths("Schema evolution", inv.SchemaEvolution)
	writePaths("Field evidence", inv.FieldEvidence)
	writePaths("Infrastructure scans", inv.Infrastructure)
	writePaths("Source SQL candidates", inv.SourceSQLHints)
	writePaths("Operational docs", inv.OperationalDocs)
	writePaths("Evidence exports", inv.EvidenceExports)
	if len(inv.NativeCommands) > 0 {
		fmt.Fprintf(&b, "## Native commands\n\n")
		for _, command := range inv.NativeCommands {
			fmt.Fprintf(&b, "- `%s` — %s\n", command.Command, command.Reason)
		}
		fmt.Fprintf(&b, "\n")
	}
	if len(inv.NextCommands) > 0 {
		fmt.Fprintf(&b, "## Next commands\n\n")
		for _, command := range inv.NextCommands {
			fmt.Fprintf(&b, "- `%s` — %s\n", command.Command, command.Reason)
		}
		fmt.Fprintf(&b, "\n")
	}
	fmt.Fprintf(&b, "The complete low-level fact stream is in `facts.jsonl`.\n")
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
	case ".prisma":
		return "Prisma"
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

func toolVersion() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "patchline@(unknown)"
	}
	version := info.Main.Version
	if version == "" {
		version = "(unknown)"
	}
	return info.Main.Path + "@" + version
}

func isFullSHA(value string) bool {
	if len(value) != 40 {
		return false
	}
	for _, r := range value {
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')) {
			return false
		}
	}
	return true
}

func detectLocalVCS(root string) (string, string) {
	if fi, err := os.Stat(filepath.Join(root, ".git")); err == nil && fi.IsDir() {
		return "git", localGitCommit(root)
	}
	if _, err := os.Stat(filepath.Join(root, ".hg")); err == nil {
		return "mercurial", localHgRevision(root)
	}
	for _, marker := range []string{"_FOSSIL_", ".fslckout", ".fos"} {
		if _, err := os.Stat(filepath.Join(root, marker)); err == nil {
			return "fossil", localFossilRevision(root)
		}
	}
	return "", ""
}

func localHgRevision(root string) string {
	if path, err := exec.LookPath("hg"); err == nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if out, err := exec.CommandContext(ctx, path, "-R", root, "id", "-i", "--debug").Output(); err == nil {
			rev := strings.TrimSuffix(strings.TrimSpace(string(out)), "+")
			if len(rev) >= 12 {
				return strings.ToLower(rev)
			}
		}
	}
	// Fall back to the dirstate parent hash recorded by Mercurial itself.
	data, err := os.ReadFile(filepath.Join(root, ".hg", "dirstate"))
	if err == nil && len(data) >= 20 {
		return hex.EncodeToString(data[:20])
	}
	if data, err := os.ReadFile(filepath.Join(root, ".hg", "cache", "branch2")); err == nil {
		fields := strings.Fields(string(data))
		if len(fields) > 0 && len(fields[0]) >= 12 {
			return strings.ToLower(fields[0])
		}
	}
	return ""
}

func localFossilRevision(root string) string {
	if path, err := exec.LookPath("fossil"); err == nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if out, err := exec.CommandContext(ctx, path, "info").Output(); err == nil {
			for _, line := range strings.Split(string(out), "\n") {
				if strings.HasPrefix(line, "checkout:") {
					fields := strings.Fields(strings.TrimPrefix(line, "checkout:"))
					if len(fields) > 0 && len(fields[0]) >= 12 {
						return strings.ToLower(fields[0])
					}
				}
			}
		}
	}
	for _, marker := range []string{".fslckout", "_FOSSIL_"} {
		if data, err := os.ReadFile(filepath.Join(root, marker)); err == nil {
			sum := sha256.Sum256(data)
			return hex.EncodeToString(sum[:])[:40]
		}
	}
	return ""
}

// hashTree computes a deterministic content hash over a directory tree, ignoring
// VCS metadata directories so the same working tree hashes identically regardless
// of which VCS produced it.
func hashTree(root string) (string, error) {
	type entry struct {
		rel  string
		hash string
	}
	var entries []entry
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", ".hg", ".fossil-settings":
				return filepath.SkipDir
			}
			return nil
		}
		switch d.Name() {
		case "_FOSSIL_", ".fslckout":
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		h, err := hashFile(path)
		if err != nil {
			return err
		}
		entries = append(entries, entry{rel: filepath.ToSlash(rel), hash: h})
		return nil
	})
	if err != nil {
		return "", err
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].rel < entries[j].rel })
	hasher := sha256.New()
	for _, e := range entries {
		fmt.Fprintf(hasher, "%s\x00%s\n", e.rel, e.hash)
	}
	return "sha256:" + hex.EncodeToString(hasher.Sum(nil)), nil
}

func localGitCommit(root string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", "-C", root, "rev-parse", "HEAD")
	output, err := cmd.Output()
	if err != nil {
		return ""
	}
	commit := strings.TrimSpace(string(output))
	if !isFullSHA(commit) {
		return ""
	}
	return strings.ToLower(commit)
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
