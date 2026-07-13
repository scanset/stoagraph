package trust

import "testing"

func TestTrustClass(t *testing.T) {
	// Ordering is Untrusted < Caller < Authoritative — the sink gate compares against it.
	if !(Untrusted < Caller && Caller < Authoritative) {
		t.Fatalf("trust class ordering broken: %d %d %d", Untrusted, Caller, Authoritative)
	}

	// String <-> ParseTrustClass round-trip.
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

	// Unknown strings fail with an error (fail closed at the parse boundary).
	for _, s := range []string{"bogus", "unknown", "untrustedx", "callerx", "authoritativex", ""} {
		if _, err := ParseTrustClass(s); err == nil {
			t.Errorf("ParseTrustClass(%q) should have returned error", s)
		}
	}
}
