package certrevocation

import (
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/thehalleyyoung/patchline/internal/attest"
	"github.com/thehalleyyoung/patchline/internal/canonical"
	"github.com/thehalleyyoung/patchline/internal/certlang"
	"github.com/thehalleyyoung/patchline/internal/ledger"
)

const (
	ProtocolVersion = "patchline.certificate-revocation/v1"
	BundleVersion   = "patchline.certificate-revocation-bundle/v1"
	ReportVersion   = "patchline.certificate-revocation-replay/v1"
)

type Bundle struct {
	Version           string             `json:"version"`
	KnownCertificates []KnownCertificate `json:"known_certificates"`
	Ledger            []ledger.Entry     `json:"ledger"`
	Records           []Record           `json:"records"`
}

type KnownCertificate struct {
	CertificateID   string `json:"certificate_id"`
	CanonicalSHA256 string `json:"canonical_sha256"`
}

type Record struct {
	Version           string        `json:"version"`
	Action            string        `json:"action"`
	CertificateID     string        `json:"certificate_id"`
	CertificateSHA256 string        `json:"certificate_sha256"`
	ReplacementID     string        `json:"replacement_id,omitempty"`
	ReplacementSHA256 string        `json:"replacement_sha256,omitempty"`
	Reason            string        `json:"reason"`
	IssuedAt          string        `json:"issued_at"`
	Attestation       attest.Signed `json:"attestation"`
}

type State struct {
	CertificateID     string `json:"certificate_id"`
	CanonicalSHA256   string `json:"canonical_sha256"`
	Status            string `json:"status"`
	ReplacementID     string `json:"replacement_id,omitempty"`
	ReplacementSHA256 string `json:"replacement_sha256,omitempty"`
	Reason            string `json:"reason,omitempty"`
	EntryIndex        int    `json:"entry_index,omitempty"`
}

type ReplayReport struct {
	Version    string            `json:"version"`
	Records    int               `json:"records"`
	Active     int               `json:"active"`
	Revoked    int               `json:"revoked"`
	Superseded int               `json:"superseded"`
	AllOK      bool              `json:"all_ok"`
	Checkpoint ledger.Checkpoint `json:"checkpoint"`
	States     []State           `json:"states"`
}

type payload struct {
	Version           string `json:"version"`
	Action            string `json:"action"`
	CertificateID     string `json:"certificate_id"`
	CertificateSHA256 string `json:"certificate_sha256"`
	ReplacementID     string `json:"replacement_id,omitempty"`
	ReplacementSHA256 string `json:"replacement_sha256,omitempty"`
	Reason            string `json:"reason"`
	IssuedAt          string `json:"issued_at"`
}

func KnownFromBytes(data []byte, opts certlang.Options) (KnownCertificate, error) {
	normalized, err := certlang.Normalize(data, opts)
	if err != nil {
		return KnownCertificate{}, err
	}
	return KnownCertificate{
		CertificateID:   normalized.CertificateID,
		CanonicalSHA256: normalized.NormalizedCanonicalSHA256,
	}, nil
}

func Create(action string, certificateData []byte, replacementData []byte, reason string, issuedAt string, opts certlang.Options, seed []byte) (Record, error) {
	cert, err := KnownFromBytes(certificateData, opts)
	if err != nil {
		return Record{}, fmt.Errorf("certificate: %w", err)
	}
	record := Record{
		Version:           ProtocolVersion,
		Action:            action,
		CertificateID:     cert.CertificateID,
		CertificateSHA256: cert.CanonicalSHA256,
		Reason:            reason,
		IssuedAt:          issuedAt,
	}
	if len(replacementData) > 0 {
		replacement, err := KnownFromBytes(replacementData, opts)
		if err != nil {
			return Record{}, fmt.Errorf("replacement certificate: %w", err)
		}
		record.ReplacementID = replacement.CertificateID
		record.ReplacementSHA256 = replacement.CanonicalSHA256
	}
	if err := validateRecordFields(record); err != nil {
		return Record{}, err
	}
	body := PayloadBytes(record)
	statement, err := attest.Sign("certificate-revocation:"+record.CertificateID, body, seed)
	if err != nil {
		return Record{}, err
	}
	record.Attestation = statement
	return record, nil
}

func AppendRecord(entries []ledger.Entry, record Record) ([]ledger.Entry, error) {
	if err := VerifyRecord(record); err != nil {
		return nil, err
	}
	return ledger.Append(entries, ledgerKind(record.Action), record.CertificateID, PayloadBytes(record))
}

func Replay(entries []ledger.Entry, records []Record, known []KnownCertificate) (ReplayReport, error) {
	if len(entries) != len(records) {
		return ReplayReport{}, fmt.Errorf("ledger entries (%d) and records (%d) differ", len(entries), len(records))
	}
	if err := ledger.Verify(entries); err != nil {
		return ReplayReport{}, err
	}
	states := map[string]State{}
	for _, cert := range known {
		if cert.CertificateID == "" || cert.CanonicalSHA256 == "" {
			return ReplayReport{}, errors.New("known certificates require id and canonical hash")
		}
		if _, exists := states[cert.CertificateID]; exists {
			return ReplayReport{}, fmt.Errorf("duplicate known certificate %q", cert.CertificateID)
		}
		states[cert.CertificateID] = State{
			CertificateID:   cert.CertificateID,
			CanonicalSHA256: cert.CanonicalSHA256,
			Status:          "active",
		}
	}
	for i, record := range records {
		if err := VerifyRecord(record); err != nil {
			return ReplayReport{}, fmt.Errorf("record %d: %w", i, err)
		}
		entry := entries[i]
		payloadBytes := PayloadBytes(record)
		if entry.PayloadHash != canonical.HashBytes(payloadBytes) {
			return ReplayReport{}, fmt.Errorf("entry %d payload hash mismatch", i)
		}
		if entry.Kind != ledgerKind(record.Action) {
			return ReplayReport{}, fmt.Errorf("entry %d kind got %q want %q", i, entry.Kind, ledgerKind(record.Action))
		}
		if entry.Subject != record.CertificateID {
			return ReplayReport{}, fmt.Errorf("entry %d subject got %q want %q", i, entry.Subject, record.CertificateID)
		}
		state, ok := states[record.CertificateID]
		if !ok {
			return ReplayReport{}, fmt.Errorf("record %d revokes unknown certificate %q", i, record.CertificateID)
		}
		if state.CanonicalSHA256 != record.CertificateSHA256 {
			return ReplayReport{}, fmt.Errorf("record %d certificate hash got %s want %s", i, record.CertificateSHA256, state.CanonicalSHA256)
		}
		if state.Status != "active" {
			return ReplayReport{}, fmt.Errorf("record %d cannot change terminal %s certificate %q", i, state.Status, record.CertificateID)
		}
		switch record.Action {
		case "revoke":
			state.Status = "revoked"
		case "supersede":
			replacement, ok := states[record.ReplacementID]
			if !ok {
				return ReplayReport{}, fmt.Errorf("record %d supersedes to unknown certificate %q", i, record.ReplacementID)
			}
			if replacement.CanonicalSHA256 != record.ReplacementSHA256 {
				return ReplayReport{}, fmt.Errorf("record %d replacement hash got %s want %s", i, record.ReplacementSHA256, replacement.CanonicalSHA256)
			}
			if replacement.Status != "active" {
				return ReplayReport{}, fmt.Errorf("record %d replacement certificate %q is %s", i, record.ReplacementID, replacement.Status)
			}
			state.Status = "superseded"
			state.ReplacementID = record.ReplacementID
			state.ReplacementSHA256 = record.ReplacementSHA256
		default:
			return ReplayReport{}, fmt.Errorf("record %d unsupported action %q", i, record.Action)
		}
		state.Reason = record.Reason
		state.EntryIndex = i
		states[record.CertificateID] = state
	}
	checkpoint, err := ledger.CheckpointFor(entries)
	if err != nil {
		return ReplayReport{}, err
	}
	report := ReplayReport{
		Version:    ReportVersion,
		Records:    len(records),
		AllOK:      true,
		Checkpoint: checkpoint,
		States:     sortedStates(states),
	}
	for _, state := range report.States {
		switch state.Status {
		case "active":
			report.Active++
		case "revoked":
			report.Revoked++
		case "superseded":
			report.Superseded++
		}
	}
	return report, nil
}

func ReplayBundle(bundle Bundle) (ReplayReport, error) {
	if bundle.Version != BundleVersion {
		return ReplayReport{}, fmt.Errorf("revocation bundle version must be %s", BundleVersion)
	}
	return Replay(bundle.Ledger, bundle.Records, bundle.KnownCertificates)
}

func VerifyRecord(record Record) error {
	if err := validateRecordFields(record); err != nil {
		return err
	}
	return attest.VerifySignature(record.Attestation, PayloadBytes(record))
}

func PayloadBytes(record Record) []byte {
	return canonical.MustBytes(recordPayload(record))
}

func recordPayload(record Record) payload {
	return payload{
		Version:           record.Version,
		Action:            record.Action,
		CertificateID:     record.CertificateID,
		CertificateSHA256: record.CertificateSHA256,
		ReplacementID:     record.ReplacementID,
		ReplacementSHA256: record.ReplacementSHA256,
		Reason:            record.Reason,
		IssuedAt:          record.IssuedAt,
	}
}

func validateRecordFields(record Record) error {
	if record.Version != ProtocolVersion {
		return fmt.Errorf("revocation version must be %s", ProtocolVersion)
	}
	if record.CertificateID == "" || record.CertificateSHA256 == "" {
		return errors.New("certificate id and hash are required")
	}
	if record.Reason == "" {
		return errors.New("revocation reason is required")
	}
	if _, err := time.Parse(time.RFC3339, record.IssuedAt); err != nil {
		return fmt.Errorf("issued_at must be RFC3339: %w", err)
	}
	switch record.Action {
	case "revoke":
		if record.ReplacementID != "" || record.ReplacementSHA256 != "" {
			return errors.New("revoke record must not include replacement certificate")
		}
	case "supersede":
		if record.ReplacementID == "" || record.ReplacementSHA256 == "" {
			return errors.New("supersede record requires replacement certificate")
		}
		if record.ReplacementID == record.CertificateID {
			return errors.New("certificate cannot supersede itself")
		}
	default:
		return fmt.Errorf("unsupported revocation action %q", record.Action)
	}
	return nil
}

func ledgerKind(action string) string {
	switch action {
	case "revoke":
		return "certificate.revoked"
	case "supersede":
		return "certificate.superseded"
	default:
		return "certificate." + action
	}
}

func sortedStates(states map[string]State) []State {
	out := make([]State, 0, len(states))
	for _, state := range states {
		out = append(out, state)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].CertificateID < out[j].CertificateID
	})
	return out
}
