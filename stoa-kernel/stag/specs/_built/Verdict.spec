name: Verdict
role: component
intent: The gate's decision and the attestation rollup, in one enum. A Verdict is the output of every gate check {Allow, Escalate, Deny}, and the rollup combinators compose child verdicts into a parent over the CRI decision tree carried from ESP. The integer value encodes restrictiveness (Allow < Escalate < Deny), so And takes the most restrictive child and Or the least. This makes the rollup fail-safe by construction: uncertainty (Escalate) and denial are never silently dropped by a conjunction. Negate is a separate involution for expressing a required-to-be-false child. No IO, no policy, no sink logic here; this is pure decision algebra.
api:
  - type Verdict int
  - const ( Allow Verdict = iota; Escalate; Deny )
  - func (v Verdict) String() string
  - func ParseVerdict(s string) (Verdict, error)
  - func And(a, b Verdict) Verdict
  - func Or(a, b Verdict) Verdict
  - func Negate(v Verdict) Verdict
  - func AndAll(vs ...Verdict) Verdict
  - func OrAll(vs ...Verdict) Verdict
behavior:
  - "ORDERING: Allow < Escalate < Deny. The three constants are exactly Allow=0, Escalate=1, Deny=2 via iota. The integer value IS the restrictiveness: a larger value is more restrictive (more fail-safe). And and Or are defined directly in terms of this order, so the ordering is load-bearing, not cosmetic."
  - "STRING: Allow -> \"allow\", Escalate -> \"escalate\", Deny -> \"deny\". String on any value outside the defined set returns \"unknown\"."
  - "PARSE: ParseVerdict is the inverse of String for the three defined names and returns an error for anything else (including \"unknown\"). ROUND-TRIP: for every defined verdict v, ParseVerdict(v.String()) returns v and no error."
  - "AND is conjunction (all children must pass): And(a, b) returns the MORE restrictive (the maximum) of a and b. It is commutative (And(a,b)==And(b,a)), associative, and idempotent (And(x,x)==x). Allow is the identity, And(x, Allow)==x for every x. Deny is absorbing, And(x, Deny)==Deny for every x. FAIL-SAFE: And(Allow, Escalate)==Escalate and And(anything, Deny)==Deny, so a conjunction never allows unless every child allows, and uncertainty is preserved, never dropped."
  - "OR is disjunction (any child suffices): Or(a, b) returns the LESS restrictive (the minimum) of a and b. It is commutative, associative, and idempotent. Deny is the identity, Or(x, Deny)==x for every x. Allow is absorbing, Or(x, Allow)==Allow for every x."
  - "NEGATE is an involution: Negate(Allow)==Deny, Negate(Deny)==Allow, and Negate(Escalate)==Escalate (a negated uncertainty is still an uncertainty). Negate(Negate(v))==v for every v. Equivalently Negate(v) maps value n to (2 - n). DE MORGAN: Negate(And(a,b))==Or(Negate(a),Negate(b)) and Negate(Or(a,b))==And(Negate(a),Negate(b)) for every a,b."
  - "ANDALL folds And left-to-right over the arguments. AndAll() with no arguments returns Allow (the identity, so a parent with no required children allows by construction). AndAll(x)==x. AndAll(a,b,c...) equals the MAXIMUM (most restrictive) verdict among the arguments."
  - "ORALL folds Or left-to-right over the arguments. OrAll() with no arguments returns Deny (the identity, so a parent with no satisfying children denies by construction, which is fail-safe). OrAll(x)==x. OrAll(a,b,c...) equals the MINIMUM (least restrictive) verdict among the arguments."
constraints: package main; standard library only.
