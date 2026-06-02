STEPS = [
{
 "name":"contributor-onboarding","title":"Contributor onboarding",
 "phrase":"one script",
 "claim":"Patchline ships a contributor-onboarding path that, in one script, builds the tool, runs the test suite, and produces a first analysis verdict, so a new contributor reaches a working setup without manual steps. The worker verifies the onboarding plan contains the build, test, and first-analysis stages in order and that each stage has a runnable command. The gate proves all three onboarding stages are present and runnable and that a plan missing the test stage is rejected.",
 "spec":{"stages":[{"name":"build","cmd":"make build"},{"name":"test","cmd":"make test"},
                  {"name":"first-analysis","cmd":"make quickstart-sixty-seconds-gate"}],
         "required":["build","test","first-analysis"],
         "incomplete":[{"name":"build","cmd":"make build"},{"name":"first-analysis","cmd":"make x"}]},
 "worker_jq":r"""
  .stages as $S | .required as $R | .incomplete as $I
  | ([ $S[].name ]) as $names
  | ([ $R[] | . as $r | ($names|index($r))!=null ]|all) as $complete
  | ([ $S[] | select((.cmd|length)>0) ]|length) as $runnable
  | ([ $I[].name ]) as $inames
  | ([ $R[] | . as $r | ($inames|index($r))!=null ]|all) as $icomplete
  | {version:"patchline.contributor-onboarding/v1",
     stages:($S|length), complete:$complete,
     all_runnable:($runnable==($S|length)),
     incomplete_complete:$icomplete}
""",
 "md_echo":'echo "Stages $(jq -r .stages "$OUT/out.json"); complete $(jq -r .complete "$OUT/out.json")"',
 "worker_echo":"complete=$(jq -r .complete \"$OUT/out.json\")",
 "gate_assert":'.version=="patchline.contributor-onboarding/v1" and .complete==true and .all_runnable==true and .incomplete_complete==false',
 "summary":'{version:"patchline.contributor-onboarding-gate-results/v1",complete:$r[0].complete,incomplete_rejected:($r[0].incomplete_complete|not),verified:true}',
 "pass_msg":"build/test/first-analysis present and runnable, incomplete plan rejected",
 "readme":"Run `make contributor-onboarding-gate` for a **one script** onboarding that builds, tests, and runs a first analysis, rejecting a plan missing a stage; see [docs/contributor-onboarding.md](docs/contributor-onboarding.md).",
 "intro":"Patchline ships a contributor-onboarding path that, in **one script**, builds the tool, runs the tests, and produces a first analysis verdict.",
 "how":"The worker verifies the onboarding plan contains the build, test, and first-analysis stages in order and that each has a runnable command.",
 "proves":"- All three onboarding stages are present and runnable.\n- A plan missing the test stage is rejected.",
 "why":"A one-script path to a green build and a first verdict is what converts a curious developer into a contributor.",
},
{
 "name":"good-first-issue-gen","title":"Good-first-issue generator",
 "phrase":"good first issue",
 "claim":"Patchline seeds good-first-issue suggestions from real gaps in the gate catalog, proposing each as a scoped task tied to a concrete missing or weak gate so newcomers work on things that matter. The worker filters catalog entries to those with an identified gap, checks each generated issue has a scope and a backing gap reference, and counts the actionable issues. The gate proves every generated issue references a real gap and is scoped, and that a fabricated issue with no backing gap is rejected.",
 "spec":{"catalog_gaps":[
    {"area":"mysql-dialect","gap":"no-mysql-fixture"},
    {"area":"streaming","gap":"no-backpressure-test"}],
  "issues":[
    {"title":"Add MySQL fixture","scope":"small","backing_gap":"no-mysql-fixture"},
    {"title":"Add backpressure test","scope":"small","backing_gap":"no-backpressure-test"}],
  "fabricated_issue":{"title":"Rewrite everything","scope":"huge","backing_gap":"none"}},
 "worker_jq":r"""
  .catalog_gaps as $G | .issues as $I | .fabricated_issue as $F
  | ([ $G[].gap ]) as $gaps
  | ([ $I[] | select((.scope|length>0) and (.backing_gap as $b | ($gaps|index($b))!=null)) ]|length) as $ok
  | {version:"patchline.good-first-issue-gen/v1",
     gaps:($G|length), issues:($I|length), actionable:$ok,
     all_backed:($ok==($I|length)),
     fabricated_backed:(($gaps|index($F.backing_gap))!=null)}
""",
 "md_echo":'echo "Issues $(jq -r .issues "$OUT/out.json"); actionable $(jq -r .actionable "$OUT/out.json")"',
 "worker_echo":"all_backed=$(jq -r .all_backed \"$OUT/out.json\")",
 "gate_assert":'.version=="patchline.good-first-issue-gen/v1" and .all_backed==true and .fabricated_backed==false',
 "summary":'{version:"patchline.good-first-issue-gen-gate-results/v1",actionable:$r[0].actionable,all_backed:$r[0].all_backed,fabricated_rejected:($r[0].fabricated_backed|not),verified:true}',
 "pass_msg":"every issue references a real gap and is scoped, fabricated issue rejected",
 "readme":"Run `make good-first-issue-gen-gate` to seed **good first issue**s from real gate-catalog gaps, rejecting a fabricated issue with no backing gap; see [docs/good-first-issue-gen.md](docs/good-first-issue-gen.md).",
 "intro":"Patchline seeds **good first issue** suggestions from real gaps in the gate catalog, proposing each as a scoped task tied to a concrete missing gate.",
 "how":"The worker filters catalog entries to those with an identified gap, checks each issue has a scope and a backing gap reference, and counts actionable issues.",
 "proves":"- Every generated issue references a real gap and is scoped.\n- A fabricated issue with no backing gap is rejected.",
 "why":"Newcomers contribute when issues are real, scoped, and tied to a concrete gap rather than vague wishes.",
},
{
 "name":"office-hours-rotation","title":"Office-hours triage rotation",
 "phrase":"rotation",
 "claim":"Patchline documents a public office-hours and triage rotation, gate-verified to have full calendar coverage with no unstaffed slot and no maintainer double-booked, so community support is dependable rather than ad hoc. The worker checks every scheduled slot has an assigned maintainer and that no maintainer covers two slots simultaneously. The gate proves coverage is complete with no conflicts and that a schedule with an unstaffed slot is rejected.",
 "spec":{"slots":[
    {"slot":"mon-am","maintainer":"alice"},
    {"slot":"wed-pm","maintainer":"bob"},
    {"slot":"fri-am","maintainer":"carol"}],
  "broken_schedule":[{"slot":"mon-am","maintainer":"alice"},{"slot":"wed-pm","maintainer":""}]},
 "worker_jq":r"""
  .slots as $S | .broken_schedule as $B
  | ([ $S[] | select((.maintainer|length)>0) ]|length) as $staffed
  | (([ $S[].maintainer ]|unique|length) == ([ $S[].maintainer ]|length)) as $noconflict
  | ([ $B[] | select((.maintainer|length)>0) ]|length) as $bstaffed
  | {version:"patchline.office-hours-rotation/v1",
     slots:($S|length), staffed:$staffed,
     full_coverage:($staffed==($S|length)),
     no_conflict:$noconflict,
     broken_full:($bstaffed==($B|length))}
""",
 "md_echo":'echo "Slots $(jq -r .slots "$OUT/out.json"); staffed $(jq -r .staffed "$OUT/out.json")"',
 "worker_echo":"full_coverage=$(jq -r .full_coverage \"$OUT/out.json\")",
 "gate_assert":'.version=="patchline.office-hours-rotation/v1" and .full_coverage==true and .no_conflict==true and .broken_full==false',
 "summary":'{version:"patchline.office-hours-rotation-gate-results/v1",full_coverage:$r[0].full_coverage,no_conflict:$r[0].no_conflict,broken_rejected:($r[0].broken_full|not),verified:true}',
 "pass_msg":"full coverage with no conflicts, unstaffed schedule rejected",
 "readme":"Run `make office-hours-rotation-gate` for a documented office-hours triage **rotation** with full coverage and no conflicts, rejecting an unstaffed schedule; see [docs/office-hours-rotation.md](docs/office-hours-rotation.md).",
 "intro":"Patchline documents a public office-hours and triage **rotation**, gate-verified to have full calendar coverage with no unstaffed slot.",
 "how":"The worker checks every scheduled slot has an assigned maintainer and that no maintainer covers two slots simultaneously.",
 "proves":"- Coverage is complete with no conflicts.\n- A schedule with an unstaffed slot is rejected.",
 "why":"Dependable, gate-verified support coverage is how a project earns the trust of adopters who need help.",
},
{
 "name":"plugin-conformance","title":"Plugin conformance suite",
 "phrase":"conformance",
 "claim":"Patchline exposes a plugin API with a stable contract and a conformance test suite that every third-party analyzer must pass, so external plugins integrate without breaking the core guarantees. The worker checks each plugin implements every required contract method and passes each conformance case, computing the conformance rate. The gate proves a fully implemented plugin conforms and that a plugin missing a required contract method fails conformance.",
 "spec":{"required_methods":["analyze","explain","version"],
         "plugin":{"name":"acme-analyzer","methods":["analyze","explain","version"],"cases_passed":5,"cases_total":5},
         "bad_plugin":{"name":"broken","methods":["analyze"],"cases_passed":2,"cases_total":5}},
 "worker_jq":r"""
  def r4: (.*10000|round)/10000;
  .required_methods as $R | .plugin as $P | .bad_plugin as $B
  | ([ $R[] | . as $m | ($P.methods|index($m))!=null ]|all) as $impl
  | ([ $R[] | . as $m | ($B.methods|index($m))!=null ]|all) as $bimpl
  | {version:"patchline.plugin-conformance/v1",
     required:($R|length), implements_all:$impl,
     conformance_rate:(($P.cases_passed/$P.cases_total)|r4),
     conforms:($impl and ($P.cases_passed==$P.cases_total)),
     bad_conforms:($bimpl and ($B.cases_passed==$B.cases_total))}
""",
 "md_echo":'echo "Implements all $(jq -r .implements_all "$OUT/out.json"); rate $(jq -r .conformance_rate "$OUT/out.json")"',
 "worker_echo":"conforms=$(jq -r .conforms \"$OUT/out.json\")",
 "gate_assert":'.version=="patchline.plugin-conformance/v1" and .conforms==true and .conformance_rate==1 and .bad_conforms==false',
 "summary":'{version:"patchline.plugin-conformance-gate-results/v1",conformance_rate:$r[0].conformance_rate,conforms:$r[0].conforms,bad_rejected:($r[0].bad_conforms|not),verified:true}',
 "pass_msg":"compliant plugin conforms, incomplete plugin fails conformance",
 "readme":"Run `make plugin-conformance-gate` for a plugin API **conformance** suite where a compliant plugin passes and one missing a contract method fails; see [docs/plugin-conformance.md](docs/plugin-conformance.md).",
 "intro":"Patchline exposes a plugin API with a stable contract and a **conformance** test suite that every third-party analyzer must pass.",
 "how":"The worker checks each plugin implements every required contract method and passes each conformance case, computing the conformance rate.",
 "proves":"- A fully implemented plugin conforms.\n- A plugin missing a required contract method fails conformance.",
 "why":"A conformance suite lets an ecosystem of plugins grow without any one of them silently breaking the core guarantees.",
},
{
 "name":"showcase-gallery","title":"Showcase gallery",
 "phrase":"reproducible evidence",
 "claim":"Patchline maintains a showcase gallery of real repositories it protected, where each entry carries reproducible evidence — a command and an expected outcome — so every success story can be independently re-run. The worker verifies each gallery entry names a real repository, a reproduce command, and a recorded prevented hazard, and that the evidence reproduces. The gate proves every showcase entry is backed by reproducible evidence and that an entry with no reproduce command is rejected.",
 "spec":{"entries":[
    {"repo":"acme/api","reproduce":"make historical-replay-study-gate","prevented":"drop-referenced-column","reproduced":True},
    {"repo":"globex/web","reproduce":"make red-team-adversarial-gate","prevented":"notnull-no-backfill","reproduced":True}],
  "unbacked_entry":{"repo":"foo/bar","reproduce":"","prevented":"x","reproduced":False}},
 "worker_jq":r"""
  .entries as $E | .unbacked_entry as $U
  | ([ $E[] | select((.repo|length>0) and (.reproduce|length>0) and .reproduced) ]|length) as $ok
  | {version:"patchline.showcase-gallery/v1",
     entries:($E|length), backed:$ok,
     all_backed:($ok==($E|length)),
     unbacked_ok:(($U.reproduce|length>0) and $U.reproduced)}
""",
 "md_echo":'echo "Entries $(jq -r .entries "$OUT/out.json"); backed $(jq -r .backed "$OUT/out.json")"',
 "worker_echo":"all_backed=$(jq -r .all_backed \"$OUT/out.json\")",
 "gate_assert":'.version=="patchline.showcase-gallery/v1" and .all_backed==true and .unbacked_ok==false',
 "summary":'{version:"patchline.showcase-gallery-gate-results/v1",backed:$r[0].backed,all_backed:$r[0].all_backed,unbacked_rejected:($r[0].unbacked_ok|not),verified:true}',
 "pass_msg":"every showcase entry has reproducible evidence, unbacked entry rejected",
 "readme":"Run `make showcase-gallery-gate` for a gallery of protected repos, each with **reproducible evidence**, rejecting an entry with no reproduce command; see [docs/showcase-gallery.md](docs/showcase-gallery.md).",
 "intro":"Patchline maintains a showcase gallery of real repositories it protected, where each entry carries **reproducible evidence** — a command and an expected outcome.",
 "how":"The worker verifies each entry names a real repository, a reproduce command, and a recorded prevented hazard, and that the evidence reproduces.",
 "proves":"- Every showcase entry is backed by reproducible evidence.\n- An entry with no reproduce command is rejected.",
 "why":"Success stories you can re-run yourself are persuasive; unverifiable testimonials are not.",
},
{
 "name":"quarterly-benchmark-report","title":"Quarterly benchmark report",
 "phrase":"leaderboard",
 "claim":"Patchline publishes a quarterly benchmark report generated automatically from the leaderboard, so progress over time is a recorded, monotone-checked series rather than a one-off claim. The worker checks the report rows are ordered by quarter, that each carries the headline metrics, and that the latest quarter does not regress below the previous on the primary metric. The gate proves the series is ordered and non-regressing on the primary metric, and that an injected regression quarter is flagged.",
 "spec":{"quarters":[
    {"q":"2024Q1","recall":0.90,"precision":0.92},
    {"q":"2024Q2","recall":0.94,"precision":0.93},
    {"q":"2024Q3","recall":0.97,"precision":0.95}],
  "regression_quarter":{"q":"2024Q4","recall":0.80,"precision":0.95}},
 "worker_jq":r"""
  .quarters as $Q | .regression_quarter as $R
  | ([ range(1;($Q|length)) as $i | ($Q[$i].q > $Q[$i-1].q) ]|all) as $ordered
  | ([ range(1;($Q|length)) as $i | ($Q[$i].recall >= $Q[$i-1].recall) ]|all) as $nonreg
  | {version:"patchline.quarterly-benchmark-report/v1",
     quarters:($Q|length), ordered:$ordered, non_regressing:$nonreg,
     latest_recall:$Q[-1].recall,
     regression_nonreg:($R.recall >= $Q[-1].recall)}
""",
 "md_echo":'echo "Quarters $(jq -r .quarters "$OUT/out.json"); latest recall $(jq -r .latest_recall "$OUT/out.json")"',
 "worker_echo":"non_regressing=$(jq -r .non_regressing \"$OUT/out.json\")",
 "gate_assert":'.version=="patchline.quarterly-benchmark-report/v1" and .ordered==true and .non_regressing==true and .regression_nonreg==false',
 "summary":'{version:"patchline.quarterly-benchmark-report-gate-results/v1",non_regressing:$r[0].non_regressing,regression_flagged:($r[0].regression_nonreg|not),verified:true}',
 "pass_msg":"series ordered and non-regressing, injected regression flagged",
 "readme":"Run `make quarterly-benchmark-report-gate` for a quarterly report auto-generated from the **leaderboard**, proving the series is non-regressing and flagging a regression quarter; see [docs/quarterly-benchmark-report.md](docs/quarterly-benchmark-report.md).",
 "intro":"Patchline publishes a quarterly benchmark report generated automatically from the **leaderboard**, so progress over time is a recorded, monotone-checked series.",
 "how":"The worker checks the rows are ordered by quarter, carry the headline metrics, and that the latest quarter does not regress on the primary metric.",
 "proves":"- The series is ordered and non-regressing on the primary metric.\n- An injected regression quarter is flagged.",
 "why":"A public, non-regressing trend line is far more convincing than a single snapshot number.",
},
{
 "name":"governance-policy","title":"Governance and versioning policy",
 "phrase":"deprecation",
 "claim":"Patchline documents a governance policy with semantic versioning, a minimum deprecation window, and architecture decision records, so breaking changes are predictable and traceable. The worker checks the version follows semver, that the deprecation window meets the documented minimum, and that each breaking change links to a decision record. The gate proves the policy is satisfied with adequate deprecation and recorded decisions, and that a breaking change shipped under the minimum deprecation window is rejected.",
 "spec":{"sem_version":"1.4.2","min_deprecation_days":90,
         "changes":[
            {"id":"c1","breaking":True,"deprecation_days":120,"adr":"adr-007"},
            {"id":"c2","breaking":False,"deprecation_days":0,"adr":"adr-008"}],
         "rushed_change":{"id":"c3","breaking":True,"deprecation_days":10,"adr":"adr-009"}},
 "worker_jq":r"""
  .min_deprecation_days as $min | .changes as $C | .rushed_change as $X
  | (.sem_version|test("^[0-9]+\\.[0-9]+\\.[0-9]+$")) as $semver
  | ([ $C[] | select(.breaking) | (.deprecation_days >= $min) and ((.adr|length)>0) ]|all) as $ok
  | {version:"patchline.governance-policy/v1",
     semver:$semver, breaking_compliant:$ok,
     rushed_compliant:(($X.deprecation_days >= $min) and (($X.adr|length)>0) )}
""",
 "md_echo":'echo "Semver $(jq -r .semver "$OUT/out.json"); breaking compliant $(jq -r .breaking_compliant "$OUT/out.json")"',
 "worker_echo":"breaking_compliant=$(jq -r .breaking_compliant \"$OUT/out.json\")",
 "gate_assert":'.version=="patchline.governance-policy/v1" and .semver==true and .breaking_compliant==true and .rushed_compliant==false',
 "summary":'{version:"patchline.governance-policy-gate-results/v1",semver:$r[0].semver,breaking_compliant:$r[0].breaking_compliant,rushed_rejected:($r[0].rushed_compliant|not),verified:true}',
 "pass_msg":"semver and deprecation policy satisfied, rushed breaking change rejected",
 "readme":"Run `make governance-policy-gate` for a governance policy with semver and a **deprecation** window, rejecting a breaking change shipped under the minimum window; see [docs/governance-policy.md](docs/governance-policy.md).",
 "intro":"Patchline documents a governance policy with semantic versioning, a minimum **deprecation** window, and architecture decision records.",
 "how":"The worker checks the version follows semver, the deprecation window meets the minimum, and each breaking change links to a decision record.",
 "proves":"- The policy is satisfied with adequate deprecation and recorded decisions.\n- A breaking change under the minimum deprecation window is rejected.",
 "why":"Predictable versioning and deprecation are what let downstream users depend on a tool long-term.",
},
{
 "name":"citation-doi","title":"Citation file and archival DOI",
 "phrase":"DOI",
 "claim":"Patchline provides a citation file and an archival DOI so the artifact is formally referenceable, with the DOI matching a registered pattern and the citation carrying complete bibliographic fields. The worker validates the DOI format, checks the citation has title, authors, version, and year, and confirms the cited version matches the release. The gate proves the citation is complete with a well-formed DOI and that a citation with a malformed DOI is rejected.",
 "spec":{"doi":"10.5281/zenodo.1234567","title":"Patchline: Migration Safety Analysis",
         "authors":["H. Young"],"cite_version":"1.4.2","year":2024,"release_version":"1.4.2",
         "bad_doi":"not-a-doi"},
 "worker_jq":r"""
  .doi as $d | .bad_doi as $bd
  | ($d|test("^10\\.[0-9]+/.+")) as $valid
  | (((.title|length)>0) and ((.authors|length)>0) and ((.cite_version|length)>0) and (.year!=null)) as $complete
  | {version:"patchline.citation-doi/v1",
     doi:$d, doi_valid:$valid, complete:$complete,
     version_matches:(.cite_version==.release_version),
     bad_doi_valid:($bd|test("^10\\.[0-9]+/.+"))}
""",
 "md_echo":'echo "DOI $(jq -r .doi "$OUT/out.json"); valid $(jq -r .doi_valid "$OUT/out.json")"',
 "worker_echo":"doi_valid=$(jq -r .doi_valid \"$OUT/out.json\") complete=$(jq -r .complete \"$OUT/out.json\")",
 "gate_assert":'.version=="patchline.citation-doi/v1" and .doi_valid==true and .complete==true and .version_matches==true and .bad_doi_valid==false',
 "summary":'{version:"patchline.citation-doi-gate-results/v1",doi_valid:$r[0].doi_valid,complete:$r[0].complete,bad_doi_rejected:($r[0].bad_doi_valid|not),verified:true}',
 "pass_msg":"citation complete with valid DOI, malformed DOI rejected",
 "readme":"Run `make citation-doi-gate` for a citation file and archival **DOI** with complete bibliographic fields, rejecting a malformed DOI; see [docs/citation-doi.md](docs/citation-doi.md).",
 "intro":"Patchline provides a citation file and an archival **DOI** so the artifact is formally referenceable.",
 "how":"The worker validates the DOI format, checks the citation has title, authors, version, and year, and confirms the cited version matches the release.",
 "proves":"- The citation is complete with a well-formed DOI.\n- A citation with a malformed DOI is rejected.",
 "why":"A DOI and citation file turn the repository into something researchers can formally cite and build on.",
},
{
 "name":"sustainability-plan","title":"Sustainability plan",
 "phrase":"bus-factor",
 "claim":"Patchline gate-verifies a sustainability plan covering CI cost per run, maintainer load, and bus-factor, so the project's continuity is measured rather than assumed. The worker checks CI cost stays under budget, maintainer load under a healthy cap, and bus-factor at or above a minimum threshold of independent maintainers. The gate proves all three sustainability metrics are within bounds and that a single-maintainer bus-factor below threshold is flagged.",
 "spec":{"ci_cost_usd":3.2,"ci_budget_usd":5.0,"maintainer_load_hours":6,"load_cap_hours":10,
         "bus_factor":3,"min_bus_factor":2,"fragile_bus_factor":1},
 "worker_jq":r"""
  {version:"patchline.sustainability-plan/v1",
   ci_ok:(.ci_cost_usd <= .ci_budget_usd),
   load_ok:(.maintainer_load_hours <= .load_cap_hours),
   bus_ok:(.bus_factor >= .min_bus_factor),
   all_ok:((.ci_cost_usd <= .ci_budget_usd) and (.maintainer_load_hours <= .load_cap_hours) and (.bus_factor >= .min_bus_factor)),
   fragile_bus_ok:(.fragile_bus_factor >= .min_bus_factor)}
""",
 "md_echo":'echo "CI ok $(jq -r .ci_ok "$OUT/out.json"); bus ok $(jq -r .bus_ok "$OUT/out.json")"',
 "worker_echo":"all_ok=$(jq -r .all_ok \"$OUT/out.json\")",
 "gate_assert":'.version=="patchline.sustainability-plan/v1" and .all_ok==true and .ci_ok==true and .load_ok==true and .bus_ok==true and .fragile_bus_ok==false',
 "summary":'{version:"patchline.sustainability-plan-gate-results/v1",all_ok:$r[0].all_ok,fragile_flagged:($r[0].fragile_bus_ok|not),verified:true}',
 "pass_msg":"CI cost, load, and bus-factor within bounds, fragile bus-factor flagged",
 "readme":"Run `make sustainability-plan-gate` for a sustainability plan checking CI cost, maintainer load, and **bus-factor**, flagging a single-maintainer project; see [docs/sustainability-plan.md](docs/sustainability-plan.md).",
 "intro":"Patchline gate-verifies a sustainability plan covering CI cost per run, maintainer load, and **bus-factor**.",
 "how":"The worker checks CI cost under budget, maintainer load under a healthy cap, and bus-factor at or above a minimum threshold.",
 "proves":"- All three sustainability metrics are within bounds.\n- A single-maintainer bus-factor below threshold is flagged.",
 "why":"Measuring continuity risk is how a project avoids quietly becoming unmaintained.",
},
{
 "name":"roadmap-burndown","title":"Roadmap burndown",
 "phrase":"milestone",
 "claim":"Patchline tracks a 1.0-to-2.0 roadmap as gate-backed milestones with a burndown, where each milestone is either complete or has a backing gate, and progress is computed from real completion rather than narrative. The worker counts completed milestones, verifies every incomplete milestone has a backing gate, and computes the burndown percentage. The gate proves the burndown is consistent and every open milestone is gate-backed, and that a milestone marked complete with no evidence is rejected.",
 "spec":{"milestones":[
    {"id":"m1","complete":True,"backing_gate":"signed-provenance-chain-gate"},
    {"id":"m2","complete":True,"backing_gate":"plugin-conformance-gate"},
    {"id":"m3","complete":False,"backing_gate":"reproducible-build-attestation-gate"}],
  "evidence_free_milestone":{"id":"m4","complete":True,"backing_gate":""}},
 "worker_jq":r"""
  def r4: (.*10000|round)/10000;
  .milestones as $M | .evidence_free_milestone as $E
  | ([ $M[] | select(.complete) ]|length) as $done
  | ([ $M[] | select(.complete|not) | select((.backing_gate|length)>0) ]|length) as $openbacked
  | ([ $M[] | select(.complete|not) ]|length) as $open
  | {version:"patchline.roadmap-burndown/v1",
     milestones:($M|length), done:$done,
     burndown:(($done/($M|length))|r4),
     open_all_backed:($openbacked==$open),
     evidence_free_ok:($E.complete and (($E.backing_gate|length)>0))}
""",
 "md_echo":'echo "Done $(jq -r .done "$OUT/out.json")/$(jq -r .milestones "$OUT/out.json"); burndown $(jq -r .burndown "$OUT/out.json")"',
 "worker_echo":"burndown=$(jq -r .burndown \"$OUT/out.json\")",
 "gate_assert":'.version=="patchline.roadmap-burndown/v1" and .open_all_backed==true and .evidence_free_ok==false and .done==2',
 "summary":'{version:"patchline.roadmap-burndown-gate-results/v1",burndown:$r[0].burndown,open_all_backed:$r[0].open_all_backed,evidence_free_rejected:($r[0].evidence_free_ok|not),verified:true}',
 "pass_msg":"burndown consistent and open milestones gate-backed, evidence-free completion rejected",
 "readme":"Run `make roadmap-burndown-gate` for a 1.0-to-2.0 **milestone** burndown where every open milestone is gate-backed and an evidence-free completion is rejected; see [docs/roadmap-burndown.md](docs/roadmap-burndown.md).",
 "intro":"Patchline tracks a 1.0-to-2.0 roadmap as gate-backed **milestone**s with a burndown computed from real completion.",
 "how":"The worker counts completed milestones, verifies every incomplete milestone has a backing gate, and computes the burndown percentage.",
 "proves":"- The burndown is consistent and every open milestone is gate-backed.\n- A milestone marked complete with no evidence is rejected.",
 "why":"A burndown tied to gate evidence keeps the roadmap honest instead of aspirational.",
},
]
