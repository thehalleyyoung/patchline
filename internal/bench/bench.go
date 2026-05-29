package bench

import (
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"

	"github.com/thehalleyyoung/patchline/internal/canonical"
	"github.com/thehalleyyoung/patchline/internal/migration"
)

const Version = "patchline.benchmark-suite/v1"

type Spec struct {
	Version string `json:"version"`
	Name    string `json:"name"`
	Cases   []Case `json:"cases"`
}

type Case struct {
	ID                 string `json:"id"`
	Path               string `json:"path"`
	Label              string `json:"label"`
	ExpectedReportHash string `json:"expected_report_hash"`
	GroundTruth        string `json:"ground_truth,omitempty"`
	RepairManifest     string `json:"repair_manifest,omitempty"`
	Invariants         string `json:"invariants,omitempty"`
	Store              string `json:"store,omitempty"`
	ArchiveSpec        string `json:"archive_spec,omitempty"`
}

type Result struct {
	Name      string       `json:"name"`
	OK        bool         `json:"ok"`
	SuiteHash string       `json:"suite_hash"`
	Metrics   Metrics      `json:"metrics"`
	Cases     []CaseResult `json:"cases"`
}

type CaseResult struct {
	ID                 string `json:"id"`
	Path               string `json:"path"`
	Label              string `json:"label"`
	Prediction         string `json:"prediction"`
	OK                 bool   `json:"ok"`
	ReportHash         string `json:"report_hash"`
	ExpectedReportHash string `json:"expected_report_hash"`
	HighRiskStatements int    `json:"high_risk_statements"`
}

type Metrics struct {
	Total     int     `json:"total"`
	Passed    int     `json:"passed"`
	Failed    int     `json:"failed"`
	TruePos   int     `json:"true_positive"`
	TrueNeg   int     `json:"true_negative"`
	FalsePos  int     `json:"false_positive"`
	FalseNeg  int     `json:"false_negative"`
	Precision float64 `json:"precision"`
	Recall    float64 `json:"recall"`
}

func Read(reader io.Reader) (Spec, error) {
	decoder := json.NewDecoder(reader)
	decoder.DisallowUnknownFields()
	var spec Spec
	if err := decoder.Decode(&spec); err != nil {
		return Spec{}, err
	}
	if spec.Version != Version {
		return Spec{}, fmt.Errorf("benchmark suite version must be %s", Version)
	}
	if spec.Name == "" {
		return Spec{}, fmt.Errorf("benchmark suite name is required")
	}
	if len(spec.Cases) == 0 {
		return Spec{}, fmt.Errorf("benchmark suite needs at least one case")
	}
	for _, c := range spec.Cases {
		if c.ID == "" || c.Path == "" || (c.Label != "unsafe" && c.Label != "safe") {
			return Spec{}, fmt.Errorf("invalid benchmark case %q", c.ID)
		}
		if c.ExpectedReportHash == "" {
			return Spec{}, fmt.Errorf("benchmark case %q must pin expected_report_hash", c.ID)
		}
	}
	return spec, nil
}

func Run(spec Spec, baseDir string) (Result, error) {
	result := Result{Name: spec.Name, OK: true}
	for _, c := range spec.Cases {
		path := c.Path
		if !filepath.IsAbs(path) {
			path = filepath.Join(baseDir, path)
		}
		report, err := migration.AnalyzeFile(path)
		if err != nil {
			return Result{}, err
		}
		prediction := "safe"
		if report.Summary.HighRisk > 0 {
			prediction = "unsafe"
		}
		caseOK := prediction == c.Label && report.Summary.ReportHash == c.ExpectedReportHash
		if !caseOK {
			result.OK = false
		}
		cr := CaseResult{
			ID:                 c.ID,
			Path:               c.Path,
			Label:              c.Label,
			Prediction:         prediction,
			OK:                 caseOK,
			ReportHash:         report.Summary.ReportHash,
			ExpectedReportHash: c.ExpectedReportHash,
			HighRiskStatements: report.Summary.HighRisk,
		}
		result.Cases = append(result.Cases, cr)
	}
	result.Metrics = computeMetrics(result.Cases)
	result.SuiteHash = canonical.Hash(struct {
		Name    string       `json:"name"`
		Metrics Metrics      `json:"metrics"`
		Cases   []CaseResult `json:"cases"`
	}{Name: result.Name, Metrics: result.Metrics, Cases: result.Cases})
	return result, nil
}

func computeMetrics(cases []CaseResult) Metrics {
	var m Metrics
	m.Total = len(cases)
	for _, c := range cases {
		if c.OK {
			m.Passed++
		} else {
			m.Failed++
		}
		switch {
		case c.Label == "unsafe" && c.Prediction == "unsafe":
			m.TruePos++
		case c.Label == "safe" && c.Prediction == "safe":
			m.TrueNeg++
		case c.Label == "safe" && c.Prediction == "unsafe":
			m.FalsePos++
		case c.Label == "unsafe" && c.Prediction == "safe":
			m.FalseNeg++
		}
	}
	if m.TruePos+m.FalsePos > 0 {
		m.Precision = float64(m.TruePos) / float64(m.TruePos+m.FalsePos)
	}
	if m.TruePos+m.FalseNeg > 0 {
		m.Recall = float64(m.TruePos) / float64(m.TruePos+m.FalseNeg)
	}
	return m
}
