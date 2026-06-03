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

	"github.com/thehalleyyoung/patchline/internal/canonical"
)

const ApprenticeshipSpecVersion = "patchline.contributor-apprenticeship/v1"
const ApprenticeshipReportVersion = "patchline.contributor-apprenticeship-report/v1"

type ApprenticeshipSpec struct {
	Version  string                 `json:"version"`
	Name     string                 `json:"name"`
	Claim    string                 `json:"claim,omitempty"`
	Criteria ApprenticeshipCriteria `json:"criteria"`
	Tracks   []ApprenticeshipTrack  `json:"tracks"`
}

type ApprenticeshipCriteria struct {
	MinTracks                   int      `json:"min_tracks"`
	RequiredDeliverables        []string `json:"required_deliverables"`
	MinReviewers                int      `json:"min_reviewers"`
	MaxFixtureBytes             int64    `json:"max_fixture_bytes"`
	RequireMentorSignoff        bool     `json:"require_mentor_signoff"`
	RequireReproducibleGate     bool     `json:"require_reproducible_gate"`
	RequireMinimizedFixture     bool     `json:"require_minimized_fixture"`
	RequireDetectorSymbol       bool     `json:"require_detector_symbol"`
	RequireNegativeControl      bool     `json:"require_negative_control"`
	RequireDocumentationPhrases bool     `json:"require_documentation_phrases"`
}

type ApprenticeshipTrack struct {
	ID            string                      `json:"id"`
	Title         string                      `json:"title"`
	HazardClass   string                      `json:"hazard_class"`
	ContributorID string                      `json:"contributor_id"`
	MentorID      string                      `json:"mentor_id"`
	Repo          string                      `json:"repo"`
	Detector      ApprenticeshipDetector      `json:"detector"`
	Gate          ApprenticeshipGate          `json:"gate"`
	Documentation ApprenticeshipDocumentation `json:"documentation"`
	Fixture       ApprenticeshipFixture       `json:"fixture"`
	Review        ApprenticeshipReview        `json:"review"`
}

type ApprenticeshipDetector struct {
	Path           string   `json:"path"`
	Symbol         string   `json:"symbol"`
	ExpectedSignal string   `json:"expected_signal"`
	EvidencePaths  []string `json:"evidence_paths,omitempty"`
}

type ApprenticeshipGate struct {
	Name              string                          `json:"name"`
	Commands          []string                        `json:"commands"`
	ExpectedArtifacts []string                        `json:"expected_artifacts"`
	NegativeControls  []ApprenticeshipNegativeControl `json:"negative_controls"`
}

type ApprenticeshipNegativeControl struct {
	ID                     string `json:"id"`
	Mutation               string `json:"mutation"`
	ExpectedCounterexample string `json:"expected_counterexample"`
}

type ApprenticeshipDocumentation struct {
	Path            string   `json:"path"`
	RequiredPhrases []string `json:"required_phrases"`
}

type ApprenticeshipFixture struct {
	Path                string `json:"path"`
	Minimized           bool   `json:"minimized"`
	NegativeControlPath string `json:"negative_control_path"`
}

type ApprenticeshipReview struct {
	Reviewers     []string `json:"reviewers"`
	MentorSignoff bool     `json:"mentor_signoff"`
	MergedPRs     []string `json:"merged_prs,omitempty"`
}

type ApprenticeshipReport struct {
	Version         string                         `json:"version"`
	Name            string                         `json:"name"`
	OK              bool                           `json:"ok"`
	Criteria        ApprenticeshipCriteria         `json:"criteria"`
	Summary         ApprenticeshipSummary          `json:"summary"`
	Tracks          []ApprenticeshipTrackReport    `json:"tracks"`
	Counterexamples []ApprenticeshipCounterexample `json:"counterexamples,omitempty"`
	Hash            string                         `json:"hash"`
}

type ApprenticeshipSummary struct {
	Tracks               int `json:"tracks"`
	GraduatedTracks      int `json:"graduated_tracks"`
	DeliverablesVerified int `json:"deliverables_verified"`
	GateBackedTracks     int `json:"gate_backed_tracks"`
	EvidenceArtifacts    int `json:"evidence_artifacts"`
	MinimizedFixtures    int `json:"minimized_fixtures"`
	NegativeControls     int `json:"negative_controls"`
	Reviewers            int `json:"reviewers"`
	Counterexamples      int `json:"counterexamples"`
}

type ApprenticeshipTrackReport struct {
	ID                     string             `json:"id"`
	Title                  string             `json:"title"`
	HazardClass            string             `json:"hazard_class"`
	ContributorID          string             `json:"contributor_id"`
	MentorID               string             `json:"mentor_id"`
	Repo                   string             `json:"repo"`
	DeliverablesVerified   []string           `json:"deliverables_verified"`
	Gate                   string             `json:"gate"`
	GateBacked             bool               `json:"gate_backed"`
	ReproducibleCommandOK  bool               `json:"reproducible_command_ok"`
	DetectorPath           string             `json:"detector_path"`
	DetectorSymbolOK       bool               `json:"detector_symbol_ok"`
	DetectorSignalOK       bool               `json:"detector_signal_ok"`
	DocumentationPath      string             `json:"documentation_path"`
	DocumentationPhrasesOK bool               `json:"documentation_phrases_ok"`
	FixturePath            string             `json:"fixture_path"`
	FixtureMinimized       bool               `json:"fixture_minimized"`
	FixtureBytes           int64              `json:"fixture_bytes"`
	NegativeControls       int                `json:"negative_controls"`
	Reviewers              int                `json:"reviewers"`
	MentorSignoff          bool               `json:"mentor_signoff"`
	Evidence               []ArtifactEvidence `json:"evidence"`
	Graduated              bool               `json:"graduated"`
}

type ApprenticeshipCounterexample struct {
	ID      string   `json:"id"`
	Kind    string   `json:"kind"`
	Subject string   `json:"subject,omitempty"`
	Message string   `json:"message"`
	Witness []string `json:"witness,omitempty"`
}

func ReadApprenticeshipSpec(reader io.Reader) (ApprenticeshipSpec, error) {
	decoder := json.NewDecoder(reader)
	decoder.DisallowUnknownFields()
	var spec ApprenticeshipSpec
	if err := decoder.Decode(&spec); err != nil {
		return ApprenticeshipSpec{}, err
	}
	if spec.Version != ApprenticeshipSpecVersion {
		return ApprenticeshipSpec{}, fmt.Errorf("contributor apprenticeship spec version must be %s", ApprenticeshipSpecVersion)
	}
	return spec, nil
}

func BuildApprenticeshipReport(spec ApprenticeshipSpec, root string) (ApprenticeshipReport, error) {
	if err := validateApprenticeshipSpec(spec); err != nil {
		return ApprenticeshipReport{}, err
	}
	if root == "" {
		root = "."
	}
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return ApprenticeshipReport{}, err
	}
	criteria := normalizeApprenticeshipCriteria(spec.Criteria)
	report := ApprenticeshipReport{
		Version:  ApprenticeshipReportVersion,
		Name:     spec.Name,
		OK:       true,
		Criteria: criteria,
	}
	if len(spec.Tracks) < criteria.MinTracks {
		report.Counterexamples = append(report.Counterexamples, ApprenticeshipCounterexample{
			ID:      "criteria.min_tracks",
			Kind:    "insufficient_tracks",
			Message: fmt.Sprintf("tracks %d below required %d", len(spec.Tracks), criteria.MinTracks),
		})
	}
	reviewerSet := map[string]struct{}{}
	for _, track := range sortedApprenticeshipTracks(spec.Tracks) {
		trackReport, counterexamples := buildApprenticeshipTrack(rootAbs, track, criteria)
		report.Tracks = append(report.Tracks, trackReport)
		report.Counterexamples = append(report.Counterexamples, counterexamples...)
		report.Summary.DeliverablesVerified += len(trackReport.DeliverablesVerified)
		report.Summary.EvidenceArtifacts += len(trackReport.Evidence)
		report.Summary.NegativeControls += trackReport.NegativeControls
		if trackReport.GateBacked {
			report.Summary.GateBackedTracks++
		}
		if trackReport.FixtureMinimized {
			report.Summary.MinimizedFixtures++
		}
		if trackReport.Graduated {
			report.Summary.GraduatedTracks++
		}
		for _, reviewer := range uniqueSorted(track.Review.Reviewers) {
			reviewerSet[reviewer] = struct{}{}
		}
	}
	sortApprenticeshipCounterexamples(report.Counterexamples)
	report.Summary.Tracks = len(report.Tracks)
	report.Summary.Reviewers = len(reviewerSet)
	report.Summary.Counterexamples = len(report.Counterexamples)
	report.OK = len(report.Counterexamples) == 0
	report.Hash = apprenticeshipReportHash(report)
	return report, nil
}

func WriteApprenticeshipArtifacts(outDir string, report ApprenticeshipReport) error {
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}
	file, err := os.Create(filepath.Join(outDir, "contributor-apprenticeship.json"))
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
	return os.WriteFile(filepath.Join(outDir, "contributor-apprenticeship.md"), []byte(RenderApprenticeshipMarkdown(report)), 0o644)
}

func RenderApprenticeshipMarkdown(report ApprenticeshipReport) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Contributor apprenticeship pathway\n\n")
	fmt.Fprintf(&b, "Patchline graduates new contributors only after a real detector, gate, documentation page, minimized fixture, negative control, mentor signoff, and reviewer evidence all validate against the repository checkout.\n\n")
	fmt.Fprintf(&b, "## Summary\n\n")
	fmt.Fprintf(&b, "| Metric | Value |\n| --- | ---: |\n")
	fmt.Fprintf(&b, "| OK | `%t` |\n", report.OK)
	fmt.Fprintf(&b, "| Tracks | %d |\n", report.Summary.Tracks)
	fmt.Fprintf(&b, "| Graduated tracks | %d |\n", report.Summary.GraduatedTracks)
	fmt.Fprintf(&b, "| Deliverables verified | %d |\n", report.Summary.DeliverablesVerified)
	fmt.Fprintf(&b, "| Gate-backed tracks | %d |\n", report.Summary.GateBackedTracks)
	fmt.Fprintf(&b, "| Evidence artifacts | %d |\n", report.Summary.EvidenceArtifacts)
	fmt.Fprintf(&b, "| Minimized fixtures | %d |\n", report.Summary.MinimizedFixtures)
	fmt.Fprintf(&b, "| Negative controls | %d |\n", report.Summary.NegativeControls)
	fmt.Fprintf(&b, "| Reviewers | %d |\n", report.Summary.Reviewers)
	fmt.Fprintf(&b, "| Counterexamples | %d |\n\n", report.Summary.Counterexamples)

	fmt.Fprintf(&b, "## Graduation tracks\n\n")
	fmt.Fprintf(&b, "| Track | Hazard | Gate | Detector | Fixture bytes | Deliverables | Graduated |\n| --- | --- | --- | --- | ---: | --- | ---: |\n")
	for _, track := range report.Tracks {
		fmt.Fprintf(&b, "| `%s` | `%s` | `%s` | `%s` | %d | %s | `%t` |\n",
			escapeTable(track.ID),
			escapeTable(track.HazardClass),
			escapeTable(track.Gate),
			escapeTable(track.DetectorPath),
			track.FixtureBytes,
			escapeTable(strings.Join(track.DeliverablesVerified, ", ")),
			track.Graduated,
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

func buildApprenticeshipTrack(root string, track ApprenticeshipTrack, criteria ApprenticeshipCriteria) (ApprenticeshipTrackReport, []ApprenticeshipCounterexample) {
	subject := track.ID
	paths := []string{track.Detector.Path, track.Documentation.Path, track.Fixture.Path, track.Fixture.NegativeControlPath}
	paths = append(paths, track.Detector.EvidencePaths...)
	evidence, counterexamples := collectApprenticeshipEvidence(root, uniqueSorted(paths), subject)
	evidenceByPath := map[string]ArtifactEvidence{}
	for _, artifact := range evidence {
		evidenceByPath[artifact.Path] = artifact
	}
	detectorSymbolOK, detectorSignalOK, detectorCounterexamples := inspectApprenticeshipDetector(root, track, criteria)
	counterexamples = append(counterexamples, detectorCounterexamples...)
	docPhrasesOK, docCounterexamples := inspectApprenticeshipDocumentation(root, track, criteria)
	counterexamples = append(counterexamples, docCounterexamples...)

	gateBacked := gateExists(root, track.Gate.Name)
	reproducible := containsCommand(track.Gate.Commands, requiredGateCommand(track.Gate.Name))
	fixturePath := filepath.ToSlash(filepath.Clean(track.Fixture.Path))
	fixtureArtifact, fixtureExists := evidenceByPath[fixturePath]
	negativePath := filepath.ToSlash(filepath.Clean(track.Fixture.NegativeControlPath))
	_, negativeFixtureExists := evidenceByPath[negativePath]
	reviewers := uniqueSorted(track.Review.Reviewers)
	mentorOK := !criteria.RequireMentorSignoff || track.Review.MentorSignoff
	reviewerOK := len(reviewers) >= criteria.MinReviewers
	fixtureSizeOK := criteria.MaxFixtureBytes <= 0 || (fixtureExists && fixtureArtifact.Bytes <= criteria.MaxFixtureBytes)
	negativeOK := !criteria.RequireNegativeControl || (strings.TrimSpace(track.Fixture.NegativeControlPath) != "" && negativeFixtureExists && len(track.Gate.NegativeControls) > 0)

	if !gateBacked {
		counterexamples = append(counterexamples, ApprenticeshipCounterexample{
			ID:      "track." + stableID(subject, "gate") + ".missing",
			Kind:    "missing_gate",
			Subject: subject,
			Message: "apprenticeship gate is not present as a Make target backed by a script",
			Witness: []string{track.Gate.Name},
		})
	}
	if criteria.RequireReproducibleGate && !reproducible {
		counterexamples = append(counterexamples, ApprenticeshipCounterexample{
			ID:      "track." + stableID(subject, "gate-command") + ".missing",
			Kind:    "missing_reproducible_command",
			Subject: subject,
			Message: "apprenticeship gate commands do not include the reproducing make target",
			Witness: []string{requiredGateCommand(track.Gate.Name)},
		})
	}
	if criteria.RequireMinimizedFixture && !track.Fixture.Minimized {
		counterexamples = append(counterexamples, ApprenticeshipCounterexample{
			ID:      "track." + stableID(subject, "fixture-minimized") + ".missing",
			Kind:    "fixture_not_minimized",
			Subject: subject,
			Message: "fixture is not declared as a minimized reproduction",
			Witness: []string{track.Fixture.Path},
		})
	}
	if fixtureExists && criteria.MaxFixtureBytes > 0 && fixtureArtifact.Bytes > criteria.MaxFixtureBytes {
		counterexamples = append(counterexamples, ApprenticeshipCounterexample{
			ID:      "track." + stableID(subject, "fixture-size") + ".too_large",
			Kind:    "fixture_too_large",
			Subject: subject,
			Message: fmt.Sprintf("fixture is %d bytes, above max %d", fixtureArtifact.Bytes, criteria.MaxFixtureBytes),
			Witness: []string{track.Fixture.Path},
		})
	}
	if criteria.RequireNegativeControl {
		if strings.TrimSpace(track.Fixture.NegativeControlPath) == "" || !negativeFixtureExists {
			counterexamples = append(counterexamples, ApprenticeshipCounterexample{
				ID:      "track." + stableID(subject, "negative-control") + ".missing",
				Kind:    "missing_negative_control",
				Subject: subject,
				Message: "apprenticeship track does not cite a readable negative-control fixture",
				Witness: []string{track.Fixture.NegativeControlPath},
			})
		}
		if len(track.Gate.NegativeControls) == 0 {
			counterexamples = append(counterexamples, ApprenticeshipCounterexample{
				ID:      "track." + stableID(subject, "negative-control-detail") + ".missing",
				Kind:    "missing_negative_control_detail",
				Subject: subject,
				Message: "apprenticeship track does not describe the failing mutation and expected counterexample",
			})
		}
	}
	for _, control := range track.Gate.NegativeControls {
		if strings.TrimSpace(control.ID) == "" || strings.TrimSpace(control.Mutation) == "" || strings.TrimSpace(control.ExpectedCounterexample) == "" {
			counterexamples = append(counterexamples, ApprenticeshipCounterexample{
				ID:      "track." + stableID(subject, control.ID, "negative-control-detail") + ".incomplete",
				Kind:    "incomplete_negative_control",
				Subject: subject,
				Message: "negative control must include id, mutation, and expected counterexample",
			})
		}
	}
	if !reviewerOK {
		counterexamples = append(counterexamples, ApprenticeshipCounterexample{
			ID:      "track." + stableID(subject, "reviewers") + ".insufficient",
			Kind:    "insufficient_reviewers",
			Subject: subject,
			Message: fmt.Sprintf("track has %d reviewers below required %d", len(reviewers), criteria.MinReviewers),
			Witness: reviewers,
		})
	}
	if criteria.RequireMentorSignoff && !track.Review.MentorSignoff {
		counterexamples = append(counterexamples, ApprenticeshipCounterexample{
			ID:      "track." + stableID(subject, "mentor-signoff") + ".missing",
			Kind:    "mentor_signoff_missing",
			Subject: subject,
			Message: "track lacks required mentor signoff",
			Witness: []string{track.MentorID},
		})
	}
	if strings.TrimSpace(track.MentorID) == "" {
		counterexamples = append(counterexamples, ApprenticeshipCounterexample{
			ID:      "track." + stableID(subject, "mentor") + ".missing",
			Kind:    "mentor_missing",
			Subject: subject,
			Message: "track does not name a mentor",
		})
	}
	if len(track.Gate.ExpectedArtifacts) == 0 {
		counterexamples = append(counterexamples, ApprenticeshipCounterexample{
			ID:      "track." + stableID(subject, "expected-artifacts") + ".missing",
			Kind:    "missing_expected_artifacts",
			Subject: subject,
			Message: "gate does not list expected artifacts",
			Witness: []string{track.Gate.Name},
		})
	}

	deliverableOK := map[string]bool{
		"detector": detectorSymbolOK && detectorSignalOK && artifactExists(evidenceByPath, track.Detector.Path),
		"gate":     gateBacked && (!criteria.RequireReproducibleGate || reproducible),
		"doc":      artifactExists(evidenceByPath, track.Documentation.Path) && docPhrasesOK,
		"fixture":  fixtureExists && fixtureSizeOK && (!criteria.RequireMinimizedFixture || track.Fixture.Minimized) && negativeOK,
	}
	deliverables := make([]string, 0, len(criteria.RequiredDeliverables))
	for _, deliverable := range criteria.RequiredDeliverables {
		if deliverableOK[deliverable] {
			deliverables = append(deliverables, deliverable)
			continue
		}
		counterexamples = append(counterexamples, ApprenticeshipCounterexample{
			ID:      "track." + stableID(subject, deliverable, "deliverable") + ".unverified",
			Kind:    "deliverable_unverified",
			Subject: subject,
			Message: "required apprenticeship deliverable is not fully verified",
			Witness: []string{deliverable},
		})
	}
	sort.Strings(deliverables)
	sortApprenticeshipCounterexamples(counterexamples)
	trackReport := ApprenticeshipTrackReport{
		ID:                     track.ID,
		Title:                  track.Title,
		HazardClass:            normalizeToken(track.HazardClass),
		ContributorID:          track.ContributorID,
		MentorID:               track.MentorID,
		Repo:                   track.Repo,
		DeliverablesVerified:   deliverables,
		Gate:                   track.Gate.Name,
		GateBacked:             gateBacked,
		ReproducibleCommandOK:  reproducible,
		DetectorPath:           filepath.ToSlash(filepath.Clean(track.Detector.Path)),
		DetectorSymbolOK:       detectorSymbolOK,
		DetectorSignalOK:       detectorSignalOK,
		DocumentationPath:      filepath.ToSlash(filepath.Clean(track.Documentation.Path)),
		DocumentationPhrasesOK: docPhrasesOK,
		FixturePath:            fixturePath,
		FixtureMinimized:       track.Fixture.Minimized,
		FixtureBytes:           fixtureArtifact.Bytes,
		NegativeControls:       len(track.Gate.NegativeControls),
		Reviewers:              len(reviewers),
		MentorSignoff:          track.Review.MentorSignoff,
		Evidence:               evidence,
		Graduated:              len(counterexamples) == 0 && reviewerOK && mentorOK,
	}
	return trackReport, counterexamples
}

func inspectApprenticeshipDetector(root string, track ApprenticeshipTrack, criteria ApprenticeshipCriteria) (bool, bool, []ApprenticeshipCounterexample) {
	fullPath, err := safeJoin(root, track.Detector.Path)
	if err != nil {
		return false, false, []ApprenticeshipCounterexample{{
			ID:      "track." + stableID(track.ID, "detector-path") + ".invalid",
			Kind:    "invalid_detector_path",
			Subject: track.ID,
			Message: err.Error(),
			Witness: []string{track.Detector.Path},
		}}
	}
	data, err := os.ReadFile(fullPath)
	if err != nil {
		return false, false, nil
	}
	contents := string(data)
	symbolOK := !criteria.RequireDetectorSymbol || strings.Contains(contents, track.Detector.Symbol)
	signalOK := strings.TrimSpace(track.Detector.ExpectedSignal) == "" || strings.Contains(contents, track.Detector.ExpectedSignal)
	var counterexamples []ApprenticeshipCounterexample
	if !symbolOK {
		counterexamples = append(counterexamples, ApprenticeshipCounterexample{
			ID:      "track." + stableID(track.ID, "detector-symbol") + ".missing",
			Kind:    "missing_detector_symbol",
			Subject: track.ID,
			Message: "detector file does not contain the required symbol",
			Witness: []string{track.Detector.Symbol, track.Detector.Path},
		})
	}
	if !signalOK {
		counterexamples = append(counterexamples, ApprenticeshipCounterexample{
			ID:      "track." + stableID(track.ID, "detector-signal") + ".missing",
			Kind:    "missing_detector_signal",
			Subject: track.ID,
			Message: "detector file does not contain the expected emitted signal",
			Witness: []string{track.Detector.ExpectedSignal, track.Detector.Path},
		})
	}
	return symbolOK, signalOK, counterexamples
}

func inspectApprenticeshipDocumentation(root string, track ApprenticeshipTrack, criteria ApprenticeshipCriteria) (bool, []ApprenticeshipCounterexample) {
	if !criteria.RequireDocumentationPhrases {
		return true, nil
	}
	fullPath, err := safeJoin(root, track.Documentation.Path)
	if err != nil {
		return false, []ApprenticeshipCounterexample{{
			ID:      "track." + stableID(track.ID, "documentation-path") + ".invalid",
			Kind:    "invalid_documentation_path",
			Subject: track.ID,
			Message: err.Error(),
			Witness: []string{track.Documentation.Path},
		}}
	}
	data, err := os.ReadFile(fullPath)
	if err != nil {
		return false, nil
	}
	contents := string(data)
	ok := true
	var counterexamples []ApprenticeshipCounterexample
	for _, phrase := range uniqueSorted(track.Documentation.RequiredPhrases) {
		if strings.Contains(contents, phrase) {
			continue
		}
		ok = false
		counterexamples = append(counterexamples, ApprenticeshipCounterexample{
			ID:      "track." + stableID(track.ID, phrase, "doc-phrase") + ".missing",
			Kind:    "missing_doc_phrase",
			Subject: track.ID,
			Message: "documentation does not contain a required phrase",
			Witness: []string{phrase, track.Documentation.Path},
		})
	}
	if len(track.Documentation.RequiredPhrases) == 0 {
		ok = false
		counterexamples = append(counterexamples, ApprenticeshipCounterexample{
			ID:      "track." + stableID(track.ID, "doc-phrase") + ".missing",
			Kind:    "missing_doc_phrase",
			Subject: track.ID,
			Message: "documentation does not list required phrases for review",
			Witness: []string{track.Documentation.Path},
		})
	}
	return ok, counterexamples
}

func validateApprenticeshipSpec(spec ApprenticeshipSpec) error {
	if strings.TrimSpace(spec.Name) == "" {
		return fmt.Errorf("contributor apprenticeship name is required")
	}
	criteria := spec.Criteria
	if criteria.MinTracks <= 0 {
		return fmt.Errorf("criteria.min_tracks must be positive")
	}
	if len(criteria.RequiredDeliverables) == 0 {
		return fmt.Errorf("criteria.required_deliverables is required")
	}
	if criteria.MinReviewers <= 0 {
		return fmt.Errorf("criteria.min_reviewers must be positive")
	}
	if criteria.MaxFixtureBytes <= 0 {
		return fmt.Errorf("criteria.max_fixture_bytes must be positive")
	}
	allowed := map[string]struct{}{"detector": {}, "gate": {}, "doc": {}, "fixture": {}}
	for _, deliverable := range criteria.RequiredDeliverables {
		if _, ok := allowed[normalizeToken(deliverable)]; !ok {
			return fmt.Errorf("unknown required deliverable %q", deliverable)
		}
	}
	trackIDs := map[string]struct{}{}
	for _, track := range spec.Tracks {
		if strings.TrimSpace(track.ID) == "" {
			return fmt.Errorf("track id is required")
		}
		if _, ok := trackIDs[track.ID]; ok {
			return fmt.Errorf("duplicate track id %q", track.ID)
		}
		trackIDs[track.ID] = struct{}{}
		if strings.TrimSpace(track.Title) == "" || strings.TrimSpace(track.HazardClass) == "" || strings.TrimSpace(track.ContributorID) == "" || strings.TrimSpace(track.Repo) == "" {
			return fmt.Errorf("track %q requires title, hazard_class, contributor_id, and repo", track.ID)
		}
		if strings.TrimSpace(track.Detector.Path) == "" || strings.TrimSpace(track.Detector.Symbol) == "" {
			return fmt.Errorf("track %q requires detector path and symbol", track.ID)
		}
		if strings.TrimSpace(track.Gate.Name) == "" {
			return fmt.Errorf("track %q requires gate name", track.ID)
		}
		if strings.TrimSpace(track.Documentation.Path) == "" {
			return fmt.Errorf("track %q requires documentation path", track.ID)
		}
		if strings.TrimSpace(track.Fixture.Path) == "" {
			return fmt.Errorf("track %q requires fixture path", track.ID)
		}
		for _, path := range apprenticeshipPaths(track) {
			if strings.TrimSpace(path) == "" {
				continue
			}
			if err := validateRelativePath(path); err != nil {
				return fmt.Errorf("track %q path: %w", track.ID, err)
			}
		}
	}
	return nil
}

func collectApprenticeshipEvidence(root string, paths []string, subject string) ([]ArtifactEvidence, []ApprenticeshipCounterexample) {
	var artifacts []ArtifactEvidence
	var counterexamples []ApprenticeshipCounterexample
	for _, relPath := range uniqueSorted(paths) {
		fullPath, err := safeJoin(root, relPath)
		if err != nil {
			counterexamples = append(counterexamples, ApprenticeshipCounterexample{
				ID:      "track." + stableID(subject, relPath, "evidence-path") + ".invalid",
				Kind:    "invalid_evidence_path",
				Subject: subject,
				Message: err.Error(),
				Witness: []string{relPath},
			})
			continue
		}
		data, err := os.ReadFile(fullPath)
		if err != nil {
			counterexamples = append(counterexamples, ApprenticeshipCounterexample{
				ID:      "track." + stableID(subject, relPath, "evidence") + ".missing",
				Kind:    "missing_evidence",
				Subject: subject,
				Message: "apprenticeship evidence could not be read",
				Witness: []string{relPath},
			})
			continue
		}
		if len(data) == 0 {
			counterexamples = append(counterexamples, ApprenticeshipCounterexample{
				ID:      "track." + stableID(subject, relPath, "evidence") + ".empty",
				Kind:    "empty_evidence",
				Subject: subject,
				Message: "apprenticeship evidence is empty",
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

func artifactExists(evidenceByPath map[string]ArtifactEvidence, relPath string) bool {
	_, ok := evidenceByPath[filepath.ToSlash(filepath.Clean(relPath))]
	return ok
}

func apprenticeshipPaths(track ApprenticeshipTrack) []string {
	paths := []string{track.Detector.Path, track.Documentation.Path, track.Fixture.Path, track.Fixture.NegativeControlPath}
	paths = append(paths, track.Detector.EvidencePaths...)
	return paths
}

func normalizeApprenticeshipCriteria(criteria ApprenticeshipCriteria) ApprenticeshipCriteria {
	criteria.RequiredDeliverables = sortedStrings(normalizedStrings(criteria.RequiredDeliverables))
	return criteria
}

func sortedApprenticeshipTracks(tracks []ApprenticeshipTrack) []ApprenticeshipTrack {
	out := append([]ApprenticeshipTrack(nil), tracks...)
	sort.SliceStable(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func sortApprenticeshipCounterexamples(counterexamples []ApprenticeshipCounterexample) {
	sort.SliceStable(counterexamples, func(i, j int) bool { return counterexamples[i].ID < counterexamples[j].ID })
}

func apprenticeshipReportHash(report ApprenticeshipReport) string {
	report.Hash = ""
	return canonical.Hash(report)
}
