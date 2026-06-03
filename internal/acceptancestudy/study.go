package acceptancestudy

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

const SpecVersion = "patchline.maintainer-acceptance-study/v1"
const ReportVersion = "patchline.maintainer-acceptance-study-report/v1"

const (
	ConditionBaseline      = "baseline"
	ConditionGeneratedPlan = "generated_plan"
)

type Spec struct {
	Version      string        `json:"version"`
	Name         string        `json:"name"`
	Criteria     Criteria      `json:"criteria"`
	Tasks        []Task        `json:"tasks"`
	Observations []Observation `json:"observations"`
}

type Criteria struct {
	MinPairs                      int     `json:"min_pairs"`
	MinReviewTimeReductionPercent float64 `json:"min_review_time_reduction_pct"`
	MinGeneratedUncertaintyRecall float64 `json:"min_generated_uncertainty_recall"`
	MaxUncertaintyRecallDrop      float64 `json:"max_uncertainty_recall_drop"`
	MaxConfidenceIncrease         float64 `json:"max_confidence_increase"`
}

type Task struct {
	ID                       string   `json:"id"`
	Repo                     string   `json:"repo"`
	HazardClass              string   `json:"hazard_class"`
	ArtifactPaths            []string `json:"artifact_paths"`
	GroundTruthUncertainties []string `json:"ground_truth_uncertainties"`
}

type Observation struct {
	ParticipantID              string   `json:"participant_id"`
	Role                       string   `json:"role"`
	TaskID                     string   `json:"task_id"`
	Condition                  string   `json:"condition"`
	ReviewMinutes              float64  `json:"review_minutes"`
	Decision                   string   `json:"decision"`
	CorrectDecision            bool     `json:"correct_decision"`
	Confidence                 float64  `json:"confidence"`
	UncertaintyItemsIdentified []string `json:"uncertainty_items_identified"`
	GeneratedPlanUncertainties []string `json:"generated_plan_uncertainties,omitempty"`
	ParticipantNotesHash       string   `json:"participant_notes_hash,omitempty"`
}

type Report struct {
	Version         string           `json:"version"`
	Name            string           `json:"name"`
	OK              bool             `json:"ok"`
	Criteria        Criteria         `json:"criteria"`
	Summary         Summary          `json:"summary"`
	Tasks           []TaskReport     `json:"tasks"`
	Pairs           []PairReport     `json:"pairs"`
	Counterexamples []Counterexample `json:"counterexamples,omitempty"`
	Hash            string           `json:"hash"`
}

type Summary struct {
	Tasks                      int     `json:"tasks"`
	Participants               int     `json:"participants"`
	Pairs                      int     `json:"pairs"`
	BaselineReviewMinutes      float64 `json:"baseline_review_minutes"`
	GeneratedReviewMinutes     float64 `json:"generated_review_minutes"`
	MeanSavedMinutes           float64 `json:"mean_saved_minutes"`
	ReviewTimeReductionPercent float64 `json:"review_time_reduction_pct"`
	BaselineUncertaintyRecall  float64 `json:"baseline_uncertainty_recall"`
	GeneratedUncertaintyRecall float64 `json:"generated_uncertainty_recall"`
	UncertaintyRecallDelta     float64 `json:"uncertainty_recall_delta"`
	BaselineConfidence         float64 `json:"baseline_confidence"`
	GeneratedConfidence        float64 `json:"generated_confidence"`
	ConfidenceDelta            float64 `json:"confidence_delta"`
	CorrectGeneratedDecisions  int     `json:"correct_generated_decisions"`
	Counterexamples            int     `json:"counterexamples"`
}

type TaskReport struct {
	ID                       string             `json:"id"`
	Repo                     string             `json:"repo"`
	HazardClass              string             `json:"hazard_class"`
	GroundTruthUncertainties []string           `json:"ground_truth_uncertainties"`
	Artifacts                []ArtifactEvidence `json:"artifacts"`
	Pairs                    int                `json:"pairs"`
	BaselineReviewMinutes    float64            `json:"baseline_review_minutes"`
	GeneratedReviewMinutes   float64            `json:"generated_review_minutes"`
	ReviewTimeReductionPct   float64            `json:"review_time_reduction_pct"`
	GeneratedRecall          float64            `json:"generated_uncertainty_recall"`
}

type ArtifactEvidence struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
	Bytes  int64  `json:"bytes"`
}

type PairReport struct {
	ID                               string   `json:"id"`
	ParticipantID                    string   `json:"participant_id"`
	Role                             string   `json:"role"`
	TaskID                           string   `json:"task_id"`
	BaselineReviewMinutes            float64  `json:"baseline_review_minutes"`
	GeneratedReviewMinutes           float64  `json:"generated_review_minutes"`
	SavedMinutes                     float64  `json:"saved_minutes"`
	ReviewTimeReductionPercent       float64  `json:"review_time_reduction_pct"`
	BaselineUncertaintyRecall        float64  `json:"baseline_uncertainty_recall"`
	GeneratedUncertaintyRecall       float64  `json:"generated_uncertainty_recall"`
	UncertaintyRecallDelta           float64  `json:"uncertainty_recall_delta"`
	BaselineConfidence               float64  `json:"baseline_confidence"`
	GeneratedConfidence              float64  `json:"generated_confidence"`
	ConfidenceDelta                  float64  `json:"confidence_delta"`
	BaselineDecision                 string   `json:"baseline_decision"`
	GeneratedDecision                string   `json:"generated_decision"`
	GeneratedCorrectDecision         bool     `json:"generated_correct_decision"`
	GeneratedMissingUncertainties    []string `json:"generated_missing_uncertainties,omitempty"`
	GeneratedPlanDeclaredUncertainty []string `json:"generated_plan_declared_uncertainties,omitempty"`
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
		return Spec{}, fmt.Errorf("maintainer acceptance study spec version must be %s", SpecVersion)
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
	report := Report{
		Version:  ReportVersion,
		Name:     spec.Name,
		OK:       true,
		Criteria: spec.Criteria,
	}
	taskByID := map[string]Task{}
	for _, task := range spec.Tasks {
		taskByID[task.ID] = task
	}
	observationPairs, unmatched := pairObservations(spec.Observations)
	for _, missing := range unmatched {
		report.Counterexamples = append(report.Counterexamples, missing)
	}
	taskReports := map[string]*TaskReport{}
	for _, task := range sortedTasks(spec.Tasks) {
		artifacts, err := collectArtifacts(rootAbs, task.ArtifactPaths)
		if err != nil {
			return Report{}, err
		}
		tr := TaskReport{
			ID:                       task.ID,
			Repo:                     task.Repo,
			HazardClass:              task.HazardClass,
			GroundTruthUncertainties: sortedStrings(task.GroundTruthUncertainties),
			Artifacts:                artifacts,
		}
		taskReports[task.ID] = &tr
		report.Tasks = append(report.Tasks, tr)
	}
	for _, pair := range sortedPairs(observationPairs) {
		task := taskByID[pair.TaskID]
		pairReport := buildPairReport(task, pair.Baseline, pair.Generated)
		report.Pairs = append(report.Pairs, pairReport)
		if tr := taskReports[pair.TaskID]; tr != nil {
			tr.Pairs++
			tr.BaselineReviewMinutes += pairReport.BaselineReviewMinutes
			tr.GeneratedReviewMinutes += pairReport.GeneratedReviewMinutes
			tr.GeneratedRecall += pairReport.GeneratedUncertaintyRecall
		}
		report.Counterexamples = append(report.Counterexamples, pairCounterexamples(pairReport, spec.Criteria)...)
	}
	for i := range report.Tasks {
		if tr := taskReports[report.Tasks[i].ID]; tr != nil {
			finalizeTaskReport(tr)
			report.Tasks[i] = *tr
		}
	}
	report.Summary = summarize(report.Tasks, report.Pairs)
	report.Counterexamples = append(report.Counterexamples, criteriaCounterexamples(report.Summary, spec.Criteria)...)
	sortCounterexamples(report.Counterexamples)
	report.Summary.Counterexamples = len(report.Counterexamples)
	report.OK = len(report.Counterexamples) == 0
	report.Hash = reportHash(report)
	return report, nil
}

func WriteArtifacts(outDir string, report Report) error {
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}
	file, err := os.Create(filepath.Join(outDir, "maintainer-acceptance-study.json"))
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
	return os.WriteFile(filepath.Join(outDir, "maintainer-acceptance-study.md"), []byte(RenderMarkdown(report)), 0o644)
}

func RenderMarkdown(report Report) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Maintainer acceptance study\n\n")
	fmt.Fprintf(&b, "Patchline compares paired maintainer reviews of baseline remediation material against generated remediation plans, then fails the study if review time improves by hiding declared uncertainty.\n\n")
	fmt.Fprintf(&b, "## Summary\n\n")
	fmt.Fprintf(&b, "| Metric | Value |\n| --- | ---: |\n")
	fmt.Fprintf(&b, "| OK | `%t` |\n", report.OK)
	fmt.Fprintf(&b, "| Tasks | %d |\n", report.Summary.Tasks)
	fmt.Fprintf(&b, "| Participants | %d |\n", report.Summary.Participants)
	fmt.Fprintf(&b, "| Paired reviews | %d |\n", report.Summary.Pairs)
	fmt.Fprintf(&b, "| Baseline minutes | %.2f |\n", report.Summary.BaselineReviewMinutes)
	fmt.Fprintf(&b, "| Generated-plan minutes | %.2f |\n", report.Summary.GeneratedReviewMinutes)
	fmt.Fprintf(&b, "| Mean minutes saved | %.2f |\n", report.Summary.MeanSavedMinutes)
	fmt.Fprintf(&b, "| Review-time reduction | %.2f%% |\n", report.Summary.ReviewTimeReductionPercent)
	fmt.Fprintf(&b, "| Baseline uncertainty recall | %.2f |\n", report.Summary.BaselineUncertaintyRecall)
	fmt.Fprintf(&b, "| Generated uncertainty recall | %.2f |\n", report.Summary.GeneratedUncertaintyRecall)
	fmt.Fprintf(&b, "| Confidence delta | %.2f |\n", report.Summary.ConfidenceDelta)
	fmt.Fprintf(&b, "| Counterexamples | %d |\n\n", report.Summary.Counterexamples)

	fmt.Fprintf(&b, "Policy: at least `%d` paired reviews, at least `%.2f%%` review-time reduction, generated uncertainty recall at least `%.2f`, recall drop no worse than `%.2f`, and confidence increase no more than `%.2f`.\n\n",
		report.Criteria.MinPairs,
		report.Criteria.MinReviewTimeReductionPercent,
		report.Criteria.MinGeneratedUncertaintyRecall,
		report.Criteria.MaxUncertaintyRecallDrop,
		report.Criteria.MaxConfidenceIncrease,
	)

	fmt.Fprintf(&b, "## Real-code evidence\n\n")
	fmt.Fprintf(&b, "| Task | Repo | Hazard | Artifact | SHA-256 | Bytes |\n| --- | --- | --- | --- | --- | ---: |\n")
	for _, task := range report.Tasks {
		for _, artifact := range task.Artifacts {
			fmt.Fprintf(&b, "| `%s` | `%s` | `%s` | `%s` | `%s` | %d |\n",
				escapeTable(task.ID),
				escapeTable(task.Repo),
				escapeTable(task.HazardClass),
				escapeTable(artifact.Path),
				artifact.SHA256[:16],
				artifact.Bytes,
			)
		}
	}

	fmt.Fprintf(&b, "\n## Paired review outcomes\n\n")
	fmt.Fprintf(&b, "| Pair | Role | Baseline min | Generated min | Saved | Generated recall | Confidence delta | Correct |\n| --- | --- | ---: | ---: | ---: | ---: | ---: | ---: |\n")
	for _, pair := range report.Pairs {
		fmt.Fprintf(&b, "| `%s` | `%s` | %.2f | %.2f | %.2f | %.2f | %.2f | `%t` |\n",
			escapeTable(pair.ID),
			escapeTable(pair.Role),
			pair.BaselineReviewMinutes,
			pair.GeneratedReviewMinutes,
			pair.SavedMinutes,
			pair.GeneratedUncertaintyRecall,
			pair.ConfidenceDelta,
			pair.GeneratedCorrectDecision,
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

type observationPair struct {
	ParticipantID string
	TaskID        string
	Baseline      Observation
	Generated     Observation
}

func validateSpec(spec Spec) error {
	if strings.TrimSpace(spec.Name) == "" {
		return fmt.Errorf("maintainer acceptance study name is required")
	}
	if spec.Criteria.MinPairs <= 0 {
		return fmt.Errorf("criteria.min_pairs must be positive")
	}
	if spec.Criteria.MinReviewTimeReductionPercent < 0 {
		return fmt.Errorf("criteria.min_review_time_reduction_pct must be non-negative")
	}
	if spec.Criteria.MinGeneratedUncertaintyRecall < 0 || spec.Criteria.MinGeneratedUncertaintyRecall > 1 {
		return fmt.Errorf("criteria.min_generated_uncertainty_recall must be between 0 and 1")
	}
	if spec.Criteria.MaxUncertaintyRecallDrop < 0 || spec.Criteria.MaxUncertaintyRecallDrop > 1 {
		return fmt.Errorf("criteria.max_uncertainty_recall_drop must be between 0 and 1")
	}
	if spec.Criteria.MaxConfidenceIncrease < 0 || spec.Criteria.MaxConfidenceIncrease > 1 {
		return fmt.Errorf("criteria.max_confidence_increase must be between 0 and 1")
	}
	taskIDs := map[string]struct{}{}
	for _, task := range spec.Tasks {
		if strings.TrimSpace(task.ID) == "" {
			return fmt.Errorf("task id is required")
		}
		if _, exists := taskIDs[task.ID]; exists {
			return fmt.Errorf("duplicate task id %q", task.ID)
		}
		taskIDs[task.ID] = struct{}{}
		if strings.TrimSpace(task.Repo) == "" || strings.TrimSpace(task.HazardClass) == "" {
			return fmt.Errorf("task %q requires repo and hazard_class", task.ID)
		}
		if len(task.ArtifactPaths) == 0 {
			return fmt.Errorf("task %q requires at least one artifact path", task.ID)
		}
		if len(task.GroundTruthUncertainties) == 0 {
			return fmt.Errorf("task %q requires ground-truth uncertainty items", task.ID)
		}
		seen := map[string]struct{}{}
		for _, item := range task.GroundTruthUncertainties {
			norm := normalizeItem(item)
			if norm == "" {
				return fmt.Errorf("task %q contains empty uncertainty item", task.ID)
			}
			if _, exists := seen[norm]; exists {
				return fmt.Errorf("task %q contains duplicate uncertainty item %q", task.ID, item)
			}
			seen[norm] = struct{}{}
		}
		for _, path := range task.ArtifactPaths {
			if err := validateRelativePath(path); err != nil {
				return fmt.Errorf("task %q artifact path: %w", task.ID, err)
			}
		}
	}
	if len(spec.Observations) == 0 {
		return fmt.Errorf("observations are required")
	}
	seenObservation := map[string]struct{}{}
	for _, observation := range spec.Observations {
		if strings.TrimSpace(observation.ParticipantID) == "" || strings.TrimSpace(observation.Role) == "" {
			return fmt.Errorf("observation requires participant_id and role")
		}
		if _, exists := taskIDs[observation.TaskID]; !exists {
			return fmt.Errorf("observation references unknown task %q", observation.TaskID)
		}
		if observation.Condition != ConditionBaseline && observation.Condition != ConditionGeneratedPlan {
			return fmt.Errorf("observation condition must be %q or %q", ConditionBaseline, ConditionGeneratedPlan)
		}
		if observation.ReviewMinutes <= 0 {
			return fmt.Errorf("observation %s/%s/%s requires positive review_minutes", observation.ParticipantID, observation.TaskID, observation.Condition)
		}
		if observation.Confidence < 0 || observation.Confidence > 1 {
			return fmt.Errorf("observation %s/%s/%s confidence must be between 0 and 1", observation.ParticipantID, observation.TaskID, observation.Condition)
		}
		if strings.TrimSpace(observation.Decision) == "" {
			return fmt.Errorf("observation %s/%s/%s requires decision", observation.ParticipantID, observation.TaskID, observation.Condition)
		}
		key := observation.ParticipantID + "\x00" + observation.TaskID + "\x00" + observation.Condition
		if _, exists := seenObservation[key]; exists {
			return fmt.Errorf("duplicate observation for participant %q task %q condition %q", observation.ParticipantID, observation.TaskID, observation.Condition)
		}
		seenObservation[key] = struct{}{}
	}
	return nil
}

func collectArtifacts(root string, paths []string) ([]ArtifactEvidence, error) {
	var artifacts []ArtifactEvidence
	for _, relPath := range sortedStrings(paths) {
		fullPath, err := safeJoin(root, relPath)
		if err != nil {
			return nil, err
		}
		data, err := os.ReadFile(fullPath)
		if err != nil {
			return nil, fmt.Errorf("read artifact %s: %w", relPath, err)
		}
		sum := sha256.Sum256(data)
		artifacts = append(artifacts, ArtifactEvidence{
			Path:   filepath.ToSlash(filepath.Clean(relPath)),
			SHA256: hex.EncodeToString(sum[:]),
			Bytes:  int64(len(data)),
		})
	}
	return artifacts, nil
}

func pairObservations(observations []Observation) (map[string]observationPair, []Counterexample) {
	type partial struct {
		participant string
		task        string
		baseline    *Observation
		generated   *Observation
	}
	partials := map[string]*partial{}
	for _, observation := range observations {
		key := observation.ParticipantID + "\x00" + observation.TaskID
		item := partials[key]
		if item == nil {
			item = &partial{participant: observation.ParticipantID, task: observation.TaskID}
			partials[key] = item
		}
		obs := observation
		if observation.Condition == ConditionBaseline {
			item.baseline = &obs
		} else {
			item.generated = &obs
		}
	}
	pairs := map[string]observationPair{}
	var counterexamples []Counterexample
	for _, key := range sortedMapKeys(partials) {
		item := partials[key]
		if item.baseline == nil || item.generated == nil {
			missing := ConditionBaseline
			if item.generated == nil {
				missing = ConditionGeneratedPlan
			}
			counterexamples = append(counterexamples, Counterexample{
				ID:      "pair." + stableID(item.participant, item.task) + ".missing_" + missing,
				Kind:    "missing_pair",
				Subject: item.task,
				Message: fmt.Sprintf("participant %s lacks a paired %s observation", item.participant, missing),
				Witness: []string{item.participant, item.task},
			})
			continue
		}
		pairs[key] = observationPair{
			ParticipantID: item.participant,
			TaskID:        item.task,
			Baseline:      *item.baseline,
			Generated:     *item.generated,
		}
	}
	return pairs, counterexamples
}

func buildPairReport(task Task, baseline Observation, generated Observation) PairReport {
	baselineRecall, _ := uncertaintyRecall(task, baseline.UncertaintyItemsIdentified)
	generatedItems := generated.UncertaintyItemsIdentified
	if len(generated.GeneratedPlanUncertainties) > 0 {
		generatedItems = append([]string{}, generated.GeneratedPlanUncertainties...)
	}
	generatedRecall, missing := uncertaintyRecall(task, generatedItems)
	saved := round4(baseline.ReviewMinutes - generated.ReviewMinutes)
	reduction := 0.0
	if baseline.ReviewMinutes > 0 {
		reduction = round4(saved / baseline.ReviewMinutes * 100)
	}
	return PairReport{
		ID:                               stableID(baseline.ParticipantID, task.ID),
		ParticipantID:                    baseline.ParticipantID,
		Role:                             baseline.Role,
		TaskID:                           task.ID,
		BaselineReviewMinutes:            round4(baseline.ReviewMinutes),
		GeneratedReviewMinutes:           round4(generated.ReviewMinutes),
		SavedMinutes:                     saved,
		ReviewTimeReductionPercent:       reduction,
		BaselineUncertaintyRecall:        baselineRecall,
		GeneratedUncertaintyRecall:       generatedRecall,
		UncertaintyRecallDelta:           round4(generatedRecall - baselineRecall),
		BaselineConfidence:               round4(baseline.Confidence),
		GeneratedConfidence:              round4(generated.Confidence),
		ConfidenceDelta:                  round4(generated.Confidence - baseline.Confidence),
		BaselineDecision:                 baseline.Decision,
		GeneratedDecision:                generated.Decision,
		GeneratedCorrectDecision:         generated.CorrectDecision,
		GeneratedMissingUncertainties:    missing,
		GeneratedPlanDeclaredUncertainty: sortedStrings(generated.GeneratedPlanUncertainties),
	}
}

func pairCounterexamples(pair PairReport, criteria Criteria) []Counterexample {
	var counterexamples []Counterexample
	if len(pair.GeneratedMissingUncertainties) > 0 {
		counterexamples = append(counterexamples, Counterexample{
			ID:      "pair." + pair.ID + ".generated.hidden_uncertainty",
			Kind:    "hidden_uncertainty",
			Subject: pair.TaskID,
			Message: "generated remediation plan did not surface every ground-truth uncertainty item",
			Witness: pair.GeneratedMissingUncertainties,
		})
	}
	if !pair.GeneratedCorrectDecision {
		counterexamples = append(counterexamples, Counterexample{
			ID:      "pair." + pair.ID + ".generated.decision",
			Kind:    "incorrect_decision",
			Subject: pair.TaskID,
			Message: "generated remediation plan led to an incorrect maintainer decision",
			Witness: []string{pair.GeneratedDecision},
		})
	}
	if pair.ConfidenceDelta > criteria.MaxConfidenceIncrease && len(pair.GeneratedMissingUncertainties) > 0 {
		counterexamples = append(counterexamples, Counterexample{
			ID:      "pair." + pair.ID + ".generated.overconfidence",
			Kind:    "overconfidence",
			Subject: pair.TaskID,
			Message: "generated remediation plan increased confidence while hiding uncertainty",
			Witness: []string{fmt.Sprintf("delta=%.4f", pair.ConfidenceDelta)},
		})
	}
	return counterexamples
}

func criteriaCounterexamples(summary Summary, criteria Criteria) []Counterexample {
	var counterexamples []Counterexample
	if summary.Pairs < criteria.MinPairs {
		counterexamples = append(counterexamples, Counterexample{
			ID:      "criteria.min_pairs",
			Kind:    "underpowered_study",
			Message: fmt.Sprintf("paired reviews %d below required %d", summary.Pairs, criteria.MinPairs),
		})
	}
	if summary.ReviewTimeReductionPercent < criteria.MinReviewTimeReductionPercent {
		counterexamples = append(counterexamples, Counterexample{
			ID:      "criteria.review_time_reduction",
			Kind:    "insufficient_time_reduction",
			Message: fmt.Sprintf("review-time reduction %.2f%% below required %.2f%%", summary.ReviewTimeReductionPercent, criteria.MinReviewTimeReductionPercent),
		})
	}
	if summary.GeneratedUncertaintyRecall < criteria.MinGeneratedUncertaintyRecall {
		counterexamples = append(counterexamples, Counterexample{
			ID:      "criteria.generated_uncertainty_recall",
			Kind:    "hidden_uncertainty",
			Message: fmt.Sprintf("generated uncertainty recall %.4f below required %.4f", summary.GeneratedUncertaintyRecall, criteria.MinGeneratedUncertaintyRecall),
		})
	}
	if summary.UncertaintyRecallDelta < -criteria.MaxUncertaintyRecallDrop {
		counterexamples = append(counterexamples, Counterexample{
			ID:      "criteria.uncertainty_recall_drop",
			Kind:    "uncertainty_regression",
			Message: fmt.Sprintf("generated uncertainty recall delta %.4f worse than allowed drop %.4f", summary.UncertaintyRecallDelta, criteria.MaxUncertaintyRecallDrop),
		})
	}
	if summary.ConfidenceDelta > criteria.MaxConfidenceIncrease {
		counterexamples = append(counterexamples, Counterexample{
			ID:      "criteria.confidence_increase",
			Kind:    "overconfidence",
			Message: fmt.Sprintf("generated confidence increase %.4f above allowed %.4f", summary.ConfidenceDelta, criteria.MaxConfidenceIncrease),
		})
	}
	return counterexamples
}

func uncertaintyRecall(task Task, observed []string) (float64, []string) {
	truth := map[string]string{}
	for _, item := range task.GroundTruthUncertainties {
		truth[normalizeItem(item)] = item
	}
	seen := map[string]struct{}{}
	for _, item := range observed {
		norm := normalizeItem(item)
		if norm != "" {
			seen[norm] = struct{}{}
		}
	}
	var missing []string
	hits := 0
	for _, norm := range sortedMapKeysString(truth) {
		if _, ok := seen[norm]; ok {
			hits++
		} else {
			missing = append(missing, truth[norm])
		}
	}
	if len(truth) == 0 {
		return 1, nil
	}
	return round4(float64(hits) / float64(len(truth))), sortedStrings(missing)
}

func summarize(tasks []TaskReport, pairs []PairReport) Summary {
	summary := Summary{Tasks: len(tasks), Pairs: len(pairs)}
	participants := map[string]struct{}{}
	for _, pair := range pairs {
		participants[pair.ParticipantID] = struct{}{}
		summary.BaselineReviewMinutes += pair.BaselineReviewMinutes
		summary.GeneratedReviewMinutes += pair.GeneratedReviewMinutes
		summary.MeanSavedMinutes += pair.SavedMinutes
		summary.BaselineUncertaintyRecall += pair.BaselineUncertaintyRecall
		summary.GeneratedUncertaintyRecall += pair.GeneratedUncertaintyRecall
		summary.BaselineConfidence += pair.BaselineConfidence
		summary.GeneratedConfidence += pair.GeneratedConfidence
		if pair.GeneratedCorrectDecision {
			summary.CorrectGeneratedDecisions++
		}
	}
	summary.Participants = len(participants)
	if len(pairs) > 0 {
		n := float64(len(pairs))
		summary.BaselineReviewMinutes = round4(summary.BaselineReviewMinutes / n)
		summary.GeneratedReviewMinutes = round4(summary.GeneratedReviewMinutes / n)
		summary.MeanSavedMinutes = round4(summary.MeanSavedMinutes / n)
		summary.BaselineUncertaintyRecall = round4(summary.BaselineUncertaintyRecall / n)
		summary.GeneratedUncertaintyRecall = round4(summary.GeneratedUncertaintyRecall / n)
		summary.UncertaintyRecallDelta = round4(summary.GeneratedUncertaintyRecall - summary.BaselineUncertaintyRecall)
		summary.BaselineConfidence = round4(summary.BaselineConfidence / n)
		summary.GeneratedConfidence = round4(summary.GeneratedConfidence / n)
		summary.ConfidenceDelta = round4(summary.GeneratedConfidence - summary.BaselineConfidence)
		if summary.BaselineReviewMinutes > 0 {
			summary.ReviewTimeReductionPercent = round4((summary.BaselineReviewMinutes - summary.GeneratedReviewMinutes) / summary.BaselineReviewMinutes * 100)
		}
	}
	return summary
}

func finalizeTaskReport(task *TaskReport) {
	if task.Pairs == 0 {
		return
	}
	n := float64(task.Pairs)
	task.BaselineReviewMinutes = round4(task.BaselineReviewMinutes / n)
	task.GeneratedReviewMinutes = round4(task.GeneratedReviewMinutes / n)
	task.GeneratedRecall = round4(task.GeneratedRecall / n)
	if task.BaselineReviewMinutes > 0 {
		task.ReviewTimeReductionPct = round4((task.BaselineReviewMinutes - task.GeneratedReviewMinutes) / task.BaselineReviewMinutes * 100)
	}
}

func sortedTasks(tasks []Task) []Task {
	out := append([]Task(nil), tasks...)
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func sortedPairs(pairs map[string]observationPair) []observationPair {
	keys := sortedMapKeys(pairs)
	out := make([]observationPair, 0, len(keys))
	for _, key := range keys {
		out = append(out, pairs[key])
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].TaskID == out[j].TaskID {
			return out[i].ParticipantID < out[j].ParticipantID
		}
		return out[i].TaskID < out[j].TaskID
	})
	return out
}

func sortedStrings(items []string) []string {
	out := append([]string(nil), items...)
	sort.Strings(out)
	return out
}

func sortedMapKeys[T any](m map[string]T) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func sortedMapKeysString(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func sortCounterexamples(counterexamples []Counterexample) {
	sort.Slice(counterexamples, func(i, j int) bool { return counterexamples[i].ID < counterexamples[j].ID })
}

func normalizeItem(item string) string {
	return strings.ToLower(strings.Join(strings.Fields(strings.TrimSpace(item)), " "))
}

func stableID(parts ...string) string {
	return canonical.Hash(strings.Join(parts, "\x00"))[:16]
}

func reportHash(report Report) string {
	report.Hash = ""
	return canonical.Hash(report)
}

func validateRelativePath(path string) error {
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("empty path")
	}
	if filepath.IsAbs(path) {
		return fmt.Errorf("absolute paths are not allowed: %s", path)
	}
	clean := filepath.Clean(path)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return fmt.Errorf("path escapes root: %s", path)
	}
	return nil
}

func safeJoin(root string, relPath string) (string, error) {
	if err := validateRelativePath(relPath); err != nil {
		return "", err
	}
	full := filepath.Join(root, relPath)
	rel, err := filepath.Rel(root, full)
	if err != nil {
		return "", err
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return "", fmt.Errorf("artifact path escapes root: %s", relPath)
	}
	return full, nil
}

func round4(value float64) float64 {
	if value == 0 {
		return 0
	}
	if value > 0 {
		return float64(int64(value*10000+0.5)) / 10000
	}
	return float64(int64(value*10000-0.5)) / 10000
}

func escapeTable(value string) string {
	return strings.ReplaceAll(value, "|", "\\|")
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
