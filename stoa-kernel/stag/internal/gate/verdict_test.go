package gate

import "testing"

var allVerdicts = []Verdict{Allow, Escalate, Deny}

func TestVerdict(t *testing.T) {
	// And table (the rollup is conjunctive: the most restrictive verdict wins).
	andCases := []struct{ a, b, want Verdict }{
		{Allow, Deny, Deny}, {Allow, Escalate, Escalate}, {Escalate, Deny, Deny},
		{Allow, Allow, Allow}, {Deny, Deny, Deny},
	}
	for _, c := range andCases {
		if got := And(c.a, c.b); got != c.want {
			t.Errorf("And(%v,%v) = %v, want %v", c.a, c.b, got, c.want)
		}
	}

	// Idempotence, identity/absorbing, commutativity over every pair.
	for _, a := range allVerdicts {
		if And(a, a) != a {
			t.Errorf("idempotence failed for %v", a)
		}
		if And(a, Allow) != a || And(a, Deny) != Deny {
			t.Errorf("And identity/absorbing failed for %v", a)
		}
		for _, b := range allVerdicts {
			if And(a, b) != And(b, a) {
				t.Errorf("commutativity failed for %v,%v", a, b)
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
			}
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

	// Fold identity and multi-arg fold (== max over the args).
	if AndAll() != Allow {
		t.Errorf("empty fold identity failed")
	}
	foldCases := []struct {
		vs  []Verdict
		and Verdict
	}{
		{[]Verdict{Escalate}, Escalate},
		{[]Verdict{Allow, Escalate, Deny}, Deny},
		{[]Verdict{Allow, Escalate}, Escalate},
		{[]Verdict{Escalate, Deny}, Deny},
		{[]Verdict{Allow, Allow}, Allow},
	}
	for _, c := range foldCases {
		if got := AndAll(c.vs...); got != c.and {
			t.Errorf("AndAll(%v) = %v, want %v", c.vs, got, c.and)
		}
	}
}

func FuzzVerdictRollup(f *testing.F) {
	f.Add([]byte{0, 1, 2})
	f.Add([]byte{})
	f.Add([]byte{2, 2, 2})

	f.Fuzz(func(t *testing.T, data []byte) {
		var vs []Verdict
		max := Allow
		for _, b := range data {
			v := Verdict(b % 3)
			vs = append(vs, v)
			if v > max {
				max = v
			}
		}
		if len(data) == 0 {
			max = Allow
		}

		// The conjunctive fold equals the most restrictive (max) verdict in the batch.
		if got := AndAll(vs...); got != max {
			t.Errorf("AndAll(%v) = %v, want %v", vs, got, max)
		}
	})
}
