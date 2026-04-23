package main

import (
	"fmt"
	"llm_toolkit"
	"os"
)

func main() {
	srv := llm_toolkit.NewMcpServer("mermaid", "0.1.0")
	srv.MustRegisterTool(llm_toolkit.NewMermaidMcpTool())
	if err := srv.ServeStdio(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
