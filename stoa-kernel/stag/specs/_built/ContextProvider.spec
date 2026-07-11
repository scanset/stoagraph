name: ContextProvider
role: component
intent: The READ channel of the dual proxy (Planning/17 refinement, Planning/18): context providers behind one interface, with the load-bearing guarantee that ALL context is stamped untrusted at origin, unbypassably. A ContextProvider yields ContextItems for a query; Gather runs a set of providers and FORCES every returned item's trust to untrusted - a provider CANNOT hand back trusted-looking context (this is what makes context injection labeling unbypassable, per Planning/20). Reads are label + record, not deny: a provider that errors contributes nothing and is reported, but does not fail the gather (the asymmetry from Planning/17 - the value is the mandatory chokepoint, not read-time blocking). One concrete adapter ships: HTTP (fetch a URL). The interface takes RAG/MCP-resource/etc. adapters later, all funnelled through Gather's untrusted stamp.
api:
  - "const Untrusted = \"untrusted\""
  - "type ContextItem struct { Source string; Text string; Trust string; Score float64 }"
  - "type ContextProvider interface { Name() string; Provide(ctx context.Context, query string) ([]ContextItem, error) }"
  - "type ProviderError struct { Provider string; Err string }"
  - func Gather(ctx context.Context, query string, providers []ContextProvider) ([]ContextItem, []ProviderError)
  - "type HTTP struct { ProviderName string; URL string; Client *http.Client }"
  - func (h HTTP) Name() string
  - func (h HTTP) Provide(ctx context.Context, query string) ([]ContextItem, error)
concept: pluggable untrusted context sources behind one interface; Gather stamps every item untrusted (unbypassable label-at-origin); read fail-open per provider (label+record, not deny); the HTTP adapter.
behavior:
  - "GATHER STAMPS UNTRUSTED (unbypassable): Gather calls each provider's Provide(ctx, query) in order and concatenates their items. For EVERY returned item it sets Trust = Untrusted, OVERRIDING whatever the provider set (a provider claiming Trust \"authoritative\" is overridden to untrusted). An item whose Source is empty is given the provider's Name. This is the load-bearing property: no context can reach the caller labeled anything but untrusted."
  - "GATHER FAIL-OPEN PER PROVIDER: a provider whose Provide returns an error contributes NO items and is recorded in the returned []ProviderError{Provider: its Name, Err: the reason}; the remaining providers still run and their items are still gathered. A missing/failing context source is not a security failure (reads are label+record); Gather never panics, and it returns whatever items it could gather plus the errors."
  - "HTTP ADAPTER: HTTP.Name() returns ProviderName. HTTP.Provide(ctx, query) issues a GET to URL with the query as a \"q\" parameter; on a 2xx it returns exactly one ContextItem{Source: ProviderName, Text: the response body} (Trust is set by Gather); a non-2xx status or a transport error returns a non-nil error (that provider is then skipped by Gather, reported). The query is a query PARAMETER and the body is DATA - neither is executed."
  - "DETERMINISTIC: Gather over deterministic providers returns the same items and errors; it performs no I/O of its own beyond delegating to the providers."
constraints: package provider at workspaces/stag/provider (public; import path github.com/scanset/StAG/provider). Depends on stdlib only (context, net/http, net/url, io, fmt, time). No MCP dependency (an MCP-resource provider is a later adapter, quarantined). The untrusted stamp is enforced in Gather, not trusted to the providers.
