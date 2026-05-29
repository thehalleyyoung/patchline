package refinement

import (
	"fmt"
	"sort"
	"strings"

	"github.com/thehalleyyoung/patchline/internal/canonical"
	"github.com/thehalleyyoung/patchline/internal/invariant"
	"github.com/thehalleyyoung/patchline/internal/proof"
	"github.com/thehalleyyoung/patchline/internal/repair"
	"github.com/thehalleyyoung/patchline/internal/replay"
	"github.com/thehalleyyoung/patchline/internal/solver"
	"github.com/thehalleyyoung/patchline/internal/symbolic"
	"github.com/thehalleyyoung/patchline/internal/workflow"
)

const Version = "patchline.cegar-refinement/v1"

type Report struct {
	Version         string       `json:"version"`
	Manifest        string       `json:"manifest"`
	StoreHash       string       `json:"store_hash"`
	OK              bool         `json:"ok"`
	Iterations      []Iteration  `json:"iterations"`
	Refinements     []Action     `json:"refinements,omitempty"`
	RemainingHoles  []proof.Hole `json:"remaining_holes,omitempty"`
	Counterexamples []Finding    `json:"counterexamples,omitempty"`
	Hash            string       `json:"hash"`
}

type Iteration struct {
	Index       int          `json:"index"`
	Abstraction string       `json:"abstraction"`
	Inputs      []string     `json:"inputs"`
	Findings    []Finding    `json:"findings,omitempty"`
	Holes       []proof.Hole `json:"proof_holes,omitempty"`
	Hashes      Hashes       `json:"hashes"`
}

type Hashes struct {
	ReplaySemantics string `json:"replay_semantics"`
	Solver          string `json:"solver_obligations"`
	Symbolic        string `json:"symbolic_execution"`
	Workflow        string `json:"workflow_model_check,omitempty"`
}

type Finding struct {
	Ref     string `json:"ref"`
	Status  string `json:"status"`
	Message string `json:"message"`
	Witness string `json:"witness,omitempty"`
}

type Action struct {
	FromIteration int    `json:"from_iteration"`
	Kind          string `json:"kind"`
	Reason        string `json:"reason"`
	Effect        string `json:"effect"`
}

func Analyze(manifest repair.Manifest, store replay.Store, spec *invariant.Spec, descriptor *workflow.Descriptor) Report {
	report := Report{
		Version:   Version,
		Manifest:  manifest.Name,
		StoreHash: store.Hash(),
	}
	initial := runIteration(0, "row-level transition abstraction", manifest, store, nil, nil)
	report.Iterations = append(report.Iterations, initial)

	if spec != nil || descriptor != nil {
		report.Refinements = append(report.Refinements, plannedRefinements(initial, spec, descriptor)...)
		refined := runIteration(1, "evidence-refined invariant/workflow abstraction", manifest, store, spec, descriptor)
		report.Iterations = append(report.Iterations, refined)
		report.Refinements = markRefinementEffects(report.Refinements, initial, refined)
	}

	last := report.Iterations[len(report.Iterations)-1]
	report.RemainingHoles = append([]proof.Hole(nil), last.Holes...)
	for _, finding := range last.Findings {
		if finding.Status == string(proof.StatusCounterexample) {
			report.Counterexamples = append(report.Counterexamples, finding)
		}
	}
	report.OK = len(report.RemainingHoles) == 0 && len(report.Counterexamples) == 0
	report.Hash = canonical.Hash(struct {
		Version         string       `json:"version"`
		Manifest        string       `json:"manifest"`
		StoreHash       string       `json:"store_hash"`
		OK              bool         `json:"ok"`
		Iterations      []Iteration  `json:"iterations"`
		Refinements     []Action     `json:"refinements,omitempty"`
		RemainingHoles  []proof.Hole `json:"remaining_holes,omitempty"`
		Counterexamples []Finding    `json:"counterexamples,omitempty"`
	}{report.Version, report.Manifest, report.StoreHash, report.OK, report.Iterations, report.Refinements, report.RemainingHoles, report.Counterexamples})
	return report
}

func runIteration(index int, abstraction string, manifest repair.Manifest, store replay.Store, spec *invariant.Spec, descriptor *workflow.Descriptor) Iteration {
	replayReport := replay.Analyze(manifest, nil, store)
	solverReport := solver.Analyze(manifest, store, spec)
	symbolicReport := symbolic.Execute(manifest, store)
	iteration := Iteration{
		Index:       index,
		Abstraction: abstraction,
		Inputs:      inputs(spec, descriptor),
		Hashes: Hashes{
			ReplaySemantics: replayReport.Hash,
			Solver:          solverReport.Hash,
			Symbolic:        symbolicReport.Hash,
		},
	}
	iteration.Findings = append(iteration.Findings, replayFindings(replayReport)...)
	iteration.Findings = append(iteration.Findings, solverFindings(solverReport)...)
	iteration.Findings = append(iteration.Findings, symbolicFindings(symbolicReport)...)
	iteration.Holes = append(iteration.Holes, solverHoles(solverReport)...)
	if spec == nil {
		iteration.Holes = append(iteration.Holes, proof.Hole{
			Ref:       "solver.invariants.loaded",
			Status:    proof.StatusAssumed,
			Reason:    "no invariant spec was loaded, so preservation is outside the current abstraction",
			Discharge: "rerun with --invariants to refine the abstraction with checked state predicates",
		})
	}
	if descriptor == nil {
		iteration.Holes = append(iteration.Holes, proof.Hole{
			Ref:       "workflow.model.loaded",
			Status:    proof.StatusAssumed,
			Reason:    "no workflow descriptor was loaded, so incident-order properties are outside the current abstraction",
			Discharge: "rerun with --workflow to refine the abstraction with temporal incident semantics",
		})
	} else {
		workflowReport := workflow.Check(*descriptor)
		iteration.Hashes.Workflow = workflowReport.Hash
		for _, counterexample := range workflowReport.Counterexamples {
			iteration.Findings = append(iteration.Findings, Finding{
				Ref:     counterexample.Ref,
				Status:  string(proof.StatusCounterexample),
				Message: counterexample.Message,
				Witness: strings.Join(actionsToStrings(counterexample.Trace), " -> "),
			})
		}
		iteration.Holes = append(iteration.Holes, workflowReport.ProofHoles...)
	}
	sortFindings(iteration.Findings)
	sortHoles(iteration.Holes)
	return iteration
}

func inputs(spec *invariant.Spec, descriptor *workflow.Descriptor) []string {
	out := []string{"manifest", "store", "replay-semantics", "solver-obligations", "symbolic-execution"}
	if spec != nil {
		out = append(out, "invariants")
	}
	if descriptor != nil {
		out = append(out, "workflow")
	}
	sort.Strings(out)
	return out
}

func plannedRefinements(initial Iteration, spec *invariant.Spec, descriptor *workflow.Descriptor) []Action {
	var actions []Action
	for _, hole := range initial.Holes {
		if hole.Ref == "solver.invariants.loaded" && spec != nil {
			actions = append(actions, Action{
				FromIteration: initial.Index,
				Kind:          "load_invariants",
				Reason:        hole.Reason,
				Effect:        "pending",
			})
		}
		if hole.Ref == "workflow.model.loaded" && descriptor != nil {
			actions = append(actions, Action{
				FromIteration: initial.Index,
				Kind:          "load_workflow_model",
				Reason:        hole.Reason,
				Effect:        "pending",
			})
		}
	}
	return actions
}

func markRefinementEffects(actions []Action, before, after Iteration) []Action {
	for i := range actions {
		switch {
		case len(after.Findings) > len(before.Findings):
			actions[i].Effect = "exposed_new_counterexample_or_obligation"
		case len(after.Holes) < len(before.Holes):
			actions[i].Effect = "reduced_proof_holes"
		default:
			actions[i].Effect = "reran_without_reducing_open_obligations"
		}
	}
	return actions
}

func replayFindings(report replay.AnalysisReport) []Finding {
	var out []Finding
	for _, counterexample := range report.Counterexamples {
		out = append(out, Finding{
			Ref:     counterexample.Ref,
			Status:  string(proof.StatusCounterexample),
			Message: counterexample.Message,
			Witness: counterexample.BeforeHash,
		})
	}
	return out
}

func solverFindings(report solver.Report) []Finding {
	var out []Finding
	for _, check := range report.ScopeImplications {
		if check.Status == solver.StatusCounterexample {
			out = append(out, Finding{Ref: "solver.scope." + check.OperationID, Status: string(proof.StatusCounterexample), Message: check.Reason, Witness: fmt.Sprint(check.Counterexample)})
		}
	}
	for _, check := range report.FrameChecks {
		if check.Status == solver.StatusCounterexample {
			out = append(out, Finding{Ref: "solver.frame." + check.OperationID, Status: string(proof.StatusCounterexample), Message: check.Reason})
		}
	}
	for _, check := range report.RowCountChecks {
		if check.Status == solver.StatusCounterexample {
			out = append(out, Finding{Ref: "solver.row_count." + check.OperationID, Status: string(proof.StatusCounterexample), Message: check.Reason, Witness: fmt.Sprintf("matched=%d upper_bound=%d", check.MatchedRows, check.UpperBound)})
		}
	}
	for _, check := range report.InvariantChecks {
		if check.Status == solver.StatusCounterexample {
			out = append(out, Finding{Ref: "solver.invariant." + check.ID, Status: string(proof.StatusCounterexample), Message: check.Reason, Witness: check.Witness})
		}
	}
	return out
}

func solverHoles(report solver.Report) []proof.Hole {
	var holes []proof.Hole
	for _, check := range report.ScopeImplications {
		appendSolverHole(&holes, "solver.scope."+check.OperationID, proof.Status(check.Status), check.Reason, "strengthen operation/scope predicates until implication is decidable")
	}
	for _, check := range report.FrameChecks {
		appendSolverHole(&holes, "solver.frame."+check.OperationID, proof.Status(check.Status), check.Reason, "use row-level operation kinds or add frame-specific proof evidence")
	}
	for _, check := range report.RowCountChecks {
		appendSolverHole(&holes, "solver.row_count."+check.OperationID, proof.Status(check.Status), check.Reason, "provide a bounded store snapshot or explicit row-count upper bound")
	}
	for _, check := range report.InvariantChecks {
		appendSolverHole(&holes, "solver.invariant."+check.ID, proof.Status(check.Status), check.Reason, "replace unsupported invariant or provide a checked invariant fragment")
	}
	return holes
}

func appendSolverHole(holes *[]proof.Hole, ref string, status proof.Status, reason, discharge string) {
	if status != proof.StatusAssumed && status != proof.StatusNotSupported {
		return
	}
	*holes = append(*holes, proof.Hole{Ref: ref, Status: status, Reason: reason, Discharge: discharge})
}

func symbolicFindings(report symbolic.Report) []Finding {
	var out []Finding
	for _, step := range report.Steps {
		if step.Status == "stuck" {
			out = append(out, Finding{
				Ref:     "symbolic.step." + step.OperationID,
				Status:  string(proof.StatusCounterexample),
				Message: step.Error,
				Witness: step.PreHash,
			})
		}
	}
	return out
}

func actionsToStrings(actions []workflow.Action) []string {
	out := make([]string, 0, len(actions))
	for _, action := range actions {
		out = append(out, string(action))
	}
	return out
}

func sortFindings(findings []Finding) {
	sort.Slice(findings, func(i, j int) bool {
		if findings[i].Ref == findings[j].Ref {
			return findings[i].Message < findings[j].Message
		}
		return findings[i].Ref < findings[j].Ref
	})
}

func sortHoles(holes []proof.Hole) {
	sort.Slice(holes, func(i, j int) bool {
		return holes[i].Ref < holes[j].Ref
	})
}
