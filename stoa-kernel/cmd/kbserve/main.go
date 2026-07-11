// Command kbserve is the downstream context provider for the k8s demo (Planning/30). It embeds a
// markdown KB once (via ollama) and answers GET /context?q=<query> with the top-k relevant facts as
// plain text. stag's `http` context provider proxies THIS service and stamps the body untrusted — so
// the EMBEDDER lives here, in a downstream service the operator runs, and the gate stays deterministic
// and model-free. This is product/demo infra, NOT part of stag.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/scanset/stoagraph/stoa-kernel/harness/kb"
)

func main() {
	addr := flag.String("addr", ":8095", "listen address")
	kbDir := flag.String("kb-dir", "examples/k8s/kb", "markdown KB directory of infra facts")
	ollama := os.Getenv("OLLAMA_URL")
	if ollama == "" {
		ollama = "http://172.18.160.1:11434" // WSL2 host gateway (ollama is not on localhost here)
	}
	embedBase := flag.String("embed-base", ollama+"/v1", "OpenAI-compatible base URL for the embedder")
	embedModel := flag.String("embed-model", "nomic-embed-text", "embedding model")
	topK := flag.Int("k", 3, "number of facts to return per query")
	flag.Parse()

	ctx := context.Background()
	embed := kb.OllamaEmbedder{BaseURL: *embedBase, Model: *embedModel, HTTP: &http.Client{Timeout: 30 * time.Second}}
	store, err := kb.LoadDir(ctx, *kbDir, embed)
	if err != nil {
		log.Fatalf("kbserve: load %s: %v", *kbDir, err)
	}
	log.Printf("kbserve on %s — KB %q embedded (model %s), returning top-%d per query", *addr, *kbDir, *embedModel, *topK)

	mux := http.NewServeMux()
	// GET /context?q= — the READ endpoint stag proxies. Returns plain text; stag wraps the whole body
	// as ONE untrusted context item (source = the provider name), so provenance rides in the headers.
	mux.HandleFunc("GET /context", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query().Get("q")
		docs, err := store.Retrieve(r.Context(), q, *topK)
		if err != nil {
			http.Error(w, "retrieve: "+err.Error(), http.StatusInternalServerError)
			return
		}
		var b strings.Builder
		fmt.Fprintf(&b, "# k8s infra facts — %d relevant to this incident\n", len(docs))
		for _, d := range docs {
			fmt.Fprintf(&b, "\n## %s\n%s\n", d.ID, strings.TrimSpace(d.Text))
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte(b.String()))
	})
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprintln(w, "ok")
	})
	log.Fatal(http.ListenAndServe(*addr, mux))
}
