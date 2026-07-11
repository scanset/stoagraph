name: KnowledgeBaseTest
role: test
intent: Verify RAG retrieval end to end with a deterministic fake Embedder (so cosine ranking is predictable), plus the OllamaEmbedder against a fake /v1/embeddings server, plus the pure helpers (Chunk, Cosine) by table and fuzz. Prove: retrieval ranks by descending cosine and is deterministic; the embedder fails closed; chunking splits markdown by section; cosine is bounded and never panics. The KB makes no trust decision - that is asserted only insofar as Retrieve returns docs untouched.
api:
  - func TestOllamaEmbedder(t *testing.T)
  - func TestChunk(t *testing.T)
  - func TestRetrieveRanking(t *testing.T)
  - func TestLoadDir(t *testing.T)
  - func FuzzCosine(f *testing.F)
prelude: "A fakeEmbedder maps specific texts to explicit vectors and falls back to a deterministic vector from the text bytes for any other text (so LoadDir embeds arbitrary chunks). A fakeAPI(t, status, body) serves an /v1/embeddings response."
behavior:
  - "OLLAMAEMBEDDER: a fake server returns 200 with data[0].embedding [0.1,0.2,0.3]; Embed(ctx, \"x\") returns that []float32; the recorded request body contains the model and the input \"x\", and Authorization is \"Bearer k\" when APIKey set, absent when empty. FAIL CLOSED: a 500 status, an empty data array ({\"data\":[]}), and a non-JSON body each return (nil, non-nil error)."
  - "CHUNK: Chunk(\"run.md\", \"pre\\n## A\\nbody a\\n## B\\nbody b\\n\") returns 3 Docs - a preamble chunk \"pre\", \"## A\\nbody a\", \"## B\\nbody b\" - with IDs run.md#0/#1/#2 and Source run.md. Content with no headers is a single chunk; blank content yields zero chunks; content beginning with a header yields no empty preamble chunk."
  - "RETRIEVE RANKING: NewStore(fakeEmbedder, docs) with docs A=[1,0,0], B=[0,1,0], C=[0.9,0.1,0] (ids a,b,c) and the query mapped to [1,0,0]. Retrieve(ctx, query, 2) returns [A, C] in that order (Cosine 1.0 then ~0.994), each with Score set descending; B (Cosine 0) is excluded. Retrieve(query, 0) is empty; Retrieve(query, 99) returns all three ranked; a second identical Retrieve is equal (determinism). An embedder that errors on the query makes Retrieve return (nil, error)."
  - "LOADDIR: a temp dir with two .md files, each with two ## sections, LoadDir(ctx, dir, fakeEmbedder) returns a store whose Retrieve over a query returns docs drawn from those files, every returned Doc has a non-empty Vec, and the doc count equals the total chunk count. A dir with an unreadable file (or LoadDir on a nonexistent dir) returns (nil, error). An empty dir yields an empty store with no error."
  - "FUZZ FuzzCosine: from fuzzed bytes build two float32 vectors; assert Cosine never panics; the result is in [-1-eps, 1+eps]; Cosine(a,b)==Cosine(b,a) (symmetry); for a non-zero a, Cosine(a,a) is within eps of 1; a zero-norm or length-mismatched pair returns exactly 0. Seed with equal vectors, orthogonal vectors, a zero vector, and mismatched lengths."
constraints: package kb_test (external test); depends on the kb package, net/http, net/http/httptest, context, math, os, path/filepath, reflect, strings, testing.
