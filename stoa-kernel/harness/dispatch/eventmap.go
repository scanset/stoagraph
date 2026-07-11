package dispatch

// file-kw: event map definition predicate dotted-path match user-authored deterministic domain-specific

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// routeModel is the sentinel a definition sets on `route` to DEFER the recipe choice to the dispatch
// model (a coarse predicate matched the event, but the exact recipe is the model's call).
const routeModel = "model"

// Event is a parsed event payload. Predicates read dotted paths into it ("source", "event.type").
type Event map[string]any

// Definition is one user-authored rule in the event map: a predicate over the event and the recipe
// it selects. Because events are domain-specific and not always clean types, the user authors these
// (like they author recipes) — the code stays flexible, the mapping is data.
type Definition struct {
	ID     string            `json:"id"`
	Match  map[string]string `json:"match"`           // field(dotted) -> exact expected value; ALL must match
	Recipe string            `json:"recipe"`          // the recipe to bind (ignored when route == "model")
	Route  string            `json:"route,omitempty"` // "model" -> defer the recipe choice to the dispatch model
	Tools  []string          `json:"tools,omitempty"` // multi-tool session: bind the config.db routes for THESE
	//                                                    tools (each gated by its own recipe). Empty -> the
	//                                                    single `recipe`'s routes (one-tool session).
	Context []string `json:"context,omitempty"` // READ channel (Planning/30): the context providers this
	//                                              session may read, by name (resolved to specs at bind).
	//                                              Empty -> no READ channel.
}

// EventMap is an ordered list of definitions; first match wins.
type EventMap []Definition

// Match returns the first definition whose predicate the event satisfies.
func (m EventMap) Match(e Event) (Definition, bool) {
	for _, d := range m {
		if matchAll(d.Match, e) {
			return d, true
		}
	}
	return Definition{}, false
}

// matchAll is true iff every predicate field equals the event's value at that dotted path. An empty
// predicate never matches (fail closed — a definition must actually assert something).
func matchAll(pred map[string]string, e Event) bool {
	if len(pred) == 0 {
		return false
	}
	for k, want := range pred {
		got, ok := getPath(e, k)
		if !ok || fmt.Sprint(got) != want {
			return false
		}
	}
	return true
}

// getPath resolves a dotted path ("event.type") through nested maps in the event payload.
func getPath(e Event, dotted string) (any, bool) {
	var cur any = map[string]any(e)
	for _, p := range strings.Split(dotted, ".") {
		m, ok := cur.(map[string]any)
		if !ok {
			return nil, false
		}
		cur, ok = m[p]
		if !ok {
			return nil, false
		}
	}
	return cur, true
}

// LoadEventMap reads a user-authored event map (JSON array of definitions). A missing file is an
// empty map (deterministic layer disabled — everything falls to the model router), not an error.
func LoadEventMap(path string) (EventMap, error) {
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if len(b) == 0 {
		return nil, nil
	}
	var m EventMap
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, fmt.Errorf("event map %s: %w", path, err)
	}
	return m, nil
}
