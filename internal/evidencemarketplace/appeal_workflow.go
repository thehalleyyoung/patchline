package evidencemarketplace

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/thehalleyyoung/patchline/internal/canonical"
)

const (
	AppealWorkflowSpecVersion   = "patchline.evidence-appeal-workflow/v1"
	AppealWorkflowReportVersion = "patchline.evidence-appeal-workflow-report/v1"
)

type AppealWorkflowSpec struct {
	Version            string        `json:"version"`
	Claim              string        `json:"claim"`
	RegistryPath       string        `json:"registry_path"`
	BoardDecisionsPath string        `json:"board_decisions_path"`
	Board              BoardPolicy   `json:"board"`
	Appeals            []AppealInput `json:"appeals"`
}

type AppealInput struct {
	AppealID               string                     `json:"appeal_id"`
	EvidenceID             string                     `json:"evidence_id"`
	DisputedFinding        string                     `json:"disputed_finding"`
	DisputeType            string                     `json:"dispute_type"`
	SubmittedBy            string                     `json:"submitted_by"`
	SubmittedAt            string                     `json:"submitted_at"`
	Rationale              string                     `json:"rationale"`
	RequestedResolution    string                     `json:"requested_resolution"`
	EvidenceHash           string                     `json:"evidence_hash"`
	CertificateSubjectHash string                     `json:"certificate_subject_hash"`
	PreservedArtifacts     []BoardArchivePreservation `json:"preserved_artifacts"`
	ReviewerRationales     []AppealReviewerRationale  `json:"reviewer_rationales"`
	Resolution             AppealResolution           `json:"resolution"`
}

type AppealReviewerRationale struct {
	Reviewer           BoardReviewer `json:"reviewer"`
	Rationale          string        `json:"rationale"`
	EvidenceReferences []string      `json:"evidence_references"`
}

type AppealResolution struct {
	Status          string   `json:"status"`
	Rationale       string   `json:"rationale"`
	ResolvedAt      string   `json:"resolved_at"`
	Resolver        string   `json:"resolver"`
	FollowUpActions []string `json:"follow_up_actions"`
}

type AppealWorkflowReport struct {
	Version            string                `json:"version"`
	OK                 bool                  `json:"ok"`
	SpecHash           string                `json:"spec_hash"`
	RegistryPath       string                `json:"registry_path"`
	BoardDecisionsPath string                `json:"board_decisions_path"`
	RegistryHash       string                `json:"registry_hash"`
	BaseReportHash     string                `json:"base_report_hash"`
	BoardReportHash    string                `json:"board_report_hash"`
	Hash               string                `json:"hash"`
	Board              BoardPolicy           `json:"board"`
	Summary            AppealWorkflowSummary `json:"summary"`
	Appeals            []AppealReport        `json:"appeals"`
	Rejected           []RejectedExample     `json:"rejected,omitempty"`
	Markdown           string                `json:"markdown,omitempty"`
}

type AppealWorkflowSummary struct {
	SubmittedAppeals   int `json:"submitted_appeals"`
	ProcessedAppeals   int `json:"processed_appeals"`
	Upheld             int `json:"upheld"`
	Modified           int `json:"modified"`
	Overturned         int `json:"overturned"`
	Rejected           int `json:"rejected"`
	BoardBindings      int `json:"board_bindings"`
	PreservedArtifacts int `json:"preserved_artifacts"`
	ReviewerRationales int `json:"reviewer_rationales"`
	IndependentReviews int `json:"independent_reviews"`
}

type AppealReport struct {
	AppealID               string                     `json:"appeal_id"`
	EvidenceID             string                     `json:"evidence_id"`
	DisputedFinding        string                     `json:"disputed_finding"`
	DisputeType            string                     `json:"dispute_type"`
	SubmittedBy            string                     `json:"submitted_by"`
	SubmittedAt            string                     `json:"submitted_at"`
	Rationale              string                     `json:"rationale"`
	RequestedResolution    string                     `json:"requested_resolution"`
	FinalResolution        string                     `json:"final_resolution"`
	EvidenceHash           string                     `json:"evidence_hash"`
	CertificateSubjectHash string                     `json:"certificate_subject_hash"`
	BoardDecision          AppealBoardDecisionBinding `json:"board_decision"`
	PreservedArtifacts     []BoardArchivePreservation `json:"preserved_artifacts"`
	Review                 BoardReviewOutcome         `json:"review"`
	ReviewerRationales     []AppealReviewerRationale  `json:"reviewer_rationales"`
	Resolution             AppealResolution           `json:"resolution"`
}

type AppealBoardDecisionBinding struct {
	EvidenceID             string   `json:"evidence_id"`
	FinalStatus            string   `json:"final_status"`
	EvidenceHash           string   `json:"evidence_hash"`
	CertificateSubjectHash string   `json:"certificate_subject_hash"`
	BoardReportHash        string   `json:"board_report_hash"`
	OriginalApprovers      []string `json:"original_approvers"`
}

func ReadAppealWorkflowSpec(reader io.Reader) (AppealWorkflowSpec, error) {
	decoder := json.NewDecoder(reader)
	decoder.DisallowUnknownFields()
	var spec AppealWorkflowSpec
	if err := decoder.Decode(&spec); err != nil {
		return AppealWorkflowSpec{}, err
	}
	return spec, nil
}

func ReadAppealWorkflowSpecFile(path string) (AppealWorkflowSpec, error) {
	file, err := os.Open(path)
	if err != nil {
		return AppealWorkflowSpec{}, err
	}
	defer file.Close()
	return ReadAppealWorkflowSpec(file)
}

func EvaluateAppealWorkflowFile(path string) (AppealWorkflowReport, error) {
	spec, err := ReadAppealWorkflowSpecFile(path)
	if err != nil {
		return AppealWorkflowReport{}, err
	}
	root, err := filepath.Abs(filepath.Dir(path))
	if err != nil {
		return AppealWorkflowReport{}, err
	}
	registryPath, err := resolveGovernanceSpecPath(root, spec.RegistryPath, "registry_path")
	if err != nil {
		return AppealWorkflowReport{}, err
	}
	boardPath, err := resolveGovernanceSpecPath(root, spec.BoardDecisionsPath, "board_decisions_path")
	if err != nil {
		return AppealWorkflowReport{}, err
	}
	registry, err := ReadRegistryFile(registryPath)
	if err != nil {
		return AppealWorkflowReport{}, err
	}
	boardReport, err := EvaluateBoardReviewFile(boardPath)
	if err != nil {
		return AppealWorkflowReport{}, err
	}
	report, err := EvaluateAppealWorkflow(spec, registry, filepath.Dir(registryPath), boardReport)
	if err != nil {
		return AppealWorkflowReport{}, err
	}
	report.RegistryPath = filepath.ToSlash(strings.TrimSpace(spec.RegistryPath))
	report.BoardDecisionsPath = filepath.ToSlash(strings.TrimSpace(spec.BoardDecisionsPath))
	return report, nil
}

func EvaluateAppealWorkflow(spec AppealWorkflowSpec, registry Registry, registryRoot string, boardReport BoardReviewReport) (AppealWorkflowReport, error) {
	rootAbs, err := filepath.Abs(registryRoot)
	if err != nil {
		return AppealWorkflowReport{}, err
	}
	base, err := PublishRegistry(registry, rootAbs)
	if err != nil {
		return AppealWorkflowReport{}, err
	}
	report := AppealWorkflowReport{
		Version:            AppealWorkflowReportVersion,
		SpecHash:           "sha256:" + canonical.Hash(spec),
		RegistryPath:       filepath.ToSlash(strings.TrimSpace(spec.RegistryPath)),
		BoardDecisionsPath: filepath.ToSlash(strings.TrimSpace(spec.BoardDecisionsPath)),
		RegistryHash:       base.RegistryHash,
		BaseReportHash:     base.Hash,
		BoardReportHash:    strings.TrimSpace(boardReport.Hash),
		Board:              normalizeBoardPolicy(spec.Board),
		Summary: AppealWorkflowSummary{
			SubmittedAppeals: len(spec.Appeals),
		},
	}
	report.Rejected = append(report.Rejected, validateAppealWorkflowSpec(spec)...)
	for _, rejected := range base.Rejected {
		report.Rejected = append(report.Rejected, RejectedExample{ID: "registry:" + rejected.ID, Reasons: rejected.Reasons})
	}
	if !boardReport.OK {
		report.Rejected = append(report.Rejected, RejectedExample{ID: "board-decisions", Reasons: []string{"board_decisions_path must evaluate to an accepted governance board report"}})
	}
	if boardReport.RegistryHash != "" && boardReport.RegistryHash != base.RegistryHash {
		report.Rejected = append(report.Rejected, RejectedExample{ID: "board-decisions", Reasons: []string{"board_decisions_path registry_hash must match appeal registry_path"}})
	}

	publishedByID := map[string]PublishedExample{}
	for _, example := range base.Examples {
		publishedByID[example.ID] = example
	}
	preservationByID := archivePreservationByExample(base.ArchiveMirror)
	boardByID := boardDecisionByEvidence(boardReport)
	seenAppeals := map[string]bool{}
	for _, input := range spec.Appeals {
		appeal, reasons := evaluateAppeal(input, report.Board, publishedByID, preservationByID, boardByID, report.BoardReportHash)
		id := stableRejectedID(input.AppealID)
		if seenAppeals[id] {
			reasons = append(reasons, "duplicate appeal_id")
		}
		seenAppeals[id] = true
		if len(reasons) > 0 {
			sort.Strings(reasons)
			report.Rejected = append(report.Rejected, RejectedExample{ID: id, Reasons: reasons})
			continue
		}
		report.Appeals = append(report.Appeals, appeal)
		report.Summary.ProcessedAppeals++
		report.Summary.BoardBindings++
		report.Summary.PreservedArtifacts += len(appeal.PreservedArtifacts)
		report.Summary.ReviewerRationales += len(appeal.ReviewerRationales)
		report.Summary.IndependentReviews += appeal.Review.IndependentApprovals
		switch appeal.FinalResolution {
		case "upheld":
			report.Summary.Upheld++
		case "modified":
			report.Summary.Modified++
		case "overturned":
			report.Summary.Overturned++
		}
	}
	sort.Slice(report.Appeals, func(i, j int) bool {
		return report.Appeals[i].AppealID < report.Appeals[j].AppealID
	})
	sort.Slice(report.Rejected, func(i, j int) bool {
		return report.Rejected[i].ID < report.Rejected[j].ID
	})
	report.Summary.Rejected = len(report.Rejected)
	report.OK = report.Summary.SubmittedAppeals > 0 && report.Summary.ProcessedAppeals > 0 && report.Summary.Rejected == 0
	report.Hash = appealWorkflowReportHash(report)
	report.Markdown = RenderAppealWorkflowMarkdown(report)
	return report, nil
}

func WriteAppealWorkflowReport(outDir string, report AppealWorkflowReport) error {
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}
	if err := writeJSON(filepath.Join(outDir, "appeal-workflow.json"), report); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(outDir, "appeal-workflow.md"), []byte(report.Markdown), 0o644); err != nil {
		return err
	}
	html, err := RenderAppealWorkflowHTML(report)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(outDir, "index.html"), []byte(html), 0o644)
}

func validateAppealWorkflowSpec(spec AppealWorkflowSpec) []RejectedExample {
	var reasons []string
	if spec.Version != AppealWorkflowSpecVersion {
		reasons = append(reasons, fmt.Sprintf("unsupported appeal workflow version %q", spec.Version))
	}
	if len(strings.TrimSpace(spec.Claim)) < 120 {
		reasons = append(reasons, "claim must describe the appeal workflow, preserved evidence, reviewer rationale, and resolution audit trail")
	}
	if strings.TrimSpace(spec.RegistryPath) == "" {
		reasons = append(reasons, "registry_path is required")
	}
	if strings.TrimSpace(spec.BoardDecisionsPath) == "" {
		reasons = append(reasons, "board_decisions_path is required")
	}
	reasons = append(reasons, validateBoardPolicy(spec.Board)...)
	if len(spec.Appeals) == 0 {
		reasons = append(reasons, "at least one appeal is required")
	}
	reasons = append(reasons, scanPublicStrings("evidence_appeal_workflow", appealWorkflowSpecStrings(spec))...)
	if len(reasons) == 0 {
		return nil
	}
	sort.Strings(reasons)
	return []RejectedExample{{ID: "appeal-workflow-spec", Reasons: reasons}}
}

func evaluateAppeal(input AppealInput, board BoardPolicy, publishedByID map[string]PublishedExample, preservationByID map[string][]BoardArchivePreservation, boardByID map[string]BoardDecisionReport, boardReportHash string) (AppealReport, []string) {
	appeal := AppealReport{
		AppealID:               strings.TrimSpace(input.AppealID),
		EvidenceID:             strings.TrimSpace(input.EvidenceID),
		DisputedFinding:        strings.TrimSpace(input.DisputedFinding),
		DisputeType:            strings.ToLower(strings.TrimSpace(input.DisputeType)),
		SubmittedBy:            strings.TrimSpace(input.SubmittedBy),
		SubmittedAt:            strings.TrimSpace(input.SubmittedAt),
		Rationale:              strings.TrimSpace(input.Rationale),
		RequestedResolution:    strings.ToLower(strings.TrimSpace(input.RequestedResolution)),
		EvidenceHash:           strings.TrimSpace(input.EvidenceHash),
		CertificateSubjectHash: strings.TrimSpace(input.CertificateSubjectHash),
		Resolution:             normalizeAppealResolution(input.Resolution),
	}
	appeal.FinalResolution = appeal.Resolution.Status
	var reasons []string
	if appeal.AppealID == "" {
		reasons = append(reasons, "appeal_id is required")
	}
	if appeal.EvidenceID == "" {
		reasons = append(reasons, "evidence_id is required")
	}
	published, ok := publishedByID[appeal.EvidenceID]
	if !ok {
		reasons = append(reasons, "evidence_id must reference a published marketplace example")
	}
	if len(appeal.DisputedFinding) < 40 {
		reasons = append(reasons, "disputed_finding must describe the disputed finding")
	}
	if !allowedAppealDisputeType(appeal.DisputeType) {
		reasons = append(reasons, "dispute_type must be evidence-integrity, false-positive, reviewer-process, or severity")
	}
	if appeal.SubmittedBy == "" {
		reasons = append(reasons, "submitted_by is required")
	}
	submittedAt, submittedOK, submittedReasons := parseAppealTimestamp("submitted_at", appeal.SubmittedAt)
	reasons = append(reasons, submittedReasons...)
	if len(appeal.Rationale) < 60 {
		reasons = append(reasons, "rationale must explain why the finding is disputed")
	}
	if !allowedAppealResolution(appeal.RequestedResolution) {
		reasons = append(reasons, "requested_resolution must be upheld, modified, or overturned")
	}
	if ok {
		if appeal.EvidenceHash != published.EvidenceHash {
			reasons = append(reasons, "evidence_hash must match published evidence hash")
		}
		if appeal.CertificateSubjectHash != published.CertificateSubjectHash {
			reasons = append(reasons, "certificate_subject_hash must match published certificate subject hash")
		}
		appeal.EvidenceHash = published.EvidenceHash
		appeal.CertificateSubjectHash = published.CertificateSubjectHash
	}
	boardDecision, boardOK := boardByID[appeal.EvidenceID]
	if !boardOK {
		reasons = append(reasons, "board_decisions_path must include a decision for evidence_id")
	} else {
		binding, bindingReasons := appealBoardBinding(boardDecision, published, boardReportHash)
		appeal.BoardDecision = binding
		reasons = append(reasons, bindingReasons...)
	}
	preserved, preservationReasons := validateAppealPreservedArtifacts(appeal.EvidenceID, input.PreservedArtifacts, preservationByID[appeal.EvidenceID])
	appeal.PreservedArtifacts = preserved
	reasons = append(reasons, preservationReasons...)
	reviewerRationales, review, reviewReasons := evaluateAppealReviewers(board, published, input.ReviewerRationales, appeal.PreservedArtifacts, appeal.BoardDecision.OriginalApprovers)
	appeal.ReviewerRationales = reviewerRationales
	appeal.Review = review
	reasons = append(reasons, reviewReasons...)
	resolvedAt, resolvedOK, resolutionReasons := validateAppealResolution(appeal.Resolution)
	reasons = append(reasons, resolutionReasons...)
	if submittedOK && resolvedOK && resolvedAt.Before(submittedAt) {
		reasons = append(reasons, "resolution.resolved_at must not be before submitted_at")
	}
	return appeal, reasons
}

func appealBoardBinding(decision BoardDecisionReport, published PublishedExample, boardReportHash string) (AppealBoardDecisionBinding, []string) {
	binding := AppealBoardDecisionBinding{
		EvidenceID:             decision.EvidenceID,
		FinalStatus:            decision.FinalStatus,
		EvidenceHash:           decision.EvidenceHash,
		CertificateSubjectHash: decision.CertificateSubjectHash,
		BoardReportHash:        boardReportHash,
		OriginalApprovers:      append([]string(nil), decision.Review.ApproverIdentities...),
	}
	sort.Strings(binding.OriginalApprovers)
	var reasons []string
	if decision.EvidenceHash != published.EvidenceHash {
		reasons = append(reasons, "board decision evidence_hash must match published evidence hash")
	}
	if decision.CertificateSubjectHash != published.CertificateSubjectHash {
		reasons = append(reasons, "board decision certificate_subject_hash must match published certificate subject hash")
	}
	if decision.FinalStatus == "" {
		reasons = append(reasons, "board decision final_status is required")
	}
	return binding, reasons
}

func validateAppealPreservedArtifacts(evidenceID string, declared []BoardArchivePreservation, expected []BoardArchivePreservation) ([]BoardArchivePreservation, []string) {
	var reasons []string
	normalizedDeclared := normalizeAppealPreservedArtifacts(declared)
	normalizedExpected := normalizeAppealPreservedArtifacts(expected)
	if len(normalizedExpected) == 0 {
		reasons = append(reasons, "preserved_artifacts must reference marketplace archive mirror entries for "+evidenceID)
	}
	if len(normalizedDeclared) == 0 {
		reasons = append(reasons, "preserved_artifacts are required")
	}
	if len(normalizedDeclared) != len(normalizedExpected) {
		reasons = append(reasons, "preserved_artifacts must include every marketplace archive mirror entry for evidence_id")
	}
	expectedKeys := map[string]bool{}
	for _, entry := range normalizedExpected {
		expectedKeys[appealPreservationKey(entry)] = true
	}
	for _, entry := range normalizedDeclared {
		if entry.ExampleID != evidenceID {
			reasons = append(reasons, "preserved_artifacts.example_id must match evidence_id")
		}
		if !expectedKeys[appealPreservationKey(entry)] {
			reasons = append(reasons, "preserved_artifacts must match marketplace archive mirror metadata")
		}
	}
	reasons = append(reasons, validateArchivePreservation(evidenceID, normalizedDeclared)...)
	if len(reasons) > 0 {
		return normalizedDeclared, reasons
	}
	return normalizedExpected, nil
}

func evaluateAppealReviewers(board BoardPolicy, example PublishedExample, inputs []AppealReviewerRationale, preserved []BoardArchivePreservation, originalApprovers []string) ([]AppealReviewerRationale, BoardReviewOutcome, []string) {
	var reviewers []BoardReviewer
	var normalized []AppealReviewerRationale
	var reasons []string
	original := map[string]bool{}
	for _, approver := range originalApprovers {
		original[reputationIdentityKey(approver)] = true
	}
	referenceSet := appealEvidenceReferenceSet(preserved)
	for i, input := range inputs {
		entry := AppealReviewerRationale{
			Reviewer:           normalizeBoardReviewer(input.Reviewer),
			Rationale:          strings.TrimSpace(input.Rationale),
			EvidenceReferences: normalizeStringList(input.EvidenceReferences, false),
		}
		reviewers = append(reviewers, entry.Reviewer)
		if len(entry.Rationale) < 50 {
			reasons = append(reasons, fmt.Sprintf("reviewer_rationales[%d].rationale must explain the appeal reviewer judgment", i))
		}
		if original[reputationIdentityKey(entry.Reviewer.Name)] {
			reasons = append(reasons, "appeal reviewers must be independent of original board approvers")
		}
		if len(entry.EvidenceReferences) == 0 {
			reasons = append(reasons, fmt.Sprintf("reviewer_rationales[%d].evidence_references must cite preserved evidence", i))
		}
		for _, ref := range entry.EvidenceReferences {
			if !referenceSet[ref] {
				reasons = append(reasons, fmt.Sprintf("reviewer_rationales[%d].evidence_references must cite preserved artifact path, checksum, mirror_path, or withdrawal_id", i))
			}
		}
		normalized = append(normalized, entry)
	}
	review, reviewReasons := evaluateBoardReviewers(board, example, reviewers)
	reasons = append(reasons, reviewReasons...)
	return normalized, review, reasons
}

func validateAppealResolution(resolution AppealResolution) (time.Time, bool, []string) {
	var reasons []string
	if !allowedAppealResolution(resolution.Status) {
		reasons = append(reasons, "resolution.status must be upheld, modified, or overturned")
	}
	if len(strings.TrimSpace(resolution.Rationale)) < 60 {
		reasons = append(reasons, "resolution.rationale must explain the final appeal outcome")
	}
	resolvedAt, ok, timeReasons := parseAppealTimestamp("resolution.resolved_at", resolution.ResolvedAt)
	reasons = append(reasons, timeReasons...)
	if strings.TrimSpace(resolution.Resolver) == "" {
		reasons = append(reasons, "resolution.resolver is required")
	}
	actions := normalizeStringList(resolution.FollowUpActions, false)
	if len(actions) == 0 {
		reasons = append(reasons, "resolution.follow_up_actions must record the audit-trail follow-up")
	}
	return resolvedAt, ok, reasons
}

func parseAppealTimestamp(field, value string) (time.Time, bool, []string) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, false, []string{field + " is required"}
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}, false, []string{field + " must be RFC3339"}
	}
	return parsed, true, nil
}

func boardDecisionByEvidence(report BoardReviewReport) map[string]BoardDecisionReport {
	out := map[string]BoardDecisionReport{}
	for _, decision := range report.Decisions {
		out[decision.EvidenceID] = decision
	}
	return out
}

func allowedAppealDisputeType(value string) bool {
	switch value {
	case "evidence-integrity", "false-positive", "reviewer-process", "severity":
		return true
	default:
		return false
	}
}

func allowedAppealResolution(value string) bool {
	switch value {
	case "upheld", "modified", "overturned":
		return true
	default:
		return false
	}
}

func normalizeAppealResolution(resolution AppealResolution) AppealResolution {
	return AppealResolution{
		Status:          strings.ToLower(strings.TrimSpace(resolution.Status)),
		Rationale:       strings.TrimSpace(resolution.Rationale),
		ResolvedAt:      strings.TrimSpace(resolution.ResolvedAt),
		Resolver:        strings.TrimSpace(resolution.Resolver),
		FollowUpActions: normalizeStringList(resolution.FollowUpActions, false),
	}
}

func normalizeAppealPreservedArtifacts(entries []BoardArchivePreservation) []BoardArchivePreservation {
	out := make([]BoardArchivePreservation, 0, len(entries))
	for _, entry := range entries {
		out = append(out, BoardArchivePreservation{
			ExampleID:                       strings.TrimSpace(entry.ExampleID),
			ArtifactPath:                    strings.TrimSpace(entry.ArtifactPath),
			MirrorPath:                      strings.TrimSpace(entry.MirrorPath),
			Checksum:                        strings.TrimSpace(entry.Checksum),
			WithdrawalID:                    strings.TrimSpace(entry.WithdrawalID),
			TombstoneRequired:               entry.TombstoneRequired,
			PreserveChecksumAfterWithdrawal: entry.PreserveChecksumAfterWithdrawal,
			ReviewRequired:                  entry.ReviewRequired,
			ReplacementAllowed:              entry.ReplacementAllowed,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].ExampleID != out[j].ExampleID {
			return out[i].ExampleID < out[j].ExampleID
		}
		return out[i].ArtifactPath < out[j].ArtifactPath
	})
	return out
}

func appealPreservationKey(entry BoardArchivePreservation) string {
	return strings.Join([]string{
		entry.ExampleID,
		entry.ArtifactPath,
		entry.MirrorPath,
		entry.Checksum,
		entry.WithdrawalID,
		fmt.Sprintf("%t", entry.TombstoneRequired),
		fmt.Sprintf("%t", entry.PreserveChecksumAfterWithdrawal),
		fmt.Sprintf("%t", entry.ReviewRequired),
	}, "\x00")
}

func appealEvidenceReferenceSet(entries []BoardArchivePreservation) map[string]bool {
	out := map[string]bool{}
	for _, entry := range entries {
		for _, value := range []string{entry.ArtifactPath, entry.MirrorPath, entry.Checksum, entry.WithdrawalID} {
			value = strings.TrimSpace(value)
			if value != "" {
				out[value] = true
			}
		}
	}
	return out
}

func appealWorkflowSpecStrings(spec AppealWorkflowSpec) []string {
	values := []string{
		spec.Claim,
		spec.RegistryPath,
		spec.BoardDecisionsPath,
		spec.Board.ID,
		spec.Board.Name,
		spec.Board.CharterURL,
		spec.Board.ConflictPolicy,
	}
	for _, appeal := range spec.Appeals {
		values = append(values,
			appeal.AppealID,
			appeal.EvidenceID,
			appeal.DisputedFinding,
			appeal.DisputeType,
			appeal.SubmittedBy,
			appeal.SubmittedAt,
			appeal.Rationale,
			appeal.RequestedResolution,
			appeal.EvidenceHash,
			appeal.CertificateSubjectHash,
			appeal.Resolution.Status,
			appeal.Resolution.Rationale,
			appeal.Resolution.ResolvedAt,
			appeal.Resolution.Resolver,
		)
		values = append(values, appeal.Resolution.FollowUpActions...)
		for _, entry := range appeal.PreservedArtifacts {
			values = append(values, entry.ExampleID, entry.ArtifactPath, entry.MirrorPath, entry.Checksum, entry.WithdrawalID)
		}
		for _, rationale := range appeal.ReviewerRationales {
			values = append(values,
				rationale.Reviewer.Name,
				rationale.Reviewer.Role,
				rationale.Reviewer.Affiliation,
				rationale.Reviewer.Vote,
				rationale.Rationale,
			)
			values = append(values, rationale.EvidenceReferences...)
		}
	}
	return values
}

func resolveGovernanceSpecPath(root, rel, field string) (string, error) {
	rel = strings.TrimSpace(rel)
	if rel == "" {
		return "", fmt.Errorf("%s is required", field)
	}
	if filepath.IsAbs(rel) {
		return rel, nil
	}
	clean := filepath.Clean(filepath.FromSlash(rel))
	if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("%s escapes spec directory", field)
	}
	return filepath.Join(root, clean), nil
}

func appealWorkflowReportHash(report AppealWorkflowReport) string {
	copy := report
	copy.Hash = ""
	copy.Markdown = ""
	return "sha256:" + canonical.Hash(copy)
}

func RenderAppealWorkflowMarkdown(report AppealWorkflowReport) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Evidence appeal workflow\n\n")
	fmt.Fprintf(&b, "Patchline appeal workflows process disputed findings without deleting the original evidence: every appeal is bound to the marketplace evidence hash, certificate subject hash, preserved archive artifacts, independent reviewer rationales, and a final resolution audit trail.\n\n")
	fmt.Fprintf(&b, "| Metric | Count |\n| --- | ---: |\n")
	fmt.Fprintf(&b, "| Submitted appeals | %d |\n", report.Summary.SubmittedAppeals)
	fmt.Fprintf(&b, "| Processed appeals | %d |\n", report.Summary.ProcessedAppeals)
	fmt.Fprintf(&b, "| Upheld | %d |\n", report.Summary.Upheld)
	fmt.Fprintf(&b, "| Modified | %d |\n", report.Summary.Modified)
	fmt.Fprintf(&b, "| Overturned | %d |\n", report.Summary.Overturned)
	fmt.Fprintf(&b, "| Board-decision bindings | %d |\n", report.Summary.BoardBindings)
	fmt.Fprintf(&b, "| Preserved artifacts | %d |\n", report.Summary.PreservedArtifacts)
	fmt.Fprintf(&b, "| Reviewer rationales | %d |\n", report.Summary.ReviewerRationales)
	fmt.Fprintf(&b, "| Independent reviews | %d |\n", report.Summary.IndependentReviews)
	fmt.Fprintf(&b, "| Rejected appeals | %d |\n", report.Summary.Rejected)
	fmt.Fprintf(&b, "\n## Appeals\n\n")
	fmt.Fprintf(&b, "| Appeal | Evidence | Dispute | Requested | Resolution | Independent reviews | Preserved artifacts | Audit rationale |\n")
	fmt.Fprintf(&b, "| --- | --- | --- | --- | --- | ---: | ---: | --- |\n")
	for _, appeal := range report.Appeals {
		fmt.Fprintf(&b, "| `%s` | `%s` | %s | %s | %s | %d | %d | %s |\n",
			escapePipe(appeal.AppealID),
			escapePipe(appeal.EvidenceID),
			escapePipe(appeal.DisputeType),
			escapePipe(appeal.RequestedResolution),
			escapePipe(appeal.FinalResolution),
			appeal.Review.IndependentApprovals,
			len(appeal.PreservedArtifacts),
			escapePipe(appeal.Resolution.Rationale),
		)
	}
	if len(report.Appeals) > 0 {
		fmt.Fprintf(&b, "\n## Reviewer rationale trail\n\n")
		fmt.Fprintf(&b, "| Appeal | Reviewer | Vote | Cited evidence |\n| --- | --- | --- | --- |\n")
		for _, appeal := range report.Appeals {
			for _, rationale := range appeal.ReviewerRationales {
				fmt.Fprintf(&b, "| `%s` | %s | %s | `%s` |\n",
					escapePipe(appeal.AppealID),
					escapePipe(rationale.Reviewer.Name),
					escapePipe(rationale.Reviewer.Vote),
					escapePipe(strings.Join(rationale.EvidenceReferences, "`, `")),
				)
			}
		}
	}
	if len(report.Rejected) > 0 {
		fmt.Fprintf(&b, "\n## Rejected appeals\n\n")
		fmt.Fprintf(&b, "| ID | Reasons |\n| --- | --- |\n")
		for _, rejected := range report.Rejected {
			fmt.Fprintf(&b, "| `%s` | %s |\n", escapePipe(rejected.ID), escapePipe(strings.Join(rejected.Reasons, "; ")))
		}
	}
	return b.String()
}

func RenderAppealWorkflowHTML(report AppealWorkflowReport) (string, error) {
	const page = `<!doctype html>
<meta charset="utf-8">
<title>Patchline evidence appeal workflow</title>
<h1>Patchline evidence appeal workflow</h1>
<p>{{.Summary.ProcessedAppeals}} appeal(s) processed with {{.Summary.PreservedArtifacts}} preserved archive artifacts and {{.Summary.ReviewerRationales}} reviewer rationales.</p>
<table>
<thead><tr><th>Appeal</th><th>Evidence</th><th>Requested</th><th>Resolution</th><th>Independent reviews</th><th>Preserved artifacts</th></tr></thead>
<tbody>{{range .Appeals}}<tr><td><code>{{.AppealID}}</code></td><td><code>{{.EvidenceID}}</code></td><td>{{.RequestedResolution}}</td><td>{{.FinalResolution}}</td><td>{{.Review.IndependentApprovals}}</td><td>{{len .PreservedArtifacts}}</td></tr>{{end}}</tbody>
</table>
`
	tmpl, err := template.New("appeal").Parse(page)
	if err != nil {
		return "", err
	}
	var b bytes.Buffer
	if err := tmpl.Execute(&b, report); err != nil {
		return "", err
	}
	return b.String(), nil
}
