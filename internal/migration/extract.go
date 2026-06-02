package migration

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/thehalleyyoung/patchline/internal/canonical"
)

const SourceSQLVersion = "patchline.source-sql/v1"

type SourceSQLReport struct {
	Version      string                 `json:"version"`
	Root         string                 `json:"root"`
	Observations []SourceSQLObservation `json:"observations"`
	Summary      SourceSQLSummary       `json:"summary"`
	Hash         string                 `json:"hash"`
}

type SourceSQLObservation struct {
	Path        string   `json:"path"`
	Language    string   `json:"language"`
	Detector    string   `json:"detector"`
	Line        int      `json:"line"`
	Kind        string   `json:"kind"`
	Framework   string   `json:"framework,omitempty"`
	Operation   string   `json:"operation,omitempty"`
	Table       string   `json:"table,omitempty"`
	SQL         string   `json:"sql,omitempty"`
	Fingerprint string   `json:"fingerprint,omitempty"`
	Risk        Risk     `json:"risk,omitempty"`
	Effect      string   `json:"effect,omitempty"`
	Reasons     []string `json:"reasons,omitempty"`
	Confidence  string   `json:"confidence"`
	SnippetHash string   `json:"snippet_hash"`
}

type SourceSQLSummary struct {
	FilesScanned int            `json:"files_scanned"`
	EmbeddedSQL  int            `json:"embedded_sql"`
	ORMQueries   int            `json:"orm_queries"`
	Frameworks   map[string]int `json:"frameworks,omitempty"`
	Languages    map[string]int `json:"languages,omitempty"`
	Tables       []string       `json:"tables,omitempty"`
}

type sourceLiteral struct {
	Text     string
	Line     int
	Form     string
	Context  string
	Language string
}

type sourceProfile struct {
	Language string
	Detector string
}

func ExtractSourceSQL(root string) (SourceSQLReport, error) {
	root = filepath.Clean(root)
	report := SourceSQLReport{
		Version: SourceSQLVersion,
		Root:    filepath.ToSlash(root),
		Summary: SourceSQLSummary{
			Frameworks: map[string]int{},
			Languages:  map[string]int{},
		},
	}
	var files []string
	info, err := os.Stat(root)
	if err != nil {
		return SourceSQLReport{}, err
	}
	if !info.IsDir() {
		files = append(files, root)
	} else {
		err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				if shouldSkipSourceDir(d.Name()) && path != root {
					return filepath.SkipDir
				}
				return nil
			}
			files = append(files, path)
			return nil
		})
		if err != nil {
			return SourceSQLReport{}, err
		}
	}
	sort.Strings(files)
	seen := map[string]bool{}
	tables := map[string]bool{}
	for _, path := range files {
		profile, ok := detectSourceProfile(path)
		if !ok {
			continue
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return SourceSQLReport{}, err
		}
		normalized := normalizeSourceContent(string(content))
		report.Summary.FilesScanned++
		report.Summary.Languages[profile.Language]++
		rel := filepath.ToSlash(path)
		if base, err := filepath.Rel(root, path); err == nil && base != "." && !strings.HasPrefix(base, "..") {
			rel = filepath.ToSlash(base)
		}
		for _, obs := range extractFileObservations(rel, normalized, profile) {
			key := fmt.Sprintf("%s:%d:%s:%s:%s:%s", obs.Path, obs.Line, obs.Kind, obs.Framework, obs.Operation, obs.SnippetHash)
			if seen[key] {
				continue
			}
			seen[key] = true
			report.Observations = append(report.Observations, obs)
			if obs.Framework != "" {
				report.Summary.Frameworks[obs.Framework]++
			}
			if obs.Table != "" {
				tables[obs.Table] = true
			}
			switch obs.Kind {
			case "embedded_sql":
				report.Summary.EmbeddedSQL++
			case "orm_query", "migration_framework":
				report.Summary.ORMQueries++
			}
		}
	}
	sort.Slice(report.Observations, func(i, j int) bool {
		a, b := report.Observations[i], report.Observations[j]
		if a.Path != b.Path {
			return a.Path < b.Path
		}
		if a.Line != b.Line {
			return a.Line < b.Line
		}
		if a.Kind != b.Kind {
			return a.Kind < b.Kind
		}
		return a.SnippetHash < b.SnippetHash
	})
	for table := range tables {
		report.Summary.Tables = append(report.Summary.Tables, table)
	}
	sort.Strings(report.Summary.Tables)
	if len(report.Summary.Frameworks) == 0 {
		report.Summary.Frameworks = nil
	}
	if len(report.Summary.Languages) == 0 {
		report.Summary.Languages = nil
	}
	report.Hash = canonical.Hash(struct {
		Version      string                 `json:"version"`
		Root         string                 `json:"root"`
		Observations []SourceSQLObservation `json:"observations"`
	}{report.Version, report.Root, report.Observations})
	return report, nil
}

func extractFileObservations(path, content string, profile sourceProfile) []SourceSQLObservation {
	if profile.Language == "sql" {
		return extractSQLFile(path, content, profile)
	}
	var observations []SourceSQLObservation
	literals := extractSourceLiterals(content, profile.Language)
	for _, literal := range literals {
		sql := normalizeExtractedSQL(literal.Text)
		if !looksLikeSQL(sql) {
			continue
		}
		obs := sqlObservation(path, profile, literal, sql)
		if framework := rawSQLFramework(literal.Context, path); framework != "" {
			obs.Framework = framework
		}
		observations = append(observations, obs)
	}
	observations = append(observations, extractORMObservations(path, content, profile)...)
	return observations
}

func extractSQLFile(path, content string, profile sourceProfile) []SourceSQLObservation {
	var observations []SourceSQLObservation
	for index, raw := range splitStatements(content) {
		sql := normalizeExtractedSQL(raw)
		if !looksLikeSQL(sql) {
			continue
		}
		literal := sourceLiteral{Text: sql, Line: lineForStatement(content, raw), Form: "sql-file", Context: raw, Language: profile.Language}
		obs := sqlObservation(path, profile, literal, sql)
		obs.Framework = migrationFrameworkFromPath(path)
		obs.Detector = fmt.Sprintf("%s.statement.%d", profile.Detector, index)
		observations = append(observations, obs)
	}
	return observations
}

func sqlObservation(path string, profile sourceProfile, literal sourceLiteral, sql string) SourceSQLObservation {
	report, err := AnalyzeBytesWithDialect(path, []byte(sql), DialectGeneric)
	statement := Statement{}
	if err == nil && len(report.Statements) > 0 {
		statement = report.Statements[0]
	}
	kind := statement.Kind
	if kind == "" {
		kind = firstToken(tokenize(sql))
	}
	confidence := "high"
	if strings.Contains(sql, "${") || strings.Contains(sql, "{") && literal.Form == "python-f" {
		confidence = "medium"
	}
	return SourceSQLObservation{
		Path:        path,
		Language:    profile.Language,
		Detector:    profile.Detector + "." + literal.Form,
		Line:        literal.Line,
		Kind:        "embedded_sql",
		Operation:   kind,
		Table:       statement.Table,
		SQL:         sql,
		Fingerprint: Fingerprint(sql),
		Risk:        statement.Risk,
		Effect:      statement.Effect,
		Reasons:     statement.Reasons,
		Confidence:  confidence,
		SnippetHash: canonical.Hash(normalizeSourceContent(sql)),
	}
}

func extractSourceLiterals(content, language string) []sourceLiteral {
	switch language {
	case "python":
		return extractPythonLiterals(content)
	case "javascript", "typescript":
		return extractCLikeLiterals(content, true, true, false)
	case "go":
		return extractCLikeLiterals(content, true, false, false)
	case "java", "csharp":
		return extractCLikeLiterals(content, false, false, true)
	case "ruby":
		return extractRubyLiterals(content)
	case "shell":
		return extractShellLiterals(content)
	default:
		return nil
	}
}

func extractCLikeLiterals(content string, backtick, template, textBlocks bool) []sourceLiteral {
	var out []sourceLiteral
	line := 1
	for i := 0; i < len(content); i++ {
		ch := content[i]
		if ch == '\n' {
			line++
			continue
		}
		if ch == '/' && i+1 < len(content) && content[i+1] == '/' {
			for i < len(content) && content[i] != '\n' {
				i++
			}
			i--
			continue
		}
		if ch == '/' && i+1 < len(content) && content[i+1] == '*' {
			i += 2
			for i+1 < len(content) && !(content[i] == '*' && content[i+1] == '/') {
				if content[i] == '\n' {
					line++
				}
				i++
			}
			i++
			continue
		}
		if textBlocks && ch == '"' && strings.HasPrefix(content[i:], `"""`) {
			startLine := line
			start := i + 3
			i = start
			for i+2 < len(content) && !strings.HasPrefix(content[i:], `"""`) {
				if content[i] == '\n' {
					line++
				}
				i++
			}
			out = append(out, sourceLiteral{Text: content[start:i], Line: startLine, Form: "text-block", Context: contextAround(content, start), Language: "java"})
			i += 2
			continue
		}
		if ch == '\'' || ch == '"' || (backtick && ch == '`') {
			quote := ch
			startLine := line
			start := i + 1
			form := "string"
			if quote == '`' {
				form = "raw-string"
				if template {
					form = "template"
				}
			}
			i++
			for i < len(content) {
				if content[i] == '\n' {
					line++
				}
				if quote != '`' && content[i] == '\\' {
					i += 2
					continue
				}
				if content[i] == quote {
					break
				}
				i++
			}
			if i <= len(content) {
				out = append(out, sourceLiteral{Text: content[start:i], Line: startLine, Form: form, Context: contextAround(content, start), Language: ""})
			}
		}
	}
	return out
}

func extractPythonLiterals(content string) []sourceLiteral {
	var out []sourceLiteral
	line := 1
	for i := 0; i < len(content); i++ {
		ch := content[i]
		if ch == '\n' {
			line++
			continue
		}
		if ch == '#' {
			for i < len(content) && content[i] != '\n' {
				i++
			}
			i--
			continue
		}
		prefixStart := i
		for i < len(content) && strings.ContainsRune("rRuUbBfF", rune(content[i])) {
			i++
		}
		if i >= len(content) || (content[i] != '\'' && content[i] != '"') {
			i = prefixStart
			continue
		}
		quote := content[i]
		prefix := strings.ToLower(content[prefixStart:i])
		triple := i+2 < len(content) && content[i+1] == quote && content[i+2] == quote
		startLine := line
		form := "string"
		if strings.Contains(prefix, "f") {
			form = "python-f"
		}
		if triple {
			form += "-triple"
			i += 3
			start := i
			for i+2 < len(content) && !(content[i] == quote && content[i+1] == quote && content[i+2] == quote) {
				if content[i] == '\n' {
					line++
				}
				i++
			}
			out = append(out, sourceLiteral{Text: content[start:i], Line: startLine, Form: form, Context: contextAround(content, start), Language: "python"})
			i += 2
			continue
		}
		i++
		start := i
		for i < len(content) {
			if content[i] == '\n' {
				line++
			}
			if content[i] == '\\' {
				i += 2
				continue
			}
			if content[i] == quote {
				break
			}
			i++
		}
		out = append(out, sourceLiteral{Text: content[start:i], Line: startLine, Form: form, Context: contextAround(content, start), Language: "python"})
	}
	return out
}

func extractRubyLiterals(content string) []sourceLiteral {
	out := extractCLikeLiterals(content, false, false, false)
	lines := strings.Split(content, "\n")
	for i := 0; i < len(lines); i++ {
		line := lines[i]
		matches := rubyHeredoc.FindStringSubmatch(line)
		if len(matches) != 2 {
			continue
		}
		tag := matches[1]
		var body []string
		startLine := i + 1
		for j := i + 1; j < len(lines); j++ {
			if strings.TrimSpace(lines[j]) == tag {
				i = j
				break
			}
			body = append(body, lines[j])
		}
		out = append(out, sourceLiteral{Text: strings.Join(body, "\n"), Line: startLine, Form: "heredoc", Context: line, Language: "ruby"})
	}
	return out
}

func extractShellLiterals(content string) []sourceLiteral {
	out := extractCLikeLiterals(content, false, false, false)
	lines := strings.Split(content, "\n")
	for i := 0; i < len(lines); i++ {
		line := lines[i]
		matches := shellHeredoc.FindStringSubmatch(line)
		if len(matches) != 2 {
			continue
		}
		tag := strings.Trim(matches[1], `'"`)
		var body []string
		startLine := i + 2
		for j := i + 1; j < len(lines); j++ {
			if strings.TrimSpace(lines[j]) == tag {
				i = j
				break
			}
			body = append(body, lines[j])
		}
		out = append(out, sourceLiteral{Text: strings.Join(body, "\n"), Line: startLine, Form: "heredoc", Context: line, Language: "shell"})
	}
	return out
}

func extractORMObservations(path, content string, profile sourceProfile) []SourceSQLObservation {
	var out []SourceSQLObservation
	windows := sourceWindows(content, 8)
	for _, window := range windows {
		for _, obs := range detectORMWindow(path, profile, window.line, window.text) {
			out = append(out, obs)
		}
	}
	return out
}

type sourceWindow struct {
	line int
	text string
}

func sourceWindows(content string, maxLines int) []sourceWindow {
	lines := strings.Split(content, "\n")
	var windows []sourceWindow
	for i := range lines {
		if !mayContainORMLine(lines[i]) {
			continue
		}
		var parts []string
		balance := 0
		for j := i; j < len(lines) && j < i+maxLines; j++ {
			line := stripSourceLineComment(lines[j])
			parts = append(parts, strings.TrimSpace(line))
			balance += strings.Count(line, "(") + strings.Count(line, "{") + strings.Count(line, "[")
			balance -= strings.Count(line, ")") + strings.Count(line, "}") + strings.Count(line, "]")
			if balance <= 0 {
				break
			}
		}
		text := strings.Join(parts, " ")
		if strings.TrimSpace(text) != "" {
			windows = append(windows, sourceWindow{line: i + 1, text: text})
		}
	}
	return windows
}

func mayContainORMLine(line string) bool {
	lower := strings.ToLower(line)
	for _, marker := range []string{
		".objects.", "prisma.", "getrepository(", ".createquerybuilder", "sequelize", "knex", "querybuilder(",
		".where(", ".update(", ".destroy(", ".findall", ".findone", ".find_by", ".update_all", ".delete_all", ".destroy_all", "create_table", "drop_table",
		"add_column", "remove_column", "op.", "context.", "db.", "db.session", "session.query", "session.add", "session.merge", "session.delete", "sqlalchemy", "entitymanager", "@query", "@modifying",
		"createquery(", "createquerybuilder(", ".executeupdate", ".model(", ".table(", ".updates(", ".save(", ".delete(", ".create(",
	} {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}

func detectORMWindow(path string, profile sourceProfile, line int, text string) []SourceSQLObservation {
	var out []SourceSQLObservation
	for _, detector := range ormDetectors {
		for _, match := range detector.Pattern.FindAllStringSubmatch(text, -1) {
			table, operation := detector.Table(match), detector.Operation(match, strings.ToLower(text))
			if table == "" || operation == "" {
				continue
			}
			kind := "orm_query"
			if detector.Migration {
				kind = "migration_framework"
			}
			obs := SourceSQLObservation{
				Path:        path,
				Language:    profile.Language,
				Detector:    profile.Detector + "." + detector.Name,
				Line:        line,
				Kind:        kind,
				Framework:   detector.Framework,
				Operation:   operation,
				Table:       cleanIdentifier(table),
				Confidence:  detector.Confidence,
				SnippetHash: canonical.Hash(normalizeSourceContent(text)),
			}
			enrichORMObservation(&obs, text)
			out = append(out, obs)
		}
	}
	return out
}

func enrichORMObservation(obs *SourceSQLObservation, text string) {
	operation := strings.ToLower(obs.Operation)
	switch operation {
	case "select":
		obs.Risk = RiskLow
		obs.Effect = "noop"
	case "delete", "destroy", "destroy_all", "delete_all", "drop_table", "remove_column", "drop_column":
		obs.Risk = RiskHigh
		obs.Effect = "destructive"
		obs.Reasons = append(obs.Reasons, "orm write effect can remove persistent data")
	case "update", "update_all", "update_many", "updatemany", "save", "merge":
		obs.Risk = RiskMedium
		obs.Effect = "unknown"
		obs.Reasons = append(obs.Reasons, "orm write effect mutates existing persistent records")
	case "insert", "create", "bulk_create", "bulkcreate", "bulk_insert", "upsert", "update_or_create", "persist":
		obs.Risk = RiskMedium
		obs.Effect = "unknown"
		obs.Reasons = append(obs.Reasons, "orm write effect creates persistent records")
	default:
		obs.Risk = RiskMedium
		obs.Effect = "unknown"
	}
	if isORMWriteOperation(operation) && !hasORMScopedWriteMarker(strings.ToLower(text)) {
		obs.Risk = RiskHigh
		obs.Reasons = append(obs.Reasons, "orm write lacks an obvious scope marker in nearby source")
	}
	if obs.Framework != "" && isORMWriteOperation(operation) {
		obs.Reasons = append(obs.Reasons, "framework="+obs.Framework)
	}
	sort.Strings(obs.Reasons)
}

func isORMWriteOperation(operation string) bool {
	switch strings.ToLower(operation) {
	case "update", "delete", "insert", "save", "merge", "persist", "upsert", "update_or_create", "bulk_create", "bulk_update":
		return true
	default:
		return false
	}
}

func hasORMScopedWriteMarker(lower string) bool {
	for _, marker := range []string{".where", "where(", ".filter", "filter(", "find_by", "find(", "findone", "findunique", "limit(", " id", "_id", ".id", "primary_key"} {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}

type ormDetector struct {
	Name       string
	Framework  string
	Pattern    *regexp.Regexp
	Confidence string
	Migration  bool
	Table      func([]string) string
	Operation  func([]string, string) string
}

var ormDetectors = []ormDetector{
	{"rails-active-record", "rails", regexp.MustCompile(`\b([A-Z][A-Za-z0-9_]*)\.(where|find_by|update|update_all|delete|delete_all|destroy|destroy_all|create|insert_all|upsert_all|find_by_sql|exec_query)\b`), "medium", false, func(m []string) string { return modelToTable(m[1]) }, func(m []string, text string) string {
		if strings.Contains(text, ".update_all") {
			return "update"
		}
		if strings.Contains(text, ".delete_all") || strings.Contains(text, ".destroy_all") {
			return "delete"
		}
		return ormOperation(m[2])
	}},
	{"rails-migration", "rails", regexp.MustCompile(`\b(create_table|drop_table|add_column|remove_column)\s+[:'"]([A-Za-z_][A-Za-z0-9_]*)`), "medium", true, func(m []string) string { return m[2] }, func(m []string, _ string) string { return m[1] }},
	{"django-queryset", "django", regexp.MustCompile(`\b([A-Za-z_][A-Za-z0-9_]*)\.objects\.(filter|get|all|create|bulk_create|bulk_update|update_or_create|raw)\b`), "medium", false, func(m []string) string { return modelToTable(m[1]) }, func(m []string, text string) string {
		if strings.Contains(text, ".update(") {
			return "update"
		}
		if strings.Contains(text, ".delete(") {
			return "delete"
		}
		return ormOperation(m[2])
	}},
	{"alembic", "alembic", regexp.MustCompile(`\bop\.(create_table|drop_table|add_column|drop_column|execute)\s*\(\s*['"]([A-Za-z_][A-Za-z0-9_]*)?`), "medium", true, func(m []string) string {
		if len(m) > 1 && m[1] == "execute" {
			return ""
		}
		return m[2]
	}, func(m []string, _ string) string { return m[1] }},
	{"prisma-client", "prisma", regexp.MustCompile(`\bprisma\.([A-Za-z_][A-Za-z0-9_]*)\.(findMany|findUnique|findFirst|create|update|updateMany|delete|deleteMany|upsert)\b`), "medium", false, func(m []string) string { return modelToTable(m[1]) }, func(m []string, _ string) string { return ormOperation(m[2]) }},
	{"typeorm", "typeorm", regexp.MustCompile(`\b(?:getRepository\((?:['"])?([A-Za-z_][A-Za-z0-9_]*)(?:['"])?\)|repository)\.(find|findOne|update|delete|save|createQueryBuilder)\b`), "low", false, func(m []string) string { return modelToTable(m[1]) }, func(m []string, _ string) string { return ormOperation(m[2]) }},
	{"sqlalchemy-query", "sqlalchemy", regexp.MustCompile(`\b(?:db\.)?session\.query\(\s*([A-Z][A-Za-z0-9_]*)\s*\).*?\.(update|delete)\s*\(`), "medium", false, func(m []string) string { return modelToTable(m[1]) }, func(m []string, _ string) string { return ormOperation(m[2]) }},
	{"sqlalchemy-session", "sqlalchemy", regexp.MustCompile(`\b(?:db\.)?session\.(add|delete|merge|bulk_insert_mappings|bulk_update_mappings)\s*\(\s*([A-Za-z_][A-Za-z0-9_]*)?`), "medium", false, func(m []string) string { return modelToTable(m[2]) }, func(m []string, _ string) string { return ormOperation(m[1]) }},
	{"hibernate-entity-manager", "hibernate", regexp.MustCompile(`\b(?:entityManager|session)\.(persist|merge|remove|save|update|delete)\s*\(\s*([A-Za-z_][A-Za-z0-9_]*)`), "medium", false, func(m []string) string { return modelToTable(m[2]) }, func(m []string, _ string) string { return ormOperation(m[1]) }},
	{"hibernate-query", "hibernate", regexp.MustCompile(`(?i)@Query\s*\(\s*(?:value\s*=\s*)?["']\s*(update|delete)\s+([A-Za-z_][A-Za-z0-9_]*)`), "medium", false, func(m []string) string { return modelToTable(m[2]) }, func(m []string, _ string) string { return ormOperation(m[1]) }},
	{"gorm-chain", "gorm", regexp.MustCompile(`\b(?:db|tx|DB)\.(?:Model\(\s*&?([A-Za-z_][A-Za-z0-9_]*)(?:\{\})?\s*\)|Table\(\s*["']([A-Za-z_][A-Za-z0-9_]*)["']\s*\)).*?\.(Create|Save|Updates?|Delete)\b`), "medium", false, func(m []string) string {
		if m[2] != "" {
			return m[2]
		}
		return modelToTable(m[1])
	}, func(m []string, _ string) string { return ormOperation(m[3]) }},
	{"gorm-direct", "gorm", regexp.MustCompile(`\b(?:db|tx|DB)\.(Create|Save|Updates?|Delete)\s*\(\s*&?([A-Za-z_][A-Za-z0-9_]*)`), "medium", false, func(m []string) string { return modelToTable(m[2]) }, func(m []string, _ string) string { return ormOperation(m[1]) }},
	{"sequelize", "sequelize", regexp.MustCompile(`\b([A-Z][A-Za-z0-9_]*)\.(findAll|findOne|update|destroy|create|bulkCreate)\b`), "medium", false, func(m []string) string { return modelToTable(m[1]) }, func(m []string, _ string) string { return ormOperation(m[2]) }},
	{"knex", "knex", regexp.MustCompile(`\bknex(?:\.schema)?\.(createTable|dropTable|table)\s*\(\s*['"]([A-Za-z_][A-Za-z0-9_]*)|knex\s*\(\s*['"]([A-Za-z_][A-Za-z0-9_]*)`), "medium", false, func(m []string) string {
		if len(m) > 3 && m[3] != "" {
			return m[3]
		}
		return m[2]
	}, func(m []string, text string) string {
		if len(m) > 1 && m[1] != "" {
			return m[1]
		}
		if strings.Contains(text, ".update(") {
			return "update"
		}
		if strings.Contains(text, ".delete(") || strings.Contains(text, ".del(") {
			return "delete"
		}
		if strings.Contains(text, ".insert(") {
			return "insert"
		}
		return "select"
	}},
	{"entity-framework", "entity-framework", regexp.MustCompile(`\b(?:context|db)\.([A-Z][A-Za-z0-9_]*)\.(Where|Add|Update|Remove|FromSqlRaw|ExecuteSqlRaw)\b`), "medium", false, func(m []string) string {
		if strings.EqualFold(m[1], "database") {
			return ""
		}
		return modelToTable(m[1])
	}, func(m []string, _ string) string { return ormOperation(m[2]) }},
	{"generic-query-builder", "query-builder", regexp.MustCompile(`\.(?:from|table)\s*\(\s*['"]([A-Za-z_][A-Za-z0-9_]*)['"]|queryBuilder\s*\(\s*['"]([A-Za-z_][A-Za-z0-9_]*)['"]`), "low", false, func(m []string) string {
		if m[1] != "" {
			return m[1]
		}
		return m[2]
	}, func(_ []string, text string) string {
		if strings.Contains(text, ".update(") {
			return "update"
		}
		if strings.Contains(text, ".delete(") {
			return "delete"
		}
		if strings.Contains(text, ".insert(") {
			return "insert"
		}
		return "select"
	}},
}

var (
	rubyHeredoc  = regexp.MustCompile(`<<[-~]?\s*['"]?([A-Za-z_][A-Za-z0-9_]*)['"]?`)
	shellHeredoc = regexp.MustCompile(`<<-?\s*(['"]?[A-Za-z_][A-Za-z0-9_]*['"]?)`)
)

func detectSourceProfile(path string) (sourceProfile, bool) {
	ext := strings.ToLower(filepath.Ext(path))
	base := strings.ToLower(filepath.Base(path))
	switch ext {
	case ".go":
		return sourceProfile{"go", "extension.go"}, true
	case ".py":
		return sourceProfile{"python", "extension.py"}, true
	case ".ts", ".tsx":
		return sourceProfile{"typescript", "extension.ts"}, true
	case ".js", ".jsx":
		return sourceProfile{"javascript", "extension.js"}, true
	case ".rb":
		return sourceProfile{"ruby", "extension.rb"}, true
	case ".java":
		return sourceProfile{"java", "extension.java"}, true
	case ".cs":
		return sourceProfile{"csharp", "extension.cs"}, true
	case ".sh", ".bash", ".zsh":
		return sourceProfile{"shell", "extension.sh"}, true
	case ".sql":
		return sourceProfile{"sql", "extension.sql"}, true
	}
	if base == "rakefile" || base == "gemfile" {
		return sourceProfile{"ruby", "filename." + base}, true
	}
	return sourceProfile{}, false
}

func shouldSkipSourceDir(name string) bool {
	switch name {
	case ".git", "node_modules", "vendor", "__pycache__", "target", "dist", "build", ".next", ".terraform":
		return true
	default:
		return false
	}
}

func looksLikeSQL(sql string) bool {
	tokens := tokenize(strings.TrimSpace(sql))
	if len(tokens) < 2 {
		return false
	}
	verb := tokens[0]
	structural := false
	for _, token := range tokens[1:] {
		switch token {
		case "from", "into", "table", "set", "values", "where", "join", "column", "index":
			structural = true
		}
	}
	if !structural {
		return false
	}
	switch verb {
	case "select", "insert", "update", "delete", "create", "alter", "drop", "truncate", "with", "replace":
		return true
	default:
		return false
	}
}

func rawSQLFramework(context, path string) string {
	lower := strings.ToLower(context + " " + path)
	switch {
	case strings.Contains(lower, "op.execute"):
		return "alembic"
	case strings.Contains(lower, "find_by_sql") || strings.Contains(lower, "exec_query") || strings.Contains(lower, "execute"):
		if strings.HasSuffix(strings.ToLower(path), ".rb") {
			return "rails"
		}
	case strings.Contains(lower, "$queryraw") || strings.Contains(lower, "$executeraw"):
		return "prisma"
	case strings.Contains(lower, "sequelize.query"):
		return "sequelize"
	case strings.Contains(lower, "fromsqlraw") || strings.Contains(lower, "executesqlraw"):
		return "entity-framework"
	case strings.Contains(lower, ".raw("):
		if strings.HasSuffix(strings.ToLower(path), ".py") {
			return "django"
		}
		return "query-builder"
	}
	return migrationFrameworkFromPath(path)
}

func migrationFrameworkFromPath(path string) string {
	lower := strings.ToLower(filepath.ToSlash(path))
	switch {
	case strings.Contains(lower, "prisma/migrations"):
		return "prisma-migrate"
	case strings.Contains(lower, "db/migrate"):
		return "rails"
	case strings.Contains(lower, "alembic"):
		return "alembic"
	case strings.Contains(lower, "flyway") || regexp.MustCompile(`(^|/)v[0-9]+__.*\.sql$`).MatchString(lower):
		return "flyway"
	case strings.Contains(lower, "liquibase"):
		return "liquibase"
	default:
		if strings.HasSuffix(lower, ".sql") {
			return "sql-migration"
		}
		return ""
	}
}

func normalizeExtractedSQL(sql string) string {
	sql = strings.ReplaceAll(sql, `\n`, "\n")
	sql = strings.ReplaceAll(sql, `\t`, "\t")
	return strings.TrimSpace(sql)
}

func normalizeSourceContent(content string) string {
	return strings.ReplaceAll(content, "\r\n", "\n")
}

func contextAround(content string, index int) string {
	start := index - 120
	if start < 0 {
		start = 0
	}
	end := index + 120
	if end > len(content) {
		end = len(content)
	}
	return content[start:end]
}

func stripSourceLineComment(line string) string {
	for _, marker := range []string{"//", "#"} {
		if idx := strings.Index(line, marker); idx >= 0 {
			return line[:idx]
		}
	}
	return line
}

func lineForStatement(content, statement string) int {
	idx := strings.Index(content, statement)
	if idx < 0 {
		return 1
	}
	return strings.Count(content[:idx], "\n") + 1
}

func modelToTable(model string) string {
	if model == "" {
		return ""
	}
	var out strings.Builder
	for i, r := range model {
		if i > 0 && r >= 'A' && r <= 'Z' {
			out.WriteRune('_')
		}
		out.WriteRune(r)
	}
	table := strings.ToLower(out.String())
	if strings.HasSuffix(table, "y") {
		return strings.TrimSuffix(table, "y") + "ies"
	}
	if strings.HasSuffix(table, "s") {
		return table
	}
	return table + "s"
}

func ormOperation(method string) string {
	method = strings.ToLower(method)
	switch method {
	case "find", "findone", "findall", "findmany", "findunique", "findfirst", "filter", "get", "all", "where", "raw", "fromsqlraw", "find_by", "find_by_sql":
		return "select"
	case "update", "updates", "updatemany", "update_all", "bulk_update", "bulk_update_mappings", "executesqlraw":
		return "update"
	case "delete", "deletemany", "delete_all", "destroy", "destroy_all", "remove":
		return "delete"
	case "create", "bulkcreate", "bulk_create", "bulk_insert_mappings", "add", "save", "upsert", "update_or_create", "persist", "merge":
		return "insert"
	default:
		return method
	}
}
