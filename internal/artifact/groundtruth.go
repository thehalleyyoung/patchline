package artifact

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type GroundTruthReport struct {
	Version          string            `json:"version"`
	Root             string            `json:"root"`
	GroundTruthFiles int               `json:"ground_truth_files"`
	Manifests        int               `json:"manifests"`
	PhaseCounts      map[string]int    `json:"phase_counts"`
	ResultCounts     map[string]int    `json:"result_counts"`
	Errors           []ValidationError `json:"errors,omitempty"`
	OK               bool              `json:"ok"`
}

type ValidationError struct {
	File    string `json:"file"`
	CaseID  string `json:"case_id,omitempty"`
	Message string `json:"message"`
}

type GroundTruthCase struct {
	CaseID         string           `json:"case_id"`
	CaseType       string           `json:"case_type"`
	Phase          string           `json:"phase"`
	Labels         GroundTruthLabel `json:"labels"`
	Evidence       []Evidence       `json:"evidence"`
	AllowedInputs  []string         `json:"allowed_inputs"`
	ExcludedInputs []string         `json:"excluded_inputs"`
}

type GroundTruthLabel struct {
	ExpectedResult string `json:"expected_result"`
	Risk           string `json:"risk"`
}

type Evidence struct {
	Kind      string `json:"kind"`
	Locator   string `json:"locator"`
	Rationale string `json:"rationale"`
}

type Manifest struct {
	Version     string         `json:"version"`
	DatasetID   string         `json:"dataset_id"`
	Description string         `json:"description"`
	Cases       []ManifestCase `json:"cases"`
}

type ManifestCase struct {
	CaseID      string `json:"case_id"`
	CaseType    string `json:"case_type"`
	AvailableAt string `json:"available_at"`
	Fixture     string `json:"fixture"`
	GroundTruth string `json:"ground_truth"`
}

func ValidateGroundTruth(root string) (GroundTruthReport, error) {
	report := GroundTruthReport{
		Version:      "patchline.artifact-ground-truth/v1",
		Root:         root,
		PhaseCounts:  map[string]int{},
		ResultCounts: map[string]int{},
	}
	groundTruthByPath := map[string]GroundTruthCase{}

	groundTruthDir := filepath.Join(root, "ground_truth")
	if err := filepath.WalkDir(groundTruthDir, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || filepath.Ext(path) != ".json" {
			return nil
		}
		report.GroundTruthFiles++
		var gt GroundTruthCase
		if err := readJSON(path, &gt); err != nil {
			report.Errors = append(report.Errors, ValidationError{File: path, Message: err.Error()})
			return nil
		}
		for _, validationErr := range validateGroundTruthCase(path, gt) {
			report.Errors = append(report.Errors, validationErr)
		}
		report.PhaseCounts[gt.Phase]++
		report.ResultCounts[gt.Labels.ExpectedResult]++
		cleanPath, _ := filepath.Abs(path)
		groundTruthByPath[cleanPath] = gt
		return nil
	}); err != nil {
		return report, err
	}

	manifestDir := filepath.Join(root, "manifests")
	if err := filepath.WalkDir(manifestDir, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || filepath.Ext(path) != ".json" {
			return nil
		}
		report.Manifests++
		var manifest Manifest
		if err := readJSON(path, &manifest); err != nil {
			report.Errors = append(report.Errors, ValidationError{File: path, Message: err.Error()})
			return nil
		}
		report.Errors = append(report.Errors, validateManifest(path, manifest, groundTruthByPath)...)
		return nil
	}); err != nil {
		return report, err
	}

	sortValidationErrors(report.Errors)
	report.OK = len(report.Errors) == 0
	return report, nil
}

func validateGroundTruthCase(path string, gt GroundTruthCase) []ValidationError {
	var errs []ValidationError
	require := func(ok bool, message string) {
		if !ok {
			errs = append(errs, ValidationError{File: path, CaseID: gt.CaseID, Message: message})
		}
	}

	require(gt.CaseID != "", "missing case_id")
	require(gt.CaseType != "", "missing case_type")
	require(gt.Phase != "", "missing phase")
	require(gt.Labels.ExpectedResult != "", "missing labels.expected_result")
	require(gt.Labels.Risk != "", "missing labels.risk")
	require(len(gt.Evidence) > 0, "missing evidence")
	require(gt.AllowedInputs != nil, "missing allowed_inputs")
	require(gt.ExcludedInputs != nil, "missing excluded_inputs")

	for i, evidence := range gt.Evidence {
		prefix := fmt.Sprintf("evidence[%d]", i)
		require(evidence.Kind != "", prefix+" missing kind")
		require(evidence.Locator != "", prefix+" missing locator")
		require(evidence.Rationale != "", prefix+" missing rationale")
	}

	if gt.Phase == "pre_deploy" {
		for _, evidence := range gt.Evidence {
			if evidence.Kind == "postmortem" {
				errs = append(errs, ValidationError{File: path, CaseID: gt.CaseID, Message: "pre_deploy case cites postmortem evidence"})
			}
		}
		if !contains(gt.ExcludedInputs, "postmortem_text") {
			errs = append(errs, ValidationError{File: path, CaseID: gt.CaseID, Message: "pre_deploy case must exclude postmortem_text"})
		}
	}

	return errs
}

func validateManifest(path string, manifest Manifest, groundTruthByPath map[string]GroundTruthCase) []ValidationError {
	var errs []ValidationError
	add := func(caseID, message string) {
		errs = append(errs, ValidationError{File: path, CaseID: caseID, Message: message})
	}
	if manifest.Version == "" {
		add("", "missing version")
	}
	if manifest.DatasetID == "" {
		add("", "missing dataset_id")
	}
	if len(manifest.Cases) == 0 {
		add("", "manifest has no cases")
	}
	for _, manifestCase := range manifest.Cases {
		if manifestCase.CaseID == "" {
			add("", "manifest case missing case_id")
			continue
		}
		if manifestCase.CaseType == "" {
			add(manifestCase.CaseID, "manifest case missing case_type")
		}
		if manifestCase.AvailableAt == "" {
			add(manifestCase.CaseID, "manifest case missing available_at")
		}
		if manifestCase.GroundTruth == "" {
			add(manifestCase.CaseID, "manifest case missing ground_truth")
			continue
		}
		gtPath, err := filepath.Abs(filepath.Join(filepath.Dir(path), manifestCase.GroundTruth))
		if err != nil {
			add(manifestCase.CaseID, err.Error())
			continue
		}
		gt, ok := groundTruthByPath[gtPath]
		if !ok {
			add(manifestCase.CaseID, "manifest references missing ground_truth file")
			continue
		}
		if gt.CaseID != manifestCase.CaseID {
			add(manifestCase.CaseID, "manifest case_id disagrees with ground_truth case_id")
		}
		if gt.CaseType != "" && manifestCase.CaseType != "" && gt.CaseType != manifestCase.CaseType {
			add(manifestCase.CaseID, "manifest case_type disagrees with ground_truth case_type")
		}
		if gt.Phase != "" && manifestCase.AvailableAt != "" && gt.Phase != manifestCase.AvailableAt {
			add(manifestCase.CaseID, "manifest available_at disagrees with ground_truth phase")
		}
		if manifestCase.Fixture != "" && !strings.HasPrefix(manifestCase.Fixture, "inline:") {
			fixturePath := filepath.Join(filepath.Dir(path), manifestCase.Fixture)
			if _, err := os.Stat(fixturePath); err != nil {
				add(manifestCase.CaseID, "manifest fixture does not exist: "+manifestCase.Fixture)
			}
		}
	}
	return errs
}

func readJSON(path string, out any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, out); err != nil {
		return err
	}
	return nil
}

func contains(values []string, needle string) bool {
	for _, value := range values {
		if value == needle {
			return true
		}
	}
	return false
}

func sortValidationErrors(errs []ValidationError) {
	sort.Slice(errs, func(i, j int) bool {
		if errs[i].File != errs[j].File {
			return errs[i].File < errs[j].File
		}
		if errs[i].CaseID != errs[j].CaseID {
			return errs[i].CaseID < errs[j].CaseID
		}
		return errs[i].Message < errs[j].Message
	})
}
