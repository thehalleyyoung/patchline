package replay

import (
	"fmt"

	"github.com/thehalleyyoung/patchline/internal/canonical"
	"github.com/thehalleyyoung/patchline/internal/repair"
)

type RollbackPlan struct {
	Version    string                `json:"version"`
	Manifest   string                `json:"manifest"`
	Incident   string                `json:"incident"`
	Statements []repair.SQLStatement `json:"statements"`
	Hash       string                `json:"hash"`
}

func GenerateRollbackPlan(report Report) (RollbackPlan, error) {
	plan := RollbackPlan{
		Version:  "patchline.rollback-plan/v1",
		Manifest: report.Manifest,
		Incident: report.Incident,
	}
	for _, operation := range report.Operations {
		for _, diff := range operation.Diffs {
			if len(diff.Changes) == 0 {
				continue
			}
			sql, kind, err := rollbackStatement(diff)
			if err != nil {
				return RollbackPlan{}, fmt.Errorf("operation %s row %s: %w", operation.OperationID, diff.ID, err)
			}
			plan.Statements = append(plan.Statements, repair.SQLStatement{
				OperationID: operation.OperationID,
				Kind:        kind,
				SQL:         sql,
			})
		}
	}
	plan.Hash = canonical.Hash(struct {
		Version    string                `json:"version"`
		Manifest   string                `json:"manifest"`
		Incident   string                `json:"incident"`
		Statements []repair.SQLStatement `json:"statements"`
	}{plan.Version, plan.Manifest, plan.Incident, plan.Statements})
	return plan, nil
}

func rollbackStatement(diff RowDiff) (string, string, error) {
	if isInsertedDiff(diff) {
		sql, err := repair.DeleteStatement(diff.Table, map[string]string{"id": diff.ID})
		return sql, "rollback-insert", err
	}
	if isDeletedDiff(diff) {
		restore := map[string]string{"id": diff.ID}
		for column, change := range diff.Changes {
			restore[column] = change.Before
		}
		sql, err := repair.InsertStatement(diff.Table, restore)
		return sql, "rollback-delete", err
	}
	restore := map[string]string{}
	for column, change := range diff.Changes {
		restore[column] = change.Before
	}
	sql, err := repair.UpdateStatement(diff.Table, restore, map[string]string{"id": diff.ID})
	return sql, "rollback-update", err
}

func isInsertedDiff(diff RowDiff) bool {
	return diff.BeforeHash == hashRow(Row{}) && diff.AfterHash != hashRow(Row{})
}

func isDeletedDiff(diff RowDiff) bool {
	return diff.BeforeHash != hashRow(Row{}) && diff.AfterHash == hashRow(Row{})
}
