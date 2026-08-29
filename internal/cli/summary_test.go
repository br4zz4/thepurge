package cli

import (
	"strings"
	"testing"
	"time"

	"br4zz4/thepurge/internal/eviction"
)

func TestSummarizeByType(t *testing.T) {
	now := time.Now()
	cands := []eviction.Candidate{
		{Path: "a", Size: 100, ModTime: now, Type: "cache"},
		{Path: "b", Size: 50, ModTime: now, Type: "cache"},
		{Path: "c", Size: 200, ModTime: now, Type: "log"},
	}
	s := summarize(cands)

	if s.TotalFiles != 3 {
		t.Errorf("TotalFiles = %d, want 3", s.TotalFiles)
	}
	if s.TotalBytes != 350 {
		t.Errorf("TotalBytes = %d, want 350", s.TotalBytes)
	}
	if s.ByType["cache"].Files != 2 || s.ByType["cache"].Bytes != 150 {
		t.Errorf("cache summary = %+v, want 2 files / 150 bytes", s.ByType["cache"])
	}
	if s.ByType["log"].Files != 1 || s.ByType["log"].Bytes != 200 {
		t.Errorf("log summary = %+v, want 1 file / 200 bytes", s.ByType["log"])
	}
}

func TestSummarizeEmpty(t *testing.T) {
	s := summarize(nil)
	if s.TotalFiles != 0 || s.TotalBytes != 0 {
		t.Errorf("empty summary = %+v, want zeros", s)
	}
	if len(s.ByType) != 0 {
		t.Errorf("empty summary ByType = %v, want empty", s.ByType)
	}
}

func TestRenderSummaryShowsTotalsAndCategories(t *testing.T) {
	now := time.Now()
	cands := []eviction.Candidate{
		{Path: "f1", Size: 1_000, ModTime: now, Type: "cache"},
		{Path: "f2", Size: 2_000, ModTime: now, Type: "log"},
	}
	rendered := renderSummary(cands, nil, 10)

	for _, want := range []string{"would free", "3.0 KB", "2 files", "cache", "log"} {
		if !strings.Contains(rendered, want) {
			t.Errorf("rendered summary missing %q; got:\n%s", want, rendered)
		}
	}
}
