package fsutil

import "testing"

func TestParseBytes(t *testing.T) {
	cases := []struct {
		in   string
		want int64
		ok   bool
	}{
		{"10GB", 10_000_000_000, true},
		{"10gb", 10_000_000_000, true},
		{"1KB", 1_000, true},
		{"1kb", 1_000, true},
		{"5MB", 5_000_000, true},
		{"2TB", 2_000_000_000_000, true},
		{"500", 500, true},
		{"12.3MB", 12_300_000, true},
		{"0B", 0, true},
		{"1.5GB", 1_500_000_000, true},
		{"512B", 512, true},
		{"", 0, false},
		{"abc", 0, false},
		{"10XB", 0, false},
	}
	for _, c := range cases {
		got, err := ParseBytes(c.in)
		if c.ok && err != nil {
			t.Errorf("ParseBytes(%q) unexpected error: %v", c.in, err)
			continue
		}
		if !c.ok {
			if err == nil {
				t.Errorf("ParseBytes(%q) expected error, got %d", c.in, got)
			}
			continue
		}
		if got != c.want {
			t.Errorf("ParseBytes(%q) = %d, want %d", c.in, got, c.want)
		}
	}
}

func TestFormatBytes(t *testing.T) {
	cases := []struct {
		in   int64
		want string
	}{
		{500, "500 B"},
		{1_000, "1.0 KB"},
		{5_000_000, "5.0 MB"},
		{10_000_000_000, "10.0 GB"},
	}
	for _, c := range cases {
		if got := FormatBytes(c.in); got != c.want {
			t.Errorf("FormatBytes(%d) = %q, want %q", c.in, got, c.want)
		}
	}
}
