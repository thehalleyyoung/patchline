package ledger

import "testing"

func TestVerifyDetectsTampering(t *testing.T) {
	entries := sampleLedger(t)
	entries[0].Subject = "repair:other"
	if err := Verify(entries); err == nil {
		t.Fatal("expected tamper detection")
	}
}

func TestVerifyCheckpointDetectsTruncation(t *testing.T) {
	entries := sampleLedger(t)
	checkpoint, err := CheckpointFor(entries)
	if err != nil {
		t.Fatal(err)
	}
	truncated := entries[:1]
	if err := VerifyCheckpoint(truncated, checkpoint); err == nil {
		t.Fatal("expected truncation detection")
	}
}

func TestAppendCopiesInput(t *testing.T) {
	entries := sampleLedger(t)
	next, err := Append(entries, "repair.verified", "repair:1", []byte(`{"ok":true}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("append mutated input slice length: %d", len(entries))
	}
	if len(next) != 3 {
		t.Fatalf("expected three entries, got %d", len(next))
	}
}

func sampleLedger(t *testing.T) []Entry {
	t.Helper()
	var entries []Entry
	var err error
	entries, err = Append(entries, "repair.planned", "repair:1", []byte(`{"name":"repair"}`))
	if err != nil {
		t.Fatal(err)
	}
	entries, err = Append(entries, "repair.applied", "repair:1", []byte(`{"rows":1}`))
	if err != nil {
		t.Fatal(err)
	}
	return entries
}
