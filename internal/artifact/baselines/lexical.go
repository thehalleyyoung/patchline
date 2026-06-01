package baselines

import (
	"regexp"
	"sort"
	"strings"

	"github.com/thehalleyyoung/patchline/internal/canonical"
)

const LexicalRuleVersion = "patchline.baseline-lexical-rules/v1"

var keyPredicatePattern = regexp.MustCompile(`\b(id|[a-z_]+_id|uuid|key)\s*(=|in\b)`)

type ScanReport struct {
	Version    string          `json:"version"`
	SourceHash string          `json:"source_hash"`
	Statements []ScanStatement `json:"statements"`
	RuleHash   string          `json:"rule_hash"`
}

type ScanStatement struct {
	Kind     string   `json:"kind"`
	HasWhere bool     `json:"has_where"`
	Rules    []string `json:"rules,omitempty"`
}

func ScanSQL(content []byte) ScanReport {
	report := ScanReport{
		Version:    LexicalRuleVersion,
		SourceHash: canonical.Hash(string(content)),
		RuleHash:   RuleHash(),
	}
	for _, raw := range splitStatements(string(content)) {
		stmt := scanStatement(raw)
		if stmt.Kind != "" {
			report.Statements = append(report.Statements, stmt)
		}
	}
	return report
}

func RuleHash() string {
	return canonical.Hash([]string{
		"ddl:drop",
		"ddl:truncate",
		"sql:update-without-where",
		"sql:delete-without-where",
		"sql:destructive-ddl",
		"sql:broad-update-predicate",
		"guardrail:update-without-where",
		"guardrail:delete-without-where",
		"guardrail:broad-update-predicate",
		"guardrail:destructive-drop",
		"guardrail:destructive-truncate",
		"guardrail:persistent-data-insert",
	})
}

func scanStatement(raw string) ScanStatement {
	normalized := strings.ToLower(strings.Join(strings.Fields(stripComments(raw)), " "))
	if normalized == "" {
		return ScanStatement{}
	}
	fields := strings.Fields(normalized)
	if len(fields) == 0 {
		return ScanStatement{}
	}
	kind := fields[0]
	if kind == "with" {
		kind = firstMutationKeyword(fields)
	}
	stmt := ScanStatement{
		Kind:     kind,
		HasWhere: strings.Contains(" "+normalized+" ", " where "),
	}
	rules := map[string]bool{}
	switch kind {
	case "drop":
		rules["ddl:drop"] = true
		rules["sql:destructive-ddl"] = true
		rules["guardrail:destructive-drop"] = true
	case "truncate":
		rules["ddl:truncate"] = true
		rules["sql:destructive-ddl"] = true
		rules["guardrail:destructive-truncate"] = true
	case "update":
		if !stmt.HasWhere {
			rules["sql:update-without-where"] = true
			rules["guardrail:update-without-where"] = true
		} else if !keyPredicatePattern.MatchString(normalized) {
			rules["sql:broad-update-predicate"] = true
			rules["guardrail:broad-update-predicate"] = true
		}
	case "delete":
		if !stmt.HasWhere {
			rules["sql:delete-without-where"] = true
			rules["guardrail:delete-without-where"] = true
		}
	case "insert":
		rules["guardrail:persistent-data-insert"] = true
	}
	stmt.Rules = sortedKeys(rules)
	return stmt
}

func firstMutationKeyword(fields []string) string {
	for _, field := range fields {
		switch field {
		case "update", "delete", "insert", "drop", "truncate", "alter", "create":
			return field
		}
	}
	return "with"
}

func splitStatements(sql string) []string {
	var out []string
	var current strings.Builder
	var quote rune
	for _, r := range sql {
		if quote != 0 {
			current.WriteRune(r)
			if r == quote {
				quote = 0
			}
			continue
		}
		switch r {
		case '\'', '"':
			quote = r
			current.WriteRune(r)
		case ';':
			out = append(out, current.String())
			current.Reset()
		default:
			current.WriteRune(r)
		}
	}
	if strings.TrimSpace(current.String()) != "" {
		out = append(out, current.String())
	}
	return out
}

func stripComments(sql string) string {
	lines := strings.Split(sql, "\n")
	for i, line := range lines {
		if idx := strings.Index(line, "--"); idx >= 0 {
			lines[i] = line[:idx]
		}
	}
	return strings.Join(lines, "\n")
}

func sortedKeys(values map[string]bool) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
