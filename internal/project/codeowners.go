package project

import (
	"bufio"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

type codeownersRule struct {
	Pattern string
	Owners  []string
	Source  string
	Line    int
	re      *regexp.Regexp
}

type codeownersFile struct {
	Source string
	Root   string
	Rules  []codeownersRule
}

func loadCodeowners(root string) (codeownersFile, bool) {
	for _, dir := range codeownersSearchDirs(root) {
		for _, name := range []string{"CODEOWNERS", ".github/CODEOWNERS", "docs/CODEOWNERS"} {
			candidate := filepath.Join(dir, filepath.FromSlash(name))
			rules, err := parseCodeownersFile(candidate, dir)
			if err == nil && len(rules) > 0 {
				return codeownersFile{Source: filepath.ToSlash(candidate), Root: dir, Rules: rules}, true
			}
		}
	}
	return codeownersFile{}, false
}

func codeownersSearchDirs(root string) []string {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil
	}
	info, err := os.Stat(abs)
	if err != nil {
		return nil
	}
	if !info.IsDir() {
		abs = filepath.Dir(abs)
	}
	var dirs []string
	seen := map[string]bool{}
	for current := abs; current != "" && current != string(filepath.Separator); current = filepath.Dir(current) {
		if !seen[current] {
			dirs = append(dirs, current)
			seen[current] = true
		}
		if _, err := os.Stat(filepath.Join(current, ".git")); err == nil {
			break
		}
		parent := filepath.Dir(current)
		if parent == current {
			break
		}
	}
	return dirs
}

func parseCodeownersFile(filePath, repoRoot string) ([]codeownersRule, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	var rules []codeownersRule
	scanner := bufio.NewScanner(file)
	line := 0
	for scanner.Scan() {
		line++
		text := strings.TrimSpace(stripCodeownersComment(scanner.Text()))
		if text == "" {
			continue
		}
		fields := strings.Fields(text)
		if len(fields) < 2 || strings.HasPrefix(fields[0], "!") || strings.Contains(fields[0], "[") {
			continue
		}
		re, err := codeownersPatternRegexp(fields[0])
		if err != nil {
			continue
		}
		rules = append(rules, codeownersRule{
			Pattern: fields[0],
			Owners:  uniqueSortedStrings(fields[1:]),
			Source:  filepath.ToSlash(filePath),
			Line:    line,
			re:      re,
		})
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	_ = repoRoot
	return rules, nil
}

func stripCodeownersComment(line string) string {
	if strings.HasPrefix(strings.TrimSpace(line), "#") {
		return ""
	}
	return line
}

func codeownersPatternRegexp(pattern string) (*regexp.Regexp, error) {
	raw := filepath.ToSlash(strings.TrimSpace(pattern))
	raw = strings.TrimPrefix(raw, "\\#")
	raw = strings.TrimPrefix(raw, "\\!")
	if raw == "" {
		return nil, fmt.Errorf("empty CODEOWNERS pattern")
	}
	anchored := strings.HasPrefix(raw, "/")
	raw = strings.TrimPrefix(raw, "/")
	dirOnly := strings.HasSuffix(raw, "/")
	raw = strings.TrimSuffix(raw, "/")
	if raw == "" {
		return nil, fmt.Errorf("empty CODEOWNERS pattern")
	}
	body := codeownersGlobRegexp(raw)
	var expr string
	switch {
	case dirOnly && anchored:
		expr = "^" + body + "/.*$"
	case dirOnly:
		expr = "(^|.*/)" + body + "/.*$"
	case anchored:
		expr = "^" + body + "$"
	case strings.Contains(raw, "/"):
		expr = "(^|.*/)" + body + "$"
	default:
		expr = "(^|.*/)" + body + "$"
	}
	return regexp.Compile(expr)
}

func codeownersGlobRegexp(glob string) string {
	var b strings.Builder
	for i := 0; i < len(glob); i++ {
		ch := glob[i]
		if ch == '*' {
			if i+1 < len(glob) && glob[i+1] == '*' {
				b.WriteString(".*")
				i++
			} else {
				b.WriteString("[^/]*")
			}
			continue
		}
		if ch == '?' {
			b.WriteString("[^/]")
			continue
		}
		b.WriteString(regexp.QuoteMeta(string(ch)))
	}
	return b.String()
}

func (file codeownersFile) match(rel string) (codeownersRule, bool) {
	rel = filepath.ToSlash(strings.TrimPrefix(rel, "/"))
	var matched codeownersRule
	ok := false
	for _, rule := range file.Rules {
		if rule.re != nil && rule.re.MatchString(rel) {
			matched = rule
			ok = true
		}
	}
	return matched, ok
}

func buildOwnerRoutes(root string, risks []BaselineRisk) []OwnerRoute {
	codeowners, ok := loadCodeowners(root)
	if !ok {
		return nil
	}
	var routes []OwnerRoute
	seen := map[string]bool{}
	for _, risk := range risks {
		route, ok := ownerRouteForPath(codeowners, root, "risk", risk.ID, risk.Path)
		if !ok {
			continue
		}
		key := route.SubjectKind + "\x00" + route.SubjectID + "\x00" + strings.Join(route.Owners, ",")
		if seen[key] {
			continue
		}
		seen[key] = true
		routes = append(routes, route)
	}
	sortOwnerRoutes(routes)
	return routes
}

func ownerRouteForPath(codeowners codeownersFile, scanRoot, subjectKind, subjectID, rel string) (OwnerRoute, bool) {
	codeownersRel := codeownersRelativePath(codeowners.Root, scanRoot, rel)
	rule, ok := codeowners.match(codeownersRel)
	if !ok || len(rule.Owners) == 0 {
		return OwnerRoute{}, false
	}
	return OwnerRoute{
		SubjectKind: subjectKind,
		SubjectID:   subjectID,
		Path:        filepath.ToSlash(rel),
		Owners:      append([]string(nil), rule.Owners...),
		Pattern:     rule.Pattern,
		Source:      rule.Source,
		Confidence:  "codeowners",
		Rationale:   "last matching CODEOWNERS rule identifies likely reviewers for this path",
	}, true
}

func codeownersRelativePath(codeownersRoot, scanRoot, rel string) string {
	if filepath.IsAbs(rel) {
		if out, err := filepath.Rel(codeownersRoot, rel); err == nil {
			return filepath.ToSlash(out)
		}
		return filepath.ToSlash(rel)
	}
	abs := filepath.Join(scanRoot, filepath.FromSlash(rel))
	if out, err := filepath.Rel(codeownersRoot, abs); err == nil && !strings.HasPrefix(out, "..") {
		return filepath.ToSlash(out)
	}
	return filepath.ToSlash(rel)
}

func routeOwnersByRisk(routes []OwnerRoute) map[string][]string {
	out := map[string][]string{}
	for _, route := range routes {
		if route.SubjectKind == "risk" && route.SubjectID != "" {
			out[route.SubjectID] = mergeStrings(out[route.SubjectID], route.Owners)
		}
	}
	return out
}

func ownersForRiskIDs(routes []OwnerRoute, riskIDs []string) []string {
	byRisk := routeOwnersByRisk(routes)
	var owners []string
	for _, riskID := range riskIDs {
		owners = mergeStrings(owners, byRisk[riskID])
	}
	return owners
}

func countOwnerRouteOwners(routes []OwnerRoute) int {
	owners := map[string]bool{}
	for _, route := range routes {
		for _, owner := range route.Owners {
			if owner != "" {
				owners[owner] = true
			}
		}
	}
	return len(owners)
}

func ownerRoutesForGeneratedFiles(baseline BaselineReport, files []GeneratedFile) []OwnerRoute {
	var routes []OwnerRoute
	seen := map[string]bool{}
	for _, file := range files {
		owners := ownersForRiskIDs(baseline.OwnerRoutes, file.RiskIDs)
		if len(owners) == 0 {
			continue
		}
		route := OwnerRoute{
			SubjectKind: "generated_file",
			SubjectID:   file.Path,
			Path:        file.Path,
			Owners:      owners,
			Confidence:  "risk-codeowners",
			Rationale:   "generated intervention inherits likely reviewers from targeted risk paths",
		}
		key := route.SubjectKind + "\x00" + route.SubjectID
		if seen[key] {
			continue
		}
		seen[key] = true
		routes = append(routes, route)
	}
	sortOwnerRoutes(routes)
	return routes
}

func mergeStrings(existing []string, values []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, value := range existing {
		if value != "" && !seen[value] {
			seen[value] = true
			out = append(out, value)
		}
	}
	for _, value := range values {
		if value != "" && !seen[value] {
			seen[value] = true
			out = append(out, value)
		}
	}
	sort.Strings(out)
	return out
}

func sortOwnerRoutes(routes []OwnerRoute) {
	sort.Slice(routes, func(i, j int) bool {
		if routes[i].SubjectKind != routes[j].SubjectKind {
			return routes[i].SubjectKind < routes[j].SubjectKind
		}
		if routes[i].SubjectID != routes[j].SubjectID {
			return routes[i].SubjectID < routes[j].SubjectID
		}
		return path.Clean(routes[i].Path) < path.Clean(routes[j].Path)
	})
}
