package verdictx

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"

	"github.com/thehalleyyoung/patchline/internal/certlang"
)

const (
	Version     = "patchline.verdict-exchange/v1"
	issuedAt    = "2026-06-02T00:00:00Z"
	subjectRef  = "verdict-exchange-v1"
	subjectRepo = "thehalleyyoung/patchline"
	producer    = "patchline-verdictx/1"
)

type Projection struct {
	Analyzer    string       `json:"analyzer"`
	Tool        string       `json:"tool"`
	CaseID      string       `json:"case_id"`
	SourcePath  string       `json:"source_path"`
	Outcome     string       `json:"outcome"`
	Verdict     string       `json:"verdict"`
	RiskBPS     int          `json:"risk_bps"`
	Obligations []Obligation `json:"obligations"`
}

type Obligation struct {
	ID       string   `json:"id"`
	Kind     string   `json:"kind"`
	Status   string   `json:"status"`
	Evidence []string `json:"evidence"`
	Formula  string   `json:"formula"`
}

type Report struct {
	Version                string            `json:"version"`
	SpecDir                string            `json:"spec_dir"`
	Analyzers              []string          `json:"analyzers"`
	PositiveCases          int               `json:"positive_cases"`
	RoundTrips             int               `json:"roundtrips"`
	NegativeControlsPassed int               `json:"negative_controls_passed"`
	Verified               bool              `json:"verified"`
	Cases                  []CaseReport      `json:"cases"`
	NegativeControls       []NegativeControl `json:"negative_controls"`
}

type CaseReport struct {
	Analyzer                  string `json:"analyzer"`
	CaseID                    string `json:"case_id"`
	Fixture                   string `json:"fixture"`
	CertificateID             string `json:"certificate_id"`
	CertificateSHA256         string `json:"certificate_sha256"`
	CertificateAccepted       bool   `json:"certificate_accepted"`
	Equivalent                bool   `json:"equivalent"`
	OriginalProjectionSHA256  string `json:"original_projection_sha256"`
	RoundTripProjectionSHA256 string `json:"roundtrip_projection_sha256"`
}

type NegativeControl struct {
	Name   string `json:"name"`
	Passed bool   `json:"passed"`
	Detail string `json:"detail"`
}

func RunSuite(specDir, root string) (Report, error) {
	report := Report{
		Version: Version + "-results",
		SpecDir: filepath.ToSlash(specDir),
	}
	paths, err := filepath.Glob(filepath.Join(specDir, "analyzers", "*", "*.json"))
	if err != nil {
		return report, err
	}
	sort.Strings(paths)
	if len(paths) == 0 {
		return report, fmt.Errorf("no analyzer fixtures found under %s", specDir)
	}

	analyzers := map[string]bool{}
	var first Projection
	var firstCertificate string
	for _, path := range paths {
		projection, err := ParseNativeFile(path)
		if err != nil {
			return report, fmt.Errorf("%s: %w", path, err)
		}
		certificate, err := RenderCertificate(projection, path, root)
		if err != nil {
			return report, fmt.Errorf("%s: render certificate: %w", path, err)
		}
		parsed, err := certlang.Parse([]byte(certificate), certlang.Options{Root: root, VerifyFiles: true})
		if err != nil {
			return report, fmt.Errorf("%s: parse rendered certificate: %w", path, err)
		}
		roundTrip, err := ProjectionFromCertificate(parsed)
		if err != nil {
			return report, fmt.Errorf("%s: projection from certificate: %w", path, err)
		}
		originalDoc, err := ProjectionDocument(projection)
		if err != nil {
			return report, err
		}
		roundTripDoc, err := ProjectionDocument(roundTrip)
		if err != nil {
			return report, err
		}
		originalHash := jsonSHA256(originalDoc)
		roundTripHash := jsonSHA256(roundTripDoc)
		equivalent := reflect.DeepEqual(originalDoc, roundTripDoc)
		rel, err := relLocal(root, path)
		if err != nil {
			return report, err
		}
		report.Cases = append(report.Cases, CaseReport{
			Analyzer:                  projection.Analyzer,
			CaseID:                    projection.CaseID,
			Fixture:                   rel,
			CertificateID:             parsed.CertificateID,
			CertificateSHA256:         sha256Hex([]byte(certificate)),
			CertificateAccepted:       true,
			Equivalent:                equivalent,
			OriginalProjectionSHA256:  originalHash,
			RoundTripProjectionSHA256: roundTripHash,
		})
		report.PositiveCases++
		if equivalent {
			report.RoundTrips++
		}
		analyzers[projection.Analyzer] = true
		if first.CertificateID() == "" {
			first = projection
			firstCertificate = certificate
		}
	}

	report.Analyzers = sortedKeys(analyzers)
	report.NegativeControls = RunNegativeControls(specDir, root, first, firstCertificate)
	for _, control := range report.NegativeControls {
		if control.Passed {
			report.NegativeControlsPassed++
		}
	}
	report.Verified = len(report.Analyzers) >= 3 &&
		report.PositiveCases >= 3 &&
		report.PositiveCases == report.RoundTrips &&
		report.NegativeControlsPassed == len(report.NegativeControls) &&
		report.NegativeControlsPassed >= 3
	return report, nil
}

func ParseNativeFile(path string) (Projection, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Projection{}, err
	}
	var header struct {
		Schema string `json:"schema"`
	}
	if err := json.Unmarshal(data, &header); err != nil {
		return Projection{}, err
	}
	switch header.Schema {
	case "strong_migrations.audit/v1":
		return parseStrongMigrations(data)
	case "django.migration.check/v1":
		return parseDjangoMigrations(data)
	case "prisma.migrate.diagnostics/v1":
		return parsePrismaMigrate(data)
	default:
		return Projection{}, fmt.Errorf("unsupported analyzer schema %q", header.Schema)
	}
}

func RenderCertificate(projection Projection, fixturePath, root string) (string, error) {
	if err := validateProjection(projection); err != nil {
		return "", err
	}
	fixtureRel, err := relLocal(root, fixturePath)
	if err != nil {
		return "", err
	}
	sourceDigest, err := fileSHA256(root, projection.SourcePath)
	if err != nil {
		return "", err
	}
	fixtureDigest, err := fileSHA256(root, fixtureRel)
	if err != nil {
		return "", err
	}
	lines := []string{
		certlang.Version,
		"certificate-id: " + projection.CertificateID(),
		"subject-repo: " + subjectRepo,
		"subject-ref: " + subjectRef,
		"subject-path: " + projection.SourcePath,
		"subject-sha256: " + sourceDigest,
		"issued-at: " + issuedAt,
		"producer: " + producer,
		"verdict: " + projection.Verdict,
		fmt.Sprintf("risk-bps: %d", projection.RiskBPS),
		"evidence: ev.source type=migration uri=file:" + projection.SourcePath + " sha256=" + sourceDigest,
		"evidence: ev.native type=report uri=file:" + fixtureRel + " sha256=" + fixtureDigest,
	}
	for _, obligation := range projection.Obligations {
		lines = append(lines, fmt.Sprintf("obligation: %s kind=%s status=%s evidence=%s formula=%q",
			obligation.ID,
			obligation.Kind,
			obligation.Status,
			strings.Join(obligation.Evidence, ","),
			obligation.Formula,
		))
	}
	canonical := strings.Join(lines, "\n") + "\n"
	lines = append(lines, "canonical-sha256: "+sha256Hex([]byte(canonical)), "END")
	return strings.Join(lines, "\n") + "\n", nil
}

func ProjectionFromCertificate(cert *certlang.Certificate) (Projection, error) {
	analyzer, caseID, err := splitCertificateID(cert.CertificateID)
	if err != nil {
		return Projection{}, err
	}
	outcome, err := outcomeForVerdict(analyzer, cert.Verdict)
	if err != nil {
		return Projection{}, err
	}
	obligations := make([]Obligation, 0, len(cert.Obligations))
	for _, obligation := range cert.Obligations {
		obligations = append(obligations, Obligation{
			ID:       obligation.ID,
			Kind:     obligation.Kind,
			Status:   obligation.Status,
			Evidence: append([]string(nil), obligation.Evidence...),
			Formula:  obligation.Formula,
		})
	}
	return Projection{
		Analyzer:    analyzer,
		Tool:        toolForAnalyzer(analyzer),
		CaseID:      caseID,
		SourcePath:  cert.SubjectPath,
		Outcome:     outcome,
		Verdict:     cert.Verdict,
		RiskBPS:     cert.RiskBPS,
		Obligations: obligations,
	}, nil
}

func ProjectionDocument(projection Projection) (map[string]any, error) {
	obligations := make([]map[string]any, 0, len(projection.Obligations))
	for _, obligation := range projection.Obligations {
		obligations = append(obligations, map[string]any{
			"id":       obligation.ID,
			"kind":     obligation.Kind,
			"status":   obligation.Status,
			"evidence": append([]string(nil), obligation.Evidence...),
			"message":  obligation.Formula,
		})
	}
	switch projection.Analyzer {
	case "strong-migrations":
		return map[string]any{
			"schema":      "strong_migrations.audit/projection-v1",
			"tool":        "strong_migrations",
			"case_id":     projection.CaseID,
			"file":        projection.SourcePath,
			"safety":      projection.Outcome,
			"risk_bps":    projection.RiskBPS,
			"obligations": obligations,
		}, nil
	case "django-migrations":
		return map[string]any{
			"schema":    "django.migration.check/projection-v1",
			"tool":      "django-migrations",
			"case_id":   projection.CaseID,
			"migration": map[string]any{"path": projection.SourcePath},
			"result":    projection.Outcome,
			"risk_bps":  projection.RiskBPS,
			"concerns":  obligations,
		}, nil
	case "prisma-migrate":
		return map[string]any{
			"schema":         "prisma.migrate.diagnostics/projection-v1",
			"tool":           "prisma-migrate",
			"case_id":        projection.CaseID,
			"migration_path": projection.SourcePath,
			"diagnostic":     map[string]any{"level": projection.Outcome, "risk_bps": projection.RiskBPS},
			"warnings":       obligations,
		}, nil
	default:
		return nil, fmt.Errorf("unsupported analyzer %q", projection.Analyzer)
	}
}

func RunNegativeControls(specDir, root string, first Projection, certificate string) []NegativeControl {
	controls := []NegativeControl{
		riskDriftControl(root, first, certificate),
		subjectDigestControl(root, certificate),
		parseMustFailControl(filepath.Join(specDir, "negative", "missing-risk-bps.json"), "missing preserved risk-bps"),
		parseMustFailControl(filepath.Join(specDir, "negative", "unsupported-analyzer.json"), "unsupported analyzer schema"),
	}
	return controls
}

func (projection Projection) CertificateID() string {
	if projection.Analyzer == "" || projection.CaseID == "" {
		return ""
	}
	return projection.Analyzer + "." + projection.CaseID + ".v1"
}

type strongFixture struct {
	Schema   string `json:"schema"`
	Tool     string `json:"tool"`
	CaseID   string `json:"case_id"`
	File     string `json:"file"`
	Check    string `json:"check"`
	Safety   string `json:"safety"`
	RiskBPS  *int   `json:"risk_bps"`
	Message  string `json:"message"`
	Evidence []struct {
		Path   string `json:"path"`
		Line   int    `json:"line"`
		Signal string `json:"signal"`
	} `json:"evidence"`
}

type djangoFixture struct {
	Schema    string `json:"schema"`
	Tool      string `json:"tool"`
	CaseID    string `json:"case_id"`
	Migration struct {
		Path      string `json:"path"`
		Operation string `json:"operation"`
		App       string `json:"app"`
	} `json:"migration"`
	Result  string `json:"result"`
	RiskBPS *int   `json:"risk_bps"`
	Message string `json:"message"`
}

type prismaFixture struct {
	Schema        string `json:"schema"`
	Tool          string `json:"tool"`
	CaseID        string `json:"case_id"`
	MigrationPath string `json:"migration_path"`
	Diagnostic    struct {
		Level     string `json:"level"`
		Code      string `json:"code"`
		Operation string `json:"operation"`
		RiskBPS   *int   `json:"risk_bps"`
		Message   string `json:"message"`
	} `json:"diagnostic"`
}

func parseStrongMigrations(data []byte) (Projection, error) {
	var fixture strongFixture
	if err := json.Unmarshal(data, &fixture); err != nil {
		return Projection{}, err
	}
	if fixture.RiskBPS == nil {
		return Projection{}, errors.New("strong_migrations fixture missing preserved risk_bps")
	}
	verdict, status, err := strongVerdict(fixture.Safety)
	if err != nil {
		return Projection{}, err
	}
	message := fmt.Sprintf("strong_migrations %s: %s", fixture.Check, fixture.Message)
	return projection("strong-migrations", fixture.Tool, fixture.CaseID, fixture.File, fixture.Safety, verdict, *fixture.RiskBPS, status, message)
}

func parseDjangoMigrations(data []byte) (Projection, error) {
	var fixture djangoFixture
	if err := json.Unmarshal(data, &fixture); err != nil {
		return Projection{}, err
	}
	if fixture.RiskBPS == nil {
		return Projection{}, errors.New("django migration fixture missing preserved risk_bps")
	}
	verdict, status, err := djangoVerdict(fixture.Result)
	if err != nil {
		return Projection{}, err
	}
	message := fmt.Sprintf("django %s %s: %s", fixture.Migration.App, fixture.Migration.Operation, fixture.Message)
	return projection("django-migrations", fixture.Tool, fixture.CaseID, fixture.Migration.Path, fixture.Result, verdict, *fixture.RiskBPS, status, message)
}

func parsePrismaMigrate(data []byte) (Projection, error) {
	var fixture prismaFixture
	if err := json.Unmarshal(data, &fixture); err != nil {
		return Projection{}, err
	}
	if fixture.Diagnostic.RiskBPS == nil {
		return Projection{}, errors.New("prisma fixture missing preserved diagnostic.risk_bps")
	}
	verdict, status, err := prismaVerdict(fixture.Diagnostic.Level)
	if err != nil {
		return Projection{}, err
	}
	message := fmt.Sprintf("prisma %s %s: %s", fixture.Diagnostic.Code, fixture.Diagnostic.Operation, fixture.Diagnostic.Message)
	return projection("prisma-migrate", fixture.Tool, fixture.CaseID, fixture.MigrationPath, fixture.Diagnostic.Level, verdict, *fixture.Diagnostic.RiskBPS, status, message)
}

func projection(analyzer, tool, caseID, sourcePath, outcome, verdict string, riskBPS int, status, formula string) (Projection, error) {
	p := Projection{
		Analyzer:   analyzer,
		Tool:       tool,
		CaseID:     caseID,
		SourcePath: filepath.ToSlash(sourcePath),
		Outcome:    outcome,
		Verdict:    verdict,
		RiskBPS:    riskBPS,
		Obligations: []Obligation{{
			ID:       "obl." + analyzer,
			Kind:     "interchange",
			Status:   status,
			Evidence: []string{"ev.source", "ev.native"},
			Formula:  formula,
		}},
	}
	return p, validateProjection(p)
}

func validateProjection(projection Projection) error {
	switch {
	case projection.Analyzer == "":
		return errors.New("projection missing analyzer")
	case projection.Tool == "":
		return errors.New("projection missing tool")
	case projection.CaseID == "":
		return errors.New("projection missing case_id")
	case projection.SourcePath == "":
		return errors.New("projection missing source path")
	case projection.Verdict == "":
		return errors.New("projection missing verdict")
	case projection.RiskBPS < 0 || projection.RiskBPS > 10000:
		return fmt.Errorf("projection risk_bps out of range: %d", projection.RiskBPS)
	case len(projection.Obligations) == 0:
		return errors.New("projection missing obligations")
	}
	if strings.Contains(projection.Analyzer, "_") || strings.Contains(projection.CaseID, "_") {
		return fmt.Errorf("projection identifiers must be PLCI-id compatible: %s.%s", projection.Analyzer, projection.CaseID)
	}
	for _, obligation := range projection.Obligations {
		if strings.Contains(obligation.Formula, `"`) {
			return fmt.Errorf("obligation %s formula contains unsupported quote", obligation.ID)
		}
	}
	return nil
}

func strongVerdict(safety string) (string, string, error) {
	switch safety {
	case "safe":
		return "safe", "checked", nil
	case "caution":
		return "guarded", "assumed", nil
	case "danger":
		return "blocked", "refuted", nil
	case "unsupported":
		return "unsupported", "unsupported", nil
	default:
		return "", "", fmt.Errorf("unsupported strong_migrations safety %q", safety)
	}
}

func djangoVerdict(result string) (string, string, error) {
	switch result {
	case "clean":
		return "safe", "checked", nil
	case "requires-review":
		return "guarded", "assumed", nil
	case "blocked":
		return "blocked", "refuted", nil
	case "unsupported":
		return "unsupported", "unsupported", nil
	default:
		return "", "", fmt.Errorf("unsupported django result %q", result)
	}
}

func prismaVerdict(level string) (string, string, error) {
	switch level {
	case "clean":
		return "safe", "checked", nil
	case "warning":
		return "guarded", "assumed", nil
	case "error":
		return "blocked", "refuted", nil
	case "unsupported":
		return "unsupported", "unsupported", nil
	default:
		return "", "", fmt.Errorf("unsupported prisma level %q", level)
	}
}

func outcomeForVerdict(analyzer, verdict string) (string, error) {
	switch analyzer {
	case "strong-migrations":
		switch verdict {
		case "safe":
			return "safe", nil
		case "guarded":
			return "caution", nil
		case "blocked":
			return "danger", nil
		case "unsupported":
			return "unsupported", nil
		}
	case "django-migrations":
		switch verdict {
		case "safe":
			return "clean", nil
		case "guarded":
			return "requires-review", nil
		case "blocked":
			return "blocked", nil
		case "unsupported":
			return "unsupported", nil
		}
	case "prisma-migrate":
		switch verdict {
		case "safe":
			return "clean", nil
		case "guarded":
			return "warning", nil
		case "blocked":
			return "error", nil
		case "unsupported":
			return "unsupported", nil
		}
	}
	return "", fmt.Errorf("unsupported analyzer/verdict pair %s/%s", analyzer, verdict)
}

func toolForAnalyzer(analyzer string) string {
	switch analyzer {
	case "strong-migrations":
		return "strong_migrations"
	case "django-migrations":
		return "django-migrations"
	case "prisma-migrate":
		return "prisma-migrate"
	default:
		return analyzer
	}
}

func splitCertificateID(id string) (string, string, error) {
	if !strings.HasSuffix(id, ".v1") {
		return "", "", fmt.Errorf("certificate id %q missing .v1 suffix", id)
	}
	body := strings.TrimSuffix(id, ".v1")
	analyzer, caseID, ok := strings.Cut(body, ".")
	if !ok || analyzer == "" || caseID == "" {
		return "", "", fmt.Errorf("certificate id %q does not encode analyzer and case", id)
	}
	return analyzer, caseID, nil
}

func riskDriftControl(root string, original Projection, certificate string) NegativeControl {
	if certificate == "" {
		return NegativeControl{Name: "risk drift projection", Detail: "missing certificate"}
	}
	nextRisk := original.RiskBPS + 1
	if nextRisk > 10000 {
		nextRisk = original.RiskBPS - 1
	}
	mutated := replaceLine(certificate, "risk-bps: ", fmt.Sprintf("risk-bps: %d", nextRisk))
	mutated = recomputeCanonical(mutated)
	parsed, err := certlang.Parse([]byte(mutated), certlang.Options{Root: root, VerifyFiles: true})
	if err != nil {
		return NegativeControl{Name: "risk drift projection", Detail: err.Error()}
	}
	roundTrip, err := ProjectionFromCertificate(parsed)
	if err != nil {
		return NegativeControl{Name: "risk drift projection", Detail: err.Error()}
	}
	originalDoc, _ := ProjectionDocument(original)
	roundTripDoc, _ := ProjectionDocument(roundTrip)
	passed := !reflect.DeepEqual(originalDoc, roundTripDoc)
	return NegativeControl{Name: "risk drift projection", Passed: passed, Detail: "changed preserved risk-bps must break projection equivalence"}
}

func subjectDigestControl(root, certificate string) NegativeControl {
	if certificate == "" {
		return NegativeControl{Name: "subject digest verification", Detail: "missing certificate"}
	}
	mutated := replaceLine(certificate, "subject-sha256: ", "subject-sha256: "+strings.Repeat("0", 64))
	mutated = recomputeCanonical(mutated)
	_, err := certlang.Parse([]byte(mutated), certlang.Options{Root: root, VerifyFiles: true})
	passed := err != nil
	detail := "digest mismatch rejected"
	if err == nil {
		detail = "digest mismatch was accepted"
	}
	return NegativeControl{Name: "subject digest verification", Passed: passed, Detail: detail}
}

func parseMustFailControl(path, name string) NegativeControl {
	_, err := ParseNativeFile(path)
	passed := err != nil
	detail := "invalid native fixture rejected"
	if err == nil {
		detail = "invalid native fixture was accepted"
	}
	return NegativeControl{Name: name, Passed: passed, Detail: detail}
}

func replaceLine(text, prefix, replacement string) string {
	lines := strings.Split(strings.TrimSuffix(text, "\n"), "\n")
	for i, line := range lines {
		if strings.HasPrefix(line, prefix) {
			lines[i] = replacement
			break
		}
	}
	return strings.Join(lines, "\n") + "\n"
}

func recomputeCanonical(text string) string {
	lines := strings.Split(strings.TrimSuffix(text, "\n"), "\n")
	for i, line := range lines {
		if strings.HasPrefix(line, "canonical-sha256: ") {
			canonical := strings.Join(lines[:i], "\n") + "\n"
			lines[i] = "canonical-sha256: " + sha256Hex([]byte(canonical))
			return strings.Join(lines, "\n") + "\n"
		}
	}
	return text
}

func fileSHA256(root, localPath string) (string, error) {
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(localPath)))
	if err != nil {
		return "", err
	}
	return sha256Hex(data), nil
}

func relLocal(root, path string) (string, error) {
	if !filepath.IsAbs(path) {
		return filepath.ToSlash(filepath.Clean(path)), nil
	}
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return "", err
	}
	return filepath.ToSlash(rel), nil
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func jsonSHA256(value any) string {
	data, err := json.Marshal(value)
	if err != nil {
		return ""
	}
	return sha256Hex(data)
}

func sortedKeys(values map[string]bool) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
