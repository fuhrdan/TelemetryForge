package main

import (
	"flag"
	"fmt"
	"github.com/fuhrdan/TelemetryForge/internal/wasmplugin"
	"os"
)

func main() {
	manifestPath := flag.String("manifest", "", "plugin manifest JSON")
	modulePath := flag.String("module", "", "WebAssembly module")
	flag.Parse()
	if *manifestPath == "" || *modulePath == "" {
		fmt.Fprintln(os.Stderr, "--manifest and --module are required")
		os.Exit(2)
	}
	m, err := wasmplugin.LoadManifest(*manifestPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	raw, err := os.ReadFile(*modulePath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := wasmplugin.VerifyModule(raw, m.ModuleSHA256); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := wasmplugin.VerifyEntrypoint(raw, m.Entrypoint); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("verified %s@%s entrypoint=%s module=%s\n", m.Name, m.Version, m.Entrypoint, m.ModuleSHA256)
}
