package cli

import (
	"fmt"
	"sort"
	"strings"

	"br4zz4/thepurge/internal/eviction"
	"br4zz4/thepurge/internal/fsutil"
)

// TypeSummary aggregates candidates of one category.
type TypeSummary struct {
	Files int
	Bytes int64
}

// Summary is the dry-run digest of what eviction would remove.
type Summary struct {
	TotalFiles int
	TotalBytes int64
	ByType     map[string]TypeSummary
}

// summarize aggregates eviction candidates by type and totals.
func summarize(cands []eviction.Candidate) Summary {
	s := Summary{ByType: map[string]TypeSummary{}}
	for _, c := range cands {
		s.TotalFiles++
		s.TotalBytes += c.Size
		t := s.ByType[c.Type]
		t.Files++
		t.Bytes += c.Size
		s.ByType[c.Type] = t
	}
	return s
}

// renderSummary renders a human digest of the dry-run: total to free, file
// count and per-category breakdown, plus the largest candidates (configurable
// via topN). skipped lists which paths could not be considered.
func renderSummary(cands []eviction.Candidate, skipped []string, topN int) string {
	if topN <= 0 {
		topN = 10
	}
	var b strings.Builder

	s := summarize(cands)
	fmt.Fprintf(&b, "would free: %s (%d files)\n", fsutil.FormatBytes(s.TotalBytes), s.TotalFiles)
	if s.TotalFiles == 0 {
		return b.String()
	}

	var types []string
	for t := range s.ByType {
		types = append(types, t)
	}
	sort.Strings(types)
	for _, t := range types {
		ts := s.ByType[t]
		fmt.Fprintf(&b, "  %-8s %s (%d files)\n", t, fsutil.FormatBytes(ts.Bytes), ts.Files)
	}

	// Largest candidates first (cands are already eviction-ordered; pick the
	// heavy ones on top).
	sorted := append([]eviction.Candidate(nil), cands...)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].Size > sorted[j].Size })
	if len(sorted) > topN {
		sorted = sorted[:topN]
	}
	b.WriteString("largest:\n")
	for _, c := range sorted {
		fmt.Fprintf(&b, "  %-10s %s\n", fsutil.FormatBytes(c.Size), c.Path)
	}
	if len(cands) > topN {
		fmt.Fprintf(&b, "  … and %d more\n", len(cands)-topN)
	}

	if len(skipped) > 0 {
		fmt.Fprintf(&b, "skipped: %d (immutable/unreadable)\n", len(skipped))
	}
	return b.String()
}
