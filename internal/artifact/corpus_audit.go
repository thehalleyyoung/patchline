package artifact

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/thehalleyyoung/patchline/internal/canonical"
)

const CorpusAuditVersion = "patchline.corpus-audit/v1"

type CorpusProtocol struct {
	Version           string                      `json:"version"`
	Description       string                      `json:"description"`
	InclusionPolicy   string                      `json:"inclusion_policy"`
	ExclusionPolicy   string                      `json:"exclusion_policy"`
	RequiredManifests []CorpusManifestRequirement `json:"required_manifests"`
	CandidatePools    []CorpusCandidatePool       `json:"candidate_pools"`
	ReviewerCommands  []CorpusReviewerCommand     `json:"reviewer_commands"`
}

type CorpusManifestRequirement struct {
	DatasetID           string         `json:"dataset_id"`
	Manifest            string         `json:"manifest"`
	MinCases            int            `json:"min_cases"`
	MinBoundaryCases    int            `json:"min_boundary_cases,omitempty"`
	BoundaryResults     []string       `json:"boundary_results,omitempty"`
	MinResults          map[string]int `json:"min_results,omitempty"`
	RequiredEvidenceAny []string       `json:"required_evidence_any,omitempty"`
	RequiredEvidenceAll []string       `json:"required_evidence_all,omitempty"`
	AllowInline         bool           `json:"allow_inline,omitempty"`
	RequiresFetch       *bool          `json:"requires_fetch,omitempty"`
}

type CorpusCandidatePool struct {
	ID          string            `json:"id"`
	Description string            `json:"description"`
	Source      string            `json:"source"`
	SourceHash  string            `json:"source_hash,omitempty"`
	Candidates  []CorpusCandidate `json:"candidates"`
}

type CorpusCandidate struct {
	CaseID          string `json:"case_id"`
	Disposition     string `json:"disposition"`
	Manifest        string `json:"manifest,omitempty"`
	Source          string `json:"source"`
	Rationale       string `json:"rationale,omitempty"`
	ExclusionReason string `json:"exclusion_reason,omitempty"`
}

type CorpusReviewerCommand struct {
	ID           string `json:"id"`
	Command      string `json:"command"`
	ExpectedExit int    `json:"expected_exit"`
	Mode         string `json:"mode"`
}

type CorpusAuditReport struct {
	Version      string                     `json:"version"`
	Root         string                     `json:"root"`
	Protocol     string                     `json:"protocol"`
	ProtocolHash string                     `json:"protocol_hash"`
	Manifests    []CorpusManifestAudit      `json:"manifests"`
	Pools        []CorpusCandidatePoolAudit `json:"candidate_pools"`
	Commands     []CorpusCommandAudit       `json:"reviewer_commands"`
	Errors       []ValidationError          `json:"errors,omitempty"`
	OK           bool                       `json:"ok"`
	Hash         string                     `json:"hash"`
	Markdown     string                     `json:"markdown,omitempty"`
}

type CorpusManifestAudit struct {
	DatasetID           string         `json:"dataset_id"`
	Manifest            string         `json:"manifest"`
	ManifestHash        string         `json:"manifest_hash"`
	GroundTruthHash     string         `json:"ground_truth_hash"`
	Cases               int            `json:"cases"`
	InlineCases         int            `json:"inline_cases"`
	BoundaryCases       int            `json:"boundary_cases"`
	ResultCounts        map[string]int `json:"result_counts"`
	CaseTypeCounts      map[string]int `json:"case_type_counts"`
	PublicEvidenceCases int            `json:"public_evidence_cases"`
	OK                  bool           `json:"ok"`
}

type CorpusCandidatePoolAudit struct {
	ID       string `json:"id"`
	Hash     string `json:"hash"`
	Included int    `json:"included"`
	Excluded int    `json:"excluded"`
	OK       bool   `json:"ok"`
}

type CorpusCommandAudit struct {
	ID           string `json:"id"`
	Command      string `json:"command"`
	ExpectedExit int    `json:"expected_exit"`
	Mode         string `json:"mode"`
	Target       string `json:"target,omitempty"`
	OK           bool   `json:"ok"`
}

func GenerateCorpusAudit(root, protocolPath string) (CorpusAuditReport, error) {
	if root == "" {
		root = "."
	}
	if protocolPath == "" {
		protocolPath = filepath.Join(root, "benchmarks", "corpus_protocol.json")
	} else if !filepath.IsAbs(protocolPath) {
		protocolPath = filepath.Join(root, protocolPath)
	}
	var protocol CorpusProtocol
	if err := readJSONFile(protocolPath, &protocol); err != nil {
		return CorpusAuditReport{}, err
	}
	protocolDigest, err := digestFile("corpus-protocol", root, protocolPath)
	if err != nil {
		return CorpusAuditReport{}, err
	}
	report := CorpusAuditReport{
		Version:      CorpusAuditVersion,
		Root:         filepath.ToSlash(root),
		Protocol:     protocolDigest.Path,
		ProtocolHash: protocolDigest.Hash,
	}
	report.Errors = append(report.Errors, validateCorpusProtocolHeader(protocolPath, protocol)...)

	groundTruthByPath, err := loadGroundTruthByAbs(filepath.Join(root, "benchmarks"))
	if err != nil {
		return CorpusAuditReport{}, err
	}
	discovered, err := discoverBenchmarkManifests(root)
	if err != nil {
		return CorpusAuditReport{}, err
	}
	requiredByManifest := map[string]CorpusManifestRequirement{}
	for _, req := range protocol.RequiredManifests {
		rel := filepath.ToSlash(req.Manifest)
		requiredByManifest[rel] = req
	}
	for _, rel := range discovered {
		if _, ok := requiredByManifest[rel]; !ok {
			report.Errors = append(report.Errors, ValidationError{File: protocolDigest.Path, Message: "manifest is not covered by corpus protocol: " + rel})
		}
	}

	candidates, poolAudits, candidateErrors := auditCandidatePools(protocol)
	report.Pools = poolAudits
	report.Errors = append(report.Errors, candidateErrors...)
	seenIncluded := map[string]bool{}
	for _, req := range protocol.RequiredManifests {
		audit, err := auditCorpusManifest(root, req, candidates, seenIncluded, groundTruthByPath)
		if err != nil {
			report.Errors = append(report.Errors, ValidationError{File: req.Manifest, Message: err.Error()})
			continue
		}
		report.Manifests = append(report.Manifests, audit)
		if !audit.OK {
			report.Errors = append(report.Errors, ValidationError{File: req.Manifest, Message: "manifest requirement failed"})
		}
	}
	for key, candidate := range candidates {
		if candidate.Disposition == "included" && !seenIncluded[key] {
			report.Errors = append(report.Errors, ValidationError{File: protocolDigest.Path, CaseID: candidate.CaseID, Message: "included candidate was not observed in its manifest: " + candidate.Manifest})
		}
	}
	commandAudits, commandErrors := auditReviewerCommands(root, protocol.ReviewerCommands)
	report.Commands = commandAudits
	report.Errors = append(report.Errors, commandErrors...)
	sortValidationErrors(report.Errors)
	sort.Slice(report.Manifests, func(i, j int) bool { return report.Manifests[i].Manifest < report.Manifests[j].Manifest })
	sort.Slice(report.Pools, func(i, j int) bool { return report.Pools[i].ID < report.Pools[j].ID })
	sort.Slice(report.Commands, func(i, j int) bool { return report.Commands[i].ID < report.Commands[j].ID })
	report.OK = len(report.Errors) == 0
	report.Hash = corpusAuditHash(report)
	report.Markdown = renderCorpusAuditMarkdown(report)
	return report, nil
}

func WriteCorpusAuditReport(outDir string, report CorpusAuditReport) error {
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(outDir, "summary.json"), append(data, '\n'), 0o644); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(outDir, "summary.md"), []byte(report.Markdown), 0o644)
}

func validateCorpusProtocolHeader(path string, protocol CorpusProtocol) []ValidationError {
	var errs []ValidationError
	add := func(message string) {
		errs = append(errs, ValidationError{File: path, Message: message})
	}
	if protocol.Version != "patchline.corpus-protocol/v1" {
		add("unexpected or missing protocol version")
	}
	if protocol.InclusionPolicy == "" {
		add("missing inclusion_policy")
	}
	if protocol.ExclusionPolicy == "" {
		add("missing exclusion_policy")
	}
	if len(protocol.RequiredManifests) == 0 {
		add("missing required_manifests")
	}
	if len(protocol.CandidatePools) == 0 {
		add("missing candidate_pools")
	}
	if len(protocol.ReviewerCommands) == 0 {
		add("missing reviewer_commands")
	}
	return errs
}

func loadGroundTruthByAbs(root string) (map[string]GroundTruthCase, error) {
	out := map[string]GroundTruthCase{}
	dir := filepath.Join(root, "ground_truth")
	err := filepath.WalkDir(dir, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || filepath.Ext(path) != ".json" {
			return nil
		}
		var gt GroundTruthCase
		if err := readJSONFile(path, &gt); err != nil {
			return err
		}
		abs, err := filepath.Abs(path)
		if err != nil {
			return err
		}
		out[abs] = gt
		return nil
	})
	return out, err
}

func discoverBenchmarkManifests(root string) ([]string, error) {
	paths, err := filepath.Glob(filepath.Join(root, "benchmarks", "manifests", "*.json"))
	if err != nil {
		return nil, err
	}
	var out []string
	for _, path := range paths {
		out = append(out, filepath.ToSlash(relativePath(root, path)))
	}
	sort.Strings(out)
	return out, nil
}

func auditCandidatePools(protocol CorpusProtocol) (map[string]CorpusCandidate, []CorpusCandidatePoolAudit, []ValidationError) {
	candidates := map[string]CorpusCandidate{}
	var audits []CorpusCandidatePoolAudit
	var errs []ValidationError
	for _, pool := range protocol.CandidatePools {
		audit := CorpusCandidatePoolAudit{ID: pool.ID, Hash: canonical.Hash(pool)}
		if pool.ID == "" {
			errs = append(errs, ValidationError{Message: "candidate pool missing id"})
			audit.OK = false
			audits = append(audits, audit)
			continue
		}
		poolOK := true
		if pool.Description == "" || pool.Source == "" {
			errs = append(errs, ValidationError{File: pool.ID, Message: "candidate pool missing description or source"})
			poolOK = false
		}
		for _, candidate := range pool.Candidates {
			key := candidateKey(candidate.Manifest, candidate.CaseID)
			switch candidate.Disposition {
			case "included":
				audit.Included++
				if candidate.CaseID == "" || candidate.Manifest == "" || candidate.Source == "" || candidate.Rationale == "" {
					errs = append(errs, ValidationError{File: pool.ID, CaseID: candidate.CaseID, Message: "included candidate missing case_id, manifest, source, or rationale"})
					poolOK = false
				}
				if _, exists := candidates[key]; exists {
					errs = append(errs, ValidationError{File: pool.ID, CaseID: candidate.CaseID, Message: "duplicate included candidate for manifest"})
					poolOK = false
				}
				candidates[key] = candidate
			case "excluded":
				audit.Excluded++
				if candidate.CaseID == "" || candidate.Source == "" || candidate.ExclusionReason == "" {
					errs = append(errs, ValidationError{File: pool.ID, CaseID: candidate.CaseID, Message: "excluded candidate missing case_id, source, or exclusion_reason"})
					poolOK = false
				}
			default:
				errs = append(errs, ValidationError{File: pool.ID, CaseID: candidate.CaseID, Message: "candidate disposition must be included or excluded"})
				poolOK = false
			}
		}
		audit.OK = poolOK
		audits = append(audits, audit)
	}
	return candidates, audits, errs
}

func auditCorpusManifest(root string, req CorpusManifestRequirement, candidates map[string]CorpusCandidate, seenIncluded map[string]bool, groundTruthByPath map[string]GroundTruthCase) (CorpusManifestAudit, error) {
	manifestPath := filepath.Join(root, filepath.FromSlash(req.Manifest))
	var manifest Manifest
	if err := readJSONFile(manifestPath, &manifest); err != nil {
		return CorpusManifestAudit{}, err
	}
	manifestDigest, err := digestFile("benchmark-manifest", root, manifestPath)
	if err != nil {
		return CorpusManifestAudit{}, err
	}
	audit := CorpusManifestAudit{
		DatasetID:      manifest.DatasetID,
		Manifest:       filepath.ToSlash(req.Manifest),
		ManifestHash:   manifestDigest.Hash,
		ResultCounts:   map[string]int{},
		CaseTypeCounts: map[string]int{},
		OK:             true,
	}
	var gtHashes []string
	fail := func() { audit.OK = false }
	if req.DatasetID != "" && manifest.DatasetID != req.DatasetID {
		fail()
	}
	if req.MinCases > 0 && len(manifest.Cases) < req.MinCases {
		fail()
	}
	if req.RequiresFetch != nil && manifest.RequiresFetch != *req.RequiresFetch {
		fail()
	}
	boundaryResults := setFromStrings(req.BoundaryResults)
	for _, c := range manifest.Cases {
		audit.Cases++
		audit.CaseTypeCounts[c.CaseType]++
		if strings.HasPrefix(c.Fixture, "inline:") {
			audit.InlineCases++
			if !req.AllowInline {
				fail()
			}
		}
		key := candidateKey(req.Manifest, c.CaseID)
		if _, ok := candidates[key]; !ok {
			fail()
		}
		seenIncluded[key] = true
		gtPath, err := filepath.Abs(filepath.Join(filepath.Dir(manifestPath), c.GroundTruth))
		if err != nil {
			return audit, err
		}
		gt, ok := groundTruthByPath[gtPath]
		if !ok {
			fail()
			continue
		}
		gtDigest, err := digestFile("ground-truth", root, gtPath)
		if err != nil {
			return audit, err
		}
		gtHashes = append(gtHashes, gtDigest.Hash)
		result := gt.Labels.ExpectedResult
		audit.ResultCounts[result]++
		if boundaryResults[result] {
			audit.BoundaryCases++
		}
		kinds := evidenceKindSet(gt.Evidence)
		if evidenceKindsSatisfied(kinds, req.RequiredEvidenceAny, req.RequiredEvidenceAll) {
			audit.PublicEvidenceCases++
		} else if len(req.RequiredEvidenceAny) > 0 || len(req.RequiredEvidenceAll) > 0 {
			fail()
		}
	}
	for result, min := range req.MinResults {
		if audit.ResultCounts[result] < min {
			fail()
		}
	}
	if audit.BoundaryCases < req.MinBoundaryCases {
		fail()
	}
	sort.Strings(gtHashes)
	audit.GroundTruthHash = canonical.Hash(gtHashes)
	return audit, nil
}

func auditReviewerCommands(root string, commands []CorpusReviewerCommand) ([]CorpusCommandAudit, []ValidationError) {
	makefile, err := os.ReadFile(filepath.Join(root, "Makefile"))
	if err != nil {
		return nil, []ValidationError{{File: "Makefile", Message: err.Error()}}
	}
	makefileText := string(makefile)
	var audits []CorpusCommandAudit
	var errs []ValidationError
	for _, command := range commands {
		audit := CorpusCommandAudit{
			ID:           command.ID,
			Command:      command.Command,
			ExpectedExit: command.ExpectedExit,
			Mode:         command.Mode,
			OK:           true,
		}
		fields := strings.Fields(command.Command)
		if command.ID == "" || command.Command == "" || command.Mode == "" {
			audit.OK = false
			errs = append(errs, ValidationError{CaseID: command.ID, Message: "reviewer command missing id, command, or mode"})
		}
		if len(fields) >= 2 && fields[0] == "make" {
			audit.Target = fields[1]
			if !strings.Contains(makefileText, "\n"+audit.Target+":") && !strings.HasPrefix(makefileText, audit.Target+":") {
				audit.OK = false
				errs = append(errs, ValidationError{File: "Makefile", CaseID: command.ID, Message: "reviewer command target missing: " + audit.Target})
			}
		}
		if command.Mode == "reviewer" && strings.Contains(command.Command, "refresh") {
			audit.OK = false
			errs = append(errs, ValidationError{CaseID: command.ID, Message: "reviewer command must not be a refresh target"})
		}
		audits = append(audits, audit)
	}
	return audits, errs
}

func candidateKey(manifest, caseID string) string {
	return filepath.ToSlash(manifest) + "\x00" + caseID
}

func setFromStrings(values []string) map[string]bool {
	out := map[string]bool{}
	for _, value := range values {
		out[value] = true
	}
	return out
}

func evidenceKindSet(evidence []Evidence) map[string]bool {
	out := map[string]bool{}
	for _, e := range evidence {
		out[e.Kind] = true
	}
	return out
}

func evidenceKindsSatisfied(kinds map[string]bool, anyOf, allOf []string) bool {
	for _, required := range allOf {
		if !kinds[required] {
			return false
		}
	}
	if len(anyOf) == 0 {
		return true
	}
	for _, required := range anyOf {
		if kinds[required] {
			return true
		}
	}
	return false
}

func corpusAuditHash(report CorpusAuditReport) string {
	copy := report
	copy.Hash = ""
	copy.Markdown = ""
	return canonical.Hash(copy)
}

func renderCorpusAuditMarkdown(report CorpusAuditReport) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Corpus audit\n\n")
	fmt.Fprintf(&b, "- ok: `%t`\n", report.OK)
	fmt.Fprintf(&b, "- hash: `%s`\n", report.Hash)
	fmt.Fprintf(&b, "- protocol: `%s`\n", report.Protocol)
	fmt.Fprintf(&b, "- protocol_hash: `%s`\n\n", report.ProtocolHash)
	fmt.Fprintf(&b, "## Manifests\n\n")
	fmt.Fprintf(&b, "| manifest | dataset | cases | boundary | inline | public-evidence | ok |\n")
	fmt.Fprintf(&b, "| --- | --- | ---: | ---: | ---: | ---: | ---: |\n")
	for _, manifest := range report.Manifests {
		fmt.Fprintf(&b, "| %s | %s | %d | %d | %d | %d | %t |\n", manifest.Manifest, manifest.DatasetID, manifest.Cases, manifest.BoundaryCases, manifest.InlineCases, manifest.PublicEvidenceCases, manifest.OK)
	}
	fmt.Fprintf(&b, "\n## Candidate pools\n\n")
	fmt.Fprintf(&b, "| pool | included | excluded | hash | ok |\n")
	fmt.Fprintf(&b, "| --- | ---: | ---: | --- | ---: |\n")
	for _, pool := range report.Pools {
		fmt.Fprintf(&b, "| %s | %d | %d | `%s` | %t |\n", pool.ID, pool.Included, pool.Excluded, pool.Hash, pool.OK)
	}
	fmt.Fprintf(&b, "\n## Reviewer commands\n\n")
	fmt.Fprintf(&b, "| command | mode | expected_exit | ok |\n")
	fmt.Fprintf(&b, "| --- | --- | ---: | ---: |\n")
	for _, command := range report.Commands {
		fmt.Fprintf(&b, "| `%s` | %s | %d | %t |\n", command.Command, command.Mode, command.ExpectedExit, command.OK)
	}
	if len(report.Errors) > 0 {
		fmt.Fprintf(&b, "\n## Errors\n\n")
		for _, err := range report.Errors {
			fmt.Fprintf(&b, "- %s %s %s\n", err.File, err.CaseID, err.Message)
		}
	}
	return b.String()
}
