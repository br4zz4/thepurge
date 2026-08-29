// Command thepurge is a macOS disk-cleanup tool: it monitors SSD free space and
// evicts caches/logs (and opt-in orphan folders) when space drops below a
// configured threshold. It runs either as a manual CLI or, via a launchd agent,
// as a periodic background service (`thepurge run`).
package main

import (
	"os"

	"br4zz4/thepurge/internal/cli"
)

func main() {
	os.Exit(cli.Run(os.Args[1:], os.Stdout, os.Stderr))
}
