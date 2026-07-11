name: VerdictTest
role: test
intent: Verify the full verdict rollup algebra - the And/Or lattice laws over every pair and triple, De Morgan, the negate involution, the string round-trip with a fail-closed error case, and the multi-arg folds - plus a fuzz target that folds AndAll and OrAll over an arbitrary sequence and asserts they equal the maximum and minimum verdict. The fuzz is the safety core: a conjunction must never drop a Deny or an Escalate. (Evolved from a minimal first cut once the author/gate loop was fast; the minimal cut proved only the fold property.)
api:
  - func TestVerdict(t *testing.T)
  - func FuzzVerdictRollup(f *testing.F)
behavior:
  - "AND / OR TABLES: And(Allow,Deny)==Deny; And(Allow,Escalate)==Escalate; And(Escalate,Deny)==Deny; And(Allow,Allow)==Allow; And(Deny,Deny)==Deny. Or(Allow,Deny)==Allow; Or(Escalate,Deny)==Escalate; Or(Allow,Escalate)==Allow; Or(Deny,Deny)==Deny."
  - "LATTICE LAWS over every pair (range over {Allow,Escalate,Deny}): commutativity And(a,b)==And(b,a) and Or(a,b)==Or(b,a); idempotence And(x,x)==x, Or(x,x)==x; identity/absorbing And(x,Allow)==x, And(x,Deny)==Deny, Or(x,Deny)==x, Or(x,Allow)==Allow."
  - "ASSOCIATIVITY over every triple: And(And(a,b),c)==And(a,And(b,c)) and Or(Or(a,b),c)==Or(a,Or(b,c))."
  - "DE MORGAN over every pair: Negate(And(a,b))==Or(Negate(a),Negate(b)) and Negate(Or(a,b))==And(Negate(a),Negate(b))."
  - "NEGATE: Negate(Allow)==Deny; Negate(Deny)==Allow; Negate(Escalate)==Escalate; involution Negate(Negate(v))==v for every v."
  - "ROUND-TRIP (defined only): ParseVerdict(v.String())==v with nil error for each of {Allow,Escalate,Deny}. ERROR CASE fails closed: ParseVerdict(\"unknown\"|\"bogus\"|\"\") each returns a non-nil error AND the returned verdict is Deny (the fail-closed value, not the permissive zero value)."
  - "FOLDS: AndAll()==Allow; OrAll()==Deny; and a table of multi-arg folds asserting AndAll(vs...)==max(vs) and OrAll(vs...)==min(vs) (e.g. AndAll(Allow,Escalate,Deny)==Deny, OrAll(Allow,Escalate,Deny)==Allow)."
  - "FUZZ FuzzVerdictRollup: seed with f.Add([]byte{0,1,2}), f.Add([]byte{}), f.Add([]byte{2,2,2}). Body takes data []byte; map each byte index-free to Verdict(b%3) building vs and tracking max/min. Assert AndAll(vs...)==max and OrAll(vs...)==min (empty -> Allow/Deny), and Negate(Negate(v))==v for each element."
constraints: package main; standard library only.
