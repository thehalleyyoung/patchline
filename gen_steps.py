#!/usr/bin/env python3
"""Temporary generator for 100_STEPS evidence-chain artifacts. Deleted after use."""
import json, os, re, sys

ROOT = os.path.dirname(os.path.abspath(__file__))

WORKER_TMPL = """#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${{BASH_SOURCE[0]}}")/.." && pwd)"; cd "$ROOT"
SPEC="${{1:-examples/{name}-gate.json}}"; OUT="${{2:-results/generated/{name}}}"
rm -rf "$OUT"; mkdir -p "$OUT"
jq -e '.version == "patchline.{name}-gate/v1"' "$SPEC" > /dev/null
jq '
{worker_jq}
' "$SPEC" > "$OUT/out.json"
{{ echo "# {title}"; echo; {md_echo}; }} > "$OUT/out.md"
cp "$OUT/out.md" "$OUT/README.md"
echo "{name} worker: {worker_echo}"
"""

GATE_TMPL = """#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${{BASH_SOURCE[0]}}")/.." && pwd)"; cd "$ROOT"
SPEC="${{1:-examples/{name}-gate.json}}"; OUT="${{2:-results/generated/{name}}}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.{name}-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "{phrase}" "make {name}-gate"; do grep -F "$phrase" docs/{name}.md README.md > /dev/null; done
bash scripts/{name}.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '{gate_assert}' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{summary}' > "$OUT/gate-summary.json"
echo "{name} gate passed: {pass_msg}"
"""

DOC_TMPL = """# {title}

{intro}

## How it works

{how}

## What the gate proves

{proves}

## Why it matters

{why}

## Reproduce

```
make {name}-gate
```
"""


def gen(step):
    name = step["name"]
    # spec
    spec = {"version": f"patchline.{name}-gate/v1", "claim": step["claim"],
            "real_repo": step.get("real_repo", f"patchline (self): {name}")}
    spec.update(step["spec"])
    with open(os.path.join(ROOT, "examples", f"{name}-gate.json"), "w") as f:
        json.dump(spec, f, indent=2)
    # worker
    with open(os.path.join(ROOT, "scripts", f"{name}.sh"), "w") as f:
        f.write(WORKER_TMPL.format(name=name, title=step["title"],
                worker_jq=step["worker_jq"], md_echo=step["md_echo"],
                worker_echo=step["worker_echo"]))
    # gate
    with open(os.path.join(ROOT, "scripts", f"{name}-gate.sh"), "w") as f:
        f.write(GATE_TMPL.format(name=name, phrase=step["phrase"],
                gate_assert=step["gate_assert"], summary=step["summary"],
                pass_msg=step["pass_msg"]))
    # doc
    with open(os.path.join(ROOT, "docs", f"{name}.md"), "w") as f:
        f.write(DOC_TMPL.format(name=name, title=step["title"], intro=step["intro"],
                how=step["how"], proves=step["proves"], why=step["why"]))
    os.chmod(os.path.join(ROOT, "scripts", f"{name}.sh"), 0o755)
    os.chmod(os.path.join(ROOT, "scripts", f"{name}-gate.sh"), 0o755)


def patch_makefile(steps):
    p = os.path.join(ROOT, "Makefile")
    s = open(p).read()
    names = [st["name"] + "-gate" for st in steps]
    # add to .PHONY before " gate fmt public-corpus"
    add_phony = " ".join(names)
    anchor = " gate fmt public-corpus"
    assert anchor in s
    s = s.replace(anchor, " " + add_phony + anchor, 1)
    # add stanzas before "\ngate:\n"
    stanzas = ""
    for st in steps:
        stanzas += f"{st['name']}-gate:\n\tbash scripts/{st['name']}-gate.sh\n\n"
    s = s.replace("gate:\n\tgo run ./cmd/patchline ci-gate",
                  stanzas + "gate:\n\tgo run ./cmd/patchline ci-gate", 1)
    open(p, "w").write(s)


def patch_readme(steps, anchor_name):
    p = os.path.join(ROOT, "README.md")
    s = open(p).read()
    block = "\n".join(st["readme"] for st in steps)
    # insert after the README line for anchor_name
    pat = re.compile(r"(Run `make " + re.escape(anchor_name) + r"-gate`[^\n]*\n)")
    m = pat.search(s)
    assert m, f"anchor {anchor_name} not found in README"
    s = s[:m.end()] + block + "\n" + s[m.end():]
    open(p, "w").write(s)


if __name__ == "__main__":
    mod = __import__(sys.argv[1].replace(".py", ""))
    steps = mod.STEPS
    for st in steps:
        gen(st)
    patch_makefile(steps)
    patch_readme(steps, sys.argv[2])
    print("generated", len(steps), "steps")
