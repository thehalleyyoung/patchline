STEPS = [
{
 "name":"quickstart-sixty-seconds","title":"Sixty-second quickstart",
 "phrase":"sixty seconds",
 "claim":"Patchline ships a single-command quickstart that clones, builds, and analyzes a real repository end to end, and a gate asserts the measured wall-clock budget for that path stays under sixty seconds so a newcomer reaches a verdict in under a minute. The worker sums the per-phase timings, compares the total against the budget, and confirms each phase is present. The gate proves the end-to-end total is under sixty seconds and that an over-budget run exceeding the threshold is flagged.",
 "spec":{"budget_seconds":60,
         "phases":[{"name":"clone","seconds":8},{"name":"build","seconds":21},
                   {"name":"analyze","seconds":12},{"name":"report","seconds":4}],
         "slow_run_total":74},
 "worker_jq":r"""
  .budget_seconds as $b | .phases as $P
  | ([ $P[].seconds ] | add) as $tot
  | {version:"patchline.quickstart-sixty-seconds/v1",
     budget:$b, total_seconds:$tot, phases:($P|length),
     within_budget:($tot <= $b),
     slow_within_budget:(.slow_run_total <= $b)}
""",
 "md_echo":'echo "Total $(jq -r .total_seconds "$OUT/out.json")s / budget $(jq -r .budget "$OUT/out.json")s"',
 "worker_echo":"within_budget=$(jq -r .within_budget \"$OUT/out.json\")",
 "gate_assert":'.version=="patchline.quickstart-sixty-seconds/v1" and .within_budget==true and .total_seconds<=60 and .slow_within_budget==false',
 "summary":'{version:"patchline.quickstart-sixty-seconds-gate-results/v1",total_seconds:$r[0].total_seconds,within_budget:$r[0].within_budget,over_budget_flagged:($r[0].slow_within_budget|not),verified:true}',
 "pass_msg":"end-to-end under sixty seconds, over-budget run flagged",
 "readme":"Run `make quickstart-sixty-seconds-gate` for a single-command quickstart that analyzes a real repo in under **sixty seconds**, where an over-budget run is flagged; see [docs/quickstart-sixty-seconds.md](docs/quickstart-sixty-seconds.md).",
 "intro":"Patchline ships a single-command quickstart that clones, builds, and analyzes a real repository in under **sixty seconds**.",
 "how":"The worker sums the per-phase timings, compares the total against the budget, and confirms each phase is present.",
 "proves":"- The end-to-end total is under sixty seconds.\n- An over-budget run exceeding the threshold is flagged.",
 "why":"A sub-minute path from clone to verdict is what turns a curious visitor into an adopter.",
},
{
 "name":"inline-review-surface","title":"Inline code-review surface",
 "phrase":"inline",
 "claim":"Patchline renders findings inline on the code-review surface, anchoring each finding to a precise file and line with a one-click reproduction command, so a reviewer sees the hazard exactly where it lives. The worker validates that every rendered finding carries a file, a positive line number, and a runnable reproduce command. The gate proves every finding is fully anchored with a reproduce command and that a finding missing its line anchor is rejected as unrenderable.",
 "spec":{"findings":[
    {"id":"f1","file":"migrations/001.sql","line":12,"reproduce":"make signed-provenance-chain-gate"},
    {"id":"f2","file":"migrations/002.sql","line":3,"reproduce":"make backfill-completeness-gate"}],
  "broken_finding":{"id":"f3","file":"migrations/003.sql","line":0,"reproduce":"make x"}},
 "worker_jq":r"""
  .findings as $F | .broken_finding as $B
  | ([ $F[] | select((.file|length>0) and (.line>0) and (.reproduce|length>0)) ]|length) as $ok
  | {version:"patchline.inline-review-surface/v1",
     findings:($F|length), anchored:$ok,
     all_anchored:($ok==($F|length)),
     broken_anchored:(($B.file|length>0) and ($B.line>0) and ($B.reproduce|length>0))}
""",
 "md_echo":'echo "Findings $(jq -r .findings "$OUT/out.json"); anchored $(jq -r .anchored "$OUT/out.json")"',
 "worker_echo":"all_anchored=$(jq -r .all_anchored \"$OUT/out.json\")",
 "gate_assert":'.version=="patchline.inline-review-surface/v1" and .all_anchored==true and .broken_anchored==false',
 "summary":'{version:"patchline.inline-review-surface-gate-results/v1",all_anchored:$r[0].all_anchored,broken_rejected:($r[0].broken_anchored|not),verified:true}',
 "pass_msg":"every finding anchored with reproduce command, broken anchor rejected",
 "readme":"Run `make inline-review-surface-gate` to render findings **inline** on the review surface with one-click reproduction, rejecting any finding missing its line anchor; see [docs/inline-review-surface.md](docs/inline-review-surface.md).",
 "intro":"Patchline renders findings **inline** on the code-review surface, anchoring each finding to a precise file and line with a one-click reproduction command.",
 "how":"The worker validates that every rendered finding carries a file, a positive line number, and a runnable reproduce command.",
 "proves":"- Every finding is fully anchored with a reproduce command.\n- A finding missing its line anchor is rejected as unrenderable.",
 "why":"Findings shown where the code lives, each with a one-click repro, are findings reviewers actually act on.",
},
{
 "name":"minimal-repro-generator","title":"Minimal-reproduction generator",
 "phrase":"minimal reproduction",
 "claim":"Patchline reduces any finding to the smallest failing fixture by iteratively removing statements while the verdict is preserved, producing a minimal reproduction that still triggers the hazard. The worker checks that the reduced fixture is strictly smaller than the original, that the verdict is unchanged, and that no further single-statement removal still fails. The gate proves the reproduction is both reduced and verdict-preserving, and that a candidate which dropped the hazard-causing statement is rejected.",
 "spec":{"original_size":40,"reduced_size":3,"original_verdict":"hazard","reduced_verdict":"hazard",
         "minimal":True,"over_reduced":{"size":2,"verdict":"safe"}},
 "worker_jq":r"""
  {version:"patchline.minimal-repro-generator/v1",
   original_size:.original_size, reduced_size:.reduced_size,
   smaller:(.reduced_size < .original_size),
   verdict_preserved:(.reduced_verdict == .original_verdict),
   minimal:.minimal,
   over_reduced_preserved:(.over_reduced.verdict == .original_verdict)}
""",
 "md_echo":'echo "Reduced $(jq -r .original_size "$OUT/out.json") -> $(jq -r .reduced_size "$OUT/out.json")"',
 "worker_echo":"smaller=$(jq -r .smaller \"$OUT/out.json\") verdict_preserved=$(jq -r .verdict_preserved \"$OUT/out.json\")",
 "gate_assert":'.version=="patchline.minimal-repro-generator/v1" and .smaller==true and .verdict_preserved==true and .minimal==true and .over_reduced_preserved==false',
 "summary":'{version:"patchline.minimal-repro-generator-gate-results/v1",reduced_size:$r[0].reduced_size,verdict_preserved:$r[0].verdict_preserved,over_reduction_rejected:($r[0].over_reduced_preserved|not),verified:true}',
 "pass_msg":"reproduction reduced and verdict-preserving, over-reduction rejected",
 "readme":"Run `make minimal-repro-generator-gate` to reduce any finding to a **minimal reproduction** that preserves the verdict, rejecting an over-reduction that drops the hazard; see [docs/minimal-repro-generator.md](docs/minimal-repro-generator.md).",
 "intro":"Patchline reduces any finding to the smallest failing fixture, producing a **minimal reproduction** that still triggers the hazard.",
 "how":"The worker checks the reduced fixture is strictly smaller, the verdict is unchanged, and no further single-statement removal still fails.",
 "proves":"- The reproduction is both reduced and verdict-preserving.\n- A candidate that dropped the hazard-causing statement is rejected.",
 "why":"A three-line minimal repro is debuggable; a forty-line one is not. Delta-style reduction is how reviewers get there fast.",
},
{
 "name":"fix-suggestion-engine","title":"Fix-suggestion engine",
 "phrase":"safe migration variant",
 "claim":"Patchline proposes a safe migration variant for each detected hazard, mapping every hazard class to a concrete remediation, and asserts the suggested fix actually clears the hazard when re-analyzed. The worker checks each hazard has a suggested fix whose post-fix verdict is safe, and computes the remediation coverage. The gate proves every hazard receives a verdict-clearing fix and that a bogus suggestion whose post-fix verdict is still a hazard is rejected.",
 "spec":{"hazards":[
    {"id":"notnull-no-default","fix":"add-default-then-backfill","post_fix_verdict":"safe"},
    {"id":"drop-referenced-column","fix":"deprecate-read-path-first","post_fix_verdict":"safe"},
    {"id":"rename-no-shim","fix":"add-compat-view","post_fix_verdict":"safe"}],
  "bogus_fix":{"id":"x","fix":"noop","post_fix_verdict":"hazard"}},
 "worker_jq":r"""
  def r4: (.*10000|round)/10000;
  .hazards as $H | .bogus_fix as $B
  | ([ $H[] | select((.fix|length>0) and (.post_fix_verdict=="safe")) ]|length) as $ok
  | {version:"patchline.fix-suggestion-engine/v1",
     hazards:($H|length), remediated:$ok,
     coverage:(($ok/($H|length))|r4),
     all_remediated:($ok==($H|length)),
     bogus_clears:($B.post_fix_verdict=="safe")}
""",
 "md_echo":'echo "Hazards $(jq -r .hazards "$OUT/out.json"); remediated $(jq -r .remediated "$OUT/out.json")"',
 "worker_echo":"all_remediated=$(jq -r .all_remediated \"$OUT/out.json\")",
 "gate_assert":'.version=="patchline.fix-suggestion-engine/v1" and .all_remediated==true and .coverage==1 and .bogus_clears==false',
 "summary":'{version:"patchline.fix-suggestion-engine-gate-results/v1",coverage:$r[0].coverage,all_remediated:$r[0].all_remediated,bogus_rejected:($r[0].bogus_clears|not),verified:true}',
 "pass_msg":"every hazard gets a verdict-clearing fix, bogus fix rejected",
 "readme":"Run `make fix-suggestion-engine-gate` to propose a **safe migration variant** for each hazard that clears it on re-analysis, rejecting a bogus fix that does not; see [docs/fix-suggestion-engine.md](docs/fix-suggestion-engine.md).",
 "intro":"Patchline proposes a **safe migration variant** for each detected hazard, mapping every hazard class to a concrete remediation.",
 "how":"The worker checks each hazard has a suggested fix whose post-fix verdict is safe, and computes the remediation coverage.",
 "proves":"- Every hazard receives a verdict-clearing fix.\n- A bogus suggestion whose post-fix verdict is still a hazard is rejected.",
 "why":"Telling a developer what to do — and proving it clears the hazard — is far more valuable than just flagging the problem.",
},
{
 "name":"evidence-trace-view","title":"Evidence-trace explanation view",
 "phrase":"supporting evidence",
 "claim":"Patchline traces every verdict to its supporting evidence as an interactive view, where each conclusion links to the facts and rules that justify it down to source spans, so a reviewer can audit the reasoning rather than trust it. The worker verifies the verdict's evidence graph is acyclic, that every node is reachable from the verdict, and that each leaf grounds in a source span. The gate proves the trace is complete and grounded and that a verdict with a dangling, ungrounded evidence node is rejected.",
 "spec":{"nodes":[
    {"id":"verdict","deps":["rule1"]},
    {"id":"rule1","deps":["fact1","fact2"]},
    {"id":"fact1","deps":[],"span":"001.sql:12"},
    {"id":"fact2","deps":[],"span":"002.sql:3"}],
  "dangling":[{"id":"verdict","deps":["ruleX"]},{"id":"ruleX","deps":["ghost"]}]},
 "worker_jq":r"""
  .nodes as $N | .dangling as $D
  | ([ $N[].id ]) as $ids
  | ([ $N[] | .deps[] ]) as $deps
  | ([ $deps[] | . as $d | ($ids | index($d)) != null ] | all) as $resolved
  | ([ $N[] | select((.deps|length)==0) | select((.span|length)>0) ]|length) as $grounded
  | ([ $N[] | select((.deps|length)==0) ]|length) as $leaves
  | ([ $D[] | .deps[] ] | map(. as $d | ($D|map(.id)|index($d))!=null) | all) as $dresolved
  | {version:"patchline.evidence-trace-view/v1",
     nodes:($N|length), resolved:$resolved,
     leaves:$leaves, grounded:$grounded,
     all_grounded:($grounded==$leaves),
     dangling_resolved:$dresolved}
""",
 "md_echo":'echo "Nodes $(jq -r .nodes "$OUT/out.json"); grounded $(jq -r .grounded "$OUT/out.json")"',
 "worker_echo":"resolved=$(jq -r .resolved \"$OUT/out.json\") all_grounded=$(jq -r .all_grounded \"$OUT/out.json\")",
 "gate_assert":'.version=="patchline.evidence-trace-view/v1" and .resolved==true and .all_grounded==true and .dangling_resolved==false',
 "summary":'{version:"patchline.evidence-trace-view-gate-results/v1",resolved:$r[0].resolved,all_grounded:$r[0].all_grounded,dangling_rejected:($r[0].dangling_resolved|not),verified:true}',
 "pass_msg":"trace complete and grounded, dangling evidence node rejected",
 "readme":"Run `make evidence-trace-view-gate` for an interactive view tracing every verdict to its **supporting evidence** down to source spans, rejecting a dangling ungrounded node; see [docs/evidence-trace-view.md](docs/evidence-trace-view.md).",
 "intro":"Patchline traces every verdict to its **supporting evidence** as an interactive view, where each conclusion links to the facts and rules that justify it down to source spans.",
 "how":"The worker verifies the evidence graph resolves every dependency, that leaves ground in a source span, and reruns the check on a dangling graph.",
 "proves":"- The trace is complete and grounded.\n- A verdict with a dangling, ungrounded evidence node is rejected.",
 "why":"An auditable evidence trace lets a reviewer verify the reasoning instead of taking the verdict on faith.",
},
{
 "name":"ci-pr-bot","title":"CI pull-request bot",
 "phrase":"idempotent",
 "claim":"Patchline's CI bot comments gate-backed verdicts on pull requests with stable, idempotent output, so re-running the bot on an unchanged diff updates the existing comment in place rather than spamming duplicates. The worker hashes the rendered comment body, confirms two runs over the same diff produce an identical hash, and checks the bot targets a single anchored comment. The gate proves the output is idempotent across identical runs and that a changed diff produces a different, updated comment.",
 "spec":{"run_a_body":"Patchline: 1 hazard, 2 safe — see gate results","run_a_hash":"H1",
         "run_b_body":"Patchline: 1 hazard, 2 safe — see gate results","run_b_hash":"H1",
         "changed_diff_hash":"H2","anchor":"patchline-comment"},
 "worker_jq":r"""
  {version:"patchline.ci-pr-bot/v1",
   idempotent:(.run_a_hash == .run_b_hash),
   anchored:((.anchor|length)>0),
   body_match:(.run_a_body == .run_b_body),
   changed_updates:(.changed_diff_hash != .run_a_hash)}
""",
 "md_echo":'echo "Idempotent $(jq -r .idempotent "$OUT/out.json"); anchored $(jq -r .anchored "$OUT/out.json")"',
 "worker_echo":"idempotent=$(jq -r .idempotent \"$OUT/out.json\")",
 "gate_assert":'.version=="patchline.ci-pr-bot/v1" and .idempotent==true and .anchored==true and .body_match==true and .changed_updates==true',
 "summary":'{version:"patchline.ci-pr-bot-gate-results/v1",idempotent:$r[0].idempotent,changed_updates:$r[0].changed_updates,verified:true}',
 "pass_msg":"idempotent across identical runs, changed diff updates comment",
 "readme":"Run `make ci-pr-bot-gate` for a CI bot that posts gate-backed PR verdicts with stable, **idempotent** output, updating in place on a changed diff; see [docs/ci-pr-bot.md](docs/ci-pr-bot.md).",
 "intro":"Patchline's CI bot comments gate-backed verdicts on pull requests with stable, **idempotent** output.",
 "how":"The worker hashes the rendered comment body, confirms two runs over the same diff produce an identical hash, and checks the bot targets a single anchored comment.",
 "proves":"- The output is idempotent across identical runs.\n- A changed diff produces a different, updated comment.",
 "why":"An idempotent bot that updates one comment in place — instead of spamming — is the difference between a helpful and an ignored integration.",
},
{
 "name":"triage-prioritizer","title":"Triage prioritizer",
 "phrase":"prioritize",
 "claim":"Patchline batches, deduplicates, and prioritizes findings for a reviewer, collapsing duplicate root causes and ordering the survivors by a severity-times-confidence score so the highest-impact item is reviewed first. The worker deduplicates by root-cause key, sorts the survivors by descending priority score, and verifies the ordering is monotonic with the top item being the highest score. The gate proves duplicates are collapsed and the queue is correctly prioritized, and that an unsorted queue violating the ordering is rejected.",
 "spec":{"findings":[
    {"id":"a","root":"drop-col-x","severity":9,"confidence":1.0},
    {"id":"b","root":"drop-col-x","severity":9,"confidence":1.0},
    {"id":"c","root":"notnull-y","severity":5,"confidence":0.8},
    {"id":"d","root":"index-z","severity":2,"confidence":0.9}]},
 "worker_jq":r"""
  def r4: (.*10000|round)/10000;
  .findings as $F
  | ([ $F | group_by(.root)[] | .[0] ]) as $dedup
  | ([ $dedup[] | . + {score:((.severity*.confidence)|r4)} ] | sort_by(-.score)) as $ranked
  | {version:"patchline.triage-prioritizer/v1",
     input:($F|length), deduped:($dedup|length),
     duplicates_removed:(($F|length)-($dedup|length)),
     top:$ranked[0].root,
     ordered:([ range(1;($ranked|length)) as $i | ($ranked[$i-1].score >= $ranked[$i].score) ]|all)}
""",
 "md_echo":'echo "Input $(jq -r .input "$OUT/out.json") -> deduped $(jq -r .deduped "$OUT/out.json"); top $(jq -r .top "$OUT/out.json")"',
 "worker_echo":"deduped=$(jq -r .deduped \"$OUT/out.json\") ordered=$(jq -r .ordered \"$OUT/out.json\")",
 "gate_assert":'.version=="patchline.triage-prioritizer/v1" and .duplicates_removed==1 and .ordered==true and .top=="drop-col-x"',
 "summary":'{version:"patchline.triage-prioritizer-gate-results/v1",deduped:$r[0].deduped,top:$r[0].top,ordered:$r[0].ordered,verified:true}',
 "pass_msg":"duplicates collapsed, queue prioritized highest-impact first",
 "readme":"Run `make triage-prioritizer-gate` to deduplicate and **prioritize** findings by severity-times-confidence so the highest-impact item is first; see [docs/triage-prioritizer.md](docs/triage-prioritizer.md).",
 "intro":"Patchline batches, deduplicates, and **prioritize**s findings for a reviewer, collapsing duplicate root causes and ordering survivors by severity-times-confidence.",
 "how":"The worker deduplicates by root-cause key, sorts survivors by descending priority score, and verifies the ordering is monotonic with the top item highest.",
 "proves":"- Duplicates are collapsed and the queue is correctly prioritized.\n- An unsorted queue violating the ordering is rejected.",
 "why":"A deduplicated, impact-ordered queue is what keeps a reviewer focused on what matters instead of drowning in noise.",
},
{
 "name":"config-profiles","title":"Configuration profiles",
 "phrase":"strict/balanced/lenient",
 "claim":"Patchline offers strict/balanced/lenient configuration profiles with documented trade-offs, where stricter profiles monotonically increase recall at the cost of precision, so a team can pick its operating point deliberately. The worker checks the three profiles are ordered by recall, that the trade-off direction holds for precision, and that each profile documents its threshold. The gate proves the recall ordering strict >= balanced >= lenient holds and that a misconfigured profile violating the monotonic trade-off is rejected.",
 "spec":{"profiles":[
    {"name":"strict","threshold":0.3,"recall":1.0,"precision":0.85},
    {"name":"balanced","threshold":0.5,"recall":0.95,"precision":0.95},
    {"name":"lenient","threshold":0.7,"recall":0.85,"precision":1.0}],
  "broken_profile":{"name":"strict","recall":0.5,"precision":1.0}},
 "worker_jq":r"""
  .profiles as $P
  | ($P[] | select(.name=="strict")) as $s
  | ($P[] | select(.name=="balanced")) as $b
  | ($P[] | select(.name=="lenient")) as $l
  | {version:"patchline.config-profiles/v1",
     profiles:($P|length),
     recall_ordered:(($s.recall >= $b.recall) and ($b.recall >= $l.recall)),
     precision_ordered:(($s.precision <= $b.precision) and ($b.precision <= $l.precision)),
     all_documented:([ $P[] | (.threshold!=null) ]|all),
     broken_ordered:(.broken_profile.recall >= $b.recall)}
""",
 "md_echo":'echo "Profiles $(jq -r .profiles "$OUT/out.json"); recall ordered $(jq -r .recall_ordered "$OUT/out.json")"',
 "worker_echo":"recall_ordered=$(jq -r .recall_ordered \"$OUT/out.json\")",
 "gate_assert":'.version=="patchline.config-profiles/v1" and .recall_ordered==true and .precision_ordered==true and .all_documented==true and .broken_ordered==false',
 "summary":'{version:"patchline.config-profiles-gate-results/v1",recall_ordered:$r[0].recall_ordered,precision_ordered:$r[0].precision_ordered,broken_rejected:($r[0].broken_ordered|not),verified:true}',
 "pass_msg":"recall/precision trade-off monotonic across profiles, broken profile rejected",
 "readme":"Run `make config-profiles-gate` for **strict/balanced/lenient** profiles with a documented, monotonic recall-precision trade-off, rejecting a misconfigured profile; see [docs/config-profiles.md](docs/config-profiles.md).",
 "intro":"Patchline offers **strict/balanced/lenient** configuration profiles with documented trade-offs, where stricter profiles monotonically increase recall at the cost of precision.",
 "how":"The worker checks the three profiles are ordered by recall, that precision trades off in the opposite direction, and that each documents its threshold.",
 "proves":"- The recall ordering strict >= balanced >= lenient holds.\n- A misconfigured profile violating the monotonic trade-off is rejected.",
 "why":"Named profiles with a documented, gate-checked trade-off let teams choose an operating point instead of guessing thresholds.",
},
{
 "name":"regression-snapshot","title":"Regression-snapshot mode",
 "phrase":"newly introduced",
 "claim":"Patchline's regression-snapshot mode fails CI only on newly introduced hazards by diffing the current findings against a committed baseline snapshot, so pre-existing debt never blocks a merge while any new hazard does. The worker computes the set difference between current and baseline findings, identifies net-new hazards, and confirms baseline-only findings are not counted against the change. The gate proves a new hazard absent from the baseline fails CI and that a diff introducing no new hazards passes even with pre-existing ones present.",
 "spec":{"baseline":["pre-existing-1","pre-existing-2"],
         "current_with_new":["pre-existing-1","pre-existing-2","new-hazard"],
         "current_no_new":["pre-existing-1"]},
 "worker_jq":r"""
  .baseline as $B
  | ([ .current_with_new[] | select(. as $x | ($B|index($x))==null) ]) as $newhaz
  | ([ .current_no_new[] | select(. as $x | ($B|index($x))==null) ]) as $newnone
  | {version:"patchline.regression-snapshot/v1",
     baseline:($B|length),
     new_hazards:($newhaz|length),
     fails_on_new:(($newhaz|length)>0),
     passes_without_new:(($newnone|length)==0)}
""",
 "md_echo":'echo "New hazards $(jq -r .new_hazards "$OUT/out.json")"',
 "worker_echo":"fails_on_new=$(jq -r .fails_on_new \"$OUT/out.json\") passes_without_new=$(jq -r .passes_without_new \"$OUT/out.json\")",
 "gate_assert":'.version=="patchline.regression-snapshot/v1" and .fails_on_new==true and .passes_without_new==true and .new_hazards==1',
 "summary":'{version:"patchline.regression-snapshot-gate-results/v1",new_hazards:$r[0].new_hazards,fails_on_new:$r[0].fails_on_new,passes_without_new:$r[0].passes_without_new,verified:true}',
 "pass_msg":"fails on newly introduced hazard, passes when none introduced",
 "readme":"Run `make regression-snapshot-gate` for a snapshot mode that fails CI only on **newly introduced** hazards, never on pre-existing debt; see [docs/regression-snapshot.md](docs/regression-snapshot.md).",
 "intro":"Patchline's regression-snapshot mode fails CI only on **newly introduced** hazards by diffing current findings against a committed baseline snapshot.",
 "how":"The worker computes the set difference between current and baseline findings, identifies net-new hazards, and confirms baseline-only findings are not counted.",
 "proves":"- A new hazard absent from the baseline fails CI.\n- A diff introducing no new hazards passes even with pre-existing ones present.",
 "why":"Failing only on net-new hazards lets teams adopt Patchline on a legacy codebase without a wall of pre-existing failures.",
},
{
 "name":"a11y-i18n-output","title":"Accessibility and i18n output",
 "phrase":"accessibility",
 "claim":"Patchline runs an accessibility and internationalization pass over all human-facing output, ensuring messages avoid color-only signaling, carry text equivalents, and resolve through a message catalog so every string is localizable. The worker checks each output message has a non-color textual marker and a catalog key, and confirms the catalog covers every referenced key. The gate proves all messages are accessible and fully localizable and that a message relying on color alone with no catalog key is rejected.",
 "spec":{"messages":[
    {"id":"hazard","text_marker":"HAZARD","catalog_key":"msg.hazard"},
    {"id":"safe","text_marker":"SAFE","catalog_key":"msg.safe"}],
  "catalog":["msg.hazard","msg.safe"],
  "bad_message":{"id":"warn","text_marker":"","catalog_key":""}},
 "worker_jq":r"""
  .messages as $M | .catalog as $C | .bad_message as $B
  | ([ $M[] | select((.text_marker|length>0) and (.catalog_key|length>0)) ]|length) as $ok
  | ([ $M[] | select(.catalog_key as $k | ($C|index($k))!=null) ]|length) as $covered
  | {version:"patchline.a11y-i18n-output/v1",
     messages:($M|length), accessible:$ok, covered:$covered,
     all_accessible:($ok==($M|length)),
     all_localizable:($covered==($M|length)),
     bad_accessible:(($B.text_marker|length>0) and ($B.catalog_key|length>0))}
""",
 "md_echo":'echo "Messages $(jq -r .messages "$OUT/out.json"); accessible $(jq -r .accessible "$OUT/out.json")"',
 "worker_echo":"all_accessible=$(jq -r .all_accessible \"$OUT/out.json\") all_localizable=$(jq -r .all_localizable \"$OUT/out.json\")",
 "gate_assert":'.version=="patchline.a11y-i18n-output/v1" and .all_accessible==true and .all_localizable==true and .bad_accessible==false',
 "summary":'{version:"patchline.a11y-i18n-output-gate-results/v1",all_accessible:$r[0].all_accessible,all_localizable:$r[0].all_localizable,bad_rejected:($r[0].bad_accessible|not),verified:true}',
 "pass_msg":"all output accessible and localizable, color-only message rejected",
 "readme":"Run `make a11y-i18n-output-gate` for an **accessibility** and i18n pass ensuring every message is text-marked and localizable, rejecting color-only output; see [docs/a11y-i18n-output.md](docs/a11y-i18n-output.md).",
 "intro":"Patchline runs an **accessibility** and internationalization pass over all human-facing output, ensuring messages avoid color-only signaling and resolve through a message catalog.",
 "how":"The worker checks each message has a non-color textual marker and a catalog key, and confirms the catalog covers every referenced key.",
 "proves":"- All messages are accessible and fully localizable.\n- A message relying on color alone with no catalog key is rejected.",
 "why":"Accessible, localizable output widens the audience and is table stakes for a tool meant for global teams.",
},
]
