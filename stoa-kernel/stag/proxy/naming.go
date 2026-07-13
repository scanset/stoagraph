package proxy

// file-kw: tool naming namespace advertised server prefix collision unambiguous split model-safe

import "strings"

// NameSep separates the server from the tool in an ADVERTISED tool name.
//
// It is "__" and not "." or "/" because the advertised names are handed to a model verbatim (the
// orchestrator passes the gate's tool surface straight to the provider's tool-use API), and both
// Anthropic and OpenAI require a tool name to match ^[a-zA-Z0-9_-]+$. A dot would be rejected and
// tool-use would break.
const NameSep = "__"

// AdvertisedName is the name the AGENT sees for a routed tool: <server>__<tool>.
//
// The gate ALWAYS namespaces, even when only one server is connected. Prefixing only on collision
// would mean that registering a second server RENAMES a tool the agent already knows — a route that
// worked yesterday resolves differently today, which is the exact surprise this gate exists to
// prevent. A stable name costs one prefix; an unstable one costs the guarantee.
// kw: advertise namespaced always-prefix stable
func AdvertisedName(server, tool string) string { return server + NameSep + tool }

// SplitAdvertised recovers (server, tool) from an advertised name.
//
// It cuts at the FIRST separator. Server names may not contain "__" (ValidServerName rejects it), so
// the split is unambiguous even when the DOWNSTREAM tool name itself contains one: "gh__foo__bar"
// is unambiguously server "gh", tool "foo__bar". The round trip is lossless.
// kw: split first-separator lossless unambiguous
func SplitAdvertised(name string) (server, tool string, ok bool) {
	s, t, found := strings.Cut(name, NameSep)
	if !found || s == "" || t == "" {
		return "", "", false
	}
	return s, t, true
}

// ValidServerName reports whether a name may be used as an MCP server name.
//
// Two constraints, both load-bearing:
//  1. ^[a-zA-Z0-9_-]+$ — the name becomes part of a tool name handed to a model, and the provider
//     tool-use APIs reject anything else.
//  2. no "__" — that is the separator, and a server named "a__b" would make SplitAdvertised
//     ambiguous. A single underscore is fine.
//
// Fail closed: an invalid name is rejected at registration, not silently mangled at advertise time.
// kw: validate server name charset separator fail-closed
func ValidServerName(s string) bool {
	if s == "" || strings.Contains(s, NameSep) {
		return false
	}
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '_', r == '-':
		default:
			return false
		}
	}
	return true
}
