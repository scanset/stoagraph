package trust

import (
	"testing"
)

func TestTrustClass(t *testing.T) {
	tests := []struct {
		name string
		a    TrustClass
		b    TrustClass
		want TrustClass
	}{
		{"Authoritative, Untrusted", Authoritative, Untrusted, Untrusted},
		{"Caller, Authoritative", Caller, Authoritative, Caller},
		{"Untrusted, Caller", Untrusted, Caller, Untrusted},
		{"Untrusted, Untrusted", Untrusted, Untrusted, Untrusted},
		{"Authoritative, Authoritative", Authoritative, Authoritative, Authoritative},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Join(tt.a, tt.b)
			if got != tt.want {
				t.Errorf("Join(%v, %v) = %v; want %v", tt.a, tt.b, got, tt.want)
			}
		})
	}

	// Test commutativity and idempotence
	for _, a := range []TrustClass{Untrusted, Caller, Authoritative} {
		for _, b := range []TrustClass{Untrusted, Caller, Authoritative} {
			if got := Join(a, b); got != Join(b, a) {
				t.Errorf("Join(%v, %v) = %v, want %v (commutative)", a, b, got, Join(b, a))
			}
			if got := Join(a, a); got != a {
				t.Errorf("Join(%v, %v) = %v, want %v (idempotent)", a, a, got, a)
			}
		}
	}

	// Test identity and absorbing laws
	for _, x := range []TrustClass{Untrusted, Caller, Authoritative} {
		if got := Join(x, Authoritative); got != x {
			t.Errorf("Join(%v, Authoritative) = %v, want %v (identity)", x, got, x)
		}
		if got := Join(x, Untrusted); got != Untrusted {
			t.Errorf("Join(%v, Untrusted) = %v, want %v (absorbing)", x, got, Untrusted)
		}
	}

	// Test round-trip
	for _, c := range []TrustClass{Untrusted, Caller, Authoritative} {
		s := c.String()
		parsed, err := ParseTrustClass(s)
		if err != nil {
			t.Errorf("ParseTrustClass(%q) returned error: %v", s, err)
		}
		if parsed != c {
			t.Errorf("ParseTrustClass(%q) = %v, want %v", s, parsed, c)
		}
	}

	// Test error cases for ParseTrustClass
	errorTests := []string{"bogus", "unknown", "untrustedx", "callerx", "authoritativex"}
	for _, s := range errorTests {
		_, err := ParseTrustClass(s)
		if err == nil {
			t.Errorf("ParseTrustClass(%q) should have returned error", s)
		}
	}

	// Test JoinAll
	joinAllTests := []struct {
		classes []TrustClass
		want    TrustClass
	}{
		{[]TrustClass{}, Authoritative},
		{[]TrustClass{Caller}, Caller},
		{[]TrustClass{Caller, Untrusted, Authoritative}, Untrusted},
		{[]TrustClass{Authoritative, Caller}, Caller},
	}

	for _, tt := range joinAllTests {
		got := JoinAll(tt.classes...)
		if got != tt.want {
			t.Errorf("JoinAll(%v) = %v, want %v", tt.classes, got, tt.want)
		}
	}
}

func FuzzTrustClassJoin(f *testing.F) {
	f.Add([]byte{0, 1, 2})
	f.Add([]byte{0, 0, 0})
	f.Add([]byte{2, 2, 2})
	f.Add([]byte{0, 2, 1, 0})

	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) == 0 {
			// Empty input should fold to Authoritative (identity)
			result := JoinAll()
			if result != Authoritative {
				t.Errorf("JoinAll() with no args = %v, want %v", result, Authoritative)
			}
			return
		}

		// Build sequence of classes from bytes
		classes := make([]TrustClass, len(data))
		for i, b := range data {
			classes[i] = TrustClass(b % 3)
		}

		// Fold Join across the sequence
		result := JoinAll(classes...)

		// Result should be the minimum class in the sequence
		min := classes[0]
		for _, c := range classes {
			if c < min {
				min = c
			}
		}
		if result != min {
			t.Errorf("JoinAll(%v) = %v, want %v (minimum)", classes, result, min)
		}

		// The fold result should never exceed any element
		for _, c := range classes {
			if result > c {
				t.Errorf("JoinAll(%v) = %v, exceeds element %v", classes, result, c)
			}
		}
	})
}
