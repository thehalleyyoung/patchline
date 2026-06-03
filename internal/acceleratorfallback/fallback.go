package acceleratorfallback

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"

	"github.com/thehalleyyoung/patchline/internal/canonical"
)

const SpecVersion = "patchline.accelerator-fallbacks/v1"
const ReportVersion = "patchline.accelerator-fallbacks-report/v1"

type Spec struct {
	Version       string      `json:"version"`
	Name          string      `json:"name"`
	Claim         string      `json:"claim,omitempty"`
	Criteria      Criteria    `json:"criteria"`
	Components    []Component `json:"components"`
	EvidencePaths []string    `json:"evidence_paths,omitempty"`
}

type Criteria struct {
	RequiredComponentIDs        []string `json:"required_component_ids,omitempty"`
	MinComponents               int      `json:"min_components"`
	MinReplayArtifacts          int      `json:"min_replay_artifacts"`
	RequireRepositoryDiscovery  bool     `json:"require_repository_discovery"`
	RequireCPUFallback          bool     `json:"require_cpu_fallback"`
	RequireAcceleratorFree      bool     `json:"require_accelerator_free"`
	RequireNoNetwork            bool     `json:"require_no_network"`
	RequireDeterministicReplay  bool     `json:"require_deterministic_replay"`
	RequireStableSeed           bool     `json:"require_stable_seed"`
	RequirePinnedLearned        bool     `json:"require_pinned_learned_artifact"`
	RequirePinnedImplementation bool     `json:"require_pinned_implementation"`
	RequirePinnedInputs         bool     `json:"require_pinned_inputs"`
	RequirePinnedOutputs        bool     `json:"require_pinned_outputs"`
	RequireParityEvidence       bool     `json:"require_parity_evidence"`
	RequireEvidenceHashes       bool     `json:"require_evidence_hashes"`
	MaxAllowedDrift             float64  `json:"max_allowed_drift"`
}

type Component struct {
	ID                    string   `json:"id"`
	Kind                  string   `json:"kind"`
	SourcePaths           []string `json:"source_paths,omitempty"`
	LearnedArtifactPath   string   `json:"learned_artifact_path"`
	LearnedArtifactSHA256 string   `json:"learned_artifact_sha256"`
	Fallback              Fallback `json:"fallback"`
	Parity                Parity   `json:"parity"`
	EvidencePaths         []string `json:"evidence_paths,omitempty"`
}

type Fallback struct {
	Runtime              string        `json:"runtime"`
	ImplementationPath   string        `json:"implementation_path"`
	ImplementationSHA256 string        `json:"implementation_sha256"`
	InputArtifacts       []ArtifactRef `json:"input_artifacts,omitempty"`
	OutputArtifact       ArtifactRef   `json:"output_artifact"`
	ReplayEvidencePaths  []string      `json:"replay_evidence_paths,omitempty"`
	GPURequired          bool          `json:"gpu_required"`
	AcceleratorRequired  bool          `json:"accelerator_required"`
	NetworkAllowed       bool          `json:"network_allowed"`
	Deterministic        bool          `json:"deterministic"`
	Seed                 string        `json:"seed"`
	Threads              int           `json:"threads"`
}

type ArtifactRef struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

type Parity struct {
	Metric          string   `json:"metric"`
	LearnedValue    *float64 `json:"learned_value"`
	FallbackValue   *float64 `json:"fallback_value"`
	MaxAllowedDrift *float64 `json:"max_allowed_drift,omitempty"`
	HigherIsBetter  bool     `json:"higher_is_better"`
	EvidencePaths   []string `json:"evidence_paths,omitempty"`
}

type Report struct {
	Version              string             `json:"version"`
	Name                 string             `json:"name"`
	OK                   bool               `json:"ok"`
	Criteria             Criteria           `json:"criteria"`
	Summary              Summary            `json:"summary"`
	DiscoveredComponents []string           `json:"discovered_components,omitempty"`
	Evidence             []ArtifactEvidence `json:"evidence,omitempty"`
	Components           []ComponentReport  `json:"components"`
	Counterexamples      []Counterexample   `json:"counterexamples,omitempty"`
	Hash                 string             `json:"hash"`
}

type Summary struct {
	DiscoveredComponents     int `json:"discovered_components"`
	Components               int `json:"components"`
	CPUFallbacks             int `json:"cpu_fallbacks"`
	AcceleratorFreeFallbacks int `json:"accelerator_free_fallbacks"`
	NoNetworkFallbacks       int `json:"no_network_fallbacks"`
	DeterministicFallbacks   int `json:"deterministic_fallbacks"`
	StableSeeds              int `json:"stable_seeds"`
	PinnedLearnedArtifacts   int `json:"pinned_learned_artifacts"`
	PinnedImplementations    int `json:"pinned_implementations"`
	PinnedInputArtifacts     int `json:"pinned_input_artifacts"`
	PinnedOutputArtifacts    int `json:"pinned_output_artifacts"`
	ReplayProofs             int `json:"replay_proofs"`
	ParityChecks             int `json:"parity_checks"`
	EvidenceArtifacts        int `json:"evidence_artifacts"`
	Counterexamples          int `json:"counterexamples"`
}

type ComponentReport struct {
	ID              string             `json:"id"`
	Kind            string             `json:"kind"`
	Discovered      bool               `json:"discovered"`
	Source          []ArtifactEvidence `json:"source,omitempty"`
	LearnedArtifact ArtifactEvidence   `json:"learned_artifact,omitempty"`
	Fallback        FallbackReport     `json:"fallback"`
	Parity          ParityReport       `json:"parity"`
	Evidence        []ArtifactEvidence `json:"evidence,omitempty"`
}

type FallbackReport struct {
	Runtime         string             `json:"runtime"`
	CPUOnly         bool               `json:"cpu_only"`
	AcceleratorFree bool               `json:"accelerator_free"`
	NoNetwork       bool               `json:"no_network"`
	Deterministic   bool               `json:"deterministic"`
	Seed            string             `json:"seed,omitempty"`
	Threads         int                `json:"threads"`
	Implementation  ArtifactEvidence   `json:"implementation,omitempty"`
	Inputs          []ArtifactEvidence `json:"inputs,omitempty"`
	Output          ArtifactEvidence   `json:"output,omitempty"`
	ReplayEvidence  []ArtifactEvidence `json:"replay_evidence,omitempty"`
	ForbiddenTokens []string           `json:"forbidden_tokens,omitempty"`
}

type ParityReport struct {
	Metric          string             `json:"metric,omitempty"`
	LearnedValue    *float64           `json:"learned_value,omitempty"`
	FallbackValue   *float64           `json:"fallback_value,omitempty"`
	Drift           float64            `json:"drift"`
	MaxAllowedDrift float64            `json:"max_allowed_drift"`
	WithinBound     bool               `json:"within_bound"`
	HigherIsBetter  bool               `json:"higher_is_better"`
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
		return Spec{}, fmt.Errorf("accelerator fallback spec version must be %s", SpecVersion)
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
		Criteria: criteria,
	}

	var counterexamples []Counterexample
	report.Evidence, counterexamples = collectArtifacts(rootAbs, spec.EvidencePaths, spec.Name, "run_evidence", false)
	report.Summary.EvidenceArtifacts += len(report.Evidence)
	if criteria.RequireEvidenceHashes && len(report.Evidence) == 0 {
		counterexamples = append(counterexamples, Counterexample{
			ID:      "run." + stableID(spec.Name, "evidence") + ".missing",
			Kind:    "missing_evidence",
			Subject: spec.Name,
			Message: "accelerator-fallbacks spec does not cite readable run-level evidence",
		})
	}

	discoveredSet := map[string]bool{}
	if criteria.RequireRepositoryDiscovery {
		discovered, err := DiscoverLearnedComponents(rootAbs)
		if err != nil {
			return Report{}, err
		}
		report.DiscoveredComponents = discovered
		report.Summary.DiscoveredComponents = len(discovered)
		for _, id := range discovered {
			discoveredSet[id] = true
		}
	}

	componentReports, componentCounterexamples, seen := buildComponentReports(rootAbs, spec.Components, discoveredSet, criteria)
	report.Components = componentReports
	counterexamples = append(counterexamples, componentCounterexamples...)
	report.Summary.Components = len(componentReports)
	for _, component := range componentReports {
		if component.Fallback.CPUOnly {
			report.Summary.CPUFallbacks++
		}
		if component.Fallback.AcceleratorFree {
			report.Summary.AcceleratorFreeFallbacks++
		}
		if component.Fallback.NoNetwork {
			report.Summary.NoNetworkFallbacks++
		}
		if component.Fallback.Deterministic {
			report.Summary.DeterministicFallbacks++
		}
		if component.Fallback.Seed != "" {
			report.Summary.StableSeeds++
		}
		if component.LearnedArtifact.Path != "" {
			report.Summary.PinnedLearnedArtifacts++
		}
		if component.Fallback.Implementation.Path != "" {
			report.Summary.PinnedImplementations++
		}
		report.Summary.PinnedInputArtifacts += len(component.Fallback.Inputs)
		if component.Fallback.Output.Path != "" {
			report.Summary.PinnedOutputArtifacts++
		}
		report.Summary.ReplayProofs += len(component.Fallback.ReplayEvidence)
		if component.Parity.WithinBound {
			report.Summary.ParityChecks++
		}
		report.Summary.EvidenceArtifacts += len(component.Source) + len(component.Evidence) + len(component.Fallback.Inputs) + len(component.Fallback.ReplayEvidence) + len(component.Parity.Evidence)
		if component.LearnedArtifact.Path != "" {
			report.Summary.EvidenceArtifacts++
		}
		if component.Fallback.Implementation.Path != "" {
			report.Summary.EvidenceArtifacts++
		}
		if component.Fallback.Output.Path != "" {
			report.Summary.EvidenceArtifacts++
		}
	}

	if len(componentReports) < criteria.MinComponents {
		counterexamples = append(counterexamples, Counterexample{
			ID:      "criteria.components.insufficient",
			Kind:    "insufficient_components",
			Message: fmt.Sprintf("components %d below required %d", len(componentReports), criteria.MinComponents),
		})
	}
	for _, id := range criteria.RequiredComponentIDs {
		normalized := normalizeID(id)
		if normalized == "" {
			continue
		}
		if !seen[normalized] {
			counterexamples = append(counterexamples, Counterexample{
				ID:      "criteria." + stableID(normalized, "required") + ".missing",
				Kind:    "missing_required_component",
				Subject: normalized,
				Message: "required learned component has no declared deterministic CPU fallback",
			})
		}
	}
	if criteria.RequireRepositoryDiscovery {
		for id := range discoveredSet {
			if !seen[id] {
				counterexamples = append(counterexamples, Counterexample{
					ID:      "inventory." + stableID(id, "fallback") + ".missing",
					Kind:    "missing_inventory_component",
					Subject: id,
					Message: "repository-discovered learned component has no declared deterministic CPU fallback",
				})
			}
		}
	}

	sortCounterexamples(counterexamples)
	report.Counterexamples = counterexamples
	report.Summary.Counterexamples = len(counterexamples)
	report.OK = len(counterexamples) == 0
	report.Hash = reportHash(report)
	return report, nil
}

func DiscoverLearnedComponents(root string) ([]string, error) {
	matches, err := filepath.Glob(filepath.Join(root, "examples", "*-gate.json"))
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	for _, match := range matches {
		name := strings.TrimSuffix(filepath.Base(match), "-gate.json")
		id := normalizeID(name)
		if id != "" && isLearnedComponentID(id) {
			seen[id] = true
		}
	}
	ids := make([]string, 0, len(seen))
	for id := range seen {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids, nil
}

func WriteArtifacts(outDir string, report Report) error {
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}
	file, err := os.Create(filepath.Join(outDir, "accelerator-fallbacks.json"))
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
	return os.WriteFile(filepath.Join(outDir, "accelerator-fallbacks.md"), []byte(RenderMarkdown(report)), 0o644)
}

func RenderMarkdown(report Report) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Deterministic accelerator-free fallbacks\n\n")
	fmt.Fprintf(&b, "Patchline verifies that every repository-discovered learned component has a deterministic CPU fallback with pinned inputs, outputs, implementation bytes, replay evidence, and bounded parity drift.\n\n")
	fmt.Fprintf(&b, "## Summary\n\n")
	fmt.Fprintf(&b, "| Metric | Value |\n| --- | ---: |\n")
	fmt.Fprintf(&b, "| OK | `%t` |\n", report.OK)
	fmt.Fprintf(&b, "| Discovered learned components | %d |\n", report.Summary.DiscoveredComponents)
	fmt.Fprintf(&b, "| Declared fallback components | %d |\n", report.Summary.Components)
	fmt.Fprintf(&b, "| CPU fallbacks | %d |\n", report.Summary.CPUFallbacks)
	fmt.Fprintf(&b, "| Accelerator-free fallbacks | %d |\n", report.Summary.AcceleratorFreeFallbacks)
	fmt.Fprintf(&b, "| No-network fallbacks | %d |\n", report.Summary.NoNetworkFallbacks)
	fmt.Fprintf(&b, "| Deterministic fallbacks | %d |\n", report.Summary.DeterministicFallbacks)
	fmt.Fprintf(&b, "| Replay proofs | %d |\n", report.Summary.ReplayProofs)
	fmt.Fprintf(&b, "| Parity checks | %d |\n", report.Summary.ParityChecks)
	fmt.Fprintf(&b, "| Counterexamples | %d |\n\n", report.Summary.Counterexamples)

	fmt.Fprintf(&b, "## Components\n\n")
	fmt.Fprintf(&b, "| Component | Kind | CPU | Accelerator-free | Deterministic | Drift | Bound |\n| --- | --- | ---: | ---: | ---: | ---: | ---: |\n")
	for _, component := range report.Components {
		fmt.Fprintf(&b, "| `%s` | `%s` | `%t` | `%t` | `%t` | %.6f | %.6f |\n",
			escapeTable(component.ID),
			escapeTable(component.Kind),
			component.Fallback.CPUOnly,
			component.Fallback.AcceleratorFree,
			component.Fallback.Deterministic,
			component.Parity.Drift,
			component.Parity.MaxAllowedDrift,
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
		return fmt.Errorf("accelerator fallback spec version must be %s", SpecVersion)
	}
	if strings.TrimSpace(spec.Name) == "" {
		return fmt.Errorf("accelerator fallback spec name is required")
	}
	return nil
}

func normalizeCriteria(criteria Criteria) Criteria {
	criteria.RequiredComponentIDs = normalizedIDs(criteria.RequiredComponentIDs)
	if criteria.MinReplayArtifacts <= 0 {
		criteria.MinReplayArtifacts = 2
	}
	if criteria.MaxAllowedDrift <= 0 {
		criteria.MaxAllowedDrift = 0.01
	}
	return criteria
}

func buildComponentReports(root string, components []Component, discovered map[string]bool, criteria Criteria) ([]ComponentReport, []Counterexample, map[string]bool) {
	sorted := append([]Component(nil), components...)
	sort.SliceStable(sorted, func(i, j int) bool {
		return normalizeID(sorted[i].ID) < normalizeID(sorted[j].ID)
	})
	var reports []ComponentReport
	var counterexamples []Counterexample
	seen := map[string]bool{}
	for _, component := range sorted {
		id := normalizeID(component.ID)
		if id == "" {
			counterexamples = append(counterexamples, Counterexample{
				ID:      "component.missing-id",
				Kind:    "missing_component_id",
				Message: "learned component fallback entry is missing an id",
			})
			continue
		}
		if seen[id] {
			counterexamples = append(counterexamples, Counterexample{
				ID:      "component." + stableID(id, "duplicate"),
				Kind:    "duplicate_component",
				Subject: id,
				Message: "learned component fallback entry is duplicated",
			})
			continue
		}
		seen[id] = true
		discoveredComponent := len(discovered) == 0 || discovered[id]
		if len(discovered) > 0 && !discovered[id] {
			counterexamples = append(counterexamples, Counterexample{
				ID:      "component." + stableID(id, "unknown"),
				Kind:    "unknown_learned_component",
				Subject: id,
				Message: "fallback entry does not match a repository-discovered learned component",
			})
		}
		kind := normalizeToken(component.Kind)
		if !allowedComponentKinds[kind] {
			counterexamples = append(counterexamples, Counterexample{
				ID:      "component." + stableID(id, kind, "kind") + ".unknown",
				Kind:    "unknown_component_kind",
				Subject: id,
				Message: "fallback entry uses an unknown learned-component kind",
				Witness: []string{component.Kind},
			})
		}

		source, sourceCounterexamples := collectArtifacts(root, component.SourcePaths, id, "source", false)
		counterexamples = append(counterexamples, sourceCounterexamples...)
		evidence, evidenceCounterexamples := collectArtifacts(root, component.EvidencePaths, id, "component_evidence", false)
		counterexamples = append(counterexamples, evidenceCounterexamples...)
		if criteria.RequireEvidenceHashes && len(evidence) == 0 {
			counterexamples = append(counterexamples, Counterexample{
				ID:      "component." + stableID(id, "evidence") + ".missing",
				Kind:    "missing_component_evidence",
				Subject: id,
				Message: "learned component fallback does not cite component-level evidence",
			})
		}

		learned, learnedCounterexamples := collectArtifact(root, component.LearnedArtifactPath, component.LearnedArtifactSHA256, id, "learned_artifact", criteria.RequirePinnedLearned)
		counterexamples = append(counterexamples, learnedCounterexamples...)
		fallbackReport, fallbackCounterexamples := buildFallbackReport(root, id, component.Fallback, criteria)
		counterexamples = append(counterexamples, fallbackCounterexamples...)
		parityReport, parityCounterexamples := buildParityReport(root, id, component.Parity, criteria)
		counterexamples = append(counterexamples, parityCounterexamples...)

		reports = append(reports, ComponentReport{
			ID:              id,
			Kind:            kind,
			Discovered:      discoveredComponent,
			Source:          source,
			LearnedArtifact: learned,
			Fallback:        fallbackReport,
			Parity:          parityReport,
			Evidence:        evidence,
		})
	}
	return reports, counterexamples, seen
}

func buildFallbackReport(root, id string, fallback Fallback, criteria Criteria) (FallbackReport, []Counterexample) {
	var counterexamples []Counterexample
	runtime := normalizeRuntime(fallback.Runtime)
	implementation, implementationCounterexamples := collectArtifact(root, fallback.ImplementationPath, fallback.ImplementationSHA256, id, "implementation", criteria.RequirePinnedImplementation)
	counterexamples = append(counterexamples, implementationCounterexamples...)
	forbiddenTokens := scanForbiddenTokens(root, fallback.ImplementationPath)
	if len(forbiddenTokens) > 0 {
		counterexamples = append(counterexamples, Counterexample{
			ID:      "fallback." + stableID(id, "forbidden-tokens"),
			Kind:    "forbidden_accelerator_or_network_token",
			Subject: id,
			Message: "fallback implementation references accelerator or network APIs",
			Witness: forbiddenTokens,
		})
	}

	var inputs []ArtifactEvidence
	for _, input := range fallback.InputArtifacts {
		artifact, inputCounterexamples := collectArtifact(root, input.Path, input.SHA256, id, "input_artifact", criteria.RequirePinnedInputs)
		counterexamples = append(counterexamples, inputCounterexamples...)
		if artifact.Path != "" {
			inputs = append(inputs, artifact)
		}
	}
	if criteria.RequirePinnedInputs && len(inputs) == 0 {
		counterexamples = append(counterexamples, Counterexample{
			ID:      "fallback." + stableID(id, "inputs") + ".missing",
			Kind:    "missing_input_artifact",
			Subject: id,
			Message: "CPU fallback has no pinned input artifact",
		})
	}
	output, outputCounterexamples := collectArtifact(root, fallback.OutputArtifact.Path, fallback.OutputArtifact.SHA256, id, "output_artifact", criteria.RequirePinnedOutputs)
	counterexamples = append(counterexamples, outputCounterexamples...)

	replayEvidence, replayCounterexamples := collectArtifacts(root, fallback.ReplayEvidencePaths, id, "replay_evidence", false)
	counterexamples = append(counterexamples, replayCounterexamples...)
	if criteria.RequireDeterministicReplay {
		if len(replayEvidence) < criteria.MinReplayArtifacts {
			counterexamples = append(counterexamples, Counterexample{
				ID:      "fallback." + stableID(id, "replay") + ".insufficient",
				Kind:    "missing_replay_evidence",
				Subject: id,
				Message: fmt.Sprintf("CPU fallback has %d replay artifacts below required %d", len(replayEvidence), criteria.MinReplayArtifacts),
			})
		}
		if len(replayEvidence) >= 2 {
			first := replayEvidence[0].SHA256
			for _, replay := range replayEvidence[1:] {
				if replay.SHA256 != first {
					counterexamples = append(counterexamples, Counterexample{
						ID:      "fallback." + stableID(id, "replay", replay.Path) + ".nondeterministic",
						Kind:    "nondeterministic_replay",
						Subject: id,
						Message: "fallback replay evidence differs across repeated CPU-only runs",
						Witness: []string{replayEvidence[0].Path, replay.Path},
					})
				}
			}
		}
		for _, replayPath := range fallback.ReplayEvidencePaths {
			if id != "" && output.SHA256 != "" {
				containsID, containsHash := artifactContains(root, replayPath, id), artifactContains(root, replayPath, output.SHA256)
				if !containsID || !containsHash {
					counterexamples = append(counterexamples, Counterexample{
						ID:      "fallback." + stableID(id, replayPath, "output-digest") + ".missing",
						Kind:    "replay_missing_component_output",
						Subject: id,
						Message: "replay evidence does not bind the component id to the pinned fallback output digest",
						Witness: []string{replayPath, output.SHA256},
					})
				}
			}
		}
	}

	cpuOnly := strings.Contains(runtime, "cpu") && !fallback.GPURequired && !fallback.AcceleratorRequired
	acceleratorFree := !fallback.GPURequired && !fallback.AcceleratorRequired && len(forbiddenTokens) == 0
	noNetwork := !fallback.NetworkAllowed && !containsForbidden(forbiddenTokens, "network")
	deterministic := fallback.Deterministic && strings.TrimSpace(fallback.Seed) != "" && len(replayEvidence) >= criteria.MinReplayArtifacts && len(counterexamplesOfKinds(counterexamples, "nondeterministic_replay", "replay_missing_component_output")) == 0

	if criteria.RequireCPUFallback && !cpuOnly {
		counterexamples = append(counterexamples, Counterexample{
			ID:      "fallback." + stableID(id, "cpu") + ".missing",
			Kind:    "missing_cpu_fallback",
			Subject: id,
			Message: "fallback runtime is not declared as CPU-only or still requires an accelerator",
			Witness: []string{fallback.Runtime},
		})
	}
	if criteria.RequireAcceleratorFree {
		if fallback.GPURequired {
			counterexamples = append(counterexamples, Counterexample{
				ID:      "fallback." + stableID(id, "gpu") + ".required",
				Kind:    "gpu_required",
				Subject: id,
				Message: "fallback requires a GPU",
			})
		}
		if fallback.AcceleratorRequired {
			counterexamples = append(counterexamples, Counterexample{
				ID:      "fallback." + stableID(id, "accelerator") + ".required",
				Kind:    "accelerator_required",
				Subject: id,
				Message: "fallback requires a non-CPU accelerator",
			})
		}
	}
	if criteria.RequireNoNetwork && fallback.NetworkAllowed {
		counterexamples = append(counterexamples, Counterexample{
			ID:      "fallback." + stableID(id, "network") + ".allowed",
			Kind:    "network_allowed",
			Subject: id,
			Message: "fallback permits network access",
		})
	}
	if criteria.RequireDeterministicReplay && !fallback.Deterministic {
		counterexamples = append(counterexamples, Counterexample{
			ID:      "fallback." + stableID(id, "deterministic") + ".missing",
			Kind:    "missing_deterministic_fallback",
			Subject: id,
			Message: "fallback is not declared deterministic",
		})
	}
	if criteria.RequireStableSeed && strings.TrimSpace(fallback.Seed) == "" {
		counterexamples = append(counterexamples, Counterexample{
			ID:      "fallback." + stableID(id, "seed") + ".missing",
			Kind:    "missing_stable_seed",
			Subject: id,
			Message: "fallback does not declare a stable seed",
		})
	}
	if fallback.Threads <= 0 {
		counterexamples = append(counterexamples, Counterexample{
			ID:      "fallback." + stableID(id, "threads") + ".invalid",
			Kind:    "invalid_thread_count",
			Subject: id,
			Message: "fallback must declare a positive CPU thread bound",
		})
	}

	return FallbackReport{
		Runtime:         runtime,
		CPUOnly:         cpuOnly,
		AcceleratorFree: acceleratorFree,
		NoNetwork:       noNetwork,
		Deterministic:   deterministic,
		Seed:            strings.TrimSpace(fallback.Seed),
		Threads:         fallback.Threads,
		Implementation:  implementation,
		Inputs:          inputs,
		Output:          output,
		ReplayEvidence:  replayEvidence,
		ForbiddenTokens: forbiddenTokens,
	}, counterexamples
}

func buildParityReport(root, id string, parity Parity, criteria Criteria) (ParityReport, []Counterexample) {
	var counterexamples []Counterexample
	evidence, evidenceCounterexamples := collectArtifacts(root, parity.EvidencePaths, id, "parity_evidence", false)
	counterexamples = append(counterexamples, evidenceCounterexamples...)
	bound := criteria.MaxAllowedDrift
	if parity.MaxAllowedDrift != nil && *parity.MaxAllowedDrift < bound {
		bound = *parity.MaxAllowedDrift
	}
	report := ParityReport{
		Metric:          normalizeToken(parity.Metric),
		LearnedValue:    parity.LearnedValue,
		FallbackValue:   parity.FallbackValue,
		MaxAllowedDrift: bound,
		HigherIsBetter:  parity.HigherIsBetter,
		Evidence:        evidence,
	}
	if parity.LearnedValue != nil && parity.FallbackValue != nil {
		report.Drift = math.Abs(*parity.LearnedValue - *parity.FallbackValue)
		report.WithinBound = bound > 0 && report.Drift <= bound+1e-12
	}
	if criteria.RequireParityEvidence {
		if report.Metric == "" || parity.LearnedValue == nil || parity.FallbackValue == nil || len(evidence) == 0 {
			counterexamples = append(counterexamples, Counterexample{
				ID:      "parity." + stableID(id, "evidence") + ".missing",
				Kind:    "missing_parity_evidence",
				Subject: id,
				Message: "fallback parity evidence must include a metric, learned value, fallback value, and readable evidence",
			})
		}
		if bound <= 0 {
			counterexamples = append(counterexamples, Counterexample{
				ID:      "parity." + stableID(id, "bound") + ".invalid",
				Kind:    "invalid_parity_bound",
				Subject: id,
				Message: "fallback parity bound must be positive",
			})
		}
		if report.Metric != "" && parity.LearnedValue != nil && parity.FallbackValue != nil && !report.WithinBound {
			counterexamples = append(counterexamples, Counterexample{
				ID:      "parity." + stableID(id, report.Metric, "drift") + ".exceeds",
				Kind:    "parity_drift_exceeds_bound",
				Subject: id,
				Message: fmt.Sprintf("fallback parity drift %.6f exceeds bound %.6f", report.Drift, bound),
			})
		}
	}
	return report, counterexamples
}

var allowedComponentKinds = map[string]bool{
	"abstraction_policy":    true,
	"active_learning":       true,
	"adversarial_hardening": true,
	"continual_learning":    true,
	"foundation_model":      true,
	"interpretability":      true,
	"methodology":           true,
	"neuro_symbolic":        true,
	"online_learning":       true,
	"policy_learning":       true,
	"pretraining":           true,
	"program_repair":        true,
	"review_policy":         true,
	"risk_model":            true,
	"safety_case":           true,
	"transfer_learning":     true,
	"trust_regression":      true,
	"verdict_prior":         true,
}

func isLearnedComponentID(id string) bool {
	if strings.HasPrefix(id, "rl-") || strings.Contains(id, "-rl-") {
		return true
	}
	exact := map[string]bool{
		"abstraction-selection-policy":    true,
		"adversarial-training-loop":       true,
		"extractable-neuro-symbolic":      true,
		"human-trust-regression":          true,
		"interpretability-probe":          true,
		"mechanistic-decision-circuits":   true,
		"reviewer-action-model":           true,
		"schema-foundation-model":         true,
		"schema-self-supervised-pretrain": true,
	}
	if exact[id] {
		return true
	}
	for _, token := range []string{
		"learned",
		"learning",
		"neuro",
		"neurosymbolic",
		"foundation-model",
		"self-supervised",
		"pretrain",
		"adversarial-training",
		"interpretability-probe",
		"mechanistic-decision-circuits",
	} {
		if strings.Contains(id, token) {
			return true
		}
	}
	return false
}

func collectArtifacts(root string, paths []string, subject, role string, required bool) ([]ArtifactEvidence, []Counterexample) {
	var artifacts []ArtifactEvidence
	var counterexamples []Counterexample
	for _, path := range sortedStrings(paths) {
		artifact, artifactCounterexamples := collectArtifact(root, path, "", subject, role, required)
		counterexamples = append(counterexamples, artifactCounterexamples...)
		if artifact.Path != "" {
			artifacts = append(artifacts, artifact)
		}
	}
	return artifacts, counterexamples
}

func collectArtifact(root, rel, expected, subject, role string, requirePinned bool) (ArtifactEvidence, []Counterexample) {
	rel = strings.TrimSpace(rel)
	expected = normalizeHash(expected)
	var counterexamples []Counterexample
	if rel == "" {
		counterexamples = append(counterexamples, Counterexample{
			ID:      role + "." + stableID(subject, "path") + ".missing",
			Kind:    "missing_" + role,
			Subject: subject,
			Message: role + " path is required",
		})
		return ArtifactEvidence{}, counterexamples
	}
	path, ok := safeJoin(root, rel)
	if !ok {
		counterexamples = append(counterexamples, Counterexample{
			ID:      role + "." + stableID(subject, rel, "path") + ".invalid",
			Kind:    "invalid_" + role + "_path",
			Subject: subject,
			Message: role + " path escapes the repository root",
			Witness: []string{rel},
		})
		return ArtifactEvidence{}, counterexamples
	}
	data, err := os.ReadFile(path)
	if err != nil {
		counterexamples = append(counterexamples, Counterexample{
			ID:      role + "." + stableID(subject, rel, "read") + ".missing",
			Kind:    "missing_" + role,
			Subject: subject,
			Message: role + " could not be read",
			Witness: []string{rel, err.Error()},
		})
		return ArtifactEvidence{}, counterexamples
	}
	if len(data) == 0 {
		counterexamples = append(counterexamples, Counterexample{
			ID:      role + "." + stableID(subject, rel, "empty") + ".empty",
			Kind:    "empty_" + role,
			Subject: subject,
			Message: role + " is empty",
			Witness: []string{rel},
		})
	}
	actual := hashBytes(data)
	if requirePinned && expected == "" {
		counterexamples = append(counterexamples, Counterexample{
			ID:      role + "." + stableID(subject, rel, "hash") + ".missing",
			Kind:    "missing_" + role + "_hash",
			Subject: subject,
			Message: role + " is not pinned to a sha256 digest",
			Witness: []string{rel},
		})
	}
	if expected != "" && actual != expected {
		counterexamples = append(counterexamples, Counterexample{
			ID:      role + "." + stableID(subject, rel, "hash") + ".mismatch",
			Kind:    role + "_hash_mismatch",
			Subject: subject,
			Message: role + " digest does not match its pinned sha256",
			Witness: []string{rel, "got " + actual, "want " + expected},
		})
	}
	return ArtifactEvidence{Path: filepath.ToSlash(rel), SHA256: actual, Bytes: int64(len(data))}, counterexamples
}

func scanForbiddenTokens(root, rel string) []string {
	rel = strings.TrimSpace(rel)
	if rel == "" {
		return nil
	}
	path, ok := safeJoin(root, rel)
	if !ok {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	text := strings.ToLower(string(data))
	var found []string
	tokens := map[string]string{
		"cuda":       "accelerator:cuda",
		"torch.cuda": "accelerator:torch.cuda",
		"tensorflow": "accelerator:tensorflow",
		"jax.":       "accelerator:jax",
		"opencl":     "accelerator:opencl",
		"rocm":       "accelerator:rocm",
		" tpu":       "accelerator:tpu",
		"net/http":   "network:net/http",
		"http.get":   "network:http.get",
		"net.dial":   "network:net.dial",
		"socket":     "network:socket",
	}
	for token, label := range tokens {
		if strings.Contains(text, token) {
			found = append(found, label)
		}
	}
	sort.Strings(found)
	return found
}

func safeJoin(root, rel string) (string, bool) {
	if filepath.IsAbs(rel) {
		return "", false
	}
	clean := filepath.Clean(filepath.FromSlash(rel))
	if clean == "." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) || clean == ".." {
		return "", false
	}
	rootClean := filepath.Clean(root)
	path := filepath.Join(rootClean, clean)
	if path != rootClean && !strings.HasPrefix(path, rootClean+string(filepath.Separator)) {
		return "", false
	}
	return path, true
}

func artifactContains(root, rel, needle string) bool {
	if needle == "" {
		return false
	}
	path, ok := safeJoin(root, rel)
	if !ok {
		return false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	return strings.Contains(string(data), needle)
}

func reportHash(report Report) string {
	report.Hash = ""
	data, err := json.Marshal(report)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func hashBytes(data []byte) string {
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func normalizeHash(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	if strings.HasPrefix(value, "sha256:") && len(value) == len("sha256:")+64 {
		for _, r := range strings.TrimPrefix(value, "sha256:") {
			if !('0' <= r && r <= '9') && !('a' <= r && r <= 'f') {
				return ""
			}
		}
		return value
	}
	return ""
}

func normalizeID(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	var b strings.Builder
	lastDash := false
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			lastDash = false
		case r == '-' || r == '_' || unicode.IsSpace(r):
			if !lastDash && b.Len() > 0 {
				b.WriteByte('-')
				lastDash = true
			}
		}
	}
	return strings.Trim(b.String(), "-")
}

func normalizeRuntime(value string) string {
	return strings.Join(strings.Fields(strings.ToLower(strings.TrimSpace(value))), "-")
}

func normalizeToken(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	value = strings.ReplaceAll(value, "-", "_")
	var b strings.Builder
	lastUnderscore := false
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			lastUnderscore = false
		case r == '_' || unicode.IsSpace(r):
			if !lastUnderscore && b.Len() > 0 {
				b.WriteByte('_')
				lastUnderscore = true
			}
		}
	}
	return strings.Trim(b.String(), "_")
}

func normalizedIDs(values []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, value := range values {
		normalized := normalizeID(value)
		if normalized != "" && !seen[normalized] {
			seen[normalized] = true
			out = append(out, normalized)
		}
	}
	sort.Strings(out)
	return out
}

func sortedStrings(values []string) []string {
	out := append([]string(nil), values...)
	sort.Strings(out)
	return out
}

func sortCounterexamples(counterexamples []Counterexample) {
	sort.SliceStable(counterexamples, func(i, j int) bool {
		if counterexamples[i].ID == counterexamples[j].ID {
			return counterexamples[i].Kind < counterexamples[j].Kind
		}
		return counterexamples[i].ID < counterexamples[j].ID
	})
}

func stableID(parts ...string) string {
	joined := strings.Join(parts, "\x00")
	sum := sha256.Sum256([]byte(joined))
	return hex.EncodeToString(sum[:])[:16]
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func escapeTable(value string) string {
	value = strings.ReplaceAll(value, "|", `\|`)
	value = strings.ReplaceAll(value, "\n", " ")
	return value
}

func containsForbidden(tokens []string, prefix string) bool {
	for _, token := range tokens {
		if strings.HasPrefix(token, prefix+":") {
			return true
		}
	}
	return false
}

func counterexamplesOfKinds(counterexamples []Counterexample, kinds ...string) []Counterexample {
	allowed := map[string]bool{}
	for _, kind := range kinds {
		allowed[kind] = true
	}
	var out []Counterexample
	for _, counterexample := range counterexamples {
		if allowed[counterexample.Kind] {
			out = append(out, counterexample)
		}
	}
	return out
}
