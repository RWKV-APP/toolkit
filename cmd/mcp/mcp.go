package main

import (
	"fmt"
	"llm_toolkit"
	"os"
)

func main() {
	if err := llm_toolkit.NewMcpServer("llm-toolkit", "0.1.0").ServeStdio(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
