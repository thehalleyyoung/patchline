package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/thehalleyyoung/patchline/internal/certconformance"
)

const dashboardVersion = "patchline.certificate-interop-dashboard/v1"
const minimizedWitnessVersion = "patchline.conformance-failure-minimizer/v1"

type checkerInput struct {
	Name string
	Path string
}

type checkerInputs []checkerInput

func (inputs *checkerInputs) String() string {
	values := make([]string, 0, len(*inputs))
	for _, input := range *inputs {
		values = append(values, input.Name+"="+input.Path)
	}
	return strings.Join(values, ",")
}

func (inputs *checkerInputs) Set(value string) error {
	name, reportPath, ok := strings.Cut(value, "=")
	if !ok || name == "" || reportPath == "" {
		return fmt.Errorf("checker must be name=report.json, got %q", value)
	}
	*inputs = append(*inputs, checkerInput{Name: name, Path: reportPath})
	return nil
}

type Dashboard struct {
	Version                  string           `json:"version"`
	Corpus                   string           `json:"corpus"`
	Standard                 string           `json:"standard"`
	StandardsBody            string           `json:"standards_body"`
	SignedReferencesVerified int              `json:"signed_references_verified"`
	TotalCases               int              `json:"total_cases"`
	TotalCheckers            int              `json:"total_checkers"`
	AllOK                    bool             `json:"all_ok"`
	DriftTotals              DriftTotals      `json:"drift_totals"`
	Checkers                 []CheckerSummary `json:"checkers"`
	Cases                    []CaseDashboard  `json:"cases"`
}

type DriftTotals struct {
	MissingPositiveVector int `json:"missing_positive_vector"`
	MissingNegativeVector int `json:"missing_negative_vector"`
	ExtraVector           int `json:"extra_vector"`
	MalformedVector       int `json:"malformed_vector"`
	PositiveAcceptance    int `json:"positive_acceptance"`
	NegativeRejection     int `json:"negative_rejection"`
	CertificateID         int `json:"certificate_id"`
	Verdict               int `json:"verdict"`
	RiskBPS               int `json:"risk_bps"`
	CanonicalSHA256       int `json:"canonical_sha256"`
	PositiveSHA256        int `json:"positive_sha256"`
	NegativeSHA256        int `json:"negative_sha256"`
	NegativeError         int `json:"negative_error"`
}

func (totals DriftTotals) sum() int {
	return totals.MissingPositiveVector +
		totals.MissingNegativeVector +
		totals.ExtraVector +
		totals.MalformedVector +
		totals.PositiveAcceptance +
		totals.NegativeRejection +
		totals.CertificateID +
		totals.Verdict +
		totals.RiskBPS +
		totals.CanonicalSHA256 +
		totals.PositiveSHA256 +
		totals.NegativeSHA256 +
		totals.NegativeError
}

func (totals *DriftTotals) add(kind string) {
	switch kind {
	case "missing_positive_vector":
		totals.MissingPositiveVector++
	case "missing_negative_vector":
		totals.MissingNegativeVector++
	case "extra_vector":
		totals.ExtraVector++
	case "malformed_vector":
		totals.MalformedVector++
	case "positive_acceptance":
		totals.PositiveAcceptance++
	case "negative_rejection":
		totals.NegativeRejection++
	case "certificate_id":
		totals.CertificateID++
	case "verdict":
		totals.Verdict++
	case "risk_bps":
		totals.RiskBPS++
	case "canonical_sha256":
		totals.CanonicalSHA256++
	case "positive_sha256":
		totals.PositiveSHA256++
	case "negative_sha256":
		totals.NegativeSHA256++
	case "negative_error":
		totals.NegativeError++
	default:
		panic("unknown drift kind: " + kind)
	}
}

func (totals *DriftTotals) merge(other DriftTotals) {
	totals.MissingPositiveVector += other.MissingPositiveVector
	totals.MissingNegativeVector += other.MissingNegativeVector
	totals.ExtraVector += other.ExtraVector
	totals.MalformedVector += other.MalformedVector
	totals.PositiveAcceptance += other.PositiveAcceptance
	totals.NegativeRejection += other.NegativeRejection
	totals.CertificateID += other.CertificateID
	totals.Verdict += other.Verdict
	totals.RiskBPS += other.RiskBPS
	totals.CanonicalSHA256 += other.CanonicalSHA256
	totals.PositiveSHA256 += other.PositiveSHA256
	totals.NegativeSHA256 += other.NegativeSHA256
	totals.NegativeError += other.NegativeError
}

type CheckerSummary struct {
	Name       string      `json:"name"`
	Report     string      `json:"report"`
	Checker    string      `json:"checker"`
	Version    string      `json:"version"`
	SpecDir    string      `json:"spec_dir"`
	ReportOK   bool        `json:"report_ok"`
	CasesOK    int         `json:"cases_ok"`
	TotalCases int         `json:"total_cases"`
	AllOK      bool        `json:"all_ok"`
	Drift      DriftTotals `json:"drift"`
}

type CaseDashboard struct {
	ID                    string              `json:"id"`
	ExpectedVerdict       string              `json:"expected_verdict"`
	ReferenceCertificate  string              `json:"reference_certificate_id"`
	ReferenceRiskBPS      int                 `json:"reference_risk_bps"`
	ReferenceCanonical    string              `json:"reference_canonical_sha256"`
	ExpectedNegativeError string              `json:"expected_negative_error"`
	Checkers              []CheckerCaseResult `json:"checkers"`
}

type CheckerCaseResult struct {
	Checker          string   `json:"checker"`
	PositivePath     string   `json:"positive_path,omitempty"`
	NegativePath     string   `json:"negative_path,omitempty"`
	PositiveAccepted bool     `json:"positive_accepted"`
	NegativeRejected bool     `json:"negative_rejected"`
	CertificateID    string   `json:"certificate_id,omitempty"`
	Verdict          string   `json:"verdict,omitempty"`
	RiskBPS          *int     `json:"risk_bps,omitempty"`
	CanonicalSHA256  string   `json:"canonical_sha256,omitempty"`
	NegativeError    string   `json:"negative_error,omitempty"`
	Drift            []string `json:"drift,omitempty"`
	OK               bool     `json:"ok"`
}

type checkerReport struct {
	Checker      string          `json:"checker"`
	Version      string          `json:"version"`
	SpecDir      string          `json:"spec_dir"`
	TotalValid   int             `json:"total_valid"`
	TotalInvalid int             `json:"total_invalid"`
	Accepted     int             `json:"accepted"`
	Rejected     int             `json:"rejected"`
	AllOK        bool            `json:"all_ok"`
	Vectors      []checkerVector `json:"vectors"`
}

type checkerVector struct {
	Path            string `json:"path"`
	Expected        string `json:"expected"`
	Accepted        bool   `json:"accepted"`
	OK              bool   `json:"ok"`
	CertificateID   string `json:"certificate_id"`
	Verdict         string `json:"verdict"`
	RiskBPS         *int   `json:"risk_bps"`
	CanonicalSHA256 string `json:"canonical_sha256"`
	Error           string `json:"error"`
}

type checkerCaseVectors struct {
	positive *checkerVector
	negative *checkerVector
}

type indexedCheckerVectors struct {
	byCase    map[string]checkerCaseVectors
	extras    []checkerVector
	malformed []checkerVector
}

type minimizedWitness struct {
	Version             string           `json:"version"`
	Status              string           `json:"status"`
	Corpus              string           `json:"corpus"`
	Standard            string           `json:"standard,omitempty"`
	Checker             string           `json:"checker,omitempty"`
	CaseID              string           `json:"case_id,omitempty"`
	DriftKind           string           `json:"drift_kind,omitempty"`
	VectorKind          string           `json:"vector_kind,omitempty"`
	VectorPath          string           `json:"vector_path,omitempty"`
	WitnessPath         string           `json:"witness_path,omitempty"`
	WitnessSHA256       string           `json:"witness_sha256,omitempty"`
	WitnessSource       string           `json:"witness_source,omitempty"`
	Reference           witnessReference `json:"reference,omitempty"`
	Observed            witnessObserved  `json:"observed,omitempty"`
	MinimizedUnits      []string         `json:"minimized_units,omitempty"`
	ReproductionCommand string           `json:"reproduction_command,omitempty"`
	SelectionOrder      []string         `json:"selection_order,omitempty"`
	AllOK               bool             `json:"all_ok"`
}

type witnessReference struct {
	CertificateID       string `json:"certificate_id,omitempty"`
	Verdict             string `json:"verdict,omitempty"`
	RiskBPS             int    `json:"risk_bps,omitempty"`
	CanonicalSHA256     string `json:"canonical_sha256,omitempty"`
	PositiveSHA256      string `json:"positive_sha256,omitempty"`
	NegativeSHA256      string `json:"negative_sha256,omitempty"`
	NegativeErrorSubstr string `json:"negative_error_substr,omitempty"`
	Accepted            *bool  `json:"accepted,omitempty"`
	Rejected            *bool  `json:"rejected,omitempty"`
}

type witnessObserved struct {
	CertificateID   string `json:"certificate_id,omitempty"`
	Verdict         string `json:"verdict,omitempty"`
	RiskBPS         *int   `json:"risk_bps,omitempty"`
	CanonicalSHA256 string `json:"canonical_sha256,omitempty"`
	VectorSHA256    string `json:"vector_sha256,omitempty"`
	Accepted        *bool  `json:"accepted,omitempty"`
	Rejected        *bool  `json:"rejected,omitempty"`
	Error           string `json:"error,omitempty"`
}

func main() {
	var inputs checkerInputs
	corpus := flag.String("corpus", "specs/certificate-conformance/v1/corpus.json", "frozen certificate conformance corpus")
	root := flag.String("root", ".", "repository root for file evidence verification")
	outJSON := flag.String("out-json", "", "write dashboard JSON to this path")
	outMD := flag.String("out-md", "", "write dashboard Markdown to this path")
	minimizeDir := flag.String("minimize-dir", "", "write the smallest certificate witness for the first checker disagreement")
	jsonStdout := flag.Bool("json", false, "write dashboard JSON to stdout")
	flag.Var(&inputs, "checker", "checker report as name=report.json; repeat for every checker")
	flag.Parse()

	if len(inputs) == 0 {
		fmt.Fprintln(os.Stderr, "at least one --checker name=report.json is required")
		os.Exit(2)
	}

	dashboard, err := buildDashboard(*corpus, *root, inputs)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if *outJSON != "" {
		if err := writeJSONFile(*outJSON, dashboard); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
	if *outMD != "" {
		if err := writeMarkdownFile(*outMD, dashboard); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
	if *minimizeDir != "" {
		witness, err := minimizeFailure(*corpus, dashboard, inputs, *minimizeDir)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		if err := writeWitness(*minimizeDir, witness); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
	if *jsonStdout || (*outJSON == "" && *outMD == "") {
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(dashboard); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
	if !dashboard.AllOK {
		os.Exit(1)
	}
}

func buildDashboard(corpusPath string, root string, inputs []checkerInput) (Dashboard, error) {
	conformanceReport, err := certconformance.Verify(corpusPath, root)
	if err != nil {
		return Dashboard{}, err
	}

	var corpus certconformance.Corpus
	if err := readJSON(corpusPath, &corpus); err != nil {
		return Dashboard{}, err
	}
	references, err := loadReferences(corpusPath, corpus)
	if err != nil {
		return Dashboard{}, err
	}

	dashboard := Dashboard{
		Version:                  dashboardVersion,
		Corpus:                   filepath.ToSlash(corpusPath),
		Standard:                 corpus.Standard,
		StandardsBody:            corpus.StandardsBody.Name,
		SignedReferencesVerified: conformanceReport.ReferencesVerified,
		TotalCases:               len(corpus.Cases),
		TotalCheckers:            len(inputs),
		AllOK:                    conformanceReport.AllOK,
	}

	checkerReports := map[string]checkerReport{}
	checkerVectors := map[string]map[string]checkerCaseVectors{}
	knownCases := map[string]bool{}
	for _, tc := range corpus.Cases {
		knownCases[tc.ID] = true
	}

	for _, input := range inputs {
		report, err := loadCheckerReport(input.Path)
		if err != nil {
			return dashboard, fmt.Errorf("%s: %w", input.Path, err)
		}
		name := input.Name
		if name == "" {
			name = report.Checker
		}
		checkerReports[name] = report
		indexed := indexVectors(report, knownCases)
		checkerVectors[name] = indexed.byCase

		summary := CheckerSummary{
			Name:       name,
			Report:     filepath.ToSlash(input.Path),
			Checker:    report.Checker,
			Version:    report.Version,
			SpecDir:    filepath.ToSlash(report.SpecDir),
			ReportOK:   report.AllOK,
			TotalCases: len(corpus.Cases),
		}
		for i := 0; i < len(indexed.malformed); i++ {
			summary.Drift.add("malformed_vector")
		}
		for i := 0; i < len(indexed.extras); i++ {
			summary.Drift.add("extra_vector")
		}
		dashboard.Checkers = append(dashboard.Checkers, summary)
	}

	for _, tc := range corpus.Cases {
		reference := references[tc.ID]
		caseRow := CaseDashboard{
			ID:                    tc.ID,
			ExpectedVerdict:       tc.ExpectedVerdict,
			ReferenceCertificate:  reference.Payload.CertificateID,
			ReferenceRiskBPS:      reference.Payload.RiskBPS,
			ReferenceCanonical:    reference.Payload.CanonicalSHA256,
			ExpectedNegativeError: reference.Payload.NegativeErrorContains,
		}
		for summaryIndex := range dashboard.Checkers {
			summary := &dashboard.Checkers[summaryIndex]
			report := checkerReports[summary.Name]
			vectors := checkerVectors[summary.Name][tc.ID]
			result := compareCase(summary.Name, report, vectors, reference)
			caseRow.Checkers = append(caseRow.Checkers, result)
			for _, drift := range result.Drift {
				summary.Drift.add(drift)
			}
			if result.OK {
				summary.CasesOK++
			}
		}
		dashboard.Cases = append(dashboard.Cases, caseRow)
	}

	for i := range dashboard.Checkers {
		summary := &dashboard.Checkers[i]
		summary.AllOK = summary.ReportOK && summary.Drift.sum() == 0 && summary.CasesOK == summary.TotalCases
		dashboard.DriftTotals.merge(summary.Drift)
		dashboard.AllOK = dashboard.AllOK && summary.AllOK
	}
	sort.Slice(dashboard.Checkers, func(i, j int) bool {
		return dashboard.Checkers[i].Name < dashboard.Checkers[j].Name
	})
	return dashboard, nil
}

type driftCandidate struct {
	checker    string
	reportPath string
	report     checkerReport
	caseID     string
	driftKind  string
	vectorKind string
	vector     *checkerVector
	reference  certconformance.ReferenceOutput
	corpusCase certconformance.Case
}

func minimizeFailure(corpusPath string, dashboard Dashboard, inputs []checkerInput, outDir string) (minimizedWitness, error) {
	witness := minimizedWitness{
		Version:        minimizedWitnessVersion,
		Status:         "no_failure",
		Corpus:         filepath.ToSlash(corpusPath),
		Standard:       dashboard.Standard,
		AllOK:          true,
		SelectionOrder: []string{"case_id", "checker", "drift_kind", "vector_kind", "vector_path"},
	}
	if dashboard.AllOK {
		return witness, nil
	}

	var corpus certconformance.Corpus
	if err := readJSON(corpusPath, &corpus); err != nil {
		return witness, err
	}
	references, err := loadReferences(corpusPath, corpus)
	if err != nil {
		return witness, err
	}
	knownCases := map[string]bool{}
	for _, tc := range corpus.Cases {
		knownCases[tc.ID] = true
	}

	var candidates []driftCandidate
	for _, input := range inputs {
		report, err := loadCheckerReport(input.Path)
		if err != nil {
			return witness, fmt.Errorf("%s: %w", input.Path, err)
		}
		name := input.Name
		if name == "" {
			name = report.Checker
		}
		indexed := indexVectors(report, knownCases)

		for i := range indexed.extras {
			vector := &indexed.extras[i]
			_, caseID, ok := parseVectorPath(vector.Path)
			if !ok {
				caseID = "unknown"
			}
			candidates = append(candidates, driftCandidate{
				checker:    name,
				reportPath: input.Path,
				report:     report,
				caseID:     caseID,
				driftKind:  "extra_vector",
				vectorKind: vectorKindFromPath(vector.Path),
				vector:     vector,
			})
		}
		for i := range indexed.malformed {
			vector := &indexed.malformed[i]
			candidates = append(candidates, driftCandidate{
				checker:    name,
				reportPath: input.Path,
				report:     report,
				caseID:     "malformed",
				driftKind:  "malformed_vector",
				vectorKind: "report",
				vector:     vector,
			})
		}

		for _, tc := range corpus.Cases {
			reference := references[tc.ID]
			vectors := indexed.byCase[tc.ID]
			result := compareCase(name, report, vectors, reference)
			for _, drift := range result.Drift {
				vectorKind := vectorKindForDrift(drift)
				var vector *checkerVector
				switch vectorKind {
				case "positive":
					vector = vectors.positive
				case "negative":
					vector = vectors.negative
				}
				candidates = append(candidates, driftCandidate{
					checker:    name,
					reportPath: input.Path,
					report:     report,
					caseID:     tc.ID,
					driftKind:  drift,
					vectorKind: vectorKind,
					vector:     vector,
					reference:  reference,
					corpusCase: tc,
				})
			}
		}
	}
	if len(candidates) == 0 {
		return witness, nil
	}
	sort.Slice(candidates, func(i, j int) bool {
		return candidateKey(candidates[i]) < candidateKey(candidates[j])
	})
	return materializeWitness(corpusPath, candidates[0], outDir, dashboard.Standard)
}

func candidateKey(candidate driftCandidate) string {
	vectorPath := ""
	if candidate.vector != nil {
		vectorPath = candidate.vector.Path
	}
	return strings.Join([]string{
		candidate.caseID,
		candidate.checker,
		candidate.driftKind,
		candidate.vectorKind,
		vectorPath,
	}, "\x00")
}

func materializeWitness(corpusPath string, candidate driftCandidate, outDir string, standard string) (minimizedWitness, error) {
	witness := minimizedWitness{
		Version:       minimizedWitnessVersion,
		Status:        "minimized",
		Corpus:        filepath.ToSlash(corpusPath),
		Standard:      standard,
		Checker:       candidate.checker,
		CaseID:        candidate.caseID,
		DriftKind:     candidate.driftKind,
		VectorKind:    candidate.vectorKind,
		WitnessSource: "checker-vector",
		AllOK:         false,
		MinimizedUnits: []string{
			"checker",
			"case",
			"vector",
			"certificate",
		},
		SelectionOrder:      []string{"case_id", "checker", "drift_kind", "vector_kind", "vector_path"},
		ReproductionCommand: fmt.Sprintf("go run ./tools/certinteropdashboard --corpus %s --checker %s=%s --minimize-dir %s", filepath.ToSlash(corpusPath), candidate.checker, filepath.ToSlash(candidate.reportPath), filepath.ToSlash(outDir)),
	}
	if candidate.vector != nil {
		witness.VectorPath = candidate.vector.Path
		witness.Observed = observedFromVector(candidate.report, *candidate.vector, candidate.vectorKind)
	}
	if candidate.reference.Version != "" {
		witness.Reference = referenceForVectorKind(candidate.reference, candidate.vectorKind)
	}

	sourcePath := ""
	switch {
	case candidate.vector != nil && candidate.vectorKind != "report":
		path, err := vectorFilePath(candidate.report.SpecDir, candidate.vector.Path)
		if err == nil {
			sourcePath = path
		}
	case candidate.vector == nil && (candidate.vectorKind == "positive" || candidate.vectorKind == "negative"):
		path, err := referenceCertificatePath(corpusPath, candidate.corpusCase, candidate.vectorKind)
		if err != nil {
			return witness, err
		}
		sourcePath = path
		witness.WitnessSource = "reference-corpus"
	}
	if sourcePath != "" {
		if err := os.MkdirAll(outDir, 0o755); err != nil {
			return witness, err
		}
		data, err := os.ReadFile(sourcePath)
		if err != nil {
			return witness, err
		}
		witnessPath := filepath.Join(outDir, "witness.plci")
		if err := os.WriteFile(witnessPath, data, 0o644); err != nil {
			return witness, err
		}
		sum := sha256.Sum256(data)
		witness.WitnessPath = "witness.plci"
		witness.WitnessSHA256 = hex.EncodeToString(sum[:])
	} else if candidate.vectorKind == "report" {
		witness.WitnessSource = "checker-report-row"
		witness.MinimizedUnits = []string{"checker", "report-vector"}
	}
	return witness, nil
}

func observedFromVector(report checkerReport, vector checkerVector, vectorKind string) witnessObserved {
	observed := witnessObserved{
		CertificateID:   vector.CertificateID,
		Verdict:         vector.Verdict,
		RiskBPS:         vector.RiskBPS,
		CanonicalSHA256: vector.CanonicalSHA256,
		Error:           vector.Error,
	}
	accepted := vector.Accepted
	rejected := !vector.Accepted
	switch vectorKind {
	case "positive":
		observed.Accepted = &accepted
	case "negative":
		observed.Rejected = &rejected
	}
	if vectorKind != "report" {
		if got, err := vectorSHA256(report.SpecDir, vector.Path); err == nil {
			observed.VectorSHA256 = got
		}
	}
	return observed
}

func referenceForVectorKind(reference certconformance.ReferenceOutput, vectorKind string) witnessReference {
	out := witnessReference{
		CertificateID:       reference.Payload.CertificateID,
		Verdict:             reference.Payload.Verdict,
		RiskBPS:             reference.Payload.RiskBPS,
		CanonicalSHA256:     reference.Payload.CanonicalSHA256,
		PositiveSHA256:      reference.Payload.PositiveSHA256,
		NegativeSHA256:      reference.Payload.NegativeSHA256,
		NegativeErrorSubstr: reference.Payload.NegativeErrorContains,
	}
	accepted := true
	rejected := true
	switch vectorKind {
	case "positive":
		out.Accepted = &accepted
	case "negative":
		out.Rejected = &rejected
	}
	return out
}

func vectorKindForDrift(drift string) string {
	switch drift {
	case "missing_positive_vector", "positive_acceptance", "certificate_id", "verdict", "risk_bps", "canonical_sha256", "positive_sha256":
		return "positive"
	case "missing_negative_vector", "negative_rejection", "negative_error", "negative_sha256":
		return "negative"
	default:
		return "report"
	}
}

func vectorKindFromPath(vectorPath string) string {
	group, _, ok := parseVectorPath(vectorPath)
	if !ok {
		return "report"
	}
	switch group {
	case "valid":
		return "positive"
	case "invalid":
		return "negative"
	default:
		return "report"
	}
}

func referenceCertificatePath(corpusPath string, tc certconformance.Case, vectorKind string) (string, error) {
	rel := tc.Positive
	if vectorKind == "negative" {
		rel = tc.NegativeControl
	}
	return resolveCorpusPath(filepath.Dir(corpusPath), rel)
}

func loadReferences(corpusPath string, corpus certconformance.Corpus) (map[string]certconformance.ReferenceOutput, error) {
	corpusDir := filepath.Dir(corpusPath)
	references := map[string]certconformance.ReferenceOutput{}
	for _, tc := range corpus.Cases {
		referencePath, err := resolveCorpusPath(corpusDir, tc.ReferenceOutput)
		if err != nil {
			return nil, fmt.Errorf("%s reference path: %w", tc.ID, err)
		}
		var reference certconformance.ReferenceOutput
		if err := readJSON(referencePath, &reference); err != nil {
			return nil, err
		}
		if reference.Payload.CaseID != tc.ID {
			return nil, fmt.Errorf("%s reference payload case_id got %q", tc.ID, reference.Payload.CaseID)
		}
		references[tc.ID] = reference
	}
	return references, nil
}

func loadCheckerReport(reportPath string) (checkerReport, error) {
	var report checkerReport
	if err := readJSON(reportPath, &report); err != nil {
		return report, err
	}
	if report.Checker == "" {
		return report, errors.New("checker report is missing checker")
	}
	if report.Version == "" {
		return report, errors.New("checker report is missing version")
	}
	if report.SpecDir == "" {
		return report, errors.New("checker report is missing spec_dir")
	}
	return report, nil
}

func indexVectors(report checkerReport, knownCases map[string]bool) indexedCheckerVectors {
	indexed := indexedCheckerVectors{
		byCase: map[string]checkerCaseVectors{},
	}
	for i := range report.Vectors {
		vector := &report.Vectors[i]
		group, caseID, ok := parseVectorPath(vector.Path)
		if !ok {
			indexed.malformed = append(indexed.malformed, *vector)
			continue
		}
		if !knownCases[caseID] {
			indexed.extras = append(indexed.extras, *vector)
			continue
		}
		row := indexed.byCase[caseID]
		switch group {
		case "valid":
			row.positive = vector
		case "invalid":
			row.negative = vector
		default:
			indexed.malformed = append(indexed.malformed, *vector)
			continue
		}
		indexed.byCase[caseID] = row
	}
	sort.Slice(indexed.extras, func(i, j int) bool { return indexed.extras[i].Path < indexed.extras[j].Path })
	sort.Slice(indexed.malformed, func(i, j int) bool { return indexed.malformed[i].Path < indexed.malformed[j].Path })
	return indexed
}

func compareCase(checker string, report checkerReport, vectors checkerCaseVectors, reference certconformance.ReferenceOutput) CheckerCaseResult {
	result := CheckerCaseResult{
		Checker: checker,
		OK:      true,
	}
	addDrift := func(kind string) {
		result.Drift = append(result.Drift, kind)
		result.OK = false
	}

	if vectors.positive == nil {
		addDrift("missing_positive_vector")
	} else {
		positive := vectors.positive
		result.PositivePath = positive.Path
		result.PositiveAccepted = positive.Accepted
		result.CertificateID = positive.CertificateID
		result.Verdict = positive.Verdict
		result.RiskBPS = positive.RiskBPS
		result.CanonicalSHA256 = positive.CanonicalSHA256
		if !positive.Accepted {
			addDrift("positive_acceptance")
		} else {
			if positive.CertificateID != reference.Payload.CertificateID {
				addDrift("certificate_id")
			}
			if positive.Verdict != reference.Payload.Verdict {
				addDrift("verdict")
			}
			if positive.RiskBPS == nil || *positive.RiskBPS != reference.Payload.RiskBPS {
				addDrift("risk_bps")
			}
			if positive.CanonicalSHA256 != reference.Payload.CanonicalSHA256 {
				addDrift("canonical_sha256")
			}
		}
		if got, err := vectorSHA256(report.SpecDir, positive.Path); err != nil || got != reference.Payload.PositiveSHA256 {
			addDrift("positive_sha256")
		}
	}

	if vectors.negative == nil {
		addDrift("missing_negative_vector")
	} else {
		negative := vectors.negative
		result.NegativePath = negative.Path
		result.NegativeRejected = !negative.Accepted
		result.NegativeError = negative.Error
		if negative.Accepted {
			addDrift("negative_rejection")
		} else if !strings.Contains(negative.Error, reference.Payload.NegativeErrorContains) {
			addDrift("negative_error")
		}
		if got, err := vectorSHA256(report.SpecDir, negative.Path); err != nil || got != reference.Payload.NegativeSHA256 {
			addDrift("negative_sha256")
		}
	}

	sort.Strings(result.Drift)
	return result
}

func parseVectorPath(value string) (group string, caseID string, ok bool) {
	clean := path.Clean(value)
	if clean != value || strings.HasPrefix(clean, "../") || clean == "." || strings.HasPrefix(clean, "/") {
		return "", "", false
	}
	parts := strings.Split(clean, "/")
	if len(parts) != 2 || !strings.HasSuffix(parts[1], ".plci") {
		return "", "", false
	}
	caseID = strings.TrimSuffix(parts[1], ".plci")
	if caseID == "" {
		return "", "", false
	}
	return parts[0], caseID, true
}

func vectorSHA256(specDir string, vectorPath string) (string, error) {
	filePath, err := vectorFilePath(specDir, vectorPath)
	if err != nil {
		return "", err
	}
	return sha256File(filePath)
}

func vectorFilePath(specDir string, vectorPath string) (string, error) {
	group, caseID, ok := parseVectorPath(vectorPath)
	if !ok {
		return "", fmt.Errorf("invalid vector path %q", vectorPath)
	}
	return filepath.Join(specDir, "vectors", filepath.FromSlash(group+"/"+caseID+".plci")), nil
}

func resolveCorpusPath(corpusDir string, rel string) (string, error) {
	if rel == "" || strings.Contains(rel, `\`) || path.IsAbs(rel) {
		return "", fmt.Errorf("invalid corpus-relative path %q", rel)
	}
	clean := path.Clean(rel)
	if clean == "." || clean != rel || strings.HasPrefix(clean, "../") || clean == ".." {
		return "", fmt.Errorf("invalid corpus-relative path %q", rel)
	}
	return filepath.Join(corpusDir, filepath.FromSlash(rel)), nil
}

func sha256File(filePath string) (string, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

func readJSON(filePath string, out any) error {
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()
	decoder := json.NewDecoder(file)
	return decoder.Decode(out)
}

func writeJSONFile(filePath string, dashboard Dashboard) error {
	if err := os.MkdirAll(filepath.Dir(filePath), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(dashboard, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(filePath, data, 0o644)
}

func writeWitness(outDir string, witness minimizedWitness) error {
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(witness, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if err := os.WriteFile(filepath.Join(outDir, "witness.json"), data, 0o644); err != nil {
		return err
	}
	file, err := os.Create(filepath.Join(outDir, "witness.md"))
	if err != nil {
		return err
	}
	defer file.Close()
	return writeWitnessMarkdown(file, witness)
}

func writeWitnessMarkdown(w io.Writer, witness minimizedWitness) error {
	if _, err := fmt.Fprintln(w, "# Conformance failure witness"); err != nil {
		return err
	}
	fmt.Fprintln(w)
	fmt.Fprintf(w, "Status: **%s**  \n", mdEscape(witness.Status))
	fmt.Fprintf(w, "Corpus: `%s`  \n", mdEscape(witness.Corpus))
	if witness.Status == "no_failure" {
		_, err := fmt.Fprintln(w, "No cross-implementation certificate disagreement was found.")
		return err
	}
	fmt.Fprintf(w, "Checker: `%s`  \n", mdEscape(witness.Checker))
	fmt.Fprintf(w, "Case: `%s`  \n", mdEscape(witness.CaseID))
	fmt.Fprintf(w, "Primary drift: `%s`  \n", mdEscape(witness.DriftKind))
	fmt.Fprintf(w, "Vector: `%s` `%s`  \n", mdEscape(witness.VectorKind), mdEscape(witness.VectorPath))
	fmt.Fprintf(w, "Witness: `%s` (`%s`)  \n\n", mdEscape(witness.WitnessPath), mdEscape(witness.WitnessSHA256))
	fmt.Fprintln(w, "## Delta")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "| Field | Reference | Observed |")
	fmt.Fprintln(w, "| --- | --- | --- |")
	for _, row := range witnessRows(witness) {
		fmt.Fprintf(w, "| `%s` | `%s` | `%s` |\n", mdEscape(row[0]), mdEscape(row[1]), mdEscape(row[2]))
	}
	if witness.ReproductionCommand != "" {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "## Reproduce")
		fmt.Fprintln(w)
		fmt.Fprintf(w, "```bash\n%s\n```\n", witness.ReproductionCommand)
	}
	return nil
}

func witnessRows(witness minimizedWitness) [][3]string {
	rows := [][3]string{
		{"certificate_id", witness.Reference.CertificateID, witness.Observed.CertificateID},
		{"verdict", witness.Reference.Verdict, witness.Observed.Verdict},
		{"risk_bps", referenceRiskString(witness.Reference), intPointerString(witness.Observed.RiskBPS)},
		{"canonical_sha256", witness.Reference.CanonicalSHA256, witness.Observed.CanonicalSHA256},
		{"negative_error", witness.Reference.NegativeErrorSubstr, witness.Observed.Error},
	}
	switch witness.VectorKind {
	case "positive":
		rows = append(rows, [3]string{"positive_sha256", witness.Reference.PositiveSHA256, witness.Observed.VectorSHA256})
	case "negative":
		rows = append(rows, [3]string{"negative_sha256", witness.Reference.NegativeSHA256, witness.Observed.VectorSHA256})
	}
	filtered := rows[:0]
	for _, row := range rows {
		if row[1] != "" || row[2] != "" {
			filtered = append(filtered, row)
		}
	}
	return filtered
}

func referenceRiskString(reference witnessReference) string {
	if reference.CertificateID == "" && reference.Verdict == "" && reference.CanonicalSHA256 == "" {
		return ""
	}
	return fmt.Sprint(reference.RiskBPS)
}

func intPointerString(value *int) string {
	if value == nil {
		return ""
	}
	return fmt.Sprint(*value)
}

func writeMarkdownFile(filePath string, dashboard Dashboard) error {
	if err := os.MkdirAll(filepath.Dir(filePath), 0o755); err != nil {
		return err
	}
	file, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer file.Close()
	return writeMarkdown(file, dashboard)
}

func writeMarkdown(w io.Writer, dashboard Dashboard) error {
	if _, err := fmt.Fprintf(w, "# Certificate interoperability dashboard\n\n"); err != nil {
		return err
	}
	fmt.Fprintf(w, "Corpus: `%s`  \n", mdEscape(dashboard.Corpus))
	fmt.Fprintf(w, "Standard: `%s`  \n", mdEscape(dashboard.Standard))
	fmt.Fprintf(w, "Signed references verified: **%d/%d**  \n", dashboard.SignedReferencesVerified, dashboard.TotalCases)
	fmt.Fprintf(w, "Checkers rerun: **%d**  \n", dashboard.TotalCheckers)
	fmt.Fprintf(w, "Dashboard verdict: **%s**\n\n", passFail(dashboard.AllOK))

	fmt.Fprintln(w, "## Checker drift")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "| Checker | Report OK | Cases OK | Drift count |")
	fmt.Fprintln(w, "| --- | ---: | ---: | ---: |")
	for _, checker := range dashboard.Checkers {
		fmt.Fprintf(w, "| `%s` | %t | %d/%d | %d |\n",
			mdEscape(checker.Name),
			checker.ReportOK,
			checker.CasesOK,
			checker.TotalCases,
			checker.Drift.sum(),
		)
	}

	fmt.Fprintln(w)
	fmt.Fprintln(w, "## Drift deltas")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "| Drift class | Count |")
	fmt.Fprintln(w, "| --- | ---: |")
	for _, row := range []struct {
		name  string
		count int
	}{
		{"missing_positive_vector", dashboard.DriftTotals.MissingPositiveVector},
		{"missing_negative_vector", dashboard.DriftTotals.MissingNegativeVector},
		{"extra_vector", dashboard.DriftTotals.ExtraVector},
		{"malformed_vector", dashboard.DriftTotals.MalformedVector},
		{"positive_acceptance", dashboard.DriftTotals.PositiveAcceptance},
		{"negative_rejection", dashboard.DriftTotals.NegativeRejection},
		{"certificate_id", dashboard.DriftTotals.CertificateID},
		{"verdict", dashboard.DriftTotals.Verdict},
		{"risk_bps", dashboard.DriftTotals.RiskBPS},
		{"canonical_sha256", dashboard.DriftTotals.CanonicalSHA256},
		{"positive_sha256", dashboard.DriftTotals.PositiveSHA256},
		{"negative_sha256", dashboard.DriftTotals.NegativeSHA256},
		{"negative_error", dashboard.DriftTotals.NegativeError},
	} {
		fmt.Fprintf(w, "| `%s` | %d |\n", row.name, row.count)
	}

	fmt.Fprintln(w)
	fmt.Fprintln(w, "## Case matrix")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "| Case | Expected verdict | Checker | Positive | Negative | Drift |")
	fmt.Fprintln(w, "| --- | --- | --- | ---: | ---: | --- |")
	for _, tc := range dashboard.Cases {
		for _, checker := range tc.Checkers {
			drift := "none"
			if len(checker.Drift) > 0 {
				drift = strings.Join(checker.Drift, ", ")
			}
			fmt.Fprintf(w, "| `%s` | `%s` | `%s` | %t | %t | %s |\n",
				mdEscape(tc.ID),
				mdEscape(tc.ExpectedVerdict),
				mdEscape(checker.Checker),
				checker.PositiveAccepted,
				checker.NegativeRejected,
				mdEscape(drift),
			)
		}
	}
	return nil
}

func passFail(ok bool) string {
	if ok {
		return "pass"
	}
	return "fail"
}

func mdEscape(value string) string {
	return strings.ReplaceAll(value, "|", "\\|")
}
