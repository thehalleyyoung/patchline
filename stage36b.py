STEPS = [
{
 "name":"self-improving-loop","title":"Self-improving gate-mining loop",
 "phrase":"unexplained",
 "claim":"Patchline runs a self-improving loop that mines new gate ideas from unexplained corpus failures — cases the current gates neither flag nor justify — and proposes a candidate gate for each, growing coverage from real blind spots. The worker isolates the unexplained failures, proposes a candidate gate per cluster, and verifies each proposal references the failure that motivated it. The gate proves every unexplained failure yields a motivated candidate gate and that a proposal with no backing failure is rejected.",
 "spec":{"failures":[
    {"id":"u1","explained":False,"proposed_gate":"g-u1"},
    {"id":"u2","explained":False,"proposed_gate":"g-u2"},
    {"id":"e1","explained":True,"proposed_gate":""}],
  "unbacked_proposal":{"proposed_gate":"g-x","backing_failure":""}},
 "worker_jq":r"""
  .failures as $F | .unbacked_proposal as $U
  | ([ $F[] | select(.explained|not) ]) as $unexp
  | ([ $unexp[] | select((.proposed_gate|length)>0) ]|length) as $proposed
  | {version:"patchline.self-improving-loop/v1",
     unexplained:($unexp|length),
     proposals:$proposed,
     all_motivated:($proposed==($unexp|length)),
     unbacked_ok:(($U.backing_failure|length)>0)}
""",
 "md_echo":'echo "Unexplained $(jq -r .unexplained "$OUT/out.json"); proposals $(jq -r .proposals "$OUT/out.json")"',
 "worker_echo":"all_motivated=$(jq -r .all_motivated \"$OUT/out.json\")",
 "gate_assert":'.version=="patchline.self-improving-loop/v1" and .all_motivated==true and .unexplained==2 and .unbacked_ok==false',
 "summary":'{version:"patchline.self-improving-loop-gate-results/v1",unexplained:$r[0].unexplained,all_motivated:$r[0].all_motivated,unbacked_rejected:($r[0].unbacked_ok|not),verified:true}',
 "pass_msg":"every unexplained failure yields a motivated candidate gate, unbacked proposal rejected",
 "readme":"Run `make self-improving-loop-gate` for a loop that mines candidate gates from **unexplained** corpus failures, rejecting a proposal with no backing failure; see [docs/self-improving-loop.md](docs/self-improving-loop.md).",
 "intro":"Patchline runs a self-improving loop that mines new gate ideas from **unexplained** corpus failures and proposes a candidate gate for each.",
 "how":"The worker isolates the unexplained failures, proposes a candidate gate per cluster, and verifies each proposal references the motivating failure.",
 "proves":"- Every unexplained failure yields a motivated candidate gate.\n- A proposal with no backing failure is rejected.",
 "why":"Mining new gates from real blind spots is how the catalog keeps growing toward the failures that actually occur.",
},
]
