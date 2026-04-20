package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"llm_toolkit"
	"os"
	"os/signal"
	"time"
)

var version = "1.0.1"

func main() {
	if len(os.Args) < 2 {
		printRootUsage(os.Stdout)
		return
	}

	switch os.Args[1] {
	case "info":
		runInfoCommand(os.Args[2:])
	case "usage":
		runUsageCommand(os.Args[2:])
	case "version":
		if _, err := fmt.Fprintln(os.Stdout, version); err != nil {
			exitWithError(err)
		}
	case "help", "-h", "--help":
		printRootUsage(os.Stdout)
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n\n", os.Args[1])
		printRootUsage(os.Stderr)
		os.Exit(1)
	}
}

func runInfoCommand(args []string) {
	fs := newFlagSet("info")
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "Usage: toolkit info")
	}
	if err := fs.Parse(args); err != nil {
		exitWithError(err)
	}

	hardware, err := llm_toolkit.GetHardwareInfo()
	if err != nil {
		exitWithError(err)
	}

	writeJSON(hardware)
}

func runUsageCommand(args []string) {
	fs := newFlagSet("usage")
	pid := fs.Int("pid", 0, "optional process id")
	watch := fs.Bool("watch", false, "continuously monitor usage")
	interval := fs.Int("interval", 3, "monitor interval in seconds")
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "Usage: toolkit usage [-pid PID] [-watch] [-interval SECONDS]")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		exitWithError(err)
	}

	if *interval <= 0 {
		exitWithError(fmt.Errorf("interval must be greater than 0"))
	}

	if !*watch {
		usage, err := llm_toolkit.GetHardwareUsageInfoForPID(int32(*pid))
		if err != nil {
			exitWithError(err)
		}

		writeJSON(usage)
		return
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	err := llm_toolkit.MonitorHardwareUsageForPID(ctx, int32(*pid), time.Duration(*interval)*time.Second, func(usage llm_toolkit.UsageInfo) error {
		writeJSON(usage)
		return nil
	})
	if err != nil && !errors.Is(err, context.Canceled) {
		exitWithError(err)
	}
}

func newFlagSet(name string) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	return fs
}

func printRootUsage(w *os.File) {
	fmt.Fprintln(w, "Usage:")
	fmt.Fprintln(w, "  toolkit info")
	fmt.Fprintln(w, "  toolkit usage [-pid PID] [-watch] [-interval SECONDS]")
	fmt.Fprintln(w, "  toolkit version")
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
