package certlang

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

const Version = "PLCI/1"

var (
	idRE     = regexp.MustCompile(`^[a-z][a-z0-9.-]{2,80}$`)
	issuedRE = regexp.MustCompile(`^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}Z$`)
	refRE    = regexp.MustCompile(`^[A-Za-z0-9._/-]{1,80}$`)
	repoRE   = regexp.MustCompile(`^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$`)
	riskRE   = regexp.MustCompile(`^(0|10000|[1-9][0-9]{0,3})$`)
)

type Certificate struct {
	CertificateID string       `json:"certificate_id"`
	SubjectRepo   string       `json:"subject_repo"`
	SubjectRef    string       `json:"subject_ref"`
	SubjectPath   string       `json:"subject_path"`
	SubjectSHA256 string       `json:"subject_sha256"`
	IssuedAt      string       `json:"issued_at"`
	Producer      string       `json:"producer"`
	Verdict       string       `json:"verdict"`
	RiskBPS       int          `json:"risk_bps"`
	Evidence      []Evidence   `json:"evidence"`
	Obligations   []Obligation `json:"obligations"`
	CanonicalHash string       `json:"canonical_sha256"`
}

type Evidence struct {
	ID     string `json:"id"`
	Type   string `json:"type"`
	URI    string `json:"uri"`
	SHA256 string `json:"sha256"`
}

type Obligation struct {
	ID       string   `json:"id"`
	Kind     string   `json:"kind"`
	Status   string   `json:"status"`
	Evidence []string `json:"evidence"`
	Formula  string   `json:"formula"`
}

type Options struct {
	Root        string
	VerifyFiles bool
}

type VectorResult struct {
	Path            string `json:"path"`
	Expected        string `json:"expected"`
	Accepted        bool   `json:"accepted"`
	OK              bool   `json:"ok"`
	CertificateID   string `json:"certificate_id,omitempty"`
	Verdict         string `json:"verdict,omitempty"`
	RiskBPS         *int   `json:"risk_bps,omitempty"`
	CanonicalSHA256 string `json:"canonical_sha256,omitempty"`
	Error           string `json:"error,omitempty"`
}

type DirectoryReport struct {
	Checker      string         `json:"checker"`
	Version      string         `json:"version"`
	SpecDir      string         `json:"spec_dir"`
	TotalValid   int            `json:"total_valid"`
	TotalInvalid int            `json:"total_invalid"`
	Accepted     int            `json:"accepted"`
	Rejected     int            `json:"rejected"`
	AllOK        bool           `json:"all_ok"`
	Vectors      []VectorResult `json:"vectors"`
}

func CheckFile(path string, opts Options) (*Certificate, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return Parse(data, opts)
}

func Parse(data []byte, opts Options) (*Certificate, error) {
	if len(data) == 0 {
		return nil, errors.New("empty certificate")
	}
	if bytes.Contains(data, []byte("\r")) {
		return nil, errors.New("PLCI certificates must use LF line endings")
	}
	if !bytes.HasSuffix(data, []byte("\n")) {
		return nil, errors.New("PLCI certificates must end with LF")
	}
	lines := strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
	if len(lines) < 14 {
		return nil, errors.New("certificate is shorter than the PLCI/1 grammar minimum")
	}

	var cert Certificate
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

	if err := expect(Version); err != nil {
		return nil, err
	}
	var err error
	if cert.CertificateID, err = field("certificate-id: "); err != nil {
		return nil, err
	}
	if cert.SubjectRepo, err = field("subject-repo: "); err != nil {
		return nil, err
	}
	if cert.SubjectRef, err = field("subject-ref: "); err != nil {
		return nil, err
	}
	if cert.SubjectPath, err = field("subject-path: "); err != nil {
		return nil, err
	}
	if cert.SubjectSHA256, err = field("subject-sha256: "); err != nil {
		return nil, err
	}
	if cert.IssuedAt, err = field("issued-at: "); err != nil {
		return nil, err
	}
	if cert.Producer, err = field("producer: "); err != nil {
		return nil, err
	}
	if cert.Verdict, err = field("verdict: "); err != nil {
		return nil, err
	}
	risk, err := field("risk-bps: ")
	if err != nil {
		return nil, err
	}
	if !riskRE.MatchString(risk) {
		return nil, lineErr(i-1, "risk-bps must be 0..10000 without leading zeros")
	}
	cert.RiskBPS, err = strconv.Atoi(risk)
	if err != nil {
		return nil, lineErr(i-1, "risk-bps must be an integer")
	}

	for i < len(lines) && strings.HasPrefix(lines[i], "evidence: ") {
		item, err := parseEvidenceLine(lines[i], i)
		if err != nil {
			return nil, err
		}
		cert.Evidence = append(cert.Evidence, item)
		i++
	}
	if len(cert.Evidence) == 0 {
		return nil, lineErr(i, "expected at least one evidence line")
	}
	for i < len(lines) && strings.HasPrefix(lines[i], "obligation: ") {
		item, err := parseObligationLine(lines[i], i)
		if err != nil {
			return nil, err
		}
		cert.Obligations = append(cert.Obligations, item)
		i++
	}
	if len(cert.Obligations) == 0 {
		return nil, lineErr(i, "expected at least one obligation line")
	}

	canonicalIndex := i
	if cert.CanonicalHash, err = field("canonical-sha256: "); err != nil {
		return nil, err
	}
	if err := expect("END"); err != nil {
		return nil, err
	}
	if i != len(lines) {
		return nil, lineErr(i, "unexpected trailing line")
	}

	canonicalText := strings.Join(lines[:canonicalIndex], "\n") + "\n"
	if got := sha256Hex([]byte(canonicalText)); cert.CanonicalHash != got {
		return nil, fmt.Errorf("canonical-sha256 mismatch: got %s want %s", cert.CanonicalHash, got)
	}
	if err := validateCertificate(cert, opts); err != nil {
		return nil, err
	}
	return &cert, nil
}

func CheckDirectory(specDir string, opts Options) (DirectoryReport, error) {
	report := DirectoryReport{
		Checker: "go",
		Version: Version,
		SpecDir: filepath.ToSlash(specDir),
		AllOK:   true,
	}
	for _, group := range []struct {
		dir      string
		expected string
	}{
		{dir: "valid", expected: "valid"},
		{dir: "invalid", expected: "invalid"},
	} {
		paths, err := filepath.Glob(filepath.Join(specDir, "vectors", group.dir, "*.plci"))
		if err != nil {
			return report, err
		}
		sort.Strings(paths)
		for _, path := range paths {
			cert, err := CheckFile(path, opts)
			accepted := err == nil
			ok := (group.expected == "valid" && accepted) || (group.expected == "invalid" && !accepted)
			rel, relErr := filepath.Rel(filepath.Join(specDir, "vectors"), path)
			if relErr != nil {
				rel = path
			}
			result := VectorResult{
				Path:     filepath.ToSlash(rel),
				Expected: group.expected,
				Accepted: accepted,
				OK:       ok,
			}
			if cert != nil {
				result.CertificateID = cert.CertificateID
				result.Verdict = cert.Verdict
				result.RiskBPS = &cert.RiskBPS
				result.CanonicalSHA256 = cert.CanonicalHash
			}
			if err != nil {
				result.Error = err.Error()
			}
			report.Vectors = append(report.Vectors, result)
			if group.expected == "valid" {
				report.TotalValid++
			} else {
				report.TotalInvalid++
			}
			if accepted {
				report.Accepted++
			} else {
				report.Rejected++
			}
			report.AllOK = report.AllOK && ok
		}
	}
	if report.TotalValid == 0 || report.TotalInvalid == 0 {
		report.AllOK = false
	}
	return report, nil
}

func ValidateGrammar(data []byte) error {
	text := string(data)
	for _, rule := range []string{
		`certificate = header LF certificate-id LF subject-repo LF subject-ref LF subject-path LF subject-sha256 LF issued-at LF producer LF verdict LF risk-bps LF 1*evidence-line 1*obligation-line canonical-sha256 LF end LF`,
		`header = "PLCI/1"`,
		`verdict-value = "safe" / "guarded" / "blocked" / "unsupported"`,
		`status = "checked" / "assumed" / "refuted" / "unsupported"`,
		`canonical-sha256 = "canonical-sha256: " sha256`,
	} {
		if !strings.Contains(text, rule) {
			return fmt.Errorf("grammar missing rule: %s", rule)
		}
	}
	return nil
}

func WriteReportJSON(path string, report DirectoryReport) error {
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o644)
}

func parseEvidenceLine(line string, index int) (Evidence, error) {
	parts := strings.Split(strings.TrimPrefix(line, "evidence: "), " ")
	if len(parts) != 4 {
		return Evidence{}, lineErr(index, "evidence line must have id, type, uri, and sha256")
	}
	for _, part := range parts {
		if part == "" {
			return Evidence{}, lineErr(index, "evidence line must use single spaces")
		}
	}
	item := Evidence{
		ID:     parts[0],
		Type:   strings.TrimPrefix(parts[1], "type="),
		URI:    strings.TrimPrefix(parts[2], "uri="),
		SHA256: strings.TrimPrefix(parts[3], "sha256="),
	}
	if !strings.HasPrefix(parts[1], "type=") || !strings.HasPrefix(parts[2], "uri=") || !strings.HasPrefix(parts[3], "sha256=") {
		return Evidence{}, lineErr(index, "evidence attributes must be type=, uri=, sha256= in order")
	}
	return item, nil
}

func parseObligationLine(line string, index int) (Obligation, error) {
	rest := strings.TrimPrefix(line, "obligation: ")
	before, formula, ok := strings.Cut(rest, ` formula="`)
	if !ok || !strings.HasSuffix(formula, `"`) {
		return Obligation{}, lineErr(index, `obligation line must end with formula="..."`)
	}
	formula = strings.TrimSuffix(formula, `"`)
	parts := strings.Split(before, " ")
	if len(parts) != 4 {
		return Obligation{}, lineErr(index, "obligation line must have id, kind, status, evidence, and formula")
	}
	for _, part := range parts {
		if part == "" {
			return Obligation{}, lineErr(index, "obligation line must use single spaces")
		}
	}
	if !strings.HasPrefix(parts[1], "kind=") || !strings.HasPrefix(parts[2], "status=") || !strings.HasPrefix(parts[3], "evidence=") {
		return Obligation{}, lineErr(index, "obligation attributes must be kind=, status=, evidence= in order")
	}
	refs := strings.Split(strings.TrimPrefix(parts[3], "evidence="), ",")
	return Obligation{
		ID:       parts[0],
		Kind:     strings.TrimPrefix(parts[1], "kind="),
		Status:   strings.TrimPrefix(parts[2], "status="),
		Evidence: refs,
		Formula:  formula,
	}, nil
}

func validateCertificate(cert Certificate, opts Options) error {
	switch {
	case !idRE.MatchString(cert.CertificateID):
		return fmt.Errorf("invalid certificate-id %q", cert.CertificateID)
	case !repoRE.MatchString(cert.SubjectRepo):
		return fmt.Errorf("invalid subject-repo %q", cert.SubjectRepo)
	case !refRE.MatchString(cert.SubjectRef):
		return fmt.Errorf("invalid subject-ref %q", cert.SubjectRef)
	case !validLocalPath(cert.SubjectPath):
		return fmt.Errorf("invalid subject-path %q", cert.SubjectPath)
	case !validSHA256(cert.SubjectSHA256):
		return fmt.Errorf("invalid subject-sha256 %q", cert.SubjectSHA256)
	case !validSHA256(cert.CanonicalHash):
		return fmt.Errorf("invalid canonical-sha256 %q", cert.CanonicalHash)
	case !validVCHAR(cert.Producer):
		return fmt.Errorf("invalid producer %q", cert.Producer)
	case cert.RiskBPS < 0 || cert.RiskBPS > 10000:
		return fmt.Errorf("risk-bps out of range: %d", cert.RiskBPS)
	}
	if !validIssuedAt(cert.IssuedAt) {
		return fmt.Errorf("invalid issued-at %q", cert.IssuedAt)
	}
	if !inSet(cert.Verdict, "safe", "guarded", "blocked", "unsupported") {
		return fmt.Errorf("invalid verdict %q", cert.Verdict)
	}
	if opts.VerifyFiles {
		root := opts.Root
		if root == "" {
			root = "."
		}
		if err := verifyFileDigest(root, cert.SubjectPath, cert.SubjectSHA256); err != nil {
			return fmt.Errorf("subject digest: %w", err)
		}
	}

	evidenceIDs := map[string]bool{}
	for _, item := range cert.Evidence {
		if !idRE.MatchString(item.ID) {
			return fmt.Errorf("invalid evidence id %q", item.ID)
		}
		if evidenceIDs[item.ID] {
			return fmt.Errorf("duplicate evidence id %q", item.ID)
		}
		evidenceIDs[item.ID] = true
		if !inSet(item.Type, "source", "migration", "report", "telemetry", "spec") {
			return fmt.Errorf("invalid evidence type %q", item.Type)
		}
		if !validSHA256(item.SHA256) {
			return fmt.Errorf("invalid evidence sha256 for %q", item.ID)
		}
		if strings.HasPrefix(item.URI, "file:") {
			path := strings.TrimPrefix(item.URI, "file:")
			if !validLocalPath(path) {
				return fmt.Errorf("invalid evidence file uri %q", item.URI)
			}
			if opts.VerifyFiles {
				root := opts.Root
				if root == "" {
					root = "."
				}
				if err := verifyFileDigest(root, path, item.SHA256); err != nil {
					return fmt.Errorf("evidence %s digest: %w", item.ID, err)
				}
			}
		} else if !(validRemoteURI(item.URI, "https://") || validRemoteURI(item.URI, "repo://")) {
			return fmt.Errorf("unsupported evidence uri %q", item.URI)
		}
	}

	obligationIDs := map[string]bool{}
	statusCounts := map[string]int{}
	for _, item := range cert.Obligations {
		if !idRE.MatchString(item.ID) {
			return fmt.Errorf("invalid obligation id %q", item.ID)
		}
		if obligationIDs[item.ID] {
			return fmt.Errorf("duplicate obligation id %q", item.ID)
		}
		obligationIDs[item.ID] = true
		if !inSet(item.Kind, "scope", "frame", "invariant", "rollback", "evidence", "interchange") {
			return fmt.Errorf("invalid obligation kind %q", item.Kind)
		}
		if !inSet(item.Status, "checked", "assumed", "refuted", "unsupported") {
			return fmt.Errorf("invalid obligation status %q", item.Status)
		}
		statusCounts[item.Status]++
		if !validFormula(item.Formula) {
			return fmt.Errorf("invalid formula for obligation %q", item.ID)
		}
		for _, ref := range item.Evidence {
			if !evidenceIDs[ref] {
				return fmt.Errorf("obligation %q references missing evidence %q", item.ID, ref)
			}
		}
	}

	switch cert.Verdict {
	case "safe":
		if statusCounts["assumed"] > 0 || statusCounts["refuted"] > 0 || statusCounts["unsupported"] > 0 {
			return errors.New("safe verdict cannot carry assumed, refuted, or unsupported obligations")
		}
	case "guarded":
		if statusCounts["refuted"] > 0 || statusCounts["unsupported"] > 0 {
			return errors.New("guarded verdict cannot carry refuted or unsupported obligations")
		}
	case "blocked":
		if statusCounts["refuted"] == 0 {
			return errors.New("blocked verdict requires at least one refuted obligation")
		}
	case "unsupported":
		if statusCounts["unsupported"] == 0 {
			return errors.New("unsupported verdict requires at least one unsupported obligation")
		}
	}
	return nil
}

func lineErr(index int, format string, args ...any) error {
	return fmt.Errorf("line %d: %s", index+1, fmt.Sprintf(format, args...))
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
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

func validIssuedAt(value string) bool {
	if !issuedRE.MatchString(value) {
		return false
	}
	year, _ := strconv.Atoi(value[0:4])
	month, _ := strconv.Atoi(value[5:7])
	day, _ := strconv.Atoi(value[8:10])
	hour, _ := strconv.Atoi(value[11:13])
	minute, _ := strconv.Atoi(value[14:16])
	second, _ := strconv.Atoi(value[17:19])
	if month < 1 || month > 12 || hour > 23 || minute > 59 || second > 59 {
		return false
	}
	days := []int{31, 28, 31, 30, 31, 30, 31, 31, 30, 31, 30, 31}
	if month == 2 && leapYear(year) {
		days[1] = 29
	}
	return day >= 1 && day <= days[month-1]
}

func leapYear(year int) bool {
	return year%400 == 0 || (year%4 == 0 && year%100 != 0)
}

func validLocalPath(value string) bool {
	if value == "" || strings.HasPrefix(value, "/") || strings.Contains(value, `\`) {
		return false
	}
	clean := path.Clean(value)
	if clean == "." || strings.HasPrefix(clean, "..") || clean != value {
		return false
	}
	for _, part := range strings.Split(value, "/") {
		if part == "" || part == "." || part == ".." {
			return false
		}
		for _, r := range part {
			if !((r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' || r == '.' || r == '-') {
				return false
			}
		}
	}
	return true
}

func validRemoteURI(value, scheme string) bool {
	if !strings.HasPrefix(value, scheme) {
		return false
	}
	rest := strings.TrimPrefix(value, scheme)
	if rest == "" {
		return false
	}
	for _, r := range rest {
		if r < 0x21 || r > 0x7e {
			return false
		}
	}
	return true
}

func validVCHAR(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if r < 0x21 || r > 0x7e {
			return false
		}
	}
	return true
}

func validFormula(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if r == '"' || r < 0x20 || r > 0x7e {
			return false
		}
	}
	return true
}

func verifyFileDigest(root, path, expected string) error {
	if !validLocalPath(path) {
		return fmt.Errorf("invalid local path %q", path)
	}
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
	if err != nil {
		return err
	}
	if got := sha256Hex(data); got != expected {
		return fmt.Errorf("%s sha256 got %s want %s", path, got, expected)
	}
	return nil
}

func inSet(value string, choices ...string) bool {
	for _, choice := range choices {
		if value == choice {
			return true
		}
	}
	return false
}
