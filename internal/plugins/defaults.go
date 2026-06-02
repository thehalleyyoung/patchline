package plugins

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/thehalleyyoung/patchline/internal/canonical"
	"github.com/thehalleyyoung/patchline/internal/intake"
	"github.com/thehalleyyoung/patchline/internal/project"
)

type inventoryParser struct{}

func (inventoryParser) Info() Info {
	return Info{
		Name:          "project-inventory-parser",
		Kind:          ParserKind,
		Version:       "v1",
		Deterministic: true,
		Inputs:        []string{"local source tree"},
		Outputs:       []string{"project.Inventory", "[]project.Fact"},
		Description:   "Walks a local project tree and extracts file, language, framework, migration, SQL, infrastructure, command, and CODEOWNERS facts.",
	}
}

func (inventoryParser) Parse(_ context.Context, req ParseRequest) (ParseResult, error) {
	inv, err := project.InventoryPath(project.InventoryOptions{Path: req.Root, Full: req.Full})
	if err != nil {
		return ParseResult{}, err
	}
	return ParseResult{Inventory: inv, Facts: append([]project.Fact(nil), inv.Facts...)}, nil
}

type intakeExtractor struct{}

func (intakeExtractor) Info() Info {
	return Info{
		Name:          "project-intake-fact-extractor",
		Kind:          FactExtractorKind,
		Version:       "v1",
		Deterministic: true,
		Inputs:        []string{"local source tree", "project.Inventory"},
		Outputs:       []string{"intake.Report", "candidate links"},
		Description:   "Extracts SQL findings, source-embedded SQL, evidence exports, repair candidates, time signals, and identifier links.",
	}
}

func (intakeExtractor) ExtractFacts(ctx context.Context, req FactRequest) (FactResult, error) {
	report, err := intake.Run(ctx, intake.Options{Path: req.Root})
	if err != nil {
		return FactResult{}, err
	}
	return FactResult{Intake: report, Facts: append([]project.Fact(nil), req.Inventory.Facts...)}, nil
}

type baselineLinker struct{}

func (baselineLinker) Info() Info {
	return Info{
		Name:          "baseline-evidence-linker",
		Kind:          LinkerKind,
		Version:       "v1",
		Deterministic: true,
		Inputs:        []string{"project.Inventory", "[]project.Fact", "intake.Report"},
		Outputs:       []string{"project.BaselineReport", "[]project.EvidenceLink"},
		Description:   "Links inventory facts, intake candidates, source SQL, and identifier evidence into baseline evidence links and provenance slices.",
	}
}

func (baselineLinker) Link(_ context.Context, req LinkRequest) (LinkResult, error) {
	baseline := project.Baseline(req.Inventory, req.Facts, req.Intake)
	return LinkResult{Baseline: baseline, Links: append([]project.EvidenceLink(nil), baseline.EvidenceLinks...)}, nil
}

type baselineRanker struct{}

func (baselineRanker) Info() Info {
	return Info{
		Name:          "baseline-risk-ranker",
		Kind:          RankerKind,
		Version:       "v1",
		Deterministic: true,
		Inputs:        []string{"project.BaselineReport"},
		Outputs:       []string{"[]project.BaselineRisk", "ranking explanations"},
		Description:   "Returns deterministic baseline risk ordering with feature contributions and capped explanation rows.",
	}
}

func (baselineRanker) Rank(_ context.Context, req RankRequest) (RankResult, error) {
	risks := append([]project.BaselineRisk(nil), req.Baseline.Risks...)
	return RankResult{Risks: risks}, nil
}

type templateProposalGenerator struct{}

func (templateProposalGenerator) Info() Info {
	return Info{
		Name:          "template-proposal-generator",
		Kind:          ProposalGeneratorKind,
		Version:       "v1",
		Deterministic: true,
		Inputs:        []string{"project.BaselineReport", "proposal kind", "scope budget"},
		Outputs:       []string{"project.ProposalReport", "generated untrusted artifacts"},
		Description:   "Uses Patchline's deterministic templates to generate untrusted repair, guard, instrumentation, and explanation artifacts.",
	}
}

func (templateProposalGenerator) Generate(_ context.Context, req ProposalRequest) (ProposalResult, error) {
	outDir := req.OutDir
	if outDir == "" {
		return ProposalResult{}, fmt.Errorf("template proposal generator requires an output directory")
	}
	baselineDir := filepath.Join(filepath.Dir(outDir), "baseline")
	if err := project.WriteBaseline(baselineDir, req.Baseline); err != nil {
		return ProposalResult{}, err
	}
	proposal, err := project.Propose(project.ProposalOptions{BaselinePath: baselineDir, Kind: defaultString(req.Kind, "all"), OutDir: outDir, NoLLM: true, Budget: defaultString(req.Budget, "files=4,lines=80,tokens=12000,changes=2")})
	if err != nil {
		return ProposalResult{}, err
	}
	return ProposalResult{Proposal: proposal}, nil
}

type interventionCompareCheck struct{}

func (interventionCompareCheck) Info() Info {
	return Info{
		Name:          "intervention-compare-check",
		Kind:          CompareCheckKind,
		Version:       "v1",
		Deterministic: true,
		Inputs:        []string{"project.BaselineReport", "project.ProposalReport"},
		Outputs:       []string{"project.CompareReport", "generated checks", "intervention loop"},
		Description:   "Re-analyzes generated artifacts as untrusted interventions and emits compare checks, risk deltas, and review badge state.",
	}
}

func (interventionCompareCheck) Check(_ context.Context, req CompareRequest) (CompareResult, error) {
	compare := project.Compare(req.Baseline, req.Proposal)
	return CompareResult{Compare: compare, Checks: append([]project.GeneratedCheck(nil), compare.GeneratedChecks...)}, nil
}

type jsonRenderer struct{}

func (jsonRenderer) Info() Info {
	return Info{
		Name:          "json-report-renderer",
		Kind:          ReportRendererKind,
		Version:       "v1",
		Deterministic: true,
		Inputs:        []string{"report struct"},
		Outputs:       []string{"canonical JSON"},
		Description:   "Renders Patchline reports as stable indented JSON for artifact exchange and tests.",
	}
}

func (jsonRenderer) Render(_ context.Context, req RenderRequest) (RenderResult, error) {
	data, err := json.MarshalIndent(req.Report, "", "  ")
	if err != nil {
		return RenderResult{}, err
	}
	content := string(append(data, '\n'))
	return RenderResult{Name: req.Name, Format: "json", Content: content, Hash: canonical.Hash(content)}, nil
}

type markdownRenderer struct{}

func (markdownRenderer) Info() Info {
	return Info{
		Name:          "markdown-report-renderer",
		Kind:          ReportRendererKind,
		Version:       "v1",
		Deterministic: true,
		Inputs:        []string{"report struct with Markdown field"},
		Outputs:       []string{"Markdown"},
		Description:   "Renders Patchline reports using their generated Markdown field for maintainer review.",
	}
}

func (markdownRenderer) Render(_ context.Context, req RenderRequest) (RenderResult, error) {
	markdown, err := reportMarkdown(req.Report)
	if err != nil {
		return RenderResult{}, err
	}
	return RenderResult{Name: req.Name, Format: "markdown", Content: markdown, Hash: canonical.Hash(markdown)}, nil
}

func reportMarkdown(report any) (string, error) {
	switch typed := report.(type) {
	case project.BaselineReport:
		return typed.Markdown, nil
	case project.ProposalReport:
		return typed.Markdown, nil
	case project.CompareReport:
		return typed.Markdown, nil
	case ProbeReport:
		return typed.Markdown, nil
	default:
		return "", fmt.Errorf("report type %T does not expose a supported Markdown field", report)
	}
}
