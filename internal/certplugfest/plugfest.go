package certplugfest

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/thehalleyyoung/patchline/internal/canonical"
	"github.com/thehalleyyoung/patchline/internal/certconformance"
	"github.com/thehalleyyoung/patchline/internal/certdiff"
	"github.com/thehalleyyoung/patchline/internal/certlang"
	"github.com/thehalleyyoung/patchline/internal/certrevocation"
	"github.com/thehalleyyoung/patchline/internal/ledger"
)

const (
	SubmissionVersion = "patchline.certificate-plugfest-submission/v1"
	ReportVersion     = "patchline.certificate-plugfest-report/v1"
)

type Submission struct {
	Version                 string                            `json:"version"`
	ToolName                string                            `json:"tool_name"`
	ToolVersion             string                            `json:"tool_version"`
	Offline                 bool                              `json:"offline"`
	CorpusPath              string                            `json:"corpus_path"`
	CorpusSHA256            string                            `json:"corpus_sha256"`
	ConformanceReportHash   string                            `json:"conformance_report_hash"`
	NormalizeCertificate    string                            `json:"normalize_certificate"`
	NormalizedCanonicalHash string                            `json:"normalized_canonical_sha256"`
	DiffOldCertificate      string                            `json:"diff_old_certificate"`
	DiffNewCertificate      string                            `json:"diff_new_certificate"`
	DiffSummary             map[string]int                    `json:"diff_summary"`
	KnownCertificates       []certrevocation.KnownCertificate `json:"known_certificates"`
	RevocationLedger        []ledger.Entry                    `json:"revocation_ledger"`
	RevocationRecords       []certrevocation.Record           `json:"revocation_records"`
	Logs                    []Log                             `json:"logs"`
}

type Log struct {
	Command      string `json:"command"`
	ExitCode     int    `json:"exit_code"`
	StdoutSHA256 string `json:"stdout_sha256"`
	StderrSHA256 string `json:"stderr_sha256"`
}

type Report struct {
	Version               string            `json:"version"`
	ToolName              string            `json:"tool_name"`
	ToolVersion           string            `json:"tool_version"`
	Offline               bool              `json:"offline"`
	CorpusVerified        bool              `json:"corpus_verified"`
	ConformanceVerified   bool              `json:"conformance_verified"`
	NormalizationVerified bool              `json:"normalization_verified"`
	DiffVerified          bool              `json:"diff_verified"`
	RevocationVerified    bool              `json:"revocation_verified"`
	LogsVerified          bool              `json:"logs_verified"`
	AllOK                 bool              `json:"all_ok"`
	Hashes                map[string]string `json:"hashes"`
	Errors                []string          `json:"errors,omitempty"`
}

func ValidateFile(path string, root string) (Report, error) {
	var submission Submission
	file, err := os.Open(path)
	if err != nil {
		return Report{}, err
	}
	defer file.Close()
	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&submission); err != nil {
		return Report{}, err
	}
	return Validate(submission, root)
}

func Validate(submission Submission, root string) (Report, error) {
	if root == "" {
		root = "."
	}
	report := Report{
		Version:     ReportVersion,
		ToolName:    submission.ToolName,
		ToolVersion: submission.ToolVersion,
		Offline:     submission.Offline,
		Hashes:      map[string]string{},
	}
	var failures []string
	if err := validateSubmissionFields(submission); err != nil {
		failures = append(failures, err.Error())
	}

	corpusPath := resolve(root, submission.CorpusPath)
	if hash, err := fileSHA256(corpusPath); err != nil {
		failures = append(failures, fmt.Sprintf("corpus hash: %s", err))
	} else {
		report.Hashes["corpus_sha256"] = hash
		report.CorpusVerified = hash == submission.CorpusSHA256
		if !report.CorpusVerified {
			failures = append(failures, fmt.Sprintf("corpus sha256 got %s want %s", hash, submission.CorpusSHA256))
		}
	}

	if conformance, err := certconformance.Verify(corpusPath, root); err != nil {
		failures = append(failures, fmt.Sprintf("conformance: %s", err))
	} else {
		hash := canonical.Hash(conformance)
		report.Hashes["conformance_report_hash"] = hash
		report.ConformanceVerified = conformance.AllOK && hash == submission.ConformanceReportHash
		if !report.ConformanceVerified {
			failures = append(failures, fmt.Sprintf("conformance hash/all_ok mismatch got hash=%s all_ok=%t", hash, conformance.AllOK))
		}
	}

	if normalized, err := normalizePath(root, submission.NormalizeCertificate); err != nil {
		failures = append(failures, fmt.Sprintf("normalization: %s", err))
	} else {
		report.Hashes["normalized_canonical_sha256"] = normalized
		report.NormalizationVerified = normalized == submission.NormalizedCanonicalHash
		if !report.NormalizationVerified {
			failures = append(failures, fmt.Sprintf("normalized canonical got %s want %s", normalized, submission.NormalizedCanonicalHash))
		}
	}

	if diff, err := diffPaths(root, submission.DiffOldCertificate, submission.DiffNewCertificate); err != nil {
		failures = append(failures, fmt.Sprintf("semantic diff: %s", err))
	} else {
		report.Hashes["diff_report_hash"] = canonical.Hash(diff)
		report.DiffVerified = equalSummary(diff.Summary, submission.DiffSummary)
		if !report.DiffVerified {
			failures = append(failures, fmt.Sprintf("diff summary got %v want %v", diff.Summary, submission.DiffSummary))
		}
	}

	if replay, err := certrevocation.Replay(submission.RevocationLedger, submission.RevocationRecords, submission.KnownCertificates); err != nil {
		failures = append(failures, fmt.Sprintf("revocation replay: %s", err))
	} else {
		report.Hashes["revocation_replay_hash"] = canonical.Hash(replay)
		report.RevocationVerified = replay.AllOK
		if !report.RevocationVerified {
			failures = append(failures, "revocation replay did not report all_ok")
		}
	}

	if err := verifyLogs(submission.Logs); err != nil {
		failures = append(failures, err.Error())
	} else {
		report.LogsVerified = true
		report.Hashes["logs_hash"] = canonical.Hash(submission.Logs)
	}

	report.AllOK = len(failures) == 0 &&
		report.Offline &&
		report.CorpusVerified &&
		report.ConformanceVerified &&
		report.NormalizationVerified &&
		report.DiffVerified &&
		report.RevocationVerified &&
		report.LogsVerified
	report.Errors = failures
	if len(failures) > 0 {
		return report, fmt.Errorf("certificate plugfest submission failed: %s", strings.Join(failures, "; "))
	}
	return report, nil
}

func validateSubmissionFields(submission Submission) error {
	var failures []string
	if submission.Version != SubmissionVersion {
		failures = append(failures, fmt.Sprintf("version must be %s", SubmissionVersion))
	}
	if submission.ToolName == "" || submission.ToolVersion == "" {
		failures = append(failures, "tool_name and tool_version are required")
	}
	if !submission.Offline {
		failures = append(failures, "offline must be true")
	}
	for name, value := range map[string]string{
		"corpus_path":                 submission.CorpusPath,
		"corpus_sha256":               submission.CorpusSHA256,
		"conformance_report_hash":     submission.ConformanceReportHash,
		"normalize_certificate":       submission.NormalizeCertificate,
		"normalized_canonical_sha256": submission.NormalizedCanonicalHash,
		"diff_old_certificate":        submission.DiffOldCertificate,
		"diff_new_certificate":        submission.DiffNewCertificate,
	} {
		if value == "" {
			failures = append(failures, name+" is required")
		}
	}
	if len(submission.DiffSummary) == 0 {
		failures = append(failures, "diff_summary is required")
	}
	if len(submission.KnownCertificates) == 0 || len(submission.RevocationRecords) == 0 || len(submission.RevocationLedger) == 0 {
		failures = append(failures, "revocation known certificates, records, and ledger are required")
	}
	if len(submission.Logs) == 0 {
		failures = append(failures, "at least one reproducible log is required")
	}
	if len(failures) > 0 {
		return errors.New(strings.Join(failures, "; "))
	}
	return nil
}

func normalizePath(root string, path string) (string, error) {
	data, err := os.ReadFile(resolve(root, path))
	if err != nil {
		return "", err
	}
	normalized, err := certlang.Normalize(data, certlang.Options{Root: root, VerifyFiles: true})
	if err != nil {
		return "", err
	}
	return normalized.NormalizedCanonicalSHA256, nil
}

func diffPaths(root string, oldPath string, newPath string) (certdiff.Report, error) {
	oldData, err := os.ReadFile(resolve(root, oldPath))
	if err != nil {
		return certdiff.Report{}, err
	}
	newData, err := os.ReadFile(resolve(root, newPath))
	if err != nil {
		return certdiff.Report{}, err
	}
	return certdiff.CompareBytes(oldData, newData, certlang.Options{Root: root, VerifyFiles: true})
}

func verifyLogs(logs []Log) error {
	for i, log := range logs {
		if log.Command == "" {
			return fmt.Errorf("log %d command is required", i)
		}
		lower := strings.ToLower(log.Command)
		for _, forbidden := range []string{"curl ", "wget ", "http://", "https://", "go get ", "go install "} {
			if strings.Contains(lower, forbidden) {
				return fmt.Errorf("log %d command is not offline-safe: %s", i, log.Command)
			}
		}
		if log.ExitCode != 0 {
			return fmt.Errorf("log %d exit code got %d want 0", i, log.ExitCode)
		}
		if !validSHA256(log.StdoutSHA256) || !validSHA256(log.StderrSHA256) {
			return fmt.Errorf("log %d stdout/stderr hashes must be sha256 hex", i)
		}
	}
	return nil
}

func equalSummary(left, right map[string]int) bool {
	keys := make([]string, 0, len(left)+len(right))
	seen := map[string]bool{}
	for key := range left {
		keys = append(keys, key)
		seen[key] = true
	}
	for key := range right {
		if !seen[key] {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	for _, key := range keys {
		if left[key] != right[key] {
			return false
		}
	}
	return true
}

func resolve(root string, value string) string {
	if filepath.IsAbs(value) {
		return filepath.Clean(value)
	}
	return filepath.Join(root, filepath.FromSlash(value))
}

func fileSHA256(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

func validSHA256(value string) bool {
	if len(value) != 64 {
		return false
	}
	for _, r := range value {
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f')) {
			return false
		}
	}
	return true
}
