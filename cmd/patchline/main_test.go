package main

import (
	"errors"
	"testing"

	"github.com/thehalleyyoung/patchline/internal/artifact"
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

func TestOnePositionalWithFlagsAllowsFlagsAfterPath(t *testing.T) {
	pos, flags, err := onePositionalWithFlags([]string{"django/django", "--subpath", "django/contrib/auth/migrations", "--json"}, map[string]bool{"--json": true})
	if err != nil {
		t.Fatal(err)
	}
	if pos != "django/django" {
		t.Fatalf("unexpected positional %q", pos)
	}
	if len(flags) != 3 || flags[0] != "--subpath" || flags[1] != "django/contrib/auth/migrations" || flags[2] != "--json" {
		t.Fatalf("unexpected flags %#v", flags)
	}
}

func TestPhaseCheckInputKindResolvesImplicitInputs(t *testing.T) {
	tests := []struct {
		name string
		c    artifact.ManifestCase
		want string
	}{
		{
			name: "migration",
			c:    artifact.ManifestCase{CaseType: "migration"},
			want: "migration_text",
		},
		{
			name: "incident override",
			c:    artifact.ManifestCase{CaseType: "incident", InputKind: "source_observations"},
			want: "source_observations",
		},
		{
			name: "inline postmortem",
			c:    artifact.ManifestCase{CaseType: "incident", Fixture: "inline:phase-guard"},
			want: "postmortem_text",
		},
		{
			name: "repair",
			c:    artifact.ManifestCase{CaseType: "repair"},
			want: "repair_plan",
		},
		{
			name: "archive regression",
			c:    artifact.ManifestCase{CaseType: "regression", Fixture: "archive.json"},
			want: "prior_archive",
		},
		{
			name: "migration regression",
			c:    artifact.ManifestCase{CaseType: "regression", Fixture: "fix.sql"},
			want: "migration_text",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := phaseCheckInputKind(tt.c); got != tt.want {
				t.Fatalf("phaseCheckInputKind() = %q, want %q", got, tt.want)
			}
		})
	}
}
