// Package kb is the embedding knowledge base: basic RAG retrieval over markdown
// runbooks. It embeds docs + a query and returns the top-k by cosine similarity.
// A pure retrieval utility — it makes no trust or gate decision; the docs it
// returns are untrusted enrichment. Hand-rolled over stdlib; no dependency.
package kb

// file-kw: kb rag embedding retrieval cosine topk markdown chunk ollama untrusted enrichment fail-closed

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// kw: embedder embed text to vector
type Embedder interface {
	Embed(ctx context.Context, text string) ([]float32, error)
}

// kw: ollama embedder endpoint model key
type OllamaEmbedder struct {
	BaseURL string
	APIKey  string
	Model   string
	HTTP    *http.Client
}

// kw: embed call embeddings api fail-closed
func (e OllamaEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	body, err := json.Marshal(struct {
		Model string `json:"model"`
		Input string `json:"input"`
	}{Model: e.Model, Input: text})
	if err != nil {
		return nil, fmt.Errorf("embed: marshal: %w", err)
	}
	url := strings.TrimRight(e.BaseURL, "/") + "/embeddings"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("embed: request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if e.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+e.APIKey)
	}
	hc := e.HTTP
	if hc == nil {
		hc = &http.Client{Timeout: 60 * time.Second}
	}
	resp, err := hc.Do(req)
	if err != nil {
		return nil, fmt.Errorf("embed: %w", err) // fail closed
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("embed: read: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("embed: status %d", resp.StatusCode)
	}
	var er struct {
		Data []struct {
			Embedding []float32 `json:"embedding"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &er); err != nil {
		return nil, fmt.Errorf("embed: decode: %w", err)
	}
	if len(er.Data) == 0 || len(er.Data[0].Embedding) == 0 {
		return nil, fmt.Errorf("embed: empty embedding")
	}
	return er.Data[0].Embedding, nil
}

// kw: doc id source text vec score
type Doc struct {
	ID     string
	Source string
	Text   string
	Vec    []float32
	Score  float64
}

// kw: mem store embedder docs
type MemStore struct {
	embed Embedder
	docs  []Doc
}

// kw: new store pre-embedded docs
func NewStore(embed Embedder, docs []Doc) *MemStore {
	return &MemStore{embed: embed, docs: docs}
}

// kw: load dir markdown chunk embed fail-closed
func LoadDir(ctx context.Context, dir string, embed Embedder) (*MemStore, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("kb: %w", err)
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".md") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names) // deterministic order
	var docs []Doc
	for _, name := range names {
		path := filepath.Join(dir, name)
		content, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("kb: %w", err)
		}
		for _, d := range Chunk(name, string(content)) {
			vec, err := embed.Embed(ctx, d.Text)
			if err != nil {
				return nil, fmt.Errorf("kb: embed %s: %w", d.ID, err) // fail closed
			}
			d.Vec = vec
			docs = append(docs, d)
		}
	}
	return &MemStore{embed: embed, docs: docs}, nil
}

// kw: retrieve embed query cosine topk fail-closed
func (s *MemStore) Retrieve(ctx context.Context, query string, k int) ([]Doc, error) {
	if k <= 0 {
		return nil, nil
	}
	qv, err := s.embed.Embed(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("kb: %w", err) // fail closed
	}
	scored := make([]Doc, len(s.docs))
	for i, d := range s.docs {
		d.Score = Cosine(qv, d.Vec)
		scored[i] = d
	}
	sort.SliceStable(scored, func(i, j int) bool {
		if scored[i].Score != scored[j].Score {
			return scored[i].Score > scored[j].Score // descending
		}
		return scored[i].ID < scored[j].ID // tie-break deterministic
	})
	if k > len(scored) {
		k = len(scored)
	}
	return scored[:k], nil
}

// kw: chunk markdown by section
func Chunk(source string, content string) []Doc {
	if strings.TrimSpace(content) == "" {
		return nil
	}
	lines := strings.Split(content, "\n")
	var chunks []string
	var cur []string
	flush := func() {
		text := strings.TrimRight(strings.Join(cur, "\n"), "\n")
		if strings.TrimSpace(text) != "" {
			chunks = append(chunks, text)
		}
		cur = nil
	}
	for _, ln := range lines {
		if strings.HasPrefix(ln, "## ") {
			flush()
		}
		cur = append(cur, ln)
	}
	flush()
	docs := make([]Doc, len(chunks))
	for i, c := range chunks {
		docs[i] = Doc{ID: fmt.Sprintf("%s#%d", source, i), Source: source, Text: c}
	}
	return docs
}

// kw: cosine similarity bounded no-nan
func Cosine(a []float32, b []float32) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	var dot, na, nb float64
	for i := range a {
		x, y := float64(a[i]), float64(b[i])
		dot += x * y
		na += x * x
		nb += y * y
	}
	if na == 0 || nb == 0 {
		return 0
	}
	c := dot / (math.Sqrt(na) * math.Sqrt(nb))
	if c > 1 {
		c = 1
	} else if c < -1 {
		c = -1
	}
	return c
}
