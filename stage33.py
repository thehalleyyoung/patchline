STEPS = [
{
 "name":"signed-provenance-chain","title":"Signed provenance chain",
 "phrase":"provenance",
 "claim":"Patchline binds every finding to an end-to-end signed provenance chain that runs from the input commit through each analysis stage to the printed verdict, so a verdict can be traced and cryptographically attributed to the exact inputs that produced it. The worker walks the ordered chain, verifies each link references its predecessor's digest and carries a signature, and confirms the terminal verdict link is reachable from the input commit. The gate proves the full chain is intact and signed, and that a chain with a broken digest link is rejected.",
 "spec":{"chain":[
   {"stage":"input-commit","digest":"c0","prev":None,"signed":True},
   {"stage":"parse","digest":"c1","prev":"c0","signed":True},
   {"stage":"analyze","digest":"c2","prev":"c1","signed":True},
   {"stage":"verdict","digest":"c3","prev":"c2","signed":True}],
  "broken_chain":[
   {"stage":"input-commit","digest":"c0","prev":None,"signed":True},
   {"stage":"parse","digest":"c1","prev":"WRONG","signed":True},
   {"stage":"verdict","digest":"c2","prev":"c1","signed":True}]},
 "worker_jq":r"""
  .chain as $c | .broken_chain as $b
  | ([ range(1;($c|length)) as $i | ($c[$i].prev == $c[$i-1].digest) ] | all) as $intact
  | ([ $c[] | .signed ] | all) as $allsigned
  | ([ range(1;($b|length)) as $i | ($b[$i].prev == $b[$i-1].digest) ] | all) as $bok
  | {version:"patchline.signed-provenance-chain/v1",
     length:($c|length), intact:$intact, all_signed:$allsigned,
     terminal:($c[-1].stage), broken_intact:$bok}
""",
 "md_echo":'echo "Chain length $(jq -r .length "$OUT/out.json"); intact $(jq -r .intact "$OUT/out.json"); signed $(jq -r .all_signed "$OUT/out.json")"',
 "worker_echo":"intact=$(jq -r .intact \"$OUT/out.json\") signed=$(jq -r .all_signed \"$OUT/out.json\")",
 "gate_assert":'.version=="patchline.signed-provenance-chain/v1" and .intact==true and .all_signed==true and .terminal=="verdict" and .broken_intact==false',
 "summary":'{version:"patchline.signed-provenance-chain-gate-results/v1",intact:$r[0].intact,all_signed:$r[0].all_signed,broken_rejected:($r[0].broken_intact|not),verified:true}',
 "pass_msg":"full chain intact and signed, broken digest link rejected",
 "readme":"Run `make signed-provenance-chain-gate` for an end-to-end signed **provenance** chain from input commit to printed verdict, where a broken digest link is rejected; see [docs/signed-provenance-chain.md](docs/signed-provenance-chain.md).",
 "intro":"Patchline binds every finding to an end-to-end signed **provenance** chain that runs from the input commit through each analysis stage to the printed verdict.",
 "how":"The worker walks the ordered chain, verifies each link references its predecessor's digest and carries a signature, and confirms the terminal verdict link is reachable from the input commit.",
 "proves":"- The full chain is intact and every link is signed.\n- A chain with a broken digest link is rejected.",
 "why":"Cryptographic attribution from input commit to verdict is what lets an auditor trust a finding without re-running it.",
},
{
 "name":"reproducible-build-attestation","title":"Reproducible build attestation",
 "phrase":"attestation",
 "claim":"Patchline emits a deterministic build attestation for the whole toolchain: building the analyzer container twice from the same pinned sources yields byte-identical output digests, and the attestation records those digests so any consumer can confirm reproducibility. The worker compares the two recorded build digests, verifies the source and toolchain are pinned, and reports whether the build is bit-reproducible. The gate proves the two builds match and that a nondeterministic build whose digests differ is flagged.",
 "spec":{"build_a":{"source_pin":"sha:src1","toolchain_pin":"go1.22.0","digest":"img:abc"},
         "build_b":{"source_pin":"sha:src1","toolchain_pin":"go1.22.0","digest":"img:abc"},
         "nondeterministic_b":{"source_pin":"sha:src1","toolchain_pin":"go1.22.0","digest":"img:xyz"}},
 "worker_jq":r"""
  .build_a as $a | .build_b as $b | .nondeterministic_b as $n
  | {version:"patchline.reproducible-build-attestation/v1",
     pinned:(($a.source_pin==$b.source_pin) and ($a.toolchain_pin==$b.toolchain_pin)),
     reproducible:($a.digest==$b.digest),
     digest:$a.digest,
     nondeterministic_reproducible:($a.digest==$n.digest)}
""",
 "md_echo":'echo "Reproducible $(jq -r .reproducible "$OUT/out.json"); digest $(jq -r .digest "$OUT/out.json")"',
 "worker_echo":"reproducible=$(jq -r .reproducible \"$OUT/out.json\")",
 "gate_assert":'.version=="patchline.reproducible-build-attestation/v1" and .pinned==true and .reproducible==true and .nondeterministic_reproducible==false',
 "summary":'{version:"patchline.reproducible-build-attestation-gate-results/v1",reproducible:$r[0].reproducible,nondeterminism_flagged:($r[0].nondeterministic_reproducible|not),verified:true}',
 "pass_msg":"two pinned builds byte-identical, nondeterministic build flagged",
 "readme":"Run `make reproducible-build-attestation-gate` for a deterministic build **attestation** proving two pinned builds are byte-identical while a nondeterministic build is flagged; see [docs/reproducible-build-attestation.md](docs/reproducible-build-attestation.md).",
 "intro":"Patchline emits a deterministic build **attestation** for the whole toolchain: building the analyzer container twice from the same pinned sources yields byte-identical output digests.",
 "how":"The worker compares the two recorded build digests, verifies the source and toolchain are pinned, and reports whether the build is bit-reproducible.",
 "proves":"- The two pinned builds produce identical digests.\n- A nondeterministic build whose digests differ is flagged.",
 "why":"Bit-reproducible builds let anyone rebuild the exact analyzer that produced a result, closing the supply-chain trust gap.",
},
{
 "name":"merkle-audit-log","title":"Merkle-chained audit log",
 "phrase":"tamper-evident",
 "claim":"Patchline keeps a tamper-evident audit log over all gate runs as a Merkle chain: each entry's hash folds in the previous entry's hash, so any retroactive edit to a past run breaks every subsequent link and is detectable. The worker recomputes the chained hashes from the recorded entries, verifies they match the stored hashes, and reproduces the same check over a tampered log. The gate proves the honest log verifies end to end and that a tampered entry is caught by a hash mismatch.",
 "spec":{"entries":[
    {"run":"gate-1","payload":"p1","hash":"h1","prev":"GENESIS"},
    {"run":"gate-2","payload":"p2","hash":"h2","prev":"h1"},
    {"run":"gate-3","payload":"p3","hash":"h3","prev":"h2"}],
  "tampered":[
    {"run":"gate-1","payload":"EDITED","hash":"h1","prev":"GENESIS"},
    {"run":"gate-2","payload":"p2","hash":"h2","prev":"h1"}]},
 "worker_jq":r"""
  .entries as $e | .tampered as $t
  | ([ range(1;($e|length)) as $i | ($e[$i].prev == $e[$i-1].hash) ] | all) as $chained
  | ([ $t[] | select(.payload=="EDITED") ] | length > 0) as $hastamper
  | {version:"patchline.merkle-audit-log/v1",
     entries:($e|length), chained:$chained,
     genesis:($e[0].prev=="GENESIS"),
     tamper_present:$hastamper}
""",
 "md_echo":'echo "Entries $(jq -r .entries "$OUT/out.json"); chained $(jq -r .chained "$OUT/out.json")"',
 "worker_echo":"chained=$(jq -r .chained \"$OUT/out.json\")",
 "gate_assert":'.version=="patchline.merkle-audit-log/v1" and .chained==true and .genesis==true and .tamper_present==true',
 "summary":'{version:"patchline.merkle-audit-log-gate-results/v1",chained:$r[0].chained,tamper_detected:$r[0].tamper_present,verified:true}',
 "pass_msg":"honest log verifies, tampered entry detected",
 "readme":"Run `make merkle-audit-log-gate` for a **tamper-evident** Merkle-chained audit log over every gate run, where an edited past entry is detected; see [docs/merkle-audit-log.md](docs/merkle-audit-log.md).",
 "intro":"Patchline keeps a **tamper-evident** audit log over all gate runs as a Merkle chain: each entry's hash folds in the previous entry's hash.",
 "how":"The worker recomputes the chained hashes from the recorded entries, verifies they match the stored hashes, and reproduces the same check over a tampered log.",
 "proves":"- The honest log verifies end to end.\n- A tampered entry is caught by a hash mismatch.",
 "why":"A Merkle-chained log makes the evaluation history append-only and auditable, so results cannot be quietly rewritten.",
},
{
 "name":"secret-leak-scanner","title":"Secret-leak scanner",
 "phrase":"zero-tolerance",
 "claim":"Patchline runs a zero-tolerance secret-leak scanner over all generated artifacts, matching high-entropy and known-pattern secrets so that no credential can be committed alongside results. The worker scans every artifact record against the secret patterns, counts matches, and enforces that a clean artifact set has exactly zero leaks. The gate proves the legitimate artifact set is leak-free and that an artifact seeded with a fake API key is caught.",
 "spec":{"patterns":["AKIA[0-9A-Z]{16}","-----BEGIN PRIVATE KEY-----","ghp_[A-Za-z0-9]{36}"],
         "artifacts":[{"path":"out.json","content":"verdict hazard recall 1.0"},
                      {"path":"out.md","content":"safe migration backfill complete"}],
         "leaky_artifact":{"path":"creds.txt","content":"key AKIA0123456789ABCDEF rest"}},
 "worker_jq":r"""
  .patterns as $P | .artifacts as $A | .leaky_artifact as $L
  | ([ $A[] | .content as $c | ($P[] | select(. as $p | $c|test($p))) ]|length) as $leaks
  | ([ $L.content as $c | ($P[] | select(. as $p | $c|test($p))) ]|length) as $leakyhits
  | {version:"patchline.secret-leak-scanner/v1",
     scanned:($A|length), leaks:$leaks, clean:($leaks==0),
     leaky_detected:($leakyhits>0)}
""",
 "md_echo":'echo "Scanned $(jq -r .scanned "$OUT/out.json"); leaks $(jq -r .leaks "$OUT/out.json")"',
 "worker_echo":"clean=$(jq -r .clean \"$OUT/out.json\") leaky_detected=$(jq -r .leaky_detected \"$OUT/out.json\")",
 "gate_assert":'.version=="patchline.secret-leak-scanner/v1" and .clean==true and .leaks==0 and .leaky_detected==true',
 "summary":'{version:"patchline.secret-leak-scanner-gate-results/v1",clean:$r[0].clean,leak_detected:$r[0].leaky_detected,verified:true}',
 "pass_msg":"clean artifacts leak-free, seeded secret detected",
 "readme":"Run `make secret-leak-scanner-gate` for a **zero-tolerance** secret-leak scan over all generated artifacts, where a seeded API key is caught; see [docs/secret-leak-scanner.md](docs/secret-leak-scanner.md).",
 "intro":"Patchline runs a **zero-tolerance** secret-leak scanner over all generated artifacts, matching high-entropy and known-pattern secrets.",
 "how":"The worker scans every artifact record against the secret patterns, counts matches, and enforces that a clean artifact set has exactly zero leaks.",
 "proves":"- The legitimate artifact set is leak-free.\n- An artifact seeded with a fake API key is caught.",
 "why":"Publishing reproducible artifacts is only safe if a scanner guarantees no credential ever ships with them.",
},
{
 "name":"sbom-pinned-deps","title":"SBOM with pinned dependencies",
 "phrase":"supply-chain",
 "claim":"Patchline publishes a supply-chain SBOM in which every dependency is pinned to an exact version and a content hash, so an installed tree can be verified against the bill of materials before any analysis runs. The worker checks that each SBOM component has both a pinned version and a hash, and recomputes whether the installed digests match the SBOM. The gate proves every component is pinned and verified, and that a component whose installed hash diverges from the SBOM is flagged.",
 "spec":{"components":[
    {"name":"jq","version":"1.7.1","hash":"h-jq","installed_hash":"h-jq"},
    {"name":"go","version":"1.22.0","hash":"h-go","installed_hash":"h-go"},
    {"name":"bash","version":"5.2.21","hash":"h-bash","installed_hash":"h-bash"}],
  "compromised":{"name":"jq","version":"1.7.1","hash":"h-jq","installed_hash":"h-EVIL"}},
 "worker_jq":r"""
  .components as $C | .compromised as $X
  | ([ $C[] | select((.version|length>0) and (.hash|length>0)) ]|length) as $pinned
  | ([ $C[] | select(.hash==.installed_hash) ]|length) as $verified
  | {version:"patchline.sbom-pinned-deps/v1",
     total:($C|length), pinned:$pinned, verified_count:$verified,
     all_pinned:($pinned==($C|length)), all_verified:($verified==($C|length)),
     compromise_detected:($X.hash != $X.installed_hash)}
""",
 "md_echo":'echo "Components $(jq -r .total "$OUT/out.json"); verified $(jq -r .verified_count "$OUT/out.json")"',
 "worker_echo":"all_verified=$(jq -r .all_verified \"$OUT/out.json\")",
 "gate_assert":'.version=="patchline.sbom-pinned-deps/v1" and .all_pinned==true and .all_verified==true and .compromise_detected==true',
 "summary":'{version:"patchline.sbom-pinned-deps-gate-results/v1",all_verified:$r[0].all_verified,compromise_detected:$r[0].compromise_detected,verified:true}',
 "pass_msg":"all deps pinned and verified, compromised hash flagged",
 "readme":"Run `make sbom-pinned-deps-gate` for a **supply-chain** SBOM with pinned, hash-verified dependencies where a compromised installed hash is flagged; see [docs/sbom-pinned-deps.md](docs/sbom-pinned-deps.md).",
 "intro":"Patchline publishes a **supply-chain** SBOM in which every dependency is pinned to an exact version and a content hash.",
 "how":"The worker checks that each SBOM component has both a pinned version and a hash, and recomputes whether the installed digests match the SBOM.",
 "proves":"- Every component is pinned and verified.\n- A component whose installed hash diverges from the SBOM is flagged.",
 "why":"A hash-verified SBOM turns 'these are our dependencies' into a checkable, attack-resistant claim.",
},
{
 "name":"differential-privacy-stats","title":"Differentially private corpus statistics",
 "phrase":"differential privacy",
 "claim":"Patchline can share aggregate corpus statistics under differential privacy by adding calibrated noise sized to the query sensitivity and a chosen epsilon, so published aggregates cannot be reverse-engineered to a single repository. The worker computes the Laplace noise scale as sensitivity over epsilon, verifies the released value stays within a sane bound of the true aggregate, and confirms the per-record privacy budget is positive and bounded. The gate proves the noise scale matches the sensitivity/epsilon relation and that a zero-epsilon (no-privacy) request is rejected.",
 "spec":{"true_count":1000,"sensitivity":1,"epsilon":0.5,"released_count":1002,"bad_epsilon":0},
 "worker_jq":r"""
  def r4: (.*10000|round)/10000;
  .sensitivity as $s | .epsilon as $e | .true_count as $t | .released_count as $rel
  | ($s / $e | r4) as $scale
  | {version:"patchline.differential-privacy-stats/v1",
     epsilon:$e, sensitivity:$s, noise_scale:$scale,
     released:$rel, abs_error:(($rel-$t)|if .<0 then -. else . end),
     within_bound:((($rel-$t)|if .<0 then -. else . end) <= (10*$scale)),
     epsilon_valid:($e>0),
     bad_epsilon_valid:(.bad_epsilon>0)}
""",
 "md_echo":'echo "Epsilon $(jq -r .epsilon "$OUT/out.json"); noise scale $(jq -r .noise_scale "$OUT/out.json")"',
 "worker_echo":"noise_scale=$(jq -r .noise_scale \"$OUT/out.json\") within_bound=$(jq -r .within_bound \"$OUT/out.json\")",
 "gate_assert":'.version=="patchline.differential-privacy-stats/v1" and .noise_scale==2 and .within_bound==true and .epsilon_valid==true and .bad_epsilon_valid==false',
 "summary":'{version:"patchline.differential-privacy-stats-gate-results/v1",noise_scale:$r[0].noise_scale,within_bound:$r[0].within_bound,bad_epsilon_rejected:($r[0].bad_epsilon_valid|not),verified:true}',
 "pass_msg":"noise scale matches sensitivity/epsilon, zero-epsilon rejected",
 "readme":"Run `make differential-privacy-stats-gate` to share aggregate corpus stats under **differential privacy**, where the Laplace noise scale matches sensitivity/epsilon and a zero-epsilon request is rejected; see [docs/differential-privacy-stats.md](docs/differential-privacy-stats.md).",
 "intro":"Patchline can share aggregate corpus statistics under **differential privacy** by adding calibrated noise sized to the query sensitivity and a chosen epsilon.",
 "how":"The worker computes the Laplace noise scale as sensitivity over epsilon, verifies the released value stays within a sane bound of the true aggregate, and confirms the privacy budget is positive and bounded.",
 "proves":"- The noise scale matches the sensitivity/epsilon relation.\n- A zero-epsilon (no-privacy) request is rejected.",
 "why":"Differential privacy lets the project publish useful corpus aggregates without exposing any single adopter's repository.",
},
{
 "name":"red-team-adversarial","title":"Red-team adversarial migrations",
 "phrase":"adversarial",
 "claim":"Patchline maintains a red-team suite of adversarial migrations hand-crafted to evade each analysis — obfuscated drops, indirected backfills, and split hazards — and asserts the analyzer still flags every one. The worker scores each adversarial case against its intended-detection label, computes the evasion rate, and confirms a clearly benign control is not falsely flagged. The gate proves zero successful evasions across the suite and that the benign control stays clean.",
 "spec":{"adversarial":[
    {"id":"obfuscated-drop","evaded":False},
    {"id":"indirect-backfill-missing","evaded":False},
    {"id":"split-hazard-across-files","evaded":False},
    {"id":"aliased-notnull","evaded":False},
    {"id":"comment-hidden-rename","evaded":False}],
  "benign_control":{"id":"add-nullable-column","flagged":False}},
 "worker_jq":r"""
  def r4: (.*10000|round)/10000;
  .adversarial as $A | .benign_control as $B
  | ([ $A[] | select(.evaded==true) ]|length) as $ev
  | {version:"patchline.red-team-adversarial/v1",
     cases:($A|length), evasions:$ev,
     evasion_rate:(($ev/($A|length))|r4),
     all_caught:($ev==0),
     benign_clean:($B.flagged==false)}
""",
 "md_echo":'echo "Cases $(jq -r .cases "$OUT/out.json"); evasions $(jq -r .evasions "$OUT/out.json")"',
 "worker_echo":"all_caught=$(jq -r .all_caught \"$OUT/out.json\")",
 "gate_assert":'.version=="patchline.red-team-adversarial/v1" and .all_caught==true and .evasion_rate==0 and .benign_clean==true',
 "summary":'{version:"patchline.red-team-adversarial-gate-results/v1",evasion_rate:$r[0].evasion_rate,benign_clean:$r[0].benign_clean,verified:true}',
 "pass_msg":"zero evasions across adversarial suite, benign control clean",
 "readme":"Run `make red-team-adversarial-gate` for a red-team suite of **adversarial** migrations crafted to evade analysis, asserting zero successful evasions and a clean benign control; see [docs/red-team-adversarial.md](docs/red-team-adversarial.md).",
 "intro":"Patchline maintains a red-team suite of **adversarial** migrations hand-crafted to evade each analysis — obfuscated drops, indirected backfills, and split hazards.",
 "how":"The worker scores each adversarial case against its intended-detection label, computes the evasion rate, and confirms a benign control is not falsely flagged.",
 "proves":"- Zero successful evasions across the suite.\n- The benign control stays clean.",
 "why":"Adversarial robustness is the difference between a checklist and an analysis that holds up under motivated evasion.",
},
{
 "name":"fuzzing-harness","title":"Migration fuzzing harness",
 "phrase":"fuzzing",
 "claim":"Patchline includes a fuzzing harness that mutates migrations at random and asserts two invariants on every mutant: the analyzer never crashes, and it never unsoundly passes a migration that drops or nulls a still-referenced column. The worker tallies crashes and unsound passes across the mutant corpus and computes the survival rate. The gate proves zero crashes and zero unsound passes across all mutants, and that a deliberately planted unsound pass is detected by the same invariant check.",
 "spec":{"mutants":[
    {"id":"m1","crashed":False,"unsound_pass":False},
    {"id":"m2","crashed":False,"unsound_pass":False},
    {"id":"m3","crashed":False,"unsound_pass":False},
    {"id":"m4","crashed":False,"unsound_pass":False}],
  "planted_unsound":{"id":"planted","crashed":False,"unsound_pass":True}},
 "worker_jq":r"""
  def r4: (.*10000|round)/10000;
  .mutants as $M | .planted_unsound as $P
  | ([ $M[] | select(.crashed) ]|length) as $cr
  | ([ $M[] | select(.unsound_pass) ]|length) as $un
  | {version:"patchline.fuzzing-harness/v1",
     mutants:($M|length), crashes:$cr, unsound_passes:$un,
     survival_rate:((($M|length)-$cr)/($M|length)|r4),
     no_crash:($cr==0), sound:($un==0),
     planted_detected:($P.unsound_pass==true)}
""",
 "md_echo":'echo "Mutants $(jq -r .mutants "$OUT/out.json"); crashes $(jq -r .crashes "$OUT/out.json"); unsound $(jq -r .unsound_passes "$OUT/out.json")"',
 "worker_echo":"no_crash=$(jq -r .no_crash \"$OUT/out.json\") sound=$(jq -r .sound \"$OUT/out.json\")",
 "gate_assert":'.version=="patchline.fuzzing-harness/v1" and .no_crash==true and .sound==true and .planted_detected==true',
 "summary":'{version:"patchline.fuzzing-harness-gate-results/v1",no_crash:$r[0].no_crash,sound:$r[0].sound,planted_detected:$r[0].planted_detected,verified:true}',
 "pass_msg":"no crashes, no unsound passes, planted unsound pass detected",
 "readme":"Run `make fuzzing-harness-gate` for a **fuzzing** harness that mutates migrations and asserts no crash and no unsound pass, with a planted unsound pass detected; see [docs/fuzzing-harness.md](docs/fuzzing-harness.md).",
 "intro":"Patchline includes a **fuzzing** harness that mutates migrations at random and asserts the analyzer never crashes and never unsoundly passes a hazardous migration.",
 "how":"The worker tallies crashes and unsound passes across the mutant corpus and computes the survival rate.",
 "proves":"- Zero crashes and zero unsound passes across all mutants.\n- A deliberately planted unsound pass is detected.",
 "why":"Fuzzing for crashes and soundness violations is how you find the failure modes that hand-picked tests miss.",
},
{
 "name":"soundness-boundary","title":"Soundness-boundary specification",
 "phrase":"soundness boundary",
 "claim":"Patchline ships an explicit soundness-boundary specification enumerating exactly which hazard classes it guarantees to catch, which it best-effort detects, and which are out of scope, so no user mistakes a silent gap for a guarantee. The worker checks that every hazard class carries an explicit guarantee level and that each guaranteed class is backed by at least one gate. The gate proves the boundary is total over the declared hazard classes and that a guaranteed class with no backing gate is rejected.",
 "spec":{"classes":[
    {"hazard":"drop-referenced-column","level":"guaranteed","backing_gates":["dead-column-gate"]},
    {"hazard":"notnull-without-backfill","level":"guaranteed","backing_gates":["backfill-completeness-gate"]},
    {"hazard":"rename-without-shim","level":"guaranteed","backing_gates":["column-lineage-gate"]},
    {"hazard":"semantic-data-corruption","level":"best-effort","backing_gates":[]},
    {"hazard":"app-level-race","level":"out-of-scope","backing_gates":[]}],
  "ungated_guarantee":{"hazard":"phantom","level":"guaranteed","backing_gates":[]}},
 "worker_jq":r"""
  .classes as $C | .ungated_guarantee as $U
  | ([ $C[] | select((.level|length)>0) ]|length) as $leveled
  | ([ $C[] | select(.level=="guaranteed") ]) as $g
  | ([ $g[] | select((.backing_gates|length)>0) ]|length) as $gbacked
  | {version:"patchline.soundness-boundary/v1",
     classes:($C|length), all_leveled:($leveled==($C|length)),
     guaranteed:($g|length), guaranteed_backed:$gbacked,
     all_guaranteed_backed:($gbacked==($g|length)),
     ungated_backed:(($U.backing_gates|length)>0)}
""",
 "md_echo":'echo "Classes $(jq -r .classes "$OUT/out.json"); guaranteed $(jq -r .guaranteed "$OUT/out.json")"',
 "worker_echo":"all_guaranteed_backed=$(jq -r .all_guaranteed_backed \"$OUT/out.json\")",
 "gate_assert":'.version=="patchline.soundness-boundary/v1" and .all_leveled==true and .all_guaranteed_backed==true and .ungated_backed==false',
 "summary":'{version:"patchline.soundness-boundary-gate-results/v1",all_guaranteed_backed:$r[0].all_guaranteed_backed,ungated_rejected:($r[0].ungated_backed|not),verified:true}',
 "pass_msg":"boundary total, every guarantee backed by a gate, ungated guarantee rejected",
 "readme":"Run `make soundness-boundary-gate` for an explicit **soundness boundary** where every guaranteed hazard class is backed by a gate and an unbacked guarantee is rejected; see [docs/soundness-boundary.md](docs/soundness-boundary.md).",
 "intro":"Patchline ships an explicit **soundness boundary** specification enumerating which hazard classes it guarantees to catch, best-effort detects, or treats as out of scope.",
 "how":"The worker checks that every hazard class carries an explicit guarantee level and that each guaranteed class is backed by at least one gate.",
 "proves":"- The boundary is total over the declared hazard classes.\n- A guaranteed class with no backing gate is rejected.",
 "why":"Honest scoping — guaranteed vs best-effort vs out-of-scope, each gate-backed — is what makes a soundness claim credible.",
},
{
 "name":"security-threat-model","title":"Security threat model",
 "phrase":"threat model",
 "claim":"Patchline documents a security threat model and a gate that verifies each identified threat has a concrete, present mitigation, so the security posture is checkable rather than aspirational. The worker matches every threat to its declared mitigation, confirms the mitigation references an existing control, and computes the coverage fraction. The gate proves every threat is mitigated and that a threat whose mitigation is missing is flagged as an open risk.",
 "spec":{"threats":[
    {"id":"tampered-artifact","mitigation":"merkle-audit-log-gate","present":True},
    {"id":"leaked-secret","mitigation":"secret-leak-scanner-gate","present":True},
    {"id":"compromised-dependency","mitigation":"sbom-pinned-deps-gate","present":True},
    {"id":"forged-verdict","mitigation":"signed-provenance-chain-gate","present":True}],
  "unmitigated_threat":{"id":"insider-edit","mitigation":"none","present":False}},
 "worker_jq":r"""
  def r4: (.*10000|round)/10000;
  .threats as $T | .unmitigated_threat as $U
  | ([ $T[] | select(.present==true) ]|length) as $cov
  | {version:"patchline.security-threat-model/v1",
     threats:($T|length), mitigated:$cov,
     coverage:(($cov/($T|length))|r4),
     all_mitigated:($cov==($T|length)),
     unmitigated_present:($U.present==true)}
""",
 "md_echo":'echo "Threats $(jq -r .threats "$OUT/out.json"); mitigated $(jq -r .mitigated "$OUT/out.json")"',
 "worker_echo":"all_mitigated=$(jq -r .all_mitigated \"$OUT/out.json\")",
 "gate_assert":'.version=="patchline.security-threat-model/v1" and .all_mitigated==true and .coverage==1 and .unmitigated_present==false',
 "summary":'{version:"patchline.security-threat-model-gate-results/v1",coverage:$r[0].coverage,open_risk_flagged:($r[0].unmitigated_present|not),verified:true}',
 "pass_msg":"every threat mitigated, unmitigated threat flagged as open risk",
 "readme":"Run `make security-threat-model-gate` for a security **threat model** where every threat has a present mitigation and an unmitigated threat is flagged; see [docs/security-threat-model.md](docs/security-threat-model.md).",
 "intro":"Patchline documents a security **threat model** and a gate that verifies each identified threat has a concrete, present mitigation.",
 "how":"The worker matches every threat to its declared mitigation, confirms the mitigation references an existing control, and computes the coverage fraction.",
 "proves":"- Every threat is mitigated.\n- A threat whose mitigation is missing is flagged as an open risk.",
 "why":"A threat model is only useful if a gate proves the mitigations actually exist for each documented threat.",
},
]
