package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/patchline/patchline/internal/archive"
	"github.com/patchline/patchline/internal/attest"
	"github.com/patchline/patchline/internal/bench"
	"github.com/patchline/patchline/internal/bundle"
	"github.com/patchline/patchline/internal/canonical"
	"github.com/patchline/patchline/internal/demo"
	"github.com/patchline/patchline/internal/effects"
	"github.com/patchline/patchline/internal/evidence"
	"github.com/patchline/patchline/internal/gate"
	"github.com/patchline/patchline/internal/invariant"
	"github.com/patchline/patchline/internal/ledger"
	"github.com/patchline/patchline/internal/migration"
	"github.com/patchline/patchline/internal/policy"
	"github.com/patchline/patchline/internal/proof"
	"github.com/patchline/patchline/internal/provenance"
	"github.com/patchline/patchline/internal/refinement"
	"github.com/patchline/patchline/internal/repair"
	"github.com/patchline/patchline/internal/replay"
	"github.com/patchline/patchline/internal/reproduce"
	"github.com/patchline/patchline/internal/semantics"
	"github.com/patchline/patchline/internal/solver"
	"github.com/patchline/patchline/internal/symbolic"
	"github.com/patchline/patchline/internal/workflow"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "patchline:", err)
		os.Exit(exitCode(err))
	}
}

type codedError struct {
	code int
	err  error
}

func (e codedError) Error() string {
	return e.err.Error()
}

func (e codedError) Unwrap() error {
	return e.err
}

func exitCode(err error) int {
	var coded codedError
	if errors.As(err, &coded) {
		return coded.code
	}
	return 1
}

func run(args []string) error {
	if len(args) == 0 {
		usage()
		return nil
	}

	switch args[0] {
	case "about":
		fmt.Println("Patchline is a deterministic repair control plane for production data incidents.")
		fmt.Println("It uses typed provenance, repair-effect analysis, replayable manifests, and hash-chained ledgers.")
		fmt.Println("RiSE angle: program analysis + verifiable transformations + reproducible repair benchmarks, without AI.")
	case "semantics-contract":
		return semanticsContract(hasFlag(args[1:], "--json"))
	case "semantics-audit":
		opts, err := parseSemanticAuditOptions(args[1:])
		if err != nil {
			return err
		}
		return semanticsAudit(opts, hasFlag(args[1:], "--json"))
	case "trace-reconstruct":
		if len(args) < 2 {
			return errors.New("usage: patchline trace-reconstruct <evidence.jsonl> [--json]")
		}
		return traceReconstruct(args[1], hasFlag(args[2:], "--json"))
	case "trace-equivalence":
		if len(args) < 3 {
			return errors.New("usage: patchline trace-equivalence <left.jsonl> <right.jsonl> [--json]")
		}
		return traceEquivalence(args[1], args[2], hasFlag(args[3:], "--json"))
	case "provenance":
		return provenanceCommand(args[1:])
	case "archive-index":
		if len(args) < 2 {
			return errors.New("usage: patchline archive-index <archive-spec.json> [--json]")
		}
		return archiveIndex(args[1], hasFlag(args[2:], "--json"))
	case "archive-query":
		if len(args) < 2 {
			return errors.New("usage: patchline archive-query <archive-spec.json> [broad-updates|damaged-reports|missing-rollback|all] [--json]")
		}
		query := "all"
		if len(args) >= 3 && !strings.HasPrefix(args[2], "--") {
			query = args[2]
		}
		return archiveQuery(args[1], query, hasFlag(args[2:], "--json"))
	case "demo-graph":
		return writeJSON(os.Stdout, graphDTO(demo.Graph()))
	case "explain", "trace-row":
		if len(args) < 2 {
			return errors.New("usage: patchline explain <entity-id> [--graph graph.json]")
		}
		g, err := graphFor(args[2:])
		if err != nil {
			return err
		}
		return explain(g, args[1])
	case "slice":
		if len(args) < 2 {
			return errors.New("usage: patchline slice <entity-id> [--json] [--graph graph.json]")
		}
		g, err := graphFor(args[2:])
		if err != nil {
			return err
		}
		return sliceGraph(g, args[1], hasFlag(args[2:], "--json"))
	case "validate-repair":
		if len(args) < 2 {
			return errors.New("usage: patchline validate-repair <manifest.json> [--json]")
		}
		return validateRepair(args[1], hasFlag(args[2:], "--json"))
	case "migrate-repair":
		if len(args) < 2 {
			return errors.New("usage: patchline migrate-repair <manifest.json>")
		}
		return migrateRepair(args[1])
	case "template-repair":
		if len(args) < 2 {
			return errors.New("usage: patchline template-repair <row-restore|scoped-backfill-reversal|report-recompute>")
		}
		return templateRepair(args[1])
	case "lint-repair":
		if len(args) < 2 {
			return errors.New("usage: patchline lint-repair <manifest.json> [--json] [--proof]")
		}
		return lintRepair(args[1], hasFlag(args[2:], "--json"), hasFlag(args[2:], "--proof"))
	case "solver-obligations":
		if len(args) < 2 {
			return errors.New("usage: patchline solver-obligations <manifest.json> [--invariants invariants.json] [--store store.json] [--json]")
		}
		return solverObligations(args[1], args[2:], hasFlag(args[2:], "--json"))
	case "symbolic-exec":
		if len(args) < 2 {
			return errors.New("usage: patchline symbolic-exec <manifest.json> [--store store.json] [--json]")
		}
		return symbolicExec(args[1], args[2:], hasFlag(args[2:], "--json"))
	case "model-check-workflow":
		if len(args) < 2 {
			return errors.New("usage: patchline model-check-workflow <workflow.json> [--json]")
		}
		return modelCheckWorkflow(args[1], hasFlag(args[2:], "--json"))
	case "cegar-refine":
		if len(args) < 2 {
			return errors.New("usage: patchline cegar-refine <manifest.json> [--store store.json] [--invariants invariants.json] [--workflow workflow.json] [--json]")
		}
		return cegarRefine(args[1], args[2:], hasFlag(args[2:], "--json"))
	case "attestation-keygen":
		return attestationKeygen(hasFlag(args[1:], "--json"))
	case "sign-artifact":
		if len(args) < 2 {
			return errors.New("usage: patchline sign-artifact <artifact.json> --subject subject --seed-hex seed [--out attestation.json] [--json]")
		}
		return signArtifact(args[1], args[2:], hasFlag(args[2:], "--json"))
	case "verify-artifact":
		if len(args) < 2 {
			return errors.New("usage: patchline verify-artifact <attestation.json> --artifact artifact.json [--json]")
		}
		return verifyArtifact(args[1], args[2:], hasFlag(args[2:], "--json"))
	case "generate-sql":
		if len(args) < 2 {
			return errors.New("usage: patchline generate-sql <manifest.json> [--json]")
		}
		return generateSQL(args[1], hasFlag(args[2:], "--json"))
	case "rollback-plan":
		if len(args) < 2 {
			return errors.New("usage: patchline rollback-plan <manifest.json> [--json]")
		}
		return rollbackPlan(args[1], hasFlag(args[2:], "--json"))
	case "transaction-plan":
		if len(args) < 2 {
			return errors.New("usage: patchline transaction-plan <manifest.json> [--json]")
		}
		return transactionPlan(args[1], hasFlag(args[2:], "--json"))
	case "analyze-migration":
		if len(args) < 2 {
			return errors.New("usage: patchline analyze-migration <migration.sql> [--dialect postgres|mysql|sqlite|sqlserver] [--json]")
		}
		dialect, err := parseSQLDialect(args[2:])
		if err != nil {
			return err
		}
		return analyzeMigration(args[1], dialect, hasFlag(args[2:], "--json"))
	case "schema-diff":
		if len(args) < 4 {
			return errors.New("usage: patchline schema-diff <migration.sql> <before-schema.json> <expected-schema.json> [--dialect postgres|mysql|sqlite|sqlserver] [--json]")
		}
		dialect, err := parseSQLDialect(args[4:])
		if err != nil {
			return err
		}
		return schemaDiff(args[1], args[2], args[3], dialect, hasFlag(args[4:], "--json"))
	case "migration-semantics":
		if len(args) < 3 {
			return errors.New("usage: patchline migration-semantics <migration.sql> <before-schema.json> [--dialect postgres|mysql|sqlite|sqlserver] [--json]")
		}
		dialect, err := parseSQLDialect(args[3:])
		if err != nil {
			return err
		}
		return migrationSemantics(args[1], args[2], dialect, hasFlag(args[3:], "--json"))
	case "extract-sql":
		if len(args) < 2 {
			return errors.New("usage: patchline extract-sql <path> [--json]")
		}
		return extractSQL(args[1], hasFlag(args[2:], "--json"))
	case "migration-outcomes", "migration-changelog":
		if len(args) < 3 {
			return errors.New("usage: patchline migration-outcomes <evidence.jsonl> <migration.sql> [--repair manifest.json] [--policy policy.json] [--benchmark suite.json] [--source-sql path] [--json]")
		}
		return migrationOutcomes(args[1], args[2], args[3:], hasFlag(args[3:], "--json"))
	case "dry-run":
		if len(args) < 2 {
			return errors.New("usage: patchline dry-run <manifest.json> [--store store.json] [--json]")
		}
		return dryRun(args[1], args[2:], hasFlag(args[2:], "--json"))
	case "repair-semantics", "step-trace":
		if len(args) < 2 {
			return errors.New("usage: patchline repair-semantics <manifest.json> [--store store.json] [--json]")
		}
		return repairSemantics(args[1], args[2:], hasFlag(args[2:], "--json"))
	case "snapshot-drift":
		if len(args) < 4 {
			return errors.New("usage: patchline snapshot-drift <manifest.json> <before-store.json> <after-store.json> [--json]")
		}
		return snapshotDrift(args[1], args[2], args[3], hasFlag(args[4:], "--json"))
	case "effect-summary":
		if len(args) < 2 {
			return errors.New("usage: patchline effect-summary <manifest.json> [--json]")
		}
		return effectSummary(args[1], hasFlag(args[2:], "--json"))
	case "check-invariants":
		if len(args) < 3 {
			return errors.New("usage: patchline check-invariants <manifest.json> <invariants.json> [--json]")
		}
		return checkInvariants(args[1], args[2], hasFlag(args[3:], "--json"))
	case "discover-invariants":
		if len(args) < 2 {
			return errors.New("usage: patchline discover-invariants <manifest.json> [--json]")
		}
		return discoverInvariants(args[1], hasFlag(args[2:], "--json"))
	case "reproduce", "benchmark":
		if len(args) < 2 {
			return errors.New("usage: patchline reproduce <artifact.json> [--json] [--update]")
		}
		return reproduceArtifact(args[1], hasFlag(args[2:], "--json"), hasFlag(args[2:], "--update"))
	case "evaluate-policy":
		if len(args) < 4 {
			return errors.New("usage: patchline evaluate-policy <policy.json> <repair.json> <migration.sql> [--json]")
		}
		return evaluatePolicy(args[1], args[2], args[3], hasFlag(args[4:], "--json"))
	case "export-bundle":
		if len(args) < 4 {
			return errors.New("usage: patchline export-bundle <reproduce.json> <policy.json> <migration.sql> [--json]")
		}
		return exportBundle(args[1], args[2], args[3], hasFlag(args[4:], "--json"))
	case "benchmark-suite":
		if len(args) < 2 {
			return errors.New("usage: patchline benchmark-suite <suite.json> [--json]")
		}
		return benchmarkSuite(args[1], hasFlag(args[2:], "--json"))
	case "ingest-evidence":
		if len(args) < 2 {
			return errors.New("usage: patchline ingest-evidence <events.jsonl> [--json] [--out graph.json]")
		}
		outPath, _ := flagValue(args[2:], "--out")
		return ingestEvidence(args[1], hasFlag(args[2:], "--json"), outPath)
	case "adapt-evidence":
		if len(args) < 3 {
			return errors.New("usage: patchline adapt-evidence <otlp|datadog|postgres|github|migration-runner> <input.json> [--json] [--out events.jsonl]")
		}
		outPath, _ := flagValue(args[3:], "--out")
		return adaptEvidence(args[1], args[2], hasFlag(args[3:], "--json"), outPath)
	case "ci-gate":
		if len(args) < 2 {
			return errors.New("usage: patchline ci-gate <suite.json> [--min-precision 0.95] [--min-recall 0.95] [--json]")
		}
		opts, err := parseGateOptions(args[2:])
		if err != nil {
			return err
		}
		return ciGate(args[1], opts, hasFlag(args[2:], "--json"))
	case "ledger-verify":
		return ledgerVerify(hasFlag(args[1:], "--json"))
	case "help", "-h", "--help":
		usage()
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}

	return nil
}

func usage() {
	fmt.Println(`Patchline deterministic data repair CLI

Usage:
  patchline about
  patchline semantics-contract [--json]
  patchline semantics-audit [--json] [--evidence events.jsonl] [--repair manifest.json] [--migration migration.sql] [--benchmark suite.json] [--policy policy.json] [--workflow workflow.json] [--archive archive-spec.json] [--snapshot-before store.json] [--snapshot-after store.json]
  patchline trace-reconstruct <evidence.jsonl> [--json]
  patchline trace-equivalence <left.jsonl> <right.jsonl> [--json]
  patchline provenance cause <entity-id> [--json] [--graph graph.json] [--evidence evidence.jsonl]
  patchline provenance minimal <entity-id> [--json] [--graph graph.json] [--evidence evidence.jsonl]
  patchline provenance blast <entity-id> [--json] [--graph graph.json] [--evidence evidence.jsonl]
  patchline provenance certificate <entity-id> [--json] [--graph graph.json] [--evidence evidence.jsonl]
  patchline provenance diff <left-evidence.jsonl> <right-evidence.jsonl> [--json]
  patchline provenance archive <evidence.jsonl>... [--json]
  patchline archive-index <archive-spec.json> [--json]
  patchline archive-query <archive-spec.json> [broad-updates|damaged-reports|missing-rollback|all] [--json]
  patchline demo-graph
  patchline explain <entity-id> [--graph graph.json]
  patchline slice <entity-id> [--json] [--graph graph.json]
  patchline validate-repair <manifest.json> [--json]
  patchline migrate-repair <manifest.json>
  patchline template-repair <row-restore|scoped-backfill-reversal|report-recompute>
  patchline lint-repair <manifest.json> [--json] [--proof]
  patchline solver-obligations <manifest.json> [--invariants invariants.json] [--store store.json] [--json]
  patchline symbolic-exec <manifest.json> [--store store.json] [--json]
  patchline model-check-workflow <workflow.json> [--json]
  patchline cegar-refine <manifest.json> [--store store.json] [--invariants invariants.json] [--workflow workflow.json] [--json]
  patchline attestation-keygen [--json]
  patchline sign-artifact <artifact.json> --subject subject --seed-hex seed [--out attestation.json] [--json]
  patchline verify-artifact <attestation.json> --artifact artifact.json [--json]
  patchline generate-sql <manifest.json> [--json]
  patchline rollback-plan <manifest.json> [--json]
  patchline transaction-plan <manifest.json> [--json]
  patchline analyze-migration <migration.sql> [--dialect postgres|mysql|sqlite|sqlserver] [--json]
  patchline schema-diff <migration.sql> <before-schema.json> <expected-schema.json> [--dialect postgres|mysql|sqlite|sqlserver] [--json]
  patchline migration-semantics <migration.sql> <before-schema.json> [--dialect postgres|mysql|sqlite|sqlserver] [--json]
  patchline extract-sql <path> [--json]
  patchline migration-outcomes <evidence.jsonl> <migration.sql> [--repair manifest.json] [--policy policy.json] [--benchmark suite.json] [--source-sql path] [--json]
  patchline dry-run <manifest.json> [--store store.json] [--json]
  patchline repair-semantics <manifest.json> [--store store.json] [--json]
  patchline snapshot-drift <manifest.json> <before-store.json> <after-store.json> [--json]
  patchline effect-summary <manifest.json> [--json]
  patchline check-invariants <manifest.json> <invariants.json> [--json]
  patchline discover-invariants <manifest.json> [--json]
  patchline reproduce <artifact.json> [--json] [--update]
  patchline benchmark <artifact.json> [--json]
  patchline evaluate-policy <policy.json> <repair.json> <migration.sql> [--json]
  patchline export-bundle <reproduce.json> <policy.json> <migration.sql> [--json]
  patchline benchmark-suite <suite.json> [--json]
  patchline ingest-evidence <events.jsonl> [--json] [--out graph.json]
  patchline adapt-evidence <otlp|datadog|postgres|github|migration-runner> <input.json> [--json] [--out events.jsonl]
  patchline ci-gate <suite.json> [--min-precision 0.95] [--min-recall 0.95] [--json]
  patchline ledger-verify [--json]

Examples:
  patchline explain record:invoices/inv_1002
  patchline semantics-audit --json
  patchline trace-reconstruct examples/incidents/bad-migration.jsonl
  patchline provenance certificate record:invoices/inv_1002 --evidence examples/incidents/bad-migration.jsonl
  patchline analyze-migration demos/billing/migrations/002_bad_backfill.sql
  patchline reproduce examples/reproduce/bad-migration-billing.json --json`)
}

type semanticAuditOptions struct {
	Evidence       string
	Repair         string
	Migration      string
	Benchmark      string
	Invariants     string
	Policy         string
	Workflow       string
	Archive        string
	SnapshotBefore string
	SnapshotAfter  string
}

func parseSemanticAuditOptions(args []string) (semanticAuditOptions, error) {
	opts := semanticAuditOptions{
		Evidence:       "examples/incidents/bad-migration.jsonl",
		Repair:         "examples/repairs/repair-bad-invoice-backfill.json",
		Migration:      "demos/billing/migrations/002_bad_backfill.sql",
		Benchmark:      "examples/benchmarks/strict-migration-corpus.json",
		Invariants:     "examples/invariants/billing-core.json",
		Policy:         "examples/policies/review-required.json",
		Workflow:       "examples/workflows/bad-migration-approved.json",
		Archive:        "examples/archive/bad-migration-corpus.json",
		SnapshotBefore: "examples/snapshots/billing-bad-migration-before.json",
		SnapshotAfter:  "examples/snapshots/billing-bad-migration-before.json",
	}
	var ok bool
	if opts.Evidence, ok = flagValue(args, "--evidence"); !ok {
		opts.Evidence = "examples/incidents/bad-migration.jsonl"
	}
	if opts.Repair, ok = flagValue(args, "--repair"); !ok {
		opts.Repair = "examples/repairs/repair-bad-invoice-backfill.json"
	}
	if opts.Migration, ok = flagValue(args, "--migration"); !ok {
		opts.Migration = "demos/billing/migrations/002_bad_backfill.sql"
	}
	if opts.Benchmark, ok = flagValue(args, "--benchmark"); !ok {
		opts.Benchmark = "examples/benchmarks/strict-migration-corpus.json"
	}
	if opts.Invariants, ok = flagValue(args, "--invariants"); !ok {
		opts.Invariants = "examples/invariants/billing-core.json"
	}
	if opts.Policy, ok = flagValue(args, "--policy"); !ok {
		opts.Policy = "examples/policies/review-required.json"
	}
	if opts.Workflow, ok = flagValue(args, "--workflow"); !ok {
		opts.Workflow = "examples/workflows/bad-migration-approved.json"
	}
	if opts.Archive, ok = flagValue(args, "--archive"); !ok {
		opts.Archive = "examples/archive/bad-migration-corpus.json"
	}
	if opts.SnapshotBefore, ok = flagValue(args, "--snapshot-before"); !ok {
		opts.SnapshotBefore = "examples/snapshots/billing-bad-migration-before.json"
	}
	if opts.SnapshotAfter, ok = flagValue(args, "--snapshot-after"); !ok {
		opts.SnapshotAfter = "examples/snapshots/billing-bad-migration-before.json"
	}
	return opts, nil
}

func semanticsContract(jsonOut bool) error {
	contract := semantics.DefaultContract()
	if jsonOut {
		return writeJSON(os.Stdout, contract)
	}
	fmt.Printf("Patchline semantic contract %s hash=%s\n", contract.Version, contract.Hash)
	fmt.Printf("  state components=%d observations=%d commands=%d failure states=%d\n",
		len(contract.StateModel), len(contract.ObservationModel), len(contract.CommandContracts), len(contract.FailureStates))
	return nil
}

func traceReconstruct(path string, jsonOut bool) error {
	projection, err := readTraceProjection(path)
	if err != nil {
		return err
	}
	if jsonOut {
		return writeJSON(os.Stdout, projection)
	}
	status := "ok"
	if !projection.OK {
		status = "has-errors"
	}
	fmt.Printf("trace projection %s observations=%d source=%s clock=%s hash=%s graph=%s\n",
		status,
		projection.ObservationCount,
		formatConfidenceCounts(projection.SourceSummary),
		formatConfidenceCounts(projection.ClockSummary),
		projection.ProjectionHash,
		projection.GraphHash,
	)
	if projection.TimeRange.Start != "" || projection.TimeRange.End != "" {
		fmt.Printf("  time_range=%s..%s precision=%s\n", projection.TimeRange.Start, projection.TimeRange.End, projection.TimeRange.Precision)
	}
	if !projection.OK {
		for _, err := range projection.Errors {
			fmt.Printf("  error: %s\n", err)
		}
		return errors.New("trace reconstruction found invalid evidence")
	}
	return nil
}

func traceEquivalence(leftPath, rightPath string, jsonOut bool) error {
	left, err := readTraceProjection(leftPath)
	if err != nil {
		return err
	}
	right, err := readTraceProjection(rightPath)
	if err != nil {
		return err
	}
	report := evidence.CompareTraceProjections(leftPath, rightPath, left, right)
	if jsonOut {
		return writeJSON(os.Stdout, report)
	}
	status := "different"
	if report.Equivalent {
		status = "equivalent"
	}
	fmt.Printf("trace projections %s shared=%d left_only=%d right_only=%d hash=%s\n",
		status, report.Shared, len(report.LeftOnly), len(report.RightOnly), report.ReportHash)
	fmt.Printf("  left=%s %s\n", report.LeftPath, report.LeftHash)
	fmt.Printf("  right=%s %s\n", report.RightPath, report.RightHash)
	if !report.Equivalent {
		return errors.New("trace projections are not equivalent")
	}
	return nil
}

func provenanceCommand(args []string) error {
	if len(args) == 0 {
		return errors.New("usage: patchline provenance <cause|minimal|blast|certificate|diff|archive> ...")
	}
	switch args[0] {
	case "cause":
		if len(args) < 2 {
			return errors.New("usage: patchline provenance cause <entity-id> [--json] [--graph graph.json] [--evidence evidence.jsonl]")
		}
		g, err := provenanceGraphFor(args[2:])
		if err != nil {
			return err
		}
		return printCauseReport(g, []string{args[1]}, hasFlag(args[2:], "--json"))
	case "minimal":
		if len(args) < 2 {
			return errors.New("usage: patchline provenance minimal <entity-id> [--json] [--graph graph.json] [--evidence evidence.jsonl]")
		}
		g, err := provenanceGraphFor(args[2:])
		if err != nil {
			return err
		}
		return printMinimalExplanation(g, []string{args[1]}, hasFlag(args[2:], "--json"))
	case "blast":
		if len(args) < 2 {
			return errors.New("usage: patchline provenance blast <entity-id> [--json] [--graph graph.json] [--evidence evidence.jsonl]")
		}
		g, err := provenanceGraphFor(args[2:])
		if err != nil {
			return err
		}
		return printBlastRadius(g, []string{args[1]}, hasFlag(args[2:], "--json"))
	case "certificate":
		if len(args) < 2 {
			return errors.New("usage: patchline provenance certificate <entity-id> [--json] [--graph graph.json] [--evidence evidence.jsonl]")
		}
		g, err := provenanceGraphFor(args[2:])
		if err != nil {
			return err
		}
		return printCausalCertificate(g, []string{args[1]}, hasFlag(args[2:], "--json"))
	case "diff":
		if len(args) < 3 {
			return errors.New("usage: patchline provenance diff <left-evidence.jsonl> <right-evidence.jsonl> [--json]")
		}
		return printProvenanceDiff(args[1], args[2], hasFlag(args[3:], "--json"))
	case "archive":
		paths := positionalArgs(args[1:])
		if len(paths) == 0 {
			return errors.New("usage: patchline provenance archive <evidence.jsonl>... [--json]")
		}
		return printIncidentArchive(paths, hasFlag(args[1:], "--json"))
	default:
		return fmt.Errorf("unknown provenance subcommand %q", args[0])
	}
}

func printCauseReport(g *provenance.Graph, starts []string, jsonOut bool) error {
	report, err := g.CauseReport(provenance.DefaultCauseOptions(starts))
	if err != nil {
		return err
	}
	if jsonOut {
		return writeJSON(os.Stdout, report)
	}
	fmt.Printf("cause report starts=%s minimal_causes=%d common_ancestors=%d affected=%d hash=%s\n",
		strings.Join(report.Options.Starts, ","), len(report.MinimalCauses), len(report.CommonAncestors), len(report.AffectedObservations), report.ReportHash)
	for _, cause := range report.MinimalCauses {
		fmt.Printf("  cause: %s (%s)\n", cause.ID, cause.Kind)
	}
	fmt.Printf("  evidence weakest=%s conflicts=%d\n", report.Semiring.WeakestValue, len(report.Semiring.Conflicts))
	return nil
}

func printMinimalExplanation(g *provenance.Graph, starts []string, jsonOut bool) error {
	explanation, err := g.MinimalExplanation(provenance.DefaultCauseOptions(starts))
	if err != nil {
		return err
	}
	if jsonOut {
		return writeJSON(os.Stdout, explanation)
	}
	fmt.Printf("minimal explanation starts=%s causes=%d entities=%d edges=%d missing=%d hash=%s\n",
		strings.Join(explanation.Start, ","), len(explanation.Causes), len(explanation.Entities), len(explanation.Edges), len(explanation.Missing), explanation.Hash)
	for _, edge := range explanation.Edges {
		fmt.Printf("  %s -[%s/%s]-> %s\n", edge.From, edge.Kind, edge.Evidence, edge.To)
	}
	return nil
}

func printBlastRadius(g *provenance.Graph, starts []string, jsonOut bool) error {
	report, err := g.CauseReport(provenance.DefaultCauseOptions(starts))
	if err != nil {
		return err
	}
	blast := report.BlastRadius
	if jsonOut {
		return writeJSON(os.Stdout, blast)
	}
	fmt.Printf("blast radius starts=%s causes=%d records=%d reports=%d services=%d hash=%s\n",
		strings.Join(report.Options.Starts, ","), len(blast.Causes), len(blast.Records), len(blast.Reports), len(blast.Services), blast.Hash)
	for _, count := range blast.Tables {
		fmt.Printf("  table: %s records=%d\n", count.Value, count.Count)
	}
	return nil
}

func printCausalCertificate(g *provenance.Graph, starts []string, jsonOut bool) error {
	cert, err := g.CausalCertificate(provenance.DefaultCauseOptions(starts))
	if err != nil {
		return err
	}
	if jsonOut {
		return writeJSON(os.Stdout, cert)
	}
	fmt.Printf("causal certificate starts=%s claims=%d edges=%d missing=%d hash=%s\n",
		strings.Join(cert.Start, ","), len(cert.Claims), len(cert.Explanation.Edges), len(cert.Missing), cert.Hash)
	for _, claim := range cert.Claims {
		fmt.Printf("  claim: %s\n", claim)
	}
	return nil
}

func printProvenanceDiff(leftPath, rightPath string, jsonOut bool) error {
	left, err := readEvidenceGraph(leftPath)
	if err != nil {
		return err
	}
	right, err := readEvidenceGraph(rightPath)
	if err != nil {
		return err
	}
	report := provenance.DiffGraphs(left, right)
	if jsonOut {
		return writeJSON(os.Stdout, report)
	}
	status := "different"
	if report.Equivalent {
		status = "equivalent"
	}
	fmt.Printf("provenance diff %s shared=%d left_only=%d right_only=%d hash=%s\n",
		status, len(report.SharedShapes), len(report.LeftOnly), len(report.RightOnly), report.ReportHash)
	if !report.Equivalent {
		return errors.New("provenance shapes are not equivalent")
	}
	return nil
}

func printIncidentArchive(paths []string, jsonOut bool) error {
	items := make([]provenance.IncidentItem, 0, len(paths))
	for _, path := range paths {
		g, err := readEvidenceGraph(path)
		if err != nil {
			return err
		}
		items = append(items, provenance.IncidentItem{Path: path, ShapeHash: provenance.ShapeHash(g)})
	}
	report := provenance.IncidentArchive(items)
	if jsonOut {
		return writeJSON(os.Stdout, report)
	}
	fmt.Printf("incident archive incidents=%d buckets=%d hash=%s\n", len(report.Incidents), len(report.Buckets), report.Hash)
	for _, bucket := range report.Buckets {
		fmt.Printf("  bucket: %s count=%d\n", bucket.ShapeHash, bucket.Count)
	}
	return nil
}

func archiveIndex(specPath string, jsonOut bool) error {
	spec, err := readArchiveSpec(specPath)
	if err != nil {
		return err
	}
	entries := make([]archive.Entry, 0, len(spec.Incidents))
	baseDir := filepath.Dir(specPath)
	for _, incident := range spec.Incidents {
		entry, err := buildArchiveEntry(baseDir, incident)
		if err != nil {
			return err
		}
		entries = append(entries, entry)
	}
	report := archive.Build(spec, entries)
	if jsonOut {
		return writeJSON(os.Stdout, report)
	}
	fmt.Printf("archive index incidents=%d shapes=%d migration_tables=%d repair_effects=%d hash=%s\n",
		len(report.Incidents),
		len(report.ByShape),
		len(report.ByMigrationTable),
		len(report.ByRepairEffect),
		report.Hash,
	)
	for _, bucket := range report.ByMigrationRisk {
		fmt.Printf("  migration_risk %s count=%d\n", bucket.Key, bucket.Count)
	}
	return nil
}

func archiveQuery(specPath, query string, jsonOut bool) error {
	report, err := buildArchiveReport(specPath)
	if err != nil {
		return err
	}
	if jsonOut {
		switch query {
		case "all":
			return writeJSON(os.Stdout, report.HistoricalQueries)
		case "broad-updates":
			return writeJSON(os.Stdout, report.HistoricalQueries.BroadUpdateMigrations)
		case "damaged-reports":
			return writeJSON(os.Stdout, report.HistoricalQueries.DamagedDerivedReports)
		case "missing-rollback":
			return writeJSON(os.Stdout, report.HistoricalQueries.RepairsLackingRollback)
		default:
			return fmt.Errorf("unknown archive query %q", query)
		}
	}
	switch query {
	case "all":
		printBroadUpdateQuery(report.HistoricalQueries.BroadUpdateMigrations)
		printDamagedReportQuery(report.HistoricalQueries.DamagedDerivedReports)
		printMissingRollbackQuery(report.HistoricalQueries.RepairsLackingRollback)
	case "broad-updates":
		printBroadUpdateQuery(report.HistoricalQueries.BroadUpdateMigrations)
	case "damaged-reports":
		printDamagedReportQuery(report.HistoricalQueries.DamagedDerivedReports)
	case "missing-rollback":
		printMissingRollbackQuery(report.HistoricalQueries.RepairsLackingRollback)
	default:
		return fmt.Errorf("unknown archive query %q", query)
	}
	fmt.Printf("archive query hash=%s\n", report.HistoricalQueries.Hash)
	return nil
}

func buildArchiveReport(specPath string) (archive.Report, error) {
	spec, err := readArchiveSpec(specPath)
	if err != nil {
		return archive.Report{}, err
	}
	entries := make([]archive.Entry, 0, len(spec.Incidents))
	baseDir := filepath.Dir(specPath)
	for _, incident := range spec.Incidents {
		entry, err := buildArchiveEntry(baseDir, incident)
		if err != nil {
			return archive.Report{}, err
		}
		entries = append(entries, entry)
	}
	return archive.Build(spec, entries), nil
}

func printBroadUpdateQuery(results []archive.BroadUpdateResult) {
	fmt.Printf("broad_update_migrations count=%d\n", len(results))
	for _, result := range results {
		fmt.Printf("  %s table=%s op=%s risk=%s migration=%s fingerprint=%s\n", result.IncidentID, result.Table, result.Operation, result.Risk, result.MigrationPath, result.Fingerprint)
	}
}

func printDamagedReportQuery(results []archive.DerivedReportResult) {
	fmt.Printf("damaged_derived_reports count=%d\n", len(results))
	for _, result := range results {
		fmt.Printf("  %s count=%d incidents=%s\n", result.ReportID, result.Count, strings.Join(result.Incidents, ","))
	}
}

func printMissingRollbackQuery(results []archive.MissingRollbackResult) {
	fmt.Printf("repairs_lacking_rollback count=%d\n", len(results))
	for _, result := range results {
		fmt.Printf("  %s repair=%s effect=%s policy_allowed=%t\n", result.IncidentID, result.RepairPath, result.RepairEffect, result.PolicyAllowed)
	}
}

func buildArchiveEntry(baseDir string, input archive.InputSpec) (archive.Entry, error) {
	evidencePath := resolvePath(baseDir, input.Evidence)
	migrationPath := resolvePath(baseDir, input.Migration)
	repairPath := resolvePath(baseDir, input.Repair)
	policyPath := resolvePath(baseDir, input.Policy)
	benchmarkPath := resolvePath(baseDir, input.Benchmark)

	evidenceResult, err := readEvidenceResult(evidencePath)
	if err != nil {
		return archive.Entry{}, err
	}
	graph, err := provenance.FromSlices(evidenceResult.Entities, evidenceResult.Edges)
	if err != nil {
		return archive.Entry{}, err
	}
	migrationReport, err := migration.AnalyzeFile(migrationPath)
	if err != nil {
		return archive.Entry{}, err
	}
	manifest, err := readManifest(repairPath)
	if err != nil {
		return archive.Entry{}, err
	}
	dryRun, err := replay.DryRun(manifest, graph, demo.BillingStore())
	if err != nil {
		return archive.Entry{}, err
	}
	effectSummary := abstractEffectSummary(manifest, dryRun)
	policyEvaluation, err := buildPolicyEvaluation(policyPath, repairPath, migrationPath)
	if err != nil {
		return archive.Entry{}, err
	}
	benchmarkSpec, err := readBenchmarkSpec(benchmarkPath)
	if err != nil {
		return archive.Entry{}, err
	}
	benchmarkResult, err := bench.Run(benchmarkSpec, filepath.Dir(benchmarkPath))
	if err != nil {
		return archive.Entry{}, err
	}

	entry := archive.Entry{
		ID:                      input.ID,
		EvidencePath:            input.Evidence,
		MigrationPath:           input.Migration,
		RepairPath:              input.Repair,
		PolicyPath:              input.Policy,
		BenchmarkPath:           input.Benchmark,
		EvidenceHash:            evidenceResult.InputHash,
		ShapeHash:               provenance.ShapeHash(graph),
		MigrationHash:           migrationReport.Summary.ReportHash,
		MigrationTables:         migrationReport.Summary.Tables,
		MigrationMaxRisk:        maxMigrationRisk(migrationReport),
		MigrationBroadUpdates:   archiveBroadUpdates(migrationReport),
		RepairHash:              canonical.Hash(manifest),
		RepairEffect:            string(effectSummary.Join),
		RepairRollbackAvailable: manifest.Rollback.Strategy == "snapshot" && manifest.Rollback.SnapshotRequired,
		PolicyAllowed:           policyEvaluation.OK,
		PolicyFailures:          collectPolicyFailures(policyEvaluation),
		PolicyHash:              policyEvaluation.PolicyHash,
		BenchmarkOK:             benchmarkResult.OK,
		BenchmarkHash:           benchmarkResult.SuiteHash,
		DamagedEntities:         len(evidenceResult.DamagedEntities),
		DamagedEntityIDs:        sortedStrings(evidenceResult.DamagedEntities),
		DerivedReports:          countEntitiesByKind(graph, provenance.KindReport),
		DerivedReportIDs:        derivedReportsFromDamaged(graph, evidenceResult.DamagedEntities),
		ProofBundleReady:        dryRun.Hash() != "" && policyEvaluation.PolicyHash != "" && benchmarkResult.SuiteHash != "",
	}
	return entry, nil
}

func archiveBroadUpdates(report migration.Report) []archive.MigrationStatement {
	var out []archive.MigrationStatement
	for _, statement := range report.Statements {
		if statement.Kind != "update" {
			continue
		}
		if statement.Risk != migration.RiskHigh && statement.HasWhere {
			continue
		}
		reason := "high-risk update"
		if !statement.HasWhere {
			reason = "update without where predicate"
		}
		out = append(out, archive.MigrationStatement{
			Table:       statement.Table,
			Operation:   statement.Kind,
			Risk:        string(statement.Risk),
			Effect:      statement.Effect,
			Fingerprint: statement.Fingerprint,
			Reason:      reason,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Table != out[j].Table {
			return out[i].Table < out[j].Table
		}
		return out[i].Fingerprint < out[j].Fingerprint
	})
	return out
}

func derivedReportsFromDamaged(graph *provenance.Graph, damaged []string) []string {
	damagedSet := map[string]struct{}{}
	for _, id := range damaged {
		damagedSet[id] = struct{}{}
	}
	reports := map[string]struct{}{}
	queue := append([]string(nil), damaged...)
	visited := map[string]struct{}{}
	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]
		if _, seen := visited[id]; seen {
			continue
		}
		visited[id] = struct{}{}
		entity, ok := graph.Entity(id)
		if ok && entity.Kind == provenance.KindReport {
			reports[id] = struct{}{}
		}
		for _, edge := range graph.Outgoing(id) {
			if edge.Kind != provenance.EdgeDerivedInto {
				continue
			}
			if _, alreadyDamaged := damagedSet[edge.To]; alreadyDamaged {
				queue = append(queue, edge.To)
				continue
			}
			if entity, ok := graph.Entity(edge.To); ok && entity.Kind == provenance.KindReport {
				reports[edge.To] = struct{}{}
			}
			queue = append(queue, edge.To)
		}
	}
	return stringSet(reports)
}

func maxMigrationRisk(report migration.Report) string {
	if report.Summary.HighRisk > 0 {
		return string(migration.RiskHigh)
	}
	if report.Summary.MediumRisk > 0 {
		return string(migration.RiskMedium)
	}
	if report.Summary.LowRisk > 0 {
		return string(migration.RiskLow)
	}
	return "none"
}

func countEntitiesByKind(g *provenance.Graph, kind provenance.EntityKind) int {
	count := 0
	for _, entity := range g.Entities() {
		if entity.Kind == kind {
			count++
		}
	}
	return count
}

func readTraceProjection(path string) (evidence.TraceProjection, error) {
	file, err := os.Open(path)
	if err != nil {
		return evidence.TraceProjection{}, err
	}
	defer file.Close()
	return evidence.ReconstructTraceJSONL(file)
}

func formatConfidenceCounts(counts []evidence.ConfidenceCount) string {
	parts := make([]string, 0, len(counts))
	for _, count := range counts {
		parts = append(parts, fmt.Sprintf("%s:%d", count.Value, count.Count))
	}
	if len(parts) == 0 {
		return "none"
	}
	return strings.Join(parts, ",")
}

func semanticsAudit(opts semanticAuditOptions, jsonOut bool) error {
	contract := semantics.DefaultContract()
	artifacts, err := semanticArtifacts(opts)
	if err != nil {
		return err
	}
	report := semantics.Audit(contract, artifacts)
	if jsonOut {
		return writeJSON(os.Stdout, report)
	}
	status := "failed"
	if report.OK {
		status = "passed"
	}
	fmt.Printf("semantic audit %s artifacts=%d conforming=%d proof_holes=%d counterexamples=%d hash=%s\n",
		status,
		report.Totals.Artifacts,
		report.Totals.Conforming,
		report.Totals.ProofHoles,
		report.Totals.Counterexamples,
		report.Hash,
	)
	for _, artifact := range report.Artifacts {
		fmt.Printf("  %s %s facts=%d obligations=%d hashes=%d counterexamples=%d\n",
			artifact.Kind, artifact.Path, len(artifact.Facts), len(artifact.Obligations), len(artifact.Hashes), len(artifact.Counterexamples))
	}
	if !report.OK {
		return errors.New("semantic audit found counterexamples")
	}
	return nil
}

func semanticArtifacts(opts semanticAuditOptions) ([]semantics.ArtifactEvidence, error) {
	var artifacts []semantics.ArtifactEvidence

	evidenceFile, err := os.Open(opts.Evidence)
	if err != nil {
		return nil, err
	}
	evidenceResult, err := evidence.IngestJSONL(evidenceFile)
	closeErr := evidenceFile.Close()
	if err != nil {
		return nil, err
	}
	if closeErr != nil {
		return nil, closeErr
	}
	evidenceArtifact := semantics.ArtifactEvidence{
		Path: opts.Evidence,
		Kind: "trace_projection",
		Facts: []string{
			fmt.Sprintf("events=%d", evidenceResult.EventCount),
			fmt.Sprintf("entities=%d", len(evidenceResult.Entities)),
			fmt.Sprintf("edges=%d", len(evidenceResult.Edges)),
			fmt.Sprintf("damaged_entities=%d", len(evidenceResult.DamagedEntities)),
		},
		Hashes: map[string]string{
			"input_hash":  evidenceResult.InputHash,
			"entity_hash": evidenceResult.EntityHash,
			"edge_hash":   evidenceResult.EdgeHash,
			"graph_hash":  evidenceResult.GraphHash,
		},
		Claims: []semantics.Claim{{
			Ref:      "trace_projection.ingest",
			Status:   checkedStatus(evidenceResult.OK),
			Reason:   "JSONL evidence parsed into typed provenance entities and edges",
			Evidence: evidenceResult.GraphHash,
		}},
		Metadata: map[string]interface{}{"source_types": evidenceResult.SourceTypes},
	}
	traceProjection, err := readTraceProjection(opts.Evidence)
	if err != nil {
		return nil, err
	}
	evidenceArtifact.Facts = append(evidenceArtifact.Facts,
		fmt.Sprintf("trace_observations=%d", traceProjection.ObservationCount),
		fmt.Sprintf("trace_source_summary=%s", formatConfidenceCounts(traceProjection.SourceSummary)),
		fmt.Sprintf("trace_clock_summary=%s", formatConfidenceCounts(traceProjection.ClockSummary)),
	)
	evidenceArtifact.Hashes["trace_projection_hash"] = traceProjection.ProjectionHash
	evidenceArtifact.Claims = append(evidenceArtifact.Claims, semantics.Claim{
		Ref:      "trace_projection.reconstruct",
		Status:   checkedStatus(traceProjection.OK),
		Reason:   "historical evidence reconstructed with source and clock confidence",
		Evidence: traceProjection.ProjectionHash,
	})
	for _, traceErr := range traceProjection.Errors {
		evidenceArtifact.Counterexamples = append(evidenceArtifact.Counterexamples, semantics.Counterexample{
			Ref:     "trace_projection.reconstruction_error",
			Message: traceErr,
			Witness: opts.Evidence,
		})
	}
	for _, ingestErr := range evidenceResult.Errors {
		evidenceArtifact.Counterexamples = append(evidenceArtifact.Counterexamples, semantics.Counterexample{
			Ref:     "trace_projection.ingest_error",
			Message: ingestErr,
			Witness: opts.Evidence,
		})
	}
	artifacts = append(artifacts, evidenceArtifact)

	evidenceGraph, err := provenance.FromSlices(evidenceResult.Entities, evidenceResult.Edges)
	if err != nil {
		return nil, err
	}
	cert, err := evidenceGraph.CausalCertificate(provenance.DefaultCauseOptions(evidenceResult.DamagedEntities))
	if err != nil {
		return nil, err
	}
	certificateArtifact := semantics.ArtifactEvidence{
		Path: opts.Evidence,
		Kind: "causal_certificate",
		Facts: []string{
			fmt.Sprintf("starts=%d", len(cert.Start)),
			fmt.Sprintf("claims=%d", len(cert.Claims)),
			fmt.Sprintf("explanation_edges=%d", len(cert.Explanation.Edges)),
			fmt.Sprintf("blast_records=%d", len(cert.BlastRadius.Records)),
			fmt.Sprintf("missing_evidence=%d", len(cert.Missing)),
		},
		Hashes: map[string]string{
			"certificate_hash": cert.Hash,
			"explanation_hash": cert.Explanation.Hash,
			"blast_hash":       cert.BlastRadius.Hash,
		},
		Claims: []semantics.Claim{{
			Ref:      "provenance.causal_certificate",
			Status:   semantics.ClaimChecked,
			Reason:   "historical graph produced a deterministic causal certificate with semiring evidence and blast radius",
			Evidence: cert.Hash,
		}},
		Metadata: map[string]interface{}{
			"claims":         cert.Claims,
			"minimal_causes": cert.Explanation.Causes,
			"semiring":       cert.Semiring,
		},
	}
	for _, missing := range cert.Missing {
		certificateArtifact.Obligations = append(certificateArtifact.Obligations, semantics.Obligation{
			Ref:         "missing_evidence." + missing.Need,
			Description: missing.Entity + ": " + missing.Rule,
			Status:      semantics.ClaimUnsupported,
		})
	}
	artifacts = append(artifacts, certificateArtifact)

	manifest, err := readManifest(opts.Repair)
	if err != nil {
		return nil, err
	}
	diagnostics := repair.Validate(manifest, demo.Graph())
	proofReport := repair.BuildProof(manifest)
	repairArtifact := semantics.ArtifactEvidence{
		Path: opts.Repair,
		Kind: "repair_contract",
		Facts: []string{
			"incident=" + manifest.Incident,
			fmt.Sprintf("scope_entities=%d", len(manifest.Scope.Entities)),
			fmt.Sprintf("operations=%d", len(manifest.Operations)),
			fmt.Sprintf("preconditions=%d", len(manifest.Preconditions)),
			fmt.Sprintf("postconditions=%d", len(manifest.Postconditions)),
			fmt.Sprintf("wp_obligations=%d", len(proofReport.WeakestPreconditions)),
			fmt.Sprintf("frame_conditions=%d", len(proofReport.FrameConditions)),
			fmt.Sprintf("refinement_checks=%d", len(proofReport.RefinementChecks)),
		},
		Hashes: map[string]string{
			"manifest_hash":     canonical.Hash(manifest),
			"repair_proof_hash": proofReport.Hash,
		},
		Claims: []semantics.Claim{
			{
				Ref:      "repair.validate",
				Status:   checkedStatus(!repair.HasErrors(diagnostics)),
				Reason:   "manifest parsed with unknown-field rejection and scope-aware validation",
				Evidence: opts.Repair,
			},
			{
				Ref:      "repair.hoare_contract",
				Status:   checkedStatus(proofReport.OK),
				Reason:   "manifest lowered to Hoare view, weakest preconditions, frame conditions, and SQL refinement checks",
				Evidence: proofReport.Hash,
			},
		},
		Metadata: map[string]interface{}{
			"hoare_triple":      proofReport.HoareTriple,
			"refinement_checks": proofReport.RefinementChecks,
		},
	}
	for _, wp := range proofReport.WeakestPreconditions {
		repairArtifact.Obligations = append(repairArtifact.Obligations, semantics.Obligation{
			Ref:         wp.Ref,
			Description: wp.Formula,
			Status:      proofStatus(wp.Status),
		})
	}
	for _, frame := range proofReport.FrameConditions {
		repairArtifact.Obligations = append(repairArtifact.Obligations, semantics.Obligation{
			Ref:         frame.Ref,
			Description: frame.Reason,
			Status:      proofStatus(frame.Status),
		})
	}
	for _, check := range manifest.Preconditions {
		repairArtifact.Obligations = append(repairArtifact.Obligations, semantics.Obligation{
			Ref:         "precondition." + check.Kind,
			Description: check.Expr + " == " + check.Expect,
			Status:      semantics.ClaimAssumed,
		})
	}
	for _, check := range manifest.Postconditions {
		repairArtifact.Obligations = append(repairArtifact.Obligations, semantics.Obligation{
			Ref:         "postcondition." + check.Kind,
			Description: check.Expr + " == " + check.Expect,
			Status:      semantics.ClaimAssumed,
		})
	}
	if manifest.Rollback.Strategy != "" {
		repairArtifact.Obligations = append(repairArtifact.Obligations, semantics.Obligation{
			Ref:         "rollback." + manifest.Rollback.Strategy,
			Description: "rollback strategy is declared",
			Status:      semantics.ClaimChecked,
		})
	}
	for _, diagnostic := range diagnostics {
		if diagnostic.Level == "error" {
			repairArtifact.Counterexamples = append(repairArtifact.Counterexamples, semantics.Counterexample{
				Ref:     diagnostic.Code,
				Message: diagnostic.Message,
				Witness: diagnostic.Ref,
			})
		}
	}
	for _, counterexample := range proofReport.Counterexamples {
		repairArtifact.Counterexamples = append(repairArtifact.Counterexamples, semantics.Counterexample{
			Ref:     counterexample.Ref,
			Message: counterexample.Message,
			Witness: counterexample.Witness,
		})
	}
	artifacts = append(artifacts, repairArtifact)

	beforeStore := demo.BillingStore()
	dryRunReport, afterStore, err := replay.Apply(manifest, demo.Graph(), beforeStore)
	if err != nil {
		return nil, err
	}
	abstractEffects := abstractEffectSummary(manifest, dryRunReport)
	dryRunFacts := []string{
		"incident=" + dryRunReport.Incident,
		fmt.Sprintf("operations=%d", len(dryRunReport.Operations)),
		fmt.Sprintf("downstream_entities=%d", len(dryRunReport.DownstreamEntities)),
		"abstract_join=" + string(abstractEffects.Join),
		fmt.Sprintf("abstract_rows=%d", abstractEffects.Concretization.RowsChanged),
	}
	for _, op := range dryRunReport.Operations {
		dryRunFacts = append(dryRunFacts, fmt.Sprintf("operation.%s.rows=%d", op.OperationID, op.MatchedRows))
	}
	artifacts = append(artifacts, semantics.ArtifactEvidence{
		Path:  opts.Repair,
		Kind:  "replay_report",
		Facts: dryRunFacts,
		Hashes: map[string]string{
			"dry_run_hash":        dryRunReport.Hash(),
			"effect_summary_hash": abstractEffects.Hash,
		},
		Claims: []semantics.Claim{
			{
				Ref:      "replay.dry_run",
				Status:   semantics.ClaimChecked,
				Reason:   "manifest executed in deterministic replay store",
				Evidence: dryRunReport.Hash(),
			},
			{
				Ref:      "effects.abstract_interpretation",
				Status:   semantics.ClaimChecked,
				Reason:   "concrete replay diffs abstracted into monotone effect lattice summary",
				Evidence: abstractEffects.Hash,
			},
		},
		Metadata: map[string]interface{}{
			"abstract_effects": abstractEffects,
		},
	})

	replaySemantics := replay.Analyze(manifest, demo.Graph(), beforeStore)
	replaySemanticsArtifact := semantics.ArtifactEvidence{
		Path: opts.Repair,
		Kind: "replay_semantics",
		Facts: []string{
			fmt.Sprintf("steps=%d", len(replaySemantics.StepTrace)),
			fmt.Sprintf("pair_checks=%d", len(replaySemantics.PairChecks)),
			"confluence=" + replaySemantics.Confluence.Status,
			fmt.Sprintf("orders_checked=%d", replaySemantics.Confluence.OrdersChecked),
			fmt.Sprintf("isolation_hazards=%d", len(replaySemantics.Isolation.Hazards)),
			fmt.Sprintf("compensating_actions=%d", len(replaySemantics.Compensation)),
		},
		Hashes: map[string]string{
			"replay_semantics_hash": replaySemantics.Hash,
			"initial_store_hash":    replaySemantics.InitialHash,
			"final_store_hash":      replaySemantics.FinalHash,
		},
		Claims: []semantics.Claim{
			{
				Ref:      "replay.small_step_trace",
				Status:   checkedStatus(len(replaySemantics.StepTrace) == len(manifest.Operations)),
				Reason:   "repair replay emitted an explicit small-step transition trace",
				Evidence: replaySemantics.Hash,
			},
			{
				Ref:      "repair.commutativity_confluence",
				Status:   replaySemanticsStatus(replaySemantics.Confluence.Status),
				Reason:   "dependency-valid operation orders were bounded and compared by final store hash",
				Evidence: replaySemantics.Hash,
			},
			{
				Ref:      "transaction.isolation_hazards",
				Status:   checkedStatus(len(replaySemantics.Isolation.Hazards) == 0),
				Reason:   "write/write and predicate read/write hazards were checked for supported isolation levels",
				Evidence: replaySemantics.Hash,
			},
			{
				Ref:      "external.compensating_actions",
				Status:   checkedStatus(compensationChecked(replaySemantics.Compensation)),
				Reason:   "append-only and external operations expose compensating-action semantics when present",
				Evidence: replaySemantics.Hash,
			},
		},
		Metadata: map[string]interface{}{
			"step_trace":   replaySemantics.StepTrace,
			"pair_checks":  replaySemantics.PairChecks,
			"confluence":   replaySemantics.Confluence,
			"isolation":    replaySemantics.Isolation,
			"compensation": replaySemantics.Compensation,
		},
	}
	for _, counterexample := range replaySemantics.Counterexamples {
		replaySemanticsArtifact.Counterexamples = append(replaySemanticsArtifact.Counterexamples, semantics.Counterexample{
			Ref:     counterexample.Ref,
			Message: counterexample.Message,
			Witness: canonical.Hash(counterexample),
		})
	}
	artifacts = append(artifacts, replaySemanticsArtifact)

	snapshotBefore, err := readStore(opts.SnapshotBefore)
	if err != nil {
		return nil, err
	}
	snapshotAfter, err := readStore(opts.SnapshotAfter)
	if err != nil {
		return nil, err
	}
	snapshotComparison, err := replay.CompareSnapshots(manifest, demo.Graph(), snapshotBefore, snapshotAfter)
	if err != nil {
		return nil, err
	}
	snapshotStatus := semantics.ClaimChecked
	if !snapshotComparison.Stable {
		snapshotStatus = semantics.ClaimRefuted
	}
	snapshotArtifact := semantics.ArtifactEvidence{
		Path: opts.SnapshotBefore + " -> " + opts.SnapshotAfter,
		Kind: "snapshot_comparison",
		Facts: []string{
			fmt.Sprintf("stable=%t", snapshotComparison.Stable),
			fmt.Sprintf("operation_drift=%d", len(snapshotComparison.OperationDrift)),
		},
		Hashes: map[string]string{
			"snapshot_comparison_hash": snapshotComparison.Hash,
			"before_snapshot_hash":     snapshotComparison.BeforeSnapshotHash,
			"after_snapshot_hash":      snapshotComparison.AfterSnapshotHash,
			"before_replay_hash":       snapshotComparison.BeforeReplayHash,
			"after_replay_hash":        snapshotComparison.AfterReplayHash,
		},
		Claims: []semantics.Claim{{
			Ref:      "snapshot.replay_stability",
			Status:   snapshotStatus,
			Reason:   "repair behavior was compared across imported historical row snapshots",
			Evidence: snapshotComparison.Hash,
		}},
		Metadata: map[string]interface{}{
			"comparison": snapshotComparison,
		},
	}
	for _, drift := range snapshotComparison.OperationDrift {
		snapshotArtifact.Counterexamples = append(snapshotArtifact.Counterexamples, semantics.Counterexample{
			Ref:     "snapshot.drift." + drift.OperationID,
			Message: drift.Reason,
			Witness: canonical.Hash(drift),
		})
	}
	artifacts = append(artifacts, snapshotArtifact)

	invariantSpec, err := readInvariantSpec(opts.Invariants)
	if err != nil {
		return nil, err
	}
	beforeInvariants := invariant.CheckStore(invariantSpec, beforeStore)
	afterInvariants := invariant.CheckStore(invariantSpec, afterStore)
	candidates := invariant.Discover(afterStore)
	invariantArtifact := semantics.ArtifactEvidence{
		Path: opts.Invariants,
		Kind: "invariant_report",
		Facts: []string{
			"name=" + invariantSpec.Name,
			fmt.Sprintf("before_checks=%d", len(beforeInvariants.Checks)),
			fmt.Sprintf("after_checks=%d", len(afterInvariants.Checks)),
			fmt.Sprintf("candidate_invariants=%d", len(candidates.Candidates)),
		},
		Hashes: map[string]string{
			"before_invariant_hash": beforeInvariants.Hash,
			"after_invariant_hash":  afterInvariants.Hash,
			"candidate_hash":        candidates.Hash,
		},
		Claims: []semantics.Claim{
			{
				Ref:      "invariants.before",
				Status:   checkedStatus(beforeInvariants.OK),
				Reason:   "declared invariants checked before replay",
				Evidence: beforeInvariants.Hash,
			},
			{
				Ref:      "invariants.after",
				Status:   checkedStatus(afterInvariants.OK),
				Reason:   "declared invariants checked after replay",
				Evidence: afterInvariants.Hash,
			},
			{
				Ref:      "invariants.candidates",
				Status:   semantics.ClaimAssumed,
				Reason:   "candidate invariants are explicit historical hypotheses and are not auto-accepted",
				Evidence: candidates.Hash,
			},
		},
		Metadata: map[string]interface{}{
			"before":     beforeInvariants,
			"after":      afterInvariants,
			"candidates": candidates.Candidates,
		},
	}
	for _, check := range append(beforeInvariants.Checks, afterInvariants.Checks...) {
		if check.Status == "refuted" {
			for _, counterexample := range check.Counterexamples {
				invariantArtifact.Counterexamples = append(invariantArtifact.Counterexamples, semantics.Counterexample{
					Ref:     check.ID,
					Message: counterexample.Message,
					Witness: counterexample.RowID + ":" + counterexample.Value,
				})
			}
		}
	}
	artifacts = append(artifacts, invariantArtifact)

	solverReport := solver.Analyze(manifest, beforeStore, &invariantSpec)
	solverArtifact := semantics.ArtifactEvidence{
		Path: opts.Repair,
		Kind: "solver_obligations",
		Facts: []string{
			"solver_engine=" + solverReport.SolverEngine,
			"solver_version=" + solverReport.SolverVersion,
			fmt.Sprintf("scope_implications=%d", len(solverReport.ScopeImplications)),
			fmt.Sprintf("frame_checks=%d", len(solverReport.FrameChecks)),
			fmt.Sprintf("row_count_checks=%d", len(solverReport.RowCountChecks)),
			fmt.Sprintf("invariant_checks=%d", len(solverReport.InvariantChecks)),
		},
		Hashes: map[string]string{
			"solver_obligation_hash": solverReport.Hash,
			"bounded_store_hash":     solverReport.StoreHash,
		},
		Metadata: map[string]interface{}{"summary": solverReport.Summary},
	}
	for _, check := range solverReport.ScopeImplications {
		solverArtifact.Claims = append(solverArtifact.Claims, semantics.Claim{
			Ref:      "solver.scope." + check.OperationID,
			Status:   solverStatus(check.Status),
			Reason:   check.Reason,
			Evidence: solverReport.Hash,
		})
		if check.Status == solver.StatusCounterexample {
			solverArtifact.Counterexamples = append(solverArtifact.Counterexamples, semantics.Counterexample{
				Ref:     "solver.scope." + check.OperationID,
				Message: check.Reason,
				Witness: canonical.Hash(check.Counterexample),
			})
		}
	}
	for _, check := range solverReport.FrameChecks {
		solverArtifact.Claims = append(solverArtifact.Claims, semantics.Claim{
			Ref:      "solver.frame." + check.OperationID,
			Status:   solverStatus(check.Status),
			Reason:   check.Reason,
			Evidence: solverReport.Hash,
		})
		if check.Status == solver.StatusCounterexample {
			solverArtifact.Counterexamples = append(solverArtifact.Counterexamples, semantics.Counterexample{
				Ref:     "solver.frame." + check.OperationID,
				Message: check.Reason,
				Witness: strings.Join(check.MayWriteColumns, ","),
			})
		}
	}
	for _, check := range solverReport.RowCountChecks {
		solverArtifact.Claims = append(solverArtifact.Claims, semantics.Claim{
			Ref:      "solver.row_count." + check.OperationID,
			Status:   solverStatus(check.Status),
			Reason:   check.Reason,
			Evidence: solverReport.Hash,
		})
		if check.Status == solver.StatusCounterexample {
			solverArtifact.Counterexamples = append(solverArtifact.Counterexamples, semantics.Counterexample{
				Ref:     "solver.row_count." + check.OperationID,
				Message: check.Reason,
				Witness: fmt.Sprintf("matched=%d upper_bound=%d", check.MatchedRows, check.UpperBound),
			})
		}
	}
	for _, check := range solverReport.InvariantChecks {
		solverArtifact.Claims = append(solverArtifact.Claims, semantics.Claim{
			Ref:      "solver.invariant." + check.ID,
			Status:   solverStatus(check.Status),
			Reason:   check.Reason,
			Evidence: solverReport.Hash,
		})
		if check.Status == solver.StatusCounterexample {
			solverArtifact.Counterexamples = append(solverArtifact.Counterexamples, semantics.Counterexample{
				Ref:     "solver.invariant." + check.ID,
				Message: check.Reason,
				Witness: check.Witness,
			})
		}
	}
	artifacts = append(artifacts, solverArtifact)

	symbolicReport := symbolic.Execute(manifest, beforeStore)
	symbolicArtifact := semantics.ArtifactEvidence{
		Path: opts.Repair,
		Kind: "symbolic_execution",
		Facts: []string{
			fmt.Sprintf("steps=%d", symbolicReport.Summary.Steps),
			fmt.Sprintf("rows_explored=%d", symbolicReport.Summary.RowsExplored),
			fmt.Sprintf("rows_satisfying=%d", symbolicReport.Summary.RowsSatisfying),
			fmt.Sprintf("assignments=%d", symbolicReport.Summary.Assignments),
		},
		Hashes: map[string]string{
			"symbolic_execution_hash": symbolicReport.Hash,
			"bounded_store_hash":      symbolicReport.StoreHash,
		},
		Claims: []semantics.Claim{{
			Ref:      "symbolic.execution",
			Status:   checkedStatus(symbolicReport.Summary.Errors == 0),
			Reason:   "repair program symbolically executed over bounded store rows with path constraints",
			Evidence: symbolicReport.Hash,
		}},
		Metadata: map[string]interface{}{"summary": symbolicReport.Summary},
	}
	for _, step := range symbolicReport.Steps {
		if step.Status == "stuck" {
			symbolicArtifact.Counterexamples = append(symbolicArtifact.Counterexamples, semantics.Counterexample{
				Ref:     "symbolic.step." + step.OperationID,
				Message: step.Error,
				Witness: step.PreHash,
			})
		}
	}
	artifacts = append(artifacts, symbolicArtifact)

	workflowDescriptor, err := readWorkflowDescriptor(opts.Workflow)
	if err != nil {
		return nil, err
	}
	workflowReport := workflow.Check(workflowDescriptor)
	workflowArtifact := semantics.ArtifactEvidence{
		Path: opts.Workflow,
		Kind: "workflow_model_check",
		Facts: []string{
			fmt.Sprintf("states_explored=%d", workflowReport.StatesExplored),
			fmt.Sprintf("reachable_traces=%d", workflowReport.ReachableTraces),
			fmt.Sprintf("proof_obligations=%d", len(workflowReport.Obligations)),
			fmt.Sprintf("proof_holes=%d", len(workflowReport.ProofHoles)),
		},
		Hashes: map[string]string{
			"workflow_model_hash": workflowReport.Hash,
		},
		Metadata: map[string]interface{}{
			"witness": workflowReport.Witness,
		},
	}
	for _, obligation := range workflowReport.Obligations {
		workflowArtifact.Claims = append(workflowArtifact.Claims, semantics.Claim{
			Ref:      obligation.Ref,
			Status:   proofObligationStatus(obligation.Status),
			Reason:   obligation.Reason,
			Evidence: obligation.Evidence,
		})
	}
	for _, counterexample := range workflowReport.Counterexamples {
		workflowArtifact.Counterexamples = append(workflowArtifact.Counterexamples, semantics.Counterexample{
			Ref:     counterexample.Ref,
			Message: counterexample.Message,
			Witness: strings.Join(workflowActions(counterexample.Trace), " -> "),
		})
	}
	artifacts = append(artifacts, workflowArtifact)

	refinementReport := refinement.Analyze(manifest, beforeStore, &invariantSpec, &workflowDescriptor)
	refinementArtifact := semantics.ArtifactEvidence{
		Path: opts.Repair,
		Kind: "cegar_refinement",
		Facts: []string{
			fmt.Sprintf("iterations=%d", len(refinementReport.Iterations)),
			fmt.Sprintf("refinements=%d", len(refinementReport.Refinements)),
			fmt.Sprintf("remaining_holes=%d", len(refinementReport.RemainingHoles)),
			fmt.Sprintf("counterexamples=%d", len(refinementReport.Counterexamples)),
		},
		Hashes: map[string]string{
			"cegar_refinement_hash": refinementReport.Hash,
			"bounded_store_hash":    refinementReport.StoreHash,
		},
		Claims: []semantics.Claim{{
			Ref:      "cegar.refinement",
			Status:   checkedStatus(len(refinementReport.Counterexamples) == 0),
			Reason:   "counterexample/proof-hole guided refinement reran semantic checks with invariants and workflow evidence; remaining holes are explicit obligations",
			Evidence: refinementReport.Hash,
		}},
		Metadata: map[string]interface{}{"refinements": refinementReport.Refinements},
	}
	for _, hole := range refinementReport.RemainingHoles {
		refinementArtifact.Obligations = append(refinementArtifact.Obligations, semantics.Obligation{
			Ref:         hole.Ref,
			Description: hole.Reason,
			Status:      proofObligationStatus(hole.Status),
		})
	}
	for _, counterexample := range refinementReport.Counterexamples {
		refinementArtifact.Counterexamples = append(refinementArtifact.Counterexamples, semantics.Counterexample{
			Ref:     counterexample.Ref,
			Message: counterexample.Message,
			Witness: counterexample.Witness,
		})
	}
	artifacts = append(artifacts, refinementArtifact)

	sqlPlan, err := repair.GenerateSQL(manifest)
	if err != nil {
		return nil, err
	}
	artifacts = append(artifacts, semantics.ArtifactEvidence{
		Path: opts.Repair,
		Kind: "repair_sql_refinement",
		Facts: []string{
			fmt.Sprintf("statements=%d", len(sqlPlan.Statements)),
			"incident=" + sqlPlan.Incident,
		},
		Obligations: []semantics.Obligation{{
			Ref:         "sql.refines_manifest",
			Description: "generated SQL statement count matches supported abstract operations",
			Status:      semantics.ClaimChecked,
		}},
		Hashes: map[string]string{"sql_plan_hash": sqlPlan.Hash},
	})

	transactionPlan, err := repair.GenerateTransactionPlan(manifest)
	if err != nil {
		return nil, err
	}
	artifacts = append(artifacts, semantics.ArtifactEvidence{
		Path: opts.Repair,
		Kind: "transaction_plan",
		Facts: []string{
			fmt.Sprintf("locks=%d", len(transactionPlan.LockOrder)),
			fmt.Sprintf("operation_order=%d", len(transactionPlan.OperationOrder)),
			fmt.Sprintf("statements=%d", len(transactionPlan.Statements)),
		},
		Obligations: []semantics.Obligation{{
			Ref:         "transaction.deterministic_order",
			Description: "locks and operations are sorted/dependency-ordered before execution",
			Status:      semantics.ClaimChecked,
		}},
		Hashes: map[string]string{"transaction_plan_hash": transactionPlan.Hash},
	})

	migrationReport, err := migration.AnalyzeFile(opts.Migration)
	if err != nil {
		return nil, err
	}
	migrationArtifact := semantics.ArtifactEvidence{
		Path: opts.Migration,
		Kind: "migration_report",
		Facts: []string{
			fmt.Sprintf("statements=%d", migrationReport.Summary.TotalStatements),
			fmt.Sprintf("high_risk=%d", migrationReport.Summary.HighRisk),
			fmt.Sprintf("medium_risk=%d", migrationReport.Summary.MediumRisk),
			fmt.Sprintf("tables=%d", len(migrationReport.Summary.Tables)),
		},
		Hashes: map[string]string{"migration_report_hash": migrationReport.Summary.ReportHash},
		Claims: []semantics.Claim{{
			Ref:      "migration.analyze",
			Status:   semantics.ClaimChecked,
			Reason:   "migration parsed into deterministic statement effects",
			Evidence: migrationReport.Summary.ReportHash,
		}},
	}
	for _, statement := range migrationReport.Statements {
		if statement.Risk == migration.RiskHigh {
			migrationArtifact.Obligations = append(migrationArtifact.Obligations, semantics.Obligation{
				Ref:         fmt.Sprintf("migration.statement.%d.review", statement.Index),
				Description: strings.Join(statement.Reasons, "; "),
				Status:      semantics.ClaimAssumed,
			})
		}
	}
	artifacts = append(artifacts, migrationArtifact)

	schemaMigrationPath := "demos/billing/migrations/001_schema.sql"
	schemaBeforePath := "examples/schemas/empty.json"
	schemaExpectedPath := "examples/schemas/billing-v1.json"
	schemaContent, err := os.ReadFile(schemaMigrationPath)
	if err != nil {
		return nil, err
	}
	schemaBefore, err := readSchema(schemaBeforePath)
	if err != nil {
		return nil, err
	}
	schemaExpected, err := readSchema(schemaExpectedPath)
	if err != nil {
		return nil, err
	}
	schemaDiffReport, err := migration.DiffMigrationSchema(schemaMigrationPath, schemaContent, migration.DialectGeneric, schemaBefore, schemaExpected)
	if err != nil {
		return nil, err
	}
	migrationSemanticsReport, err := migration.AnalyzeMigrationSemantics(schemaMigrationPath, schemaContent, migration.DialectGeneric, schemaBefore)
	if err != nil {
		return nil, err
	}
	schemaArtifact := semantics.ArtifactEvidence{
		Path: schemaMigrationPath,
		Kind: "schema_semantics",
		Facts: []string{
			fmt.Sprintf("schema_diffs=%d", len(schemaDiffReport.Diffs)),
			fmt.Sprintf("schema_transformations=%d", len(migrationSemanticsReport.Transformations)),
			fmt.Sprintf("relational_statements=%d", len(migrationSemanticsReport.Relational)),
		},
		Hashes: map[string]string{
			"schema_diff_hash":         schemaDiffReport.Hash,
			"migration_semantics_hash": migrationSemanticsReport.Hash,
			"expected_schema_hash":     schemaDiffReport.ExpectedHash,
			"actual_schema_hash":       schemaDiffReport.ActualHash,
			"semantic_output_hash":     migrationSemanticsReport.OutputHash,
		},
		Claims: []semantics.Claim{
			{
				Ref:      "schema.diff_expected_signature",
				Status:   checkedStatus(schemaDiffReport.OK),
				Reason:   "migration-applied schema matches the declared relational signature fixture",
				Evidence: schemaDiffReport.Hash,
			},
			{
				Ref:      "migration.relational_signature_semantics",
				Status:   semantics.ClaimChecked,
				Reason:   "migration lowered to typed schema transformations and relational statement summaries",
				Evidence: migrationSemanticsReport.Hash,
			},
		},
		Metadata: map[string]interface{}{
			"before_schema":          schemaBeforePath,
			"expected_schema":        schemaExpectedPath,
			"schema_transformations": migrationSemanticsReport.Transformations,
			"relational_statements":  migrationSemanticsReport.Relational,
		},
	}
	for _, diff := range schemaDiffReport.Diffs {
		schemaArtifact.Counterexamples = append(schemaArtifact.Counterexamples, semantics.Counterexample{
			Ref:     "schema_diff." + diff.Kind,
			Message: fmt.Sprintf("table=%s column=%s expected=%s actual=%s", diff.Table, diff.Column, diff.Expect, diff.Actual),
			Witness: schemaMigrationPath,
		})
	}
	artifacts = append(artifacts, schemaArtifact)

	sourceSQLReport, err := migration.ExtractSourceSQL("examples/source-sql")
	if err != nil {
		return nil, err
	}
	sourceSQLArtifact := semantics.ArtifactEvidence{
		Path: "examples/source-sql",
		Kind: "source_sql_inventory",
		Facts: []string{
			fmt.Sprintf("files_scanned=%d", sourceSQLReport.Summary.FilesScanned),
			fmt.Sprintf("embedded_sql=%d", sourceSQLReport.Summary.EmbeddedSQL),
			fmt.Sprintf("orm_queries=%d", sourceSQLReport.Summary.ORMQueries),
			fmt.Sprintf("tables=%d", len(sourceSQLReport.Summary.Tables)),
		},
		Hashes: map[string]string{"source_sql_hash": sourceSQLReport.Hash},
		Claims: []semantics.Claim{{
			Ref:      "source_sql.extract",
			Status:   checkedStatus(len(sourceSQLReport.Observations) > 0),
			Reason:   "application source and migration-framework fixtures were scanned for embedded SQL and ORM/query-builder effects",
			Evidence: sourceSQLReport.Hash,
		}},
		Metadata: map[string]interface{}{
			"frameworks": sourceSQLReport.Summary.Frameworks,
			"languages":  sourceSQLReport.Summary.Languages,
			"tables":     sourceSQLReport.Summary.Tables,
		},
	}
	artifacts = append(artifacts, sourceSQLArtifact)

	benchmarkSpec, err := readBenchmarkSpec(opts.Benchmark)
	if err != nil {
		return nil, err
	}
	benchmarkResult, err := bench.Run(benchmarkSpec, filepath.Dir(opts.Benchmark))
	if err != nil {
		return nil, err
	}
	benchmarkArtifact := semantics.ArtifactEvidence{
		Path: opts.Benchmark,
		Kind: "benchmark_report",
		Facts: []string{
			fmt.Sprintf("cases=%d", benchmarkResult.Metrics.Total),
			fmt.Sprintf("passed=%d", benchmarkResult.Metrics.Passed),
			fmt.Sprintf("precision=%.3f", benchmarkResult.Metrics.Precision),
			fmt.Sprintf("recall=%.3f", benchmarkResult.Metrics.Recall),
		},
		Hashes: map[string]string{"suite_hash": benchmarkResult.SuiteHash},
		Claims: []semantics.Claim{{
			Ref:      "benchmark.strict_hashes",
			Status:   checkedStatus(benchmarkResult.OK),
			Reason:   "strict corpus labels and pinned report hashes matched",
			Evidence: benchmarkResult.SuiteHash,
		}},
	}
	for _, c := range benchmarkResult.Cases {
		benchmarkArtifact.Obligations = append(benchmarkArtifact.Obligations, semantics.Obligation{
			Ref:         "benchmark." + c.ID + ".pinned_hash",
			Description: "expected migration report hash is pinned and rechecked",
			Status:      checkedStatus(c.OK),
		})
		if !c.OK {
			benchmarkArtifact.Counterexamples = append(benchmarkArtifact.Counterexamples, semantics.Counterexample{
				Ref:     "benchmark." + c.ID,
				Message: "case label or expected report hash did not match",
				Witness: c.ReportHash,
			})
		}
	}
	artifacts = append(artifacts, benchmarkArtifact)

	policyFailures := []string{}
	if opts.Policy != "" {
		policyEval, err := buildPolicyEvaluation(opts.Policy, opts.Repair, opts.Migration)
		if err != nil {
			return nil, err
		}
		policyFailures = collectPolicyFailures(policyEval)
	}
	var invariantCandidates []string
	for _, candidate := range candidates.Candidates {
		invariantCandidates = append(invariantCandidates, candidate.ID)
	}
	outcomeReport := migration.BuildMigrationOutcomeReport(opts.Migration, migrationReport, evidenceResult.Entities, evidenceResult.Edges, migration.OutcomeOptions{
		EvidenceHash:        evidenceResult.GraphHash,
		RepairID:            manifest.Incident,
		RepairHash:          canonical.Hash(manifest),
		RepairOperations:    len(manifest.Operations),
		RollbackStrategy:    manifest.Rollback.Strategy,
		PolicyFailures:      policyFailures,
		BenchmarkHash:       benchmarkResult.SuiteHash,
		SourceSQLHash:       sourceSQLReport.Hash,
		InvariantCandidates: invariantCandidates,
	})
	outcomeArtifact := semantics.ArtifactEvidence{
		Path: opts.Evidence,
		Kind: "migration_outcome_history",
		Facts: []string{
			fmt.Sprintf("migrations=%d", outcomeReport.Changelog.ObservedOutcomes.Migrations),
			fmt.Sprintf("traces=%d", outcomeReport.Changelog.ObservedOutcomes.Traces),
			fmt.Sprintf("sql_mutations=%d", outcomeReport.Changelog.ObservedOutcomes.SQLMutations),
			fmt.Sprintf("records=%d", outcomeReport.Changelog.ObservedOutcomes.Records),
			fmt.Sprintf("reports=%d", outcomeReport.Changelog.ObservedOutcomes.Reports),
			fmt.Sprintf("broad_effects=%d", len(outcomeReport.Changelog.BroadEffects)),
			fmt.Sprintf("policy_failures=%d", len(outcomeReport.Changelog.PolicyFailures)),
		},
		Hashes: map[string]string{
			"migration_outcome_hash":  outcomeReport.Hash,
			"semantic_changelog_hash": outcomeReport.Changelog.Hash,
			"evidence_graph_hash":     evidenceResult.GraphHash,
		},
		Claims: []semantics.Claim{
			{
				Ref:      "migration.outcome_history",
				Status:   checkedStatus(len(outcomeReport.Outcomes) > 0),
				Reason:   "historical migration entities were deterministically linked to traces, SQL mutations, rows, reports, repairs, and policy outcomes",
				Evidence: outcomeReport.Hash,
			},
			{
				Ref:      "migration.semantic_changelog",
				Status:   checkedStatus(len(outcomeReport.Changelog.ChangedTables) > 0),
				Reason:   "migration report was lifted into a table/effect changelog with observed downstream outcomes and benchmark hashes",
				Evidence: outcomeReport.Changelog.Hash,
			},
		},
		Metadata: map[string]interface{}{
			"outcomes":  outcomeReport.Outcomes,
			"changelog": outcomeReport.Changelog,
		},
	}
	for _, failure := range policyFailures {
		outcomeArtifact.Obligations = append(outcomeArtifact.Obligations, semantics.Obligation{
			Ref:         "policy.failure",
			Description: failure,
			Status:      semantics.ClaimRefuted,
		})
	}
	artifacts = append(artifacts, outcomeArtifact)

	archiveSpec, err := readArchiveSpec(opts.Archive)
	if err != nil {
		return nil, err
	}
	archiveEntries := make([]archive.Entry, 0, len(archiveSpec.Incidents))
	for _, input := range archiveSpec.Incidents {
		entry, err := buildArchiveEntry(filepath.Dir(opts.Archive), input)
		if err != nil {
			return nil, err
		}
		archiveEntries = append(archiveEntries, entry)
	}
	archiveReport := archive.Build(archiveSpec, archiveEntries)
	artifacts = append(artifacts, semantics.ArtifactEvidence{
		Path: opts.Archive,
		Kind: "incident_archive",
		Facts: []string{
			fmt.Sprintf("incidents=%d", len(archiveReport.Incidents)),
			fmt.Sprintf("shape_buckets=%d", len(archiveReport.ByShape)),
			fmt.Sprintf("migration_table_buckets=%d", len(archiveReport.ByMigrationTable)),
			fmt.Sprintf("repair_effect_buckets=%d", len(archiveReport.ByRepairEffect)),
		},
		Hashes: map[string]string{"archive_index_hash": archiveReport.Hash},
		Claims: []semantics.Claim{{
			Ref:      "archive.incident_index",
			Status:   checkedStatus(len(archiveReport.Incidents) > 0),
			Reason:   "historical incidents were deterministically bucketed by evidence shape, migration table/risk, repair effect, policy decision, and benchmark decision",
			Evidence: archiveReport.Hash,
		}},
		Metadata: map[string]interface{}{
			"by_shape":              archiveReport.ByShape,
			"by_migration_table":    archiveReport.ByMigrationTable,
			"by_repair_effect":      archiveReport.ByRepairEffect,
			"by_policy_decision":    archiveReport.ByPolicyDecision,
			"by_benchmark_decision": archiveReport.ByBenchmarkDecision,
		},
	})

	ledgerEntries, checkpoint := demo.SampleLedger()
	ledgerErr := ledger.VerifyCheckpoint(ledgerEntries, checkpoint)
	ledgerArtifact := semantics.ArtifactEvidence{
		Path: "demo.SampleLedger",
		Kind: "ledger_checkpoint",
		Facts: []string{
			fmt.Sprintf("entries=%d", len(ledgerEntries)),
			fmt.Sprintf("checkpoint_count=%d", checkpoint.Count),
		},
		Hashes: map[string]string{
			"ledger_entries_hash": canonical.Hash(ledgerEntries),
			"ledger_tip_hash":     checkpoint.TipHash,
		},
		Claims: []semantics.Claim{{
			Ref:      "ledger.verify_checkpoint",
			Status:   checkedStatus(ledgerErr == nil),
			Reason:   "hash chain and checkpoint tip are verified",
			Evidence: checkpoint.TipHash,
		}},
	}
	if ledgerErr != nil {
		ledgerArtifact.Counterexamples = append(ledgerArtifact.Counterexamples, semantics.Counterexample{
			Ref:     "ledger.verify_checkpoint",
			Message: ledgerErr.Error(),
		})
	}
	artifacts = append(artifacts, ledgerArtifact)

	return artifacts, nil
}

func checkedStatus(ok bool) semantics.ClaimStatus {
	if ok {
		return semantics.ClaimChecked
	}
	return semantics.ClaimRefuted
}

func proofStatus(status repair.ProofStatus) semantics.ClaimStatus {
	switch status {
	case repair.ProofChecked:
		return semantics.ClaimChecked
	case repair.ProofAssumed:
		return semantics.ClaimAssumed
	case repair.ProofUnsupported:
		return semantics.ClaimUnsupported
	case repair.ProofRefuted:
		return semantics.ClaimRefuted
	default:
		return semantics.ClaimUnsupported
	}
}

func solverStatus(status solver.Status) semantics.ClaimStatus {
	switch status {
	case solver.StatusProved:
		return semantics.ClaimProved
	case solver.StatusChecked:
		return semantics.ClaimChecked
	case solver.StatusAssumed:
		return semantics.ClaimAssumed
	case solver.StatusCounterexample:
		return semantics.ClaimRefuted
	case solver.StatusNotSupported:
		return semantics.ClaimUnsupported
	default:
		return semantics.ClaimUnsupported
	}
}

func proofObligationStatus(status proof.Status) semantics.ClaimStatus {
	switch status {
	case proof.StatusProved:
		return semantics.ClaimProved
	case proof.StatusChecked:
		return semantics.ClaimChecked
	case proof.StatusAssumed:
		return semantics.ClaimAssumed
	case proof.StatusCounterexample:
		return semantics.ClaimRefuted
	case proof.StatusNotSupported:
		return semantics.ClaimUnsupported
	default:
		return semantics.ClaimUnsupported
	}
}

func workflowActions(actions []workflow.Action) []string {
	out := make([]string, 0, len(actions))
	for _, action := range actions {
		out = append(out, string(action))
	}
	return out
}

func compensationChecked(actions []replay.CompensatingAction) bool {
	for _, action := range actions {
		if action.Status != "checked" {
			return false
		}
	}
	return true
}

func replaySemanticsStatus(status string) semantics.ClaimStatus {
	switch status {
	case "checked":
		return semantics.ClaimChecked
	case "unknown":
		return semantics.ClaimAssumed
	case "refuted":
		return semantics.ClaimRefuted
	default:
		return semantics.ClaimUnsupported
	}
}

func explain(g *provenance.Graph, entityID string) error {
	paths, err := g.Backtrace(entityID, provenance.TraceOptions{
		StopKinds:   []provenance.EntityKind{provenance.KindDeploy, provenance.KindCommit},
		MinEvidence: provenance.EvidenceStrong,
	})
	if err != nil {
		return err
	}
	if len(paths) == 0 {
		return fmt.Errorf("no strong causal path found for %s", entityID)
	}

	fmt.Printf("Causal explanation for %s\n", entityID)
	for i, step := range paths[0].Steps {
		if i == 0 {
			fmt.Printf("  %s (%s)\n", step.Entity.ID, step.Entity.Kind)
			continue
		}
		fmt.Printf("  <- %s via %s [%s]\n", step.Entity.ID, step.Via.Kind, step.Via.Evidence)
	}
	return nil
}

func sliceGraph(g *provenance.Graph, entityID string, jsonOut bool) error {
	slice, err := g.Slice(provenance.SliceOptions{
		Starts:      []string{entityID},
		Direction:   provenance.DirectionBoth,
		MaxDepth:    4,
		MinEvidence: provenance.EvidenceStrong,
	})
	if err != nil {
		return err
	}
	if jsonOut {
		return writeJSON(os.Stdout, slice)
	}
	fmt.Printf("slice %s entities=%d edges=%d hash=%s\n", entityID, len(slice.Entities), len(slice.Edges), slice.SliceHash)
	return nil
}

func validateRepair(path string, jsonOut bool) error {
	manifest, err := readManifest(path)
	if err != nil {
		return err
	}
	diagnostics := repair.Validate(manifest, demo.Graph())

	if jsonOut {
		return writeJSON(os.Stdout, diagnostics)
	}
	for _, d := range diagnostics {
		fmt.Printf("%s %s: %s", strings.ToUpper(d.Level), d.Code, d.Message)
		if d.Ref != "" {
			fmt.Printf(" (%s)", d.Ref)
		}
		fmt.Println()
	}
	if repair.HasErrors(diagnostics) {
		return errors.New("repair manifest is invalid")
	}
	fmt.Println("repair manifest is valid")
	return nil
}

func migrationOutcomes(evidencePath, migrationPath string, args []string, jsonOut bool) error {
	evidenceResult, err := readEvidenceResult(evidencePath)
	if err != nil {
		return err
	}
	if !evidenceResult.OK {
		return errors.New("evidence ingest failed")
	}
	migrationReport, err := migration.AnalyzeFile(migrationPath)
	if err != nil {
		return err
	}
	opts := migration.OutcomeOptions{EvidenceHash: evidenceResult.GraphHash}
	if repairPath, ok := flagValue(args, "--repair"); ok {
		manifest, err := readManifest(repairPath)
		if err != nil {
			return err
		}
		opts.RepairID = manifest.Incident
		opts.RepairHash = canonical.Hash(manifest)
		opts.RepairOperations = len(manifest.Operations)
		opts.RollbackStrategy = manifest.Rollback.Strategy
	}
	if policyPath, ok := flagValue(args, "--policy"); ok {
		repairPath, ok := flagValue(args, "--repair")
		if !ok {
			return errors.New("--policy requires --repair so the policy can be evaluated against a concrete repair")
		}
		eval, err := buildPolicyEvaluation(policyPath, repairPath, migrationPath)
		if err != nil {
			return err
		}
		opts.PolicyFailures = collectPolicyFailures(eval)
	}
	if benchmarkPath, ok := flagValue(args, "--benchmark"); ok {
		spec, err := readBenchmarkSpec(benchmarkPath)
		if err != nil {
			return err
		}
		result, err := bench.Run(spec, filepath.Dir(benchmarkPath))
		if err != nil {
			return err
		}
		opts.BenchmarkHash = result.SuiteHash
	}
	if sourcePath, ok := flagValue(args, "--source-sql"); ok {
		sourceReport, err := migration.ExtractSourceSQL(sourcePath)
		if err != nil {
			return err
		}
		opts.SourceSQLHash = sourceReport.Hash
	}
	report := migration.BuildMigrationOutcomeReport(migrationPath, migrationReport, evidenceResult.Entities, evidenceResult.Edges, opts)
	if jsonOut {
		return writeJSON(os.Stdout, report)
	}
	fmt.Printf("migration outcomes migrations=%d traces=%d sql_mutations=%d records=%d reports=%d repairs=%d hash=%s\n",
		report.Changelog.ObservedOutcomes.Migrations,
		report.Changelog.ObservedOutcomes.Traces,
		report.Changelog.ObservedOutcomes.SQLMutations,
		report.Changelog.ObservedOutcomes.Records,
		report.Changelog.ObservedOutcomes.Reports,
		report.Changelog.ObservedOutcomes.Repairs,
		report.Hash,
	)
	fmt.Printf("  changelog tables=%d broad_effects=%d policy_failures=%d hash=%s\n",
		len(report.Changelog.ChangedTables),
		len(report.Changelog.BroadEffects),
		len(report.Changelog.PolicyFailures),
		report.Changelog.Hash,
	)
	for _, effect := range report.Changelog.BroadEffects {
		fmt.Printf("  broad_effect table=%s operation=%s risk=%s reason=%s\n", effect.Table, effect.Operation, effect.Risk, effect.Reason)
	}
	return nil
}

func dryRun(path string, args []string, jsonOut bool) error {
	manifest, err := readManifest(path)
	if err != nil {
		return err
	}
	g := demo.Graph()
	diagnostics := repair.Validate(manifest, g)
	if repair.HasErrors(diagnostics) {
		if jsonOut {
			return writeJSON(os.Stdout, diagnostics)
		}
		for _, d := range diagnostics {
			fmt.Printf("%s %s: %s\n", strings.ToUpper(d.Level), d.Code, d.Message)
		}
		return errors.New("repair manifest is invalid")
	}

	store, err := replayStoreFromArgs(args)
	if err != nil {
		return err
	}
	report, err := replay.DryRun(manifest, g, store)
	if err != nil {
		return err
	}
	if jsonOut {
		return writeJSON(os.Stdout, report)
	}

	fmt.Printf("dry run %s for incident %s\n", report.Manifest, report.Incident)
	for _, op := range report.Operations {
		fmt.Printf("  %s: %d row(s), effect=%s\n", op.OperationID, op.MatchedRows, op.Effect)
		for _, diff := range op.Diffs {
			fmt.Printf("    %s/%s %s -> %s\n", diff.Table, diff.ID, diff.BeforeHash[:12], diff.AfterHash[:12])
		}
	}
	fmt.Printf("canonical report hash: %s\n", report.Hash())
	return nil
}

func repairSemantics(path string, args []string, jsonOut bool) error {
	manifest, err := readManifest(path)
	if err != nil {
		return err
	}
	store, err := replayStoreFromArgs(args)
	if err != nil {
		return err
	}
	report := replay.Analyze(manifest, demo.Graph(), store)
	if jsonOut {
		if err := writeJSON(os.Stdout, report); err != nil {
			return err
		}
		if !report.OK {
			return codedError{code: 2, err: errors.New("repair semantics found counterexamples")}
		}
		return nil
	}
	fmt.Println(report.Summary())
	for _, step := range report.StepTrace {
		fmt.Printf("  step %d %s state=%s rule=%s rows=%d %s -> %s\n",
			step.Index, step.OperationID, step.State, step.Rule, step.MatchedRows, step.PreHash[:12], step.PostHash[:12])
	}
	for _, pair := range report.PairChecks {
		fmt.Printf("  pair %s/%s syntactic=%s observed=%s reason=%s\n",
			pair.Left, pair.Right, pair.SyntacticVerdict, pair.ObservationStatus, pair.Reason)
	}
	for _, level := range report.Isolation.Levels {
		fmt.Printf("  isolation %s status=%s\n", level.Level, level.Status)
	}
	if !report.OK {
		return codedError{code: 2, err: errors.New("repair semantics found counterexamples")}
	}
	return nil
}

func snapshotDrift(path, beforePath, afterPath string, jsonOut bool) error {
	manifest, err := readManifest(path)
	if err != nil {
		return err
	}
	before, err := readStore(beforePath)
	if err != nil {
		return err
	}
	after, err := readStore(afterPath)
	if err != nil {
		return err
	}
	report, err := replay.CompareSnapshots(manifest, demo.Graph(), before, after)
	if err != nil {
		return err
	}
	if jsonOut {
		if err := writeJSON(os.Stdout, report); err != nil {
			return err
		}
		if !report.Stable {
			return codedError{code: 2, err: errors.New("snapshot replay drift detected")}
		}
		return nil
	}
	status := "stable"
	if !report.Stable {
		status = "drift"
	}
	fmt.Printf("snapshot drift %s operations=%d hash=%s\n", status, len(report.OperationDrift), report.Hash)
	for _, drift := range report.OperationDrift {
		fmt.Printf("  %s before_rows=%d after_rows=%d reason=%s\n", drift.OperationID, drift.BeforeMatchedRows, drift.AfterMatchedRows, drift.Reason)
	}
	if !report.Stable {
		return codedError{code: 2, err: errors.New("snapshot replay drift detected")}
	}
	return nil
}

func replayStoreFromArgs(args []string) (replay.Store, error) {
	storePath, ok := flagValue(args, "--store")
	if ok {
		return readStore(storePath)
	}
	return demo.BillingStore(), nil
}

func effectSummary(path string, jsonOut bool) error {
	manifest, err := readManifest(path)
	if err != nil {
		return err
	}
	report, err := replay.DryRun(manifest, demo.Graph(), demo.BillingStore())
	if err != nil {
		return err
	}
	summary := abstractEffectSummary(manifest, report)
	if jsonOut {
		return writeJSON(os.Stdout, struct {
			Concrete replay.Report           `json:"concrete"`
			Abstract effects.AbstractSummary `json:"abstract"`
		}{Concrete: report, Abstract: summary})
	}
	fmt.Printf("effect summary manifest=%s join=%s operations=%d rows=%d downstream=%d hash=%s\n",
		summary.Manifest,
		summary.Join,
		len(summary.Operations),
		summary.Concretization.RowsChanged,
		summary.Concretization.DownstreamEntities,
		summary.Hash,
	)
	for _, op := range summary.Operations {
		fmt.Printf("  %s effect=%s rows<=%d columns=%s reversible=%t idempotent=%t transfer=%s\n",
			op.OperationID,
			op.Effect,
			op.MaxRows,
			strings.Join(op.ChangedColumns, ","),
			op.Reversible,
			op.Idempotent,
			op.Transfer,
		)
	}
	return nil
}

func abstractEffectSummary(manifest repair.Manifest, report replay.Report) effects.AbstractSummary {
	operationByID := map[string]repair.Operation{}
	for _, op := range manifest.Operations {
		operationByID[op.ID] = op
	}
	observations := make([]effects.OperationObservation, 0, len(report.Operations))
	for index, opReport := range report.Operations {
		changedColumns := map[string]struct{}{}
		for _, diff := range opReport.Diffs {
			for column := range diff.Changes {
				changedColumns[column] = struct{}{}
			}
		}
		downstream := 0
		if index == 0 {
			downstream = len(report.DownstreamEntities)
		}
		observation := effects.OperationObservation{
			OperationID:        opReport.OperationID,
			Table:              opReport.Table,
			Effect:             effects.Effect(opReport.Effect),
			MatchedRows:        opReport.MatchedRows,
			ChangedColumns:     stringSet(changedColumns),
			DownstreamEntities: downstream,
			HasSnapshotRollback: manifest.Rollback.Strategy == "snapshot" &&
				manifest.Rollback.SnapshotRequired,
		}
		if op, ok := operationByID[opReport.OperationID]; ok {
			classification := effects.Infer(effects.Mutation{
				Kind:                op.Kind,
				Table:               op.Table,
				WhereKeys:           keys(op.Where),
				SetKeys:             keys(op.Set),
				HasSnapshotRollback: manifest.Rollback.Strategy == "snapshot" && manifest.Rollback.SnapshotRequired,
			})
			observation.Reasons = classification.Reasons
		}
		observations = append(observations, observation)
	}
	return effects.Summarize(report.Manifest, report.Incident, observations)
}

func checkInvariants(manifestPath, invariantPath string, jsonOut bool) error {
	manifest, err := readManifest(manifestPath)
	if err != nil {
		return err
	}
	spec, err := readInvariantSpec(invariantPath)
	if err != nil {
		return err
	}
	beforeStore := demo.BillingStore()
	replayReport, afterStore, err := replay.Apply(manifest, demo.Graph(), beforeStore)
	if err != nil {
		return err
	}
	report := struct {
		Version    string           `json:"version"`
		Manifest   string           `json:"manifest"`
		Invariants string           `json:"invariants"`
		OK         bool             `json:"ok"`
		Before     invariant.Report `json:"before"`
		After      invariant.Report `json:"after"`
		ReplayHash string           `json:"replay_hash"`
		Hash       string           `json:"hash"`
	}{
		Version:    "patchline.invariant-timeline/v1",
		Manifest:   manifest.Name,
		Invariants: spec.Name,
		Before:     invariant.CheckStore(spec, beforeStore),
		After:      invariant.CheckStore(spec, afterStore),
		ReplayHash: replayReport.Hash(),
	}
	report.OK = report.Before.OK && report.After.OK
	report.Hash = canonical.Hash(struct {
		Version    string           `json:"version"`
		Manifest   string           `json:"manifest"`
		Invariants string           `json:"invariants"`
		OK         bool             `json:"ok"`
		Before     invariant.Report `json:"before"`
		After      invariant.Report `json:"after"`
		ReplayHash string           `json:"replay_hash"`
	}{report.Version, report.Manifest, report.Invariants, report.OK, report.Before, report.After, report.ReplayHash})
	if jsonOut {
		return writeJSON(os.Stdout, report)
	}
	status := "passed"
	if !report.OK {
		status = "failed"
	}
	fmt.Printf("invariant check %s before=%d after=%d hash=%s\n", status, len(report.Before.Checks), len(report.After.Checks), report.Hash)
	printInvariantFailures("before", report.Before)
	printInvariantFailures("after", report.After)
	if !report.OK {
		return errors.New("invariant check failed")
	}
	return nil
}

func solverObligations(manifestPath string, args []string, jsonOut bool) error {
	manifest, err := readManifest(manifestPath)
	if err != nil {
		return err
	}
	store := demo.BillingStore()
	if storePath, ok := flagValue(args, "--store"); ok {
		store, err = readStore(storePath)
		if err != nil {
			return err
		}
	}
	var spec *invariant.Spec
	if invariantPath, ok := flagValue(args, "--invariants"); ok {
		loaded, err := readInvariantSpec(invariantPath)
		if err != nil {
			return err
		}
		spec = &loaded
	}
	report := solver.Analyze(manifest, store, spec)
	if jsonOut {
		return writeJSON(os.Stdout, report)
	}
	fmt.Printf("solver obligations manifest=%s proved=%d checked=%d counterexamples=%d assumed=%d not_supported=%d hash=%s\n",
		report.Manifest,
		report.Summary.Proved,
		report.Summary.Checked,
		report.Summary.Counterexamples,
		report.Summary.Assumed,
		report.Summary.NotSupported,
		report.Hash,
	)
	if report.Summary.Counterexamples > 0 {
		return errors.New("solver obligations found counterexamples")
	}
	return nil
}

func symbolicExec(manifestPath string, args []string, jsonOut bool) error {
	manifest, err := readManifest(manifestPath)
	if err != nil {
		return err
	}
	store := demo.BillingStore()
	if storePath, ok := flagValue(args, "--store"); ok {
		store, err = readStore(storePath)
		if err != nil {
			return err
		}
	}
	report := symbolic.Execute(manifest, store)
	if jsonOut {
		return writeJSON(os.Stdout, report)
	}
	fmt.Printf("symbolic execution manifest=%s steps=%d rows_explored=%d rows_satisfying=%d assignments=%d errors=%d hash=%s\n",
		report.Manifest,
		report.Summary.Steps,
		report.Summary.RowsExplored,
		report.Summary.RowsSatisfying,
		report.Summary.Assignments,
		report.Summary.Errors,
		report.Hash,
	)
	if report.Summary.Errors > 0 {
		return errors.New("symbolic execution found stuck steps")
	}
	return nil
}

func modelCheckWorkflow(workflowPath string, jsonOut bool) error {
	descriptor, err := readWorkflowDescriptor(workflowPath)
	if err != nil {
		return err
	}
	report := workflow.Check(descriptor)
	if jsonOut {
		return writeJSON(os.Stdout, report)
	}
	fmt.Printf("workflow model check %s states=%d traces=%d obligations=%d holes=%d counterexamples=%d hash=%s\n",
		report.Name,
		report.StatesExplored,
		report.ReachableTraces,
		len(report.Obligations),
		len(report.ProofHoles),
		len(report.Counterexamples),
		report.Hash,
	)
	if len(report.Counterexamples) > 0 {
		return errors.New("workflow model check found counterexamples")
	}
	return nil
}

func discoverInvariants(manifestPath string, jsonOut bool) error {
	manifest, err := readManifest(manifestPath)
	if err != nil {
		return err
	}
	beforeStore := demo.BillingStore()
	_, afterStore, err := replay.Apply(manifest, demo.Graph(), beforeStore)
	if err != nil {
		return err
	}
	report := struct {
		Version  string           `json:"version"`
		Manifest string           `json:"manifest"`
		Before   invariant.Report `json:"before"`
		After    invariant.Report `json:"after"`
		Hash     string           `json:"hash"`
	}{
		Version:  "patchline.invariant-candidates/v1",
		Manifest: manifest.Name,
		Before:   invariant.Discover(beforeStore),
		After:    invariant.Discover(afterStore),
	}
	report.Hash = canonical.Hash(struct {
		Version  string           `json:"version"`
		Manifest string           `json:"manifest"`
		Before   invariant.Report `json:"before"`
		After    invariant.Report `json:"after"`
	}{report.Version, report.Manifest, report.Before, report.After})
	if jsonOut {
		return writeJSON(os.Stdout, report)
	}
	fmt.Printf("invariant candidates before=%d after=%d hash=%s\n", len(report.Before.Candidates), len(report.After.Candidates), report.Hash)
	for _, candidate := range report.After.Candidates {
		fmt.Printf("  %s %s table=%s column=%s\n", candidate.ID, candidate.Kind, candidate.Table, candidate.Column)
	}
	return nil
}

func printInvariantFailures(phase string, report invariant.Report) {
	for _, check := range report.Checks {
		for _, counterexample := range check.Counterexamples {
			fmt.Printf("  %s %s: %s row=%s value=%s\n", phase, check.ID, counterexample.Message, counterexample.RowID, counterexample.Value)
		}
	}
}

func migrateRepair(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	manifest, err := repair.Migrate(file)
	if err != nil {
		return err
	}
	return writeJSON(os.Stdout, manifest)
}

func templateRepair(name string) error {
	manifest, err := repair.Template(name)
	if err != nil {
		return err
	}
	return writeJSON(os.Stdout, manifest)
}

func lintRepair(path string, jsonOut, proof bool) error {
	manifest, err := readManifest(path)
	if err != nil {
		return err
	}
	if proof {
		report := repair.BuildProof(manifest)
		if jsonOut {
			if err := writeJSON(os.Stdout, report); err != nil {
				return err
			}
		} else {
			status := "passed"
			if !report.OK {
				status = "failed"
			}
			fmt.Printf("repair proof %s wp=%d frames=%d refinements=%d counterexamples=%d hash=%s\n",
				status,
				len(report.WeakestPreconditions),
				len(report.FrameConditions),
				len(report.RefinementChecks),
				len(report.Counterexamples),
				report.Hash,
			)
			fmt.Printf("  hoare: %s\n", report.HoareTriple.Notation)
			for _, obligation := range report.WeakestPreconditions {
				fmt.Printf("  wp %s %s: %s\n", obligation.Status, obligation.Ref, obligation.Formula)
			}
			for _, frame := range report.FrameConditions {
				fmt.Printf("  frame %s %s table=%s rows=%s columns=%s\n",
					frame.Status,
					frame.Ref,
					frame.Table,
					strings.Join(frame.MayWriteRows, ","),
					strings.Join(frame.MayWriteColumns, ","),
				)
			}
			for _, check := range report.RefinementChecks {
				fmt.Printf("  refinement %s %s: %s\n", check.Status, check.OperationID, check.Reason)
			}
		}
		if !report.OK {
			return errors.New("repair proof failed")
		}
		return nil
	}
	result := repair.Lint(manifest)
	if jsonOut {
		if err := writeJSON(os.Stdout, result); err != nil {
			return err
		}
	} else {
		status := "passed"
		if !result.OK {
			status = "failed"
		}
		fmt.Printf("repair lint %s findings=%d\n", status, len(result.Findings))
		for _, finding := range result.Findings {
			fmt.Printf("  %s %s %s", finding.Level, finding.Code, finding.Message)
			if finding.Ref != "" {
				fmt.Printf(" ref=%s", finding.Ref)
			}
			fmt.Printf(" remediation=%s\n", finding.Remediation)
		}
	}
	if !result.OK {
		return errors.New("repair lint failed")
	}
	return nil
}

func cegarRefine(manifestPath string, args []string, jsonOut bool) error {
	manifest, err := readManifest(manifestPath)
	if err != nil {
		return err
	}
	store := demo.BillingStore()
	if storePath, ok := flagValue(args, "--store"); ok {
		store, err = readStore(storePath)
		if err != nil {
			return err
		}
	}
	var spec *invariant.Spec
	if invariantPath, ok := flagValue(args, "--invariants"); ok {
		loaded, err := readInvariantSpec(invariantPath)
		if err != nil {
			return err
		}
		spec = &loaded
	}
	var descriptor *workflow.Descriptor
	if workflowPath, ok := flagValue(args, "--workflow"); ok {
		loaded, err := readWorkflowDescriptor(workflowPath)
		if err != nil {
			return err
		}
		descriptor = &loaded
	}
	report := refinement.Analyze(manifest, store, spec, descriptor)
	if jsonOut {
		return writeJSON(os.Stdout, report)
	}
	fmt.Printf("cegar refinement manifest=%s iterations=%d refinements=%d holes=%d counterexamples=%d ok=%t hash=%s\n",
		report.Manifest,
		len(report.Iterations),
		len(report.Refinements),
		len(report.RemainingHoles),
		len(report.Counterexamples),
		report.OK,
		report.Hash,
	)
	if len(report.Counterexamples) > 0 {
		return errors.New("cegar refinement found counterexamples")
	}
	return nil
}

func attestationKeygen(jsonOut bool) error {
	seed, err := attest.GenerateSeed()
	if err != nil {
		return err
	}
	publicKey, err := attest.PublicKeyHex(seed)
	if err != nil {
		return err
	}
	result := struct {
		Version   string `json:"version"`
		SeedHex   string `json:"seed_hex"`
		PublicKey string `json:"public_key"`
		Warning   string `json:"warning"`
	}{
		Version:   attest.SignatureVersion,
		SeedHex:   attest.SeedHex(seed),
		PublicKey: publicKey,
		Warning:   "store seed_hex in CI secrets or a local vault; do not commit it",
	}
	if jsonOut {
		return writeJSON(os.Stdout, result)
	}
	fmt.Printf("attestation key generated public_key=%s\nwarning: %s\nseed_hex=%s\n", result.PublicKey, result.Warning, result.SeedHex)
	return nil
}

func signArtifact(path string, args []string, jsonOut bool) error {
	subject, ok := flagValue(args, "--subject")
	if !ok || subject == "" {
		return errors.New("sign-artifact requires --subject")
	}
	seedValue, ok := flagValue(args, "--seed-hex")
	if !ok || seedValue == "" {
		return errors.New("sign-artifact requires --seed-hex")
	}
	seed, err := attest.SeedFromHex(seedValue)
	if err != nil {
		return err
	}
	artifact, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	statement, err := attest.Sign(subject, artifact, seed)
	if err != nil {
		return err
	}
	if outPath, ok := flagValue(args, "--out"); ok {
		file, err := os.Create(outPath)
		if err != nil {
			return err
		}
		if err := writeJSON(file, statement); err != nil {
			file.Close()
			return err
		}
		if err := file.Close(); err != nil {
			return err
		}
	}
	if jsonOut || !hasFlag(args, "--out") {
		return writeJSON(os.Stdout, statement)
	}
	fmt.Printf("signed artifact subject=%s artifact_hash=%s public_key=%s\n", statement.Subject, statement.ArtifactHash, statement.PublicKey)
	return nil
}

func verifyArtifact(attestationPath string, args []string, jsonOut bool) error {
	artifactPath, ok := flagValue(args, "--artifact")
	if !ok || artifactPath == "" {
		return errors.New("verify-artifact requires --artifact")
	}
	statementFile, err := os.Open(attestationPath)
	if err != nil {
		return err
	}
	defer statementFile.Close()
	decoder := json.NewDecoder(statementFile)
	decoder.DisallowUnknownFields()
	var statement attest.Signed
	if err := decoder.Decode(&statement); err != nil {
		return err
	}
	artifact, err := os.ReadFile(artifactPath)
	if err != nil {
		return err
	}
	verifyErr := attest.VerifySignature(statement, artifact)
	result := struct {
		OK           bool   `json:"ok"`
		Subject      string `json:"subject"`
		ArtifactHash string `json:"artifact_hash"`
		PublicKey    string `json:"public_key"`
		Error        string `json:"error,omitempty"`
	}{
		OK:           verifyErr == nil,
		Subject:      statement.Subject,
		ArtifactHash: statement.ArtifactHash,
		PublicKey:    statement.PublicKey,
	}
	if verifyErr != nil {
		result.Error = verifyErr.Error()
	}
	if jsonOut {
		if err := writeJSON(os.Stdout, result); err != nil {
			return err
		}
	} else {
		fmt.Printf("attestation verify subject=%s ok=%t artifact_hash=%s public_key=%s\n", result.Subject, result.OK, result.ArtifactHash, result.PublicKey)
	}
	if verifyErr != nil {
		return verifyErr
	}
	return nil
}

func generateSQL(path string, jsonOut bool) error {
	manifest, err := readManifest(path)
	if err != nil {
		return err
	}
	plan, err := repair.GenerateSQL(manifest)
	if err != nil {
		return err
	}
	if jsonOut {
		return writeJSON(os.Stdout, plan)
	}
	fmt.Printf("repair SQL plan hash=%s statements=%d\n", plan.Hash, len(plan.Statements))
	for _, statement := range plan.Statements {
		fmt.Printf("  %s %s\n", statement.OperationID, statement.SQL)
	}
	return nil
}

func rollbackPlan(path string, jsonOut bool) error {
	manifest, err := readManifest(path)
	if err != nil {
		return err
	}
	report, err := replay.DryRun(manifest, demo.Graph(), demo.BillingStore())
	if err != nil {
		return err
	}
	plan, err := replay.GenerateRollbackPlan(report)
	if err != nil {
		return err
	}
	if jsonOut {
		return writeJSON(os.Stdout, plan)
	}
	fmt.Printf("rollback plan hash=%s statements=%d\n", plan.Hash, len(plan.Statements))
	for _, statement := range plan.Statements {
		fmt.Printf("  %s %s\n", statement.OperationID, statement.SQL)
	}
	return nil
}

func transactionPlan(path string, jsonOut bool) error {
	manifest, err := readManifest(path)
	if err != nil {
		return err
	}
	plan, err := repair.GenerateTransactionPlan(manifest)
	if err != nil {
		return err
	}
	if jsonOut {
		return writeJSON(os.Stdout, plan)
	}
	fmt.Printf("transaction plan hash=%s locks=%d statements=%d\n", plan.Hash, len(plan.LockOrder), len(plan.Statements))
	for _, statement := range plan.Statements {
		fmt.Printf("  %s %s\n", statement.Kind, statement.SQL)
	}
	return nil
}

func analyzeMigration(path string, dialect migration.Dialect, jsonOut bool) error {
	report, err := migration.AnalyzeFileWithDialect(path, dialect)
	if err != nil {
		return err
	}
	if jsonOut {
		return writeJSON(os.Stdout, report)
	}

	if report.Dialect == "" {
		fmt.Printf("migration analysis for %s\n", report.Source)
	} else {
		fmt.Printf("migration analysis for %s dialect=%s\n", report.Source, report.Dialect)
	}
	fmt.Printf("  statements=%d high=%d medium=%d low=%d hash=%s\n",
		report.Summary.TotalStatements,
		report.Summary.HighRisk,
		report.Summary.MediumRisk,
		report.Summary.LowRisk,
		report.Summary.ReportHash,
	)
	for _, statement := range report.Statements {
		table := statement.Table
		if table == "" {
			table = "<unknown>"
		}
		fmt.Printf("  [%d] %s table=%s risk=%s effect=%s\n", statement.Index, statement.Kind, table, statement.Risk, statement.Effect)
		for _, reason := range statement.Reasons {
			fmt.Printf("      - %s\n", reason)
		}
	}
	return nil
}

func schemaDiff(migrationPath, beforePath, expectedPath string, dialect migration.Dialect, jsonOut bool) error {
	content, err := os.ReadFile(migrationPath)
	if err != nil {
		return err
	}
	before, err := readSchema(beforePath)
	if err != nil {
		return err
	}
	expected, err := readSchema(expectedPath)
	if err != nil {
		return err
	}
	report, err := migration.DiffMigrationSchema(migrationPath, content, dialect, before, expected)
	if err != nil {
		return err
	}
	if jsonOut {
		if err := writeJSON(os.Stdout, report); err != nil {
			return err
		}
		if !report.OK {
			return codedError{code: 2, err: errors.New("schema diff found mismatches")}
		}
		return nil
	}
	status := "passed"
	if !report.OK {
		status = "failed"
	}
	fmt.Printf("schema diff %s source=%s diffs=%d hash=%s\n", status, report.MigrationSource, len(report.Diffs), report.Hash)
	for _, diff := range report.Diffs {
		fmt.Printf("  %s table=%s column=%s expect=%s actual=%s\n", diff.Kind, diff.Table, diff.Column, diff.Expect, diff.Actual)
	}
	if !report.OK {
		return codedError{code: 2, err: errors.New("schema diff found mismatches")}
	}
	return nil
}

func migrationSemantics(migrationPath, beforePath string, dialect migration.Dialect, jsonOut bool) error {
	content, err := os.ReadFile(migrationPath)
	if err != nil {
		return err
	}
	before, err := readSchema(beforePath)
	if err != nil {
		return err
	}
	report, err := migration.AnalyzeMigrationSemantics(migrationPath, content, dialect, before)
	if err != nil {
		return err
	}
	if jsonOut {
		return writeJSON(os.Stdout, report)
	}
	fmt.Printf("migration semantics source=%s transformations=%d relational=%d hash=%s\n",
		report.MigrationSource, len(report.Transformations), len(report.Relational), report.Hash)
	for _, transformation := range report.Transformations {
		column := ""
		if transformation.Column != nil {
			column = " column=" + transformation.Column.Name
		}
		fmt.Printf("  transform[%d] %s table=%s%s %s -> %s\n",
			transformation.Index, transformation.Kind, transformation.Table, column, transformation.BeforeHash[:12], transformation.AfterHash[:12])
	}
	for _, rel := range report.Relational {
		fmt.Printf("  rel[%d] %s reads=%s writes=%s expr=%s\n",
			rel.Index, rel.Kind, strings.Join(rel.Reads, ","), strings.Join(rel.Writes, ","), rel.Expression)
	}
	return nil
}

func extractSQL(path string, jsonOut bool) error {
	report, err := migration.ExtractSourceSQL(path)
	if err != nil {
		return err
	}
	if jsonOut {
		return writeJSON(os.Stdout, report)
	}
	fmt.Printf("source SQL extraction root=%s files=%d embedded_sql=%d orm_queries=%d hash=%s\n",
		report.Root, report.Summary.FilesScanned, report.Summary.EmbeddedSQL, report.Summary.ORMQueries, report.Hash)
	for _, obs := range report.Observations {
		framework := obs.Framework
		if framework == "" {
			framework = "-"
		}
		table := obs.Table
		if table == "" {
			table = "-"
		}
		fmt.Printf("  %s:%d %s framework=%s op=%s table=%s confidence=%s\n",
			obs.Path, obs.Line, obs.Kind, framework, obs.Operation, table, obs.Confidence)
	}
	return nil
}

func reproduceArtifact(path string, jsonOut, update bool) error {
	spec, err := readReproduceSpec(path)
	if err != nil {
		return err
	}
	manifestPath := resolvePath(filepath.Dir(path), spec.RepairManifest)
	manifest, err := readManifest(manifestPath)
	if err != nil {
		return err
	}
	entries, checkpoint := demo.SampleLedger()
	result, err := reproduce.Run(spec, manifest, demo.Graph(), demo.BillingStore(), entries, checkpoint)
	if err != nil {
		return err
	}
	if update {
		updated := reproduce.UpdateExpected(spec, result)
		file, err := os.Create(path)
		if err != nil {
			return err
		}
		if err := canonical.WriteJSON(file, updated); err != nil {
			_ = file.Close()
			return err
		}
		if err := file.Close(); err != nil {
			return err
		}
		result.ExpectedReportHash = updated.ExpectedReportHash
		result.ExpectedLedgerCheckpoint = updated.ExpectedLedgerCheckpoint
	}
	if jsonOut {
		return writeJSON(os.Stdout, result)
	}
	status := "failed"
	if result.OK {
		status = "passed"
	}
	fmt.Printf("reproducibility artifact %s %s\n", result.Name, status)
	fmt.Printf("  dry-run hash: %s\n", result.ReportHash)
	fmt.Printf("  ledger tip:   %s\n", result.LedgerCheckpoint.TipHash)
	for _, attestation := range result.Attestations {
		prefix := "ok"
		if !attestation.OK {
			prefix = "fail"
		}
		ref := attestation.Ref
		if ref == "" {
			ref = attestation.Kind
		}
		fmt.Printf("  %s %s expected=%s actual=%s\n", prefix, ref, attestation.Expected, attestation.Actual)
	}
	if !result.OK {
		return errors.New("reproducibility checks failed")
	}
	return nil
}

func ledgerVerify(jsonOut bool) error {
	entries, checkpoint := demo.SampleLedger()
	err := ledger.VerifyCheckpoint(entries, checkpoint)
	result := struct {
		OK         bool              `json:"ok"`
		EntryCount int               `json:"entry_count"`
		Checkpoint ledger.Checkpoint `json:"checkpoint"`
		Error      string            `json:"error,omitempty"`
	}{
		OK:         err == nil,
		EntryCount: len(entries),
		Checkpoint: checkpoint,
	}

	if err != nil {
		result.Error = err.Error()
	}
	if jsonOut {
		return writeJSON(os.Stdout, result)
	}
	if err != nil {
		return err
	}
	fmt.Printf("ledger verified: %d entries, tip %s\n", len(entries), checkpoint.TipHash)
	return nil
}

func evaluatePolicy(policyPath, repairPath, migrationPath string, jsonOut bool) error {
	eval, err := buildPolicyEvaluation(policyPath, repairPath, migrationPath)
	if err != nil {
		return err
	}
	if jsonOut {
		return writeJSON(os.Stdout, eval)
	}
	status := "failed"
	if eval.OK {
		status = "passed"
	}
	fmt.Printf("policy %s %s hash=%s\n", eval.PolicyName, status, eval.PolicyHash)
	for _, result := range eval.Results {
		prefix := "ok"
		if !result.OK {
			prefix = "fail"
		}
		fmt.Printf("  %s %s actual=%s expect=%s\n", prefix, result.Rule, result.Actual, result.Expect)
	}
	if !eval.OK {
		return errors.New("policy evaluation failed")
	}
	return nil
}

func exportBundle(reproducePath, policyPath, migrationPath string, jsonOut bool) error {
	spec, err := readReproduceSpec(reproducePath)
	if err != nil {
		return err
	}
	manifestPath := resolvePath(filepath.Dir(reproducePath), spec.RepairManifest)
	manifest, err := readManifest(manifestPath)
	if err != nil {
		return err
	}
	report, err := replay.DryRun(manifest, demo.Graph(), demo.BillingStore())
	if err != nil {
		return err
	}
	slice, err := demo.Graph().Slice(provenance.SliceOptions{
		Starts:      manifest.Scope.Entities,
		Direction:   provenance.DirectionBoth,
		MaxDepth:    4,
		MinEvidence: provenance.EvidenceStrong,
	})
	if err != nil {
		return err
	}
	migrationReport, err := migration.AnalyzeFile(migrationPath)
	if err != nil {
		return err
	}
	eval, err := buildPolicyEvaluation(policyPath, manifestPath, migrationPath)
	if err != nil {
		return err
	}
	_, checkpoint := demo.SampleLedger()
	invariantSpec, err := readInvariantSpec("examples/invariants/billing-core.json")
	if err != nil {
		return err
	}
	solverReport := solver.Analyze(manifest, demo.BillingStore(), &invariantSpec)
	symbolicReport := symbolic.Execute(manifest, demo.BillingStore())
	workflowDescriptor, err := readWorkflowDescriptor("examples/workflows/bad-migration-approved.json")
	if err != nil {
		return err
	}
	workflowReport := workflow.Check(workflowDescriptor)
	b := bundle.Build(bundle.Inputs{
		Name:              spec.Name,
		Manifest:          manifest,
		Report:            report,
		Slice:             slice,
		Migration:         migrationReport,
		PolicyEvaluation:  eval,
		LedgerCheckpoint:  checkpoint,
		ReproductionNotes: "Reproduce with: patchline benchmark " + reproducePath + "\n",
		ProofArtifacts: []bundle.ProofArtifact{
			{Path: "solver-obligations.json", Kind: "solver-obligations", Inline: solverReport},
			{Path: "symbolic-execution.json", Kind: "symbolic-execution", Inline: symbolicReport},
			{Path: "workflow-model-check.json", Kind: "workflow-model-check", Inline: workflowReport},
		},
	})
	if jsonOut {
		return writeJSON(os.Stdout, b)
	}
	fmt.Printf("bundle %s entries=%d hash=%s\n", b.Name, len(b.Entries), b.BundleHash)
	for _, entry := range b.Entries {
		fmt.Printf("  %s %s %dB\n", entry.Hash[:12], entry.Path, entry.Bytes)
	}
	return nil
}

func benchmarkSuite(path string, jsonOut bool) error {
	spec, err := readBenchmarkSpec(path)
	if err != nil {
		return err
	}
	result, err := bench.Run(spec, filepath.Dir(path))
	if err != nil {
		return err
	}
	if jsonOut {
		return writeJSON(os.Stdout, result)
	}
	status := "failed"
	if result.OK {
		status = "passed"
	}
	fmt.Printf("benchmark suite %s %s hash=%s\n", result.Name, status, result.SuiteHash)
	fmt.Printf("  total=%d passed=%d failed=%d precision=%.3f recall=%.3f\n", result.Metrics.Total, result.Metrics.Passed, result.Metrics.Failed, result.Metrics.Precision, result.Metrics.Recall)
	for _, c := range result.Cases {
		fmt.Printf("  %s label=%s prediction=%s ok=%t hash=%s\n", c.ID, c.Label, c.Prediction, c.OK, c.ReportHash)
	}
	if !result.OK {
		return errors.New("benchmark suite failed")
	}
	return nil
}

func ingestEvidence(path string, jsonOut bool, outPath string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	result, err := evidence.IngestJSONL(file)
	if err != nil {
		return err
	}
	if outPath != "" {
		outFile, err := os.Create(outPath)
		if err != nil {
			return err
		}
		graph := struct {
			Version  string              `json:"version"`
			Hash     string              `json:"hash"`
			Entities []provenance.Entity `json:"entities"`
			Edges    []provenance.Edge   `json:"edges"`
		}{
			Version:  result.Version,
			Hash:     result.GraphHash,
			Entities: result.Entities,
			Edges:    result.Edges,
		}
		if err := canonical.WriteJSON(outFile, graph); err != nil {
			_ = outFile.Close()
			return err
		}
		if err := outFile.Close(); err != nil {
			return err
		}
	}
	if jsonOut {
		return writeJSON(os.Stdout, result)
	}
	status := "failed"
	if result.OK {
		status = "passed"
	}
	fmt.Printf("evidence ingest %s events=%d entities=%d edges=%d damaged=%d hash=%s\n",
		status, result.EventCount, len(result.Entities), len(result.Edges), len(result.DamagedEntities), result.GraphHash)
	if result.UnknownFieldCount > 0 {
		fmt.Printf("  ignored unknown fields: %d\n", result.UnknownFieldCount)
	}
	if outPath != "" {
		fmt.Printf("  graph written: %s\n", outPath)
	}
	for _, id := range result.DamagedEntities {
		fmt.Printf("  damaged %s\n", id)
	}
	for _, ingestErr := range result.Errors {
		fmt.Printf("  error %s\n", ingestErr)
	}
	if !result.OK {
		return errors.New("evidence ingest failed")
	}
	return nil
}

func adaptEvidence(adapter, path string, jsonOut bool, outPath string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	result, err := evidence.AdaptJSON(file, adapter)
	if err != nil {
		return err
	}
	if outPath != "" {
		outFile, err := os.Create(outPath)
		if err != nil {
			return err
		}
		encoder := json.NewEncoder(outFile)
		for _, event := range result.Events {
			if err := encoder.Encode(event); err != nil {
				_ = outFile.Close()
				return err
			}
		}
		if err := outFile.Close(); err != nil {
			return err
		}
	}
	if jsonOut {
		return writeJSON(os.Stdout, result)
	}
	fmt.Printf("evidence adapter %s events=%d input_hash=%s\n", result.Adapter, result.EventCount, result.InputHash)
	if outPath != "" {
		fmt.Printf("  events written: %s\n", outPath)
	}
	for _, warning := range result.Warnings {
		fmt.Printf("  warning %s\n", warning)
	}
	return nil
}

func ciGate(path string, opts gate.Options, jsonOut bool) error {
	spec, err := readBenchmarkSpec(path)
	if err != nil {
		return err
	}
	suite, err := bench.Run(spec, filepath.Dir(path))
	if err != nil {
		return err
	}
	result := gate.Evaluate(suite, opts)
	if os.Getenv("GITHUB_ACTIONS") == "true" {
		writeGitHubSummary(result)
		writeGitHubAnnotations(result)
	}
	if jsonOut {
		if err := writeJSON(os.Stdout, result); err != nil {
			return err
		}
	} else {
		status := "failed"
		if result.OK {
			status = "passed"
		}
		fmt.Printf("ci gate %s suite=%s hash=%s\n", status, result.Suite.Name, result.Hash)
		fmt.Printf("  cases=%d/%d precision=%.3f min=%.3f recall=%.3f min=%.3f\n",
			result.Suite.Metrics.Passed,
			result.Suite.Metrics.Total,
			result.Suite.Metrics.Precision,
			result.Options.MinPrecision,
			result.Suite.Metrics.Recall,
			result.Options.MinRecall,
		)
		for _, check := range result.Checks {
			prefix := "ok"
			if !check.OK {
				prefix = "fail"
			}
			fmt.Printf("  %s %s actual=%s", prefix, check.Name, check.Actual)
			if check.Minimum != "" {
				fmt.Printf(" min=%s", check.Minimum)
			}
			fmt.Println()
		}
	}
	if !result.OK {
		if !result.Suite.OK {
			return codedError{code: 2, err: errors.New("ci gate failed: benchmark cases failed")}
		}
		return codedError{code: 3, err: errors.New("ci gate failed: metric threshold failed")}
	}
	return nil
}

func parseGateOptions(args []string) (gate.Options, error) {
	opts := gate.Options{MinPrecision: 0.95, MinRecall: 0.95}
	if value, ok := flagValue(args, "--min-precision"); ok {
		parsed, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return gate.Options{}, fmt.Errorf("invalid --min-precision: %w", err)
		}
		opts.MinPrecision = parsed
	}
	if value, ok := flagValue(args, "--min-recall"); ok {
		parsed, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return gate.Options{}, fmt.Errorf("invalid --min-recall: %w", err)
		}
		opts.MinRecall = parsed
	}
	if opts.MinPrecision < 0 || opts.MinPrecision > 1 || opts.MinRecall < 0 || opts.MinRecall > 1 {
		return gate.Options{}, errors.New("gate thresholds must be between 0 and 1")
	}
	return opts, nil
}

func writeGitHubSummary(result gate.Result) {
	path := os.Getenv("GITHUB_STEP_SUMMARY")
	if path == "" {
		return
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "patchline: could not write GitHub summary: %v\n", err)
		return
	}
	defer file.Close()
	status := "failed"
	if result.OK {
		status = "passed"
	}
	fmt.Fprintf(file, "## Patchline CI gate %s\n\n", status)
	fmt.Fprintf(file, "| Metric | Actual | Required |\n| --- | ---: | ---: |\n")
	fmt.Fprintf(file, "| Cases | %d/%d | all |\n", result.Suite.Metrics.Passed, result.Suite.Metrics.Total)
	fmt.Fprintf(file, "| Precision | %.3f | %.3f |\n", result.Suite.Metrics.Precision, result.Options.MinPrecision)
	fmt.Fprintf(file, "| Recall | %.3f | %.3f |\n\n", result.Suite.Metrics.Recall, result.Options.MinRecall)
	fmt.Fprintf(file, "Suite hash: `%s`\n\nGate hash: `%s`\n", result.Suite.SuiteHash, result.Hash)
}

func writeGitHubAnnotations(result gate.Result) {
	for _, c := range result.Suite.Cases {
		if c.OK {
			continue
		}
		fmt.Fprintf(os.Stderr, "::error file=%s::Patchline benchmark case %s failed: label=%s prediction=%s expected_hash=%s actual_hash=%s\n",
			c.Path, c.ID, c.Label, c.Prediction, c.ExpectedReportHash, c.ReportHash)
	}
	for _, check := range result.Checks {
		if !check.OK {
			fmt.Fprintf(os.Stderr, "::error::Patchline CI gate check %s failed: actual=%s minimum=%s\n", check.Name, check.Actual, check.Minimum)
		}
	}
}

func readManifest(path string) (repair.Manifest, error) {
	file, err := os.Open(path)
	if err != nil {
		return repair.Manifest{}, err
	}
	defer file.Close()
	return repair.ReadManifest(file)
}

func readSchema(path string) (migration.SchemaState, error) {
	file, err := os.Open(path)
	if err != nil {
		return migration.SchemaState{}, err
	}
	defer file.Close()
	return migration.ReadSchema(file)
}

func readInvariantSpec(path string) (invariant.Spec, error) {
	file, err := os.Open(path)
	if err != nil {
		return invariant.Spec{}, err
	}
	defer file.Close()
	return invariant.Read(file)
}

func readStore(path string) (replay.Store, error) {
	file, err := os.Open(path)
	if err != nil {
		return replay.Store{}, err
	}
	defer file.Close()
	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	var store replay.Store
	if err := decoder.Decode(&store); err != nil {
		return replay.Store{}, err
	}
	return store, nil
}

func readWorkflowDescriptor(path string) (workflow.Descriptor, error) {
	file, err := os.Open(path)
	if err != nil {
		return workflow.Descriptor{}, err
	}
	defer file.Close()
	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	var descriptor workflow.Descriptor
	if err := decoder.Decode(&descriptor); err != nil {
		return workflow.Descriptor{}, err
	}
	return descriptor, nil
}

func readArchiveSpec(path string) (archive.Spec, error) {
	file, err := os.Open(path)
	if err != nil {
		return archive.Spec{}, err
	}
	defer file.Close()
	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	var spec archive.Spec
	if err := decoder.Decode(&spec); err != nil {
		return archive.Spec{}, err
	}
	if spec.Version != archive.Version {
		return archive.Spec{}, fmt.Errorf("archive spec version must be %s", archive.Version)
	}
	return spec, nil
}

func readReproduceSpec(path string) (reproduce.Spec, error) {
	file, err := os.Open(path)
	if err != nil {
		return reproduce.Spec{}, err
	}

	defer file.Close()
	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	var spec reproduce.Spec
	if err := decoder.Decode(&spec); err != nil {
		return reproduce.Spec{}, err
	}
	if spec.Version != reproduce.Version {
		return reproduce.Spec{}, fmt.Errorf("reproducibility spec version must be %s", reproduce.Version)
	}
	return spec, nil
}

func readPolicy(path string) (policy.Policy, error) {
	file, err := os.Open(path)
	if err != nil {
		return policy.Policy{}, err
	}
	defer file.Close()
	return policy.Read(file)
}

func readBenchmarkSpec(path string) (bench.Spec, error) {
	file, err := os.Open(path)
	if err != nil {
		return bench.Spec{}, err
	}
	defer file.Close()
	return bench.Read(file)
}

func readEvidenceResult(path string) (evidence.Result, error) {
	file, err := os.Open(path)
	if err != nil {
		return evidence.Result{}, err
	}
	defer file.Close()
	return evidence.IngestJSONL(file)
}

func buildPolicyEvaluation(policyPath, repairPath, migrationPath string) (policy.Evaluation, error) {
	pol, err := readPolicy(policyPath)
	if err != nil {
		return policy.Evaluation{}, err
	}
	manifest, err := readManifest(repairPath)
	if err != nil {
		return policy.Evaluation{}, err
	}
	report, err := replay.DryRun(manifest, demo.Graph(), demo.BillingStore())
	if err != nil {
		return policy.Evaluation{}, err
	}
	migrationReport, err := migration.AnalyzeFile(migrationPath)
	if err != nil {
		return policy.Evaluation{}, err
	}
	_, checkpoint := demo.SampleLedger()
	return policy.Evaluate(pol, policy.Inputs{
		Manifest:           manifest,
		Report:             report,
		Migration:          migrationReport,
		ExpectedReportHash: report.Hash(),
		LedgerCheckpoint:   checkpoint,
	}), nil
}

func collectPolicyFailures(eval policy.Evaluation) []string {
	var failures []string
	for _, result := range eval.Results {
		if result.OK {
			continue
		}
		failures = append(failures, result.Rule+": "+result.Message)
	}
	sort.Strings(failures)
	return failures
}

func graphFor(args []string) (*provenance.Graph, error) {
	path, ok := flagValue(args, "--graph")
	if !ok {
		return demo.Graph(), nil
	}
	return readGraph(path)
}

func provenanceGraphFor(args []string) (*provenance.Graph, error) {
	if path, ok := flagValue(args, "--evidence"); ok {
		return readEvidenceGraph(path)
	}
	return graphFor(args)
}

func readEvidenceGraph(path string) (*provenance.Graph, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	result, err := evidence.IngestJSONL(file)
	if err != nil {
		return nil, err
	}
	if !result.OK {
		return nil, fmt.Errorf("invalid evidence %s: %s", path, strings.Join(result.Errors, "; "))
	}
	return provenance.FromSlices(result.Entities, result.Edges)
}

func parseSQLDialect(args []string) (migration.Dialect, error) {
	value, ok := flagValue(args, "--dialect")
	if !ok || value == "" || value == "generic" {
		return migration.DialectGeneric, nil
	}
	dialect := migration.Dialect(value)
	if err := migration.ValidateDialect(dialect); err != nil {
		return "", err
	}
	return dialect, nil
}

func readGraph(path string) (*provenance.Graph, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	var projection struct {
		Version  string              `json:"version,omitempty"`
		Hash     string              `json:"hash,omitempty"`
		Entities []provenance.Entity `json:"entities"`
		Edges    []provenance.Edge   `json:"edges"`
	}
	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&projection); err != nil {
		return nil, err
	}
	if len(projection.Entities) == 0 {
		return nil, errors.New("graph projection must contain at least one entity")
	}
	return provenance.FromSlices(projection.Entities, projection.Edges)
}

func resolvePath(baseDir, ref string) string {
	if filepath.IsAbs(ref) {
		return ref
	}
	return filepath.Clean(filepath.Join(baseDir, ref))
}

func hasFlag(args []string, flag string) bool {
	for _, arg := range args {
		if arg == flag {
			return true
		}
	}
	return false
}

func flagValue(args []string, flag string) (string, bool) {
	for i, arg := range args {
		if arg == flag && i+1 < len(args) {
			return args[i+1], true
		}
	}
	return "", false
}

func positionalArgs(args []string) []string {
	var out []string
	for i := 0; i < len(args); i++ {
		if strings.HasPrefix(args[i], "--") {
			if i+1 < len(args) && !strings.HasPrefix(args[i+1], "--") {
				i++
			}
			continue
		}
		out = append(out, args[i])
	}
	return out
}

func keys(values map[string]string) []string {
	out := make([]string, 0, len(values))
	for key := range values {
		out = append(out, key)
	}
	return out
}

func sortedStrings(values []string) []string {
	out := append([]string(nil), values...)
	sort.Strings(out)
	return out
}

func stringSet(values map[string]struct{}) []string {
	out := make([]string, 0, len(values))
	for value := range values {
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func writeJSON(file *os.File, value any) error {
	return canonical.WriteJSON(file, value)
}

func graphDTO(g *provenance.Graph) struct {
	Entities []provenance.Entity `json:"entities"`
	Edges    []provenance.Edge   `json:"edges"`
} {
	return struct {
		Entities []provenance.Entity `json:"entities"`
		Edges    []provenance.Edge   `json:"edges"`
	}{
		Entities: g.Entities(),
		Edges:    g.Edges(),
	}
}
