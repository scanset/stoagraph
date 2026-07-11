name: TrustClass
role: component
intent: The trust label that rides every value through StAG's bind graph, and the join that propagates it. A value's class is the least-trusted of everything in its lineage, so combining any input with an untrusted one yields untrusted. This is the meet-semilattice at the heart of the information-flow layer. Raising a class (declassification) is a separate deliberate operation and is NOT done here.
api:
  - type TrustClass int
  - const ( Untrusted TrustClass = iota; Caller; Authoritative )
  - func (c TrustClass) String() string
  - func ParseTrustClass(s string) (TrustClass, error)
  - func Join(a, b TrustClass) TrustClass
  - func JoinAll(classes ...TrustClass) TrustClass
behavior:
  - "ORDERING: Untrusted < Caller < Authoritative. Untrusted is the bottom (least trusted); Authoritative is the top (most trusted). The three constants are exactly Untrusted=0, Caller=1, Authoritative=2 via iota."
  - "STRING: Untrusted -> \"untrusted\", Caller -> \"caller\", Authoritative -> \"authoritative\". String on any value outside the defined set returns \"unknown\"."
  - "PARSE: ParseTrustClass is the inverse of String for the three defined names and returns an error for anything else (including \"unknown\"). ROUND-TRIP: for every defined class c, ParseTrustClass(c.String()) returns c and no error."
  - "JOIN spreads taint downward: Join(a, b) returns the LEAST-trusted (the minimum) of a and b. It is commutative (Join(a,b)==Join(b,a)), associative, and idempotent (Join(x,x)==x)."
  - "LATTICE LAWS: Authoritative is the identity, Join(x, Authoritative)==x for every x. Untrusted is absorbing, Join(x, Untrusted)==Untrusted for every x."
  - "JOIN NEVER RAISES: Join(a, b) <= a and Join(a, b) <= b, always. A value's class can only be lowered by combination, never raised. (Declassification, the only thing that raises a class, is a separate unit, not Join.)"
  - "JOINALL folds Join left-to-right over the arguments. JoinAll() with no arguments returns Authoritative (the identity, so an action with no inputs is maximally trusted by construction). JoinAll(x)==x. JoinAll(a,b,c...) equals the minimum class among the arguments."
constraints: package main; standard library only.
