package sloreport

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildReportVerifiesPublicUptimeAndReproducibilitySLOs(t *testing.T) {
	root, spec := testSLOSpec(t)
	report, err := BuildReport(spec, root)
	if err != nil {
		t.Fatalf("BuildReport failed: %v", err)
	}
	if !report.OK {
		t.Fatalf("expected SLO report to pass: %#v", report.Counterexamples)
	}
	if report.Summary.Surfaces != 4 || report.Summary.Kinds != 4 || report.Summary.PublicStatusURLs != 4 {
		t.Fatalf("unexpected surface coverage: %#v", report.Summary)
	}
	if report.Summary.UptimeSLOMet != 4 || report.Summary.ReproducibilitySLOMet != 4 || report.Summary.LatencySLOMet != 4 {
		t.Fatalf("expected all SLO families to pass: %#v", report.Summary)
	}
	if report.Summary.Probes != 12 || report.Summary.ReproducibilityProbes != 4 || report.Summary.IncidentMinutes != 12 || report.Summary.ReviewedIncidents != 1 {
		t.Fatalf("unexpected probe/incident summary: %#v", report.Summary)
	}
	if report.Hash == "" {
		t.Fatal("expected deterministic report hash")
	}
	repeat, err := BuildReport(spec, root)
	if err != nil {
		t.Fatalf("repeat BuildReport failed: %v", err)
	}
	if repeat.Hash != report.Hash {
		t.Fatalf("expected deterministic hash, got %s then %s", report.Hash, repeat.Hash)
	}
	markdown := RenderMarkdown(report)
	for _, phrase := range []string{"Public uptime and reproducibility SLO report", "hosted docs, artifacts, marketplace evidence, and corpus APIs", "Surfaces"} {
		if !strings.Contains(markdown, phrase) {
			t.Fatalf("markdown missing %q:\n%s", phrase, markdown)
		}
	}
}

func TestBuildReportRejectsPublicSLORegressions(t *testing.T) {
	root, spec := testSLOSpec(t)
	spec.Surfaces = append(spec.Surfaces[:3], spec.Surfaces[4:]...)
	spec.Surfaces[0].StatusURL = ""
	spec.Surfaces[0].Probes[0].Status = "fail"
	spec.Surfaces[0].Probes[1].Status = "fail"
	spec.Surfaces[0].Probes[2].LatencyMS = 3000
	spec.Surfaces[0].Probes[2].Artifact.SHA256 = "sha256:0000000000000000000000000000000000000000000000000000000000000000"
	spec.Surfaces[1].Probes = spec.Surfaces[1].Probes[:2]
	spec.Surfaces[1].EvidencePaths = nil
	spec.Surfaces[2].Incidents[0].CorrectiveAction = ""
	spec.Surfaces[2].SLO.MaxIncidentMinutes = 5
	spec.Surfaces[2].Probes[0].ObservedAt = "2026-05-20T00:00:00Z"

	report, err := BuildReport(spec, root)
	if err != nil {
		t.Fatalf("BuildReport failed: %v", err)
	}
	if report.OK {
		t.Fatalf("expected deficient SLO spec to fail: %#v", report)
	}
	for _, kind := range []string{
		"missing_required_kind",
		"missing_public_status_url",
		"uptime_slo_breached",
		"reproducibility_slo_breached",
		"latency_slo_breached",
		"probe_artifact_hash_mismatch",
		"missing_reproducibility_probe",
		"missing_surface_evidence",
		"incident_review_missing",
		"incident_budget_exceeded",
		"stale_probe",
		"stale_probe_window",
	} {
		if !hasCounterexample(report, kind) {
			t.Fatalf("expected %s counterexample, got %#v", kind, report.Counterexamples)
		}
	}
}

func TestReadSpecRejectsUnknownFields(t *testing.T) {
	_, err := ReadSpec(strings.NewReader(`{"version":"patchline.public-slo/v1","name":"x","period":{"end":"2026-06-03T00:00:00Z"},"criteria":{},"surfaces":[],"extra":true}`))
	if err == nil {
		t.Fatal("expected unknown field rejection")
	}
}

func TestWriteArtifactsIsDeterministic(t *testing.T) {
	root, spec := testSLOSpec(t)
	report, err := BuildReport(spec, root)
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "slo")
	if err := WriteArtifacts(out, report); err != nil {
		t.Fatal(err)
	}
	var reread Report
	file, err := os.Open(filepath.Join(out, "public-slo-report.json"))
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
	if stat, err := os.Stat(filepath.Join(out, "public-slo-report.md")); err != nil || stat.Size() == 0 {
		t.Fatalf("expected markdown artifact, stat=%#v err=%v", stat, err)
	}
}

func testSLOSpec(t *testing.T) (string, Spec) {
	t.Helper()
	root := t.TempDir()
	files := map[string]string{
		"evidence/overview.md":           "public SLO report ties hosted docs, artifacts, marketplace evidence, and corpus APIs to status pages and reproducibility probes\n",
		"evidence/docs.md":               "hosted docs probes are public and reproducible from docs build artifacts\n",
		"evidence/artifacts.md":          "artifact mirror probes verify release manifests and checksums\n",
		"evidence/marketplace.md":        "marketplace evidence probes verify redacted registry exports and archive mirrors\n",
		"evidence/corpus-api.md":         "corpus API probes verify pinned benchmark queries and cache-backed reproducibility\n",
		"evidence/incident-review.md":    "reviewed twelve-minute marketplace archive mirror incident with corrective action\n",
		"outputs/docs-repro.json":        `{"surface":"hosted-docs","reproduced":true}` + "\n",
		"outputs/artifacts-repro.json":   `{"surface":"artifacts","reproduced":true}` + "\n",
		"outputs/marketplace-repro.json": `{"surface":"marketplace-evidence","reproduced":true}` + "\n",
		"outputs/corpus-api-repro.json":  `{"surface":"corpus-api","reproduced":true}` + "\n",
	}
	for path, contents := range files {
		writeSLOTestFile(t, root, path, contents)
	}
	return root, Spec{
		Version: SpecVersion,
		Name:    "unit test public SLO report",
		Claim:   "Patchline publishes a public uptime and reproducibility SLO report spanning hosted documentation, release and paper artifacts, evidence-marketplace exports, and corpus APIs. Every surface has public status, fresh uptime probes, a rerunnable reproducibility probe, hash-backed evidence, incident review obligations, deterministic report hashing, and negative controls for missing status pages, stale probes, broken hashes, breached uptime, missing reproducibility, and unreviewed incidents.",
		Period:  Period{Start: "2026-06-01T00:00:00Z", End: "2026-06-03T12:00:00Z"},
		Criteria: Criteria{
			RequiredKinds:                 []string{"hosted-docs", "artifacts", "marketplace-evidence", "corpus-api"},
			MinSurfaces:                   4,
			MinProbesPerSurface:           3,
			MinUptimePercent:              99,
			MinReproducibilityPercent:     100,
			MaxP95LatencyMS:               900,
			MaxProbeAgeHours:              96,
			MaxIncidentMinutes:            30,
			RequirePublicStatusURL:        true,
			RequireReproducibilityProbe:   true,
			RequireIncidentReview:         true,
			RequireEvidenceHashes:         true,
			RequireReproducibilityCommand: true,
		},
		Surfaces: []Surface{
			testSLOSurface(t, root, "hosted-docs", "hosted-docs", "https://docs.patchline.dev", "https://status.patchline.dev/docs", "evidence/docs.md", "outputs/docs-repro.json", nil),
			testSLOSurface(t, root, "release-artifacts", "artifacts", "https://artifacts.patchline.dev", "https://status.patchline.dev/artifacts", "evidence/artifacts.md", "outputs/artifacts-repro.json", nil),
			testSLOSurface(t, root, "marketplace-evidence", "marketplace-evidence", "https://evidence.patchline.dev", "https://status.patchline.dev/evidence", "evidence/marketplace.md", "outputs/marketplace-repro.json", []Incident{{
				ID:               "marketplace-mirror-delay",
				StartedAt:        "2026-06-02T10:00:00Z",
				ResolvedAt:       "2026-06-02T10:12:00Z",
				Severity:         "minor",
				PublicSummaryURL: "https://status.patchline.dev/incidents/marketplace-mirror-delay",
				ReviewPath:       "evidence/incident-review.md",
				CorrectiveAction: "mirror freshness monitor now pages before artifact publication stalls",
				EvidencePaths:    []string{"evidence/incident-review.md"},
			}}),
			testSLOSurface(t, root, "corpus-api", "corpus-api", "https://corpus.patchline.dev/api", "https://status.patchline.dev/corpus", "evidence/corpus-api.md", "outputs/corpus-api-repro.json", nil),
		},
		EvidencePaths: []string{"evidence/overview.md"},
	}
}

func testSLOSurface(t *testing.T, root, id, kind, publicURL, statusURL, evidence, output string, incidents []Incident) Surface {
	t.Helper()
	return Surface{
		ID:        id,
		Kind:      kind,
		PublicURL: publicURL,
		StatusURL: statusURL,
		Owner:     "maintainers",
		SLO: SurfaceSLO{
			UptimeTargetPercent:          99,
			ReproducibilityTargetPercent: 100,
			MaxP95LatencyMS:              900,
			MaxIncidentMinutes:           30,
		},
		Probes: []Probe{
			{ID: id + "-uptime-a", Kind: "uptime", ObservedAt: "2026-06-03T10:00:00Z", Status: "pass", LatencyMS: 120, EvidencePaths: []string{evidence}},
			{ID: id + "-uptime-b", Kind: "uptime", ObservedAt: "2026-06-03T11:00:00Z", Status: "pass", LatencyMS: 180, EvidencePaths: []string{evidence}},
			{ID: id + "-reproduce", Kind: "reproducibility", ObservedAt: "2026-06-03T11:30:00Z", Status: "pass", LatencyMS: 420, Command: []string{"make", id + "-gate"}, Artifact: ArtifactRef{Path: output, SHA256: sloTestFileHash(t, root, output)}, EvidencePaths: []string{evidence}},
		},
		Incidents:     incidents,
		EvidencePaths: []string{evidence},
	}
}

func writeSLOTestFile(t *testing.T, root, rel, contents string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
}

func sloTestFileHash(t *testing.T, root, rel string) string {
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
