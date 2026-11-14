package cli

import (
	"fmt"
	"io"

	"br4zz4/thepurge/internal/version"
)

// currentVersion is a seam for tests.
var currentVersion = version.Current

// cmdVersion prints the binary's version, commit and Go toolchain.
func cmdVersion(args []string, out, errOut io.Writer) (int, error) {
	for _, line := range currentVersion().Lines() {
		fmt.Fprintln(out, line)
	}
	return 0, nil
}
