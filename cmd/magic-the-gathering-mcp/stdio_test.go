package main

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestBuiltExecutableNegotiatesListsCallsAndExitsOnEOF(t *testing.T) {
	binary := buildExecutable(t)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	var stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, binary)
	cmd.Stderr = &stderr
	cmd.Env = append(os.Environ(), "MTG_MCP_CACHE_DIR="+t.TempDir(), "MTG_MCP_LOG_LEVEL=debug")
	client := mcp.NewClient(&mcp.Implementation{Name: "stdio-smoke", Version: "v0.1.0"}, nil)
	session, err := client.Connect(ctx, &mcp.CommandTransport{Command: cmd, TerminateDuration: time.Second}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := session.InitializeResult().ProtocolVersion; got != "2026-07-28" {
		t.Fatalf("protocol version = %q", got)
	}
	listed, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"search_cards", "get_oracle_card", "get_printing", "get_rulings", "get_card_legality"}
	if len(listed.Tools) != len(want) {
		t.Fatalf("tools = %#v", listed.Tools)
	}
	for i, name := range want {
		if listed.Tools[i].Name != name {
			t.Fatalf("tool %d = %q, want %q", i, listed.Tools[i].Name, name)
		}
	}
	result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "get_oracle_card", Arguments: map[string]any{}})
	if err != nil || !result.IsError || !strings.Contains(result.Content[0].(*mcp.TextContent).Text, `"code":"invalid_identifier"`) {
		t.Fatalf("representative call result=%#v err=%v", result, err)
	}
	if err := session.Close(); err != nil {
		t.Fatalf("graceful EOF: %v; stderr=%q", err, stderr.String())
	}
}

func TestInvalidStartupConfigurationUsesStderrOnly(t *testing.T) {
	binary := buildExecutable(t)
	cmd := exec.CommandContext(t.Context(), binary)
	cmd.Env = append(os.Environ(), "MTG_MCP_CACHE_MAX_BYTES=invalid")
	stdout, stderr := bytes.Buffer{}, bytes.Buffer{}
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err == nil {
		t.Fatal("startup succeeded")
	}
	if stdout.Len() != 0 || !strings.Contains(stderr.String(), "MTG_MCP_CACHE_MAX_BYTES") {
		t.Fatalf("stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
}

func buildExecutable(t *testing.T) string {
	t.Helper()
	binary := filepath.Join(t.TempDir(), "magic-the-gathering-mcp")
	cmd := exec.CommandContext(t.Context(), "go", "build", "-o", binary, ".")
	cmd.Env = append(os.Environ(), "GOCACHE="+filepath.Join(t.TempDir(), "go-build"))
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build executable: %v\n%s", err, output)
	}
	return binary
}
