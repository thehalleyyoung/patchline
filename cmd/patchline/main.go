package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/thehalleyyoung/patchline/internal/archive"
	"github.com/thehalleyyoung/patchline/internal/artifact"
	"github.com/thehalleyyoung/patchline/internal/attest"
	"github.com/thehalleyyoung/patchline/internal/bench"
	"github.com/thehalleyyoung/patchline/internal/bundle"
	"github.com/thehalleyyoung/patchline/internal/canonical"
	"github.com/thehalleyyoung/patchline/internal/dbdryrun"
	"github.com/thehalleyyoung/patchline/internal/demo"
	"github.com/thehalleyyoung/patchline/internal/diagnostics"
	"github.com/thehalleyyoung/patchline/internal/effects"
	"github.com/thehalleyyoung/patchline/internal/evidence"
	"github.com/thehalleyyoung/patchline/internal/gate"
	"github.com/thehalleyyoung/patchline/internal/goldenfixture"
	"github.com/thehalleyyoung/patchline/internal/historical"
	"github.com/thehalleyyoung/patchline/internal/intake"
	"github.com/thehalleyyoung/patchline/internal/invariant"
	"github.com/thehalleyyoung/patchline/internal/ledger"
	"github.com/thehalleyyoung/patchline/internal/migration"
	"github.com/thehalleyyoung/patchline/internal/plugins"
	"github.com/thehalleyyoung/patchline/internal/policy"
	"github.com/thehalleyyoung/patchline/internal/project"
	"github.com/thehalleyyoung/patchline/internal/proof"
	"github.com/thehalleyyoung/patchline/internal/provenance"
	"github.com/thehalleyyoung/patchline/internal/refinement"
	"github.com/thehalleyyoung/patchline/internal/repair"
	"github.com/thehalleyyoung/patchline/internal/replay"
	"github.com/thehalleyyoung/patchline/internal/reproduce"
	"github.com/thehalleyyoung/patchline/internal/semantics"
	"github.com/thehalleyyoung/patchline/internal/solver"
	"github.com/thehalleyyoung/patchline/internal/symbolic"
	"github.com/thehalleyyoung/patchline/internal/workflow"
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
		fmt.Println("Patchline is a deterministic checker for the migration SQL, source SQL, telemetry exports, deploy metadata, and repair artifacts you already have.")
		fmt.Println("Start with: patchline intake <repo-or-export-dir> --out results/generated/intake")
		fmt.Println("RiSE angle: program analysis + verifiable transformations + reproducible repair benchmarks, without AI.")
	case "intake":
		return currentIntake(args[1:])
	case "doctor":
		return repoDoctor(args[1:])
	case "quickstart":
		return quickstart(args[1:])
	case "repo":
		return repoCommand(args[1:])
	case "plugins":
		return pluginsCommand(args[1:])
	case "golden-fixture":
		return goldenFixtureCommand(args[1:])
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
			return errors.New("usage: patchline archive-query <archive-spec.json> [broad-updates|damaged-reports|missing-rollback|repair-outcomes|semantic-regressions|all] [--json]")
		}
		query := "all"
		if len(args) >= 3 && !strings.HasPrefix(args[2], "--") {
			query = args[2]
		}
		return archiveQuery(args[1], query, hasFlag(args[2:], "--json"))
	case "repair-outcomes":
		if len(args) < 2 {
			return errors.New("usage: patchline repair-outcomes <archive-spec.json> [--json]")
		}
		return repairOutcomes(args[1], hasFlag(args[2:], "--json"))
	case "semantic-regressions":
		if len(args) < 2 {
			return errors.New("usage: patchline semantic-regressions <archive-spec.json> [--json]")
		}
		return semanticRegressions(args[1], hasFlag(args[2:], "--json"))
	case "historical-failures":
		if len(args) < 2 {
			return errors.New("usage: patchline historical-failures <suite.json> [--json]")
		}
		return historicalFailures(args[1], hasFlag(args[2:], "--json"))
	case "demo-graph":
		return writeJSON(os.Stdout, graphDTO(demo.Graph()))
	case "explain", "trace-row":
		if len(args) < 2 {
			return errors.New("usage: patchline explain <entity-id> [--graph graph.json] [--analysis analysis-dir] [--json]")
		}
		return explainCommand(args[1:])
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
			return fmt.Errorf("usage: patchline analyze-migration <migration.sql> [--dialect %s] [--json]", sqlDialectUsage())
		}
		dialect, err := parseSQLDialect(args[2:])
		if err != nil {
			return err
		}
		return analyzeMigration(args[1], dialect, hasFlag(args[2:], "--json"))
	case "schema-diff":
		if len(args) < 4 {
			return fmt.Errorf("usage: patchline schema-diff <migration.sql> <before-schema.json> <expected-schema.json> [--dialect %s] [--json]", sqlDialectUsage())
		}
		dialect, err := parseSQLDialect(args[4:])
		if err != nil {
			return err
		}
		return schemaDiff(args[1], args[2], args[3], dialect, hasFlag(args[4:], "--json"))
	case "migration-semantics":
		if len(args) < 3 {
			return fmt.Errorf("usage: patchline migration-semantics <migration.sql> <before-schema.json> [--dialect %s] [--json]", sqlDialectUsage())
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
	case "db-dry-run":
		if len(args) < 2 {
			return errors.New("usage: patchline db-dry-run <manifest.json> --dialect <postgres|mysql> [--dsn local-dsn] [--execute] [--json]")
		}
		return dbDryRun(args[1], args[2:], hasFlag(args[2:], "--json"))
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
	case "artifact-ground-truth":
		root := "benchmarks"
		if len(args) >= 2 && !strings.HasPrefix(args[1], "--") {
			root = args[1]
		}
		return artifactGroundTruth(root, hasFlag(args[1:], "--json"))
	case "phase-check":
		if len(args) < 2 {
			return errors.New("usage: patchline phase-check <manifest.json> [--json]")
		}
		return phaseCheck(args[1], hasFlag(args[2:], "--json"))
	case "artifact-baselines":
		if len(args) < 2 {
			return errors.New("usage: patchline artifact-baselines <suite.json> [--out dir] [--json]")
		}
		outPath, _ := flagValue(args[2:], "--out")
		return artifactBaselines(args[1], outPath, hasFlag(args[2:], "--json"))
	case "artifact-ablations":
		if len(args) < 2 {
			return errors.New("usage: patchline artifact-ablations <suite.json> [--out dir] [--json]")
		}
		outPath, _ := flagValue(args[2:], "--out")
		return artifactAblations(args[1], outPath, hasFlag(args[2:], "--json"))
	case "artifact-scale":
		if len(args) < 2 {
			return errors.New("usage: patchline artifact-scale <suite.json> [--out dir] [--json]")
		}
		outPath, _ := flagValue(args[2:], "--out")
		return artifactScale(args[1], outPath, hasFlag(args[2:], "--json"))
	case "artifact-benchmark":
		if len(args) < 3 {
			return errors.New("usage: patchline artifact-benchmark <validate|run|compare> <args> [--json] [--out path]")
		}
		return artifactBenchmark(args[1:])
	case "artifact-study":
		if len(args) < 3 {
			return errors.New("usage: patchline artifact-study <summarize|compare> <args> [--json] [--out path]")
		}
		return artifactStudy(args[1:])
	case "artifact-tables":
		return artifactTables(args[1:])
	case "artifact-numbers":
		return artifactNumbers(args[1:])
	case "artifact-subtasks":
		return artifactSubtasks(args[1:])
	case "artifact-corpus-audit":
		return artifactCorpusAudit(args[1:])
	case "artifact-provenance":
		return artifactProvenance(args[1:])
	case "ingest-evidence":
		if len(args) < 2 {
			return errors.New("usage: patchline ingest-evidence <events.jsonl> [--json] [--out graph.json]")
		}
		outPath, _ := flagValue(args[2:], "--out")
		return ingestEvidence(args[1], hasFlag(args[2:], "--json"), outPath)
	case "adapt-evidence":
		if len(args) < 3 {
			return errors.New("usage: patchline adapt-evidence <otlp|datadog|postgres|github|migration-runner|jira|linear> <input.json> [--json] [--out events.jsonl]")
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
  patchline doctor [<path>|--github owner/repo] [--ref ref] [--subpath path] [--out dir] [--download-dir dir] [--json]
  patchline quickstart --github owner/repo --subpath path [--ref ref] [--out dir] [--json]
  patchline plugins list [--json]
  patchline plugins probe [<path>|--github owner/repo] [--ref ref] [--subpath path] [--out dir] [--download-dir dir] [--json]
  patchline golden-fixture generate [<path>|--github owner/repo] [--ref ref] [--subpath path] --out dir [--max-files n] [--json]
  patchline repo doctor [<path>|--github owner/repo] [--ref ref] [--subpath path] [--out dir] [--download-dir dir] [--json]
  patchline repo fetch <owner/repo|github-url|path|archive> [--ref ref] [--subpath path] [--out dir] [--download-dir dir] [--json]
  patchline repo analyze [<path>|--github owner/repo] [--ref ref] [--subpath path] [--stages inventory,baseline,propose,compare,deep] [--proposal-kind tests|guards|instrumentation|repair|all] [--budget files=N,lines=N,tokens=N,changes=N] [--ci] [--redact] [--resume] [--trace] [--no-llm] [--llm-command cmd] [--prompt-without-facts] [--out dir] [--json]
  patchline repo inventory <path> [--out dir] [--full] [--json]
  patchline repo baseline --inventory inventory-dir --intake intake-dir [--out dir] [--json]
  patchline repo propose --from-report baseline-dir --proposal-kind tests|guards|instrumentation|repair|all [--budget files=N,lines=N,tokens=N,changes=N] [--no-llm] [--llm-command cmd] [--prompt-without-facts] [--out dir] [--json]
  patchline repo compare --before baseline-dir --after proposal-dir [--out dir] [--run-native-tests] [--json]
  patchline repo proposal-minimize --before baseline-dir --after proposal-dir [--out dir] [--json]
  patchline repo replay --analysis analysis-dir [--out dir] [--json]
  patchline repo suppressions --baseline baseline-dir --suppressions suppressions.json [--out dir] [--json]
  patchline repo why-now --previous baseline-dir --current baseline-dir [--out dir] [--json]
  patchline repo changes --previous analysis-dir --current analysis-dir [--out dir] [--json]
  patchline repo hook <pre-commit|pre-push> [--root repo] [--base ref] [--out dir] [--json]
  patchline repo offline --analysis analysis-dir [--adapter adapter-result.json]... [--out dir] [--json]
  patchline repo pr-comment --base baseline-dir --head baseline-dir [--max-findings n] [--out dir] [--json]
  patchline repo notify-summary --analysis analysis-dir [--bundle-link url] [--out dir] [--json]
  patchline repo minimize --analysis analysis-dir [--out dir] [--json]
  patchline repo recurrence --analyses analysis-dir[,analysis-dir...] [--out dir] [--json]
  patchline intake <path> [--out results/generated/intake] [--json]
  patchline intake --github owner/repo [--ref ref] [--subpath path] [--out results/generated/intake] [--json]
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
  patchline archive-query <archive-spec.json> [broad-updates|damaged-reports|missing-rollback|repair-outcomes|semantic-regressions|all] [--json]
  patchline repair-outcomes <archive-spec.json> [--json]
  patchline semantic-regressions <archive-spec.json> [--json]
  patchline historical-failures <suite.json> [--json]
  patchline demo-graph
  patchline explain <entity-id> [--graph graph.json] [--analysis analysis-dir] [--json]
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
  patchline analyze-migration <migration.sql> [--dialect generic|postgres|mysql|sqlite|sqlserver|oracle|bigquery] [--json]
  patchline schema-diff <migration.sql> <before-schema.json> <expected-schema.json> [--dialect generic|postgres|mysql|sqlite|sqlserver|oracle|bigquery] [--json]
  patchline migration-semantics <migration.sql> <before-schema.json> [--dialect generic|postgres|mysql|sqlite|sqlserver|oracle|bigquery] [--json]
  patchline extract-sql <path> [--json]
  patchline migration-outcomes <evidence.jsonl> <migration.sql> [--repair manifest.json] [--policy policy.json] [--benchmark suite.json] [--source-sql path] [--json]
  patchline dry-run <manifest.json> [--store store.json] [--json]
  patchline db-dry-run <manifest.json> --dialect <postgres|mysql> [--dsn local-dsn] [--execute] [--json]
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
  patchline artifact-ground-truth [benchmarks-dir] [--json]
  patchline phase-check <manifest.json> [--json]
  patchline artifact-baselines <suite.json> [--out dir] [--json]
  patchline artifact-ablations <suite.json> [--out dir] [--json]
  patchline artifact-scale <suite.json> [--out dir] [--json]
  patchline artifact-study summarize <report-dir> [--out expected.json] [--json]
  patchline artifact-study compare <report-dir> <expected.json> [--json]
  patchline artifact-tables [--root repo-root] [--out results/generated/artifact-tables] [--json]
  patchline artifact-numbers [--root repo-root] [--out results/generated/artifact-numbers] [--json]
  patchline artifact-subtasks [--root repo-root] [--out results/generated/artifact-subtasks] [--json]
  patchline artifact-corpus-audit [--root repo-root] [--protocol benchmarks/corpus_protocol.json] [--out results/generated/artifact-corpus-audit] [--json]
  patchline artifact-provenance [--root repo-root] [--out results/generated/artifact-provenance] [--json]
  patchline artifact-benchmark validate <manifest.json> [--json]
  patchline artifact-benchmark run <manifest.json> [--out report.json] [--json]
  patchline artifact-benchmark compare <actual.json> <expected.json> [--json]
  patchline ingest-evidence <events.jsonl> [--json] [--out graph.json]
  patchline adapt-evidence <otlp|datadog|postgres|github|migration-runner|jira|linear> <input.json> [--json] [--out events.jsonl]
  patchline ci-gate <suite.json> [--min-precision 0.95] [--min-recall 0.95] [--json]
  patchline ledger-verify [--json]

Examples:
  patchline repo fetch bytebase/bytebase --subpath backend/migrator/migration --out results/generated/repos/bytebase-migrations
  patchline repo analyze --github bytebase/bytebase --subpath backend/migrator/migration --stages inventory,baseline,propose,compare,deep --proposal-kind all --out results/generated/repos/bytebase-analysis
  patchline repo inventory results/generated/repos/bytebase-migrations --out results/generated/repos/bytebase-migrations/inventory
  patchline repo baseline --inventory results/generated/repos/bytebase-migrations/inventory --intake results/generated/intake --out results/generated/repos/bytebase-migrations/baseline
  patchline repo propose --from-report results/generated/repos/bytebase-migrations/baseline --proposal-kind all --out results/generated/repos/bytebase-migrations/proposal
  patchline repo compare --before results/generated/repos/bytebase-migrations/baseline --after results/generated/repos/bytebase-migrations/proposal --out results/generated/repos/bytebase-migrations/compare
  patchline intake . --out results/generated/intake
  patchline intake --github bytebase/bytebase --subpath store/migration --out results/generated/intake
  patchline explain record:invoices/inv_1002
  patchline semantics-audit --json
  patchline trace-reconstruct examples/incidents/bad-migration.jsonl
  patchline provenance certificate record:invoices/inv_1002 --evidence examples/incidents/bad-migration.jsonl
  patchline analyze-migration demos/billing/migrations/002_bad_backfill.sql
  patchline reproduce examples/reproduce/bad-migration-billing.json --json`)
}

func repoCommand(args []string) error {
	if len(args) == 0 {
		return errors.New("usage: patchline repo <doctor|fetch|analyze|inventory|baseline|propose|compare|proposal-minimize|replay|suppressions|why-now|changes|hook|offline|notify-summary|minimize|recurrence> ...")
	}
	switch args[0] {
	case "doctor":
		return repoDoctor(args[1:])
	case "fetch":
		return repoFetch(args[1:])
	case "analyze":
		return repoAnalyze(args[1:])
	case "inventory":
		return repoInventory(args[1:])
	case "baseline":
		return repoBaseline(args[1:])
	case "propose":
		return repoPropose(args[1:])
	case "compare":
		return repoCompare(args[1:])
	case "proposal-minimize":
		return repoProposalMinimize(args[1:])
	case "replay":
		return repoReplay(args[1:])
	case "suppressions":
		return repoSuppressions(args[1:])
	case "why-now":
		return repoWhyNow(args[1:])
	case "changes":
		return repoChanges(args[1:])
	case "hook":
		return repoHook(args[1:])
	case "offline":
		return repoOffline(args[1:])
	case "pr-comment":
		return repoPRComment(args[1:])
	case "notify-summary":
		return repoNotifySummary(args[1:])
	case "minimize":
		return repoMinimize(args[1:])
	case "recurrence":
		return repoRecurrence(args[1:])
	default:
		return fmt.Errorf("unknown repo subcommand %q", args[0])
	}
}

type repoAnalyzeReport struct {
	Version      string                 `json:"version"`
	Input        string                 `json:"input"`
	Subpath      string                 `json:"subpath,omitempty"`
	Stages       []string               `json:"stages"`
	Outputs      map[string]string      `json:"outputs"`
	Source       project.Source         `json:"source,omitempty"`
	CI           bool                   `json:"ci"`
	CIArtifacts  repoAnalyzeCIArtifacts `json:"ci_artifacts,omitempty"`
	CommandsPath string                 `json:"commands_path,omitempty"`
	Resume       bool                   `json:"resume"`
	ReusedStages []string               `json:"reused_stages,omitempty"`
	Redact       bool                   `json:"redact"`
	Diagnostics  *diagnostics.Summary   `json:"diagnostics,omitempty"`
	Summary      repoAnalyzeSummary     `json:"summary"`
	DeepAnalysis repoAnalyzeDeepSummary `json:"deep_analysis,omitempty"`
	NextCommands []project.Command      `json:"next_commands,omitempty"`
	Hash         string                 `json:"hash"`
}

type repoAnalyzeSummary struct {
	FilesScanned         int    `json:"files_scanned"`
	Facts                int    `json:"facts"`
	RankedRisks          int    `json:"ranked_risks"`
	RankingExplanations  int    `json:"ranking_explanations"`
	ProvenanceSlices     int    `json:"provenance_slices"`
	PolicyChecks         int    `json:"policy_checks"`
	RepairProofSummaries int    `json:"repair_proof_summaries"`
	Infrastructure       int    `json:"infrastructure_findings"`
	GeneratedFiles       int    `json:"generated_files"`
	ProposalGenerator    string `json:"proposal_generator,omitempty"`
	DeterministicOnly    bool   `json:"deterministic_only"`
	ScopeBudget          string `json:"scope_budget,omitempty"`
	InterventionLoops    int    `json:"intervention_loops"`
	CompareChecksFailed  int    `json:"compare_checks_failed"`
	NativeChecksSkipped  int    `json:"native_checks_skipped"`
	BaselineHash         string `json:"baseline_hash,omitempty"`
	ProposalHash         string `json:"proposal_hash,omitempty"`
	CompareHash          string `json:"compare_hash,omitempty"`
}

type repoAnalyzeCIArtifacts struct {
	SummaryPath           string `json:"summary_path,omitempty"`
	SARIFPath             string `json:"sarif_path,omitempty"`
	GitLabCodeQualityPath string `json:"gitlab_code_quality_path,omitempty"`
	BitbucketInsightsPath string `json:"bitbucket_insights_path,omitempty"`
	BundlePath            string `json:"bundle_path,omitempty"`
	ActionsSnippet        string `json:"actions_snippet,omitempty"`
	GitLabSnippet         string `json:"gitlab_snippet,omitempty"`
	BitbucketSnippet      string `json:"bitbucket_snippet,omitempty"`
	ArtifactName          string `json:"artifact_name,omitempty"`
	CodeScanningTool      string `json:"code_scanning_tool,omitempty"`
	GitHubStepSummary     bool   `json:"github_step_summary"`
}

type repoHookReport struct {
	Version       string                 `json:"version"`
	Mode          string                 `json:"mode"`
	Root          string                 `json:"root"`
	Base          string                 `json:"base,omitempty"`
	Network       bool                   `json:"network"`
	ChangedFiles  []repoHookChangedFile  `json:"changed_files"`
	Summary       repoHookSummary        `json:"summary"`
	FindingDeltas []repoHookFindingDelta `json:"finding_deltas,omitempty"`
	Outputs       map[string]string      `json:"outputs,omitempty"`
	Hash          string                 `json:"hash"`
	Markdown      string                 `json:"markdown,omitempty"`
}

type repoHookChangedFile struct {
	Path   string `json:"path"`
	Bytes  int    `json:"bytes"`
	Source string `json:"source"`
}

type repoHookSummary struct {
	ChangedFiles      int `json:"changed_files"`
	ScannedFiles      int `json:"scanned_files"`
	Facts             int `json:"facts"`
	RankedRisks       int `json:"ranked_risks"`
	HighRisks         int `json:"high_risks"`
	MediumRisks       int `json:"medium_risks"`
	Infrastructure    int `json:"infrastructure_findings"`
	NetworkOperations int `json:"network_operations"`
}

type repoHookFindingDelta struct {
	Status      string `json:"status"`
	StableID    string `json:"stable_id,omitempty"`
	RiskID      string `json:"risk_id"`
	Path        string `json:"path"`
	Kind        string `json:"kind"`
	Severity    string `json:"severity"`
	Score       int    `json:"score"`
	Table       string `json:"table,omitempty"`
	Rationale   string `json:"rationale"`
	NextCommand string `json:"next_command,omitempty"`
}

type repoOfflineReport struct {
	Version     string                `json:"version"`
	Analysis    string                `json:"analysis,omitempty"`
	Network     bool                  `json:"network"`
	OK          bool                  `json:"ok"`
	Summary     repoOfflineSummary    `json:"summary"`
	CacheInputs []repoOfflineCache    `json:"cache_inputs,omitempty"`
	Adapters    []repoOfflineAdapter  `json:"adapters,omitempty"`
	Reports     []repoOfflineArtifact `json:"reports,omitempty"`
	Errors      []string              `json:"errors,omitempty"`
	Warnings    []string              `json:"warnings,omitempty"`
	Hash        string                `json:"hash"`
	Markdown    string                `json:"markdown,omitempty"`
}

type repoOfflineSummary struct {
	CacheInputs        int `json:"cache_inputs"`
	CacheInputsValid   int `json:"cache_inputs_valid"`
	Adapters           int `json:"adapters"`
	AdaptersValid      int `json:"adapters_valid"`
	Reports            int `json:"reports"`
	ReportsValid       int `json:"reports_valid"`
	GeneratedArtifacts int `json:"generated_artifacts"`
	NetworkOperations  int `json:"network_operations"`
	Errors             int `json:"errors"`
	Warnings           int `json:"warnings"`
}

type repoOfflineCache struct {
	SourcePath     string `json:"source_path"`
	Input          string `json:"input,omitempty"`
	Mode           string `json:"mode,omitempty"`
	CacheKey       string `json:"cache_key,omitempty"`
	CachePath      string `json:"cache_path,omitempty"`
	ArchiveHash    string `json:"archive_hash,omitempty"`
	ActualHash     string `json:"actual_hash,omitempty"`
	ScannedRoot    string `json:"scanned_root,omitempty"`
	ResolvedCommit string `json:"resolved_commit,omitempty"`
	Valid          bool   `json:"valid"`
	Rationale      string `json:"rationale"`
}

type repoOfflineAdapter struct {
	Path       string `json:"path"`
	Adapter    string `json:"adapter,omitempty"`
	EventCount int    `json:"event_count"`
	InputHash  string `json:"input_hash,omitempty"`
	Valid      bool   `json:"valid"`
	Rationale  string `json:"rationale"`
}

type repoOfflineArtifact struct {
	Kind      string `json:"kind"`
	Path      string `json:"path"`
	Hash      string `json:"hash,omitempty"`
	Valid     bool   `json:"valid"`
	Rationale string `json:"rationale"`
}

type gitlabCodeQualityIssue struct {
	Description string                    `json:"description"`
	CheckName   string                    `json:"check_name"`
	Fingerprint string                    `json:"fingerprint"`
	Severity    string                    `json:"severity"`
	Location    gitlabCodeQualityLocation `json:"location"`
}

type gitlabCodeQualityLocation struct {
	Path  string                 `json:"path"`
	Lines gitlabCodeQualityLines `json:"lines"`
}

type gitlabCodeQualityLines struct {
	Begin int `json:"begin"`
}

type bitbucketCodeInsightsReport struct {
	Version     string                           `json:"version"`
	Title       string                           `json:"title"`
	Details     string                           `json:"details"`
	Reporter    string                           `json:"reporter"`
	Result      string                           `json:"result"`
	Link        string                           `json:"link,omitempty"`
	Annotations []bitbucketCodeInsightAnnotation `json:"annotations"`
}

type bitbucketCodeInsightAnnotation struct {
	ExternalID string `json:"external_id"`
	Title      string `json:"title"`
	Summary    string `json:"summary"`
	Severity   string `json:"severity"`
	Path       string `json:"path"`
	Line       int    `json:"line"`
}

type repoAnalyzeDeepSummary struct {
	AbstractEffects        int `json:"abstract_effects"`
	SymbolicChecks         int `json:"symbolic_checks"`
	TemporalWindows        int `json:"temporal_windows"`
	Recurrences            int `json:"recurrences"`
	RepairProofRefuted     int `json:"repair_proof_refuted"`
	AblationSensitiveRisks int `json:"ablation_sensitive_risks"`
}

type maintainerTriageReport struct {
	Version      string                  `json:"version"`
	BaselineHash string                  `json:"baseline_hash"`
	ProposalHash string                  `json:"proposal_hash,omitempty"`
	CompareHash  string                  `json:"compare_hash,omitempty"`
	Groups       []maintainerTriageGroup `json:"groups"`
	OwnerRoutes  []project.OwnerRoute    `json:"owner_routes,omitempty"`
	Summary      maintainerTriageSummary `json:"summary"`
	Hash         string                  `json:"hash"`
	Markdown     string                  `json:"markdown,omitempty"`
}

type maintainerTriageSummary struct {
	Groups                 int `json:"groups"`
	GroupsWithFindings     int `json:"groups_with_findings"`
	TotalFindings          int `json:"total_findings"`
	GeneratedInterventions int `json:"generated_interventions"`
	NativeChecks           int `json:"native_checks"`
	OwnerRoutes            int `json:"owner_routes"`
	OwnerRouteOwners       int `json:"owner_route_owners"`
}

type maintainerTriageGroup struct {
	Surface         string                  `json:"surface"`
	OwnerHint       string                  `json:"owner_hint"`
	LikelyReviewers []string                `json:"likely_reviewers,omitempty"`
	FindingCount    int                     `json:"finding_count"`
	TopRisks        []maintainerTriageRisk  `json:"top_risks,omitempty"`
	EvidencePaths   []string                `json:"evidence_paths,omitempty"`
	GeneratedFiles  []project.GeneratedFile `json:"generated_files,omitempty"`
	NativeChecks    []project.Command       `json:"native_checks,omitempty"`
	Rationale       string                  `json:"rationale"`
}

type maintainerTriageRisk struct {
	ID        string   `json:"id"`
	Path      string   `json:"path"`
	Kind      string   `json:"kind"`
	Table     string   `json:"table,omitempty"`
	Severity  string   `json:"severity"`
	Score     int      `json:"score"`
	Reviewers []string `json:"reviewers,omitempty"`
}

type repoDoctorReport struct {
	Version      string            `json:"version"`
	Input        string            `json:"input"`
	Subpath      string            `json:"subpath,omitempty"`
	Source       project.Source    `json:"source,omitempty"`
	ScanRoot     string            `json:"scan_root"`
	Tools        []repoDoctorTool  `json:"tools"`
	Cache        repoDoctorCache   `json:"cache"`
	NativeChecks []project.Command `json:"native_checks,omitempty"`
	Summary      repoDoctorSummary `json:"summary"`
	NextCommands []project.Command `json:"next_commands,omitempty"`
	Hash         string            `json:"hash"`
	Markdown     string            `json:"markdown,omitempty"`
}

type repoDoctorTool struct {
	Name      string `json:"name"`
	Found     bool   `json:"found"`
	Path      string `json:"path,omitempty"`
	Required  bool   `json:"required"`
	Rationale string `json:"rationale"`
}

type repoDoctorCache struct {
	DownloadDir string `json:"download_dir"`
	Exists      bool   `json:"exists"`
	CacheHit    bool   `json:"cache_hit,omitempty"`
	ArchiveHash string `json:"archive_hash,omitempty"`
	CachePath   string `json:"cache_path,omitempty"`
}

type repoDoctorSummary struct {
	FilesScanned          int  `json:"files_scanned"`
	Facts                 int  `json:"facts"`
	ToolsFound            int  `json:"tools_found"`
	ToolsMissing          int  `json:"tools_missing"`
	RequiredToolsMissing  int  `json:"required_tools_missing"`
	NativeChecksAvailable int  `json:"native_checks_available"`
	ReadyForAnalyze       bool `json:"ready_for_analyze"`
	NetworkFetchUsed      bool `json:"network_fetch_used"`
}

type quickstartReport struct {
	Version           string               `json:"version"`
	GitHub            string               `json:"github"`
	Ref               string               `json:"ref,omitempty"`
	Subpath           string               `json:"subpath"`
	OutDir            string               `json:"out_dir"`
	Commands          []project.Command    `json:"commands"`
	ExpectedArtifacts []quickstartArtifact `json:"expected_artifacts"`
	Hash              string               `json:"hash"`
	Markdown          string               `json:"markdown,omitempty"`
	Script            string               `json:"script,omitempty"`
}

type quickstartArtifact struct {
	Path        string `json:"path"`
	Description string `json:"description"`
}

type suppressionLedger struct {
	Version      string             `json:"version"`
	Suppressions []suppressionEntry `json:"suppressions"`
}

type suppressionEntry struct {
	StableID     string `json:"stable_id"`
	Owner        string `json:"owner"`
	Rationale    string `json:"rationale"`
	Expires      string `json:"expires"`
	EvidenceHash string `json:"evidence_hash"`
}

type suppressionReport struct {
	Version      string              `json:"version"`
	BaselineHash string              `json:"baseline_hash"`
	Summary      suppressionSummary  `json:"summary"`
	Results      []suppressionResult `json:"results"`
	Hash         string              `json:"hash"`
	Markdown     string              `json:"markdown,omitempty"`
}

type suppressionSummary struct {
	Total     int `json:"total"`
	Active    int `json:"active"`
	Expired   int `json:"expired"`
	Stale     int `json:"stale"`
	Invalid   int `json:"invalid"`
	Unmatched int `json:"unmatched"`
}

type suppressionResult struct {
	StableID             string `json:"stable_id"`
	Status               string `json:"status"`
	Owner                string `json:"owner,omitempty"`
	Expires              string `json:"expires,omitempty"`
	ExpectedEvidenceHash string `json:"expected_evidence_hash,omitempty"`
	ActualEvidenceHash   string `json:"actual_evidence_hash,omitempty"`
	Reason               string `json:"reason"`
}

type whyNowReport struct {
	Version       string        `json:"version"`
	PreviousHash  string        `json:"previous_hash"`
	CurrentHash   string        `json:"current_hash"`
	Summary       whyNowSummary `json:"summary"`
	NewRisks      []whyNowRisk  `json:"new_risks,omitempty"`
	ResolvedRisks []whyNowRisk  `json:"resolved_risks,omitempty"`
	Persisting    []whyNowRisk  `json:"persisting_risks,omitempty"`
	Hash          string        `json:"hash"`
	Markdown      string        `json:"markdown,omitempty"`
}

type whyNowSummary struct {
	PreviousRisks   int `json:"previous_risks"`
	CurrentRisks    int `json:"current_risks"`
	NewRisks        int `json:"new_risks"`
	ResolvedRisks   int `json:"resolved_risks"`
	PersistingRisks int `json:"persisting_risks"`
}

type whyNowRisk struct {
	StableID string `json:"stable_id"`
	RiskID   string `json:"risk_id"`
	Path     string `json:"path"`
	Kind     string `json:"kind"`
	Table    string `json:"table,omitempty"`
	Severity string `json:"severity"`
	Score    int    `json:"score"`
	Reason   string `json:"reason"`
}

type prCommentReport struct {
	Version     string             `json:"version"`
	BaseHash    string             `json:"base_hash"`
	HeadHash    string             `json:"head_hash"`
	Summary     prCommentSummary   `json:"summary"`
	Findings    []prCommentFinding `json:"findings,omitempty"`
	Truncated   bool               `json:"truncated"`
	PostCommand string             `json:"post_command,omitempty"`
	Hash        string             `json:"hash"`
	Markdown    string             `json:"markdown,omitempty"`
}

type prCommentSummary struct {
	BaseRisks        int `json:"base_risks"`
	HeadRisks        int `json:"head_risks"`
	NewFindings      int `json:"new_findings"`
	ChangedFindings  int `json:"changed_findings"`
	UnchangedRisks   int `json:"unchanged_risks"`
	RenderedFindings int `json:"rendered_findings"`
}

type prCommentFinding struct {
	Status           string `json:"status"`
	StableID         string `json:"stable_id"`
	RiskID           string `json:"risk_id"`
	Path             string `json:"path"`
	Kind             string `json:"kind"`
	Table            string `json:"table,omitempty"`
	Severity         string `json:"severity"`
	PreviousSeverity string `json:"previous_severity,omitempty"`
	Score            int    `json:"score"`
	PreviousScore    int    `json:"previous_score,omitempty"`
	Reason           string `json:"reason"`
	Rationale        string `json:"rationale,omitempty"`
}

type changesReport struct {
	Version   string         `json:"version"`
	Previous  string         `json:"previous"`
	Current   string         `json:"current"`
	Summary   changesSummary `json:"summary"`
	Facts     changesDelta   `json:"facts"`
	Risks     changesDelta   `json:"ranked_risks"`
	Links     changesDelta   `json:"links"`
	Generated changesDelta   `json:"generated_artifacts"`
	Checks    checksDelta    `json:"deterministic_checks"`
	Hash      string         `json:"hash"`
	Markdown  string         `json:"markdown,omitempty"`
}

type changesSummary struct {
	ChangedDimensions int `json:"changed_dimensions"`
	PreviousFacts     int `json:"previous_facts"`
	CurrentFacts      int `json:"current_facts"`
	PreviousRisks     int `json:"previous_risks"`
	CurrentRisks      int `json:"current_risks"`
	PreviousGenerated int `json:"previous_generated"`
	CurrentGenerated  int `json:"current_generated"`
	PreviousFailures  int `json:"previous_failures"`
	CurrentFailures   int `json:"current_failures"`
}

type changesDelta struct {
	Previous int      `json:"previous"`
	Current  int      `json:"current"`
	Added    int      `json:"added"`
	Removed  int      `json:"removed"`
	Examples []string `json:"examples,omitempty"`
}

type checksDelta struct {
	PreviousFailures int `json:"previous_failures"`
	CurrentFailures  int `json:"current_failures"`
	PreviousPassed   int `json:"previous_passed"`
	CurrentPassed    int `json:"current_passed"`
	FailureDelta     int `json:"failure_delta"`
	PassDelta        int `json:"pass_delta"`
}

type notifySummaryReport struct {
	Version             string            `json:"version"`
	Analysis            string            `json:"analysis"`
	BundleLink          string            `json:"bundle_link"`
	TopMaintainerAction string            `json:"top_maintainer_action"`
	TopRisk             notifySummaryRisk `json:"top_risk"`
	ReproductionCommand string            `json:"reproduction_command"`
	SlackText           string            `json:"slack_text"`
	GitHubMarkdown      string            `json:"github_markdown"`
	Hash                string            `json:"hash"`
	Markdown            string            `json:"markdown,omitempty"`
}

type notifySummaryRisk struct {
	ID       string `json:"id"`
	StableID string `json:"stable_id,omitempty"`
	Path     string `json:"path"`
	Table    string `json:"table,omitempty"`
	Severity string `json:"severity"`
	Score    int    `json:"score"`
	Reason   string `json:"reason,omitempty"`
}

type corpusMinimizerReport struct {
	Version           string                 `json:"version"`
	Analysis          string                 `json:"analysis"`
	Source            project.Source         `json:"source"`
	Summary           corpusMinimizerSummary `json:"summary"`
	Entries           []corpusMinimizerEntry `json:"entries"`
	ExtractedSubpaths []string               `json:"extracted_subpaths"`
	Hash              string                 `json:"hash"`
	Markdown          string                 `json:"markdown,omitempty"`
}

type corpusMinimizerSummary struct {
	Risks             int `json:"risks"`
	Entries           int `json:"entries"`
	UniqueSourceFiles int `json:"unique_source_files"`
	EvidenceLinks     int `json:"evidence_links"`
	GeneratedFiles    int `json:"generated_files"`
	CopiedFiles       int `json:"copied_files"`
}

type corpusMinimizerEntry struct {
	RiskID           string                  `json:"risk_id"`
	StableID         string                  `json:"stable_id,omitempty"`
	Severity         string                  `json:"severity"`
	Score            int                     `json:"score"`
	PublicSubpath    string                  `json:"public_subpath"`
	SourcePaths      []string                `json:"source_paths"`
	EvidenceLinks    []project.EvidenceLink  `json:"evidence_links,omitempty"`
	GeneratedFiles   []project.GeneratedFile `json:"generated_files,omitempty"`
	PreservationNote string                  `json:"preservation_note"`
}

type recurrenceReport struct {
	Version     string              `json:"version"`
	Analyses    []string            `json:"analyses"`
	Summary     recurrenceSummary   `json:"summary"`
	Recurrences []recurrenceCluster `json:"recurrences"`
	Hash        string              `json:"hash"`
	Markdown    string              `json:"markdown,omitempty"`
}

type recurrenceSummary struct {
	Analyses          int `json:"analyses"`
	Risks             int `json:"risks"`
	Signatures        int `json:"signatures"`
	Repeated          int `json:"repeated"`
	RedactedFields    int `json:"redacted_fields"`
	UnrelatedProjects int `json:"unrelated_projects"`
}

type recurrenceCluster struct {
	Signature       string                 `json:"signature"`
	RiskKind        string                 `json:"risk_kind"`
	Severity        string                 `json:"severity"`
	FactorNames     []string               `json:"factor_names"`
	OccurrenceCount int                    `json:"occurrence_count"`
	ProjectCount    int                    `json:"project_count"`
	Occurrences     []recurrenceOccurrence `json:"occurrences"`
}

type recurrenceOccurrence struct {
	Analysis string `json:"analysis"`
	Project  string `json:"project"`
	RiskID   string `json:"risk_id"`
	StableID string `json:"stable_id,omitempty"`
	Severity string `json:"severity"`
	Score    int    `json:"score"`
}

func repoAnalyze(args []string) (err error) {
	fs := flag.NewFlagSet("repo analyze", flag.ContinueOnError)
	fs.SetOutput(ioDiscard{})
	githubRepo := fs.String("github", "", "GitHub owner/repo to fetch and analyze")
	ref := fs.String("ref", "", "git ref")
	subpath := fs.String("subpath", "", "subpath to focus on")
	stagesValue := fs.String("stages", "inventory,baseline,propose,compare,deep", "comma-separated stages: inventory,baseline,propose,compare,deep")
	outPath := fs.String("out", filepath.Join("results", "generated", "repo-analysis"), "output directory")
	downloadDir := fs.String("download-dir", "", "download/cache directory")
	proposalKind := fs.String("proposal-kind", "all", "proposal kind: tests|guards|instrumentation|repair|explain|all")
	llmCommand := fs.String("llm-command", "", "optional user-provided generator command; prompt is passed on stdin")
	promptNoFacts := fs.Bool("prompt-without-facts", false, "ablation mode: send generator a prompt without repository facts")
	budget := fs.String("budget", "", "generated scope budget: files=N,lines=N,tokens=N,changes=N")
	budgetRisks := fs.Int("budget-risks", 3, "maximum ranked risks to include")
	noLLM := fs.Bool("no-llm", false, "force deterministic template proposals and reject LLM generation")
	resume := fs.Bool("resume", false, "reuse existing fetch, inventory, intake, baseline, proposal, and compare artifacts when present")
	redact := fs.Bool("redact", false, "write stable-token redacted analysis-bundle artifacts")
	ciMode := fs.Bool("ci", false, "write CI metadata for SARIF upload and analysis-bundle artifact storage")
	traceMode := fs.Bool("trace", false, "write structured diagnostic logs and trace spans")
	runNativeTests := fs.Bool("run-native-tests", false, "run safe allowlisted native test commands during compare")
	jsonOut := fs.Bool("json", false, "emit JSON")
	input, flagArgs, err := onePositionalWithFlags(args, map[string]bool{"--ci": true, "--json": true, "--no-llm": true, "--prompt-without-facts": true, "--redact": true, "--resume": true, "--run-native-tests": true, "--trace": true})
	if err != nil {
		return err
	}
	if err := fs.Parse(flagArgs); err != nil {
		return err
	}
	if *githubRepo != "" {
		input = *githubRepo
	}
	if input == "" || fs.NArg() != 0 {
		return errors.New("usage: patchline repo analyze [<path>|--github owner/repo] [--ref ref] [--subpath path] [--stages inventory,baseline,propose,compare,deep] [--proposal-kind kind] [--budget files=N,lines=N,tokens=N,changes=N] [--ci] [--redact] [--resume] [--trace] [--no-llm] [--llm-command cmd] [--out dir] [--json]")
	}
	stages, err := parseAnalyzeStages(*stagesValue)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(*outPath, 0o755); err != nil {
		return err
	}
	diag := diagnostics.New(filepath.Join(*outPath, "diagnostics"), *traceMode)
	rootSpan := diag.StartSpan("repo.analyze", map[string]any{"input": input, "subpath": *subpath, "stages": strings.Join(stages, ","), "ci": *ciMode, "resume": *resume, "redact": *redact})
	diagnosticsWritten := false
	defer func() {
		if diag.Enabled() && !diagnosticsWritten {
			rootSpan.End(err, nil)
			_, writeErr := diag.Write()
			if err == nil && writeErr != nil {
				err = writeErr
			}
		}
	}()
	if diag.Enabled() {
		diag.Log("analyze.start", "repo analysis started", map[string]any{"input": input, "out": *outPath})
	}

	report := repoAnalyzeReport{
		Version: "patchline.repo-analyze/v1",
		Input:   input,
		Subpath: *subpath,
		Stages:  stages,
		Outputs: map[string]string{},
		CI:      *ciMode,
		Resume:  *resume,
		Redact:  *redact,
	}
	if diag.Enabled() {
		report.Outputs["diagnostics"] = filepath.Join(*outPath, "diagnostics")
	}
	stageSet := analyzeStageSet(stages)
	scanRoot := input
	if *githubRepo != "" {
		fetchOut := filepath.Join(*outPath, "fetch")
		span := rootSpan.Child("fetch", map[string]any{"repo": *githubRepo, "ref": *ref, "subpath": *subpath, "resume": *resume})
		if *resume && fileExists(filepath.Join(fetchOut, "source.json")) {
			source, err := loadSource(filepath.Join(fetchOut, "source.json"))
			if err != nil {
				span.End(err, nil)
				return err
			}
			report.Source = source
			report.ReusedStages = append(report.ReusedStages, "fetch")
			scanRoot = source.ScannedRoot
			span.End(nil, map[string]any{"reused": true, "scanned_root": scanRoot})
		} else {
			fetchResult, err := project.Fetch(context.Background(), project.FetchOptions{Input: *githubRepo, Ref: *ref, Subpath: *subpath, OutDir: fetchOut, DownloadDir: *downloadDir})
			if err != nil {
				span.End(err, nil)
				return err
			}
			report.Source = fetchResult.Source
			scanRoot = fetchResult.Source.ScannedRoot
			span.End(nil, map[string]any{"reused": false, "scanned_root": scanRoot, "archive_hash": fetchResult.Source.ArchiveHash})
		}
		report.Outputs["fetch"] = fetchOut
	}

	var inv project.Inventory
	var intakeReport intake.Report
	var baseline project.BaselineReport
	var proposal project.ProposalReport
	var compare project.CompareReport

	if analyzeNeeds(stageSet, "inventory") {
		inventoryOut := filepath.Join(*outPath, "inventory")
		span := rootSpan.Child("inventory", map[string]any{"path": scanRoot, "resume": *resume})
		if *resume && fileExists(filepath.Join(inventoryOut, "inventory.json")) {
			inv, _, err = project.LoadInventory(inventoryOut)
			if err != nil {
				span.End(err, nil)
				return err
			}
			report.ReusedStages = append(report.ReusedStages, "inventory")
		} else {
			inv, err = project.InventoryPath(project.InventoryOptions{Path: scanRoot})
			if err != nil {
				span.End(err, nil)
				return err
			}
			if err := project.WriteInventory(inventoryOut, inv); err != nil {
				span.End(err, nil)
				return err
			}
		}
		report.Outputs["inventory"] = inventoryOut
		report.Summary.FilesScanned = inv.FilesScanned
		report.Summary.Facts = len(inv.Facts)
		span.End(nil, map[string]any{"files_scanned": inv.FilesScanned, "facts": len(inv.Facts), "reused": stringSliceContains(report.ReusedStages, "inventory")})
	}
	if analyzeNeeds(stageSet, "baseline") {
		if inv.Root == "" {
			span := rootSpan.Child("inventory.implicit", map[string]any{"path": scanRoot})
			inv, err = project.InventoryPath(project.InventoryOptions{Path: scanRoot})
			if err != nil {
				span.End(err, nil)
				return err
			}
			span.End(nil, map[string]any{"files_scanned": inv.FilesScanned, "facts": len(inv.Facts)})
		}
		intakeOut := filepath.Join(*outPath, "intake")
		intakeSpan := rootSpan.Child("intake", map[string]any{"path": scanRoot, "resume": *resume})
		if *resume && fileExists(filepath.Join(intakeOut, "summary.json")) {
			intakeReport, err = project.LoadIntakeReport(intakeOut)
			if err != nil {
				intakeSpan.End(err, nil)
				return err
			}
			report.ReusedStages = append(report.ReusedStages, "intake")
		} else {
			intakeReport, err = intake.Run(context.Background(), intake.Options{Path: scanRoot})
			if err != nil {
				intakeSpan.End(err, nil)
				return err
			}
			if err := intake.WriteReport(intakeOut, intakeReport); err != nil {
				intakeSpan.End(err, nil)
				return err
			}
		}
		report.Outputs["intake"] = intakeOut
		intakeSpan.End(nil, map[string]any{"files_scanned": intakeReport.Summary.FilesScanned, "sql_findings": len(intakeReport.SQL), "problem_candidates": intakeReport.Summary.ProblemCandidates, "reused": stringSliceContains(report.ReusedStages, "intake")})
		baselineOut := filepath.Join(*outPath, "baseline")
		baselineSpan := rootSpan.Child("baseline", map[string]any{"inventory_root": inv.Root, "resume": *resume})
		if *resume && fileExists(filepath.Join(baselineOut, "baseline.json")) {
			baseline, err = project.LoadBaseline(baselineOut)
			if err != nil {
				baselineSpan.End(err, nil)
				return err
			}
			report.ReusedStages = append(report.ReusedStages, "baseline")
		} else {
			baseline = project.Baseline(inv, inv.Facts, intakeReport)
			if err := project.WriteBaseline(baselineOut, baseline); err != nil {
				baselineSpan.End(err, nil)
				return err
			}
		}
		report.Outputs["baseline"] = baselineOut
		report.Summary.RankedRisks = baseline.Summary.RankedRisks
		report.Summary.RankingExplanations = baseline.Summary.RankingExplanations
		report.Summary.ProvenanceSlices = baseline.Summary.ProvenanceSlices
		report.Summary.PolicyChecks = baseline.Summary.PolicyChecks
		report.Summary.RepairProofSummaries = baseline.Summary.RepairProofs
		report.Summary.Infrastructure = baseline.Summary.InfraFindings
		report.Summary.BaselineHash = baseline.Hash
		deepSpan := rootSpan.Child("deep-summary", map[string]any{"requested": stageSet["deep"]})
		report.DeepAnalysis = repoAnalyzeDeepSummary{
			AbstractEffects:        baseline.Summary.AbstractEffects,
			SymbolicChecks:         baseline.Summary.SymbolicChecks,
			TemporalWindows:        baseline.Summary.TemporalWindows,
			Recurrences:            baseline.Summary.Recurrences,
			RepairProofRefuted:     baseline.Summary.RepairProofRefuted,
			AblationSensitiveRisks: baseline.Summary.AblationSensitive,
		}
		deepSpan.End(nil, map[string]any{"abstract_effects": report.DeepAnalysis.AbstractEffects, "symbolic_checks": report.DeepAnalysis.SymbolicChecks, "temporal_windows": report.DeepAnalysis.TemporalWindows, "recurrences": report.DeepAnalysis.Recurrences})
		report.NextCommands = append(report.NextCommands, baseline.NativeChecks...)
		baselineSpan.End(nil, map[string]any{"ranked_risks": baseline.Summary.RankedRisks, "policy_checks": baseline.Summary.PolicyChecks, "provenance_slices": baseline.Summary.ProvenanceSlices, "reused": stringSliceContains(report.ReusedStages, "baseline")})
	}
	if analyzeNeeds(stageSet, "propose") {
		proposalOut := filepath.Join(*outPath, "proposal")
		proposalSpan := rootSpan.Child("proposal", map[string]any{"kind": *proposalKind, "budget": *budget, "resume": *resume, "no_llm": *noLLM})
		if *resume && fileExists(filepath.Join(proposalOut, "proposal.json")) {
			proposal, err = project.LoadProposal(proposalOut)
			if err != nil {
				proposalSpan.End(err, nil)
				return err
			}
			report.ReusedStages = append(report.ReusedStages, "proposal")
		} else {
			proposal, err = project.Propose(project.ProposalOptions{BaselinePath: filepath.Join(*outPath, "baseline"), Kind: *proposalKind, OutDir: proposalOut, LLMCommand: *llmCommand, NoLLM: *noLLM, PromptNoFacts: *promptNoFacts, Budget: *budget, BudgetRisks: *budgetRisks})
			if err != nil {
				proposalSpan.End(err, nil)
				return err
			}
			if err := project.WriteProposal(proposalOut, proposal); err != nil {
				proposalSpan.End(err, nil)
				return err
			}
		}
		report.Outputs["proposal"] = proposalOut
		report.Summary.GeneratedFiles = len(proposal.GeneratedFiles)
		report.Summary.ProposalGenerator = proposal.Generator
		report.Summary.DeterministicOnly = proposal.Deterministic
		report.Summary.ScopeBudget = proposal.ScopeBudget.Raw
		report.Summary.ProposalHash = proposal.OutputHash
		proposalSpan.End(nil, map[string]any{"generated_files": len(proposal.GeneratedFiles), "generator": proposal.Generator, "deterministic": proposal.Deterministic, "reused": stringSliceContains(report.ReusedStages, "proposal")})
	}
	if analyzeNeeds(stageSet, "compare") {
		if baseline.Hash == "" {
			baseline, err = project.LoadBaseline(filepath.Join(*outPath, "baseline"))
			if err != nil {
				return err
			}
		}
		if proposal.OutputHash == "" {
			proposal, err = project.LoadProposal(filepath.Join(*outPath, "proposal"))
			if err != nil {
				return err
			}
		}
		compareOut := filepath.Join(*outPath, "compare")
		compareSpan := rootSpan.Child("compare", map[string]any{"run_native_tests": *runNativeTests, "resume": *resume})
		if *resume && fileExists(filepath.Join(compareOut, "compare.json")) {
			compare, err = loadCompareReport(filepath.Join(compareOut, "compare.json"))
			if err != nil {
				compareSpan.End(err, nil)
				return err
			}
			report.ReusedStages = append(report.ReusedStages, "compare")
		} else {
			compare = project.CompareWithOptions(baseline, proposal, project.CompareOptions{RunNativeTests: *runNativeTests})
			if err := project.WriteCompare(compareOut, compare); err != nil {
				compareSpan.End(err, nil)
				return err
			}
		}
		report.Outputs["compare"] = compareOut
		report.Summary.InterventionLoops = compare.Summary.InterventionLoops
		report.Summary.CompareChecksFailed = compare.Summary.PatchlineChecksFailed
		report.Summary.NativeChecksSkipped = compare.Summary.NativeChecksSkipped
		report.Summary.CompareHash = compare.Hash
		compareSpan.End(nil, map[string]any{"intervention_loops": compare.Summary.InterventionLoops, "checks_failed": compare.Summary.PatchlineChecksFailed, "native_skipped": compare.Summary.NativeChecksSkipped, "reused": stringSliceContains(report.ReusedStages, "compare")})
	}
	if baseline.Hash != "" {
		triageSpan := rootSpan.Child("triage", map[string]any{"baseline_hash": baseline.Hash})
		triage := buildMaintainerTriage(baseline, proposal, compare)
		triageOut := filepath.Join(*outPath, "triage")
		if err := writeMaintainerTriage(triageOut, triage); err != nil {
			triageSpan.End(err, nil)
			return err
		}
		report.Outputs["triage"] = triageOut
		triageSpan.End(nil, map[string]any{"groups": len(triage.Groups), "owner_routes": triage.Summary.OwnerRoutes})
	}
	report.Outputs["analysis_bundle"] = filepath.Join(*outPath, "analysis-bundle")
	report.CommandsPath = filepath.Join(*outPath, "commands.md")
	report.Outputs["commands"] = report.CommandsPath
	if err := writeCopyCommandsReport(*outPath, report, *githubRepo != "", *ref, *proposalKind, *llmCommand, *budget, *noLLM, *promptNoFacts, *redact, *ciMode); err != nil {
		return err
	}
	if diag.Enabled() {
		diag.Log("analyze.outputs", "analysis outputs prepared", map[string]any{"outputs": len(report.Outputs)})
	}
	report.Hash = canonical.Hash(struct {
		Version      string                 `json:"version"`
		Input        string                 `json:"input"`
		Subpath      string                 `json:"subpath,omitempty"`
		Stages       []string               `json:"stages"`
		Outputs      map[string]string      `json:"outputs"`
		CI           bool                   `json:"ci"`
		CommandsPath string                 `json:"commands_path,omitempty"`
		Resume       bool                   `json:"resume"`
		ReusedStages []string               `json:"reused_stages,omitempty"`
		Redact       bool                   `json:"redact"`
		Summary      repoAnalyzeSummary     `json:"summary"`
		Deep         repoAnalyzeDeepSummary `json:"deep_analysis,omitempty"`
	}{report.Version, report.Input, report.Subpath, report.Stages, report.Outputs, report.CI, report.CommandsPath, report.Resume, report.ReusedStages, report.Redact, report.Summary, report.DeepAnalysis})
	if err := writeRepoAnalyzeReport(*outPath, report); err != nil {
		return err
	}
	bundleSpan := rootSpan.Child("analysis-bundle", map[string]any{"redact": *redact})
	if err := writeAnalysisBundle(*outPath, report); err != nil {
		bundleSpan.End(err, nil)
		return err
	}
	bundleSpan.End(nil, map[string]any{"bundle": report.Outputs["analysis_bundle"]})
	if report.CI {
		ciSpan := rootSpan.Child("ci-artifacts", nil)
		ciArtifacts, err := writeRepoAnalyzeCIArtifacts(*outPath, report)
		if err != nil {
			ciSpan.End(err, nil)
			return err
		}
		report.CIArtifacts = ciArtifacts
		report.Outputs["ci"] = filepath.Join(*outPath, "ci")
		report.Hash = canonical.Hash(struct {
			Version      string                 `json:"version"`
			Input        string                 `json:"input"`
			Subpath      string                 `json:"subpath,omitempty"`
			Stages       []string               `json:"stages"`
			Outputs      map[string]string      `json:"outputs"`
			CI           bool                   `json:"ci"`
			CIArtifacts  repoAnalyzeCIArtifacts `json:"ci_artifacts,omitempty"`
			CommandsPath string                 `json:"commands_path,omitempty"`
			Resume       bool                   `json:"resume"`
			Redact       bool                   `json:"redact"`
			Summary      repoAnalyzeSummary     `json:"summary"`
		}{report.Version, report.Input, report.Subpath, report.Stages, report.Outputs, report.CI, report.CIArtifacts, report.CommandsPath, report.Resume, report.Redact, report.Summary})
		if err := writeRepoAnalyzeReport(*outPath, report); err != nil {
			ciSpan.End(err, nil)
			return err
		}
		if err := writeAnalysisBundle(*outPath, report); err != nil {
			ciSpan.End(err, nil)
			return err
		}
		ciSpan.End(nil, map[string]any{"summary": ciArtifacts.SummaryPath})
	}
	if diag.Enabled() {
		rootSpan.End(nil, map[string]any{"ranked_risks": report.Summary.RankedRisks, "generated_files": report.Summary.GeneratedFiles})
		diagSummary, err := diag.Write()
		if err != nil {
			return err
		}
		diagnosticsWritten = true
		report.Diagnostics = &diagSummary
		report.Outputs["diagnostics"] = filepath.Join(*outPath, "diagnostics")
		report.Hash = canonical.Hash(struct {
			Version      string               `json:"version"`
			Input        string               `json:"input"`
			Subpath      string               `json:"subpath,omitempty"`
			Stages       []string             `json:"stages"`
			Outputs      map[string]string    `json:"outputs"`
			CI           bool                 `json:"ci"`
			Diagnostics  *diagnostics.Summary `json:"diagnostics,omitempty"`
			CommandsPath string               `json:"commands_path,omitempty"`
			Resume       bool                 `json:"resume"`
			Redact       bool                 `json:"redact"`
			Summary      repoAnalyzeSummary   `json:"summary"`
		}{report.Version, report.Input, report.Subpath, report.Stages, report.Outputs, report.CI, report.Diagnostics, report.CommandsPath, report.Resume, report.Redact, report.Summary})
		if err := writeRepoAnalyzeReport(*outPath, report); err != nil {
			return err
		}
	}
	if *jsonOut {
		return writeJSON(os.Stdout, report)
	}
	fmt.Printf("repo analyze input=%s stages=%s out=%s risks=%d generated=%d intervention_loops=%d hash=%s\n",
		report.Input,
		strings.Join(report.Stages, ","),
		*outPath,
		report.Summary.RankedRisks,
		report.Summary.GeneratedFiles,
		report.Summary.InterventionLoops,
		report.Hash,
	)
	return nil
}

func repoFetch(args []string) error {
	fs := flag.NewFlagSet("repo fetch", flag.ContinueOnError)
	fs.SetOutput(ioDiscard{})
	ref := fs.String("ref", "", "git ref")
	subpath := fs.String("subpath", "", "subpath to focus on")
	outPath := fs.String("out", "", "output directory")
	downloadDir := fs.String("download-dir", "", "download/cache directory")
	jsonOut := fs.Bool("json", false, "emit JSON")
	full := fs.Bool("full", false, "record exhaustive fetch intent")
	input, flagArgs, err := onePositionalWithFlags(args, map[string]bool{"--json": true, "--full": true})
	if err != nil {
		return err
	}
	if err := fs.Parse(flagArgs); err != nil {
		return err
	}
	if input == "" || fs.NArg() != 0 {
		return errors.New("usage: patchline repo fetch <owner/repo|github-url|gitlab:namespace/repo|bitbucket:owner/repo|sourcehut:owner/repo|path|archive-url> [--ref ref] [--subpath path] [--out dir] [--download-dir dir] [--json]")
	}
	result, err := project.Fetch(context.Background(), project.FetchOptions{
		Input:       input,
		Ref:         *ref,
		Subpath:     *subpath,
		OutDir:      *outPath,
		DownloadDir: *downloadDir,
		Full:        *full,
	})
	if err != nil {
		return err
	}
	if *jsonOut {
		return writeJSON(os.Stdout, result)
	}
	fmt.Printf("repo source=%s mode=%s out=%s scanned_root=%s",
		result.Source.Input,
		result.Source.Mode,
		result.Source.OutDir,
		result.Source.ScannedRoot,
	)
	if result.Source.ArchiveHash != "" {
		fmt.Printf(" archive_hash=%s", result.Source.ArchiveHash)
	}
	if result.Source.CommitHint != "" {
		fmt.Printf(" commit_hint=%s", result.Source.CommitHint)
	}
	fmt.Println()
	fmt.Printf("  source=%s\n", filepath.Join(result.Source.OutDir, "source.json"))
	return nil
}

func repoDoctor(args []string) error {
	fs := flag.NewFlagSet("repo doctor", flag.ContinueOnError)
	fs.SetOutput(ioDiscard{})
	githubRepo := fs.String("github", "", "GitHub owner/repo to fetch before diagnosis")
	ref := fs.String("ref", "", "git ref")
	subpath := fs.String("subpath", "", "subpath to focus on")
	outPath := fs.String("out", filepath.Join("results", "generated", "repo-doctor"), "output directory")
	downloadDir := fs.String("download-dir", "", "download/cache directory")
	jsonOut := fs.Bool("json", false, "emit JSON")
	input, flagArgs, err := onePositionalWithFlags(args, map[string]bool{"--json": true})
	if err != nil {
		return err
	}
	if err := fs.Parse(flagArgs); err != nil {
		return err
	}
	if *githubRepo != "" {
		input = *githubRepo
	}
	if input == "" || fs.NArg() != 0 {
		return errors.New("usage: patchline repo doctor [<path>|--github owner/repo] [--ref ref] [--subpath path] [--out dir] [--download-dir dir] [--json]")
	}
	report, err := buildRepoDoctorReport(input, *githubRepo != "", *ref, *subpath, *outPath, *downloadDir)
	if err != nil {
		return err
	}
	if *outPath != "" {
		if err := writeRepoDoctorReport(*outPath, report); err != nil {
			return err
		}
	}
	if *jsonOut {
		return writeJSON(os.Stdout, report)
	}
	fmt.Printf("repo doctor input=%s files=%d facts=%d tools_found=%d native_checks=%d ready=%t hash=%s\n",
		report.Input,
		report.Summary.FilesScanned,
		report.Summary.Facts,
		report.Summary.ToolsFound,
		report.Summary.NativeChecksAvailable,
		report.Summary.ReadyForAnalyze,
		report.Hash,
	)
	if *outPath != "" {
		fmt.Printf("  out=%s\n", *outPath)
	}
	return nil
}

func buildRepoDoctorReport(input string, githubInput bool, ref, subpath, outDir, downloadDir string) (repoDoctorReport, error) {
	report := repoDoctorReport{
		Version: "patchline.repo-doctor/v1",
		Input:   input,
		Subpath: subpath,
	}
	scanRoot := input
	if githubInput {
		fetchOut := filepath.Join(outDir, "fetch")
		fetchResult, err := project.Fetch(context.Background(), project.FetchOptions{Input: input, Ref: ref, Subpath: subpath, OutDir: fetchOut, DownloadDir: downloadDir})
		if err != nil {
			return repoDoctorReport{}, err
		}
		report.Source = fetchResult.Source
		scanRoot = fetchResult.Source.ScannedRoot
	}
	report.ScanRoot = filepath.ToSlash(scanRoot)
	if downloadDir == "" {
		downloadDir = filepath.Join("results", "generated", "repo-downloads")
	}
	if info, err := os.Stat(downloadDir); err == nil && info.IsDir() {
		report.Cache.Exists = true
	}
	report.Cache.DownloadDir = filepath.ToSlash(downloadDir)
	report.Cache.CacheHit = report.Source.CacheHit
	report.Cache.ArchiveHash = report.Source.ArchiveHash
	report.Cache.CachePath = report.Source.CachePath

	inv, err := project.InventoryPath(project.InventoryOptions{Path: scanRoot})
	if err != nil {
		return repoDoctorReport{}, err
	}
	report.NativeChecks = uniqueDoctorCommands(inv.TestCommands)
	report.NextCommands = uniqueDoctorCommands(inv.NextCommands)
	report.Tools = doctorTools(report.NativeChecks)
	for _, tool := range report.Tools {
		if tool.Found {
			report.Summary.ToolsFound++
		} else {
			report.Summary.ToolsMissing++
			if tool.Required {
				report.Summary.RequiredToolsMissing++
			}
		}
	}
	report.Summary.FilesScanned = inv.FilesScanned
	report.Summary.Facts = len(inv.Facts)
	report.Summary.NativeChecksAvailable = len(report.NativeChecks)
	report.Summary.ReadyForAnalyze = report.Summary.FilesScanned > 0 && report.Summary.RequiredToolsMissing == 0
	report.Summary.NetworkFetchUsed = githubInput
	report.Hash = repoDoctorHash(report)
	report.Markdown = renderRepoDoctorMarkdown(report)
	return report, nil
}

func doctorTools(nativeChecks []project.Command) []repoDoctorTool {
	names := []string{"go", "git", "jq"}
	for _, command := range nativeChecks {
		fields := strings.Fields(command.Command)
		if len(fields) > 0 {
			names = append(names, fields[0])
		}
	}
	names = uniqueDoctorStrings(names)
	sort.Strings(names)
	var tools []repoDoctorTool
	for _, name := range names {
		path, err := exec.LookPath(name)
		tools = append(tools, repoDoctorTool{
			Name:      name,
			Found:     err == nil,
			Path:      filepath.ToSlash(path),
			Required:  name == "go",
			Rationale: doctorToolRationale(name),
		})
	}
	return tools
}

func doctorToolRationale(name string) string {
	switch name {
	case "go":
		return "required for go run ./cmd/patchline and Go native checks"
	case "git":
		return "useful for local resolved-commit metadata and maintainer workflows"
	case "jq":
		return "useful for inspecting JSON outputs in copied commands and gates"
	default:
		return "discovered native project check command"
	}
}

func uniqueDoctorStrings(values []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}

func uniqueDoctorCommands(commands []project.Command) []project.Command {
	seen := map[string]bool{}
	var out []project.Command
	for _, command := range commands {
		key := strings.TrimSpace(command.Command)
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, command)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Command < out[j].Command })
	return out
}

func writeRepoDoctorReport(outDir string, report repoDoctorReport) error {
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}
	copy := report
	copy.Markdown = ""
	data, err := json.MarshalIndent(copy, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(outDir, "doctor.json"), append(data, '\n'), 0o644); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(outDir, "doctor.md"), []byte(report.Markdown), 0o644)
}

func repoDoctorHash(report repoDoctorReport) string {
	copy := report
	copy.Hash = ""
	copy.Markdown = ""
	return canonical.Hash(copy)
}

func renderRepoDoctorMarkdown(report repoDoctorReport) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Patchline repo doctor\n\n")
	fmt.Fprintf(&b, "- input: `%s`\n", report.Input)
	fmt.Fprintf(&b, "- scan root: `%s`\n", report.ScanRoot)
	fmt.Fprintf(&b, "- ready for analyze: `%t`\n", report.Summary.ReadyForAnalyze)
	fmt.Fprintf(&b, "- hash: `%s`\n\n", report.Hash)
	fmt.Fprintf(&b, "## Summary\n\n| area | count |\n| --- | ---: |\n")
	fmt.Fprintf(&b, "| files scanned | %d |\n", report.Summary.FilesScanned)
	fmt.Fprintf(&b, "| facts | %d |\n", report.Summary.Facts)
	fmt.Fprintf(&b, "| tools found | %d |\n", report.Summary.ToolsFound)
	fmt.Fprintf(&b, "| tools missing | %d |\n", report.Summary.ToolsMissing)
	fmt.Fprintf(&b, "| required tools missing | %d |\n", report.Summary.RequiredToolsMissing)
	fmt.Fprintf(&b, "| native checks available | %d |\n\n", report.Summary.NativeChecksAvailable)
	fmt.Fprintf(&b, "## Cache\n\n| field | value |\n| --- | --- |\n")
	fmt.Fprintf(&b, "| download dir | `%s` |\n", report.Cache.DownloadDir)
	fmt.Fprintf(&b, "| exists | `%t` |\n", report.Cache.Exists)
	fmt.Fprintf(&b, "| cache hit | `%t` |\n", report.Cache.CacheHit)
	if report.Cache.ArchiveHash != "" {
		fmt.Fprintf(&b, "| archive hash | `%s` |\n", report.Cache.ArchiveHash)
	}
	fmt.Fprintf(&b, "\n## Tools\n\n| tool | found | required | rationale |\n| --- | --- | --- | --- |\n")
	for _, tool := range report.Tools {
		fmt.Fprintf(&b, "| `%s` | %t | %t | %s |\n", tool.Name, tool.Found, tool.Required, tool.Rationale)
	}
	if len(report.NativeChecks) > 0 {
		fmt.Fprintf(&b, "\n## Safe native checks discovered\n\n")
		for _, command := range report.NativeChecks {
			fmt.Fprintf(&b, "- `%s` — %s\n", command.Command, command.Reason)
		}
	}
	if len(report.NextCommands) > 0 {
		fmt.Fprintf(&b, "\n## Next commands\n\n")
		for _, command := range report.NextCommands {
			fmt.Fprintf(&b, "- `%s` — %s\n", command.Command, command.Reason)
		}
	}
	return b.String()
}

func quickstart(args []string) error {
	fs := flag.NewFlagSet("quickstart", flag.ContinueOnError)
	fs.SetOutput(ioDiscard{})
	githubRepo := fs.String("github", "", "GitHub owner/repo to analyze")
	ref := fs.String("ref", "", "git ref")
	subpath := fs.String("subpath", "", "subpath to focus on")
	outPath := fs.String("out", "", "output directory")
	jsonOut := fs.Bool("json", false, "emit JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *githubRepo == "" || *subpath == "" || fs.NArg() != 0 {
		return errors.New("usage: patchline quickstart --github owner/repo --subpath path [--ref ref] [--out dir] [--json]")
	}
	report := buildQuickstartReport(*githubRepo, *ref, *subpath, *outPath)
	if report.OutDir != "" {
		if err := writeQuickstartReport(report.OutDir, report); err != nil {
			return err
		}
	}
	if *jsonOut {
		return writeJSON(os.Stdout, report)
	}
	fmt.Print(report.Markdown)
	if report.OutDir != "" {
		fmt.Printf("\nquickstart files written to %s\n", report.OutDir)
	}
	return nil
}

func buildQuickstartReport(githubRepo, ref, subpath, outDir string) quickstartReport {
	if outDir == "" {
		outDir = filepath.Join("results", "generated", "quickstart", quickstartSlug(githubRepo+"-"+subpath))
	}
	analysisOut := filepath.Join(outDir, "analysis")
	doctorOut := filepath.Join(outDir, "doctor")
	doctorCommand := fmt.Sprintf("go run ./cmd/patchline doctor --github %s%s --subpath %s --out %s", shellArg(githubRepo), refFlag(ref), shellArg(subpath), shellArg(doctorOut))
	analyzeCommand := fmt.Sprintf("go run ./cmd/patchline repo analyze --github %s%s --subpath %s --stages inventory,baseline,propose,compare,deep --proposal-kind all --budget files=6,lines=120,tokens=20000,changes=3 --no-llm --out %s", shellArg(githubRepo), refFlag(ref), shellArg(subpath), shellArg(analysisOut))
	verifyCommand := fmt.Sprintf("test -s %s && test -s %s && test -s %s", shellArg(filepath.Join(doctorOut, "doctor.json")), shellArg(filepath.Join(analysisOut, "analysis-bundle", "summary.md")), shellArg(filepath.Join(analysisOut, "commands.md")))
	report := quickstartReport{
		Version: "patchline.quickstart/v1",
		GitHub:  githubRepo,
		Ref:     ref,
		Subpath: subpath,
		OutDir:  filepath.ToSlash(outDir),
		Commands: []project.Command{
			{Command: doctorCommand, Reason: "diagnose tools, cache state, network fetch, and safe native checks before analysis"},
			{Command: analyzeCommand, Reason: "run deterministic baseline, bounded proposal generation, compare, and deep analysis"},
			{Command: verifyCommand, Reason: "verify expected quickstart artifacts were written"},
		},
		ExpectedArtifacts: []quickstartArtifact{
			{Path: filepath.ToSlash(filepath.Join(doctorOut, "doctor.json")), Description: "preflight diagnostic JSON"},
			{Path: filepath.ToSlash(filepath.Join(doctorOut, "doctor.md")), Description: "preflight diagnostic Markdown"},
			{Path: filepath.ToSlash(filepath.Join(analysisOut, "analysis-bundle", "summary.md")), Description: "shareable analysis summary"},
			{Path: filepath.ToSlash(filepath.Join(analysisOut, "analysis-bundle", "summary.sarif")), Description: "SARIF report for code-scanning systems"},
			{Path: filepath.ToSlash(filepath.Join(analysisOut, "commands.md")), Description: "reproducible staged command report"},
		},
	}
	report.Script = renderQuickstartScript(report)
	report.Hash = quickstartHash(report)
	report.Markdown = renderQuickstartMarkdown(report)
	return report
}

func refFlag(ref string) string {
	if strings.TrimSpace(ref) == "" {
		return ""
	}
	return " --ref " + shellArg(ref)
}

func quickstartSlug(value string) string {
	value = strings.ToLower(value)
	value = regexp.MustCompile(`[^a-z0-9._-]+`).ReplaceAllString(value, "-")
	value = strings.Trim(value, "-")
	if value == "" {
		return "repo"
	}
	if len(value) > 80 {
		value = value[:80]
	}
	return value
}

func writeQuickstartReport(outDir string, report quickstartReport) error {
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}
	copy := report
	copy.Markdown = ""
	copy.Script = ""
	data, err := json.MarshalIndent(copy, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(outDir, "quickstart.json"), append(data, '\n'), 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(outDir, "quickstart.md"), []byte(report.Markdown), 0o644); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(outDir, "commands.sh"), []byte(report.Script), 0o755)
}

func quickstartHash(report quickstartReport) string {
	copy := report
	copy.Hash = ""
	copy.Markdown = ""
	copy.Script = ""
	return canonical.Hash(copy)
}

func renderQuickstartScript(report quickstartReport) string {
	var b strings.Builder
	fmt.Fprintf(&b, "#!/usr/bin/env bash\nset -euo pipefail\n\n")
	for _, command := range report.Commands {
		fmt.Fprintf(&b, "%s\n", command.Command)
	}
	return b.String()
}

func renderQuickstartMarkdown(report quickstartReport) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Patchline quickstart\n\n")
	fmt.Fprintf(&b, "Run these three commands from the Patchline checkout:\n\n")
	fmt.Fprintf(&b, "```bash\n")
	for _, command := range report.Commands {
		fmt.Fprintf(&b, "%s\n", command.Command)
	}
	fmt.Fprintf(&b, "```\n\n")
	fmt.Fprintf(&b, "## Expected artifacts\n\n")
	for _, artifact := range report.ExpectedArtifacts {
		fmt.Fprintf(&b, "- `%s` — %s\n", artifact.Path, artifact.Description)
	}
	fmt.Fprintf(&b, "\nHash: `%s`\n", report.Hash)
	return b.String()
}

func pluginsCommand(args []string) error {
	if len(args) == 0 {
		return errors.New("usage: patchline plugins <list|probe> [--json]")
	}
	switch args[0] {
	case "list":
		catalog := plugins.DefaultCatalog()
		if hasFlag(args[1:], "--json") {
			return writeJSON(os.Stdout, catalog)
		}
		fmt.Printf("plugin catalog %s hash=%s\n", catalog.Version, catalog.Hash)
		for _, plugin := range catalog.Plugins {
			fmt.Printf("- %s (%s): %s\n", plugin.Name, plugin.Kind, plugin.Description)
		}
		return nil
	case "probe":
		return pluginsProbe(args[1:])
	default:
		return fmt.Errorf("unknown plugins subcommand %q", args[0])
	}
}

func pluginsProbe(args []string) error {
	fs := flag.NewFlagSet("plugins probe", flag.ContinueOnError)
	fs.SetOutput(ioDiscard{})
	githubRepo := fs.String("github", "", "GitHub owner/repo")
	ref := fs.String("ref", "", "GitHub ref")
	subpath := fs.String("subpath", "", "source subpath")
	outPath := fs.String("out", "", "output directory")
	downloadDir := fs.String("download-dir", "", "download cache directory")
	kind := fs.String("proposal-kind", "all", "proposal kind")
	budget := fs.String("budget", "files=4,lines=80,tokens=12000,changes=2", "proposal budget")
	jsonOut := fs.Bool("json", false, "print JSON")
	positional, flagArgs, err := onePositionalWithFlags(args, map[string]bool{"--json": true})
	if err != nil {
		return err
	}
	if err := fs.Parse(flagArgs); err != nil {
		return err
	}
	if *githubRepo == "" && positional == "" {
		return errors.New("usage: patchline plugins probe [<path>|--github owner/repo] [--ref ref] [--subpath path] [--out dir] [--download-dir dir] [--json]")
	}
	if *githubRepo != "" && *outPath == "" {
		return errors.New("patchline plugins probe --github requires --out so fetched source metadata is persisted")
	}
	report, err := plugins.Probe(context.Background(), plugins.ProbeOptions{
		Path:        positional,
		GitHub:      *githubRepo,
		Ref:         *ref,
		Subpath:     *subpath,
		DownloadDir: *downloadDir,
		OutDir:      *outPath,
		Kind:        *kind,
		Budget:      *budget,
	})
	if err != nil {
		return err
	}
	if *outPath != "" {
		if err := plugins.WriteProbe(*outPath, report); err != nil {
			return err
		}
	}
	if *jsonOut {
		return writeJSON(os.Stdout, report)
	}
	fmt.Printf("plugins probe input=%s plugins=%d files=%d risks=%d generated=%d checks=%d rendered=%d hash=%s\n", report.Input, len(report.Catalog.Plugins), report.Summary.FilesScanned, report.Summary.RankedRisks, report.Summary.GeneratedFiles, report.Summary.GeneratedChecks, report.Summary.RenderedReports, report.Hash)
	return nil
}

func goldenFixtureCommand(args []string) error {
	if len(args) == 0 {
		return errors.New("usage: patchline golden-fixture generate [<path>|--github owner/repo] --out dir [--json]")
	}
	switch args[0] {
	case "generate":
		return goldenFixtureGenerate(args[1:])
	default:
		return fmt.Errorf("unknown golden-fixture subcommand %q", args[0])
	}
}

func goldenFixtureGenerate(args []string) error {
	fs := flag.NewFlagSet("golden-fixture generate", flag.ContinueOnError)
	fs.SetOutput(ioDiscard{})
	githubRepo := fs.String("github", "", "GitHub owner/repo")
	ref := fs.String("ref", "", "GitHub ref")
	subpath := fs.String("subpath", "", "source subpath")
	outPath := fs.String("out", "", "output directory")
	downloadDir := fs.String("download-dir", "", "download cache directory")
	packageName := fs.String("package", "goldenfixture", "generated Go package name")
	testName := fs.String("test-name", "", "generated test function name")
	maxFiles := fs.Int("max-files", 3, "maximum embedded source files")
	maxFileBytes := fs.Int64("max-file-bytes", 24*1024, "maximum bytes per embedded file")
	maxTotalBytes := fs.Int64("max-total-bytes", 48*1024, "maximum total embedded source bytes")
	minRisks := fs.Int("min-ranked-risks", 1, "minimum ranked risks the generated fixture must preserve")
	jsonOut := fs.Bool("json", false, "print JSON")
	positional, flagArgs, err := onePositionalWithFlags(args, map[string]bool{"--json": true})
	if err != nil {
		return err
	}
	if err := fs.Parse(flagArgs); err != nil {
		return err
	}
	if *githubRepo == "" && positional == "" {
		return errors.New("usage: patchline golden-fixture generate [<path>|--github owner/repo] --out dir [--json]")
	}
	if *outPath == "" {
		return errors.New("patchline golden-fixture generate requires --out")
	}
	report, err := goldenfixture.Generate(context.Background(), goldenfixture.Options{
		Path:           positional,
		GitHub:         *githubRepo,
		Ref:            *ref,
		Subpath:        *subpath,
		DownloadDir:    *downloadDir,
		OutDir:         *outPath,
		PackageName:    *packageName,
		TestName:       *testName,
		MaxFiles:       *maxFiles,
		MaxFileBytes:   *maxFileBytes,
		MaxTotalBytes:  *maxTotalBytes,
		MinRankedRisks: *minRisks,
	})
	if err != nil {
		return err
	}
	if *jsonOut {
		return writeJSON(os.Stdout, report)
	}
	fmt.Printf("golden fixture id=%s selected=%d/%d risks=%d test=%s hash=%s\n", report.ID, report.Summary.SelectedFiles, report.Summary.OriginalFilesScanned, report.Expectations.RankedRisks, report.Outputs["test"], report.Hash)
	return nil
}

func parseAnalyzeStages(value string) ([]string, error) {
	if strings.TrimSpace(value) == "" {
		return nil, errors.New("--stages must include at least one stage")
	}
	allowed := map[string]bool{"inventory": true, "baseline": true, "propose": true, "compare": true, "deep": true}
	seen := map[string]bool{}
	var stages []string
	for _, raw := range strings.Split(value, ",") {
		stage := strings.ToLower(strings.TrimSpace(raw))
		if stage == "" {
			continue
		}
		if !allowed[stage] {
			return nil, fmt.Errorf("unknown analyze stage %q", stage)
		}
		if !seen[stage] {
			seen[stage] = true
			stages = append(stages, stage)
		}
	}
	if len(stages) == 0 {
		return nil, errors.New("--stages must include at least one stage")
	}
	return stages, nil
}

func analyzeStageSet(stages []string) map[string]bool {
	out := map[string]bool{}
	for _, stage := range stages {
		out[stage] = true
	}
	return out
}

func analyzeNeeds(stages map[string]bool, stage string) bool {
	if stages[stage] {
		return true
	}
	switch stage {
	case "inventory":
		return stages["baseline"] || stages["propose"] || stages["compare"] || stages["deep"]
	case "baseline":
		return stages["propose"] || stages["compare"] || stages["deep"]
	case "propose":
		return stages["compare"]
	case "compare":
		return stages["deep"]
	default:
		return false
	}
}

func stringSliceContains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func writeRepoAnalyzeReport(outDir string, report repoAnalyzeReport) error {
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(outDir, "analyze.json"), append(data, '\n'), 0o644); err != nil {
		return err
	}
	var b strings.Builder
	fmt.Fprintf(&b, "# Patchline repo analyze\n\n")
	fmt.Fprintf(&b, "- input: `%s`\n", report.Input)
	fmt.Fprintf(&b, "- stages: `%s`\n", strings.Join(report.Stages, ","))
	fmt.Fprintf(&b, "- ci: `%t`\n", report.CI)
	fmt.Fprintf(&b, "- resume: `%t`\n", report.Resume)
	fmt.Fprintf(&b, "- redact: `%t`\n", report.Redact)
	if report.CIArtifacts.ActionsSnippet != "" {
		fmt.Fprintf(&b, "- ci_upload_snippet: `%s`\n", report.CIArtifacts.ActionsSnippet)
	}
	if report.CommandsPath != "" {
		fmt.Fprintf(&b, "- commands: `%s`\n", report.CommandsPath)
	}
	if report.Diagnostics != nil && report.Diagnostics.SummaryPath != "" {
		fmt.Fprintf(&b, "- diagnostics: `%s` (%d spans, %d logs, %d failed spans)\n", report.Diagnostics.SummaryPath, report.Diagnostics.Spans, report.Diagnostics.Logs, report.Diagnostics.FailedSpans)
	}
	if len(report.ReusedStages) > 0 {
		fmt.Fprintf(&b, "- reused_stages: `%s`\n", strings.Join(report.ReusedStages, ","))
	}
	fmt.Fprintf(&b, "- hash: `%s`\n\n", report.Hash)
	fmt.Fprintf(&b, "## Summary\n\n| area | count |\n| --- | ---: |\n")
	fmt.Fprintf(&b, "| files scanned | %d |\n", report.Summary.FilesScanned)
	fmt.Fprintf(&b, "| facts | %d |\n", report.Summary.Facts)
	fmt.Fprintf(&b, "| ranked risks | %d |\n", report.Summary.RankedRisks)
	fmt.Fprintf(&b, "| ranking explanations | %d |\n", report.Summary.RankingExplanations)
	fmt.Fprintf(&b, "| provenance slices | %d |\n", report.Summary.ProvenanceSlices)
	fmt.Fprintf(&b, "| policy checks | %d |\n", report.Summary.PolicyChecks)
	fmt.Fprintf(&b, "| repair proof summaries | %d |\n", report.Summary.RepairProofSummaries)
	fmt.Fprintf(&b, "| generated files | %d |\n", report.Summary.GeneratedFiles)
	if report.Summary.ProposalGenerator != "" {
		fmt.Fprintf(&b, "| proposal generator | %s |\n", report.Summary.ProposalGenerator)
	}
	fmt.Fprintf(&b, "| deterministic only | %t |\n", report.Summary.DeterministicOnly)
	if report.Summary.ScopeBudget != "" {
		fmt.Fprintf(&b, "| scope budget | %s |\n", report.Summary.ScopeBudget)
	}
	fmt.Fprintf(&b, "| intervention loops | %d |\n", report.Summary.InterventionLoops)
	fmt.Fprintf(&b, "| compare checks failed | %d |\n\n", report.Summary.CompareChecksFailed)
	fmt.Fprintf(&b, "## Outputs\n\n| stage | path |\n| --- | --- |\n")
	keys := make([]string, 0, len(report.Outputs))
	for key := range report.Outputs {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		fmt.Fprintf(&b, "| %s | `%s` |\n", key, report.Outputs[key])
	}
	return os.WriteFile(filepath.Join(outDir, "analyze.md"), []byte(b.String()), 0o644)
}

func buildMaintainerTriage(baseline project.BaselineReport, proposal project.ProposalReport, compare project.CompareReport) maintainerTriageReport {
	report := maintainerTriageReport{
		Version:      "patchline.maintainer-triage/v1",
		BaselineHash: baseline.Hash,
		ProposalHash: proposal.OutputHash,
		CompareHash:  compare.Hash,
	}
	report.OwnerRoutes = append(report.OwnerRoutes, baseline.OwnerRoutes...)
	report.OwnerRoutes = append(report.OwnerRoutes, proposal.OwnerRoutes...)
	ownersByRisk := map[string][]string{}
	for _, route := range baseline.OwnerRoutes {
		if route.SubjectKind == "risk" {
			ownersByRisk[route.SubjectID] = mergeTriageReviewers(ownersByRisk[route.SubjectID], route.Owners)
		}
	}
	groups := []maintainerTriageGroup{
		{Surface: "migrations", OwnerHint: "database or migration owner", Rationale: "schema/data migrations and raw migration SQL with ranked risk"},
		{Surface: "app_write_paths", OwnerHint: "service or feature owner", Rationale: "application source paths that write persistent data"},
		{Surface: "jobs", OwnerHint: "background job owner", Rationale: "workers, jobs, cron tasks, and async data changes"},
		{Surface: "tests", OwnerHint: "test owner", Rationale: "native tests and test commands linked to risky data changes"},
		{Surface: "incidents", OwnerHint: "on-call or reliability owner", Rationale: "incident-like paths and linked operational evidence"},
		{Surface: "runbooks", OwnerHint: "operations or repair owner", Rationale: "runbooks, repair paths, rollback notes, and repair clusters"},
		{Surface: "generated_interventions", OwnerHint: "reviewer of generated artifacts", Rationale: "untrusted generated tests, guards, instrumentation, and repairs"},
	}
	bySurface := map[string]*maintainerTriageGroup{}
	for i := range groups {
		bySurface[groups[i].Surface] = &groups[i]
	}
	for _, risk := range baseline.Risks {
		addTriageRisk(bySurface[triageRiskSurface(risk)], risk, ownersByRisk[risk.ID])
	}
	for _, slice := range baseline.Provenance {
		for _, path := range slice.SourcePaths {
			addTriagePath(bySurface["app_write_paths"], path)
		}
		for _, path := range slice.IncidentPaths {
			addTriagePath(bySurface["incidents"], path)
		}
		for _, path := range slice.RepairPaths {
			addTriagePath(bySurface["runbooks"], path)
		}
		for _, command := range slice.TestCommands {
			addTriageCommand(bySurface["tests"], command)
		}
		for _, command := range slice.NativeCommands {
			addTriageCommand(bySurface["tests"], command)
		}
	}
	for _, cluster := range baseline.CauseClusters {
		addTriagePath(bySurface["incidents"], cluster.Path)
	}
	for _, cluster := range baseline.RepairClusters {
		addTriagePath(bySurface["runbooks"], cluster.Path)
	}
	for _, command := range baseline.NativeChecks {
		addTriageCommand(bySurface["tests"], command)
	}
	for _, generated := range proposal.GeneratedFiles {
		bySurface["generated_interventions"].GeneratedFiles = append(bySurface["generated_interventions"].GeneratedFiles, generated)
		bySurface["generated_interventions"].LikelyReviewers = mergeTriageReviewers(bySurface["generated_interventions"].LikelyReviewers, generated.Reviewers)
	}
	for i := range groups {
		sort.Slice(groups[i].TopRisks, func(a, b int) bool {
			if groups[i].TopRisks[a].Score != groups[i].TopRisks[b].Score {
				return groups[i].TopRisks[a].Score > groups[i].TopRisks[b].Score
			}
			return groups[i].TopRisks[a].ID < groups[i].TopRisks[b].ID
		})
		if len(groups[i].TopRisks) > 10 {
			groups[i].TopRisks = groups[i].TopRisks[:10]
		}
		sort.Strings(groups[i].EvidencePaths)
		sort.Strings(groups[i].LikelyReviewers)
		sort.Slice(groups[i].GeneratedFiles, func(a, b int) bool { return groups[i].GeneratedFiles[a].Path < groups[i].GeneratedFiles[b].Path })
		sort.Slice(groups[i].NativeChecks, func(a, b int) bool { return groups[i].NativeChecks[a].Command < groups[i].NativeChecks[b].Command })
		groups[i].FindingCount = len(groups[i].TopRisks) + len(groups[i].EvidencePaths) + len(groups[i].GeneratedFiles) + len(groups[i].NativeChecks)
		report.Summary.TotalFindings += groups[i].FindingCount
		report.Summary.GeneratedInterventions += len(groups[i].GeneratedFiles)
		report.Summary.NativeChecks += len(groups[i].NativeChecks)
		if groups[i].FindingCount > 0 {
			report.Summary.GroupsWithFindings++
		}
	}
	report.Groups = groups
	report.Summary.Groups = len(groups)
	report.Summary.OwnerRoutes = len(report.OwnerRoutes)
	report.Summary.OwnerRouteOwners = countTriageRouteOwners(report.OwnerRoutes)
	report.Hash = maintainerTriageHash(report)
	report.Markdown = renderMaintainerTriageMarkdown(report)
	return report
}

func triageRiskSurface(risk project.BaselineRisk) string {
	path := strings.ToLower(risk.Path)
	kind := strings.ToLower(risk.Kind)
	switch {
	case strings.Contains(path, "test") || strings.Contains(path, "spec"):
		return "tests"
	case strings.Contains(path, "job") || strings.Contains(path, "worker") || strings.Contains(path, "cron") || strings.Contains(path, "task"):
		return "jobs"
	case strings.Contains(path, "incident") || strings.Contains(path, "postmortem") || strings.Contains(path, "outage"):
		return "incidents"
	case strings.Contains(path, "runbook") || strings.Contains(path, "rollback") || strings.Contains(path, "repair") || strings.Contains(path, "reconcile"):
		return "runbooks"
	case strings.Contains(path, "migrate") || strings.Contains(path, "migration") || strings.Contains(kind, "schema") || strings.Contains(kind, "sql"):
		return "migrations"
	default:
		return "app_write_paths"
	}
}

func addTriageRisk(group *maintainerTriageGroup, risk project.BaselineRisk, reviewers []string) {
	if group == nil {
		return
	}
	group.TopRisks = append(group.TopRisks, maintainerTriageRisk{ID: risk.ID, Path: risk.Path, Kind: risk.Kind, Table: risk.Table, Severity: risk.Severity, Score: risk.Score, Reviewers: reviewers})
	group.LikelyReviewers = mergeTriageReviewers(group.LikelyReviewers, reviewers)
}

func addTriagePath(group *maintainerTriageGroup, path string) {
	if group == nil || strings.TrimSpace(path) == "" {
		return
	}
	for _, existing := range group.EvidencePaths {
		if existing == path {
			return
		}
	}
	group.EvidencePaths = append(group.EvidencePaths, path)
}

func addTriageCommand(group *maintainerTriageGroup, command project.Command) {
	if group == nil || strings.TrimSpace(command.Command) == "" {
		return
	}
	for _, existing := range group.NativeChecks {
		if existing.Command == command.Command {
			return
		}
	}
	group.NativeChecks = append(group.NativeChecks, command)
}

func mergeTriageReviewers(existing, values []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, value := range existing {
		if value != "" && !seen[value] {
			seen[value] = true
			out = append(out, value)
		}
	}
	for _, value := range values {
		if value != "" && !seen[value] {
			seen[value] = true
			out = append(out, value)
		}
	}
	sort.Strings(out)
	return out
}

func countTriageRouteOwners(routes []project.OwnerRoute) int {
	owners := map[string]bool{}
	for _, route := range routes {
		for _, owner := range route.Owners {
			if owner != "" {
				owners[owner] = true
			}
		}
	}
	return len(owners)
}

func writeMaintainerTriage(outDir string, report maintainerTriageReport) error {
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}
	copy := report
	copy.Markdown = ""
	data, err := json.MarshalIndent(copy, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(outDir, "triage.json"), append(data, '\n'), 0o644); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(outDir, "triage.md"), []byte(report.Markdown), 0o644)
}

func maintainerTriageHash(report maintainerTriageReport) string {
	copy := report
	copy.Hash = ""
	copy.Markdown = ""
	return canonical.Hash(copy)
}

func renderMaintainerTriageMarkdown(report maintainerTriageReport) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Patchline maintainer triage\n\n")
	fmt.Fprintf(&b, "- baseline_hash: `%s`\n", report.BaselineHash)
	if report.ProposalHash != "" {
		fmt.Fprintf(&b, "- proposal_hash: `%s`\n", report.ProposalHash)
	}
	if report.CompareHash != "" {
		fmt.Fprintf(&b, "- compare_hash: `%s`\n", report.CompareHash)
	}
	fmt.Fprintf(&b, "- hash: `%s`\n\n", report.Hash)
	fmt.Fprintf(&b, "## Summary\n\n| area | count |\n| --- | ---: |\n")
	fmt.Fprintf(&b, "| groups | %d |\n", report.Summary.Groups)
	fmt.Fprintf(&b, "| groups with findings | %d |\n", report.Summary.GroupsWithFindings)
	fmt.Fprintf(&b, "| total findings | %d |\n", report.Summary.TotalFindings)
	fmt.Fprintf(&b, "| generated interventions | %d |\n", report.Summary.GeneratedInterventions)
	fmt.Fprintf(&b, "| native checks | %d |\n", report.Summary.NativeChecks)
	fmt.Fprintf(&b, "| CODEOWNERS routes | %d |\n", report.Summary.OwnerRoutes)
	fmt.Fprintf(&b, "| CODEOWNERS owners | %d |\n\n", report.Summary.OwnerRouteOwners)
	fmt.Fprintf(&b, "## Owner surfaces\n\n| surface | owner hint | likely reviewers | findings | top risk | rationale |\n| --- | --- | --- | ---: | --- | --- |\n")
	for _, group := range report.Groups {
		topRisk := ""
		if len(group.TopRisks) > 0 {
			topRisk = fmt.Sprintf("%s (%s %d)", group.TopRisks[0].ID, group.TopRisks[0].Severity, group.TopRisks[0].Score)
		}
		fmt.Fprintf(&b, "| %s | %s | %s | %d | %s | %s |\n", group.Surface, group.OwnerHint, strings.Join(group.LikelyReviewers, ", "), group.FindingCount, topRisk, group.Rationale)
	}
	if len(report.OwnerRoutes) > 0 {
		fmt.Fprintf(&b, "\n## CODEOWNERS routes\n\n| subject | path | likely reviewers | confidence |\n| --- | --- | --- | --- |\n")
		for _, route := range report.OwnerRoutes {
			fmt.Fprintf(&b, "| `%s` | %s | %s | %s |\n", route.SubjectID, route.Path, strings.Join(route.Owners, ", "), route.Confidence)
		}
	}
	return b.String()
}

func writeAnalysisBundle(outDir string, report repoAnalyzeReport) error {
	bundleDir := filepath.Join(outDir, "analysis-bundle")
	if err := os.MkdirAll(bundleDir, 0o755); err != nil {
		return err
	}
	redactor := newBundleRedactor()
	copyIfExists := func(src, name string) error {
		if src == "" {
			return nil
		}
		if _, err := os.Stat(src); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return nil
			}
			return err
		}
		return copyBundleFile(src, filepath.Join(bundleDir, name), report.Redact, redactor)
	}
	if fetchOut := report.Outputs["fetch"]; fetchOut != "" {
		if err := copyIfExists(filepath.Join(fetchOut, "source.json"), "source.json"); err != nil {
			return err
		}
	} else {
		source := map[string]any{"input": report.Input, "subpath": report.Subpath, "redacted_source": report.Redact}
		data, err := json.MarshalIndent(source, "", "  ")
		if err != nil {
			return err
		}
		if report.Redact {
			data, err = redactor.redactJSONBytes(data)
			if err != nil {
				return err
			}
		}
		if err := os.WriteFile(filepath.Join(bundleDir, "source.json"), append(data, '\n'), 0o644); err != nil {
			return err
		}
	}
	if inventoryOut := report.Outputs["inventory"]; inventoryOut != "" {
		if err := copyIfExists(filepath.Join(inventoryOut, "facts.jsonl"), "facts.jsonl"); err != nil {
			return err
		}
	}
	if baselineOut := report.Outputs["baseline"]; baselineOut != "" {
		if err := copyIfExists(filepath.Join(baselineOut, "baseline.json"), "baseline.json"); err != nil {
			return err
		}
		if err := copyIfExists(filepath.Join(baselineOut, "baseline.sarif"), "summary.sarif"); err != nil {
			return err
		}
	}
	if proposalOut := report.Outputs["proposal"]; proposalOut != "" {
		if err := copyIfExists(filepath.Join(proposalOut, "proposal.patch"), "proposal.patch"); err != nil {
			return err
		}
	}
	if compareOut := report.Outputs["compare"]; compareOut != "" {
		if err := copyIfExists(filepath.Join(compareOut, "compare.json"), "compare.json"); err != nil {
			return err
		}
	}
	if triageOut := report.Outputs["triage"]; triageOut != "" {
		if err := copyIfExists(filepath.Join(triageOut, "triage.json"), "triage.json"); err != nil {
			return err
		}
		if err := copyIfExists(filepath.Join(triageOut, "triage.md"), "triage.md"); err != nil {
			return err
		}
	}
	if commandsPath := report.Outputs["commands"]; commandsPath != "" {
		if err := copyIfExists(commandsPath, "commands.md"); err != nil {
			return err
		}
	}
	return copyIfExists(filepath.Join(outDir, "analyze.md"), "summary.md")
}

func writeCopyCommandsReport(outDir string, report repoAnalyzeReport, githubInput bool, githubRef, proposalKind, llmCommand, budget string, noLLM, promptNoFacts, redact, ciMode bool) error {
	var b strings.Builder
	fmt.Fprintf(&b, "# Copy these Patchline commands\n\n")
	fmt.Fprintf(&b, "Run this first for an end-to-end analysis of the same project slice:\n\n")
	fmt.Fprintf(&b, "```bash\n")
	fmt.Fprintf(&b, "go run ./cmd/patchline repo analyze")
	if githubInput {
		fmt.Fprintf(&b, " --github %s", shellArg(report.Input))
		if githubRef != "" {
			fmt.Fprintf(&b, " --ref %s", shellArg(githubRef))
		}
	} else {
		fmt.Fprintf(&b, " %s", shellArg(report.Input))
	}
	if report.Subpath != "" {
		fmt.Fprintf(&b, " --subpath %s", shellArg(report.Subpath))
	}
	fmt.Fprintf(&b, " --stages %s", shellArg(strings.Join(report.Stages, ",")))
	if proposalKind != "" {
		fmt.Fprintf(&b, " --proposal-kind %s", shellArg(proposalKind))
	}
	if budget != "" {
		fmt.Fprintf(&b, " --budget %s", shellArg(budget))
	}
	if llmCommand != "" {
		fmt.Fprintf(&b, " --llm-command %s", shellArg(llmCommand))
	}
	if promptNoFacts {
		fmt.Fprintf(&b, " --prompt-without-facts")
	}
	if ciMode {
		fmt.Fprintf(&b, " --ci")
	}
	if redact {
		fmt.Fprintf(&b, " --redact")
	}
	if noLLM {
		fmt.Fprintf(&b, " --no-llm")
	}
	fmt.Fprintf(&b, " --out %s\n", shellArg(outDir))
	fmt.Fprintf(&b, "```\n\n")
	fmt.Fprintf(&b, "If you want to inspect each stage separately, run:\n\n```bash\n")
	if githubInput {
		fmt.Fprintf(&b, "go run ./cmd/patchline repo fetch %s", shellArg(report.Input))
		if githubRef != "" {
			fmt.Fprintf(&b, " --ref %s", shellArg(githubRef))
		}
		if report.Subpath != "" {
			fmt.Fprintf(&b, " --subpath %s", shellArg(report.Subpath))
		}
		fmt.Fprintf(&b, " --out %s\n", shellArg(filepath.Join(outDir, "fetch")))
		fmt.Fprintf(&b, "SCAN_ROOT=%s\n", shellArg(report.Source.ScannedRoot))
	} else {
		fmt.Fprintf(&b, "SCAN_ROOT=%s\n", shellArg(report.Input))
	}
	fmt.Fprintf(&b, "go run ./cmd/patchline repo inventory \"$SCAN_ROOT\" --out %s\n", shellArg(filepath.Join(outDir, "inventory")))
	fmt.Fprintf(&b, "go run ./cmd/patchline intake \"$SCAN_ROOT\" --out %s\n", shellArg(filepath.Join(outDir, "intake")))
	fmt.Fprintf(&b, "go run ./cmd/patchline repo baseline --inventory %s --intake %s --out %s\n", shellArg(filepath.Join(outDir, "inventory")), shellArg(filepath.Join(outDir, "intake")), shellArg(filepath.Join(outDir, "baseline")))
	fmt.Fprintf(&b, "go run ./cmd/patchline repo propose --from-report %s --proposal-kind %s", shellArg(filepath.Join(outDir, "baseline")), shellArg(firstNonEmpty(proposalKind, "all")))
	if budget != "" {
		fmt.Fprintf(&b, " --budget %s", shellArg(budget))
	}
	if llmCommand != "" {
		fmt.Fprintf(&b, " --llm-command %s", shellArg(llmCommand))
	}
	if promptNoFacts {
		fmt.Fprintf(&b, " --prompt-without-facts")
	}
	if noLLM {
		fmt.Fprintf(&b, " --no-llm")
	}
	fmt.Fprintf(&b, " --out %s\n", shellArg(filepath.Join(outDir, "proposal")))
	fmt.Fprintf(&b, "go run ./cmd/patchline repo compare --before %s --after %s --out %s\n", shellArg(filepath.Join(outDir, "baseline")), shellArg(filepath.Join(outDir, "proposal")), shellArg(filepath.Join(outDir, "compare")))
	fmt.Fprintf(&b, "```\n\n")
	fmt.Fprintf(&b, "Shareable outputs:\n\n")
	fmt.Fprintf(&b, "- analysis bundle: `%s`\n", filepath.Join(outDir, "analysis-bundle"))
	fmt.Fprintf(&b, "- SARIF: `%s`\n", filepath.Join(outDir, "analysis-bundle", "summary.sarif"))
	fmt.Fprintf(&b, "- summary: `%s`\n", filepath.Join(outDir, "analysis-bundle", "summary.md"))
	return os.WriteFile(filepath.Join(outDir, "commands.md"), []byte(b.String()), 0o644)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func shellArg(value string) string {
	if value == "" {
		return "''"
	}
	if regexp.MustCompile(`^[A-Za-z0-9_./:@,=-]+$`).MatchString(value) {
		return value
	}
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}

func writeRepoAnalyzeCIArtifacts(outDir string, report repoAnalyzeReport) (repoAnalyzeCIArtifacts, error) {
	ciDir := filepath.Join(outDir, "ci")
	if err := os.MkdirAll(ciDir, 0o755); err != nil {
		return repoAnalyzeCIArtifacts{}, err
	}
	artifacts := repoAnalyzeCIArtifacts{
		SummaryPath:           filepath.Join(ciDir, "summary.md"),
		SARIFPath:             filepath.Join(outDir, "analysis-bundle", "summary.sarif"),
		GitLabCodeQualityPath: filepath.Join(ciDir, "gl-code-quality-report.json"),
		BitbucketInsightsPath: filepath.Join(ciDir, "bitbucket-code-insights.json"),
		BundlePath:            filepath.Join(outDir, "analysis-bundle"),
		ActionsSnippet:        filepath.Join(ciDir, "github-actions-upload.yml"),
		GitLabSnippet:         filepath.Join(ciDir, "gitlab-ci-snippet.yml"),
		BitbucketSnippet:      filepath.Join(ciDir, "bitbucket-pipelines-snippet.yml"),
		ArtifactName:          "patchline-analysis-bundle",
		CodeScanningTool:      "patchline",
	}
	baseline, err := project.LoadBaseline(filepath.Join(outDir, "baseline"))
	if err != nil {
		return repoAnalyzeCIArtifacts{}, err
	}
	var summary strings.Builder
	fmt.Fprintf(&summary, "# Patchline CI analysis\n\n")
	fmt.Fprintf(&summary, "- risks: %d\n", report.Summary.RankedRisks)
	fmt.Fprintf(&summary, "- generated files: %d\n", report.Summary.GeneratedFiles)
	fmt.Fprintf(&summary, "- intervention loops: %d\n", report.Summary.InterventionLoops)
	fmt.Fprintf(&summary, "- SARIF: `%s`\n", artifacts.SARIFPath)
	fmt.Fprintf(&summary, "- GitLab Code Quality: `%s`\n", artifacts.GitLabCodeQualityPath)
	fmt.Fprintf(&summary, "- Bitbucket Code Insights JSON: `%s`\n", artifacts.BitbucketInsightsPath)
	fmt.Fprintf(&summary, "- bundle: `%s`\n", artifacts.BundlePath)
	if err := os.WriteFile(artifacts.SummaryPath, []byte(summary.String()), 0o644); err != nil {
		return repoAnalyzeCIArtifacts{}, err
	}
	if err := writeGitLabCodeQuality(artifacts.GitLabCodeQualityPath, baseline); err != nil {
		return repoAnalyzeCIArtifacts{}, err
	}
	if err := writeBitbucketInsights(artifacts.BitbucketInsightsPath, baseline); err != nil {
		return repoAnalyzeCIArtifacts{}, err
	}
	snippet := fmt.Sprintf(`- name: Upload Patchline SARIF
  uses: github/codeql-action/upload-sarif@v3
  with:
    sarif_file: %s
- name: Store Patchline analysis bundle
  uses: actions/upload-artifact@v4
  with:
    name: %s
    path: %s
`, artifacts.SARIFPath, artifacts.ArtifactName, artifacts.BundlePath)
	if err := os.WriteFile(artifacts.ActionsSnippet, []byte(snippet), 0o644); err != nil {
		return repoAnalyzeCIArtifacts{}, err
	}
	gitlabSnippet := fmt.Sprintf(`patchline:
  image: golang:1.24
  script:
    - go run github.com/thehalleyyoung/patchline/cmd/patchline@main repo analyze . --stages inventory,baseline --ci --out results/patchline
  artifacts:
    when: always
    paths:
      - %s
    reports:
      codequality: %s
`, artifacts.BundlePath, artifacts.GitLabCodeQualityPath)
	if err := os.WriteFile(artifacts.GitLabSnippet, []byte(gitlabSnippet), 0o644); err != nil {
		return repoAnalyzeCIArtifacts{}, err
	}
	bitbucketSnippet := fmt.Sprintf(`pipelines:
  pull-requests:
    "**":
      - step:
          name: Patchline data-risk analysis
          image: golang:1.24
          script:
            - go run github.com/thehalleyyoung/patchline/cmd/patchline@main repo analyze . --stages inventory,baseline --ci --out results/patchline
          artifacts:
            - %s/**
            - %s
`, artifacts.BundlePath, artifacts.BitbucketInsightsPath)
	if err := os.WriteFile(artifacts.BitbucketSnippet, []byte(bitbucketSnippet), 0o644); err != nil {
		return repoAnalyzeCIArtifacts{}, err
	}
	if stepSummary := os.Getenv("GITHUB_STEP_SUMMARY"); stepSummary != "" {
		file, err := os.OpenFile(stepSummary, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if err != nil {
			return repoAnalyzeCIArtifacts{}, err
		}
		if _, err := file.WriteString(summary.String() + "\n"); err != nil {
			_ = file.Close()
			return repoAnalyzeCIArtifacts{}, err
		}
		if err := file.Close(); err != nil {
			return repoAnalyzeCIArtifacts{}, err
		}
		artifacts.GitHubStepSummary = true
	}
	return artifacts, nil
}

func writeGitLabCodeQuality(path string, baseline project.BaselineReport) error {
	issues := make([]gitlabCodeQualityIssue, 0, len(baseline.Risks))
	for _, risk := range baseline.Risks {
		issues = append(issues, gitlabCodeQualityIssue{
			Description: fmt.Sprintf("%s data-risk score %d: %s", risk.Severity, risk.Score, risk.Rationale),
			CheckName:   "patchline/" + firstNonEmpty(risk.Kind, "data-risk"),
			Fingerprint: firstNonEmpty(risk.StableID, risk.ID, canonical.Hash(risk.Path+"\x00"+risk.Kind+"\x00"+risk.Table)),
			Severity:    gitlabCodeQualitySeverity(risk.Severity),
			Location: gitlabCodeQualityLocation{
				Path: firstNonEmpty(risk.Path, "unknown"),
				Lines: gitlabCodeQualityLines{
					Begin: riskLine(risk),
				},
			},
		})
	}
	data, err := json.MarshalIndent(issues, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}

func writeBitbucketInsights(path string, baseline project.BaselineReport) error {
	report := bitbucketCodeInsightsReport{
		Version:  "patchline.bitbucket-code-insights/v1",
		Title:    "Patchline data-risk analysis",
		Details:  fmt.Sprintf("%d ranked data-risk findings from Patchline baseline %s", len(baseline.Risks), baseline.Hash),
		Reporter: "patchline",
		Result:   "PASSED",
	}
	for _, risk := range baseline.Risks {
		if risk.Severity == "high" {
			report.Result = "FAILED"
		}
		report.Annotations = append(report.Annotations, bitbucketCodeInsightAnnotation{
			ExternalID: firstNonEmpty(risk.StableID, risk.ID),
			Title:      firstNonEmpty(risk.Kind, "data-risk"),
			Summary:    fmt.Sprintf("%s risk score %d: %s", risk.Severity, risk.Score, risk.Rationale),
			Severity:   bitbucketInsightSeverity(risk.Severity),
			Path:       firstNonEmpty(risk.Path, "unknown"),
			Line:       riskLine(risk),
		})
	}
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}

func riskLine(risk project.BaselineRisk) int {
	if risk.Statement > 0 {
		return risk.Statement
	}
	return 1
}

func gitlabCodeQualitySeverity(severity string) string {
	switch severity {
	case "high":
		return "major"
	case "medium":
		return "minor"
	default:
		return "info"
	}
}

func bitbucketInsightSeverity(severity string) string {
	switch severity {
	case "high":
		return "HIGH"
	case "medium":
		return "MEDIUM"
	default:
		return "LOW"
	}
}

func copyBundleFile(src, dst string, redact bool, redactor *bundleRedactor) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	if redact {
		data, err = redactor.redactFileBytes(src, data)
		if err != nil {
			return err
		}
	}
	return os.WriteFile(dst, data, 0o644)
}

type bundleRedactor struct {
	tokenByValue map[string]string
	wordPattern  *regexp.Regexp
	emailPattern *regexp.Regexp
	quotePattern *regexp.Regexp
}

func newBundleRedactor() *bundleRedactor {
	return &bundleRedactor{
		tokenByValue: map[string]string{},
		wordPattern:  regexp.MustCompile(`[A-Za-z_][A-Za-z0-9_./:@-]{2,}`),
		emailPattern: regexp.MustCompile(`[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}`),
		quotePattern: regexp.MustCompile(`'[^']{1,120}'|"[^"\n]{1,120}"`),
	}
}

func (r *bundleRedactor) redactFileBytes(path string, data []byte) ([]byte, error) {
	switch {
	case strings.HasSuffix(path, ".json"), strings.HasSuffix(path, ".sarif"):
		return r.redactJSONBytes(data)
	case strings.HasSuffix(path, ".jsonl"):
		lines := strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
		for i, line := range lines {
			if strings.TrimSpace(line) == "" {
				continue
			}
			redacted, err := r.redactJSONBytes([]byte(line))
			if err != nil {
				lines[i] = r.redactText(line)
				continue
			}
			lines[i] = strings.TrimSuffix(string(redacted), "\n")
		}
		return []byte(strings.Join(lines, "\n") + "\n"), nil
	default:
		return []byte(r.redactText(string(data))), nil
	}
}

func (r *bundleRedactor) redactJSONBytes(data []byte) ([]byte, error) {
	var value any
	if err := json.Unmarshal(data, &value); err != nil {
		return nil, err
	}
	value = r.redactJSONValue("", value)
	out, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(out, '\n'), nil
}

func (r *bundleRedactor) redactJSONValue(key string, value any) any {
	switch v := value.(type) {
	case map[string]any:
		out := make(map[string]any, len(v))
		for childKey, childValue := range v {
			out[childKey] = r.redactJSONValue(childKey, childValue)
		}
		return out
	case []any:
		for i := range v {
			v[i] = r.redactJSONValue(key, v[i])
		}
		return v
	case string:
		if preserveRedactionValue(key, v) {
			return v
		}
		if shouldRedactString(key, v) {
			return r.token(redactionKind(key, v), v)
		}
		return r.redactText(v)
	default:
		return v
	}
}

func preserveRedactionValue(key, value string) bool {
	lowerKey := strings.ToLower(key)
	lowerValue := strings.ToLower(value)
	if strings.Contains(lowerKey, "hash") || lowerKey == "version" || lowerKey == "status" || lowerKey == "severity" || lowerKey == "level" {
		return true
	}
	switch lowerValue {
	case "", "true", "false", "pass", "fail", "warn", "high", "medium", "low", "checked", "conditional", "open", "refuted":
		return true
	default:
		return false
	}
}

func shouldRedactString(key, value string) bool {
	lowerKey := strings.ToLower(key)
	lowerValue := strings.ToLower(value)
	if strings.Contains(lowerKey, "hash") {
		return false
	}
	if containsAny(lowerKey, "id", "path", "uri", "input", "repo", "owner", "subpath", "table", "column", "identifier", "name", "email", "customer", "literal", "secret", "token", "password", "authorization", "command", "rationale", "reason", "message", "source", "target", "expr", "expect") {
		return strings.TrimSpace(value) != ""
	}
	return containsAny(lowerValue, "secret", "token", "password", "authorization", "bearer", "api_key", "apikey") || strings.Contains(value, "@")
}

func redactionKind(key, value string) string {
	lower := strings.ToLower(key + " " + value)
	switch {
	case containsAny(lower, "secret", "token", "password", "authorization", "bearer", "api_key", "apikey"):
		return "secret"
	case containsAny(lower, "customer", "email") || strings.Contains(value, "@"):
		return "customer"
	case containsAny(lower, "table", "column", "identifier", "id", "name"):
		return "identifier"
	case containsAny(lower, "literal", "expr", "expect"):
		return "literal"
	default:
		return "value"
	}
}

func (r *bundleRedactor) redactText(text string) string {
	text = r.emailPattern.ReplaceAllStringFunc(text, func(value string) string {
		return r.token("customer", value)
	})
	text = r.quotePattern.ReplaceAllStringFunc(text, func(value string) string {
		return r.token("literal", value)
	})
	return r.wordPattern.ReplaceAllStringFunc(text, func(value string) string {
		if preserveTextToken(value) {
			return value
		}
		if containsAny(strings.ToLower(value), "secret", "token", "password", "authorization", "bearer", "api_key", "apikey") {
			return r.token("secret", value)
		}
		if strings.Contains(value, "/") || strings.Contains(value, ".") || strings.Contains(value, "_") || strings.Contains(value, "-") {
			return r.token("identifier", value)
		}
		return value
	})
}

func preserveTextToken(value string) bool {
	lower := strings.ToLower(value)
	if len(value) <= 3 || strings.HasPrefix(lower, "sha256:") || regexp.MustCompile(`^[0-9a-f]{12,64}$`).MatchString(lower) {
		return true
	}
	switch lower {
	case "patchline", "repo", "analyze", "baseline", "proposal", "compare", "summary", "sarif", "json", "true", "false", "pass", "fail", "warn", "high", "medium", "low", "risk", "risks", "files", "facts", "stage", "stages", "hash", "version", "generated", "checks", "redact", "resume":
		return true
	default:
		return false
	}
}

func (r *bundleRedactor) token(kind, value string) string {
	if existing := r.tokenByValue[kind+"\x00"+value]; existing != "" {
		return existing
	}
	token := "[redacted:" + kind + ":" + canonical.Hash(value)[:12] + "]"
	r.tokenByValue[kind+"\x00"+value] = token
	return token
}

func containsAny(text string, needles ...string) bool {
	for _, needle := range needles {
		if strings.Contains(text, needle) {
			return true
		}
	}
	return false
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func loadSource(path string) (project.Source, error) {
	var source project.Source
	data, err := os.ReadFile(path)
	if err != nil {
		return source, err
	}
	if err := json.Unmarshal(data, &source); err != nil {
		return source, err
	}
	return source, nil
}

func loadCompareReport(path string) (project.CompareReport, error) {
	var report project.CompareReport
	data, err := os.ReadFile(path)
	if err != nil {
		return report, err
	}
	if err := json.Unmarshal(data, &report); err != nil {
		return report, err
	}
	return report, nil
}

func repoInventory(args []string) error {
	fs := flag.NewFlagSet("repo inventory", flag.ContinueOnError)
	fs.SetOutput(ioDiscard{})
	outPath := fs.String("out", "", "output directory")
	full := fs.Bool("full", false, "include normally skipped directories")
	jsonOut := fs.Bool("json", false, "emit JSON")
	input, flagArgs, err := onePositionalWithFlags(args, map[string]bool{"--json": true, "--full": true})
	if err != nil {
		return err
	}
	if err := fs.Parse(flagArgs); err != nil {
		return err
	}
	if input == "" || fs.NArg() != 0 {
		return errors.New("usage: patchline repo inventory <path> [--out dir] [--full] [--json]")
	}
	inv, err := project.InventoryPath(project.InventoryOptions{Path: input, Full: *full})
	if err != nil {
		return err
	}
	if *outPath != "" {
		if err := project.WriteInventory(*outPath, inv); err != nil {
			return err
		}
	}
	if *jsonOut {
		return writeJSON(os.Stdout, inv)
	}
	fmt.Printf("inventory root=%s files=%d languages=%d frameworks=%d migration_roots=%d ci=%d source_sql_hints=%d evidence_exports=%d\n",
		inv.Root,
		inv.FilesScanned,
		len(inv.Languages),
		len(inv.Frameworks),
		len(inv.MigrationRoots)+len(inv.MigrationSystems),
		len(inv.CI),
		len(inv.SourceSQLHints),
		len(inv.EvidenceExports),
	)
	if *outPath != "" {
		fmt.Printf("  out=%s\n", *outPath)
	}
	for _, command := range inv.NextCommands {
		fmt.Printf("  next: %s # %s\n", command.Command, command.Reason)
	}
	return nil
}

func repoBaseline(args []string) error {
	fs := flag.NewFlagSet("repo baseline", flag.ContinueOnError)
	fs.SetOutput(ioDiscard{})
	inventoryPath := fs.String("inventory", "", "inventory directory or inventory.json")
	intakePath := fs.String("intake", "", "intake directory or summary.json")
	outPath := fs.String("out", "", "output directory")
	jsonOut := fs.Bool("json", false, "emit JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *inventoryPath == "" || *intakePath == "" {
		return errors.New("usage: patchline repo baseline --inventory inventory-dir --intake intake-dir [--out dir] [--json]")
	}
	inv, _, err := project.LoadInventory(*inventoryPath)
	if err != nil {
		return err
	}
	intakeReport, err := project.LoadIntakeReport(*intakePath)
	if err != nil {
		return err
	}
	report := project.Baseline(inv, inv.Facts, intakeReport)
	if *outPath != "" {
		if err := project.WriteBaseline(*outPath, report); err != nil {
			return err
		}
	}
	if *jsonOut {
		return writeJSON(os.Stdout, report)
	}
	fmt.Printf("baseline root=%s risks=%d links=%d grep_only=%d sql_only=%d hash=%s\n",
		report.InventoryRoot,
		report.Summary.RankedRisks,
		report.Summary.EvidenceLinks,
		report.Summary.GrepOnlyMatches,
		report.Summary.SQLOnlyRankedRisks,
		report.Hash,
	)
	if *outPath != "" {
		fmt.Printf("  out=%s\n", *outPath)
	}
	for _, command := range report.NativeChecks {
		fmt.Printf("  native: %s # %s\n", command.Command, command.Reason)
	}
	return nil
}

func repoPropose(args []string) error {
	fs := flag.NewFlagSet("repo propose", flag.ContinueOnError)
	fs.SetOutput(ioDiscard{})
	baselinePath := fs.String("from-report", "", "baseline directory or baseline.json")
	kind := fs.String("kind", "", "deprecated alias for --proposal-kind")
	proposalKind := fs.String("proposal-kind", "", "proposal kind: tests|guards|instrumentation|repair|explain|all")
	outPath := fs.String("out", "", "output directory")
	llmCommand := fs.String("llm-command", "", "optional user-provided generator command; prompt is passed on stdin")
	noLLM := fs.Bool("no-llm", false, "force deterministic template proposals and reject LLM generation")
	promptNoFacts := fs.Bool("prompt-without-facts", false, "ablation mode: send generator a prompt without repository facts")
	budget := fs.String("budget", "", "generated scope budget: files=N,lines=N,tokens=N,changes=N")
	budgetRisks := fs.Int("budget-risks", 3, "maximum ranked risks to include")
	jsonOut := fs.Bool("json", false, "emit JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *baselinePath == "" {
		return errors.New("usage: patchline repo propose --from-report baseline-dir --proposal-kind tests|guards|instrumentation|repair|all --out dir [--budget files=N,lines=N,tokens=N,changes=N] [--no-llm] [--llm-command cmd] [--prompt-without-facts] [--json]")
	}
	selectedKind, err := selectProposalKind(*kind, *proposalKind)
	if err != nil {
		return err
	}
	report, err := project.Propose(project.ProposalOptions{
		BaselinePath:  *baselinePath,
		Kind:          selectedKind,
		OutDir:        *outPath,
		LLMCommand:    *llmCommand,
		NoLLM:         *noLLM,
		PromptNoFacts: *promptNoFacts,
		Budget:        *budget,
		BudgetRisks:   *budgetRisks,
	})
	if err != nil {
		return err
	}
	if *outPath != "" {
		if err := project.WriteProposal(*outPath, report); err != nil {
			return err
		}
	}
	if *jsonOut {
		return writeJSON(os.Stdout, report)
	}
	fmt.Printf("proposal baseline=%s kind=%s generator=%s deterministic_only=%t files=%d output_hash=%s\n",
		report.BaselineHash,
		report.Kind,
		report.Generator,
		report.Deterministic,
		len(report.GeneratedFiles),
		report.OutputHash,
	)
	if *outPath != "" {
		fmt.Printf("  out=%s\n", *outPath)
	}
	return nil
}

func selectProposalKind(kind, proposalKind string) (string, error) {
	kind = strings.TrimSpace(kind)
	proposalKind = strings.TrimSpace(proposalKind)
	if kind != "" && proposalKind != "" && kind != proposalKind {
		return "", fmt.Errorf("--kind and --proposal-kind disagree: %q != %q", kind, proposalKind)
	}
	selected := proposalKind
	if selected == "" {
		selected = kind
	}
	if selected == "" {
		selected = "all"
	}
	return selected, nil
}

func repoCompare(args []string) error {
	fs := flag.NewFlagSet("repo compare", flag.ContinueOnError)
	fs.SetOutput(ioDiscard{})
	beforePath := fs.String("before", "", "baseline directory or baseline.json")
	afterPath := fs.String("after", "", "proposal directory or proposal.json")
	outPath := fs.String("out", "", "output directory")
	runNativeTests := fs.Bool("run-native-tests", false, "run safe allowlisted native test commands discovered during inventory")
	nativeTestTimeout := fs.Duration("native-test-timeout", 30*time.Second, "timeout for each native test command")
	jsonOut := fs.Bool("json", false, "emit JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *beforePath == "" || *afterPath == "" {
		return errors.New("usage: patchline repo compare --before baseline-dir --after proposal-dir [--out dir] [--run-native-tests] [--json]")
	}
	baseline, err := project.LoadBaseline(*beforePath)
	if err != nil {
		return err
	}
	proposal, err := project.LoadProposal(*afterPath)
	if err != nil {
		return err
	}
	report := project.CompareWithOptions(baseline, proposal, project.CompareOptions{
		RunNativeTests:    *runNativeTests,
		NativeTestTimeout: *nativeTestTimeout,
	})
	if *outPath != "" {
		if err := project.WriteCompare(*outPath, report); err != nil {
			return err
		}
	}
	if *jsonOut {
		return writeJSON(os.Stdout, report)
	}
	fmt.Printf("compare baseline=%s proposal=%s generated=%d covered=%d failed=%d hash=%s\n",
		report.BaselineHash,
		report.ProposalHash,
		report.Summary.GeneratedFiles,
		report.Summary.RisksWithCoverage,
		report.Summary.PatchlineChecksFailed,
		report.Hash,
	)
	if *outPath != "" {
		fmt.Printf("  out=%s\n", *outPath)
	}
	return nil
}

func repoProposalMinimize(args []string) error {
	fs := flag.NewFlagSet("repo proposal-minimize", flag.ContinueOnError)
	fs.SetOutput(ioDiscard{})
	beforePath := fs.String("before", "", "baseline directory or baseline.json")
	afterPath := fs.String("after", "", "proposal directory or proposal.json")
	outPath := fs.String("out", filepath.Join("results", "generated", "proposal-minimized"), "output directory")
	jsonOut := fs.Bool("json", false, "emit JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *beforePath == "" || *afterPath == "" {
		return errors.New("usage: patchline repo proposal-minimize --before baseline-dir --after proposal-dir [--out dir] [--json]")
	}
	baseline, err := project.LoadBaseline(*beforePath)
	if err != nil {
		return err
	}
	proposal, err := project.LoadProposal(*afterPath)
	if err != nil {
		return err
	}
	minimized := project.MinimizeGeneratedProposal(baseline, proposal)
	if *outPath != "" {
		if err := project.WriteProposal(*outPath, minimized); err != nil {
			return err
		}
	}
	if *jsonOut {
		return writeJSON(os.Stdout, minimized)
	}
	fmt.Printf("proposal minimize before=%d after=%d removed=%d output_hash=%s\n",
		minimized.Minimization.BeforeFiles,
		minimized.Minimization.AfterFiles,
		minimized.Minimization.RemovedFiles,
		minimized.OutputHash,
	)
	if *outPath != "" {
		fmt.Printf("  out=%s\n", *outPath)
	}
	return nil
}

type repoReplayReport struct {
	Version    string               `json:"version"`
	Analysis   string               `json:"analysis"`
	Artifacts  []repoReplayArtifact `json:"artifacts"`
	PatchApply repoReplayPatchApply `json:"patch_apply"`
	Hash       string               `json:"hash"`
	Markdown   string               `json:"markdown,omitempty"`
}

type repoReplayArtifact struct {
	Name  string `json:"name"`
	Path  string `json:"path"`
	Hash  string `json:"hash"`
	Bytes int    `json:"bytes"`
}

type repoReplayPatchApply struct {
	Status   string `json:"status"`
	DiffPath string `json:"diff_path,omitempty"`
	DiffHash string `json:"diff_hash,omitempty"`
	LogHash  string `json:"log_hash,omitempty"`
	Log      string `json:"log,omitempty"`
}

func repoReplay(args []string) error {
	fs := flag.NewFlagSet("repo replay", flag.ContinueOnError)
	fs.SetOutput(ioDiscard{})
	analysisPath := fs.String("analysis", "", "repo analyze output directory")
	outPath := fs.String("out", filepath.Join("results", "generated", "repo-replay"), "output directory")
	jsonOut := fs.Bool("json", false, "emit JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *analysisPath == "" {
		return errors.New("usage: patchline repo replay --analysis analysis-dir [--out dir] [--json]")
	}
	report, err := buildRepoReplayReport(*analysisPath, *outPath)
	if err != nil {
		return err
	}
	if *outPath != "" {
		if err := writeRepoReplayReport(*outPath, report); err != nil {
			return err
		}
	}
	if *jsonOut {
		return writeJSON(os.Stdout, report)
	}
	fmt.Printf("repo replay artifacts=%d patch_apply=%s hash=%s\n", len(report.Artifacts), report.PatchApply.Status, report.Hash)
	if *outPath != "" {
		fmt.Printf("  out=%s\n", *outPath)
	}
	return nil
}

func buildRepoReplayReport(analysisPath, outPath string) (repoReplayReport, error) {
	report := repoReplayReport{Version: "patchline.repo-replay/v1", Analysis: analysisPath}
	candidates := []struct {
		name string
		path string
	}{
		{"source", filepath.Join(analysisPath, "fetch", "source.json")},
		{"prompt_context", filepath.Join(analysisPath, "proposal", "prompt-context.json")},
		{"prompt", filepath.Join(analysisPath, "proposal", "prompt.txt")},
		{"generation_output", filepath.Join(analysisPath, "proposal", "proposal.json")},
		{"generated_patch", filepath.Join(analysisPath, "proposal", "proposal.patch")},
		{"compare_results", filepath.Join(analysisPath, "compare", "compare.json")},
	}
	for _, candidate := range candidates {
		artifact, ok, err := replayArtifact(candidate.name, candidate.path)
		if err != nil {
			return repoReplayReport{}, err
		}
		if ok {
			report.Artifacts = append(report.Artifacts, artifact)
		}
	}
	report.PatchApply = replayApplyPatch(analysisPath, outPath)
	if report.PatchApply.DiffPath != "" {
		if artifact, ok, err := replayArtifact("applied_diff", report.PatchApply.DiffPath); err != nil {
			return repoReplayReport{}, err
		} else if ok {
			report.Artifacts = append(report.Artifacts, artifact)
		}
	}
	report.Hash = repoReplayHash(report)
	report.Markdown = renderRepoReplayMarkdown(report)
	return report, nil
}

func replayArtifact(name, path string) (repoReplayArtifact, bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return repoReplayArtifact{}, false, nil
		}
		return repoReplayArtifact{}, false, err
	}
	return repoReplayArtifact{Name: name, Path: filepath.ToSlash(path), Hash: "sha256:" + canonical.Hash(data), Bytes: len(data)}, true, nil
}

func replayApplyPatch(analysisPath, outPath string) repoReplayPatchApply {
	patchPath := filepath.Join(analysisPath, "proposal", "proposal.patch")
	absPatchPath, err := filepath.Abs(patchPath)
	if err != nil {
		absPatchPath = patchPath
	}
	source, err := loadSource(filepath.Join(analysisPath, "fetch", "source.json"))
	if err != nil || source.ScannedRoot == "" || !fileExists(patchPath) {
		return repoReplayPatchApply{Status: "unavailable", Log: "source metadata or proposal patch unavailable"}
	}
	tmp, err := os.MkdirTemp("", "patchline-replay-*")
	if err != nil {
		return repoReplayPatchApply{Status: "failed", Log: err.Error(), LogHash: canonical.Hash(err.Error())}
	}
	defer os.RemoveAll(tmp)
	work := filepath.Join(tmp, "work")
	if err := copyReplayTree(source.ScannedRoot, work); err != nil {
		return repoReplayPatchApply{Status: "failed", Log: err.Error(), LogHash: canonical.Hash(err.Error())}
	}
	run := func(args ...string) (string, error) {
		cmd := exec.Command("git", args...)
		cmd.Dir = work
		out, err := cmd.CombinedOutput()
		return string(out), err
	}
	var logs []string
	for _, args := range [][]string{
		{"init", "--quiet"},
		{"add", "-A"},
		{"-c", "user.name=Patchline", "-c", "user.email=patchline@example.invalid", "commit", "--quiet", "--allow-empty", "-m", "replay baseline"},
		{"apply", absPatchPath},
		{"add", "-N", "--", "."},
	} {
		log, err := run(args...)
		logs = append(logs, "$ git "+strings.Join(args, " ")+"\n"+log)
		if err != nil {
			joined := strings.Join(logs, "\n")
			return repoReplayPatchApply{Status: "failed", Log: joined, LogHash: canonical.Hash(joined)}
		}
	}
	diff, err := run("diff", "--binary")
	logs = append(logs, "$ git diff --binary\n"+diff)
	if err != nil {
		joined := strings.Join(logs, "\n")
		return repoReplayPatchApply{Status: "failed", Log: joined, LogHash: canonical.Hash(joined)}
	}
	diffPath := filepath.Join(outPath, "applied.diff")
	if outPath != "" {
		_ = os.MkdirAll(outPath, 0o755)
		_ = os.WriteFile(diffPath, []byte(diff), 0o644)
	}
	return repoReplayPatchApply{Status: "applied", DiffPath: filepath.ToSlash(diffPath), DiffHash: "sha256:" + canonical.Hash(diff), LogHash: canonical.Hash(strings.Join(logs, "\n"))}
}

func copyReplayTree(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return os.MkdirAll(dst, 0o755)
		}
		if d.IsDir() && d.Name() == ".git" {
			return filepath.SkipDir
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		return os.WriteFile(target, data, info.Mode().Perm())
	})
}

func repoReplayHash(report repoReplayReport) string {
	copy := report
	copy.Hash = ""
	copy.Markdown = ""
	return canonical.Hash(copy)
}

func renderRepoReplayMarkdown(report repoReplayReport) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Patchline generated intervention replay\n\n")
	fmt.Fprintf(&b, "- analysis: `%s`\n", report.Analysis)
	fmt.Fprintf(&b, "- patch_apply: `%s`\n", report.PatchApply.Status)
	if report.PatchApply.DiffHash != "" {
		fmt.Fprintf(&b, "- applied_diff_hash: `%s`\n", report.PatchApply.DiffHash)
	}
	fmt.Fprintf(&b, "- hash: `%s`\n\n", report.Hash)
	fmt.Fprintf(&b, "## Replay artifacts\n\n| artifact | hash | bytes |\n| --- | --- | ---: |\n")
	for _, artifact := range report.Artifacts {
		fmt.Fprintf(&b, "| %s | `%s` | %d |\n", artifact.Name, artifact.Hash, artifact.Bytes)
	}
	return b.String()
}

func writeRepoReplayReport(outDir string, report repoReplayReport) error {
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}
	copy := report
	copy.Markdown = ""
	data, err := json.MarshalIndent(copy, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(outDir, "replay.json"), append(data, '\n'), 0o644); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(outDir, "replay.md"), []byte(report.Markdown), 0o644)
}

func repoSuppressions(args []string) error {
	fs := flag.NewFlagSet("repo suppressions", flag.ContinueOnError)
	fs.SetOutput(ioDiscard{})
	baselinePath := fs.String("baseline", "", "baseline directory or baseline.json")
	suppressionsPath := fs.String("suppressions", "", "suppression ledger JSON")
	outPath := fs.String("out", "", "output directory")
	jsonOut := fs.Bool("json", false, "emit JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *baselinePath == "" || *suppressionsPath == "" || fs.NArg() != 0 {
		return errors.New("usage: patchline repo suppressions --baseline baseline-dir --suppressions suppressions.json [--out dir] [--json]")
	}
	baseline, err := project.LoadBaseline(*baselinePath)
	if err != nil {
		return err
	}
	ledger, err := loadSuppressionLedger(*suppressionsPath)
	if err != nil {
		return err
	}
	report := evaluateSuppressions(baseline, ledger)
	if *outPath != "" {
		if err := writeSuppressionReport(*outPath, report); err != nil {
			return err
		}
	}
	if *jsonOut {
		return writeJSON(os.Stdout, report)
	}
	fmt.Printf("suppressions baseline=%s total=%d active=%d expired=%d stale=%d invalid=%d unmatched=%d hash=%s\n",
		report.BaselineHash,
		report.Summary.Total,
		report.Summary.Active,
		report.Summary.Expired,
		report.Summary.Stale,
		report.Summary.Invalid,
		report.Summary.Unmatched,
		report.Hash,
	)
	if *outPath != "" {
		fmt.Printf("  out=%s\n", *outPath)
	}
	return nil
}

func loadSuppressionLedger(path string) (suppressionLedger, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return suppressionLedger{}, err
	}
	var ledger suppressionLedger
	if err := json.Unmarshal(data, &ledger); err != nil {
		return suppressionLedger{}, err
	}
	if ledger.Version == "" {
		return suppressionLedger{}, errors.New("suppression ledger version is required")
	}
	return ledger, nil
}

func evaluateSuppressions(baseline project.BaselineReport, ledger suppressionLedger) suppressionReport {
	risks := map[string]project.BaselineRisk{}
	for _, risk := range baseline.Risks {
		if risk.StableID != "" {
			risks[risk.StableID] = risk
		}
	}
	report := suppressionReport{Version: "patchline.suppression-report/v1", BaselineHash: baseline.Hash}
	now := time.Now().UTC()
	for _, entry := range ledger.Suppressions {
		result := evaluateSuppression(entry, risks, now)
		report.Results = append(report.Results, result)
		report.Summary.Total++
		switch result.Status {
		case "active":
			report.Summary.Active++
		case "expired":
			report.Summary.Expired++
		case "stale":
			report.Summary.Stale++
		case "unmatched":
			report.Summary.Unmatched++
		default:
			report.Summary.Invalid++
		}
	}
	report.Hash = suppressionReportHash(report)
	report.Markdown = renderSuppressionMarkdown(report)
	return report
}

func evaluateSuppression(entry suppressionEntry, risks map[string]project.BaselineRisk, now time.Time) suppressionResult {
	result := suppressionResult{StableID: entry.StableID, Owner: entry.Owner, Expires: entry.Expires, ExpectedEvidenceHash: entry.EvidenceHash}
	if strings.TrimSpace(entry.StableID) == "" || strings.TrimSpace(entry.Owner) == "" || strings.TrimSpace(entry.Rationale) == "" || strings.TrimSpace(entry.Expires) == "" || strings.TrimSpace(entry.EvidenceHash) == "" {
		result.Status = "invalid"
		result.Reason = "stable_id, owner, rationale, expires, and evidence_hash are required"
		return result
	}
	expires, err := parseSuppressionExpiry(entry.Expires)
	if err != nil {
		result.Status = "invalid"
		result.Reason = "expires must be YYYY-MM-DD or RFC3339"
		return result
	}
	if !expires.After(now) {
		result.Status = "expired"
		result.Reason = "suppression expiry is in the past"
		return result
	}
	risk, ok := risks[entry.StableID]
	if !ok {
		result.Status = "unmatched"
		result.Reason = "stable_id no longer matches any ranked risk"
		return result
	}
	actual := suppressionEvidenceHash(risk)
	result.ActualEvidenceHash = actual
	if entry.EvidenceHash != actual {
		result.Status = "stale"
		result.Reason = "evidence_hash does not match the current ranked risk"
		return result
	}
	result.Status = "active"
	result.Reason = "suppression is current and matches a ranked risk"
	return result
}

func parseSuppressionExpiry(value string) (time.Time, error) {
	if t, err := time.Parse("2006-01-02", value); err == nil {
		return t, nil
	}
	return time.Parse(time.RFC3339, value)
}

func suppressionEvidenceHash(risk project.BaselineRisk) string {
	if risk.EvidenceHash != "" {
		return risk.EvidenceHash
	}
	return "sha256:" + canonical.Hash(strings.Join([]string{
		risk.StableID,
		strings.ToLower(strings.TrimSpace(risk.Table)),
		operationFamilyForSuppression(risk.Kind),
		risk.Severity,
	}, "\x00"))
}

func operationFamilyForSuppression(kind string) string {
	kind = strings.ToLower(strings.TrimSpace(kind))
	switch {
	case strings.Contains(kind, "delete"), strings.Contains(kind, "drop"), strings.Contains(kind, "truncate"):
		return "destructive"
	case strings.Contains(kind, "update"), strings.Contains(kind, "backfill"), strings.Contains(kind, "write"):
		return "write"
	case strings.Contains(kind, "schema"), strings.Contains(kind, "migration"):
		return "schema"
	case strings.Contains(kind, "sql"):
		return "sql"
	default:
		return kind
	}
}

func writeSuppressionReport(outDir string, report suppressionReport) error {
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}
	copy := report
	copy.Markdown = ""
	data, err := json.MarshalIndent(copy, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(outDir, "suppressions.json"), append(data, '\n'), 0o644); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(outDir, "suppressions.md"), []byte(report.Markdown), 0o644)
}

func suppressionReportHash(report suppressionReport) string {
	copy := report
	copy.Hash = ""
	copy.Markdown = ""
	return canonical.Hash(copy)
}

func renderSuppressionMarkdown(report suppressionReport) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Patchline suppressions\n\n")
	fmt.Fprintf(&b, "- baseline_hash: `%s`\n", report.BaselineHash)
	fmt.Fprintf(&b, "- hash: `%s`\n\n", report.Hash)
	fmt.Fprintf(&b, "## Summary\n\n| status | count |\n| --- | ---: |\n")
	fmt.Fprintf(&b, "| total | %d |\n", report.Summary.Total)
	fmt.Fprintf(&b, "| active | %d |\n", report.Summary.Active)
	fmt.Fprintf(&b, "| expired | %d |\n", report.Summary.Expired)
	fmt.Fprintf(&b, "| stale | %d |\n", report.Summary.Stale)
	fmt.Fprintf(&b, "| invalid | %d |\n", report.Summary.Invalid)
	fmt.Fprintf(&b, "| unmatched | %d |\n\n", report.Summary.Unmatched)
	fmt.Fprintf(&b, "## Results\n\n| stable id | status | owner | expires | reason |\n| --- | --- | --- | --- | --- |\n")
	for _, result := range report.Results {
		fmt.Fprintf(&b, "| `%s` | %s | %s | %s | %s |\n", result.StableID, result.Status, result.Owner, result.Expires, result.Reason)
	}
	return b.String()
}

func repoWhyNow(args []string) error {
	fs := flag.NewFlagSet("repo why-now", flag.ContinueOnError)
	fs.SetOutput(ioDiscard{})
	previousPath := fs.String("previous", "", "previous baseline directory or baseline.json")
	currentPath := fs.String("current", "", "current baseline directory or baseline.json")
	outPath := fs.String("out", "", "output directory")
	jsonOut := fs.Bool("json", false, "emit JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *previousPath == "" || *currentPath == "" || fs.NArg() != 0 {
		return errors.New("usage: patchline repo why-now --previous baseline-dir --current baseline-dir [--out dir] [--json]")
	}
	previous, err := project.LoadBaseline(*previousPath)
	if err != nil {
		return err
	}
	current, err := project.LoadBaseline(*currentPath)
	if err != nil {
		return err
	}
	report := buildWhyNowReport(previous, current)
	if *outPath != "" {
		if err := writeWhyNowReport(*outPath, report); err != nil {
			return err
		}
	}
	if *jsonOut {
		return writeJSON(os.Stdout, report)
	}
	fmt.Printf("why-now previous=%s current=%s new=%d resolved=%d persisting=%d hash=%s\n",
		report.PreviousHash,
		report.CurrentHash,
		report.Summary.NewRisks,
		report.Summary.ResolvedRisks,
		report.Summary.PersistingRisks,
		report.Hash,
	)
	if *outPath != "" {
		fmt.Printf("  out=%s\n", *outPath)
	}
	return nil
}

func buildWhyNowReport(previous, current project.BaselineReport) whyNowReport {
	prev := risksByStableID(previous.Risks)
	cur := risksByStableID(current.Risks)
	report := whyNowReport{
		Version:      "patchline.why-now/v1",
		PreviousHash: previous.Hash,
		CurrentHash:  current.Hash,
		Summary: whyNowSummary{
			PreviousRisks: len(prev),
			CurrentRisks:  len(cur),
		},
	}
	for stableID, risk := range cur {
		if _, ok := prev[stableID]; !ok {
			report.NewRisks = append(report.NewRisks, whyNowRiskFromBaseline(risk, "stable ID is present in current baseline but absent from previous baseline"))
		} else {
			report.Persisting = append(report.Persisting, whyNowRiskFromBaseline(risk, "stable ID is present in both baselines"))
		}
	}
	for stableID, risk := range prev {
		if _, ok := cur[stableID]; !ok {
			report.ResolvedRisks = append(report.ResolvedRisks, whyNowRiskFromBaseline(risk, "stable ID was present previously but is absent from current baseline"))
		}
	}
	sortWhyNowRisks(report.NewRisks)
	sortWhyNowRisks(report.ResolvedRisks)
	sortWhyNowRisks(report.Persisting)
	report.Summary.NewRisks = len(report.NewRisks)
	report.Summary.ResolvedRisks = len(report.ResolvedRisks)
	report.Summary.PersistingRisks = len(report.Persisting)
	report.Hash = whyNowHash(report)
	report.Markdown = renderWhyNowMarkdown(report)
	return report
}

func risksByStableID(risks []project.BaselineRisk) map[string]project.BaselineRisk {
	out := map[string]project.BaselineRisk{}
	for _, risk := range risks {
		key := risk.StableID
		if key == "" {
			key = risk.ID
		}
		if key != "" {
			out[key] = risk
		}
	}
	return out
}

func whyNowRiskFromBaseline(risk project.BaselineRisk, reason string) whyNowRisk {
	return whyNowRisk{
		StableID: risk.StableID,
		RiskID:   risk.ID,
		Path:     risk.Path,
		Kind:     risk.Kind,
		Table:    risk.Table,
		Severity: risk.Severity,
		Score:    risk.Score,
		Reason:   reason,
	}
}

func sortWhyNowRisks(risks []whyNowRisk) {
	sort.Slice(risks, func(i, j int) bool {
		if risks[i].Score != risks[j].Score {
			return risks[i].Score > risks[j].Score
		}
		return risks[i].StableID < risks[j].StableID
	})
}

func writeWhyNowReport(outDir string, report whyNowReport) error {
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}
	copy := report
	copy.Markdown = ""
	data, err := json.MarshalIndent(copy, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(outDir, "why-now.json"), append(data, '\n'), 0o644); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(outDir, "why-now.md"), []byte(report.Markdown), 0o644)
}

func whyNowHash(report whyNowReport) string {
	copy := report
	copy.Hash = ""
	copy.Markdown = ""
	return canonical.Hash(copy)
}

func renderWhyNowMarkdown(report whyNowReport) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Patchline why now\n\n")
	fmt.Fprintf(&b, "- previous_hash: `%s`\n", report.PreviousHash)
	fmt.Fprintf(&b, "- current_hash: `%s`\n", report.CurrentHash)
	fmt.Fprintf(&b, "- hash: `%s`\n\n", report.Hash)
	fmt.Fprintf(&b, "## Summary\n\n| area | count |\n| --- | ---: |\n")
	fmt.Fprintf(&b, "| previous risks | %d |\n", report.Summary.PreviousRisks)
	fmt.Fprintf(&b, "| current risks | %d |\n", report.Summary.CurrentRisks)
	fmt.Fprintf(&b, "| new risks | %d |\n", report.Summary.NewRisks)
	fmt.Fprintf(&b, "| resolved risks | %d |\n", report.Summary.ResolvedRisks)
	fmt.Fprintf(&b, "| persisting risks | %d |\n\n", report.Summary.PersistingRisks)
	if len(report.NewRisks) > 0 {
		fmt.Fprintf(&b, "## Newly introduced risks\n\n| stable id | score | severity | path | reason |\n| --- | ---: | --- | --- | --- |\n")
		limit := len(report.NewRisks)
		if limit > 20 {
			limit = 20
		}
		for _, risk := range report.NewRisks[:limit] {
			fmt.Fprintf(&b, "| `%s` | %d | %s | %s | %s |\n", risk.StableID, risk.Score, risk.Severity, risk.Path, risk.Reason)
		}
	}
	return b.String()
}

func repoPRComment(args []string) error {
	fs := flag.NewFlagSet("repo pr-comment", flag.ContinueOnError)
	fs.SetOutput(ioDiscard{})
	basePath := fs.String("base", "", "base branch baseline output directory or baseline.json")
	headPath := fs.String("head", "", "head branch baseline output directory or baseline.json")
	maxFindings := fs.Int("max-findings", 20, "maximum new or changed findings to render in the PR comment")
	outPath := fs.String("out", "", "output directory")
	jsonOut := fs.Bool("json", false, "emit JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *basePath == "" || *headPath == "" || fs.NArg() != 0 {
		return errors.New("usage: patchline repo pr-comment --base baseline-dir --head baseline-dir [--max-findings N] [--out dir] [--json]")
	}
	base, err := project.LoadBaseline(*basePath)
	if err != nil {
		return err
	}
	head, err := project.LoadBaseline(*headPath)
	if err != nil {
		return err
	}
	report := buildPRCommentReport(base, head, *maxFindings)
	if *outPath != "" {
		if err := writePRCommentReport(*outPath, report); err != nil {
			return err
		}
	}
	if *jsonOut {
		return writeJSON(os.Stdout, report)
	}
	fmt.Printf("pr-comment new=%d changed=%d rendered=%d hash=%s\n", report.Summary.NewFindings, report.Summary.ChangedFindings, report.Summary.RenderedFindings, report.Hash)
	if *outPath != "" {
		fmt.Printf("  out=%s\n", *outPath)
	}
	return nil
}

func buildPRCommentReport(base, head project.BaselineReport, maxFindings int) prCommentReport {
	if maxFindings <= 0 {
		maxFindings = 20
	}
	baseByKey := risksByStableID(base.Risks)
	headByKey := risksByStableID(head.Risks)
	report := prCommentReport{
		Version:  "patchline.pr-comment/v1",
		BaseHash: base.Hash,
		HeadHash: head.Hash,
		Summary: prCommentSummary{
			BaseRisks: len(baseByKey),
			HeadRisks: len(headByKey),
		},
	}
	for stableID, risk := range headByKey {
		previous, existed := baseByKey[stableID]
		if !existed {
			report.Findings = append(report.Findings, prCommentFindingFromRisk(risk, "new", project.BaselineRisk{}, "stable risk key is present in the PR baseline but absent from the base baseline"))
			continue
		}
		if prRiskSignature(previous) != prRiskSignature(risk) {
			report.Findings = append(report.Findings, prCommentFindingFromRisk(risk, "changed", previous, prRiskChangeReason(previous, risk)))
		} else {
			report.Summary.UnchangedRisks++
		}
	}
	sort.Slice(report.Findings, func(i, j int) bool {
		if report.Findings[i].Status != report.Findings[j].Status {
			return report.Findings[i].Status == "new"
		}
		if report.Findings[i].Score != report.Findings[j].Score {
			return report.Findings[i].Score > report.Findings[j].Score
		}
		return report.Findings[i].StableID < report.Findings[j].StableID
	})
	for _, finding := range report.Findings {
		if finding.Status == "new" {
			report.Summary.NewFindings++
		}
		if finding.Status == "changed" {
			report.Summary.ChangedFindings++
		}
	}
	if len(report.Findings) > maxFindings {
		report.Findings = append([]prCommentFinding(nil), report.Findings[:maxFindings]...)
		report.Truncated = true
	}
	report.Summary.RenderedFindings = len(report.Findings)
	report.PostCommand = `gh pr comment "$PR_NUMBER" --body-file pr-comment.md`
	report.Hash = prCommentHash(report)
	report.Markdown = renderPRCommentMarkdown(report)
	return report
}

func prCommentFindingFromRisk(risk project.BaselineRisk, status string, previous project.BaselineRisk, reason string) prCommentFinding {
	stableID := risk.StableID
	if stableID == "" {
		stableID = risk.ID
	}
	finding := prCommentFinding{
		Status:    status,
		StableID:  stableID,
		RiskID:    risk.ID,
		Path:      risk.Path,
		Kind:      risk.Kind,
		Table:     risk.Table,
		Severity:  risk.Severity,
		Score:     risk.Score,
		Reason:    reason,
		Rationale: risk.Rationale,
	}
	if previous.ID != "" {
		finding.PreviousSeverity = previous.Severity
		finding.PreviousScore = previous.Score
	}
	return finding
}

func prRiskSignature(risk project.BaselineRisk) string {
	return strings.Join([]string{
		risk.Path,
		risk.Kind,
		risk.Table,
		risk.Severity,
		fmt.Sprintf("%d", risk.Score),
		risk.Rationale,
	}, "\x00")
}

func prRiskChangeReason(previous, current project.BaselineRisk) string {
	var reasons []string
	if previous.Severity != current.Severity {
		reasons = append(reasons, "severity "+previous.Severity+" -> "+current.Severity)
	}
	if previous.Score != current.Score {
		reasons = append(reasons, fmt.Sprintf("score %d -> %d", previous.Score, current.Score))
	}
	if previous.Path != current.Path {
		reasons = append(reasons, "path changed")
	}
	if previous.Kind != current.Kind {
		reasons = append(reasons, "kind changed")
	}
	if previous.Table != current.Table {
		reasons = append(reasons, "table changed")
	}
	if previous.Rationale != current.Rationale {
		reasons = append(reasons, "rationale changed")
	}
	if len(reasons) == 0 {
		return "risk details changed"
	}
	return strings.Join(reasons, "; ")
}

func writePRCommentReport(outDir string, report prCommentReport) error {
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}
	copy := report
	copy.Markdown = ""
	data, err := json.MarshalIndent(copy, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(outDir, "pr-comment.json"), append(data, '\n'), 0o644); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(outDir, "pr-comment.md"), []byte(report.Markdown), 0o644)
}

func prCommentHash(report prCommentReport) string {
	copy := report
	copy.Hash = ""
	copy.Markdown = ""
	return canonical.Hash(copy)
}

func renderPRCommentMarkdown(report prCommentReport) string {
	var b strings.Builder
	fmt.Fprintf(&b, "## Patchline data-risk changes\n\n")
	fmt.Fprintf(&b, "Only new or changed data-risk findings are shown. Unchanged baseline risks are intentionally omitted.\n\n")
	fmt.Fprintf(&b, "| metric | count |\n| --- | ---: |\n")
	fmt.Fprintf(&b, "| base risks | %d |\n", report.Summary.BaseRisks)
	fmt.Fprintf(&b, "| head risks | %d |\n", report.Summary.HeadRisks)
	fmt.Fprintf(&b, "| new findings | %d |\n", report.Summary.NewFindings)
	fmt.Fprintf(&b, "| changed findings | %d |\n", report.Summary.ChangedFindings)
	fmt.Fprintf(&b, "| unchanged omitted | %d |\n\n", report.Summary.UnchangedRisks)
	if len(report.Findings) == 0 {
		fmt.Fprintf(&b, "No new or changed data-risk findings were detected.\n")
		return b.String()
	}
	fmt.Fprintf(&b, "| status | severity | score | path | kind | table | reason |\n| --- | --- | ---: | --- | --- | --- | --- |\n")
	for _, finding := range report.Findings {
		severity := finding.Severity
		if finding.PreviousSeverity != "" && finding.PreviousSeverity != finding.Severity {
			severity = finding.PreviousSeverity + " -> " + finding.Severity
		}
		fmt.Fprintf(&b, "| %s | %s | %d | `%s` | %s | %s | %s |\n", finding.Status, severity, finding.Score, finding.Path, finding.Kind, finding.Table, finding.Reason)
	}
	if report.Truncated {
		fmt.Fprintf(&b, "\n_Comment truncated to %d rendered findings; inspect the Patchline analysis bundle for the complete machine-readable report._\n", report.Summary.RenderedFindings)
	}
	return b.String()
}

func repoChanges(args []string) error {
	fs := flag.NewFlagSet("repo changes", flag.ContinueOnError)
	fs.SetOutput(ioDiscard{})
	previousPath := fs.String("previous", "", "previous repo analyze output directory")
	currentPath := fs.String("current", "", "current repo analyze output directory")
	outPath := fs.String("out", "", "output directory")
	jsonOut := fs.Bool("json", false, "emit JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *previousPath == "" || *currentPath == "" || fs.NArg() != 0 {
		return errors.New("usage: patchline repo changes --previous analysis-dir --current analysis-dir [--out dir] [--json]")
	}
	report, err := buildChangesReport(*previousPath, *currentPath)
	if err != nil {
		return err
	}
	if *outPath != "" {
		if err := writeChangesReport(*outPath, report); err != nil {
			return err
		}
	}
	if *jsonOut {
		return writeJSON(os.Stdout, report)
	}
	fmt.Printf("changes facts=%+d risks=%+d links=%+d generated=%+d check_failures=%+d hash=%s\n",
		report.Facts.Current-report.Facts.Previous,
		report.Risks.Current-report.Risks.Previous,
		report.Links.Current-report.Links.Previous,
		report.Generated.Current-report.Generated.Previous,
		report.Checks.FailureDelta,
		report.Hash,
	)
	if *outPath != "" {
		fmt.Printf("  out=%s\n", *outPath)
	}
	return nil
}

func repoHook(args []string) error {
	if len(args) == 0 {
		return errors.New("usage: patchline repo hook <pre-commit|pre-push> [--root repo] [--base ref] [--out dir] [--json]")
	}
	mode := args[0]
	if mode != "pre-commit" && mode != "pre-push" {
		return fmt.Errorf("repo hook mode must be pre-commit or pre-push, got %q", mode)
	}
	fs := flag.NewFlagSet("repo hook "+mode, flag.ContinueOnError)
	fs.SetOutput(ioDiscard{})
	root := fs.String("root", ".", "local git repository root")
	base := fs.String("base", "", "base ref for pre-push diffs")
	outPath := fs.String("out", "", "output directory")
	jsonOut := fs.Bool("json", false, "emit JSON")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return errors.New("usage: patchline repo hook <pre-commit|pre-push> [--root repo] [--base ref] [--out dir] [--json]")
	}
	report, err := buildRepoHookReport(mode, *root, *base, *outPath)
	if err != nil {
		return err
	}
	if *outPath != "" {
		if err := writeRepoHookReport(*outPath, report); err != nil {
			return err
		}
	}
	if *jsonOut {
		return writeJSON(os.Stdout, report)
	}
	fmt.Printf("repo hook mode=%s changed=%d scanned=%d risks=%d high=%d network=%t hash=%s\n",
		report.Mode,
		report.Summary.ChangedFiles,
		report.Summary.ScannedFiles,
		report.Summary.RankedRisks,
		report.Summary.HighRisks,
		report.Network,
		report.Hash,
	)
	if *outPath != "" {
		fmt.Printf("  out=%s\n", *outPath)
	}
	return nil
}

func buildRepoHookReport(mode, root, base, outPath string) (repoHookReport, error) {
	repoRoot, err := gitRepoRoot(root)
	if err != nil {
		return repoHookReport{}, err
	}
	changed, resolvedBase, err := repoHookChangedPaths(repoRoot, mode, base)
	if err != nil {
		return repoHookReport{}, err
	}
	requestedOut := outPath != ""
	if outPath == "" {
		outPath, err = os.MkdirTemp("", "patchline-repo-hook-*")
		if err != nil {
			return repoHookReport{}, err
		}
		defer os.RemoveAll(outPath)
	}
	scratch := filepath.Join(outPath, "changed-files")
	if err := os.RemoveAll(scratch); err != nil {
		return repoHookReport{}, err
	}
	if err := os.MkdirAll(scratch, 0o755); err != nil {
		return repoHookReport{}, err
	}
	report := repoHookReport{
		Version: "patchline.repo-hook/v1",
		Mode:    mode,
		Root:    filepath.ToSlash(repoRoot),
		Base:    resolvedBase,
		Network: false,
		Outputs: map[string]string{},
	}
	if requestedOut {
		report.Outputs["changed_files"] = filepath.ToSlash(scratch)
	}
	for _, rel := range changed {
		size, err := mirrorRepoHookFile(repoRoot, scratch, rel, mode)
		if err != nil {
			return repoHookReport{}, err
		}
		report.ChangedFiles = append(report.ChangedFiles, repoHookChangedFile{Path: rel, Bytes: size, Source: hookSource(mode)})
	}
	var inv project.Inventory
	var intakeReport intake.Report
	var baseline project.BaselineReport
	if len(report.ChangedFiles) > 0 {
		inv, err = project.InventoryPath(project.InventoryOptions{Path: scratch})
		if err != nil {
			return repoHookReport{}, err
		}
		intakeReport, err = intake.Run(context.Background(), intake.Options{Path: scratch})
		if err != nil {
			return repoHookReport{}, err
		}
		baseline = project.Baseline(inv, inv.Facts, intakeReport)
		for i := range baseline.Risks {
			remapBaselineRiskPath(&baseline.Risks[i], scratch)
		}
		if requestedOut {
			inventoryOut := filepath.Join(outPath, "inventory")
			intakeOut := filepath.Join(outPath, "intake")
			baselineOut := filepath.Join(outPath, "baseline")
			if err := project.WriteInventory(inventoryOut, inv); err != nil {
				return repoHookReport{}, err
			}
			if err := intake.WriteReport(intakeOut, intakeReport); err != nil {
				return repoHookReport{}, err
			}
			if err := project.WriteBaseline(baselineOut, baseline); err != nil {
				return repoHookReport{}, err
			}
			report.Outputs["inventory"] = filepath.ToSlash(inventoryOut)
			report.Outputs["intake"] = filepath.ToSlash(intakeOut)
			report.Outputs["baseline"] = filepath.ToSlash(baselineOut)
		}
	}
	report.Summary = repoHookSummary{
		ChangedFiles:      len(report.ChangedFiles),
		ScannedFiles:      inv.FilesScanned,
		Facts:             len(inv.Facts),
		RankedRisks:       len(baseline.Risks),
		HighRisks:         countHookRisksBySeverity(baseline.Risks, "high"),
		MediumRisks:       countHookRisksBySeverity(baseline.Risks, "medium"),
		Infrastructure:    baseline.Summary.InfraFindings,
		NetworkOperations: 0,
	}
	report.FindingDeltas = hookFindingDeltas(baseline.Risks)
	report.Hash = repoHookHash(report)
	report.Markdown = renderRepoHookMarkdown(report)
	return report, nil
}

func gitRepoRoot(root string) (string, error) {
	if strings.TrimSpace(root) == "" {
		root = "."
	}
	output, err := runGit(root, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", err
	}
	resolved := strings.TrimSpace(string(output))
	if resolved == "" {
		return "", fmt.Errorf("%s is not inside a git repository", root)
	}
	return filepath.Abs(resolved)
}

func repoHookChangedPaths(root, mode, base string) ([]string, string, error) {
	var output []byte
	var err error
	resolvedBase := strings.TrimSpace(base)
	switch mode {
	case "pre-commit":
		output, err = runGit(root, "diff", "--cached", "--name-only", "--diff-filter=ACMR", "-z")
	case "pre-push":
		if resolvedBase == "" {
			resolvedBase = defaultPrePushBase(root)
		}
		if resolvedBase == "" {
			return nil, "", fmt.Errorf("pre-push mode needs --base when no upstream or HEAD~1 exists")
		}
		output, err = runGit(root, "diff", "--name-only", "--diff-filter=ACMR", "-z", resolvedBase+"...HEAD")
		if err != nil {
			output, err = runGit(root, "diff", "--name-only", "--diff-filter=ACMR", "-z", resolvedBase, "HEAD")
		}
	default:
		return nil, "", fmt.Errorf("unsupported hook mode %q", mode)
	}
	if err != nil {
		return nil, "", err
	}
	return safeGitPaths(output), resolvedBase, nil
}

func defaultPrePushBase(root string) string {
	if _, err := runGit(root, "rev-parse", "--verify", "@{upstream}"); err == nil {
		return "@{upstream}"
	}
	if _, err := runGit(root, "rev-parse", "--verify", "HEAD~1"); err == nil {
		return "HEAD~1"
	}
	return ""
}

func runGit(root string, args ...string) ([]byte, error) {
	cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(output)))
	}
	return output, nil
}

func safeGitPaths(data []byte) []string {
	seen := map[string]bool{}
	var out []string
	for _, raw := range strings.Split(string(data), "\x00") {
		rel := filepath.ToSlash(strings.TrimSpace(raw))
		if rel == "" || filepath.IsAbs(rel) || strings.HasPrefix(rel, "../") || strings.Contains(rel, "/../") || rel == ".." {
			continue
		}
		if !seen[rel] {
			seen[rel] = true
			out = append(out, rel)
		}
	}
	sort.Strings(out)
	return out
}

func mirrorRepoHookFile(repoRoot, scratch, rel, mode string) (int, error) {
	dst := filepath.Join(scratch, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return 0, err
	}
	var data []byte
	var err error
	if mode == "pre-commit" {
		data, err = runGit(repoRoot, "show", ":"+rel)
	} else {
		src := filepath.Join(repoRoot, filepath.FromSlash(rel))
		info, statErr := os.Lstat(src)
		if statErr != nil {
			return 0, statErr
		}
		if info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return 0, fmt.Errorf("refusing to mirror non-regular changed path %s", rel)
		}
		data, err = os.ReadFile(src)
	}
	if err != nil {
		return 0, err
	}
	if err := os.WriteFile(dst, data, 0o644); err != nil {
		return 0, err
	}
	return len(data), nil
}

func hookSource(mode string) string {
	if mode == "pre-commit" {
		return "git-index"
	}
	return "working-tree"
}

func remapBaselineRiskPath(risk *project.BaselineRisk, scratch string) {
	prefix := filepath.ToSlash(scratch) + "/"
	risk.Path = strings.TrimPrefix(filepath.ToSlash(risk.Path), prefix)
	if risk.NextCommand != "" {
		risk.NextCommand = strings.ReplaceAll(risk.NextCommand, filepath.ToSlash(scratch)+"/", "")
	}
}

func countHookRisksBySeverity(risks []project.BaselineRisk, severity string) int {
	count := 0
	for _, risk := range risks {
		if risk.Severity == severity {
			count++
		}
	}
	return count
}

func hookFindingDeltas(risks []project.BaselineRisk) []repoHookFindingDelta {
	var out []repoHookFindingDelta
	for _, risk := range risks {
		out = append(out, repoHookFindingDelta{
			Status:      "changed-file",
			StableID:    risk.StableID,
			RiskID:      risk.ID,
			Path:        risk.Path,
			Kind:        risk.Kind,
			Severity:    risk.Severity,
			Score:       risk.Score,
			Table:       risk.Table,
			Rationale:   risk.Rationale,
			NextCommand: risk.NextCommand,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Score != out[j].Score {
			return out[i].Score > out[j].Score
		}
		return out[i].RiskID < out[j].RiskID
	})
	return out
}

func writeRepoHookReport(outDir string, report repoHookReport) error {
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}
	copy := report
	copy.Markdown = ""
	data, err := json.MarshalIndent(copy, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(outDir, "hook.json"), append(data, '\n'), 0o644); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(outDir, "hook.md"), []byte(report.Markdown), 0o644)
}

func repoHookHash(report repoHookReport) string {
	copy := report
	copy.Hash = ""
	copy.Markdown = ""
	return canonical.Hash(copy)
}

func renderRepoHookMarkdown(report repoHookReport) string {
	var b strings.Builder
	fmt.Fprintf(&b, "## Patchline %s hook delta\n\n", report.Mode)
	fmt.Fprintf(&b, "Local-only hook scan: no external repository downloads or network operations are used.\n\n")
	fmt.Fprintf(&b, "| metric | count |\n| --- | ---: |\n")
	fmt.Fprintf(&b, "| changed files | %d |\n", report.Summary.ChangedFiles)
	fmt.Fprintf(&b, "| scanned files | %d |\n", report.Summary.ScannedFiles)
	fmt.Fprintf(&b, "| ranked risks | %d |\n", report.Summary.RankedRisks)
	fmt.Fprintf(&b, "| high risks | %d |\n", report.Summary.HighRisks)
	fmt.Fprintf(&b, "| network operations | %d |\n\n", report.Summary.NetworkOperations)
	if len(report.FindingDeltas) == 0 {
		fmt.Fprintf(&b, "No data-risk findings were detected in changed files.\n")
		return b.String()
	}
	fmt.Fprintf(&b, "| severity | score | path | kind | table | rationale |\n| --- | ---: | --- | --- | --- | --- |\n")
	for _, finding := range report.FindingDeltas {
		fmt.Fprintf(&b, "| %s | %d | `%s` | %s | %s | %s |\n", finding.Severity, finding.Score, finding.Path, finding.Kind, finding.Table, finding.Rationale)
	}
	return b.String()
}

func repoOffline(args []string) error {
	fs := flag.NewFlagSet("repo offline", flag.ContinueOnError)
	fs.SetOutput(ioDiscard{})
	analysisPath := fs.String("analysis", "", "repo analyze output directory to validate offline")
	outPath := fs.String("out", "", "output directory")
	jsonOut := fs.Bool("json", false, "emit JSON")
	var adapterPaths multiFlag
	fs.Var(&adapterPaths, "adapter", "adapter result JSON to validate; repeatable")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *analysisPath == "" || fs.NArg() != 0 {
		return errors.New("usage: patchline repo offline --analysis analysis-dir [--adapter adapter-result.json]... [--out dir] [--json]")
	}
	report := buildRepoOfflineReport(*analysisPath, adapterPaths)
	if *outPath != "" {
		if err := writeRepoOfflineReport(*outPath, report); err != nil {
			return err
		}
	}
	if *jsonOut {
		if err := writeJSON(os.Stdout, report); err != nil {
			return err
		}
		if !report.OK {
			return codedError{code: 2, err: errors.New("offline validation failed")}
		}
		return nil
	}
	status := "failed"
	if report.OK {
		status = "passed"
	}
	fmt.Printf("offline validation %s analysis=%s caches=%d/%d reports=%d/%d adapters=%d/%d network=%t hash=%s\n",
		status,
		report.Analysis,
		report.Summary.CacheInputsValid,
		report.Summary.CacheInputs,
		report.Summary.ReportsValid,
		report.Summary.Reports,
		report.Summary.AdaptersValid,
		report.Summary.Adapters,
		report.Network,
		report.Hash,
	)
	if *outPath != "" {
		fmt.Printf("  out=%s\n", *outPath)
	}
	if !report.OK {
		return codedError{code: 2, err: errors.New("offline validation failed")}
	}
	return nil
}

type multiFlag []string

func (m *multiFlag) String() string {
	return strings.Join(*m, ",")
}

func (m *multiFlag) Set(value string) error {
	if strings.TrimSpace(value) != "" {
		*m = append(*m, value)
	}
	return nil
}

func buildRepoOfflineReport(analysisPath string, adapterPaths []string) repoOfflineReport {
	report := repoOfflineReport{
		Version:  "patchline.repo-offline/v1",
		Analysis: filepath.ToSlash(analysisPath),
		Network:  false,
		OK:       true,
	}
	addErr := func(format string, args ...any) {
		report.Errors = append(report.Errors, fmt.Sprintf(format, args...))
		report.OK = false
	}
	addWarn := func(format string, args ...any) {
		report.Warnings = append(report.Warnings, fmt.Sprintf(format, args...))
	}

	if info, err := os.Stat(analysisPath); err != nil || !info.IsDir() {
		addErr("analysis directory %s is not readable", analysisPath)
	} else {
		sourcePaths := offlineSourcePaths(analysisPath)
		if len(sourcePaths) == 0 {
			addWarn("no source.json found under %s", analysisPath)
		}
		for _, sourcePath := range sourcePaths {
			cache := validateOfflineSource(sourcePath)
			report.CacheInputs = append(report.CacheInputs, cache)
			if !cache.Valid {
				addErr("invalid cache input %s: %s", sourcePath, cache.Rationale)
			}
		}
		for _, artifact := range validateOfflineReports(analysisPath) {
			report.Reports = append(report.Reports, artifact)
			if !artifact.Valid {
				addErr("invalid %s report %s: %s", artifact.Kind, artifact.Path, artifact.Rationale)
			}
		}
	}
	for _, adapterPath := range adapterPaths {
		adapter := validateOfflineAdapter(adapterPath)
		report.Adapters = append(report.Adapters, adapter)
		if !adapter.Valid {
			addErr("invalid adapter result %s: %s", adapterPath, adapter.Rationale)
		}
	}
	if len(adapterPaths) == 0 {
		addWarn("no adapter result files supplied; cached adapter output validation skipped")
	}
	report.Summary = repoOfflineSummary{
		CacheInputs:        len(report.CacheInputs),
		CacheInputsValid:   countValidOfflineCaches(report.CacheInputs),
		Adapters:           len(report.Adapters),
		AdaptersValid:      countValidOfflineAdapters(report.Adapters),
		Reports:            len(report.Reports),
		ReportsValid:       countValidOfflineArtifacts(report.Reports),
		GeneratedArtifacts: countOfflineGeneratedArtifacts(report.Reports),
		NetworkOperations:  0,
		Errors:             len(report.Errors),
		Warnings:           len(report.Warnings),
	}
	report.Hash = repoOfflineHash(report)
	report.Markdown = renderRepoOfflineMarkdown(report)
	return report
}

func offlineSourcePaths(analysisPath string) []string {
	candidates := []string{
		filepath.Join(analysisPath, "fetch", "source.json"),
		filepath.Join(analysisPath, "analysis-bundle", "source.json"),
	}
	var paths []string
	seen := map[string]bool{}
	for _, candidate := range candidates {
		if fileExists(candidate) && !seen[candidate] {
			paths = append(paths, candidate)
			seen[candidate] = true
		}
	}
	return paths
}

func validateOfflineSource(sourcePath string) repoOfflineCache {
	cache := repoOfflineCache{SourcePath: filepath.ToSlash(sourcePath)}
	source, err := loadSource(sourcePath)
	if err != nil {
		cache.Rationale = err.Error()
		return cache
	}
	cache.Input = source.Input
	cache.Mode = source.Mode
	cache.CacheKey = source.CacheKey
	cache.CachePath = source.CachePath
	cache.ArchiveHash = source.ArchiveHash
	cache.ScannedRoot = source.ScannedRoot
	cache.ResolvedCommit = source.ResolvedCommit
	if strings.TrimSpace(source.CachePath) != "" {
		actual, err := hashFileSHA256(source.CachePath)
		cache.ActualHash = actual
		if err != nil {
			cache.Rationale = "cached archive is not readable: " + err.Error()
			return cache
		}
		if source.ArchiveHash != "" && actual != source.ArchiveHash {
			cache.Rationale = "cached archive hash does not match source metadata"
			return cache
		}
	}
	if strings.TrimSpace(source.ScannedRoot) != "" {
		if info, err := os.Stat(source.ScannedRoot); err != nil || !info.IsDir() {
			cache.Rationale = "scanned_root is not available locally"
			return cache
		}
	}
	switch {
	case source.Mode == "local":
		cache.Valid = true
		cache.Rationale = "local source is readable without network access"
	case source.CachePath != "" && source.ArchiveHash != "":
		cache.Valid = true
		cache.Rationale = "cached archive hash matches source metadata"
	default:
		cache.Rationale = "non-local source lacks cache path and archive hash"
	}
	return cache
}

func validateOfflineReports(analysisPath string) []repoOfflineArtifact {
	checks := []struct {
		kind string
		path string
		load func(string) error
	}{
		{"analyze", filepath.Join(analysisPath, "analyze.json"), validateGenericJSON},
		{"inventory", filepath.Join(analysisPath, "inventory"), func(path string) error {
			_, _, err := project.LoadInventory(path)
			return err
		}},
		{"intake", filepath.Join(analysisPath, "intake"), func(path string) error {
			_, err := project.LoadIntakeReport(path)
			return err
		}},
		{"baseline", filepath.Join(analysisPath, "baseline"), func(path string) error {
			_, err := project.LoadBaseline(path)
			return err
		}},
		{"proposal", filepath.Join(analysisPath, "proposal"), func(path string) error {
			_, err := project.LoadProposal(path)
			return err
		}},
		{"compare", filepath.Join(analysisPath, "compare", "compare.json"), func(path string) error {
			_, err := loadCompareReport(path)
			return err
		}},
		{"triage", filepath.Join(analysisPath, "triage", "triage.json"), validateGenericJSON},
	}
	var artifacts []repoOfflineArtifact
	for _, check := range checks {
		if _, err := os.Stat(check.path); err != nil {
			if check.kind == "compare" || check.kind == "triage" {
				continue
			}
			artifacts = append(artifacts, repoOfflineArtifact{Kind: check.kind, Path: filepath.ToSlash(check.path), Valid: false, Rationale: "required report artifact is missing"})
			continue
		}
		artifact := repoOfflineArtifact{Kind: check.kind, Path: filepath.ToSlash(check.path)}
		hash, err := hashPathSHA256(check.path)
		if err == nil {
			artifact.Hash = hash
		}
		if err := check.load(check.path); err != nil {
			artifact.Valid = false
			artifact.Rationale = err.Error()
		} else {
			artifact.Valid = true
			artifact.Rationale = "report loaded from local artifacts"
		}
		artifacts = append(artifacts, artifact)
	}
	return artifacts
}

func validateGenericJSON(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var value any
	return json.Unmarshal(data, &value)
}

func validateOfflineAdapter(path string) repoOfflineAdapter {
	adapter := repoOfflineAdapter{Path: filepath.ToSlash(path)}
	data, err := os.ReadFile(path)
	if err != nil {
		adapter.Rationale = err.Error()
		return adapter
	}
	var result evidence.AdaptResult
	if err := json.Unmarshal(data, &result); err != nil {
		adapter.Rationale = err.Error()
		return adapter
	}
	adapter.Adapter = result.Adapter
	adapter.EventCount = result.EventCount
	adapter.InputHash = result.InputHash
	if result.Version != evidence.AdapterVersion || !result.OK || result.EventCount != len(result.Events) || result.InputHash == "" {
		adapter.Rationale = "adapter result has invalid version, status, event count, or input hash"
		return adapter
	}
	adapter.Valid = true
	adapter.Rationale = "adapter result is self-contained and internally consistent"
	return adapter
}

func hashPathSHA256(path string) (string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return hashFileSHA256(path)
	}
	var hashes []string
	err = filepath.WalkDir(path, func(item string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return nil
		}
		hash, err := hashFileSHA256(item)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(path, item)
		hashes = append(hashes, filepath.ToSlash(rel)+"="+hash)
		return nil
	})
	if err != nil {
		return "", err
	}
	sort.Strings(hashes)
	return "sha256:" + canonical.Hash(strings.Join(hashes, "\n")), nil
}

func hashFileSHA256(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return "", err
	}
	return "sha256:" + hex.EncodeToString(hasher.Sum(nil)), nil
}

func countValidOfflineCaches(caches []repoOfflineCache) int {
	count := 0
	for _, cache := range caches {
		if cache.Valid {
			count++
		}
	}
	return count
}

func countValidOfflineAdapters(adapters []repoOfflineAdapter) int {
	count := 0
	for _, adapter := range adapters {
		if adapter.Valid {
			count++
		}
	}
	return count
}

func countValidOfflineArtifacts(artifacts []repoOfflineArtifact) int {
	count := 0
	for _, artifact := range artifacts {
		if artifact.Valid {
			count++
		}
	}
	return count
}

func countOfflineGeneratedArtifacts(artifacts []repoOfflineArtifact) int {
	for _, artifact := range artifacts {
		if artifact.Kind == "proposal" && artifact.Valid {
			if proposal, err := project.LoadProposal(artifact.Path); err == nil {
				return len(proposal.GeneratedFiles)
			}
		}
	}
	return 0
}

func writeRepoOfflineReport(outDir string, report repoOfflineReport) error {
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}
	copy := report
	copy.Markdown = ""
	data, err := json.MarshalIndent(copy, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(outDir, "offline.json"), append(data, '\n'), 0o644); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(outDir, "offline.md"), []byte(report.Markdown), 0o644)
}

func repoOfflineHash(report repoOfflineReport) string {
	copy := report
	copy.Hash = ""
	copy.Markdown = ""
	return canonical.Hash(copy)
}

func renderRepoOfflineMarkdown(report repoOfflineReport) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Patchline offline validation\n\n")
	fmt.Fprintf(&b, "- analysis: `%s`\n", report.Analysis)
	fmt.Fprintf(&b, "- ok: `%t`\n", report.OK)
	fmt.Fprintf(&b, "- network: `%t`\n", report.Network)
	fmt.Fprintf(&b, "- hash: `%s`\n\n", report.Hash)
	fmt.Fprintf(&b, "## Summary\n\n| area | count |\n| --- | ---: |\n")
	fmt.Fprintf(&b, "| cache inputs | %d |\n", report.Summary.CacheInputs)
	fmt.Fprintf(&b, "| cache inputs valid | %d |\n", report.Summary.CacheInputsValid)
	fmt.Fprintf(&b, "| reports | %d |\n", report.Summary.Reports)
	fmt.Fprintf(&b, "| reports valid | %d |\n", report.Summary.ReportsValid)
	fmt.Fprintf(&b, "| adapters | %d |\n", report.Summary.Adapters)
	fmt.Fprintf(&b, "| adapters valid | %d |\n", report.Summary.AdaptersValid)
	fmt.Fprintf(&b, "| network operations | %d |\n", report.Summary.NetworkOperations)
	fmt.Fprintf(&b, "| errors | %d |\n", report.Summary.Errors)
	fmt.Fprintf(&b, "| warnings | %d |\n\n", report.Summary.Warnings)
	if len(report.CacheInputs) > 0 {
		fmt.Fprintf(&b, "## Cache inputs\n\n| valid | mode | input | cache path | rationale |\n| --- | --- | --- | --- | --- |\n")
		for _, cache := range report.CacheInputs {
			fmt.Fprintf(&b, "| %t | %s | %s | `%s` | %s |\n", cache.Valid, cache.Mode, cache.Input, cache.CachePath, cache.Rationale)
		}
	}
	if len(report.Reports) > 0 {
		fmt.Fprintf(&b, "\n## Reports\n\n| valid | kind | path | rationale |\n| --- | --- | --- | --- |\n")
		for _, artifact := range report.Reports {
			fmt.Fprintf(&b, "| %t | %s | `%s` | %s |\n", artifact.Valid, artifact.Kind, artifact.Path, artifact.Rationale)
		}
	}
	if len(report.Adapters) > 0 {
		fmt.Fprintf(&b, "\n## Adapters\n\n| valid | adapter | events | path | rationale |\n| --- | --- | ---: | --- | --- |\n")
		for _, adapter := range report.Adapters {
			fmt.Fprintf(&b, "| %t | %s | %d | `%s` | %s |\n", adapter.Valid, adapter.Adapter, adapter.EventCount, adapter.Path, adapter.Rationale)
		}
	}
	return b.String()
}

func buildChangesReport(previousPath, currentPath string) (changesReport, error) {
	prev, err := loadAnalysisSnapshot(previousPath)
	if err != nil {
		return changesReport{}, err
	}
	cur, err := loadAnalysisSnapshot(currentPath)
	if err != nil {
		return changesReport{}, err
	}
	report := changesReport{
		Version:   "patchline.repo-changes/v1",
		Previous:  filepath.ToSlash(previousPath),
		Current:   filepath.ToSlash(currentPath),
		Facts:     compareStringSets(prev.Facts, cur.Facts),
		Risks:     compareStringSets(prev.Risks, cur.Risks),
		Links:     compareStringSets(prev.Links, cur.Links),
		Generated: compareStringSets(prev.Generated, cur.Generated),
		Checks: checksDelta{
			PreviousFailures: prev.CheckFailures,
			CurrentFailures:  cur.CheckFailures,
			PreviousPassed:   prev.CheckPassed,
			CurrentPassed:    cur.CheckPassed,
			FailureDelta:     cur.CheckFailures - prev.CheckFailures,
			PassDelta:        cur.CheckPassed - prev.CheckPassed,
		},
	}
	report.Summary = changesSummary{
		PreviousFacts:     len(prev.Facts),
		CurrentFacts:      len(cur.Facts),
		PreviousRisks:     len(prev.Risks),
		CurrentRisks:      len(cur.Risks),
		PreviousGenerated: len(prev.Generated),
		CurrentGenerated:  len(cur.Generated),
		PreviousFailures:  prev.CheckFailures,
		CurrentFailures:   cur.CheckFailures,
	}
	for _, changed := range []bool{
		report.Facts.Added > 0 || report.Facts.Removed > 0,
		report.Risks.Added > 0 || report.Risks.Removed > 0,
		report.Links.Added > 0 || report.Links.Removed > 0,
		report.Generated.Added > 0 || report.Generated.Removed > 0,
		report.Checks.FailureDelta != 0 || report.Checks.PassDelta != 0,
	} {
		if changed {
			report.Summary.ChangedDimensions++
		}
	}
	report.Hash = changesHash(report)
	report.Markdown = renderChangesMarkdown(report)
	return report, nil
}

type analysisSnapshot struct {
	Facts         []string
	Risks         []string
	Links         []string
	Generated     []string
	CheckFailures int
	CheckPassed   int
}

func loadAnalysisSnapshot(path string) (analysisSnapshot, error) {
	var snap analysisSnapshot
	facts, err := project.LoadFacts(filepath.Join(path, "inventory", "facts.jsonl"))
	if err != nil {
		return snap, err
	}
	for _, fact := range facts {
		snap.Facts = append(snap.Facts, fact.ID)
	}
	baseline, err := project.LoadBaseline(filepath.Join(path, "baseline"))
	if err != nil {
		return snap, err
	}
	for _, risk := range baseline.Risks {
		id := risk.StableID
		if id == "" {
			id = risk.ID
		}
		snap.Risks = append(snap.Risks, id)
	}
	for _, link := range baseline.EvidenceLinks {
		snap.Links = append(snap.Links, link.RiskID+"\x00"+link.FactID)
	}
	generated, err := loadGeneratedFileKeys(filepath.Join(path, "proposal", "proposal.json"))
	if err != nil {
		return snap, err
	}
	snap.Generated = append(snap.Generated, generated...)
	if compare, err := loadCompareReport(filepath.Join(path, "compare", "compare.json")); err == nil {
		snap.CheckFailures = compare.Summary.PatchlineChecksFailed + compare.Summary.NativeChecksFailed
		snap.CheckPassed = compare.Summary.PatchlineChecksPassed + compare.Summary.NativeChecksPassed
	}
	sort.Strings(snap.Facts)
	sort.Strings(snap.Risks)
	sort.Strings(snap.Links)
	sort.Strings(snap.Generated)
	return snap, nil
}

func loadGeneratedFileKeys(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var report project.ProposalReport
	if err := json.Unmarshal(data, &report); err != nil {
		return nil, err
	}
	keys := make([]string, 0, len(report.GeneratedFiles))
	for _, generated := range report.GeneratedFiles {
		keys = append(keys, generated.Path+"\x00"+generated.ContentHash)
	}
	sort.Strings(keys)
	return keys, nil
}

func compareStringSets(previous, current []string) changesDelta {
	prev := map[string]bool{}
	cur := map[string]bool{}
	for _, value := range previous {
		prev[value] = true
	}
	for _, value := range current {
		cur[value] = true
	}
	var added, removed []string
	for value := range cur {
		if !prev[value] {
			added = append(added, value)
		}
	}
	for value := range prev {
		if !cur[value] {
			removed = append(removed, value)
		}
	}
	sort.Strings(added)
	sort.Strings(removed)
	examples := append([]string{}, firstNStrings(added, 3)...)
	examples = append(examples, firstNStrings(removed, 3)...)
	return changesDelta{Previous: len(prev), Current: len(cur), Added: len(added), Removed: len(removed), Examples: examples}
}

func firstNStrings(values []string, n int) []string {
	if len(values) < n {
		n = len(values)
	}
	return append([]string(nil), values[:n]...)
}

func writeChangesReport(outDir string, report changesReport) error {
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}
	copy := report
	copy.Markdown = ""
	data, err := json.MarshalIndent(copy, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(outDir, "changes.json"), append(data, '\n'), 0o644); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(outDir, "changes.md"), []byte(report.Markdown), 0o644)
}

func changesHash(report changesReport) string {
	copy := report
	copy.Hash = ""
	copy.Markdown = ""
	return canonical.Hash(copy)
}

func renderChangesMarkdown(report changesReport) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Patchline repo changes\n\n")
	fmt.Fprintf(&b, "- previous: `%s`\n", report.Previous)
	fmt.Fprintf(&b, "- current: `%s`\n", report.Current)
	fmt.Fprintf(&b, "- hash: `%s`\n\n", report.Hash)
	fmt.Fprintf(&b, "## Summary\n\n| dimension | previous | current | added | removed |\n| --- | ---: | ---: | ---: | ---: |\n")
	fmt.Fprintf(&b, "| facts | %d | %d | %d | %d |\n", report.Facts.Previous, report.Facts.Current, report.Facts.Added, report.Facts.Removed)
	fmt.Fprintf(&b, "| ranked risks | %d | %d | %d | %d |\n", report.Risks.Previous, report.Risks.Current, report.Risks.Added, report.Risks.Removed)
	fmt.Fprintf(&b, "| links | %d | %d | %d | %d |\n", report.Links.Previous, report.Links.Current, report.Links.Added, report.Links.Removed)
	fmt.Fprintf(&b, "| generated artifacts | %d | %d | %d | %d |\n", report.Generated.Previous, report.Generated.Current, report.Generated.Added, report.Generated.Removed)
	fmt.Fprintf(&b, "\n## Deterministic checks\n\n| area | previous | current | delta |\n| --- | ---: | ---: | ---: |\n")
	fmt.Fprintf(&b, "| failures | %d | %d | %+d |\n", report.Checks.PreviousFailures, report.Checks.CurrentFailures, report.Checks.FailureDelta)
	fmt.Fprintf(&b, "| passed | %d | %d | %+d |\n", report.Checks.PreviousPassed, report.Checks.CurrentPassed, report.Checks.PassDelta)
	return b.String()
}

func repoNotifySummary(args []string) error {
	fs := flag.NewFlagSet("repo notify-summary", flag.ContinueOnError)
	fs.SetOutput(ioDiscard{})
	analysisPath := fs.String("analysis", "", "repo analyze output directory")
	bundleLink := fs.String("bundle-link", "", "analysis bundle URL or path")
	outPath := fs.String("out", "", "output directory")
	jsonOut := fs.Bool("json", false, "emit JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *analysisPath == "" || fs.NArg() != 0 {
		return errors.New("usage: patchline repo notify-summary --analysis analysis-dir [--bundle-link url] [--out dir] [--json]")
	}
	report, err := buildNotifySummaryReport(*analysisPath, *bundleLink)
	if err != nil {
		return err
	}
	if *outPath != "" {
		if err := writeNotifySummaryReport(*outPath, report); err != nil {
			return err
		}
	}
	if *jsonOut {
		return writeJSON(os.Stdout, report)
	}
	fmt.Println(report.SlackText)
	if *outPath != "" {
		fmt.Printf("  out=%s\n", *outPath)
	}
	return nil
}

func buildNotifySummaryReport(analysisPath, bundleLink string) (notifySummaryReport, error) {
	baseline, err := project.LoadBaseline(filepath.Join(analysisPath, "baseline"))
	if err != nil {
		return notifySummaryReport{}, err
	}
	var analyze repoAnalyzeReport
	analyzeData, analyzeErr := os.ReadFile(filepath.Join(analysisPath, "analyze.json"))
	if analyzeErr == nil {
		if err := json.Unmarshal(analyzeData, &analyze); err != nil {
			return notifySummaryReport{}, err
		}
	} else if !errors.Is(analyzeErr, os.ErrNotExist) {
		return notifySummaryReport{}, analyzeErr
	}
	var proposal project.ProposalReport
	if data, err := os.ReadFile(filepath.Join(analysisPath, "proposal", "proposal.json")); err == nil {
		if err := json.Unmarshal(data, &proposal); err != nil {
			return notifySummaryReport{}, err
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return notifySummaryReport{}, err
	}
	compare, _ := loadCompareReport(filepath.Join(analysisPath, "compare", "compare.json"))
	if bundleLink == "" {
		if analyze.Outputs != nil && analyze.Outputs["analysis_bundle"] != "" {
			bundleLink = analyze.Outputs["analysis_bundle"]
		} else {
			bundleLink = filepath.ToSlash(filepath.Join(analysisPath, "analysis-bundle"))
		}
	}
	var top project.BaselineRisk
	if len(baseline.Risks) > 0 {
		top = baseline.Risks[0]
	}
	report := notifySummaryReport{
		Version:             "patchline.notify-summary/v1",
		Analysis:            filepath.ToSlash(analysisPath),
		BundleLink:          bundleLink,
		TopMaintainerAction: notifyTopAction(top, proposal, compare),
		TopRisk: notifySummaryRisk{
			ID:       top.ID,
			StableID: top.StableID,
			Path:     top.Path,
			Table:    top.Table,
			Severity: top.Severity,
			Score:    top.Score,
			Reason:   top.Rationale,
		},
		ReproductionCommand: notifyReproductionCommand(top, analyze),
	}
	report.SlackText = renderNotifySlackText(report)
	report.GitHubMarkdown = renderNotifyGitHubMarkdown(report)
	report.Hash = notifySummaryHash(report)
	report.Markdown = renderNotifySummaryMarkdown(report)
	return report, nil
}

func notifyTopAction(risk project.BaselineRisk, proposal project.ProposalReport, compare project.CompareReport) string {
	if compare.Summary.PatchlineChecksFailed+compare.Summary.NativeChecksFailed > 0 {
		return "Fix deterministic check failures before using generated interventions"
	}
	if risk.ID == "" {
		return "Run repo analysis and inspect the generated analysis bundle"
	}
	if len(proposal.GeneratedFiles) > 0 {
		return "Review generated intervention for the top-ranked data-change risk"
	}
	if risk.Table != "" {
		return "Review the top-ranked risk on table " + risk.Table
	}
	return "Review the top-ranked data-change risk"
}

func notifyReproductionCommand(risk project.BaselineRisk, analyze repoAnalyzeReport) string {
	if strings.TrimSpace(risk.NextCommand) != "" {
		return risk.NextCommand
	}
	if analyze.Input != "" {
		args := []string{"go run ./cmd/patchline repo analyze"}
		if analyze.Source.Mode == "github" && analyze.Source.Repo != "" {
			repo := analyze.Source.Repo
			if analyze.Source.Owner != "" && !strings.Contains(repo, "/") {
				repo = analyze.Source.Owner + "/" + repo
			}
			args = append(args, "--github "+shellArg(repo))
			if analyze.Source.Ref != "" {
				args = append(args, "--ref "+shellArg(analyze.Source.Ref))
			}
		} else {
			args = append(args, shellArg(analyze.Input))
		}
		if analyze.Subpath != "" {
			args = append(args, "--subpath "+shellArg(analyze.Subpath))
		}
		args = append(args, "--stages inventory,baseline,propose,compare", "--no-llm", "--out results/generated/reproduce-analysis")
		return strings.Join(args, " ")
	}
	if risk.Path != "" {
		return "go run ./cmd/patchline analyze-migration " + shellArg(risk.Path) + " --json"
	}
	return "go run ./cmd/patchline repo analyze <repo-or-path> --stages inventory,baseline,propose,compare --no-llm"
}

func renderNotifySlackText(report notifySummaryReport) string {
	risk := report.TopRisk.ID
	if risk == "" {
		risk = "no ranked risk"
	}
	return fmt.Sprintf("Patchline: %s | top risk: %s (%s %d) | reproduce: `%s` | bundle: %s",
		report.TopMaintainerAction,
		risk,
		report.TopRisk.Severity,
		report.TopRisk.Score,
		report.ReproductionCommand,
		report.BundleLink,
	)
}

func renderNotifyGitHubMarkdown(report notifySummaryReport) string {
	var b strings.Builder
	fmt.Fprintf(&b, "### Patchline summary\n\n")
	fmt.Fprintf(&b, "**Top action:** %s\n\n", report.TopMaintainerAction)
	fmt.Fprintf(&b, "**Top risk:** `%s`", report.TopRisk.ID)
	if report.TopRisk.StableID != "" {
		fmt.Fprintf(&b, " / `%s`", report.TopRisk.StableID)
	}
	fmt.Fprintf(&b, " (%s, score %d)", report.TopRisk.Severity, report.TopRisk.Score)
	if report.TopRisk.Path != "" {
		fmt.Fprintf(&b, " in `%s`", report.TopRisk.Path)
	}
	fmt.Fprintf(&b, "\n\n**Reproduce:** `%s`\n\n", report.ReproductionCommand)
	fmt.Fprintf(&b, "**Bundle:** %s\n", report.BundleLink)
	return b.String()
}

func writeNotifySummaryReport(outDir string, report notifySummaryReport) error {
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}
	copy := report
	copy.Markdown = ""
	data, err := json.MarshalIndent(copy, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(outDir, "notify-summary.json"), append(data, '\n'), 0o644); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(outDir, "notify-summary.md"), []byte(report.Markdown), 0o644)
}

func notifySummaryHash(report notifySummaryReport) string {
	copy := report
	copy.Hash = ""
	copy.Markdown = ""
	return canonical.Hash(copy)
}

func renderNotifySummaryMarkdown(report notifySummaryReport) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Patchline notification summary\n\n")
	fmt.Fprintf(&b, "- action: %s\n", report.TopMaintainerAction)
	fmt.Fprintf(&b, "- top_risk: `%s`\n", report.TopRisk.ID)
	fmt.Fprintf(&b, "- reproduce: `%s`\n", report.ReproductionCommand)
	fmt.Fprintf(&b, "- bundle: %s\n", report.BundleLink)
	fmt.Fprintf(&b, "- hash: `%s`\n\n", report.Hash)
	fmt.Fprintf(&b, "## Slack\n\n%s\n\n", report.SlackText)
	fmt.Fprintf(&b, "## GitHub\n\n%s", report.GitHubMarkdown)
	return b.String()
}

func repoMinimize(args []string) error {
	fs := flag.NewFlagSet("repo minimize", flag.ContinueOnError)
	fs.SetOutput(ioDiscard{})
	analysisPath := fs.String("analysis", "", "repo analyze output directory")
	outPath := fs.String("out", filepath.Join("results", "generated", "corpus-minimized"), "output directory")
	jsonOut := fs.Bool("json", false, "emit JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *analysisPath == "" || fs.NArg() != 0 {
		return errors.New("usage: patchline repo minimize --analysis analysis-dir [--out dir] [--json]")
	}
	report, err := buildCorpusMinimizerReport(*analysisPath, *outPath)
	if err != nil {
		return err
	}
	if err := writeCorpusMinimizerReport(*outPath, report); err != nil {
		return err
	}
	if *jsonOut {
		return writeJSON(os.Stdout, report)
	}
	fmt.Printf("corpus minimize risks=%d files=%d subpaths=%d copied=%d hash=%s\n",
		report.Summary.Risks,
		report.Summary.UniqueSourceFiles,
		len(report.ExtractedSubpaths),
		report.Summary.CopiedFiles,
		report.Hash,
	)
	fmt.Printf("  out=%s\n", *outPath)
	return nil
}

func buildCorpusMinimizerReport(analysisPath, outPath string) (corpusMinimizerReport, error) {
	baseline, err := project.LoadBaseline(filepath.Join(analysisPath, "baseline"))
	if err != nil {
		return corpusMinimizerReport{}, err
	}
	source, err := loadSource(filepath.Join(analysisPath, "fetch", "source.json"))
	if err != nil {
		source = project.Source{Input: analysisPath, ScannedRoot: filepath.ToSlash(filepath.Join(analysisPath, "fetch"))}
	}
	generated, err := loadGeneratedFilesMetadata(filepath.Join(analysisPath, "proposal", "proposal.json"))
	if err != nil {
		return corpusMinimizerReport{}, err
	}
	generatedByRisk := map[string][]project.GeneratedFile{}
	for _, file := range generated {
		for _, riskID := range file.RiskIDs {
			generatedByRisk[riskID] = append(generatedByRisk[riskID], file)
		}
	}
	linksByRisk := map[string][]project.EvidenceLink{}
	for _, link := range baseline.EvidenceLinks {
		linksByRisk[link.RiskID] = append(linksByRisk[link.RiskID], link)
	}
	uniqueFiles := map[string]bool{}
	uniqueSubpaths := map[string]bool{}
	report := corpusMinimizerReport{
		Version:  "patchline.corpus-minimizer/v1",
		Analysis: filepath.ToSlash(analysisPath),
		Source:   source,
	}
	for _, risk := range baseline.Risks {
		sourcePaths := minimizerSourcePaths(risk, linksByRisk[risk.ID])
		for _, path := range sourcePaths {
			uniqueFiles[path] = true
		}
		publicSubpath := minimizerPublicSubpath(source.Subpath, sourcePaths)
		uniqueSubpaths[publicSubpath] = true
		entry := corpusMinimizerEntry{
			RiskID:           risk.ID,
			StableID:         risk.StableID,
			Severity:         risk.Severity,
			Score:            risk.Score,
			PublicSubpath:    publicSubpath,
			SourcePaths:      sourcePaths,
			EvidenceLinks:    linksByRisk[risk.ID],
			GeneratedFiles:   generatedByRisk[risk.ID],
			PreservationNote: "copy these source paths and rerun deterministic analysis to preserve the ranked finding, evidence links, and generated intervention context",
		}
		report.Summary.EvidenceLinks += len(entry.EvidenceLinks)
		report.Summary.GeneratedFiles += len(entry.GeneratedFiles)
		report.Entries = append(report.Entries, entry)
	}
	report.Summary.Risks = len(baseline.Risks)
	report.Summary.Entries = len(report.Entries)
	report.Summary.UniqueSourceFiles = len(uniqueFiles)
	report.ExtractedSubpaths = boolKeys(uniqueSubpaths)
	copied, err := copyMinimizerFiles(source.ScannedRoot, filepath.Join(outPath, "minimized-source"), boolKeys(uniqueFiles))
	if err != nil {
		return corpusMinimizerReport{}, err
	}
	report.Summary.CopiedFiles = copied
	report.Hash = corpusMinimizerHash(report)
	report.Markdown = renderCorpusMinimizerMarkdown(report)
	return report, nil
}

func loadGeneratedFilesMetadata(path string) ([]project.GeneratedFile, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var report project.ProposalReport
	if err := json.Unmarshal(data, &report); err != nil {
		return nil, err
	}
	return report.GeneratedFiles, nil
}

func minimizerSourcePaths(risk project.BaselineRisk, links []project.EvidenceLink) []string {
	seen := map[string]bool{}
	add := func(path string) {
		path = strings.Trim(filepath.ToSlash(path), "/")
		if path == "" || strings.Contains(path, "..") {
			return
		}
		seen[path] = true
	}
	add(risk.Path)
	for _, link := range links {
		add(link.Path)
	}
	return boolKeys(seen)
}

func minimizerPublicSubpath(base string, paths []string) string {
	dir := commonPathDir(paths)
	base = strings.Trim(filepath.ToSlash(base), "/")
	if dir == "." || dir == "" {
		if base == "" {
			return "."
		}
		return base
	}
	if base == "" {
		return dir
	}
	return filepath.ToSlash(filepath.Join(base, dir))
}

func commonPathDir(paths []string) string {
	if len(paths) == 0 {
		return "."
	}
	parts := strings.Split(strings.Trim(filepath.ToSlash(filepath.Dir(paths[0])), "/"), "/")
	if len(parts) == 1 && parts[0] == "." {
		parts = nil
	}
	for _, path := range paths[1:] {
		current := strings.Split(strings.Trim(filepath.ToSlash(filepath.Dir(path)), "/"), "/")
		if len(current) == 1 && current[0] == "." {
			current = nil
		}
		limit := len(parts)
		if len(current) < limit {
			limit = len(current)
		}
		i := 0
		for i < limit && parts[i] == current[i] {
			i++
		}
		parts = parts[:i]
	}
	if len(parts) == 0 {
		return "."
	}
	return strings.Join(parts, "/")
}

func copyMinimizerFiles(root, outDir string, paths []string) (int, error) {
	copied := 0
	for _, rel := range paths {
		src := filepath.Join(filepath.FromSlash(root), filepath.FromSlash(rel))
		info, err := os.Stat(src)
		if err != nil {
			return copied, err
		}
		if info.IsDir() {
			return copied, fmt.Errorf("minimizer source path %q is a directory", rel)
		}
		dst := filepath.Join(outDir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return copied, err
		}
		data, err := os.ReadFile(src)
		if err != nil {
			return copied, err
		}
		if err := os.WriteFile(dst, data, 0o644); err != nil {
			return copied, err
		}
		copied++
	}
	return copied, nil
}

func writeCorpusMinimizerReport(outDir string, report corpusMinimizerReport) error {
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}
	copy := report
	copy.Markdown = ""
	data, err := json.MarshalIndent(copy, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(outDir, "minimizer.json"), append(data, '\n'), 0o644); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(outDir, "minimizer.md"), []byte(report.Markdown), 0o644)
}

func corpusMinimizerHash(report corpusMinimizerReport) string {
	copy := report
	copy.Hash = ""
	copy.Markdown = ""
	return canonical.Hash(copy)
}

func renderCorpusMinimizerMarkdown(report corpusMinimizerReport) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Patchline corpus minimizer\n\n")
	fmt.Fprintf(&b, "- analysis: `%s`\n", report.Analysis)
	fmt.Fprintf(&b, "- source: `%s`\n", report.Source.Input)
	fmt.Fprintf(&b, "- hash: `%s`\n\n", report.Hash)
	fmt.Fprintf(&b, "## Summary\n\n| area | count |\n| --- | ---: |\n")
	fmt.Fprintf(&b, "| risks | %d |\n", report.Summary.Risks)
	fmt.Fprintf(&b, "| unique source files | %d |\n", report.Summary.UniqueSourceFiles)
	fmt.Fprintf(&b, "| evidence links | %d |\n", report.Summary.EvidenceLinks)
	fmt.Fprintf(&b, "| generated files | %d |\n", report.Summary.GeneratedFiles)
	fmt.Fprintf(&b, "| copied files | %d |\n\n", report.Summary.CopiedFiles)
	fmt.Fprintf(&b, "## Minimal public subpaths\n\n| finding | subpath | files | generated |\n| --- | --- | ---: | ---: |\n")
	for _, entry := range report.Entries {
		id := entry.StableID
		if id == "" {
			id = entry.RiskID
		}
		fmt.Fprintf(&b, "| `%s` | `%s` | %d | %d |\n", id, entry.PublicSubpath, len(entry.SourcePaths), len(entry.GeneratedFiles))
	}
	return b.String()
}

func repoRecurrence(args []string) error {
	fs := flag.NewFlagSet("repo recurrence", flag.ContinueOnError)
	fs.SetOutput(ioDiscard{})
	analysesValue := fs.String("analyses", "", "comma-separated repo analyze output directories")
	outPath := fs.String("out", filepath.Join("results", "generated", "repo-recurrence"), "output directory")
	jsonOut := fs.Bool("json", false, "emit JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *analysesValue == "" || fs.NArg() != 0 {
		return errors.New("usage: patchline repo recurrence --analyses analysis-dir[,analysis-dir...] [--out dir] [--json]")
	}
	analyses := splitNonEmpty(*analysesValue, ",")
	report, err := buildRecurrenceReport(analyses)
	if err != nil {
		return err
	}
	if err := writeRecurrenceReport(*outPath, report); err != nil {
		return err
	}
	if *jsonOut {
		return writeJSON(os.Stdout, report)
	}
	fmt.Printf("repo recurrence analyses=%d repeated=%d signatures=%d hash=%s\n", report.Summary.Analyses, report.Summary.Repeated, report.Summary.Signatures, report.Hash)
	fmt.Printf("  out=%s\n", *outPath)
	return nil
}

func buildRecurrenceReport(analyses []string) (recurrenceReport, error) {
	clusters := map[string]*recurrenceCluster{}
	totalRisks := 0
	projectSet := map[string]bool{}
	for _, analysis := range analyses {
		baseline, err := project.LoadBaseline(filepath.Join(analysis, "baseline"))
		if err != nil {
			return recurrenceReport{}, err
		}
		projectName := recurrenceProjectName(analysis)
		if analyzeData, err := os.ReadFile(filepath.Join(analysis, "analyze.json")); err == nil {
			var analyze repoAnalyzeReport
			if err := json.Unmarshal(analyzeData, &analyze); err != nil {
				return recurrenceReport{}, err
			}
			if analyze.Input != "" {
				projectName = analyze.Input
			}
		} else if !errors.Is(err, os.ErrNotExist) {
			return recurrenceReport{}, err
		}
		projectSet[projectName] = true
		for _, risk := range baseline.Risks {
			totalRisks++
			signature, factorNames := recurrenceSignature(risk)
			cluster := clusters[signature]
			if cluster == nil {
				cluster = &recurrenceCluster{Signature: signature, RiskKind: risk.Kind, Severity: risk.Severity, FactorNames: factorNames}
				clusters[signature] = cluster
			}
			cluster.Occurrences = append(cluster.Occurrences, recurrenceOccurrence{
				Analysis: filepath.ToSlash(analysis),
				Project:  projectName,
				RiskID:   risk.ID,
				StableID: risk.StableID,
				Severity: risk.Severity,
				Score:    risk.Score,
			})
		}
	}
	var repeated []recurrenceCluster
	for _, cluster := range clusters {
		projects := map[string]bool{}
		for _, occurrence := range cluster.Occurrences {
			projects[occurrence.Project] = true
		}
		cluster.OccurrenceCount = len(cluster.Occurrences)
		cluster.ProjectCount = len(projects)
		sort.Slice(cluster.Occurrences, func(i, j int) bool {
			if cluster.Occurrences[i].Project != cluster.Occurrences[j].Project {
				return cluster.Occurrences[i].Project < cluster.Occurrences[j].Project
			}
			return cluster.Occurrences[i].StableID < cluster.Occurrences[j].StableID
		})
		if cluster.ProjectCount >= 2 {
			repeated = append(repeated, *cluster)
		}
	}
	sort.Slice(repeated, func(i, j int) bool {
		if repeated[i].ProjectCount != repeated[j].ProjectCount {
			return repeated[i].ProjectCount > repeated[j].ProjectCount
		}
		if repeated[i].OccurrenceCount != repeated[j].OccurrenceCount {
			return repeated[i].OccurrenceCount > repeated[j].OccurrenceCount
		}
		return repeated[i].Signature < repeated[j].Signature
	})
	report := recurrenceReport{
		Version:     "patchline.repo-recurrence/v1",
		Analyses:    sortedStrings(analyses),
		Recurrences: repeated,
		Summary: recurrenceSummary{
			Analyses:          len(analyses),
			Risks:             totalRisks,
			Signatures:        len(clusters),
			Repeated:          len(repeated),
			RedactedFields:    3,
			UnrelatedProjects: len(projectSet),
		},
	}
	report.Hash = recurrenceHash(report)
	report.Markdown = renderRecurrenceMarkdown(report)
	return report, nil
}

func recurrenceProjectName(analysis string) string {
	base := filepath.Base(filepath.Clean(analysis))
	if base == "." || base == string(filepath.Separator) || base == "" {
		return filepath.ToSlash(analysis)
	}
	return base
}

func recurrenceSignature(risk project.BaselineRisk) (string, []string) {
	factors := make([]string, 0, len(risk.Factors))
	for _, factor := range risk.Factors {
		factors = append(factors, factor.Name)
	}
	sort.Strings(factors)
	material := strings.Join([]string{risk.Kind, risk.Severity, strings.Join(factors, ",")}, "\x00")
	return "recurrence:" + canonical.Hash(material)[:16], factors
}

func writeRecurrenceReport(outDir string, report recurrenceReport) error {
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}
	copy := report
	copy.Markdown = ""
	data, err := json.MarshalIndent(copy, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(outDir, "recurrence.json"), append(data, '\n'), 0o644); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(outDir, "recurrence.md"), []byte(report.Markdown), 0o644)
}

func recurrenceHash(report recurrenceReport) string {
	copy := report
	copy.Hash = ""
	copy.Markdown = ""
	return canonical.Hash(copy)
}

func renderRecurrenceMarkdown(report recurrenceReport) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Patchline cross-repo recurrence\n\n")
	fmt.Fprintf(&b, "- analyses: `%d`\n", report.Summary.Analyses)
	fmt.Fprintf(&b, "- repeated signatures: `%d`\n", report.Summary.Repeated)
	fmt.Fprintf(&b, "- redacted fields: paths, SQL text, table identifiers\n")
	fmt.Fprintf(&b, "- hash: `%s`\n\n", report.Hash)
	fmt.Fprintf(&b, "## Repeated failure-mode signatures\n\n| signature | kind | severity | projects | occurrences | factors |\n| --- | --- | --- | ---: | ---: | --- |\n")
	for _, cluster := range report.Recurrences {
		fmt.Fprintf(&b, "| `%s` | %s | %s | %d | %d | %s |\n", cluster.Signature, cluster.RiskKind, cluster.Severity, cluster.ProjectCount, cluster.OccurrenceCount, strings.Join(cluster.FactorNames, ", "))
	}
	return b.String()
}

type ioDiscard struct{}

func (ioDiscard) Write(p []byte) (int, error) { return len(p), nil }

func onePositionalWithFlags(args []string, boolFlags map[string]bool) (string, []string, error) {
	var positional string
	var flagArgs []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if strings.HasPrefix(arg, "--") {
			flagArgs = append(flagArgs, arg)
			if strings.Contains(arg, "=") || boolFlags[arg] {
				continue
			}
			if i+1 >= len(args) || strings.HasPrefix(args[i+1], "--") {
				return "", nil, fmt.Errorf("flag %s requires a value", arg)
			}
			i++
			flagArgs = append(flagArgs, args[i])
			continue
		}
		if positional != "" {
			return "", nil, fmt.Errorf("unexpected extra positional argument %q", arg)
		}
		positional = arg
	}
	return positional, flagArgs, nil
}

func currentIntake(args []string) error {
	githubRepo, _ := flagValue(args, "--github")
	ref, _ := flagValue(args, "--ref")
	subpath, _ := flagValue(args, "--subpath")
	outPath, _ := flagValue(args, "--out")
	downloadDir, _ := flagValue(args, "--download-dir")
	positionals := positionalArgs(args)
	path := ""
	if len(positionals) > 0 {
		path = positionals[0]
	}
	if githubRepo == "" && path == "" {
		return errors.New("usage: patchline intake <path> [--out dir] [--json] or patchline intake --github owner/repo [--ref ref] [--subpath path] [--out dir] [--json]")
	}
	report, err := intake.Run(context.Background(), intake.Options{
		Path:        path,
		GitHub:      githubRepo,
		Ref:         ref,
		Subpath:     subpath,
		DownloadDir: downloadDir,
	})
	if err != nil {
		return err
	}
	if outPath != "" {
		if err := intake.WriteReport(outPath, report); err != nil {
			return err
		}
	}
	if hasFlag(args, "--json") {
		return writeJSON(os.Stdout, report)
	}
	fmt.Printf("intake source=%s files=%d sql_files=%d loose_sql=%d high_risk=%d evidence_files=%d generic_signals=%d repairs=%d problems=%d causes=%d repair_candidates=%d links=%d hash=%s\n",
		report.Source.Input,
		report.Summary.FilesScanned,
		report.Summary.SQLFiles,
		report.Summary.LooseSQLSnippets,
		report.Summary.HighRiskSQLStatements,
		report.Summary.EvidenceFiles,
		report.Summary.GenericEvidenceSignals,
		report.Summary.RepairManifests,
		report.Summary.ProblemCandidates,
		report.Summary.CauseCandidates,
		report.Summary.RepairCandidates,
		report.Summary.LinkedCandidates,
		report.Hash,
	)
	if outPath != "" {
		fmt.Printf("  out=%s\n", outPath)
	}
	for _, suggestion := range report.Suggestions {
		fmt.Printf("  next: %s # %s\n", suggestion.Command, suggestion.Reason)
	}
	return nil
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
		case "repair-outcomes":
			return writeJSON(os.Stdout, report.HistoricalQueries.RepairOutcomeHistory)
		case "semantic-regressions":
			return writeJSON(os.Stdout, report.HistoricalQueries.SemanticRegressions)
		default:
			return fmt.Errorf("unknown archive query %q", query)
		}
	}
	switch query {
	case "all":
		printBroadUpdateQuery(report.HistoricalQueries.BroadUpdateMigrations)
		printDamagedReportQuery(report.HistoricalQueries.DamagedDerivedReports)
		printMissingRollbackQuery(report.HistoricalQueries.RepairsLackingRollback)
		printRepairOutcomes(report.HistoricalQueries.RepairOutcomeHistory)
		printSemanticRegressions(report.HistoricalQueries.SemanticRegressions)
	case "broad-updates":
		printBroadUpdateQuery(report.HistoricalQueries.BroadUpdateMigrations)
	case "damaged-reports":
		printDamagedReportQuery(report.HistoricalQueries.DamagedDerivedReports)
	case "missing-rollback":
		printMissingRollbackQuery(report.HistoricalQueries.RepairsLackingRollback)
	case "repair-outcomes":
		printRepairOutcomes(report.HistoricalQueries.RepairOutcomeHistory)
	case "semantic-regressions":
		printSemanticRegressions(report.HistoricalQueries.SemanticRegressions)
	default:
		return fmt.Errorf("unknown archive query %q", query)
	}
	fmt.Printf("archive query hash=%s\n", report.HistoricalQueries.Hash)
	return nil
}

func repairOutcomes(specPath string, jsonOut bool) error {
	report, err := buildArchiveReport(specPath)
	if err != nil {
		return err
	}
	if jsonOut {
		return writeJSON(os.Stdout, report.RepairOutcomes)
	}
	printRepairOutcomes(report.RepairOutcomes)
	return nil
}

func semanticRegressions(specPath string, jsonOut bool) error {
	report, err := buildArchiveReport(specPath)
	if err != nil {
		return err
	}
	if jsonOut {
		return writeJSON(os.Stdout, report.SemanticRegressions)
	}
	printSemanticRegressions(report.SemanticRegressions)
	return nil
}

func historicalFailures(specPath string, jsonOut bool) error {
	spec, err := historical.ReadSpec(specPath)
	if err != nil {
		return err
	}
	report, err := historical.Run(spec, filepath.Dir(specPath))
	if err != nil {
		return err
	}
	if jsonOut {
		if err := writeJSON(os.Stdout, report); err != nil {
			return err
		}
	} else {
		status := "passed"
		if !report.OK {
			status = "failed"
		}
		fmt.Printf("historical failure suite %s cases=%d hash=%s\n", status, len(report.Cases), report.Hash)
		for _, c := range report.Cases {
			caseStatus := "passed"
			if !c.OK {
				caseStatus = "failed"
			}
			fmt.Printf("  %s %s signals=%d hash=%s\n", caseStatus, c.ID, len(c.Signals), c.Hash)
			for _, expectation := range c.ExpectedSignals {
				marker := "missing"
				if expectation.Present {
					marker = "present"
				}
				fmt.Printf("    %s %s\n", marker, expectation.ID)
			}
		}
	}
	if !report.OK {
		return codedError{code: 2, err: errors.New("historical failure suite expectations failed")}
	}
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

func printRepairOutcomes(results []archive.RepairOutcome) {
	fmt.Printf("repair_outcomes count=%d\n", len(results))
	for _, result := range results {
		recurrences := "-"
		if len(result.LaterRecurrences) > 0 {
			recurrences = strings.Join(result.LaterRecurrences, ",")
		}
		fmt.Printf("  %s verification=%s rollback=%t dry_run=%s applied_sql=%s later_recurrences=%s\n",
			result.IncidentID,
			result.VerificationResult,
			result.RollbackAvailable,
			shortHash(result.DryRunHash),
			shortHash(result.AppliedSQLHash),
			recurrences,
		)
	}
}

func printSemanticRegressions(results []archive.SemanticRegression) {
	fmt.Printf("semantic_regressions count=%d\n", len(results))
	for _, result := range results {
		anchor := semanticRegressionAnchor(result)
		fmt.Printf("  %s resembles %s relation=%s severity=%s anchor=%s invariant=%q\n",
			result.IncidentID,
			result.PriorIncidentID,
			result.Relation,
			result.Severity,
			shortRegressionAnchor(anchor),
			result.LearnedInvariant,
		)
	}
}

func shortRegressionAnchor(anchor string) string {
	if strings.Contains(anchor, ":") {
		return anchor
	}
	return shortHash(anchor)
}

func semanticRegressionAnchor(result archive.SemanticRegression) string {
	if result.Table != "" {
		return result.Table
	}
	if result.ShapeHash != "" {
		return result.ShapeHash
	}
	for _, evidence := range result.Evidence {
		if strings.HasPrefix(evidence, "derived_report:") || strings.HasPrefix(evidence, "table:") || strings.HasPrefix(evidence, "shape_hash:") {
			return evidence
		}
	}
	return "unknown"
}

func shortHash(hash string) string {
	if len(hash) <= 12 {
		return hash
	}
	return hash[:12]
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
	store := demo.BillingStore()
	if input.Store != "" {
		store, err = readStore(resolvePath(baseDir, input.Store))
		if err != nil {
			return archive.Entry{}, err
		}
	}
	dryRun, err := replay.DryRun(manifest, graph, store)
	if err != nil {
		return archive.Entry{}, err
	}
	lint := repair.Lint(manifest)
	sqlHash := ""
	verificationReason := "repair-manifest-lint:" + archiveRepairLintReason(lint)
	if lint.OK {
		sqlPlan, err := repair.GenerateSQL(manifest)
		if err != nil {
			return archive.Entry{}, err
		}
		sqlHash = sqlPlan.Hash
		verificationReason = ""
	}
	effectSummary := abstractEffectSummary(manifest, dryRun)
	policyEvaluation, err := buildPolicyEvaluation(policyPath, repairPath, migrationPath, graph, store)
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
	rollbackOK := manifest.Rollback.Strategy == "snapshot" && manifest.Rollback.SnapshotRequired
	verificationResult := "cannot_prove"
	verificationHash := ""
	if lint.OK && sqlHash != "" && rollbackOK {
		verificationResult = repairVerificationResult(dryRun, policyEvaluation.OK, benchmarkResult.OK)
		if verificationResult == "verified" {
			verificationHash = canonical.Hash(struct {
				DryRunHash    string `json:"dry_run_hash"`
				PolicyHash    string `json:"policy_hash"`
				PolicyOK      bool   `json:"policy_ok"`
				BenchmarkHash string `json:"benchmark_hash"`
				BenchmarkOK   bool   `json:"benchmark_ok"`
				RollbackOK    bool   `json:"rollback_ok"`
			}{dryRun.Hash(), policyEvaluation.PolicyHash, policyEvaluation.OK, benchmarkResult.SuiteHash, benchmarkResult.OK, true})
		} else {
			verificationReason = verificationResult
		}
	} else if verificationReason == "" {
		verificationReason = "rollback-not-available"
	}

	entry := archive.Entry{
		ID:                       input.ID,
		EvidencePath:             input.Evidence,
		MigrationPath:            input.Migration,
		RepairPath:               input.Repair,
		ReplayStorePath:          input.Store,
		PolicyPath:               input.Policy,
		BenchmarkPath:            input.Benchmark,
		EvidenceHash:             evidenceResult.InputHash,
		ShapeHash:                provenance.ShapeHash(graph),
		MigrationHash:            migrationReport.Summary.ReportHash,
		MigrationTables:          migrationReport.Summary.Tables,
		MigrationMaxRisk:         maxMigrationRisk(migrationReport),
		MigrationBroadUpdates:    archiveBroadUpdates(migrationReport),
		RepairHash:               canonical.Hash(manifest),
		RepairEffect:             string(effectSummary.Join),
		RepairDryRunHash:         dryRun.Hash(),
		RepairAppliedSQLHash:     sqlHash,
		RepairRollbackAvailable:  rollbackOK,
		RepairVerificationResult: verificationResult,
		RepairVerificationHash:   verificationHash,
		RepairVerificationReason: verificationReason,
		PolicyAllowed:            policyEvaluation.OK,
		PolicyFailures:           collectPolicyFailures(policyEvaluation),
		PolicyHash:               policyEvaluation.PolicyHash,
		BenchmarkOK:              benchmarkResult.OK,
		BenchmarkHash:            benchmarkResult.SuiteHash,
		DamagedEntities:          len(evidenceResult.DamagedEntities),
		DamagedEntityIDs:         sortedStrings(evidenceResult.DamagedEntities),
		DerivedReports:           countEntitiesByKind(graph, provenance.KindReport),
		DerivedReportIDs:         derivedReportsFromDamaged(graph, evidenceResult.DamagedEntities),
		ProofBundleReady:         dryRun.Hash() != "" && policyEvaluation.PolicyHash != "" && benchmarkResult.SuiteHash != "" && verificationResult == "verified",
	}
	return entry, nil
}

func archiveRepairLintReason(lint repair.LintResult) string {
	if lint.OK {
		return "ok"
	}
	var codes []string
	for _, finding := range lint.Findings {
		if finding.Level == "error" {
			codes = append(codes, finding.Code)
		}
	}
	if len(codes) == 0 {
		return "not-ok"
	}
	sort.Strings(codes)
	return strings.Join(codes, ",")
}

func repairVerificationResult(dryRun replay.Report, policyOK, benchmarkOK bool) string {
	if dryRun.Hash() == "" {
		return "failed:no-dry-run-hash"
	}
	if !policyOK {
		return "failed:policy"
	}
	if !benchmarkOK {
		return "failed:benchmark"
	}
	return "verified"
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
		policyEval, err := buildPolicyEvaluation(opts.Policy, opts.Repair, opts.Migration, demo.Graph(), demo.BillingStore())
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
			fmt.Sprintf("repair_outcomes=%d", len(archiveReport.RepairOutcomes)),
			fmt.Sprintf("semantic_regressions=%d", len(archiveReport.SemanticRegressions)),
		},
		Hashes: map[string]string{"archive_index_hash": archiveReport.Hash},
		Claims: []semantics.Claim{{
			Ref:      "archive.incident_index",
			Status:   checkedStatus(len(archiveReport.Incidents) > 0),
			Reason:   "historical incidents were deterministically bucketed by evidence shape, migration table/risk, repair effect, policy decision, benchmark decision, repair outcome, and semantic regression",
			Evidence: archiveReport.Hash,
		}},
		Metadata: map[string]interface{}{
			"by_shape":              archiveReport.ByShape,
			"by_migration_table":    archiveReport.ByMigrationTable,
			"by_repair_effect":      archiveReport.ByRepairEffect,
			"by_policy_decision":    archiveReport.ByPolicyDecision,
			"by_benchmark_decision": archiveReport.ByBenchmarkDecision,
			"repair_outcomes":       archiveReport.RepairOutcomes,
			"semantic_regressions":  archiveReport.SemanticRegressions,
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

type findingExplainReport struct {
	Version            string                       `json:"version"`
	FindingID          string                       `json:"finding_id"`
	Analysis           string                       `json:"analysis"`
	Risk               project.BaselineRisk         `json:"risk"`
	Evidence           []project.EvidenceLink       `json:"evidence"`
	Facts              []project.Fact               `json:"facts"`
	RankingFactors     []project.ScoreFactor        `json:"ranking_factors"`
	RankingExplanation project.RankingExplanation   `json:"ranking_explanation,omitempty"`
	Alternatives       []findingExplainAlternative  `json:"alternatives_considered,omitempty"`
	ProofHoles         []string                     `json:"proof_holes,omitempty"`
	PolicyChecks       []project.PolicyCheck        `json:"policy_checks,omitempty"`
	SymbolicChecks     []project.SymbolicCheck      `json:"symbolic_checks,omitempty"`
	RepairProofs       []project.RepairProofSummary `json:"repair_proof_summaries,omitempty"`
	Verification       []findingExplainVerification `json:"verification_commands"`
	Hash               string                       `json:"hash"`
	Markdown           string                       `json:"markdown,omitempty"`
}

type findingExplainAlternative struct {
	ID       string `json:"id"`
	StableID string `json:"stable_id,omitempty"`
	Severity string `json:"severity"`
	Score    int    `json:"score"`
	Reason   string `json:"reason,omitempty"`
}

type findingExplainVerification struct {
	Command string `json:"command"`
	Reason  string `json:"reason"`
}

func explainCommand(args []string) error {
	id := args[0]
	if analysis, ok := flagValue(args[1:], "--analysis"); ok {
		report, err := buildFindingExplainReport(id, analysis)
		if err != nil {
			return err
		}
		if hasFlag(args[1:], "--json") {
			return writeJSON(os.Stdout, report)
		}
		fmt.Print(report.Markdown)
		return nil
	}
	g, err := graphFor(args[1:])
	if err != nil {
		return err
	}
	return explain(g, id)
}

func buildFindingExplainReport(findingID, analysisPath string) (findingExplainReport, error) {
	baseline, err := project.LoadBaseline(filepath.Join(analysisPath, "baseline"))
	if err != nil {
		return findingExplainReport{}, err
	}
	facts, err := project.LoadFacts(filepath.Join(analysisPath, "inventory", "facts.jsonl"))
	if err != nil {
		return findingExplainReport{}, err
	}
	riskIndex := -1
	for i, risk := range baseline.Risks {
		if risk.ID == findingID || risk.StableID == findingID {
			riskIndex = i
			break
		}
	}
	if riskIndex < 0 {
		return findingExplainReport{}, fmt.Errorf("finding %q not found in %s", findingID, analysisPath)
	}
	risk := baseline.Risks[riskIndex]
	factsByID := map[string]project.Fact{}
	for _, fact := range facts {
		factsByID[fact.ID] = fact
	}
	report := findingExplainReport{
		Version:        "patchline.finding-explain/v1",
		FindingID:      findingID,
		Analysis:       filepath.ToSlash(analysisPath),
		Risk:           risk,
		RankingFactors: append([]project.ScoreFactor(nil), risk.Factors...),
		Verification:   findingVerificationCommands(risk, analysisPath),
		Alternatives:   findingAlternatives(baseline.Risks, riskIndex),
		PolicyChecks:   matchingPolicyChecks(baseline.PolicyChecks, risk.ID),
		SymbolicChecks: matchingSymbolicChecks(baseline.SymbolicChecks, risk.ID),
		RepairProofs:   matchingRepairProofs(baseline.RepairProofs, risk.ID),
	}
	for _, explanation := range baseline.Rankings {
		if explanation.RiskID == risk.ID {
			report.RankingExplanation = explanation
			break
		}
	}
	for _, link := range baseline.EvidenceLinks {
		if link.RiskID == risk.ID || link.RiskID == risk.StableID {
			report.Evidence = append(report.Evidence, link)
			if fact, ok := factsByID[link.FactID]; ok {
				report.Facts = append(report.Facts, fact)
			}
		}
	}
	report.ProofHoles = findingProofHoles(report.SymbolicChecks, report.RepairProofs)
	report.Hash = findingExplainHash(report)
	report.Markdown = renderFindingExplainMarkdown(report)
	return report, nil
}

func findingAlternatives(risks []project.BaselineRisk, riskIndex int) []findingExplainAlternative {
	var alternatives []findingExplainAlternative
	for i, risk := range risks {
		if i == riskIndex {
			continue
		}
		alternatives = append(alternatives, findingExplainAlternative{ID: risk.ID, StableID: risk.StableID, Severity: risk.Severity, Score: risk.Score, Reason: risk.Rationale})
		if len(alternatives) == 3 {
			break
		}
	}
	return alternatives
}

func matchingPolicyChecks(checks []project.PolicyCheck, riskID string) []project.PolicyCheck {
	var out []project.PolicyCheck
	for _, check := range checks {
		if check.RiskID == riskID {
			out = append(out, check)
		}
	}
	return out
}

func matchingSymbolicChecks(checks []project.SymbolicCheck, riskID string) []project.SymbolicCheck {
	var out []project.SymbolicCheck
	for _, check := range checks {
		if check.RiskID == riskID {
			out = append(out, check)
		}
	}
	return out
}

func matchingRepairProofs(proofs []project.RepairProofSummary, riskID string) []project.RepairProofSummary {
	var out []project.RepairProofSummary
	for _, proof := range proofs {
		if proof.RiskID == riskID {
			out = append(out, proof)
		}
	}
	return out
}

func findingProofHoles(symbolic []project.SymbolicCheck, proofs []project.RepairProofSummary) []string {
	seen := map[string]bool{}
	var holes []string
	for _, check := range symbolic {
		if check.Status == "fail" || check.Status == "warn" {
			value := check.Property + ": " + check.Reason
			if !seen[value] {
				seen[value] = true
				holes = append(holes, value)
			}
		}
	}
	for _, proof := range proofs {
		for _, hole := range proof.ProofHoles {
			if !seen[hole] {
				seen[hole] = true
				holes = append(holes, hole)
			}
		}
	}
	sort.Strings(holes)
	return holes
}

func findingVerificationCommands(risk project.BaselineRisk, analysisPath string) []findingExplainVerification {
	var commands []findingExplainVerification
	if strings.TrimSpace(risk.NextCommand) != "" {
		commands = append(commands, findingExplainVerification{Command: risk.NextCommand, Reason: "re-run the exact command attached to the ranked finding"})
	}
	commands = append(commands, findingExplainVerification{Command: "go run ./cmd/patchline repo baseline --inventory " + shellArg(filepath.Join(analysisPath, "inventory")) + " --intake " + shellArg(filepath.Join(analysisPath, "intake")) + " --out results/generated/explain-baseline --json", Reason: "regenerate the ranked baseline and confirm the finding is still present"})
	if strings.TrimSpace(risk.StableID) != "" {
		commands = append(commands, findingExplainVerification{Command: "go run ./cmd/patchline explain " + shellArg(risk.StableID) + " --analysis " + shellArg(analysisPath) + " --json", Reason: "verify this explanation report by stable finding ID"})
	}
	return commands
}

func findingExplainHash(report findingExplainReport) string {
	copy := report
	copy.Hash = ""
	copy.Markdown = ""
	return canonical.Hash(copy)
}

func renderFindingExplainMarkdown(report findingExplainReport) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Patchline finding explanation\n\n")
	fmt.Fprintf(&b, "- finding: `%s`\n", report.FindingID)
	fmt.Fprintf(&b, "- risk: `%s`\n", report.Risk.ID)
	if report.Risk.StableID != "" {
		fmt.Fprintf(&b, "- stable_id: `%s`\n", report.Risk.StableID)
	}
	fmt.Fprintf(&b, "- severity: `%s`\n", report.Risk.Severity)
	fmt.Fprintf(&b, "- score: `%d`\n", report.Risk.Score)
	fmt.Fprintf(&b, "- path: `%s`\n", report.Risk.Path)
	fmt.Fprintf(&b, "- hash: `%s`\n\n", report.Hash)
	fmt.Fprintf(&b, "## Evidence\n\n")
	for _, link := range firstNEvidenceLinks(report.Evidence, 8) {
		fmt.Fprintf(&b, "- `%s` %s `%s` confidence=%s\n", link.FactID, link.FactKind, link.Path, link.Confidence)
	}
	fmt.Fprintf(&b, "\n## Ranking factors\n\n")
	for _, factor := range report.RankingFactors {
		fmt.Fprintf(&b, "- %s %+d: %s\n", factor.Name, factor.Weight, factor.Reason)
	}
	fmt.Fprintf(&b, "\n## Alternatives considered\n\n")
	for _, alt := range report.Alternatives {
		fmt.Fprintf(&b, "- `%s` score=%d severity=%s\n", alt.ID, alt.Score, alt.Severity)
	}
	fmt.Fprintf(&b, "\n## Proof holes\n\n")
	if len(report.ProofHoles) == 0 {
		fmt.Fprintf(&b, "- none recorded for this finding\n")
	} else {
		for _, hole := range report.ProofHoles {
			fmt.Fprintf(&b, "- %s\n", hole)
		}
	}
	fmt.Fprintf(&b, "\n## Verification commands\n\n")
	for _, command := range report.Verification {
		fmt.Fprintf(&b, "- `%s` — %s\n", command.Command, command.Reason)
	}
	return b.String()
}

func firstNEvidenceLinks(values []project.EvidenceLink, n int) []project.EvidenceLink {
	if len(values) < n {
		n = len(values)
	}
	return append([]project.EvidenceLink(nil), values[:n]...)
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
		eval, err := buildPolicyEvaluation(policyPath, repairPath, migrationPath, demo.Graph(), demo.BillingStore())
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

func dbDryRun(path string, args []string, jsonOut bool) error {
	fs := flag.NewFlagSet("db-dry-run", flag.ContinueOnError)
	fs.SetOutput(ioDiscard{})
	dialect := fs.String("dialect", "", "database dialect: postgres or mysql")
	dsn := fs.String("dsn", "", "optional localhost/container DSN")
	execute := fs.Bool("execute", false, "execute the generated schema-only script against the local DSN")
	_ = fs.Bool("json", false, "emit JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 || *dialect == "" {
		return errors.New("usage: patchline db-dry-run <manifest.json> --dialect <postgres|mysql> [--dsn local-dsn] [--execute] [--json]")
	}
	manifest, err := readManifest(path)
	if err != nil {
		return err
	}
	report, err := dbdryrun.Build(manifest, dbdryrun.Options{Dialect: *dialect, DSN: *dsn, Execute: *execute})
	if err != nil {
		return err
	}
	if *execute {
		report, err = dbdryrun.Execute(report, *dsn)
		if err != nil && !jsonOut {
			return err
		}
	}
	if jsonOut {
		if err := writeJSON(os.Stdout, report); err != nil {
			return err
		}
		if !report.OK {
			return codedError{code: 2, err: errors.New("database dry-run failed")}
		}
		return nil
	}
	fmt.Printf("db dry-run manifest=%s dialect=%s mode=%s schema_tables=%d statements=%d hash=%s\n",
		report.Manifest, report.Dialect, report.Mode, len(report.Schema), len(report.Statements), report.Hash)
	fmt.Printf("  container: %s\n", report.Container.RunCommand)
	fmt.Printf("  client: %s\n", report.Container.ClientHint)
	for _, warning := range report.Warnings {
		fmt.Printf("  warning: %s\n", warning)
	}
	if report.Execution != nil {
		fmt.Printf("  execution client=%s exit=%d\n", report.Execution.Client, report.Execution.ExitCode)
	}
	if !report.OK {
		return codedError{code: 2, err: errors.New("database dry-run failed")}
	}
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
	eval, err := buildPolicyEvaluation(policyPath, repairPath, migrationPath, demo.Graph(), demo.BillingStore())
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
	eval, err := buildPolicyEvaluation(policyPath, manifestPath, migrationPath, demo.Graph(), demo.BillingStore())
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

func artifactGroundTruth(root string, jsonOut bool) error {
	report, err := artifact.ValidateGroundTruth(root)
	if err != nil {
		return err
	}
	if jsonOut {
		return writeJSON(os.Stdout, report)
	}
	fmt.Printf("artifact_ground_truth ok=%t files=%d manifests=%d\n", report.OK, report.GroundTruthFiles, report.Manifests)
	if len(report.Errors) > 0 {
		for _, validationErr := range report.Errors {
			fmt.Printf("  %s case=%s error=%s\n", validationErr.File, validationErr.CaseID, validationErr.Message)
		}
		return fmt.Errorf("artifact ground truth validation failed with %d error(s)", len(report.Errors))
	}
	return nil
}

func artifactBaselines(path string, outPath string, jsonOut bool) error {
	spec, err := readBenchmarkSpec(path)
	if err != nil {
		return err
	}
	report, err := artifact.EvaluateBaselines(spec, filepath.Dir(path))
	if err != nil {
		return err
	}
	if outPath != "" {
		if err := writeArtifactStudy(outPath, "baselines", report, report.Markdown); err != nil {
			return err
		}
	}
	if jsonOut {
		return writeJSON(os.Stdout, report)
	}
	fmt.Printf("artifact baselines suite=%s hash=%s patchline_actionability=%.2f\n", report.Suite, report.Hash, report.Patchline.MeanActionability)
	for _, baseline := range report.Baselines {
		fmt.Printf("  baseline=%s precision=%.3f recall=%.3f actionability=%.2f\n", baseline.Name, baseline.Metrics.Precision, baseline.Metrics.Recall, baseline.Metrics.MeanActionability)
	}
	return nil
}

func artifactAblations(path string, outPath string, jsonOut bool) error {
	spec, err := readBenchmarkSpec(path)
	if err != nil {
		return err
	}
	report, err := artifact.RunAblations(spec, filepath.Dir(path))
	if err != nil {
		return err
	}
	if outPath != "" {
		if err := writeArtifactStudy(outPath, "ablations", report, report.Markdown); err != nil {
			return err
		}
	}
	if jsonOut {
		return writeJSON(os.Stdout, report)
	}
	fmt.Printf("artifact ablations suite=%s hash=%s modes=%d\n", report.Suite, report.Hash, len(report.Modes))
	for _, mode := range report.Modes {
		fmt.Printf("  mode=%s precision=%.3f recall=%.3f actionability=%.2f proof_backed=%d archive_linked=%d\n", mode.Name, mode.Metrics.Precision, mode.Metrics.Recall, mode.Metrics.MeanActionability, mode.Metrics.ProofBackedCases, mode.Metrics.ArchiveLinkedCases)
	}
	return nil
}

func artifactScale(path string, outPath string, jsonOut bool) error {
	spec, err := readBenchmarkSpec(path)
	if err != nil {
		return err
	}
	report, err := artifact.MeasureScale(spec, filepath.Dir(path))
	if err != nil {
		return err
	}
	if outPath != "" {
		if err := writeArtifactStudy(outPath, "scale", report, report.Markdown); err != nil {
			return err
		}
	}
	if jsonOut {
		return writeJSON(os.Stdout, report)
	}
	fmt.Printf("artifact scale suite=%s hash=%s cases=%d statements=%d bytes=%d analyze_ms=%d\n", report.Suite, report.Hash, report.Totals.Cases, report.Totals.Statements, report.Totals.Bytes, report.Totals.AnalyzeMillis)
	return nil
}

func phaseCheck(path string, jsonOut bool) error {
	report, err := artifact.ValidateBenchmarkManifest(path)
	if err != nil {
		return err
	}
	manifest, err := readPhaseCheckManifest(path)
	if err != nil {
		return err
	}
	if jsonOut {
		if err := writeJSON(os.Stdout, report); err != nil {
			return err
		}
	} else {
		fmt.Printf("phase check manifest=%s ok=%t cases=%d ground_truth_files=%d errors=%d\n", path, report.OK, len(manifest.Cases), report.GroundTruthFiles, len(report.Errors))
		caseErrors := map[string]int{}
		for _, validationErr := range report.Errors {
			caseErrors[validationErr.CaseID]++
		}
		for _, c := range manifest.Cases {
			fmt.Printf("  case=%s phase=%s input_kind=%s errors=%d\n", c.CaseID, c.AvailableAt, phaseCheckInputKind(c), caseErrors[c.CaseID])
		}
		for _, validationErr := range report.Errors {
			fmt.Printf("  error case=%s file=%s message=%s\n", validationErr.CaseID, validationErr.File, validationErr.Message)
		}
	}
	if !report.OK {
		return errors.New("phase check failed")
	}
	return nil
}

func readPhaseCheckManifest(path string) (artifact.Manifest, error) {
	f, err := os.Open(path)
	if err != nil {
		return artifact.Manifest{}, err
	}
	defer f.Close()
	var manifest artifact.Manifest
	if err := json.NewDecoder(f).Decode(&manifest); err != nil {
		return artifact.Manifest{}, err
	}
	return manifest, nil
}

func phaseCheckInputKind(c artifact.ManifestCase) string {
	if c.InputKind != "" {
		return c.InputKind
	}
	if fixture, ok := phaseCheckInlineInputKinds[c.Fixture]; ok {
		return fixture
	}
	switch c.CaseType {
	case "migration":
		return "migration_text"
	case "incident":
		return "evidence_jsonl"
	case "repair":
		return "repair_plan"
	case "regression":
		if strings.HasSuffix(c.Fixture, ".sql") {
			return "migration_text"
		}
		return "prior_archive"
	default:
		return "unknown"
	}
}

var phaseCheckInlineInputKinds = map[string]string{
	"inline:procedural-sql":          "migration_text",
	"inline:public-summary-too-thin": "postmortem_text",
	"inline:phase-guard":             "postmortem_text",
}

func artifactBenchmark(args []string) error {
	switch args[0] {
	case "validate":
		if len(args) < 2 {
			return errors.New("usage: patchline artifact-benchmark validate <manifest.json> [--json]")
		}
		report, err := artifact.ValidateBenchmarkManifest(args[1])
		if err != nil {
			return err
		}
		if hasFlag(args[2:], "--json") {
			if err := writeJSON(os.Stdout, report); err != nil {
				return err
			}
		} else {
			fmt.Printf("artifact benchmark manifest=%s ok=%t ground_truth_files=%d errors=%d\n", args[1], report.OK, report.GroundTruthFiles, len(report.Errors))
		}
		if !report.OK {
			return errors.New("artifact benchmark manifest validation failed")
		}
		return nil
	case "run":
		if len(args) < 2 {
			return errors.New("usage: patchline artifact-benchmark run <manifest.json> [--out report.json] [--json]")
		}
		outPath, _ := flagValue(args[2:], "--out")
		report, err := artifact.RunBenchmarkManifest(args[1])
		if err != nil {
			return err
		}
		if outPath != "" {
			if err := writeBenchmarkRun(outPath, report); err != nil {
				return err
			}
		}
		if hasFlag(args[2:], "--json") {
			if err := writeJSON(os.Stdout, report); err != nil {
				return err
			}
		} else {
			fmt.Printf("artifact benchmark dataset=%s ok=%t hash=%s passed=%d failed=%d\n", report.DatasetID, report.OK, report.Hash, report.Metrics.Passed, report.Metrics.Failed)
			for _, c := range report.Cases {
				fmt.Printf("  case=%s expected=%s actual=%s ok=%t\n", c.CaseID, c.ExpectedResult, c.ActualResult, c.OK)
			}
		}
		if !report.OK {
			return errors.New("artifact benchmark mismatched ground truth")
		}
		return nil
	case "compare":
		if len(args) < 3 {
			return errors.New("usage: patchline artifact-benchmark compare <actual.json> <expected.json> [--json]")
		}
		report, err := artifact.CompareBenchmarkReports(args[1], args[2])
		if err != nil {
			return err
		}
		if hasFlag(args[3:], "--json") {
			if err := writeJSON(os.Stdout, report); err != nil {
				return err
			}
		} else {
			fmt.Printf("artifact benchmark compare ok=%t actual_hash=%s expected_hash=%s mismatches=%d\n", report.OK, report.ActualHash, report.ExpectedHash, len(report.Mismatches))
			for _, mismatch := range report.Mismatches {
				fmt.Printf("  case=%s field=%s actual=%s expected=%s\n", mismatch.CaseID, mismatch.Field, mismatch.Actual, mismatch.Expected)
			}
		}
		if !report.OK {
			return errors.New("artifact benchmark comparison failed")
		}
		return nil
	default:
		return fmt.Errorf("unknown artifact-benchmark subcommand %q", args[0])
	}
}

func artifactStudy(args []string) error {
	switch args[0] {
	case "summarize":
		if len(args) < 2 {
			return errors.New("usage: patchline artifact-study summarize <report-dir> [--out expected.json] [--json]")
		}
		outPath, _ := flagValue(args[2:], "--out")
		manifest, err := artifact.SummarizeStudyReports(args[1])
		if err != nil {
			return err
		}
		if outPath != "" {
			if err := artifact.WriteStudyExpectedManifest(outPath, manifest); err != nil {
				return err
			}
		}
		if hasFlag(args[2:], "--json") {
			return writeJSON(os.Stdout, manifest)
		}
		fmt.Printf("artifact study expected suite=%s hash=%s reports=%d\n", manifest.Suite, manifest.Hash, len(manifest.Reports))
		if outPath != "" {
			fmt.Printf("  written: %s\n", outPath)
		}
		return nil
	case "compare":
		if len(args) < 3 {
			return errors.New("usage: patchline artifact-study compare <report-dir> <expected.json> [--json]")
		}
		report, err := artifact.CompareStudyReports(args[1], args[2])
		if err != nil {
			return err
		}
		if hasFlag(args[3:], "--json") {
			if err := writeJSON(os.Stdout, report); err != nil {
				return err
			}
		} else {
			fmt.Printf("artifact study compare ok=%t suite=%s actual_hash=%s expected_hash=%s mismatches=%d\n", report.OK, report.Suite, report.ActualHash, report.ExpectedHash, len(report.Mismatches))
			for _, mismatch := range report.Mismatches {
				fmt.Printf("  report=%s field=%s actual=%s expected=%s\n", mismatch.Report, mismatch.Field, mismatch.Actual, mismatch.Expected)
			}
		}
		if !report.OK {
			return errors.New("artifact study comparison failed")
		}
		return nil
	default:
		return fmt.Errorf("unknown artifact-study subcommand %q", args[0])
	}
}

func artifactTables(args []string) error {
	root, _ := flagValue(args, "--root")
	if root == "" {
		root = "."
	}
	outPath, _ := flagValue(args, "--out")
	if outPath == "" {
		outPath = filepath.Join("results", "generated", "artifact-tables")
	}
	report, err := artifact.GeneratePaperTables(root)
	if err != nil {
		return err
	}
	if outPath != "" {
		if err := artifact.WritePaperTablesReport(outPath, report); err != nil {
			return err
		}
	}
	if hasFlag(args, "--json") {
		return writeJSON(os.Stdout, report)
	}
	fmt.Printf("artifact tables hash=%s tables=%d out=%s\n", report.Hash, len(report.Tables), outPath)
	for _, table := range report.Tables {
		fmt.Printf("  %s rows=%d\n", table.ID, len(table.Rows))
	}
	return nil
}

func artifactNumbers(args []string) error {
	root, _ := flagValue(args, "--root")
	if root == "" {
		root = "."
	}
	outPath, _ := flagValue(args, "--out")
	if outPath == "" {
		outPath = filepath.Join("results", "generated", "artifact-numbers")
	}
	report, err := artifact.GenerateExperimentNumbers(root)
	if err != nil {
		return err
	}
	if outPath != "" {
		if err := artifact.WriteExperimentNumbersReport(outPath, report); err != nil {
			return err
		}
	}
	if hasFlag(args, "--json") {
		return writeJSON(os.Stdout, report)
	}
	fmt.Printf("artifact numbers hash=%s studies=%d benchmarks=%d inputs=%d out=%s\n", report.Hash, len(report.Studies), len(report.Benchmarks), len(report.Inputs), outPath)
	for _, study := range report.Studies {
		fmt.Printf("  %s cases=%d baselines=%d ablations=%d statements=%d\n", study.Name, study.Patchline.Total, len(study.Baselines), len(study.Ablations), study.Scale.Statements)
	}
	return nil
}

func artifactSubtasks(args []string) error {
	root, _ := flagValue(args, "--root")
	if root == "" {
		root = "."
	}
	outPath, _ := flagValue(args, "--out")
	if outPath == "" {
		outPath = filepath.Join("results", "generated", "artifact-subtasks")
	}
	report, err := artifact.GenerateSubtaskComparisonReport(root)
	if err != nil {
		return err
	}
	if outPath != "" {
		if err := artifact.WriteSubtaskComparisonReport(outPath, report); err != nil {
			return err
		}
	}
	if hasFlag(args, "--json") {
		return writeJSON(os.Stdout, report)
	}
	fmt.Printf("artifact subtasks hash=%s subtasks=%d source_numbers=%s out=%s\n", report.Hash, len(report.Subtasks), report.SourceNumbersHash, outPath)
	for _, subtask := range report.Subtasks {
		fmt.Printf("  %s comparators=%d wins=%d\n", subtask.ID, len(subtask.Comparators), len(subtask.Wins))
	}
	return nil
}

func artifactCorpusAudit(args []string) error {
	root, _ := flagValue(args, "--root")
	if root == "" {
		root = "."
	}
	protocol, _ := flagValue(args, "--protocol")
	outPath, _ := flagValue(args, "--out")
	if outPath == "" {
		outPath = filepath.Join("results", "generated", "artifact-corpus-audit")
	}
	report, err := artifact.GenerateCorpusAudit(root, protocol)
	if err != nil {
		return err
	}
	if outPath != "" {
		if err := artifact.WriteCorpusAuditReport(outPath, report); err != nil {
			return err
		}
	}
	if hasFlag(args, "--json") {
		return writeJSON(os.Stdout, report)
	}
	fmt.Printf("artifact corpus audit hash=%s manifests=%d pools=%d commands=%d out=%s\n", report.Hash, len(report.Manifests), len(report.Pools), len(report.Commands), outPath)
	for _, manifest := range report.Manifests {
		fmt.Printf("  %s cases=%d boundary=%d ok=%t\n", manifest.DatasetID, manifest.Cases, manifest.BoundaryCases, manifest.OK)
	}
	if !report.OK {
		return errors.New("artifact corpus audit failed")
	}
	return nil
}

func artifactProvenance(args []string) error {
	root, _ := flagValue(args, "--root")
	if root == "" {
		root = "."
	}
	outPath, _ := flagValue(args, "--out")
	if outPath == "" {
		outPath = filepath.Join("results", "generated", "artifact-provenance")
	}
	report, err := artifact.GenerateArtifactProvenance(root)
	if err != nil {
		return err
	}
	if outPath != "" {
		if err := artifact.WriteArtifactProvenanceReport(outPath, report); err != nil {
			return err
		}
	}
	if hasFlag(args, "--json") {
		return writeJSON(os.Stdout, report)
	}
	fmt.Printf("artifact provenance hash=%s files=%d checks=%d out=%s\n", report.Hash, len(report.Files), len(report.Checks), outPath)
	for _, check := range report.Checks {
		result := "pass"
		if !check.OK {
			result = "fail"
		}
		fmt.Printf("  %s=%s %s\n", check.ID, result, check.Summary)
	}
	return nil
}

func writeBenchmarkRun(outPath string, report artifact.BenchmarkRunReport) error {
	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		return err
	}
	if err := artifact.WriteBenchmarkReport(outPath, report); err != nil {
		return err
	}
	if report.Markdown != "" && strings.HasSuffix(outPath, ".json") {
		mdPath := strings.TrimSuffix(outPath, ".json") + ".md"
		if err := os.WriteFile(mdPath, []byte(report.Markdown), 0o644); err != nil {
			return err
		}
	}
	return nil
}

func writeArtifactStudy(outPath string, stem string, report any, markdown string) error {
	if err := os.MkdirAll(outPath, 0o755); err != nil {
		return err
	}
	jsonPath := filepath.Join(outPath, stem+".json")
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(jsonPath, append(data, '\n'), 0o644); err != nil {
		return err
	}
	if markdown != "" {
		if err := os.WriteFile(filepath.Join(outPath, stem+".md"), []byte(markdown), 0o644); err != nil {
			return err
		}
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

func buildPolicyEvaluation(policyPath, repairPath, migrationPath string, graph *provenance.Graph, store replay.Store) (policy.Evaluation, error) {
	pol, err := readPolicy(policyPath)
	if err != nil {
		return policy.Evaluation{}, err
	}
	manifest, err := readManifest(repairPath)
	if err != nil {
		return policy.Evaluation{}, err
	}
	report, err := replay.DryRun(manifest, graph, store)
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

func sqlDialectUsage() string {
	return "generic|postgres|mysql|sqlite|sqlserver|oracle|bigquery"
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

func boolKeys(values map[string]bool) []string {
	out := make([]string, 0, len(values))
	for key := range values {
		out = append(out, key)
	}
	sort.Strings(out)
	return out
}

func splitNonEmpty(value, sep string) []string {
	var out []string
	for _, part := range strings.Split(value, sep) {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
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
