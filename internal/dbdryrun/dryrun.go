package dbdryrun

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"os/exec"
	"sort"
	"strings"

	"github.com/thehalleyyoung/patchline/internal/canonical"
	"github.com/thehalleyyoung/patchline/internal/repair"
)

const Version = "patchline.db-dry-run/v1"

type Options struct {
	Dialect string
	DSN     string
	Execute bool
}

type Report struct {
	Version          string            `json:"version"`
	OK               bool              `json:"ok"`
	Mode             string            `json:"mode"`
	Dialect          string            `json:"dialect"`
	Manifest         string            `json:"manifest"`
	Incident         string            `json:"incident"`
	CredentialPolicy string            `json:"credential_policy"`
	Target           Target            `json:"target"`
	Container        ContainerHook     `json:"container"`
	Schema           []TableSchema     `json:"schema"`
	Statements       []DryRunStatement `json:"statements"`
	Script           string            `json:"script"`
	Execution        *ExecutionResult  `json:"execution,omitempty"`
	Warnings         []string          `json:"warnings,omitempty"`
	Errors           []string          `json:"errors,omitempty"`
	Hash             string            `json:"hash"`
}

type Target struct {
	DSNProvided bool   `json:"dsn_provided"`
	LocalOnly   bool   `json:"local_only"`
	Host        string `json:"host,omitempty"`
	Port        string `json:"port,omitempty"`
}

type ContainerHook struct {
	Image        string `json:"image"`
	RunCommand   string `json:"run_command"`
	Client       string `json:"client"`
	ClientHint   string `json:"client_hint"`
	SchemaOnly   bool   `json:"schema_only"`
	NoProduction bool   `json:"no_production_credentials"`
}

type TableSchema struct {
	Table   string   `json:"table"`
	Columns []string `json:"columns"`
}

type DryRunStatement struct {
	OperationID string `json:"operation_id"`
	Kind        string `json:"kind"`
	SQL         string `json:"sql"`
	ExplainSQL  string `json:"explain_sql"`
}

type ExecutionResult struct {
	Ran      bool   `json:"ran"`
	Client   string `json:"client"`
	ExitCode int    `json:"exit_code"`
	Stdout   string `json:"stdout,omitempty"`
	Stderr   string `json:"stderr,omitempty"`
}

func Build(manifest repair.Manifest, opts Options) (Report, error) {
	dialect := normalizeDialect(opts.Dialect)
	if dialect == "" {
		return Report{}, fmt.Errorf("dialect must be postgres or mysql")
	}
	if opts.DSN != "" && !IsLocalDSN(opts.DSN) {
		return Report{}, fmt.Errorf("refusing non-local database target %q; use a localhost/container DSN only", redactedDSN(opts.DSN))
	}
	report := Report{
		Version:          Version,
		OK:               true,
		Mode:             "schema-only",
		Dialect:          dialect,
		Manifest:         manifest.Name,
		Incident:         manifest.Incident,
		CredentialPolicy: "schema-only dry-runs never require production credentials; execution is allowed only for explicit localhost/container DSNs",
		Target:           targetFromDSN(opts.DSN),
		Container:        containerHook(dialect),
		Schema:           schemasForManifest(manifest),
	}
	for _, op := range manifest.Operations {
		statement, err := statementForOperation(dialect, op)
		if err != nil {
			return Report{}, err
		}
		report.Statements = append(report.Statements, statement)
	}
	report.Script = scriptForReport(report)
	if !opts.Execute {
		report.Warnings = append(report.Warnings, "execution skipped; generated schema-only script for an explicit local container target")
	}
	report.Hash = hashReport(report)
	return report, nil
}

func Execute(report Report, dsn string) (Report, error) {
	if strings.TrimSpace(dsn) == "" {
		return Report{}, fmt.Errorf("--execute requires --dsn with an explicit localhost/container target")
	}
	if !IsLocalDSN(dsn) {
		return Report{}, fmt.Errorf("refusing non-local database target %q; use a localhost/container DSN only", redactedDSN(dsn))
	}
	client := report.Container.Client
	if _, err := exec.LookPath(client); err != nil {
		return Report{}, fmt.Errorf("%s client not found; start/use the local container hook and install the matching client, or omit --execute", client)
	}
	tmp, err := os.CreateTemp("", "patchline-db-dry-run-*.sql")
	if err != nil {
		return Report{}, err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if _, err := tmp.WriteString(report.Script); err != nil {
		_ = tmp.Close()
		return Report{}, err
	}
	if err := tmp.Close(); err != nil {
		return Report{}, err
	}
	cmd := exec.Command(client, clientArgs(report.Dialect, dsn, tmpPath)...)
	output, err := cmd.CombinedOutput()
	result := &ExecutionResult{Ran: true, Client: client, Stdout: string(output)}
	if cmd.ProcessState != nil {
		result.ExitCode = cmd.ProcessState.ExitCode()
	}
	if err != nil {
		result.Stderr = err.Error()
		report.OK = false
		report.Errors = append(report.Errors, err.Error())
	}
	report.Execution = result
	report.Hash = hashReport(report)
	if err != nil {
		return report, err
	}
	return report, nil
}

func IsLocalDSN(raw string) bool {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return false
	}
	if strings.HasPrefix(raw, "/") || strings.Contains(raw, "host=/") {
		return true
	}
	if strings.HasPrefix(raw, "postgres://") || strings.HasPrefix(raw, "postgresql://") || strings.HasPrefix(raw, "mysql://") {
		parsed, err := url.Parse(raw)
		if err != nil {
			return false
		}
		return isLocalHost(parsed.Hostname())
	}
	for _, part := range strings.Fields(raw) {
		if strings.HasPrefix(part, "host=") {
			return isLocalHost(strings.TrimPrefix(part, "host="))
		}
		if strings.HasPrefix(part, "tcp(") && strings.Contains(part, ")") {
			hostPort := strings.TrimSuffix(strings.TrimPrefix(part, "tcp("), ")")
			host, _, err := net.SplitHostPort(hostPort)
			if err != nil {
				host = strings.Split(hostPort, ":")[0]
			}
			return isLocalHost(host)
		}
	}
	return strings.Contains(raw, "@tcp(127.0.0.1:") || strings.Contains(raw, "@tcp(localhost:")
}

func normalizeDialect(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "postgres", "postgresql", "pg":
		return "postgres"
	case "mysql", "mariadb":
		return "mysql"
	default:
		return ""
	}
}

func isLocalHost(host string) bool {
	host = strings.Trim(host, "[]")
	switch strings.ToLower(host) {
	case "", "localhost":
		return true
	case "127.0.0.1", "::1":
		return true
	default:
		ip := net.ParseIP(host)
		return ip != nil && ip.IsLoopback()
	}
}

func targetFromDSN(dsn string) Target {
	target := Target{DSNProvided: strings.TrimSpace(dsn) != "", LocalOnly: true}
	if dsn == "" {
		return target
	}
	if parsed, err := url.Parse(dsn); err == nil && parsed.Hostname() != "" {
		target.Host = parsed.Hostname()
		target.Port = parsed.Port()
	}
	return target
}

func containerHook(dialect string) ContainerHook {
	switch dialect {
	case "mysql":
		return ContainerHook{
			Image:        "mysql:8",
			RunCommand:   "docker run --rm --name patchline-mysql-dry-run -e MYSQL_ALLOW_EMPTY_PASSWORD=yes -e MYSQL_DATABASE=patchline -p 127.0.0.1:3307:3306 mysql:8",
			Client:       "mysql",
			ClientHint:   "mysql --host=127.0.0.1 --port=3307 --user=root patchline < script.sql",
			SchemaOnly:   true,
			NoProduction: true,
		}
	default:
		return ContainerHook{
			Image:        "postgres:16",
			RunCommand:   "docker run --rm --name patchline-postgres-dry-run -e POSTGRES_HOST_AUTH_METHOD=trust -e POSTGRES_DB=patchline -p 127.0.0.1:55432:5432 postgres:16",
			Client:       "psql",
			ClientHint:   "psql postgres://postgres@127.0.0.1:55432/patchline -v ON_ERROR_STOP=1 -f script.sql",
			SchemaOnly:   true,
			NoProduction: true,
		}
	}
}

func schemasForManifest(manifest repair.Manifest) []TableSchema {
	columns := map[string]map[string]bool{}
	for _, op := range manifest.Operations {
		if op.Table == "" {
			continue
		}
		if columns[op.Table] == nil {
			columns[op.Table] = map[string]bool{}
		}
		for key := range op.Where {
			columns[op.Table][key] = true
		}
		for key := range op.Set {
			columns[op.Table][key] = true
		}
		if len(columns[op.Table]) == 0 {
			columns[op.Table]["id"] = true
		}
	}
	tables := make([]string, 0, len(columns))
	for table := range columns {
		tables = append(tables, table)
	}
	sort.Strings(tables)
	var out []TableSchema
	for _, table := range tables {
		out = append(out, TableSchema{Table: table, Columns: sortedBoolKeys(columns[table])})
	}
	return out
}

func statementForOperation(dialect string, op repair.Operation) (DryRunStatement, error) {
	var sql string
	var err error
	switch op.Kind {
	case "insert":
		sql, err = insertSQL(dialect, op.Table, op.Set)
	case "update":
		sql, err = updateSQL(dialect, op.Table, op.Set, op.Where)
	case "delete":
		sql, err = deleteSQL(dialect, op.Table, op.Where)
	default:
		err = fmt.Errorf("operation %s: schema-only DB dry-run supports insert, update, and delete, not %q", op.ID, op.Kind)
	}
	if err != nil {
		return DryRunStatement{}, err
	}
	return DryRunStatement{OperationID: op.ID, Kind: op.Kind, SQL: sql, ExplainSQL: "EXPLAIN " + strings.TrimSuffix(sql, ";") + ";"}, nil
}

func scriptForReport(report Report) string {
	var b strings.Builder
	fmt.Fprintf(&b, "-- %s\n-- mode: schema-only\n-- manifest: %s\n", Version, report.Manifest)
	switch report.Dialect {
	case "mysql":
		fmt.Fprintf(&b, "START TRANSACTION;\n")
		for _, schema := range report.Schema {
			fmt.Fprintf(&b, "CREATE TEMPORARY TABLE %s (%s);\n", quoteIdent(report.Dialect, schema.Table), columnDefs(report.Dialect, schema.Columns))
		}
	default:
		fmt.Fprintf(&b, "BEGIN;\n")
		for _, schema := range report.Schema {
			fmt.Fprintf(&b, "CREATE TEMP TABLE %s (%s) ON COMMIT DROP;\n", quoteIdent(report.Dialect, schema.Table), columnDefs(report.Dialect, schema.Columns))
		}
	}
	for _, statement := range report.Statements {
		fmt.Fprintf(&b, "%s\n", statement.ExplainSQL)
	}
	fmt.Fprintf(&b, "ROLLBACK;\n")
	return b.String()
}

func columnDefs(dialect string, columns []string) string {
	var defs []string
	for _, column := range columns {
		defs = append(defs, quoteIdent(dialect, column)+" TEXT")
	}
	if len(defs) == 0 {
		defs = append(defs, quoteIdent(dialect, "id")+" TEXT")
	}
	return strings.Join(defs, ", ")
}

func insertSQL(dialect, table string, values map[string]string) (string, error) {
	if table == "" || len(values) == 0 {
		return "", fmt.Errorf("insert requires table and values")
	}
	keys := sortedStringKeys(values)
	var columns, literals []string
	for _, key := range keys {
		columns = append(columns, quoteIdent(dialect, key))
		literals = append(literals, quoteLiteral(values[key]))
	}
	return fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s);", quoteIdent(dialect, table), strings.Join(columns, ", "), strings.Join(literals, ", ")), nil
}

func updateSQL(dialect, table string, set, where map[string]string) (string, error) {
	if table == "" || len(set) == 0 || len(where) == 0 {
		return "", fmt.Errorf("update requires table, set values, and where predicate")
	}
	return fmt.Sprintf("UPDATE %s SET %s WHERE %s;", quoteIdent(dialect, table), assignments(dialect, set, ", "), assignments(dialect, where, " AND ")), nil
}

func deleteSQL(dialect, table string, where map[string]string) (string, error) {
	if table == "" || len(where) == 0 {
		return "", fmt.Errorf("delete requires table and where predicate")
	}
	return fmt.Sprintf("DELETE FROM %s WHERE %s;", quoteIdent(dialect, table), assignments(dialect, where, " AND ")), nil
}

func assignments(dialect string, values map[string]string, sep string) string {
	keys := sortedStringKeys(values)
	var parts []string
	for _, key := range keys {
		parts = append(parts, quoteIdent(dialect, key)+" = "+quoteLiteral(values[key]))
	}
	return strings.Join(parts, sep)
}

func quoteIdent(dialect, value string) string {
	quote := `"`
	escapedQuote := `""`
	if dialect == "mysql" {
		quote = "`"
		escapedQuote = "``"
	}
	parts := strings.Split(value, ".")
	for i, part := range parts {
		parts[i] = quote + strings.ReplaceAll(part, quote, escapedQuote) + quote
	}
	return strings.Join(parts, ".")
}

func quoteLiteral(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "''") + "'"
}

func clientArgs(dialect, dsn, script string) []string {
	if dialect == "mysql" {
		return mysqlClientArgs(dsn, script)
	}
	return []string{dsn, "-v", "ON_ERROR_STOP=1", "-f", script}
}

func mysqlClientArgs(dsn, script string) []string {
	args := []string{"--batch", "--raw", "--execute", "source " + script}
	if parsed, err := url.Parse(dsn); err == nil && strings.EqualFold(parsed.Scheme, "mysql") {
		if parsed.Hostname() != "" {
			args = append(args, "--host="+parsed.Hostname())
		}
		if parsed.Port() != "" {
			args = append(args, "--port="+parsed.Port())
		}
		if parsed.User != nil {
			args = append(args, "--user="+parsed.User.Username())
			if password, ok := parsed.User.Password(); ok {
				args = append(args, "--password="+password)
			}
		}
		if db := strings.TrimPrefix(parsed.Path, "/"); db != "" {
			args = append(args, db)
		}
		return args
	}
	if before, after, ok := strings.Cut(dsn, "@tcp("); ok {
		user := before
		password := ""
		if u, p, ok := strings.Cut(before, ":"); ok {
			user, password = u, p
		}
		hostPort, rest, ok := strings.Cut(after, ")")
		if ok {
			host, port, err := net.SplitHostPort(hostPort)
			if err != nil {
				host = strings.Split(hostPort, ":")[0]
			}
			args = append(args, "--host="+host)
			if port != "" {
				args = append(args, "--port="+port)
			}
			if user != "" {
				args = append(args, "--user="+user)
			}
			if password != "" {
				args = append(args, "--password="+password)
			}
			if db := strings.Trim(strings.Split(rest, "?")[0], "/"); db != "" {
				args = append(args, db)
			}
			return args
		}
	}
	return append(args, dsn)
}

func sortedStringKeys(values map[string]string) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func sortedBoolKeys(values map[string]bool) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		if key != "" {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	return keys
}

func redactedDSN(dsn string) string {
	if parsed, err := url.Parse(dsn); err == nil && parsed.Host != "" {
		parsed.User = nil
		return parsed.String()
	}
	return "<redacted>"
}

func hashReport(report Report) string {
	copy := report
	copy.Hash = ""
	copy.Execution = nil
	return canonical.Hash(copy)
}
