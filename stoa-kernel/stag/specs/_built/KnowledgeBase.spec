name: KnowledgeBase
role: component
intent: The embedding knowledge base (runtime, unit 2) - basic RAG retrieval over markdown runbooks. It embeds documents and a query with an Embedder (ollama nomic-embed via /v1/embeddings) and returns the top-k most similar docs by cosine similarity. It is a pure RETRIEVAL utility: it makes NO trust decision and NO gate decision. The docs it returns are UNTRUSTED enrichment - the bind unit (next) places them in the model's Input as labeled data, and the gate handles trust downstream; the KB just finds relevant text. In-memory at demo scale (embed-on-load, cosine over a slice); a persistent cache and a real vector DB are later swaps behind the Store interface (basic minimum now, complexity layered in). Hand-rolled over stdlib - no dependency; fail-closed on embedder/read errors.
api:
  - "type Embedder interface { Embed(ctx context.Context, text string) ([]float32, error) }"
  - "type OllamaEmbedder struct { BaseURL string; APIKey string; Model string; HTTP *http.Client }"
  - func (e OllamaEmbedder) Embed(ctx context.Context, text string) ([]float32, error)
  - "type Doc struct { ID string; Source string; Text string; Vec []float32; Score float64 }"
  - "type MemStore struct { ... }   // in-memory Store; safe for concurrent Retrieve"
  - func NewStore(embed Embedder, docs []Doc) *MemStore
  - func LoadDir(ctx context.Context, dir string, embed Embedder) (*MemStore, error)
  - func (s *MemStore) Retrieve(ctx context.Context, query string, k int) ([]Doc, error)
  - func Chunk(source string, content string) []Doc
  - func Cosine(a []float32, b []float32) float64
concept: RAG untrusted enrichment retrieval; cosine similarity; top-k; markdown chunking; the KB never decides, it retrieves.
behavior:
  - "OLLAMAEMBEDDER: Embed POSTs to BaseURL (trailing slash trimmed) + \"/embeddings\" the body {model: e.Model, input: text}; header Content-Type application/json and Authorization: Bearer <APIKey> only when APIKey is non-empty (ollama ignores it). On a 2xx it returns data[0].embedding as []float32. FAIL CLOSED: a non-2xx status, transport error, context cancellation, undecodable body, or an empty data array returns (nil, error) - never a fabricated vector."
  - "CHUNK: Chunk(source, content) splits markdown into Docs by lines beginning with \"## \" - the preamble before the first header (if non-blank) is one chunk, and each \"## \" starts a new chunk that includes its header line through the text before the next header. Each Doc has ID = source + \"#\" + its index, Source = source, Text = the chunk verbatim (no trimming beyond trailing blank lines), and no Vec yet. Blank/whitespace-only input yields no chunks."
  - "LOADDIR: LoadDir reads every *.md file under dir (sorted for determinism), Chunks each, embeds each chunk's Text via embed (setting Doc.Vec), and returns a MemStore over the embedded docs. A read error or an embed error returns (nil, error) - fail closed, no partial store. An empty dir yields an empty store (not an error)."
  - "COSINE: Cosine(a, b) = dot(a,b) / (|a| * |b|) as a float64. If the lengths differ, or either vector has zero norm, it returns 0 (no similarity) - never NaN, never a panic. The result lies in [-1, 1] (within float rounding). Cosine(a, a) == 1 for any non-zero a; Cosine is symmetric."
  - "RETRIEVE: Retrieve(ctx, query, k) embeds the query via the store's Embedder, scores every stored Doc by Cosine(queryVec, doc.Vec), and returns the top-k Docs sorted by DESCENDING score, each a copy with Score set. k <= 0 returns an empty slice; k greater than the store size returns all docs ranked. Ties break by ascending Doc.ID (deterministic order). An embedder error returns (nil, error) - fail closed. Retrieve makes no trust judgment; the returned docs are untrusted."
  - "DETERMINISTIC: with a fixed Embedder, LoadDir builds an equal store and Retrieve returns an equal ranking for the same query every call; the KB adds no nondeterminism of its own."
constraints: package kb at workspaces/stag/kb (public; import path github.com/scanset/StAG/kb). Depends on stdlib ONLY (net/http, encoding/json, bytes, context, fmt, io, math, os, path/filepath, sort, strings, time). No third-party dependency; no kernel/model/broker dependency.
