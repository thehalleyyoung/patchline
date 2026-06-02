package migration

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"github.com/thehalleyyoung/patchline/internal/canonical"
	"github.com/thehalleyyoung/patchline/internal/effects"
)

const Version = "patchline.migration-lex/v1"

type Dialect string

const (
	DialectGeneric   Dialect = ""
	DialectPostgres  Dialect = "postgres"
	DialectMySQL     Dialect = "mysql"
	DialectSQLite    Dialect = "sqlite"
	DialectSQLServer Dialect = "sqlserver"
	DialectOracle    Dialect = "oracle"
	DialectBigQuery  Dialect = "bigquery"
)

type Risk string

const (
	RiskLow    Risk = "low"
	RiskMedium Risk = "medium"
	RiskHigh   Risk = "high"
)

type Report struct {
	Version    string      `json:"version"`
	Source     string      `json:"source"`
	Dialect    Dialect     `json:"dialect,omitempty"`
	Statements []Statement `json:"statements"`
	Summary    Summary     `json:"summary"`
}

type Statement struct {
	Index       int      `json:"index"`
	Kind        string   `json:"kind"`
	Table       string   `json:"table,omitempty"`
	HasWhere    bool     `json:"has_where"`
	Normalized  string   `json:"normalized_sql"`
	Fingerprint string   `json:"fingerprint"`
	Risk        Risk     `json:"risk"`
	Effect      string   `json:"effect"`
	Reasons     []string `json:"reasons,omitempty"`
}

type Summary struct {
	TotalStatements int      `json:"total_statements"`
	HighRisk        int      `json:"high_risk"`
	MediumRisk      int      `json:"medium_risk"`
	LowRisk         int      `json:"low_risk"`
	Tables          []string `json:"tables"`
	ReportHash      string   `json:"report_hash"`
}

func AnalyzeFile(path string) (Report, error) {
	return AnalyzeFileWithDialect(path, DialectGeneric)
}

func AnalyzeFileWithDialect(path string, dialect Dialect) (Report, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return Report{}, err
	}
	return AnalyzeBytesWithDialect(filepath.ToSlash(path), content, dialect)
}

func AnalyzeBytes(source string, content []byte) (Report, error) {
	return AnalyzeBytesWithDialect(source, content, DialectGeneric)
}

func AnalyzeBytesWithDialect(source string, content []byte, dialect Dialect) (Report, error) {
	if err := ValidateDialect(dialect); err != nil {
		return Report{}, err
	}
	statements := splitStatements(string(content))
	report := Report{Version: Version, Source: source, Dialect: dialect}
	tables := map[string]bool{}
	for i, raw := range statements {
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" {
			continue
		}
		statement := analyzeStatement(i, trimmed, dialect)
		report.Statements = append(report.Statements, statement)
		if statement.Table != "" {
			tables[statement.Table] = true
		}
		switch statement.Risk {
		case RiskHigh:
			report.Summary.HighRisk++
		case RiskMedium:
			report.Summary.MediumRisk++
		case RiskLow:
			report.Summary.LowRisk++
		}
	}
	report.Summary.TotalStatements = len(report.Statements)
	for table := range tables {
		report.Summary.Tables = append(report.Summary.Tables, table)
	}
	sort.Strings(report.Summary.Tables)
	report.Summary.ReportHash = canonical.Hash(struct {
		Version    string      `json:"version"`
		Source     string      `json:"source"`
		Dialect    Dialect     `json:"dialect,omitempty"`
		Statements []Statement `json:"statements"`
	}{
		Version:    report.Version,
		Source:     report.Source,
		Dialect:    report.Dialect,
		Statements: report.Statements,
	})
	return report, nil
}

func ValidateDialect(dialect Dialect) error {
	switch dialect {
	case DialectGeneric, DialectPostgres, DialectMySQL, DialectSQLite, DialectSQLServer, DialectOracle, DialectBigQuery:
		return nil
	default:
		return fmt.Errorf("unsupported SQL dialect %q", dialect)
	}
}

func analyzeStatement(index int, sql string, dialect Dialect) Statement {
	stripped := stripComments(sql)
	normalized := NormalizeSQLWithDialect(stripped, dialect)
	tokens := tokenize(normalized)
	ast := parseStatementAST(tokens)
	statement := Statement{
		Index:       index,
		Kind:        ast.Kind,
		Table:       ast.Table,
		HasWhere:    ast.HasWhere,
		Normalized:  normalized,
		Fingerprint: normalized,
		Risk:        RiskLow,
		Effect:      string(effects.EffectNoop),
	}

	switch statement.Kind {
	case "update":
		classification := effects.Infer(effects.Mutation{
			Kind:      "update",
			Table:     statement.Table,
			WhereKeys: whereMarker(statement.HasWhere),
			SetKeys:   []string{"<unknown>"},
		})
		statement.Effect = string(classification.Effect)
		if statement.HasWhere {
			if hasKeyPredicate(tokens) {
				statement.Risk = RiskMedium
				statement.Reasons = append(statement.Reasons, "predicate-bounded update still changes persistent data")
			} else {
				statement.Risk = RiskHigh
				statement.Reasons = append(statement.Reasons, "broad update predicate lacks an obvious row key")
			}
		} else {
			statement.Risk = RiskHigh
			statement.Reasons = append(statement.Reasons, "unbounded update can rewrite an entire table")
		}
	case "delete":
		classification := effects.Infer(effects.Mutation{Kind: "delete", Table: statement.Table, WhereKeys: whereMarker(statement.HasWhere)})
		statement.Effect = string(classification.Effect)
		if statement.HasWhere {
			statement.Risk = RiskHigh
			statement.Reasons = append(statement.Reasons, "delete removes rows even when predicate-bounded")
		} else {
			statement.Risk = RiskHigh
			statement.Reasons = append(statement.Reasons, "unbounded delete can remove an entire table")
		}
	case "alter":
		statement.Risk = RiskMedium
		statement.Effect = string(effects.EffectUnknown)
		statement.Reasons = append(statement.Reasons, "schema alteration can invalidate code and repair manifests")
	case "drop", "truncate":
		statement.Risk = RiskHigh
		statement.Effect = string(effects.EffectDestructive)
		statement.Reasons = append(statement.Reasons, statement.Kind+" is destructive")
	case "insert":
		statement.Risk = RiskMedium
		statement.Effect = string(effects.EffectUnknown)
		statement.Reasons = append(statement.Reasons, "insert changes persistent data and may need provenance")
	case "merge":
		statement.Risk = RiskHigh
		statement.Effect = string(effects.EffectUnknown)
		statement.Reasons = append(statement.Reasons, "merge can update or insert many persistent rows")
	case "create":
		statement.Risk = RiskLow
		statement.Effect = string(effects.EffectNoop)
	default:
		statement.Risk = RiskMedium
		statement.Effect = string(effects.EffectUnknown)
		statement.Reasons = append(statement.Reasons, "statement kind is not recognized by lexical analyzer")
	}
	applyDialectRules(&statement, tokens, dialect)
	sort.Strings(statement.Reasons)
	return statement
}

func Fingerprint(sql string) string {
	return NormalizeSQLWithDialect(sql, DialectGeneric)
}

func NormalizeSQLWithDialect(sql string, dialect Dialect) string {
	withoutComments := stripComments(sql)
	dialectSQL := normalizeDialectSyntax(withoutComments, dialect)
	normalizedStrings := replaceStringLiterals(dialectSQL)
	normalizedNumbers := regexp.MustCompile(`\b\d+(\.\d+)?\b`).ReplaceAllString(normalizedStrings, "?")
	normalizedWhitespace := strings.Join(strings.Fields(strings.ToLower(normalizedNumbers)), " ")
	return strings.TrimSpace(normalizedWhitespace)
}

func splitStatements(sql string) []string {
	var statements []string
	var current strings.Builder
	var quote rune
	dollarTag := ""
	lineComment := false
	blockCommentDepth := 0

	for i := 0; i < len(sql); i++ {
		ch := rune(sql[i])
		next := rune(0)
		if i+1 < len(sql) {
			next = rune(sql[i+1])
		}

		if lineComment {
			current.WriteRune(ch)
			if ch == '\n' {
				lineComment = false
			}
			continue
		}
		if blockCommentDepth > 0 {
			current.WriteRune(ch)
			if ch == '/' && next == '*' {
				blockCommentDepth++
				current.WriteRune(next)
				i++
				continue
			}
			if ch == '*' && next == '/' {
				blockCommentDepth--
				current.WriteRune(next)
				i++
			}
			continue
		}
		if dollarTag != "" {
			current.WriteRune(ch)
			if strings.HasPrefix(sql[i:], dollarTag) {
				for j := 1; j < len(dollarTag); j++ {
					current.WriteByte(sql[i+j])
				}
				i += len(dollarTag) - 1
				dollarTag = ""
			}
			continue
		}
		if quote != 0 {
			current.WriteRune(ch)
			if ch == quote {
				if next == quote {
					current.WriteRune(next)
					i++
					continue
				}
				quote = 0
			}
			continue
		}

		if ch == '-' && next == '-' {
			lineComment = true
			current.WriteRune(ch)
			current.WriteRune(next)
			i++
			continue
		}
		if ch == '/' && next == '*' {
			blockCommentDepth = 1
			current.WriteRune(ch)
			current.WriteRune(next)
			i++
			continue
		}
		if ch == '\'' || ch == '"' {
			quote = ch
			current.WriteRune(ch)
			continue
		}
		if ch == '$' {
			if tag := readDollarTag(sql[i:]); tag != "" {
				dollarTag = tag
				current.WriteString(tag)
				i += len(tag) - 1
				continue
			}
		}
		if ch == ';' {
			statements = append(statements, current.String())
			current.Reset()
			continue
		}
		current.WriteRune(ch)
	}
	if strings.TrimSpace(current.String()) != "" {
		statements = append(statements, current.String())
	}
	return statements
}

func stripComments(sql string) string {
	var out strings.Builder
	lineComment := false
	blockCommentDepth := 0
	var quote rune

	for i := 0; i < len(sql); i++ {
		ch := rune(sql[i])
		next := rune(0)
		if i+1 < len(sql) {
			next = rune(sql[i+1])
		}
		if lineComment {
			if ch == '\n' {
				lineComment = false
				out.WriteRune(ch)
			}
			continue
		}
		if blockCommentDepth > 0 {
			if ch == '/' && next == '*' {
				blockCommentDepth++
				i++
				continue
			}
			if ch == '*' && next == '/' {
				blockCommentDepth--
				i++
			}
			continue
		}
		if quote != 0 {
			out.WriteRune(ch)
			if ch == quote {
				if next == quote {
					out.WriteRune(next)
					i++
					continue
				}
				quote = 0
			}
			continue
		}
		if ch == '-' && next == '-' {
			lineComment = true
			i++
			continue
		}
		if ch == '/' && next == '*' {
			blockCommentDepth = 1
			i++
			continue
		}
		if ch == '\'' || ch == '"' {
			quote = ch
		}
		out.WriteRune(ch)
	}
	return out.String()
}

func replaceStringLiterals(sql string) string {
	var out strings.Builder
	var quote rune
	for i := 0; i < len(sql); i++ {
		ch := rune(sql[i])
		next := rune(0)
		if i+1 < len(sql) {
			next = rune(sql[i+1])
		}
		if quote != 0 {
			if ch == quote {
				if next == quote {
					i++
					continue
				}
				quote = 0
			}
			continue
		}
		if ch == '\'' {
			quote = ch
			out.WriteRune('?')
			continue
		}
		out.WriteRune(ch)
	}
	return out.String()
}

func tokenize(sql string) []string {
	var tokens []string
	var current strings.Builder
	flush := func() {
		if current.Len() > 0 {
			tokens = append(tokens, strings.ToLower(current.String()))
			current.Reset()
		}
	}
	for _, ch := range sql {
		if unicode.IsLetter(ch) || unicode.IsDigit(ch) || ch == '_' || ch == '.' {
			current.WriteRune(ch)
			continue
		}
		flush()
	}
	flush()
	return tokens
}

func normalizeDialectSyntax(sql string, dialect Dialect) string {
	switch dialect {
	case DialectMySQL:
		return strings.ReplaceAll(sql, "`", `"`)
	case DialectSQLServer:
		replacer := strings.NewReplacer("[", "", "]", "")
		return replacer.Replace(sql)
	case DialectOracle:
		sql = regexp.MustCompile(`(?i)\bnumber\s*\(\s*\d+\s*,\s*\d+\s*\)`).ReplaceAllString(sql, "numeric")
		sql = regexp.MustCompile(`(?i)\bvarchar2\s*\(\s*\d+\s*(?:byte|char)?\s*\)`).ReplaceAllString(sql, "varchar")
		return regexp.MustCompile(`(?i)\bnow\s*\(\s*\)`).ReplaceAllString(sql, "current_timestamp")
	case DialectBigQuery:
		sql = strings.ReplaceAll(sql, "`", "")
		sql = regexp.MustCompile(`(?i)\bcreate\s+or\s+replace\s+table\b`).ReplaceAllString(sql, "create table")
		return regexp.MustCompile(`(?i)\bmerge\s+into\b`).ReplaceAllString(sql, "merge into")
	default:
		return sql
	}
}

func applyDialectRules(statement *Statement, tokens []string, dialect Dialect) {
	if dialect == DialectGeneric {
		return
	}
	statement.Reasons = append(statement.Reasons, "dialect="+string(dialect))

	switch dialect {
	case DialectPostgres:
		applyPostgresRules(statement, tokens)
	case DialectMySQL:
		applyMySQLRules(statement, tokens)
	case DialectSQLite:
		applySQLiteRules(statement, tokens)
	case DialectSQLServer:
		applySQLServerRules(statement, tokens)
	case DialectOracle:
		applyOracleRules(statement, tokens)
	case DialectBigQuery:
		applyBigQueryRules(statement, tokens)
	}
}

func applyPostgresRules(statement *Statement, tokens []string) {
	if statement.Kind == "create" && containsToken(tokens, "index") && containsToken(tokens, "concurrently") {
		statement.Risk = RiskLow
		statement.Effect = string(effects.EffectNoop)
		statement.Reasons = append(statement.Reasons, "postgres create index concurrently avoids long exclusive table locks")
	}
	if statement.Kind == "alter" && containsToken(tokens, "add") && containsToken(tokens, "column") && containsToken(tokens, "default") && !containsToken(tokens, "null") {
		statement.Risk = RiskHigh
		statement.Reasons = append(statement.Reasons, "postgres add column with non-null default can rewrite large tables on older versions")
	}
	if statement.Kind == "update" && containsToken(tokens, "from") {
		statement.Reasons = append(statement.Reasons, "postgres update-from joins can multiply affected rows")
	}
}

func applyMySQLRules(statement *Statement, tokens []string) {
	if statement.Kind == "replace" {
		statement.Kind = "replace"
		statement.Table = tableAfterReplace(tokens)
		statement.Risk = RiskHigh
		statement.Effect = string(effects.EffectDestructive)
		statement.Reasons = append(statement.Reasons, "mysql replace can delete and reinsert existing rows")
		return
	}
	if statement.Kind == "alter" && containsToken(tokens, "algorithm") && containsToken(tokens, "copy") {
		statement.Risk = RiskHigh
		statement.Reasons = append(statement.Reasons, "mysql algorithm=copy can rebuild and lock the table")
	}
	if statement.Kind == "insert" && containsToken(tokens, "ignore") {
		statement.Reasons = append(statement.Reasons, "mysql insert ignore can silently skip constraint violations")
	}
}

func applySQLiteRules(statement *Statement, tokens []string) {
	if statement.Kind == "pragma" && containsToken(tokens, "foreign_keys") && containsToken(tokens, "off") {
		statement.Risk = RiskHigh
		statement.Effect = string(effects.EffectUnknown)
		statement.Reasons = append(statement.Reasons, "sqlite disabling foreign keys can permit inconsistent repair data")
	}
	if statement.Kind == "vacuum" {
		statement.Risk = RiskMedium
		statement.Effect = string(effects.EffectUnknown)
		statement.Reasons = append(statement.Reasons, "sqlite vacuum rewrites the database file")
	}
	if statement.Kind == "alter" && containsToken(tokens, "drop") && containsToken(tokens, "column") {
		statement.Risk = RiskHigh
		statement.Reasons = append(statement.Reasons, "sqlite drop column removes persisted data from each row")
	}
}

func applySQLServerRules(statement *Statement, tokens []string) {
	if statement.Kind == "update" && containsToken(tokens, "top") {
		if table := sqlServerUpdateTable(tokens); table != "" {
			statement.Table = table
		}
	}
	if statement.Kind == "delete" && containsToken(tokens, "top") {
		if table := sqlServerDeleteTable(tokens); table != "" {
			statement.Table = table
		}
	}
	if statement.Kind == "update" && containsToken(tokens, "top") {
		statement.Risk = RiskHigh
		statement.Reasons = append(statement.Reasons, "sqlserver update top without deterministic order can repair an arbitrary subset")
	}
	if statement.Kind == "delete" && containsToken(tokens, "top") {
		statement.Risk = RiskHigh
		statement.Reasons = append(statement.Reasons, "sqlserver delete top without deterministic order can remove an arbitrary subset")
	}
	if statement.Kind == "alter" && containsToken(tokens, "with") && containsToken(tokens, "check") {
		statement.Reasons = append(statement.Reasons, "sqlserver with check validates existing rows during constraint changes")
	}
}

func applyOracleRules(statement *Statement, tokens []string) {
	if statement.Kind == "merge" {
		statement.Table = tokenAfter(tokens, "into")
		statement.Risk = RiskHigh
		statement.Effect = string(effects.EffectUnknown)
		statement.Reasons = append(statement.Reasons, "oracle merge can update and insert many rows in one statement")
	}
	if statement.Kind == "alter" && containsToken(tokens, "modify") && containsToken(tokens, "not") && containsToken(tokens, "null") {
		statement.Risk = RiskHigh
		statement.Reasons = append(statement.Reasons, "oracle modify not null validates existing rows and can block writes")
	}
	if statement.Kind == "update" && containsToken(tokens, "rownum") {
		statement.Risk = RiskHigh
		statement.Reasons = append(statement.Reasons, "oracle rownum-limited update can affect arbitrary rows without deterministic order")
	}
}

func applyBigQueryRules(statement *Statement, tokens []string) {
	if statement.Kind == "merge" {
		statement.Table = tokenAfter(tokens, "into")
		statement.Risk = RiskHigh
		statement.Effect = string(effects.EffectUnknown)
		statement.Reasons = append(statement.Reasons, "bigquery merge can update, insert, or delete large partitions")
	}
	if statement.Kind == "create" && containsToken(tokens, "table") && containsToken(tokens, "as") && containsToken(tokens, "select") {
		statement.Risk = RiskMedium
		statement.Effect = string(effects.EffectUnknown)
		statement.Reasons = append(statement.Reasons, "bigquery create table as select materializes query output")
	}
	if statement.Kind == "delete" && !statement.HasWhere {
		statement.Reasons = append(statement.Reasons, "bigquery unbounded delete can scan and remove large tables")
	}
}

func readDollarTag(s string) string {
	if len(s) < 2 || s[0] != '$' {
		return ""
	}
	for i := 1; i < len(s); i++ {
		if s[i] == '$' {
			return s[:i+1]
		}
		if !(unicode.IsLetter(rune(s[i])) || unicode.IsDigit(rune(s[i])) || s[i] == '_') {
			return ""
		}
	}
	return ""
}

func firstToken(tokens []string) string {
	if len(tokens) == 0 {
		return "empty"
	}
	return tokens[0]
}

func containsToken(tokens []string, token string) bool {
	for _, candidate := range tokens {
		if candidate == token {
			return true
		}
	}
	return false
}

func hasKeyPredicate(tokens []string) bool {
	whereIndex := -1
	for i, token := range tokens {
		if token == "where" {
			whereIndex = i
			break
		}
	}
	if whereIndex == -1 {
		return false
	}
	for _, token := range tokens[whereIndex+1:] {
		parts := strings.Split(token, ".")
		candidate := parts[len(parts)-1]
		if candidate == "id" || strings.HasSuffix(candidate, "_id") {
			return true
		}
	}
	return false
}

func tokenAfter(tokens []string, token string) string {
	for i, candidate := range tokens {
		if candidate == token && i+1 < len(tokens) {
			return cleanIdentifier(tokens[i+1])
		}
	}
	return ""
}

func tableAfterDelete(tokens []string) string {
	for i := 0; i+1 < len(tokens); i++ {
		if tokens[i] == "from" {
			return cleanIdentifier(tokens[i+1])
		}
	}
	return ""
}

func tableAfterAlter(tokens []string) string {
	for i := 0; i+2 < len(tokens); i++ {
		if tokens[i] == "alter" && tokens[i+1] == "table" {
			if tokens[i+2] == "if" && i+4 < len(tokens) && tokens[i+3] == "exists" {
				return cleanIdentifier(tokens[i+4])
			}
			return cleanIdentifier(tokens[i+2])
		}
	}
	return ""
}

func tableAfterInsert(tokens []string) string {
	for i := 0; i+2 < len(tokens); i++ {
		if tokens[i] == "insert" && tokens[i+1] == "into" {
			return cleanIdentifier(tokens[i+2])
		}
	}
	return ""
}

func tableAfterCreate(tokens []string) string {
	for i := 0; i+2 < len(tokens); i++ {
		if tokens[i] == "create" && tokens[i+1] == "table" {
			return cleanIdentifier(tokens[i+2])
		}
	}
	return ""
}

func tableAfterDropOrTruncate(tokens []string, kind string) string {
	for i, candidate := range tokens {
		if candidate != kind || i+1 >= len(tokens) {
			continue
		}
		next := tokens[i+1]
		if (next == "table" || next == "index") && i+2 < len(tokens) {
			return cleanIdentifier(tokens[i+2])
		}
		return cleanIdentifier(next)
	}
	return ""
}

func tableAfterReplace(tokens []string) string {
	for i := 0; i+1 < len(tokens); i++ {
		if tokens[i] == "replace" {
			if tokens[i+1] == "into" && i+2 < len(tokens) {
				return cleanIdentifier(tokens[i+2])
			}
			return cleanIdentifier(tokens[i+1])
		}
	}
	return ""
}

func sqlServerUpdateTable(tokens []string) string {
	for i := 0; i < len(tokens); i++ {
		if tokens[i] != "update" {
			continue
		}
		j := i + 1
		if j < len(tokens) && tokens[j] == "top" {
			j++
			if j+1 < len(tokens) && tokens[j+1] != "set" {
				j++
			}
		}
		if j < len(tokens) {
			return cleanIdentifier(tokens[j])
		}
	}
	return ""
}

func sqlServerDeleteTable(tokens []string) string {
	for i := 0; i < len(tokens); i++ {
		if tokens[i] != "from" {
			continue
		}
		j := i + 1
		if j < len(tokens) && tokens[j] == "top" {
			j++
			if j < len(tokens) {
				j++
			}
		}
		if j < len(tokens) {
			return cleanIdentifier(tokens[j])
		}
	}
	return ""
}

func cleanIdentifier(identifier string) string {
	return strings.Trim(identifier, "\"`[]")
}

func whereMarker(hasWhere bool) []string {
	if hasWhere {
		return []string{"<predicate>"}
	}
	return nil
}

func ParsePositiveInt(value string) (int, error) {
	n, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("expected integer, got %q", value)
	}
	if n < 0 {
		return 0, fmt.Errorf("expected non-negative integer, got %d", n)
	}
	return n, nil
}
