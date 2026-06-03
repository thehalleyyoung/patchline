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
	BoardReviewSpecVersion   = "patchline.evidence-governance-board/v1"
	BoardReviewReportVersion = "patchline.evidence-governance-board-report/v1"
)

type BoardReviewSpec struct {
	Version      string               `json:"version"`
	Claim        string               `json:"claim"`
	RegistryPath string               `json:"registry_path"`
	Board        BoardPolicy          `json:"board"`
	Decisions    []BoardDecisionInput `json:"decisions"`
}

type BoardPolicy struct {
	ID                      string `json:"id"`
	Name                    string `json:"name"`
	CharterURL              string `json:"charter_url"`
	ConflictPolicy          string `json:"conflict_policy"`
	Quorum                  int    `json:"quorum"`
	MinIndependentApprovers int    `json:"min_independent_approvers"`
}

type BoardDecisionInput struct {
	EvidenceID             string           `json:"evidence_id"`
	RequestedStatus        string           `json:"requested_status"`
	Rationale              string           `json:"rationale"`
	EvidenceHash           string           `json:"evidence_hash"`
	CertificateSubjectHash string           `json:"certificate_subject_hash"`
	Reviewers              []BoardReviewer  `json:"reviewers"`
	Deprecation            *DeprecationPlan `json:"deprecation,omitempty"`
	Quarantine             *QuarantinePlan  `json:"quarantine,omitempty"`
}

type BoardReviewer struct {
	Name              string `json:"name"`
	Role              string `json:"role"`
	Affiliation       string `json:"affiliation"`
	ConflictDisclosed bool   `json:"conflict_disclosed"`
	Vote              string `json:"vote"`
}

type DeprecationPlan struct {
	EffectiveDate         string `json:"effective_date"`
	ReplacementEvidenceID string `json:"replacement_evidence_id,omitempty"`
	ContinuingValidity    string `json:"continuing_validity"`
}

type QuarantinePlan struct {
	Trigger                     string `json:"trigger"`
	Reason                      string `json:"reason"`
	RevocationOrSupersessionURL string `json:"revocation_or_supersession_url"`
	PreserveTombstone           bool   `json:"preserve_tombstone"`
}

type BoardReviewReport struct {
	Version                 string                     `json:"version"`
	OK                      bool                       `json:"ok"`
	SpecHash                string                     `json:"spec_hash"`
	RegistryPath            string                     `json:"registry_path"`
	RegistryHash            string                     `json:"registry_hash"`
	BaseReportHash          string                     `json:"base_report_hash"`
	Hash                    string                     `json:"hash"`
	Board                   BoardPolicy                `json:"board"`
	Summary                 BoardReviewSummary         `json:"summary"`
	Decisions               []BoardDecisionReport      `json:"decisions"`
	Rejected                []RejectedExample          `json:"rejected,omitempty"`
	ActiveEvidenceIDs       []string                   `json:"active_evidence_ids"`
	PreservedArchiveEntries []BoardArchivePreservation `json:"preserved_archive_entries"`
	Markdown                string                     `json:"markdown,omitempty"`
}

type BoardReviewSummary struct {
	SubmittedDecisions        int `json:"submitted_decisions"`
	Accepted                  int `json:"accepted"`
	Deprecated                int `json:"deprecated"`
	Quarantined               int `json:"quarantined"`
	Rejected                  int `json:"rejected"`
	ActiveEvidence            int `json:"active_evidence"`
	IndependentApprovals      int `json:"independent_approvals"`
	PreservedArchiveArtifacts int `json:"preserved_archive_artifacts"`
	TombstonesRequired        int `json:"tombstones_required"`
}

type BoardDecisionReport struct {
	EvidenceID             string                     `json:"evidence_id"`
	RequestedStatus        string                     `json:"requested_status"`
	FinalStatus            string                     `json:"final_status"`
	Rationale              string                     `json:"rationale"`
	EvidenceHash           string                     `json:"evidence_hash"`
	CertificateSubjectHash string                     `json:"certificate_subject_hash"`
	Review                 BoardReviewOutcome         `json:"review"`
	Deprecation            *DeprecationPlan           `json:"deprecation,omitempty"`
	Quarantine             *QuarantinePlan            `json:"quarantine,omitempty"`
	ArchivePreservation    []BoardArchivePreservation `json:"archive_preservation,omitempty"`
}

type BoardReviewOutcome struct {
	QuorumSatisfied      bool     `json:"quorum_satisfied"`
	Approvals            int      `json:"approvals"`
	IndependentApprovals int      `json:"independent_approvals"`
	ConflictedApprovals  int      `json:"conflicted_approvals"`
	Rejects              int      `json:"rejects"`
	Abstentions          int      `json:"abstentions"`
	ApproverIdentities   []string `json:"approver_identities"`
	IndependentApprovers []string `json:"independent_approvers"`
}

type BoardArchivePreservation struct {
	ExampleID                       string `json:"example_id"`
	ArtifactPath                    string `json:"artifact_path"`
	MirrorPath                      string `json:"mirror_path"`
	Checksum                        string `json:"checksum"`
	WithdrawalID                    string `json:"withdrawal_id"`
	TombstoneRequired               bool   `json:"tombstone_required"`
	PreserveChecksumAfterWithdrawal bool   `json:"preserve_checksum_after_withdrawal"`
	ReviewRequired                  bool   `json:"review_required"`
	ReplacementAllowed              bool   `json:"replacement_allowed"`
}

func ReadBoardReviewSpec(reader io.Reader) (BoardReviewSpec, error) {
	decoder := json.NewDecoder(reader)
	decoder.DisallowUnknownFields()
	var spec BoardReviewSpec
	if err := decoder.Decode(&spec); err != nil {
		return BoardReviewSpec{}, err
	}
	return spec, nil
}

func ReadBoardReviewSpecFile(path string) (BoardReviewSpec, error) {
	file, err := os.Open(path)
	if err != nil {
		return BoardReviewSpec{}, err
	}
	defer file.Close()
	return ReadBoardReviewSpec(file)
}

func EvaluateBoardReviewFile(path string) (BoardReviewReport, error) {
	spec, err := ReadBoardReviewSpecFile(path)
	if err != nil {
		return BoardReviewReport{}, err
	}
	root, err := filepath.Abs(filepath.Dir(path))
	if err != nil {
		return BoardReviewReport{}, err
	}
	registryPath, err := resolveBoardSpecPath(root, spec.RegistryPath)
	if err != nil {
		return BoardReviewReport{}, err
	}
	registry, err := ReadRegistryFile(registryPath)
	if err != nil {
		return BoardReviewReport{}, err
	}
	report, err := EvaluateBoardReview(spec, registry, filepath.Dir(registryPath))
	if err != nil {
		return BoardReviewReport{}, err
	}
	report.RegistryPath = filepath.ToSlash(strings.TrimSpace(spec.RegistryPath))
	return report, nil
}

func EvaluateBoardReview(spec BoardReviewSpec, registry Registry, registryRoot string) (BoardReviewReport, error) {
	rootAbs, err := filepath.Abs(registryRoot)
	if err != nil {
		return BoardReviewReport{}, err
	}
	base, err := PublishRegistry(registry, rootAbs)
	if err != nil {
		return BoardReviewReport{}, err
	}
	report := BoardReviewReport{
		Version:        BoardReviewReportVersion,
		SpecHash:       "sha256:" + canonical.Hash(spec),
		RegistryPath:   filepath.ToSlash(strings.TrimSpace(spec.RegistryPath)),
		RegistryHash:   base.RegistryHash,
		BaseReportHash: base.Hash,
		Board:          normalizeBoardPolicy(spec.Board),
		Summary: BoardReviewSummary{
			SubmittedDecisions: len(spec.Decisions),
		},
	}
	report.Rejected = append(report.Rejected, validateBoardSpec(spec)...)
	for _, rejected := range base.Rejected {
		report.Rejected = append(report.Rejected, RejectedExample{ID: "registry:" + rejected.ID, Reasons: rejected.Reasons})
	}

	publishedByID := map[string]PublishedExample{}
	for _, example := range base.Examples {
		publishedByID[example.ID] = example
	}
	preservationByID := archivePreservationByExample(base.ArchiveMirror)
	statusByID := map[string]string{}
	seenDecisions := map[string]bool{}

	for _, input := range spec.Decisions {
		decision, reasons := evaluateBoardDecision(input, report.Board, publishedByID, preservationByID)
		id := stableRejectedID(input.EvidenceID)
		if seenDecisions[id] {
			reasons = append(reasons, "duplicate board decision for evidence_id")
		}
		seenDecisions[id] = true
		if len(reasons) > 0 {
			sort.Strings(reasons)
			report.Rejected = append(report.Rejected, RejectedExample{ID: id, Reasons: reasons})
			continue
		}
		statusByID[decision.EvidenceID] = decision.FinalStatus
		report.Decisions = append(report.Decisions, decision)
		report.Summary.IndependentApprovals += decision.Review.IndependentApprovals
		switch decision.FinalStatus {
		case "accepted":
			report.Summary.Accepted++
		case "deprecated":
			report.Summary.Deprecated++
		case "quarantined":
			report.Summary.Quarantined++
		}
		for _, preserved := range decision.ArchivePreservation {
			report.PreservedArchiveEntries = append(report.PreservedArchiveEntries, preserved)
			report.Summary.PreservedArchiveArtifacts++
			if preserved.TombstoneRequired {
				report.Summary.TombstonesRequired++
			}
		}
	}

	sort.Slice(report.Decisions, func(i, j int) bool {
		return report.Decisions[i].EvidenceID < report.Decisions[j].EvidenceID
	})
	sort.Slice(report.PreservedArchiveEntries, func(i, j int) bool {
		if report.PreservedArchiveEntries[i].ExampleID != report.PreservedArchiveEntries[j].ExampleID {
			return report.PreservedArchiveEntries[i].ExampleID < report.PreservedArchiveEntries[j].ExampleID
		}
		return report.PreservedArchiveEntries[i].ArtifactPath < report.PreservedArchiveEntries[j].ArtifactPath
	})
	for _, example := range base.Examples {
		if statusByID[example.ID] == "deprecated" || statusByID[example.ID] == "quarantined" {
			continue
		}
		if example.ReleaseAdmission.PublicReleaseEligible {
			report.ActiveEvidenceIDs = append(report.ActiveEvidenceIDs, example.ID)
		}
	}
	sort.Strings(report.ActiveEvidenceIDs)
	sort.Slice(report.Rejected, func(i, j int) bool {
		return report.Rejected[i].ID < report.Rejected[j].ID
	})
	report.Summary.ActiveEvidence = len(report.ActiveEvidenceIDs)
	report.Summary.Rejected = len(report.Rejected)
	report.OK = report.Summary.SubmittedDecisions > 0 &&
		report.Summary.Accepted > 0 &&
		report.Summary.Deprecated > 0 &&
		report.Summary.Quarantined > 0 &&
		report.Summary.Rejected == 0
	report.Hash = boardReviewReportHash(report)
	report.Markdown = RenderBoardReviewMarkdown(report)
	return report, nil
}

func WriteBoardReviewReport(outDir string, report BoardReviewReport) error {
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}
	if err := writeJSON(filepath.Join(outDir, "governance-board.json"), report); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(outDir, "governance-board.md"), []byte(report.Markdown), 0o644); err != nil {
		return err
	}
	html, err := RenderBoardReviewHTML(report)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(outDir, "index.html"), []byte(html), 0o644)
}

func validateBoardSpec(spec BoardReviewSpec) []RejectedExample {
	var reasons []string
	if spec.Version != BoardReviewSpecVersion {
		reasons = append(reasons, fmt.Sprintf("unsupported board review version %q", spec.Version))
	}
	if len(strings.TrimSpace(spec.Claim)) < 120 {
		reasons = append(reasons, "claim must describe the governance process and evidence lifecycle")
	}
	if strings.TrimSpace(spec.RegistryPath) == "" {
		reasons = append(reasons, "registry_path is required")
	}
	boardReasons := validateBoardPolicy(spec.Board)
	reasons = append(reasons, boardReasons...)
	if len(spec.Decisions) == 0 {
		reasons = append(reasons, "at least one board decision is required")
	}
	reasons = append(reasons, scanPublicStrings("evidence_governance_board", boardSpecStrings(spec))...)
	if len(reasons) == 0 {
		return nil
	}
	sort.Strings(reasons)
	return []RejectedExample{{ID: "board-spec", Reasons: reasons}}
}

func validateBoardPolicy(board BoardPolicy) []string {
	normalized := normalizeBoardPolicy(board)
	var reasons []string
	for field, value := range map[string]string{
		"board.id":              normalized.ID,
		"board.name":            normalized.Name,
		"board.charter_url":     normalized.CharterURL,
		"board.conflict_policy": normalized.ConflictPolicy,
	} {
		if value == "" {
			reasons = append(reasons, field+" is required")
		}
	}
	if normalized.Quorum <= 0 {
		reasons = append(reasons, "board.quorum must be positive")
	}
	if normalized.MinIndependentApprovers <= 0 {
		reasons = append(reasons, "board.min_independent_approvers must be positive")
	}
	if normalized.Quorum > 0 && normalized.MinIndependentApprovers > normalized.Quorum {
		reasons = append(reasons, "board.min_independent_approvers must not exceed quorum")
	}
	return reasons
}

func evaluateBoardDecision(input BoardDecisionInput, board BoardPolicy, publishedByID map[string]PublishedExample, preservationByID map[string][]BoardArchivePreservation) (BoardDecisionReport, []string) {
	id := strings.TrimSpace(input.EvidenceID)
	status := strings.ToLower(strings.TrimSpace(input.RequestedStatus))
	decision := BoardDecisionReport{
		EvidenceID:             id,
		RequestedStatus:        status,
		Rationale:              strings.TrimSpace(input.Rationale),
		EvidenceHash:           strings.TrimSpace(input.EvidenceHash),
		CertificateSubjectHash: strings.TrimSpace(input.CertificateSubjectHash),
		Deprecation:            normalizeDeprecationPlan(input.Deprecation),
		Quarantine:             normalizeQuarantinePlan(input.Quarantine),
	}
	var reasons []string
	if id == "" {
		reasons = append(reasons, "evidence_id is required")
	}
	published, ok := publishedByID[id]
	if !ok {
		reasons = append(reasons, "evidence_id must reference a published marketplace example")
	}
	if status != "accept" && status != "deprecate" && status != "quarantine" {
		reasons = append(reasons, "requested_status must be accept, deprecate, or quarantine")
	}
	if len(decision.Rationale) < 40 {
		reasons = append(reasons, "rationale must explain the board decision")
	}
	if ok {
		if decision.EvidenceHash != published.EvidenceHash {
			reasons = append(reasons, "evidence_hash must match published evidence hash")
		}
		if decision.CertificateSubjectHash != published.CertificateSubjectHash {
			reasons = append(reasons, "certificate_subject_hash must match published certificate subject hash")
		}
		if !published.ReleaseAdmission.PublicReleaseEligible {
			reasons = append(reasons, "published evidence must remain public-release eligible before board transition")
		}
		decision.EvidenceHash = published.EvidenceHash
		decision.CertificateSubjectHash = published.CertificateSubjectHash
	}
	if ok {
		review, reviewReasons := evaluateBoardReviewers(board, published, input.Reviewers)
		decision.Review = review
		reasons = append(reasons, reviewReasons...)
	}
	switch status {
	case "accept":
		decision.FinalStatus = "accepted"
	case "deprecate":
		decision.FinalStatus = "deprecated"
		reasons = append(reasons, validateDeprecationPlan(decision.Deprecation, publishedByID)...)
		decision.ArchivePreservation = append(decision.ArchivePreservation, preservationByID[id]...)
		reasons = append(reasons, validateArchivePreservation(id, decision.ArchivePreservation)...)
	case "quarantine":
		decision.FinalStatus = "quarantined"
		reasons = append(reasons, validateQuarantinePlan(decision.Quarantine)...)
		decision.ArchivePreservation = append(decision.ArchivePreservation, preservationByID[id]...)
		reasons = append(reasons, validateArchivePreservation(id, decision.ArchivePreservation)...)
	}
	return decision, reasons
}

func evaluateBoardReviewers(board BoardPolicy, example PublishedExample, reviewers []BoardReviewer) (BoardReviewOutcome, []string) {
	var outcome BoardReviewOutcome
	var reasons []string
	if len(reviewers) < board.Quorum {
		reasons = append(reasons, "reviewers must satisfy board quorum")
	}
	outcome.QuorumSatisfied = len(reviewers) >= board.Quorum && board.Quorum > 0
	seen := map[string]bool{}
	submitterKey := reputationIdentityKey(example.Organization)
	for i, raw := range reviewers {
		reviewer := normalizeBoardReviewer(raw)
		for field, value := range map[string]string{
			"name":        reviewer.Name,
			"role":        reviewer.Role,
			"affiliation": reviewer.Affiliation,
			"vote":        reviewer.Vote,
		} {
			if value == "" {
				reasons = append(reasons, fmt.Sprintf("reviewers[%d].%s is required", i, field))
			}
		}
		identity := reputationIdentityKey(reviewer.Name)
		if identity != "" {
			if seen[identity] {
				reasons = append(reasons, "duplicate board reviewer "+reviewer.Name)
			}
			seen[identity] = true
		}
		if reviewer.Vote != "approve" && reviewer.Vote != "reject" && reviewer.Vote != "abstain" {
			reasons = append(reasons, fmt.Sprintf("reviewers[%d].vote must be approve, reject, or abstain", i))
			continue
		}
		switch reviewer.Vote {
		case "approve":
			outcome.Approvals++
			outcome.ApproverIdentities = append(outcome.ApproverIdentities, reviewer.Name)
			conflicted := reviewer.ConflictDisclosed || (submitterKey != "" && reputationIdentityKey(reviewer.Affiliation) == submitterKey)
			if conflicted {
				outcome.ConflictedApprovals++
				reasons = append(reasons, "conflicted reviewers may not approve evidence from their own affiliation")
			} else {
				outcome.IndependentApprovals++
				outcome.IndependentApprovers = append(outcome.IndependentApprovers, reviewer.Name)
			}
		case "reject":
			outcome.Rejects++
		case "abstain":
			outcome.Abstentions++
		}
	}
	sort.Strings(outcome.ApproverIdentities)
	sort.Strings(outcome.IndependentApprovers)
	if outcome.IndependentApprovals < board.MinIndependentApprovers {
		reasons = append(reasons, "independent approvals must meet board.min_independent_approvers")
	}
	return outcome, reasons
}

func validateDeprecationPlan(plan *DeprecationPlan, publishedByID map[string]PublishedExample) []string {
	if plan == nil {
		return []string{"deprecation plan is required for deprecate decisions"}
	}
	var reasons []string
	if strings.TrimSpace(plan.EffectiveDate) == "" {
		reasons = append(reasons, "deprecation.effective_date is required")
	} else if _, err := time.Parse("2006-01-02", strings.TrimSpace(plan.EffectiveDate)); err != nil {
		reasons = append(reasons, "deprecation.effective_date must be YYYY-MM-DD")
	}
	if strings.TrimSpace(plan.ReplacementEvidenceID) == "" && len(strings.TrimSpace(plan.ContinuingValidity)) < 30 {
		reasons = append(reasons, "deprecation must name a replacement evidence_id or explain continuing validity")
	}
	if replacement := strings.TrimSpace(plan.ReplacementEvidenceID); replacement != "" {
		if _, ok := publishedByID[replacement]; !ok {
			reasons = append(reasons, "deprecation.replacement_evidence_id must reference published evidence")
		}
	}
	return reasons
}

func validateQuarantinePlan(plan *QuarantinePlan) []string {
	if plan == nil {
		return []string{"quarantine plan is required for quarantine decisions"}
	}
	var reasons []string
	if strings.TrimSpace(plan.Trigger) == "" {
		reasons = append(reasons, "quarantine.trigger is required")
	}
	if len(strings.TrimSpace(plan.Reason)) < 40 {
		reasons = append(reasons, "quarantine.reason must explain the evidence risk")
	}
	if strings.TrimSpace(plan.RevocationOrSupersessionURL) == "" {
		reasons = append(reasons, "quarantine.revocation_or_supersession_url is required")
	}
	if !plan.PreserveTombstone {
		reasons = append(reasons, "quarantine.preserve_tombstone must be true")
	}
	return reasons
}

func validateArchivePreservation(exampleID string, entries []BoardArchivePreservation) []string {
	if len(entries) == 0 {
		return []string{"archive preservation must reference marketplace mirror entries for " + exampleID}
	}
	var reasons []string
	for _, entry := range entries {
		if !strings.HasPrefix(entry.Checksum, "sha256:") {
			reasons = append(reasons, "archive preservation checksum must be sha256-prefixed")
		}
		if !strings.HasPrefix(entry.MirrorPath, "archive/sha256/") {
			reasons = append(reasons, "archive preservation mirror_path must use archive/sha256/")
		}
		if !strings.HasPrefix(entry.WithdrawalID, "sha256:") {
			reasons = append(reasons, "archive preservation withdrawal_id must be sha256-prefixed")
		}
		if !entry.TombstoneRequired {
			reasons = append(reasons, "archive preservation must require tombstones")
		}
		if !entry.PreserveChecksumAfterWithdrawal {
			reasons = append(reasons, "archive preservation must preserve checksum after withdrawal")
		}
		if !entry.ReviewRequired {
			reasons = append(reasons, "archive preservation must require review")
		}
	}
	return reasons
}

func archivePreservationByExample(mirror ArchiveMirror) map[string][]BoardArchivePreservation {
	out := map[string][]BoardArchivePreservation{}
	for _, entry := range mirror.Entries {
		preserved := BoardArchivePreservation{
			ExampleID:                       entry.ExampleID,
			ArtifactPath:                    entry.ArtifactPath,
			MirrorPath:                      entry.MirrorPath,
			Checksum:                        entry.Checksum,
			WithdrawalID:                    entry.Withdrawal.WithdrawalID,
			TombstoneRequired:               entry.Withdrawal.TombstoneRequired,
			PreserveChecksumAfterWithdrawal: entry.Withdrawal.PreserveChecksumAfterWithdrawal,
			ReviewRequired:                  entry.Withdrawal.ReviewRequired,
			ReplacementAllowed:              entry.Withdrawal.ReplacementAllowed,
		}
		out[entry.ExampleID] = append(out[entry.ExampleID], preserved)
	}
	for id := range out {
		sort.Slice(out[id], func(i, j int) bool {
			return out[id][i].ArtifactPath < out[id][j].ArtifactPath
		})
	}
	return out
}

func normalizeBoardPolicy(board BoardPolicy) BoardPolicy {
	return BoardPolicy{
		ID:                      strings.TrimSpace(board.ID),
		Name:                    strings.TrimSpace(board.Name),
		CharterURL:              strings.TrimSpace(board.CharterURL),
		ConflictPolicy:          strings.TrimSpace(board.ConflictPolicy),
		Quorum:                  board.Quorum,
		MinIndependentApprovers: board.MinIndependentApprovers,
	}
}

func normalizeBoardReviewer(reviewer BoardReviewer) BoardReviewer {
	return BoardReviewer{
		Name:              strings.TrimSpace(reviewer.Name),
		Role:              strings.TrimSpace(reviewer.Role),
		Affiliation:       strings.TrimSpace(reviewer.Affiliation),
		ConflictDisclosed: reviewer.ConflictDisclosed,
		Vote:              strings.ToLower(strings.TrimSpace(reviewer.Vote)),
	}
}

func normalizeDeprecationPlan(plan *DeprecationPlan) *DeprecationPlan {
	if plan == nil {
		return nil
	}
	return &DeprecationPlan{
		EffectiveDate:         strings.TrimSpace(plan.EffectiveDate),
		ReplacementEvidenceID: strings.TrimSpace(plan.ReplacementEvidenceID),
		ContinuingValidity:    strings.TrimSpace(plan.ContinuingValidity),
	}
}

func normalizeQuarantinePlan(plan *QuarantinePlan) *QuarantinePlan {
	if plan == nil {
		return nil
	}
	return &QuarantinePlan{
		Trigger:                     strings.TrimSpace(plan.Trigger),
		Reason:                      strings.TrimSpace(plan.Reason),
		RevocationOrSupersessionURL: strings.TrimSpace(plan.RevocationOrSupersessionURL),
		PreserveTombstone:           plan.PreserveTombstone,
	}
}

func boardSpecStrings(spec BoardReviewSpec) []string {
	values := []string{
		spec.Claim,
		spec.RegistryPath,
		spec.Board.ID,
		spec.Board.Name,
		spec.Board.CharterURL,
		spec.Board.ConflictPolicy,
	}
	for _, decision := range spec.Decisions {
		values = append(values,
			decision.EvidenceID,
			decision.RequestedStatus,
			decision.Rationale,
			decision.EvidenceHash,
			decision.CertificateSubjectHash,
		)
		if decision.Deprecation != nil {
			values = append(values, decision.Deprecation.EffectiveDate, decision.Deprecation.ReplacementEvidenceID, decision.Deprecation.ContinuingValidity)
		}
		if decision.Quarantine != nil {
			values = append(values, decision.Quarantine.Trigger, decision.Quarantine.Reason, decision.Quarantine.RevocationOrSupersessionURL)
		}
		for _, reviewer := range decision.Reviewers {
			values = append(values, reviewer.Name, reviewer.Role, reviewer.Affiliation, reviewer.Vote)
		}
	}
	return values
}

func resolveBoardSpecPath(root, rel string) (string, error) {
	rel = strings.TrimSpace(rel)
	if rel == "" {
		return "", fmt.Errorf("registry_path is required")
	}
	if filepath.IsAbs(rel) {
		return rel, nil
	}
	clean := filepath.Clean(filepath.FromSlash(rel))
	if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("registry_path escapes board spec directory")
	}
	return filepath.Join(root, clean), nil
}

func boardReviewReportHash(report BoardReviewReport) string {
	copy := report
	copy.Hash = ""
	copy.Markdown = ""
	return "sha256:" + canonical.Hash(copy)
}

func RenderBoardReviewMarkdown(report BoardReviewReport) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Evidence governance board\n\n")
	fmt.Fprintf(&b, "Patchline's shared evidence governance board accepts, deprecates, or quarantines marketplace evidence with quorum, independent approvals, conflict checks, and archive-preserving tombstones.\n\n")
	fmt.Fprintf(&b, "| Metric | Count |\n| --- | ---: |\n")
	fmt.Fprintf(&b, "| Submitted decisions | %d |\n", report.Summary.SubmittedDecisions)
	fmt.Fprintf(&b, "| Accepted evidence | %d |\n", report.Summary.Accepted)
	fmt.Fprintf(&b, "| Deprecated evidence | %d |\n", report.Summary.Deprecated)
	fmt.Fprintf(&b, "| Quarantined evidence | %d |\n", report.Summary.Quarantined)
	fmt.Fprintf(&b, "| Active evidence after board review | %d |\n", report.Summary.ActiveEvidence)
	fmt.Fprintf(&b, "| Independent approvals | %d |\n", report.Summary.IndependentApprovals)
	fmt.Fprintf(&b, "| Preserved archive artifacts | %d |\n", report.Summary.PreservedArchiveArtifacts)
	fmt.Fprintf(&b, "| Tombstones required | %d |\n", report.Summary.TombstonesRequired)
	fmt.Fprintf(&b, "| Rejected decisions | %d |\n", report.Summary.Rejected)
	fmt.Fprintf(&b, "\n## Decisions\n\n")
	fmt.Fprintf(&b, "| Evidence | Status | Independent approvals | Preserved artifacts | Rationale |\n")
	fmt.Fprintf(&b, "| --- | --- | ---: | ---: | --- |\n")
	for _, decision := range report.Decisions {
		fmt.Fprintf(&b, "| `%s` | %s | %d | %d | %s |\n",
			escapePipe(decision.EvidenceID),
			escapePipe(decision.FinalStatus),
			decision.Review.IndependentApprovals,
			len(decision.ArchivePreservation),
			escapePipe(decision.Rationale),
		)
	}
	if len(report.ActiveEvidenceIDs) > 0 {
		fmt.Fprintf(&b, "\n## Active evidence IDs\n\n")
		for _, id := range report.ActiveEvidenceIDs {
			fmt.Fprintf(&b, "- `%s`\n", escapePipe(id))
		}
	}
	if len(report.PreservedArchiveEntries) > 0 {
		fmt.Fprintf(&b, "\n## Preserved archive entries\n\n")
		fmt.Fprintf(&b, "| Evidence | Artifact | Checksum | Withdrawal |\n")
		fmt.Fprintf(&b, "| --- | --- | --- | --- |\n")
		for _, entry := range report.PreservedArchiveEntries {
			fmt.Fprintf(&b, "| `%s` | `%s` | `%s` | `%s` |\n",
				escapePipe(entry.ExampleID),
				escapePipe(entry.ArtifactPath),
				escapePipe(entry.Checksum),
				escapePipe(entry.WithdrawalID),
			)
		}
	}
	if len(report.Rejected) > 0 {
		fmt.Fprintf(&b, "\n## Rejected decisions\n\n")
		fmt.Fprintf(&b, "| ID | Reasons |\n| --- | --- |\n")
		for _, rejected := range report.Rejected {
			fmt.Fprintf(&b, "| `%s` | %s |\n", escapePipe(rejected.ID), escapePipe(strings.Join(rejected.Reasons, "; ")))
		}
	}
	return b.String()
}

func RenderBoardReviewHTML(report BoardReviewReport) (string, error) {
	const page = `<!doctype html>
<meta charset="utf-8">
<title>Patchline evidence governance board</title>
<h1>Patchline evidence governance board</h1>
<p>{{.Summary.Accepted}} accepted, {{.Summary.Deprecated}} deprecated, and {{.Summary.Quarantined}} quarantined evidence records; {{.Summary.PreservedArchiveArtifacts}} archive artifacts preserved.</p>
<table>
<thead><tr><th>Evidence</th><th>Status</th><th>Independent approvals</th><th>Preserved artifacts</th></tr></thead>
<tbody>{{range .Decisions}}<tr><td><code>{{.EvidenceID}}</code></td><td>{{.FinalStatus}}</td><td>{{.Review.IndependentApprovals}}</td><td>{{len .ArchivePreservation}}</td></tr>{{end}}</tbody>
</table>
`
	tmpl, err := template.New("board").Parse(page)
	if err != nil {
		return "", err
	}
	var b bytes.Buffer
	if err := tmpl.Execute(&b, report); err != nil {
		return "", err
	}
	return b.String(), nil
}
