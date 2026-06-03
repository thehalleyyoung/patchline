package offlinedeploy

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildReportValidatesRegulatedOfflineDeployments(t *testing.T) {
	root := offlineDeployRoot(t)
	report, err := BuildReport(validOfflineDeploySpec(t, root), root)
	if err != nil {
		t.Fatal(err)
	}
	if !report.OK {
		t.Fatalf("expected clean offline deployment report, got counterexamples %#v", report.Counterexamples)
	}
	if report.Summary.Profiles != 2 || report.Summary.NoNetworkProfiles != 2 || report.Summary.TelemetryDisabledProfiles != 2 {
		t.Fatalf("unexpected offline profile summary: %#v", report.Summary)
	}
	if report.Summary.Bundles != 4 || report.Summary.PinnedBundles != 4 || report.Summary.UpdateBundles != 2 || report.Summary.OfflineUpdateBundles != 2 {
		t.Fatalf("expected pinned install and update bundles, got %#v", report.Summary)
	}
	if report.Summary.RollbackPlans != 2 || report.Summary.ReproducibleCommands != 6 || report.Summary.EvidenceArtifacts < 18 {
		t.Fatalf("expected rollback, local commands, and evidence hashes, got %#v", report.Summary)
	}
	for _, profile := range report.Profiles {
		for _, bundle := range profile.Bundles {
			if !bundle.Pinned || !strings.HasPrefix(bundle.Artifact.SHA256, "sha256:") || len(bundle.Artifact.SHA256) != 71 {
				t.Fatalf("bundle not pinned with sha256 evidence: %#v", bundle)
			}
		}
	}
	markdown := RenderMarkdown(report)
	if !strings.Contains(markdown, "Reproducible edge/offline deployment") || !strings.Contains(markdown, "Deployment profiles") {
		t.Fatalf("expected useful markdown, got:\n%s", markdown)
	}
}

func TestBuildReportRefutesNetworkTelemetryAndUnpinnedUpdates(t *testing.T) {
	root := offlineDeployRoot(t)
	spec := validOfflineDeploySpec(t, root)
	spec.Profiles = spec.Profiles[:1]
	spec.Profiles[0].NetworkPolicy.Mode = "internet"
	spec.Profiles[0].NetworkPolicy.EgressAllowed = true
	spec.Profiles[0].NetworkPolicy.AllowedEndpoints = []string{"updates.patchline.example"}
	spec.Profiles[0].TelemetryPolicy.Mode = "enabled"
	spec.Profiles[0].TelemetryPolicy.Enabled = true
	spec.Profiles[0].TelemetryPolicy.Destinations = []string{"https://otel.patchline.example/v1/traces"}
	spec.Profiles[0].InstallCommands = append(spec.Profiles[0].InstallCommands, "curl https://updates.patchline.example/patchline.tar -o patchline.tar")
	spec.Profiles[0].VerifyCommands = append(spec.Profiles[0].VerifyCommands, "patchline --telemetry=enabled")
	spec.Profiles[0].Bundles[0].SHA256 = "sha256:0000000000000000000000000000000000000000000000000000000000000000"
	spec.Profiles[0].Bundles[1].SignaturePath = ""
	spec.Profiles[0].Bundles[1].SBOMPath = ""
	spec.Profiles[0].UpdateBundles[0].SHA256 = ""
	spec.Profiles[0].UpdateBundles[0].Offline = false
	spec.Profiles[0].UpdateBundles[0].ManifestPath = ""
	spec.Profiles[0].RollbackPlan.ID = ""
	spec.Profiles[0].RollbackPlan.Commands = nil
	spec.Profiles[0].RollbackPlan.PreviousBundleSHA256 = ""

	report, err := BuildReport(spec, root)
	if err != nil {
		t.Fatal(err)
	}
	if report.OK {
		t.Fatalf("expected deficient offline deployment to fail: %#v", report)
	}
	for _, kind := range []string{
		"missing_required_environment",
		"network_not_disabled",
		"network_command",
		"telemetry_enabled",
		"telemetry_command",
		"bundle_hash_mismatch",
		"missing_signature",
		"missing_sbom",
		"unpinned_update_bundle",
		"missing_update_manifest",
		"update_not_offline",
		"missing_rollback_plan",
	} {
		if !hasCounterexample(report, kind) {
			t.Fatalf("expected %s counterexample, got %#v", kind, report.Counterexamples)
		}
	}
}

func TestReadSpecRejectsUnknownFields(t *testing.T) {
	_, err := ReadSpec(strings.NewReader(`{"version":"patchline.offline-deploy/v1","name":"x","criteria":{"required_environments":["edge"],"min_profiles":1,"min_bundles_per_profile":1,"min_update_bundles_per_profile":1,"min_regulatory_controls_per_profile":1},"profiles":[],"extra":true}`))
	if err == nil {
		t.Fatal("expected unknown field error")
	}
}

func TestWriteArtifactsIsDeterministic(t *testing.T) {
	root := offlineDeployRoot(t)
	report, err := BuildReport(validOfflineDeploySpec(t, root), root)
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "offline-deploy")
	if err := WriteArtifacts(out, report); err != nil {
		t.Fatal(err)
	}
	var reread Report
	file, err := os.Open(filepath.Join(out, "offline-deploy.json"))
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
	repeat, err := BuildReport(validOfflineDeploySpec(t, root), root)
	if err != nil {
		t.Fatal(err)
	}
	if repeat.Hash != report.Hash {
		t.Fatalf("report hash is not deterministic: got %s want %s", repeat.Hash, report.Hash)
	}
}

func validOfflineDeploySpec(t *testing.T, root string) Spec {
	t.Helper()
	clinicPatchline := "examples/offline-deploy/bundles/clinic/patchline-1.0.0.bundle"
	clinicRules := "examples/offline-deploy/bundles/clinic/rules-2026-06.bundle"
	clinicUpdate := "examples/offline-deploy/updates/clinic/rules-2026-07.update"
	financePatchline := "examples/offline-deploy/bundles/finance/patchline-1.0.0.bundle"
	financeDocs := "examples/offline-deploy/bundles/finance/docs-2026-06.bundle"
	financeUpdate := "examples/offline-deploy/updates/finance/docs-2026-07.update"
	return Spec{
		Version: SpecVersion,
		Name:    "Patchline regulated offline deployment",
		Claim:   "Patchline proves regulated edge deployments are reproducible without network or telemetry by hashing every install, update, signature, SBOM, evidence, and rollback artifact, then rejecting commands or policies that phone home.",
		Criteria: Criteria{
			RequiredEnvironments:            []string{"clinic-edge", "finance-review-room"},
			MinProfiles:                     2,
			MinBundlesPerProfile:            2,
			MinUpdateBundlesPerProfile:      1,
			MinRegulatoryControlsPerProfile: 2,
			RequireNoNetwork:                true,
			RequireTelemetryDisabled:        true,
			RequirePinnedBundles:            true,
			RequirePinnedUpdateBundles:      true,
			RequireOfflineUpdates:           true,
			RequireReproducibleCommands:     true,
			RequireRollbackPlan:             true,
			RequireEvidenceHashes:           true,
			RequireBundleSignatures:         true,
			RequireSoftwareBillOfMaterials:  true,
		},
		Profiles: []Profile{{
			ID:                 "clinic-edge-arm64",
			Environment:        "clinic-edge",
			Site:               "regional-clinic-a",
			InstallTarget:      "linux-arm64-edge-appliance",
			RegulatoryControls: []string{"hipaa", "no-telemetry", "offline-change-window"},
			NetworkPolicy:      NetworkPolicy{Mode: "none", EgressAllowed: false},
			TelemetryPolicy:    TelemetryPolicy{Mode: "disabled", Enabled: false},
			InstallCommands: []string{
				"patchline install --bundle examples/offline-deploy/bundles/clinic/patchline-1.0.0.bundle --telemetry=off",
				"patchline rules import --bundle examples/offline-deploy/bundles/clinic/rules-2026-06.bundle --offline",
			},
			VerifyCommands: []string{"shasum -a 256 -c examples/offline-deploy/updates/clinic/MANIFEST.checks"},
			EvidencePaths:  []string{"docs/offline-deploy.md", "examples/offline-deploy/evidence/clinic-window.md"},
			Bundles: []Bundle{
				bundle(t, root, "patchline-cli", "cli", "1.0.0", clinicPatchline, "examples/offline-deploy/bundles/clinic/patchline-1.0.0.bundle.sig", "examples/offline-deploy/bundles/clinic/patchline-1.0.0.sbom.json"),
				bundle(t, root, "rules-pack", "rules", "2026.06", clinicRules, "examples/offline-deploy/bundles/clinic/rules-2026-06.bundle.sig", "examples/offline-deploy/bundles/clinic/rules-2026-06.sbom.json"),
			},
			UpdateBundles: []UpdateBundle{updateBundle(t, root, "clinic-rules-2026-07", "2026.06", "2026.07", clinicUpdate, "examples/offline-deploy/updates/clinic/MANIFEST.checks", "examples/offline-deploy/updates/clinic/rules-2026-07.update.sig", "rules-pack")},
			RollbackPlan: RollbackPlan{
				ID:                   "clinic-rollback",
				MaxMinutes:           15,
				PreviousBundleSHA256: fileSHA(t, root, clinicRules),
				Commands:             []string{"patchline rules import --bundle examples/offline-deploy/bundles/clinic/rules-2026-06.bundle --offline"},
				EvidencePaths:        []string{"examples/offline-deploy/evidence/clinic-rollback.md"},
			},
		}, {
			ID:                 "finance-review-amd64",
			Environment:        "finance-review-room",
			Site:               "regulated-review-vault",
			InstallTarget:      "linux-amd64-review-workstation",
			RegulatoryControls: []string{"sox", "pci-dss", "no-telemetry"},
			NetworkPolicy:      NetworkPolicy{Mode: "air-gapped", EgressAllowed: false},
			TelemetryPolicy:    TelemetryPolicy{Mode: "off", Enabled: false},
			InstallCommands: []string{
				"patchline install --bundle examples/offline-deploy/bundles/finance/patchline-1.0.0.bundle --telemetry=off",
				"patchline docs import --bundle examples/offline-deploy/bundles/finance/docs-2026-06.bundle --offline",
			},
			VerifyCommands: []string{"shasum -a 256 -c examples/offline-deploy/updates/finance/MANIFEST.checks"},
			EvidencePaths:  []string{"docs/offline-deploy.md", "examples/offline-deploy/evidence/finance-review.md"},
			Bundles: []Bundle{
				bundle(t, root, "patchline-cli", "cli", "1.0.0", financePatchline, "examples/offline-deploy/bundles/finance/patchline-1.0.0.bundle.sig", "examples/offline-deploy/bundles/finance/patchline-1.0.0.sbom.json"),
				bundle(t, root, "docs-pack", "docs", "2026.06", financeDocs, "examples/offline-deploy/bundles/finance/docs-2026-06.bundle.sig", "examples/offline-deploy/bundles/finance/docs-2026-06.sbom.json"),
			},
			UpdateBundles: []UpdateBundle{updateBundle(t, root, "finance-docs-2026-07", "2026.06", "2026.07", financeUpdate, "examples/offline-deploy/updates/finance/MANIFEST.checks", "examples/offline-deploy/updates/finance/docs-2026-07.update.sig", "docs-pack")},
			RollbackPlan: RollbackPlan{
				ID:                   "finance-rollback",
				MaxMinutes:           20,
				PreviousBundleSHA256: fileSHA(t, root, financeDocs),
				Commands:             []string{"patchline docs import --bundle examples/offline-deploy/bundles/finance/docs-2026-06.bundle --offline"},
				EvidencePaths:        []string{"examples/offline-deploy/evidence/finance-rollback.md"},
			},
		}},
	}
}

func bundle(t *testing.T, root, id, kind, version, path, signature, sbom string) Bundle {
	t.Helper()
	return Bundle{
		ID:            id,
		Kind:          kind,
		Version:       version,
		Path:          path,
		SHA256:        fileSHA(t, root, path),
		SignaturePath: signature,
		SBOMPath:      sbom,
		UpdateChannel: "stable",
	}
}

func updateBundle(t *testing.T, root, id, from, to, path, manifest, signature, appliesTo string) UpdateBundle {
	t.Helper()
	return UpdateBundle{
		ID:            id,
		FromVersion:   from,
		ToVersion:     to,
		Path:          path,
		SHA256:        fileSHA(t, root, path),
		ManifestPath:  manifest,
		SignaturePath: signature,
		Offline:       true,
		AppliesTo:     []string{appliesTo},
	}
}

func offlineDeployRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeOfflineDeployFile(t, root, "docs/offline-deploy.md", "Offline deployment evidence.\n")
	files := map[string]string{
		"examples/offline-deploy/bundles/clinic/patchline-1.0.0.bundle":      "patchline cli linux arm64 bundle v1\n",
		"examples/offline-deploy/bundles/clinic/patchline-1.0.0.bundle.sig":  "signature for clinic cli bundle\n",
		"examples/offline-deploy/bundles/clinic/patchline-1.0.0.sbom.json":   `{"name":"patchline-cli","version":"1.0.0"}` + "\n",
		"examples/offline-deploy/bundles/clinic/rules-2026-06.bundle":        "clinic rules bundle 2026-06\n",
		"examples/offline-deploy/bundles/clinic/rules-2026-06.bundle.sig":    "signature for clinic rules bundle\n",
		"examples/offline-deploy/bundles/clinic/rules-2026-06.sbom.json":     `{"name":"patchline-rules","version":"2026.06"}` + "\n",
		"examples/offline-deploy/updates/clinic/rules-2026-07.update":        "clinic rules offline update 2026-07\n",
		"examples/offline-deploy/updates/clinic/rules-2026-07.update.sig":    "signature for clinic update\n",
		"examples/offline-deploy/updates/clinic/MANIFEST.checks":             "sha256 checks for clinic update bundle\n",
		"examples/offline-deploy/evidence/clinic-window.md":                  "Approved clinic offline change window.\n",
		"examples/offline-deploy/evidence/clinic-rollback.md":                "Clinic rollback proof with prior bundle import.\n",
		"examples/offline-deploy/bundles/finance/patchline-1.0.0.bundle":     "patchline cli linux amd64 bundle v1\n",
		"examples/offline-deploy/bundles/finance/patchline-1.0.0.bundle.sig": "signature for finance cli bundle\n",
		"examples/offline-deploy/bundles/finance/patchline-1.0.0.sbom.json":  `{"name":"patchline-cli","version":"1.0.0"}` + "\n",
		"examples/offline-deploy/bundles/finance/docs-2026-06.bundle":        "finance docs bundle 2026-06\n",
		"examples/offline-deploy/bundles/finance/docs-2026-06.bundle.sig":    "signature for finance docs bundle\n",
		"examples/offline-deploy/bundles/finance/docs-2026-06.sbom.json":     `{"name":"patchline-docs","version":"2026.06"}` + "\n",
		"examples/offline-deploy/updates/finance/docs-2026-07.update":        "finance docs offline update 2026-07\n",
		"examples/offline-deploy/updates/finance/docs-2026-07.update.sig":    "signature for finance update\n",
		"examples/offline-deploy/updates/finance/MANIFEST.checks":            "sha256 checks for finance update bundle\n",
		"examples/offline-deploy/evidence/finance-review.md":                 "Finance review room deployment evidence.\n",
		"examples/offline-deploy/evidence/finance-rollback.md":               "Finance rollback proof with prior docs bundle.\n",
	}
	for path, contents := range files {
		writeOfflineDeployFile(t, root, path, contents)
	}
	return root
}

func writeOfflineDeployFile(t *testing.T, root, relPath, contents string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(relPath))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
}

func fileSHA(t *testing.T, root, relPath string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(relPath)))
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
