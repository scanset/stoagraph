name: TrustClassTest
role: test
intent: Verify the trust lattice laws, the string round-trip, and the JoinAll fold, with a fuzz target that folds Join over an arbitrary sequence and asserts the result equals the minimum class. The lattice is the foundation of the whole information-flow layer, so its laws are tested directly and adversarially.
api:
  - func TestTrustClass(t *testing.T)
  - func FuzzTrustClassJoin(f *testing.F)
behavior:
  - "TABLE (Join): Join(Authoritative, Untrusted)==Untrusted; Join(Caller, Authoritative)==Caller; Join(Untrusted, Caller)==Untrusted; Join(Untrusted, Untrusted)==Untrusted; Join(Authoritative, Authoritative)==Authoritative."
  - "COMMUTATIVITY and IDEMPOTENCE: for every pair (a,b) of the three classes, Join(a,b)==Join(b,a); for every class x, Join(x,x)==x."
  - "IDENTITY and ABSORBING: for every class x, Join(x, Authoritative)==x and Join(x, Untrusted)==Untrusted."
  - "ROUND-TRIP: for every defined class x, ParseTrustClass(x.String())==x with a nil error. ParseTrustClass(\"bogus\") and ParseTrustClass(\"unknown\") each return a non-nil error."
  - "JOINALL: JoinAll()==Authoritative; JoinAll(Caller)==Caller; JoinAll(Caller, Untrusted, Authoritative)==Untrusted; JoinAll(Authoritative, Caller)==Caller."
  - "FUZZ FuzzTrustClassJoin: seed the corpus with a few byte slices. In the fuzz body, map each input byte to one of the three classes (for example byte%3), building a sequence; fold Join across the sequence and assert the result equals the minimum class value in that sequence. Also assert the fold never exceeds any element (result <= every element). An empty input sequence must fold to Authoritative (the identity). The property must hold for every input, so a crash or mismatch is a real lattice bug."
constraints: package main; standard library only.
