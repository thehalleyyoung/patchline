package resourceprofile

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/thehalleyyoung/patchline/internal/canonical"
)

const SpecVersion = "patchline.resource-profiles/v1"
const ReportVersion = "patchline.resource-profiles-report/v1"

type Spec struct {
	Version       string    `json:"version"`
	Name          string    `json:"name"`
	Claim         string    `json:"claim,omitempty"`
	Criteria      Criteria  `json:"criteria"`
	Profiles      []Profile `json:"profiles"`
	EvidencePaths []string  `json:"evidence_paths,omitempty"`
}

type Criteria struct {
	RequiredProfileIDs          []string `json:"required_profile_ids,omitempty"`
	RequiredTiers               []string `json:"required_tiers,omitempty"`
	MinProfiles                 int      `json:"min_profiles"`
	MinCommandsPerProfile       int      `json:"min_commands_per_profile"`
	MinBudgetsPerProfile        int      `json:"min_budgets_per_profile"`
	RequireEvidenceHashes       bool     `json:"require_evidence_hashes"`
	RequireDeterministic        bool     `json:"require_deterministic"`
	RequireOfflineProfile       bool     `json:"require_offline_profile"`
	RequireCIProfile            bool     `json:"require_ci_profile"`
	RequireLaptopProfile        bool     `json:"require_laptop_profile"`
	RequireHostedServiceProfile bool     `json:"require_hosted_service_profile"`
	RequireNoNetworkWhenOffline bool     `json:"require_no_network_when_offline"`
	RequireCacheStrategy        bool     `json:"require_cache_strategy"`
	RequireNativeTestPolicy     bool     `json:"require_native_test_policy"`
	RequireGracefulDegradation  bool     `json:"require_graceful_degradation"`
	MaxLaptopCPU                int      `json:"max_laptop_cpu"`
	MaxLaptopMemoryMB           int      `json:"max_laptop_memory_mb"`
	MaxCIMinutes                int      `json:"max_ci_minutes"`
	MaxHostedCostCents          int      `json:"max_hosted_cost_cents"`
}

type Profile struct {
	ID                  string            `json:"id"`
	Tier                string            `json:"tier"`
	Description         string            `json:"description,omitempty"`
	Constraints         Constraints       `json:"constraints"`
	Budgets             []Budget          `json:"budgets,omitempty"`
	Commands            []Command         `json:"commands,omitempty"`
	CacheStrategy       string            `json:"cache_strategy,omitempty"`
	NativeTestPolicy    string            `json:"native_test_policy,omitempty"`
	DegradationBehavior string            `json:"degradation_behavior,omitempty"`
	Outputs             []ArtifactRef     `json:"outputs,omitempty"`
	EvidencePaths       []string          `json:"evidence_paths,omitempty"`
	Metadata            map[string]string `json:"metadata,omitempty"`
}

type Constraints struct {
	CPU               int  `json:"cpu"`
	MemoryMB          int  `json:"memory_mb"`
	TimeoutMinutes    int  `json:"timeout_minutes"`
	MaxCostCents      int  `json:"max_cost_cents,omitempty"`
	NetworkAllowed    bool `json:"network_allowed"`
	NativeTests       bool `json:"native_tests"`
	LLMAllowed        bool `json:"llm_allowed"`
	HostedMultiTenant bool `json:"hosted_multi_tenant,omitempty"`
}

type Budget struct {
	Name    string `json:"name"`
	Files   int    `json:"files"`
	Lines   int    `json:"lines"`
	Tokens  int    `json:"tokens"`
	Changes int    `json:"changes"`
}

type Command struct {
	Stage string   `json:"stage"`
	Args  []string `json:"args"`
}

type ArtifactRef struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

type Report struct {
	Version         string             `json:"version"`
	Name            string             `json:"name"`
	OK              bool               `json:"ok"`
	Criteria        Criteria           `json:"criteria"`
	Summary         Summary            `json:"summary"`
	Evidence        []ArtifactEvidence `json:"evidence,omitempty"`
	Profiles        []ProfileReport    `json:"profiles"`
	Counterexamples []Counterexample   `json:"counterexamples,omitempty"`
	Hash            string             `json:"hash"`
}

type Summary struct {
	Profiles              int `json:"profiles"`
	Tiers                 int `json:"tiers"`
	LaptopProfiles        int `json:"laptop_profiles"`
	CIProfiles            int `json:"ci_profiles"`
	OfflineProfiles       int `json:"offline_profiles"`
	HostedProfiles        int `json:"hosted_profiles"`
	DeterministicProfiles int `json:"deterministic_profiles"`
	NoNetworkProfiles     int `json:"no_network_profiles"`
	CachedProfiles        int `json:"cached_profiles"`
	NativeTestPolicies    int `json:"native_test_policies"`
	GracefulDegradations  int `json:"graceful_degradations"`
	CommandPlans          int `json:"command_plans"`
	Budgets               int `json:"budgets"`
	EvidenceArtifacts     int `json:"evidence_artifacts"`
	Counterexamples       int `json:"counterexamples"`
}

type ProfileReport struct {
	ID                  string             `json:"id"`
	Tier                string             `json:"tier"`
	Deterministic       bool               `json:"deterministic"`
	NoNetwork           bool               `json:"no_network"`
	WithinCPU           bool               `json:"within_cpu"`
	WithinMemory        bool               `json:"within_memory"`
	WithinTime          bool               `json:"within_time"`
	WithinCost          bool               `json:"within_cost"`
	HasCacheStrategy    bool               `json:"has_cache_strategy"`
	HasNativeTestPolicy bool               `json:"has_native_test_policy"`
	HasDegradation      bool               `json:"has_degradation"`
	Commands            []Command          `json:"commands,omitempty"`
	Budgets             []Budget           `json:"budgets,omitempty"`
	Outputs             []ArtifactEvidence `json:"outputs,omitempty"`
	Evidence            []ArtifactEvidence `json:"evidence,omitempty"`
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
		return Spec{}, fmt.Errorf("resource profiles spec version must be %s", SpecVersion)
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
			Kind:    "missing_run_evidence",
			Subject: spec.Name,
			Message: "resource profile spec does not cite readable run-level evidence",
		})
	}

	profiles, profileCounterexamples, seen, tiers := buildProfileReports(rootAbs, spec.Profiles, criteria)
	report.Profiles = profiles
	counterexamples = append(counterexamples, profileCounterexamples...)
	report.Summary.Profiles = len(profiles)
	report.Summary.Tiers = len(tiers)
	for _, profile := range profiles {
		switch profile.Tier {
		case "laptop":
			report.Summary.LaptopProfiles++
		case "ci":
			report.Summary.CIProfiles++
		case "air-gapped":
			report.Summary.OfflineProfiles++
		case "hosted-public-good":
			report.Summary.HostedProfiles++
		}
		if profile.Deterministic {
			report.Summary.DeterministicProfiles++
		}
		if profile.NoNetwork {
			report.Summary.NoNetworkProfiles++
		}
		if profile.HasCacheStrategy {
			report.Summary.CachedProfiles++
		}
		if profile.HasNativeTestPolicy {
			report.Summary.NativeTestPolicies++
		}
		if profile.HasDegradation {
			report.Summary.GracefulDegradations++
		}
		report.Summary.CommandPlans += len(profile.Commands)
		report.Summary.Budgets += len(profile.Budgets)
		report.Summary.EvidenceArtifacts += len(profile.Evidence) + len(profile.Outputs)
	}

	if len(profiles) < criteria.MinProfiles {
		counterexamples = append(counterexamples, Counterexample{
			ID:      "criteria.profiles.insufficient",
			Kind:    "insufficient_profiles",
			Message: fmt.Sprintf("profiles %d below required %d", len(profiles), criteria.MinProfiles),
		})
	}
	for _, id := range criteria.RequiredProfileIDs {
		normalized := normalizeID(id)
		if normalized != "" && !seen[normalized] {
			counterexamples = append(counterexamples, Counterexample{
				ID:      "criteria." + stableID(normalized, "required") + ".missing",
				Kind:    "missing_required_profile",
				Subject: normalized,
				Message: "required resource-adaptive profile is not declared",
			})
		}
	}
	for _, tier := range criteria.RequiredTiers {
		normalized := normalizeTier(tier)
		if normalized != "" && !tiers[normalized] {
			counterexamples = append(counterexamples, Counterexample{
				ID:      "criteria." + stableID(normalized, "tier") + ".missing",
				Kind:    "missing_required_tier",
				Subject: normalized,
				Message: "required resource tier has no profile",
			})
		}
	}
	if criteria.RequireLaptopProfile && report.Summary.LaptopProfiles == 0 {
		counterexamples = append(counterexamples, requiredTierCounterexample("laptop"))
	}
	if criteria.RequireCIProfile && report.Summary.CIProfiles == 0 {
		counterexamples = append(counterexamples, requiredTierCounterexample("ci"))
	}
	if criteria.RequireOfflineProfile && report.Summary.OfflineProfiles == 0 {
		counterexamples = append(counterexamples, requiredTierCounterexample("air-gapped"))
	}
	if criteria.RequireHostedServiceProfile && report.Summary.HostedProfiles == 0 {
		counterexamples = append(counterexamples, requiredTierCounterexample("hosted-public-good"))
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
	file, err := os.Create(filepath.Join(outDir, "resource-profiles.json"))
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
	return os.WriteFile(filepath.Join(outDir, "resource-profiles.md"), []byte(RenderMarkdown(report)), 0o644)
}

func RenderMarkdown(report Report) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Resource-adaptive analysis profiles\n\n")
	fmt.Fprintf(&b, "Patchline proves each operating environment maps to deterministic command plans, explicit budgets, cache policy, native-test policy, graceful degradation, and hash-backed evidence.\n\n")
	fmt.Fprintf(&b, "## Summary\n\n")
	fmt.Fprintf(&b, "| Metric | Value |\n| --- | ---: |\n")
	fmt.Fprintf(&b, "| OK | `%t` |\n", report.OK)
	fmt.Fprintf(&b, "| Profiles | %d |\n", report.Summary.Profiles)
	fmt.Fprintf(&b, "| Tiers | %d |\n", report.Summary.Tiers)
	fmt.Fprintf(&b, "| Laptop / CI / offline / hosted | %d / %d / %d / %d |\n", report.Summary.LaptopProfiles, report.Summary.CIProfiles, report.Summary.OfflineProfiles, report.Summary.HostedProfiles)
	fmt.Fprintf(&b, "| Deterministic profiles | %d |\n", report.Summary.DeterministicProfiles)
	fmt.Fprintf(&b, "| No-network profiles | %d |\n", report.Summary.NoNetworkProfiles)
	fmt.Fprintf(&b, "| Command plans | %d |\n", report.Summary.CommandPlans)
	fmt.Fprintf(&b, "| Budgets | %d |\n", report.Summary.Budgets)
	fmt.Fprintf(&b, "| Counterexamples | %d |\n\n", report.Summary.Counterexamples)
	fmt.Fprintf(&b, "## Profiles\n\n")
	fmt.Fprintf(&b, "| Profile | Tier | CPU | Memory MB | Time min | Cost cents | Commands | Budgets | Cache | Degrades |\n| --- | --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |\n")
	for _, profile := range report.Profiles {
		fmt.Fprintf(&b, "| `%s` | `%s` | `%t` | `%t` | `%t` | `%t` | %d | %d | `%t` | `%t` |\n",
			escapeTable(profile.ID),
			escapeTable(profile.Tier),
			profile.WithinCPU,
			profile.WithinMemory,
			profile.WithinTime,
			profile.WithinCost,
			len(profile.Commands),
			len(profile.Budgets),
			profile.HasCacheStrategy,
			profile.HasDegradation,
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
		return fmt.Errorf("resource profiles spec version must be %s", SpecVersion)
	}
	if strings.TrimSpace(spec.Name) == "" {
		return fmt.Errorf("resource profiles spec name is required")
	}
	return nil
}

func normalizeCriteria(criteria Criteria) Criteria {
	criteria.RequiredProfileIDs = normalizedIDs(criteria.RequiredProfileIDs)
	for i := range criteria.RequiredTiers {
		criteria.RequiredTiers[i] = normalizeTier(criteria.RequiredTiers[i])
	}
	sort.Strings(criteria.RequiredTiers)
	criteria.RequiredTiers = compactStrings(criteria.RequiredTiers)
	if criteria.MinCommandsPerProfile <= 0 {
		criteria.MinCommandsPerProfile = 1
	}
	if criteria.MinBudgetsPerProfile <= 0 {
		criteria.MinBudgetsPerProfile = 1
	}
	if criteria.MaxLaptopCPU <= 0 {
		criteria.MaxLaptopCPU = 4
	}
	if criteria.MaxLaptopMemoryMB <= 0 {
		criteria.MaxLaptopMemoryMB = 8192
	}
	if criteria.MaxCIMinutes <= 0 {
		criteria.MaxCIMinutes = 20
	}
	if criteria.MaxHostedCostCents <= 0 {
		criteria.MaxHostedCostCents = 50
	}
	return criteria
}

func buildProfileReports(root string, profiles []Profile, criteria Criteria) ([]ProfileReport, []Counterexample, map[string]bool, map[string]bool) {
	sorted := append([]Profile(nil), profiles...)
	sort.SliceStable(sorted, func(i, j int) bool {
		return normalizeID(sorted[i].ID) < normalizeID(sorted[j].ID)
	})
	var reports []ProfileReport
	var counterexamples []Counterexample
	seen := map[string]bool{}
	tiers := map[string]bool{}
	for _, profile := range sorted {
		id := normalizeID(profile.ID)
		if id == "" {
			counterexamples = append(counterexamples, Counterexample{ID: "profile.missing-id", Kind: "missing_profile_id", Message: "resource profile entry is missing an id"})
			continue
		}
		if seen[id] {
			counterexamples = append(counterexamples, Counterexample{ID: "profile." + stableID(id, "duplicate"), Kind: "duplicate_profile", Subject: id, Message: "resource profile entry is duplicated"})
			continue
		}
		seen[id] = true
		tier := normalizeTier(profile.Tier)
		if !allowedTiers[tier] {
			counterexamples = append(counterexamples, Counterexample{ID: "profile." + stableID(id, tier, "tier") + ".unknown", Kind: "unknown_profile_tier", Subject: id, Message: "resource profile tier is unknown", Witness: []string{profile.Tier}})
		} else {
			tiers[tier] = true
		}
		evidence, evidenceCounterexamples := collectArtifacts(root, profile.EvidencePaths, id, "profile_evidence", false)
		counterexamples = append(counterexamples, evidenceCounterexamples...)
		if criteria.RequireEvidenceHashes && len(evidence) == 0 {
			counterexamples = append(counterexamples, Counterexample{ID: "profile." + stableID(id, "evidence") + ".missing", Kind: "missing_profile_evidence", Subject: id, Message: "resource profile does not cite readable profile-level evidence"})
		}
		outputs, outputCounterexamples := collectArtifactRefs(root, profile.Outputs, id)
		counterexamples = append(counterexamples, outputCounterexamples...)
		profileCounterexamples := validateProfile(profile, id, tier, criteria)
		counterexamples = append(counterexamples, profileCounterexamples...)
		reports = append(reports, ProfileReport{
			ID:                  id,
			Tier:                tier,
			Deterministic:       profileDeterministic(profile),
			NoNetwork:           !profile.Constraints.NetworkAllowed,
			WithinCPU:           withinCPU(profile, tier, criteria),
			WithinMemory:        withinMemory(profile, tier, criteria),
			WithinTime:          withinTime(profile, tier, criteria),
			WithinCost:          withinCost(profile, tier, criteria),
			HasCacheStrategy:    strings.TrimSpace(profile.CacheStrategy) != "",
			HasNativeTestPolicy: strings.TrimSpace(profile.NativeTestPolicy) != "",
			HasDegradation:      strings.TrimSpace(profile.DegradationBehavior) != "",
			Commands:            normalizedCommands(profile.Commands),
			Budgets:             normalizedBudgets(profile.Budgets),
			Outputs:             outputs,
			Evidence:            evidence,
		})
	}
	return reports, counterexamples, seen, tiers
}

func validateProfile(profile Profile, id, tier string, criteria Criteria) []Counterexample {
	var counterexamples []Counterexample
	if criteria.RequireDeterministic && !profileDeterministic(profile) {
		counterexamples = append(counterexamples, Counterexample{ID: "profile." + stableID(id, "deterministic") + ".missing", Kind: "nondeterministic_profile", Subject: id, Message: "profile command plan does not use deterministic Patchline stages"})
	}
	if len(profile.Commands) < criteria.MinCommandsPerProfile {
		counterexamples = append(counterexamples, Counterexample{ID: "profile." + stableID(id, "commands") + ".insufficient", Kind: "insufficient_commands", Subject: id, Message: fmt.Sprintf("profile has %d commands below required %d", len(profile.Commands), criteria.MinCommandsPerProfile)})
	}
	if len(profile.Budgets) < criteria.MinBudgetsPerProfile {
		counterexamples = append(counterexamples, Counterexample{ID: "profile." + stableID(id, "budgets") + ".insufficient", Kind: "insufficient_budgets", Subject: id, Message: fmt.Sprintf("profile has %d budgets below required %d", len(profile.Budgets), criteria.MinBudgetsPerProfile)})
	}
	for _, budget := range profile.Budgets {
		if budget.Files <= 0 || budget.Lines <= 0 || budget.Tokens <= 0 || budget.Changes <= 0 {
			counterexamples = append(counterexamples, Counterexample{ID: "profile." + stableID(id, budget.Name, "budget") + ".invalid", Kind: "invalid_budget", Subject: id, Message: "profile budget must bound files, lines, tokens, and changes with positive values"})
		}
	}
	for _, command := range profile.Commands {
		if len(command.Args) == 0 || command.Args[0] != "patchline" {
			counterexamples = append(counterexamples, Counterexample{ID: "profile." + stableID(id, command.Stage, "command") + ".invalid", Kind: "invalid_command_plan", Subject: id, Message: "profile command must start with patchline", Witness: command.Args})
			continue
		}
		if containsAny(command.Args, "--llm-command") && !profile.Constraints.LLMAllowed {
			counterexamples = append(counterexamples, Counterexample{ID: "profile." + stableID(id, command.Stage, "llm") + ".forbidden", Kind: "llm_not_allowed", Subject: id, Message: "profile disallows LLMs but command plan enables an LLM command", Witness: command.Args})
		}
		if containsAny(command.Args, "--run-native-tests") && !profile.Constraints.NativeTests {
			counterexamples = append(counterexamples, Counterexample{ID: "profile." + stableID(id, command.Stage, "native") + ".forbidden", Kind: "native_tests_not_allowed", Subject: id, Message: "profile disallows native tests but command plan enables them", Witness: command.Args})
		}
	}
	if criteria.RequireNoNetworkWhenOffline && tier == "air-gapped" && profile.Constraints.NetworkAllowed {
		counterexamples = append(counterexamples, Counterexample{ID: "profile." + stableID(id, "offline-network") + ".allowed", Kind: "offline_network_allowed", Subject: id, Message: "air-gapped profile allows network access"})
	}
	if criteria.RequireCacheStrategy && strings.TrimSpace(profile.CacheStrategy) == "" {
		counterexamples = append(counterexamples, Counterexample{ID: "profile." + stableID(id, "cache") + ".missing", Kind: "missing_cache_strategy", Subject: id, Message: "profile does not declare cache reuse or warming behavior"})
	}
	if criteria.RequireNativeTestPolicy && strings.TrimSpace(profile.NativeTestPolicy) == "" {
		counterexamples = append(counterexamples, Counterexample{ID: "profile." + stableID(id, "native-tests") + ".missing", Kind: "missing_native_test_policy", Subject: id, Message: "profile does not declare whether safe native tests may run"})
	}
	if criteria.RequireGracefulDegradation && strings.TrimSpace(profile.DegradationBehavior) == "" {
		counterexamples = append(counterexamples, Counterexample{ID: "profile." + stableID(id, "degradation") + ".missing", Kind: "missing_degradation_behavior", Subject: id, Message: "profile does not explain graceful degradation when resources or optional tools are unavailable"})
	}
	if !withinCPU(profile, tier, criteria) {
		counterexamples = append(counterexamples, Counterexample{ID: "profile." + stableID(id, "cpu") + ".over", Kind: "cpu_budget_exceeded", Subject: id, Message: "profile exceeds its tier CPU budget"})
	}
	if !withinMemory(profile, tier, criteria) {
		counterexamples = append(counterexamples, Counterexample{ID: "profile." + stableID(id, "memory") + ".over", Kind: "memory_budget_exceeded", Subject: id, Message: "profile exceeds its tier memory budget"})
	}
	if !withinTime(profile, tier, criteria) {
		counterexamples = append(counterexamples, Counterexample{ID: "profile." + stableID(id, "timeout") + ".over", Kind: "time_budget_exceeded", Subject: id, Message: "profile exceeds its tier time budget"})
	}
	if !withinCost(profile, tier, criteria) {
		counterexamples = append(counterexamples, Counterexample{ID: "profile." + stableID(id, "cost") + ".over", Kind: "cost_budget_exceeded", Subject: id, Message: "profile exceeds its hosted-service cost budget"})
	}
	return counterexamples
}

func collectArtifactRefs(root string, refs []ArtifactRef, subject string) ([]ArtifactEvidence, []Counterexample) {
	var artifacts []ArtifactEvidence
	var counterexamples []Counterexample
	for _, ref := range refs {
		artifact, artifactCounterexamples := collectArtifact(root, ref.Path, ref.SHA256, subject, "output", true)
		counterexamples = append(counterexamples, artifactCounterexamples...)
		if artifact.Path != "" {
			artifacts = append(artifacts, artifact)
		}
	}
	return artifacts, counterexamples
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
		if require {
			return ArtifactEvidence{}, []Counterexample{{ID: kind + "." + stableID(subject, "missing") + ".missing", Kind: "missing_" + kind, Subject: subject, Message: kind + " path is required"}}
		}
		return ArtifactEvidence{}, nil
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

func profileDeterministic(profile Profile) bool {
	if profile.Constraints.LLMAllowed {
		return false
	}
	for _, command := range profile.Commands {
		if containsAny(command.Args, "--llm-command", "--prompt-without-facts") {
			return false
		}
		if len(command.Args) >= 2 && command.Args[0] == "patchline" && command.Args[1] == "repo" {
			if containsAny(command.Args, "analyze", "propose") && !containsAny(command.Args, "--no-llm") {
				return false
			}
		}
	}
	return len(profile.Commands) > 0
}

func withinCPU(profile Profile, tier string, criteria Criteria) bool {
	if profile.Constraints.CPU <= 0 {
		return false
	}
	if tier == "laptop" {
		return profile.Constraints.CPU <= criteria.MaxLaptopCPU
	}
	return true
}

func withinMemory(profile Profile, tier string, criteria Criteria) bool {
	if profile.Constraints.MemoryMB <= 0 {
		return false
	}
	if tier == "laptop" {
		return profile.Constraints.MemoryMB <= criteria.MaxLaptopMemoryMB
	}
	return true
}

func withinTime(profile Profile, tier string, criteria Criteria) bool {
	if profile.Constraints.TimeoutMinutes <= 0 {
		return false
	}
	if tier == "ci" {
		return profile.Constraints.TimeoutMinutes <= criteria.MaxCIMinutes
	}
	return true
}

func withinCost(profile Profile, tier string, criteria Criteria) bool {
	if tier != "hosted-public-good" {
		return true
	}
	return profile.Constraints.MaxCostCents > 0 && profile.Constraints.MaxCostCents <= criteria.MaxHostedCostCents
}

func normalizedCommands(commands []Command) []Command {
	out := append([]Command(nil), commands...)
	sort.SliceStable(out, func(i, j int) bool {
		return normalizeToken(out[i].Stage) < normalizeToken(out[j].Stage)
	})
	return out
}

func normalizedBudgets(budgets []Budget) []Budget {
	out := append([]Budget(nil), budgets...)
	sort.SliceStable(out, func(i, j int) bool {
		return normalizeToken(out[i].Name) < normalizeToken(out[j].Name)
	})
	return out
}

func requiredTierCounterexample(tier string) Counterexample {
	return Counterexample{
		ID:      "criteria." + stableID(tier, "required") + ".missing",
		Kind:    "missing_required_tier",
		Subject: tier,
		Message: "required resource profile tier is not declared",
	}
}

var allowedTiers = map[string]bool{
	"laptop":             true,
	"ci":                 true,
	"air-gapped":         true,
	"hosted-public-good": true,
}

func normalizeTier(value string) string {
	value = normalizeToken(value)
	value = strings.ReplaceAll(value, "_", "-")
	switch value {
	case "airgapped", "offline", "air-gapped-server":
		return "air-gapped"
	case "public-good", "hosted", "hosted-service", "public-good-hosted":
		return "hosted-public-good"
	case "ci-runner", "ci-runners":
		return "ci"
	case "developer-laptop":
		return "laptop"
	default:
		return value
	}
}

func normalizeID(value string) string {
	return strings.Trim(strings.ToLower(strings.ReplaceAll(strings.TrimSpace(value), "_", "-")), "-")
}

func normalizedIDs(values []string) []string {
	var out []string
	for _, value := range values {
		if normalized := normalizeID(value); normalized != "" {
			out = append(out, normalized)
		}
	}
	sort.Strings(out)
	return compactStrings(out)
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

func normalizeToken(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	value = strings.ReplaceAll(value, " ", "-")
	return value
}

func containsAny(values []string, needles ...string) bool {
	for _, value := range values {
		for _, needle := range needles {
			if value == needle {
				return true
			}
		}
	}
	return false
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
