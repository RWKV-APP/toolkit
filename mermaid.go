package llm_toolkit

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/dreampuf/mermaid.go"
	"github.com/mark3labs/mcp-go/mcp"
)

var mermaidInputSchema = []byte(`{
	"type":"object",
	"properties":{
		"content":{"type":"string","description":"Mermaid diagram source."},
		"format":{"type":"string","enum":["svg","png"],"default":"svg","description":"Rendered output format."},
		"scale":{"type":"number","default":1,"description":"PNG scale factor. Ignored for SVG."},
		"bundle":{"type":"boolean","default":false,"description":"Include the original Mermaid source in the SVG desc tag."}
	},
	"required":["content"],
	"additionalProperties":false
}`)

type MermaidRenderArgs struct {
	Content string  `json:"content"`
	Format  string  `json:"format"`
	Scale   float64 `json:"scale"`
	Bundle  bool    `json:"bundle"`
}

type MermaidRenderResult struct {
	Format   string `json:"format"`
	MIMEType string `json:"mime_type"`
	Path     string `json:"path"`
}

func NewMermaidMcpTool() McpTool {
	return McpTool{
		Name:        "mermaid",
		Description: "Render Mermaid diagram source to SVG or PNG.",
		InputSchema: mermaidInputSchema,
		Handler: func(ctx context.Context, request mcp.CallToolRequest) (any, error) {
			var args MermaidRenderArgs
			if err := request.BindArguments(&args); err != nil {
				return nil, err
			}
			return RenderMermaid(ctx, args)
		},
	}
}

func RenderMermaid(ctx context.Context, args MermaidRenderArgs) (MermaidRenderResult, error) {
	args.Content = strings.TrimSpace(args.Content)
	if args.Content == "" {
		return MermaidRenderResult{}, fmt.Errorf("content is required")
	}

	format := strings.ToLower(strings.TrimSpace(args.Format))
	if format == "" {
		format = "svg"
	}
	if args.Scale <= 0 {
		args.Scale = 1
	}

	re, err := mermaid_go.NewRenderEngine(ctx, nil)
	if err != nil {
		return MermaidRenderResult{}, err
	}
	defer re.Cancel()

	switch format {
	case "svg":
		opts := []mermaid_go.RenderOption{}
		if args.Bundle {
			opts = append(opts, mermaid_go.WithBundle())
		}
		svg, err := re.Render(args.Content, opts...)
		if err != nil {
			return MermaidRenderResult{}, fmt.Errorf("invalid mermaid syntax: %w", err)
		}
		if strings.TrimSpace(svg) == "" {
			return MermaidRenderResult{}, fmt.Errorf("invalid mermaid syntax: empty render result")
		}
		outputPath, err := writeMermaidTempFile("svg", []byte(svg))
		if err != nil {
			return MermaidRenderResult{}, err
		}
		return MermaidRenderResult{
			Format:   "svg",
			MIMEType: "image/svg+xml",
			Path:     outputPath,
		}, nil
	case "png":
		svg, err := re.Render(args.Content)
		if err != nil {
			return MermaidRenderResult{}, fmt.Errorf("invalid mermaid syntax: %w", err)
		}
		if strings.TrimSpace(svg) == "" {
			return MermaidRenderResult{}, fmt.Errorf("invalid mermaid syntax: empty render result")
		}
		png, _, err := re.RenderAsScaledPng(args.Content, args.Scale)
		if err != nil {
			return MermaidRenderResult{}, err
		}
		outputPath, err := writeMermaidTempFile("png", png)
		if err != nil {
			return MermaidRenderResult{}, err
		}
		return MermaidRenderResult{
			Format:   "png",
			MIMEType: "image/png",
			Path:     outputPath,
		}, nil
	default:
		return MermaidRenderResult{}, fmt.Errorf("unsupported mermaid output format: %s", format)
	}
}

func writeMermaidTempFile(ext string, data []byte) (string, error) {
	ext = strings.ToLower(strings.TrimPrefix(strings.TrimSpace(ext), "."))
	if ext == "" {
		return "", fmt.Errorf("file extension is required")
	}

	dir := filepath.Join(os.TempDir(), "llm_toolkit", "mermaid")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}

	file, err := os.CreateTemp(dir, "mermaid-*."+ext)
	if err != nil {
		return "", err
	}

	path := file.Name()
	defer file.Close()

	if _, err := file.Write(data); err != nil {
		return "", err
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	return absPath, nil
}
