package trust

import "errors"

// file-kw: trust class ordered levels origin label information flow

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
