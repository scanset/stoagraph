package gate

import "fmt"

// file-kw: verdict decision rollup conjunction disjunction negation gate fail-safe

// kw: verdict type gate decision rollup
type Verdict int

// kw: verdict constants allow escalate deny
const (
	Allow Verdict = iota
	Escalate
	Deny
)

// kw: verdict string
func (v Verdict) String() string {
	switch v {
	case Allow:
		return "allow"
	case Escalate:
		return "escalate"
	case Deny:
		return "deny"
	default:
		return "unknown"
	}
}

// kw: parse verdict
func ParseVerdict(s string) (Verdict, error) {
	switch s {
	case "allow":
		return Allow, nil
	case "escalate":
		return Escalate, nil
	case "deny":
		return Deny, nil
	default:
		return Deny, fmt.Errorf("invalid verdict: %q", s) // fail closed
	}
}

// kw: verdict and conjunction max restrictive
func And(a, b Verdict) Verdict {
	if a > b {
		return a
	}
	return b
}

// kw: verdict or disjunction min restrictive
func Or(a, b Verdict) Verdict {
	if a < b {
		return a
	}
	return b
}

// kw: verdict negate involution
func Negate(v Verdict) Verdict {
	switch v {
	case Allow:
		return Deny
	case Deny:
		return Allow
	default:
		return v
	}
}

// kw: verdict andall fold identity allow
func AndAll(vs ...Verdict) Verdict {
	result := Allow
	for _, v := range vs {
		result = And(result, v)
	}
	return result
}

// kw: verdict orall fold identity deny
func OrAll(vs ...Verdict) Verdict {
	result := Deny
	for _, v := range vs {
		result = Or(result, v)
	}
	return result
}
