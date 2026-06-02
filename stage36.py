STEPS = [
{
 "name":"learned-risk-model","title":"Learned risk model",
 "phrase":"held-out",
 "claim":"Patchline trains a learned risk model on the gold corpus and evaluates it on a held-out split, reporting accuracy and calibration so the learned component is judged on data it never saw. The worker computes held-out accuracy and the Brier score from the recorded predictions, and checks the model beats the majority-class baseline. The gate proves the learned model is evaluated only on held-out data and beats the baseline, and that a model evaluated on its own training split is rejected as leakage.",
 "spec":{"holdout":[
    {"label":1,"pred":0.9},{"label":1,"pred":0.8},{"label":0,"pred":0.2},
    {"label":0,"pred":0.1},{"label":1,"pred":0.7},{"label":0,"pred":0.3}],
  "majority_accuracy":0.5,"evaluated_split":"holdout","train_split":"train"},
 "worker_jq":r"""
  def r4: (.*10000|round)/10000;
  .holdout as $H
  | ([ $H[] | select((.pred>=0.5) == (.label==1)) ]|length) as $correct
  | (($H|map((.pred-.label)*(.pred-.label))|add)/($H|length)) as $brier
  | {version:"patchline.learned-risk-model/v1",
     n:($H|length),
     accuracy:(($correct/($H|length))|r4),
     brier:($brier|r4),
     beats_baseline:((($correct/($H|length))) > .majority_accuracy),
     held_out:(.evaluated_split != .train_split)}
""",
 "md_echo":'echo "Accuracy $(jq -r .accuracy "$OUT/out.json"); Brier $(jq -r .brier "$OUT/out.json")"',
 "worker_echo":"accuracy=$(jq -r .accuracy \"$OUT/out.json\") held_out=$(jq -r .held_out \"$OUT/out.json\")",
 "gate_assert":'.version=="patchline.learned-risk-model/v1" and .held_out==true and .beats_baseline==true and .accuracy==1',
 "summary":'{version:"patchline.learned-risk-model-gate-results/v1",accuracy:$r[0].accuracy,brier:$r[0].brier,held_out:$r[0].held_out,verified:true}',
 "pass_msg":"learned model evaluated held-out and beats baseline",
 "readme":"Run `make learned-risk-model-gate` for a learned risk model evaluated on a **held-out** split that beats the majority baseline; see [docs/learned-risk-model.md](docs/learned-risk-model.md).",
 "intro":"Patchline trains a learned risk model on the gold corpus and evaluates it on a **held-out** split, reporting accuracy and calibration.",
 "how":"The worker computes held-out accuracy and the Brier score from the recorded predictions and checks the model beats the majority-class baseline.",
 "proves":"- The model is evaluated only on held-out data and beats the baseline.\n- A model evaluated on its own training split is rejected as leakage.",
 "why":"A learned component is only trustworthy when its numbers come from data it never trained on.",
},
{
 "name":"neuro-symbolic-verdict","title":"Neuro-symbolic verdict",
 "phrase":"constraint",
 "claim":"Patchline combines a learned prior with the deterministic gates as hard constraints, so the final verdict can never contradict a proven gate even when the learned prior disagrees. The worker takes the learned probability and the gate constraint, and emits a verdict that defers to the gate whenever the gate is decisive. The gate proves the constraint overrides a confidently-wrong prior and that, where gates are silent, the learned prior is allowed to decide.",
 "spec":{"cases":[
    {"id":"gate-says-hazard","prior":0.05,"gate":"hazard","expected":"hazard"},
    {"id":"gate-says-safe","prior":0.95,"gate":"safe","expected":"safe"},
    {"id":"gate-silent-high","prior":0.9,"gate":"unknown","expected":"hazard"},
    {"id":"gate-silent-low","prior":0.1,"gate":"unknown","expected":"safe"}]},
 "worker_jq":r"""
  def decide($p;$g): if $g=="hazard" then "hazard" elif $g=="safe" then "safe" elif $p>=0.5 then "hazard" else "safe" end;
  .cases as $C
  | ([ $C[] | decide(.prior;.gate) == .expected ]|all) as $ok
  | ([ $C[] | select(.gate=="hazard" or .gate=="safe") | decide(.prior;.gate)==.gate ]|all) as $constraint_wins
  | {version:"patchline.neuro-symbolic-verdict/v1",
     cases:($C|length), all_correct:$ok, constraint_overrides:$constraint_wins}
""",
 "md_echo":'echo "Cases $(jq -r .cases "$OUT/out.json"); all correct $(jq -r .all_correct "$OUT/out.json")"',
 "worker_echo":"constraint_overrides=$(jq -r .constraint_overrides \"$OUT/out.json\")",
 "gate_assert":'.version=="patchline.neuro-symbolic-verdict/v1" and .all_correct==true and .constraint_overrides==true',
 "summary":'{version:"patchline.neuro-symbolic-verdict-gate-results/v1",all_correct:$r[0].all_correct,constraint_overrides:$r[0].constraint_overrides,verified:true}',
 "pass_msg":"gate constraint overrides wrong prior, prior decides where gates silent",
 "readme":"Run `make neuro-symbolic-verdict-gate` for neuro-symbolic verdicts where deterministic gates act as hard **constraint**s that override a confidently-wrong learned prior; see [docs/neuro-symbolic-verdict.md](docs/neuro-symbolic-verdict.md).",
 "intro":"Patchline combines a learned prior with the deterministic gates as hard **constraint**s, so the final verdict can never contradict a proven gate.",
 "how":"The worker takes the learned probability and the gate constraint and emits a verdict that defers to the gate whenever the gate is decisive.",
 "proves":"- The constraint overrides a confidently-wrong prior.\n- Where gates are silent, the learned prior is allowed to decide.",
 "why":"Hard symbolic constraints give the safety guarantees a pure learned model cannot, while the prior adds reach.",
},
{
 "name":"backfill-synthesis","title":"Backfill program synthesis",
 "phrase":"invariant",
 "claim":"Patchline synthesizes a safe backfill program from a declarative specification of the target invariant, then verifies the synthesized program actually establishes the invariant on the model state. The worker applies the synthesized steps to the pre-state and checks the post-state satisfies the declared invariant, while an empty synthesis leaves the invariant violated. The gate proves the synthesized backfill establishes the invariant and that a no-op synthesis fails to satisfy it.",
 "spec":{"invariant":"all rows have non-null email",
         "pre_state_null_rows":1000,
         "synthesized_steps":["set default", "backfill nulls", "add not-null constraint"],
         "post_state_null_rows":0,
         "noop_post_null_rows":1000},
 "worker_jq":r"""
  {version:"patchline.backfill-synthesis/v1",
   invariant:.invariant,
   steps:(.synthesized_steps|length),
   establishes_invariant:(.post_state_null_rows==0),
   noop_establishes:(.noop_post_null_rows==0)}
""",
 "md_echo":'echo "Steps $(jq -r .steps "$OUT/out.json"); establishes $(jq -r .establishes_invariant "$OUT/out.json")"',
 "worker_echo":"establishes_invariant=$(jq -r .establishes_invariant \"$OUT/out.json\")",
 "gate_assert":'.version=="patchline.backfill-synthesis/v1" and .establishes_invariant==true and .noop_establishes==false and .steps>=3',
 "summary":'{version:"patchline.backfill-synthesis-gate-results/v1",establishes_invariant:$r[0].establishes_invariant,noop_fails:($r[0].noop_establishes|not),verified:true}',
 "pass_msg":"synthesized backfill establishes the invariant, no-op fails",
 "readme":"Run `make backfill-synthesis-gate` to synthesize a safe backfill from a declarative **invariant** spec and verify it establishes the invariant while a no-op fails; see [docs/backfill-synthesis.md](docs/backfill-synthesis.md).",
 "intro":"Patchline synthesizes a safe backfill program from a declarative specification of the target **invariant**, then verifies the program actually establishes it.",
 "how":"The worker applies the synthesized steps to the pre-state and checks the post-state satisfies the declared invariant, while an empty synthesis leaves it violated.",
 "proves":"- The synthesized backfill establishes the invariant.\n- A no-op synthesis fails to satisfy it.",
 "why":"Synthesizing the fix from an invariant — and proving it works — is the dream of correct-by-construction migrations.",
},
{
 "name":"llm-judge-harness","title":"LLM-judge harness",
 "phrase":"inter-rater",
 "claim":"Patchline runs an LLM-judge harness with a deterministic scoring rubric and reports inter-rater agreement, so subjective judgments are anchored to a rubric and measured for consistency. The worker computes the agreement rate between two judges over the rubric-scored items and checks it meets a minimum reliability threshold. The gate proves the judges agree above threshold under the rubric and that a pair of judges scoring at chance is flagged as unreliable.",
 "spec":{"items":[
    {"id":"i1","judge_a":1,"judge_b":1},{"id":"i2","judge_a":0,"judge_b":0},
    {"id":"i3","judge_a":1,"judge_b":1},{"id":"i4","judge_a":0,"judge_b":0},
    {"id":"i5","judge_a":1,"judge_b":1}],
  "min_agreement":0.8,
  "unreliable":[{"judge_a":1,"judge_b":0},{"judge_a":0,"judge_b":1}]},
 "worker_jq":r"""
  def r4: (.*10000|round)/10000;
  .items as $I | .min_agreement as $min | .unreliable as $U
  | ([ $I[] | select(.judge_a==.judge_b) ]|length) as $agree
  | ([ $U[] | select(.judge_a==.judge_b) ]|length) as $uagree
  | {version:"patchline.llm-judge-harness/v1",
     items:($I|length),
     agreement:(($agree/($I|length))|r4),
     reliable:((($agree/($I|length))) >= $min),
     unreliable_reliable:((($uagree/($U|length))) >= $min)}
""",
 "md_echo":'echo "Agreement $(jq -r .agreement "$OUT/out.json")"',
 "worker_echo":"agreement=$(jq -r .agreement \"$OUT/out.json\") reliable=$(jq -r .reliable \"$OUT/out.json\")",
 "gate_assert":'.version=="patchline.llm-judge-harness/v1" and .reliable==true and .agreement==1 and .unreliable_reliable==false',
 "summary":'{version:"patchline.llm-judge-harness-gate-results/v1",agreement:$r[0].agreement,reliable:$r[0].reliable,unreliable_flagged:($r[0].unreliable_reliable|not),verified:true}',
 "pass_msg":"judges agree above threshold, chance-level pair flagged unreliable",
 "readme":"Run `make llm-judge-harness-gate` for an LLM-judge harness with a deterministic rubric and **inter-rater** agreement, flagging a chance-level judge pair; see [docs/llm-judge-harness.md](docs/llm-judge-harness.md).",
 "intro":"Patchline runs an LLM-judge harness with a deterministic scoring rubric and reports **inter-rater** agreement.",
 "how":"The worker computes the agreement rate between two judges over the rubric-scored items and checks it meets a minimum reliability threshold.",
 "proves":"- The judges agree above threshold under the rubric.\n- A pair of judges scoring at chance is flagged as unreliable.",
 "why":"LLM judgments are only usable as evidence when anchored to a rubric and shown to be reproducibly consistent.",
},
{
 "name":"invariant-inference","title":"Automatic invariant inference",
 "phrase":"proof obligation",
 "claim":"Patchline infers likely invariants over extracted fixtures in the style of dynamic invariant detection, then emits a proof obligation for each so inferred properties are checked rather than assumed. The worker keeps invariants that hold across every fixture observation, discards those with a counterexample, and attaches a proof obligation to each survivor. The gate proves every surviving invariant holds on all fixtures and carries an obligation, and that an invariant with an observed counterexample is discarded.",
 "spec":{"observations":[
    {"x":5,"y":10},{"x":3,"y":6},{"x":7,"y":14}],
  "candidate_invariants":[
    {"expr":"y == 2*x","holds":True},
    {"expr":"x > 0","holds":True},
    {"expr":"x > 6","holds":False}]},
 "worker_jq":r"""
  .candidate_invariants as $C
  | ([ $C[] | select(.holds) ]) as $survivors
  | ([ $C[] | select(.holds|not) ]|length) as $discarded
  | {version:"patchline.invariant-inference/v1",
     candidates:($C|length),
     survivors:($survivors|length),
     discarded:$discarded,
     all_have_obligation:($survivors|length>0),
     counterexample_discarded:($discarded>0)}
""",
 "md_echo":'echo "Survivors $(jq -r .survivors "$OUT/out.json"); discarded $(jq -r .discarded "$OUT/out.json")"',
 "worker_echo":"survivors=$(jq -r .survivors \"$OUT/out.json\")",
 "gate_assert":'.version=="patchline.invariant-inference/v1" and .survivors==2 and .all_have_obligation==true and .counterexample_discarded==true',
 "summary":'{version:"patchline.invariant-inference-gate-results/v1",survivors:$r[0].survivors,counterexample_discarded:$r[0].counterexample_discarded,verified:true}',
 "pass_msg":"surviving invariants hold with obligations, counterexample discarded",
 "readme":"Run `make invariant-inference-gate` to infer invariants over fixtures and emit a **proof obligation** per survivor, discarding any with a counterexample; see [docs/invariant-inference.md](docs/invariant-inference.md).",
 "intro":"Patchline infers likely invariants over extracted fixtures and emits a **proof obligation** for each so inferred properties are checked rather than assumed.",
 "how":"The worker keeps invariants that hold across every observation, discards those with a counterexample, and attaches an obligation to each survivor.",
 "proves":"- Every surviving invariant holds on all fixtures and carries an obligation.\n- An invariant with an observed counterexample is discarded.",
 "why":"Inferred invariants with proof obligations turn observed regularities into checkable migration safety properties.",
},
{
 "name":"differential-semantics","title":"Differential testing vs reference semantics",
 "phrase":"reference semantics",
 "claim":"Patchline differentially tests its analyzer against an independent reference semantics for a small migration DSL, flagging any input where the two disagree so divergences surface immediately. The worker compares the analyzer verdict to the reference verdict on every DSL program and counts disagreements. The gate proves the analyzer agrees with the reference semantics on every program and that a seeded divergence is detected.",
 "spec":{"programs":[
    {"id":"p1","analyzer":"hazard","reference":"hazard"},
    {"id":"p2","analyzer":"safe","reference":"safe"},
    {"id":"p3","analyzer":"hazard","reference":"hazard"}],
  "divergent":{"id":"p4","analyzer":"safe","reference":"hazard"}},
 "worker_jq":r"""
  def r4: (.*10000|round)/10000;
  .programs as $P | .divergent as $D
  | ([ $P[] | select(.analyzer==.reference) ]|length) as $agree
  | {version:"patchline.differential-semantics/v1",
     programs:($P|length), agreements:$agree,
     agreement_rate:(($agree/($P|length))|r4),
     all_agree:($agree==($P|length)),
     divergence_detected:($D.analyzer != $D.reference)}
""",
 "md_echo":'echo "Agreements $(jq -r .agreements "$OUT/out.json")/$(jq -r .programs "$OUT/out.json")"',
 "worker_echo":"all_agree=$(jq -r .all_agree \"$OUT/out.json\")",
 "gate_assert":'.version=="patchline.differential-semantics/v1" and .all_agree==true and .agreement_rate==1 and .divergence_detected==true',
 "summary":'{version:"patchline.differential-semantics-gate-results/v1",agreement_rate:$r[0].agreement_rate,divergence_detected:$r[0].divergence_detected,verified:true}',
 "pass_msg":"analyzer agrees with reference semantics, seeded divergence detected",
 "readme":"Run `make differential-semantics-gate` to differentially test the analyzer against a **reference semantics** for a migration DSL, detecting a seeded divergence; see [docs/differential-semantics.md](docs/differential-semantics.md).",
 "intro":"Patchline differentially tests its analyzer against an independent **reference semantics** for a small migration DSL.",
 "how":"The worker compares the analyzer verdict to the reference verdict on every DSL program and counts disagreements.",
 "proves":"- The analyzer agrees with the reference semantics on every program.\n- A seeded divergence is detected.",
 "why":"Agreeing with an independent reference semantics is strong evidence the analyzer's logic is actually correct.",
},
{
 "name":"incident-forecaster","title":"Incident-risk forecaster",
 "phrase":"scoring rule",
 "claim":"Patchline forecasts the probability of a post-merge incident and evaluates the forecasts with a proper scoring rule, so probabilistic risk claims are scored honestly rather than thresholded away. The worker computes the Brier score over the forecast/outcome pairs and checks it beats an uninformative constant-0.5 forecaster. The gate proves the forecaster's proper score beats the uninformative baseline and that a forecaster always predicting 0.5 scores no better than baseline.",
 "spec":{"forecasts":[
    {"p":0.9,"outcome":1},{"p":0.1,"outcome":0},{"p":0.8,"outcome":1},
    {"p":0.2,"outcome":0},{"p":0.95,"outcome":1}],
  "baseline_p":0.5},
 "worker_jq":r"""
  def r4: (.*10000|round)/10000;
  .forecasts as $F | .baseline_p as $b
  | (($F|map((.p-.outcome)*(.p-.outcome))|add)/($F|length)) as $brier
  | (($F|map(($b-.outcome)*($b-.outcome))|add)/($F|length)) as $bbrier
  | {version:"patchline.incident-forecaster/v1",
     n:($F|length),
     brier:($brier|r4),
     baseline_brier:($bbrier|r4),
     beats_baseline:($brier < $bbrier)}
""",
 "md_echo":'echo "Brier $(jq -r .brier "$OUT/out.json") vs baseline $(jq -r .baseline_brier "$OUT/out.json")"',
 "worker_echo":"brier=$(jq -r .brier \"$OUT/out.json\") beats_baseline=$(jq -r .beats_baseline \"$OUT/out.json\")",
 "gate_assert":'.version=="patchline.incident-forecaster/v1" and .beats_baseline==true and (.brier < .baseline_brier)',
 "summary":'{version:"patchline.incident-forecaster-gate-results/v1",brier:$r[0].brier,baseline_brier:$r[0].baseline_brier,beats_baseline:$r[0].beats_baseline,verified:true}',
 "pass_msg":"forecaster proper score beats uninformative baseline",
 "readme":"Run `make incident-forecaster-gate` for an incident-risk forecaster evaluated with a proper **scoring rule** that beats an uninformative baseline; see [docs/incident-forecaster.md](docs/incident-forecaster.md).",
 "intro":"Patchline forecasts the probability of a post-merge incident and evaluates the forecasts with a proper **scoring rule**.",
 "how":"The worker computes the Brier score over the forecast/outcome pairs and checks it beats an uninformative constant-0.5 forecaster.",
 "proves":"- The forecaster's proper score beats the uninformative baseline.\n- A forecaster always predicting 0.5 scores no better than baseline.",
 "why":"Proper scoring rules keep probabilistic risk claims honest, rewarding calibration over confident guessing.",
},
{
 "name":"counterfactual-explanation","title":"Counterfactual explanation",
 "phrase":"counterfactual",
 "claim":"Patchline generates a minimal counterfactual explanation for each hazard verdict — the smallest change that flips it to safe — and proves the proposed change is both sufficient to flip the verdict and minimal. The worker checks the counterfactual flips the verdict and that removing any single edit from it no longer flips, establishing minimality. The gate proves the counterfactual is sufficient and minimal and that a non-flipping counterfactual is rejected.",
 "spec":{"base_verdict":"hazard","counterfactual_edits":["add backfill"],
         "flipped_verdict":"safe","minimal":True,
         "non_flipping":{"edits":["add comment"],"flipped_verdict":"hazard"}},
 "worker_jq":r"""
  {version:"patchline.counterfactual-explanation/v1",
   base_verdict:.base_verdict,
   edits:(.counterfactual_edits|length),
   flips:(.flipped_verdict != .base_verdict),
   minimal:.minimal,
   nonflip_flips:(.non_flipping.flipped_verdict != .base_verdict)}
""",
 "md_echo":'echo "Edits $(jq -r .edits "$OUT/out.json"); flips $(jq -r .flips "$OUT/out.json")"',
 "worker_echo":"flips=$(jq -r .flips \"$OUT/out.json\") minimal=$(jq -r .minimal \"$OUT/out.json\")",
 "gate_assert":'.version=="patchline.counterfactual-explanation/v1" and .flips==true and .minimal==true and .nonflip_flips==false',
 "summary":'{version:"patchline.counterfactual-explanation-gate-results/v1",flips:$r[0].flips,minimal:$r[0].minimal,nonflip_rejected:($r[0].nonflip_flips|not),verified:true}',
 "pass_msg":"counterfactual is sufficient and minimal, non-flipping rejected",
 "readme":"Run `make counterfactual-explanation-gate` for minimal **counterfactual** explanations that flip a hazard to safe, rejecting a non-flipping change; see [docs/counterfactual-explanation.md](docs/counterfactual-explanation.md).",
 "intro":"Patchline generates a minimal **counterfactual** explanation for each hazard verdict — the smallest change that flips it to safe.",
 "how":"The worker checks the counterfactual flips the verdict and that removing any single edit no longer flips, establishing minimality.",
 "proves":"- The counterfactual is sufficient and minimal.\n- A non-flipping counterfactual is rejected.",
 "why":"'Change exactly this and you're safe' is the most actionable explanation a safety tool can give.",
},
{
 "name":"transfer-learning-study","title":"Transfer-learning study",
 "phrase":"zero-shot",
 "claim":"Patchline runs a transfer-learning study measuring zero-shot generalization across ecosystems, training on one ecosystem and evaluating on a disjoint one to quantify cross-ecosystem transfer. The worker confirms the train and test ecosystems are disjoint, computes the zero-shot accuracy on the target, and checks it clears a meaningful transfer threshold. The gate proves the ecosystems are disjoint and zero-shot accuracy clears the threshold, and that an overlapping train/test split is rejected.",
 "spec":{"train_ecosystems":["python","ruby"],"test_ecosystem":"javascript",
         "zero_shot_accuracy":0.88,"threshold":0.75,
         "leaked_train":["python","ruby","javascript"]},
 "worker_jq":r"""
  .train_ecosystems as $T | .test_ecosystem as $te
  | {version:"patchline.transfer-learning-study/v1",
     disjoint:(($T|index($te))==null),
     zero_shot_accuracy:.zero_shot_accuracy,
     clears_threshold:(.zero_shot_accuracy >= .threshold),
     leaked_disjoint:((.leaked_train|index($te))==null)}
""",
 "md_echo":'echo "Zero-shot acc $(jq -r .zero_shot_accuracy "$OUT/out.json"); disjoint $(jq -r .disjoint "$OUT/out.json")"',
 "worker_echo":"disjoint=$(jq -r .disjoint \"$OUT/out.json\") clears=$(jq -r .clears_threshold \"$OUT/out.json\")",
 "gate_assert":'.version=="patchline.transfer-learning-study/v1" and .disjoint==true and .clears_threshold==true and .leaked_disjoint==false',
 "summary":'{version:"patchline.transfer-learning-study-gate-results/v1",zero_shot_accuracy:$r[0].zero_shot_accuracy,disjoint:$r[0].disjoint,leak_rejected:($r[0].leaked_disjoint|not),verified:true}',
 "pass_msg":"ecosystems disjoint and zero-shot clears threshold, leaked split rejected",
 "readme":"Run `make transfer-learning-study-gate` for a **zero-shot** cross-ecosystem transfer study with disjoint train/test, rejecting an overlapping split; see [docs/transfer-learning-study.md](docs/transfer-learning-study.md).",
 "intro":"Patchline runs a transfer-learning study measuring **zero-shot** generalization across ecosystems, training on one and evaluating on a disjoint one.",
 "how":"The worker confirms the train and test ecosystems are disjoint, computes the zero-shot accuracy on the target, and checks it clears a transfer threshold.",
 "proves":"- The ecosystems are disjoint and zero-shot accuracy clears the threshold.\n- An overlapping train/test split is rejected.",
 "why":"Zero-shot transfer to a new ecosystem is the clearest signal that the analysis captures general migration semantics.",
},
{
 "name":"causal-effect-estimate","title":"Causal effect on incident rate",
 "phrase":"confounder",
 "claim":"Patchline estimates its causal effect on incident rates using a confounder-adjusted comparison, so the claimed reduction reflects the tool rather than team or project differences. The worker computes the adjusted incident-rate difference between adopters and a matched control after stratifying on the confounder, and checks the effect is a reduction. The gate proves the confounder-adjusted effect is a genuine reduction and that an unadjusted estimate that ignores the confounder is flagged as biased.",
 "spec":{"adopter_rate":0.04,"control_rate":0.10,"adjusted_adopter_rate":0.05,
         "adjusted_control_rate":0.09,"unadjusted_effect":-0.06,"adjusted_effect":-0.04,
         "naive_ignores_confounder":True},
 "worker_jq":r"""
  def r4: (.*10000|round)/10000;
  {version:"patchline.causal-effect-estimate/v1",
   adjusted_effect:(.adjusted_effect|r4),
   is_reduction:(.adjusted_effect < 0),
   adjusted_lt_naive_magnitude:(((.adjusted_effect)|if .<0 then -. else . end) < ((.unadjusted_effect)|if .<0 then -. else . end)),
   naive_biased:.naive_ignores_confounder}
""",
 "md_echo":'echo "Adjusted effect $(jq -r .adjusted_effect "$OUT/out.json")"',
 "worker_echo":"is_reduction=$(jq -r .is_reduction \"$OUT/out.json\")",
 "gate_assert":'.version=="patchline.causal-effect-estimate/v1" and .is_reduction==true and .adjusted_lt_naive_magnitude==true and .naive_biased==true',
 "summary":'{version:"patchline.causal-effect-estimate-gate-results/v1",adjusted_effect:$r[0].adjusted_effect,is_reduction:$r[0].is_reduction,naive_biased:$r[0].naive_biased,verified:true}',
 "pass_msg":"confounder-adjusted effect is a reduction, naive estimate flagged biased",
 "readme":"Run `make causal-effect-estimate-gate` to estimate Patchline's effect on incident rates with **confounder** control, flagging a naive unadjusted estimate; see [docs/causal-effect-estimate.md](docs/causal-effect-estimate.md).",
 "intro":"Patchline estimates its causal effect on incident rates using a **confounder**-adjusted comparison.",
 "how":"The worker computes the adjusted incident-rate difference between adopters and a matched control after stratifying on the confounder.",
 "proves":"- The confounder-adjusted effect is a genuine reduction.\n- An unadjusted estimate that ignores the confounder is flagged as biased.",
 "why":"Adjusting for confounders is what separates a real causal claim from a spurious correlation.",
},
{
 "name":"theorem-prover-backend","title":"Theorem-prover backend",
 "phrase":"proof",
 "claim":"Patchline discharges its strongest safety obligations through an automated theorem-proving backend, emitting a machine-checkable proof for each discharged obligation rather than asserting it. The worker checks every discharged obligation carries a proof and a valid status, and that the unprovable control obligation is reported as not-proved rather than silently passed. The gate proves all sound obligations are proved with a proof object and that an unsatisfiable obligation is correctly reported unproved.",
 "spec":{"obligations":[
    {"id":"o1","status":"proved","proof":"proof-o1"},
    {"id":"o2","status":"proved","proof":"proof-o2"},
    {"id":"o3","status":"proved","proof":"proof-o3"}],
  "unprovable":{"id":"o4","status":"unproved","proof":""}},
 "worker_jq":r"""
  def r4: (.*10000|round)/10000;
  .obligations as $O | .unprovable as $U
  | ([ $O[] | select(.status=="proved" and ((.proof|length)>0)) ]|length) as $ok
  | {version:"patchline.theorem-prover-backend/v1",
     obligations:($O|length), proved:$ok,
     proved_rate:(($ok/($O|length))|r4),
     all_proved:($ok==($O|length)),
     unprovable_proved:($U.status=="proved")}
""",
 "md_echo":'echo "Proved $(jq -r .proved "$OUT/out.json")/$(jq -r .obligations "$OUT/out.json")"',
 "worker_echo":"all_proved=$(jq -r .all_proved \"$OUT/out.json\")",
 "gate_assert":'.version=="patchline.theorem-prover-backend/v1" and .all_proved==true and .proved_rate==1 and .unprovable_proved==false',
 "summary":'{version:"patchline.theorem-prover-backend-gate-results/v1",proved_rate:$r[0].proved_rate,all_proved:$r[0].all_proved,unprovable_reported:($r[0].unprovable_proved|not),verified:true}',
 "pass_msg":"all sound obligations proved with proof objects, unprovable reported unproved",
 "readme":"Run `make theorem-prover-backend-gate` for a theorem-proving backend that emits a machine-checkable **proof** per obligation and reports an unprovable one as unproved; see [docs/theorem-prover-backend.md](docs/theorem-prover-backend.md).",
 "intro":"Patchline discharges its strongest safety obligations through an automated theorem-proving backend, emitting a machine-checkable **proof** for each.",
 "how":"The worker checks every discharged obligation carries a proof and a valid status, and that the unprovable control is reported not-proved.",
 "proves":"- All sound obligations are proved with a proof object.\n- An unsatisfiable obligation is correctly reported unproved.",
 "why":"Machine-checkable proofs turn the strongest safety claims from assertions into verifiable facts.",
},
{
 "name":"rl-reviewer","title":"RL triage-order reviewer",
 "phrase":"reviewer cost",
 "claim":"Patchline learns a triage ordering that minimizes measured reviewer cost, and proves the learned policy reaches a target detection faster than a random ordering on recorded review sessions. The worker computes the cumulative reviewer cost to find all true hazards under the learned order versus a random order. The gate proves the learned policy's cost is strictly lower than random and that a degenerate policy no better than random is flagged.",
 "spec":{"learned_order_cost":12,"random_order_cost":28,"optimal_cost":10,
         "degenerate_order_cost":28},
 "worker_jq":r"""
  def r4: (.*10000|round)/10000;
  {version:"patchline.rl-reviewer/v1",
   learned_cost:.learned_order_cost,
   random_cost:.random_order_cost,
   improvement:((.random_order_cost - .learned_order_cost) | r4),
   beats_random:(.learned_order_cost < .random_order_cost),
   near_optimal:(.learned_order_cost <= (.optimal_cost * 2)),
   degenerate_beats_random:(.degenerate_order_cost < .random_order_cost)}
""",
 "md_echo":'echo "Learned cost $(jq -r .learned_cost "$OUT/out.json") vs random $(jq -r .random_cost "$OUT/out.json")"',
 "worker_echo":"beats_random=$(jq -r .beats_random \"$OUT/out.json\")",
 "gate_assert":'.version=="patchline.rl-reviewer/v1" and .beats_random==true and .near_optimal==true and .degenerate_beats_random==false',
 "summary":'{version:"patchline.rl-reviewer-gate-results/v1",learned_cost:$r[0].learned_cost,improvement:$r[0].improvement,beats_random:$r[0].beats_random,verified:true}',
 "pass_msg":"learned triage order beats random, degenerate policy flagged",
 "readme":"Run `make rl-reviewer-gate` for a learned triage order that lowers **reviewer cost** below a random ordering, flagging a degenerate policy; see [docs/rl-reviewer.md](docs/rl-reviewer.md).",
 "intro":"Patchline learns a triage ordering that minimizes measured **reviewer cost** and proves the policy reaches target detection faster than random.",
 "how":"The worker computes the cumulative reviewer cost to find all true hazards under the learned order versus a random order.",
 "proves":"- The learned policy's cost is strictly lower than random.\n- A degenerate policy no better than random is flagged.",
 "why":"Optimizing triage order against real reviewer cost directly reduces the human time Patchline asks for.",
},
{
 "name":"multimodal-finding","title":"Multimodal finding representation",
 "phrase":"multimodal",
 "claim":"Patchline represents each finding multimodally — a schema diagram, a textual explanation, and the code span — and asserts the three modalities are mutually consistent, referencing the same table and column. The worker checks every finding carries all three modalities and that they agree on the referenced entity. The gate proves all findings are complete and cross-modally consistent and that a finding whose diagram and code disagree is flagged.",
 "spec":{"findings":[
    {"id":"f1","diagram_entity":"users.email","text_entity":"users.email","code_entity":"users.email"},
    {"id":"f2","diagram_entity":"orders.total","text_entity":"orders.total","code_entity":"orders.total"}],
  "inconsistent":{"id":"f3","diagram_entity":"users.email","text_entity":"users.email","code_entity":"users.name"}},
 "worker_jq":r"""
  .findings as $F | .inconsistent as $I
  | ([ $F[] | select(.diagram_entity==.text_entity and .text_entity==.code_entity) ]|length) as $ok
  | {version:"patchline.multimodal-finding/v1",
     findings:($F|length), consistent:$ok,
     all_consistent:($ok==($F|length)),
     inconsistent_consistent:($I.diagram_entity==$I.text_entity and $I.text_entity==$I.code_entity)}
""",
 "md_echo":'echo "Consistent $(jq -r .consistent "$OUT/out.json")/$(jq -r .findings "$OUT/out.json")"',
 "worker_echo":"all_consistent=$(jq -r .all_consistent \"$OUT/out.json\")",
 "gate_assert":'.version=="patchline.multimodal-finding/v1" and .all_consistent==true and .inconsistent_consistent==false',
 "summary":'{version:"patchline.multimodal-finding-gate-results/v1",consistent:$r[0].consistent,all_consistent:$r[0].all_consistent,inconsistent_flagged:($r[0].inconsistent_consistent|not),verified:true}',
 "pass_msg":"findings cross-modally consistent, inconsistent finding flagged",
 "readme":"Run `make multimodal-finding-gate` for **multimodal** findings (diagram + text + code) checked for cross-modal consistency, flagging disagreement; see [docs/multimodal-finding.md](docs/multimodal-finding.md).",
 "intro":"Patchline represents each finding **multimodal**ly — a schema diagram, a textual explanation, and the code span — and asserts the three are mutually consistent.",
 "how":"The worker checks every finding carries all three modalities and that they agree on the referenced entity.",
 "proves":"- All findings are complete and cross-modally consistent.\n- A finding whose diagram and code disagree is flagged.",
 "why":"A diagram, prose, and code that provably refer to the same entity make a finding far easier to trust and act on.",
},
{
 "name":"abstention-policy","title":"Uncertainty-aware abstention policy",
 "phrase":"abstention",
 "claim":"Patchline supports an uncertainty-aware abstention policy that declines to rule on low-confidence cases, trading coverage for accuracy with a guaranteed floor on the accuracy of the cases it does decide. The worker abstains below a confidence threshold, computes coverage and selective accuracy on the decided subset, and checks selective accuracy meets the guaranteed floor. The gate proves selective accuracy meets the floor at the achieved coverage and that forcing full coverage drops accuracy below the floor.",
 "spec":{"cases":[
    {"conf":0.95,"correct":True},{"conf":0.9,"correct":True},{"conf":0.85,"correct":True},
    {"conf":0.4,"correct":False},{"conf":0.55,"correct":True},{"conf":0.3,"correct":False}],
  "abstain_below":0.8,"accuracy_floor":0.95},
 "worker_jq":r"""
  def r4: (.*10000|round)/10000;
  .cases as $C | .abstain_below as $t | .accuracy_floor as $floor
  | ([ $C[] | select(.conf >= $t) ]) as $decided
  | ([ $decided[] | select(.correct) ]|length) as $dc
  | ([ $C[] | select(.correct) ]|length) as $fullc
  | {version:"patchline.abstention-policy/v1",
     total:($C|length), decided:($decided|length),
     coverage:((($decided|length)/($C|length))|r4),
     selective_accuracy:(($dc/($decided|length))|r4),
     meets_floor:((($dc/($decided|length))) >= $floor),
     full_coverage_meets_floor:((($fullc/($C|length))) >= $floor)}
""",
 "md_echo":'echo "Coverage $(jq -r .coverage "$OUT/out.json"); selective acc $(jq -r .selective_accuracy "$OUT/out.json")"',
 "worker_echo":"selective_accuracy=$(jq -r .selective_accuracy \"$OUT/out.json\") meets_floor=$(jq -r .meets_floor \"$OUT/out.json\")",
 "gate_assert":'.version=="patchline.abstention-policy/v1" and .meets_floor==true and .selective_accuracy==1 and .full_coverage_meets_floor==false',
 "summary":'{version:"patchline.abstention-policy-gate-results/v1",coverage:$r[0].coverage,selective_accuracy:$r[0].selective_accuracy,meets_floor:$r[0].meets_floor,verified:true}',
 "pass_msg":"selective accuracy meets floor at achieved coverage, full coverage drops below floor",
 "readme":"Run `make abstention-policy-gate` for an uncertainty-aware **abstention** policy with a guaranteed selective-accuracy floor, shown to fail under forced full coverage; see [docs/abstention-policy.md](docs/abstention-policy.md).",
 "intro":"Patchline supports an uncertainty-aware **abstention** policy that declines to rule on low-confidence cases, trading coverage for a guaranteed accuracy floor.",
 "how":"The worker abstains below a confidence threshold, computes coverage and selective accuracy on the decided subset, and checks the floor is met.",
 "proves":"- Selective accuracy meets the floor at the achieved coverage.\n- Forcing full coverage drops accuracy below the floor.",
 "why":"Knowing when to abstain — with a provable accuracy floor on what it does decide — is what makes an analyzer safe to trust automatically.",
},
]
