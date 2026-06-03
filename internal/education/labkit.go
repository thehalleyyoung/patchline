package education

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
	"unicode"

	"github.com/thehalleyyoung/patchline/internal/canonical"
)

const LabKitSpecVersion = "patchline.classroom-lab-kit/v1"
const LabKitReportVersion = "patchline.classroom-lab-kit-report/v1"

type LabKitSpec struct {
	Version  string         `json:"version"`
	Name     string         `json:"name"`
	Claim    string         `json:"claim,omitempty"`
	Criteria LabKitCriteria `json:"criteria"`
	Courses  []Course       `json:"courses"`
}

type LabKitCriteria struct {
	RequiredAudiences             []string `json:"required_audiences"`
	MinCourses                    int      `json:"min_courses"`
	MinLabsPerCourse              int      `json:"min_labs_per_course"`
	MinObjectivesPerLab           int      `json:"min_objectives_per_lab"`
	MinEvidenceArtifactsPerLab    int      `json:"min_evidence_artifacts_per_lab"`
	RequireInstructorSolutionGate bool     `json:"require_instructor_solution_gate"`
	RequireReproducibleCommand    bool     `json:"require_reproducible_command"`
	RequireNegativeControl        bool     `json:"require_negative_control"`
}

type Course struct {
	ID       string `json:"id"`
	Audience string `json:"audience"`
	Title    string `json:"title"`
	Repo     string `json:"repo"`
	Labs     []Lab  `json:"labs"`
}

type Lab struct {
	ID               string             `json:"id"`
	Title            string             `json:"title"`
	HazardClass      string             `json:"hazard_class"`
	StudentPrompt    string             `json:"student_prompt"`
	TimeboxMinutes   int                `json:"timebox_minutes"`
	Objectives       []string           `json:"objectives"`
	EvidencePaths    []string           `json:"evidence_paths"`
	Instructor       InstructorSolution `json:"instructor_solution"`
	NegativeControls []NegativeControl  `json:"negative_controls"`
}

type InstructorSolution struct {
	Gate              string   `json:"gate"`
	Commands          []string `json:"commands"`
	SolutionOutline   []string `json:"solution_outline"`
	EvidencePaths     []string `json:"evidence_paths"`
	ExpectedArtifacts []string `json:"expected_artifacts"`
}

type NegativeControl struct {
	ID                     string `json:"id"`
	Mutation               string `json:"mutation"`
	ExpectedCounterexample string `json:"expected_counterexample"`
}

type LabKitReport struct {
	Version         string              `json:"version"`
	Name            string              `json:"name"`
	OK              bool                `json:"ok"`
	Criteria        LabKitCriteria      `json:"criteria"`
	Summary         LabKitSummary       `json:"summary"`
	Courses         []CourseReport      `json:"courses"`
	Counterexamples []LabKitCountercase `json:"counterexamples,omitempty"`
	Hash            string              `json:"hash"`
}

type LabKitSummary struct {
	Courses           int `json:"courses"`
	Labs              int `json:"labs"`
	AudiencesCovered  int `json:"audiences_covered"`
	GateBackedLabs    int `json:"gate_backed_labs"`
	EvidenceArtifacts int `json:"evidence_artifacts"`
	NegativeControls  int `json:"negative_controls"`
	Counterexamples   int `json:"counterexamples"`
}

type CourseReport struct {
	ID             string      `json:"id"`
	Audience       string      `json:"audience"`
	Title          string      `json:"title"`
	Repo           string      `json:"repo"`
	Labs           int         `json:"labs"`
	GateBackedLabs int         `json:"gate_backed_labs"`
	LabReports     []LabReport `json:"lab_reports"`
}

type LabReport struct {
	ID                    string             `json:"id"`
	Title                 string             `json:"title"`
	HazardClass           string             `json:"hazard_class"`
	Gate                  string             `json:"gate"`
	GateBacked            bool               `json:"gate_backed"`
	ReproducibleCommandOK bool               `json:"reproducible_command_ok"`
	Objectives            int                `json:"objectives"`
	Evidence              []ArtifactEvidence `json:"evidence"`
	ExpectedArtifacts     []string           `json:"expected_artifacts"`
	NegativeControls      int                `json:"negative_controls"`
}

type ArtifactEvidence struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
	Bytes  int64  `json:"bytes"`
}

type LabKitCountercase struct {
	ID      string   `json:"id"`
	Kind    string   `json:"kind"`
	Subject string   `json:"subject,omitempty"`
	Message string   `json:"message"`
	Witness []string `json:"witness,omitempty"`
}

func ReadLabKitSpec(reader io.Reader) (LabKitSpec, error) {
	decoder := json.NewDecoder(reader)
	decoder.DisallowUnknownFields()
	var spec LabKitSpec
	if err := decoder.Decode(&spec); err != nil {
		return LabKitSpec{}, err
	}
	if spec.Version != LabKitSpecVersion {
		return LabKitSpec{}, fmt.Errorf("classroom lab kit spec version must be %s", LabKitSpecVersion)
	}
	return spec, nil
}

func BuildLabKitReport(spec LabKitSpec, root string) (LabKitReport, error) {
	if err := validateLabKitSpec(spec); err != nil {
		return LabKitReport{}, err
	}
	if root == "" {
		root = "."
	}
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return LabKitReport{}, err
	}
	report := LabKitReport{
		Version:  LabKitReportVersion,
		Name:     spec.Name,
		OK:       true,
		Criteria: normalizeLabKitCriteria(spec.Criteria),
	}
	audiences := map[string]struct{}{}
	if len(spec.Courses) < spec.Criteria.MinCourses {
		report.Counterexamples = append(report.Counterexamples, LabKitCountercase{
			ID:      "criteria.min_courses",
			Kind:    "insufficient_courses",
			Message: fmt.Sprintf("courses %d below required %d", len(spec.Courses), spec.Criteria.MinCourses),
		})
	}
	for _, course := range sortedCourses(spec.Courses) {
		audience := normalizeToken(course.Audience)
		audiences[audience] = struct{}{}
		cr := CourseReport{
			ID:       course.ID,
			Audience: audience,
			Title:    course.Title,
			Repo:     course.Repo,
			Labs:     len(course.Labs),
		}
		if len(course.Labs) < spec.Criteria.MinLabsPerCourse {
			report.Counterexamples = append(report.Counterexamples, LabKitCountercase{
				ID:      "course." + stableID(course.ID) + ".min_labs",
				Kind:    "insufficient_course_labs",
				Subject: course.ID,
				Message: fmt.Sprintf("course has %d labs below required %d", len(course.Labs), spec.Criteria.MinLabsPerCourse),
			})
		}
		for _, lab := range sortedLabs(course.Labs) {
			lr, counterexamples := buildLabReport(rootAbs, course, lab, spec.Criteria)
			cr.LabReports = append(cr.LabReports, lr)
			if lr.GateBacked {
				cr.GateBackedLabs++
				report.Summary.GateBackedLabs++
			}
			report.Summary.Labs++
			report.Summary.EvidenceArtifacts += len(lr.Evidence)
			report.Summary.NegativeControls += lr.NegativeControls
			report.Counterexamples = append(report.Counterexamples, counterexamples...)
		}
		report.Courses = append(report.Courses, cr)
	}
	for _, audience := range sortedStrings(normalizedStrings(spec.Criteria.RequiredAudiences)) {
		if _, ok := audiences[audience]; !ok {
			report.Counterexamples = append(report.Counterexamples, LabKitCountercase{
				ID:      "audience." + stableID(audience) + ".missing",
				Kind:    "missing_required_audience",
				Subject: audience,
				Message: "required classroom audience is not covered by any lab kit",
			})
		}
	}
	sortLabKitCountercases(report.Counterexamples)
	report.Summary.Courses = len(report.Courses)
	report.Summary.AudiencesCovered = len(audiences)
	report.Summary.Counterexamples = len(report.Counterexamples)
	report.OK = len(report.Counterexamples) == 0
	report.Hash = labKitReportHash(report)
	return report, nil
}

func WriteLabKitArtifacts(outDir string, report LabKitReport) error {
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}
	file, err := os.Create(filepath.Join(outDir, "classroom-lab-kits.json"))
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
	return os.WriteFile(filepath.Join(outDir, "classroom-lab-kits.md"), []byte(RenderLabKitMarkdown(report)), 0o644)
}

func RenderLabKitMarkdown(report LabKitReport) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Classroom lab kits\n\n")
	fmt.Fprintf(&b, "Patchline validates gate-backed classroom labs for database, software engineering, programming languages, and DevOps courses. Each lab cites real repo evidence, an instructor solution gate, reproducible commands, and a negative control.\n\n")
	fmt.Fprintf(&b, "## Summary\n\n")
	fmt.Fprintf(&b, "| Metric | Value |\n| --- | ---: |\n")
	fmt.Fprintf(&b, "| OK | `%t` |\n", report.OK)
	fmt.Fprintf(&b, "| Courses | %d |\n", report.Summary.Courses)
	fmt.Fprintf(&b, "| Audiences covered | %d |\n", report.Summary.AudiencesCovered)
	fmt.Fprintf(&b, "| Labs | %d |\n", report.Summary.Labs)
	fmt.Fprintf(&b, "| Gate-backed labs | %d |\n", report.Summary.GateBackedLabs)
	fmt.Fprintf(&b, "| Evidence artifacts | %d |\n", report.Summary.EvidenceArtifacts)
	fmt.Fprintf(&b, "| Negative controls | %d |\n", report.Summary.NegativeControls)
	fmt.Fprintf(&b, "| Counterexamples | %d |\n\n", report.Summary.Counterexamples)
	fmt.Fprintf(&b, "## Labs\n\n")
	fmt.Fprintf(&b, "| Course | Audience | Lab | Hazard | Instructor gate | Evidence hashes |\n| --- | --- | --- | --- | --- | --- |\n")
	for _, course := range report.Courses {
		for _, lab := range course.LabReports {
			hashes := make([]string, 0, len(lab.Evidence))
			for _, evidence := range lab.Evidence {
				hash := evidence.SHA256
				if len(hash) > 16 {
					hash = hash[:16]
				}
				hashes = append(hashes, evidence.Path+":"+hash)
			}
			fmt.Fprintf(&b, "| `%s` | `%s` | `%s` | `%s` | `%s` | %s |\n",
				escapeTable(course.ID),
				escapeTable(course.Audience),
				escapeTable(lab.ID),
				escapeTable(lab.HazardClass),
				escapeTable(lab.Gate),
				escapeTable(strings.Join(hashes, "; ")),
			)
		}
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

func buildLabReport(root string, course Course, lab Lab, criteria LabKitCriteria) (LabReport, []LabKitCountercase) {
	subject := course.ID + "/" + lab.ID
	paths := append([]string{}, lab.EvidencePaths...)
	paths = append(paths, lab.Instructor.EvidencePaths...)
	evidence, evidenceCounterexamples := collectLabKitArtifacts(root, uniqueSorted(paths), subject)
	gateBacked := gateExists(root, lab.Instructor.Gate)
	reproducible := containsCommand(lab.Instructor.Commands, requiredGateCommand(lab.Instructor.Gate))
	lr := LabReport{
		ID:                    lab.ID,
		Title:                 lab.Title,
		HazardClass:           lab.HazardClass,
		Gate:                  lab.Instructor.Gate,
		GateBacked:            gateBacked,
		ReproducibleCommandOK: reproducible,
		Objectives:            len(lab.Objectives),
		Evidence:              evidence,
		ExpectedArtifacts:     sortedStrings(lab.Instructor.ExpectedArtifacts),
		NegativeControls:      len(lab.NegativeControls),
	}
	counterexamples := evidenceCounterexamples
	if criteria.RequireInstructorSolutionGate && !gateBacked {
		counterexamples = append(counterexamples, LabKitCountercase{
			ID:      "lab." + stableID(subject) + ".gate",
			Kind:    "missing_instructor_solution_gate",
			Subject: subject,
			Message: "lab instructor solution gate is not present as a Make target backed by a script",
			Witness: []string{lab.Instructor.Gate},
		})
	}
	if criteria.RequireReproducibleCommand && !reproducible {
		counterexamples = append(counterexamples, LabKitCountercase{
			ID:      "lab." + stableID(subject) + ".command",
			Kind:    "missing_reproducible_command",
			Subject: subject,
			Message: "lab instructor solution does not include the reproducing gate command",
			Witness: []string{requiredGateCommand(lab.Instructor.Gate)},
		})
	}
	if len(lab.Objectives) < criteria.MinObjectivesPerLab {
		counterexamples = append(counterexamples, LabKitCountercase{
			ID:      "lab." + stableID(subject) + ".objectives",
			Kind:    "insufficient_learning_objectives",
			Subject: subject,
			Message: fmt.Sprintf("lab has %d objectives below required %d", len(lab.Objectives), criteria.MinObjectivesPerLab),
		})
	}
	if len(evidence) < criteria.MinEvidenceArtifactsPerLab {
		counterexamples = append(counterexamples, LabKitCountercase{
			ID:      "lab." + stableID(subject) + ".evidence",
			Kind:    "insufficient_lab_evidence",
			Subject: subject,
			Message: fmt.Sprintf("lab has %d evidence artifacts below required %d", len(evidence), criteria.MinEvidenceArtifactsPerLab),
		})
	}
	if criteria.RequireNegativeControl && len(lab.NegativeControls) == 0 {
		counterexamples = append(counterexamples, LabKitCountercase{
			ID:      "lab." + stableID(subject) + ".negative_control",
			Kind:    "missing_negative_control",
			Subject: subject,
			Message: "lab does not include a failing mutation and expected counterexample for instructor grading",
		})
	}
	if len(lab.Instructor.SolutionOutline) == 0 {
		counterexamples = append(counterexamples, LabKitCountercase{
			ID:      "lab." + stableID(subject) + ".solution_outline",
			Kind:    "missing_solution_outline",
			Subject: subject,
			Message: "lab instructor solution does not include a reviewable solution outline",
		})
	}
	if len(lab.Instructor.ExpectedArtifacts) == 0 {
		counterexamples = append(counterexamples, LabKitCountercase{
			ID:      "lab." + stableID(subject) + ".expected_artifacts",
			Kind:    "missing_expected_artifacts",
			Subject: subject,
			Message: "lab instructor solution does not list expected artifacts",
		})
	}
	if strings.TrimSpace(lab.StudentPrompt) == "" {
		counterexamples = append(counterexamples, LabKitCountercase{
			ID:      "lab." + stableID(subject) + ".prompt",
			Kind:    "missing_student_prompt",
			Subject: subject,
			Message: "lab does not include a student-facing prompt",
		})
	}
	if lab.TimeboxMinutes <= 0 {
		counterexamples = append(counterexamples, LabKitCountercase{
			ID:      "lab." + stableID(subject) + ".timebox",
			Kind:    "missing_timebox",
			Subject: subject,
			Message: "lab does not include a positive timebox_minutes value",
		})
	}
	for _, control := range lab.NegativeControls {
		if strings.TrimSpace(control.ID) == "" || strings.TrimSpace(control.Mutation) == "" || strings.TrimSpace(control.ExpectedCounterexample) == "" {
			counterexamples = append(counterexamples, LabKitCountercase{
				ID:      "lab." + stableID(subject, control.ID) + ".negative_control_detail",
				Kind:    "incomplete_negative_control",
				Subject: subject,
				Message: "negative control must include id, mutation, and expected counterexample",
			})
		}
	}
	return lr, counterexamples
}

func validateLabKitSpec(spec LabKitSpec) error {
	if strings.TrimSpace(spec.Name) == "" {
		return fmt.Errorf("classroom lab kit name is required")
	}
	criteria := spec.Criteria
	if criteria.MinCourses <= 0 {
		return fmt.Errorf("criteria.min_courses must be positive")
	}
	if criteria.MinLabsPerCourse <= 0 {
		return fmt.Errorf("criteria.min_labs_per_course must be positive")
	}
	if criteria.MinObjectivesPerLab <= 0 {
		return fmt.Errorf("criteria.min_objectives_per_lab must be positive")
	}
	if criteria.MinEvidenceArtifactsPerLab <= 0 {
		return fmt.Errorf("criteria.min_evidence_artifacts_per_lab must be positive")
	}
	if len(criteria.RequiredAudiences) == 0 {
		return fmt.Errorf("criteria.required_audiences is required")
	}
	courseIDs := map[string]struct{}{}
	for _, course := range spec.Courses {
		if strings.TrimSpace(course.ID) == "" {
			return fmt.Errorf("course id is required")
		}
		if _, ok := courseIDs[course.ID]; ok {
			return fmt.Errorf("duplicate course id %q", course.ID)
		}
		courseIDs[course.ID] = struct{}{}
		if strings.TrimSpace(course.Audience) == "" || strings.TrimSpace(course.Title) == "" || strings.TrimSpace(course.Repo) == "" {
			return fmt.Errorf("course %q requires audience, title, and repo", course.ID)
		}
		labIDs := map[string]struct{}{}
		for _, lab := range course.Labs {
			if strings.TrimSpace(lab.ID) == "" {
				return fmt.Errorf("course %q lab id is required", course.ID)
			}
			if _, ok := labIDs[lab.ID]; ok {
				return fmt.Errorf("course %q contains duplicate lab id %q", course.ID, lab.ID)
			}
			labIDs[lab.ID] = struct{}{}
			if strings.TrimSpace(lab.Title) == "" || strings.TrimSpace(lab.HazardClass) == "" {
				return fmt.Errorf("lab %q requires title and hazard_class", lab.ID)
			}
			if strings.TrimSpace(lab.Instructor.Gate) == "" {
				return fmt.Errorf("lab %q requires instructor_solution.gate", lab.ID)
			}
			for _, path := range append(append([]string{}, lab.EvidencePaths...), lab.Instructor.EvidencePaths...) {
				if err := validateRelativePath(path); err != nil {
					return fmt.Errorf("lab %q evidence path: %w", lab.ID, err)
				}
			}
		}
	}
	return nil
}

func collectLabKitArtifacts(root string, paths []string, subject string) ([]ArtifactEvidence, []LabKitCountercase) {
	var artifacts []ArtifactEvidence
	var counterexamples []LabKitCountercase
	for _, relPath := range paths {
		fullPath, err := safeJoin(root, relPath)
		if err != nil {
			counterexamples = append(counterexamples, LabKitCountercase{
				ID:      "lab." + stableID(subject, relPath) + ".evidence_path",
				Kind:    "invalid_evidence_path",
				Subject: subject,
				Message: err.Error(),
				Witness: []string{relPath},
			})
			continue
		}
		data, err := os.ReadFile(fullPath)
		if err != nil {
			counterexamples = append(counterexamples, LabKitCountercase{
				ID:      "lab." + stableID(subject, relPath) + ".evidence_missing",
				Kind:    "missing_evidence",
				Subject: subject,
				Message: "lab evidence could not be read",
				Witness: []string{relPath},
			})
			continue
		}
		if len(data) == 0 {
			counterexamples = append(counterexamples, LabKitCountercase{
				ID:      "lab." + stableID(subject, relPath) + ".evidence_empty",
				Kind:    "empty_evidence",
				Subject: subject,
				Message: "lab evidence is empty",
				Witness: []string{relPath},
			})
			continue
		}
		sum := sha256.Sum256(data)
		artifacts = append(artifacts, ArtifactEvidence{
			Path:   filepath.ToSlash(filepath.Clean(relPath)),
			SHA256: hex.EncodeToString(sum[:]),
			Bytes:  int64(len(data)),
		})
	}
	return artifacts, counterexamples
}

func gateExists(root, gate string) bool {
	if strings.TrimSpace(gate) == "" {
		return false
	}
	makefile, err := os.ReadFile(filepath.Join(root, "Makefile"))
	if err != nil {
		return false
	}
	foundTarget := false
	target := gate + ":"
	for _, line := range strings.Split(string(makefile), "\n") {
		if strings.TrimSpace(line) == target {
			foundTarget = true
			break
		}
	}
	if !foundTarget {
		return false
	}
	info, err := os.Stat(filepath.Join(root, "scripts", gate+".sh"))
	return err == nil && !info.IsDir() && info.Size() > 0
}

func normalizeLabKitCriteria(criteria LabKitCriteria) LabKitCriteria {
	criteria.RequiredAudiences = sortedStrings(normalizedStrings(criteria.RequiredAudiences))
	return criteria
}

func normalizedStrings(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		normalized := normalizeToken(value)
		if normalized != "" {
			out = append(out, normalized)
		}
	}
	return uniqueSorted(out)
}

func containsCommand(commands []string, want string) bool {
	normalizedWant := normalizeCommand(want)
	for _, command := range commands {
		if normalizeCommand(command) == normalizedWant {
			return true
		}
	}
	return false
}

func requiredGateCommand(gate string) string {
	if strings.TrimSpace(gate) == "" {
		return "make <missing-gate>"
	}
	return "make " + strings.TrimSpace(gate)
}

func sortedCourses(courses []Course) []Course {
	out := append([]Course(nil), courses...)
	sort.SliceStable(out, func(i, j int) bool {
		left := normalizeToken(out[i].Audience) + "\x00" + out[i].ID
		right := normalizeToken(out[j].Audience) + "\x00" + out[j].ID
		return left < right
	})
	return out
}

func sortedLabs(labs []Lab) []Lab {
	out := append([]Lab(nil), labs...)
	sort.SliceStable(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func sortedStrings(values []string) []string {
	out := append([]string(nil), values...)
	sort.Strings(out)
	return out
}

func uniqueSorted(values []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func normalizeToken(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var b strings.Builder
	lastDash := false
	for _, r := range value {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash {
			b.WriteByte('-')
			lastDash = true
		}
	}
	return strings.Trim(b.String(), "-")
}

func normalizeCommand(value string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(value)), " ")
}

func stableID(parts ...string) string {
	joined := strings.Join(parts, "\x00")
	sum := sha256.Sum256([]byte(joined))
	return hex.EncodeToString(sum[:])[:12]
}

func sortLabKitCountercases(counterexamples []LabKitCountercase) {
	sort.SliceStable(counterexamples, func(i, j int) bool { return counterexamples[i].ID < counterexamples[j].ID })
}

func labKitReportHash(report LabKitReport) string {
	report.Hash = ""
	return canonical.Hash(report)
}

func safeJoin(root, relPath string) (string, error) {
	clean := filepath.Clean(relPath)
	if clean == "." || filepath.IsAbs(clean) || strings.HasPrefix(clean, ".."+string(filepath.Separator)) || clean == ".." {
		return "", fmt.Errorf("path %q must stay under root", relPath)
	}
	full := filepath.Join(root, clean)
	rel, err := filepath.Rel(root, full)
	if err != nil {
		return "", err
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path %q escapes root", relPath)
	}
	return full, nil
}

func validateRelativePath(path string) error {
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("path is required")
	}
	_, err := safeJoin(string(filepath.Separator), path)
	return err
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
