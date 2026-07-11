name: Store
role: component
intent: The SQLite config store for the admin console's Adapters (Planning/18) - the persisted, RELATIONAL config that the file-based recipes bind to. It holds MCP servers (+ their discovered tools), context providers, and routes (tool -> recipe -> gated arg - the tool/recipe bindings the live gate builds its router from). Persistence only: typed, fail-closed CRUD over a single SQLite database, with the whole schema in ONE embedded DDL file. NO MIGRATIONS (project rule): to change the schema, edit the DDL and re-init (remove the DB file); recollecting data is fine. Uses modernc.org/sqlite (pure Go, no cgo), QUARANTINED to this package - the kernel/broker/gate never import a DB driver. All queries are parameterized, so arbitrary strings (SQL metacharacters, quotes, unicode) are inert data, never executed.
api:
  - "type MCPServer struct { Name string; Transport string; Target string; Enabled bool; Tools []MCPTool }"
  - "type MCPTool struct { Server string; Name string; InputSchema string }"
  - "type ContextProvider struct { Name string; Kind string; Config string; Enabled bool }"
  - "type Route struct { Tool string; Recipe string; GateArg string }"
  - "type Store struct { ... unexported db ... }"
  - func Open(path string) (*Store, error)
  - func (s *Store) Close() error
  - func (s *Store) PutMCPServer(ctx context.Context, srv MCPServer) error
  - func (s *Store) GetMCPServer(ctx context.Context, name string) (MCPServer, error)
  - func (s *Store) ListMCPServers(ctx context.Context) ([]MCPServer, error)
  - func (s *Store) DeleteMCPServer(ctx context.Context, name string) error
  - func (s *Store) PutProvider(ctx context.Context, p ContextProvider) error
  - func (s *Store) ListProviders(ctx context.Context) ([]ContextProvider, error)
  - func (s *Store) DeleteProvider(ctx context.Context, name string) error
  - func (s *Store) PutRoute(ctx context.Context, r Route) error
  - func (s *Store) ListRoutes(ctx context.Context) ([]Route, error)
  - func (s *Store) DeleteRoute(ctx context.Context, tool string) error
concept: file-backed relational config for adapters; one embedded DDL, no migrations, re-init on change; fail-closed parameterized CRUD; the tool->recipe route table; modernc sqlite quarantined.
behavior:
  - "OPEN + SCHEMA: Open(path) opens (or creates) the SQLite DB at path, sets MaxOpenConns to 1 (SQLite is single-writer; also keeps an in-memory DB consistent), and executes the single embedded DDL (CREATE TABLE IF NOT EXISTS for mcp_server, mcp_tool, context_provider, route). Open on a fresh path yields an empty store; Open on an existing DB reuses it. There are NO migrations: changing the schema means editing the DDL and removing the DB file to re-init. Open returns a non-nil error on an unusable path/driver."
  - "MCP SERVER CRUD (atomic tools): PutMCPServer upserts the server row AND replaces its tools in one transaction (delete the server's tools, insert srv.Tools) - a partial write never persists. GetMCPServer(name) returns the server with its Tools (ordered by tool name); an ABSENT server returns a non-nil not-found error, never a zero MCPServer as success. ListMCPServers returns every server (each with its tools) ordered by name. DeleteMCPServer removes the server and its tools (transaction)."
  - "PROVIDER + ROUTE CRUD: PutProvider upserts a context_provider by Name; ListProviders returns all ordered by name; DeleteProvider removes by name. PutRoute upserts a route by Tool (the primary key - one recipe governs one tool); ListRoutes returns all ordered by tool; DeleteRoute removes by tool. A Get/Delete of an absent row is handled fail-closed (Delete of absent is a no-op or a benign error; never a silent corrupt state)."
  - "PARAMETERIZED / INJECTION-SAFE + ROUND-TRIP: every query binds values as parameters, so any string - including SQL metacharacters (a quote, a semicolon, \"'; DROP TABLE mcp_server; --\"), NUL, or arbitrary unicode - is stored and read back BYTE-FOR-BYTE with no SQL side effect. For any valid entity, a Put followed by a Get/List returns an equal entity."
  - "FAIL CLOSED: after Close, or on any driver/query error, the CRUD methods return a non-nil error; nothing reports success on a failed write. Re-opening the same file path returns the previously stored rows (durability)."
constraints: package store at workspaces/stag/store (public; import path github.com/scanset/StAG/store). The DDL lives in this package as store/schema.sql, embedded via //go:embed (a core artifact of the module). Depends on modernc.org/sqlite (pure Go; QUARANTINED here - imported blank for driver registration) and stdlib (context, database/sql, embed, fmt, errors). No dependency on recipe/proxy/broker (the route->proxy.Router build is a separate wiring step); no network.
