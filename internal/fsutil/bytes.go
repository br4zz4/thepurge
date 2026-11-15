package fsutil

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// byteSuffixes maps a size suffix (case-insensitive) to its decimal multiplier.
var byteSuffixes = map[string]int64{
	"B":  1,
	"KB": 1_000,
	"MB": 1_000_000,
	"GB": 1_000_000_000,
	"TB": 1_000_000_000_000,
}

// ParseBytes parses a human-readable size like "10GB", "5MB", "1KB" or a bare
// byte count ("500") into bytes. Suffixes are case-insensitive. Without a
// suffix the value is treated as plain bytes. Returns an error for empty input,
// unknown suffixes or malformed numbers.
func ParseBytes(s string) (int64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("empty size")
	}

	i := 0
	for i < len(s) && ((s[i] >= '0' && s[i] <= '9') || s[i] == '.') {
		i++
	}
	numPart := s[:i]
	sufPart := strings.ToUpper(strings.TrimSpace(s[i:]))

	if numPart == "" {
		return 0, fmt.Errorf("invalid size %q: no numeric value", s)
	}

	var mult int64 = 1
	if sufPart != "" {
		m, ok := byteSuffixes[sufPart]
		if !ok {
			return 0, fmt.Errorf("invalid size %q: unknown suffix %q", s, sufPart)
		}
		mult = m
	}

	n, err := strconv.ParseFloat(numPart, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid size %q: %v", s, err)
	}
	return int64(math.Round(n * float64(mult))), nil
}

// FormatBytes renders a byte count as a human-readable decimal string with two
// significant digits, e.g. 10_000_000_000 -> "10.0 GB". Values under 1000 render
// as plain bytes.
func FormatBytes(n int64) string {
	const (
		kb = 1_000
		mb = 1_000 * kb
		gb = 1_000 * mb
		tb = 1_000 * gb
	)
	switch {
	case n >= tb:
		return fmt.Sprintf("%.1f TB", float64(n)/tb)
	case n >= gb:
		return fmt.Sprintf("%.1f GB", float64(n)/gb)
	case n >= mb:
		return fmt.Sprintf("%.1f MB", float64(n)/mb)
	case n >= kb:
		return fmt.Sprintf("%.1f KB", float64(n)/kb)
	default:
		return fmt.Sprintf("%d B", n)
	}
}
