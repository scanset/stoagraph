package release

// file-kw: release rule declassifier closed set membership signed equality numeric range laundering

import (
	"fmt"
	"strconv"
)

// kw: release rule kind set_membership signed_equality numeric_range
type RuleKind int

// kw: rule kind constants
const (
	RuleSetMembership RuleKind = iota
	RuleSignedEquality
	RuleNumericRange
)

// kw: rule kind string
func (k RuleKind) String() string {
	switch k {
	case RuleSetMembership:
		return "set_membership"
	case RuleSignedEquality:
		return "signed_equality"
	case RuleNumericRange:
		return "numeric_range"
	default:
		return "unknown"
	}
}

// kw: parse rule kind fail-closed inverse of string
func ParseRuleKind(s string) (RuleKind, error) {
	switch s {
	case "set_membership":
		return RuleSetMembership, nil
	case "signed_equality":
		return RuleSignedEquality, nil
	case "numeric_range":
		return RuleNumericRange, nil
	default:
		return RuleKind(-1), fmt.Errorf("invalid rule kind: %q", s) // fail closed (inv 8)
	}
}

// kw: release rule closed predicate
type ReleaseRule struct {
	Kind   RuleKind
	Set    []string
	Signed string
	Min    int64
	Max    int64
}

// kw: release predicate exact membership bounds
func (r ReleaseRule) Release(value string) bool {
	switch r.Kind {
	case RuleSetMembership:
		for _, m := range r.Set {
			if value == m {
				return true
			}
		}
		return false
	case RuleSignedEquality:
		return r.Signed != "" && value == r.Signed // empty signed: fail closed
	case RuleNumericRange:
		n, err := strconv.ParseInt(value, 10, 64)
		// canonical form only: keeps accepted strings a finite enumerable set (inv 6)
		return err == nil && value == strconv.FormatInt(n, 10) && n >= r.Min && n <= r.Max
	default:
		return false // fail closed (inv 6/8)
	}
}
