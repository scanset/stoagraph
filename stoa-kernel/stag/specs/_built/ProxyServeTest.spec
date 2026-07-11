name: ProxyServeTest
role: test
intent: Verify the HTTP API over the gating proxy: a well-formed decide gates and returns a DecisionView; a malformed body / empty tool / wrong method fails closed without invoking the gate; an unknown tool is denied (200, verdict deny); log returns the verify status; policies and health respond; CORS + JSON always. A fuzz drives arbitrary request bodies at /api/decide and asserts the handler never panics, always returns valid JSON, and invokes the gate ONLY for a well-formed decide body.
api:
  - func TestDecideAllowed(t *testing.T)
  - func TestDecideUnknownToolDenied(t *testing.T)
  - func TestDecideFailsClosed(t *testing.T)
  - func TestLogEndpoint(t *testing.T)
  - func TestPoliciesAndHealth(t *testing.T)
  - func FuzzDecideHandler(f *testing.F)
prelude: "A helper builds a serve.Server whose Gate has a write_note policy (text gated against a set allowing {hello,...}) recorded to an egress.JSONLSink over a temp file. Requests go through httptest to Server.Handler(). A countingSink or a spy on the gate is not needed; the gate is already fuzzed - the test asserts the HTTP layer's decode/shape/fail-closed behavior."
behavior:
  - "DECIDE ALLOWED: POST /api/decide {tool: write_note, args:{text: hello}} returns 200, Content-Type application/json, a DecisionView with Verdict allow, Forward true, Value hello, RuleFired non-empty, SubjectClass untrusted, Chain.Decide == allow and Chain.Act == ok, and a non-empty Events."
  - "DECIDE UNKNOWN TOOL DENIED: POST /api/decide {tool: delete_everything, args:{}} returns 200 with Verdict deny, Forward false, Chain.Act == skip - an unknown tool is a valid request that the gate denies, not an HTTP error."
  - "DECIDE FAILS CLOSED: POST /api/decide with a non-JSON body returns 400 and a JSON error; POST with {args:{}} (empty tool) returns 400; a GET to /api/decide returns 405. In each case the body is valid JSON and no panic occurs."
  - "LOG: after a couple of allowed decides, GET /api/log returns 200 with LogView.Verify.Count > 0, a non-empty Head, and no Error; with a signing public key set and a checkpoint present, Verify.Signed and Verify.Verified are true. With no log file, GET /api/log returns 200 and Verify.Count 0."
  - "POLICIES + HEALTH: GET /api/policies returns 200 with the write_note policy listed (Tool write_note); GET /api/health returns 200 with ok true. An unknown path returns 404 with a JSON body. An OPTIONS preflight to /api/decide returns 204 with Access-Control-Allow-Origin set."
  - "FUZZ FuzzDecideHandler(body []byte): POST the fuzzed bytes to /api/decide via httptest. ASSERT: (1) the handler always writes a response with a status code and never panics; (2) the response body is valid JSON (or empty); (3) the gate is invoked ONLY when the body JSON-decodes to an object with a non-empty tool - parse the body the same way and, when it is not a well-formed decide, assert the status is 400 (fail closed); (4) a 200 response always carries a DecisionView whose Verdict is one of allow/deny/escalate. Seed with a valid request, a malformed body, an empty-tool object, and non-object JSON."
constraints: package serve_test (external test); depends on the serve package, the proxy package, the recipe package (Parse), the egress package (NewJSONLSink), the stag root, and stdlib (bytes, encoding/json, net/http, net/http/httptest, os, path/filepath, testing). No MCP dependency, no network.
