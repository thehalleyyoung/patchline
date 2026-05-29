package main

import (
	"errors"
	"testing"
)

func TestExitCodeDefaultsToUsageOrGenericFailure(t *testing.T) {
	if got := exitCode(errors.New("boom")); got != 1 {
		t.Fatalf("expected default exit code 1, got %d", got)
	}
}

func TestExitCodeUsesCodedError(t *testing.T) {
	err := codedError{code: 3, err: errors.New("threshold failed")}
	if got := exitCode(err); got != 3 {
		t.Fatalf("expected coded exit code 3, got %d", got)
	}
}

func TestParseGateOptionsRejectsInvalidThresholds(t *testing.T) {
	_, err := parseGateOptions([]string{"--min-precision", "1.5"})
	if err == nil {
		t.Fatal("expected invalid threshold error")
	}
}
