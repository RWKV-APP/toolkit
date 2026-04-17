package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"llm_toolkit"
	"os"
)

type output struct {
	Hardware llm_toolkit.HardwareInfo `json:"hardware"`
	Usage    llm_toolkit.UsageInfo    `json:"usage"`
}

func main() {
	pid := flag.Int("pid", 0, "optional process id")
	flag.Parse()

	hardware, err := llm_toolkit.GetHardwareInfo()
	if err != nil {
		exitWithError(err)
	}

	usage, err := llm_toolkit.GetHardwareUsageInfoForPID(int32(*pid))
	if err != nil {
		exitWithError(err)
	}

	writeJSON(output{
		Hardware: hardware,
		Usage:    usage,
	})
}

func writeJSON(v any) {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(v); err != nil {
		exitWithError(err)
	}
}

func exitWithError(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
