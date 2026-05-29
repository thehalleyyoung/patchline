package migration

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/patchline/patchline/internal/canonical"
)

const SchemaVersion = "patchline.schema/v1"
const SchemaDiffVersion = "patchline.schema-diff/v1"
const MigrationSemanticsVersion = "patchline.migration-semantics/v1"

type SchemaState struct {
	Version string        `json:"version"`
	Tables  []SchemaTable `json:"tables"`
}

type SchemaTable struct {
	Name    string         `json:"name"`
	Columns []SchemaColumn `json:"columns"`
}

type SchemaColumn struct {
	Name string `json:"name"`
	Type string `json:"type,omitempty"`
}

type SchemaDiffReport struct {
	Version         string       `json:"version"`
	MigrationSource string       `json:"migration_source"`
	Dialect         Dialect      `json:"dialect,omitempty"`
	OK              bool         `json:"ok"`
	ExpectedHash    string       `json:"expected_hash"`
	ActualHash      string       `json:"actual_hash"`
	Hash            string       `json:"hash"`
	Diffs           []SchemaDiff `json:"diffs"`
}

type SchemaDiff struct {
	Kind   string `json:"kind"`
	Table  string `json:"table"`
	Column string `json:"column,omitempty"`
	Expect string `json:"expect,omitempty"`
	Actual string `json:"actual,omitempty"`
}

type MigrationSemanticsReport struct {
	Version         string                 `json:"version"`
	MigrationSource string                 `json:"migration_source"`
	Dialect         Dialect                `json:"dialect,omitempty"`
	InputHash       string                 `json:"input_hash"`
	OutputHash      string                 `json:"output_hash"`
	Hash            string                 `json:"hash"`
	Transformations []SchemaTransformation `json:"transformations"`
	Relational      []RelationalStatement  `json:"relational"`
}

type SchemaTransformation struct {
	Index      int           `json:"index"`
	Kind       string        `json:"kind"`
	Table      string        `json:"table,omitempty"`
	Column     *SchemaColumn `json:"column,omitempty"`
	BeforeHash string        `json:"before_hash"`
	AfterHash  string        `json:"after_hash"`
}

type RelationalStatement struct {
	Index           int      `json:"index"`
	Kind            string   `json:"kind"`
	Table           string   `json:"table,omitempty"`
	Reads           []string `json:"reads,omitempty"`
	Writes          []string `json:"writes,omitempty"`
	Expression      string   `json:"expression"`
	SignatureEffect string   `json:"signature_effect"`
}

func ReadSchema(reader io.Reader) (SchemaState, error) {
	decoder := json.NewDecoder(reader)
	decoder.DisallowUnknownFields()
	var state SchemaState
	if err := decoder.Decode(&state); err != nil {
		return SchemaState{}, err
	}
	if state.Version != SchemaVersion {
		return SchemaState{}, fmt.Errorf("schema version must be %s", SchemaVersion)
	}
	if err := validateSchema(state); err != nil {
		return SchemaState{}, err
	}
	return NormalizeSchema(state), nil
}

func DiffMigrationSchema(source string, content []byte, dialect Dialect, before, expected SchemaState) (SchemaDiffReport, error) {
	if err := ValidateDialect(dialect); err != nil {
		return SchemaDiffReport{}, err
	}
	actual, err := ApplySchemaMigration(before, content, dialect)
	if err != nil {
		return SchemaDiffReport{}, err
	}
	expected = NormalizeSchema(expected)
	report := SchemaDiffReport{
		Version:         SchemaDiffVersion,
		MigrationSource: source,
		Dialect:         dialect,
		ExpectedHash:    canonical.Hash(expected),
		ActualHash:      canonical.Hash(actual),
	}
	report.Diffs = CompareSchemas(expected, actual)
	report.OK = len(report.Diffs) == 0
	report.Hash = canonical.Hash(struct {
		Version      string       `json:"version"`
		Source       string       `json:"source"`
		Dialect      Dialect      `json:"dialect,omitempty"`
		ExpectedHash string       `json:"expected_hash"`
		ActualHash   string       `json:"actual_hash"`
		Diffs        []SchemaDiff `json:"diffs"`
	}{report.Version, report.MigrationSource, report.Dialect, report.ExpectedHash, report.ActualHash, report.Diffs})
	return report, nil
}

func AnalyzeMigrationSemantics(source string, content []byte, dialect Dialect, before SchemaState) (MigrationSemanticsReport, error) {
	if err := ValidateDialect(dialect); err != nil {
		return MigrationSemanticsReport{}, err
	}
	state := NormalizeSchema(before)
	report := MigrationSemanticsReport{
		Version:         MigrationSemanticsVersion,
		MigrationSource: source,
		Dialect:         dialect,
		InputHash:       canonical.Hash(state),
	}
	for index, raw := range splitStatements(string(content)) {
		sql := strings.TrimSpace(stripComments(raw))
		if sql == "" {
			continue
		}
		normalized := normalizeDialectSyntax(sql, dialect)
		tokens := tokenize(normalized)
		ast := parseStatementAST(tokens)
		beforeHash := canonical.Hash(state)
		next, transformation := applySchemaStatement(index, sql, dialect, state, ast, tokens)
		state = next
		if transformation.Kind != "" && transformation.Kind != "identity" {
			transformation.BeforeHash = beforeHash
			transformation.AfterHash = canonical.Hash(state)
			report.Transformations = append(report.Transformations, transformation)
		}
		report.Relational = append(report.Relational, relationalStatement(index, ast, tokens))
	}
	report.OutputHash = canonical.Hash(state)
	report.Hash = canonical.Hash(struct {
		Version         string                 `json:"version"`
		MigrationSource string                 `json:"migration_source"`
		Dialect         Dialect                `json:"dialect,omitempty"`
		InputHash       string                 `json:"input_hash"`
		OutputHash      string                 `json:"output_hash"`
		Transformations []SchemaTransformation `json:"transformations"`
		Relational      []RelationalStatement  `json:"relational"`
	}{report.Version, report.MigrationSource, report.Dialect, report.InputHash, report.OutputHash, report.Transformations, report.Relational})
	return report, nil
}

func ApplySchemaMigration(before SchemaState, content []byte, dialect Dialect) (SchemaState, error) {
	state := NormalizeSchema(before)
	tables := schemaMap(state)
	for _, raw := range splitStatements(string(content)) {
		sql := strings.TrimSpace(stripComments(raw))
		if sql == "" {
			continue
		}
		normalized := normalizeDialectSyntax(sql, dialect)
		tokens := tokenize(normalized)
		ast := parseStatementAST(tokens)
		switch ast.Kind {
		case "create":
			if containsToken(tokens, "table") {
				table := parseCreateTable(sql, dialect)
				if table.Name != "" {
					tables[table.Name] = table
				}
			}
		case "alter":
			applyAlterTable(tables, sql, tokens)
		case "drop":
			if ast.Table != "" {
				delete(tables, ast.Table)
			}
		}
	}
	return schemaFromMap(tables), nil
}

func applySchemaStatement(index int, sql string, dialect Dialect, before SchemaState, ast StatementAST, tokens []string) (SchemaState, SchemaTransformation) {
	tables := schemaMap(before)
	transformation := SchemaTransformation{Index: index, Kind: "identity", Table: ast.Table}
	switch ast.Kind {
	case "create":
		if containsToken(tokens, "table") {
			table := parseCreateTable(sql, dialect)
			if table.Name != "" {
				tables[table.Name] = table
				transformation.Kind = "create_table"
				transformation.Table = table.Name
			}
		}
	case "alter":
		transformation = alterTransformation(index, tables, sql, ast.Table, tokens)
	case "drop":
		if ast.Table != "" {
			delete(tables, ast.Table)
			transformation.Kind = "drop_table"
			transformation.Table = ast.Table
		}
	default:
		transformation.Kind = "identity"
		transformation.Table = ast.Table
	}
	return schemaFromMap(tables), transformation
}

func alterTransformation(index int, tables map[string]SchemaTable, sql, tableName string, tokens []string) SchemaTransformation {
	transformation := SchemaTransformation{Index: index, Kind: "alter_table", Table: tableName}
	if tableName == "" {
		return transformation
	}
	table := tables[tableName]
	table.Name = tableName
	if idx := tokenIndex(tokens, "add"); idx >= 0 {
		if idx+1 < len(tokens) && tokens[idx+1] == "column" {
			idx++
		}
		if idx+1 < len(tokens) {
			column := parseColumnDefinition(strings.Join(tokens[idx+1:], " "), DialectGeneric)
			table.Columns = upsertColumn(table.Columns, column)
			transformation.Kind = "add_column"
			transformation.Column = &column
		}
	}
	if idx := tokenIndex(tokens, "drop"); idx >= 0 && idx+1 < len(tokens) {
		columnToken := idx + 1
		if tokens[columnToken] == "column" && columnToken+1 < len(tokens) {
			columnToken++
		}
		column := SchemaColumn{Name: cleanIdentifier(tokens[columnToken])}
		table.Columns = removeColumn(table.Columns, column.Name)
		transformation.Kind = "drop_column"
		transformation.Column = &column
	}
	_ = sql
	tables[tableName] = NormalizeSchema(SchemaState{Tables: []SchemaTable{table}}).Tables[0]
	return transformation
}

func relationalStatement(index int, ast StatementAST, tokens []string) RelationalStatement {
	statement := RelationalStatement{
		Index:           index,
		Kind:            ast.Kind,
		Table:           ast.Table,
		SignatureEffect: "none",
	}
	switch ast.Kind {
	case "select":
		statement.Reads = relationReads(ast, tokens)
		statement.Expression = "project(filter(" + relationName(ast.Table) + "))"
	case "insert", "replace":
		statement.Writes = nonEmptyStrings(ast.Table)
		statement.Expression = "insert(" + relationName(ast.Table) + ", tuple)"
	case "update":
		statement.Reads = relationReads(ast, tokens)
		statement.Writes = nonEmptyStrings(ast.Table)
		statement.Expression = "update(filter(" + relationName(ast.Table) + "), assignments)"
	case "delete":
		statement.Reads = relationReads(ast, tokens)
		statement.Writes = nonEmptyStrings(ast.Table)
		statement.Expression = "delete(filter(" + relationName(ast.Table) + "))"
	case "create":
		statement.Writes = nonEmptyStrings(ast.Table)
		statement.SignatureEffect = "create_relation"
		statement.Expression = "signature_add_relation(" + relationName(ast.Table) + ")"
	case "alter":
		statement.Reads = nonEmptyStrings(ast.Table)
		statement.Writes = nonEmptyStrings(ast.Table)
		statement.SignatureEffect = "alter_relation"
		statement.Expression = "signature_rewrite_relation(" + relationName(ast.Table) + ")"
	case "drop", "truncate":
		statement.Reads = nonEmptyStrings(ast.Table)
		statement.Writes = nonEmptyStrings(ast.Table)
		statement.SignatureEffect = "drop_or_empty_relation"
		statement.Expression = ast.Kind + "(" + relationName(ast.Table) + ")"
	default:
		statement.Expression = "unknown"
	}
	statement.Reads = stringSet(statement.Reads)
	statement.Writes = stringSet(statement.Writes)
	return statement
}

func relationReads(ast StatementAST, tokens []string) []string {
	reads := nonEmptyStrings(ast.Table)
	for i, token := range tokens {
		if (token == "from" || token == "join" || token == "using") && i+1 < len(tokens) {
			reads = append(reads, cleanIdentifier(tokens[i+1]))
		}
	}
	return stringSet(reads)
}

func relationName(table string) string {
	if table == "" {
		return "<unknown>"
	}
	return table
}

func nonEmptyStrings(values ...string) []string {
	var out []string
	for _, value := range values {
		if value != "" {
			out = append(out, value)
		}
	}
	return out
}

func stringSet(values []string) []string {
	seen := map[string]bool{}
	for _, value := range values {
		if value != "" {
			seen[value] = true
		}
	}
	out := make([]string, 0, len(seen))
	for value := range seen {
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func CompareSchemas(expected, actual SchemaState) []SchemaDiff {
	expectedTables := schemaMap(expected)
	actualTables := schemaMap(actual)
	var diffs []SchemaDiff
	for table, expect := range expectedTables {
		got, ok := actualTables[table]
		if !ok {
			diffs = append(diffs, SchemaDiff{Kind: "missing_table", Table: table})
			continue
		}
		diffs = append(diffs, compareColumns(expect, got)...)
	}
	for table := range actualTables {
		if _, ok := expectedTables[table]; !ok {
			diffs = append(diffs, SchemaDiff{Kind: "unexpected_table", Table: table})
		}
	}
	sortSchemaDiffs(diffs)
	return diffs
}

func NormalizeSchema(state SchemaState) SchemaState {
	state.Version = SchemaVersion
	for i := range state.Tables {
		state.Tables[i].Name = cleanIdentifier(strings.ToLower(state.Tables[i].Name))
		for j := range state.Tables[i].Columns {
			state.Tables[i].Columns[j].Name = cleanIdentifier(strings.ToLower(state.Tables[i].Columns[j].Name))
			state.Tables[i].Columns[j].Type = strings.ToLower(strings.TrimSpace(state.Tables[i].Columns[j].Type))
		}
		sort.Slice(state.Tables[i].Columns, func(a, b int) bool {
			return state.Tables[i].Columns[a].Name < state.Tables[i].Columns[b].Name
		})
	}
	sort.Slice(state.Tables, func(i, j int) bool {
		return state.Tables[i].Name < state.Tables[j].Name
	})
	return state
}

func validateSchema(state SchemaState) error {
	seenTables := map[string]bool{}
	for _, table := range state.Tables {
		if table.Name == "" {
			return fmt.Errorf("schema table name is required")
		}
		tableName := strings.ToLower(table.Name)
		if seenTables[tableName] {
			return fmt.Errorf("duplicate schema table %q", table.Name)
		}
		seenTables[tableName] = true
		seenColumns := map[string]bool{}
		for _, column := range table.Columns {
			if column.Name == "" {
				return fmt.Errorf("schema column name is required for table %q", table.Name)
			}
			columnName := strings.ToLower(column.Name)
			if seenColumns[columnName] {
				return fmt.Errorf("duplicate schema column %q on table %q", column.Name, table.Name)
			}
			seenColumns[columnName] = true
		}
	}
	return nil
}

func parseCreateTable(sql string, dialect Dialect) SchemaTable {
	normalized := normalizeDialectSyntax(sql, dialect)
	tokens := tokenize(normalized)
	table := SchemaTable{Name: tableAfterCreate(tokens)}
	body := betweenFirstParens(sql)
	for _, part := range splitTopLevelComma(body) {
		column := parseColumnDefinition(part, dialect)
		if column.Name != "" {
			table.Columns = append(table.Columns, column)
		}
	}
	return NormalizeSchema(SchemaState{Tables: []SchemaTable{table}}).Tables[0]
}

func applyAlterTable(tables map[string]SchemaTable, sql string, tokens []string) {
	tableName := tableAfterAlter(tokens)
	if tableName == "" {
		return
	}
	table := tables[tableName]
	table.Name = tableName
	if idx := tokenIndex(tokens, "add"); idx >= 0 {
		if idx+1 < len(tokens) && tokens[idx+1] == "column" {
			idx++
		}
		if idx+1 < len(tokens) {
			column := parseColumnDefinition(strings.Join(tokens[idx+1:], " "), DialectGeneric)
			table.Columns = upsertColumn(table.Columns, column)
		}
	}
	if idx := tokenIndex(tokens, "drop"); idx >= 0 && idx+1 < len(tokens) {
		columnToken := idx + 1
		if tokens[columnToken] == "column" && columnToken+1 < len(tokens) {
			columnToken++
		}
		table.Columns = removeColumn(table.Columns, cleanIdentifier(tokens[columnToken]))
	}
	_ = sql
	tables[tableName] = NormalizeSchema(SchemaState{Tables: []SchemaTable{table}}).Tables[0]
}

func parseColumnDefinition(definition string, dialect Dialect) SchemaColumn {
	normalized := normalizeDialectSyntax(definition, dialect)
	tokens := tokenize(normalized)
	if len(tokens) == 0 || isTableConstraint(tokens[0]) {
		return SchemaColumn{}
	}
	column := SchemaColumn{Name: cleanIdentifier(tokens[0])}
	if len(tokens) > 1 && !isColumnConstraint(tokens[1]) {
		column.Type = tokens[1]
	}
	return column
}

func betweenFirstParens(sql string) string {
	start := strings.Index(sql, "(")
	end := strings.LastIndex(sql, ")")
	if start == -1 || end == -1 || end <= start {
		return ""
	}
	return sql[start+1 : end]
}

func splitTopLevelComma(input string) []string {
	var parts []string
	var current strings.Builder
	depth := 0
	var quote rune
	for _, ch := range input {
		if quote != 0 {
			current.WriteRune(ch)
			if ch == quote {
				quote = 0
			}
			continue
		}
		switch ch {
		case '\'', '"', '`':
			quote = ch
		case '(':
			depth++
		case ')':
			if depth > 0 {
				depth--
			}
		case ',':
			if depth == 0 {
				parts = append(parts, strings.TrimSpace(current.String()))
				current.Reset()
				continue
			}
		}
		current.WriteRune(ch)
	}
	if strings.TrimSpace(current.String()) != "" {
		parts = append(parts, strings.TrimSpace(current.String()))
	}
	return parts
}

func schemaMap(state SchemaState) map[string]SchemaTable {
	state = NormalizeSchema(state)
	out := map[string]SchemaTable{}
	for _, table := range state.Tables {
		out[table.Name] = table
	}
	return out
}

func schemaFromMap(tables map[string]SchemaTable) SchemaState {
	state := SchemaState{Version: SchemaVersion}
	for _, table := range tables {
		state.Tables = append(state.Tables, table)
	}
	return NormalizeSchema(state)
}

func compareColumns(expected, actual SchemaTable) []SchemaDiff {
	expectedColumns := columnMap(expected.Columns)
	actualColumns := columnMap(actual.Columns)
	var diffs []SchemaDiff
	for column, expect := range expectedColumns {
		got, ok := actualColumns[column]
		if !ok {
			diffs = append(diffs, SchemaDiff{Kind: "missing_column", Table: expected.Name, Column: column})
			continue
		}
		if expect.Type != "" && got.Type != "" && expect.Type != got.Type {
			diffs = append(diffs, SchemaDiff{Kind: "column_type_mismatch", Table: expected.Name, Column: column, Expect: expect.Type, Actual: got.Type})
		}
	}
	for column := range actualColumns {
		if _, ok := expectedColumns[column]; !ok {
			diffs = append(diffs, SchemaDiff{Kind: "unexpected_column", Table: expected.Name, Column: column})
		}
	}
	return diffs
}

func columnMap(columns []SchemaColumn) map[string]SchemaColumn {
	out := map[string]SchemaColumn{}
	for _, column := range columns {
		out[column.Name] = column
	}
	return out
}

func upsertColumn(columns []SchemaColumn, column SchemaColumn) []SchemaColumn {
	if column.Name == "" {
		return columns
	}
	for i := range columns {
		if columns[i].Name == column.Name {
			columns[i] = column
			return columns
		}
	}
	return append(columns, column)
}

func removeColumn(columns []SchemaColumn, name string) []SchemaColumn {
	var out []SchemaColumn
	for _, column := range columns {
		if column.Name != name {
			out = append(out, column)
		}
	}
	return out
}

func isTableConstraint(token string) bool {
	switch token {
	case "primary", "foreign", "unique", "constraint", "check", "exclude":
		return true
	default:
		return false
	}
}

func isColumnConstraint(token string) bool {
	switch token {
	case "primary", "not", "null", "default", "unique", "references", "check", "collate", "generated":
		return true
	default:
		return false
	}
}

func tokenIndex(tokens []string, token string) int {
	for i, candidate := range tokens {
		if candidate == token {
			return i
		}
	}
	return -1
}

func sortSchemaDiffs(diffs []SchemaDiff) {
	sort.Slice(diffs, func(i, j int) bool {
		if diffs[i].Kind != diffs[j].Kind {
			return diffs[i].Kind < diffs[j].Kind
		}
		if diffs[i].Table != diffs[j].Table {
			return diffs[i].Table < diffs[j].Table
		}
		return diffs[i].Column < diffs[j].Column
	})
}
