// Package provider is the READ channel of the dual proxy (Planning/17/18): context
// providers behind one interface, with the load-bearing guarantee that ALL context
// is stamped untrusted at origin, unbypassably. A provider yields ContextItems for a
// query; Gather runs a set of providers and FORCES every item's trust to untrusted —
// a provider cannot hand back trusted-looking context. Reads are label+record, not
// deny: a provider that errors contributes nothing and is reported, not fatal.
package provider

// file-kw: context provider read channel untrusted gather label-at-origin fail-open http adapter

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// Untrusted is the only trust class context ever carries — enforced by Gather.
const Untrusted = "untrusted"

// ReadEvent is the audit record of one READ crossing (Planning/30): a resources/read on a context
// provider. Reads are label+record, not deny — this is the "record" half. Provider/Query/Items are
// always set; Sources names each returned item's origin; Errors carries any provider failures (reads
// are fail-open, so an error is recorded, not fatal).
// kw: read event audit provider query items sources read-channel crossing
type ReadEvent struct {
	Provider string   `json:"provider"`
	Query    string   `json:"query"`
	Items    int      `json:"items"`
	Sources  []string `json:"sources,omitempty"`
	Errors   []string `json:"errors,omitempty"`
}

// kw: context item source text trust score
type ContextItem struct {
	Source string
	Text   string
	Trust  string
	Score  float64
}

// kw: context provider name provide query items
type ContextProvider interface {
	Name() string
	Provide(ctx context.Context, query string) ([]ContextItem, error)
}

// kw: provider error name reason
type ProviderError struct {
	Provider string
	Err      string
}

// kw: gather run providers stamp untrusted fail-open per-provider
func Gather(ctx context.Context, query string, providers []ContextProvider) ([]ContextItem, []ProviderError) {
	var items []ContextItem
	var errs []ProviderError
	for _, p := range providers {
		got, err := p.Provide(ctx, query)
		if err != nil {
			// read fail-open: a failing source is skipped and reported, not fatal.
			errs = append(errs, ProviderError{Provider: p.Name(), Err: err.Error()})
			continue
		}
		for _, it := range got {
			it.Trust = Untrusted // UNBYPASSABLE label-at-origin — override whatever the provider set
			if it.Source == "" {
				it.Source = p.Name()
			}
			items = append(items, it)
		}
	}
	return items, errs
}

const defaultTimeout = 30 * time.Second

// kw: http provider name url client fetch body untrusted
type HTTP struct {
	ProviderName string
	URL          string
	Client       *http.Client
}

// kw: http name
func (h HTTP) Name() string { return h.ProviderName }

// kw: http provide get url query param body one item data
func (h HTTP) Provide(ctx context.Context, query string) ([]ContextItem, error) {
	u, err := url.Parse(h.URL)
	if err != nil {
		return nil, fmt.Errorf("provider %s: url: %w", h.ProviderName, err)
	}
	q := u.Query()
	q.Set("q", query) // the query is a PARAMETER, never executed
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("provider %s: request: %w", h.ProviderName, err)
	}
	client := h.Client
	if client == nil {
		client = &http.Client{Timeout: defaultTimeout}
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("provider %s: %w", h.ProviderName, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("provider %s: read: %w", h.ProviderName, err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("provider %s: status %d", h.ProviderName, resp.StatusCode)
	}
	// the body is DATA; Gather stamps it untrusted.
	return []ContextItem{{Source: h.ProviderName, Text: string(body)}}, nil
}

// FromConfig builds a ContextProvider from a stored provider row (name/kind/config — Planning/30).
// Only "http" is wired in v1.1: the KB/embedder lives in a DOWNSTREAM service, so the gate proxies +
// labels + records but holds no model (the deterministic/model-free property survives). "rag" and
// "mcp_resource" are reserved kinds that fail closed — the caller drops such a provider from the
// session (logged), never fabricates one.
func FromConfig(name, kind, config string) (ContextProvider, error) {
	switch kind {
	case "http":
		var c struct {
			URL string `json:"url"`
		}
		if config != "" {
			if err := json.Unmarshal([]byte(config), &c); err != nil {
				return nil, fmt.Errorf("provider %s: config: %w", name, err)
			}
		}
		if c.URL == "" {
			return nil, fmt.Errorf("provider %s: http config needs a url", name)
		}
		return HTTP{ProviderName: name, URL: c.URL}, nil
	default:
		return nil, fmt.Errorf("provider %s: kind %q not supported yet (v1.1: http only)", name, kind)
	}
}
