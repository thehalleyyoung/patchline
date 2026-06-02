package main

import (
	"encoding/json"
	"strings"
	"testing"
)

func FuzzBundleRedactor(f *testing.F) {
	f.Add("summary.json", `{"customer_email":"alice@example.com","token":"secret-token-123","hash":"abc123","message":"DELETE FROM accounts WHERE id = 'acct_1'"}`)
	f.Add("facts.jsonl", `{"path":"db/migrate/001.sql","literal":"acct_1"}`+"\n"+`{"email":"bob@example.com"}`)
	f.Add("summary.md", "customer alice@example.com used bearer token secret-token in db/migrate/001.sql")
	f.Fuzz(func(t *testing.T, path, content string) {
		if len(content) > 8192 {
			t.Skip()
		}
		redactor := newBundleRedactor()
		out, err := redactor.redactFileBytes(path, []byte(content))
		if strings.HasSuffix(path, ".json") && json.Valid([]byte(content)) && err != nil {
			t.Fatalf("valid JSON should redact without error: %v", err)
		}
		if err != nil {
			return
		}
		if len(out) == 0 && len(content) > 0 {
			t.Fatalf("redaction unexpectedly dropped all content")
		}
		lower := strings.ToLower(string(out))
		for _, forbidden := range []string{"alice@example.com", "bob@example.com", "secret-token-123"} {
			if strings.Contains(lower, forbidden) {
				t.Fatalf("redacted output leaked %q: %s", forbidden, string(out))
			}
		}
	})
}
