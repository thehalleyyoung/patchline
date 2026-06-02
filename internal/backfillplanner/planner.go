package backfillplanner

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/thehalleyyoung/patchline/internal/canonical"
	"github.com/thehalleyyoung/patchline/internal/replay"
)

const SpecVersion = "patchline.backfill-plan/v1"
const ReportVersion = "patchline.backfill-plan-report/v1"

type Spec struct {
	Version               string      `json:"version"`
	Name                  string      `json:"name"`
	Table                 string      `json:"table"`
	PrimaryKey            string      `json:"primary_key,omitempty"`
	SourceColumn          string      `json:"source_column,omitempty"`
	TargetColumn          string      `json:"target_column"`
	ExpectedRows          int         `json:"expected_rows,omitempty"`
	CompatibilityCodeRefs []string    `json:"compatibility_code_refs,omitempty"`
	Stages                []StageSpec `json:"stages"`
}

type StageSpec struct {
	ID                   string   `json:"id"`
	Kind                 string   `json:"kind"`
	DependsOn            []string `json:"depends_on,omitempty"`
	Command              string   `json:"command,omitempty"`
	TightensConstraint   bool     `json:"tightens_constraint,omitempty"`
	DeletesCompatibility bool     `json:"deletes_compatibility,omitempty"`
}

type Report struct {
	Version     string            `json:"version"`
	Name        string            `json:"name"`
	OK          bool              `json:"ok"`
	Scope       Scope             `json:"scope"`
	Summary     Summary           `json:"summary"`
	Stages      []Stage           `json:"stages"`
	Obligations []Obligation      `json:"obligations"`
	Proof       CompletenessProof `json:"proof"`
	SQL         []SQLStatement    `json:"sql"`
	Hash        string            `json:"hash"`
}

type Scope struct {
	Table                 string   `json:"table"`
	PrimaryKey            string   `json:"primary_key"`
	SourceColumn          string   `json:"source_column,omitempty"`
	TargetColumn          string   `json:"target_column"`
	CompatibilityCodeRefs []string `json:"compatibility_code_refs,omitempty"`
}

type Summary struct {
	RowsChecked     int `json:"rows_checked"`
	Stages          int `json:"stages"`
	ReadyStages     int `json:"ready_stages"`
	BlockedStages   int `json:"blocked_stages"`
	Obligations     int `json:"obligations"`
	Counterexamples int `json:"counterexamples"`
}

type Stage struct {
	ID                   string   `json:"id"`
	Kind                 string   `json:"kind"`
	DependsOn            []string `json:"depends_on,omitempty"`
	Command              string   `json:"command,omitempty"`
	TightensConstraint   bool     `json:"tightens_constraint,omitempty"`
	DeletesCompatibility bool     `json:"deletes_compatibility,omitempty"`
	RequiresCompleteness bool     `json:"requires_completeness"`
	Ready                bool     `json:"ready"`
	BlockReason          string   `json:"block_reason,omitempty"`
}

type Obligation struct {
	ID      string `json:"id"`
	Status  string `json:"status"`
	Formula string `json:"formula"`
	Reason  string `json:"reason"`
}

type CompletenessProof struct {
	Status          string           `json:"status"`
	TableHash       string           `json:"table_hash,omitempty"`
	RowsChecked     int              `json:"rows_checked"`
	RowProofs       []RowProof       `json:"row_proofs,omitempty"`
	Counterexamples []Counterexample `json:"counterexamples,omitempty"`
	Limitation      string           `json:"limitation"`
}

type RowProof struct {
	RowID      string `json:"row_id"`
	SourceHash string `json:"source_hash,omitempty"`
	TargetHash string `json:"target_hash,omitempty"`
}

type Counterexample struct {
	RowID      string `json:"row_id,omitempty"`
	Code       string `json:"code"`
	Message    string `json:"message"`
	SourceHash string `json:"source_hash,omitempty"`
	TargetHash string `json:"target_hash,omitempty"`
}

type SQLStatement struct {
	StageID string `json:"stage_id"`
	Purpose string `json:"purpose"`
	SQL     string `json:"sql"`
}

func ReadSpec(reader io.Reader) (Spec, error) {
	decoder := json.NewDecoder(reader)
	decoder.DisallowUnknownFields()
	var spec Spec
	if err := decoder.Decode(&spec); err != nil {
		return Spec{}, err
	}
	if spec.Version != SpecVersion {
		return Spec{}, fmt.Errorf("backfill plan spec version must be %s", SpecVersion)
	}
	return spec, nil
}

func BuildPlan(spec Spec, store replay.Store) (Report, error) {
	if err := validateSpec(spec); err != nil {
		return Report{}, err
	}
	spec.PrimaryKey = firstNonEmpty(spec.PrimaryKey, "id")
	if len(spec.Stages) == 0 {
		spec.Stages = defaultStages()
	}

	proof := proveCompleteness(spec, store)
	stages, stageObligations := evaluateStages(spec, proof.Status == "checked")
	obligations := append([]Obligation{scopeObligation(spec), completenessObligation(spec, proof)}, stageObligations...)
	sort.SliceStable(obligations, func(i, j int) bool { return obligations[i].ID < obligations[j].ID })

	report := Report{
		Version: ReportVersion,
		Name:    spec.Name,
		OK:      proof.Status == "checked" && obligationsChecked(obligations),
		Scope: Scope{
			Table:                 spec.Table,
			PrimaryKey:            spec.PrimaryKey,
			SourceColumn:          spec.SourceColumn,
			TargetColumn:          spec.TargetColumn,
			CompatibilityCodeRefs: sortedStrings(spec.CompatibilityCodeRefs),
		},
		Stages:      stages,
		Obligations: obligations,
		Proof:       proof,
		SQL:         buildSQL(spec),
	}
	report.Summary.RowsChecked = proof.RowsChecked
	report.Summary.Stages = len(report.Stages)
	report.Summary.Obligations = len(report.Obligations)
	report.Summary.Counterexamples = len(proof.Counterexamples)
	for _, stage := range report.Stages {
		if stage.Ready {
			report.Summary.ReadyStages++
		} else {
			report.Summary.BlockedStages++
		}
	}
	report.Hash = reportHash(report)
	return report, nil
}

func RenderMarkdown(report Report) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Staged data-backfill plan\n\n")
	fmt.Fprintf(&b, "Patchline gates constraint-tightening and compatibility-code deletion on a deterministic completeness proof over the provided replay store.\n\n")
	fmt.Fprintf(&b, "## Summary\n\n")
	fmt.Fprintf(&b, "| Metric | Value |\n| --- | ---: |\n")
	fmt.Fprintf(&b, "| Rows checked | %d |\n", report.Summary.RowsChecked)
	fmt.Fprintf(&b, "| Ready stages | %d |\n", report.Summary.ReadyStages)
	fmt.Fprintf(&b, "| Blocked stages | %d |\n", report.Summary.BlockedStages)
	fmt.Fprintf(&b, "| Counterexamples | %d |\n", report.Summary.Counterexamples)
	fmt.Fprintf(&b, "| OK | `%t` |\n\n", report.OK)
	fmt.Fprintf(&b, "Scope: `%s.%s` backfilled from `%s` using primary key `%s`.\n\n", report.Scope.Table, report.Scope.TargetColumn, firstNonEmpty(report.Scope.SourceColumn, "<non-empty target>"), report.Scope.PrimaryKey)

	fmt.Fprintf(&b, "## Stage readiness\n\n")
	fmt.Fprintf(&b, "| Stage | Kind | Ready | Requires completeness | Block reason |\n| --- | --- | ---: | ---: | --- |\n")
	for _, stage := range report.Stages {
		reason := "-"
		if stage.BlockReason != "" {
			reason = stage.BlockReason
		}
		fmt.Fprintf(&b, "| `%s` | `%s` | `%t` | `%t` | %s |\n", stage.ID, stage.Kind, stage.Ready, stage.RequiresCompleteness, reason)
	}
	fmt.Fprintf(&b, "\n## Completeness proof\n\n")
	fmt.Fprintf(&b, "Status: `%s`; table hash: `%s`.\n\n", report.Proof.Status, report.Proof.TableHash)
	if len(report.Proof.Counterexamples) > 0 {
		fmt.Fprintf(&b, "| Row | Code | Message |\n| --- | --- | --- |\n")
		for _, counterexample := range report.Proof.Counterexamples {
			fmt.Fprintf(&b, "| `%s` | `%s` | %s |\n", counterexample.RowID, counterexample.Code, counterexample.Message)
		}
		fmt.Fprintf(&b, "\n")
	}
	fmt.Fprintf(&b, "Limitation: %s\n", report.Proof.Limitation)
	return b.String()
}

func RenderSQL(report Report) string {
	var b strings.Builder
	for _, statement := range report.SQL {
		fmt.Fprintf(&b, "-- stage: %s purpose: %s\n%s\n", statement.StageID, statement.Purpose, statement.SQL)
	}
	return b.String()
}

func validateSpec(spec Spec) error {
	if spec.Version != SpecVersion {
		return fmt.Errorf("backfill plan spec version must be %s", SpecVersion)
	}
	if spec.Name == "" {
		return fmt.Errorf("spec name is required")
	}
	if spec.Table == "" {
		return fmt.Errorf("table is required")
	}
	if spec.TargetColumn == "" {
		return fmt.Errorf("target_column is required")
	}
	if spec.ExpectedRows < 0 {
		return fmt.Errorf("expected_rows must be non-negative")
	}
	ids := map[string]bool{}
	for _, stage := range spec.Stages {
		if stage.ID == "" {
			return fmt.Errorf("stage id is required")
		}
		if ids[stage.ID] {
			return fmt.Errorf("stage id %q is duplicated", stage.ID)
		}
		ids[stage.ID] = true
	}
	return nil
}

func proveCompleteness(spec Spec, store replay.Store) CompletenessProof {
	proof := CompletenessProof{
		Status:     "checked",
		Limitation: "Completeness is exhaustive over the finite replay store provided to this command; SQL NULL and empty string are both represented as missing/empty string in replay fixtures.",
	}
	rows, ok := store.Tables[spec.Table]
	if !ok {
		proof.Status = "refuted"
		proof.Counterexamples = append(proof.Counterexamples, Counterexample{
			Code:    "table_missing",
			Message: "target table is missing from replay store",
		})
		return proof
	}
	proof.TableHash = canonical.Hash(rows)
	rowIDs := sortedMapKeys(rows)
	if len(rowIDs) == 0 {
		proof.Status = "inconclusive"
		proof.Counterexamples = append(proof.Counterexamples, Counterexample{
			Code:    "table_empty",
			Message: "target table has no rows, so backfill completeness is not proven for legacy data",
		})
		return proof
	}
	if spec.ExpectedRows > 0 && spec.ExpectedRows != len(rowIDs) {
		proof.Status = "refuted"
		proof.Counterexamples = append(proof.Counterexamples, Counterexample{
			Code:    "row_count_mismatch",
			Message: fmt.Sprintf("expected %d row(s), found %d", spec.ExpectedRows, len(rowIDs)),
		})
	}
	for _, rowID := range rowIDs {
		row := rows[rowID]
		source, sourceOK := row[spec.SourceColumn]
		target, targetOK := row[spec.TargetColumn]
		rowProof := RowProof{
			RowID:      rowID,
			SourceHash: cellHash(source, sourceOK && spec.SourceColumn != ""),
			TargetHash: cellHash(target, targetOK),
		}
		proof.RowProofs = append(proof.RowProofs, rowProof)
		proof.RowsChecked++
		if !targetOK {
			proof.Counterexamples = append(proof.Counterexamples, Counterexample{
				RowID:      rowID,
				Code:       "target_missing",
				Message:    "backfill target column is absent",
				SourceHash: rowProof.SourceHash,
			})
			continue
		}
		if target == "" {
			proof.Counterexamples = append(proof.Counterexamples, Counterexample{
				RowID:      rowID,
				Code:       "target_empty",
				Message:    "backfill target column is empty before contract",
				SourceHash: rowProof.SourceHash,
				TargetHash: rowProof.TargetHash,
			})
			continue
		}
		if spec.SourceColumn == "" {
			continue
		}
		if !sourceOK {
			proof.Counterexamples = append(proof.Counterexamples, Counterexample{
				RowID:      rowID,
				Code:       "source_missing",
				Message:    "declared source column is absent, so target equality cannot be proven",
				TargetHash: rowProof.TargetHash,
			})
			continue
		}
		if source == "" {
			proof.Counterexamples = append(proof.Counterexamples, Counterexample{
				RowID:      rowID,
				Code:       "source_empty",
				Message:    "declared source column is empty, so target derivation cannot be proven",
				SourceHash: rowProof.SourceHash,
				TargetHash: rowProof.TargetHash,
			})
			continue
		}
		if target != source {
			proof.Counterexamples = append(proof.Counterexamples, Counterexample{
				RowID:      rowID,
				Code:       "stale_target",
				Message:    "target column does not match declared source column",
				SourceHash: rowProof.SourceHash,
				TargetHash: rowProof.TargetHash,
			})
		}
	}
	sort.SliceStable(proof.Counterexamples, func(i, j int) bool {
		if proof.Counterexamples[i].RowID != proof.Counterexamples[j].RowID {
			return proof.Counterexamples[i].RowID < proof.Counterexamples[j].RowID
		}
		return proof.Counterexamples[i].Code < proof.Counterexamples[j].Code
	})
	if len(proof.Counterexamples) > 0 && proof.Status == "checked" {
		proof.Status = "refuted"
	}
	return proof
}

func evaluateStages(spec Spec, complete bool) ([]Stage, []Obligation) {
	stageSpecs := append([]StageSpec(nil), spec.Stages...)
	sort.SliceStable(stageSpecs, func(i, j int) bool { return stageSpecs[i].ID < stageSpecs[j].ID })
	graph := map[string][]string{}
	for _, stage := range stageSpecs {
		deps := append([]string(nil), stage.DependsOn...)
		sort.Strings(deps)
		graph[stage.ID] = deps
		stage.DependsOn = deps
	}

	expandID := findStageID(stageSpecs, "expand")
	backfillID := findStageID(stageSpecs, "backfill")
	validateID := findStageID(stageSpecs, "validate")
	var obligations []Obligation
	obligations = append(obligations,
		stageOrderObligation("stage.expand_before_backfill", expandID, backfillID, graph, "backfill depends on expand"),
		stageOrderObligation("stage.backfill_before_validate", backfillID, validateID, graph, "validate depends on backfill"),
	)
	compatDeleteRequested := false
	for _, stage := range stageSpecs {
		requiresCompleteness := stage.TightensConstraint || stage.DeletesCompatibility || stage.Kind == "contract" || stage.Kind == "delete_compatibility"
		if stage.DeletesCompatibility || stage.Kind == "delete_compatibility" {
			compatDeleteRequested = true
		}
		if requiresCompleteness {
			status := "checked"
			reason := "stage is gated by validate, which is gated by backfill completeness"
			if validateID == "" || !dependsOn(stage.ID, validateID, graph, map[string]bool{}) {
				status = "refuted"
				reason = "stage that tightens constraints or deletes compatibility code must depend on validate"
			}
			obligations = append(obligations, Obligation{
				ID:      "stage." + stage.ID + ".after_validate",
				Status:  status,
				Formula: fmt.Sprintf("%s depends_on %s", stage.ID, firstNonEmpty(validateID, "validate")),
				Reason:  reason,
			})
		}
	}
	if compatDeleteRequested && len(spec.CompatibilityCodeRefs) == 0 {
		obligations = append(obligations, Obligation{
			ID:      "compatibility.refs",
			Status:  "refuted",
			Formula: "compatibility_code_refs non-empty when compatibility deletion is planned",
			Reason:  "compatibility deletion needs owner-reviewable code references",
		})
	} else {
		obligations = append(obligations, Obligation{
			ID:      "compatibility.refs",
			Status:  "checked",
			Formula: "compatibility code references are recorded or no deletion is requested",
			Reason:  "planner can identify what compatibility code is safe to remove after validation",
		})
	}

	var stages []Stage
	for _, specStage := range stageSpecs {
		requiresCompleteness := specStage.TightensConstraint || specStage.DeletesCompatibility || specStage.Kind == "contract" || specStage.Kind == "delete_compatibility"
		stage := Stage{
			ID:                   specStage.ID,
			Kind:                 specStage.Kind,
			DependsOn:            specStage.DependsOn,
			Command:              specStage.Command,
			TightensConstraint:   specStage.TightensConstraint,
			DeletesCompatibility: specStage.DeletesCompatibility,
			RequiresCompleteness: requiresCompleteness,
			Ready:                true,
		}
		if requiresCompleteness && !complete {
			stage.Ready = false
			stage.BlockReason = "backfill completeness proof is not checked"
		} else if requiresCompleteness && (validateID == "" || !dependsOn(stage.ID, validateID, graph, map[string]bool{})) {
			stage.Ready = false
			stage.BlockReason = "stage does not depend on validate"
		}
		stages = append(stages, stage)
	}
	return stages, obligations
}

func stageOrderObligation(id, earlier, later string, graph map[string][]string, reason string) Obligation {
	status := "checked"
	formula := fmt.Sprintf("%s depends_on %s", firstNonEmpty(later, "<missing>"), firstNonEmpty(earlier, "<missing>"))
	if earlier == "" || later == "" || !dependsOn(later, earlier, graph, map[string]bool{}) {
		status = "refuted"
	}
	return Obligation{ID: id, Status: status, Formula: formula, Reason: reason}
}

func scopeObligation(spec Spec) Obligation {
	return Obligation{
		ID:      "scope.columns",
		Status:  "checked",
		Formula: fmt.Sprintf("table=%s target=%s source=%s primary_key=%s", spec.Table, spec.TargetColumn, firstNonEmpty(spec.SourceColumn, "<none>"), spec.PrimaryKey),
		Reason:  "all generated SQL and proof obligations are derived from one declared scope",
	}
}

func completenessObligation(spec Spec, proof CompletenessProof) Obligation {
	return Obligation{
		ID:      "backfill.completeness",
		Status:  proof.Status,
		Formula: fmt.Sprintf("forall row in %s: %s populated%s before contract", spec.Table, spec.TargetColumn, sourceFormulaSuffix(spec)),
		Reason:  fmt.Sprintf("%d row(s) checked with %d counterexample(s)", proof.RowsChecked, len(proof.Counterexamples)),
	}
}

func sourceFormulaSuffix(spec Spec) string {
	if spec.SourceColumn == "" {
		return ""
	}
	return " and equals " + spec.SourceColumn
}

func findStageID(stages []StageSpec, kind string) string {
	for _, stage := range stages {
		if stage.ID == kind || stage.Kind == kind {
			return stage.ID
		}
	}
	return ""
}

func dependsOn(stageID, dependencyID string, graph map[string][]string, seen map[string]bool) bool {
	if stageID == dependencyID {
		return true
	}
	if seen[stageID] {
		return false
	}
	seen[stageID] = true
	for _, dep := range graph[stageID] {
		if dep == dependencyID || dependsOn(dep, dependencyID, graph, seen) {
			return true
		}
	}
	return false
}

func buildSQL(spec Spec) []SQLStatement {
	table := quoteIdent(spec.Table)
	target := quoteIdent(spec.TargetColumn)
	source := quoteIdent(spec.SourceColumn)
	primary := quoteIdent(spec.PrimaryKey)
	sql := []SQLStatement{}
	if spec.SourceColumn != "" {
		sql = append(sql, SQLStatement{
			StageID: "backfill",
			Purpose: "populate missing target values from the declared compatibility source",
			SQL:     fmt.Sprintf("UPDATE %s SET %s = %s WHERE %s IS NULL OR %s = '';", table, target, source, target, target),
		})
	}
	sql = append(sql, SQLStatement{
		StageID: "validate",
		Purpose: "find rows that would fail NOT NULL after backfill",
		SQL:     fmt.Sprintf("SELECT %s FROM %s WHERE %s IS NULL OR %s = '';", primary, table, target, target),
	})
	if spec.SourceColumn != "" {
		sql = append(sql, SQLStatement{
			StageID: "validate",
			Purpose: "find rows whose target is stale relative to the declared source",
			SQL:     fmt.Sprintf("SELECT %s FROM %s WHERE %s IS NOT NULL AND %s <> '' AND %s <> %s;", primary, table, source, source, target, source),
		})
	}
	sql = append(sql, SQLStatement{
		StageID: "contract",
		Purpose: "tighten the target column only after backfill-plan proof is checked",
		SQL:     fmt.Sprintf("ALTER TABLE %s ALTER COLUMN %s SET NOT NULL;", table, target),
	})
	return sql
}

func defaultStages() []StageSpec {
	return []StageSpec{{
		ID: "expand", Kind: "expand", Command: "add nullable target column and keep compatibility writers",
	}, {
		ID: "backfill", Kind: "backfill", DependsOn: []string{"expand"}, Command: "copy source values into target in bounded batches",
	}, {
		ID: "validate", Kind: "validate", DependsOn: []string{"backfill"}, Command: "run generated validation SQL and this planner proof",
	}, {
		ID: "contract", Kind: "contract", DependsOn: []string{"validate"}, TightensConstraint: true, Command: "tighten target constraint after validation",
	}}
}

func obligationsChecked(obligations []Obligation) bool {
	for _, obligation := range obligations {
		if obligation.Status != "checked" {
			return false
		}
	}
	return true
}

func reportHash(report Report) string {
	report.Hash = ""
	return canonical.Hash(report)
}

func cellHash(value string, ok bool) string {
	if !ok {
		return ""
	}
	return canonical.Hash(struct {
		Present bool   `json:"present"`
		Value   string `json:"value"`
	}{true, value})
}

func sortedMapKeys(rows map[string]replay.Row) []string {
	keys := make([]string, 0, len(rows))
	for key := range rows {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func sortedStrings(values []string) []string {
	out := append([]string(nil), values...)
	sort.Strings(out)
	return out
}

func quoteIdent(value string) string {
	parts := strings.Split(value, ".")
	for i, part := range parts {
		parts[i] = `"` + strings.ReplaceAll(part, `"`, `""`) + `"`
	}
	return strings.Join(parts, ".")
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
