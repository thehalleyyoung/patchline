package project

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/thehalleyyoung/patchline/internal/canonical"
)

const ProposalVersion = "patchline.repo-proposal/v1"

type ProposalOptions struct {
	BaselinePath  string
	Kind          string
	OutDir        string
	LLMCommand    string
	NoLLM         bool
	PromptNoFacts bool
	Budget        string
	BudgetRisks   int
}

type ProposalReport struct {
	Version        string               `json:"version"`
	BaselineHash   string               `json:"baseline_hash"`
	Kind           string               `json:"kind"`
	Generator      string               `json:"generator"`
	Deterministic  bool                 `json:"deterministic_only"`
	PromptMode     string               `json:"prompt_mode"`
	Trust          string               `json:"trust"`
	BudgetRisks    int                  `json:"budget_risks"`
	ScopeBudget    ProposalBudget       `json:"scope_budget,omitempty"`
	TargetRiskIDs  []string             `json:"target_risk_ids"`
	ContextHash    string               `json:"context_hash"`
	PromptHash     string               `json:"prompt_hash"`
	OutputHash     string               `json:"output_hash"`
	Intervention   RepairIntervention   `json:"intervention"`
	Quarantine     GeneratedQuarantine  `json:"quarantine"`
	Minimization   ProposalMinimization `json:"minimization,omitempty"`
	ContextMin     PromptContextMin     `json:"prompt_context_minimization,omitempty"`
	GeneratedFiles []GeneratedFile      `json:"generated_files,omitempty"`
	OwnerRoutes    []OwnerRoute         `json:"owner_routes,omitempty"`
	Constraints    []string             `json:"constraints,omitempty"`
	Warnings       []string             `json:"warnings,omitempty"`
	Artifacts      map[string]string    `json:"artifacts,omitempty"`
	Markdown       string               `json:"markdown,omitempty"`
	Context        ProposalContext      `json:"-"`
	Prompt         string               `json:"-"`
	Generated      []GeneratedArtifact  `json:"-"`
	Patch          string               `json:"-"`
}

type ProposalContext struct {
	Version       string                `json:"version"`
	BaselineHash  string                `json:"baseline_hash"`
	Kind          string                `json:"kind"`
	InventoryRoot string                `json:"inventory_root,omitempty"`
	Constraints   []string              `json:"constraints"`
	Risks         []ProposalRiskContext `json:"risks"`
	NativeChecks  []Command             `json:"native_checks,omitempty"`
	Minimization  PromptContextMin      `json:"minimization,omitempty"`
}

type ProposalRiskContext struct {
	ID            string        `json:"id"`
	Path          string        `json:"path"`
	Statement     int           `json:"statement,omitempty"`
	Kind          string        `json:"kind"`
	Table         string        `json:"table,omitempty"`
	Severity      string        `json:"severity"`
	Score         int           `json:"score"`
	Rationale     string        `json:"rationale"`
	Factors       []ScoreFactor `json:"factors,omitempty"`
	FactHashes    []string      `json:"fact_hashes,omitempty"`
	EvidencePaths []string      `json:"evidence_paths,omitempty"`
	Excerpt       string        `json:"excerpt,omitempty"`
	Reviewers     []string      `json:"reviewers,omitempty"`
}

type GeneratedFile struct {
	Path        string   `json:"path"`
	Kind        string   `json:"kind"`
	ContentHash string   `json:"content_hash"`
	RiskIDs     []string `json:"risk_ids,omitempty"`
	Reviewers   []string `json:"reviewers,omitempty"`
}

type GeneratedQuarantine struct {
	Status                       string   `json:"status"`
	Trust                        string   `json:"trust"`
	GeneratedArtifactsExecutable bool     `json:"generated_artifacts_executable"`
	GeneratedArtifactsApplied    bool     `json:"generated_artifacts_applied"`
	NativeChecksRequireOptIn     bool     `json:"native_checks_require_opt_in"`
	SafeNativeChecksEnabled      bool     `json:"safe_native_checks_enabled"`
	NativeExecutionMode          string   `json:"native_execution_mode"`
	RequiredFlag                 string   `json:"required_flag"`
	WriteMode                    string   `json:"write_mode"`
	Rules                        []string `json:"rules"`
	QuarantinedPaths             []string `json:"quarantined_paths,omitempty"`
}

type GeneratedArtifact struct {
	Path    string
	Kind    string
	Content string
	RiskIDs []string
}

func buildGeneratedQuarantine(artifacts []GeneratedArtifact, safeNativeChecksEnabled bool) GeneratedQuarantine {
	paths := make([]string, 0, len(artifacts))
	for _, artifact := range artifacts {
		if strings.TrimSpace(artifact.Path) != "" {
			paths = append(paths, artifact.Path)
		}
	}
	sort.Strings(paths)
	mode := "skipped-by-default"
	if safeNativeChecksEnabled {
		mode = "safe-native-checks-enabled"
	}
	return GeneratedQuarantine{
		Status:                       "enforced",
		Trust:                        "untrusted-generated-proposal",
		GeneratedArtifactsExecutable: false,
		GeneratedArtifactsApplied:    false,
		NativeChecksRequireOptIn:     true,
		SafeNativeChecksEnabled:      safeNativeChecksEnabled,
		NativeExecutionMode:          mode,
		RequiredFlag:                 "--run-native-tests",
		WriteMode:                    "0644",
		Rules: []string{
			"generated artifacts are written as non-executable proposal files",
			"generated artifacts are not applied to the scanned repository by propose or compare",
			"project-native commands are skipped unless --run-native-tests is explicitly passed",
			"native commands must match the safe allowlist and run without a shell in an offline sandbox",
		},
		QuarantinedPaths: paths,
	}
}

type ProposalMinimization struct {
	Applied                    bool                       `json:"applied,omitempty"`
	BeforeFiles                int                        `json:"before_files,omitempty"`
	AfterFiles                 int                        `json:"after_files,omitempty"`
	RemovedFiles               int                        `json:"removed_files,omitempty"`
	PreservedRisksWithCoverage int                        `json:"preserved_risks_with_coverage,omitempty"`
	PreservedCheckFailures     int                        `json:"preserved_check_failures,omitempty"`
	Removed                    []RemovedGeneratedArtifact `json:"removed,omitempty"`
}

type PromptContextMin struct {
	Applied                  bool   `json:"applied,omitempty"`
	SelectedRisks            int    `json:"selected_risks"`
	ExcludedRisks            int    `json:"excluded_risks"`
	IncludedEvidenceLinks    int    `json:"included_evidence_links"`
	ExcludedEvidenceLinks    int    `json:"excluded_evidence_links"`
	IncludedProvenanceSlices int    `json:"included_provenance_slices"`
	ExcludedProvenanceSlices int    `json:"excluded_provenance_slices"`
	IncludedNativeChecks     int    `json:"included_native_checks"`
	ExcludedNativeChecks     int    `json:"excluded_native_checks"`
	IncludedExcerptLines     int    `json:"included_excerpt_lines"`
	ExcludedExcerptLines     int    `json:"excluded_excerpt_lines"`
	IncludedEvidencePaths    int    `json:"included_evidence_paths"`
	ExcludedEvidencePaths    int    `json:"excluded_evidence_paths"`
	Reason                   string `json:"reason,omitempty"`
}

type RemovedGeneratedArtifact struct {
	Path    string   `json:"path"`
	Kind    string   `json:"kind"`
	RiskIDs []string `json:"risk_ids,omitempty"`
	Reason  string   `json:"reason"`
}

type testPlacement struct {
	Path     string
	Language string
}

type ProposalBudget struct {
	Raw        string `json:"raw,omitempty"`
	MaxFiles   int    `json:"files,omitempty"`
	MaxLines   int    `json:"lines,omitempty"`
	MaxTokens  int    `json:"tokens,omitempty"`
	MaxChanges int    `json:"changes,omitempty"`
}

type RepairIntervention struct {
	ID                 string   `json:"id"`
	Stage              string   `json:"stage"`
	BaselineHash       string   `json:"baseline_hash"`
	OutputHash         string   `json:"output_hash"`
	TargetRiskIDs      []string `json:"target_risk_ids"`
	ArtifactKinds      []string `json:"artifact_kinds"`
	Hypothesis         string   `json:"hypothesis"`
	RequiredReanalysis []string `json:"required_reanalysis"`
	Trust              string   `json:"trust"`
}

func Propose(opts ProposalOptions) (ProposalReport, error) {
	if opts.Kind == "" {
		opts.Kind = "all"
	}
	if opts.BudgetRisks <= 0 {
		opts.BudgetRisks = 3
	}
	if opts.OutDir == "" {
		opts.OutDir = filepath.Join("results", "generated", "repo-proposal")
	}
	if opts.NoLLM && opts.LLMCommand != "" {
		return ProposalReport{}, fmt.Errorf("--no-llm cannot be combined with --llm-command")
	}
	budget, err := parseProposalBudget(opts.Budget)
	if err != nil {
		return ProposalReport{}, err
	}
	if budget.MaxChanges > 0 && budget.MaxChanges < opts.BudgetRisks {
		opts.BudgetRisks = budget.MaxChanges
	}
	if err := os.MkdirAll(opts.OutDir, 0o755); err != nil {
		return ProposalReport{}, err
	}
	baseline, err := LoadBaseline(opts.BaselinePath)
	if err != nil {
		return ProposalReport{}, err
	}
	context, contextMin := buildProposalContext(baseline, opts.Kind, opts.BudgetRisks)
	promptMode := "fact-grounded"
	prompt := renderProposalPrompt(context)
	if opts.PromptNoFacts {
		promptMode = "without-facts"
		prompt = renderProposalPromptWithoutFacts(context)
	}
	generator := "patchline-template"
	var generated []GeneratedArtifact
	if opts.LLMCommand != "" {
		output, err := runLLMCommand(opts.LLMCommand, prompt)
		if err != nil {
			return ProposalReport{}, err
		}
		generator = "llm-command"
		generated = []GeneratedArtifact{{
			Path:    "patchline-proposals/llm-output.md",
			Kind:    "llm-output",
			Content: output,
			RiskIDs: riskIDs(context.Risks),
		}}
	} else {
		generated = generateTemplateArtifacts(context)
	}
	generated, budgetWarnings := applyProposalBudget(generated, budget)
	patch := renderProposalPatch(generated)
	report := ProposalReport{
		Version:       ProposalVersion,
		BaselineHash:  baseline.Hash,
		Kind:          opts.Kind,
		Generator:     generator,
		Deterministic: opts.LLMCommand == "",
		PromptMode:    promptMode,
		Trust:         "untrusted-generated-proposal",
		BudgetRisks:   opts.BudgetRisks,
		ScopeBudget:   budget,
		TargetRiskIDs: riskIDs(context.Risks),
		ContextHash:   canonical.Hash(context),
		PromptHash:    canonical.Hash(prompt),
		OutputHash:    canonical.Hash(patch),
		Constraints:   context.Constraints,
		ContextMin:    contextMin,
		Artifacts: map[string]string{
			"prompt_context": "prompt-context.json",
			"prompt":         "prompt.txt",
			"patch":          "proposal.patch",
		},
		Context:   context,
		Prompt:    prompt,
		Generated: generated,
		Patch:     patch,
	}
	report.Warnings = append(report.Warnings, budgetWarnings...)
	report.Intervention = buildRepairIntervention(report.BaselineHash, report.OutputHash, report.TargetRiskIDs, generated)
	for _, artifact := range generated {
		report.GeneratedFiles = append(report.GeneratedFiles, GeneratedFile{
			Path:        artifact.Path,
			Kind:        artifact.Kind,
			ContentHash: "sha256:" + canonical.Hash(artifact.Content),
			RiskIDs:     artifact.RiskIDs,
			Reviewers:   ownersForRiskIDs(baseline.OwnerRoutes, artifact.RiskIDs),
		})
	}
	sort.Slice(report.GeneratedFiles, func(i, j int) bool { return report.GeneratedFiles[i].Path < report.GeneratedFiles[j].Path })
	report.OwnerRoutes = ownerRoutesForGeneratedFiles(baseline, report.GeneratedFiles)
	report.Quarantine = buildGeneratedQuarantine(report.Generated, false)
	report.Markdown = renderProposalMarkdown(report)
	return report, nil
}

func WriteProposal(outDir string, report ProposalReport) error {
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}
	contextData, err := json.MarshalIndent(report.Context, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(outDir, "prompt-context.json"), append(contextData, '\n'), 0o644); err != nil {
		return err
	}
	prompt := report.Prompt
	if prompt == "" {
		prompt = renderProposalPrompt(report.Context)
	}
	if err := os.WriteFile(filepath.Join(outDir, "prompt.txt"), []byte(prompt), 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(outDir, "proposal.patch"), []byte(report.Patch), 0o644); err != nil {
		return err
	}
	for _, artifact := range report.Generated {
		path := filepath.Join(outDir, filepath.FromSlash(artifact.Path))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(path, []byte(artifact.Content), 0o644); err != nil {
			return err
		}
		if err := os.Chmod(path, 0o644); err != nil {
			return err
		}
	}
	copy := report
	copy.Context = ProposalContext{}
	copy.Generated = nil
	copy.Patch = ""
	copy.Markdown = ""
	data, err := json.MarshalIndent(copy, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(outDir, "proposal.json"), append(data, '\n'), 0o644); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(outDir, "proposal.md"), []byte(report.Markdown), 0o644)
}

func LoadBaseline(path string) (BaselineReport, error) {
	reportPath := path
	if info, err := os.Stat(path); err == nil && info.IsDir() {
		reportPath = filepath.Join(path, "baseline.json")
	}
	data, err := os.ReadFile(reportPath)
	if err != nil {
		return BaselineReport{}, err
	}
	var report BaselineReport
	if err := json.Unmarshal(data, &report); err != nil {
		return BaselineReport{}, err
	}
	return report, nil
}

func buildProposalContext(baseline BaselineReport, kind string, budget int) (ProposalContext, PromptContextMin) {
	context := ProposalContext{
		Version:       ProposalVersion,
		BaselineHash:  baseline.Hash,
		Kind:          kind,
		InventoryRoot: baseline.InventoryRoot,
		Constraints: []string{
			"Generated output is untrusted until re-analyzed.",
			"Do not assume production data access.",
			"Keep changes bounded to the listed risk IDs.",
			"Prefer fail-closed guards and explicit rollback/validation notes.",
			"Prompt context is minimized to selected risks and linked evidence only.",
		},
	}
	limit := minInt(len(baseline.Risks), budget)
	minimization := PromptContextMin{
		Applied:       true,
		SelectedRisks: limit,
		ExcludedRisks: maxInt(0, len(baseline.Risks)-limit),
		Reason:        "Only the highest-ranked budgeted risks and their linked evidence/provenance are included in prompt context.",
	}
	selectedRiskIDs := map[string]bool{}
	for _, risk := range baseline.Risks[:limit] {
		selectedRiskIDs[risk.ID] = true
		factHashes, evidencePaths, includedEvidence, includedProvenance := proposalProvenance(baseline, risk)
		excerpt, includedLines, excludedLines := excerptForRiskMinimized(baseline.InventoryRoot, risk)
		minimization.IncludedEvidenceLinks += includedEvidence
		minimization.IncludedProvenanceSlices += includedProvenance
		minimization.IncludedEvidencePaths += len(evidencePaths)
		minimization.IncludedExcerptLines += includedLines
		minimization.ExcludedExcerptLines += excludedLines
		context.Risks = append(context.Risks, ProposalRiskContext{
			ID:            risk.ID,
			Path:          risk.Path,
			Statement:     risk.Statement,
			Kind:          risk.Kind,
			Table:         risk.Table,
			Severity:      risk.Severity,
			Score:         risk.Score,
			Rationale:     risk.Rationale,
			Factors:       append([]ScoreFactor(nil), risk.Factors...),
			FactHashes:    factHashes,
			EvidencePaths: evidencePaths,
			Excerpt:       excerpt,
			Reviewers:     ownersForRiskIDs(baseline.OwnerRoutes, []string{risk.ID}),
		})
	}
	context.NativeChecks = nativeChecksForSelectedRisks(baseline, selectedRiskIDs)
	minimization.IncludedNativeChecks = len(context.NativeChecks)
	minimization.ExcludedNativeChecks = maxInt(0, len(uniqueCommands(baseline.NativeChecks))-minimization.IncludedNativeChecks)
	for _, link := range baseline.EvidenceLinks {
		if !selectedRiskIDs[link.RiskID] {
			minimization.ExcludedEvidenceLinks++
			if strings.TrimSpace(link.Path) != "" {
				minimization.ExcludedEvidencePaths++
			}
		}
	}
	for _, slice := range baseline.Provenance {
		if !selectedRiskIDs[slice.RiskID] {
			minimization.ExcludedProvenanceSlices++
			minimization.ExcludedEvidencePaths += countProvenancePaths(slice)
		}
	}
	context.Minimization = minimization
	return context, minimization
}

func proposalProvenance(baseline BaselineReport, risk BaselineRisk) ([]string, []string, int, int) {
	var factHashes []string
	var paths []string
	includedEvidence := 0
	includedProvenance := 0
	addFact := func(factID string) {
		if strings.TrimSpace(factID) != "" {
			factHashes = append(factHashes, "sha256:"+canonical.Hash(factID)[:16])
		}
	}
	addPath := func(path string) {
		if strings.TrimSpace(path) != "" {
			paths = append(paths, sanitizedEvidencePath(path))
		}
	}
	addPath(risk.Path)
	for _, link := range baseline.EvidenceLinks {
		if link.RiskID != risk.ID {
			continue
		}
		includedEvidence++
		addFact(link.FactID)
		addPath(link.Path)
	}
	for _, slice := range baseline.Provenance {
		if slice.RiskID != risk.ID {
			continue
		}
		includedProvenance++
		addPath(slice.MigrationPath)
		for _, path := range slice.SourcePaths {
			addPath(path)
		}
		for _, path := range slice.IncidentPaths {
			addPath(path)
		}
		for _, path := range slice.RepairPaths {
			addPath(path)
		}
		for _, link := range slice.Links {
			addFact(link.FactID)
			addPath(link.Path)
		}
	}
	factHashes = capStrings(uniqueStrings(factHashes), 8)
	paths = capStrings(uniqueStrings(paths), 8)
	sort.Strings(factHashes)
	sort.Strings(paths)
	return factHashes, paths, includedEvidence, includedProvenance
}

func sanitizedEvidencePath(path string) string {
	path = filepath.ToSlash(strings.TrimSpace(path))
	if path == "" {
		return ""
	}
	parts := strings.Split(path, "/")
	for i, part := range parts {
		lower := strings.ToLower(part)
		if containsAny(lower, "secret", "token", "password", "passwd", "credential", "apikey", "api_key", "private_key") {
			parts[i] = "redacted-" + canonical.Hash(part)[:8]
		}
	}
	return strings.Join(parts, "/")
}

func nativeChecksForSelectedRisks(baseline BaselineReport, selectedRiskIDs map[string]bool) []Command {
	var commands []Command
	for _, slice := range baseline.Provenance {
		if !selectedRiskIDs[slice.RiskID] {
			continue
		}
		commands = append(commands, slice.TestCommands...)
		commands = append(commands, slice.NativeCommands...)
	}
	if len(commands) == 0 {
		for _, risk := range baseline.Risks {
			if !selectedRiskIDs[risk.ID] {
				continue
			}
			for _, command := range baseline.NativeChecks {
				joined := strings.ToLower(command.Command)
				if strings.Contains(joined, strings.ToLower(risk.Path)) || (risk.Table != "" && strings.Contains(joined, strings.ToLower(risk.Table))) {
					commands = append(commands, command)
				}
			}
		}
	}
	return uniqueCommands(commands)
}

func countProvenancePaths(slice ProvenanceSlice) int {
	count := 0
	if strings.TrimSpace(slice.MigrationPath) != "" {
		count++
	}
	count += len(slice.SourcePaths)
	count += len(slice.IncidentPaths)
	count += len(slice.RepairPaths)
	for _, link := range slice.Links {
		if strings.TrimSpace(link.Path) != "" {
			count++
		}
	}
	return count
}

func excerptForRiskMinimized(root string, risk BaselineRisk) (string, int, int) {
	if root == "" || risk.Path == "" {
		return "", 0, 0
	}
	path := filepath.Join(root, filepath.FromSlash(risk.Path))
	text, err := readTextPrefix(path, 8<<10)
	if err != nil {
		return "", 0, 0
	}
	lines := strings.Split(text, "\n")
	kind := strings.ToLower(risk.Kind)
	var keywords []string
	for _, keyword := range []string{"update", "delete", "drop", "truncate", "alter", "insert"} {
		if strings.Contains(kind, keyword) {
			keywords = append(keywords, keyword)
		}
	}
	if len(keywords) == 0 && strings.TrimSpace(kind) != "" {
		keywords = append(keywords, kind)
	}
	keep := map[int]bool{}
	if risk.Statement > 0 && risk.Statement <= len(lines) {
		keep[risk.Statement-1] = true
	}
	for idx, line := range lines {
		lower := strings.ToLower(line)
		for _, keyword := range keywords {
			if strings.TrimSpace(keyword) != "" && strings.Contains(lower, keyword) {
				keep[idx] = true
				break
			}
		}
	}
	var selected []string
	for idx, line := range lines {
		if keep[idx] {
			selected = append(selected, line)
		}
		if len(selected) >= 12 {
			break
		}
	}
	if len(selected) == 0 {
		limit := minInt(len(lines), 3)
		selected = append(selected, lines[:limit]...)
	}
	return strings.Join(selected, "\n"), len(selected), maxInt(0, len(lines)-len(selected))
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func renderProposalPrompt(context ProposalContext) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Patchline proposal context\n")
	fmt.Fprintf(&b, "Baseline hash: %s\n", context.BaselineHash)
	fmt.Fprintf(&b, "Proposal kind: %s\n\n", context.Kind)
	fmt.Fprintf(&b, "Constraints:\n")
	for _, constraint := range context.Constraints {
		fmt.Fprintf(&b, "- %s\n", constraint)
	}
	fmt.Fprintf(&b, "\nContext minimization:\n")
	fmt.Fprintf(&b, "- risks selected=%d excluded=%d\n", context.Minimization.SelectedRisks, context.Minimization.ExcludedRisks)
	fmt.Fprintf(&b, "- evidence_links included=%d excluded=%d provenance_slices included=%d excluded=%d\n", context.Minimization.IncludedEvidenceLinks, context.Minimization.ExcludedEvidenceLinks, context.Minimization.IncludedProvenanceSlices, context.Minimization.ExcludedProvenanceSlices)
	fmt.Fprintf(&b, "- native_checks included=%d excluded=%d excerpt_lines included=%d excluded=%d evidence_paths included=%d excluded=%d\n", context.Minimization.IncludedNativeChecks, context.Minimization.ExcludedNativeChecks, context.Minimization.IncludedExcerptLines, context.Minimization.ExcludedExcerptLines, context.Minimization.IncludedEvidencePaths, context.Minimization.ExcludedEvidencePaths)
	fmt.Fprintf(&b, "\nRisks:\n")
	for _, risk := range context.Risks {
		fmt.Fprintf(&b, "- %s path=%s table=%s severity=%s score=%d rationale=%s\n", risk.ID, risk.Path, risk.Table, risk.Severity, risk.Score, risk.Rationale)
		if len(risk.FactHashes) > 0 || len(risk.EvidencePaths) > 0 {
			fmt.Fprintf(&b, "  provenance fact_hashes=%s evidence_paths=%s\n", provenanceValue(risk.FactHashes), provenanceValue(risk.EvidencePaths))
		}
		if risk.Excerpt != "" {
			fmt.Fprintf(&b, "  excerpt:\n%s\n", indent(risk.Excerpt, "    "))
		}
	}
	return b.String()
}

func renderProposalPromptWithoutFacts(context ProposalContext) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Patchline proposal context\n")
	fmt.Fprintf(&b, "Baseline hash: %s\n", context.BaselineHash)
	fmt.Fprintf(&b, "Kind: %s\n", context.Kind)
	fmt.Fprintf(&b, "Prompt mode: without-facts\n")
	fmt.Fprintf(&b, "Risks: %d withheld for ablation\n", len(context.Risks))
	fmt.Fprintf(&b, "Context minimization: selected_risks=%d excluded_risks=%d excluded_evidence_links=%d excluded_provenance_slices=%d excluded_native_checks=%d excluded_excerpt_lines=%d\n", context.Minimization.SelectedRisks, context.Minimization.ExcludedRisks, context.Minimization.ExcludedEvidenceLinks, context.Minimization.ExcludedProvenanceSlices, context.Minimization.ExcludedNativeChecks, context.Minimization.ExcludedExcerptLines)
	fmt.Fprintf(&b, "Constraints:\n")
	for _, constraint := range context.Constraints {
		fmt.Fprintf(&b, "- %s\n", constraint)
	}
	fmt.Fprintf(&b, "Generate a bounded proposal without repository facts; deterministic re-analysis must catch unsupported output.\n")
	return b.String()
}

func runLLMCommand(command, prompt string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "sh", "-c", command)
	cmd.Stdin = strings.NewReader(prompt)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &limitedWriter{writer: &stdout, limit: 1 << 20}
	cmd.Stderr = &limitedWriter{writer: &stderr, limit: 64 << 10}
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("llm command failed: %w: %s", err, stderr.String())
	}
	return stdout.String(), nil
}

type limitedWriter struct {
	writer io.Writer
	limit  int
	wrote  int
}

func (w *limitedWriter) Write(p []byte) (int, error) {
	original := len(p)
	remaining := w.limit - w.wrote
	if remaining <= 0 {
		return original, nil
	}
	if len(p) > remaining {
		p = p[:remaining]
	}
	n, err := w.writer.Write(p)
	w.wrote += n
	return original, err
}

func generateTemplateArtifacts(context ProposalContext) []GeneratedArtifact {
	kinds := expandProposalKinds(context.Kind)
	var artifacts []GeneratedArtifact
	for _, kind := range kinds {
		for _, risk := range context.Risks {
			slug := safeProposalSlug(risk.ID + "-" + risk.Path)
			switch kind {
			case "tests":
				placement := languageAwareTestPlacement(context, risk, slug)
				artifacts = append(artifacts, GeneratedArtifact{Path: placement.Path, Kind: kind, Content: renderTestProposal(risk, placement.Language), RiskIDs: []string{risk.ID}})
			case "guards":
				artifacts = append(artifacts, GeneratedArtifact{Path: "patchline-proposals/guards/" + slug + ".sql", Kind: kind, Content: renderGuardProposal(risk), RiskIDs: []string{risk.ID}})
			case "instrumentation":
				artifacts = append(artifacts, GeneratedArtifact{Path: "patchline-proposals/instrumentation/" + slug + ".md", Kind: kind, Content: renderInstrumentationProposal(risk), RiskIDs: []string{risk.ID}})
			case "repair":
				artifacts = append(artifacts, GeneratedArtifact{Path: "patchline-proposals/repair/" + slug + ".json", Kind: kind, Content: renderRepairProposal(risk), RiskIDs: []string{risk.ID}})
			case "explain":
				artifacts = append(artifacts, GeneratedArtifact{Path: "patchline-proposals/explain/" + slug + ".sql", Kind: kind, Content: renderExplainProposal(risk), RiskIDs: []string{risk.ID}})
			}
		}
	}
	sort.Slice(artifacts, func(i, j int) bool { return artifacts[i].Path < artifacts[j].Path })
	return artifacts
}

func languageAwareTestPlacement(context ProposalContext, risk ProposalRiskContext, slug string) testPlacement {
	lowerPath := strings.ToLower(filepath.ToSlash(risk.Path))
	lowerRoot := strings.ToLower(filepath.ToSlash(context.InventoryRoot))
	haystack := lowerPath + "\n" + lowerRoot
	commands := strings.ToLower(nativeCommandText(context.NativeChecks))
	switch {
	case (strings.Contains(haystack, "db/migrate") && strings.HasSuffix(lowerPath, ".rb")) || isRailsMigrationFile(lowerPath) || strings.Contains(commands, "rails db:migrate") || strings.Contains(commands, "rails test"):
		return testPlacement{Path: "test/patchline/" + slug + "_test.rb", Language: "ruby"}
	case strings.Contains(commands, "manage.py") || (strings.Contains(haystack, "migrations") && strings.HasSuffix(lowerPath, ".py")):
		return testPlacement{Path: "tests/patchline/test_" + slug + ".py", Language: "python"}
	case strings.HasSuffix(lowerPath, ".go") || strings.Contains(commands, "go test") || strings.Contains(haystack, "/go/") || strings.Contains(haystack, "migrator/migration"):
		return testPlacement{Path: "patchline-proposals/tests/" + slug + "_test.go", Language: "go"}
	case strings.Contains(haystack, "src/main/resources") || strings.HasSuffix(lowerPath, ".java") || strings.HasSuffix(lowerPath, ".xml") || strings.Contains(commands, "mvn ") || strings.Contains(commands, "gradle"):
		return testPlacement{Path: "src/test/java/patchline/" + javaClassName(slug) + "Test.java", Language: "java"}
	case strings.Contains(haystack, "prisma") || strings.Contains(haystack, "typeorm") || strings.Contains(haystack, "database/migrations") || strings.HasSuffix(lowerPath, ".js") || strings.HasSuffix(lowerPath, ".ts") || strings.Contains(commands, "npm ") || strings.Contains(commands, "npx ") || strings.Contains(commands, "yarn "):
		return testPlacement{Path: "test/patchline/" + slug + ".test.js", Language: "javascript"}
	case strings.HasSuffix(lowerPath, ".py") || strings.Contains(commands, "pytest"):
		return testPlacement{Path: "tests/patchline/test_" + slug + ".py", Language: "python"}
	default:
		return testPlacement{Path: "patchline-proposals/tests/" + slug + ".md", Language: "markdown"}
	}
}

func isRailsMigrationFile(path string) bool {
	base := filepath.Base(path)
	if !strings.HasSuffix(base, ".rb") || len(base) < len("20060102150405_x.rb") {
		return false
	}
	prefix := strings.TrimSuffix(base, ".rb")
	if len(prefix) < 15 || prefix[14] != '_' {
		return false
	}
	for _, r := range prefix[:14] {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func nativeCommandText(commands []Command) string {
	var parts []string
	for _, command := range commands {
		parts = append(parts, command.Command, command.Reason)
	}
	return strings.Join(parts, "\n")
}

func javaClassName(slug string) string {
	parts := strings.FieldsFunc(slug, func(r rune) bool {
		return r == '-' || r == '_' || r == '.' || r == '/'
	})
	if len(parts) == 0 {
		return "PatchlineGenerated"
	}
	var b strings.Builder
	for _, part := range parts {
		if part == "" {
			continue
		}
		b.WriteString(strings.ToUpper(part[:1]))
		if len(part) > 1 {
			b.WriteString(part[1:])
		}
	}
	if b.Len() == 0 {
		return "PatchlineGenerated"
	}
	return b.String()
}

func expandProposalKinds(kind string) []string {
	switch kind {
	case "all", "":
		return []string{"tests", "guards", "instrumentation", "repair", "explain"}
	case "tests", "guards", "instrumentation", "repair", "explain":
		return []string{kind}
	default:
		return []string{"tests"}
	}
}

func parseProposalBudget(value string) (ProposalBudget, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return ProposalBudget{}, nil
	}
	budget := ProposalBudget{Raw: value}
	for _, part := range strings.Split(value, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		key, raw, ok := strings.Cut(part, "=")
		if !ok {
			return ProposalBudget{}, fmt.Errorf("budget item %q must be key=value", part)
		}
		key = strings.TrimSpace(strings.ToLower(key))
		n, err := strconv.Atoi(strings.TrimSpace(raw))
		if err != nil || n <= 0 {
			return ProposalBudget{}, fmt.Errorf("budget %s must be a positive integer", key)
		}
		switch key {
		case "files":
			budget.MaxFiles = n
		case "lines":
			budget.MaxLines = n
		case "tokens":
			budget.MaxTokens = n
		case "changes":
			budget.MaxChanges = n
		default:
			return ProposalBudget{}, fmt.Errorf("unknown budget key %q", key)
		}
	}
	return budget, nil
}

func applyProposalBudget(artifacts []GeneratedArtifact, budget ProposalBudget) ([]GeneratedArtifact, []string) {
	if budget.Raw == "" {
		return artifacts, nil
	}
	out := append([]GeneratedArtifact(nil), artifacts...)
	var warnings []string
	if budget.MaxFiles > 0 && len(out) > budget.MaxFiles {
		warnings = append(warnings, fmt.Sprintf("budget files=%d dropped %d generated artifacts", budget.MaxFiles, len(out)-budget.MaxFiles))
		out = out[:budget.MaxFiles]
	}
	if budget.MaxLines > 0 {
		for i := range out {
			trimmed, changed := truncateLines(out[i].Content, budget.MaxLines)
			if changed {
				warnings = append(warnings, fmt.Sprintf("budget lines=%d truncated %s", budget.MaxLines, out[i].Path))
				out[i].Content = trimmed
			}
		}
	}
	if budget.MaxTokens > 0 {
		out, warnings = applyTokenBudget(out, budget.MaxTokens, warnings)
	}
	return out, warnings
}

func truncateLines(content string, maxLines int) (string, bool) {
	lines := strings.Split(strings.TrimSuffix(content, "\n"), "\n")
	if len(lines) <= maxLines {
		return content, false
	}
	return strings.Join(lines[:maxLines], "\n") + "\n", true
}

func applyTokenBudget(artifacts []GeneratedArtifact, maxTokens int, warnings []string) ([]GeneratedArtifact, []string) {
	remainingChars := maxTokens * 4
	var out []GeneratedArtifact
	for _, artifact := range artifacts {
		if remainingChars <= 0 {
			warnings = append(warnings, fmt.Sprintf("budget tokens=%d dropped %s", maxTokens, artifact.Path))
			continue
		}
		if len(artifact.Content) <= remainingChars {
			remainingChars -= len(artifact.Content)
			out = append(out, artifact)
			continue
		}
		artifact.Content = artifact.Content[:remainingChars]
		if !strings.HasSuffix(artifact.Content, "\n") {
			artifact.Content += "\n"
		}
		warnings = append(warnings, fmt.Sprintf("budget tokens=%d truncated %s", maxTokens, artifact.Path))
		remainingChars = 0
		out = append(out, artifact)
	}
	return out, warnings
}

func buildRepairIntervention(baselineHash, outputHash string, riskIDs []string, artifacts []GeneratedArtifact) RepairIntervention {
	kinds := make([]string, 0, len(artifacts))
	for _, artifact := range artifacts {
		kinds = append(kinds, artifact.Kind)
	}
	riskIDs = append([]string(nil), riskIDs...)
	sort.Strings(riskIDs)
	kinds = uniqueSortedStrings(kinds)
	return RepairIntervention{
		ID:            "intervention:" + canonical.Hash(baselineHash + "\x00" + outputHash + "\x00" + strings.Join(riskIDs, ","))[:16],
		Stage:         "generated-untrusted",
		BaselineHash:  baselineHash,
		OutputHash:    outputHash,
		TargetRiskIDs: riskIDs,
		ArtifactKinds: kinds,
		Hypothesis:    "generated artifacts are an intervention intended to reduce or bound the targeted repo risks after deterministic re-analysis",
		RequiredReanalysis: []string{
			"patchline repo compare --before <baseline-report> --after <proposal-report>",
			"apply only in an isolated worktree or patch review",
			"rerun project-native tests before trusting the intervention",
		},
		Trust: "untrusted-until-reanalyzed",
	}
}

func renderTestProposal(risk ProposalRiskContext, language string) string {
	body := fmt.Sprintf(`Untrusted generated test proposal

risk: %s
path: %s
table: %s
fact_hashes: %s
evidence_paths: %s
rationale: %s

Suggested assertions:

1. Build a fixture where table %q has both affected and unaffected rows.
2. Run the project-native migration or repair path touching %q.
3. Assert row counts, scoped predicates, and rollback behavior before accepting the change.
`, risk.ID, risk.Path, risk.Table, provenanceValue(risk.FactHashes), provenanceValue(risk.EvidencePaths), risk.Rationale, risk.Table, risk.Table)
	switch language {
	case "ruby", "python":
		return commentLines(body, "# ")
	case "go", "java", "javascript":
		return commentLines(body, "// ")
	default:
		return "# " + strings.ReplaceAll(body, "\n", "\n")
	}
}

func commentLines(content, prefix string) string {
	lines := strings.Split(strings.TrimSuffix(content, "\n"), "\n")
	for i, line := range lines {
		if strings.TrimSpace(line) == "" {
			lines[i] = prefix
		} else {
			lines[i] = prefix + line
		}
	}
	return strings.Join(lines, "\n") + "\n"
}

func renderGuardProposal(risk ProposalRiskContext) string {
	table := risk.Table
	if table == "" {
		table = "<table_name>"
	}
	return fmt.Sprintf(`-- Untrusted generated guard proposal
-- risk: %s
-- source: %s
-- fact-hashes: %s
-- evidence-paths: %s
-- rationale: %s

BEGIN;

-- Fail closed if the expected table is missing.
SELECT 1 FROM %s LIMIT 1;

-- Review the expected blast radius before applying the risky change.
SELECT count(*) AS patchline_candidate_rows FROM %s;

ROLLBACK;
`, risk.ID, risk.Path, provenanceValue(risk.FactHashes), provenanceValue(risk.EvidencePaths), risk.Rationale, table, table)
}

func renderInstrumentationProposal(risk ProposalRiskContext) string {
	return fmt.Sprintf(`# Untrusted generated instrumentation proposal

- risk: %s
- path: %s
- table: %s
- fact_hashes: %s
- evidence_paths: %s

Add a structured event or metric around this data-change path with:

- risk_id=%q
- table=%q
- operation=%q
- affected_row_count
- dry_run=true/false
- rollback_available=true/false
`, risk.ID, risk.Path, risk.Table, provenanceValue(risk.FactHashes), provenanceValue(risk.EvidencePaths), risk.ID, risk.Table, risk.Kind)
}

func renderRepairProposal(risk ProposalRiskContext) string {
	value := map[string]any{
		"version": "patchline.generated-repair/v1",
		"trust":   "untrusted-generated-proposal",
		"risk_id": risk.ID,
		"source":  risk.Path,
		"provenance": map[string]any{
			"risk_id":        risk.ID,
			"fact_hashes":    risk.FactHashes,
			"evidence_paths": risk.EvidencePaths,
		},
		"scope": map[string]any{
			"table": risk.Table,
			"where": "TODO: replace with bounded predicate before use",
		},
		"preconditions": []string{
			"table exists",
			"candidate row count reviewed",
			"rollback path documented",
		},
		"postconditions": []string{
			"only scoped rows changed",
			"native tests and Patchline re-analysis pass",
		},
		"rollback": map[string]any{
			"required": true,
			"strategy": "snapshot-or-inverse-migration",
			"steps": []string{
				"capture a pre-change snapshot or inverse migration before applying",
				"restore only rows matching the reviewed scope predicate if validation fails",
				"rerun validation commands after rollback",
			},
		},
		"validation_commands": []Command{
			{Command: "patchline repo compare --before <baseline-report> --after <proposal-report>", Reason: "re-run deterministic generated-artifact checks"},
		},
		"owner_review": map[string]any{
			"required": true,
			"status":   "pending",
			"owner":    "data-change-owner",
		},
	}
	data, _ := json.MarshalIndent(value, "", "  ")
	return string(append(data, '\n'))
}

func renderExplainProposal(risk ProposalRiskContext) string {
	table := risk.Table
	if table == "" {
		table = "<table_name>"
	}
	return fmt.Sprintf(`-- Untrusted generated explain/dry-run proposal
-- risk: %s
-- source: %s
-- fact-hashes: %s
-- evidence-paths: %s

EXPLAIN SELECT * FROM %s LIMIT 1;
SELECT count(*) AS patchline_candidate_rows FROM %s;
`, risk.ID, risk.Path, provenanceValue(risk.FactHashes), provenanceValue(risk.EvidencePaths), table, table)
}

func provenanceValue(values []string) string {
	if len(values) == 0 {
		return "none"
	}
	return strings.Join(values, ",")
}

func renderProposalPatch(artifacts []GeneratedArtifact) string {
	var b strings.Builder
	for _, artifact := range artifacts {
		lines := strings.Split(strings.TrimSuffix(artifact.Content, "\n"), "\n")
		fmt.Fprintf(&b, "diff --git a/%s b/%s\n", artifact.Path, artifact.Path)
		fmt.Fprintf(&b, "new file mode 100644\n")
		fmt.Fprintf(&b, "index 0000000..0000000\n")
		fmt.Fprintf(&b, "--- /dev/null\n")
		fmt.Fprintf(&b, "+++ b/%s\n", artifact.Path)
		fmt.Fprintf(&b, "@@ -0,0 +1,%d @@\n", len(lines))
		for _, line := range lines {
			fmt.Fprintf(&b, "+%s\n", line)
		}
	}
	return b.String()
}

func MinimizeGeneratedProposal(baseline BaselineReport, proposal ProposalReport) ProposalReport {
	before := append([]GeneratedArtifact(nil), proposal.Generated...)
	sort.Slice(before, func(i, j int) bool { return before[i].Path < before[j].Path })
	checks := checkGeneratedArtifacts(before)
	checkByPath := map[string]GeneratedCheck{}
	failuresBefore := 0
	for _, check := range checks {
		checkByPath[check.Path] = check
		if check.Status == "fail" {
			failuresBefore++
		}
	}
	validRisks := map[string]bool{}
	for _, risk := range baseline.Risks {
		validRisks[risk.ID] = true
	}
	targetRisks := map[string]bool{}
	for _, id := range proposal.TargetRiskIDs {
		if validRisks[id] {
			targetRisks[id] = true
		}
	}
	if len(targetRisks) == 0 {
		for id := range validRisks {
			targetRisks[id] = true
		}
	}
	covered := map[string]bool{}
	seen := map[string]bool{}
	var kept []GeneratedArtifact
	var removed []RemovedGeneratedArtifact
	for _, artifact := range before {
		relevant := relevantRiskIDs(artifact.RiskIDs, targetRisks)
		reason := ""
		switch {
		case len(relevant) == 0:
			reason = "no-target-risk-coverage"
		case checkByPath[artifact.Path].Status == "fail":
			reason = "deterministic-check-failed"
		case seen[minimizationKey(artifact)]:
			reason = "duplicate-generated-hunk"
		case !addsNewCoverage(relevant, covered):
			reason = "no-new-risk-coverage"
		}
		if reason != "" {
			removed = append(removed, RemovedGeneratedArtifact{Path: artifact.Path, Kind: artifact.Kind, RiskIDs: append([]string(nil), artifact.RiskIDs...), Reason: reason})
			continue
		}
		seen[minimizationKey(artifact)] = true
		for _, id := range relevant {
			covered[id] = true
		}
		kept = append(kept, artifact)
	}
	afterChecks := checkGeneratedArtifacts(kept)
	failuresAfter := 0
	for _, check := range afterChecks {
		if check.Status == "fail" {
			failuresAfter++
		}
	}
	proposal.Generated = kept
	proposal.GeneratedFiles = generatedFilesForArtifacts(kept)
	proposal.Patch = renderProposalPatch(kept)
	proposal.OutputHash = canonical.Hash(proposal.Patch)
	proposal.Intervention = buildRepairIntervention(proposal.BaselineHash, proposal.OutputHash, proposal.TargetRiskIDs, kept)
	proposal.Quarantine = buildGeneratedQuarantine(kept, false)
	proposal.Minimization = ProposalMinimization{
		Applied:                    true,
		BeforeFiles:                len(before),
		AfterFiles:                 len(kept),
		RemovedFiles:               len(removed),
		PreservedRisksWithCoverage: len(covered),
		PreservedCheckFailures:     minInt(failuresBefore, failuresAfter),
		Removed:                    removed,
	}
	proposal.Markdown = renderProposalMarkdown(proposal)
	return proposal
}

func generatedFilesForArtifacts(artifacts []GeneratedArtifact) []GeneratedFile {
	var files []GeneratedFile
	for _, artifact := range artifacts {
		files = append(files, GeneratedFile{Path: artifact.Path, Kind: artifact.Kind, ContentHash: "sha256:" + canonical.Hash(artifact.Content), RiskIDs: append([]string(nil), artifact.RiskIDs...)})
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	return files
}

func relevantRiskIDs(ids []string, target map[string]bool) []string {
	var out []string
	for _, id := range ids {
		if target[id] {
			out = append(out, id)
		}
	}
	return uniqueSortedStrings(out)
}

func addsNewCoverage(ids []string, covered map[string]bool) bool {
	for _, id := range ids {
		if !covered[id] {
			return true
		}
	}
	return false
}

func minimizationKey(artifact GeneratedArtifact) string {
	ids := append([]string(nil), artifact.RiskIDs...)
	sort.Strings(ids)
	return artifact.Kind + "\x00" + strings.Join(ids, ",") + "\x00" + canonical.Hash(artifact.Content)
}

func renderProposalMarkdown(report ProposalReport) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Patchline repo proposal\n\n")
	fmt.Fprintf(&b, "- baseline_hash: `%s`\n", report.BaselineHash)
	fmt.Fprintf(&b, "- kind: `%s`\n", report.Kind)
	fmt.Fprintf(&b, "- generator: `%s`\n", report.Generator)
	fmt.Fprintf(&b, "- deterministic_only: `%t`\n", report.Deterministic)
	fmt.Fprintf(&b, "- trust: `%s`\n", report.Trust)
	if report.ScopeBudget.Raw != "" {
		fmt.Fprintf(&b, "- scope_budget: `%s`\n", report.ScopeBudget.Raw)
	}
	fmt.Fprintf(&b, "- output_hash: `%s`\n\n", report.OutputHash)
	if report.Minimization.Applied {
		fmt.Fprintf(&b, "## Minimization\n\n")
		fmt.Fprintf(&b, "| before files | after files | removed files | preserved covered risks |\n")
		fmt.Fprintf(&b, "| ---: | ---: | ---: | ---: |\n")
		fmt.Fprintf(&b, "| %d | %d | %d | %d |\n\n", report.Minimization.BeforeFiles, report.Minimization.AfterFiles, report.Minimization.RemovedFiles, report.Minimization.PreservedRisksWithCoverage)
		if len(report.Minimization.Removed) > 0 {
			fmt.Fprintf(&b, "| removed path | reason |\n| --- | --- |\n")
			for _, removed := range report.Minimization.Removed {
				fmt.Fprintf(&b, "| %s | %s |\n", removed.Path, removed.Reason)
			}
			fmt.Fprintf(&b, "\n")
		}
	}
	fmt.Fprintf(&b, "## Intervention\n\n")
	fmt.Fprintf(&b, "- id: `%s`\n", report.Intervention.ID)
	fmt.Fprintf(&b, "- stage: `%s`\n", report.Intervention.Stage)
	fmt.Fprintf(&b, "- trust: `%s`\n", report.Intervention.Trust)
	fmt.Fprintf(&b, "- hypothesis: %s\n\n", report.Intervention.Hypothesis)
	if report.Quarantine.Status != "" {
		fmt.Fprintf(&b, "## Generated-code quarantine\n\n")
		fmt.Fprintf(&b, "- status: `%s`\n", report.Quarantine.Status)
		fmt.Fprintf(&b, "- generated artifacts executable: `%t`\n", report.Quarantine.GeneratedArtifactsExecutable)
		fmt.Fprintf(&b, "- generated artifacts applied: `%t`\n", report.Quarantine.GeneratedArtifactsApplied)
		fmt.Fprintf(&b, "- native execution mode: `%s`\n", report.Quarantine.NativeExecutionMode)
		fmt.Fprintf(&b, "- required flag: `%s`\n\n", report.Quarantine.RequiredFlag)
	}
	fmt.Fprintf(&b, "## Generated files\n\n| path | kind | risks | likely reviewers |\n| --- | --- | --- | --- |\n")
	for _, file := range report.GeneratedFiles {
		fmt.Fprintf(&b, "| %s | %s | %s | %s |\n", file.Path, file.Kind, strings.Join(file.RiskIDs, ", "), strings.Join(file.Reviewers, ", "))
	}
	if len(report.OwnerRoutes) > 0 {
		fmt.Fprintf(&b, "\n## Owner routing\n\n| generated file | likely reviewers | rationale |\n| --- | --- | --- |\n")
		for _, route := range report.OwnerRoutes {
			fmt.Fprintf(&b, "| %s | %s | %s |\n", route.Path, strings.Join(route.Owners, ", "), route.Rationale)
		}
	}
	fmt.Fprintf(&b, "\n## Constraints\n\n")
	for _, constraint := range report.Constraints {
		fmt.Fprintf(&b, "- %s\n", constraint)
	}
	if len(report.Warnings) > 0 {
		fmt.Fprintf(&b, "\n## Warnings\n\n")
		for _, warning := range report.Warnings {
			fmt.Fprintf(&b, "- %s\n", warning)
		}
	}
	return b.String()
}

func riskIDs(risks []ProposalRiskContext) []string {
	out := make([]string, 0, len(risks))
	for _, risk := range risks {
		out = append(out, risk.ID)
	}
	sort.Strings(out)
	return out
}

func safeProposalSlug(value string) string {
	value = strings.ToLower(value)
	var b strings.Builder
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		default:
			b.WriteByte('-')
		}
	}
	slug := strings.Trim(b.String(), "-")
	if len(slug) > 80 {
		slug = slug[:80]
	}
	if slug == "" {
		return "proposal"
	}
	return slug
}

func indent(text, prefix string) string {
	lines := strings.Split(text, "\n")
	for i := range lines {
		lines[i] = prefix + lines[i]
	}
	return strings.Join(lines, "\n")
}
