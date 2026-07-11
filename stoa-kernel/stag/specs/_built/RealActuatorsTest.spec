name: RealActuatorsTest
role: test
intent: Verify the real actuators execute a gated value safely and fail closed. Prove Command execs the value as one discrete argv element with NO shell interpretation (shell metacharacters cause no side effect and arrive as one literal argument), surfaces a non-zero exit as an error, and honors a timeout; prove HTTP posts the value as a JSON body field (never the URL/header), returns the body on 2xx, and fails closed on non-2xx. A fuzz proves Argv places any value as exactly the trailing element. A self-exec helper (TestMain) stands in for a real program so the tests need no external binaries.
api:
  - func TestMain(m *testing.M)
  - func TestCommandRuns(t *testing.T)
  - func TestCommandFailsClosed(t *testing.T)
  - func TestCommandNoShell(t *testing.T)
  - func TestCommandTimeout(t *testing.T)
  - func TestHTTPPostsValue(t *testing.T)
  - func TestHTTPFailsClosed(t *testing.T)
  - func FuzzArgv(f *testing.F)
prelude: "TestMain re-execs the test binary as a helper when an env var is set: an 'echo' mode prints os.Args[1:] as JSON to stdout and exits 0; a 'fail' mode prints and exits non-zero; a 'sleep' mode sleeps well beyond any test timeout. Command actuators in the tests point Path at os.Args[0] with the helper mode selected via Command.Env, so no external programs are needed. HTTP tests use net/http/httptest."
behavior:
  - "COMMAND RUNS: a Command with Path os.Args[0] and Env selecting echo mode, fired with value \"scale_up\", returns a nil error and output whose decoded argv has \"scale_up\" as its final element."
  - "COMMAND FAILS CLOSED: a Command in fail mode (exit non-zero) returns a NON-nil error; the captured output is still returned. A Command with a non-existent Path also returns a non-nil error."
  - "COMMAND NO SHELL: fire the echo-mode helper with value \"; touch SENTINEL; echo $(whoami)\" (SENTINEL a temp path). Assert (1) the decoded argv's final element equals the value BYTE-FOR-BYTE (one argument, not split on spaces or ;), and (2) the SENTINEL file was NOT created — the shell metacharacters were never interpreted. Repeat for a value with a newline and a value with backticks."
  - "COMMAND TIMEOUT: a Command in sleep mode with Timeout 100ms returns a non-nil error well before the sleep would finish (context deadline), and does not hang."
  - "HTTP POSTS VALUE: an httptest.Server records the request. HTTP.Fire(ctx, \"restart_service\") makes a POST with Content-Type application/json whose body JSON-decodes to {\"action\":\"restart_service\"}; the URL path/query contain no part of the value. The server returns 200 with a body; Fire returns that body and a nil error."
  - "HTTP FAILS CLOSED: a server returning 500 makes Fire return a non-nil error (with the body); a request to an unreachable URL returns a non-nil error. A 2xx is the only success."
  - "FUZZ FuzzArgv(nArgs uint8, value string): build nArgs%8 fixed args; Argv(args, value) has length len(args)+1, its final element equals value exactly, and its first len(args) elements equal args in order. The value never lands anywhere but the final position and is never altered. Seed with empty args, several args, and values containing spaces/semicolons/quotes."
constraints: package actuator_test (external test); depends on the actuator package and stdlib (context, encoding/json, net/http, net/http/httptest, os, path/filepath, strings, testing, time). No third-party deps, no real external programs (the test binary self-execs as the helper).
