package cli

import (
	"regexp"
	"strings"
)

// ANSI escape codes used by colorYAML. Exported for tests.
const (
	ansiReset  = "\x1b[0m"
	ansiBold   = "\x1b[1m"
	ansiCyan   = "\x1b[36m"
	ansiGreen  = "\x1b[32m"
	ansiYellow = "\x1b[33m"
	ansiDim    = "\x1b[2m"
)

var (
	reKey     = regexp.MustCompile(`^(\s*[A-Za-z_][A-Za-z0-9_]*):`)
	reComment = regexp.MustCompile(`^(\s*#.*)$`)
)

// colorYAML renders a YAML document with light ANSI coloring: keys in bold cyan,
// plain string values in green, numeric values in yellow and comments dimmed.
// Non-string values (booleans, lists) keep default color. Lines are processed
// per-line so inline comments keep their own color; a trailing comment is dimmed
// only if it is the whole rest of the line.
func colorYAML(src string) string {
	if src == "" {
		return ""
	}
	lines := strings.Split(src, "\n")
	for i, line := range lines {
		lines[i] = colorLine(line)
	}
	return strings.Join(lines, "\n")
}

func colorLine(line string) string {
	// Whole-line comment.
	if reComment.MatchString(line) {
		return ansiDim + line + ansiReset
	}

	// Key followed by a value ("key: value").
	if m := reKey.FindStringSubmatchIndex(line); m != nil {
		prefix := line[:m[1]] // indentation + key + colon (bold cyan)
		rest := line[m[1]:]
		if strings.TrimSpace(rest) == "" {
			// "key:" with no value on this line.
			return ansiBold + ansiCyan + prefix + ansiReset
		}
		// Value part: strip a trailing inline comment for separate coloring.
		value, comment := splitInlineComment(rest)
		colored := colorValue(strings.TrimSpace(value))
		out := ansiBold + ansiCyan + prefix + ansiReset + " " + colored
		if comment != "" {
			out += " " + ansiDim + comment + ansiReset
		}
		return out
	}

	// List item value ("- value").
	if strings.HasPrefix(line, "- ") {
		val := strings.TrimPrefix(line, "- ")
		return "- " + colorValue(val)
	}
	return line
}

func splitInlineComment(s string) (value, comment string) {
	// Split on ' #' that starts a comment (not inside a quoted string — good
	// enough for config files).
	idx := strings.Index(s, " #")
	if idx >= 0 {
		return s[:idx], s[idx+1:]
	}
	return s, ""
}

func colorValue(val string) string {
	if val == "" {
		return val
	}
	if isNumber(val) {
		return ansiYellow + val + ansiReset
	}
	if isQuoted(val) {
		return ansiGreen + val + ansiReset
	}
	// Plain scalars (booleans, paths): color as string too.
	return ansiGreen + val + ansiReset
}

func isNumber(s string) bool {
	if s == "" {
		return false
	}
	// Allow optional sign and decimals.
	i := 0
	if s[0] == '+' || s[0] == '-' {
		i++
	}
	digits := 0
	dot := 0
	for ; i < len(s); i++ {
		switch {
		case s[i] >= '0' && s[i] <= '9':
			digits++
		case s[i] == '.':
			dot++
		default:
			return false
		}
	}
	return digits > 0 && dot <= 1
}

func isQuoted(s string) bool {
	if len(s) < 2 {
		return false
	}
	return (s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'')
}
