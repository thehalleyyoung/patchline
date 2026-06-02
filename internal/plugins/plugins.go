package plugins

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/thehalleyyoung/patchline/internal/canonical"
	"github.com/thehalleyyoung/patchline/internal/intake"
	"github.com/thehalleyyoung/patchline/internal/project"
)

const (
	CatalogVersion = "patchline.plugin-catalog/v1"
	ProbeVersion   = "patchline.plugin-probe/v1"
)

type Kind string

const (
	ParserKind            Kind = "parser"
	FactExtractorKind     Kind = "fact-extractor"
	LinkerKind            Kind = "linker"
	RankerKind            Kind = "ranker"
	ProposalGeneratorKind Kind = "proposal-generator"
	CompareCheckKind      Kind = "compare-check"
	ReportRendererKind    Kind = "report-renderer"
)

type Info struct {
	Name          string   `json:"name"`
	Kind          Kind     `json:"kind"`
	Version       string   `json:"version"`
	Deterministic bool     `json:"deterministic"`
	Inputs        []string `json:"inputs,omitempty"`
	Outputs       []string `json:"outputs,omitempty"`
	Description   string   `json:"description"`
}

type Catalog struct {
	Version string `json:"version"`
	Plugins []Info `json:"plugins"`
	Hash    string `json:"hash"`
}

type Registry struct {
	Parsers            []Parser
	FactExtractors     []FactExtractor
	Linkers            []Linker
	Rankers            []Ranker
	ProposalGenerators []ProposalGenerator
	CompareChecks      []CompareCheck
	ReportRenderers    []ReportRenderer
}

type Parser interface {
	Info() Info
	Parse(context.Context, ParseRequest) (ParseResult, error)
}

type FactExtractor interface {
	Info() Info
	ExtractFacts(context.Context, FactRequest) (FactResult, error)
}

type Linker interface {
	Info() Info
	Link(context.Context, LinkRequest) (LinkResult, error)
}

type Ranker interface {
	Info() Info
	Rank(context.Context, RankRequest) (RankResult, error)
}

type ProposalGenerator interface {
	Info() Info
	Generate(context.Context, ProposalRequest) (ProposalResult, error)
}

type CompareCheck interface {
	Info() Info
	Check(context.Context, CompareRequest) (CompareResult, error)
}

type ReportRenderer interface {
	Info() Info
	Render(context.Context, RenderRequest) (RenderResult, error)
}

type ParseRequest struct {
	Root string `json:"root"`
	Full bool   `json:"full,omitempty"`
}

type ParseResult struct {
	Inventory project.Inventory `json:"inventory"`
	Facts     []project.Fact    `json:"facts,omitempty"`
}

type FactRequest struct {
	Root      string            `json:"root"`
	Inventory project.Inventory `json:"-"`
}

type FactResult struct {
	Intake intake.Report  `json:"intake"`
	Facts  []project.Fact `json:"facts,omitempty"`
}

type LinkRequest struct {
	Inventory project.Inventory `json:"-"`
	Facts     []project.Fact    `json:"-"`
	Intake    intake.Report     `json:"-"`
}

type LinkResult struct {
	Baseline project.BaselineReport `json:"baseline"`
	Links    []project.EvidenceLink `json:"links,omitempty"`
}

type RankRequest struct {
	Baseline project.BaselineReport `json:"-"`
}

type RankResult struct {
	Risks []project.BaselineRisk `json:"risks,omitempty"`
}

type ProposalRequest struct {
	Baseline project.BaselineReport `json:"-"`
	Kind     string                 `json:"kind,omitempty"`
	Budget   string                 `json:"budget,omitempty"`
	OutDir   string                 `json:"out_dir,omitempty"`
}

type ProposalResult struct {
	Proposal project.ProposalReport `json:"proposal"`
}

type CompareRequest struct {
	Baseline project.BaselineReport `json:"-"`
	Proposal project.ProposalReport `json:"-"`
}

type CompareResult struct {
	Compare project.CompareReport    `json:"compare"`
	Checks  []project.GeneratedCheck `json:"checks,omitempty"`
}

type RenderRequest struct {
	Name   string `json:"name"`
	Format string `json:"format"`
	Report any    `json:"-"`
}

type RenderResult struct {
	Name    string `json:"name"`
	Format  string `json:"format"`
	Content string `json:"content"`
	Hash    string `json:"hash"`
}

type ProbeOptions struct {
	Path        string
	GitHub      string
	Ref         string
	Subpath     string
	DownloadDir string
	OutDir      string
	Kind        string
	Budget      string
}

type ProbeReport struct {
	Version  string            `json:"version"`
	Input    string            `json:"input"`
	Subpath  string            `json:"subpath,omitempty"`
	Catalog  Catalog           `json:"catalog"`
	Source   project.Source    `json:"source,omitempty"`
	Outputs  map[string]string `json:"outputs,omitempty"`
	Summary  ProbeSummary      `json:"summary"`
	Rendered []RenderedProbe   `json:"rendered,omitempty"`
	Hash     string            `json:"hash"`
	Markdown string            `json:"markdown,omitempty"`
}

type ProbeSummary struct {
	Parsers            int `json:"parsers"`
	FactExtractors     int `json:"fact_extractors"`
	Linkers            int `json:"linkers"`
	Rankers            int `json:"rankers"`
	ProposalGenerators int `json:"proposal_generators"`
	CompareChecks      int `json:"compare_checks"`
	ReportRenderers    int `json:"report_renderers"`
	FilesScanned       int `json:"files_scanned"`
	Facts              int `json:"facts"`
	IntakeLinks        int `json:"intake_links"`
	EvidenceLinks      int `json:"evidence_links"`
	RankedRisks        int `json:"ranked_risks"`
	GeneratedFiles     int `json:"generated_files"`
	GeneratedChecks    int `json:"generated_checks"`
	RenderedReports    int `json:"rendered_reports"`
}

type RenderedProbe struct {
	Name   string `json:"name"`
	Format string `json:"format"`
	Hash   string `json:"hash"`
	Bytes  int    `json:"bytes"`
}

func DefaultRegistry() Registry {
	return Registry{
		Parsers:            []Parser{inventoryParser{}},
		FactExtractors:     []FactExtractor{intakeExtractor{}},
		Linkers:            []Linker{baselineLinker{}},
		Rankers:            []Ranker{baselineRanker{}},
		ProposalGenerators: []ProposalGenerator{templateProposalGenerator{}},
		CompareChecks:      []CompareCheck{interventionCompareCheck{}},
		ReportRenderers:    []ReportRenderer{jsonRenderer{}, markdownRenderer{}},
	}
}

func DefaultCatalog() Catalog {
	return DefaultRegistry().Catalog()
}

func (r Registry) Catalog() Catalog {
	plugins := r.Infos()
	sort.Slice(plugins, func(i, j int) bool {
		if plugins[i].Kind == plugins[j].Kind {
			return plugins[i].Name < plugins[j].Name
		}
		return plugins[i].Kind < plugins[j].Kind
	})
	catalog := Catalog{Version: CatalogVersion, Plugins: plugins}
	catalog.Hash = canonical.Hash(struct {
		Version string `json:"version"`
		Plugins []Info `json:"plugins"`
	}{catalog.Version, catalog.Plugins})
	return catalog
}

func (r Registry) Infos() []Info {
	var infos []Info
	for _, plugin := range r.Parsers {
		infos = append(infos, plugin.Info())
	}
	for _, plugin := range r.FactExtractors {
		infos = append(infos, plugin.Info())
	}
	for _, plugin := range r.Linkers {
		infos = append(infos, plugin.Info())
	}
	for _, plugin := range r.Rankers {
		infos = append(infos, plugin.Info())
	}
	for _, plugin := range r.ProposalGenerators {
		infos = append(infos, plugin.Info())
	}
	for _, plugin := range r.CompareChecks {
		infos = append(infos, plugin.Info())
	}
	for _, plugin := range r.ReportRenderers {
		infos = append(infos, plugin.Info())
	}
	return infos
}

func Probe(ctx context.Context, opts ProbeOptions) (ProbeReport, error) {
	registry := DefaultRegistry()
	root, source, err := resolveProbeSource(ctx, opts)
	if err != nil {
		return ProbeReport{}, err
	}
	report := ProbeReport{
		Version: ProbeVersion,
		Input:   root,
		Subpath: opts.Subpath,
		Catalog: registry.Catalog(),
		Source:  source,
		Outputs: map[string]string{},
		Summary: ProbeSummary{
			Parsers:            len(registry.Parsers),
			FactExtractors:     len(registry.FactExtractors),
			Linkers:            len(registry.Linkers),
			Rankers:            len(registry.Rankers),
			ProposalGenerators: len(registry.ProposalGenerators),
			CompareChecks:      len(registry.CompareChecks),
			ReportRenderers:    len(registry.ReportRenderers),
		},
	}
	if opts.OutDir != "" {
		report.Outputs["root"] = opts.OutDir
	}

	parse, err := registry.Parsers[0].Parse(ctx, ParseRequest{Root: root})
	if err != nil {
		return ProbeReport{}, err
	}
	report.Summary.FilesScanned = parse.Inventory.FilesScanned
	report.Summary.Facts = len(parse.Facts)

	factResult, err := registry.FactExtractors[0].ExtractFacts(ctx, FactRequest{Root: root, Inventory: parse.Inventory})
	if err != nil {
		return ProbeReport{}, err
	}
	report.Summary.IntakeLinks = len(factResult.Intake.Links)

	linkResult, err := registry.Linkers[0].Link(ctx, LinkRequest{Inventory: parse.Inventory, Facts: parse.Facts, Intake: factResult.Intake})
	if err != nil {
		return ProbeReport{}, err
	}
	report.Summary.EvidenceLinks = len(linkResult.Links)

	rankResult, err := registry.Rankers[0].Rank(ctx, RankRequest{Baseline: linkResult.Baseline})
	if err != nil {
		return ProbeReport{}, err
	}
	report.Summary.RankedRisks = len(rankResult.Risks)

	proposalOut := ""
	if opts.OutDir != "" {
		proposalOut = filepath.Join(opts.OutDir, "proposal")
		if err := project.WriteBaseline(filepath.Join(opts.OutDir, "baseline"), linkResult.Baseline); err != nil {
			return ProbeReport{}, err
		}
		report.Outputs["baseline"] = filepath.Join(opts.OutDir, "baseline")
		report.Outputs["proposal"] = proposalOut
	}
	proposal, err := registry.ProposalGenerators[0].Generate(ctx, ProposalRequest{Baseline: linkResult.Baseline, Kind: defaultString(opts.Kind, "all"), Budget: defaultString(opts.Budget, "files=4,lines=80,tokens=12000,changes=2"), OutDir: proposalOut})
	if err != nil {
		return ProbeReport{}, err
	}
	report.Summary.GeneratedFiles = len(proposal.Proposal.GeneratedFiles)
	if opts.OutDir != "" {
		if err := project.WriteProposal(proposalOut, proposal.Proposal); err != nil {
			return ProbeReport{}, err
		}
	}

	compare, err := registry.CompareChecks[0].Check(ctx, CompareRequest{Baseline: linkResult.Baseline, Proposal: proposal.Proposal})
	if err != nil {
		return ProbeReport{}, err
	}
	report.Summary.GeneratedChecks = len(compare.Checks)
	if opts.OutDir != "" {
		compareOut := filepath.Join(opts.OutDir, "compare")
		if err := project.WriteCompare(compareOut, compare.Compare); err != nil {
			return ProbeReport{}, err
		}
		report.Outputs["compare"] = compareOut
	}

	renderInputs := []RenderRequest{
		{Name: "baseline", Format: "json", Report: linkResult.Baseline},
		{Name: "proposal", Format: "json", Report: proposal.Proposal},
		{Name: "compare", Format: "json", Report: compare.Compare},
		{Name: "baseline", Format: "markdown", Report: linkResult.Baseline},
	}
	for _, renderInput := range renderInputs {
		renderer := selectRenderer(registry.ReportRenderers, renderInput.Format)
		if renderer == nil {
			return ProbeReport{}, fmt.Errorf("no renderer for format %q", renderInput.Format)
		}
		rendered, err := renderer.Render(ctx, renderInput)
		if err != nil {
			return ProbeReport{}, err
		}
		report.Rendered = append(report.Rendered, RenderedProbe{Name: rendered.Name, Format: rendered.Format, Hash: rendered.Hash, Bytes: len(rendered.Content)})
		if opts.OutDir != "" {
			renderDir := filepath.Join(opts.OutDir, "rendered")
			if err := os.MkdirAll(renderDir, 0o755); err != nil {
				return ProbeReport{}, err
			}
			ext := "txt"
			if rendered.Format == "json" {
				ext = "json"
			} else if rendered.Format == "markdown" {
				ext = "md"
			}
			path := filepath.Join(renderDir, rendered.Name+"."+ext)
			if err := os.WriteFile(path, []byte(rendered.Content), 0o644); err != nil {
				return ProbeReport{}, err
			}
			report.Outputs["rendered_"+rendered.Name+"_"+rendered.Format] = path
		}
	}
	report.Summary.RenderedReports = len(report.Rendered)
	report.Hash = canonical.Hash(struct {
		Version     string          `json:"version"`
		Input       string          `json:"input"`
		Subpath     string          `json:"subpath,omitempty"`
		CatalogHash string          `json:"catalog_hash"`
		Summary     ProbeSummary    `json:"summary"`
		Rendered    []RenderedProbe `json:"rendered,omitempty"`
	}{report.Version, report.Input, report.Subpath, report.Catalog.Hash, report.Summary, report.Rendered})
	report.Markdown = renderProbeMarkdown(report)
	return report, nil
}

func WriteProbe(outDir string, report ProbeReport) error {
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}
	copy := report
	copy.Markdown = ""
	data, err := json.MarshalIndent(copy, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(outDir, "plugin-probe.json"), append(data, '\n'), 0o644); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(outDir, "plugin-probe.md"), []byte(report.Markdown), 0o644)
}

func resolveProbeSource(ctx context.Context, opts ProbeOptions) (string, project.Source, error) {
	if opts.GitHub != "" {
		if opts.OutDir == "" {
			return "", project.Source{}, fmt.Errorf("--out is required with --github for plugin probe cache metadata")
		}
		fetched, err := project.Fetch(ctx, project.FetchOptions{Input: opts.GitHub, Ref: opts.Ref, Subpath: opts.Subpath, OutDir: filepath.Join(opts.OutDir, "fetch"), DownloadDir: opts.DownloadDir})
		if err != nil {
			return "", project.Source{}, err
		}
		return fetched.Source.ScannedRoot, fetched.Source, nil
	}
	root := opts.Path
	if root == "" {
		root = "."
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return "", project.Source{}, err
	}
	return abs, project.Source{Mode: "local", Input: root, Subpath: opts.Subpath, ScannedRoot: filepath.ToSlash(abs)}, nil
}

func selectRenderer(renderers []ReportRenderer, format string) ReportRenderer {
	for _, renderer := range renderers {
		if renderer.Info().Name == format+"-report-renderer" {
			return renderer
		}
	}
	return nil
}

func renderProbeMarkdown(report ProbeReport) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Patchline plugin probe\n\n")
	fmt.Fprintf(&b, "- input: `%s`\n", report.Input)
	fmt.Fprintf(&b, "- catalog_hash: `%s`\n", report.Catalog.Hash)
	fmt.Fprintf(&b, "- hash: `%s`\n\n", report.Hash)
	fmt.Fprintf(&b, "## Summary\n\n| area | count |\n| --- | ---: |\n")
	fmt.Fprintf(&b, "| parsers | %d |\n", report.Summary.Parsers)
	fmt.Fprintf(&b, "| fact extractors | %d |\n", report.Summary.FactExtractors)
	fmt.Fprintf(&b, "| linkers | %d |\n", report.Summary.Linkers)
	fmt.Fprintf(&b, "| rankers | %d |\n", report.Summary.Rankers)
	fmt.Fprintf(&b, "| proposal generators | %d |\n", report.Summary.ProposalGenerators)
	fmt.Fprintf(&b, "| compare checks | %d |\n", report.Summary.CompareChecks)
	fmt.Fprintf(&b, "| report renderers | %d |\n", report.Summary.ReportRenderers)
	fmt.Fprintf(&b, "| files scanned | %d |\n", report.Summary.FilesScanned)
	fmt.Fprintf(&b, "| facts | %d |\n", report.Summary.Facts)
	fmt.Fprintf(&b, "| ranked risks | %d |\n", report.Summary.RankedRisks)
	fmt.Fprintf(&b, "| generated files | %d |\n", report.Summary.GeneratedFiles)
	fmt.Fprintf(&b, "| generated checks | %d |\n", report.Summary.GeneratedChecks)
	fmt.Fprintf(&b, "| rendered reports | %d |\n", report.Summary.RenderedReports)
	return b.String()
}

func defaultString(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}
