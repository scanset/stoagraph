package kb_test

import (
	"context"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/scanset/stoagraph/stoa-kernel/harness/kb"
)

// fakeEmbedder: explicit vecs for named texts, deterministic fallback otherwise.
type fakeEmbedder struct{ m map[string][]float32 }

func (f fakeEmbedder) Embed(_ context.Context, text string) ([]float32, error) {
	if v, ok := f.m[text]; ok {
		return v, nil
	}
	v := make([]float32, 4)
	for i, b := range []byte(text) {
		v[i%4] += float32(b)
	}
	return v, nil
}

func fakeAPI(t *testing.T, status int, body string) (*httptest.Server, *[]byte, *string) {
	t.Helper()
	var last []byte
	var auth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		last, _ = io.ReadAll(r.Body)
		auth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = io.WriteString(w, body)
	}))
	t.Cleanup(srv.Close)
	return srv, &last, &auth
}

func TestOllamaEmbedder(t *testing.T) {
	srv, last, auth := fakeAPI(t, 200, `{"object":"list","data":[{"object":"embedding","embedding":[0.1,0.2,0.3],"index":0}],"model":"nomic-embed-text"}`)
	e := kb.OllamaEmbedder{BaseURL: srv.URL + "/v1", APIKey: "k", Model: "nomic-embed-text"}
	v, err := e.Embed(context.Background(), "x")
	if err != nil {
		t.Fatal(err)
	}
	if len(v) != 3 || v[0] != 0.1 || v[2] != 0.3 {
		t.Errorf("vec: %v", v)
	}
	body := string(*last)
	if !strings.Contains(body, "nomic-embed-text") || !strings.Contains(body, `"x"`) {
		t.Errorf("body: %s", body)
	}
	if *auth != "Bearer k" {
		t.Errorf("auth: %q", *auth)
	}
	// fail closed
	for _, tc := range []struct {
		status int
		body   string
	}{{500, `{}`}, {200, `{"data":[]}`}, {200, `not json`}} {
		s, _, _ := fakeAPI(t, tc.status, tc.body)
		if _, err := (kb.OllamaEmbedder{BaseURL: s.URL + "/v1", Model: "m"}).Embed(context.Background(), "x"); err == nil {
			t.Errorf("expected error for %d %q", tc.status, tc.body)
		}
	}
	// empty key -> no Authorization
	s2, _, a2 := fakeAPI(t, 200, `{"data":[{"embedding":[1]}]}`)
	if _, err := (kb.OllamaEmbedder{BaseURL: s2.URL + "/v1", Model: "m"}).Embed(context.Background(), "x"); err != nil {
		t.Fatal(err)
	}
	if *a2 != "" {
		t.Errorf("empty key should send no auth: %q", *a2)
	}
}

func TestChunk(t *testing.T) {
	docs := kb.Chunk("run.md", "pre\n## A\nbody a\n## B\nbody b\n")
	if len(docs) != 3 {
		t.Fatalf("want 3 chunks, got %d: %+v", len(docs), docs)
	}
	if docs[0].ID != "run.md#0" || !strings.HasPrefix(docs[0].Text, "pre") ||
		docs[1].ID != "run.md#1" || !strings.Contains(docs[1].Text, "## A") ||
		docs[2].Source != "run.md" || !strings.Contains(docs[2].Text, "body b") {
		t.Errorf("chunks: %+v", docs)
	}
	if n := len(kb.Chunk("x", "no headers here")); n != 1 {
		t.Errorf("no-header: want 1, got %d", n)
	}
	if n := len(kb.Chunk("x", "   \n\n")); n != 0 {
		t.Errorf("blank: want 0, got %d", n)
	}
	if d := kb.Chunk("x", "## only\nbody"); len(d) != 1 || d[0].ID != "x#0" {
		t.Errorf("leading-header: %+v", d)
	}
}

func TestRetrieveRanking(t *testing.T) {
	ctx := context.Background()
	fe := fakeEmbedder{m: map[string][]float32{"q": {1, 0, 0}}}
	docs := []kb.Doc{
		{ID: "a", Source: "s", Text: "a", Vec: []float32{1, 0, 0}},
		{ID: "b", Source: "s", Text: "b", Vec: []float32{0, 1, 0}},
		{ID: "c", Source: "s", Text: "c", Vec: []float32{0.9, 0.1, 0}},
	}
	s := kb.NewStore(fe, docs)

	got, err := s.Retrieve(ctx, "q", 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].ID != "a" || got[1].ID != "c" {
		t.Fatalf("ranking: %+v", got)
	}
	if !(got[0].Score >= got[1].Score) || got[0].Score < 0.99 {
		t.Errorf("scores: %v %v", got[0].Score, got[1].Score)
	}
	if r0, _ := s.Retrieve(ctx, "q", 0); len(r0) != 0 {
		t.Errorf("k=0 should be empty")
	}
	if all, _ := s.Retrieve(ctx, "q", 99); len(all) != 3 {
		t.Errorf("k>size should return all")
	}
	if g2, _ := s.Retrieve(ctx, "q", 2); !reflect.DeepEqual(got, g2) {
		t.Errorf("nondeterministic retrieve")
	}
	// embedder error on the query -> fail closed
	errStore := kb.NewStore(errEmbedder{}, docs)
	if _, err := errStore.Retrieve(ctx, "q", 1); err == nil {
		t.Errorf("embedder error should fail closed")
	}
}

type errEmbedder struct{}

func (errEmbedder) Embed(context.Context, string) ([]float32, error) {
	return nil, io.ErrUnexpectedEOF
}

func TestLoadDir(t *testing.T) {
	dir := t.TempDir()
	write := func(name, body string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write("r1.md", "## X\nrestart the pool\n## Y\nscale up\n")
	write("r2.md", "## Z\nisolate the host\n## W\nrollback\n")
	write("ignore.txt", "not markdown")

	s, err := kb.LoadDir(context.Background(), dir, fakeEmbedder{})
	if err != nil {
		t.Fatal(err)
	}
	got, err := s.Retrieve(context.Background(), "anything", 99)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 4 {
		t.Errorf("want 4 chunks from 2 md files, got %d", len(got))
	}
	for _, d := range got {
		if len(d.Vec) == 0 {
			t.Errorf("doc has no vec: %+v", d)
		}
	}
	// nonexistent dir -> error
	if _, err := kb.LoadDir(context.Background(), filepath.Join(dir, "nope"), fakeEmbedder{}); err == nil {
		t.Errorf("nonexistent dir should error")
	}
	// empty dir -> empty store, no error
	empty, err := kb.LoadDir(context.Background(), t.TempDir(), fakeEmbedder{})
	if err != nil {
		t.Fatal(err)
	}
	if r, _ := empty.Retrieve(context.Background(), "q", 5); len(r) != 0 {
		t.Errorf("empty dir should yield empty store")
	}
}

func FuzzCosine(f *testing.F) {
	f.Add([]byte{1, 0, 0, 0, 1, 0, 0, 0}, []byte{1, 0, 0, 0, 1, 0, 0, 0})
	f.Add([]byte{1, 0, 0, 0}, []byte{0, 0, 0, 0})
	f.Add([]byte{1, 0, 0, 0}, []byte{0, 0, 0, 0, 1, 0, 0, 0})
	f.Fuzz(func(t *testing.T, ab, bb []byte) {
		a := toVec(ab)
		b := toVec(bb)
		got := kb.Cosine(a, b)
		if math.IsNaN(got) || math.IsInf(got, 0) {
			t.Fatalf("cosine not finite: %v", got)
		}
		if got < -1.0001 || got > 1.0001 {
			t.Errorf("cosine out of range: %v", got)
		}
		if r := kb.Cosine(b, a); math.Abs(r-got) > 1e-6 {
			t.Errorf("asymmetric: %v vs %v", got, r)
		}
		if len(a) != len(b) {
			if got != 0 {
				t.Errorf("length mismatch should be 0, got %v", got)
			}
		}
		if norm(a) == 0 && got != 0 {
			t.Errorf("zero norm should be 0, got %v", got)
		}
		if len(a) > 0 && norm(a) > 0 {
			if s := kb.Cosine(a, a); math.Abs(s-1) > 1e-4 {
				t.Errorf("cosine(a,a)=%v, want 1", s)
			}
		}
	})
}

func toVec(b []byte) []float32 {
	v := make([]float32, len(b))
	for i, x := range b {
		v[i] = float32(int8(x))
	}
	return v
}

func norm(v []float32) float64 {
	var s float64
	for _, x := range v {
		s += float64(x) * float64(x)
	}
	return s
}
