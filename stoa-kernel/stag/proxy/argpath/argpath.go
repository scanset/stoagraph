// Package argpath extracts the values a policy judges out of a tool call's RAW arguments.
//
// A gateArg used to name a top-level argument, and the proxy read it with fmt.Sprint. That worked for
// strings and numbers and was meaningless for anything else: a `files` array arrived at the gate as the
// Go rendering of its own memory, `"[map[content:rm -rf / path:a.go]]"`. No set_membership can judge
// that, so the only tools you could really govern were the ones whose whole risk sat in a flat scalar.
// For a tool like push_files that is precisely backwards — the scalars (owner, repo) are the harmless
// part and the payload is the dangerous one.
//
// So a gateArg is now a PATH into the arguments:
//
//	owner                  a top-level scalar
//	issue_fields.title     a scalar inside an object
//	files[].path           the `path` of EVERY element of the `files` array
//	reviewers[]            every element of a scalar array
//
// A path may select MORE THAN ONE value (any path crossing `[]`). Every selected value is judged, and
// the call clears only if EVERY one of them clears — an array is not a way to smuggle one bad element
// past a rule that the other elements satisfy.
//
// Two things fail CLOSED rather than being coerced into a string:
//
//   - a path that lands on an object or an array (a composite). There is no honest way to ask
//     "is this object in my allowed set", and pretending there is would be the fmt.Sprint bug again
//     wearing a better hat. Name a scalar leaf inside it.
//   - a path that does not exist in the arguments at all. A missing value is not an empty value; it
//     binds "" and, being outside any allowed set, is denied.
package argpath

// file-kw: gatearg path extract json leaf scalar array wildcard composite fail-closed structured-args

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// Extract pulls every value the path selects out of raw JSON arguments.
//
// It returns the values in document order (deterministic — the audit and the hash depend on it), and an
// error when the path selects a composite or cannot be resolved. An error means DENY: the caller must
// not fall back to some stringified approximation.
// kw: extract path values deterministic fail-closed
func Extract(raw json.RawMessage, path string) ([]string, error) {
	if strings.TrimSpace(path) == "" {
		return nil, fmt.Errorf("empty gate path")
	}
	var doc any
	if len(raw) == 0 {
		// No arguments at all: every path is absent, which binds "" and fails any allow-set. Denying is
		// the outcome; refusing to decide is not needed.
		return []string{""}, nil
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, fmt.Errorf("arguments are not valid JSON: %w", err)
	}
	vals, err := walk(doc, splitPath(path), path)
	if err != nil {
		return nil, err
	}
	if len(vals) == 0 {
		// A path that resolves to nothing (an empty array, an absent key) must not silently clear. It
		// binds one empty value, which no allow-set contains.
		return []string{""}, nil
	}
	return vals, nil
}

// segment is one step of a path: a field name, optionally iterating into an array.
type segment struct {
	field string // "" for a bare "[]" step
	each  bool   // the field is an array to iterate
}

// splitPath parses "files[].path" into [{files,each} {path}].
func splitPath(p string) []segment {
	out := make([]segment, 0, 4)
	for _, part := range strings.Split(p, ".") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		s := segment{field: part}
		if strings.HasSuffix(part, "[]") {
			s.field, s.each = strings.TrimSuffix(part, "[]"), true
		}
		out = append(out, s)
	}
	return out
}

// walk descends the document, fanning out over every `[]`.
func walk(node any, segs []segment, full string) ([]string, error) {
	if len(segs) == 0 {
		return leaf(node, full)
	}
	s := segs[0]

	if s.field != "" {
		obj, ok := node.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("path %q: %q is not an object", full, s.field)
		}
		child, present := obj[s.field]
		if !present {
			return []string{""}, nil // absent binds "" -> fails any allow-set
		}
		node = child
	}

	if !s.each {
		return walk(node, segs[1:], full)
	}

	arr, ok := node.([]any)
	if !ok {
		return nil, fmt.Errorf("path %q: %q is not an array (drop the [])", full, s.field)
	}
	// EVERY element is judged. Order is document order, so the decision and its audit value are stable.
	out := make([]string, 0, len(arr))
	for _, el := range arr {
		vs, err := walk(el, segs[1:], full)
		if err != nil {
			return nil, err
		}
		out = append(out, vs...)
	}
	return out, nil
}

// leaf renders one selected value, refusing composites.
//
// The rendering is canonical, not Go's: a JSON number is written the way it was meant (1 not 1e+00),
// because the string the rule sees is the string a policy author wrote in their allow-set.
func leaf(node any, full string) ([]string, error) {
	switch v := node.(type) {
	case string:
		return []string{v}, nil
	case bool:
		return []string{strconv.FormatBool(v)}, nil
	case float64:
		return []string{canonicalNumber(v)}, nil
	case nil:
		return []string{""}, nil // JSON null binds "" -> fails any allow-set
	case map[string]any:
		return nil, fmt.Errorf("path %q lands on an object — a policy cannot judge a whole object; name a scalar inside it (e.g. %s.<field>)", full, full)
	case []any:
		return nil, fmt.Errorf("path %q lands on an array — append [] to judge each element (e.g. %s[])", full, full)
	default:
		return nil, fmt.Errorf("path %q: unsupported value type %T", full, node)
	}
}

// canonicalNumber writes a JSON number the way a human wrote it: an integral value has no decimal tail,
// so an allow-set containing "3" matches the JSON number 3.
func canonicalNumber(f float64) string {
	if f == float64(int64(f)) {
		return strconv.FormatInt(int64(f), 10)
	}
	return strconv.FormatFloat(f, 'f', -1, 64)
}
