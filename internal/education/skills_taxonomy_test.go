package education

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildSkillsTaxonomyReportMapsHazardsToReviewerConcepts(t *testing.T) {
	root := skillsTaxonomyRoot(t)
	report, err := BuildSkillsTaxonomyReport(validSkillsTaxonomySpec(), root)
	if err != nil {
		t.Fatal(err)
	}
	if !report.OK {
		t.Fatalf("expected clean skills taxonomy, got counterexamples %#v", report.Counterexamples)
	}
	if report.Summary.HazardClasses != 5 || report.Summary.Concepts != 10 || report.Summary.ReviewerAudiences != 5 || report.Summary.GateBackedHazards != 5 {
		t.Fatalf("unexpected taxonomy summary: %#v", report.Summary)
	}
	if report.Summary.EvidenceArtifacts != 15 || report.Summary.NegativeControls != 5 || report.Summary.CrosswalkEntries != 10 {
		t.Fatalf("expected evidence, negative controls, and crosswalks for every hazard, got %#v", report.Summary)
	}
	if len(report.HazardClasses[0].Evidence) != 3 || len(report.HazardClasses[0].Evidence[0].SHA256) != 64 {
		t.Fatalf("expected hashed taxonomy evidence, got %#v", report.HazardClasses[0].Evidence)
	}
	markdown := RenderSkillsTaxonomyMarkdown(report)
	if !strings.Contains(markdown, "Reviewer skills taxonomy") || !strings.Contains(markdown, "Gate-backed hazards") {
		t.Fatalf("expected useful markdown, got:\n%s", markdown)
	}
}

func TestBuildSkillsTaxonomyReportRefutesIncompleteTaxonomy(t *testing.T) {
	root := skillsTaxonomyRoot(t)
	spec := validSkillsTaxonomySpec()
	spec.HazardClasses = spec.HazardClasses[:4]
	spec.HazardClasses[0].Concepts = spec.HazardClasses[0].Concepts[:1]
	spec.HazardClasses[0].Concepts[0].Prerequisites = nil
	spec.HazardClasses[0].Concepts[0].AssessmentPrompt = ""
	spec.HazardClasses[0].Concepts[0].EvidencePaths = nil
	spec.HazardClasses[0].Gates[0].Name = "missing-gate"
	spec.HazardClasses[0].Gates[0].Commands = nil
	spec.HazardClasses[0].Gates[0].EvidencePaths = nil
	spec.HazardClasses[0].Gates[0].NegativeControls = nil
	spec.HazardClasses[0].RelatedTutorials = nil
	spec.HazardClasses[0].RelatedCertificationScenarios = nil
	spec.HazardClasses[3].ReviewerAudiences = nil

	report, err := BuildSkillsTaxonomyReport(spec, root)
	if err != nil {
		t.Fatal(err)
	}
	if report.OK {
		t.Fatalf("expected incomplete skills taxonomy to fail: %#v", report)
	}
	for _, kind := range []string{
		"insufficient_hazard_classes",
		"missing_required_audience",
		"missing_reviewer_audience",
		"insufficient_concepts",
		"insufficient_prerequisites",
		"missing_assessment_prompt",
		"missing_concept_evidence",
		"missing_gate",
		"missing_reproducible_command",
		"missing_negative_control",
		"insufficient_hazard_evidence",
		"missing_tutorial_crosswalk",
		"missing_certification_crosswalk",
	} {
		if !hasSkillsTaxonomyCounterexample(report, kind) {
			t.Fatalf("expected %s counterexample, got %#v", kind, report.Counterexamples)
		}
	}
}

func TestReadSkillsTaxonomySpecRejectsUnknownFields(t *testing.T) {
	_, err := ReadSkillsTaxonomySpec(strings.NewReader(`{"version":"patchline.skills-taxonomy/v1","name":"x","criteria":{"required_audiences":["dba"],"min_hazard_classes":1,"min_concepts_per_hazard":1,"min_prerequisites_per_concept":1,"min_evidence_artifacts_per_hazard":1,"require_gate":true,"require_reproducible_command":true,"require_negative_control":true,"require_assessment_prompt":true,"require_crosswalk":true},"hazard_classes":[],"extra":true}`))
	if err == nil {
		t.Fatal("expected unknown field error")
	}
}

func TestWriteSkillsTaxonomyArtifactsIsDeterministic(t *testing.T) {
	root := skillsTaxonomyRoot(t)
	report, err := BuildSkillsTaxonomyReport(validSkillsTaxonomySpec(), root)
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "skills-taxonomy")
	if err := WriteSkillsTaxonomyArtifacts(out, report); err != nil {
		t.Fatal(err)
	}
	var reread SkillsTaxonomyReport
	file, err := os.Open(filepath.Join(out, "skills-taxonomy.json"))
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

func validSkillsTaxonomySpec() SkillsTaxonomySpec {
	return SkillsTaxonomySpec{
		Version: SkillsTaxonomySpecVersion,
		Name:    "Patchline reviewer skills taxonomy",
		Claim:   "Patchline publishes a skills taxonomy that maps each gate-backed data-change hazard class to reviewer concepts, prerequisites, assessment prompts, evidence hashes, role-specific tutorials, certification scenarios, and negative controls so education remains testable rather than a prose-only curriculum.",
		Criteria: SkillsTaxonomyCriteria{
			RequiredAudiences:             []string{"app-developer", "dba", "sre", "security-reviewer", "engineering-manager"},
			MinHazardClasses:              5,
			MinConceptsPerHazard:          2,
			MinPrerequisitesPerConcept:    2,
			MinEvidenceArtifactsPerHazard: 3,
			RequireGate:                   true,
			RequireReproducibleCommand:    true,
			RequireNegativeControl:        true,
			RequireAssessmentPrompt:       true,
			RequireCrosswalk:              true,
		},
		HazardClasses: []SkillHazardClass{
			skillHazard("partial-backfill", "Partial backfill before contract", "high", []string{"app-developer", "dba"}, "staged-backfill-planner-gate", "docs/staged-backfill-planner.md", "examples/staged-backfill-plan.json", "scripts/staged-backfill-planner-gate.sh", []string{"backfill-completeness", "contract-staging"}),
			skillHazard("query-plan-regression", "Representative workload regression", "medium", []string{"app-developer", "dba"}, "query-plan-regression-gate", "docs/query-plan-regression.md", "examples/query-plan-regression-gate.json", "scripts/query-plan-regression-gate.sh", []string{"plan-shape-diff", "measurement-honesty"}),
			skillHazard("online-schema-change", "Online schema-change cutover", "high", []string{"dba", "sre"}, "online-schema-change-adapters-gate", "docs/online-schema-change-adapters.md", "examples/online-schema-change-adapters-gate.json", "scripts/online-schema-change-adapters-gate.sh", []string{"adapter-obligations", "cutover-risk"}),
			skillHazard("data-retention-privacy", "Broad retention and privacy impact", "critical", []string{"security-reviewer", "engineering-manager"}, "data-retention-privacy-gate", "docs/privacy-impact.md", "examples/data-retention-privacy-hazards.json", "scripts/data-retention-privacy-gate.sh", []string{"data-minimization", "rollback-retention-gap"}),
			skillHazard("migration-job-ordering", "Migration job deployment ordering", "high", []string{"sre", "engineering-manager"}, "infra-ordering-gate", "docs/infra-ordering.md", "examples/infra-ordering-gate.json", "scripts/infra-ordering-gate.sh", []string{"deploy-ordering", "race-negative-control"}),
		},
	}
}

func skillHazard(hazard, title, severity string, audiences []string, gate, docPath, fixturePath, scriptPath string, conceptIDs []string) SkillHazardClass {
	return SkillHazardClass{
		HazardClass:       hazard,
		Title:             title,
		SeverityBand:      severity,
		ReviewerAudiences: audiences,
		Concepts: []ReviewerConcept{{
			ID:               conceptIDs[0],
			Title:            "Understand " + conceptIDs[0],
			Description:      "Reviewer can connect the hazard evidence to the first required safety concept.",
			Prerequisites:    []string{"repo evidence navigation", "data-change failure modes"},
			AssessmentPrompt: "Explain which evidence hash proves or refutes " + conceptIDs[0] + ".",
			EvidencePaths:    []string{docPath},
		}, {
			ID:               conceptIDs[1],
			Title:            "Apply " + conceptIDs[1],
			Description:      "Reviewer can apply the hazard-specific concept to a negative control.",
			Prerequisites:    []string{"gate output reading", "negative-control reasoning"},
			AssessmentPrompt: "Run the gate and describe the negative control for " + conceptIDs[1] + ".",
			EvidencePaths:    []string{fixturePath},
		}},
		Gates: []TaxonomyGate{{
			Name:          gate,
			Commands:      []string{"make " + gate},
			EvidencePaths: []string{scriptPath},
			NegativeControls: []TaxonomyNegativeControl{{
				ID:                     "remove-required-skill-evidence",
				Mutation:               "drop the evidence or gate command that demonstrates the reviewer concept",
				ExpectedCounterexample: "the skills taxonomy reports the missing concept, evidence, or gate as ok=false",
			}},
		}},
		RelatedTutorials:              []string{"role-specific-tutorials:" + audiences[0] + ":" + hazard},
		RelatedCertificationScenarios: []string{"practitioner-certification:" + hazard},
	}
}

func skillsTaxonomyRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	gates := []string{"staged-backfill-planner-gate", "query-plan-regression-gate", "online-schema-change-adapters-gate", "data-retention-privacy-gate", "infra-ordering-gate"}
	var makefile strings.Builder
	for _, gate := range gates {
		makefile.WriteString(gate + ":\n\tbash scripts/" + gate + ".sh\n\n")
		writeSkillsTaxonomyFile(t, root, "scripts/"+gate+".sh", "#!/usr/bin/env bash\nset -euo pipefail\n")
	}
	writeSkillsTaxonomyFile(t, root, "Makefile", makefile.String())
	writeSkillsTaxonomyFile(t, root, "docs/staged-backfill-planner.md", "Backfill planner proves complete replay-store evidence before contract migration.\n")
	writeSkillsTaxonomyFile(t, root, "examples/staged-backfill-plan.json", `{"version":"patchline.backfill-plan/v1"}`)
	writeSkillsTaxonomyFile(t, root, "docs/query-plan-regression.md", "Query-plan regression checks representative workload shape without invented EXPLAIN costs.\n")
	writeSkillsTaxonomyFile(t, root, "examples/query-plan-regression-gate.json", `{"version":"patchline.query-plan-regression-gate/v1"}`)
	writeSkillsTaxonomyFile(t, root, "docs/online-schema-change-adapters.md", "Online schema-change adapters require cutover and rollback obligations.\n")
	writeSkillsTaxonomyFile(t, root, "examples/online-schema-change-adapters-gate.json", `{"version":"patchline.online-schema-change-adapters-gate/v1"}`)
	writeSkillsTaxonomyFile(t, root, "docs/privacy-impact.md", "Privacy impact review covers data minimization, broad deletes, and retention rollback gaps.\n")
	writeSkillsTaxonomyFile(t, root, "examples/data-retention-privacy-hazards.json", `{"version":"patchline.data-retention-privacy-hazards/v1"}`)
	writeSkillsTaxonomyFile(t, root, "docs/infra-ordering.md", "Infrastructure ordering proves migration jobs are sequenced before dependent deploys.\n")
	writeSkillsTaxonomyFile(t, root, "examples/infra-ordering-gate.json", `{"version":"patchline.infra-ordering-gate/v1"}`)
	return root
}

func writeSkillsTaxonomyFile(t *testing.T, root, relPath, contents string) {
	t.Helper()
	path := filepath.Join(root, relPath)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
}

func hasSkillsTaxonomyCounterexample(report SkillsTaxonomyReport, kind string) bool {
	for _, counterexample := range report.Counterexamples {
		if counterexample.Kind == kind {
			return true
		}
	}
	return false
}
