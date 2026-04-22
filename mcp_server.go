package llm_toolkit

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

type McpToolHandler func(ctx context.Context, request mcp.CallToolRequest) (any, error)

type McpTool struct {
	Name         string
	Description  string
	InputSchema  json.RawMessage
	OutputSchema json.RawMessage
	Definition   mcp.Tool
	Handler      McpToolHandler
}

type McpServer struct {
	name    string
	version string

	server *server.MCPServer

	toolsMu sync.RWMutex
	tools   map[string]server.ToolHandlerFunc
}

type mcpCallToolJSONRequest struct {
	ToolName  string `json:"tool_name"`
	Arguments any    `json:"arguments,omitempty"`
}

var defaultMcpInputSchema = json.RawMessage(`{"type":"object","additionalProperties":true}`)

func NewMcpServer(name string, version string) *McpServer {
	return NewMcpServerWithOptions(name, version)
}

func NewMcpServerWithOptions(name string, version string, opts ...server.ServerOption) *McpServer {
	name = strings.TrimSpace(name)
	if name == "" {
		name = "llm-toolkit"
	}
	version = strings.TrimSpace(version)
	if version == "" {
		version = "0.1.0"
	}

	serverOptions := []server.ServerOption{
		server.WithToolCapabilities(true),
		server.WithRecovery(),
	}
	serverOptions = append(serverOptions, opts...)

	return &McpServer{
		name:    name,
		version: version,
		server:  server.NewMCPServer(name, version, serverOptions...),
		tools:   make(map[string]server.ToolHandlerFunc),
	}
}

func NewLLMToolkitMcpServer() *McpServer {
	return NewMcpServer("llm-toolkit", "0.1.0")
}

func (s *McpServer) RegisterTool(tool McpTool) error {
	if tool.Handler == nil {
		return fmt.Errorf("tool handler is required")
	}
	definition, err := buildMcpToolDefinition(tool)
	if err != nil {
		return err
	}
	return s.RegisterMCPTool(definition, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		result, err := tool.Handler(ctx, request)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return toMcpToolResult(result)
	})
}

func (s *McpServer) MustRegisterTool(tool McpTool) {
	if err := s.RegisterTool(tool); err != nil {
		panic(err)
	}
}

func (s *McpServer) RegisterMCPTool(tool mcp.Tool, handler server.ToolHandlerFunc) error {
	if s == nil || s.server == nil {
		return fmt.Errorf("mcp server is nil")
	}
	tool.Name = strings.TrimSpace(tool.Name)
	if tool.Name == "" {
		return fmt.Errorf("tool name is required")
	}
	if handler == nil {
		return fmt.Errorf("tool handler is required: %s", tool.Name)
	}

	s.toolsMu.Lock()
	if _, exists := s.tools[tool.Name]; exists {
		s.toolsMu.Unlock()
		return fmt.Errorf("tool already registered: %s", tool.Name)
	}
	s.tools[tool.Name] = handler
	s.toolsMu.Unlock()

	s.server.AddTool(tool, handler)
	return nil
}

func (s *McpServer) MustRegisterMCPTool(tool mcp.Tool, handler server.ToolHandlerFunc) {
	if err := s.RegisterMCPTool(tool, handler); err != nil {
		panic(err)
	}
}

func (s *McpServer) RegisterJSONTool(name string, description string, inputSchema json.RawMessage, fn func(string) string) error {
	if fn == nil {
		return fmt.Errorf("json tool function is nil: %s", name)
	}
	return s.RegisterTool(McpTool{
		Name:        name,
		Description: description,
		InputSchema: inputSchema,
		Handler: func(ctx context.Context, request mcp.CallToolRequest) (any, error) {
			input, err := json.Marshal(request.GetRawArguments())
			if err != nil {
				return nil, err
			}

			raw := fn(string(input))
			var payload any
			if err := json.Unmarshal([]byte(raw), &payload); err != nil {
				return nil, fmt.Errorf("tool returned invalid json: %w", err)
			}
			return payload, nil
		},
	})
}

func RegisterTypedMcpTool[T any](s *McpServer, tool mcp.Tool, handler func(context.Context, mcp.CallToolRequest, T) (any, error)) error {
	if handler == nil {
		return fmt.Errorf("typed tool handler is required: %s", tool.Name)
	}
	return s.RegisterTool(McpTool{
		Definition: tool,
		Handler: func(ctx context.Context, request mcp.CallToolRequest) (any, error) {
			var args T
			if err := request.BindArguments(&args); err != nil {
				return nil, err
			}
			return handler(ctx, request, args)
		},
	})
}

func (s *McpServer) MustRegisterJSONTool(name string, description string, inputSchema json.RawMessage, fn func(string) string) {
	if err := s.RegisterJSONTool(name, description, inputSchema, fn); err != nil {
		panic(err)
	}
}

func (s *McpServer) AddTools(tools ...server.ServerTool) {
	s.server.AddTools(tools...)
	for _, tool := range tools {
		if tool.Handler == nil {
			continue
		}
		s.toolsMu.Lock()
		s.tools[tool.Tool.Name] = tool.Handler
		s.toolsMu.Unlock()
	}
}

func (s *McpServer) AddResource(resource mcp.Resource, handler server.ResourceHandlerFunc) {
	s.server.AddResource(resource, handler)
}

func (s *McpServer) AddResources(resources ...server.ServerResource) {
	s.server.AddResources(resources...)
}

func (s *McpServer) AddResourceTemplate(template mcp.ResourceTemplate, handler server.ResourceTemplateHandlerFunc) {
	s.server.AddResourceTemplate(template, handler)
}

func (s *McpServer) AddResourceTemplates(resourceTemplates ...server.ServerResourceTemplate) {
	s.server.AddResourceTemplates(resourceTemplates...)
}

func (s *McpServer) AddPrompt(prompt mcp.Prompt, handler server.PromptHandlerFunc) {
	s.server.AddPrompt(prompt, handler)
}

func (s *McpServer) AddPrompts(prompts ...server.ServerPrompt) {
	s.server.AddPrompts(prompts...)
}

func (s *McpServer) AddNotificationHandler(method string, handler server.NotificationHandlerFunc) {
	s.server.AddNotificationHandler(method, handler)
}

func (s *McpServer) Use(mw ...server.ToolHandlerMiddleware) {
	s.server.Use(mw...)
}

func (s *McpServer) ServeStdio() error {
	if s == nil || s.server == nil {
		return fmt.Errorf("mcp server is nil")
	}
	return server.ServeStdio(s.server)
}

func (s *McpServer) NewStreamableHTTPServer(opts ...server.StreamableHTTPOption) (*server.StreamableHTTPServer, error) {
	if s == nil || s.server == nil {
		return nil, fmt.Errorf("mcp server is nil")
	}
	return server.NewStreamableHTTPServer(s.server, opts...), nil
}

func (s *McpServer) ServeStreamableHTTP(addr string, opts ...server.StreamableHTTPOption) error {
	httpServer, err := s.NewStreamableHTTPServer(opts...)
	if err != nil {
		return err
	}
	return httpServer.Start(addr)
}

func (s *McpServer) NewSSEServer(opts ...server.SSEOption) (*server.SSEServer, error) {
	if s == nil || s.server == nil {
		return nil, fmt.Errorf("mcp server is nil")
	}
	return server.NewSSEServer(s.server, opts...), nil
}

func (s *McpServer) ServeSSE(addr string, opts ...server.SSEOption) error {
	sseServer, err := s.NewSSEServer(opts...)
	if err != nil {
		return err
	}
	return sseServer.Start(addr)
}

func (s *McpServer) InnerServer() *server.MCPServer {
	if s == nil {
		return nil
	}
	return s.server
}

func (s *McpServer) CallToolJSON(input string) string {
	return writeJSON(func() (any, error) {
		var req mcpCallToolJSONRequest
		if err := decodeInput(input, &req); err != nil {
			return nil, err
		}
		req.ToolName = strings.TrimSpace(req.ToolName)
		if req.ToolName == "" {
			return nil, fmt.Errorf("tool_name is required")
		}

		s.toolsMu.RLock()
		handler, ok := s.tools[req.ToolName]
		s.toolsMu.RUnlock()
		if !ok {
			return nil, fmt.Errorf("tool not found: %s", req.ToolName)
		}

		result, err := handler(context.Background(), mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name:      req.ToolName,
				Arguments: req.Arguments,
			},
		})
		if err != nil {
			return nil, err
		}
		return result, nil
	})
}

func buildMcpToolDefinition(tool McpTool) (mcp.Tool, error) {
	if tool.Definition.Name != "" {
		if len(tool.OutputSchema) > 0 {
			if !json.Valid(tool.OutputSchema) {
				return mcp.Tool{}, fmt.Errorf("invalid output schema for tool %s", tool.Definition.Name)
			}
			tool.Definition.RawOutputSchema = tool.OutputSchema
		}
		return tool.Definition, nil
	}

	tool.Name = strings.TrimSpace(tool.Name)
	if tool.Name == "" {
		return mcp.Tool{}, fmt.Errorf("tool name is required")
	}
	if len(tool.InputSchema) == 0 {
		tool.InputSchema = defaultMcpInputSchema
	}
	if !json.Valid(tool.InputSchema) {
		return mcp.Tool{}, fmt.Errorf("invalid input schema for tool %s", tool.Name)
	}
	if len(tool.OutputSchema) > 0 && !json.Valid(tool.OutputSchema) {
		return mcp.Tool{}, fmt.Errorf("invalid output schema for tool %s", tool.Name)
	}

	definition := mcp.NewToolWithRawSchema(tool.Name, tool.Description, tool.InputSchema)
	if len(tool.OutputSchema) > 0 {
		definition.RawOutputSchema = tool.OutputSchema
	}
	return definition, nil
}

func toMcpToolResult(result any) (*mcp.CallToolResult, error) {
	switch v := result.(type) {
	case nil:
		return mcp.NewToolResultText(""), nil
	case *mcp.CallToolResult:
		return v, nil
	case mcp.CallToolResult:
		return &v, nil
	case string:
		return mcp.NewToolResultText(v), nil
	case []byte:
		return mcp.NewToolResultText(string(v)), nil
	default:
		return mcp.NewToolResultStructuredOnly(v), nil
	}
}
