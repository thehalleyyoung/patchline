package certconformance

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/thehalleyyoung/patchline/internal/certlang"
)

const (
	CorpusVersion    = "patchline.certificate-conformance-corpus/v1"
	ReferenceVersion = "patchline.certificate-conformance.reference/v1"
	ReportVersion    = "patchline.certificate-conformance-corpus-results/v1"
)

type Corpus struct {
	Version       string        `json:"version"`
	Standard      string        `json:"standard"`
	StandardsBody StandardsBody `json:"standards_body"`
	PublicKey     string        `json:"public_key"`
	Cases         []Case        `json:"cases"`
}

type StandardsBody struct {
	Name         string `json:"name"`
	Track        string `json:"track"`
	Ratification string `json:"ratification"`
}

type Case struct {
	ID                    string `json:"id"`
	Clause                string `json:"clause"`
	Claim                 string `json:"claim"`
	Positive              string `json:"positive"`
	NegativeControl       string `json:"negative_control"`
	ReferenceOutput       string `json:"reference_output"`
	ExpectedVerdict       string `json:"expected_verdict"`
	ExpectedNegativeError string `json:"expected_negative_error"`
}

type ReferenceOutput struct {
	Version   string           `json:"version"`
	CaseID    string           `json:"case_id"`
	Payload   ReferencePayload `json:"payload"`
	Signature Signature        `json:"signature"`
}

type ReferencePayload struct {
	CaseID                string `json:"case_id"`
	Standard              string `json:"standard"`
	Checker               string `json:"checker"`
	Positive              string `json:"positive"`
	NegativeControl       string `json:"negative_control"`
	Accepted              bool   `json:"accepted"`
	NegativeRejected      bool   `json:"negative_rejected"`
	CertificateID         string `json:"certificate_id"`
	Verdict               string `json:"verdict"`
	RiskBPS               int    `json:"risk_bps"`
	CanonicalSHA256       string `json:"canonical_sha256"`
	PositiveSHA256        string `json:"positive_sha256"`
	NegativeSHA256        string `json:"negative_sha256"`
	NegativeErrorContains string `json:"negative_error_contains"`
}

type Signature struct {
	Algorithm string `json:"algorithm"`
	PublicKey string `json:"public_key"`
	Value     string `json:"value"`
}

type Report struct {
	Version            string       `json:"version"`
	Corpus             string       `json:"corpus"`
	Standard           string       `json:"standard"`
	StandardsBody      string       `json:"standards_body"`
	TotalCases         int          `json:"total_cases"`
	PositivesAccepted  int          `json:"positives_accepted"`
	NegativesRejected  int          `json:"negatives_rejected"`
	ReferencesVerified int          `json:"references_verified"`
	AllOK              bool         `json:"all_ok"`
	Cases              []CaseReport `json:"cases"`
}

type CaseReport struct {
	ID                string `json:"id"`
	CertificateID     string `json:"certificate_id,omitempty"`
	Verdict           string `json:"verdict,omitempty"`
	RiskBPS           int    `json:"risk_bps,omitempty"`
	PositiveAccepted  bool   `json:"positive_accepted"`
	NegativeRejected  bool   `json:"negative_rejected"`
	ReferenceVerified bool   `json:"reference_verified"`
	CanonicalSHA256   string `json:"canonical_sha256,omitempty"`
	NegativeError     string `json:"negative_error,omitempty"`
	OK                bool   `json:"ok"`
	Error             string `json:"error,omitempty"`
}

type signedReferenceBody struct {
	Version string           `json:"version"`
	CaseID  string           `json:"case_id"`
	Payload ReferencePayload `json:"payload"`
}

func Verify(corpusPath string, root string) (Report, error) {
	if root == "" {
		root = "."
	}
	corpus, err := loadCorpus(corpusPath)
	if err != nil {
		return Report{}, err
	}
	if err := validateCorpus(corpus); err != nil {
		return Report{}, err
	}
	publicKey, err := decodePublicKey(corpus.PublicKey)
	if err != nil {
		return Report{}, err
	}
	corpusDir := filepath.Dir(corpusPath)
	report := Report{
		Version:       ReportVersion,
		Corpus:        filepath.ToSlash(corpusPath),
		Standard:      corpus.Standard,
		StandardsBody: corpus.StandardsBody.Name,
		TotalCases:    len(corpus.Cases),
		AllOK:         true,
	}
	var failures []string
	seen := map[string]bool{}
	for _, tc := range corpus.Cases {
		caseReport, err := verifyCase(corpusDir, root, publicKey, corpus.Standard, tc)
		if seen[tc.ID] {
			err = errors.Join(err, fmt.Errorf("duplicate case id %q", tc.ID))
		}
		seen[tc.ID] = true
		if caseReport.ID == "" {
			caseReport.ID = tc.ID
		}
		if err != nil {
			caseReport.OK = false
			caseReport.Error = err.Error()
			failures = append(failures, fmt.Sprintf("%s: %s", tc.ID, err))
		}
		if caseReport.PositiveAccepted {
			report.PositivesAccepted++
		}
		if caseReport.NegativeRejected {
			report.NegativesRejected++
		}
		if caseReport.ReferenceVerified {
			report.ReferencesVerified++
		}
		report.AllOK = report.AllOK && caseReport.OK
		report.Cases = append(report.Cases, caseReport)
	}
	if len(failures) > 0 {
		return report, fmt.Errorf("certificate conformance corpus failed: %s", strings.Join(failures, "; "))
	}
	return report, nil
}

func loadCorpus(corpusPath string) (Corpus, error) {
	var corpus Corpus
	if err := readJSON(corpusPath, &corpus); err != nil {
		return corpus, err
	}
	return corpus, nil
}

func validateCorpus(corpus Corpus) error {
	switch {
	case corpus.Version != CorpusVersion:
		return fmt.Errorf("unexpected corpus version %q", corpus.Version)
	case corpus.Standard != certlang.Version:
		return fmt.Errorf("unexpected corpus standard %q", corpus.Standard)
	case corpus.StandardsBody.Name == "":
		return errors.New("standards_body.name is required")
	case corpus.StandardsBody.Track == "":
		return errors.New("standards_body.track is required")
	case corpus.StandardsBody.Ratification == "":
		return errors.New("standards_body.ratification is required")
	case len(corpus.Cases) == 0:
		return errors.New("corpus must contain at least one case")
	}
	return nil
}

func verifyCase(corpusDir string, root string, publicKey ed25519.PublicKey, standard string, tc Case) (CaseReport, error) {
	report := CaseReport{ID: tc.ID}
	var failures []string
	for name, value := range map[string]string{
		"id":                      tc.ID,
		"clause":                  tc.Clause,
		"claim":                   tc.Claim,
		"positive":                tc.Positive,
		"negative_control":        tc.NegativeControl,
		"reference_output":        tc.ReferenceOutput,
		"expected_verdict":        tc.ExpectedVerdict,
		"expected_negative_error": tc.ExpectedNegativeError,
	} {
		if value == "" {
			failures = append(failures, fmt.Sprintf("%s is required", name))
		}
	}
	if len(failures) > 0 {
		return report, errors.New(strings.Join(failures, "; "))
	}
	positivePath, err := resolveCorpusPath(corpusDir, tc.Positive)
	if err != nil {
		return report, fmt.Errorf("positive path: %w", err)
	}
	negativePath, err := resolveCorpusPath(corpusDir, tc.NegativeControl)
	if err != nil {
		return report, fmt.Errorf("negative path: %w", err)
	}
	referencePath, err := resolveCorpusPath(corpusDir, tc.ReferenceOutput)
	if err != nil {
		return report, fmt.Errorf("reference path: %w", err)
	}
	cert, err := certlang.CheckFile(positivePath, certlang.Options{Root: root, VerifyFiles: true})
	if err != nil {
		failures = append(failures, fmt.Sprintf("positive proof rejected: %s", err))
	} else {
		report.PositiveAccepted = true
		report.CertificateID = cert.CertificateID
		report.Verdict = cert.Verdict
		report.RiskBPS = cert.RiskBPS
		report.CanonicalSHA256 = cert.CanonicalHash
		if cert.Verdict != tc.ExpectedVerdict {
			failures = append(failures, fmt.Sprintf("positive verdict got %q want %q", cert.Verdict, tc.ExpectedVerdict))
		}
	}
	_, negativeErr := certlang.CheckFile(negativePath, certlang.Options{Root: root, VerifyFiles: true})
	if negativeErr == nil {
		failures = append(failures, "negative control was accepted")
	} else {
		report.NegativeRejected = true
		report.NegativeError = negativeErr.Error()
		if !strings.Contains(negativeErr.Error(), tc.ExpectedNegativeError) {
			failures = append(failures, fmt.Sprintf("negative error %q does not contain %q", negativeErr.Error(), tc.ExpectedNegativeError))
		}
	}
	var reference ReferenceOutput
	if err := readJSON(referencePath, &reference); err != nil {
		failures = append(failures, fmt.Sprintf("reference output: %s", err))
	} else if err := verifyReference(reference, publicKey); err != nil {
		failures = append(failures, fmt.Sprintf("reference signature: %s", err))
	} else if cert != nil && negativeErr != nil {
		expected := ReferencePayload{
			CaseID:                tc.ID,
			Standard:              standard,
			Checker:               "go-certlang",
			Positive:              tc.Positive,
			NegativeControl:       tc.NegativeControl,
			Accepted:              true,
			NegativeRejected:      true,
			CertificateID:         cert.CertificateID,
			Verdict:               cert.Verdict,
			RiskBPS:               cert.RiskBPS,
			CanonicalSHA256:       cert.CanonicalHash,
			PositiveSHA256:        sha256File(positivePath),
			NegativeSHA256:        sha256File(negativePath),
			NegativeErrorContains: tc.ExpectedNegativeError,
		}
		if reference.CaseID != tc.ID {
			failures = append(failures, fmt.Sprintf("reference case_id got %q want %q", reference.CaseID, tc.ID))
		}
		if reference.Payload != expected {
			failures = append(failures, "reference payload does not match checker output")
		} else {
			report.ReferenceVerified = true
		}
	}
	report.OK = len(failures) == 0 && report.PositiveAccepted && report.NegativeRejected && report.ReferenceVerified
	if len(failures) > 0 {
		return report, errors.New(strings.Join(failures, "; "))
	}
	return report, nil
}

func verifyReference(reference ReferenceOutput, publicKey ed25519.PublicKey) error {
	if reference.Version != ReferenceVersion {
		return fmt.Errorf("unexpected reference version %q", reference.Version)
	}
	if reference.Signature.Algorithm != "ed25519" {
		return fmt.Errorf("unexpected signature algorithm %q", reference.Signature.Algorithm)
	}
	if reference.Signature.PublicKey != hex.EncodeToString(publicKey) {
		return errors.New("reference public key does not match corpus public key")
	}
	signature, err := hex.DecodeString(reference.Signature.Value)
	if err != nil {
		return err
	}
	body := signedReferenceBody{
		Version: reference.Version,
		CaseID:  reference.CaseID,
		Payload: reference.Payload,
	}
	data, err := json.Marshal(body)
	if err != nil {
		return err
	}
	if !ed25519.Verify(publicKey, data, signature) {
		return errors.New("ed25519 signature verification failed")
	}
	return nil
}

func decodePublicKey(value string) (ed25519.PublicKey, error) {
	key, err := hex.DecodeString(value)
	if err != nil {
		return nil, err
	}
	if len(key) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("ed25519 public key length got %d want %d", len(key), ed25519.PublicKeySize)
	}
	return ed25519.PublicKey(key), nil
}

func readJSON(filePath string, out any) error {
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()
	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	return decoder.Decode(out)
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

func sha256File(filePath string) string {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
