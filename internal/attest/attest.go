package attest

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"github.com/patchline/patchline/internal/canonical"
	"github.com/patchline/patchline/internal/migration"
	"github.com/patchline/patchline/internal/repair"
	"github.com/patchline/patchline/internal/replay"
)

const SignatureVersion = "patchline.signed-attestation/v1"

type Check struct {
	Kind   string `json:"kind"`
	Ref    string `json:"ref,omitempty"`
	Expect string `json:"expect"`
}

type Result struct {
	Kind     string `json:"kind"`
	Ref      string `json:"ref,omitempty"`
	OK       bool   `json:"ok"`
	Expected string `json:"expected"`
	Actual   string `json:"actual"`
	Message  string `json:"message,omitempty"`
}

type Signed struct {
	Version      string `json:"version"`
	Algorithm    string `json:"algorithm"`
	Subject      string `json:"subject"`
	ArtifactHash string `json:"artifact_hash"`
	PublicKey    string `json:"public_key"`
	Signature    string `json:"signature"`
}

func Verify(report replay.Report, manifest repair.Manifest, checks []Check) []Result {
	results := make([]Result, 0, len(checks))
	for _, check := range checks {
		results = append(results, verifyOne(report, manifest, check))
	}
	return results
}

func OK(results []Result) bool {
	for _, result := range results {
		if !result.OK {
			return false
		}
	}
	return true
}

func GenerateSeed() ([]byte, error) {
	seed := make([]byte, ed25519.SeedSize)
	if _, err := rand.Read(seed); err != nil {
		return nil, err
	}
	return seed, nil
}

func SeedFromHex(value string) ([]byte, error) {
	seed, err := hex.DecodeString(value)
	if err != nil {
		return nil, err
	}
	if len(seed) != ed25519.SeedSize {
		return nil, fmt.Errorf("ed25519 seed must be %d bytes", ed25519.SeedSize)
	}
	return seed, nil
}

func SeedHex(seed []byte) string {
	return hex.EncodeToString(seed)
}

func PublicKeyHex(seed []byte) (string, error) {
	if len(seed) != ed25519.SeedSize {
		return "", fmt.Errorf("ed25519 seed must be %d bytes", ed25519.SeedSize)
	}
	privateKey := ed25519.NewKeyFromSeed(seed)
	return hex.EncodeToString(privateKey.Public().(ed25519.PublicKey)), nil
}

func Sign(subject string, artifact []byte, seed []byte) (Signed, error) {
	if subject == "" {
		return Signed{}, errors.New("attestation subject is required")
	}
	if len(seed) != ed25519.SeedSize {
		return Signed{}, fmt.Errorf("ed25519 seed must be %d bytes", ed25519.SeedSize)
	}
	privateKey := ed25519.NewKeyFromSeed(seed)
	publicKey := privateKey.Public().(ed25519.PublicKey)
	statement := Signed{
		Version:      SignatureVersion,
		Algorithm:    "ed25519",
		Subject:      subject,
		ArtifactHash: artifactHash(artifact),
		PublicKey:    hex.EncodeToString(publicKey),
	}
	signature := ed25519.Sign(privateKey, statementMessage(statement))
	statement.Signature = hex.EncodeToString(signature)
	return statement, nil
}

func VerifySignature(statement Signed, artifact []byte) error {
	if statement.Version != SignatureVersion {
		return fmt.Errorf("attestation version must be %s", SignatureVersion)
	}
	if statement.Algorithm != "ed25519" {
		return fmt.Errorf("unsupported attestation algorithm %q", statement.Algorithm)
	}
	if statement.Subject == "" {
		return errors.New("attestation subject is required")
	}
	if statement.ArtifactHash != artifactHash(artifact) {
		return fmt.Errorf("artifact hash mismatch: attestation=%s actual=%s", statement.ArtifactHash, artifactHash(artifact))
	}
	publicKey, err := hex.DecodeString(statement.PublicKey)
	if err != nil {
		return err
	}
	if len(publicKey) != ed25519.PublicKeySize {
		return fmt.Errorf("ed25519 public key must be %d bytes", ed25519.PublicKeySize)
	}
	signature, err := hex.DecodeString(statement.Signature)
	if err != nil {
		return err
	}
	if len(signature) != ed25519.SignatureSize {
		return fmt.Errorf("ed25519 signature must be %d bytes", ed25519.SignatureSize)
	}
	if !ed25519.Verify(ed25519.PublicKey(publicKey), statementMessage(statement), signature) {
		return errors.New("attestation signature verification failed")
	}
	return nil
}

func verifyOne(report replay.Report, manifest repair.Manifest, check Check) Result {
	result := Result{Kind: check.Kind, Ref: check.Ref, Expected: check.Expect}
	switch check.Kind {
	case "report_hash_equals":
		result.Actual = report.Hash()
		result.OK = result.Actual == check.Expect
	case "max_changed_rows":
		max, err := migration.ParsePositiveInt(check.Expect)
		if err != nil {
			result.Message = err.Error()
			return result
		}
		actual := totalChangedRows(report)
		result.Actual = fmt.Sprintf("%d", actual)
		result.OK = actual <= max
	case "operation_effect_equals":
		actual, ok := operationEffect(report, check.Ref)
		result.Actual = actual
		result.OK = ok && actual == check.Expect
		if !ok {
			result.Message = "operation not found"
		}
	case "changed_row_equals":
		actual, ok := changedRowValue(report, check.Ref)
		result.Actual = actual
		result.OK = ok && actual == check.Expect
		if !ok {
			result.Message = "changed row value not found"
		}
	case "downstream_contains":
		result.Actual = "false"
		for _, entityID := range report.DownstreamEntities {
			if entityID == check.Ref {
				result.Actual = "true"
				break
			}
		}
		result.OK = result.Actual == check.Expect
	case "no_unscoped_updates":
		result.Actual = fmt.Sprintf("%t", noUnscopedUpdates(report, manifest))
		result.OK = result.Actual == check.Expect
	default:
		result.Message = "unknown attestation check kind"
	}
	return result
}

func artifactHash(artifact []byte) string {
	sum := sha256.Sum256(artifact)
	return hex.EncodeToString(sum[:])
}

func statementMessage(statement Signed) []byte {
	statement.Signature = ""
	return []byte(canonical.Hash(statement))
}

func totalChangedRows(report replay.Report) int {
	total := 0
	for _, op := range report.Operations {
		total += len(op.Diffs)
	}
	return total
}

func operationEffect(report replay.Report, operationID string) (string, bool) {
	for _, op := range report.Operations {
		if op.OperationID == operationID {
			return op.Effect, true
		}
	}
	return "", false
}

func changedRowValue(report replay.Report, ref string) (string, bool) {
	table, id, column, ok := parseChangedRowRef(ref)
	if !ok {
		return "", false
	}
	for _, op := range report.Operations {
		for _, diff := range op.Diffs {
			if diff.Table == table && diff.ID == id {
				change, ok := diff.Changes[column]
				if ok {
					return change.After, true
				}
			}
		}
	}
	return "", false
}

func parseChangedRowRef(ref string) (string, string, string, bool) {
	columnStart := strings.LastIndex(ref, ".")
	rowSep := strings.Index(ref, "/")
	if columnStart <= 0 || rowSep <= 0 || rowSep > columnStart {
		return "", "", "", false
	}
	return ref[:rowSep], ref[rowSep+1 : columnStart], ref[columnStart+1:], true
}

func noUnscopedUpdates(report replay.Report, manifest repair.Manifest) bool {
	if manifest.Scope.Table == "" {
		return len(report.Operations) == 0
	}
	scopedID := manifest.Scope.Where["id"]
	for _, op := range report.Operations {
		if op.Table != manifest.Scope.Table {
			return false
		}
		for _, diff := range op.Diffs {
			if diff.Table != manifest.Scope.Table {
				return false
			}
			if scopedID != "" && diff.ID != scopedID {
				return false
			}
		}
	}
	return true
}
