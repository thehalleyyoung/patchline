package dbsemantics

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/thehalleyyoung/patchline/internal/canonical"
)

const ReproducibilityReportVersion = "patchline.db-semantics-reproducibility/v1"

type ReproducibilityReport struct {
	Version      string                       `json:"version"`
	SourceReport []ReproducibilitySource      `json:"source_reports"`
	EnginePins   []EngineReproducibilityPin   `json:"engine_pins"`
	Observations []EngineBehaviorObservation  `json:"observations"`
	Summary      ReproducibilityReportSummary `json:"summary"`
	Hash         string                       `json:"hash"`
}

type ReproducibilitySource struct {
	Engine           Engine `json:"engine"`
	RequestedVersion string `json:"requested_version"`
	ResolvedVersion  string `json:"resolved_version"`
	Source           string `json:"source"`
	InputHash        string `json:"input_hash"`
	RuntimeHintHash  string `json:"runtime_hint_hash,omitempty"`
	ReportHash       string `json:"report_hash"`
	Statements       int    `json:"statements"`
}

type EngineReproducibilityPin struct {
	Engine                  Engine   `json:"engine"`
	RequestedVersion        string   `json:"requested_version"`
	ResolvedVersion         string   `json:"resolved_version"`
	RepresentativeVersions  []string `json:"representative_versions"`
	ContainerImage          string   `json:"container_image,omitempty"`
	ContainerImagePinnedBy  string   `json:"container_image_pinned_by,omitempty"`
	RuntimeKind             string   `json:"runtime_kind"`
	LocalExecutionAvailable bool     `json:"local_execution_available"`
	VerificationStatus      string   `json:"verification_status"`
	SmokeCommand            string   `json:"smoke_command"`
	ProfileEvidenceRefs     []string `json:"profile_evidence_refs"`
	ReportHash              string   `json:"report_hash"`
	Notes                   []string `json:"notes,omitempty"`
}

type EngineBehaviorObservation struct {
	ID              string `json:"id"`
	Engine          Engine `json:"engine"`
	ResolvedVersion string `json:"resolved_version"`
	ReportHash      string `json:"report_hash"`
	StatementIndex  int    `json:"statement_index,omitempty"`
	Kind            string `json:"kind"`
	Table           string `json:"table,omitempty"`
	ObservationKind string `json:"observation_kind"`
	Status          string `json:"status"`
	Reference       string `json:"reference,omitempty"`
	RuleID          string `json:"rule_id,omitempty"`
	Severity        string `json:"severity,omitempty"`
	Verdict         string `json:"verdict,omitempty"`
	LockMode        string `json:"lock_mode,omitempty"`
	DurationClass   string `json:"duration_class,omitempty"`
	Evidence        string `json:"evidence"`
}

type ReproducibilityReportSummary struct {
	Reports                   int      `json:"reports"`
	Engines                   int      `json:"engines"`
	EngineVersionPins         int      `json:"engine_version_pins"`
	ContainerImages           int      `json:"container_images"`
	LocalExecutionRuntimes    int      `json:"local_execution_runtimes"`
	ManagedOrEmbeddedProfiles int      `json:"managed_or_embedded_profiles"`
	Statements                int      `json:"statements"`
	Observations              int      `json:"observations"`
	ProfileEvidence           int      `json:"profile_evidence"`
	StatementRules            int      `json:"statement_rules"`
	LockSimulations           int      `json:"lock_simulations"`
	ContainerSmokeFixtures    int      `json:"container_smoke_fixtures"`
	EngineNegativeControls    int      `json:"engine_negative_controls"`
	RollbackChecks            int      `json:"rollback_checks"`
	QueryPlanChecks           int      `json:"query_plan_checks"`
	RuntimeEstimates          int      `json:"runtime_estimates"`
	ReplicationLagRisks       int      `json:"replication_lag_risks"`
	PartitionShardingFindings int      `json:"partition_sharding_findings"`
	SupportedEnginesCovered   []Engine `json:"supported_engines_covered"`
}

type engineRuntimePin struct {
	image                   string
	pinnedBy                string
	runtimeKind             string
	localExecutionAvailable bool
	verificationStatus      string
	smokeCommand            string
	notes                   []string
}

func BuildReproducibilityReport(reports []Report) (ReproducibilityReport, error) {
	if len(reports) == 0 {
		return ReproducibilityReport{}, fmt.Errorf("database-semantics reproducibility report requires at least one db-semantics report")
	}
	normalized := append([]Report(nil), reports...)
	sort.Slice(normalized, func(i, j int) bool {
		left, right := sourceKey(normalized[i]), sourceKey(normalized[j])
		return left < right
	})
	seenVersion := map[string]bool{}
	coveredEngines := map[Engine]bool{}
	report := ReproducibilityReport{Version: ReproducibilityReportVersion}
	for _, source := range normalized {
		if source.Version != ReportVersion {
			return ReproducibilityReport{}, fmt.Errorf("report %q has version %q, want %q", source.Source, source.Version, ReportVersion)
		}
		if source.Hash == "" || source.InputHash == "" {
			return ReproducibilityReport{}, fmt.Errorf("report %q is missing input/report hash evidence", source.Source)
		}
		if len(source.Statements) == 0 {
			return ReproducibilityReport{}, fmt.Errorf("report %q has no statement observations", source.Source)
		}
		key := string(source.Profile.Engine) + "@" + source.Profile.ResolvedVersion
		if seenVersion[key] {
			return ReproducibilityReport{}, fmt.Errorf("duplicate engine/version report %s", key)
		}
		seenVersion[key] = true
		coveredEngines[source.Profile.Engine] = true
		report.SourceReport = append(report.SourceReport, reproducibilitySource(source))
		report.EnginePins = append(report.EnginePins, engineReproducibilityPin(source))
		report.Observations = append(report.Observations, reportObservations(source)...)
	}
	for _, engine := range SupportedEngines() {
		if !coveredEngines[engine] {
			return ReproducibilityReport{}, fmt.Errorf("missing db-semantics report for supported engine %s", engine)
		}
		report.Summary.SupportedEnginesCovered = append(report.Summary.SupportedEnginesCovered, engine)
	}
	finalizeReproducibilityReport(&report)
	report.Hash = reproducibilityReportHash(report)
	return report, nil
}

func reproducibilitySource(report Report) ReproducibilitySource {
	return ReproducibilitySource{
		Engine:           report.Profile.Engine,
		RequestedVersion: report.Profile.RequestedVersion,
		ResolvedVersion:  report.Profile.ResolvedVersion,
		Source:           report.Source,
		InputHash:        report.InputHash,
		RuntimeHintHash:  report.RuntimeHintHash,
		ReportHash:       report.Hash,
		Statements:       len(report.Statements),
	}
}

func engineReproducibilityPin(report Report) EngineReproducibilityPin {
	runtime := pinnedRuntime(report.Profile.Engine, report.Profile.ResolvedVersion)
	return EngineReproducibilityPin{
		Engine:                  report.Profile.Engine,
		RequestedVersion:        report.Profile.RequestedVersion,
		ResolvedVersion:         report.Profile.ResolvedVersion,
		RepresentativeVersions:  append([]string(nil), report.Profile.RepresentativeVersions...),
		ContainerImage:          runtime.image,
		ContainerImagePinnedBy:  runtime.pinnedBy,
		RuntimeKind:             runtime.runtimeKind,
		LocalExecutionAvailable: runtime.localExecutionAvailable,
		VerificationStatus:      runtime.verificationStatus,
		SmokeCommand:            runtime.smokeCommand,
		ProfileEvidenceRefs:     evidenceRefs(report.Profile.Evidence),
		ReportHash:              report.Hash,
		Notes:                   append([]string(nil), runtime.notes...),
	}
}

func pinnedRuntime(engine Engine, version string) engineRuntimePin {
	switch engine {
	case EnginePostgres:
		return executableImage("postgres:"+version, "engine-version-tag", "docker-compatible-engine", "docker run --rm postgres:"+version+" postgres --version")
	case EngineMySQL:
		return executableImage("mysql:"+version, "engine-version-tag", "docker-compatible-engine", "docker run --rm mysql:"+version+" mysqld --version")
	case EngineSQLite:
		return engineRuntimePin{
			runtimeKind:             "embedded-engine",
			verificationStatus:      "pinned-by-sqlite-version-profile",
			smokeCommand:            "sqlite3 --version",
			localExecutionAvailable: true,
			notes:                   []string{"SQLite is embedded rather than a server image; the report pins the resolved SQLite version profile and requires a local sqlite3 binary or fixture replay."},
		}
	case EngineSQLServer:
		tag := version
		if tag == "2022" {
			tag = "2022-CU14-ubuntu-22.04"
		}
		return executableImage("mcr.microsoft.com/mssql/server:"+tag, "engine-release-tag", "docker-compatible-engine", "docker run --rm mcr.microsoft.com/mssql/server:"+tag+" /opt/mssql/bin/sqlservr --version")
	case EngineOracle:
		tag := version
		if strings.HasPrefix(version, "23") {
			tag = "23.4.0.0"
		}
		return executableImage("container-registry.oracle.com/database/free:"+tag, "engine-release-tag", "docker-compatible-engine", "docker run --rm container-registry.oracle.com/database/free:"+tag+" sqlplus -V")
	case EngineBigQuery:
		return engineRuntimePin{
			image:                   "ghcr.io/goccy/bigquery-emulator:0.6.6",
			pinnedBy:                "emulator-release-tag",
			runtimeKind:             "managed-service-emulator",
			localExecutionAvailable: true,
			verificationStatus:      "emulator-pin-plus-managed-service-evidence",
			smokeCommand:            "docker run --rm ghcr.io/goccy/bigquery-emulator:0.6.6 --help",
			notes:                   []string{"BigQuery production behavior is a managed service; the report pins an emulator image for local replay and keeps managed-service semantics as documented evidence."},
		}
	case EngineSnowflake:
		return engineRuntimePin{
			runtimeKind:        "managed-service-profile",
			verificationStatus: "managed-service-documentation-only",
			smokeCommand:       "run db-semantics against the resolved Snowflake profile; attach account-local query-profile evidence when available",
			notes:              []string{"Snowflake engine behavior is managed-service evidence; no local engine container is required or trusted by default."},
		}
	case EngineClickHouse:
		return executableImage("clickhouse/clickhouse-server:"+version, "engine-version-tag", "docker-compatible-engine", "docker run --rm clickhouse/clickhouse-server:"+version+" clickhouse-server --version")
	default:
		return engineRuntimePin{runtimeKind: "unknown", verificationStatus: "unsupported"}
	}
}

func executableImage(image, pinnedBy, kind, command string) engineRuntimePin {
	return engineRuntimePin{
		image:                   image,
		pinnedBy:                pinnedBy,
		runtimeKind:             kind,
		localExecutionAvailable: true,
		verificationStatus:      "pinned-reference-not-executed-by-default",
		smokeCommand:            command,
	}
}

func reportObservations(report Report) []EngineBehaviorObservation {
	var observations []EngineBehaviorObservation
	for _, evidence := range report.Profile.Evidence {
		observations = append(observations, EngineBehaviorObservation{
			ID:              observationID(report, "profile", evidence.Ref, -1),
			Engine:          report.Profile.Engine,
			ResolvedVersion: report.Profile.ResolvedVersion,
			ReportHash:      report.Hash,
			ObservationKind: "profile_evidence",
			Status:          "documented-profile",
			Reference:       evidence.Ref,
			Evidence:        evidence.Rule,
		})
	}
	for _, statement := range report.Statements {
		for _, rule := range statement.Rules {
			observations = append(observations, EngineBehaviorObservation{
				ID:              observationID(report, "rule", rule.ID, statement.Index),
				Engine:          report.Profile.Engine,
				ResolvedVersion: report.Profile.ResolvedVersion,
				ReportHash:      report.Hash,
				StatementIndex:  statement.Index,
				Kind:            statement.Kind,
				Table:           statement.Table,
				ObservationKind: "statement_rule",
				Status:          rule.Verdict,
				RuleID:          rule.ID,
				Severity:        rule.Severity,
				Verdict:         rule.Verdict,
				Evidence:        rule.Evidence,
			})
		}
		if statement.Lock.Mode != "" {
			observations = append(observations, EngineBehaviorObservation{
				ID:              observationID(report, "lock", statement.Lock.Mode, statement.Index),
				Engine:          report.Profile.Engine,
				ResolvedVersion: report.Profile.ResolvedVersion,
				ReportHash:      report.Hash,
				StatementIndex:  statement.Index,
				Kind:            statement.Kind,
				Table:           statement.Table,
				ObservationKind: "lock_simulation",
				Status:          "modeled",
				LockMode:        statement.Lock.Mode,
				DurationClass:   statement.Lock.DurationClass,
				Evidence:        lockObservationEvidence(statement.Lock),
			})
			if statement.Lock.ContainerSmoke.ID != "" {
				observations = append(observations, EngineBehaviorObservation{
					ID:              observationID(report, "container-smoke", statement.Lock.ContainerSmoke.ID, statement.Index),
					Engine:          report.Profile.Engine,
					ResolvedVersion: report.Profile.ResolvedVersion,
					ReportHash:      report.Hash,
					StatementIndex:  statement.Index,
					Kind:            statement.Kind,
					Table:           statement.Table,
					ObservationKind: "container_smoke_fixture",
					Status:          statement.Lock.ContainerSmoke.Status,
					Reference:       statement.Lock.ContainerSmoke.Image,
					Evidence:        statement.Lock.ContainerSmoke.Observation + " via " + statement.Lock.ContainerSmoke.Command,
				})
			}
		}
		for _, control := range statement.NegativeControls {
			observations = append(observations, EngineBehaviorObservation{
				ID:              observationID(report, "negative-control", control.ID, statement.Index),
				Engine:          report.Profile.Engine,
				ResolvedVersion: report.Profile.ResolvedVersion,
				ReportHash:      report.Hash,
				StatementIndex:  statement.Index,
				Kind:            statement.Kind,
				Table:           statement.Table,
				ObservationKind: "engine_negative_control",
				Status:          control.ControlVerdict,
				RuleID:          control.ControlRule,
				Severity:        control.ControlRisk,
				Verdict:         control.ControlVerdict,
				Evidence:        control.Evidence,
			})
		}
		if rollback := statement.RollbackFeasibility; rollback != nil {
			observations = append(observations, EngineBehaviorObservation{
				ID:              observationID(report, "rollback", rollback.Class, statement.Index),
				Engine:          report.Profile.Engine,
				ResolvedVersion: report.Profile.ResolvedVersion,
				ReportHash:      report.Hash,
				StatementIndex:  statement.Index,
				Kind:            statement.Kind,
				Table:           statement.Table,
				ObservationKind: "rollback_feasibility",
				Status:          rollback.Status,
				RuleID:          "rollback." + rollback.Class,
				Verdict:         rollback.Status,
				Evidence:        rollback.RecoveryMechanism,
			})
		}
		if queryPlan := statement.QueryPlanRegression; queryPlan != nil {
			observations = append(observations, EngineBehaviorObservation{
				ID:              observationID(report, "query-plan", queryPlan.Class, statement.Index),
				Engine:          report.Profile.Engine,
				ResolvedVersion: report.Profile.ResolvedVersion,
				ReportHash:      report.Hash,
				StatementIndex:  statement.Index,
				Kind:            statement.Kind,
				Table:           statement.Table,
				ObservationKind: "query_plan_regression",
				Status:          queryPlan.Status,
				RuleID:          "query_plan." + queryPlan.Class,
				Verdict:         queryPlan.Status,
				Evidence:        queryPlan.ChangeKind + " workloads=" + strconv.Itoa(len(queryPlan.RepresentativeWorkloads)),
			})
		}
		if runtime := statement.RuntimeEstimate; runtime != nil {
			observations = append(observations, EngineBehaviorObservation{
				ID:              observationID(report, "runtime", runtime.Class, statement.Index),
				Engine:          report.Profile.Engine,
				ResolvedVersion: report.Profile.ResolvedVersion,
				ReportHash:      report.Hash,
				StatementIndex:  statement.Index,
				Kind:            statement.Kind,
				Table:           statement.Table,
				ObservationKind: "runtime_estimate",
				Status:          runtime.Severity,
				RuleID:          "runtime." + runtime.Class,
				Severity:        runtime.Severity,
				Evidence:        runtime.SourceKind + " rows<=" + strconv.FormatInt(runtime.RowsUpperBound, 10),
			})
		}
		if lag := statement.ReplicationLagRisk; lag != nil {
			observations = append(observations, EngineBehaviorObservation{
				ID:              observationID(report, "replication-lag", lag.MigrationShape, statement.Index),
				Engine:          report.Profile.Engine,
				ResolvedVersion: report.Profile.ResolvedVersion,
				ReportHash:      report.Hash,
				StatementIndex:  statement.Index,
				Kind:            statement.Kind,
				Table:           statement.Table,
				ObservationKind: "replication_lag_risk",
				Status:          lag.Class,
				RuleID:          "replication_lag." + lag.MigrationShape,
				Severity:        lag.Class,
				Evidence:        lag.EstimatedLagClass,
			})
		}
		if partition := statement.PartitionSharding; partition != nil {
			observations = append(observations, EngineBehaviorObservation{
				ID:              observationID(report, "partition-sharding", partition.Operation, statement.Index),
				Engine:          report.Profile.Engine,
				ResolvedVersion: report.Profile.ResolvedVersion,
				ReportHash:      report.Hash,
				StatementIndex:  statement.Index,
				Kind:            statement.Kind,
				Table:           statement.Table,
				ObservationKind: "partition_sharding",
				Status:          partition.Class,
				RuleID:          "partition_sharding." + partition.Operation,
				Severity:        partition.Class,
				Evidence:        partition.AffectedScope,
			})
		}
	}
	sort.Slice(observations, func(i, j int) bool { return observations[i].ID < observations[j].ID })
	return observations
}

func lockObservationEvidence(lock LockSimulation) string {
	return lock.Mode + " " + lock.DurationClass + " readers=" + strconv.FormatBool(lock.BlocksReaders) +
		" writers=" + strconv.FormatBool(lock.BlocksWriters) + " ddl=" + strconv.FormatBool(lock.BlocksDDL)
}

func evidenceRefs(evidence []Evidence) []string {
	refs := make([]string, 0, len(evidence))
	for _, item := range evidence {
		refs = append(refs, item.Ref)
	}
	sort.Strings(refs)
	return refs
}

func observationID(report Report, kind, key string, statementIndex int) string {
	parts := []string{"dbsem", string(report.Profile.Engine), report.Profile.ResolvedVersion, kind}
	if statementIndex >= 0 {
		parts = append(parts, strconv.Itoa(statementIndex))
	}
	parts = append(parts, normalizeObservationKey(key))
	return strings.Join(parts, ".")
}

func normalizeObservationKey(value string) string {
	var b strings.Builder
	lastDash := false
	for _, r := range strings.ToLower(value) {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash {
			b.WriteByte('-')
			lastDash = true
		}
	}
	return strings.Trim(b.String(), "-")
}

func sourceKey(report Report) string {
	return string(report.Profile.Engine) + "\x00" + report.Profile.ResolvedVersion + "\x00" + report.Source + "\x00" + report.Hash
}

func finalizeReproducibilityReport(report *ReproducibilityReport) {
	report.Summary.Reports = len(report.SourceReport)
	report.Summary.EngineVersionPins = len(report.EnginePins)
	engines := map[Engine]bool{}
	for _, pin := range report.EnginePins {
		engines[pin.Engine] = true
		if pin.ContainerImage != "" {
			report.Summary.ContainerImages++
		}
		if pin.LocalExecutionAvailable {
			report.Summary.LocalExecutionRuntimes++
		}
		if pin.RuntimeKind == "managed-service-profile" || pin.RuntimeKind == "embedded-engine" || pin.RuntimeKind == "managed-service-emulator" {
			report.Summary.ManagedOrEmbeddedProfiles++
		}
	}
	report.Summary.Engines = len(engines)
	for _, source := range report.SourceReport {
		report.Summary.Statements += source.Statements
	}
	for _, observation := range report.Observations {
		report.Summary.Observations++
		switch observation.ObservationKind {
		case "profile_evidence":
			report.Summary.ProfileEvidence++
		case "statement_rule":
			report.Summary.StatementRules++
		case "lock_simulation":
			report.Summary.LockSimulations++
		case "container_smoke_fixture":
			report.Summary.ContainerSmokeFixtures++
		case "engine_negative_control":
			report.Summary.EngineNegativeControls++
		case "rollback_feasibility":
			report.Summary.RollbackChecks++
		case "query_plan_regression":
			report.Summary.QueryPlanChecks++
		case "runtime_estimate":
			report.Summary.RuntimeEstimates++
		case "replication_lag_risk":
			report.Summary.ReplicationLagRisks++
		case "partition_sharding":
			report.Summary.PartitionShardingFindings++
		}
	}
}

func reproducibilityReportHash(report ReproducibilityReport) string {
	report.Hash = ""
	return canonical.Hash(report)
}
