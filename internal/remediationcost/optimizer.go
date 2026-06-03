package remediationcost

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/thehalleyyoung/patchline/internal/canonical"
)

const SpecVersion = "patchline.remediation-cost/v1"
const ReportVersion = "patchline.remediation-cost-report/v1"

type Spec struct {
	Version    string     `json:"version"`
	Name       string     `json:"name"`
	Thresholds Thresholds `json:"thresholds"`
	Cases      []Case     `json:"cases"`
}

type Thresholds struct {
	MaxResidualLoss float64 `json:"max_residual_loss"`
	MaxUncertainty  float64 `json:"max_uncertainty"`
}

type Case struct {
	ID           string   `json:"id"`
	HazardClass  string   `json:"hazard_class"`
	AffectedRows int      `json:"affected_rows"`
	Probability  float64  `json:"probability"`
	ImpactPerRow float64  `json:"impact_per_row"`
	Uncertainty  float64  `json:"uncertainty"`
	Evidence     Evidence `json:"evidence"`
	Options      []Option `json:"options"`
}

type Evidence struct {
	RuntimeGuard      bool `json:"runtime_guard,omitempty"`
	BackfillProof     bool `json:"backfill_proof,omitempty"`
	InvariantTemplate bool `json:"invariant_template,omitempty"`
	ORMCheck          bool `json:"orm_check,omitempty"`
	CanaryValidation  bool `json:"canary_validation,omitempty"`
}

type Option struct {
	ID                 string   `json:"id"`
	Kind               string   `json:"kind"`
	DirectCost         float64  `json:"direct_cost"`
	RiskReduction      float64  `json:"risk_reduction"`
	LatencyHours       float64  `json:"latency_hours,omitempty"`
	LatencyCostPerHour float64  `json:"latency_cost_per_hour,omitempty"`
	Requires           []string `json:"requires,omitempty"`
	Notes              string   `json:"notes,omitempty"`
}

type Report struct {
	Version         string           `json:"version"`
	Name            string           `json:"name"`
	OK              bool             `json:"ok"`
	Thresholds      Thresholds       `json:"thresholds"`
	Summary         Summary          `json:"summary"`
	Cases           []CaseReport     `json:"cases"`
	Counterexamples []Counterexample `json:"counterexamples,omitempty"`
	Hash            string           `json:"hash"`
}

type Summary struct {
	Cases             int     `json:"cases"`
	OKCases           int     `json:"ok_cases"`
	Guard             int     `json:"guard"`
	Backfill          int     `json:"backfill"`
	ExpandContract    int     `json:"expand_contract"`
	ManualReview      int     `json:"manual_review"`
	Counterexamples   int     `json:"counterexamples"`
	ExpectedLoss      float64 `json:"expected_loss"`
	ResidualLoss      float64 `json:"residual_loss"`
	TotalExpectedLoss float64 `json:"total_expected_loss"`
}

type CaseReport struct {
	ID                      string           `json:"id"`
	HazardClass             string           `json:"hazard_class"`
	ExpectedLoss            float64          `json:"expected_loss"`
	UncertaintyPremium      float64          `json:"uncertainty_premium"`
	UncertaintyAdjustedLoss float64          `json:"uncertainty_adjusted_loss"`
	Selected                OptionReport     `json:"selected"`
	SelectionReason         string           `json:"selection_reason"`
	Rankings                []OptionReport   `json:"rankings"`
	Obligations             []Obligation     `json:"obligations"`
	Counterexamples         []Counterexample `json:"counterexamples,omitempty"`
	OK                      bool             `json:"ok"`
}

type OptionReport struct {
	Rank                int      `json:"rank"`
	ID                  string   `json:"id"`
	Kind                string   `json:"kind"`
	DirectCost          float64  `json:"direct_cost"`
	RiskReduction       float64  `json:"risk_reduction"`
	ResidualLoss        float64  `json:"residual_loss"`
	LatencyLoss         float64  `json:"latency_loss"`
	TotalExpectedLoss   float64  `json:"total_expected_loss"`
	Viable              bool     `json:"viable"`
	ClearsResidualBound bool     `json:"clears_residual_bound"`
	MissingRequirements []string `json:"missing_requirements,omitempty"`
	Notes               string   `json:"notes,omitempty"`
}

type Obligation struct {
	ID      string `json:"id"`
	Status  string `json:"status"`
	Formula string `json:"formula"`
	Reason  string `json:"reason"`
}

type Counterexample struct {
	ID      string `json:"id"`
	Kind    string `json:"kind"`
	Subject string `json:"subject,omitempty"`
	Message string `json:"message"`
}

func ReadSpec(reader io.Reader) (Spec, error) {
	decoder := json.NewDecoder(reader)
	decoder.DisallowUnknownFields()
	var spec Spec
	if err := decoder.Decode(&spec); err != nil {
		return Spec{}, err
	}
	if spec.Version != SpecVersion {
		return Spec{}, fmt.Errorf("remediation-cost spec version must be %s", SpecVersion)
	}
	return spec, nil
}

func BuildReport(spec Spec) (Report, error) {
	if err := validateSpec(spec); err != nil {
		return Report{}, err
	}
	report := Report{
		Version:    ReportVersion,
		Name:       spec.Name,
		OK:         true,
		Thresholds: spec.Thresholds,
	}
	for _, item := range sortedCases(spec.Cases) {
		caseReport := evaluateCase(item, spec.Thresholds)
		report.Cases = append(report.Cases, caseReport)
		report.Counterexamples = append(report.Counterexamples, caseReport.Counterexamples...)
		if !caseReport.OK {
			report.OK = false
		}
	}
	sortCounterexamples(report.Counterexamples)
	report.Summary = summarize(report.Cases, report.Counterexamples)
	report.Hash = reportHash(report)
	return report, nil
}

func WriteArtifacts(outDir string, report Report) error {
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}
	jsonFile, err := os.Create(filepath.Join(outDir, "remediation-cost.json"))
	if err != nil {
		return err
	}
	if err := canonical.WriteJSON(jsonFile, report); err != nil {
		_ = jsonFile.Close()
		return err
	}
	if err := jsonFile.Close(); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(outDir, "remediation-cost.md"), []byte(RenderMarkdown(report)), 0o644)
}

func RenderMarkdown(report Report) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Remediation-cost optimizer\n\n")
	fmt.Fprintf(&b, "Patchline ranks guard, backfill, expand/contract, and manual-review remediation options by uncertainty-adjusted expected loss, then applies explicit human-review escalation when uncertainty or residual loss exceeds policy.\n\n")
	fmt.Fprintf(&b, "## Summary\n\n")
	fmt.Fprintf(&b, "| Metric | Value |\n| --- | ---: |\n")
	fmt.Fprintf(&b, "| OK | `%t` |\n", report.OK)
	fmt.Fprintf(&b, "| Cases | %d |\n", report.Summary.Cases)
	fmt.Fprintf(&b, "| Guard selections | %d |\n", report.Summary.Guard)
	fmt.Fprintf(&b, "| Backfill selections | %d |\n", report.Summary.Backfill)
	fmt.Fprintf(&b, "| Expand/contract selections | %d |\n", report.Summary.ExpandContract)
	fmt.Fprintf(&b, "| Manual-review selections | %d |\n", report.Summary.ManualReview)
	fmt.Fprintf(&b, "| Counterexamples | %d |\n", report.Summary.Counterexamples)
	fmt.Fprintf(&b, "| Total expected loss | %.2f |\n\n", report.Summary.TotalExpectedLoss)
	fmt.Fprintf(&b, "Policy: residual loss must be at most `%.2f`; uncertainty above `%.2f` escalates to manual review before economic ranking.\n\n", report.Thresholds.MaxResidualLoss, report.Thresholds.MaxUncertainty)

	fmt.Fprintf(&b, "## Selected remediations\n\n")
	fmt.Fprintf(&b, "| Case | Hazard | Selected | Reason | Residual loss | Total expected loss | OK |\n| --- | --- | --- | --- | ---: | ---: | ---: |\n")
	for _, item := range report.Cases {
		fmt.Fprintf(&b, "| `%s` | `%s` | `%s` | `%s` | %.2f | %.2f | `%t` |\n", item.ID, item.HazardClass, displayKind(item.Selected.Kind), item.SelectionReason, item.Selected.ResidualLoss, item.Selected.TotalExpectedLoss, item.OK)
	}

	fmt.Fprintf(&b, "\n## Ranking evidence\n\n")
	for _, item := range report.Cases {
		fmt.Fprintf(&b, "### %s\n\n", item.ID)
		fmt.Fprintf(&b, "Expected loss `%.2f`; uncertainty premium `%.2f`; adjusted loss `%.2f`.\n\n", item.ExpectedLoss, item.UncertaintyPremium, item.UncertaintyAdjustedLoss)
		fmt.Fprintf(&b, "| Rank | Option | Viable | Clears residual bound | Residual loss | Total expected loss | Missing requirements |\n| ---: | --- | ---: | ---: | ---: | ---: | --- |\n")
		for _, option := range item.Rankings {
			missing := "-"
			if len(option.MissingRequirements) > 0 {
				missing = strings.Join(option.MissingRequirements, ", ")
			}
			fmt.Fprintf(&b, "| %d | `%s` | `%t` | `%t` | %.2f | %.2f | %s |\n", option.Rank, displayKind(option.Kind), option.Viable, option.ClearsResidualBound, option.ResidualLoss, option.TotalExpectedLoss, missing)
		}
		fmt.Fprintf(&b, "\n")
	}
	if len(report.Counterexamples) > 0 {
		fmt.Fprintf(&b, "## Counterexamples\n\n")
		fmt.Fprintf(&b, "| ID | Kind | Subject | Message |\n| --- | --- | --- | --- |\n")
		for _, counterexample := range report.Counterexamples {
			fmt.Fprintf(&b, "| `%s` | `%s` | `%s` | %s |\n", counterexample.ID, counterexample.Kind, firstNonEmpty(counterexample.Subject, "-"), counterexample.Message)
		}
	}
	return b.String()
}

func evaluateCase(item Case, thresholds Thresholds) CaseReport {
	expectedLoss := round4(float64(item.AffectedRows) * item.Probability * item.ImpactPerRow)
	uncertaintyPremium := round4(expectedLoss * item.Uncertainty)
	adjustedLoss := round4(expectedLoss + uncertaintyPremium)
	rankings := buildRankings(item, adjustedLoss, thresholds)
	selected, reason := selectOption(item, rankings, thresholds)
	report := CaseReport{
		ID:                      item.ID,
		HazardClass:             item.HazardClass,
		ExpectedLoss:            expectedLoss,
		UncertaintyPremium:      uncertaintyPremium,
		UncertaintyAdjustedLoss: adjustedLoss,
		Selected:                selected,
		SelectionReason:         reason,
		Rankings:                rankings,
	}
	report.Obligations = buildObligations(item, thresholds, report)
	report.Counterexamples = buildCounterexamples(report)
	report.OK = obligationsChecked(report.Obligations)
	return report
}

func buildRankings(item Case, adjustedLoss float64, thresholds Thresholds) []OptionReport {
	var reports []OptionReport
	for _, option := range sortedOptions(item.Options) {
		missing := missingRequirements(option, item.Evidence)
		residualLoss := round4(adjustedLoss * (1 - option.RiskReduction))
		latencyLoss := round4(option.LatencyHours * option.LatencyCostPerHour)
		reports = append(reports, OptionReport{
			ID:                  option.ID,
			Kind:                option.Kind,
			DirectCost:          round4(option.DirectCost),
			RiskReduction:       option.RiskReduction,
			ResidualLoss:        residualLoss,
			LatencyLoss:         latencyLoss,
			TotalExpectedLoss:   round4(option.DirectCost + residualLoss + latencyLoss),
			Viable:              option.Kind == "manual_review" || len(missing) == 0,
			ClearsResidualBound: residualLoss <= thresholds.MaxResidualLoss,
			MissingRequirements: missing,
			Notes:               option.Notes,
		})
	}
	sort.SliceStable(reports, func(i, j int) bool {
		if reports[i].Viable != reports[j].Viable {
			return reports[i].Viable
		}
		if reports[i].ClearsResidualBound != reports[j].ClearsResidualBound {
			return reports[i].ClearsResidualBound
		}
		if reports[i].TotalExpectedLoss != reports[j].TotalExpectedLoss {
			return reports[i].TotalExpectedLoss < reports[j].TotalExpectedLoss
		}
		if kindOrder(reports[i].Kind) != kindOrder(reports[j].Kind) {
			return kindOrder(reports[i].Kind) < kindOrder(reports[j].Kind)
		}
		return reports[i].ID < reports[j].ID
	})
	for i := range reports {
		reports[i].Rank = i + 1
	}
	return reports
}

func selectOption(item Case, rankings []OptionReport, thresholds Thresholds) (OptionReport, string) {
	manual, hasManual := manualOption(rankings)
	if thresholds.MaxUncertainty > 0 && item.Uncertainty > thresholds.MaxUncertainty {
		if hasManual {
			return manual, "uncertainty_exceeds_threshold"
		}
		return OptionReport{}, "manual_review_missing"
	}
	automated := viableCandidates(rankings, false)
	if len(automated) == 0 {
		if hasManual {
			return manual, "no_automated_option_clears_residual_bound"
		}
		return OptionReport{}, "no_viable_option_clears_residual_bound"
	}
	all := viableCandidates(rankings, true)
	if len(all) == 0 {
		return OptionReport{}, "no_viable_option_clears_residual_bound"
	}
	return all[0], "lowest_total_expected_loss"
}

func viableCandidates(rankings []OptionReport, includeManual bool) []OptionReport {
	var out []OptionReport
	for _, option := range rankings {
		if option.Viable && option.ClearsResidualBound && (includeManual || option.Kind != "manual_review") {
			out = append(out, option)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].TotalExpectedLoss != out[j].TotalExpectedLoss {
			return out[i].TotalExpectedLoss < out[j].TotalExpectedLoss
		}
		if kindOrder(out[i].Kind) != kindOrder(out[j].Kind) {
			return kindOrder(out[i].Kind) < kindOrder(out[j].Kind)
		}
		return out[i].ID < out[j].ID
	})
	return out
}

func buildObligations(item Case, thresholds Thresholds, report CaseReport) []Obligation {
	status := func(ok bool) string {
		if ok {
			return "checked"
		}
		return "refuted"
	}
	selected := report.Selected
	obligations := []Obligation{{
		ID:      "selected.viable",
		Status:  status(selected.ID != "" && selected.Viable),
		Formula: fmt.Sprintf("selected=%s has all required evidence", firstNonEmpty(selected.ID, "<none>")),
		Reason:  "unsupported guard, backfill, and expand/contract options stay ranked but cannot be selected",
	}, {
		ID:      "selected.residual_bound",
		Status:  status(selected.ID != "" && selected.ResidualLoss <= thresholds.MaxResidualLoss),
		Formula: fmt.Sprintf("selected_residual_loss=%.2f <= max_residual_loss=%.2f", selected.ResidualLoss, thresholds.MaxResidualLoss),
		Reason:  "chosen remediation must leave bounded expected loss after risk reduction",
	}}
	if thresholds.MaxUncertainty > 0 && item.Uncertainty > thresholds.MaxUncertainty {
		obligations = append(obligations, Obligation{
			ID:      "manual_review.escalation",
			Status:  status(selected.Kind == "manual_review"),
			Formula: fmt.Sprintf("uncertainty=%.2f > max_uncertainty=%.2f => selected_kind=manual_review", item.Uncertainty, thresholds.MaxUncertainty),
			Reason:  "high uncertainty is escalated before cost minimization so automated fixes do not hide unknowns",
		})
	} else if len(viableCandidates(report.Rankings, false)) == 0 {
		obligations = append(obligations, Obligation{
			ID:      "manual_review.fallback",
			Status:  status(selected.Kind == "manual_review"),
			Formula: "no viable automated option clears the residual-loss bound",
			Reason:  "manual review is the safe fallback when guard, backfill, and expand/contract are unsupported or too risky",
		})
	} else {
		best := viableCandidates(report.Rankings, true)[0]
		obligations = append(obligations, Obligation{
			ID:      "selection.expected_loss_minimum",
			Status:  status(selected.ID == best.ID),
			Formula: fmt.Sprintf("selected_total=%.2f equals minimum_viable_total=%.2f", selected.TotalExpectedLoss, best.TotalExpectedLoss),
			Reason:  "among viable options that clear residual loss, the optimizer chooses the smallest total expected loss",
		})
	}
	sort.SliceStable(obligations, func(i, j int) bool { return obligations[i].ID < obligations[j].ID })
	return obligations
}

func buildCounterexamples(report CaseReport) []Counterexample {
	var out []Counterexample
	for _, obligation := range report.Obligations {
		if obligation.Status == "checked" {
			continue
		}
		out = append(out, Counterexample{
			ID:      report.ID + "." + obligation.ID,
			Kind:    obligation.ID,
			Subject: report.ID,
			Message: obligation.Reason,
		})
	}
	sortCounterexamples(out)
	return out
}

func validateSpec(spec Spec) error {
	if spec.Version != SpecVersion {
		return fmt.Errorf("remediation-cost spec version must be %s", SpecVersion)
	}
	if spec.Name == "" {
		return fmt.Errorf("spec name is required")
	}
	if spec.Thresholds.MaxResidualLoss <= 0 {
		return fmt.Errorf("max_residual_loss must be positive")
	}
	if spec.Thresholds.MaxUncertainty < 0 {
		return fmt.Errorf("max_uncertainty must be non-negative")
	}
	if len(spec.Cases) == 0 {
		return fmt.Errorf("at least one case is required")
	}
	ids := map[string]bool{}
	for _, item := range spec.Cases {
		if item.ID == "" {
			return fmt.Errorf("case id is required")
		}
		if ids[item.ID] {
			return fmt.Errorf("case id %q is duplicated", item.ID)
		}
		ids[item.ID] = true
		if item.HazardClass == "" {
			return fmt.Errorf("case %q hazard_class is required", item.ID)
		}
		if item.AffectedRows <= 0 {
			return fmt.Errorf("case %q affected_rows must be positive", item.ID)
		}
		if item.Probability < 0 || item.Probability > 1 {
			return fmt.Errorf("case %q probability must be between 0 and 1", item.ID)
		}
		if item.ImpactPerRow < 0 || item.Uncertainty < 0 {
			return fmt.Errorf("case %q impact_per_row and uncertainty must be non-negative", item.ID)
		}
		if err := validateOptions(item); err != nil {
			return err
		}
	}
	return nil
}

func validateOptions(item Case) error {
	if len(item.Options) == 0 {
		return fmt.Errorf("case %q must declare options", item.ID)
	}
	ids := map[string]bool{}
	hasManual := false
	for _, option := range item.Options {
		if option.ID == "" {
			return fmt.Errorf("case %q option id is required", item.ID)
		}
		if ids[option.ID] {
			return fmt.Errorf("case %q option id %q is duplicated", item.ID, option.ID)
		}
		ids[option.ID] = true
		if !validKind(option.Kind) {
			return fmt.Errorf("case %q option %q has unsupported kind %q", item.ID, option.ID, option.Kind)
		}
		if option.Kind == "manual_review" {
			hasManual = true
			if len(option.Requires) > 0 {
				return fmt.Errorf("case %q manual_review option %q must not declare requires because manual review is always viable", item.ID, option.ID)
			}
		}
		if option.DirectCost < 0 || option.LatencyHours < 0 || option.LatencyCostPerHour < 0 {
			return fmt.Errorf("case %q option %q costs must be non-negative", item.ID, option.ID)
		}
		if option.RiskReduction < 0 || option.RiskReduction > 1 {
			return fmt.Errorf("case %q option %q risk_reduction must be between 0 and 1", item.ID, option.ID)
		}
		for _, requirement := range option.Requires {
			if !validRequirement(requirement) {
				return fmt.Errorf("case %q option %q has unsupported requirement %q", item.ID, option.ID, requirement)
			}
		}
	}
	if !hasManual {
		return fmt.Errorf("case %q must include a manual_review option", item.ID)
	}
	return nil
}

func missingRequirements(option Option, evidence Evidence) []string {
	var missing []string
	if option.Kind == "manual_review" {
		return nil
	}
	for _, requirement := range sortedStrings(option.Requires) {
		if !hasEvidence(evidence, requirement) {
			missing = append(missing, requirement)
		}
	}
	return missing
}

func hasEvidence(evidence Evidence, requirement string) bool {
	switch requirement {
	case "runtime_guard":
		return evidence.RuntimeGuard
	case "backfill_proof":
		return evidence.BackfillProof
	case "invariant_template":
		return evidence.InvariantTemplate
	case "orm_check":
		return evidence.ORMCheck
	case "canary_validation":
		return evidence.CanaryValidation
	default:
		return false
	}
}

func validKind(kind string) bool {
	switch kind {
	case "guard", "backfill", "expand_contract", "manual_review":
		return true
	default:
		return false
	}
}

func validRequirement(requirement string) bool {
	switch requirement {
	case "runtime_guard", "backfill_proof", "invariant_template", "orm_check", "canary_validation":
		return true
	default:
		return false
	}
}

func manualOption(rankings []OptionReport) (OptionReport, bool) {
	for _, option := range rankings {
		if option.Kind == "manual_review" {
			return option, true
		}
	}
	return OptionReport{}, false
}

func summarize(cases []CaseReport, counterexamples []Counterexample) Summary {
	var summary Summary
	summary.Cases = len(cases)
	summary.Counterexamples = len(counterexamples)
	for _, item := range cases {
		if item.OK {
			summary.OKCases++
		}
		summary.ExpectedLoss = round4(summary.ExpectedLoss + item.UncertaintyAdjustedLoss)
		summary.ResidualLoss = round4(summary.ResidualLoss + item.Selected.ResidualLoss)
		summary.TotalExpectedLoss = round4(summary.TotalExpectedLoss + item.Selected.TotalExpectedLoss)
		switch item.Selected.Kind {
		case "guard":
			summary.Guard++
		case "backfill":
			summary.Backfill++
		case "expand_contract":
			summary.ExpandContract++
		case "manual_review":
			summary.ManualReview++
		}
	}
	return summary
}

func sortedCases(cases []Case) []Case {
	out := append([]Case(nil), cases...)
	sort.SliceStable(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func sortedOptions(options []Option) []Option {
	out := append([]Option(nil), options...)
	sort.SliceStable(out, func(i, j int) bool {
		if kindOrder(out[i].Kind) != kindOrder(out[j].Kind) {
			return kindOrder(out[i].Kind) < kindOrder(out[j].Kind)
		}
		return out[i].ID < out[j].ID
	})
	return out
}

func kindOrder(kind string) int {
	switch kind {
	case "guard":
		return 0
	case "backfill":
		return 1
	case "expand_contract":
		return 2
	case "manual_review":
		return 3
	default:
		return 9
	}
}

func sortedStrings(values []string) []string {
	out := append([]string(nil), values...)
	sort.Strings(out)
	return out
}

func sortCounterexamples(counterexamples []Counterexample) {
	sort.SliceStable(counterexamples, func(i, j int) bool {
		if counterexamples[i].ID != counterexamples[j].ID {
			return counterexamples[i].ID < counterexamples[j].ID
		}
		if counterexamples[i].Kind != counterexamples[j].Kind {
			return counterexamples[i].Kind < counterexamples[j].Kind
		}
		return counterexamples[i].Subject < counterexamples[j].Subject
	})
}

func obligationsChecked(obligations []Obligation) bool {
	for _, obligation := range obligations {
		if obligation.Status != "checked" {
			return false
		}
	}
	return true
}

func reportHash(report Report) string {
	report.Hash = ""
	return canonical.Hash(report)
}

func displayKind(kind string) string {
	if kind == "expand_contract" {
		return "expand/contract"
	}
	return kind
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func round4(value float64) float64 {
	return math.Round(value*10000) / 10000
}
