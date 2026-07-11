package trust

import "errors"

// file-kw: trust lattice information flow semilattice

// kw: represent trust levels
type TrustClass int

// kw: define trust constants
const (
	Untrusted TrustClass = iota
	Caller
	Authoritative
)

// kw: convert trust class to string
func (c TrustClass) String() string {
	switch c {
	case Untrusted:
		return "untrusted"
	case Caller:
		return "caller"
	case Authoritative:
		return "authoritative"
	default:
		return "unknown"
	}
}

// kw: parse string to trust class
func ParseTrustClass(s string) (TrustClass, error) {
	switch s {
	case "untrusted":
		return Untrusted, nil
	case "caller":
		return Caller, nil
	case "authoritative":
		return Authoritative, nil
	default:
		return 0, errors.New("invalid trust class string")
	}
}

// kw: join two trust classes
func Join(a, b TrustClass) TrustClass {
	if a < b {
		return a
	}
	return b
}

// kw: join multiple trust classes
func JoinAll(classes ...TrustClass) TrustClass {
	if len(classes) == 0 {
		return Authoritative
	}
	min := classes[0]
	for _, c := range classes {
		if c < min {
			min = c
		}
	}
	return min
}
