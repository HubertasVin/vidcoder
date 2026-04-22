package main

import (
	"errors"
	"fmt"
	"os"
	"runtime/debug"
)

func main() {
	cfg, err := parseArgs(os.Args[1:])
	if err != nil {
		if errors.Is(err, errHelp) {
			printUsage(os.Stdout)
			return
		}
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if cfg.ShowVersion {
		info, ok := debug.ReadBuildInfo()
		if ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
			fmt.Fprintln(os.Stdout, info.Main.Version)
		} else {
			fmt.Fprintln(os.Stdout, "dev")
		}
		return
	}

	if err := run(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
