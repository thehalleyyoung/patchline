package certlang

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

const LegacyVersion0 = "PLCI/0"

type MigrationResult struct {
	SourceVersion          string       `json:"source_version"`
	TargetVersion          string       `json:"target_version"`
	CertificateID          string       `json:"certificate_id"`
	Verdict                string       `json:"verdict"`
	RiskBPS                int          `json:"risk_bps"`
	RiskClass              string       `json:"risk_class,omitempty"`
	Changed                bool         `json:"changed"`
	SourceSHA256           string       `json:"source_sha256"`
	TargetSHA256           string       `json:"target_sha256"`
	SourceCanonicalSHA256  string       `json:"source_canonical_sha256"`
	TargetCanonicalSHA256  string       `json:"target_canonical_sha256"`
	PreservedObligations   int          `json:"preserved_obligations"`
	PreservedEvidenceItems int          `json:"preserved_evidence_items"`
	Notes                  []string     `json:"notes"`
	Certificate            *Certificate `json:"-"`
	Migrated               []byte       `json:"-"`
}

type MigrationVectorResult struct {
	Path                  string `json:"path"`
	Expected              string `json:"expected"`
	Accepted              bool   `json:"accepted"`
	OK                    bool   `json:"ok"`
	SourceVersion         string `json:"source_version,omitempty"`
	TargetVersion         string `json:"target_version,omitempty"`
	CertificateID         string `json:"certificate_id,omitempty"`
	Verdict               string `json:"verdict,omitempty"`
	RiskBPS               int    `json:"risk_bps,omitempty"`
	RiskClass             string `json:"risk_class,omitempty"`
	SourceSHA256          string `json:"source_sha256,omitempty"`
	TargetSHA256          string `json:"target_sha256,omitempty"`
	SourceCanonicalSHA256 string `json:"source_canonical_sha256,omitempty"`
	TargetCanonicalSHA256 string `json:"target_canonical_sha256,omitempty"`
	Error                 string `json:"error,omitempty"`
}

type MigrationReport struct {
	Checker            string                  `json:"checker"`
	Version            string                  `json:"version"`
	SourceVersion      string                  `json:"source_version"`
	TargetVersion      string                  `json:"target_version"`
	SpecDir            string                  `json:"spec_dir"`
	TotalLegacyValid   int                     `json:"total_legacy_valid"`
	TotalLegacyInvalid int                     `json:"total_legacy_invalid"`
	Migrated           int                     `json:"migrated"`
	Rejected           int                     `json:"rejected"`
	AllOK              bool                    `json:"all_ok"`
	Verdicts           map[string]int          `json:"verdicts"`
	Vectors            []MigrationVectorResult `json:"vectors"`
}

type legacyCertificateV0 struct {
	Certificate Certificate
	RiskClass   string
}

func MigrateToCurrent(data []byte, opts Options) (MigrationResult, error) {
	if len(data) == 0 {
		return MigrationResult{}, errors.New("empty certificate")
	}
	switch firstLine(data) {
	case Version:
		cert, err := Parse(data, opts)
		if err != nil {
			return MigrationResult{}, err
		}
		return MigrationResult{
			SourceVersion:          Version,
			TargetVersion:          Version,
			CertificateID:          cert.CertificateID,
			Verdict:                cert.Verdict,
			RiskBPS:                cert.RiskBPS,
			Changed:                false,
			SourceSHA256:           sha256Hex(data),
			TargetSHA256:           sha256Hex(data),
			SourceCanonicalSHA256:  cert.CanonicalHash,
			TargetCanonicalSHA256:  cert.CanonicalHash,
			PreservedObligations:   len(cert.Obligations),
			PreservedEvidenceItems: len(cert.Evidence),
			Notes:                  []string{"certificate already uses current PLCI/1 schema"},
			Certificate:            cert,
			Migrated:               append([]byte(nil), data...),
		}, nil
	case LegacyVersion0:
		legacy, err := parseLegacyV0(data, opts)
		if err != nil {
			return MigrationResult{}, err
		}
		cert := legacy.Certificate
		migrated, cert, err := renderCurrentCertificate(cert)
		if err != nil {
			return MigrationResult{}, err
		}
		if _, err := Parse(migrated, opts); err != nil {
			return MigrationResult{}, fmt.Errorf("migrated PLCI/1 certificate failed validation: %w", err)
		}
		return MigrationResult{
			SourceVersion:          LegacyVersion0,
			TargetVersion:          Version,
			CertificateID:          cert.CertificateID,
			Verdict:                cert.Verdict,
			RiskBPS:                cert.RiskBPS,
			RiskClass:              legacy.RiskClass,
			Changed:                true,
			SourceSHA256:           sha256Hex(data),
			TargetSHA256:           sha256Hex(migrated),
			SourceCanonicalSHA256:  legacy.Certificate.CanonicalHash,
			TargetCanonicalSHA256:  cert.CanonicalHash,
			PreservedObligations:   len(cert.Obligations),
			PreservedEvidenceItems: len(cert.Evidence),
			Notes:                  []string{"PLCI/0 risk-class migrated to PLCI/1 risk-bps", "verdict, evidence, obligations, and file digests rechecked under PLCI/1"},
			Certificate:            &cert,
			Migrated:               migrated,
		}, nil
	default:
		return MigrationResult{}, fmt.Errorf("unsupported certificate version %q", firstLine(data))
	}
}

func CheckMigrationDirectory(specDir string, opts Options) (MigrationReport, error) {
	report := MigrationReport{
		Checker:       "go",
		Version:       Version + "-migration-results",
		SourceVersion: LegacyVersion0,
		TargetVersion: Version,
		SpecDir:       filepath.ToSlash(specDir),
		AllOK:         true,
		Verdicts:      map[string]int{},
	}
	for _, group := range []struct {
		dir      string
		expected string
	}{
		{dir: "legacy-valid", expected: "legacy-valid"},
		{dir: "legacy-invalid", expected: "legacy-invalid"},
	} {
		paths, err := filepath.Glob(filepath.Join(specDir, "vectors", group.dir, "*.plci"))
		if err != nil {
			return report, err
		}
		sort.Strings(paths)
		for _, path := range paths {
			data, err := os.ReadFile(path)
			if err != nil {
				return report, err
			}
			result, migrateErr := MigrateToCurrent(data, opts)
			accepted := migrateErr == nil
			ok := (group.expected == "legacy-valid" && accepted && result.Changed && result.TargetVersion == Version) ||
				(group.expected == "legacy-invalid" && !accepted)
			rel, relErr := filepath.Rel(filepath.Join(specDir, "vectors"), path)
			if relErr != nil {
				rel = path
			}
			vector := MigrationVectorResult{
				Path:     filepath.ToSlash(rel),
				Expected: group.expected,
				Accepted: accepted,
				OK:       ok,
			}
			if migrateErr != nil {
				vector.Error = migrateErr.Error()
			} else {
				vector.SourceVersion = result.SourceVersion
				vector.TargetVersion = result.TargetVersion
				vector.CertificateID = result.CertificateID
				vector.Verdict = result.Verdict
				vector.RiskBPS = result.RiskBPS
				vector.RiskClass = result.RiskClass
				vector.SourceSHA256 = result.SourceSHA256
				vector.TargetSHA256 = result.TargetSHA256
				vector.SourceCanonicalSHA256 = result.SourceCanonicalSHA256
				vector.TargetCanonicalSHA256 = result.TargetCanonicalSHA256
			}
			report.Vectors = append(report.Vectors, vector)
			if group.expected == "legacy-valid" {
				report.TotalLegacyValid++
			} else {
				report.TotalLegacyInvalid++
			}
			if accepted {
				report.Migrated++
				report.Verdicts[result.Verdict]++
			} else {
				report.Rejected++
			}
			report.AllOK = report.AllOK && ok
		}
	}
	if report.TotalLegacyValid == 0 || report.TotalLegacyInvalid == 0 {
		report.AllOK = false
	}
	for _, verdict := range []string{"safe", "guarded", "blocked", "unsupported"} {
		if report.Verdicts[verdict] == 0 {
			report.AllOK = false
		}
	}
	return report, nil
}

func parseLegacyV0(data []byte, opts Options) (legacyCertificateV0, error) {
	if bytes.Contains(data, []byte("\r")) {
		return legacyCertificateV0{}, errors.New("PLCI certificates must use LF line endings")
	}
	if !bytes.HasSuffix(data, []byte("\n")) {
		return legacyCertificateV0{}, errors.New("PLCI certificates must end with LF")
	}
	lines := strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
	if len(lines) < 14 {
		return legacyCertificateV0{}, errors.New("certificate is shorter than the PLCI/0 grammar minimum")
	}
	i := 0
	expect := func(want string) error {
		if i >= len(lines) || lines[i] != want {
			return lineErr(i, "expected %q", want)
		}
		i++
		return nil
	}
	field := func(prefix string) (string, error) {
		if i >= len(lines) || !strings.HasPrefix(lines[i], prefix) {
			return "", lineErr(i, "expected %q field", strings.TrimSuffix(prefix, ": "))
		}
		value := strings.TrimPrefix(lines[i], prefix)
		if value == "" {
			return "", lineErr(i, "%s must not be empty", strings.TrimSuffix(prefix, ": "))
		}
		i++
		return value, nil
	}

	var cert Certificate
	if err := expect(LegacyVersion0); err != nil {
		return legacyCertificateV0{}, err
	}
	var err error
	if cert.CertificateID, err = field("certificate-id: "); err != nil {
		return legacyCertificateV0{}, err
	}
	if cert.SubjectRepo, err = field("subject-repo: "); err != nil {
		return legacyCertificateV0{}, err
	}
	if cert.SubjectRef, err = field("subject-ref: "); err != nil {
		return legacyCertificateV0{}, err
	}
	if cert.SubjectPath, err = field("subject-path: "); err != nil {
		return legacyCertificateV0{}, err
	}
	if cert.SubjectSHA256, err = field("subject-sha256: "); err != nil {
		return legacyCertificateV0{}, err
	}
	if cert.IssuedAt, err = field("issued-at: "); err != nil {
		return legacyCertificateV0{}, err
	}
	if cert.Producer, err = field("producer: "); err != nil {
		return legacyCertificateV0{}, err
	}
	if cert.Verdict, err = field("verdict: "); err != nil {
		return legacyCertificateV0{}, err
	}
	riskClass, err := field("risk-class: ")
	if err != nil {
		return legacyCertificateV0{}, err
	}
	cert.RiskBPS, err = legacyRiskClassBPS(riskClass)
	if err != nil {
		return legacyCertificateV0{}, lineErr(i-1, err.Error())
	}

	for i < len(lines) && strings.HasPrefix(lines[i], "evidence: ") {
		item, err := parseEvidenceLine(lines[i], i)
		if err != nil {
			return legacyCertificateV0{}, err
		}
		cert.Evidence = append(cert.Evidence, item)
		i++
	}
	if len(cert.Evidence) == 0 {
		return legacyCertificateV0{}, lineErr(i, "expected at least one evidence line")
	}
	for i < len(lines) && strings.HasPrefix(lines[i], "obligation: ") {
		item, err := parseObligationLine(lines[i], i)
		if err != nil {
			return legacyCertificateV0{}, err
		}
		cert.Obligations = append(cert.Obligations, item)
		i++
	}
	if len(cert.Obligations) == 0 {
		return legacyCertificateV0{}, lineErr(i, "expected at least one obligation line")
	}

	canonicalIndex := i
	if cert.CanonicalHash, err = field("canonical-sha256: "); err != nil {
		return legacyCertificateV0{}, err
	}
	if err := expect("END"); err != nil {
		return legacyCertificateV0{}, err
	}
	if i != len(lines) {
		return legacyCertificateV0{}, lineErr(i, "unexpected trailing line")
	}
	canonicalText := strings.Join(lines[:canonicalIndex], "\n") + "\n"
	if got := sha256Hex([]byte(canonicalText)); cert.CanonicalHash != got {
		return legacyCertificateV0{}, fmt.Errorf("canonical-sha256 mismatch: got %s want %s", cert.CanonicalHash, got)
	}
	if err := validateCertificate(cert, opts); err != nil {
		return legacyCertificateV0{}, err
	}
	return legacyCertificateV0{Certificate: cert, RiskClass: riskClass}, nil
}

func legacyRiskClassBPS(riskClass string) (int, error) {
	switch riskClass {
	case "low":
		return 40, nil
	case "medium":
		return 180, nil
	case "high":
		return 9000, nil
	case "unknown":
		return 10000, nil
	default:
		return 0, fmt.Errorf("invalid risk-class %q", riskClass)
	}
}

func renderCurrentCertificate(cert Certificate) ([]byte, Certificate, error) {
	lines := []string{
		Version,
		"certificate-id: " + cert.CertificateID,
		"subject-repo: " + cert.SubjectRepo,
		"subject-ref: " + cert.SubjectRef,
		"subject-path: " + cert.SubjectPath,
		"subject-sha256: " + cert.SubjectSHA256,
		"issued-at: " + cert.IssuedAt,
		"producer: " + cert.Producer,
		"verdict: " + cert.Verdict,
		"risk-bps: " + strconv.Itoa(cert.RiskBPS),
	}
	for _, evidence := range cert.Evidence {
		lines = append(lines, "evidence: "+evidence.ID+" type="+evidence.Type+" uri="+evidence.URI+" sha256="+evidence.SHA256)
	}
	for _, obligation := range cert.Obligations {
		lines = append(lines, fmt.Sprintf("obligation: %s kind=%s status=%s evidence=%s formula=%q",
			obligation.ID,
			obligation.Kind,
			obligation.Status,
			strings.Join(obligation.Evidence, ","),
			obligation.Formula,
		))
	}
	canonical := strings.Join(lines, "\n") + "\n"
	cert.CanonicalHash = sha256Hex([]byte(canonical))
	lines = append(lines, "canonical-sha256: "+cert.CanonicalHash, "END")
	data := []byte(strings.Join(lines, "\n") + "\n")
	return data, cert, nil
}

func firstLine(data []byte) string {
	text := string(data)
	line, _, _ := strings.Cut(text, "\n")
	return line
}
