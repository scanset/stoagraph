name: ContextProviderTest
role: test
intent: Verify the read channel: Gather stamps every item untrusted (overriding a provider that claims otherwise), fails open per provider (an erroring provider contributes nothing but is reported, others still run), and the HTTP adapter fetches a URL body as one item. A fuzz drives a provider that claims arbitrary trust/source/text (or errors) and asserts Gather's output is always untrusted with a non-empty source, and an erroring provider yields no items + one error.
api:
  - func TestGatherStampsUntrusted(t *testing.T)
  - func TestGatherFailsOpen(t *testing.T)
  - func TestHTTPAdapter(t *testing.T)
  - func FuzzGatherUntrusted(f *testing.F)
prelude: "A fake provider returns a fixed list of items (or an error) and a Name. The HTTP adapter test uses net/http/httptest to serve a body."
behavior:
  - "GATHER STAMPS UNTRUSTED: two fake providers, one returning an item that CLAIMS Trust \"authoritative\" with an empty Source, the other returning an item with Source set. Gather returns both items in provider order; BOTH have Trust == Untrusted (the authoritative claim is overridden); the first item's Source is the provider's Name (filled because it was empty); the second keeps its Source. Errors is empty."
  - "GATHER FAILS OPEN: three providers - one good (1 item), one that returns an error, one good (1 item). Gather returns 2 items (from the good providers) and one ProviderError naming the failing provider. The failure does not drop the others."
  - "HTTP ADAPTER: an httptest server returns \"runbook text\". HTTP{ProviderName:\"docs\", URL: srv.URL}.Provide(ctx, \"cpu spike\") returns one ContextItem with Source \"docs\" and Text \"runbook text\"; the server saw a GET whose \"q\" query param was \"cpu spike\". A server returning 500 makes Provide return an error (and Gather would skip+report it)."
  - "FUZZ FuzzGatherUntrusted(trust, source, text string, fail bool): a fake provider (Name \"p\") returns, if fail, an error; else one ContextItem{Trust:trust, Source:source, Text:text}. Gather with just this provider. ASSERT: if fail -> 0 items and exactly one ProviderError for \"p\"; else -> exactly one item whose Trust == Untrusted (whatever trust was), whose Source is non-empty (source, or \"p\" if source was empty), and whose Text == text. Never panics. Seed with an authoritative-claim item, an empty-source item, and a failing provider."
constraints: package provider_test (external test); depends on the provider package and stdlib (context, net/http, net/http/httptest, testing). No network beyond httptest.
