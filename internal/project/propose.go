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
	BaselinePath string
	Kind         string
	OutDir       string
	LLMCommand   string
	NoLLM        bool
	Budget       string
	BudgetRisks  int
}

type ProposalReport struct {
	Version        string              `json:"version"`
	BaselineHash   string              `json:"baseline_hash"`
	Kind           string              `json:"kind"`
	Generator      string              `json:"generator"`
	Deterministic  bool                `json:"deterministic_only"`
	Trust          string              `json:"trust"`
	BudgetRisks    int                 `json:"budget_risks"`
	ScopeBudget    ProposalBudget      `json:"scope_budget,omitempty"`
	TargetRiskIDs  []string            `json:"target_risk_ids"`
	ContextHash    string              `json:"context_hash"`
	PromptHash     string              `json:"prompt_hash"`
	OutputHash     string              `json:"output_hash"`
	Intervention   RepairIntervention  `json:"intervention"`
	GeneratedFiles []GeneratedFile     `json:"generated_files,omitempty"`
	Constraints    []string            `json:"constraints,omitempty"`
	Warnings       []string            `json:"warnings,omitempty"`
	Artifacts      map[string]string   `json:"artifacts,omitempty"`
	Markdown       string              `json:"markdown,omitempty"`
	Context        ProposalContext     `json:"-"`
	Generated      []GeneratedArtifact `json:"-"`
	Patch          string              `json:"-"`
}

type ProposalContext struct {
	Version      string                `json:"version"`
	BaselineHash string                `json:"baseline_hash"`
	Kind         string                `json:"kind"`
	Constraints  []string              `json:"constraints"`
	Risks        []ProposalRiskContext `json:"risks"`
	NativeChecks []Command             `json:"native_checks,omitempty"`
}

type ProposalRiskContext struct {
	ID        string        `json:"id"`
	Path      string        `json:"path"`
	Statement int           `json:"statement,omitempty"`
	Kind      string        `json:"kind"`
	Table     string        `json:"table,omitempty"`
	Severity  string        `json:"severity"`
	Score     int           `json:"score"`
	Rationale string        `json:"rationale"`
	Factors   []ScoreFactor `json:"factors,omitempty"`
	Excerpt   string        `json:"excerpt,omitempty"`
}

type GeneratedFile struct {
	Path        string   `json:"path"`
	Kind        string   `json:"kind"`
	ContentHash string   `json:"content_hash"`
	RiskIDs     []string `json:"risk_ids,omitempty"`
}

type GeneratedArtifact struct {
	Path    string
	Kind    string
	Content string
	RiskIDs []string
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
	context := buildProposalContext(baseline, opts.Kind, opts.BudgetRisks)
	prompt := renderProposalPrompt(context)
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
		Trust:         "untrusted-generated-proposal",
		BudgetRisks:   opts.BudgetRisks,
		ScopeBudget:   budget,
		TargetRiskIDs: riskIDs(context.Risks),
		ContextHash:   canonical.Hash(context),
		PromptHash:    canonical.Hash(prompt),
		OutputHash:    canonical.Hash(patch),
		Constraints:   context.Constraints,
		Artifacts: map[string]string{
			"prompt_context": "prompt-context.json",
			"prompt":         "prompt.txt",
			"patch":          "proposal.patch",
		},
		Context:   context,
		Generated: generated,
		Patch:     patch,
	}
	report.Warnings = append(report.Warnings, budgetWarnings...)
	report.Intervention = buildRepairIntervention(report.BaselineHash, report.OutputHash, report.TargetRiskIDs, generated)
	for _, artifact := range generated {
		report.GeneratedFiles = append(report.GeneratedFiles, GeneratedFile{Path: artifact.Path, Kind: artifact.Kind, ContentHash: "sha256:" + canonical.Hash(artifact.Content), RiskIDs: artifact.RiskIDs})
	}
	sort.Slice(report.GeneratedFiles, func(i, j int) bool { return report.GeneratedFiles[i].Path < report.GeneratedFiles[j].Path })
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
	if err := os.WriteFile(filepath.Join(outDir, "prompt.txt"), []byte(renderProposalPrompt(report.Context)), 0o644); err != nil {
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

func buildProposalContext(baseline BaselineReport, kind string, budget int) ProposalContext {
	context := ProposalContext{
		Version:      ProposalVersion,
		BaselineHash: baseline.Hash,
		Kind:         kind,
		Constraints: []string{
			"Generated output is untrusted until re-analyzed.",
			"Do not assume production data access.",
			"Keep changes bounded to the listed risk IDs.",
			"Prefer fail-closed guards and explicit rollback/validation notes.",
		},
		NativeChecks: baseline.NativeChecks,
	}
	limit := minInt(len(baseline.Risks), budget)
	for _, risk := range baseline.Risks[:limit] {
		context.Risks = append(context.Risks, ProposalRiskContext{
			ID:        risk.ID,
			Path:      risk.Path,
			Statement: risk.Statement,
			Kind:      risk.Kind,
			Table:     risk.Table,
			Severity:  risk.Severity,
			Score:     risk.Score,
			Rationale: risk.Rationale,
			Factors:   append([]ScoreFactor(nil), risk.Factors...),
			Excerpt:   excerptForRisk(baseline.InventoryRoot, risk.Path),
		})
	}
	return context
}

func excerptForRisk(root, rel string) string {
	if root == "" || rel == "" {
		return ""
	}
	path := filepath.Join(root, filepath.FromSlash(rel))
	text, err := readTextPrefix(path, 8<<10)
	if err != nil {
		return ""
	}
	lines := strings.Split(text, "\n")
	if len(lines) > 20 {
		lines = lines[:20]
	}
	return strings.Join(lines, "\n")
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
	fmt.Fprintf(&b, "\nRisks:\n")
	for _, risk := range context.Risks {
		fmt.Fprintf(&b, "- %s path=%s table=%s severity=%s score=%d rationale=%s\n", risk.ID, risk.Path, risk.Table, risk.Severity, risk.Score, risk.Rationale)
		if risk.Excerpt != "" {
			fmt.Fprintf(&b, "  excerpt:\n%s\n", indent(risk.Excerpt, "    "))
		}
	}
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
				artifacts = append(artifacts, GeneratedArtifact{Path: "patchline-proposals/tests/" + slug + ".md", Kind: kind, Content: renderTestProposal(risk), RiskIDs: []string{risk.ID}})
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

func renderTestProposal(risk ProposalRiskContext) string {
	return fmt.Sprintf(`# Untrusted generated test proposal

- risk: %s
- path: %s
- table: %s
- rationale: %s

Suggested assertions:

1. Build a fixture where table %q has both affected and unaffected rows.
2. Run the project-native migration or repair path touching %q.
3. Assert row counts, scoped predicates, and rollback behavior before accepting the change.
`, risk.ID, risk.Path, risk.Table, risk.Rationale, risk.Table, risk.Table)
}

func renderGuardProposal(risk ProposalRiskContext) string {
	table := risk.Table
	if table == "" {
		table = "<table_name>"
	}
	return fmt.Sprintf(`-- Untrusted generated guard proposal
-- risk: %s
-- source: %s
-- rationale: %s

BEGIN;

-- Fail closed if the expected table is missing.
SELECT 1 FROM %s LIMIT 1;

-- Review the expected blast radius before applying the risky change.
SELECT count(*) AS patchline_candidate_rows FROM %s;

ROLLBACK;
`, risk.ID, risk.Path, risk.Rationale, table, table)
}

func renderInstrumentationProposal(risk ProposalRiskContext) string {
	return fmt.Sprintf(`# Untrusted generated instrumentation proposal

- risk: %s
- path: %s
- table: %s

Add a structured event or metric around this data-change path with:

- risk_id=%q
- table=%q
- operation=%q
- affected_row_count
- dry_run=true/false
- rollback_available=true/false
`, risk.ID, risk.Path, risk.Table, risk.ID, risk.Table, risk.Kind)
}

func renderRepairProposal(risk ProposalRiskContext) string {
	value := map[string]any{
		"version": "patchline.generated-repair/v1",
		"trust":   "untrusted-generated-proposal",
		"risk_id": risk.ID,
		"source":  risk.Path,
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
		"rollback": map[string]any{"required": true, "strategy": "snapshot-or-inverse-migration"},
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

EXPLAIN SELECT * FROM %s LIMIT 1;
SELECT count(*) AS patchline_candidate_rows FROM %s;
`, risk.ID, risk.Path, table, table)
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
	fmt.Fprintf(&b, "## Intervention\n\n")
	fmt.Fprintf(&b, "- id: `%s`\n", report.Intervention.ID)
	fmt.Fprintf(&b, "- stage: `%s`\n", report.Intervention.Stage)
	fmt.Fprintf(&b, "- trust: `%s`\n", report.Intervention.Trust)
	fmt.Fprintf(&b, "- hypothesis: %s\n\n", report.Intervention.Hypothesis)
	fmt.Fprintf(&b, "## Generated files\n\n| path | kind | risks |\n| --- | --- | --- |\n")
	for _, file := range report.GeneratedFiles {
		fmt.Fprintf(&b, "| %s | %s | %s |\n", file.Path, file.Kind, strings.Join(file.RiskIDs, ", "))
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
