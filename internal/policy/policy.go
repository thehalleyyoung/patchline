package policy

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"

	"github.com/thehalleyyoung/patchline/internal/canonical"
	"github.com/thehalleyyoung/patchline/internal/ledger"
	"github.com/thehalleyyoung/patchline/internal/migration"
	"github.com/thehalleyyoung/patchline/internal/repair"
	"github.com/thehalleyyoung/patchline/internal/replay"
)

const Version = "patchline.policy/v1"

type Policy struct {
	Version string `json:"version"`
	Name    string `json:"name"`
	Rules   Rules  `json:"rules"`
}

type Rules struct {
	RequireSnapshotRollback bool     `json:"require_snapshot_rollback"`
	MaxChangedRows          *int     `json:"max_changed_rows,omitempty"`
	AllowHighRiskMigration  bool     `json:"allow_high_risk_migration"`
	RequirePinnedReportHash bool     `json:"require_pinned_report_hash"`
	AllowedEffects          []string `json:"allowed_effects,omitempty"`
	RequireLedgerCheckpoint bool     `json:"require_ledger_checkpoint"`
}

type Inputs struct {
	Manifest           repair.Manifest
	Report             replay.Report
	Migration          migration.Report
	ExpectedReportHash string
	LedgerCheckpoint   ledger.Checkpoint
}

type Evaluation struct {
	PolicyName string       `json:"policy_name"`
	PolicyHash string       `json:"policy_hash"`
	OK         bool         `json:"ok"`
	Results    []RuleResult `json:"results"`
}

type RuleResult struct {
	Rule    string `json:"rule"`
	OK      bool   `json:"ok"`
	Message string `json:"message"`
	Actual  string `json:"actual,omitempty"`
	Expect  string `json:"expect,omitempty"`
}

func Read(reader io.Reader) (Policy, error) {
	decoder := json.NewDecoder(reader)
	decoder.DisallowUnknownFields()
	var policy Policy
	if err := decoder.Decode(&policy); err != nil {
		return Policy{}, err
	}
	if policy.Version != Version {
		return Policy{}, fmt.Errorf("policy version must be %s", Version)
	}
	if policy.Name == "" {
		return Policy{}, fmt.Errorf("policy name is required")
	}
	return policy, nil
}

func Evaluate(policy Policy, inputs Inputs) Evaluation {
	eval := Evaluation{
		PolicyName: policy.Name,
		PolicyHash: canonical.Hash(policy),
		OK:         true,
	}
	add := func(rule string, ok bool, message, actual, expect string) {
		eval.Results = append(eval.Results, RuleResult{Rule: rule, OK: ok, Message: message, Actual: actual, Expect: expect})
		if !ok {
			eval.OK = false
		}
	}

	if policy.Rules.RequireSnapshotRollback {
		ok := inputs.Manifest.Rollback.Strategy == "snapshot" && inputs.Manifest.Rollback.SnapshotRequired
		add("require_snapshot_rollback", ok, "repair must require snapshot rollback", fmt.Sprintf("%s/%t", inputs.Manifest.Rollback.Strategy, inputs.Manifest.Rollback.SnapshotRequired), "snapshot/true")
	}
	if policy.Rules.MaxChangedRows != nil {
		actual := totalChangedRows(inputs.Report)
		max := *policy.Rules.MaxChangedRows
		add("max_changed_rows", actual <= max, "dry-run changed rows must not exceed policy maximum", fmt.Sprintf("%d", actual), fmt.Sprintf("<=%d", max))
	}
	if !policy.Rules.AllowHighRiskMigration && inputs.Migration.Summary.HighRisk > 0 {
		add("allow_high_risk_migration", false, "high-risk migration statements are not allowed by policy", fmt.Sprintf("%d", inputs.Migration.Summary.HighRisk), "0")
	} else {
		add("allow_high_risk_migration", true, "migration risk accepted by policy", fmt.Sprintf("%d", inputs.Migration.Summary.HighRisk), "accepted")
	}
	if policy.Rules.RequirePinnedReportHash {
		actual := inputs.Report.Hash()
		ok := inputs.ExpectedReportHash != "" && actual == inputs.ExpectedReportHash
		add("require_pinned_report_hash", ok, "dry-run report hash must be pinned and match", actual, inputs.ExpectedReportHash)
	}
	if len(policy.Rules.AllowedEffects) > 0 {
		allowed := map[string]bool{}
		for _, effect := range policy.Rules.AllowedEffects {
			allowed[effect] = true
		}
		var disallowed []string
		for _, op := range inputs.Report.Operations {
			if !allowed[op.Effect] {
				disallowed = append(disallowed, op.OperationID+":"+op.Effect)
			}
		}
		sort.Strings(disallowed)
		add("allowed_effects", len(disallowed) == 0, "all operation effects must be policy-allowed", fmt.Sprintf("%v", disallowed), fmt.Sprintf("%v", policy.Rules.AllowedEffects))
	}
	if policy.Rules.RequireLedgerCheckpoint {
		ok := inputs.LedgerCheckpoint.Count > 0 && inputs.LedgerCheckpoint.TipHash != ""
		add("require_ledger_checkpoint", ok, "ledger checkpoint must be present", fmt.Sprintf("%d/%s", inputs.LedgerCheckpoint.Count, inputs.LedgerCheckpoint.TipHash), "non-empty")
	}
	return eval
}

func totalChangedRows(report replay.Report) int {
	total := 0
	for _, op := range report.Operations {
		total += len(op.Diffs)
	}
	return total
}
