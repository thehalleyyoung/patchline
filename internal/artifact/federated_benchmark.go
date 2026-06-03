package artifact

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/thehalleyyoung/patchline/internal/attest"
	"github.com/thehalleyyoung/patchline/internal/canonical"
)

const (
	FederatedBenchmarkSplitVersion        = "patchline.federated-benchmark-split/v1"
	FederatedBenchmarkAggregateVersion    = "patchline.federated-benchmark-aggregate/v1"
	FederatedBenchmarkVerificationVersion = "patchline.federated-benchmark-verification/v1"
	DefaultFederatedMinPrivateCases       = 3
)

type FederatedBenchmarkSplitOptions struct {
	ManifestPath    string
	AdopterID       string
	PrivateCases    []string
	MinPrivateCases int
	PartitionSalt   string
}

type FederatedBenchmarkRunOptions struct {
	SplitPath string
	SeedHex   string
}

type FederatedBenchmarkSplit struct {
	Version          string   `json:"version"`
	DatasetID        string   `json:"dataset_id"`
	AdopterID        string   `json:"adopter_id"`
	Manifest         string   `json:"manifest"`
	ManifestHash     string   `json:"manifest_hash"`
	MinPrivateCases  int      `json:"min_private_cases"`
	PrivateCases     []string `json:"private_cases"`
	PublicCases      []string `json:"public_cases"`
	PrivateCaseCount int      `json:"private_case_count"`
	PublicCaseCount  int      `json:"public_case_count"`
	TotalCaseCount   int      `json:"total_case_count"`
	PartitionSalt    string   `json:"partition_salt"`
	PartitionHash    string   `json:"partition_hash"`
	Hash             string   `json:"hash"`
}

type FederatedBenchmarkAggregate struct {
	Version          string                             `json:"version"`
	OK               bool                               `json:"ok"`
	DatasetID        string                             `json:"dataset_id"`
	AdopterID        string                             `json:"adopter_id"`
	ManifestHash     string                             `json:"manifest_hash"`
	SplitHash        string                             `json:"split_hash"`
	PartitionHash    string                             `json:"partition_hash"`
	MinPrivateCases  int                                `json:"min_private_cases"`
	PrivateCaseCount int                                `json:"private_case_count"`
	PublicCaseCount  int                                `json:"public_case_count"`
	TotalCaseCount   int                                `json:"total_case_count"`
	Metrics          FederatedBenchmarkAggregateMetrics `json:"metrics"`
	Signature        attest.Signed                      `json:"signature"`
	Hash             string                             `json:"hash"`
	Markdown         string                             `json:"markdown,omitempty"`
}

type FederatedBenchmarkAggregateMetrics struct {
	Total             int            `json:"total"`
	Buckets           map[string]int `json:"buckets"`
	SuppressedBuckets int            `json:"suppressed_buckets"`
}

type FederatedBenchmarkVerificationReport struct {
	Version       string   `json:"version"`
	OK            bool     `json:"ok"`
	AggregateHash string   `json:"aggregate_hash"`
	Subject       string   `json:"subject"`
	PublicKey     string   `json:"public_key"`
	Errors        []string `json:"errors,omitempty"`
	Hash          string   `json:"hash"`
}

type federatedBenchmarkPartitionCommitment struct {
	Version       string   `json:"version"`
	DatasetID     string   `json:"dataset_id"`
	AdopterID     string   `json:"adopter_id"`
	ManifestHash  string   `json:"manifest_hash"`
	PartitionSalt string   `json:"partition_salt"`
	PrivateCases  []string `json:"private_cases"`
	PublicCases   []string `json:"public_cases"`
}

type federatedBenchmarkAggregatePayload struct {
	Version          string                             `json:"version"`
	OK               bool                               `json:"ok"`
	DatasetID        string                             `json:"dataset_id"`
	AdopterID        string                             `json:"adopter_id"`
	ManifestHash     string                             `json:"manifest_hash"`
	SplitHash        string                             `json:"split_hash"`
	PartitionHash    string                             `json:"partition_hash"`
	MinPrivateCases  int                                `json:"min_private_cases"`
	PrivateCaseCount int                                `json:"private_case_count"`
	PublicCaseCount  int                                `json:"public_case_count"`
	TotalCaseCount   int                                `json:"total_case_count"`
	Metrics          FederatedBenchmarkAggregateMetrics `json:"metrics"`
}

func CreateFederatedBenchmarkSplit(options FederatedBenchmarkSplitOptions) (FederatedBenchmarkSplit, error) {
	if strings.TrimSpace(options.ManifestPath) == "" {
		return FederatedBenchmarkSplit{}, fmt.Errorf("manifest path is required")
	}
	adopterID := strings.TrimSpace(options.AdopterID)
	if adopterID == "" {
		return FederatedBenchmarkSplit{}, fmt.Errorf("adopter id is required")
	}
	minPrivateCases := normalizeFederatedMinPrivateCases(options.MinPrivateCases)
	validation, err := ValidateBenchmarkManifest(options.ManifestPath)
	if err != nil {
		return FederatedBenchmarkSplit{}, err
	}
	if !validation.OK {
		return FederatedBenchmarkSplit{}, fmt.Errorf("benchmark manifest validation failed with %d error(s)", len(validation.Errors))
	}
	manifest, err := readManifestFile(options.ManifestPath)
	if err != nil {
		return FederatedBenchmarkSplit{}, err
	}
	if len(manifest.Cases) == 0 {
		return FederatedBenchmarkSplit{}, fmt.Errorf("manifest must contain at least one case")
	}
	privateCases, publicCases, err := federatedPartitionCases(manifest.Cases, options.PrivateCases)
	if err != nil {
		return FederatedBenchmarkSplit{}, err
	}
	if len(privateCases) < minPrivateCases {
		return FederatedBenchmarkSplit{}, fmt.Errorf("private split has %d case(s), below k-anonymity minimum %d", len(privateCases), minPrivateCases)
	}
	manifestAbs, err := filepath.Abs(options.ManifestPath)
	if err != nil {
		return FederatedBenchmarkSplit{}, err
	}
	partitionSalt, err := federatedPartitionSalt(options.PartitionSalt)
	if err != nil {
		return FederatedBenchmarkSplit{}, err
	}
	split := FederatedBenchmarkSplit{
		Version:          FederatedBenchmarkSplitVersion,
		DatasetID:        strings.TrimSpace(manifest.DatasetID),
		AdopterID:        adopterID,
		Manifest:         filepath.ToSlash(manifestAbs),
		ManifestHash:     "sha256:" + canonical.Hash(manifest),
		MinPrivateCases:  minPrivateCases,
		PrivateCases:     privateCases,
		PublicCases:      publicCases,
		PrivateCaseCount: len(privateCases),
		PublicCaseCount:  len(publicCases),
		TotalCaseCount:   len(manifest.Cases),
		PartitionSalt:    partitionSalt,
	}
	split.PartitionHash = federatedPartitionHash(split)
	split.Hash = federatedSplitHash(split)
	return split, nil
}

func RunFederatedBenchmarkAggregate(options FederatedBenchmarkRunOptions) (FederatedBenchmarkAggregate, error) {
	split, err := ReadFederatedBenchmarkSplitFile(options.SplitPath)
	if err != nil {
		return FederatedBenchmarkAggregate{}, err
	}
	if split.Hash != federatedSplitHash(split) {
		return FederatedBenchmarkAggregate{}, fmt.Errorf("split hash mismatch: declared=%s actual=%s", split.Hash, federatedSplitHash(split))
	}
	if split.PartitionHash != federatedPartitionHash(split) {
		return FederatedBenchmarkAggregate{}, fmt.Errorf("partition hash mismatch: declared=%s actual=%s", split.PartitionHash, federatedPartitionHash(split))
	}
	seed, err := attest.SeedFromHex(strings.TrimSpace(options.SeedHex))
	if err != nil {
		return FederatedBenchmarkAggregate{}, err
	}
	run, err := RunBenchmarkManifest(filepath.FromSlash(split.Manifest))
	if err != nil {
		return FederatedBenchmarkAggregate{}, err
	}
	privateResults, err := privateBenchmarkResults(run.Cases, split.PrivateCases)
	if err != nil {
		return FederatedBenchmarkAggregate{}, err
	}
	metrics, err := federatedAggregateMetrics(privateResults, split.MinPrivateCases)
	if err != nil {
		return FederatedBenchmarkAggregate{}, err
	}
	aggregate := FederatedBenchmarkAggregate{
		Version:          FederatedBenchmarkAggregateVersion,
		OK:               true,
		DatasetID:        split.DatasetID,
		AdopterID:        split.AdopterID,
		ManifestHash:     split.ManifestHash,
		SplitHash:        split.Hash,
		PartitionHash:    split.PartitionHash,
		MinPrivateCases:  split.MinPrivateCases,
		PrivateCaseCount: split.PrivateCaseCount,
		PublicCaseCount:  split.PublicCaseCount,
		TotalCaseCount:   split.TotalCaseCount,
		Metrics:          metrics,
	}
	signature, err := attest.Sign(federatedAggregateSubject(split.AdopterID), federatedAggregatePayloadBytes(aggregate), seed)
	if err != nil {
		return FederatedBenchmarkAggregate{}, err
	}
	aggregate.Signature = signature
	aggregate.Hash = federatedAggregateHash(aggregate)
	aggregate.Markdown = renderFederatedAggregateMarkdown(aggregate)
	return aggregate, nil
}

func VerifyFederatedBenchmarkAggregateFile(path string) (FederatedBenchmarkVerificationReport, error) {
	aggregate, err := ReadFederatedBenchmarkAggregateFile(path)
	if err != nil {
		return FederatedBenchmarkVerificationReport{}, err
	}
	return VerifyFederatedBenchmarkAggregate(aggregate), nil
}

func VerifyFederatedBenchmarkAggregate(aggregate FederatedBenchmarkAggregate) FederatedBenchmarkVerificationReport {
	report := FederatedBenchmarkVerificationReport{
		Version:       FederatedBenchmarkVerificationVersion,
		AggregateHash: federatedAggregateHash(aggregate),
		Subject:       aggregate.Signature.Subject,
		PublicKey:     aggregate.Signature.PublicKey,
	}
	if aggregate.Version != FederatedBenchmarkAggregateVersion {
		report.Errors = append(report.Errors, fmt.Sprintf("version must be %s", FederatedBenchmarkAggregateVersion))
	}
	if aggregate.Hash != report.AggregateHash {
		report.Errors = append(report.Errors, fmt.Sprintf("aggregate hash mismatch: declared=%s actual=%s", aggregate.Hash, report.AggregateHash))
	}
	if aggregate.Signature.Subject != federatedAggregateSubject(aggregate.AdopterID) {
		report.Errors = append(report.Errors, fmt.Sprintf("signature subject must be %s", federatedAggregateSubject(aggregate.AdopterID)))
	}
	if err := attest.VerifySignature(aggregate.Signature, federatedAggregatePayloadBytes(aggregate)); err != nil {
		report.Errors = append(report.Errors, err.Error())
	}
	report.Errors = append(report.Errors, validateFederatedAggregatePrivacy(aggregate)...)
	sort.Strings(report.Errors)
	report.OK = len(report.Errors) == 0
	report.Hash = federatedVerificationHash(report)
	return report
}

func ReadFederatedBenchmarkSplitFile(path string) (FederatedBenchmarkSplit, error) {
	var split FederatedBenchmarkSplit
	if err := readStrictJSON(path, &split); err != nil {
		return FederatedBenchmarkSplit{}, err
	}
	return split, nil
}

func ReadFederatedBenchmarkAggregateFile(path string) (FederatedBenchmarkAggregate, error) {
	var aggregate FederatedBenchmarkAggregate
	if err := readStrictJSON(path, &aggregate); err != nil {
		return FederatedBenchmarkAggregate{}, err
	}
	return aggregate, nil
}

func WriteFederatedBenchmarkSplit(path string, split FederatedBenchmarkSplit) error {
	return writeArtifactJSON(path, split)
}

func WriteFederatedBenchmarkAggregate(path string, aggregate FederatedBenchmarkAggregate) error {
	if err := writeArtifactJSON(path, aggregate); err != nil {
		return err
	}
	md := strings.TrimSuffix(path, filepath.Ext(path)) + ".md"
	return os.WriteFile(md, []byte(aggregate.Markdown), 0o644)
}

func WriteFederatedBenchmarkVerification(path string, report FederatedBenchmarkVerificationReport) error {
	return writeArtifactJSON(path, report)
}

func federatedPartitionCases(cases []ManifestCase, privateIDs []string) ([]string, []string, error) {
	caseSet := map[string]bool{}
	for _, c := range cases {
		id := strings.TrimSpace(c.CaseID)
		if id == "" {
			return nil, nil, fmt.Errorf("manifest contains a case without case_id")
		}
		if caseSet[id] {
			return nil, nil, fmt.Errorf("duplicate manifest case_id %q", id)
		}
		caseSet[id] = true
	}
	privateSet := map[string]bool{}
	if len(privateIDs) == 0 {
		for id := range caseSet {
			privateSet[id] = true
		}
	} else {
		for _, raw := range privateIDs {
			id := strings.TrimSpace(raw)
			if id == "" {
				continue
			}
			if !caseSet[id] {
				return nil, nil, fmt.Errorf("private case %q is not present in manifest", id)
			}
			if privateSet[id] {
				return nil, nil, fmt.Errorf("duplicate private case %q", id)
			}
			privateSet[id] = true
		}
	}
	var privateCases []string
	var publicCases []string
	for id := range caseSet {
		if privateSet[id] {
			privateCases = append(privateCases, id)
		} else {
			publicCases = append(publicCases, id)
		}
	}
	sort.Strings(privateCases)
	sort.Strings(publicCases)
	return privateCases, publicCases, nil
}

func privateBenchmarkResults(results []BenchmarkCaseResult, privateCases []string) ([]BenchmarkCaseResult, error) {
	byID := map[string]BenchmarkCaseResult{}
	for _, result := range results {
		byID[result.CaseID] = result
	}
	private := make([]BenchmarkCaseResult, 0, len(privateCases))
	for _, id := range privateCases {
		result, ok := byID[id]
		if !ok {
			return nil, fmt.Errorf("private case %q was not present in benchmark run", id)
		}
		private = append(private, result)
	}
	sort.Slice(private, func(i, j int) bool { return private[i].CaseID < private[j].CaseID })
	return private, nil
}

func federatedAggregateMetrics(cases []BenchmarkCaseResult, minPrivateCases int) (FederatedBenchmarkAggregateMetrics, error) {
	if len(cases) < minPrivateCases {
		return FederatedBenchmarkAggregateMetrics{}, fmt.Errorf("private benchmark result has %d case(s), below k-anonymity minimum %d", len(cases), minPrivateCases)
	}
	raw := map[string]int{}
	for _, c := range cases {
		if c.OK {
			raw["matched"]++
		} else {
			raw["mismatched"]++
		}
		actual := strings.TrimSpace(c.ActualResult)
		if actual == "" {
			actual = "unknown"
		}
		raw["actual:"+actual]++
	}
	buckets := map[string]int{}
	suppressed := 0
	for _, key := range sortedIntKeys(raw) {
		count := raw[key]
		if count >= minPrivateCases {
			buckets[key] = count
		} else if count > 0 {
			suppressed++
		}
	}
	return FederatedBenchmarkAggregateMetrics{
		Total:             len(cases),
		Buckets:           buckets,
		SuppressedBuckets: suppressed,
	}, nil
}

func validateFederatedAggregatePrivacy(aggregate FederatedBenchmarkAggregate) []string {
	var errors []string
	if aggregate.MinPrivateCases < 2 {
		errors = append(errors, "min_private_cases must be at least 2")
	}
	if aggregate.PrivateCaseCount < aggregate.MinPrivateCases {
		errors = append(errors, "private_case_count is below min_private_cases")
	}
	if aggregate.TotalCaseCount != aggregate.PrivateCaseCount+aggregate.PublicCaseCount {
		errors = append(errors, "total_case_count must equal private_case_count + public_case_count")
	}
	if aggregate.Metrics.Total != aggregate.PrivateCaseCount {
		errors = append(errors, "metrics.total must equal private_case_count")
	}
	if strings.TrimSpace(aggregate.ManifestHash) == "" || strings.TrimSpace(aggregate.SplitHash) == "" || strings.TrimSpace(aggregate.PartitionHash) == "" {
		errors = append(errors, "manifest_hash, split_hash, and partition_hash are required")
	}
	if aggregate.Metrics.SuppressedBuckets < 0 {
		errors = append(errors, "suppressed_buckets must be non-negative")
	}
	for key, count := range aggregate.Metrics.Buckets {
		if strings.TrimSpace(key) == "" {
			errors = append(errors, "metrics bucket key is required")
		}
		if count < aggregate.MinPrivateCases {
			errors = append(errors, fmt.Sprintf("metrics bucket %q count %d is below min_private_cases %d", key, count, aggregate.MinPrivateCases))
		}
	}
	return errors
}

func federatedPartitionSalt(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value != "" {
		decoded, err := hex.DecodeString(value)
		if err != nil {
			return "", fmt.Errorf("partition salt must be hex: %w", err)
		}
		if len(decoded) < 16 {
			return "", fmt.Errorf("partition salt must be at least 16 bytes")
		}
		return strings.ToLower(value), nil
	}
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	return hex.EncodeToString(salt), nil
}

func normalizeFederatedMinPrivateCases(value int) int {
	if value <= 0 {
		return DefaultFederatedMinPrivateCases
	}
	return value
}

func federatedPartitionHash(split FederatedBenchmarkSplit) string {
	return "sha256:" + canonical.Hash(federatedBenchmarkPartitionCommitment{
		Version:       split.Version,
		DatasetID:     split.DatasetID,
		AdopterID:     split.AdopterID,
		ManifestHash:  split.ManifestHash,
		PartitionSalt: split.PartitionSalt,
		PrivateCases:  sortedStrings(split.PrivateCases),
		PublicCases:   sortedStrings(split.PublicCases),
	})
}

func federatedSplitHash(split FederatedBenchmarkSplit) string {
	copy := split
	copy.Hash = ""
	return "sha256:" + canonical.Hash(copy)
}

func federatedAggregateSubject(adopterID string) string {
	return "federated-benchmark-split:" + strings.TrimSpace(adopterID)
}

func federatedAggregatePayload(aggregate FederatedBenchmarkAggregate) federatedBenchmarkAggregatePayload {
	return federatedBenchmarkAggregatePayload{
		Version:          aggregate.Version,
		OK:               aggregate.OK,
		DatasetID:        aggregate.DatasetID,
		AdopterID:        aggregate.AdopterID,
		ManifestHash:     aggregate.ManifestHash,
		SplitHash:        aggregate.SplitHash,
		PartitionHash:    aggregate.PartitionHash,
		MinPrivateCases:  aggregate.MinPrivateCases,
		PrivateCaseCount: aggregate.PrivateCaseCount,
		PublicCaseCount:  aggregate.PublicCaseCount,
		TotalCaseCount:   aggregate.TotalCaseCount,
		Metrics:          aggregate.Metrics,
	}
}

func federatedAggregatePayloadBytes(aggregate FederatedBenchmarkAggregate) []byte {
	return canonical.MustBytes(federatedAggregatePayload(aggregate))
}

func federatedAggregateHash(aggregate FederatedBenchmarkAggregate) string {
	copy := aggregate
	copy.Hash = ""
	copy.Markdown = ""
	return "sha256:" + canonical.Hash(copy)
}

func federatedVerificationHash(report FederatedBenchmarkVerificationReport) string {
	copy := report
	copy.Hash = ""
	return "sha256:" + canonical.Hash(copy)
}

func renderFederatedAggregateMarkdown(aggregate FederatedBenchmarkAggregate) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Federated benchmark aggregate\n\n")
	fmt.Fprintf(&b, "Patchline evaluated `%d` adopter-local private benchmark cases and published only k-anonymous aggregate buckets signed by `%s`.\n\n", aggregate.PrivateCaseCount, aggregate.Signature.PublicKey)
	fmt.Fprintf(&b, "| Metric | Value |\n| --- | ---: |\n")
	fmt.Fprintf(&b, "| Private cases | %d |\n", aggregate.PrivateCaseCount)
	fmt.Fprintf(&b, "| Public cases in split | %d |\n", aggregate.PublicCaseCount)
	fmt.Fprintf(&b, "| Minimum private bucket size | %d |\n", aggregate.MinPrivateCases)
	fmt.Fprintf(&b, "| Suppressed low-count buckets | %d |\n", aggregate.Metrics.SuppressedBuckets)
	fmt.Fprintf(&b, "\n| Bucket | Count |\n| --- | ---: |\n")
	for _, key := range sortedIntKeys(aggregate.Metrics.Buckets) {
		fmt.Fprintf(&b, "| `%s` | %d |\n", key, aggregate.Metrics.Buckets[key])
	}
	fmt.Fprintf(&b, "\nSigned subject: `%s`\n", aggregate.Signature.Subject)
	return b.String()
}

func readStrictJSON(path string, out any) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(out); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return fmt.Errorf("unexpected trailing JSON in %s", path)
		}
		return err
	}
	if decoder.More() {
		return fmt.Errorf("unexpected trailing JSON in %s", path)
	}
	return nil
}

func sortedIntKeys(values map[string]int) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
