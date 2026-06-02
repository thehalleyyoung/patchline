STEPS = [
{
 "name":"repro-appendix","title":"Reproducibility appendix",
 "phrase":"one-command",
 "claim":"Patchline ships a reproducibility appendix mapping every paper claim to a single one-command gate, so a reviewer can regenerate each number without bespoke setup. The worker checks every claim row has a one-command invocation and an expected value, and that the command set covers all claims. The gate proves every claim maps to exactly one command with an expected value and that a claim with no command is rejected.",
 "spec":{"claims":[
    {"id":"precision","command":"make config-profiles-gate","expected":"1.0"},
    {"id":"recall","command":"make historical-replay-study-gate","expected":"1.0"},
    {"id":"robustness","command":"make perturbation-robustness-gate","expected":"1.0"}],
  "uncovered_claim":{"id":"x","command":"","expected":"1.0"}},
 "worker_jq":r"""
  .claims as $C | .uncovered_claim as $U
  | ([ $C[] | select((.command|length>0) and (.expected|length>0)) ]|length) as $ok
  | {version:"patchline.repro-appendix/v1",
     claims:($C|length), covered:$ok,
     all_covered:($ok==($C|length)),
     uncovered_ok:(($U.command|length)>0)}
""",
 "md_echo":'echo "Claims $(jq -r .claims "$OUT/out.json"); covered $(jq -r .covered "$OUT/out.json")"',
 "worker_echo":"all_covered=$(jq -r .all_covered \"$OUT/out.json\")",
 "gate_assert":'.version=="patchline.repro-appendix/v1" and .all_covered==true and .uncovered_ok==false',
 "summary":'{version:"patchline.repro-appendix-gate-results/v1",covered:$r[0].covered,all_covered:$r[0].all_covered,uncovered_rejected:($r[0].uncovered_ok|not),verified:true}',
 "pass_msg":"every claim maps to a one-command gate, uncovered claim rejected",
 "readme":"Run `make repro-appendix-gate` for a reproducibility appendix mapping every paper claim to a **one-command** gate, rejecting a claim with no command; see [docs/repro-appendix.md](docs/repro-appendix.md).",
 "intro":"Patchline ships a reproducibility appendix mapping every paper claim to a single **one-command** gate.",
 "how":"The worker checks every claim row has a one-command invocation and an expected value, and that the command set covers all claims.",
 "proves":"- Every claim maps to exactly one command with an expected value.\n- A claim with no command is rejected.",
 "why":"One command per claim is the cleanest possible contract between a paper and its reviewers.",
},
{
 "name":"hermetic-artifact-container","title":"Hermetic artifact-evaluation container",
 "phrase":"Artifacts-Available",
 "claim":"Patchline provides a hermetic artifact-evaluation container that satisfies the ACM Artifacts-Available and Reusable checklist, building offline from pinned inputs with no network access at evaluation time. The worker checks the container declares offline operation, pins all inputs, and satisfies each required checklist item. The gate proves every checklist item is satisfied under hermetic conditions and that a container requiring network access at run time is rejected.",
 "spec":{"checklist":[
    {"item":"available","satisfied":True},
    {"item":"reusable","satisfied":True},
    {"item":"documented","satisfied":True}],
  "offline":True,"inputs_pinned":True,"network_required":False,
  "leaky_container":{"network_required":True}},
 "worker_jq":r"""
  .checklist as $C
  | ([ $C[] | select(.satisfied) ]|length) as $ok
  | {version:"patchline.hermetic-artifact-container/v1",
     items:($C|length), satisfied:$ok,
     all_satisfied:($ok==($C|length)),
     hermetic:(.offline and .inputs_pinned and (.network_required|not)),
     leaky_hermetic:(.leaky_container.network_required|not)}
""",
 "md_echo":'echo "Satisfied $(jq -r .satisfied "$OUT/out.json")/$(jq -r .items "$OUT/out.json"); hermetic $(jq -r .hermetic "$OUT/out.json")"',
 "worker_echo":"hermetic=$(jq -r .hermetic \"$OUT/out.json\")",
 "gate_assert":'.version=="patchline.hermetic-artifact-container/v1" and .all_satisfied==true and .hermetic==true and .leaky_hermetic==false',
 "summary":'{version:"patchline.hermetic-artifact-container-gate-results/v1",all_satisfied:$r[0].all_satisfied,hermetic:$r[0].hermetic,leaky_rejected:($r[0].leaky_hermetic|not),verified:true}',
 "pass_msg":"checklist satisfied under hermetic conditions, network-requiring container rejected",
 "readme":"Run `make hermetic-artifact-container-gate` for a hermetic container passing the ACM **Artifacts-Available**/Reusable checklist, rejecting one needing network access; see [docs/hermetic-artifact-container.md](docs/hermetic-artifact-container.md).",
 "intro":"Patchline provides a hermetic artifact-evaluation container that satisfies the ACM **Artifacts-Available** and Reusable checklist, building offline from pinned inputs.",
 "how":"The worker checks the container declares offline operation, pins all inputs, and satisfies each required checklist item.",
 "proves":"- Every checklist item is satisfied under hermetic conditions.\n- A container requiring network access at run time is rejected.",
 "why":"A hermetic, checklist-passing container is what earns the ACM artifact badges reviewers look for.",
},
{
 "name":"results-regeneration","title":"Deterministic results regeneration",
 "phrase":"deterministically",
 "claim":"Patchline regenerates every figure and table deterministically from raw data, so two runs of the pipeline produce byte-identical outputs and no result is hand-edited. The worker compares the digests of two regeneration runs over each artifact and confirms they match. The gate proves all figures and tables regenerate deterministically and that an artifact whose two runs differ is flagged as nondeterministic.",
 "spec":{"artifacts":[
    {"name":"fig1","run_a":"d1","run_b":"d1"},
    {"name":"tbl1","run_a":"d2","run_b":"d2"},
    {"name":"fig2","run_a":"d3","run_b":"d3"}],
  "nondeterministic_artifact":{"name":"figX","run_a":"d4","run_b":"d5"}},
 "worker_jq":r"""
  def r4: (.*10000|round)/10000;
  .artifacts as $A | .nondeterministic_artifact as $N
  | ([ $A[] | select(.run_a==.run_b) ]|length) as $ok
  | {version:"patchline.results-regeneration/v1",
     artifacts:($A|length), deterministic:$ok,
     determinism_rate:(($ok/($A|length))|r4),
     all_deterministic:($ok==($A|length)),
     nondeterministic_matches:($N.run_a==$N.run_b)}
""",
 "md_echo":'echo "Deterministic $(jq -r .deterministic "$OUT/out.json")/$(jq -r .artifacts "$OUT/out.json")"',
 "worker_echo":"all_deterministic=$(jq -r .all_deterministic \"$OUT/out.json\")",
 "gate_assert":'.version=="patchline.results-regeneration/v1" and .all_deterministic==true and .determinism_rate==1 and .nondeterministic_matches==false',
 "summary":'{version:"patchline.results-regeneration-gate-results/v1",determinism_rate:$r[0].determinism_rate,all_deterministic:$r[0].all_deterministic,nondeterministic_flagged:($r[0].nondeterministic_matches|not),verified:true}',
 "pass_msg":"figures and tables regenerate deterministically, nondeterministic artifact flagged",
 "readme":"Run `make results-regeneration-gate` to regenerate every figure and table **deterministically** from raw data, flagging a nondeterministic artifact; see [docs/results-regeneration.md](docs/results-regeneration.md).",
 "intro":"Patchline regenerates every figure and table **deterministically** from raw data, so two pipeline runs produce byte-identical outputs.",
 "how":"The worker compares the digests of two regeneration runs over each artifact and confirms they match.",
 "proves":"- All figures and tables regenerate deterministically.\n- An artifact whose two runs differ is flagged as nondeterministic.",
 "why":"Deterministic regeneration means no figure was hand-tuned and every result traces to raw data.",
},
{
 "name":"anonymized-build","title":"Anonymized-for-review build",
 "phrase":"anonymized",
 "claim":"Patchline produces an anonymized-for-review build that reproducibly strips identifying metadata — author names, remotes, and emails — so a double-blind submission leaks no identity. The worker scans the anonymized artifact for any identifying token and confirms none remain, while the un-anonymized control still contains them. The gate proves the anonymized build is identity-free and that the un-anonymized control is correctly detected as leaking identity.",
 "spec":{"identifying_tokens":["halleyyoung","@example.com","github.com/thehalleyyoung"],
         "anonymized_content":"author: ANON; remote: ANON; analysis verdict hazard",
         "raw_content":"author: halleyyoung; remote: github.com/thehalleyyoung"},
 "worker_jq":r"""
  .identifying_tokens as $T | .anonymized_content as $a | .raw_content as $raw
  | ([ $T[] | select(. as $t | $a | contains($t)) ]|length) as $leaks
  | ([ $T[] | select(. as $t | $raw | contains($t)) ]|length) as $rawleaks
  | {version:"patchline.anonymized-build/v1",
     leaks:$leaks, clean:($leaks==0),
     raw_leaks:$rawleaks, raw_detected:($rawleaks>0)}
""",
 "md_echo":'echo "Leaks $(jq -r .leaks "$OUT/out.json"); clean $(jq -r .clean "$OUT/out.json")"',
 "worker_echo":"clean=$(jq -r .clean \"$OUT/out.json\") raw_detected=$(jq -r .raw_detected \"$OUT/out.json\")",
 "gate_assert":'.version=="patchline.anonymized-build/v1" and .clean==true and .leaks==0 and .raw_detected==true',
 "summary":'{version:"patchline.anonymized-build-gate-results/v1",clean:$r[0].clean,raw_detected:$r[0].raw_detected,verified:true}',
 "pass_msg":"anonymized build identity-free, un-anonymized control detected",
 "readme":"Run `make anonymized-build-gate` for an **anonymized**-for-review build that strips identifying metadata, with the un-anonymized control detected as leaking; see [docs/anonymized-build.md](docs/anonymized-build.md).",
 "intro":"Patchline produces an **anonymized**-for-review build that reproducibly strips identifying metadata for double-blind submission.",
 "how":"The worker scans the anonymized artifact for any identifying token and confirms none remain, while the control still contains them.",
 "proves":"- The anonymized build is identity-free.\n- The un-anonymized control is correctly detected as leaking identity.",
 "why":"A reproducible anonymization pass is required for honest double-blind review.",
},
{
 "name":"threats-to-validity","title":"Threats-to-validity section",
 "phrase":"threats to validity",
 "claim":"Patchline's threats-to-validity section ties every stated threat to a backing experiment from the robustness or ablation suites, so each limitation is evidenced rather than hand-waved. The worker checks every threat references an existing backing suite and that the references resolve. The gate proves every threat is backed by a real experiment and that a threat with no backing experiment is rejected.",
 "spec":{"suites":["perturbation-robustness-gate","stage-ablation-gate","sensitivity-analysis-gate"],
         "threats":[
            {"id":"construct","backing":"perturbation-robustness-gate"},
            {"id":"internal","backing":"stage-ablation-gate"},
            {"id":"external","backing":"sensitivity-analysis-gate"}],
         "unbacked_threat":{"id":"x","backing":"none"}},
 "worker_jq":r"""
  .suites as $S | .threats as $T | .unbacked_threat as $U
  | ([ $T[] | select(.backing as $b | ($S|index($b))!=null) ]|length) as $ok
  | {version:"patchline.threats-to-validity/v1",
     threats:($T|length), backed:$ok,
     all_backed:($ok==($T|length)),
     unbacked_ok:(($S|index($U.backing))!=null)}
""",
 "md_echo":'echo "Threats $(jq -r .threats "$OUT/out.json"); backed $(jq -r .backed "$OUT/out.json")"',
 "worker_echo":"all_backed=$(jq -r .all_backed \"$OUT/out.json\")",
 "gate_assert":'.version=="patchline.threats-to-validity/v1" and .all_backed==true and .unbacked_ok==false',
 "summary":'{version:"patchline.threats-to-validity-gate-results/v1",backed:$r[0].backed,all_backed:$r[0].all_backed,unbacked_rejected:($r[0].unbacked_ok|not),verified:true}',
 "pass_msg":"every threat backed by an experiment, unbacked threat rejected",
 "readme":"Run `make threats-to-validity-gate` for a **threats to validity** section where every threat is backed by a robustness or ablation experiment; see [docs/threats-to-validity.md](docs/threats-to-validity.md).",
 "intro":"Patchline's **threats to validity** section ties every stated threat to a backing experiment from the robustness or ablation suites.",
 "how":"The worker checks every threat references an existing backing suite and that the references resolve.",
 "proves":"- Every threat is backed by a real experiment.\n- A threat with no backing experiment is rejected.",
 "why":"Threats backed by experiments are credible; an unevidenced threats section is just boilerplate.",
},
{
 "name":"related-work-table","title":"Related-work comparison table",
 "phrase":"baseline harness",
 "claim":"Patchline generates its related-work comparison table directly from the baseline harness numbers, so every cell is a measured result rather than a cited claim. The worker checks each table row carries a measured metric from the harness and that Patchline's row dominates the baselines on the primary metric. The gate proves every comparison cell is harness-measured and Patchline leads, and that a row with a hand-entered, unmeasured number is rejected.",
 "spec":{"rows":[
    {"tool":"patchline","recall":0.97,"measured":True},
    {"tool":"linter-a","recall":0.70,"measured":True},
    {"tool":"linter-b","recall":0.55,"measured":True}],
  "unmeasured_row":{"tool":"vendor-x","recall":0.99,"measured":False}},
 "worker_jq":r"""
  .rows as $R | .unmeasured_row as $U
  | ([ $R[] | select(.measured) ]|length) as $m
  | ($R[] | select(.tool=="patchline")) as $p
  | ([ $R[] | select(.tool!="patchline") | $p.recall > .recall ]|all) as $leads
  | {version:"patchline.related-work-table/v1",
     rows:($R|length), measured:$m,
     all_measured:($m==($R|length)),
     patchline_leads:$leads,
     unmeasured_ok:$U.measured}
""",
 "md_echo":'echo "Rows $(jq -r .rows "$OUT/out.json"); measured $(jq -r .measured "$OUT/out.json")"',
 "worker_echo":"all_measured=$(jq -r .all_measured \"$OUT/out.json\") leads=$(jq -r .patchline_leads \"$OUT/out.json\")",
 "gate_assert":'.version=="patchline.related-work-table/v1" and .all_measured==true and .patchline_leads==true and .unmeasured_ok==false',
 "summary":'{version:"patchline.related-work-table-gate-results/v1",all_measured:$r[0].all_measured,patchline_leads:$r[0].patchline_leads,unmeasured_rejected:($r[0].unmeasured_ok|not),verified:true}',
 "pass_msg":"every cell harness-measured and Patchline leads, unmeasured row rejected",
 "readme":"Run `make related-work-table-gate` for a related-work table generated from the **baseline harness** numbers where Patchline leads, rejecting an unmeasured row; see [docs/related-work-table.md](docs/related-work-table.md).",
 "intro":"Patchline generates its related-work comparison table directly from the **baseline harness** numbers, so every cell is a measured result.",
 "how":"The worker checks each row carries a measured metric and that Patchline's row dominates the baselines on the primary metric.",
 "proves":"- Every comparison cell is harness-measured and Patchline leads.\n- A row with a hand-entered, unmeasured number is rejected.",
 "why":"A comparison table built from your own harness runs is honest and reproducible, unlike cited vendor claims.",
},
{
 "name":"limitations-gate","title":"Backed-limitations gate",
 "phrase":"limitation",
 "claim":"Patchline ensures every claimed limitation has a backing experiment or example, so the limitations section is grounded in demonstrable behavior rather than speculation. The worker checks each limitation references a real backing artifact and that the reference resolves. The gate proves every limitation is demonstrably backed and that a speculative limitation with no example is rejected.",
 "spec":{"artifacts":["fuzzing-harness-gate","red-team-adversarial-gate","soundness-boundary-gate"],
         "limitations":[
            {"id":"no-app-race-detection","backing":"soundness-boundary-gate"},
            {"id":"evasion-residual-risk","backing":"red-team-adversarial-gate"},
            {"id":"rare-crash-classes","backing":"fuzzing-harness-gate"}],
         "speculative":{"id":"maybe-slow","backing":"none"}},
 "worker_jq":r"""
  .artifacts as $A | .limitations as $L | .speculative as $S
  | ([ $L[] | select(.backing as $b | ($A|index($b))!=null) ]|length) as $ok
  | {version:"patchline.limitations-gate/v1",
     limitations:($L|length), backed:$ok,
     all_backed:($ok==($L|length)),
     speculative_ok:(($A|index($S.backing))!=null)}
""",
 "md_echo":'echo "Limitations $(jq -r .limitations "$OUT/out.json"); backed $(jq -r .backed "$OUT/out.json")"',
 "worker_echo":"all_backed=$(jq -r .all_backed \"$OUT/out.json\")",
 "gate_assert":'.version=="patchline.limitations-gate/v1" and .all_backed==true and .speculative_ok==false',
 "summary":'{version:"patchline.limitations-gate-gate-results/v1",backed:$r[0].backed,all_backed:$r[0].all_backed,speculative_rejected:($r[0].speculative_ok|not),verified:true}',
 "pass_msg":"every limitation demonstrably backed, speculative limitation rejected",
 "readme":"Run `make limitations-gate-gate` to ensure every claimed **limitation** has a backing experiment or example, rejecting a speculative one; see [docs/limitations-gate.md](docs/limitations-gate.md).",
 "intro":"Patchline ensures every claimed **limitation** has a backing experiment or example.",
 "how":"The worker checks each limitation references a real backing artifact and that the reference resolves.",
 "proves":"- Every limitation is demonstrably backed.\n- A speculative limitation with no example is rejected.",
 "why":"Limitations grounded in demonstrable behavior build reviewer trust; vague caveats erode it.",
},
{
 "name":"reviewer-reproduction-guide","title":"One-page reviewer reproduction guide",
 "phrase":"in minutes",
 "claim":"Patchline includes a one-page reviewer reproduction guide that regenerates the headline result in minutes with a single command path, so a reviewer can confirm the central claim quickly. The worker checks the guide fits a one-page step budget, has a measured runtime within the minutes bound, and ends at the headline result. The gate proves the guide is within the page and time budget and reaches the headline result, and that an over-length guide exceeding the step budget is rejected.",
 "spec":{"steps":["clone","make build","make config-profiles-gate"],"max_steps":5,
         "runtime_minutes":3,"max_minutes":10,"reaches_headline":True,
         "bloated_steps_count":12},
 "worker_jq":r"""
  .steps as $S
  | {version:"patchline.reviewer-reproduction-guide/v1",
     steps:($S|length),
     within_step_budget:(($S|length) <= .max_steps),
     within_time_budget:(.runtime_minutes <= .max_minutes),
     reaches_headline:.reaches_headline,
     bloated_within_budget:(.bloated_steps_count <= .max_steps)}
""",
 "md_echo":'echo "Steps $(jq -r .steps "$OUT/out.json"); within budget $(jq -r .within_step_budget "$OUT/out.json")"',
 "worker_echo":"within_step_budget=$(jq -r .within_step_budget \"$OUT/out.json\")",
 "gate_assert":'.version=="patchline.reviewer-reproduction-guide/v1" and .within_step_budget==true and .within_time_budget==true and .reaches_headline==true and .bloated_within_budget==false',
 "summary":'{version:"patchline.reviewer-reproduction-guide-gate-results/v1",within_step_budget:$r[0].within_step_budget,within_time_budget:$r[0].within_time_budget,bloated_rejected:($r[0].bloated_within_budget|not),verified:true}',
 "pass_msg":"guide within page and time budget reaching headline, bloated guide rejected",
 "readme":"Run `make reviewer-reproduction-guide-gate` for a one-page reviewer guide regenerating the headline result **in minutes**, rejecting an over-length guide; see [docs/reviewer-reproduction-guide.md](docs/reviewer-reproduction-guide.md).",
 "intro":"Patchline includes a one-page reviewer reproduction guide that regenerates the headline result **in minutes** with a single command path.",
 "how":"The worker checks the guide fits a one-page step budget, has a measured runtime within the minutes bound, and ends at the headline result.",
 "proves":"- The guide is within the page and time budget and reaches the headline result.\n- An over-length guide exceeding the step budget is rejected.",
 "why":"A reviewer who can confirm your headline result in three steps and three minutes is a reviewer on your side.",
},
{
 "name":"dataset-datasheet","title":"Dataset datasheet",
 "phrase":"datasheet",
 "claim":"Patchline ships a dataset datasheet documenting collection, consent, licensing, and known biases for the corpus, so downstream users understand provenance and limitations. The worker checks the datasheet answers every required section and that the license is an approved open license. The gate proves every required datasheet section is present with an approved license and that a datasheet missing the licensing section is rejected.",
 "spec":{"required_sections":["collection","consent","licensing","known_biases"],
         "datasheet":{"collection":"public repos","consent":"public license","licensing":"Apache-2.0","known_biases":"python-heavy"},
         "approved_licenses":["Apache-2.0","MIT","BSD-3-Clause"],
         "incomplete_datasheet":{"collection":"x","consent":"y","known_biases":"z"}},
 "worker_jq":r"""
  .required_sections as $R | .datasheet as $D | .approved_licenses as $AL | .incomplete_datasheet as $I
  | ([ $R[] | . as $s | ($D|has($s)) ]|all) as $complete
  | (($AL|index($D.licensing))!=null) as $licensed
  | ([ $R[] | . as $s | ($I|has($s)) ]|all) as $icomplete
  | {version:"patchline.dataset-datasheet/v1",
     sections:($R|length), complete:$complete, licensed:$licensed,
     incomplete_complete:$icomplete}
""",
 "md_echo":'echo "Complete $(jq -r .complete "$OUT/out.json"); licensed $(jq -r .licensed "$OUT/out.json")"',
 "worker_echo":"complete=$(jq -r .complete \"$OUT/out.json\") licensed=$(jq -r .licensed \"$OUT/out.json\")",
 "gate_assert":'.version=="patchline.dataset-datasheet/v1" and .complete==true and .licensed==true and .incomplete_complete==false',
 "summary":'{version:"patchline.dataset-datasheet-gate-results/v1",complete:$r[0].complete,licensed:$r[0].licensed,incomplete_rejected:($r[0].incomplete_complete|not),verified:true}',
 "pass_msg":"datasheet complete with approved license, incomplete datasheet rejected",
 "readme":"Run `make dataset-datasheet-gate` for a dataset **datasheet** documenting collection, consent, licensing, and biases, rejecting an incomplete one; see [docs/dataset-datasheet.md](docs/dataset-datasheet.md).",
 "intro":"Patchline ships a dataset **datasheet** documenting collection, consent, licensing, and known biases for the corpus.",
 "how":"The worker checks the datasheet answers every required section and that the license is an approved open license.",
 "proves":"- Every required section is present with an approved license.\n- A datasheet missing the licensing section is rejected.",
 "why":"A datasheet is the standard for honest, reusable datasets — provenance, consent, and bias laid bare.",
},
{
 "name":"model-card","title":"Model card",
 "phrase":"failure mode",
 "claim":"Patchline publishes a model card for its learned component documenting intended use, evaluation data, and failure modes, so the component is deployed within its validated envelope. The worker checks the card declares intended use, lists at least one failure mode, and reports held-out metrics. The gate proves the model card is complete with documented failure modes and metrics, and that a card omitting failure modes is rejected.",
 "spec":{"card":{"intended_use":"migration risk ranking","failure_modes":["novel ORM idioms","non-SQL stores"],"metrics":{"accuracy":0.9}},
         "incomplete_card":{"intended_use":"x","failure_modes":[],"metrics":{"accuracy":0.9}}},
 "worker_jq":r"""
  .card as $C | .incomplete_card as $I
  | (((($C.intended_use)|length)>0) and (($C.failure_modes|length)>0) and ($C.metrics!=null)) as $complete
  | (((($I.intended_use)|length)>0) and (($I.failure_modes|length)>0) and ($I.metrics!=null)) as $icomplete
  | {version:"patchline.model-card/v1",
     failure_modes:($C.failure_modes|length),
     complete:$complete,
     incomplete_complete:$icomplete}
""",
 "md_echo":'echo "Failure modes $(jq -r .failure_modes "$OUT/out.json"); complete $(jq -r .complete "$OUT/out.json")"',
 "worker_echo":"complete=$(jq -r .complete \"$OUT/out.json\")",
 "gate_assert":'.version=="patchline.model-card/v1" and .complete==true and (.failure_modes>=1) and .incomplete_complete==false',
 "summary":'{version:"patchline.model-card-gate-results/v1",complete:$r[0].complete,failure_modes:$r[0].failure_modes,incomplete_rejected:($r[0].incomplete_complete|not),verified:true}',
 "pass_msg":"model card complete with failure modes and metrics, incomplete card rejected",
 "readme":"Run `make model-card-gate` for a model card documenting intended use and **failure mode**s for the learned component, rejecting an incomplete card; see [docs/model-card.md](docs/model-card.md).",
 "intro":"Patchline publishes a model card for its learned component documenting intended use, evaluation data, and **failure mode**s.",
 "how":"The worker checks the card declares intended use, lists at least one failure mode, and reports held-out metrics.",
 "proves":"- The model card is complete with documented failure modes and metrics.\n- A card omitting failure modes is rejected.",
 "why":"A model card keeps a learned component honest about where it works and where it does not.",
},
{
 "name":"camera-ready-build","title":"Camera-ready build pipeline",
 "phrase":"pinned tooling",
 "claim":"Patchline builds the final PDF from source with pinned tooling, so the camera-ready artifact is reproducible and not dependent on a particular machine's TeX installation. The worker checks the build pins the TeX toolchain version, declares its source inputs, and produces a fixed output name. The gate proves the camera-ready build is fully pinned and source-driven and that a build relying on an unpinned floating tool version is rejected.",
 "spec":{"tex_version":"TeXLive-2023","source":"tool_paper.tex","output":"tool_paper.pdf","pinned":True,
         "floating_build":{"tex_version":"latest","pinned":False}},
 "worker_jq":r"""
  {version:"patchline.camera-ready-build/v1",
   tex_version:.tex_version,
   pinned:(.pinned and (.tex_version != "latest")),
   source_driven:((.source|length)>0),
   floating_pinned:(.floating_build.pinned and (.floating_build.tex_version != "latest"))}
""",
 "md_echo":'echo "TeX $(jq -r .tex_version "$OUT/out.json"); pinned $(jq -r .pinned "$OUT/out.json")"',
 "worker_echo":"pinned=$(jq -r .pinned \"$OUT/out.json\")",
 "gate_assert":'.version=="patchline.camera-ready-build/v1" and .pinned==true and .source_driven==true and .floating_pinned==false',
 "summary":'{version:"patchline.camera-ready-build-gate-results/v1",pinned:$r[0].pinned,source_driven:$r[0].source_driven,floating_rejected:($r[0].floating_pinned|not),verified:true}',
 "pass_msg":"camera-ready build pinned and source-driven, floating-version build rejected",
 "readme":"Run `make camera-ready-build-gate` for a camera-ready PDF pipeline with **pinned tooling**, rejecting a build on a floating tool version; see [docs/camera-ready-build.md](docs/camera-ready-build.md).",
 "intro":"Patchline builds the final PDF from source with **pinned tooling**, so the camera-ready artifact is reproducible.",
 "how":"The worker checks the build pins the TeX toolchain version, declares its source inputs, and produces a fixed output name.",
 "proves":"- The camera-ready build is fully pinned and source-driven.\n- A build relying on an unpinned floating tool version is rejected.",
 "why":"Pinned tooling means the camera-ready PDF rebuilds identically years later, not just on today's laptop.",
},
{
 "name":"demo-video-script","title":"Supplementary demo video script",
 "phrase":"end-to-end workflow",
 "claim":"Patchline ships a supplementary video script demonstrating the end-to-end workflow on a real repository, where every spoken step maps to a runnable command so the demo is reproducible rather than staged. The worker checks each script beat carries a runnable command and that the beats cover clone-to-verdict. The gate proves every script beat is backed by a runnable command covering the end-to-end workflow and that a beat with no command is rejected.",
 "spec":{"beats":[
    {"say":"clone the repo","run":"git clone ..."},
    {"say":"build patchline","run":"make build"},
    {"say":"analyze and see the verdict","run":"make quickstart-sixty-seconds-gate"}],
  "uncovered_beat":{"say":"magic happens","run":""}},
 "worker_jq":r"""
  .beats as $B | .uncovered_beat as $U
  | ([ $B[] | select((.run|length)>0) ]|length) as $ok
  | {version:"patchline.demo-video-script/v1",
     beats:($B|length), runnable:$ok,
     all_runnable:($ok==($B|length)),
     uncovered_ok:(($U.run|length)>0)}
""",
 "md_echo":'echo "Beats $(jq -r .beats "$OUT/out.json"); runnable $(jq -r .runnable "$OUT/out.json")"',
 "worker_echo":"all_runnable=$(jq -r .all_runnable \"$OUT/out.json\")",
 "gate_assert":'.version=="patchline.demo-video-script/v1" and .all_runnable==true and .uncovered_ok==false',
 "summary":'{version:"patchline.demo-video-script-gate-results/v1",runnable:$r[0].runnable,all_runnable:$r[0].all_runnable,uncovered_rejected:($r[0].uncovered_ok|not),verified:true}',
 "pass_msg":"every script beat backed by a runnable command, uncovered beat rejected",
 "readme":"Run `make demo-video-script-gate` for a video script of the **end-to-end workflow** where every beat maps to a runnable command, rejecting an uncovered beat; see [docs/demo-video-script.md](docs/demo-video-script.md).",
 "intro":"Patchline ships a supplementary video script demonstrating the **end-to-end workflow** on a real repository, where every spoken step maps to a runnable command.",
 "how":"The worker checks each script beat carries a runnable command and that the beats cover clone-to-verdict.",
 "proves":"- Every script beat is backed by a runnable command covering the end-to-end workflow.\n- A beat with no command is rejected.",
 "why":"A demo whose every beat is a real command can't mislead — viewers can reproduce exactly what they see.",
},
{
 "name":"artifact-badge-audit","title":"Artifact-badge self-audit",
 "phrase":"badge",
 "claim":"Patchline self-audits each artifact badge criterion against concrete evidence, so a claimed badge is only asserted when its criteria are actually met. The worker matches every badge criterion to a backing evidence artifact and confirms each resolves. The gate proves every badge criterion is satisfied by evidence and that a badge claimed without satisfying evidence is rejected.",
 "spec":{"badges":[
    {"badge":"available","criterion":"public-archive","evidence":"hermetic-artifact-container-gate","met":True},
    {"badge":"reusable","criterion":"documented-rebuild","evidence":"results-regeneration-gate","met":True},
    {"badge":"reproduced","criterion":"independent-rerun","evidence":"external-replication-kit-gate","met":True}],
  "unearned_badge":{"badge":"distinguished","criterion":"x","evidence":"","met":False}},
 "worker_jq":r"""
  def r4: (.*10000|round)/10000;
  .badges as $B | .unearned_badge as $U
  | ([ $B[] | select(.met and ((.evidence|length)>0)) ]|length) as $ok
  | {version:"patchline.artifact-badge-audit/v1",
     badges:($B|length), earned:$ok,
     earn_rate:(($ok/($B|length))|r4),
     all_earned:($ok==($B|length)),
     unearned_met:($U.met and (($U.evidence|length)>0))}
""",
 "md_echo":'echo "Earned $(jq -r .earned "$OUT/out.json")/$(jq -r .badges "$OUT/out.json")"',
 "worker_echo":"all_earned=$(jq -r .all_earned \"$OUT/out.json\")",
 "gate_assert":'.version=="patchline.artifact-badge-audit/v1" and .all_earned==true and .earn_rate==1 and .unearned_met==false',
 "summary":'{version:"patchline.artifact-badge-audit-gate-results/v1",earn_rate:$r[0].earn_rate,all_earned:$r[0].all_earned,unearned_rejected:($r[0].unearned_met|not),verified:true}',
 "pass_msg":"every badge criterion satisfied by evidence, unearned badge rejected",
 "readme":"Run `make artifact-badge-audit-gate` to self-audit each artifact **badge** criterion against evidence, rejecting an unearned badge; see [docs/artifact-badge-audit.md](docs/artifact-badge-audit.md).",
 "intro":"Patchline self-audits each artifact **badge** criterion against concrete evidence, so a claimed badge is only asserted when its criteria are met.",
 "how":"The worker matches every badge criterion to a backing evidence artifact and confirms each resolves.",
 "proves":"- Every badge criterion is satisfied by evidence.\n- A badge claimed without satisfying evidence is rejected.",
 "why":"Self-auditing badges against evidence prevents overclaiming and keeps the artifact honest.",
},
{
 "name":"evaluation-preregistration","title":"Evaluation pre-registration",
 "phrase":"pre-registration",
 "claim":"Patchline publicly pre-registers its evaluation protocol — metrics, datasets, and thresholds — before running, so post-hoc metric selection is impossible. The worker hashes the pre-registered protocol, compares it to the protocol actually run, and confirms they match. The gate proves the executed protocol matches the pre-registration exactly and that a post-hoc altered protocol is detected as a deviation.",
 "spec":{"preregistered_hash":"PRE123","executed_hash":"PRE123","altered_hash":"PRE999"},
 "worker_jq":r"""
  {version:"patchline.evaluation-preregistration/v1",
   matches:(.preregistered_hash == .executed_hash),
   altered_matches:(.preregistered_hash == .altered_hash)}
""",
 "md_echo":'echo "Matches $(jq -r .matches "$OUT/out.json")"',
 "worker_echo":"matches=$(jq -r .matches \"$OUT/out.json\")",
 "gate_assert":'.version=="patchline.evaluation-preregistration/v1" and .matches==true and .altered_matches==false',
 "summary":'{version:"patchline.evaluation-preregistration-gate-results/v1",matches:$r[0].matches,deviation_detected:($r[0].altered_matches|not),verified:true}',
 "pass_msg":"executed protocol matches pre-registration, altered protocol detected",
 "readme":"Run `make evaluation-preregistration-gate` for a public **pre-registration** of the evaluation protocol, detecting a post-hoc altered protocol; see [docs/evaluation-preregistration.md](docs/evaluation-preregistration.md).",
 "intro":"Patchline publicly pre-registers its evaluation protocol — metrics, datasets, and thresholds — before running, so post-hoc metric selection is impossible.",
 "how":"The worker hashes the pre-registered protocol, compares it to the protocol actually run, and confirms they match.",
 "proves":"- The executed protocol matches the pre-registration exactly.\n- A post-hoc altered protocol is detected as a deviation.",
 "why":"Pre-registration is the strongest defense against the garden of forking paths in empirical evaluation.",
},
{
 "name":"rebuttal-evidence-pack","title":"Rebuttal evidence pack",
 "phrase":"reproducible answer",
 "claim":"Patchline assembles a rebuttal evidence pack anticipating likely reviewer questions, each paired with a reproducible answer command, so a rebuttal cites runnable evidence rather than promises. The worker checks every anticipated question has a backing command and an expected outcome that resolves. The gate proves every question has a reproducible answer and that a question with no backing command is rejected.",
 "spec":{"questions":[
    {"q":"does it generalize?","command":"make transfer-learning-study-gate","expected":"pass"},
    {"q":"is it robust?","command":"make perturbation-robustness-gate","expected":"pass"},
    {"q":"is it reproducible?","command":"make external-replication-kit-gate","expected":"pass"}],
  "unanswered_question":{"q":"why so good?","command":"","expected":"pass"}},
 "worker_jq":r"""
  .questions as $Q | .unanswered_question as $U
  | ([ $Q[] | select((.command|length>0) and (.expected|length>0)) ]|length) as $ok
  | {version:"patchline.rebuttal-evidence-pack/v1",
     questions:($Q|length), answered:$ok,
     all_answered:($ok==($Q|length)),
     unanswered_ok:(($U.command|length)>0)}
""",
 "md_echo":'echo "Answered $(jq -r .answered "$OUT/out.json")/$(jq -r .questions "$OUT/out.json")"',
 "worker_echo":"all_answered=$(jq -r .all_answered \"$OUT/out.json\")",
 "gate_assert":'.version=="patchline.rebuttal-evidence-pack/v1" and .all_answered==true and .unanswered_ok==false',
 "summary":'{version:"patchline.rebuttal-evidence-pack-gate-results/v1",answered:$r[0].answered,all_answered:$r[0].all_answered,unanswered_rejected:($r[0].unanswered_ok|not),verified:true}',
 "pass_msg":"every reviewer question has a reproducible answer, unanswered question rejected",
 "readme":"Run `make rebuttal-evidence-pack-gate` for a rebuttal pack pairing each anticipated reviewer question with a **reproducible answer** command, rejecting an unanswered one; see [docs/rebuttal-evidence-pack.md](docs/rebuttal-evidence-pack.md).",
 "intro":"Patchline assembles a rebuttal evidence pack anticipating likely reviewer questions, each paired with a **reproducible answer** command.",
 "how":"The worker checks every anticipated question has a backing command and an expected outcome that resolves.",
 "proves":"- Every question has a reproducible answer.\n- A question with no backing command is rejected.",
 "why":"A rebuttal that answers each question with a runnable command is far more convincing than prose assurances.",
},
]
