package cli

import (
	"fmt"
	"io"
	"strings"
)

// runFunc executes a subcommand. It returns an exit code and, for errors, the
// raw error. Output goes to out; diagnostics go to errOut.
type runFunc func(args []string, out, errOut io.Writer) (int, error)

// commands maps subcommand names to their handlers.
var commands = map[string]runFunc{
	"status":  cmdStatus,
	"doctor":  cmdDoctor,
	"purge":   cmdPurge,
	"run":     cmdRun,
	"service": cmdService,
	"config":  cmdConfig,
}

// Run dispatches a subcommand from argv and returns the process exit code.
// out/errOut are injectable for testing.
func Run(argv []string, out, errOut io.Writer) int {
	if len(argv) == 0 {
		printUsage(out)
		return 0
	}

	name := argv[0]
	if name == "--help" || name == "-h" || name == "help" {
		printUsage(out)
		return 0
	}

	fn, ok := commands[name]
	if !ok {
		fmt.Fprintf(errOut, "thepurge: unknown command %q\n\n", name)
		printUsage(errOut)
		return 2
	}

	code, err := fn(argv[1:], out, errOut)
	if err != nil {
		fmt.Fprintf(errOut, "thepurge: %v\n", err)
		if code == 0 {
			code = 1
		}
	}
	return code
}

func printUsage(w io.Writer) {
	fmt.Fprint(w, `Usage:
  thepurge status                    Show disk space, threshold and last run
  thepurge doctor                    Validate config, permissions and targets
  thepurge purge [--apply]           Preview (default) or perform eviction
  thepurge run                       Run one eviction pass (used by the service)
  thepurge service install|uninstall|status
  thepurge config init|get|set|validate
  thepurge config orphans.enable|disable --path <PATH>
  thepurge help

Environment:
  THEPURGE_CONFIG   path to config.yml (default ~/.config/thepurge/config.yml)
`)
}

// splitFlags splits args into positional and a flag map (supports --apply, --force).
func splitFlags(args []string) (flagMap map[string]bool, positional []string) {
	flagMap = map[string]bool{}
	for _, a := range args {
		if strings.HasPrefix(a, "--") {
			flagMap[a] = true
		} else {
			positional = append(positional, a)
		}
	}
	return flagMap, positional
}
