package sloreport

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/thehalleyyoung/patchline/internal/canonical"
)

const SpecVersion = "patchline.public-slo/v1"
const ReportVersion = "patchline.public-slo-report/v1"

type Spec struct {
	Version       string    `json:"version"`
	Name          string    `json:"name"`
	Claim         string    `json:"claim,omitempty"`
	Period        Period    `json:"period"`
	Criteria      Criteria  `json:"criteria"`
	Surfaces      []Surface `json:"surfaces"`
	EvidencePaths []string  `json:"evidence_paths,omitempty"`
}

type Period struct {
	Start string `json:"start"`
	End   string `json:"end"`
}

type Criteria struct {
	RequiredKinds                 []string `json:"required_kinds,omitempty"`
	MinSurfaces                   int      `json:"min_surfaces"`
	MinProbesPerSurface           int      `json:"min_probes_per_surface"`
	MinUptimePercent              float64  `json:"min_uptime_percent"`
	MinReproducibilityPercent     float64  `json:"min_reproducibility_percent"`
	MaxP95LatencyMS               int      `json:"max_p95_latency_ms"`
	MaxProbeAgeHours              int      `json:"max_probe_age_hours"`
	MaxIncidentMinutes            int      `json:"max_incident_minutes"`
	RequirePublicStatusURL        bool     `json:"require_public_status_url"`
	RequireReproducibilityProbe   bool     `json:"require_reproducibility_probe"`
	RequireIncidentReview         bool     `json:"require_incident_review"`
	RequireEvidenceHashes         bool     `json:"require_evidence_hashes"`
	RequireReproducibilityCommand bool     `json:"require_reproducibility_command"`
}

type Surface struct {
	ID            string     `json:"id"`
	Kind          string     `json:"kind"`
	DisplayName   string     `json:"display_name,omitempty"`
	PublicURL     string     `json:"public_url"`
	StatusURL     string     `json:"status_url,omitempty"`
	Owner         string     `json:"owner,omitempty"`
	SLO           SurfaceSLO `json:"slo"`
	Probes        []Probe    `json:"probes"`
	Incidents     []Incident `json:"incidents,omitempty"`
	EvidencePaths []string   `json:"evidence_paths,omitempty"`
}

type SurfaceSLO struct {
	UptimeTargetPercent          float64 `json:"uptime_target_percent,omitempty"`
	ReproducibilityTargetPercent float64 `json:"reproducibility_target_percent,omitempty"`
	MaxP95LatencyMS              int     `json:"max_p95_latency_ms,omitempty"`
	MaxIncidentMinutes           int     `json:"max_incident_minutes,omitempty"`
}

type Probe struct {
	ID            string            `json:"id"`
	Kind          string            `json:"kind"`
	ObservedAt    string            `json:"observed_at"`
	Status        string            `json:"status"`
	LatencyMS     int               `json:"latency_ms,omitempty"`
	Command       []string          `json:"command,omitempty"`
	Artifact      ArtifactRef       `json:"artifact,omitempty"`
	EvidencePaths []string          `json:"evidence_paths,omitempty"`
	Metadata      map[string]string `json:"metadata,omitempty"`
}

type ArtifactRef struct {
	Path   string `json:"path,omitempty"`
	SHA256 string `json:"sha256,omitempty"`
}

type Incident struct {
	ID               string   `json:"id"`
	StartedAt        string   `json:"started_at,omitempty"`
	ResolvedAt       string   `json:"resolved_at,omitempty"`
	DurationMinutes  int      `json:"duration_minutes,omitempty"`
	Severity         string   `json:"severity,omitempty"`
	PublicSummaryURL string   `json:"public_summary_url,omitempty"`
	ReviewPath       string   `json:"review_path,omitempty"`
	CorrectiveAction string   `json:"corrective_action,omitempty"`
	EvidencePaths    []string `json:"evidence_paths,omitempty"`
}

type Report struct {
	Version         string             `json:"version"`
	Name            string             `json:"name"`
	OK              bool               `json:"ok"`
	Period          Period             `json:"period"`
	Criteria        Criteria           `json:"criteria"`
	Summary         Summary            `json:"summary"`
	Evidence        []ArtifactEvidence `json:"evidence,omitempty"`
	Surfaces        []SurfaceReport    `json:"surfaces"`
	Counterexamples []Counterexample   `json:"counterexamples,omitempty"`
	Hash            string             `json:"hash"`
}

type Summary struct {
	Surfaces                    int `json:"surfaces"`
	Kinds                       int `json:"kinds"`
	HostedDocsSurfaces          int `json:"hosted_docs_surfaces"`
	ArtifactSurfaces            int `json:"artifact_surfaces"`
	MarketplaceEvidenceSurfaces int `json:"marketplace_evidence_surfaces"`
	CorpusAPISurfaces           int `json:"corpus_api_surfaces"`
	PublicStatusURLs            int `json:"public_status_urls"`
	Probes                      int `json:"probes"`
	PassingProbes               int `json:"passing_probes"`
	FailingProbes               int `json:"failing_probes"`
	UptimeProbes                int `json:"uptime_probes"`
	ReproducibilityProbes       int `json:"reproducibility_probes"`
	UptimeSLOMet                int `json:"uptime_slo_met"`
	ReproducibilitySLOMet       int `json:"reproducibility_slo_met"`
	LatencySLOMet               int `json:"latency_slo_met"`
	FreshSurfaces               int `json:"fresh_surfaces"`
	IncidentMinutes             int `json:"incident_minutes"`
	ReviewedIncidents           int `json:"reviewed_incidents"`
	EvidenceArtifacts           int `json:"evidence_artifacts"`
	Counterexamples             int `json:"counterexamples"`
}

type SurfaceReport struct {
	ID                        string             `json:"id"`
	Kind                      string             `json:"kind"`
	DisplayName               string             `json:"display_name,omitempty"`
	PublicURL                 string             `json:"public_url"`
	StatusURL                 string             `json:"status_url,omitempty"`
	Owner                     string             `json:"owner,omitempty"`
	UptimePercent             float64            `json:"uptime_percent"`
	ReproducibilityPercent    float64            `json:"reproducibility_percent"`
	P95LatencyMS              int                `json:"p95_latency_ms"`
	IncidentMinutes           int                `json:"incident_minutes"`
	PublicStatus              bool               `json:"public_status"`
	MeetsUptime               bool               `json:"meets_uptime"`
	MeetsReproducibility      bool               `json:"meets_reproducibility"`
	MeetsLatency              bool               `json:"meets_latency"`
	WithinIncidentBudget      bool               `json:"within_incident_budget"`
	FreshProbes               bool               `json:"fresh_probes"`
	HasReproducibilityProbe   bool               `json:"has_reproducibility_probe"`
	HasReproducibilityCommand bool               `json:"has_reproducibility_command"`
	Probes                    []ProbeReport      `json:"probes,omitempty"`
	Incidents                 []IncidentReport   `json:"incidents,omitempty"`
	Evidence                  []ArtifactEvidence `json:"evidence,omitempty"`
}

type ProbeReport struct {
	ID         string             `json:"id"`
	Kind       string             `json:"kind"`
	ObservedAt string             `json:"observed_at"`
	Status     string             `json:"status"`
	Passing    bool               `json:"passing"`
	LatencyMS  int                `json:"latency_ms,omitempty"`
	Command    []string           `json:"command,omitempty"`
	Artifact   ArtifactEvidence   `json:"artifact,omitempty"`
	Evidence   []ArtifactEvidence `json:"evidence,omitempty"`
}

type IncidentReport struct {
	ID              string             `json:"id"`
	DurationMinutes int                `json:"duration_minutes"`
	Severity        string             `json:"severity,omitempty"`
	Reviewed        bool               `json:"reviewed"`
	Evidence        []ArtifactEvidence `json:"evidence,omitempty"`
}

type ArtifactEvidence struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
	Bytes  int64  `json:"bytes"`
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
		return Spec{}, fmt.Errorf("public SLO spec version must be %s", SpecVersion)
	}
	return spec, nil
}

func BuildReport(spec Spec, root string) (Report, error) {
	if err := validateSpec(spec); err != nil {
		return Report{}, err
	}
	if root == "" {
		root = "."
	}
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return Report{}, err
	}
	criteria := normalizeCriteria(spec.Criteria)
	report := Report{
		Version:  ReportVersion,
		Name:     spec.Name,
		OK:       true,
		Period:   spec.Period,
		Criteria: criteria,
	}
	var counterexamples []Counterexample
	report.Evidence, counterexamples = collectArtifacts(rootAbs, spec.EvidencePaths, spec.Name, "run_evidence", false)
	report.Summary.EvidenceArtifacts += len(report.Evidence)
	if criteria.RequireEvidenceHashes && len(report.Evidence) == 0 {
		counterexamples = append(counterexamples, Counterexample{
			ID:      "run." + stableID(spec.Name, "evidence") + ".missing",
			Kind:    "missing_run_evidence",
			Subject: spec.Name,
			Message: "public SLO report does not cite readable run-level evidence",
		})
	}
	windowEnd, timeCounterexamples := parseWindowEnd(spec.Period)
	counterexamples = append(counterexamples, timeCounterexamples...)

	surfaceReports, surfaceCounterexamples, seenKinds := buildSurfaceReports(rootAbs, spec.Surfaces, criteria, windowEnd)
	report.Surfaces = surfaceReports
	counterexamples = append(counterexamples, surfaceCounterexamples...)
	report.Summary.Surfaces = len(surfaceReports)
	report.Summary.Kinds = len(seenKinds)
	for _, surface := range surfaceReports {
		switch surface.Kind {
		case "hosted-docs":
			report.Summary.HostedDocsSurfaces++
		case "artifacts":
			report.Summary.ArtifactSurfaces++
		case "marketplace-evidence":
			report.Summary.MarketplaceEvidenceSurfaces++
		case "corpus-api":
			report.Summary.CorpusAPISurfaces++
		}
		if surface.PublicStatus {
			report.Summary.PublicStatusURLs++
		}
		if surface.MeetsUptime {
			report.Summary.UptimeSLOMet++
		}
		if surface.MeetsReproducibility {
			report.Summary.ReproducibilitySLOMet++
		}
		if surface.MeetsLatency {
			report.Summary.LatencySLOMet++
		}
		if surface.FreshProbes {
			report.Summary.FreshSurfaces++
		}
		report.Summary.Probes += len(surface.Probes)
		for _, probe := range surface.Probes {
			if probe.Passing {
				report.Summary.PassingProbes++
			} else {
				report.Summary.FailingProbes++
			}
			switch probe.Kind {
			case "uptime":
				report.Summary.UptimeProbes++
			case "reproducibility":
				report.Summary.ReproducibilityProbes++
			}
			if probe.Artifact.Path != "" {
				report.Summary.EvidenceArtifacts++
			}
			report.Summary.EvidenceArtifacts += len(probe.Evidence)
		}
		for _, incident := range surface.Incidents {
			report.Summary.IncidentMinutes += incident.DurationMinutes
			if incident.Reviewed {
				report.Summary.ReviewedIncidents++
			}
			report.Summary.EvidenceArtifacts += len(incident.Evidence)
		}
		report.Summary.EvidenceArtifacts += len(surface.Evidence)
	}
	if len(surfaceReports) < criteria.MinSurfaces {
		counterexamples = append(counterexamples, Counterexample{
			ID:      "criteria.surfaces.insufficient",
			Kind:    "insufficient_surfaces",
			Message: fmt.Sprintf("surfaces %d below required %d", len(surfaceReports), criteria.MinSurfaces),
		})
	}
	for _, kind := range criteria.RequiredKinds {
		if !seenKinds[kind] {
			counterexamples = append(counterexamples, Counterexample{
				ID:      "criteria." + stableID(kind, "required") + ".missing",
				Kind:    "missing_required_kind",
				Subject: kind,
				Message: "required public SLO surface kind is not declared",
			})
		}
	}
	sortCounterexamples(counterexamples)
	report.Counterexamples = counterexamples
	report.Summary.Counterexamples = len(counterexamples)
	report.OK = len(counterexamples) == 0
	report.Hash = reportHash(report)
	return report, nil
}

func WriteArtifacts(outDir string, report Report) error {
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}
	file, err := os.Create(filepath.Join(outDir, "public-slo-report.json"))
	if err != nil {
		return err
	}
	if err := canonical.WriteJSON(file, report); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(outDir, "public-slo-report.md"), []byte(RenderMarkdown(report)), 0o644)
}

func RenderMarkdown(report Report) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Public uptime and reproducibility SLO report\n\n")
	fmt.Fprintf(&b, "Patchline verifies hosted docs, artifacts, marketplace evidence, and corpus APIs against public availability, latency, reproducibility, incident-review, and hash-backed evidence commitments.\n\n")
	fmt.Fprintf(&b, "## Summary\n\n")
	fmt.Fprintf(&b, "| Metric | Value |\n| --- | ---: |\n")
	fmt.Fprintf(&b, "| OK | `%t` |\n", report.OK)
	fmt.Fprintf(&b, "| Surfaces | %d |\n", report.Summary.Surfaces)
	fmt.Fprintf(&b, "| Kinds | %d |\n", report.Summary.Kinds)
	fmt.Fprintf(&b, "| Public status URLs | %d |\n", report.Summary.PublicStatusURLs)
	fmt.Fprintf(&b, "| Probes | %d |\n", report.Summary.Probes)
	fmt.Fprintf(&b, "| Uptime SLOs met | %d |\n", report.Summary.UptimeSLOMet)
	fmt.Fprintf(&b, "| Reproducibility SLOs met | %d |\n", report.Summary.ReproducibilitySLOMet)
	fmt.Fprintf(&b, "| Latency SLOs met | %d |\n", report.Summary.LatencySLOMet)
	fmt.Fprintf(&b, "| Incident minutes | %d |\n", report.Summary.IncidentMinutes)
	fmt.Fprintf(&b, "| Evidence artifacts | %d |\n", report.Summary.EvidenceArtifacts)
	fmt.Fprintf(&b, "| Counterexamples | %d |\n\n", report.Summary.Counterexamples)
	fmt.Fprintf(&b, "## Surfaces\n\n")
	fmt.Fprintf(&b, "| Surface | Kind | Uptime | Reproducibility | p95 ms | Incidents min | Status | Fresh |\n| --- | --- | ---: | ---: | ---: | ---: | ---: | ---: |\n")
	for _, surface := range report.Surfaces {
		fmt.Fprintf(&b, "| `%s` | `%s` | %.2f%% | %.2f%% | %d | %d | `%t` | `%t` |\n",
			escapeTable(surface.ID),
			escapeTable(surface.Kind),
			surface.UptimePercent,
			surface.ReproducibilityPercent,
			surface.P95LatencyMS,
			surface.IncidentMinutes,
			surface.PublicStatus,
			surface.FreshProbes,
		)
	}
	if len(report.Counterexamples) > 0 {
		fmt.Fprintf(&b, "\n## Counterexamples\n\n")
		fmt.Fprintf(&b, "| ID | Kind | Subject | Message |\n| --- | --- | --- | --- |\n")
		for _, counterexample := range report.Counterexamples {
			fmt.Fprintf(&b, "| `%s` | `%s` | `%s` | %s |\n",
				escapeTable(counterexample.ID),
				escapeTable(counterexample.Kind),
				escapeTable(firstNonEmpty(counterexample.Subject, "-")),
				escapeTable(counterexample.Message),
			)
		}
	}
	return b.String()
}

func validateSpec(spec Spec) error {
	if spec.Version != SpecVersion {
		return fmt.Errorf("public SLO spec version must be %s", SpecVersion)
	}
	if strings.TrimSpace(spec.Name) == "" {
		return fmt.Errorf("public SLO spec name is required")
	}
	return nil
}

func normalizeCriteria(criteria Criteria) Criteria {
	criteria.RequiredKinds = normalizedKinds(criteria.RequiredKinds)
	if criteria.MinSurfaces <= 0 {
		criteria.MinSurfaces = len(criteria.RequiredKinds)
	}
	if criteria.MinProbesPerSurface <= 0 {
		criteria.MinProbesPerSurface = 2
	}
	if criteria.MinUptimePercent <= 0 {
		criteria.MinUptimePercent = 99
	}
	if criteria.MinReproducibilityPercent <= 0 {
		criteria.MinReproducibilityPercent = 100
	}
	if criteria.MaxP95LatencyMS <= 0 {
		criteria.MaxP95LatencyMS = 1000
	}
	if criteria.MaxProbeAgeHours <= 0 {
		criteria.MaxProbeAgeHours = 168
	}
	if criteria.MaxIncidentMinutes < 0 {
		criteria.MaxIncidentMinutes = 0
	}
	return criteria
}

func buildSurfaceReports(root string, surfaces []Surface, criteria Criteria, windowEnd time.Time) ([]SurfaceReport, []Counterexample, map[string]bool) {
	sorted := append([]Surface(nil), surfaces...)
	sort.SliceStable(sorted, func(i, j int) bool {
		return normalizeID(sorted[i].ID) < normalizeID(sorted[j].ID)
	})
	var reports []SurfaceReport
	var counterexamples []Counterexample
	seen := map[string]bool{}
	seenKinds := map[string]bool{}
	for _, surface := range sorted {
		id := normalizeID(surface.ID)
		if id == "" {
			counterexamples = append(counterexamples, Counterexample{ID: "surface.missing-id", Kind: "missing_surface_id", Message: "public SLO surface is missing an id"})
			continue
		}
		if seen[id] {
			counterexamples = append(counterexamples, Counterexample{ID: "surface." + stableID(id, "duplicate"), Kind: "duplicate_surface", Subject: id, Message: "public SLO surface is duplicated"})
			continue
		}
		seen[id] = true
		kind := normalizeKind(surface.Kind)
		if !allowedKinds[kind] {
			counterexamples = append(counterexamples, Counterexample{ID: "surface." + stableID(id, surface.Kind, "kind") + ".unknown", Kind: "unknown_surface_kind", Subject: id, Message: "public SLO surface kind is unknown", Witness: []string{surface.Kind}})
		} else {
			seenKinds[kind] = true
		}
		evidence, evidenceCounterexamples := collectArtifacts(root, surface.EvidencePaths, id, "surface_evidence", false)
		counterexamples = append(counterexamples, evidenceCounterexamples...)
		if criteria.RequireEvidenceHashes && len(evidence) == 0 {
			counterexamples = append(counterexamples, Counterexample{ID: "surface." + stableID(id, "evidence") + ".missing", Kind: "missing_surface_evidence", Subject: id, Message: "surface does not cite readable evidence"})
		}
		probeReports, probeCounterexamples := buildProbeReports(root, id, surface.Probes, criteria, windowEnd)
		counterexamples = append(counterexamples, probeCounterexamples...)
		incidentReports, incidentMinutes, incidentCounterexamples := buildIncidentReports(root, id, surface.Incidents, criteria)
		counterexamples = append(counterexamples, incidentCounterexamples...)
		metrics := surfaceMetrics(probeReports, incidentMinutes, criteria, surface.SLO)
		report := SurfaceReport{
			ID:                        id,
			Kind:                      kind,
			DisplayName:               strings.TrimSpace(surface.DisplayName),
			PublicURL:                 strings.TrimSpace(surface.PublicURL),
			StatusURL:                 strings.TrimSpace(surface.StatusURL),
			Owner:                     strings.TrimSpace(surface.Owner),
			UptimePercent:             metrics.uptimePercent,
			ReproducibilityPercent:    metrics.reproducibilityPercent,
			P95LatencyMS:              metrics.p95LatencyMS,
			IncidentMinutes:           incidentMinutes,
			PublicStatus:              validHTTPURL(surface.StatusURL),
			MeetsUptime:               metrics.meetsUptime,
			MeetsReproducibility:      metrics.meetsReproducibility,
			MeetsLatency:              metrics.meetsLatency,
			WithinIncidentBudget:      metrics.withinIncidentBudget,
			FreshProbes:               metrics.freshProbes,
			HasReproducibilityProbe:   metrics.hasReproducibilityProbe,
			HasReproducibilityCommand: metrics.hasReproducibilityCommand,
			Probes:                    probeReports,
			Incidents:                 incidentReports,
			Evidence:                  evidence,
		}
		counterexamples = append(counterexamples, validateSurface(surface, report, criteria, metrics)...)
		reports = append(reports, report)
	}
	return reports, counterexamples, seenKinds
}

type metricSummary struct {
	uptimePercent             float64
	reproducibilityPercent    float64
	p95LatencyMS              int
	meetsUptime               bool
	meetsReproducibility      bool
	meetsLatency              bool
	withinIncidentBudget      bool
	freshProbes               bool
	hasReproducibilityProbe   bool
	hasReproducibilityCommand bool
	uptimeProbeCount          int
	reproducibilityProbeCount int
}

func surfaceMetrics(probes []ProbeReport, incidentMinutes int, criteria Criteria, slo SurfaceSLO) metricSummary {
	var uptimeTotal, uptimePass, reproTotal, reproPass int
	var latencies []int
	fresh := true
	hasReproCommand := false
	for _, probe := range probes {
		if probe.Kind == "uptime" {
			uptimeTotal++
			if probe.Passing {
				uptimePass++
			}
		}
		if probe.Kind == "reproducibility" {
			reproTotal++
			if probe.Passing {
				reproPass++
			}
			if len(probe.Command) > 0 {
				hasReproCommand = true
			}
		}
		if probe.LatencyMS > 0 {
			latencies = append(latencies, probe.LatencyMS)
		}
		if probe.ObservedAt == "stale" {
			fresh = false
		}
	}
	uptimeTarget := maxFloat(criteria.MinUptimePercent, slo.UptimeTargetPercent)
	reproTarget := maxFloat(criteria.MinReproducibilityPercent, slo.ReproducibilityTargetPercent)
	latencyTarget := firstPositive(slo.MaxP95LatencyMS, criteria.MaxP95LatencyMS)
	incidentBudget := firstPositive(slo.MaxIncidentMinutes, criteria.MaxIncidentMinutes)
	uptimePercent := percent(uptimePass, uptimeTotal)
	reproPercent := percent(reproPass, reproTotal)
	p95 := percentile95(latencies)
	return metricSummary{
		uptimePercent:             uptimePercent,
		reproducibilityPercent:    reproPercent,
		p95LatencyMS:              p95,
		meetsUptime:               uptimeTotal > 0 && uptimePercent >= uptimeTarget,
		meetsReproducibility:      reproTotal > 0 && reproPercent >= reproTarget,
		meetsLatency:              p95 > 0 && p95 <= latencyTarget,
		withinIncidentBudget:      incidentMinutes <= incidentBudget,
		freshProbes:               fresh,
		hasReproducibilityProbe:   reproTotal > 0,
		hasReproducibilityCommand: hasReproCommand,
		uptimeProbeCount:          uptimeTotal,
		reproducibilityProbeCount: reproTotal,
	}
}

func validateSurface(surface Surface, report SurfaceReport, criteria Criteria, metrics metricSummary) []Counterexample {
	var counterexamples []Counterexample
	if !validHTTPURL(surface.PublicURL) {
		counterexamples = append(counterexamples, Counterexample{ID: "surface." + stableID(report.ID, "public-url") + ".invalid", Kind: "invalid_public_url", Subject: report.ID, Message: "surface public URL must be an absolute http(s) URL", Witness: []string{surface.PublicURL}})
	}
	if criteria.RequirePublicStatusURL && strings.TrimSpace(surface.StatusURL) == "" {
		counterexamples = append(counterexamples, Counterexample{ID: "surface." + stableID(report.ID, "status-url") + ".missing", Kind: "missing_public_status_url", Subject: report.ID, Message: "surface must publish a public status URL"})
	} else if strings.TrimSpace(surface.StatusURL) != "" && !validHTTPURL(surface.StatusURL) {
		counterexamples = append(counterexamples, Counterexample{ID: "surface." + stableID(report.ID, "status-url") + ".invalid", Kind: "invalid_public_status_url", Subject: report.ID, Message: "surface status URL must be an absolute http(s) URL", Witness: []string{surface.StatusURL}})
	}
	if len(report.Probes) < criteria.MinProbesPerSurface {
		counterexamples = append(counterexamples, Counterexample{ID: "surface." + stableID(report.ID, "probes") + ".insufficient", Kind: "insufficient_probes", Subject: report.ID, Message: fmt.Sprintf("surface has %d probes below required %d", len(report.Probes), criteria.MinProbesPerSurface)})
	}
	if !metrics.meetsUptime {
		counterexamples = append(counterexamples, Counterexample{ID: "surface." + stableID(report.ID, "uptime") + ".breached", Kind: "uptime_slo_breached", Subject: report.ID, Message: "surface uptime probes do not meet the public uptime SLO"})
	}
	if criteria.RequireReproducibilityProbe && !metrics.hasReproducibilityProbe {
		counterexamples = append(counterexamples, Counterexample{ID: "surface." + stableID(report.ID, "repro-probe") + ".missing", Kind: "missing_reproducibility_probe", Subject: report.ID, Message: "surface has no reproducibility probe"})
	}
	if !metrics.meetsReproducibility {
		counterexamples = append(counterexamples, Counterexample{ID: "surface." + stableID(report.ID, "repro") + ".breached", Kind: "reproducibility_slo_breached", Subject: report.ID, Message: "surface reproducibility probes do not meet the reproducibility SLO"})
	}
	if criteria.RequireReproducibilityCommand && !metrics.hasReproducibilityCommand {
		counterexamples = append(counterexamples, Counterexample{ID: "surface." + stableID(report.ID, "repro-command") + ".missing", Kind: "missing_reproducibility_command", Subject: report.ID, Message: "surface reproducibility probes do not include a rerunnable command"})
	}
	if !metrics.meetsLatency {
		counterexamples = append(counterexamples, Counterexample{ID: "surface." + stableID(report.ID, "latency") + ".breached", Kind: "latency_slo_breached", Subject: report.ID, Message: "surface p95 latency exceeds the public latency SLO"})
	}
	if !metrics.withinIncidentBudget {
		counterexamples = append(counterexamples, Counterexample{ID: "surface." + stableID(report.ID, "incidents") + ".over", Kind: "incident_budget_exceeded", Subject: report.ID, Message: "surface incident minutes exceed the allowed budget"})
	}
	if !metrics.freshProbes {
		counterexamples = append(counterexamples, Counterexample{ID: "surface." + stableID(report.ID, "freshness") + ".stale", Kind: "stale_probe_window", Subject: report.ID, Message: "one or more probes are older than the allowed report freshness window"})
	}
	return counterexamples
}

func buildProbeReports(root, surfaceID string, probes []Probe, criteria Criteria, windowEnd time.Time) ([]ProbeReport, []Counterexample) {
	sorted := append([]Probe(nil), probes...)
	sort.SliceStable(sorted, func(i, j int) bool {
		return normalizeID(sorted[i].ID) < normalizeID(sorted[j].ID)
	})
	var reports []ProbeReport
	var counterexamples []Counterexample
	seen := map[string]bool{}
	for _, probe := range sorted {
		id := normalizeID(probe.ID)
		if id == "" {
			counterexamples = append(counterexamples, Counterexample{ID: "probe." + stableID(surfaceID, "missing-id"), Kind: "missing_probe_id", Subject: surfaceID, Message: "probe is missing an id"})
			continue
		}
		if seen[id] {
			counterexamples = append(counterexamples, Counterexample{ID: "probe." + stableID(surfaceID, id, "duplicate"), Kind: "duplicate_probe", Subject: surfaceID, Message: "probe id is duplicated"})
			continue
		}
		seen[id] = true
		kind := normalizeProbeKind(probe.Kind)
		if !allowedProbeKinds[kind] {
			counterexamples = append(counterexamples, Counterexample{ID: "probe." + stableID(surfaceID, id, "kind") + ".unknown", Kind: "unknown_probe_kind", Subject: surfaceID, Message: "probe kind is unknown", Witness: []string{probe.Kind}})
		}
		status := normalizeStatus(probe.Status)
		if status == "" {
			counterexamples = append(counterexamples, Counterexample{ID: "probe." + stableID(surfaceID, id, "status") + ".unknown", Kind: "unknown_probe_status", Subject: surfaceID, Message: "probe status must be pass or fail", Witness: []string{probe.Status}})
		}
		if probe.LatencyMS <= 0 {
			counterexamples = append(counterexamples, Counterexample{ID: "probe." + stableID(surfaceID, id, "latency") + ".invalid", Kind: "invalid_probe_latency", Subject: surfaceID, Message: "probe latency must be positive"})
		}
		observedAt := strings.TrimSpace(probe.ObservedAt)
		if windowEnd.IsZero() {
			if observedAt == "" {
				counterexamples = append(counterexamples, Counterexample{ID: "probe." + stableID(surfaceID, id, "observed-at") + ".missing", Kind: "missing_probe_time", Subject: surfaceID, Message: "probe observed_at timestamp is required"})
			}
		} else {
			parsed, err := time.Parse(time.RFC3339, observedAt)
			if err != nil {
				counterexamples = append(counterexamples, Counterexample{ID: "probe." + stableID(surfaceID, id, "observed-at") + ".invalid", Kind: "invalid_probe_time", Subject: surfaceID, Message: "probe observed_at timestamp must be RFC3339", Witness: []string{observedAt}})
			} else if criteria.MaxProbeAgeHours > 0 && windowEnd.Sub(parsed) > time.Duration(criteria.MaxProbeAgeHours)*time.Hour {
				counterexamples = append(counterexamples, Counterexample{ID: "probe." + stableID(surfaceID, id, "freshness") + ".stale", Kind: "stale_probe", Subject: surfaceID, Message: "probe is older than the allowed report freshness window", Witness: []string{observedAt, windowEnd.Format(time.RFC3339)}})
				observedAt = "stale"
			}
		}
		evidence, evidenceCounterexamples := collectArtifacts(root, probe.EvidencePaths, surfaceID+"/"+id, "probe_evidence", false)
		counterexamples = append(counterexamples, evidenceCounterexamples...)
		if criteria.RequireEvidenceHashes && len(evidence) == 0 {
			counterexamples = append(counterexamples, Counterexample{ID: "probe." + stableID(surfaceID, id, "evidence") + ".missing", Kind: "missing_probe_evidence", Subject: surfaceID, Message: "probe does not cite readable evidence"})
		}
		artifact, artifactCounterexamples := collectArtifact(root, probe.Artifact.Path, probe.Artifact.SHA256, surfaceID+"/"+id, "probe_artifact", false)
		counterexamples = append(counterexamples, artifactCounterexamples...)
		reports = append(reports, ProbeReport{
			ID:         id,
			Kind:       kind,
			ObservedAt: observedAt,
			Status:     status,
			Passing:    status == "pass",
			LatencyMS:  probe.LatencyMS,
			Command:    append([]string(nil), probe.Command...),
			Artifact:   artifact,
			Evidence:   evidence,
		})
	}
	return reports, counterexamples
}

func buildIncidentReports(root, surfaceID string, incidents []Incident, criteria Criteria) ([]IncidentReport, int, []Counterexample) {
	sorted := append([]Incident(nil), incidents...)
	sort.SliceStable(sorted, func(i, j int) bool {
		return normalizeID(sorted[i].ID) < normalizeID(sorted[j].ID)
	})
	var reports []IncidentReport
	var counterexamples []Counterexample
	total := 0
	for _, incident := range sorted {
		id := normalizeID(incident.ID)
		if id == "" {
			counterexamples = append(counterexamples, Counterexample{ID: "incident." + stableID(surfaceID, "missing-id"), Kind: "missing_incident_id", Subject: surfaceID, Message: "incident is missing an id"})
			continue
		}
		duration, durationCounterexamples := incidentDuration(surfaceID, incident)
		counterexamples = append(counterexamples, durationCounterexamples...)
		total += duration
		evidencePaths := append([]string(nil), incident.EvidencePaths...)
		if strings.TrimSpace(incident.ReviewPath) != "" {
			evidencePaths = append(evidencePaths, incident.ReviewPath)
		}
		evidence, evidenceCounterexamples := collectArtifacts(root, evidencePaths, surfaceID+"/"+id, "incident_evidence", false)
		counterexamples = append(counterexamples, evidenceCounterexamples...)
		reviewed := strings.TrimSpace(incident.CorrectiveAction) != "" && (validHTTPURL(incident.PublicSummaryURL) || strings.TrimSpace(incident.ReviewPath) != "") && len(evidence) > 0
		if criteria.RequireIncidentReview && !reviewed {
			counterexamples = append(counterexamples, Counterexample{ID: "incident." + stableID(surfaceID, id, "review") + ".missing", Kind: "incident_review_missing", Subject: surfaceID, Message: "incident does not have public/review evidence plus corrective action"})
		}
		reports = append(reports, IncidentReport{
			ID:              id,
			DurationMinutes: duration,
			Severity:        strings.TrimSpace(incident.Severity),
			Reviewed:        reviewed,
			Evidence:        evidence,
		})
	}
	return reports, total, counterexamples
}

func incidentDuration(surfaceID string, incident Incident) (int, []Counterexample) {
	if incident.DurationMinutes > 0 {
		return incident.DurationMinutes, nil
	}
	if strings.TrimSpace(incident.StartedAt) == "" || strings.TrimSpace(incident.ResolvedAt) == "" {
		return 0, []Counterexample{{ID: "incident." + stableID(surfaceID, incident.ID, "duration") + ".missing", Kind: "missing_incident_duration", Subject: surfaceID, Message: "incident must declare duration_minutes or start/resolved timestamps"}}
	}
	start, err := time.Parse(time.RFC3339, incident.StartedAt)
	if err != nil {
		return 0, []Counterexample{{ID: "incident." + stableID(surfaceID, incident.ID, "start") + ".invalid", Kind: "invalid_incident_time", Subject: surfaceID, Message: "incident started_at timestamp must be RFC3339"}}
	}
	end, err := time.Parse(time.RFC3339, incident.ResolvedAt)
	if err != nil {
		return 0, []Counterexample{{ID: "incident." + stableID(surfaceID, incident.ID, "end") + ".invalid", Kind: "invalid_incident_time", Subject: surfaceID, Message: "incident resolved_at timestamp must be RFC3339"}}
	}
	if !end.After(start) {
		return 0, []Counterexample{{ID: "incident." + stableID(surfaceID, incident.ID, "duration") + ".invalid", Kind: "invalid_incident_duration", Subject: surfaceID, Message: "incident resolved_at must be after started_at"}}
	}
	return int(end.Sub(start).Minutes()), nil
}

func parseWindowEnd(period Period) (time.Time, []Counterexample) {
	if strings.TrimSpace(period.End) == "" {
		return time.Time{}, []Counterexample{{ID: "period.end.missing", Kind: "missing_period_end", Message: "SLO report period end is required for freshness checks"}}
	}
	end, err := time.Parse(time.RFC3339, period.End)
	if err != nil {
		return time.Time{}, []Counterexample{{ID: "period.end.invalid", Kind: "invalid_period_end", Message: "SLO report period end must be RFC3339", Witness: []string{period.End}}}
	}
	if strings.TrimSpace(period.Start) != "" {
		start, err := time.Parse(time.RFC3339, period.Start)
		if err != nil {
			return end, []Counterexample{{ID: "period.start.invalid", Kind: "invalid_period_start", Message: "SLO report period start must be RFC3339", Witness: []string{period.Start}}}
		}
		if !end.After(start) {
			return end, []Counterexample{{ID: "period.range.invalid", Kind: "invalid_period_range", Message: "SLO report period end must be after start"}}
		}
	}
	return end, nil
}

func collectArtifacts(root string, paths []string, subject, kind string, require bool) ([]ArtifactEvidence, []Counterexample) {
	var artifacts []ArtifactEvidence
	var counterexamples []Counterexample
	for _, path := range paths {
		artifact, artifactCounterexamples := collectArtifact(root, path, "", subject, kind, require)
		counterexamples = append(counterexamples, artifactCounterexamples...)
		if artifact.Path != "" {
			artifacts = append(artifacts, artifact)
		}
	}
	return artifacts, counterexamples
}

func collectArtifact(root, rel, expected, subject, kind string, require bool) (ArtifactEvidence, []Counterexample) {
	rel = filepath.ToSlash(strings.TrimSpace(rel))
	if rel == "" {
		if require || strings.TrimSpace(expected) != "" {
			return ArtifactEvidence{}, []Counterexample{{ID: kind + "." + stableID(subject, "missing") + ".missing", Kind: "missing_" + kind, Subject: subject, Message: kind + " path is required"}}
		}
		return ArtifactEvidence{}, nil
	}
	if invalidRelPath(rel) {
		return ArtifactEvidence{}, []Counterexample{{ID: kind + "." + stableID(subject, rel, "path") + ".invalid", Kind: "invalid_" + kind + "_path", Subject: subject, Message: "referenced artifact path must stay under the report root", Witness: []string{rel}}}
	}
	path := filepath.Join(root, filepath.FromSlash(rel))
	data, err := os.ReadFile(path)
	if err != nil {
		return ArtifactEvidence{}, []Counterexample{{ID: kind + "." + stableID(subject, rel) + ".unreadable", Kind: "unreadable_" + kind, Subject: subject, Message: "referenced artifact is unreadable", Witness: []string{rel}}}
	}
	sum := "sha256:" + sha256Hex(data)
	if expected != "" && expected != sum {
		return ArtifactEvidence{Path: rel, SHA256: sum, Bytes: int64(len(data))}, []Counterexample{{ID: kind + "." + stableID(subject, rel, "hash") + ".mismatch", Kind: kind + "_hash_mismatch", Subject: subject, Message: "referenced artifact hash does not match the spec", Witness: []string{rel, expected, sum}}}
	}
	return ArtifactEvidence{Path: rel, SHA256: sum, Bytes: int64(len(data))}, nil
}

func invalidRelPath(rel string) bool {
	clean := filepath.Clean(filepath.FromSlash(rel))
	return filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator))
}

var allowedKinds = map[string]bool{
	"hosted-docs":          true,
	"artifacts":            true,
	"marketplace-evidence": true,
	"corpus-api":           true,
}

var allowedProbeKinds = map[string]bool{
	"uptime":          true,
	"reproducibility": true,
}

func normalizeKind(value string) string {
	value = normalizeToken(value)
	switch value {
	case "docs", "documentation", "hosted-documentation", "hosted-doc":
		return "hosted-docs"
	case "artifact", "artifact-store", "artifact-mirror", "artifacts-api":
		return "artifacts"
	case "marketplace", "evidence-marketplace", "marketplace-evidence-api":
		return "marketplace-evidence"
	case "corpus", "corpus-apis", "public-corpus-api":
		return "corpus-api"
	default:
		return value
	}
}

func normalizeProbeKind(value string) string {
	value = normalizeToken(value)
	switch value {
	case "availability", "health", "healthcheck":
		return "uptime"
	case "reproduce", "repro", "reproducible":
		return "reproducibility"
	default:
		return value
	}
}

func normalizeStatus(value string) string {
	value = normalizeToken(value)
	switch value {
	case "pass", "passed", "ok", "healthy", "success":
		return "pass"
	case "fail", "failed", "error", "unhealthy", "timeout":
		return "fail"
	default:
		return ""
	}
}

func normalizedKinds(values []string) []string {
	var out []string
	for _, value := range values {
		if normalized := normalizeKind(value); normalized != "" {
			out = append(out, normalized)
		}
	}
	sort.Strings(out)
	return compactStrings(out)
}

func normalizeID(value string) string {
	return strings.Trim(strings.ToLower(strings.ReplaceAll(strings.TrimSpace(value), "_", "-")), "-")
}

func normalizeToken(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	value = strings.ReplaceAll(value, " ", "-")
	value = strings.ReplaceAll(value, "_", "-")
	return value
}

func compactStrings(values []string) []string {
	var out []string
	last := ""
	for _, value := range values {
		if value == "" || value == last {
			continue
		}
		out = append(out, value)
		last = value
	}
	return out
}

func validHTTPURL(value string) bool {
	parsed, err := url.Parse(strings.TrimSpace(value))
	return err == nil && parsed.IsAbs() && (parsed.Scheme == "https" || parsed.Scheme == "http") && parsed.Host != ""
}

func percent(pass, total int) float64 {
	if total == 0 {
		return 0
	}
	return float64(pass) * 100 / float64(total)
}

func percentile95(values []int) int {
	if len(values) == 0 {
		return 0
	}
	sorted := append([]int(nil), values...)
	sort.Ints(sorted)
	index := (95*len(sorted)+99)/100 - 1
	if index < 0 {
		index = 0
	}
	if index >= len(sorted) {
		index = len(sorted) - 1
	}
	return sorted[index]
}

func firstPositive(values ...int) int {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 0
}

func maxFloat(left, right float64) float64 {
	if right > left {
		return right
	}
	return left
}

func stableID(parts ...string) string {
	return sha256Hex([]byte(strings.Join(parts, "\x00")))[:12]
}

func reportHash(report Report) string {
	clone := report
	clone.Hash = ""
	return canonical.Hash(clone)
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func sortCounterexamples(counterexamples []Counterexample) {
	sort.SliceStable(counterexamples, func(i, j int) bool {
		if counterexamples[i].Kind != counterexamples[j].Kind {
			return counterexamples[i].Kind < counterexamples[j].Kind
		}
		return counterexamples[i].ID < counterexamples[j].ID
	})
}

func escapeTable(value string) string {
	value = strings.ReplaceAll(value, "|", "\\|")
	value = strings.ReplaceAll(value, "\n", " ")
	return value
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
