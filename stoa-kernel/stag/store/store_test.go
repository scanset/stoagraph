package store_test

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/scanset/stoagraph/stoa-kernel/stag/store"
)

func openTemp(t *testing.T) (*store.Store, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.db")
	s, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s, path
}

func sampleServer() store.MCPServer {
	return store.MCPServer{
		Name: "fs", Transport: "stdio", Target: "npx server-filesystem", Enabled: true,
		Tools: []store.MCPTool{
			{Server: "fs", Name: "read_file", InputSchema: `{"type":"object"}`},
			{Server: "fs", Name: "write_file", InputSchema: `{"type":"object"}`},
		},
	}
}

func TestServerCRUD(t *testing.T) {
	ctx := context.Background()
	s, _ := openTemp(t)

	if err := s.PutMCPServer(ctx, sampleServer()); err != nil {
		t.Fatal(err)
	}
	got, err := s.GetMCPServer(ctx, "fs")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, sampleServer()) {
		t.Fatalf("round-trip mismatch:\n got %+v\nwant %+v", got, sampleServer())
	}
	if list, _ := s.ListMCPServers(ctx); len(list) != 1 || list[0].Name != "fs" {
		t.Fatalf("list: %+v", list)
	}

	// re-put with ONE tool: the tool set is REPLACED, not appended
	one := sampleServer()
	one.Tools = one.Tools[:1]
	if err := s.PutMCPServer(ctx, one); err != nil {
		t.Fatal(err)
	}
	got, _ = s.GetMCPServer(ctx, "fs")
	if len(got.Tools) != 1 || got.Tools[0].Name != "read_file" {
		t.Errorf("tools must be replaced, got %+v", got.Tools)
	}

	if err := s.DeleteMCPServer(ctx, "fs"); err != nil {
		t.Fatal(err)
	}
	if list, _ := s.ListMCPServers(ctx); len(list) != 0 {
		t.Errorf("after delete: %+v", list)
	}
	if _, err := s.GetMCPServer(ctx, "fs"); err == nil {
		t.Error("get of deleted server must error")
	}
}

func TestProviderAndRouteCRUD(t *testing.T) {
	ctx := context.Background()
	s, _ := openTemp(t)

	p := store.ContextProvider{Name: "runbooks", Kind: "rag", Config: `{"dir":"kb"}`, Enabled: true}
	if err := s.PutProvider(ctx, p); err != nil {
		t.Fatal(err)
	}
	if list, _ := s.ListProviders(ctx); len(list) != 1 || !reflect.DeepEqual(list[0], p) {
		t.Fatalf("providers: %+v", list)
	}
	_ = s.DeleteProvider(ctx, "runbooks")
	if list, _ := s.ListProviders(ctx); len(list) != 0 {
		t.Errorf("providers after delete: %+v", list)
	}

	r := store.Route{Tool: "write_note", Recipe: "write_note_policy", GateArg: "text"}
	if err := s.PutRoute(ctx, r); err != nil {
		t.Fatal(err)
	}
	// same tool, different recipe -> replaced (one route per tool)
	if err := s.PutRoute(ctx, store.Route{Tool: "write_note", Recipe: "other", GateArg: "text"}); err != nil {
		t.Fatal(err)
	}
	list, _ := s.ListRoutes(ctx)
	if len(list) != 1 || list[0].Recipe != "other" {
		t.Fatalf("route must be replaced: %+v", list)
	}
	_ = s.DeleteRoute(ctx, "write_note")
	if list, _ := s.ListRoutes(ctx); len(list) != 0 {
		t.Errorf("routes after delete: %+v", list)
	}
}

func TestAbsentFailsClosed(t *testing.T) {
	ctx := context.Background()
	s, _ := openTemp(t)
	got, err := s.GetMCPServer(ctx, "nope")
	if err == nil {
		t.Error("absent server must error")
	}
	if !reflect.DeepEqual(got, store.MCPServer{}) {
		t.Errorf("absent server must be zero, got %+v", got)
	}
	_ = s.Close()
	if err := s.PutRoute(ctx, store.Route{Tool: "x", Recipe: "y", GateArg: "z"}); err == nil {
		t.Error("op after Close must error")
	}
}

func TestDurabilityAndReInit(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "cfg.db")

	s, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.PutMCPServer(ctx, sampleServer()); err != nil {
		t.Fatal(err)
	}
	_ = s.Close()

	// re-open the same path: durable
	s2, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if list, _ := s2.ListMCPServers(ctx); len(list) != 1 {
		t.Errorf("re-open must be durable: %+v", list)
	}
	_ = s2.Close()

	// re-init: remove the DB file, open fresh -> empty (no migration, DDL recreates)
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	s3, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s3.Close()
	if list, _ := s3.ListMCPServers(ctx); len(list) != 0 {
		t.Errorf("re-init must be empty: %+v", list)
	}
}

func FuzzRoundTrip(f *testing.F) {
	f.Add("fs", "npx server", "read_file")
	f.Add("s", "'; DROP TABLE mcp_server; --", "t")
	f.Add("srv", "über\x00null", "tøøl")
	f.Fuzz(func(t *testing.T, name, target, tool string) {
		if name == "" || tool == "" {
			return // name is the PK; tool is part of the tool PK
		}
		ctx := context.Background()
		s, err := store.Open(":memory:")
		if err != nil {
			t.Fatal(err)
		}
		defer s.Close()

		srv := store.MCPServer{Name: name, Transport: "stdio", Target: target, Enabled: true,
			Tools: []store.MCPTool{{Server: name, Name: tool}}}
		if err := s.PutMCPServer(ctx, srv); err != nil {
			t.Fatalf("put: %v", err)
		}
		got, err := s.GetMCPServer(ctx, name)
		if err != nil {
			t.Fatalf("get: %v", err)
		}
		// byte-for-byte round trip; injection strings are inert data
		if got.Target != target {
			t.Fatalf("target not faithful: %q != %q", got.Target, target)
		}
		if len(got.Tools) != 1 || got.Tools[0].Name != tool {
			t.Fatalf("tool not faithful: %+v", got.Tools)
		}
		// the table still exists and holds exactly the one row we wrote
		if list, err := s.ListMCPServers(ctx); err != nil || len(list) != 1 {
			t.Fatalf("table intact? err=%v list=%d (injection must not execute)", err, len(list))
		}
	})
}
