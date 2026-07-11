// Command stoagraph is the installer and launcher: one binary that gets you from nothing to a running,
// authenticated gate with a working demo.
//
//	stoagraph up      mint the role secrets, pull the signed images, start, print your tokens
//	stoagraph demo    load the containment demo (no model or API key needed)
//	stoagraph down    stop
//	stoagraph tokens  print the control-plane tokens again
//
// WHY THIS EXISTS AND NOT A `docker run` ONE-LINER: the control plane uses per-role secrets, and
// `approve` — the token that releases a held action — must never reach the orchestrator's environment.
// Something has to mint four distinct secrets and hand each service only what it is entitled to,
// BEFORE anything starts. That is the step a single `docker run` cannot do, and it is exactly the thing
// we deliberately made impossible to shortcut. So the installer does it, and then gets out of the way.
//
// It holds no secrets itself: it writes .env (0600) and never transmits it anywhere.
package main

// file-kw: cli installer launcher up down demo tokens compose ghcr role-secrets mint no-shortcut

import (
	"crypto/rand"
	_ "embed"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// The demo policy is EMBEDDED, not downloaded. `stoagraph demo` must not depend on GitHub being
// reachable — and a demo that dies on a 404 is a first impression you do not get back. These are byte
// copies of examples/pii-demo/recipes/; tools/check.sh fails if they drift, so there is still one
// source of truth.
//
//go:embed recipes/internal_lookup_policy.yaml
var recipeInternalLookup []byte

//go:embed recipes/external_reply_policy.yaml
var recipeExternalReply []byte

// Version is stamped at build time (-ldflags "-X main.Version=v0.1.0"). It pins BOTH the images we pull
// and the compose file we fetch, so an install is reproducible instead of "whatever main looked like".
var Version = "latest"

const repoRaw = "https://raw.githubusercontent.com/scanset/stoagraph"

func main() {
	cmd := "help"
	if len(os.Args) > 1 {
		cmd = os.Args[1]
	}
	var err error
	switch cmd {
	case "up":
		err = up()
	case "down":
		err = compose("down")
	case "demo":
		err = demo()
	case "tokens":
		err = printTokens()
	case "version":
		fmt.Println("stoagraph", Version)
	default:
		usage()
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "stoagraph:", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Print(`stoagraph — verifiable control for AI agents

  stoagraph up      mint role secrets, pull the images, start, print your tokens
  stoagraph demo    load the containment demo (no model or API key needed)
  stoagraph tokens  print the control-plane tokens
  stoagraph down    stop everything
  stoagraph version

Docs: https://github.com/scanset/stoagraph
`)
}

func up() error {
	if _, err := exec.LookPath("docker"); err != nil {
		return fmt.Errorf("docker is required: https://docs.docker.com/get-docker/")
	}

	// 1. the compose file. If you are inside a clone we use yours; otherwise we fetch the one pinned to
	//    THIS binary's version. Nothing is embedded, so there is no copy that can silently drift.
	if _, err := os.Stat("compose.yml"); os.IsNotExist(err) {
		fmt.Printf("== fetching compose.yml (%s) ==\n", Version)
		if err := fetch("compose.yml", "compose.yml"); err != nil {
			return err
		}
	}

	// 2. the four role secrets. This is the step that cannot be skipped: `approve` is minted here and
	//    injected ONLY into the gate, never into the orchestrator.
	if _, err := os.Stat(".env"); os.IsNotExist(err) {
		fmt.Println("== minting control-plane role secrets (.env, 0600) ==")
		if err := genEnv(); err != nil {
			return err
		}
	}

	// `--pull missing` does the right thing in both worlds: a user with no images pulls the signed ones
	// from GHCR; a contributor who just ran `docker compose build` uses what they built. compose.yml is
	// pull-only, and compose.override.yml (present only in a clone) adds the build blocks.
	fmt.Println("== starting (pulling signed images if needed) ==")
	if err := compose("up", "-d", "--pull", "missing"); err != nil {
		return err
	}

	fmt.Print("== waiting for the gate ")
	for i := 0; i < 60; i++ {
		if get("http://localhost:8080/health") == nil {
			break
		}
		fmt.Print(".")
		time.Sleep(time.Second)
	}
	fmt.Println(" ready ==")

	if err := printTokens(); err != nil {
		return err
	}
	fmt.Print(`
  console   http://localhost:3000
  demo      stoagraph demo      (no model or API key needed)
`)
	return nil
}

// demo wires the containment example into the (deliberately empty) gate. The gate ships with NO policy
// — a security control must not arrive already permitting something you never authored — so this is the
// explicit "yes, I meant it" step.
func demo() error {
	tok, err := envToken("STAG_ADMIN_TOKEN")
	if err != nil {
		return err
	}
	if err := get("http://localhost:8080/health"); err != nil {
		return fmt.Errorf("the gate is not up — run: stoagraph up")
	}

	fmt.Println("== policy ==")
	for _, r := range []struct {
		name string
		src  []byte
	}{
		{"internal_lookup_policy", recipeInternalLookup},
		{"external_reply_policy", recipeExternalReply},
	} {
		if err := post("/api/recipes", tok, "text/plain", r.src); err != nil {
			return err
		}
		fmt.Println("  ", r.name)
	}

	fmt.Println("== the tool server ==")
	if err := post("/api/mcp-servers", tok, "application/json",
		[]byte(`{"name":"pii-demo","transport":"http","target":"http://pii-demo:9000/mcp"}`)); err != nil {
		return err
	}

	fmt.Println("== routes ==")
	for _, r := range []string{
		`{"tool":"fetch_user_profile","recipe":"internal_lookup_policy","gateArg":"user_id"}`,
		`{"tool":"send_external_reply","recipe":"external_reply_policy","gateArg":"message_body"}`,
	} {
		if err := post("/api/routes", tok, "application/json", []byte(r)); err != nil {
			return err
		}
	}

	fmt.Print(`
  Try it — no model, no API key:

    fetch_user_profile(123)                          ALLOW   returns Alice's record, INCLUDING her SSN
    send_external_reply("Your SSN is 000-12-3456")   DENY    never reaches the tool
    send_external_reply("Hi Alice, you're unlocked") DENY    <- innocent, and STILL blocked
    send_external_reply("tmpl:account_unlocked")     ALLOW   an approved template

  The third line is the point: no free-form value can cross at all. The policy never scans for SSNs —
  it permits four template ids. There is no clever phrasing that becomes an approved template id, which
  is why a jailbroken model cannot get around it.

  Watch it in the console: http://localhost:3000
`)
	return nil
}

// genEnv mints four INDEPENDENT secrets. Independent is the whole point: compose injects each service
// only the ones it is entitled to, so `approve` is not merely unused by the orchestrator — it is not in
// its environment to be read.
func genEnv() error {
	var b strings.Builder
	b.WriteString("# StoaGraph control-plane secrets. Keep private; rotate by deleting this file.\n")
	for _, k := range []string{"STAG_ADMIN_TOKEN", "STAG_APPROVE_TOKEN", "STAG_DISPATCH_TOKEN", "HARNESS_OPERATOR_TOKEN"} {
		s, err := secret()
		if err != nil {
			return err
		}
		fmt.Fprintf(&b, "%s=%s\n", k, s)
	}
	fmt.Fprintf(&b, "STOAGRAPH_VERSION=%s\n", Version)
	fmt.Fprintf(&b, "HOST_UID=%d\nHOST_GID=%d\n", os.Getuid(), os.Getgid())
	return os.WriteFile(".env", []byte(b.String()), 0o600)
}

func printTokens() error {
	admin, err := envToken("STAG_ADMIN_TOKEN")
	if err != nil {
		return err
	}
	approve, _ := envToken("STAG_APPROVE_TOKEN")
	operator, _ := envToken("HARNESS_OPERATOR_TOKEN")
	fmt.Printf(`
  Paste these into the console sidebar:

    gate token          (admin)     %s
    orchestrator token  (operator)  %s

  Keep this one for when you mean it — it is what RELEASES a held action:

    approve                         %s

  The orchestrator holds neither. It waits on a human; it can never be one.
`, admin, operator, approve)
	return nil
}

func secret() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func envToken(key string) (string, error) {
	b, err := os.ReadFile(".env")
	if err != nil {
		return "", fmt.Errorf("no .env here — run: stoagraph up")
	}
	for _, line := range strings.Split(string(b), "\n") {
		if k, v, ok := strings.Cut(strings.TrimSpace(line), "="); ok && k == key {
			return v, nil
		}
	}
	return "", fmt.Errorf("%s not in .env", key)
}

func compose(args ...string) error {
	c := exec.Command("docker", append([]string{"compose"}, args...)...)
	c.Stdout, c.Stderr = os.Stdout, os.Stderr
	return c.Run()
}

// fetch pulls a file from the repo AT THIS BINARY'S VERSION — never from a moving branch, so an
// install is reproducible and reviewable.
func fetch(path, dst string) error {
	b, err := fetchBytes(path)
	if err != nil {
		return err
	}
	if dir := filepath.Dir(dst); dir != "." {
		_ = os.MkdirAll(dir, 0o755)
	}
	return os.WriteFile(dst, b, 0o644)
}

func fetchBytes(path string) ([]byte, error) {
	// prefer the local copy when run inside a clone
	if b, err := os.ReadFile(path); err == nil {
		return b, nil
	}
	url := fmt.Sprintf("%s/%s/%s", repoRaw, Version, path)
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("fetch %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("fetch %s: HTTP %d", url, resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}

func get(url string) error {
	c := &http.Client{Timeout: 2 * time.Second}
	resp, err := c.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return nil
}

func post(path, token, ctype string, body []byte) error {
	req, err := http.NewRequest("POST", "http://localhost:8080"+path, strings.NewReader(string(body)))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", ctype)
	resp, err := (&http.Client{Timeout: 15 * time.Second}).Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("POST %s: HTTP %d: %s", path, resp.StatusCode, strings.TrimSpace(string(b)))
	}
	return nil
}
