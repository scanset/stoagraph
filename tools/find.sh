#!/usr/bin/env bash
# tools/find.sh <words…> — find the code that does a thing.
#
#   tools/find.sh escalation approval        -> where a held action is released
#   tools/find.sh untrusted context read     -> the READ channel
#   tools/find.sh session recipe bind        -> where a session is bound to a policy
#
# It searches the KEYWORDS a human wrote (`// file-kw:` / `// kw:`), not the identifiers a compiler
# sees. That is the difference between "where is the word `approve`" (60 hits) and "where is the code
# that decides an escalation" (one file, one function, one line number).
#
# For a contributor — human or agent — this is the first command to run, before grep and before
# reading anything. Then: make the change, run tools/check.sh, re-run tools/index.sh if you added a file.
set -euo pipefail
cd "$(dirname "$0")/.." || exit 1

[ $# -eq 0 ] && { echo "usage: tools/find.sh <words…>   e.g. tools/find.sh escalation approval"; exit 2; }
[ -f .index/code.json ] || { echo "no index — run tools/index.sh"; exit 1; }

python3 - "$@" <<'PY'
import json, sys, pathlib

terms = [t.lower() for t in sys.argv[1:]]
idx = json.loads(pathlib.Path(".index/code.json").read_text())

def score(hay: str) -> int:
    hay = hay.lower()
    n = 0
    for t in terms:
        if t in hay:
            n += 2
            # a whole-word hit beats a substring one ("read" should not rank on "already")
            if t in hay.split() or any(t == w.strip(".,;:") for w in hay.split()):
                n += 3
    return n

hits = []
for f in idx["files"]:
    base = " ".join(f["keywords"]) + " " + f["doc"] + " " + f["path"] + " " + f["package"]
    s = score(base)
    for sym in f["symbols"]:
        ss = score(" ".join(sym["kw"]) + " " + sym["name"])
        if ss:
            hits.append((ss + s // 2, f, sym))
    if s:
        hits.append((s, f, None))

if not hits:
    print("no match. try fewer/looser words, or: grep -rn '<term>' stoa-kernel/")
    sys.exit(1)

hits.sort(key=lambda h: -h[0])
seen = set()
shown = 0
for sc, f, sym in hits:
    key = (f["path"], sym["name"] if sym else None)
    if key in seen:
        continue
    seen.add(key)
    if sym:
        print(f"\n  \033[1m{f['path']}:{sym['line']}\033[0m  \033[2m[{f['layer']}]\033[0m")
        print(f"    func/type \033[1m{sym['name']}\033[0m — {' · '.join(sym['kw'])}")
    else:
        print(f"\n  \033[1m{f['path']}\033[0m  \033[2m[{f['layer']}]\033[0m")
        if f["doc"]:
            print(f"    {f['doc'][:120]}")
        print(f"    kw: {' · '.join(f['keywords'])}")
    shown += 1
    if shown >= 8:
        break

print(f"\n  ({len(hits)} matches; showing {shown}. `layer` tells you which side of the gate/orchestrator")
print("   boundary you are on — see docs/development.md.)")
PY
