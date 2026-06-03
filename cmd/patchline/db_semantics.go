package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/thehalleyyoung/patchline/internal/dbsemantics"
)

func dbSemanticsCommand(args []string) error {
	engineValue, ok := flagValue(args, "--engine")
	if !ok || strings.TrimSpace(engineValue) == "" {
		return errors.New("usage: patchline db-semantics --engine postgres|mysql|sqlite|sqlserver|oracle|bigquery|snowflake|clickhouse [--version version] --sql <sql-or-path> [--table-hints hints.json] [--out report.json] [--json]")
	}
	sqlValue, ok := flagValue(args, "--sql")
	if !ok || strings.TrimSpace(sqlValue) == "" {
		return errors.New("usage: patchline db-semantics --engine postgres|mysql|sqlite|sqlserver|oracle|bigquery|snowflake|clickhouse [--version version] --sql <sql-or-path> [--table-hints hints.json] [--out report.json] [--json]")
	}
	version, _ := flagValue(args, "--version")
	source, content, err := readDBSemanticsSQL(sqlValue)
	if err != nil {
		return err
	}
	options := dbsemantics.AnalysisOptions{}
	if hintsPath, ok := flagValue(args, "--table-hints"); ok {
		if strings.TrimSpace(hintsPath) == "" {
			return errors.New("--table-hints requires a JSON path")
		}
		hintContent, err := os.ReadFile(hintsPath)
		if err != nil {
			return fmt.Errorf("read runtime table hints %q: %w", hintsPath, err)
		}
		hints, err := dbsemantics.ParseRuntimeHints(filepath.ToSlash(hintsPath), hintContent)
		if err != nil {
			return err
		}
		options.RuntimeHints = hints
	}
	report, err := dbsemantics.EvaluateWithOptions(dbsemantics.Engine(engineValue), version, source, content, options)
	if err != nil {
		return err
	}
	if outPath, ok := flagValue(args, "--out"); ok && outPath != "" {
		if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
			return err
		}
		if err := writeJSONArtifact(outPath, report); err != nil {
			return err
		}
	}
	if hasFlag(args, "--json") {
		return writeJSON(os.Stdout, report)
	}
	fmt.Printf("db semantics engine=%s version=%s statements=%d high=%d medium=%d low=%d hash=%s\n",
		report.Profile.Engine,
		report.Profile.ResolvedVersion,
		report.Summary.Statements,
		report.Summary.HighRisk,
		report.Summary.MediumRisk,
		report.Summary.LowRisk,
		report.Hash,
	)
	for _, statement := range report.Statements {
		fmt.Printf("  [%d] %s table=%s risk=%s rules=%d\n", statement.Index, statement.Kind, nonEmpty(statement.Table, "<unknown>"), statement.Risk, len(statement.Rules))
		fmt.Printf("      lock mode=%s duration=%s readers_blocked=%t writers_blocked=%t ddl_blocked=%t\n",
			statement.Lock.Mode,
			statement.Lock.DurationClass,
			statement.Lock.BlocksReaders,
			statement.Lock.BlocksWriters,
			statement.Lock.BlocksDDL,
		)
		if rollback := statement.RollbackFeasibility; rollback != nil {
			fmt.Printf("      rollback class=%s status=%s feasible=%t transactional=%t implicit_commit=%t irreversible_metadata=%t recovery=%s\n",
				rollback.Class,
				rollback.Status,
				rollback.Feasible,
				rollback.TransactionalRollback,
				rollback.ImplicitCommit,
				rollback.IrreversibleMetadata,
				rollback.RecoveryMechanism,
			)
		}
		if queryPlan := statement.QueryPlanRegression; queryPlan != nil {
			fmt.Printf("      query-plan class=%s change=%s workloads=%d regressions=%d\n",
				queryPlan.Class,
				queryPlan.ChangeKind,
				len(queryPlan.RepresentativeWorkloads),
				len(queryPlan.Regressions),
			)
		}
		if runtime := statement.RuntimeEstimate; runtime != nil {
			fmt.Printf("      runtime class=%s duration=%s rows=%d bytes=%d source=%s\n",
				runtime.Class,
				runtime.EstimatedDurationClass,
				runtime.RowsUpperBound,
				runtime.BytesUpperBound,
				runtime.SourceKind,
			)
		}
		for _, control := range statement.NegativeControls {
			fmt.Printf("      negative-control %s: %s %s -> %s %s rule=%s risk=%s verdict=%s\n",
				control.ID,
				control.CurrentEngine,
				control.CurrentVersion,
				control.ControlEngine,
				control.ControlVersion,
				control.ControlRule,
				control.ControlRisk,
				control.ControlVerdict,
			)
		}
		for _, rule := range statement.Rules {
			fmt.Printf("      - %s %s: %s\n", rule.Severity, rule.ID, rule.Evidence)
		}
	}
	return nil
}

func readDBSemanticsSQL(value string) (string, []byte, error) {
	if content, err := os.ReadFile(value); err == nil {
		return filepath.ToSlash(value), content, nil
	}
	if strings.Contains(value, ";") || strings.ContainsAny(value, "\n\t ") {
		return "<inline-sql>", []byte(value), nil
	}
	return "", nil, fmt.Errorf("sql path %q does not exist; pass inline SQL containing whitespace or semicolon", value)
}

func nonEmpty(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
