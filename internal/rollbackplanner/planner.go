package rollbackplanner

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/thehalleyyoung/patchline/internal/canonical"
)

const SpecVersion = "patchline.multi-service-rollback-plan/v1"
const ReportVersion = "patchline.multi-service-rollback-plan-report/v1"

type Spec struct {
	Version         string          `json:"version"`
	Name            string          `json:"name"`
	DependencyBound DependencyBound `json:"dependency_bound"`
	DataLossBound   DataLossBound   `json:"data_loss_bound"`
	Services        []Service       `json:"services,omitempty"`
	Migrations      []Migration     `json:"migrations,omitempty"`
}

type DependencyBound struct {
	MaxDepth  int `json:"max_depth"`
	MaxFanout int `json:"max_fanout"`
	MaxWaves  int `json:"max_waves,omitempty"`
}

type DataLossBound struct {
	MaxRows             int `json:"max_rows"`
	MaxCriticalRows     int `json:"max_critical_rows"`
	MaxAffectedServices int `json:"max_affected_services"`
}

type Service struct {
	ID                 string   `json:"id"`
	Name               string   `json:"name,omitempty"`
	Owners             []string `json:"owners,omitempty"`
	Criticality        string   `json:"criticality,omitempty"`
	UpstreamServices   []string `json:"upstream_services,omitempty"`
	DownstreamServices []string `json:"downstream_services,omitempty"`
}

type Migration struct {
	ID                 string   `json:"id"`
	ServiceID          string   `json:"service_id"`
	Stage              string   `json:"stage,omitempty"`
	Kind               string   `json:"kind,omitempty"`
	Operation          string   `json:"operation,omitempty"`
	DataClasses        []string `json:"data_classes,omitempty"`
	EstimatedRows      int      `json:"estimated_rows,omitempty"`
	CriticalRows       int      `json:"critical_rows,omitempty"`
	RollbackAction     string   `json:"rollback_action,omitempty"`
	RollbackVerified   bool     `json:"rollback_verified,omitempty"`
	Irreversible       bool     `json:"irreversible,omitempty"`
	DependsOn          []string `json:"depends_on,omitempty"`
	UpstreamServices   []string `json:"upstream_services,omitempty"`
	DownstreamServices []string `json:"downstream_services,omitempty"`
}

type Report struct {
	Version         string           `json:"version"`
	Name            string           `json:"name"`
	OK              bool             `json:"ok"`
	Summary         Summary          `json:"summary"`
	DependencyProof DependencyProof  `json:"dependency_proof"`
	DataLossProof   DataLossProof    `json:"data_loss_proof"`
	Services        []ServiceReport  `json:"services,omitempty"`
	RollbackWaves   []RollbackWave   `json:"rollback_waves,omitempty"`
	Obligations     []Obligation     `json:"obligations,omitempty"`
	Counterexamples []Counterexample `json:"counterexamples,omitempty"`
	Hash            string           `json:"hash"`
}

type Summary struct {
	Services             int `json:"services"`
	Migrations           int `json:"migrations"`
	RollbackSteps        int `json:"rollback_steps"`
	RollbackWaves        int `json:"rollback_waves"`
	DependencyDepth      int `json:"dependency_depth"`
	DependencyFanout     int `json:"dependency_fanout"`
	DataLossRows         int `json:"data_loss_rows"`
	CriticalDataLossRows int `json:"critical_data_loss_rows"`
	AffectedServices     int `json:"affected_services"`
	Obligations          int `json:"obligations"`
	Counterexamples      int `json:"counterexamples"`
}

type DependencyProof struct {
	Status            string             `json:"status"`
	ForwardOrder      []string           `json:"forward_order,omitempty"`
	RollbackOrder     []string           `json:"rollback_order,omitempty"`
	LongestPath       []string           `json:"longest_path,omitempty"`
	MaxDepth          int                `json:"max_depth"`
	MaxFanout         int                `json:"max_fanout"`
	MaxWaves          int                `json:"max_waves"`
	DeclaredBound     DependencyBound    `json:"declared_bound"`
	CrossServiceEdges []CrossServiceEdge `json:"cross_service_edges,omitempty"`
	Counterexamples   []Counterexample   `json:"counterexamples,omitempty"`
}

type CrossServiceEdge struct {
	FromMigration string `json:"from_migration"`
	ToMigration   string `json:"to_migration"`
	FromService   string `json:"from_service"`
	ToService     string `json:"to_service"`
	Declared      bool   `json:"declared"`
}

type DataLossProof struct {
	Status           string           `json:"status"`
	Bounds           DataLossBound    `json:"bounds"`
	Rows             int              `json:"rows"`
	CriticalRows     int              `json:"critical_rows"`
	AffectedServices []string         `json:"affected_services,omitempty"`
	Losses           []DataLossEntry  `json:"losses,omitempty"`
	Counterexamples  []Counterexample `json:"counterexamples,omitempty"`
}

type DataLossEntry struct {
	MigrationID  string   `json:"migration_id"`
	ServiceID    string   `json:"service_id"`
	Reason       string   `json:"reason"`
	Rows         int      `json:"rows"`
	CriticalRows int      `json:"critical_rows"`
	DataClasses  []string `json:"data_classes,omitempty"`
}

type ServiceReport struct {
	ID                   string   `json:"id"`
	Name                 string   `json:"name,omitempty"`
	Owners               []string `json:"owners,omitempty"`
	Criticality          string   `json:"criticality,omitempty"`
	Migrations           int      `json:"migrations"`
	RollbackSteps        int      `json:"rollback_steps"`
	DataLossRows         int      `json:"data_loss_rows"`
	CriticalDataLossRows int      `json:"critical_data_loss_rows"`
	Status               string   `json:"status"`
}

type RollbackWave struct {
	Wave  int            `json:"wave"`
	Steps []RollbackStep `json:"steps"`
}

type RollbackStep struct {
	Order                int      `json:"order"`
	Wave                 int      `json:"wave"`
	MigrationID          string   `json:"migration_id"`
	ServiceID            string   `json:"service_id"`
	Stage                string   `json:"stage,omitempty"`
	Kind                 string   `json:"kind,omitempty"`
	Operation            string   `json:"operation,omitempty"`
	RollbackAction       string   `json:"rollback_action,omitempty"`
	RollbackVerified     bool     `json:"rollback_verified"`
	Irreversible         bool     `json:"irreversible,omitempty"`
	DependsOn            []string `json:"depends_on,omitempty"`
	DataClasses          []string `json:"data_classes,omitempty"`
	EstimatedRows        int      `json:"estimated_rows,omitempty"`
	CriticalRows         int      `json:"critical_rows,omitempty"`
	DataLossRows         int      `json:"data_loss_rows,omitempty"`
	CriticalDataLossRows int      `json:"critical_data_loss_rows,omitempty"`
}

type Obligation struct {
	ID      string `json:"id"`
	Status  string `json:"status"`
	Formula string `json:"formula"`
	Reason  string `json:"reason"`
}

type Counterexample struct {
	ID      string   `json:"id"`
	Kind    string   `json:"kind"`
	Subject string   `json:"subject,omitempty"`
	Message string   `json:"message"`
	Witness []string `json:"witness,omitempty"`
}

func ReadSpec(reader io.Reader) (Spec, error) {
	decoder := json.NewDecoder(reader)
	decoder.DisallowUnknownFields()
	var spec Spec
	if err := decoder.Decode(&spec); err != nil {
		return Spec{}, err
	}
	if spec.Version != SpecVersion {
		return Spec{}, fmt.Errorf("multi-service rollback plan spec version must be %s", SpecVersion)
	}
	return spec, nil
}

func BuildReport(spec Spec) (Report, error) {
	if err := validateSpec(spec); err != nil {
		return Report{}, err
	}
	services := servicesByID(spec.Services)
	migrations := migrationsByID(spec.Migrations)
	edges, unknownDeps := dependencyEdges(spec.Migrations, migrations)
	cycle := dependencyCycle(spec.Migrations, edges)

	dependencyProof := buildDependencyProof(spec, services, migrations, edges, unknownDeps, cycle)
	dataLossProof := buildDataLossProof(spec)
	steps, waves := buildRollbackPlan(spec, dependencyProof)
	obligations := buildObligations(spec, dependencyProof, dataLossProof)
	counterexamples := append([]Counterexample{}, dependencyProof.Counterexamples...)
	counterexamples = append(counterexamples, dataLossProof.Counterexamples...)
	sortCounterexamples(counterexamples)
	report := Report{
		Version:         ReportVersion,
		Name:            spec.Name,
		OK:              obligationsChecked(obligations),
		DependencyProof: dependencyProof,
		DataLossProof:   dataLossProof,
		Services:        buildServiceReports(spec, steps, dataLossProof, counterexamples),
		RollbackWaves:   waves,
		Obligations:     obligations,
		Counterexamples: counterexamples,
	}
	report.Summary = summarizeReport(spec, report, steps)
	report.Hash = reportHash(report)
	return report, nil
}

func WriteArtifacts(outDir string, report Report) error {
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}
	if err := writeJSON(filepath.Join(outDir, "multi-service-rollback-plan.json"), report); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(outDir, "multi-service-rollback-plan.md"), []byte(RenderMarkdown(report)), 0o644)
}

func RenderMarkdown(report Report) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Verified multi-service rollback planner\n\n")
	fmt.Fprintf(&b, "Patchline derives rollback order from migration dependencies only, then proves that the reverse plan fits explicit dependency and data-loss bounds.\n\n")
	fmt.Fprintf(&b, "## Summary\n\n")
	fmt.Fprintf(&b, "| Metric | Value |\n| --- | ---: |\n")
	fmt.Fprintf(&b, "| OK | `%t` |\n", report.OK)
	fmt.Fprintf(&b, "| Services | %d |\n", report.Summary.Services)
	fmt.Fprintf(&b, "| Migrations | %d |\n", report.Summary.Migrations)
	fmt.Fprintf(&b, "| Rollback waves | %d |\n", report.Summary.RollbackWaves)
	fmt.Fprintf(&b, "| Dependency depth | %d |\n", report.Summary.DependencyDepth)
	fmt.Fprintf(&b, "| Dependency fanout | %d |\n", report.Summary.DependencyFanout)
	fmt.Fprintf(&b, "| Data-loss rows | %d |\n", report.Summary.DataLossRows)
	fmt.Fprintf(&b, "| Critical data-loss rows | %d |\n", report.Summary.CriticalDataLossRows)
	fmt.Fprintf(&b, "| Counterexamples | %d |\n\n", report.Summary.Counterexamples)

	fmt.Fprintf(&b, "## Dependency proof\n\n")
	fmt.Fprintf(&b, "Status: `%s`. Depth is counted as nodes on the longest dependency path; fanout is the maximum number of direct downstream dependents.\n\n", report.DependencyProof.Status)
	fmt.Fprintf(&b, "- forward order: `%s`\n", strings.Join(report.DependencyProof.ForwardOrder, ", "))
	fmt.Fprintf(&b, "- rollback order: `%s`\n", strings.Join(report.DependencyProof.RollbackOrder, ", "))
	fmt.Fprintf(&b, "- longest path: `%s`\n\n", strings.Join(report.DependencyProof.LongestPath, " -> "))

	fmt.Fprintf(&b, "## Rollback waves\n\n")
	fmt.Fprintf(&b, "| Wave | Order | Migration | Service | Action | Data-loss rows |\n| ---: | ---: | --- | --- | --- | ---: |\n")
	for _, wave := range report.RollbackWaves {
		for _, step := range wave.Steps {
			fmt.Fprintf(&b, "| %d | %d | `%s` | `%s` | %s | %d |\n", step.Wave, step.Order, step.MigrationID, step.ServiceID, firstNonEmpty(step.RollbackAction, "<bounded data loss>"), step.DataLossRows)
		}
	}

	fmt.Fprintf(&b, "\n## Service handoffs\n\n")
	fmt.Fprintf(&b, "| Service | Owners | Status | Steps | Data-loss rows |\n| --- | --- | --- | ---: | ---: |\n")
	for _, service := range report.Services {
		owners := "-"
		if len(service.Owners) > 0 {
			owners = strings.Join(service.Owners, ", ")
		}
		fmt.Fprintf(&b, "| `%s` | %s | `%s` | %d | %d |\n", service.ID, owners, service.Status, service.RollbackSteps, service.DataLossRows)
	}

	if len(report.Counterexamples) > 0 {
		fmt.Fprintf(&b, "\n## Counterexamples\n\n")
		fmt.Fprintf(&b, "| ID | Kind | Subject | Message |\n| --- | --- | --- | --- |\n")
		for _, counterexample := range report.Counterexamples {
			fmt.Fprintf(&b, "| `%s` | `%s` | `%s` | %s |\n", counterexample.ID, counterexample.Kind, firstNonEmpty(counterexample.Subject, "-"), counterexample.Message)
		}
	}
	return b.String()
}

func validateSpec(spec Spec) error {
	if spec.Version != SpecVersion {
		return fmt.Errorf("multi-service rollback plan spec version must be %s", SpecVersion)
	}
	if spec.Name == "" {
		return fmt.Errorf("spec name is required")
	}
	if spec.DependencyBound.MaxDepth < 0 || spec.DependencyBound.MaxFanout < 0 || spec.DependencyBound.MaxWaves < 0 {
		return fmt.Errorf("dependency bounds must be non-negative")
	}
	if spec.DataLossBound.MaxRows < 0 || spec.DataLossBound.MaxCriticalRows < 0 || spec.DataLossBound.MaxAffectedServices < 0 {
		return fmt.Errorf("data-loss bounds must be non-negative")
	}
	serviceIDs := map[string]bool{}
	for _, service := range spec.Services {
		if service.ID == "" {
			return fmt.Errorf("service id is required")
		}
		if serviceIDs[service.ID] {
			return fmt.Errorf("service id %q is duplicated", service.ID)
		}
		serviceIDs[service.ID] = true
	}
	migrationIDs := map[string]bool{}
	for _, migration := range spec.Migrations {
		if migration.ID == "" {
			return fmt.Errorf("migration id is required")
		}
		if migrationIDs[migration.ID] {
			return fmt.Errorf("migration id %q is duplicated", migration.ID)
		}
		migrationIDs[migration.ID] = true
		if migration.ServiceID == "" {
			return fmt.Errorf("migration %q service_id is required", migration.ID)
		}
		if !serviceIDs[migration.ServiceID] {
			return fmt.Errorf("migration %q references unknown service %q", migration.ID, migration.ServiceID)
		}
		if migration.EstimatedRows < 0 || migration.CriticalRows < 0 {
			return fmt.Errorf("migration %q row estimates must be non-negative", migration.ID)
		}
		if migration.CriticalRows > migration.EstimatedRows {
			return fmt.Errorf("migration %q critical_rows cannot exceed estimated_rows", migration.ID)
		}
	}
	if len(spec.Migrations) > 0 && spec.DependencyBound.MaxDepth == 0 {
		return fmt.Errorf("non-empty plans must declare a positive dependency max_depth bound")
	}
	return nil
}

func buildDependencyProof(spec Spec, services map[string]Service, migrations map[string]Migration, edges map[string][]string, unknownDeps []Counterexample, cycle []string) DependencyProof {
	proof := DependencyProof{
		Status:        "checked",
		DeclaredBound: spec.DependencyBound,
	}
	proof.Counterexamples = append(proof.Counterexamples, unknownDeps...)
	if len(cycle) > 0 {
		proof.Counterexamples = append(proof.Counterexamples, Counterexample{
			ID:      "dependency.cycle",
			Kind:    "dependency",
			Subject: strings.Join(cycle, " -> "),
			Message: "migration dependency graph contains a cycle",
			Witness: cycle,
		})
	}
	proof.CrossServiceEdges = crossServiceEdges(spec.Migrations, services, migrations)
	for _, edge := range proof.CrossServiceEdges {
		if !edge.Declared {
			proof.Counterexamples = append(proof.Counterexamples, Counterexample{
				ID:      "dependency.cross_service_metadata." + edge.ToMigration,
				Kind:    "dependency_metadata",
				Subject: edge.FromService + " -> " + edge.ToService,
				Message: "cross-service migration dependency is not declared in service or migration upstream/downstream metadata",
				Witness: []string{edge.FromMigration, edge.ToMigration},
			})
		}
	}

	if len(proof.Counterexamples) == 0 {
		proof.ForwardOrder = topoOrder(spec.Migrations, edges)
		proof.RollbackOrder = reverseStrings(proof.ForwardOrder)
		proof.LongestPath, proof.MaxDepth = longestPath(proof.ForwardOrder, edges)
		proof.MaxFanout = maxFanout(edges)
		proof.MaxWaves = proof.MaxDepth
		if proof.MaxDepth > spec.DependencyBound.MaxDepth {
			proof.Counterexamples = append(proof.Counterexamples, Counterexample{
				ID:      "dependency.bound.depth",
				Kind:    "dependency_bound",
				Subject: fmt.Sprintf("%d > %d", proof.MaxDepth, spec.DependencyBound.MaxDepth),
				Message: "longest dependency path exceeds declared max_depth",
				Witness: proof.LongestPath,
			})
		}
		if proof.MaxFanout > spec.DependencyBound.MaxFanout {
			proof.Counterexamples = append(proof.Counterexamples, Counterexample{
				ID:      "dependency.bound.fanout",
				Kind:    "dependency_bound",
				Subject: fmt.Sprintf("%d > %d", proof.MaxFanout, spec.DependencyBound.MaxFanout),
				Message: "dependency fanout exceeds declared max_fanout",
			})
		}
		if spec.DependencyBound.MaxWaves > 0 && proof.MaxWaves > spec.DependencyBound.MaxWaves {
			proof.Counterexamples = append(proof.Counterexamples, Counterexample{
				ID:      "dependency.bound.waves",
				Kind:    "dependency_bound",
				Subject: fmt.Sprintf("%d > %d", proof.MaxWaves, spec.DependencyBound.MaxWaves),
				Message: "rollback waves exceed declared max_waves",
			})
		}
	} else {
		proof.ForwardOrder = sortedMigrationIDs(spec.Migrations)
		proof.RollbackOrder = reverseStrings(proof.ForwardOrder)
		proof.MaxFanout = maxFanout(edges)
	}
	sortCounterexamples(proof.Counterexamples)
	if len(proof.Counterexamples) > 0 {
		proof.Status = "refuted"
	}
	return proof
}

func buildDataLossProof(spec Spec) DataLossProof {
	proof := DataLossProof{
		Status: "checked",
		Bounds: spec.DataLossBound,
	}
	affected := map[string]bool{}
	for _, migration := range sortedMigrations(spec.Migrations) {
		if migration.Irreversible {
			entry := dataLossEntry(migration, "irreversible migration is bounded as explicit data loss")
			proof.Losses = append(proof.Losses, entry)
			proof.Rows += entry.Rows
			proof.CriticalRows += entry.CriticalRows
			if entry.Rows > 0 || entry.CriticalRows > 0 {
				affected[migration.ServiceID] = true
			}
			continue
		}
		if !migration.RollbackVerified || strings.TrimSpace(migration.RollbackAction) == "" {
			proof.Counterexamples = append(proof.Counterexamples, Counterexample{
				ID:      "data_loss.rollback_unverified." + migration.ID,
				Kind:    "rollback_verification",
				Subject: migration.ID,
				Message: "migration with reversible semantics must have rollback_verified=true and a non-empty rollback_action",
			})
			entry := dataLossEntry(migration, "rollback is not verified")
			proof.Losses = append(proof.Losses, entry)
			proof.Rows += entry.Rows
			proof.CriticalRows += entry.CriticalRows
			if entry.Rows > 0 || entry.CriticalRows > 0 {
				affected[migration.ServiceID] = true
			}
		}
	}
	proof.AffectedServices = sortedKeys(affected)
	if proof.Rows > spec.DataLossBound.MaxRows {
		proof.Counterexamples = append(proof.Counterexamples, Counterexample{
			ID:      "data_loss.bound.rows",
			Kind:    "data_loss_bound",
			Subject: fmt.Sprintf("%d > %d", proof.Rows, spec.DataLossBound.MaxRows),
			Message: "estimated rollback data loss exceeds declared max_rows",
		})
	}
	if proof.CriticalRows > spec.DataLossBound.MaxCriticalRows {
		proof.Counterexamples = append(proof.Counterexamples, Counterexample{
			ID:      "data_loss.bound.critical_rows",
			Kind:    "data_loss_bound",
			Subject: fmt.Sprintf("%d > %d", proof.CriticalRows, spec.DataLossBound.MaxCriticalRows),
			Message: "critical rollback data loss exceeds declared max_critical_rows",
		})
	}
	if len(proof.AffectedServices) > spec.DataLossBound.MaxAffectedServices {
		proof.Counterexamples = append(proof.Counterexamples, Counterexample{
			ID:      "data_loss.bound.affected_services",
			Kind:    "data_loss_bound",
			Subject: fmt.Sprintf("%d > %d", len(proof.AffectedServices), spec.DataLossBound.MaxAffectedServices),
			Message: "affected services with data loss exceed declared max_affected_services",
			Witness: proof.AffectedServices,
		})
	}
	sortCounterexamples(proof.Counterexamples)
	if len(proof.Counterexamples) > 0 {
		proof.Status = "refuted"
	} else if proof.Rows > 0 || proof.CriticalRows > 0 {
		proof.Status = "bounded_data_loss"
	}
	return proof
}

func buildRollbackPlan(spec Spec, proof DependencyProof) ([]RollbackStep, []RollbackWave) {
	migrations := migrationsByID(spec.Migrations)
	levelByID := migrationLevels(proof.ForwardOrder, dependencyEdgesOnly(spec.Migrations, migrations))
	maxLevel := 0
	for _, level := range levelByID {
		if level > maxLevel {
			maxLevel = level
		}
	}
	if maxLevel == 0 && len(proof.RollbackOrder) > 0 {
		maxLevel = 1
	}
	var steps []RollbackStep
	for i, id := range proof.RollbackOrder {
		migration, ok := migrations[id]
		if !ok {
			continue
		}
		lossRows, criticalRows := 0, 0
		if migration.Irreversible || !migration.RollbackVerified || strings.TrimSpace(migration.RollbackAction) == "" {
			lossRows = migration.EstimatedRows
			criticalRows = migration.CriticalRows
		}
		level := levelByID[id]
		if level == 0 {
			level = 1
		}
		wave := maxLevel - level + 1
		steps = append(steps, RollbackStep{
			Order:                i + 1,
			Wave:                 wave,
			MigrationID:          migration.ID,
			ServiceID:            migration.ServiceID,
			Stage:                migration.Stage,
			Kind:                 migration.Kind,
			Operation:            migration.Operation,
			RollbackAction:       migration.RollbackAction,
			RollbackVerified:     migration.RollbackVerified,
			Irreversible:         migration.Irreversible,
			DependsOn:            sortedStrings(migration.DependsOn),
			DataClasses:          sortedStrings(migration.DataClasses),
			EstimatedRows:        migration.EstimatedRows,
			CriticalRows:         migration.CriticalRows,
			DataLossRows:         lossRows,
			CriticalDataLossRows: criticalRows,
		})
	}
	sort.SliceStable(steps, func(i, j int) bool {
		if steps[i].Wave != steps[j].Wave {
			return steps[i].Wave < steps[j].Wave
		}
		return steps[i].Order < steps[j].Order
	})
	byWave := map[int][]RollbackStep{}
	for _, step := range steps {
		byWave[step.Wave] = append(byWave[step.Wave], step)
	}
	var waveIDs []int
	for wave := range byWave {
		waveIDs = append(waveIDs, wave)
	}
	sort.Ints(waveIDs)
	var waves []RollbackWave
	for _, wave := range waveIDs {
		waves = append(waves, RollbackWave{Wave: wave, Steps: byWave[wave]})
	}
	return steps, waves
}

func buildObligations(spec Spec, dependencyProof DependencyProof, dataLossProof DataLossProof) []Obligation {
	status := func(ok bool) string {
		if ok {
			return "checked"
		}
		return "refuted"
	}
	obligations := []Obligation{{
		ID:      "dependency.acyclic",
		Status:  status(!hasCounterexampleKind(dependencyProof.Counterexamples, "dependency")),
		Formula: "migration depends_on graph is acyclic and references known migrations",
		Reason:  "rollback order is the reverse of a deterministic topological sort",
	}, {
		ID:      "dependency.bounds.depth",
		Status:  status(!hasCounterexampleID(dependencyProof.Counterexamples, "dependency.bound.depth")),
		Formula: fmt.Sprintf("actual_depth=%d <= declared_max_depth=%d", dependencyProof.MaxDepth, spec.DependencyBound.MaxDepth),
		Reason:  "depth is counted as migrations on the longest dependency path",
	}, {
		ID:      "dependency.bounds.fanout",
		Status:  status(!hasCounterexampleID(dependencyProof.Counterexamples, "dependency.bound.fanout")),
		Formula: fmt.Sprintf("actual_fanout=%d <= declared_max_fanout=%d", dependencyProof.MaxFanout, spec.DependencyBound.MaxFanout),
		Reason:  "fanout bounds simultaneous downstream rollback pressure",
	}, {
		ID:      "dependency.metadata.cross_service",
		Status:  status(!hasCounterexampleKind(dependencyProof.Counterexamples, "dependency_metadata")),
		Formula: "cross-service depends_on edges are declared in service or migration metadata",
		Reason:  "service handoffs stay auditable without making service metadata an ordering source",
	}, {
		ID:      "data_loss.rollback_verified",
		Status:  status(!hasCounterexampleKind(dataLossProof.Counterexamples, "rollback_verification")),
		Formula: "each reversible migration has rollback_verified=true and rollback_action",
		Reason:  "unverified reversible steps are treated as bounded data-loss counterexamples",
	}, {
		ID:      "data_loss.bounds.rows",
		Status:  status(!hasCounterexampleID(dataLossProof.Counterexamples, "data_loss.bound.rows")),
		Formula: fmt.Sprintf("data_loss_rows=%d <= max_rows=%d", dataLossProof.Rows, spec.DataLossBound.MaxRows),
		Reason:  "rollback planner sums rows for irreversible or unverified steps",
	}, {
		ID:      "data_loss.bounds.critical_rows",
		Status:  status(!hasCounterexampleID(dataLossProof.Counterexamples, "data_loss.bound.critical_rows")),
		Formula: fmt.Sprintf("critical_data_loss_rows=%d <= max_critical_rows=%d", dataLossProof.CriticalRows, spec.DataLossBound.MaxCriticalRows),
		Reason:  "critical rows have a separate, stricter bound",
	}, {
		ID:      "data_loss.bounds.services",
		Status:  status(!hasCounterexampleID(dataLossProof.Counterexamples, "data_loss.bound.affected_services")),
		Formula: fmt.Sprintf("affected_services=%d <= max_affected_services=%d", len(dataLossProof.AffectedServices), spec.DataLossBound.MaxAffectedServices),
		Reason:  "multi-service plans must cap how many services lose data",
	}}
	sort.SliceStable(obligations, func(i, j int) bool { return obligations[i].ID < obligations[j].ID })
	return obligations
}

func buildServiceReports(spec Spec, steps []RollbackStep, dataLossProof DataLossProof, counterexamples []Counterexample) []ServiceReport {
	stepCounts := map[string]int{}
	migrationCounts := map[string]int{}
	lossRows := map[string]int{}
	criticalRows := map[string]int{}
	serviceRefuted := map[string]bool{}
	for _, migration := range spec.Migrations {
		migrationCounts[migration.ServiceID]++
	}
	for _, step := range steps {
		stepCounts[step.ServiceID]++
	}
	for _, loss := range dataLossProof.Losses {
		lossRows[loss.ServiceID] += loss.Rows
		criticalRows[loss.ServiceID] += loss.CriticalRows
	}
	for _, counterexample := range counterexamples {
		for _, migration := range spec.Migrations {
			if counterexample.Subject == migration.ID || stringSliceContains(counterexample.Witness, migration.ID) {
				serviceRefuted[migration.ServiceID] = true
			}
		}
	}
	var reports []ServiceReport
	for _, service := range sortedServices(spec.Services) {
		status := "checked"
		if lossRows[service.ID] > 0 || criticalRows[service.ID] > 0 {
			status = "bounded_data_loss"
		}
		if serviceRefuted[service.ID] {
			status = "refuted"
		}
		reports = append(reports, ServiceReport{
			ID:                   service.ID,
			Name:                 service.Name,
			Owners:               sortedStrings(service.Owners),
			Criticality:          service.Criticality,
			Migrations:           migrationCounts[service.ID],
			RollbackSteps:        stepCounts[service.ID],
			DataLossRows:         lossRows[service.ID],
			CriticalDataLossRows: criticalRows[service.ID],
			Status:               status,
		})
	}
	return reports
}

func summarizeReport(spec Spec, report Report, steps []RollbackStep) Summary {
	return Summary{
		Services:             len(spec.Services),
		Migrations:           len(spec.Migrations),
		RollbackSteps:        len(steps),
		RollbackWaves:        len(report.RollbackWaves),
		DependencyDepth:      report.DependencyProof.MaxDepth,
		DependencyFanout:     report.DependencyProof.MaxFanout,
		DataLossRows:         report.DataLossProof.Rows,
		CriticalDataLossRows: report.DataLossProof.CriticalRows,
		AffectedServices:     len(report.DataLossProof.AffectedServices),
		Obligations:          len(report.Obligations),
		Counterexamples:      len(report.Counterexamples),
	}
}

func dependencyEdges(migrations []Migration, byID map[string]Migration) (map[string][]string, []Counterexample) {
	edges := map[string][]string{}
	var counterexamples []Counterexample
	for _, migration := range migrations {
		edges[migration.ID] = nil
	}
	for _, migration := range migrations {
		for _, dep := range sortedStrings(migration.DependsOn) {
			if _, ok := byID[dep]; !ok {
				counterexamples = append(counterexamples, Counterexample{
					ID:      "dependency.unknown." + migration.ID + "." + dep,
					Kind:    "dependency",
					Subject: migration.ID,
					Message: "migration depends on an unknown migration",
					Witness: []string{dep},
				})
				continue
			}
			edges[dep] = append(edges[dep], migration.ID)
		}
	}
	for id := range edges {
		edges[id] = sortedStrings(edges[id])
	}
	sortCounterexamples(counterexamples)
	return edges, counterexamples
}

func dependencyEdgesOnly(migrations []Migration, byID map[string]Migration) map[string][]string {
	edges, _ := dependencyEdges(migrations, byID)
	return edges
}

func topoOrder(migrations []Migration, edges map[string][]string) []string {
	ids := sortedMigrationIDs(migrations)
	indegree := map[string]int{}
	for _, id := range ids {
		indegree[id] = 0
	}
	for _, tos := range edges {
		for _, to := range tos {
			indegree[to]++
		}
	}
	var ready []string
	for _, id := range ids {
		if indegree[id] == 0 {
			ready = append(ready, id)
		}
	}
	sort.Strings(ready)
	var order []string
	for len(ready) > 0 {
		id := ready[0]
		ready = ready[1:]
		order = append(order, id)
		for _, to := range edges[id] {
			indegree[to]--
			if indegree[to] == 0 {
				ready = append(ready, to)
				sort.Strings(ready)
			}
		}
	}
	return order
}

func dependencyCycle(migrations []Migration, edges map[string][]string) []string {
	ids := sortedMigrationIDs(migrations)
	visiting := map[string]bool{}
	visited := map[string]bool{}
	var stack []string
	var visit func(string) []string
	visit = func(id string) []string {
		if visiting[id] {
			for i, entry := range stack {
				if entry == id {
					return append(append([]string(nil), stack[i:]...), id)
				}
			}
			return []string{id, id}
		}
		if visited[id] {
			return nil
		}
		visiting[id] = true
		stack = append(stack, id)
		for _, to := range edges[id] {
			if cycle := visit(to); len(cycle) > 0 {
				return cycle
			}
		}
		stack = stack[:len(stack)-1]
		visiting[id] = false
		visited[id] = true
		return nil
	}
	for _, id := range ids {
		if cycle := visit(id); len(cycle) > 0 {
			return cycle
		}
	}
	return nil
}

func longestPath(order []string, edges map[string][]string) ([]string, int) {
	if len(order) == 0 {
		return nil, 0
	}
	dist := map[string]int{}
	path := map[string][]string{}
	for _, id := range order {
		if dist[id] == 0 {
			dist[id] = 1
			path[id] = []string{id}
		}
		for _, to := range edges[id] {
			candidate := dist[id] + 1
			candidatePath := append(append([]string(nil), path[id]...), to)
			if candidate > dist[to] || (candidate == dist[to] && strings.Join(candidatePath, "\x00") < strings.Join(path[to], "\x00")) {
				dist[to] = candidate
				path[to] = candidatePath
			}
		}
	}
	bestID := order[0]
	for _, id := range order {
		if dist[id] > dist[bestID] || (dist[id] == dist[bestID] && strings.Join(path[id], "\x00") < strings.Join(path[bestID], "\x00")) {
			bestID = id
		}
	}
	return append([]string(nil), path[bestID]...), dist[bestID]
}

func migrationLevels(order []string, edges map[string][]string) map[string]int {
	levels := map[string]int{}
	for _, id := range order {
		if levels[id] == 0 {
			levels[id] = 1
		}
		for _, to := range edges[id] {
			if levels[id]+1 > levels[to] {
				levels[to] = levels[id] + 1
			}
		}
	}
	return levels
}

func maxFanout(edges map[string][]string) int {
	max := 0
	for _, tos := range edges {
		if len(tos) > max {
			max = len(tos)
		}
	}
	return max
}

func crossServiceEdges(migrations []Migration, services map[string]Service, byID map[string]Migration) []CrossServiceEdge {
	var out []CrossServiceEdge
	for _, migration := range sortedMigrations(migrations) {
		for _, depID := range sortedStrings(migration.DependsOn) {
			dep, ok := byID[depID]
			if !ok || dep.ServiceID == migration.ServiceID {
				continue
			}
			declared := relationDeclared(dep.ServiceID, migration.ServiceID, services, dep, migration)
			out = append(out, CrossServiceEdge{
				FromMigration: dep.ID,
				ToMigration:   migration.ID,
				FromService:   dep.ServiceID,
				ToService:     migration.ServiceID,
				Declared:      declared,
			})
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].FromMigration != out[j].FromMigration {
			return out[i].FromMigration < out[j].FromMigration
		}
		return out[i].ToMigration < out[j].ToMigration
	})
	return out
}

func relationDeclared(fromService, toService string, services map[string]Service, from, to Migration) bool {
	if stringSliceContains(services[fromService].DownstreamServices, toService) || stringSliceContains(services[toService].UpstreamServices, fromService) {
		return true
	}
	if stringSliceContains(from.DownstreamServices, toService) || stringSliceContains(to.UpstreamServices, fromService) {
		return true
	}
	return false
}

func dataLossEntry(migration Migration, reason string) DataLossEntry {
	return DataLossEntry{
		MigrationID:  migration.ID,
		ServiceID:    migration.ServiceID,
		Reason:       reason,
		Rows:         migration.EstimatedRows,
		CriticalRows: migration.CriticalRows,
		DataClasses:  sortedStrings(migration.DataClasses),
	}
}

func obligationsChecked(obligations []Obligation) bool {
	for _, obligation := range obligations {
		if obligation.Status != "checked" {
			return false
		}
	}
	return true
}

func hasCounterexampleKind(counterexamples []Counterexample, kind string) bool {
	for _, counterexample := range counterexamples {
		if counterexample.Kind == kind {
			return true
		}
	}
	return false
}

func hasCounterexampleID(counterexamples []Counterexample, id string) bool {
	for _, counterexample := range counterexamples {
		if counterexample.ID == id {
			return true
		}
	}
	return false
}

func servicesByID(services []Service) map[string]Service {
	out := map[string]Service{}
	for _, service := range services {
		out[service.ID] = service
	}
	return out
}

func migrationsByID(migrations []Migration) map[string]Migration {
	out := map[string]Migration{}
	for _, migration := range migrations {
		out[migration.ID] = migration
	}
	return out
}

func sortedServices(services []Service) []Service {
	out := append([]Service(nil), services...)
	sort.SliceStable(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func sortedMigrations(migrations []Migration) []Migration {
	out := append([]Migration(nil), migrations...)
	sort.SliceStable(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func sortedMigrationIDs(migrations []Migration) []string {
	var ids []string
	for _, migration := range migrations {
		ids = append(ids, migration.ID)
	}
	sort.Strings(ids)
	return ids
}

func sortedKeys(values map[string]bool) []string {
	var keys []string
	for key := range values {
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

func reverseStrings(values []string) []string {
	out := append([]string(nil), values...)
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out
}

func stringSliceContains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func sortCounterexamples(counterexamples []Counterexample) {
	sort.SliceStable(counterexamples, func(i, j int) bool {
		if counterexamples[i].ID != counterexamples[j].ID {
			return counterexamples[i].ID < counterexamples[j].ID
		}
		return counterexamples[i].Subject < counterexamples[j].Subject
	})
}

func reportHash(report Report) string {
	report.Hash = ""
	return canonical.Hash(report)
}

func writeJSON(path string, value any) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	if err := canonical.WriteJSON(file, value); err != nil {
		file.Close()
		return err
	}
	return file.Close()
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
