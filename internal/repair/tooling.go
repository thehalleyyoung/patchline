package repair

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"sort"
)

const LegacyVersionV0 = "patchline.repair/v0"

type LintResult struct {
	OK       bool      `json:"ok"`
	Findings []Finding `json:"findings"`
}

type Finding struct {
	Level       string `json:"level"`
	Code        string `json:"code"`
	Message     string `json:"message"`
	Ref         string `json:"ref,omitempty"`
	Remediation string `json:"remediation"`
}

func Migrate(reader io.Reader) (Manifest, error) {
	content, err := io.ReadAll(reader)
	if err != nil {
		return Manifest{}, err
	}
	var header struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(content, &header); err != nil {
		return Manifest{}, err
	}
	switch header.Version {
	case Version:
		return ReadManifest(bytes.NewReader(content))
	case LegacyVersionV0:
		return migrateV0(content)
	default:
		return Manifest{}, fmt.Errorf("unsupported repair manifest version %q", header.Version)
	}
}

type manifestV0 struct {
	Version          string              `json:"version"`
	Title            string              `json:"title"`
	IncidentID       string              `json:"incident_id"`
	AffectedEntities []string            `json:"affected_entities"`
	Table            string              `json:"table"`
	Where            map[string]string   `json:"where"`
	Steps            []operationV0       `json:"steps"`
	RollbackSnapshot bool                `json:"rollback_snapshot"`
	Checks           []Check             `json:"checks"`
	Metadata         map[string][]string `json:"metadata"`
}

type operationV0 struct {
	Name      string            `json:"name"`
	Action    string            `json:"action"`
	Table     string            `json:"table"`
	Predicate map[string]string `json:"predicate"`
	Values    map[string]string `json:"values"`
	After     []string          `json:"after"`
}

func migrateV0(content []byte) (Manifest, error) {
	var legacy manifestV0
	decoder := json.NewDecoder(bytes.NewReader(content))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&legacy); err != nil {
		return Manifest{}, err
	}
	manifest := Manifest{
		Version:  Version,
		Name:     legacy.Title,
		Incident: legacy.IncidentID,
		Scope: Scope{
			Entities: append([]string(nil), legacy.AffectedEntities...),
			Table:    legacy.Table,
			Where:    copyStringMap(legacy.Where),
		},
		Preconditions: append([]Check(nil), legacy.Checks...),
		Operations:    make([]Operation, 0, len(legacy.Steps)),
		Rollback: Rollback{
			Strategy:         "snapshot",
			SnapshotRequired: legacy.RollbackSnapshot,
		},
	}
	if !legacy.RollbackSnapshot {
		manifest.Rollback.Strategy = "manual"
	}
	for index, step := range legacy.Steps {
		id := step.Name
		if id == "" {
			id = fmt.Sprintf("step_%d", index+1)
		}
		kind := step.Action
		if kind == "" {
			kind = "update"
		}
		manifest.Operations = append(manifest.Operations, Operation{
			ID:        id,
			Kind:      kind,
			Table:     firstString(step.Table, legacy.Table),
			Where:     copyStringMap(firstMap(step.Predicate, legacy.Where)),
			Set:       copyStringMap(step.Values),
			DependsOn: append([]string(nil), step.After...),
		})
	}
	return manifest, nil
}

func Template(name string) (Manifest, error) {
	switch name {
	case "row-restore":
		return Manifest{
			Version:  Version,
			Name:     "restore one corrupted row",
			Incident: "incident:replace-me",
			Scope: Scope{
				Entities: []string{"record:table/id"},
				Table:    "table",
				Where:    map[string]string{"id": "replace-me"},
			},
			Preconditions: []Check{{Kind: "sql", Expr: "select count(*) from table where id = 'replace-me'", Expect: "1"}},
			Operations: []Operation{{
				ID:    "restore_row",
				Kind:  "update",
				Table: "table",
				Where: map[string]string{"id": "replace-me"},
				Set:   map[string]string{"column": "correct-value"},
			}},
			Postconditions: []Check{{Kind: "sql", Expr: "select column from table where id = 'replace-me'", Expect: "correct-value"}},
			Rollback:       Rollback{Strategy: "snapshot", SnapshotRequired: true},
		}, nil
	case "scoped-backfill-reversal":
		return Manifest{
			Version:  Version,
			Name:     "reverse scoped backfill",
			Incident: "incident:replace-me",
			Scope:    Scope{Table: "table", Where: map[string]string{"migration_id": "replace-me"}},
			Preconditions: []Check{
				{Kind: "sql", Expr: "select count(*) from table where migration_id = 'replace-me'", Expect: "<= max_changed_rows"},
			},
			Operations: []Operation{{
				ID:    "reverse_backfill",
				Kind:  "update",
				Table: "table",
				Where: map[string]string{"migration_id": "replace-me"},
				Set:   map[string]string{"column": "previous-value"},
			}},
			Postconditions: []Check{{Kind: "sql", Expr: "select count(*) from table where column = 'bad-value'", Expect: "0"}},
			Rollback:       Rollback{Strategy: "snapshot", SnapshotRequired: true},
		}, nil
	case "report-recompute":
		return Manifest{
			Version:  Version,
			Name:     "recompute derived report",
			Incident: "incident:replace-me",
			Scope:    Scope{Entities: []string{"report:name"}},
			Preconditions: []Check{
				{Kind: "graph", Expr: "slice(report:name)", Expect: "reviewed"},
			},
			Operations: []Operation{{
				ID:   "recompute_report",
				Kind: "replay",
			}},
			Postconditions: []Check{{Kind: "sql", Expr: "select checksum from reports where id = 'name'", Expect: "expected-checksum"}},
			Rollback:       Rollback{Strategy: "snapshot", SnapshotRequired: true},
		}, nil
	default:
		return Manifest{}, fmt.Errorf("unknown repair template %q", name)
	}
}

func Lint(manifest Manifest) LintResult {
	var findings []Finding
	for _, diagnostic := range Validate(manifest, nil) {
		findings = append(findings, Finding{
			Level:       diagnostic.Level,
			Code:        diagnostic.Code,
			Message:     diagnostic.Message,
			Ref:         diagnostic.Ref,
			Remediation: remediation(diagnostic.Code),
		})
	}
	if len(manifest.Preconditions) == 0 {
		findings = append(findings, Finding{"warning", "preconditions.empty", "manifest has no preconditions", "", "Add checks that prove the incident scope is still present before repair."})
	}
	if len(manifest.Postconditions) == 0 {
		findings = append(findings, Finding{"warning", "postconditions.empty", "manifest has no postconditions", "", "Add checks that prove the damaged values or derived outputs are corrected."})
	}
	if manifest.Rollback.Strategy == "" {
		findings = append(findings, Finding{"error", "rollback.empty", "manifest has no rollback strategy", "", "Use snapshot rollback for updates/deletes or document a manual rollback strategy."})
	}
	if manifest.Scope.Table != "" && len(manifest.Scope.Where) == 0 {
		findings = append(findings, Finding{"warning", "scope.table_without_where", "table scope has no predicate", manifest.Scope.Table, "Add a scope predicate that constrains every operation."})
	}
	sort.SliceStable(findings, func(i, j int) bool {
		if findings[i].Level != findings[j].Level {
			return findings[i].Level < findings[j].Level
		}
		if findings[i].Code != findings[j].Code {
			return findings[i].Code < findings[j].Code
		}
		return findings[i].Ref < findings[j].Ref
	})
	return LintResult{OK: !hasErrorFinding(findings), Findings: findings}
}

func hasErrorFinding(findings []Finding) bool {
	for _, finding := range findings {
		if finding.Level == "error" {
			return true
		}
	}
	return false
}

func remediation(code string) string {
	switch code {
	case "manifest.version":
		return "Run patchline migrate-repair and review the upgraded manifest."
	case "operation.where", "operation.delete_scope", "operation.escapes_scope":
		return "Constrain the operation with the declared scope predicate or narrow the scope."
	case "operation.risky_without_snapshot":
		return "Require snapshot rollback before running the operation."
	case "operations.empty":
		return "Add at least one explicit repair operation."
	default:
		return "Edit the manifest and re-run patchline lint-repair."
	}
}

func firstString(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func firstMap(values ...map[string]string) map[string]string {
	for _, value := range values {
		if len(value) > 0 {
			return value
		}
	}
	return nil
}

func copyStringMap(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}
