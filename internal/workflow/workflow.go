package workflow

import (
	"fmt"
	"sort"
	"strings"

	"github.com/thehalleyyoung/patchline/internal/canonical"
	"github.com/thehalleyyoung/patchline/internal/ledger"
	"github.com/thehalleyyoung/patchline/internal/proof"
)

const Version = "patchline.workflow-model/v1"

type Action string

const (
	ActionIngest   Action = "ingest"
	ActionExplain  Action = "explain"
	ActionApprove  Action = "approve"
	ActionDryRun   Action = "dry_run"
	ActionApply    Action = "apply"
	ActionVerify   Action = "verify"
	ActionRollback Action = "rollback"
	ActionAudit    Action = "audit"
	ActionArchive  Action = "archive"
)

type Descriptor struct {
	Version           string            `json:"version"`
	Name              string            `json:"name"`
	Bound             int               `json:"bound"`
	EvidenceHash      string            `json:"evidence_hash"`
	ManifestHash      string            `json:"manifest_hash"`
	PolicyHash        string            `json:"policy_hash"`
	DryRunHash        string            `json:"dry_run_hash"`
	SemanticAuditHash string            `json:"semantic_audit_hash,omitempty"`
	BundleHash        string            `json:"bundle_hash,omitempty"`
	LedgerCheckpoint  ledger.Checkpoint `json:"ledger_checkpoint"`
	PolicyAllowed     bool              `json:"policy_allowed"`
	RollbackAvailable bool              `json:"rollback_available"`
	Witness           []Action          `json:"witness"`
}

type Report struct {
	Version         string             `json:"version"`
	Name            string             `json:"name"`
	Bound           int                `json:"bound"`
	StatesExplored  int                `json:"states_explored"`
	Transitions     int                `json:"transitions"`
	ReachableTraces int                `json:"reachable_traces"`
	Witness         WitnessResult      `json:"witness"`
	Properties      []PropertyResult   `json:"properties"`
	Obligations     []proof.Obligation `json:"proof_obligations"`
	ProofHoles      []proof.Hole       `json:"proof_holes,omitempty"`
	Counterexamples []Counterexample   `json:"counterexamples,omitempty"`
	Hash            string             `json:"hash"`
}

type WitnessResult struct {
	Actions []Action `json:"actions"`
	Status  string   `json:"status"`
	Reason  string   `json:"reason"`
}

type PropertyResult struct {
	Ref     string       `json:"ref"`
	Status  proof.Status `json:"status"`
	Formula string       `json:"formula"`
	Reason  string       `json:"reason"`
	Witness []Action     `json:"witness,omitempty"`
	Checked int          `json:"checked_traces"`
}

type Counterexample struct {
	Ref     string   `json:"ref"`
	Message string   `json:"message"`
	Trace   []Action `json:"trace,omitempty"`
}

type state struct {
	Ingested   bool
	Explained  bool
	Approved   bool
	DryRun     bool
	Applied    bool
	Verified   bool
	RolledBack bool
	Audited    bool
	Archived   bool
}

func Check(descriptor Descriptor) Report {
	if descriptor.Bound <= 0 {
		descriptor.Bound = 9
	}
	traces := enumerate(descriptor)
	report := Report{
		Version:         Version,
		Name:            descriptor.Name,
		Bound:           descriptor.Bound,
		ReachableTraces: len(traces),
		Witness:         validateWitness(descriptor),
	}
	seenStates := map[string]bool{}
	for _, trace := range traces {
		s := state{}
		seenStates[stateKey(s)] = true
		for _, action := range trace {
			s = apply(s, action)
			seenStates[stateKey(s)] = true
			report.Transitions++
		}
	}
	report.StatesExplored = len(seenStates)
	report.Properties = []PropertyResult{
		checkApplyAfterApproval(traces, report.Witness),
		checkApprovalRequiresEvidence(descriptor, traces, report.Witness),
		checkEventualVerificationOrRollback(traces, descriptor.Bound),
		checkRollbackAvailability(descriptor, traces),
		checkImmutableAudit(descriptor, traces, report.Witness),
	}
	for _, property := range report.Properties {
		report.Obligations = append(report.Obligations, proof.Obligation{
			Ref:       property.Ref,
			Status:    property.Status,
			Formula:   property.Formula,
			Reason:    property.Reason,
			Producer:  "bounded-workflow-model-checker",
			Evidence:  descriptorHash(descriptor),
			Bound:     descriptor.Bound,
			Witness:   strings.Join(actionsToStrings(property.Witness), " -> "),
			Discharge: discharge(property.Status),
		})
		if property.Status == proof.StatusCounterexample {
			report.Counterexamples = append(report.Counterexamples, Counterexample{
				Ref:     property.Ref,
				Message: property.Reason,
				Trace:   property.Witness,
			})
		}
	}
	if report.Witness.Status == "invalid" {
		report.Counterexamples = append(report.Counterexamples, Counterexample{
			Ref:     "workflow.witness.valid",
			Message: report.Witness.Reason,
			Trace:   report.Witness.Actions,
		})
		report.Obligations = append(report.Obligations, proof.Obligation{
			Ref:       "workflow.witness.valid",
			Status:    proof.StatusCounterexample,
			Formula:   "witness trace follows workflow transition relation",
			Reason:    report.Witness.Reason,
			Producer:  "bounded-workflow-model-checker",
			Evidence:  descriptorHash(descriptor),
			Bound:     descriptor.Bound,
			Witness:   strings.Join(actionsToStrings(report.Witness.Actions), " -> "),
			Discharge: "fix witness ordering or provide missing artifact evidence",
		})
	}
	report.ProofHoles = proof.Holes(report.Obligations)
	report.Hash = canonical.Hash(struct {
		Version         string             `json:"version"`
		Name            string             `json:"name"`
		Bound           int                `json:"bound"`
		StatesExplored  int                `json:"states_explored"`
		Transitions     int                `json:"transitions"`
		ReachableTraces int                `json:"reachable_traces"`
		Witness         WitnessResult      `json:"witness"`
		Properties      []PropertyResult   `json:"properties"`
		Obligations     []proof.Obligation `json:"proof_obligations"`
		ProofHoles      []proof.Hole       `json:"proof_holes,omitempty"`
		Counterexamples []Counterexample   `json:"counterexamples,omitempty"`
	}{report.Version, report.Name, report.Bound, report.StatesExplored, report.Transitions, report.ReachableTraces, report.Witness, report.Properties, report.Obligations, report.ProofHoles, report.Counterexamples})
	return report
}

func enumerate(descriptor Descriptor) [][]Action {
	var traces [][]Action
	var walk func(state, []Action)
	walk = func(s state, prefix []Action) {
		traces = append(traces, append([]Action(nil), prefix...))
		if len(prefix) >= descriptor.Bound {
			return
		}
		for _, action := range enabledActions(descriptor, s) {
			next := apply(s, action)
			walk(next, append(prefix, action))
		}
	}
	walk(state{}, nil)
	sort.Slice(traces, func(i, j int) bool {
		return strings.Join(actionsToStrings(traces[i]), "/") < strings.Join(actionsToStrings(traces[j]), "/")
	})
	return traces
}

func enabledActions(descriptor Descriptor, s state) []Action {
	var out []Action
	if !s.Ingested && descriptor.EvidenceHash != "" {
		out = append(out, ActionIngest)
	}
	if s.Ingested && !s.Explained {
		out = append(out, ActionExplain)
	}
	if s.Explained && !s.Approved && descriptor.PolicyAllowed && descriptor.EvidenceHash != "" {
		out = append(out, ActionApprove)
	}
	if s.Explained && !s.DryRun && descriptor.DryRunHash != "" {
		out = append(out, ActionDryRun)
	}
	if s.Approved && s.DryRun && !s.Applied {
		out = append(out, ActionApply)
	}
	if s.Applied && !s.Verified && !s.RolledBack {
		out = append(out, ActionVerify)
	}
	if s.Applied && !s.Verified && !s.RolledBack && descriptor.RollbackAvailable {
		out = append(out, ActionRollback)
	}
	if (s.Verified || s.RolledBack) && !s.Audited && descriptor.LedgerCheckpoint.TipHash != "" {
		out = append(out, ActionAudit)
	}
	if s.Audited && !s.Archived {
		out = append(out, ActionArchive)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

func validateWitness(descriptor Descriptor) WitnessResult {
	result := WitnessResult{Actions: append([]Action(nil), descriptor.Witness...), Status: "valid"}
	s := state{}
	for i, action := range descriptor.Witness {
		if !containsAction(enabledActions(descriptor, s), action) {
			result.Status = "invalid"
			result.Reason = fmt.Sprintf("action %s at index %d is not enabled", action, i)
			return result
		}
		s = apply(s, action)
	}
	result.Reason = "witness follows the bounded workflow transition relation"
	return result
}

func checkApplyAfterApproval(traces [][]Action, witness WitnessResult) PropertyResult {
	result := PropertyResult{
		Ref:     "temporal.no_apply_before_approval",
		Status:  proof.StatusChecked,
		Formula: "always(apply -> once(approve))",
		Reason:  "all bounded reachable traces apply only after approval",
		Checked: len(traces),
	}
	for _, trace := range append(traces, witness.Actions) {
		approved := false
		for _, action := range trace {
			if action == ActionApprove {
				approved = true
			}
			if action == ActionApply && !approved {
				result.Status = proof.StatusCounterexample
				result.Reason = "trace applies a repair before approval"
				result.Witness = trace
				return result
			}
		}
	}
	return result
}

func checkApprovalRequiresEvidence(descriptor Descriptor, traces [][]Action, witness WitnessResult) PropertyResult {
	result := PropertyResult{
		Ref:     "temporal.no_approval_without_evidence",
		Status:  proof.StatusChecked,
		Formula: "always(approve -> evidence_hash != empty)",
		Reason:  "all approvals are guarded by evidence hash presence",
		Checked: len(traces),
	}
	if descriptor.EvidenceHash == "" {
		for _, trace := range append(traces, witness.Actions) {
			if containsAction(trace, ActionApprove) {
				result.Status = proof.StatusCounterexample
				result.Reason = "trace approves a repair without evidence"
				result.Witness = trace
				return result
			}
		}
	}
	return result
}

func checkEventualVerificationOrRollback(traces [][]Action, bound int) PropertyResult {
	result := PropertyResult{
		Ref:     "temporal.eventual_verification",
		Status:  proof.StatusChecked,
		Formula: "always(apply -> eventually(verify or rollback) within bound)",
		Reason:  "every bounded reachable applied trace can reach verification or rollback within the exploration bound",
		Checked: len(traces),
	}
	for _, trace := range traces {
		if containsAction(trace, ActionApply) && !containsAction(trace, ActionVerify) && !containsAction(trace, ActionRollback) && len(trace) >= bound {
			result.Status = proof.StatusCounterexample
			result.Reason = "bounded trace applies a repair without reaching verification or rollback"
			result.Witness = trace
			return result
		}
	}
	return result
}

func checkRollbackAvailability(descriptor Descriptor, traces [][]Action) PropertyResult {
	result := PropertyResult{
		Ref:     "temporal.rollback_available_after_apply",
		Status:  proof.StatusChecked,
		Formula: "always(apply -> rollback_available or verify)",
		Reason:  "applied traces either have rollback available or reach verification",
		Checked: len(traces),
	}
	if descriptor.RollbackAvailable {
		return result
	}
	for _, trace := range traces {
		if containsAction(trace, ActionApply) && !containsAction(trace, ActionVerify) {
			result.Status = proof.StatusAssumed
			result.Reason = "rollback is unavailable; safety depends on successful verification within the bound"
			result.Witness = trace
			return result
		}
	}
	return result
}

func checkImmutableAudit(descriptor Descriptor, traces [][]Action, witness WitnessResult) PropertyResult {
	result := PropertyResult{
		Ref:     "temporal.immutable_audit",
		Status:  proof.StatusChecked,
		Formula: "always(audit -> ledger_checkpoint.tip_hash != empty)",
		Reason:  "all audit transitions require a ledger checkpoint tip hash",
		Checked: len(traces),
	}
	if descriptor.LedgerCheckpoint.TipHash == "" {
		for _, trace := range append(traces, witness.Actions) {
			if containsAction(trace, ActionAudit) {
				result.Status = proof.StatusCounterexample
				result.Reason = "trace audits without an immutable ledger checkpoint"
				result.Witness = trace
				return result
			}
		}
	}
	return result
}

func apply(s state, action Action) state {
	switch action {
	case ActionIngest:
		s.Ingested = true
	case ActionExplain:
		s.Explained = true
	case ActionApprove:
		s.Approved = true
	case ActionDryRun:
		s.DryRun = true
	case ActionApply:
		s.Applied = true
	case ActionVerify:
		s.Verified = true
	case ActionRollback:
		s.RolledBack = true
	case ActionAudit:
		s.Audited = true
	case ActionArchive:
		s.Archived = true
	}
	return s
}

func stateKey(s state) string {
	return fmt.Sprintf("%t/%t/%t/%t/%t/%t/%t/%t/%t", s.Ingested, s.Explained, s.Approved, s.DryRun, s.Applied, s.Verified, s.RolledBack, s.Audited, s.Archived)
}

func descriptorHash(descriptor Descriptor) string {
	descriptor.Witness = nil
	return canonical.Hash(descriptor)
}

func discharge(status proof.Status) string {
	switch status {
	case proof.StatusAssumed:
		return "provide a stronger bound, rollback witness, or verification evidence"
	case proof.StatusNotSupported:
		return "extend the workflow transition fragment"
	case proof.StatusCounterexample:
		return "fix workflow ordering or required artifact evidence"
	default:
		return ""
	}
}

func containsAction(actions []Action, target Action) bool {
	for _, action := range actions {
		if action == target {
			return true
		}
	}
	return false
}

func actionsToStrings(actions []Action) []string {
	out := make([]string, 0, len(actions))
	for _, action := range actions {
		out = append(out, string(action))
	}
	return out
}
