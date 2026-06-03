package acceleratorfallback

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildReportVerifiesAcceleratorFallbacks(t *testing.T) {
	root, spec := testAcceleratorFallbackSpec(t)
	report, err := BuildReport(spec, root)
	if err != nil {
		t.Fatalf("BuildReport failed: %v", err)
	}
	if !report.OK {
		t.Fatalf("expected accelerator fallback report to pass, got %#v", report.Counterexamples)
	}
	if report.Summary.DiscoveredComponents != 3 || report.Summary.Components != 3 {
		t.Fatalf("unexpected component coverage summary: %#v", report.Summary)
	}
	if report.Summary.CPUFallbacks != 3 || report.Summary.AcceleratorFreeFallbacks != 3 || report.Summary.NoNetworkFallbacks != 3 {
		t.Fatalf("expected all fallbacks to be CPU-only, accelerator-free, and no-network: %#v", report.Summary)
	}
	if report.Summary.DeterministicFallbacks != 3 || report.Summary.ReplayProofs != 6 || report.Summary.ParityChecks != 3 {
		t.Fatalf("expected deterministic replay and parity for every component: %#v", report.Summary)
	}
	repeat, err := BuildReport(spec, root)
	if err != nil {
		t.Fatalf("repeat BuildReport failed: %v", err)
	}
	if report.Hash == "" || report.Hash != repeat.Hash {
		t.Fatalf("expected deterministic report hash, got %q then %q", report.Hash, repeat.Hash)
	}
	markdown := RenderMarkdown(report)
	for _, phrase := range []string{"Deterministic accelerator-free fallbacks", "Discovered learned components", "Parity checks"} {
		if !strings.Contains(markdown, phrase) {
			t.Fatalf("markdown missing %q:\n%s", phrase, markdown)
		}
	}
}

func TestBuildReportRejectsFallbackRegressions(t *testing.T) {
	root, spec := testAcceleratorFallbackSpec(t)
	writeAcceleratorTestFile(t, root, "fallback/bad_cuda.py", "import torch\nmodel.to('cuda')\n")
	spec.Components = spec.Components[1:]
	spec.Components[0].Fallback.GPURequired = true
	spec.Components[0].Fallback.ReplayEvidencePaths = spec.Components[0].Fallback.ReplayEvidencePaths[:1]
	spec.Components[1].Fallback.InputArtifacts[0].SHA256 = hashLiteral("wrong input")
	spec.Components[1].Fallback.ImplementationPath = "fallback/bad_cuda.py"
	spec.Components[1].Fallback.ImplementationSHA256 = testFileHash(t, root, "fallback/bad_cuda.py")
	low := 0.10
	spec.Components[1].Parity.FallbackValue = &low

	report, err := BuildReport(spec, root)
	if err != nil {
		t.Fatalf("BuildReport failed: %v", err)
	}
	if report.OK {
		t.Fatalf("expected deficient accelerator fallback spec to fail: %#v", report)
	}
	for _, kind := range []string{
		"missing_inventory_component",
		"missing_required_component",
		"gpu_required",
		"missing_replay_evidence",
		"input_artifact_hash_mismatch",
		"forbidden_accelerator_or_network_token",
		"parity_drift_exceeds_bound",
	} {
		if !hasCounterexample(report, kind) {
			t.Fatalf("expected %s counterexample, got %#v", kind, report.Counterexamples)
		}
	}
}

func TestDiscoverLearnedComponentsUsesRepositoryFixtures(t *testing.T) {
	root, _ := testAcceleratorFallbackSpec(t)
	discovered, err := DiscoverLearnedComponents(root)
	if err != nil {
		t.Fatal(err)
	}
	got := strings.Join(discovered, ",")
	want := "active-learning,learned-risk-model,rl-reviewer"
	if got != want {
		t.Fatalf("unexpected discovered learned components: got %s want %s", got, want)
	}
}

func TestReadSpecRejectsUnknownFields(t *testing.T) {
	_, err := ReadSpec(strings.NewReader(`{"version":"patchline.accelerator-fallbacks/v1","name":"x","criteria":{},"components":[],"extra":true}`))
	if err == nil {
		t.Fatal("expected unknown field error")
	}
}

func TestWriteArtifactsIsDeterministic(t *testing.T) {
	root, spec := testAcceleratorFallbackSpec(t)
	report, err := BuildReport(spec, root)
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "accelerator-fallbacks")
	if err := WriteArtifacts(out, report); err != nil {
		t.Fatal(err)
	}
	var reread Report
	file, err := os.Open(filepath.Join(out, "accelerator-fallbacks.json"))
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
	if stat, err := os.Stat(filepath.Join(out, "accelerator-fallbacks.md")); err != nil || stat.Size() == 0 {
		t.Fatalf("expected markdown artifact, stat=%#v err=%v", stat, err)
	}
}

func testAcceleratorFallbackSpec(t *testing.T) (string, Spec) {
	t.Helper()
	root := t.TempDir()
	components := []struct {
		id   string
		kind string
	}{
		{id: "active-learning", kind: "active_learning"},
		{id: "learned-risk-model", kind: "risk_model"},
		{id: "rl-reviewer", kind: "review_policy"},
	}
	files := map[string]string{
		"evidence/overview.md":    "accelerator-free fallback coverage evidence for learned components\n",
		"evidence/component.md":   "each learned component has a CPU fallback with pinned artifacts\n",
		"evidence/parity.md":      "CPU fallback parity stays within the declared drift budget\n",
		"fallback/cpu.go":         "package fallbackfixture\n\nfunc Score(seed uint64, feature uint64) uint64 { return (seed ^ feature) % 997 }\n",
		"artifacts/learned.json":  `{"artifact":"learned-component-catalog","components":3}` + "\n",
		"artifacts/inputs.jsonl":  "{\"case\":\"migration-a\",\"feature\":7}\n{\"case\":\"migration-b\",\"feature\":11}\n",
		"artifacts/outputs.jsonl": "{\"component\":\"active-learning\",\"score\":0.910}\n{\"component\":\"learned-risk-model\",\"score\":0.909}\n{\"component\":\"rl-reviewer\",\"score\":0.911}\n",
	}
	for _, component := range components {
		files["examples/"+component.id+"-gate.json"] = `{"version":"patchline.` + component.id + `-gate/v1","claim":"` + component.id + ` learned fixture"}` + "\n"
	}
	for path, contents := range files {
		writeAcceleratorTestFile(t, root, path, contents)
	}
	outputHash := testFileHash(t, root, "artifacts/outputs.jsonl")
	replay := ""
	for _, component := range components {
		replay += `{"component":"` + component.id + `","output_sha256":"` + outputHash + `","seed":"patchline-cpu-fallback-v1"}` + "\n"
	}
	writeAcceleratorTestFile(t, root, "replay/pass-1.jsonl", replay)
	writeAcceleratorTestFile(t, root, "replay/pass-2.jsonl", replay)

	learnedHash := testFileHash(t, root, "artifacts/learned.json")
	implementationHash := testFileHash(t, root, "fallback/cpu.go")
	inputHash := testFileHash(t, root, "artifacts/inputs.jsonl")
	high := 0.91
	slightlyLower := 0.908
	maxDrift := 0.01
	var specComponents []Component
	var ids []string
	for _, component := range components {
		ids = append(ids, component.id)
		specComponents = append(specComponents, Component{
			ID:                    component.id,
			Kind:                  component.kind,
			SourcePaths:           []string{"examples/" + component.id + "-gate.json"},
			LearnedArtifactPath:   "artifacts/learned.json",
			LearnedArtifactSHA256: learnedHash,
			Fallback: Fallback{
				Runtime:              "go-cpu",
				ImplementationPath:   "fallback/cpu.go",
				ImplementationSHA256: implementationHash,
				InputArtifacts:       []ArtifactRef{{Path: "artifacts/inputs.jsonl", SHA256: inputHash}},
				OutputArtifact:       ArtifactRef{Path: "artifacts/outputs.jsonl", SHA256: outputHash},
				ReplayEvidencePaths:  []string{"replay/pass-1.jsonl", "replay/pass-2.jsonl"},
				GPURequired:          false,
				AcceleratorRequired:  false,
				NetworkAllowed:       false,
				Deterministic:        true,
				Seed:                 "patchline-cpu-fallback-v1",
				Threads:              1,
			},
			Parity: Parity{
				Metric:          "accuracy",
				LearnedValue:    &high,
				FallbackValue:   &slightlyLower,
				MaxAllowedDrift: &maxDrift,
				HigherIsBetter:  true,
				EvidencePaths:   []string{"evidence/parity.md"},
			},
			EvidencePaths: []string{"evidence/component.md"},
		})
	}
	return root, Spec{
		Version: SpecVersion,
		Name:    "unit test accelerator-free learned fallbacks",
		Claim:   "Patchline proves every repository-discovered learned component has a deterministic CPU fallback with pinned artifacts, no accelerator dependency, no network access, stable replay evidence, and parity against the learned artifact.",
		Criteria: Criteria{
			RequiredComponentIDs:        ids,
			MinComponents:               len(ids),
			MinReplayArtifacts:          2,
			RequireRepositoryDiscovery:  true,
			RequireCPUFallback:          true,
			RequireAcceleratorFree:      true,
			RequireNoNetwork:            true,
			RequireDeterministicReplay:  true,
			RequireStableSeed:           true,
			RequirePinnedLearned:        true,
			RequirePinnedImplementation: true,
			RequirePinnedInputs:         true,
			RequirePinnedOutputs:        true,
			RequireParityEvidence:       true,
			RequireEvidenceHashes:       true,
			MaxAllowedDrift:             0.01,
		},
		Components:    specComponents,
		EvidencePaths: []string{"evidence/overview.md"},
	}
}

func writeAcceleratorTestFile(t *testing.T, root, rel, contents string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
}

func testFileHash(t *testing.T, root, rel string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func hashLiteral(value string) string {
	sum := sha256.Sum256([]byte(value))
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
