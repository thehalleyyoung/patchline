package repair

import (
	"fmt"
	"sort"
	"strings"

	"github.com/patchline/patchline/internal/canonical"
)

type SQLPlan struct {
	Version    string         `json:"version"`
	Manifest   string         `json:"manifest"`
	Incident   string         `json:"incident"`
	Statements []SQLStatement `json:"statements"`
	Hash       string         `json:"hash"`
}

type SQLStatement struct {
	OperationID string `json:"operation_id"`
	Kind        string `json:"kind"`
	SQL         string `json:"sql"`
}

func GenerateSQL(manifest Manifest) (SQLPlan, error) {
	lint := Lint(manifest)
	if !lint.OK {
		return SQLPlan{}, fmt.Errorf("repair manifest must pass lint before SQL generation")
	}
	plan := SQLPlan{
		Version:  "patchline.repair-sql/v1",
		Manifest: manifest.Name,
		Incident: manifest.Incident,
	}
	for _, op := range manifest.Operations {
		switch op.Kind {
		case "insert":
			sql, err := InsertStatement(op.Table, op.Set)
			if err != nil {
				return SQLPlan{}, fmt.Errorf("operation %s: %w", op.ID, err)
			}
			plan.Statements = append(plan.Statements, SQLStatement{OperationID: op.ID, Kind: op.Kind, SQL: sql})
		case "update":
			sql, err := UpdateStatement(op.Table, op.Set, op.Where)
			if err != nil {
				return SQLPlan{}, fmt.Errorf("operation %s: %w", op.ID, err)
			}
			plan.Statements = append(plan.Statements, SQLStatement{OperationID: op.ID, Kind: op.Kind, SQL: sql})
		case "delete":
			sql, err := DeleteStatement(op.Table, op.Where)
			if err != nil {
				return SQLPlan{}, fmt.Errorf("operation %s: %w", op.ID, err)
			}
			plan.Statements = append(plan.Statements, SQLStatement{OperationID: op.ID, Kind: op.Kind, SQL: sql})
		default:
			return SQLPlan{}, fmt.Errorf("operation %s: SQL generation does not support kind %q", op.ID, op.Kind)
		}
	}
	plan.Hash = canonical.Hash(struct {
		Version    string         `json:"version"`
		Manifest   string         `json:"manifest"`
		Incident   string         `json:"incident"`
		Statements []SQLStatement `json:"statements"`
	}{plan.Version, plan.Manifest, plan.Incident, plan.Statements})
	return plan, nil
}

func InsertStatement(table string, values map[string]string) (string, error) {
	if table == "" {
		return "", fmt.Errorf("table is required")
	}
	if len(values) == 0 {
		return "", fmt.Errorf("row values are required")
	}
	keys := sortedKeys(values)
	columns := make([]string, 0, len(keys))
	literals := make([]string, 0, len(keys))
	for _, key := range keys {
		columns = append(columns, quoteIdent(key))
		literals = append(literals, quoteLiteral(values[key]))
	}
	return "INSERT INTO " + quoteIdent(table) + " (" + strings.Join(columns, ", ") + ") VALUES (" + strings.Join(literals, ", ") + ");", nil
}

func UpdateStatement(table string, set, where map[string]string) (string, error) {
	if table == "" {
		return "", fmt.Errorf("table is required")
	}
	if len(set) == 0 {
		return "", fmt.Errorf("set values are required")
	}
	if len(where) == 0 {
		return "", fmt.Errorf("where predicate is required")
	}
	setParts := assignments(set)
	whereParts := assignments(where)
	return "UPDATE " + quoteIdent(table) + " SET " + strings.Join(setParts, ", ") + " WHERE " + strings.Join(whereParts, " AND ") + ";", nil
}

func DeleteStatement(table string, where map[string]string) (string, error) {
	if table == "" {
		return "", fmt.Errorf("table is required")
	}
	if len(where) == 0 {
		return "", fmt.Errorf("where predicate is required")
	}
	return "DELETE FROM " + quoteIdent(table) + " WHERE " + strings.Join(assignments(where), " AND ") + ";", nil
}

func assignments(values map[string]string) []string {
	keys := sortedKeys(values)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, quoteIdent(key)+" = "+quoteLiteral(values[key]))
	}
	return parts
}

func sortedKeys(values map[string]string) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func quoteIdent(value string) string {
	parts := strings.Split(value, ".")
	for index, part := range parts {
		parts[index] = `"` + strings.ReplaceAll(part, `"`, `""`) + `"`
	}
	return strings.Join(parts, ".")
}

func quoteLiteral(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "''") + "'"
}
