package expandcontract

import (
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"

	"github.com/thehalleyyoung/patchline/internal/canonical"
	"github.com/thehalleyyoung/patchline/internal/invariant"
)

const SpecVersion = "patchline.expand-contract/v1"
const ReportVersion = "patchline.expand-contract-report/v1"

type Spec struct {
	Version       string            `json:"version"`
	Name          string            `json:"name"`
	InvariantSpec invariant.Spec    `json:"invariant_spec"`
	Templates     []TemplateRequest `json:"templates"`
	ORMProjects   []ORMProjectSpec  `json:"orm_projects"`
}

type TemplateRequest struct {
	ID                 string `json:"id"`
	InvariantID        string `json:"invariant_id"`
	LegacyColumn       string `json:"legacy_column,omitempty"`
	NewColumn          string `json:"new_column,omitempty"`
	BackfillExpression string `json:"backfill_expression,omitempty"`
}

type ORMProjectSpec struct {
	Name         string `json:"name"`
	Ecosystem    string `json:"ecosystem"`
	Root         string `json:"root"`
	Table        string `json:"table,omitempty"`
	Column       string `json:"column,omitempty"`
	LegacyColumn string `json:"legacy_column,omitempty"`
}

type Report struct {
	Version   string     `json:"version"`
	Name      string     `json:"name"`
	OK        bool       `json:"ok"`
	Summary   Summary    `json:"summary"`
	Templates []Template `json:"templates"`
	ORMChecks []ORMCheck `json:"orm_checks"`
	Hash      string     `json:"hash"`
}

type Summary struct {
	Templates        int `json:"templates"`
	TemplatesChecked int `json:"templates_checked"`
	Projects         int `json:"projects"`
	ProjectsVerified int `json:"projects_verified"`
	Stages           int `json:"stages"`
	RefutedChecks    int `json:"refuted_checks"`
}

type Template struct {
	ID           string                `json:"id"`
	InvariantID  string                `json:"invariant_id"`
	Invariant    invariant.Declaration `json:"invariant"`
	Table        string                `json:"table"`
	NewColumn    string                `json:"new_column"`
	LegacyColumn string                `json:"legacy_column,omitempty"`
	BackfillFrom string                `json:"backfill_from"`
	Stages       []Stage               `json:"stages"`
	Obligations  []Obligation          `json:"obligations"`
	Valid        bool                  `json:"valid"`
	Diagnostics  []Diagnostic          `json:"diagnostics,omitempty"`
}

type Stage struct {
	ID        string   `json:"id"`
	Purpose   string   `json:"purpose"`
	SQL       []string `json:"sql"`
	Checks    []string `json:"checks"`
	DependsOn []string `json:"depends_on,omitempty"`
}

type Obligation struct {
	ID      string `json:"id"`
	Status  string `json:"status"`
	Formula string `json:"formula"`
	Reason  string `json:"reason"`
}

type Diagnostic struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type ORMCheck struct {
	ProjectName string     `json:"project_name"`
	Ecosystem   string     `json:"ecosystem"`
	Root        string     `json:"root"`
	TemplateID  string     `json:"template_id"`
	Table       string     `json:"table"`
	Column      string     `json:"column"`
	EvidenceOK  bool       `json:"evidence_ok"`
	Evidence    []Evidence `json:"evidence"`
	Missing     []string   `json:"missing,omitempty"`
}

type Evidence struct {
	Requirement string `json:"requirement"`
	Kind        string `json:"kind"`
	Phase       string `json:"phase,omitempty"`
	Path        string `json:"path"`
	Line        int    `json:"line"`
}

type scannedFile struct {
	path  string
	lines []string
}

func ReadSpec(reader io.Reader) (Spec, error) {
	decoder := json.NewDecoder(reader)
	decoder.DisallowUnknownFields()
	var spec Spec
	if err := decoder.Decode(&spec); err != nil {
		return Spec{}, err
	}
	if spec.Version != SpecVersion {
		return Spec{}, fmt.Errorf("expand/contract spec version must be %s", SpecVersion)
	}
	if spec.InvariantSpec.Version != invariant.Version {
		return Spec{}, fmt.Errorf("embedded invariant spec version must be %s", invariant.Version)
	}
	return spec, nil
}

func BuildReport(spec Spec, baseDir string) (Report, error) {
	if spec.Version != SpecVersion {
		return Report{}, fmt.Errorf("expand/contract spec version must be %s", SpecVersion)
	}
	if spec.InvariantSpec.Version != invariant.Version {
		return Report{}, fmt.Errorf("embedded invariant spec version must be %s", invariant.Version)
	}
	if spec.Name == "" {
		return Report{}, fmt.Errorf("spec name is required")
	}
	if len(spec.Templates) == 0 {
		return Report{}, fmt.Errorf("at least one template request is required")
	}
	if len(spec.ORMProjects) == 0 {
		return Report{}, fmt.Errorf("at least one ORM project is required")
	}

	invariants := map[string]invariant.Declaration{}
	for _, declaration := range spec.InvariantSpec.Invariants {
		if declaration.ID != "" {
			invariants[declaration.ID] = declaration
		}
	}

	report := Report{
		Version: ReportVersion,
		Name:    spec.Name,
		OK:      true,
	}
	for _, request := range sortedTemplateRequests(spec.Templates) {
		template := buildTemplate(request, invariants[request.InvariantID], request.InvariantID != "" && invariants[request.InvariantID].ID != "")
		if !template.Valid {
			report.OK = false
			report.Summary.RefutedChecks++
		} else {
			report.Summary.TemplatesChecked++
		}
		report.Summary.Stages += len(template.Stages)
		report.Templates = append(report.Templates, template)

		for _, project := range sortedProjects(spec.ORMProjects) {
			check := checkORMProject(project, template, baseDir)
			if !check.EvidenceOK {
				report.OK = false
				report.Summary.RefutedChecks++
			} else {
				report.Summary.ProjectsVerified++
			}
			report.ORMChecks = append(report.ORMChecks, check)
		}
	}
	report.Summary.Templates = len(report.Templates)
	report.Summary.Projects = len(report.ORMChecks)
	report.Hash = canonical.Hash(struct {
		Version   string     `json:"version"`
		Name      string     `json:"name"`
		OK        bool       `json:"ok"`
		Summary   Summary    `json:"summary"`
		Templates []Template `json:"templates"`
		ORMChecks []ORMCheck `json:"orm_checks"`
	}{report.Version, report.Name, report.OK, report.Summary, report.Templates, report.ORMChecks})
	return report, nil
}

func RenderMarkdown(report Report) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Verified expand/contract migration templates\n\n")
	fmt.Fprintf(&b, "Patchline generated expand/contract templates from invariant specifications and checked them against ORM project evidence.\n\n")
	fmt.Fprintf(&b, "## Summary\n\n")
	fmt.Fprintf(&b, "| Metric | Value |\n| --- | ---: |\n")
	fmt.Fprintf(&b, "| Templates | %d |\n", report.Summary.Templates)
	fmt.Fprintf(&b, "| Templates checked | %d |\n", report.Summary.TemplatesChecked)
	fmt.Fprintf(&b, "| ORM checks | %d |\n", report.Summary.Projects)
	fmt.Fprintf(&b, "| ORM checks verified | %d |\n", report.Summary.ProjectsVerified)
	fmt.Fprintf(&b, "| Refuted checks | %d |\n\n", report.Summary.RefutedChecks)
	for _, template := range report.Templates {
		fmt.Fprintf(&b, "## Template `%s`\n\n", template.ID)
		fmt.Fprintf(&b, "Invariant `%s` on `%s.%s`; valid: `%t`.\n\n", template.InvariantID, template.Table, template.NewColumn, template.Valid)
		fmt.Fprintf(&b, "| Stage | Purpose | Depends on |\n| --- | --- | --- |\n")
		for _, stage := range template.Stages {
			fmt.Fprintf(&b, "| `%s` | %s | %s |\n", stage.ID, stage.Purpose, strings.Join(stage.DependsOn, ", "))
		}
		fmt.Fprintf(&b, "\n### SQL template\n\n```sql\n")
		for _, stage := range template.Stages {
			fmt.Fprintf(&b, "-- %s\n", stage.ID)
			for _, statement := range stage.SQL {
				fmt.Fprintf(&b, "%s\n", statement)
			}
		}
		fmt.Fprintf(&b, "```\n\n")
	}
	fmt.Fprintf(&b, "## ORM project evidence\n\n")
	fmt.Fprintf(&b, "| Project | Ecosystem | Template | Evidence | Missing |\n| --- | --- | --- | ---: | --- |\n")
	for _, check := range report.ORMChecks {
		missing := "-"
		if len(check.Missing) > 0 {
			missing = strings.Join(check.Missing, ", ")
		}
		fmt.Fprintf(&b, "| `%s` | `%s` | `%s` | %d | %s |\n", check.ProjectName, check.Ecosystem, check.TemplateID, len(check.Evidence), missing)
	}
	fmt.Fprintf(&b, "\nEvidence rows intentionally store only relative paths, line numbers, and obligation names, not source snippets.\n")
	return b.String()
}

func RenderSQL(report Report) string {
	var b strings.Builder
	for _, template := range report.Templates {
		fmt.Fprintf(&b, "-- template: %s invariant: %s\n", template.ID, template.InvariantID)
		for _, stage := range template.Stages {
			fmt.Fprintf(&b, "-- stage: %s\n", stage.ID)
			for _, statement := range stage.SQL {
				fmt.Fprintf(&b, "%s\n", statement)
			}
		}
		fmt.Fprintf(&b, "\n")
	}
	return b.String()
}

func buildTemplate(request TemplateRequest, declaration invariant.Declaration, found bool) Template {
	id := request.ID
	if id == "" {
		id = request.InvariantID
	}
	template := Template{
		ID:           id,
		InvariantID:  request.InvariantID,
		Invariant:    declaration,
		Table:        declaration.Table,
		NewColumn:    firstNonEmpty(request.NewColumn, declaration.Column),
		LegacyColumn: request.LegacyColumn,
		BackfillFrom: firstNonEmpty(request.BackfillExpression, request.LegacyColumn),
		Valid:        true,
	}
	if !found {
		template.Valid = false
		template.Diagnostics = append(template.Diagnostics, Diagnostic{Code: "invariant.missing", Message: "template references an unknown invariant id"})
	}
	if template.Table == "" || template.NewColumn == "" {
		template.Valid = false
		template.Diagnostics = append(template.Diagnostics, Diagnostic{Code: "invariant.scope", Message: "expand/contract templates require an invariant with table and column scope"})
	}
	if template.BackfillFrom == "" {
		template.BackfillFrom = "/* source expression required */"
		template.Obligations = append(template.Obligations, Obligation{
			ID:      "backfill.source",
			Status:  "assumed",
			Formula: "backfill expression supplied before execution",
			Reason:  "the template can be generated, but a project-specific source expression must be reviewed",
		})
	}

	table := sqlIdent(template.Table)
	column := sqlIdent(template.NewColumn)
	legacy := sqlIdent(template.LegacyColumn)
	template.Stages = []Stage{{
		ID:      "expand",
		Purpose: "add a nullable compatibility column before readers require it",
		SQL: []string{
			fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s TEXT NULL;", table, column),
		},
		Checks: []string{"column exists", "legacy readers remain compatible"},
	}, {
		ID:      "backfill",
		Purpose: "populate the new column from the declared compatibility source",
		SQL: []string{
			fmt.Sprintf("UPDATE %s SET %s = %s WHERE %s IS NULL;", table, column, template.BackfillFrom, column),
		},
		Checks:    []string{"backfill completeness proof is recorded before contract"},
		DependsOn: []string{"expand"},
	}, {
		ID:        "validate",
		Purpose:   "discharge the invariant before tightening the schema",
		SQL:       invariantValidationSQL(declaration, table, column),
		Checks:    []string{"invariant engine accepts the declaration", "zero counterexamples before contract"},
		DependsOn: []string{"backfill"},
	}, {
		ID:        "contract",
		Purpose:   "tighten the new invariant and remove old compatibility only after validation",
		SQL:       contractSQL(table, column, legacy),
		Checks:    []string{"contract runs only after expand/backfill/validate evidence"},
		DependsOn: []string{"validate"},
	}}
	template.Obligations = append(template.Obligations,
		Obligation{
			ID:      "invariant." + request.InvariantID,
			Status:  checked(template.Valid),
			Formula: invariantFormula(declaration, template.Table, template.NewColumn),
			Reason:  "template stages are generated from the embedded patchline.invariants/v1 declaration",
		},
		Obligation{
			ID:      "phase.order",
			Status:  "checked",
			Formula: "expand < backfill < validate < contract",
			Reason:  "contract stage depends on validation, which depends on backfill, which depends on expand",
		},
		Obligation{
			ID:      "backfill.completeness",
			Status:  "assumed",
			Formula: "no NULL or stale rows remain before contract",
			Reason:  "this template records the obligation consumed by the existing backfill-completeness gate and the staged planner",
		},
	)
	return template
}

func checkORMProject(project ORMProjectSpec, template Template, baseDir string) ORMCheck {
	rootPath := resolveProjectRoot(baseDir, project.Root)
	check := ORMCheck{
		ProjectName: project.Name,
		Ecosystem:   project.Ecosystem,
		Root:        stableRel(baseDir, rootPath),
		TemplateID:  template.ID,
		Table:       firstNonEmpty(project.Table, template.Table),
		Column:      firstNonEmpty(project.Column, template.NewColumn),
	}
	legacy := firstNonEmpty(project.LegacyColumn, template.LegacyColumn)
	files, err := scanFiles(rootPath)
	if err != nil {
		check.Missing = []string{"project_root"}
		return check
	}
	requirements := map[string]Evidence{}
	for _, file := range files {
		role := fileRole(file.path, project.Ecosystem)
		if role == "" {
			continue
		}
		for i, line := range file.lines {
			if !targetMention(line, file.path, check.Table, check.Column, legacy) {
				continue
			}
			lineNumber := i + 1
			switch role {
			case "migration":
				addFirst(requirements, "migration_file", Evidence{Requirement: "migration_file", Kind: "migration", Path: file.path, Line: lineNumber})
				if hasExpandMarker(line) {
					addFirst(requirements, "expand_phase", Evidence{Requirement: "expand_phase", Kind: "migration", Phase: "expand", Path: file.path, Line: lineNumber})
				}
				if hasBackfillMarker(line) {
					addFirst(requirements, "backfill_phase", Evidence{Requirement: "backfill_phase", Kind: "migration", Phase: "backfill", Path: file.path, Line: lineNumber})
				}
				if hasContractMarker(line) {
					addFirst(requirements, "contract_phase", Evidence{Requirement: "contract_phase", Kind: "migration", Phase: "contract", Path: file.path, Line: lineNumber})
				}
			case "model":
				addFirst(requirements, "model_file", Evidence{Requirement: "model_file", Kind: "model", Path: file.path, Line: lineNumber})
				if legacy != "" && hasDualWriteMarker(line) && targetMention(line, file.path, check.Table, check.Column, legacy) {
					addFirst(requirements, "dual_write", Evidence{Requirement: "dual_write", Kind: "model", Phase: "expand", Path: file.path, Line: lineNumber})
				}
			}
		}
	}

	required := []string{"migration_file", "model_file", "expand_phase", "backfill_phase", "contract_phase", "dual_write"}
	for _, key := range required {
		if evidence, ok := requirements[key]; ok {
			check.Evidence = append(check.Evidence, evidence)
		} else {
			check.Missing = append(check.Missing, key)
		}
	}
	if !phaseOrderOK(requirements) {
		check.Missing = append(check.Missing, "phase_order")
	}
	sort.Slice(check.Evidence, func(i, j int) bool {
		if check.Evidence[i].Requirement != check.Evidence[j].Requirement {
			return check.Evidence[i].Requirement < check.Evidence[j].Requirement
		}
		if check.Evidence[i].Path != check.Evidence[j].Path {
			return check.Evidence[i].Path < check.Evidence[j].Path
		}
		return check.Evidence[i].Line < check.Evidence[j].Line
	})
	sort.Strings(check.Missing)
	check.EvidenceOK = len(check.Missing) == 0
	return check
}

func invariantValidationSQL(declaration invariant.Declaration, table, column string) []string {
	switch declaration.Kind {
	case "unique":
		return []string{fmt.Sprintf("SELECT %s, COUNT(*) AS duplicates FROM %s WHERE %s IS NOT NULL GROUP BY %s HAVING COUNT(*) > 1;", column, table, column, column)}
	case "nonnegative":
		return []string{fmt.Sprintf("SELECT COUNT(*) AS invalid_rows FROM %s WHERE %s < 0;", table, column)}
	case "enum":
		var values []string
		for _, value := range declaration.Values {
			values = append(values, "'"+strings.ReplaceAll(value, "'", "''")+"'")
		}
		return []string{fmt.Sprintf("SELECT COUNT(*) AS invalid_rows FROM %s WHERE %s IS NOT NULL AND %s NOT IN (%s);", table, column, column, strings.Join(values, ", "))}
	case "foreign_key":
		return []string{fmt.Sprintf("SELECT COUNT(*) AS orphan_rows FROM %s left_table LEFT JOIN %s ref_table ON left_table.%s = ref_table.%s WHERE left_table.%s IS NOT NULL AND ref_table.%s IS NULL;", table, sqlIdent(declaration.RefTable), column, sqlIdent(declaration.RefColumn), column, sqlIdent(declaration.RefColumn))}
	default:
		return []string{fmt.Sprintf("SELECT COUNT(*) AS unchecked_rows FROM %s WHERE %s IS NULL;", table, column)}
	}
}

func contractSQL(table, column, legacy string) []string {
	sql := []string{fmt.Sprintf("ALTER TABLE %s ALTER COLUMN %s SET NOT NULL;", table, column)}
	if legacy != "" {
		sql = append(sql, fmt.Sprintf("ALTER TABLE %s DROP COLUMN %s;", table, legacy))
	}
	return sql
}

func invariantFormula(declaration invariant.Declaration, table, column string) string {
	switch declaration.Kind {
	case "unique":
		return fmt.Sprintf("unique(%s.%s)", table, column)
	case "nonnegative":
		return fmt.Sprintf("%s.%s >= 0", table, column)
	case "enum":
		return fmt.Sprintf("%s.%s in {%s}", table, column, strings.Join(declaration.Values, ","))
	case "foreign_key":
		return fmt.Sprintf("foreign_key(%s.%s -> %s.%s)", table, column, declaration.RefTable, declaration.RefColumn)
	default:
		return fmt.Sprintf("%s(%s.%s)", declaration.Kind, table, column)
	}
}

func scanFiles(root string) ([]scannedFile, error) {
	var files []scannedFile
	if _, err := os.Stat(root); err != nil {
		return nil, err
	}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		name := entry.Name()
		if entry.IsDir() {
			switch name {
			case ".git", "node_modules", "vendor", "__pycache__":
				return filepath.SkipDir
			}
			return nil
		}
		if !scanExtension(path) {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		files = append(files, scannedFile{path: filepath.ToSlash(rel), lines: strings.Split(string(data), "\n")})
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(files, func(i, j int) bool { return files[i].path < files[j].path })
	return files, nil
}

func scanExtension(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".rb", ".py", ".prisma", ".sql", ".ts", ".js":
		return true
	default:
		return false
	}
}

func fileRole(path, ecosystem string) string {
	lower := strings.ToLower(filepath.ToSlash(path))
	if strings.Contains(lower, "migrate") || strings.Contains(lower, "migration") || strings.HasSuffix(lower, ".sql") {
		return "migration"
	}
	if strings.Contains(lower, "model") || strings.Contains(lower, "entity") || strings.HasSuffix(lower, "schema.prisma") {
		return "model"
	}
	switch strings.ToLower(ecosystem) {
	case "prisma":
		if strings.Contains(lower, "schema.prisma") || strings.Contains(lower, "dual_write") {
			return "model"
		}
	case "typeorm":
		if strings.Contains(lower, "entities") {
			return "model"
		}
	}
	return ""
}

func targetMention(line, path, table, column, legacy string) bool {
	text := compact(line + " " + path)
	columnOK := containsCompact(text, column)
	if legacy != "" {
		columnOK = columnOK || containsCompact(text, legacy)
	}
	tableOK := containsCompact(text, table) || containsCompact(text, singular(table))
	return columnOK || (tableOK && (strings.Contains(text, "model") || strings.Contains(text, "entity")))
}

func hasExpandMarker(line string) bool {
	lower := strings.ToLower(line)
	return strings.Contains(lower, "add_column") ||
		strings.Contains(lower, "addfield") ||
		strings.Contains(lower, "add column") ||
		strings.Contains(lower, "null: true") ||
		strings.Contains(lower, "null=true") ||
		strings.Contains(lower, " null") ||
		strings.Contains(lower, "string?")
}

func hasBackfillMarker(line string) bool {
	lower := strings.ToLower(line)
	return strings.Contains(lower, "backfill") ||
		strings.Contains(lower, "update_all") ||
		strings.Contains(lower, "runpython") ||
		strings.Contains(lower, "update ") ||
		strings.Contains(lower, "execute") ||
		strings.Contains(lower, "find_each") ||
		strings.Contains(lower, "data migration")
}

func hasContractMarker(line string) bool {
	lower := strings.ToLower(line)
	return strings.Contains(lower, "null: false") ||
		strings.Contains(lower, "null=false") ||
		strings.Contains(lower, "not null") ||
		strings.Contains(lower, "change_column_null") ||
		strings.Contains(lower, "alterfield") ||
		strings.Contains(lower, "set not null") ||
		strings.Contains(lower, "drop column") ||
		strings.Contains(lower, "remove_column")
}

func hasDualWriteMarker(line string) bool {
	lower := strings.ToLower(line)
	return strings.Contains(lower, "dual_write") ||
		strings.Contains(lower, "dual write") ||
		strings.Contains(lower, "before_save") ||
		strings.Contains(lower, "before_validation") ||
		strings.Contains(lower, "save(") ||
		strings.Contains(lower, "setter") ||
		strings.Contains(lower, "prepersist") ||
		strings.Contains(lower, "$use") ||
		strings.Contains(lower, "middleware")
}

func phaseOrderOK(requirements map[string]Evidence) bool {
	expand, okExpand := requirements["expand_phase"]
	backfill, okBackfill := requirements["backfill_phase"]
	contract, okContract := requirements["contract_phase"]
	if !okExpand || !okBackfill || !okContract {
		return true
	}
	return evidenceBefore(expand, backfill) && evidenceBefore(backfill, contract)
}

func evidenceBefore(left, right Evidence) bool {
	if left.Path != right.Path {
		return left.Path < right.Path
	}
	return left.Line < right.Line
}

func addFirst(values map[string]Evidence, key string, evidence Evidence) {
	if existing, ok := values[key]; ok && evidenceBefore(existing, evidence) {
		return
	}
	values[key] = evidence
}

func sortedTemplateRequests(requests []TemplateRequest) []TemplateRequest {
	out := append([]TemplateRequest(nil), requests...)
	sort.SliceStable(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func sortedProjects(projects []ORMProjectSpec) []ORMProjectSpec {
	out := append([]ORMProjectSpec(nil), projects...)
	sort.SliceStable(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func resolveProjectRoot(baseDir, root string) string {
	if filepath.IsAbs(root) {
		return filepath.Clean(root)
	}
	return filepath.Clean(filepath.Join(baseDir, root))
}

func stableRel(baseDir, path string) string {
	rel, err := filepath.Rel(baseDir, path)
	if err != nil {
		return filepath.ToSlash(filepath.Clean(path))
	}
	return filepath.ToSlash(rel)
}

func sqlIdent(value string) string {
	var b strings.Builder
	for i, r := range value {
		if r == '_' || unicode.IsLetter(r) || (i > 0 && unicode.IsDigit(r)) {
			b.WriteRune(r)
		}
	}
	if b.Len() == 0 {
		return "unnamed"
	}
	return b.String()
}

func compact(value string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(value) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func containsCompact(text, needle string) bool {
	if needle == "" {
		return false
	}
	return strings.Contains(text, compact(needle))
}

func singular(value string) string {
	if strings.HasSuffix(value, "ies") {
		return strings.TrimSuffix(value, "ies") + "y"
	}
	return strings.TrimSuffix(value, "s")
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func checked(ok bool) string {
	if ok {
		return "checked"
	}
	return "refuted"
}
