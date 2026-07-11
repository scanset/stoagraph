// Command healthcheck is a container liveness probe: GET a URL, exit 0 on 2xx, 1 otherwise.
//
// It exists because the service images are DISTROLESS — no shell, no curl, no wget, nothing to pivot
// to if a binary is ever popped. That is worth keeping, so instead of dropping a shell into the image
// just to run `curl -f`, we ship 3MB of static Go that does exactly one thing.
//
// Every service in this product answers GET /health, unauthenticated, so one probe shape works for
// all of them (a probe has no credential to present, and liveness is not a secret).
package main

// file-kw: container liveness probe static distroless no-shell exit-code

import (
	"fmt"
	"net/http"
	"os"
	"time"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: healthcheck <url>")
		os.Exit(2)
	}
	c := &http.Client{Timeout: 3 * time.Second}
	resp, err := c.Get(os.Args[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, "healthcheck:", err)
		os.Exit(1)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		fmt.Fprintf(os.Stderr, "healthcheck: HTTP %d\n", resp.StatusCode)
		os.Exit(1)
	}
}
