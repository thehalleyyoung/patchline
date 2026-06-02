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

func main() {
	var inputs checkerInputs
	corpus := flag.String("corpus", "specs/certificate-conformance/v1/corpus.json", "frozen certificate conformance corpus")
	root := flag.String("root", ".", "repository root for file evidence verification")
	outJSON := flag.String("out-json", "", "write dashboard JSON to this path")
	outMD := flag.String("out-md", "", "write dashboard Markdown to this path")
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
		caseVectors, malformed, extra := indexVectors(report, knownCases)
		checkerVectors[name] = caseVectors

		summary := CheckerSummary{
			Name:       name,
			Report:     filepath.ToSlash(input.Path),
			Checker:    report.Checker,
			Version:    report.Version,
			SpecDir:    filepath.ToSlash(report.SpecDir),
			ReportOK:   report.AllOK,
			TotalCases: len(corpus.Cases),
		}
		for i := 0; i < malformed; i++ {
			summary.Drift.add("malformed_vector")
		}
		for i := 0; i < extra; i++ {
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

func indexVectors(report checkerReport, knownCases map[string]bool) (map[string]checkerCaseVectors, int, int) {
	byCase := map[string]checkerCaseVectors{}
	malformed := 0
	extra := 0
	for i := range report.Vectors {
		vector := &report.Vectors[i]
		group, caseID, ok := parseVectorPath(vector.Path)
		if !ok {
			malformed++
			continue
		}
		if !knownCases[caseID] {
			extra++
			continue
		}
		row := byCase[caseID]
		switch group {
		case "valid":
			row.positive = vector
		case "invalid":
			row.negative = vector
		default:
			malformed++
			continue
		}
		byCase[caseID] = row
	}
	return byCase, malformed, extra
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
	group, caseID, ok := parseVectorPath(vectorPath)
	if !ok {
		return "", fmt.Errorf("invalid vector path %q", vectorPath)
	}
	filePath := filepath.Join(specDir, "vectors", filepath.FromSlash(group+"/"+caseID+".plci"))
	return sha256File(filePath)
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
