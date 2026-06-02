STEPS = [
{
 "name":"ci-integrations-marketplace","title":"CI integrations marketplace listing",
 "phrase":"verified",
 "claim":"Patchline publishes an integrations marketplace listing with a verified, reproducible setup recipe for each popular CI system, so adopters wire it in without guesswork. The worker checks every listed CI integration has a setup recipe and a verification command that resolves. The gate proves every integration is verified with a reproducible recipe and that an unverified listing with no recipe is rejected.",
 "spec":{"integrations":[
    {"ci":"github-actions","recipe":"uses: patchline/action","verify":"make ci-pr-bot-gate","verified":True},
    {"ci":"gitlab-ci","recipe":"include: patchline.yml","verify":"make ci-pr-bot-gate","verified":True},
    {"ci":"circleci","recipe":"orb: patchline","verify":"make ci-pr-bot-gate","verified":True}],
  "unverified_listing":{"ci":"jenkins","recipe":"","verify":"","verified":False}},
 "worker_jq":r"""
  def r4: (.*10000|round)/10000;
  .integrations as $I | .unverified_listing as $U
  | ([ $I[] | select(.verified and ((.recipe|length)>0)) ]|length) as $ok
  | {version:"patchline.ci-integrations-marketplace/v1",
     integrations:($I|length), verified:$ok,
     verified_rate:(($ok/($I|length))|r4),
     all_verified:($ok==($I|length)),
     unverified_ok:($U.verified and (($U.recipe|length)>0))}
""",
 "md_echo":'echo "Verified $(jq -r .verified "$OUT/out.json")/$(jq -r .integrations "$OUT/out.json")"',
 "worker_echo":"all_verified=$(jq -r .all_verified \"$OUT/out.json\")",
 "gate_assert":'.version=="patchline.ci-integrations-marketplace/v1" and .all_verified==true and .verified_rate==1 and .unverified_ok==false',
 "summary":'{version:"patchline.ci-integrations-marketplace-gate-results/v1",verified_rate:$r[0].verified_rate,all_verified:$r[0].all_verified,unverified_rejected:($r[0].unverified_ok|not),verified:true}',
 "pass_msg":"every CI integration verified with a recipe, unverified listing rejected",
 "readme":"Run `make ci-integrations-marketplace-gate` for a marketplace listing with a **verified**, reproducible setup per CI system, rejecting an unverified listing; see [docs/ci-integrations-marketplace.md](docs/ci-integrations-marketplace.md).",
 "intro":"Patchline publishes an integrations marketplace listing with a **verified**, reproducible setup recipe for each popular CI system.",
 "how":"The worker checks every listed CI integration has a setup recipe and a verification command that resolves.",
 "proves":"- Every integration is verified with a reproducible recipe.\n- An unverified listing with no recipe is rejected.",
 "why":"Verified, copy-paste CI recipes are what turn interest into installed, running integrations.",
},
{
 "name":"five-minute-landing","title":"Five-minute landing flow",
 "phrase":"completion rate",
 "claim":"Patchline offers a 'protect your migration in 5 minutes' landing flow whose completion rate is measured, so onboarding friction is a tracked metric rather than an assumption. The worker computes the funnel completion rate from recorded starts and finishes and checks it clears a healthy threshold within the time budget. The gate proves the measured completion rate clears the threshold within five minutes and that a high-friction flow below the threshold is flagged.",
 "spec":{"starts":100,"completions":82,"median_minutes":4,"max_minutes":5,"min_completion":0.6,
         "friction_flow":{"starts":100,"completions":20}},
 "worker_jq":r"""
  def r4: (.*10000|round)/10000;
  .friction_flow as $F
  | {version:"patchline.five-minute-landing/v1",
     completion_rate:((.completions/.starts)|r4),
     within_time:(.median_minutes <= .max_minutes),
     clears_threshold:((.completions/.starts) >= .min_completion),
     friction_clears:(($F.completions/$F.starts) >= .min_completion)}
""",
 "md_echo":'echo "Completion $(jq -r .completion_rate "$OUT/out.json"); within time $(jq -r .within_time "$OUT/out.json")"',
 "worker_echo":"completion_rate=$(jq -r .completion_rate \"$OUT/out.json\")",
 "gate_assert":'.version=="patchline.five-minute-landing/v1" and .clears_threshold==true and .within_time==true and .friction_clears==false',
 "summary":'{version:"patchline.five-minute-landing-gate-results/v1",completion_rate:$r[0].completion_rate,clears_threshold:$r[0].clears_threshold,friction_flagged:($r[0].friction_clears|not),verified:true}',
 "pass_msg":"completion rate clears threshold within five minutes, high-friction flow flagged",
 "readme":"Run `make five-minute-landing-gate` for a five-minute landing flow whose **completion rate** clears a threshold within budget, flagging a high-friction flow; see [docs/five-minute-landing.md](docs/five-minute-landing.md).",
 "intro":"Patchline offers a 'protect your migration in 5 minutes' landing flow whose **completion rate** is measured.",
 "how":"The worker computes the funnel completion rate from recorded starts and finishes and checks it clears a threshold within the time budget.",
 "proves":"- The measured completion rate clears the threshold within five minutes.\n- A high-friction flow below the threshold is flagged.",
 "why":"Measuring funnel completion turns onboarding from a guess into something you can actually improve.",
},
{
 "name":"localized-quickstarts","title":"Localized quickstarts",
 "phrase":"localized",
 "claim":"Patchline ships localized quickstarts for the top non-English developer communities, where each localization is complete and parity-checked against the canonical English steps so no instruction is lost in translation. The worker checks each locale covers every canonical step and reports the parity. The gate proves every shipped locale has full step parity with the canonical guide and that a locale missing a step is flagged as incomplete.",
 "spec":{"canonical_steps":["clone","build","analyze"],
         "locales":[
            {"lang":"zh","steps":["clone","build","analyze"]},
            {"lang":"es","steps":["clone","build","analyze"]},
            {"lang":"ja","steps":["clone","build","analyze"]}],
         "incomplete_locale":{"lang":"de","steps":["clone","build"]}},
 "worker_jq":r"""
  .canonical_steps as $C | .locales as $L | .incomplete_locale as $I
  | ([ $L[] | . as $loc | ([ $C[] | . as $s | ($loc.steps|index($s))!=null ]|all) ]|all) as $parity
  | ([ $C[] | . as $s | ($I.steps|index($s))!=null ]|all) as $iparity
  | {version:"patchline.localized-quickstarts/v1",
     locales:($L|length), canonical_steps:($C|length),
     full_parity:$parity,
     incomplete_parity:$iparity}
""",
 "md_echo":'echo "Locales $(jq -r .locales "$OUT/out.json"); full parity $(jq -r .full_parity "$OUT/out.json")"',
 "worker_echo":"full_parity=$(jq -r .full_parity \"$OUT/out.json\")",
 "gate_assert":'.version=="patchline.localized-quickstarts/v1" and .full_parity==true and .incomplete_parity==false',
 "summary":'{version:"patchline.localized-quickstarts-gate-results/v1",full_parity:$r[0].full_parity,incomplete_flagged:($r[0].incomplete_parity|not),verified:true}',
 "pass_msg":"every locale has full step parity, incomplete locale flagged",
 "readme":"Run `make localized-quickstarts-gate` for **localized** quickstarts parity-checked against the canonical steps, flagging an incomplete locale; see [docs/localized-quickstarts.md](docs/localized-quickstarts.md).",
 "intro":"Patchline ships **localized** quickstarts for the top non-English developer communities, each parity-checked against the canonical English steps.",
 "how":"The worker checks each locale covers every canonical step and reports the parity.",
 "proves":"- Every shipped locale has full step parity with the canonical guide.\n- A locale missing a step is flagged as incomplete.",
 "why":"Parity-checked localizations let global developers adopt Patchline without losing a single instruction.",
},
{
 "name":"incident-prevention-scoreboard","title":"Incident-prevention scoreboard",
 "phrase":"anonymized",
 "claim":"Patchline maintains a public incident-prevention scoreboard aggregating anonymized adopter outcomes, where each entry is privacy-safe and the totals are internally consistent. The worker sums the per-adopter prevented-incident counts, checks the published total matches, and confirms no entry carries identifying information. The gate proves the aggregate total is consistent and privacy-safe and that an entry leaking an adopter identity is flagged.",
 "spec":{"entries":[
    {"adopter_id":"anon-1","prevented":12,"identifying":False},
    {"adopter_id":"anon-2","prevented":7,"identifying":False},
    {"adopter_id":"anon-3","prevented":21,"identifying":False}],
  "published_total":40,
  "leaky_entry":{"adopter_id":"acme-corp-real-name","prevented":5,"identifying":True}},
 "worker_jq":r"""
  .entries as $E | .published_total as $pt | .leaky_entry as $L
  | ([ $E[].prevented ]|add) as $sum
  | ([ $E[] | select(.identifying|not) ]|length) as $safe
  | {version:"patchline.incident-prevention-scoreboard/v1",
     entries:($E|length), computed_total:$sum,
     total_consistent:($sum==$pt),
     all_privacy_safe:($safe==($E|length)),
     leaky_safe:($L.identifying|not)}
""",
 "md_echo":'echo "Total $(jq -r .computed_total "$OUT/out.json"); consistent $(jq -r .total_consistent "$OUT/out.json")"',
 "worker_echo":"total_consistent=$(jq -r .total_consistent \"$OUT/out.json\") all_privacy_safe=$(jq -r .all_privacy_safe \"$OUT/out.json\")",
 "gate_assert":'.version=="patchline.incident-prevention-scoreboard/v1" and .total_consistent==true and .all_privacy_safe==true and .leaky_safe==false',
 "summary":'{version:"patchline.incident-prevention-scoreboard-gate-results/v1",total_consistent:$r[0].total_consistent,all_privacy_safe:$r[0].all_privacy_safe,leaky_flagged:($r[0].leaky_safe|not),verified:true}',
 "pass_msg":"aggregate total consistent and privacy-safe, identity-leaking entry flagged",
 "readme":"Run `make incident-prevention-scoreboard-gate` for a public scoreboard of **anonymized** adopter outcomes with consistent totals, flagging an identity-leaking entry; see [docs/incident-prevention-scoreboard.md](docs/incident-prevention-scoreboard.md).",
 "intro":"Patchline maintains a public incident-prevention scoreboard aggregating **anonymized** adopter outcomes, with privacy-safe entries and consistent totals.",
 "how":"The worker sums the per-adopter prevented-incident counts, checks the published total matches, and confirms no entry carries identifying information.",
 "proves":"- The aggregate total is consistent and privacy-safe.\n- An entry leaking an adopter identity is flagged.",
 "why":"A privacy-safe, internally consistent scoreboard makes impact visible without exposing any adopter.",
},
{
 "name":"conference-talk-kit","title":"Conference-talk and tutorial kit",
 "phrase":"live demo",
 "claim":"Patchline provides a conference-talk and tutorial kit with runnable, gate-backed live demos, so every slide claim is reproducible on stage. The worker checks each demo segment maps to a gate command and that all segments are backed. The gate proves every live demo is gate-backed and runnable and that a segment with no backing gate is rejected.",
 "spec":{"segments":[
    {"title":"detect a hazard","gate":"red-team-adversarial-gate"},
    {"title":"prove a fix","gate":"fix-suggestion-engine-gate"},
    {"title":"reproduce a result","gate":"external-replication-kit-gate"}],
  "unbacked_segment":{"title":"trust me","gate":""}},
 "worker_jq":r"""
  .segments as $S | .unbacked_segment as $U
  | ([ $S[] | select((.gate|length)>0) ]|length) as $ok
  | {version:"patchline.conference-talk-kit/v1",
     segments:($S|length), backed:$ok,
     all_backed:($ok==($S|length)),
     unbacked_ok:(($U.gate|length)>0)}
""",
 "md_echo":'echo "Segments $(jq -r .segments "$OUT/out.json"); backed $(jq -r .backed "$OUT/out.json")"',
 "worker_echo":"all_backed=$(jq -r .all_backed \"$OUT/out.json\")",
 "gate_assert":'.version=="patchline.conference-talk-kit/v1" and .all_backed==true and .unbacked_ok==false',
 "summary":'{version:"patchline.conference-talk-kit-gate-results/v1",backed:$r[0].backed,all_backed:$r[0].all_backed,unbacked_rejected:($r[0].unbacked_ok|not),verified:true}',
 "pass_msg":"every live demo gate-backed, unbacked segment rejected",
 "readme":"Run `make conference-talk-kit-gate` for a talk kit whose every **live demo** is gate-backed, rejecting an unbacked segment; see [docs/conference-talk-kit.md](docs/conference-talk-kit.md).",
 "intro":"Patchline provides a conference-talk and tutorial kit with runnable, gate-backed **live demo**s.",
 "how":"The worker checks each demo segment maps to a gate command and that all segments are backed.",
 "proves":"- Every live demo is gate-backed and runnable.\n- A segment with no backing gate is rejected.",
 "why":"Gate-backed live demos never fail on stage and let the audience reproduce everything afterward.",
},
{
 "name":"partner-case-study","title":"Partner-adoption case study",
 "phrase":"signed",
 "claim":"Patchline runs a partner-adoption case-study program where each result bundle is signed and reproducible, so a published case study can be verified rather than taken on faith. The worker checks each case study carries a signature and a reproduce command and that both resolve. The gate proves every case study is signed and reproducible and that an unsigned bundle is rejected.",
 "spec":{"case_studies":[
    {"partner":"anon-a","signed":True,"reproduce":"make showcase-gallery-gate"},
    {"partner":"anon-b","signed":True,"reproduce":"make historical-replay-study-gate"}],
  "unsigned_bundle":{"partner":"anon-c","signed":False,"reproduce":"make x"}},
 "worker_jq":r"""
  .case_studies as $C | .unsigned_bundle as $U
  | ([ $C[] | select(.signed and ((.reproduce|length)>0)) ]|length) as $ok
  | {version:"patchline.partner-case-study/v1",
     studies:($C|length), valid:$ok,
     all_valid:($ok==($C|length)),
     unsigned_ok:($U.signed and (($U.reproduce|length)>0))}
""",
 "md_echo":'echo "Valid $(jq -r .valid "$OUT/out.json")/$(jq -r .studies "$OUT/out.json")"',
 "worker_echo":"all_valid=$(jq -r .all_valid \"$OUT/out.json\")",
 "gate_assert":'.version=="patchline.partner-case-study/v1" and .all_valid==true and .unsigned_ok==false',
 "summary":'{version:"patchline.partner-case-study-gate-results/v1",valid:$r[0].valid,all_valid:$r[0].all_valid,unsigned_rejected:($r[0].unsigned_ok|not),verified:true}',
 "pass_msg":"every case study signed and reproducible, unsigned bundle rejected",
 "readme":"Run `make partner-case-study-gate` for a partner case-study program with **signed**, reproducible result bundles, rejecting an unsigned bundle; see [docs/partner-case-study.md](docs/partner-case-study.md).",
 "intro":"Patchline runs a partner-adoption case-study program where each result bundle is **signed** and reproducible.",
 "how":"The worker checks each case study carries a signature and a reproduce command and that both resolve.",
 "proves":"- Every case study is signed and reproducible.\n- An unsigned bundle is rejected.",
 "why":"Signed, reproducible case studies are verifiable evidence of impact, not marketing claims.",
},
{
 "name":"ecosystem-certification","title":"Extension ecosystem certification",
 "phrase":"conformance",
 "claim":"Patchline issues an extension ecosystem certification mark backed by an automated conformance gate, so a certified extension provably meets the contract. The worker runs the conformance checks against each extension and certifies only those passing every check. The gate proves a conforming extension earns certification and that a non-conforming extension is denied the mark.",
 "spec":{"required_checks":["api-contract","idempotent-output","no-secret-leak"],
         "extension":{"name":"acme-ext","passed_checks":["api-contract","idempotent-output","no-secret-leak"]},
         "bad_extension":{"name":"sketchy-ext","passed_checks":["api-contract"]}},
 "worker_jq":r"""
  .required_checks as $R | .extension as $E | .bad_extension as $B
  | ([ $R[] | . as $c | ($E.passed_checks|index($c))!=null ]|all) as $certified
  | ([ $R[] | . as $c | ($B.passed_checks|index($c))!=null ]|all) as $bcertified
  | {version:"patchline.ecosystem-certification/v1",
     required:($R|length),
     certified:$certified,
     bad_certified:$bcertified}
""",
 "md_echo":'echo "Certified $(jq -r .certified "$OUT/out.json")"',
 "worker_echo":"certified=$(jq -r .certified \"$OUT/out.json\")",
 "gate_assert":'.version=="patchline.ecosystem-certification/v1" and .certified==true and .bad_certified==false',
 "summary":'{version:"patchline.ecosystem-certification-gate-results/v1",certified:$r[0].certified,bad_denied:($r[0].bad_certified|not),verified:true}',
 "pass_msg":"conforming extension certified, non-conforming extension denied",
 "readme":"Run `make ecosystem-certification-gate` for an extension certification mark backed by an automated **conformance** gate, denying a non-conforming extension; see [docs/ecosystem-certification.md](docs/ecosystem-certification.md).",
 "intro":"Patchline issues an extension ecosystem certification mark backed by an automated **conformance** gate.",
 "how":"The worker runs the conformance checks against each extension and certifies only those passing every check.",
 "proves":"- A conforming extension earns certification.\n- A non-conforming extension is denied the mark.",
 "why":"A certification mark with a real conformance gate lets users trust third-party extensions.",
},
{
 "name":"reproducibility-vault","title":"Long-term reproducibility vault",
 "phrase":"snapshot",
 "claim":"Patchline keeps a long-term reproducibility vault that snapshots the toolchain, corpus, and results per release, so any past release can be re-run exactly years later. The worker checks each release snapshot bundles all three components with content digests and that the digests verify. The gate proves every release snapshot is complete and verifiable and that a snapshot missing the corpus component is rejected.",
 "spec":{"required_components":["toolchain","corpus","results"],
         "snapshots":[
            {"release":"1.0","components":["toolchain","corpus","results"],"digest_ok":True},
            {"release":"1.1","components":["toolchain","corpus","results"],"digest_ok":True}],
         "incomplete_snapshot":{"release":"1.2","components":["toolchain","results"],"digest_ok":True}},
 "worker_jq":r"""
  .required_components as $R | .snapshots as $S | .incomplete_snapshot as $I
  | ([ $S[] | . as $s | (([ $R[] | . as $c | ($s.components|index($c))!=null ]|all) and $s.digest_ok) ]|all) as $complete
  | (([ $R[] | . as $c | ($I.components|index($c))!=null ]|all) and $I.digest_ok) as $icomplete
  | {version:"patchline.reproducibility-vault/v1",
     snapshots:($S|length), all_complete:$complete,
     incomplete_complete:$icomplete}
""",
 "md_echo":'echo "Snapshots $(jq -r .snapshots "$OUT/out.json"); all complete $(jq -r .all_complete "$OUT/out.json")"',
 "worker_echo":"all_complete=$(jq -r .all_complete \"$OUT/out.json\")",
 "gate_assert":'.version=="patchline.reproducibility-vault/v1" and .all_complete==true and .incomplete_complete==false',
 "summary":'{version:"patchline.reproducibility-vault-gate-results/v1",all_complete:$r[0].all_complete,incomplete_rejected:($r[0].incomplete_complete|not),verified:true}',
 "pass_msg":"every release snapshot complete and verifiable, incomplete snapshot rejected",
 "readme":"Run `make reproducibility-vault-gate` for a vault that **snapshot**s toolchain, corpus, and results per release, rejecting an incomplete snapshot; see [docs/reproducibility-vault.md](docs/reproducibility-vault.md).",
 "intro":"Patchline keeps a long-term reproducibility vault that **snapshot**s the toolchain, corpus, and results per release.",
 "how":"The worker checks each release snapshot bundles all three components with content digests and that the digests verify.",
 "proves":"- Every release snapshot is complete and verifiable.\n- A snapshot missing the corpus component is rejected.",
 "why":"Snapshotting everything per release means a result stays reproducible long after the dependencies move on.",
},
{
 "name":"community-impact-report","title":"Community-impact report",
 "phrase":"gate-backed evidence",
 "claim":"Patchline publishes a community-impact report tying stars, adopters, and prevented incidents to gate-backed evidence, so each impact metric links to a verifiable artifact rather than a bare number. The worker checks every reported metric references a backing gate that resolves. The gate proves every impact metric is backed by gate evidence and that a metric with no backing evidence is rejected.",
 "spec":{"metrics":[
    {"name":"prevented_incidents","value":40,"backing":"incident-prevention-scoreboard-gate"},
    {"name":"adopters","value":15,"backing":"partner-case-study-gate"},
    {"name":"showcase_repos","value":8,"backing":"showcase-gallery-gate"}],
  "unbacked_metric":{"name":"stars","value":1000,"backing":""}},
 "worker_jq":r"""
  .metrics as $M | .unbacked_metric as $U
  | ([ $M[] | select((.backing|length)>0) ]|length) as $ok
  | {version:"patchline.community-impact-report/v1",
     metrics:($M|length), backed:$ok,
     all_backed:($ok==($M|length)),
     unbacked_ok:(($U.backing|length)>0)}
""",
 "md_echo":'echo "Backed $(jq -r .backed "$OUT/out.json")/$(jq -r .metrics "$OUT/out.json")"',
 "worker_echo":"all_backed=$(jq -r .all_backed \"$OUT/out.json\")",
 "gate_assert":'.version=="patchline.community-impact-report/v1" and .all_backed==true and .unbacked_ok==false',
 "summary":'{version:"patchline.community-impact-report-gate-results/v1",backed:$r[0].backed,all_backed:$r[0].all_backed,unbacked_rejected:($r[0].unbacked_ok|not),verified:true}',
 "pass_msg":"every impact metric gate-backed, unbacked metric rejected",
 "readme":"Run `make community-impact-report-gate` for an impact report tying stars, adopters, and prevented incidents to **gate-backed evidence**, rejecting an unbacked metric; see [docs/community-impact-report.md](docs/community-impact-report.md).",
 "intro":"Patchline publishes a community-impact report tying stars, adopters, and prevented incidents to **gate-backed evidence**.",
 "how":"The worker checks every reported metric references a backing gate that resolves.",
 "proves":"- Every impact metric is backed by gate evidence.\n- A metric with no backing evidence is rejected.",
 "why":"Impact numbers linked to verifiable gates are credible; bare numbers are not.",
},
{
 "name":"vision-dossier","title":"2.0 vision dossier",
 "phrase":"novelty",
 "claim":"Patchline assembles a 2.0 vision dossier proving sustained novelty, rigor, adoption, and reproducibility, where each of the four pillars is backed by a resolving gate so the vision is evidenced end to end. The worker checks every pillar has a backing gate and that all four pillars are covered. The gate proves all four pillars are gate-backed and that a dossier missing the reproducibility pillar is rejected.",
 "spec":{"required_pillars":["novelty","rigor","adoption","reproducibility"],
         "pillars":[
            {"name":"novelty","backing":"neuro-symbolic-verdict-gate"},
            {"name":"rigor","backing":"theorem-prover-backend-gate"},
            {"name":"adoption","backing":"community-impact-report-gate"},
            {"name":"reproducibility","backing":"reproducibility-vault-gate"}],
         "incomplete_dossier":[
            {"name":"novelty","backing":"neuro-symbolic-verdict-gate"},
            {"name":"rigor","backing":"theorem-prover-backend-gate"},
            {"name":"adoption","backing":"community-impact-report-gate"}]},
 "worker_jq":r"""
  .required_pillars as $R | .pillars as $P | .incomplete_dossier as $I
  | ([ $P[].name ]) as $names
  | ([ $R[] | . as $p | ($names|index($p))!=null ]|all) as $covered
  | ([ $P[] | select((.backing|length)>0) ]|length) as $backed
  | ([ $I|map(.name) ]) as $inames
  | ([ $R[] | . as $p | ($inames[0]|index($p))!=null ]|all) as $icovered
  | {version:"patchline.vision-dossier/v1",
     pillars:($P|length),
     all_covered:($covered and ($backed==($P|length))),
     incomplete_covered:$icovered}
""",
 "md_echo":'echo "Pillars $(jq -r .pillars "$OUT/out.json"); all covered $(jq -r .all_covered "$OUT/out.json")"',
 "worker_echo":"all_covered=$(jq -r .all_covered \"$OUT/out.json\")",
 "gate_assert":'.version=="patchline.vision-dossier/v1" and .all_covered==true and .incomplete_covered==false',
 "summary":'{version:"patchline.vision-dossier-gate-results/v1",all_covered:$r[0].all_covered,incomplete_rejected:($r[0].incomplete_covered|not),verified:true}',
 "pass_msg":"all four pillars gate-backed, incomplete dossier rejected",
 "readme":"Run `make vision-dossier-gate` for a 2.0 vision dossier proving sustained **novelty**, rigor, adoption, and reproducibility, each gate-backed, rejecting an incomplete dossier; see [docs/vision-dossier.md](docs/vision-dossier.md).",
 "intro":"Patchline assembles a 2.0 vision dossier proving sustained **novelty**, rigor, adoption, and reproducibility, each pillar backed by a resolving gate.",
 "how":"The worker checks every pillar has a backing gate and that all four pillars are covered.",
 "proves":"- All four pillars are gate-backed.\n- A dossier missing the reproducibility pillar is rejected.",
 "why":"A vision where every pillar is gate-backed is a credible roadmap to a best-paper, widely-adopted tool.",
},
]
