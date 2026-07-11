# 08 — Recipe format recon (broker phase, pre-spec)

Recorded 2026-07-01. Inputs to the RecipeParse spec: a three-angle design panel with an adversarial
judge, YAML threat research verified against the yaml.v3 source, Claude API facts for the proposer
adapter, and a ProofLayer egress sweep. This doc is the durable record; the raw agent outputs were
session-ephemeral. Nothing here is ratified until the open decisions at the bottom are settled.

## Verdict: lint-first declarations-then-graph shape, hardened with grafts

Three independent proposals (ansible-fidelity 7.0, kernel-fidelity 8.1, lint-first 8.6; safety and
determinism weighted highest). The winning shape:

- A single YAML mapping. Header: `recipe`, `version` (pinned to exactly 1, fail closed).
- `ingredients:` section: each entry is `{origin, trust}`. A `value:` key is unknown and rejects
  (the broker binds values at runtime; an authored value under `trust: authoritative` would smuggle
  trusted content past the binder).
- `rules:` registry: every release predicate lives under an id, pure data, closed kinds.
- `steps:` (tasks) list: ordered, exactly one module key per task from the closed kind set; document
  order is evaluation order and becomes `ReleaseEvent.Ordering`.
- Three laws make lint proofs one-pass and mechanical: declare-before-use (the static image of
  Eval's own slice walk and severed-label denial), explicit one-to-one edges, rules referenced only
  by id (the loader guarantees `Step.RuleID` and `Step.Rule` denote the same declared predicate, so
  a signed event's `AuthorizingRule` resolves to exactly one diffable rule).
- `actor` is mandatory wherever a rule is present (the WHO dimension of a crossing is never empty).

Grafts adopted from the runners-up:

1. Kernel-owned enum spellings: every YAML enum string is exactly a kernel `String()` output with a
   fail-closed `Parse` inverse. Requires `release.ParseRuleKind` and a `NodeKind` String/Parse pair,
   added through the ratchet oracles. One spelling register across recipes and signed records.
2. Per-kind required/forbidden key tables: illegal keys for a kind reject, never zero-fill. The
   rule/actor pair is legal only on `sensitivity: authoritative`; a rule on a benign sink rejects as
   dead policy (GateSink ignores release for SinkBenign; accepting it would be a declared claim with
   no enforcement behind it).
3. Reject-before-hash: `gate`/`branch`/`foreach`/`exit` are recognized vocabulary that fails with a
   distinct not-yet-implemented error, distinguishable from unknown-kind typos; a file containing
   them is never canonicalized, hashed, or signed.
4. Teaching rejections: `when:`, `loop:`, `register:`, `vars:`, `{{ }}` are refused by name with the
   StAG alternative in the error text (branch, foreach, propose out-slots, declared ingredients and
   release rules).
5. The rules registry is documented and diffed as the single "what may ever be released" block an
   assessor reviews before the recipe runs.
6. Held in reserve: a draft-mode hygiene-warnings tier for dead declarations (never relaxed at sign
   time), pending open decision 7.

Deliberately absent, stated per the honest-ceiling invariant: no `propose.inputs` unless ratified
(Step has no field for it and Eval stamps Untrusted regardless; the file must not declare lineage
the kernel does not compute); no templating, composite arguments, or expressions; trust vocabulary
is exactly the three kernel classes; the linter duplicates judgments the kernel makes anyway and
must be fuzzed against Eval in the U7 pattern.

## YAML threat surface (verified against go-yaml/yaml v3 branch source)

Meta-hazard: go-yaml/yaml was archived April 2025; frozen, security-fixes only via the YAML-org
fork `go.yaml.in/yaml/v3`. CVE-2022-28948 (Unmarshal panic) is fixed in v3.0.1. Pin v3.0.1 or the
maintained fork.

Load-bearing findings (each verified in decode.go/resolve.go/readerc.go unless marked):

- Aliases/anchors: expansion limits exist but only when expanding into Go values; decoding into
  `*yaml.Node` does no expansion (AliasNode holds a pointer), so a Node-first parser is immune at
  parse time but a lint walk that follows `.Alias` reintroduces blowup. Reject `Anchor != ""` and
  `Kind == AliasNode` outright.
- Merge keys: a plain unquoted `<<:` is a live merge even with aliases banned (inline mapping needs
  no alias); merged-vs-explicit collisions bypass the duplicate-key error. Reject the key text `<<`.
- Norway problem: fixed for untyped decode in v3, but typed Go `bool` targets resurrect YAML-1.1
  coercion (`yes/on/y` -> true). No bool-typed fields anywhere in the schema or decode targets.
- Integers: base-0 parsing with underscores stripped, so `0x1F`, `014` (legacy octal), `0o`, `0b`,
  `+5`, `1_000` all parse as ints; int-parse failure falls through to float, so `09` -> 9.0 and
  huge literals lose precision silently; float into an int64 field truncates silently (`5.5` -> 5).
  YAML implicit typing launders exactly the non-canonical numerals the kernel's canonical-only rule
  refuses. Numbers must be parsed from raw scalar text with the kernel's own predicate.
- Timestamps: bare `2026-7-1` untagged resolves to time.Time under untyped decode; json.Marshal
  would silently rewrite it RFC3339. String-typed targets preserve raw text.
- Duplicate keys: v3 always errors (uniqueKeys not disableable), textual comparison, applies to
  Unmarshal, Decoder, and Node.Decode. Merge-introduced collisions are exempt (see above).
- Multi-document: `yaml.Unmarshal` silently ignores everything after the first `---`. Enforce one
  Decode then exactly io.EOF.
- KnownFields: cannot be enabled on `Node.Decode` at all (fresh internal decoder); map[string]T
  fields and inline maps swallow unknown keys. Unknown-key rejection must be our own closed-key-set
  check in the lint walk.
- Tags: unknown tags are silently ignored on string targets; `!!binary` base64-decodes into a Go
  string and can smuggle invalid UTF-8 that json.Marshal mutates to U+FFFD before hashing. Allowlist
  tags; reject `!!binary` and `!!timestamp`.
- Size/depth: no input-size, node-count, or depth limit in the library; node construction is
  recursive, so deep nesting is a stack-exhaustion process kill (crash depth UNVERIFIED). Cap input
  bytes; walk iteratively with depth and node caps.
- Encoding: UTF-16 with BOM is silently transcoded; UTF-8 is validated but homoglyphs, zero-width,
  and bidi characters are legal, with no Unicode normalization. Reject BOMs pre-parse; ASCII key
  grammar.
- Canonicalization: `map[interface{}]interface{}` and Inf/NaN make json.Marshal error (fail closed
  but reachable); int64(5) and float64(5.0) marshal identically so float leaks are invisible for
  small integral values; discipline must come from construction typing, not output inspection. Our
  canonical JSON is Go-flavored (sorted keys, HTML escaping), not RFC 8785; a future cross-language
  verifier must replicate it exactly.

## Parser rules for the RecipeParse spec (flat, ordered, fail-closed)

1. Pin yaml.v3 >= v3.0.1 or `go.yaml.in/yaml/v3`; treat the library as untrusted-adjacent.
2. Cap input size before parsing (e.g. 64 KiB).
3. Reject non-UTF-8 input pre-parse: UTF-8 BOM, UTF-16 BOMs, NUL bytes all refuse.
4. Decode into `*yaml.Node`; wrap the call in recover() converting panic to parse error.
5. Single document: first Decode succeeds, next Decode must return exactly io.EOF. Empty/null
   document rejects.
6. Walk the node tree iteratively with a depth cap (e.g. 32) and node-count cap (e.g. 10,000).
7. Reject any node with `Anchor != ""` and any `Kind == AliasNode`.
8. Reject the key text `<<` and tag `!!merge` wherever they appear.
9. Tag allowlist: scalars in {"", "!", "!!str"}; collections in {"", "!!map", "!!seq"}; all else
   rejects (kills !!binary, !!timestamp, custom and %TAG-expanded tags).
10. Map keys: ScalarNode with ShortTag "!!str", non-empty, matching `^[a-z][a-z0-9_]{0,63}$`.
11. Duplicate keys: rely on v3's uniqueKeys error AND run our own byte-exact per-mapping check.
12. Unknown keys: closed key set for every mapping enforced in our walk; never rely on KnownFields.
13. Read every scalar as raw text (`node.Value`); never decode values through interface{}; any
    struct decode targets contain only string fields.
14. Integers (`min`/`max`): kernel-canonical decimal text only, the releaserule.go predicate
    (ParseInt(s,10,64) succeeds AND s == FormatInt(n,10)); additionally min <= max.
15. Enum fields: exact-match closed string sets via the kernel Parse inverses.
16. Strings bound for Slot/rule fields: byte-exact, no trimming or normalization; empty `signed`
    rejects at parse time (runtime already fails closed).
17. Referential integrity after the walk: every `in:` resolves to a declared ingredient or an
    earlier step's `out:`; every rule reference resolves; exactly one module key per task.
18. All rejections terminal and specific, with Node line/column; no lenient mode.
19. Unique `field:` per recipe (U7 v2 adversarial obligation): duplicate sink field strings
    collapse the event-to-crossing correspondence to many-to-many (events discriminate by
    Ordering, but the stated invariant matches on TargetField). The linter rejects duplicates so
    the attestation stays one-to-one.
20. Rule id and rule body always derive from the same authored registry entry (U7 v2 adversarial
    obligation): the kernel records Step.RuleID verbatim and never cross-checks it against
    Step.Rule; the parser's denormalization is the only thing binding label to predicate, so it
    must be structural, never author-supplied twice.
21. Empty string is not a legal `set:` member (U7 v2 adversarial obligation): the kernel now
    refuses to release a MISSING slot regardless (hardened), but a present empty value releasing
    against an author-enumerated "" is legal declassifier behavior; the linter should still warn,
    since signed_equality already treats empty as unconfigured.

Broker contract note (from the same adversarial pass): ReleaseEvents are per-crossing
attestations emitted before the step's outgoing edge resolves; a faulted or halted run can
legitimately carry a genuine cleared-crossing event. Downstream consumers read Events together
with Verdict and Fault, never Events alone.

Parser implementation status (built as U8, transcripts/kernel-u8-recipeparse.md): all 21 rules and
9 format laws implemented in `recipe/`. The U8 adversarial pass upgraded rule 17's declare-before-use
from a document-order scan to a definite-assignment dataflow (a consumed slot must be defined on
every path to its consumer; a branch can skip its producer, which a document-order check misses and
which trips the kernel Fault a parsed recipe must never trip). It also closed two accept-of-forbidden
gaps not in the original rule list but implied by rules 7 and 9: an anchor on a mapping KEY, and a
custom/%TAG tag on a COLLECTION node (rules 7 and 9 now enforced on keys and collections, not only on
values and scalars). String values additionally reject C0/C1 control characters (defense-in-depth for
format law 1's byte-exact-verbatim surfaces, closing a double-quote-escape smuggle into signed
fields).

## Format-level consequences (spec text, not implementation detail)

1. `min`/`max` are canonical decimal integer literals by format law (`0` or `-?[1-9][0-9]*`); all
   other scalars are strings taken byte-exact. Without this, any other YAML implementation in the
   pipeline legally types `014`/`0x1F`/`1_000`/`5.5` differently: a cross-tool parsing-differential
   attack on the gate definition itself. This is the format analog of canonical-only release.
2. `set` members and `signed` values are required-quoted in the format, so ambiguity rejects
   instead of being interpreted.
3. Anchors, aliases, and merge keys are banned by format definition; the rules registry and
   ingredient names are the only sanctioned reuse mechanism; `<<` is an illegal key forever.
4. No boolean-typed fields anywhere in the schema; every flag is a closed string enum.
5. One recipe file is exactly one YAML document; `---`/`...` separators are illegal.
6. Key grammar `^[a-z][a-z0-9_]{0,63}$` (ASCII-only) is format law, killing homoglyph collisions.
7. Encoding is format law: UTF-8, no BOM.
8. `steps` order is load-bearing (becomes ReleaseEvent.Ordering); with the alias ban this makes
   ordering unforgeable from the file alone.
9. Canonicalization: two hashes via record.CanonicalHash. An artifact hash over the raw file bytes
   as received (provenance, immune to parse-layer ambiguity), and a semantic hash over a
   parser-constructed canonical form built only from string/int64/[]string/map[string]any, enums by
   canonical name, sets sorted and deduped, tasks as an order-preserving array. Never hash anything
   YAML-decoded into interface{}. Interacts with open decision 2.

## Proposer adapter facts (Claude API, verified against live docs 2026-07-01)

- Default proposer model: claude-haiku-4-5-20251001 (cheapest, fastest). Model choice buys zero
  extra trust; output is always stamped Untrusted.
- On Opus 4.7+ models, non-default temperature/top_p/top_k return HTTP 400. The adapter must not
  send temperature at all.
- Endpoint POST https://api.anthropic.com/v1/messages; headers x-api-key ($ANTHROPIC_API_KEY),
  anthropic-version: 2023-06-01 (current). Required body: model, max_tokens, messages. System
  prompt as a top-level string. Completion text at content[0].text; check stop_reason.
- Official Go SDK github.com/anthropics/anthropic-sdk-go (v1.55+, Go >= 1.23, matches the module):
  auto-retries 429/5xx/408/409 twice, 10-minute non-streaming timeout. Recommended over raw
  net/http; retry-after header is authoritative when present.
- Errors: retryable 429/500/504/529; everything else terminal. Single-shot timeout 30 s for small
  max_tokens.

## Egress recon (ProofLayer sweep; honest ceiling)

ProofLayer has NO external hash ingestion path today. Its ingest (`POST /api/scans`, mTLS) expects
a full IngestScanRequest scan envelope and computes its own replay_hash; the transparency log
(`POST /log/entries`) accepts only certificate + SAN entries. No Rekor/sigstore/DSSE code exists in
the repo; its transparency log is a self-contained Merkle implementation. Docs must not claim
"anchors to ProofLayer" until an endpoint exists.

What lines up well: signing is ECDSA-P256 over SHA256(replay_hash), where replay_hash is a 64-hex
sha256 over canonical JSON, structurally identical to ReleaseEvent.Hash(). Reference client path:
esp-daemon-R9 src/output/signing.rs and src/submission/transform.rs.

Phase-3 options: (a) a new ProofLayer endpoint (e.g. POST /api/attestations accepting hash +
signature block), (b) direct-to-Rekor from StAG, (c) both. Owner's call when the phase opens.

## Decisions (ratified by Curtis 2026-07-01; see Planning/09 for the use case that shaped 8)

1. Enum spelling register: RATIFIED. Change the kernel String() outputs to the snake_case planning
   spellings (set_membership, signed_equality, numeric_range) with fail-closed Parse inverses, a
   ratchet-gated change to the proven units, frozen before the first signed recipe.
2. The signed contract: RATIFIED. The canonical JSON of the authored document is the contract; the
   compiled Recipe is derived from it. Two hashes per the recommendation above: an artifact hash
   over the raw file bytes (provenance, broker-kept) and a semantic hash over the parser-built
   canonical form. ReleaseEvent grows a recipe_hash field carrying the semantic hash (reopens U6
   through the ladder).
3. `propose.inputs`: RATIFIED. Reject in v1 as unrepresentable; the file must not declare lineage
   the kernel does not compute.
4. Actor vocabulary: RATIFIED. Free string recorded verbatim in v1; registry validation is a later
   linter upgrade.
5. Guaranteed-Deny sinks: RATIFIED. Hard error at sign time (the load-bearing lint, Planning/04).
6. Caller-class scope: RATIFIED. `trust: caller` ingredients may flow to authoritative sinks under
   a rule; GateSink deny-unless-released already proves the semantics.
7. Dead declarations: RATIFIED. Draft-mode warnings that become errors at sign time; signing
   strictness never relaxes.
8. Deferred kinds: SUPERSEDED by the first use case (Planning/09). branch and gate enter v1 scope
   with full semantics (parse, lint, Eval, invariant re-fuzzed over runtime-chosen edges); foreach
   and exit remain recognized-but-rejected (reject-before-hash), schema version bumps when they
   ship.
9. Duplicate `out:` slot names: RATIFIED. Reject at lint. Noted: mutually exclusive branch paths
   may pressure this later; revisit with foreach.
10. Hash scope of step ids: RATIFIED. Ids are part of the signed canonical document (a rename is a
    semantic edit). Ids are load-bearing regardless: branch and goto targets are ids.

## Unverified items (recorded per the honest-ceiling discipline)

Exact stack-overflow depth for nested YAML input (recursion confirmed in source, threshold not
measured); the float-to-int64 truncation guard's exact presence in the v3.0.1 tag (confirmed on
branch v3); %YAML directive version-acceptance semantics (immaterial under the tag allowlist);
Claude SDK TCP keep-alive details and cross-version determinism guarantees.

Sources: go-yaml/yaml v3 branch decode.go/resolve.go/yaml.go/readerc.go; GHSA-hp87-p4gw-j4gq
(CVE-2022-28948); go.yaml.in/yaml/v3 on pkg.go.dev; docs.claude.com Messages API and models pages;
prooflayer crates prooflayer-2 (routes/evidence, routes/transparency, transparency/merkle) and
assessor-core (signing/envelope.rs), esp-daemon-R9 (output/signing.rs, submission/transform.rs).
