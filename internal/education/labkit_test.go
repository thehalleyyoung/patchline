package education

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildLabKitReportValidatesGateBackedClassroomCourses(t *testing.T) {
	root := labKitRoot(t)
	report, err := BuildLabKitReport(validLabKitSpec(), root)
	if err != nil {
		t.Fatal(err)
	}
	if !report.OK {
		t.Fatalf("expected clean lab kit report, got counterexamples %#v", report.Counterexamples)
	}
	if report.Summary.Courses != 4 || report.Summary.Labs != 4 || report.Summary.GateBackedLabs != 4 || report.Summary.AudiencesCovered != 4 {
		t.Fatalf("unexpected summary: %#v", report.Summary)
	}
	if report.Summary.EvidenceArtifacts != 8 || report.Summary.NegativeControls != 4 {
		t.Fatalf("expected hashed evidence and negative controls for every lab, got %#v", report.Summary)
	}
	for _, course := range report.Courses {
		if len(course.LabReports) != 1 || len(course.LabReports[0].Evidence) != 2 || len(course.LabReports[0].Evidence[0].SHA256) != 64 {
			t.Fatalf("course report missing lab evidence hashes: %#v", course)
		}
	}
	markdown := RenderLabKitMarkdown(report)
	if !strings.Contains(markdown, "Classroom lab kits") || !strings.Contains(markdown, "Instructor gate") {
		t.Fatalf("expected useful markdown, got:\n%s", markdown)
	}
}

func TestBuildLabKitReportFindsUngatedAndUncoveredLabs(t *testing.T) {
	root := labKitRoot(t)
	spec := validLabKitSpec()
	spec.Courses = spec.Courses[:3]
	spec.Courses[0].Labs[0].Instructor.Gate = "missing-gate"
	spec.Courses[0].Labs[0].Instructor.Commands = nil
	spec.Courses[0].Labs[0].NegativeControls = nil
	spec.Courses[0].Labs[0].EvidencePaths = []string{"docs/missing-lab-evidence.md"}
	report, err := BuildLabKitReport(spec, root)
	if err != nil {
		t.Fatal(err)
	}
	if report.OK {
		t.Fatalf("expected deficient lab kit to fail: %#v", report)
	}
	for _, kind := range []string{"missing_required_audience", "missing_instructor_solution_gate", "missing_reproducible_command", "missing_negative_control", "missing_evidence", "insufficient_lab_evidence"} {
		if !hasLabKitCountercase(report, kind) {
			t.Fatalf("expected %s counterexample, got %#v", kind, report.Counterexamples)
		}
	}
}

func TestReadLabKitSpecRejectsUnknownFields(t *testing.T) {
	_, err := ReadLabKitSpec(strings.NewReader(`{"version":"patchline.classroom-lab-kit/v1","name":"x","criteria":{"required_audiences":["database"],"min_courses":1,"min_labs_per_course":1,"min_objectives_per_lab":1,"min_evidence_artifacts_per_lab":1,"require_instructor_solution_gate":true,"require_reproducible_command":true,"require_negative_control":true},"courses":[],"extra":true}`))
	if err == nil {
		t.Fatal("expected unknown field error")
	}
}

func TestWriteLabKitArtifactsIsDeterministic(t *testing.T) {
	root := labKitRoot(t)
	report, err := BuildLabKitReport(validLabKitSpec(), root)
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "lab-kit")
	if err := WriteLabKitArtifacts(out, report); err != nil {
		t.Fatal(err)
	}
	var reread LabKitReport
	file, err := os.Open(filepath.Join(out, "classroom-lab-kits.json"))
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

func validLabKitSpec() LabKitSpec {
	return LabKitSpec{
		Version: LabKitSpecVersion,
		Name:    "Patchline classroom lab kits",
		Claim:   "Patchline classroom lab kits package instructor solution gates, real evidence hashes, negative controls, and reproducible commands for database, software engineering, programming languages, and DevOps courses.",
		Criteria: LabKitCriteria{
			RequiredAudiences:             []string{"database", "software-engineering", "programming-languages", "devops"},
			MinCourses:                    4,
			MinLabsPerCourse:              1,
			MinObjectivesPerLab:           2,
			MinEvidenceArtifactsPerLab:    2,
			RequireInstructorSolutionGate: true,
			RequireReproducibleCommand:    true,
			RequireNegativeControl:        true,
		},
		Courses: []Course{{
			ID:       "database-systems",
			Audience: "database",
			Title:    "Database systems migration safety",
			Repo:     "thehalleyyoung/patchline",
			Labs:     []Lab{lab("backfill-contract", "staged-backfill-planner-gate", "partial-backfill", []string{"docs/staged-backfill-planner.md", "examples/staged-backfill-plan.json"})},
		}, {
			ID:       "software-engineering",
			Audience: "software-engineering",
			Title:    "Software engineering review workflows",
			Repo:     "thehalleyyoung/patchline",
			Labs:     []Lab{lab("patch-series-review", "patch-series-verifier-gate", "migration-pr-intermediate-state", []string{"docs/patch-series-verifier.md", "examples/patch-series-verifier.json"})},
		}, {
			ID:       "programming-languages",
			Audience: "programming-languages",
			Title:    "Program analysis for repair guards",
			Repo:     "thehalleyyoung/patchline",
			Labs:     []Lab{lab("symbolic-guard", "symexec-gate", "symbolic-guard-safety", []string{"docs/symexec.md", "examples/symexec-gate.json"})},
		}, {
			ID:       "devops",
			Audience: "devops",
			Title:    "DevOps deployment ordering",
			Repo:     "thehalleyyoung/patchline",
			Labs:     []Lab{lab("infra-ordering", "infra-ordering-gate", "migration-job-ordering", []string{"docs/infra-ordering.md", "examples/infra-ordering-gate.json"})},
		}},
	}
}

func lab(id, gate, hazard string, evidence []string) Lab {
	return Lab{
		ID:             id,
		Title:          "Gate-backed " + id,
		HazardClass:    hazard,
		StudentPrompt:  "Review the evidence, run the instructor gate, and explain whether the migration safety claim should pass.",
		TimeboxMinutes: 45,
		Objectives:     []string{"trace evidence hashes to a safety verdict", "explain the negative control that should fail"},
		EvidencePaths:  evidence[:1],
		Instructor: InstructorSolution{
			Gate:              gate,
			Commands:          []string{"make " + gate},
			SolutionOutline:   []string{"run the gate", "inspect the JSON report", "explain the negative control"},
			EvidencePaths:     evidence[1:],
			ExpectedArtifacts: []string{"gate-summary.json", "report.md"},
		},
		NegativeControls: []NegativeControl{{
			ID:                     "remove-required-proof",
			Mutation:               "delete the required proof artifact or gate command",
			ExpectedCounterexample: "missing required proof is reported as ok=false",
		}},
	}
}

func labKitRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeLabKitFile(t, root, "Makefile", "staged-backfill-planner-gate:\n\tbash scripts/staged-backfill-planner-gate.sh\n\npatch-series-verifier-gate:\n\tbash scripts/patch-series-verifier-gate.sh\n\nsymexec-gate:\n\tbash scripts/symexec-gate.sh\n\ninfra-ordering-gate:\n\tbash scripts/infra-ordering-gate.sh\n")
	for _, gate := range []string{"staged-backfill-planner-gate", "patch-series-verifier-gate", "symexec-gate", "infra-ordering-gate"} {
		writeLabKitFile(t, root, "scripts/"+gate+".sh", "#!/usr/bin/env bash\nset -euo pipefail\n")
	}
	writeLabKitFile(t, root, "docs/staged-backfill-planner.md", "Backfill lab evidence.\n")
	writeLabKitFile(t, root, "examples/staged-backfill-plan.json", `{"version":"patchline.backfill-plan/v1"}`)
	writeLabKitFile(t, root, "docs/patch-series-verifier.md", "Patch-series lab evidence.\n")
	writeLabKitFile(t, root, "examples/patch-series-verifier.json", `{"version":"patchline.patch-series/v1"}`)
	writeLabKitFile(t, root, "docs/symexec.md", "Symbolic execution lab evidence.\n")
	writeLabKitFile(t, root, "examples/symexec-gate.json", `{"version":"patchline.symexec-gate/v1"}`)
	writeLabKitFile(t, root, "docs/infra-ordering.md", "Infrastructure ordering lab evidence.\n")
	writeLabKitFile(t, root, "examples/infra-ordering-gate.json", `{"version":"patchline.infra-ordering-gate/v1"}`)
	return root
}

func writeLabKitFile(t *testing.T, root, relPath, contents string) {
	t.Helper()
	path := filepath.Join(root, relPath)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
}

func hasLabKitCountercase(report LabKitReport, kind string) bool {
	for _, counterexample := range report.Counterexamples {
		if counterexample.Kind == kind {
			return true
		}
	}
	return false
}
