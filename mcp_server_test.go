package llm_toolkit

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

func TestMcpServerCallToolJSON(t *testing.T) {
	s := NewMcpServer("test", "0.0.1")
	err := s.RegisterTool(McpTool{
		Name: "echo",
		Handler: func(ctx context.Context, request mcp.CallToolRequest) (any, error) {
			return map[string]any{"message": request.GetString("message", "")}, nil
		},
	})
	if err != nil {
		t.Fatalf("RegisterTool() error = %v", err)
	}

	raw := s.CallToolJSON(`{"tool_name":"echo","arguments":{"message":"hello"}}`)
	var got struct {
		OK   bool `json:"ok"`
		Data struct {
			StructuredContent struct {
				Message string `json:"message"`
			} `json:"structuredContent"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(raw), &got); err != nil {
		t.Fatalf("CallToolJSON() returned invalid json: %v", err)
	}
	if !got.OK || got.Data.StructuredContent.Message != "hello" {
		t.Fatalf("CallToolJSON() = %s", raw)
	}
}

func TestMcpServerRejectsDuplicateTool(t *testing.T) {
	s := NewMcpServer("test", "0.0.1")
	tool := McpTool{
		Name: "dup",
		Handler: func(ctx context.Context, request mcp.CallToolRequest) (any, error) {
			return nil, nil
		},
	}
	if err := s.RegisterTool(tool); err != nil {
		t.Fatalf("first RegisterTool() error = %v", err)
	}
	if err := s.RegisterTool(tool); err == nil {
		t.Fatal("second RegisterTool() error = nil, want duplicate error")
	}
}

func TestMcpServerRegisterJSONTool(t *testing.T) {
	s := NewMcpServer("test", "0.0.1")
	err := s.RegisterJSONTool("json_echo", "", nil, func(input string) string {
		return `{"ok":true,"data":` + input + `}`
	})
	if err != nil {
		t.Fatalf("RegisterJSONTool() error = %v", err)
	}

	raw := s.CallToolJSON(`{"tool_name":"json_echo","arguments":{"value":42}}`)
	if !strings.Contains(raw, `"value":42`) {
		t.Fatalf("CallToolJSON() = %s, want wrapped json data", raw)
	}
}

func TestMcpServerRegisterTypedToolWithNestedArguments(t *testing.T) {
	type args struct {
		User struct {
			Name string `json:"name"`
		} `json:"user"`
		Tags []string `json:"tags"`
	}

	s := NewMcpServer("test", "0.0.1")
	err := RegisterTypedMcpTool(s, mcp.NewTool("typed"), func(ctx context.Context, request mcp.CallToolRequest, input args) (any, error) {
		return map[string]any{
			"name": input.User.Name,
			"tags": input.Tags,
		}, nil
	})
	if err != nil {
		t.Fatalf("RegisterTypedTool() error = %v", err)
	}

	raw := s.CallToolJSON(`{"tool_name":"typed","arguments":{"user":{"name":"Ada"},"tags":["go","mcp"]}}`)
	if !strings.Contains(raw, `"name":"Ada"`) || !strings.Contains(raw, `"go"`) {
		t.Fatalf("CallToolJSON() = %s, want nested typed data", raw)
	}
}
