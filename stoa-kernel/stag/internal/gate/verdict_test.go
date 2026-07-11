package gate

import "testing"

var allVerdicts = []Verdict{Allow, Escalate, Deny}

func TestVerdict(t *testing.T) {
	// And / Or tables.
	andCases := []struct{ a, b, want Verdict }{
		{Allow, Deny, Deny}, {Allow, Escalate, Escalate}, {Escalate, Deny, Deny},
		{Allow, Allow, Allow}, {Deny, Deny, Deny},
	}
	for _, c := range andCases {
		if got := And(c.a, c.b); got != c.want {
			t.Errorf("And(%v,%v) = %v, want %v", c.a, c.b, got, c.want)
		}
	}
	orCases := []struct{ a, b, want Verdict }{
		{Allow, Deny, Allow}, {Escalate, Deny, Escalate}, {Allow, Escalate, Allow}, {Deny, Deny, Deny},
	}
	for _, c := range orCases {
		if got := Or(c.a, c.b); got != c.want {
			t.Errorf("Or(%v,%v) = %v, want %v", c.a, c.b, got, c.want)
		}
	}

	// Commutativity, idempotence, identity, absorbing, De Morgan over every pair.
	for _, a := range allVerdicts {
		if And(a, a) != a || Or(a, a) != a {
			t.Errorf("idempotence failed for %v", a)
		}
		if And(a, Allow) != a || And(a, Deny) != Deny {
			t.Errorf("And identity/absorbing failed for %v", a)
		}
		if Or(a, Deny) != a || Or(a, Allow) != Allow {
			t.Errorf("Or identity/absorbing failed for %v", a)
		}
		for _, b := range allVerdicts {
			if And(a, b) != And(b, a) || Or(a, b) != Or(b, a) {
				t.Errorf("commutativity failed for %v,%v", a, b)
			}
			if Negate(And(a, b)) != Or(Negate(a), Negate(b)) {
				t.Errorf("De Morgan (And) failed for %v,%v", a, b)
			}
			if Negate(Or(a, b)) != And(Negate(a), Negate(b)) {
				t.Errorf("De Morgan (Or) failed for %v,%v", a, b)
			}
		}
	}

	// Associativity over every triple.
	for _, a := range allVerdicts {
		for _, b := range allVerdicts {
			for _, c := range allVerdicts {
				if And(And(a, b), c) != And(a, And(b, c)) {
					t.Errorf("And associativity failed for %v,%v,%v", a, b, c)
				}
				if Or(Or(a, b), c) != Or(a, Or(b, c)) {
					t.Errorf("Or associativity failed for %v,%v,%v", a, b, c)
				}
			}
		}
	}

	// Negate: table + involution.
	if Negate(Allow) != Deny || Negate(Deny) != Allow || Negate(Escalate) != Escalate {
		t.Errorf("Negate table failed")
	}
	for _, v := range allVerdicts {
		if Negate(Negate(v)) != v {
			t.Errorf("Negate involution failed for %v", v)
		}
	}

	// String round-trip; error cases fail closed to Deny.
	for _, v := range allVerdicts {
		if got, err := ParseVerdict(v.String()); err != nil || got != v {
			t.Errorf("ParseVerdict(%q) = (%v,%v), want (%v,nil)", v.String(), got, err, v)
		}
	}
	for _, s := range []string{"unknown", "bogus", ""} {
		if got, err := ParseVerdict(s); err == nil {
			t.Errorf("ParseVerdict(%q) should error", s)
		} else if got != Deny {
			t.Errorf("ParseVerdict(%q) error = %v, want Deny (fail closed)", s, got)
		}
	}

	// Fold identities and multi-arg folds (== max / min over the args).
	if AndAll() != Allow || OrAll() != Deny {
		t.Errorf("empty fold identities failed")
	}
	foldCases := []struct {
		vs       []Verdict
		and, or_ Verdict
	}{
		{[]Verdict{Escalate}, Escalate, Escalate},
		{[]Verdict{Allow, Escalate, Deny}, Deny, Allow},
		{[]Verdict{Allow, Escalate}, Escalate, Allow},
		{[]Verdict{Escalate, Deny}, Deny, Escalate},
		{[]Verdict{Allow, Allow}, Allow, Allow},
	}
	for _, c := range foldCases {
		if got := AndAll(c.vs...); got != c.and {
			t.Errorf("AndAll(%v) = %v, want %v", c.vs, got, c.and)
		}
		if got := OrAll(c.vs...); got != c.or_ {
			t.Errorf("OrAll(%v) = %v, want %v", c.vs, got, c.or_)
		}
	}
}

func FuzzVerdictRollup(f *testing.F) {
	f.Add([]byte{0, 1, 2})
	f.Add([]byte{})
	f.Add([]byte{2, 2, 2})

	f.Fuzz(func(t *testing.T, data []byte) {
		var vs []Verdict
		max, min := Allow, Deny
		for _, b := range data {
			v := Verdict(b % 3)
			vs = append(vs, v)
			if v > max {
				max = v
			}
			if v < min {
				min = v
			}
		}
		if len(data) == 0 {
			max, min = Allow, Deny
		}

		// Folds equal max / min; and each element obeys De Morgan + involution.
		if got := AndAll(vs...); got != max {
			t.Errorf("AndAll(%v) = %v, want %v", vs, got, max)
		}
		if got := OrAll(vs...); got != min {
			t.Errorf("OrAll(%v) = %v, want %v", vs, got, min)
		}
		for _, v := range vs {
			if Negate(Negate(v)) != v {
				t.Errorf("involution failed for %v", v)
			}
		}
	})
}
