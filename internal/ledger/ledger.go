package ledger

import (
	"fmt"

	"github.com/patchline/patchline/internal/canonical"
)

const domain = "patchline-ledger-v1"

type Entry struct {
	Index       int    `json:"index"`
	Kind        string `json:"kind"`
	Subject     string `json:"subject"`
	PayloadHash string `json:"payload_hash"`
	PrevHash    string `json:"prev_hash"`
	Hash        string `json:"hash"`
}

type Checkpoint struct {
	Count   int    `json:"count"`
	TipHash string `json:"tip_hash"`
}

func Append(entries []Entry, kind, subject string, payload []byte) ([]Entry, error) {
	if kind == "" {
		return nil, fmt.Errorf("ledger entry kind is required")
	}
	if subject == "" {
		return nil, fmt.Errorf("ledger entry subject is required")
	}
	if err := Verify(entries); err != nil {
		return nil, err
	}

	prevHash := ""
	if len(entries) > 0 {
		prevHash = entries[len(entries)-1].Hash
	}
	entry := Entry{
		Index:       len(entries),
		Kind:        kind,
		Subject:     subject,
		PayloadHash: hashBytes(payload),
		PrevHash:    prevHash,
	}
	entry.Hash = computeHash(entry)
	return append(append([]Entry(nil), entries...), entry), nil
}

func Verify(entries []Entry) error {
	for i, entry := range entries {
		if entry.Index != i {
			return fmt.Errorf("entry %d has index %d", i, entry.Index)
		}
		wantPrev := ""
		if i > 0 {
			wantPrev = entries[i-1].Hash
		}
		if entry.PrevHash != wantPrev {
			return fmt.Errorf("entry %d previous hash mismatch", i)
		}
		if got := computeHash(entry); got != entry.Hash {
			return fmt.Errorf("entry %d hash mismatch", i)
		}
	}
	return nil
}

func CheckpointFor(entries []Entry) (Checkpoint, error) {
	if err := Verify(entries); err != nil {
		return Checkpoint{}, err
	}
	checkpoint := Checkpoint{Count: len(entries)}
	if len(entries) > 0 {
		checkpoint.TipHash = entries[len(entries)-1].Hash
	}
	return checkpoint, nil
}

func VerifyCheckpoint(entries []Entry, checkpoint Checkpoint) error {
	if err := Verify(entries); err != nil {
		return err
	}
	if len(entries) != checkpoint.Count {
		return fmt.Errorf("checkpoint count mismatch: got %d, want %d", len(entries), checkpoint.Count)
	}
	tip := ""
	if len(entries) > 0 {
		tip = entries[len(entries)-1].Hash
	}
	if tip != checkpoint.TipHash {
		return fmt.Errorf("checkpoint tip mismatch")
	}
	return nil
}

func computeHash(entry Entry) string {
	preimage := struct {
		Domain      string `json:"domain"`
		Index       int    `json:"index"`
		Kind        string `json:"kind"`
		Subject     string `json:"subject"`
		PayloadHash string `json:"payload_hash"`
		PrevHash    string `json:"prev_hash"`
	}{
		Domain:      domain,
		Index:       entry.Index,
		Kind:        entry.Kind,
		Subject:     entry.Subject,
		PayloadHash: entry.PayloadHash,
		PrevHash:    entry.PrevHash,
	}
	return canonical.Hash(preimage)
}

func hashBytes(bytes []byte) string {
	return canonical.HashBytes(bytes)
}
