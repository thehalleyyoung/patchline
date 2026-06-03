package education

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildApprenticeshipReportGraduatesRealDeliverables(t *testing.T) {
	root := apprenticeshipRoot(t)
	report, err := BuildApprenticeshipReport(validApprenticeshipSpec(), root)
	if err != nil {
		t.Fatal(err)
	}
	if !report.OK {
		t.Fatalf("expected clean apprenticeship report, got counterexamples %#v", report.Counterexamples)
	}
	if report.Summary.Tracks != 3 || report.Summary.GraduatedTracks != 3 || report.Summary.GateBackedTracks != 3 {
		t.Fatalf("unexpected graduation summary: %#v", report.Summary)
	}
	if report.Summary.DeliverablesVerified != 12 || report.Summary.EvidenceArtifacts != 15 || report.Summary.MinimizedFixtures != 3 || report.Summary.NegativeControls != 3 {
		t.Fatalf("expected all detector/gate/doc/fixture evidence to be verified, got %#v", report.Summary)
	}
	if len(report.Tracks[0].Evidence) != 5 || len(report.Tracks[0].Evidence[0].SHA256) != 64 {
		t.Fatalf("expected hashed track evidence, got %#v", report.Tracks[0].Evidence)
	}
	markdown := RenderApprenticeshipMarkdown(report)
	if !strings.Contains(markdown, "Contributor apprenticeship pathway") || !strings.Contains(markdown, "Graduated tracks") {
		t.Fatalf("expected useful markdown, got:\n%s", markdown)
	}
}

func TestBuildApprenticeshipReportRefutesIncompleteGraduation(t *testing.T) {
	root := apprenticeshipRoot(t)
	spec := validApprenticeshipSpec()
	spec.Tracks[0].Detector.Symbol = "MissingDetectorSymbol"
	spec.Tracks[0].Gate.Name = "missing-gate"
	spec.Tracks[0].Gate.Commands = nil
	spec.Tracks[0].Gate.NegativeControls = nil
	spec.Tracks[0].Documentation.RequiredPhrases = []string{"phrase absent from the doc"}
	spec.Tracks[0].Fixture.Minimized = false
	spec.Tracks[0].Fixture.NegativeControlPath = "examples/missing-negative-control.json"
	spec.Tracks[0].Review.Reviewers = []string{"reviewer-a"}
	spec.Tracks[0].Review.MentorSignoff = false

	report, err := BuildApprenticeshipReport(spec, root)
	if err != nil {
		t.Fatal(err)
	}
	if report.OK {
		t.Fatalf("expected incomplete apprenticeship to fail: %#v", report)
	}
	for _, kind := range []string{
		"missing_detector_symbol",
		"missing_gate",
		"missing_reproducible_command",
		"missing_doc_phrase",
		"fixture_not_minimized",
		"missing_negative_control",
		"missing_negative_control_detail",
		"insufficient_reviewers",
		"mentor_signoff_missing",
		"deliverable_unverified",
	} {
		if !hasApprenticeshipCounterexample(report, kind) {
			t.Fatalf("expected %s counterexample, got %#v", kind, report.Counterexamples)
		}
	}
	if findApprenticeshipTrackReport(report, "query-plan-apprentice").Graduated {
		t.Fatalf("expected deficient track not to graduate: %#v", report.Tracks)
	}
}

func TestReadApprenticeshipSpecRejectsUnknownFields(t *testing.T) {
	_, err := ReadApprenticeshipSpec(strings.NewReader(`{"version":"patchline.contributor-apprenticeship/v1","name":"x","criteria":{"min_tracks":1,"required_deliverables":["detector","gate","doc","fixture"],"min_reviewers":1,"max_fixture_bytes":1000},"tracks":[],"extra":true}`))
	if err == nil {
		t.Fatal("expected unknown field error")
	}
}

func TestWriteApprenticeshipArtifactsIsDeterministic(t *testing.T) {
	root := apprenticeshipRoot(t)
	report, err := BuildApprenticeshipReport(validApprenticeshipSpec(), root)
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "apprenticeship")
	if err := WriteApprenticeshipArtifacts(out, report); err != nil {
		t.Fatal(err)
	}
	var reread ApprenticeshipReport
	file, err := os.Open(filepath.Join(out, "contributor-apprenticeship.json"))
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
}

func validApprenticeshipSpec() ApprenticeshipSpec {
	return ApprenticeshipSpec{
		Version: ApprenticeshipSpecVersion,
		Name:    "Patchline contributor apprenticeship",
		Claim:   "Patchline graduates contributors by requiring each apprentice to ship a real detector, gate, documentation, minimized fixture, negative control, mentor signoff, and independent review evidence against the live repository.",
		Criteria: ApprenticeshipCriteria{
			MinTracks:                   3,
			RequiredDeliverables:        []string{"detector", "gate", "doc", "fixture"},
			MinReviewers:                2,
			MaxFixtureBytes:             8192,
			RequireMentorSignoff:        true,
			RequireReproducibleGate:     true,
			RequireMinimizedFixture:     true,
			RequireDetectorSymbol:       true,
			RequireNegativeControl:      true,
			RequireDocumentationPhrases: true,
		},
		Tracks: []ApprenticeshipTrack{
			apprenticeshipTrack("query-plan-apprentice", "Query-plan regression detector", "query-plan-regression", "internal/dbsemantics/semantics.go", "detectQueryPlanRegression", "QueryPlanRegression", "query-plan-regression-gate", "docs/query-plan-regression.md", []string{"query-plan regression", "make query-plan-regression-gate"}, "examples/query-plan-regression-gate.json", "examples/query-plan-negative-control.json", "internal/dbsemantics/semantics_test.go"),
			apprenticeshipTrack("online-schema-apprentice", "Online schema change adapter", "online-schema-change", "internal/dbsemantics/semantics.go", "detectOnlineSchemaChange", "OnlineSchemaChange", "online-schema-change-adapters-gate", "docs/online-schema-change-adapters.md", []string{"online-schema-change adapters", "make online-schema-change-adapters-gate"}, "examples/online-schema-change-adapters-gate.json", "examples/online-schema-negative-control.json", "internal/dbsemantics/semantics_test.go"),
			apprenticeshipTrack("backfill-apprentice", "Staged backfill completeness", "partial-backfill", "internal/backfillplanner/planner.go", "BuildPlan", "CompletenessProof", "staged-backfill-planner-gate", "docs/staged-backfill-planner.md", []string{"Staged data-backfill plan", "make staged-backfill-planner-gate"}, "examples/staged-backfill-plan.json", "examples/staged-backfill-store-incomplete.json", "internal/backfillplanner/planner_test.go"),
		},
	}
}

func apprenticeshipTrack(id, title, hazard, detectorPath, symbol, signal, gate, doc string, phrases []string, fixture, negative, evidencePath string) ApprenticeshipTrack {
	return ApprenticeshipTrack{
		ID:            id,
		Title:         title,
		HazardClass:   hazard,
		ContributorID: id + "-contributor",
		MentorID:      id + "-mentor",
		Repo:          "thehalleyyoung/patchline",
		Detector: ApprenticeshipDetector{
			Path:           detectorPath,
			Symbol:         symbol,
			ExpectedSignal: signal,
			EvidencePaths:  []string{evidencePath},
		},
		Gate: ApprenticeshipGate{
			Name:              gate,
			Commands:          []string{"make " + gate},
			ExpectedArtifacts: []string{"results/generated/" + gate + "/gate-summary.json"},
			NegativeControls: []ApprenticeshipNegativeControl{{
				ID:                     "remove-required-proof",
				Mutation:               "remove the proof marker or minimized failing fixture",
				ExpectedCounterexample: "the apprenticeship report emits ok=false with the missing proof as witness",
			}},
		},
		Documentation: ApprenticeshipDocumentation{
			Path:            doc,
			RequiredPhrases: phrases,
		},
		Fixture: ApprenticeshipFixture{
			Path:                fixture,
			Minimized:           true,
			NegativeControlPath: negative,
		},
		Review: ApprenticeshipReview{
			Reviewers:     []string{id + "-reviewer-a", id + "-reviewer-b"},
			MentorSignoff: true,
			MergedPRs:     []string{"local:" + id},
		},
	}
}

func apprenticeshipRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	gates := []string{"query-plan-regression-gate", "online-schema-change-adapters-gate", "staged-backfill-planner-gate"}
	var makefile strings.Builder
	for _, gate := range gates {
		fmt.Fprintf(&makefile, "%s:\n\tbash scripts/%s.sh\n\n", gate, gate)
		writeApprenticeshipFile(t, root, "scripts/"+gate+".sh", "#!/usr/bin/env bash\nset -euo pipefail\n")
	}
	writeApprenticeshipFile(t, root, "Makefile", makefile.String())
	writeApprenticeshipFile(t, root, "internal/dbsemantics/semantics.go", "package dbsemantics\n\ntype QueryPlanRegression struct{}\ntype OnlineSchemaChange struct{}\nfunc detectQueryPlanRegression() *QueryPlanRegression { return nil }\nfunc detectOnlineSchemaChange() *OnlineSchemaChange { return nil }\n")
	writeApprenticeshipFile(t, root, "internal/dbsemantics/semantics_test.go", "package dbsemantics\n// query-plan regression and online-schema-change adapter tests\n")
	writeApprenticeshipFile(t, root, "internal/backfillplanner/planner.go", "package backfillplanner\n\ntype CompletenessProof struct{}\nfunc BuildPlan() CompletenessProof { return CompletenessProof{} }\n")
	writeApprenticeshipFile(t, root, "internal/backfillplanner/planner_test.go", "package backfillplanner\n// backfill completeness proof tests\n")
	writeApprenticeshipFile(t, root, "docs/query-plan-regression.md", "The query-plan regression detector is reproduced with make query-plan-regression-gate.\n")
	writeApprenticeshipFile(t, root, "docs/online-schema-change-adapters.md", "The online-schema-change adapters detector is reproduced with make online-schema-change-adapters-gate.\n")
	writeApprenticeshipFile(t, root, "docs/staged-backfill-planner.md", "The Staged data-backfill plan is reproduced with make staged-backfill-planner-gate.\n")
	writeApprenticeshipFile(t, root, "examples/query-plan-regression-gate.json", `{"version":"patchline.query-plan-regression-gate/v1","case":"drop-index"}`)
	writeApprenticeshipFile(t, root, "examples/query-plan-negative-control.json", `{"version":"patchline.apprenticeship-negative-control/v1","case":"add-column-control"}`)
	writeApprenticeshipFile(t, root, "examples/online-schema-change-adapters-gate.json", `{"version":"patchline.online-schema-change-adapters-gate/v1","case":"pt-osc"}`)
	writeApprenticeshipFile(t, root, "examples/online-schema-negative-control.json", `{"version":"patchline.apprenticeship-negative-control/v1","case":"plain-alter"}`)
	writeApprenticeshipFile(t, root, "examples/staged-backfill-plan.json", `{"version":"patchline.backfill-plan/v1","table":"invoices"}`)
	writeApprenticeshipFile(t, root, "examples/staged-backfill-store-incomplete.json", `{"tables":{"invoices":{"3":{"external_id":""}}}}`)
	return root
}

func writeApprenticeshipFile(t *testing.T, root, relPath, contents string) {
	t.Helper()
	path := filepath.Join(root, relPath)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
}

func hasApprenticeshipCounterexample(report ApprenticeshipReport, kind string) bool {
	for _, counterexample := range report.Counterexamples {
		if counterexample.Kind == kind {
			return true
		}
	}
	return false
}

func findApprenticeshipTrackReport(report ApprenticeshipReport, id string) ApprenticeshipTrackReport {
	for _, track := range report.Tracks {
		if track.ID == id {
			return track
		}
	}
	return ApprenticeshipTrackReport{}
}
