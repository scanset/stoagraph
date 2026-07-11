name: StoreTest
role: test
intent: Verify the SQLite config store: MCP-server CRUD with atomic tool replacement, provider and route CRUD, absent-row fail-closed behaviour, durability across re-open, and re-init from a fresh file. A fuzz drives arbitrary strings (SQL metacharacters, unicode, NUL) through a Put/Get round-trip, proving parameterized queries are injection-safe and byte-faithful.
api:
  - func TestServerCRUD(t *testing.T)
  - func TestProviderAndRouteCRUD(t *testing.T)
  - func TestAbsentFailsClosed(t *testing.T)
  - func TestDurabilityAndReInit(t *testing.T)
  - func FuzzRoundTrip(f *testing.F)
prelude: "Helpers open a Store on a temp-file path (t.TempDir) or on \":memory:\" for the fuzz. A sample MCPServer with two tools, a ContextProvider, and a Route are constructed for the CRUD tests."
behavior:
  - "SERVER CRUD: Open a temp store; PutMCPServer a server with two tools; GetMCPServer returns an equal server (name/transport/target/enabled and both tools, tools ordered by name); ListMCPServers returns it. Put the SAME server again with ONE tool; Get now returns exactly one tool (the tool set was REPLACED, not appended). DeleteMCPServer removes it; List is empty and Get returns a not-found error."
  - "PROVIDER + ROUTE CRUD: PutProvider then ListProviders returns it; DeleteProvider empties it. PutRoute{Tool: write_note, Recipe: write_note_policy, GateArg: text}; ListRoutes returns it; a second PutRoute with the same Tool but a different Recipe REPLACES it (one route per tool); DeleteRoute(write_note) empties it."
  - "ABSENT FAILS CLOSED: GetMCPServer(\"nope\") returns a non-nil error and a zero server (not a false success). Operations after Close return a non-nil error."
  - "DURABILITY + RE-INIT: Put a server, Close, re-Open the SAME path -> the server is still there (durable). Then simulate re-init: remove the DB file and Open the path fresh -> the store is empty (the DDL recreated a clean schema; recollecting data is fine, no migration)."
  - "FUZZ FuzzRoundTrip(name, target, tool string): open a fresh in-memory store; skip the empty-name case (name is the PK). PutMCPServer{Name:name, Transport:\"stdio\", Target:target, Tools:[{Name:tool}]} then GetMCPServer(name). ASSERT: no error; the returned server's Target equals target byte-for-byte and its single tool's Name equals tool byte-for-byte - arbitrary strings (quotes, semicolons, \"'; DROP TABLE mcp_server; --\", unicode, NUL) round-trip as data and never execute (the mcp_server table still exists and holds exactly one row). Never panics. Seed with a normal server, an injection string, and unicode."
constraints: package store_test (external test); depends on the store package and stdlib (context, os, path/filepath, reflect, testing). No network. The fuzz uses \":memory:\".
