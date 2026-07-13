package localtools

// file-kw: run local tool exec argv substitute no-shell clean-env timeout truncate

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
)

// maxOutput caps what one tool call returns. Unbounded output floods the model's context (and is a
// cheap denial-of-context), so it is truncated and SAID SO — silently dropping data would let a tool
// hide its own tail.
const maxOutput = 64 * 1024

// Result is one tool run.
// kw: result output exit-code truncated timed-out
type Result struct {
	Output    string
	ExitCode  int
	Truncated bool
	TimedOut  bool
}

// Run executes a declared tool with the model-supplied args.
//
// THE INJECTION DEFENCE IS STRUCTURAL, not a filter. Substitution is PER-ARGV-ELEMENT: a value lands
// inside exactly one element and is never re-split, never re-parsed, never seen by a shell. So a
// `pattern` of `foo; rm -rf /` is searched for, literally — it is one argument to rg, not two commands.
// There is no escaping to get right, because nothing is ever parsed.
// kw: run substitute argv exec clean-env timeout
func (c Config) Run(ctx context.Context, t Tool, args map[string]string) (Result, error) {
	// Every declared, non-optional arg must be supplied. A missing one would substitute to empty and
	// silently run a DIFFERENT command than the operator declared.
	for name, a := range t.Args {
		if _, ok := args[name]; !ok && !a.Optional {
			return Result{}, fmt.Errorf("tool %q: missing required argument %q", t.Name, name)
		}
	}
	// And nothing UNDECLARED may be passed: the model does not get to invent parameters.
	for name := range args {
		if _, ok := t.Args[name]; !ok {
			return Result{}, fmt.Errorf("tool %q: undeclared argument %q", t.Name, name)
		}
	}

	tmpl := t.argvTemplate()
	argv := make([]string, len(tmpl))
	for i, tok := range tmpl {
		argv[i] = substitute(tok, args) // per-element: the value cannot escape its own argv slot
	}

	timeout := t.Timeout
	if timeout <= 0 {
		timeout = c.Timeout
	}
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	rctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// exec.CommandContext, NOT a shell. argv goes to execve as-is.
	cmd := exec.CommandContext(rctx, argv[0], argv[1:]...)
	cmd.Dir = c.workdir(t)
	cmd.Env = c.env() // scrubbed: a local tool never sees the gate's secrets
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out

	err := cmd.Run()

	res := Result{Output: out.String()}
	if len(res.Output) > maxOutput {
		res.Output = res.Output[:maxOutput]
		res.Truncated = true
	}
	if errors.Is(rctx.Err(), context.DeadlineExceeded) {
		res.TimedOut = true
		res.ExitCode = -1
		return res, fmt.Errorf("tool %q timed out after %s", t.Name, timeout)
	}
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		res.ExitCode = ee.ExitCode()
		return res, nil // a non-zero exit is a RESULT, not a failure to run: the agent should see it
	}
	if err != nil {
		return res, fmt.Errorf("tool %q: %w", t.Name, err)
	}
	return res, nil
}

// workdir is the tool's cwd: its own `cwd` resolved under the config Root, or the Root itself.
func (c Config) workdir(t Tool) string {
	if t.Cwd == "" {
		return c.Root
	}
	if filepath.IsAbs(t.Cwd) {
		return t.Cwd
	}
	return filepath.Join(c.Root, t.Cwd)
}

// baseEnv is the minimum a tool needs to find its interpreter and a home.
var baseEnv = []string{"PATH", "HOME", "LANG", "LC_ALL", "TMPDIR"}

// env builds the tool's environment from an ALLOWLIST.
//
// This is not paranoia. The tool server may itself be spawned by the gate over stdio, and the gate's
// process environment holds the control-plane secrets — `dispatch` binds sessions to any recipe,
// `approve` releases held actions. A local tool has no business reading either. So the tool gets PATH,
// HOME, and whatever the OPERATOR explicitly allowed (an API key its command needs), and nothing else.
// kw: env allowlist scrub secrets local tool
func (c Config) env() []string {
	allow := append(slices.Clone(baseEnv), c.EnvAllow...)
	out := make([]string, 0, len(allow))
	for _, kv := range os.Environ() {
		name, _, _ := strings.Cut(kv, "=")
		if slices.Contains(allow, name) {
			out = append(out, kv)
		}
	}
	return out
}

// substitute replaces {name} tokens inside ONE argv element. The result stays one element: a value is
// never split on whitespace, never re-tokenised. This is the whole injection defence.
func substitute(tok string, args map[string]string) string {
	return placeholder.ReplaceAllStringFunc(tok, func(m string) string {
		name := m[1 : len(m)-1]
		if v, ok := args[name]; ok {
			return v
		}
		return "" // optional and unsupplied
	})
}

// Find returns the declared tool by name.
func (c Config) Find(name string) (Tool, bool) {
	for _, t := range c.Tools {
		if t.Name == name {
			return t, true
		}
	}
	return Tool{}, false
}
