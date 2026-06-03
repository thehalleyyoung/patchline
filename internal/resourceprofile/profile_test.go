package resourceprofile

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildReportVerifiesResourceAdaptiveProfiles(t *testing.T) {
	root, spec := testResourceProfileSpec(t)
	report, err := BuildReport(spec, root)
	if err != nil {
		t.Fatalf("BuildReport failed: %v", err)
	}
	if !report.OK {
		t.Fatalf("expected resource profile report to pass, got %#v", report.Counterexamples)
	}
	if report.Summary.Profiles != 4 || report.Summary.Tiers != 4 {
		t.Fatalf("unexpected profile coverage summary: %#v", report.Summary)
	}
	if report.Summary.LaptopProfiles != 1 || report.Summary.CIProfiles != 1 || report.Summary.OfflineProfiles != 1 || report.Summary.HostedProfiles != 1 {
		t.Fatalf("expected all four resource tiers: %#v", report.Summary)
	}
	if report.Summary.DeterministicProfiles != 4 || report.Summary.CachedProfiles != 4 || report.Summary.GracefulDegradations != 4 {
		t.Fatalf("expected deterministic cached graceful profiles: %#v", report.Summary)
	}
	repeat, err := BuildReport(spec, root)
	if err != nil {
		t.Fatalf("repeat BuildReport failed: %v", err)
	}
	if report.Hash == "" || report.Hash != repeat.Hash {
		t.Fatalf("expected deterministic report hash, got %q then %q", report.Hash, repeat.Hash)
	}
	markdown := RenderMarkdown(report)
	for _, phrase := range []string{"Resource-adaptive analysis profiles", "Laptop / CI / offline / hosted", "Command plans"} {
		if !strings.Contains(markdown, phrase) {
			t.Fatalf("markdown missing %q:\n%s", phrase, markdown)
		}
	}
}

func TestBuildReportRejectsUnsafeResourceProfiles(t *testing.T) {
	root, spec := testResourceProfileSpec(t)
	spec.Profiles = append(spec.Profiles[:1], spec.Profiles[2:]...)
	spec.Profiles[0].Constraints.CPU = 8
	spec.Profiles[0].Commands[0].Args = append(spec.Profiles[0].Commands[0].Args, "--llm-command", "remote")
	spec.Profiles[1].Constraints.NetworkAllowed = true
	spec.Profiles[1].CacheStrategy = ""
	spec.Profiles[2].Constraints.MaxCostCents = 99
	spec.Profiles[2].Budgets[0].Tokens = 0
	spec.Profiles[2].EvidencePaths = nil

	report, err := BuildReport(spec, root)
	if err != nil {
		t.Fatalf("BuildReport failed: %v", err)
	}
	if report.OK {
		t.Fatalf("expected deficient resource profile spec to fail: %#v", report)
	}
	for _, kind := range []string{
		"missing_required_profile",
		"missing_required_tier",
		"cpu_budget_exceeded",
		"nondeterministic_profile",
		"llm_not_allowed",
		"offline_network_allowed",
		"missing_cache_strategy",
		"cost_budget_exceeded",
		"invalid_budget",
		"missing_profile_evidence",
	} {
		if !hasCounterexample(report, kind) {
			t.Fatalf("expected %s counterexample, got %#v", kind, report.Counterexamples)
		}
	}
}

func TestReadSpecRejectsUnknownFields(t *testing.T) {
	_, err := ReadSpec(strings.NewReader(`{"version":"patchline.resource-profiles/v1","name":"x","criteria":{},"profiles":[],"extra":true}`))
	if err == nil {
		t.Fatal("expected unknown field error")
	}
}

func TestWriteArtifactsIsDeterministic(t *testing.T) {
	root, spec := testResourceProfileSpec(t)
	report, err := BuildReport(spec, root)
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "resource-profiles")
	if err := WriteArtifacts(out, report); err != nil {
		t.Fatal(err)
	}
	var reread Report
	file, err := os.Open(filepath.Join(out, "resource-profiles.json"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	if err := json.NewDecoder(file).Decode(&reread); err != nil {
		t.Fatal(err)
	}
	if reread.Hash != report.Hash {
		t.Fatalf("report hash changed after write/read: got %s want %s", reread.Hash, report.Hash)
	}
	if stat, err := os.Stat(filepath.Join(out, "resource-profiles.md")); err != nil || stat.Size() == 0 {
		t.Fatalf("expected markdown artifact, stat=%#v err=%v", stat, err)
	}
}

func testResourceProfileSpec(t *testing.T) (string, Spec) {
	t.Helper()
	root := t.TempDir()
	files := map[string]string{
		"evidence/overview.md": "resource-adaptive profile evidence across laptop, CI, offline, and hosted tiers\n",
		"evidence/laptop.md":   "laptop profile uses bounded files, line, token, and change budgets\n",
		"evidence/ci.md":       "CI profile finishes inside runner time and native-test policy bounds\n",
		"evidence/offline.md":  "air-gapped profile uses cached bundles and no network\n",
		"evidence/hosted.md":   "hosted public-good profile caps cost and multi-tenant work\n",
		"outputs/laptop.json":  `{"profile":"laptop","ok":true}` + "\n",
		"outputs/ci.json":      `{"profile":"ci","ok":true}` + "\n",
		"outputs/offline.json": `{"profile":"offline","ok":true}` + "\n",
		"outputs/hosted.json":  `{"profile":"hosted","ok":true}` + "\n",
	}
	for path, contents := range files {
		writeResourceTestFile(t, root, path, contents)
	}
	ids := []string{"laptop-fast", "ci-balanced", "airgap-offline", "hosted-public-good"}
	return root, Spec{
		Version: SpecVersion,
		Name:    "unit test resource-adaptive profiles",
		Claim:   "Patchline maps constrained laptops, CI runners, air-gapped servers, and public-good hosted service tiers to deterministic analysis budgets, cache strategies, native-test policies, graceful degradation, and hash-backed evidence.",
		Criteria: Criteria{
			RequiredProfileIDs:          ids,
			RequiredTiers:               []string{"laptop", "ci", "air-gapped", "hosted-public-good"},
			MinProfiles:                 4,
			MinCommandsPerProfile:       2,
			MinBudgetsPerProfile:        1,
			RequireEvidenceHashes:       true,
			RequireDeterministic:        true,
			RequireOfflineProfile:       true,
			RequireCIProfile:            true,
			RequireLaptopProfile:        true,
			RequireHostedServiceProfile: true,
			RequireNoNetworkWhenOffline: true,
			RequireCacheStrategy:        true,
			RequireNativeTestPolicy:     true,
			RequireGracefulDegradation:  true,
			MaxLaptopCPU:                4,
			MaxLaptopMemoryMB:           8192,
			MaxCIMinutes:                20,
			MaxHostedCostCents:          50,
		},
		Profiles: []Profile{
			testProfile(t, root, "laptop-fast", "laptop", "evidence/laptop.md", "outputs/laptop.json", Constraints{CPU: 2, MemoryMB: 4096, TimeoutMinutes: 8, NetworkAllowed: true, NativeTests: false}),
			testProfile(t, root, "ci-balanced", "ci", "evidence/ci.md", "outputs/ci.json", Constraints{CPU: 4, MemoryMB: 8192, TimeoutMinutes: 12, NetworkAllowed: true, NativeTests: true}),
			testProfile(t, root, "airgap-offline", "air-gapped", "evidence/offline.md", "outputs/offline.json", Constraints{CPU: 8, MemoryMB: 16384, TimeoutMinutes: 30, NetworkAllowed: false, NativeTests: false}),
			testProfile(t, root, "hosted-public-good", "hosted-public-good", "evidence/hosted.md", "outputs/hosted.json", Constraints{CPU: 2, MemoryMB: 4096, TimeoutMinutes: 10, MaxCostCents: 25, NetworkAllowed: true, NativeTests: false, HostedMultiTenant: true}),
		},
		EvidencePaths: []string{"evidence/overview.md"},
	}
}

func testProfile(t *testing.T, root, id, tier, evidence, output string, constraints Constraints) Profile {
	t.Helper()
	return Profile{
		ID:          id,
		Tier:        tier,
		Description: id + " resource profile",
		Constraints: constraints,
		Budgets:     []Budget{{Name: "repo-analyze", Files: 12, Lines: 120, Tokens: 30000, Changes: 3}},
		Commands: []Command{
			{Stage: "inventory", Args: []string{"patchline", "repo", "inventory", ".", "--out", "results/generated/" + id + "/inventory"}},
			{Stage: "analyze", Args: []string{"patchline", "repo", "analyze", ".", "--stages", "inventory,baseline,compare", "--budget", "files=12,lines=120,tokens=30000,changes=3", "--no-llm", "--out", "results/generated/" + id + "/analysis"}},
		},
		CacheStrategy:       "reuse content-addressed source and inventory caches before recomputing",
		NativeTestPolicy:    "run safe native tests only when declared by profile constraints",
		DegradationBehavior: "skip optional generators, keep deterministic inventory/baseline, and emit proof holes",
		Outputs:             []ArtifactRef{{Path: output, SHA256: testResourceFileHash(t, root, output)}},
		EvidencePaths:       []string{evidence},
	}
}

func writeResourceTestFile(t *testing.T, root, rel, contents string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
}

func testResourceFileHash(t *testing.T, root, rel string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func hasCounterexample(report Report, kind string) bool {
	for _, counterexample := range report.Counterexamples {
		if counterexample.Kind == kind {
			return true
		}
	}
	return false
}
