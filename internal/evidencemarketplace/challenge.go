package evidencemarketplace

import (
	"bytes"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/thehalleyyoung/patchline/internal/canonical"
	"github.com/thehalleyyoung/patchline/internal/migration"
)

const ChallengeReportVersion = "patchline.adversarial-challenge-report/v1"

type ChallengeReport struct {
	Version        string                     `json:"version"`
	OK             bool                       `json:"ok"`
	RegistryHash   string                     `json:"registry_hash"`
	BaseReportHash string                     `json:"base_report_hash"`
	Hash           string                     `json:"hash"`
	Track          ChallengeTrack             `json:"track"`
	Summary        ChallengeSummary           `json:"summary"`
	Scoreboard     []ChallengeScoreboardEntry `json:"scoreboard"`
	Excluded       []ChallengeScoreboardEntry `json:"excluded,omitempty"`
	Rejected       []RejectedExample          `json:"rejected,omitempty"`
	Markdown       string                     `json:"markdown,omitempty"`
}

type ChallengeSummary struct {
	Submitted              int `json:"submitted"`
	Accepted               int `json:"accepted"`
	Rejected               int `json:"rejected"`
	PublicSafe             int `json:"public_safe"`
	DisclosureReady        int `json:"disclosure_ready"`
	AnalyzerProofs         int `json:"analyzer_proofs"`
	ScoreboardEntries      int `json:"scoreboard_entries"`
	ExcludedBelowThreshold int `json:"excluded_below_threshold"`
	MaxScore               int `json:"max_score"`
	MinScoreboardScore     int `json:"min_scoreboard_score"`
}

type ChallengeScoreboardEntry struct {
	ID                     string                     `json:"id"`
	Title                  string                     `json:"title"`
	Ecosystem              string                     `json:"ecosystem"`
	HazardClass            string                     `json:"hazard_class"`
	CertificateSubjectHash string                     `json:"certificate_subject_hash"`
	EvidenceHash           string                     `json:"evidence_hash"`
	Score                  int                        `json:"score"`
	MaxScore               int                        `json:"max_score"`
	ScorePercent           int                        `json:"score_percent"`
	Tier                   string                     `json:"tier"`
	ScoreboardEligible     bool                       `json:"scoreboard_eligible"`
	PublicSafe             bool                       `json:"public_safe"`
	DisclosureReady        bool                       `json:"disclosure_ready"`
	Challenge              ChallengeSubmission        `json:"challenge_submission"`
	MigrationAnalysis      ChallengeMigrationAnalysis `json:"migration_analysis"`
	Breakdown              ChallengeScoreBreakdown    `json:"breakdown"`
}

type ChallengeMigrationAnalysis struct {
	ArtifactPath     string   `json:"artifact_path"`
	ArtifactSHA256   string   `json:"artifact_sha256"`
	ReportHash       string   `json:"report_hash"`
	TotalStatements  int      `json:"total_statements"`
	HighRisk         int      `json:"high_risk"`
	MediumRisk       int      `json:"medium_risk"`
	LowRisk          int      `json:"low_risk"`
	Tables           []string `json:"tables"`
	ActualBehavior   string   `json:"actual_behavior"`
	AnalyzerMatched  bool     `json:"analyzer_matched_expected"`
	PublicProofLines int      `json:"public_proof_lines"`
}

type ChallengeScoreBreakdown struct {
	AnalyzerSignal        int `json:"analyzer_signal"`
	Reproducibility       int `json:"reproducibility"`
	Minimization          int `json:"minimization"`
	Novelty               int `json:"novelty"`
	ResponsibleDisclosure int `json:"responsible_disclosure"`
}

func PublishChallengeTrackFile(path string) (ChallengeReport, error) {
	registry, err := ReadRegistryFile(path)
	if err != nil {
		return ChallengeReport{}, err
	}
	return PublishChallengeTrack(registry, filepath.Dir(path))
}

func PublishChallengeTrack(registry Registry, root string) (ChallengeReport, error) {
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return ChallengeReport{}, err
	}
	base, err := PublishRegistry(registry, rootAbs)
	if err != nil {
		return ChallengeReport{}, err
	}
	report := ChallengeReport{
		Version:        ChallengeReportVersion,
		RegistryHash:   base.RegistryHash,
		BaseReportHash: base.Hash,
		Summary: ChallengeSummary{
			Submitted: countChallengeSubmissions(registry.Examples),
		},
	}
	if registry.ChallengeTrack != nil {
		report.Track = normalizeChallengeTrack(*registry.ChallengeTrack)
	}
	report.Summary.MaxScore = challengeMaxScore(report.Track.Scoring.Weights)
	report.Summary.MinScoreboardScore = report.Track.Scoring.MinScoreboardScore

	for _, rejected := range base.Rejected {
		report.Rejected = append(report.Rejected, rejected)
	}

	publishedByID := map[string]PublishedExample{}
	for _, example := range base.Examples {
		publishedByID[example.ID] = example
	}
	trackReasons := validateChallengeTrack(registry.ChallengeTrack)
	for _, example := range registry.Examples {
		if example.Challenge == nil {
			continue
		}
		id := stableRejectedID(example.ID)
		published, ok := publishedByID[id]
		if !ok {
			continue
		}
		if len(trackReasons) > 0 {
			report.Rejected = append(report.Rejected, RejectedExample{ID: id, Reasons: trackReasons})
			continue
		}
		entry, reasons := scoreChallengeSubmission(example, published, report.Track, rootAbs)
		if len(reasons) > 0 {
			sort.Strings(reasons)
			report.Rejected = append(report.Rejected, RejectedExample{ID: id, Reasons: reasons})
			continue
		}
		report.Summary.Accepted++
		if entry.PublicSafe {
			report.Summary.PublicSafe++
		}
		if entry.DisclosureReady {
			report.Summary.DisclosureReady++
		}
		if entry.MigrationAnalysis.ReportHash != "" {
			report.Summary.AnalyzerProofs++
		}
		if entry.ScoreboardEligible {
			report.Scoreboard = append(report.Scoreboard, entry)
		} else {
			report.Excluded = append(report.Excluded, entry)
		}
	}

	sortChallengeEntries(report.Scoreboard)
	sortChallengeEntries(report.Excluded)
	sort.Slice(report.Rejected, func(i, j int) bool {
		return report.Rejected[i].ID < report.Rejected[j].ID
	})
	report.Summary.ScoreboardEntries = len(report.Scoreboard)
	report.Summary.ExcludedBelowThreshold = len(report.Excluded)
	report.Summary.Rejected = len(report.Rejected)
	report.OK = report.Summary.Submitted > 0 && report.Summary.Accepted > 0 && report.Summary.Rejected == 0
	report.Hash = challengeReportHash(report)
	report.Markdown = RenderChallengeMarkdown(report)
	return report, nil
}

func WriteChallengeReport(outDir string, report ChallengeReport) error {
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}
	if err := writeJSON(filepath.Join(outDir, "challenge.json"), report); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(outDir, "challenge.md"), []byte(report.Markdown), 0o644); err != nil {
		return err
	}
	html, err := RenderChallengeHTML(report)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(outDir, "index.html"), []byte(html), 0o644)
}

func countChallengeSubmissions(examples []Example) int {
	count := 0
	for _, example := range examples {
		if example.Challenge != nil {
			count++
		}
	}
	return count
}

func validateChallengeTrack(track *ChallengeTrack) []string {
	if track == nil {
		return []string{"challenge_track is required when challenge_submission examples are present"}
	}
	normalized := normalizeChallengeTrack(*track)
	var reasons []string
	for field, value := range map[string]string{
		"challenge_track.id":                                normalized.ID,
		"challenge_track.name":                              normalized.Name,
		"challenge_track.rules_url":                         normalized.RulesURL,
		"challenge_track.responsible_disclosure.contact":    normalized.ResponsibleDisclosure.Contact,
		"challenge_track.responsible_disclosure.policy_url": normalized.ResponsibleDisclosure.PolicyURL,
	} {
		if value == "" {
			reasons = append(reasons, field+" is required")
		}
	}
	if normalized.ResponsibleDisclosure.EmbargoDays <= 0 || normalized.ResponsibleDisclosure.EmbargoDays > 365 {
		reasons = append(reasons, "challenge_track.responsible_disclosure.embargo_days must be between 1 and 365")
	}
	if !normalized.ResponsibleDisclosure.PublicSafeArtifactsOnly {
		reasons = append(reasons, "challenge_track.responsible_disclosure.public_safe_artifacts_only must be true")
	}
	weightTotal := challengeMaxScore(normalized.Scoring.Weights)
	if weightTotal <= 0 {
		reasons = append(reasons, "challenge_track.scoring.weights must assign positive deterministic scoring points")
	}
	if normalized.Scoring.MinScoreboardScore <= 0 {
		reasons = append(reasons, "challenge_track.scoring.min_scoreboard_score must be positive")
	}
	if weightTotal > 0 && normalized.Scoring.MinScoreboardScore > weightTotal {
		reasons = append(reasons, "challenge_track.scoring.min_scoreboard_score must not exceed total weights")
	}
	reasons = append(reasons, scanPublicStrings("challenge_track", challengeTrackStrings(normalized))...)
	sort.Strings(reasons)
	return reasons
}

func scoreChallengeSubmission(example Example, published PublishedExample, track ChallengeTrack, root string) (ChallengeScoreboardEntry, []string) {
	challenge := normalizeChallengeSubmission(example.Challenge)
	if challenge == nil {
		return ChallengeScoreboardEntry{}, []string{"challenge_submission is required"}
	}
	var reasons []string
	if challenge.TrackID != track.ID {
		reasons = append(reasons, "challenge_submission.track_id must match challenge_track.id")
	}
	for field, value := range map[string]string{
		"challenge_submission.adversarial_goal":            challenge.AdversarialGoal,
		"challenge_submission.attack_surface":              challenge.AttackSurface,
		"challenge_submission.expected_detector_behavior":  challenge.ExpectedDetectorBehavior,
		"challenge_submission.migration_artifact":          challenge.MigrationArtifact,
		"challenge_submission.novelty_statement":           challenge.NoveltyStatement,
		"challenge_submission.disclosure.status":           challenge.Disclosure.Status,
		"challenge_submission.disclosure.coordinated_with": challenge.Disclosure.CoordinatedWith,
		"challenge_submission.disclosure.reported_at":      challenge.Disclosure.ReportedAt,
	} {
		if value == "" {
			reasons = append(reasons, field+" is required")
		}
	}
	if challenge.ExpectedDetectorBehavior != "" && challenge.ExpectedDetectorBehavior != "flag-high-risk-migration" {
		reasons = append(reasons, "challenge_submission.expected_detector_behavior must be flag-high-risk-migration")
	}
	if challenge.MaxPublicProofLines <= 0 {
		reasons = append(reasons, "challenge_submission.max_public_proof_lines must be positive")
	}
	if challenge.Disclosure.Status != "" && challenge.Disclosure.Status != "public-safe" {
		reasons = append(reasons, "challenge_submission.disclosure.status must be public-safe for public challenge publication")
	}
	if !challenge.Disclosure.PublicReleaseAllowed {
		reasons = append(reasons, "challenge_submission.disclosure.public_release_allowed must be true")
	}
	if challenge.Disclosure.FullExploitHash != "" && !strings.HasPrefix(challenge.Disclosure.FullExploitHash, "sha256:") {
		reasons = append(reasons, "challenge_submission.disclosure.full_exploit_hash must be sha256-prefixed when present")
	}
	if challenge.Disclosure.ReportedAt != "" {
		if _, err := time.Parse(time.RFC3339, challenge.Disclosure.ReportedAt); err != nil {
			reasons = append(reasons, "challenge_submission.disclosure.reported_at must be RFC3339")
		}
	}
	if challenge.Disclosure.EmbargoEndsAt != "" {
		if _, err := time.Parse(time.RFC3339, challenge.Disclosure.EmbargoEndsAt); err != nil {
			reasons = append(reasons, "challenge_submission.disclosure.embargo_ends_at must be RFC3339")
		}
	}
	obligations := stringSet(example.Certificate.Obligations)
	if !obligations["responsible-disclosure-cleared"] {
		reasons = append(reasons, "certificate missing challenge obligation responsible-disclosure-cleared")
	}
	reasons = append(reasons, scanPublicStrings("challenge_submission", challengeStrings(*challenge))...)

	artifact, artifactReasons := challengeMigrationArtifact(published, challenge.MigrationArtifact)
	reasons = append(reasons, artifactReasons...)
	var analysis ChallengeMigrationAnalysis
	if len(artifactReasons) == 0 && challenge.MigrationArtifact != "" {
		var analysisReasons []string
		analysis, analysisReasons = analyzeChallengeMigration(root, artifact, *challenge)
		reasons = append(reasons, analysisReasons...)
	}
	if len(reasons) > 0 {
		return ChallengeScoreboardEntry{}, reasons
	}

	breakdown := scoreChallengeBreakdown(published, analysis, track.Scoring.Weights)
	score := breakdown.AnalyzerSignal + breakdown.Reproducibility + breakdown.Minimization + breakdown.Novelty + breakdown.ResponsibleDisclosure
	maxScore := challengeMaxScore(track.Scoring.Weights)
	entry := ChallengeScoreboardEntry{
		ID:                     published.ID,
		Title:                  published.Title,
		Ecosystem:              published.Ecosystem,
		HazardClass:            published.HazardClass,
		CertificateSubjectHash: published.CertificateSubjectHash,
		EvidenceHash:           published.EvidenceHash,
		Score:                  score,
		MaxScore:               maxScore,
		Tier:                   challengeScoreTier(score, maxScore),
		ScoreboardEligible:     score >= track.Scoring.MinScoreboardScore,
		PublicSafe:             challenge.Disclosure.Status == "public-safe" && challenge.Disclosure.PublicReleaseAllowed,
		DisclosureReady:        true,
		Challenge:              *challenge,
		MigrationAnalysis:      analysis,
		Breakdown:              breakdown,
	}
	if maxScore > 0 {
		entry.ScorePercent = (score * 100) / maxScore
	}
	return entry, nil
}

func challengeMigrationArtifact(published PublishedExample, path string) (ArtifactSummary, []string) {
	var reasons []string
	for _, artifact := range published.Artifacts {
		if artifact.Path != path {
			continue
		}
		if artifact.Role != "adversarial-migration" {
			reasons = append(reasons, "challenge_submission.migration_artifact must reference an adversarial-migration artifact")
		}
		if !artifact.Redacted {
			reasons = append(reasons, "challenge_submission.migration_artifact must be redacted")
		}
		return artifact, reasons
	}
	return ArtifactSummary{}, []string{"challenge_submission.migration_artifact must reference a published artifact"}
}

func analyzeChallengeMigration(root string, artifact ArtifactSummary, challenge ChallengeSubmission) (ChallengeMigrationAnalysis, []string) {
	path, err := resolveArtifact(root, artifact.Path)
	if err != nil {
		return ChallengeMigrationAnalysis{}, []string{"challenge migration artifact " + artifact.Path + ": " + err.Error()}
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return ChallengeMigrationAnalysis{}, []string{"challenge migration artifact " + artifact.Path + ": " + err.Error()}
	}
	lineCount := countLines(content)
	report, err := migration.AnalyzeBytes(artifact.Path, content)
	if err != nil {
		return ChallengeMigrationAnalysis{}, []string{"challenge migration analyzer failed: " + err.Error()}
	}
	analysis := ChallengeMigrationAnalysis{
		ArtifactPath:     artifact.Path,
		ArtifactSHA256:   artifact.SHA256,
		ReportHash:       "sha256:" + report.Summary.ReportHash,
		TotalStatements:  report.Summary.TotalStatements,
		HighRisk:         report.Summary.HighRisk,
		MediumRisk:       report.Summary.MediumRisk,
		LowRisk:          report.Summary.LowRisk,
		Tables:           append([]string(nil), report.Summary.Tables...),
		PublicProofLines: lineCount,
	}
	if report.Summary.HighRisk > 0 {
		analysis.ActualBehavior = "flag-high-risk-migration"
	} else {
		analysis.ActualBehavior = "no-high-risk-migration"
	}
	analysis.AnalyzerMatched = analysis.ActualBehavior == challenge.ExpectedDetectorBehavior
	return analysis, nil
}

func scoreChallengeBreakdown(published PublishedExample, analysis ChallengeMigrationAnalysis, weights ChallengeScoreWeights) ChallengeScoreBreakdown {
	var score ChallengeScoreBreakdown
	if analysis.AnalyzerMatched {
		score.AnalyzerSignal = weights.AnalyzerSignal
	} else if analysis.HighRisk > 0 || analysis.MediumRisk > 0 {
		score.AnalyzerSignal = weights.AnalyzerSignal / 2
	}
	if published.GateReputation.Score >= 50 && len(published.Reproduction) >= 2 {
		score.Reproducibility = weights.Reproducibility
	} else if len(published.Reproduction) > 0 {
		score.Reproducibility = weights.Reproducibility / 2
	}
	maxProofLines := 0
	if published.Challenge != nil {
		maxProofLines = published.Challenge.MaxPublicProofLines
	}
	if maxProofLines > 0 && analysis.PublicProofLines > 0 {
		if analysis.PublicProofLines <= maxProofLines {
			score.Minimization = weights.Minimization
		} else if analysis.PublicProofLines <= maxProofLines*2 {
			score.Minimization = weights.Minimization / 2
		}
	}
	switch published.DuplicateAnalysis.PrevalenceGroupKind {
	case "unique":
		score.Novelty = weights.Novelty
	case "exact", "near":
		if published.DuplicateAnalysis.PrevalenceRepresentative {
			score.Novelty = weights.Novelty / 2
		}
	}
	score.ResponsibleDisclosure = weights.ResponsibleDisclosure
	return score
}

func normalizeChallengeTrack(track ChallengeTrack) ChallengeTrack {
	return ChallengeTrack{
		ID:       strings.TrimSpace(track.ID),
		Name:     strings.TrimSpace(track.Name),
		RulesURL: strings.TrimSpace(track.RulesURL),
		ResponsibleDisclosure: ResponsibleDisclosurePolicy{
			Contact:                 strings.TrimSpace(track.ResponsibleDisclosure.Contact),
			PolicyURL:               strings.TrimSpace(track.ResponsibleDisclosure.PolicyURL),
			EmbargoDays:             track.ResponsibleDisclosure.EmbargoDays,
			PublicSafeArtifactsOnly: track.ResponsibleDisclosure.PublicSafeArtifactsOnly,
		},
		Scoring: ChallengeScoringPolicy{
			MinScoreboardScore: track.Scoring.MinScoreboardScore,
			Weights:            track.Scoring.Weights,
		},
	}
}

func challengeTrackStrings(track ChallengeTrack) []string {
	return []string{
		track.ID,
		track.Name,
		track.RulesURL,
		track.ResponsibleDisclosure.Contact,
		track.ResponsibleDisclosure.PolicyURL,
	}
}

func challengeMaxScore(weights ChallengeScoreWeights) int {
	return weights.AnalyzerSignal + weights.Reproducibility + weights.Minimization + weights.Novelty + weights.ResponsibleDisclosure
}

func challengeScoreTier(score, maxScore int) string {
	if maxScore <= 0 {
		return "unscored"
	}
	percent := (score * 100) / maxScore
	switch {
	case percent >= 90:
		return "gold"
	case percent >= 75:
		return "silver"
	case percent >= 50:
		return "bronze"
	default:
		return "listed"
	}
}

func countLines(content []byte) int {
	trimmed := strings.TrimRight(string(content), "\n\r")
	if trimmed == "" {
		return 0
	}
	return strings.Count(trimmed, "\n") + 1
}

func sortChallengeEntries(entries []ChallengeScoreboardEntry) {
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Score != entries[j].Score {
			return entries[i].Score > entries[j].Score
		}
		return entries[i].ID < entries[j].ID
	})
}

func challengeReportHash(report ChallengeReport) string {
	copy := report
	copy.Hash = ""
	copy.Markdown = ""
	return "sha256:" + canonical.Hash(copy)
}

func RenderChallengeMarkdown(report ChallengeReport) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Public adversarial migration challenge\n\n")
	fmt.Fprintf(&b, "Patchline scores public-safe adversarial migration submissions deterministically from certificate-bound metadata, verified artifacts, responsible-disclosure checks, and the migration analyzer.\n\n")
	fmt.Fprintf(&b, "| Metric | Count |\n| --- | ---: |\n")
	fmt.Fprintf(&b, "| Submitted challenge examples | %d |\n", report.Summary.Submitted)
	fmt.Fprintf(&b, "| Accepted challenge examples | %d |\n", report.Summary.Accepted)
	fmt.Fprintf(&b, "| Scoreboard entries | %d |\n", report.Summary.ScoreboardEntries)
	fmt.Fprintf(&b, "| Excluded below threshold | %d |\n", report.Summary.ExcludedBelowThreshold)
	fmt.Fprintf(&b, "| Rejected examples | %d |\n", report.Summary.Rejected)
	fmt.Fprintf(&b, "| Analyzer-backed proofs | %d |\n", report.Summary.AnalyzerProofs)
	fmt.Fprintf(&b, "| Public-safe disclosures | %d |\n", report.Summary.PublicSafe)
	fmt.Fprintf(&b, "\n## Scoreboard\n\n")
	if len(report.Scoreboard) == 0 {
		fmt.Fprintf(&b, "No challenge submissions reached the deterministic scoreboard threshold.\n\n")
	} else {
		fmt.Fprintf(&b, "| Rank | ID | Ecosystem | Hazard | Score | Tier | Analyzer | Artifact |\n")
		fmt.Fprintf(&b, "| ---: | --- | --- | --- | ---: | --- | --- | --- |\n")
		for i, entry := range report.Scoreboard {
			fmt.Fprintf(&b, "| %d | `%s` | %s | %s | %d/%d | %s | %s | `%s` |\n",
				i+1,
				escapePipe(entry.ID),
				escapePipe(entry.Ecosystem),
				escapePipe(entry.HazardClass),
				entry.Score,
				entry.MaxScore,
				escapePipe(entry.Tier),
				escapePipe(entry.MigrationAnalysis.ActualBehavior),
				escapePipe(entry.MigrationAnalysis.ArtifactPath),
			)
		}
		fmt.Fprintf(&b, "\n")
	}
	if len(report.Rejected) > 0 {
		fmt.Fprintf(&b, "## Rejected examples\n\n")
		fmt.Fprintf(&b, "| ID | Reasons |\n| --- | --- |\n")
		for _, rejected := range report.Rejected {
			fmt.Fprintf(&b, "| `%s` | %s |\n", escapePipe(rejected.ID), escapePipe(strings.Join(rejected.Reasons, "; ")))
		}
	}
	return b.String()
}

func RenderChallengeHTML(report ChallengeReport) (string, error) {
	const page = `<!doctype html>
<meta charset="utf-8">
<title>Patchline adversarial migration challenge</title>
<h1>Patchline adversarial migration challenge</h1>
<p>{{.Summary.ScoreboardEntries}} public-safe adversarial migration submissions reached the deterministic scoreboard threshold; rejected {{.Summary.Rejected}}.</p>
<table>
<thead><tr><th>Rank</th><th>ID</th><th>Hazard</th><th>Ecosystem</th><th>Score</th><th>Tier</th><th>Analyzer</th><th>Artifact</th></tr></thead>
<tbody>
{{range $i, $entry := .Scoreboard}}<tr><td>{{$i}}</td><td><code>{{$entry.ID}}</code></td><td>{{$entry.HazardClass}}</td><td>{{$entry.Ecosystem}}</td><td>{{$entry.Score}}/{{$entry.MaxScore}}</td><td>{{$entry.Tier}}</td><td>{{$entry.MigrationAnalysis.ActualBehavior}}</td><td><code>{{$entry.MigrationAnalysis.ArtifactPath}}</code></td></tr>
{{end}}</tbody>
</table>
`
	tmpl, err := template.New("challenge").Parse(page)
	if err != nil {
		return "", err
	}
	var b bytes.Buffer
	if err := tmpl.Execute(&b, report); err != nil {
		return "", err
	}
	return b.String(), nil
}
