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
	SourceSQLHints    []Finding      `json:"source_sql_hints,omitempty"`
	OperationalDocs   []Finding      `json:"operational_docs,omitempty"`
	EvidenceExports   []Finding      `json:"evidence_exports,omitempty"`
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
	case isHTTPArchive(input):
		source.Mode = "archive-url"
		root, archiveHash, cache, err := fetchArchiveURL(ctx, input, outDir, downloadDir)
		if err != nil {
			return FetchResult{}, err
		}
		applyCacheResult(&source, cache)
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
		source.ResolvedCommit = localGitCommit(root)
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
	inv.finalize()
	return inv, nil
}

func (inv *Inventory) inspectRoot(root string) {
	lower := strings.ToLower(filepath.ToSlash(root))
	switch {
	case strings.HasSuffix(lower, "/db/migrate") || strings.Contains(lower, "/db/migrate/"):
		finding := Finding{Kind: "rails-or-generic-db-migrate", Path: ".", Confidence: "path", Rationale: "inventory root is a db/migrate migration path"}
		inv.MigrationRoots = append(inv.MigrationRoots, finding)
		inv.addFindingFact("migration_root", finding)
	case strings.HasSuffix(lower, "/migrations") || strings.Contains(lower, "/migrations/"):
		finding := Finding{Kind: "migrations", Path: ".", Confidence: "path", Rationale: "inventory root is a migrations path"}
		inv.MigrationRoots = append(inv.MigrationRoots, finding)
		inv.addFindingFact("migration_root", finding)
	case strings.HasSuffix(lower, "/migration") || strings.Contains(lower, "/migration/"):
		finding := Finding{Kind: "migration", Path: ".", Confidence: "path", Rationale: "inventory root is a migration path"}
		inv.MigrationRoots = append(inv.MigrationRoots, finding)
		inv.addFindingFact("migration_root", finding)
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

func fetchGitHubArchive(ctx context.Context, owner, repo, ref, resolvedCommit, outDir, downloadDir string) (string, string, string, downloadCacheResult, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/tarball/%s", owner, repo, resolvedCommit)
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
	case base == "manage.py":
		add("framework", &inv.Frameworks, "django", "Django manage.py entrypoint")
		inv.TestCommands = append(inv.TestCommands, Command{Command: "python manage.py test", Reason: "Django project test command"})
	case base == "package.json":
		add("framework", &inv.Frameworks, "node", "Node package manifest")
		inv.TestCommands = append(inv.TestCommands, Command{Command: "npm test", Reason: "package.json suggests npm tests may be available"})
	case base == "go.mod":
		add("framework", &inv.Frameworks, "go", "Go module")
		inv.TestCommands = append(inv.TestCommands, Command{Command: "go test ./...", Reason: "Go module test command"})
	case base == "pyproject.toml":
		add("framework", &inv.Frameworks, "python", "Python project metadata")
		inv.TestCommands = append(inv.TestCommands, Command{Command: "pytest", Reason: "Python project metadata suggests pytest may be available"})
	}
	switch {
	case strings.Contains(lower, "db/migrate/"):
		add("migration_root", &inv.MigrationRoots, "rails-or-generic-db-migrate", "db/migrate migration path")
	case strings.Contains(lower, "migrations/"):
		add("migration_root", &inv.MigrationRoots, "migrations", "migrations path")
	case strings.Contains(lower, "prisma/migrations/"):
		add("migration_system", &inv.MigrationSystems, "prisma", "Prisma migrations path")
	case base == "alembic.ini":
		add("migration_system", &inv.MigrationSystems, "alembic", "Alembic config")
	case base == "flyway.conf":
		add("migration_system", &inv.MigrationSystems, "flyway", "Flyway config")
	case base == "liquibase.properties":
		add("migration_system", &inv.MigrationSystems, "liquibase", "Liquibase config")
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
)

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
	case "column", "model", "queue", "job", "report", "deploy", "error":
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
	inv.OperationalDocs = capFindings(uniqueFindings(inv.OperationalDocs), 50)
	inv.EvidenceExports = capFindings(uniqueFindings(inv.EvidenceExports), 50)
	inv.TestCommands = uniqueCommands(inv.TestCommands)
	inv.NextCommands = append(inv.NextCommands, Command{Command: fmt.Sprintf("patchline intake %s --out results/generated/intake", shellPath(inv.Root)), Reason: "run deterministic data/code repair intake on this project"})
	if len(inv.TestCommands) > 0 {
		inv.NextCommands = append(inv.NextCommands, inv.TestCommands...)
	}
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
	for _, command := range inv.NextCommands {
		inv.addCommandFact("next_command", command)
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
	fmt.Fprintf(&b, "| source SQL hints | %d |\n", len(inv.SourceSQLHints))
	fmt.Fprintf(&b, "| operational docs | %d |\n", len(inv.OperationalDocs))
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
	writePaths("Source SQL candidates", inv.SourceSQLHints)
	writePaths("Operational docs", inv.OperationalDocs)
	writePaths("Evidence exports", inv.EvidenceExports)
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
