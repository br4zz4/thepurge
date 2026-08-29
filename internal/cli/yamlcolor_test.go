package cli

import (
	"strings"
	"testing"
)

func TestColorYAMLAddsANSI(t *testing.T) {
	src := "storage:\n  threshold_type: percent\n  threshold_value: 15\n  # a comment\n"
	out := colorYAML(src)

	if !strings.Contains(out, "\x1b[") {
		t.Error("colorYAML produced no ANSI escapes")
	}
	// Text content is preserved.
	for _, want := range []string{"storage:", "threshold_type:", "percent", "threshold_value:", "15", "a comment"} {
		if !strings.Contains(out, want) {
			t.Errorf("colorYAML lost %q; got:\n%q", want, out)
		}
	}
}

func TestColorYAMLColorsKeyAndValue(t *testing.T) {
	out := colorYAML("threshold_type: percent\n")
	if !strings.Contains(out, ansiBold) || !strings.Contains(out, ansiCyan) {
		t.Errorf("key not colored bold cyan:\n%q", out)
	}
	if !strings.Contains(out, ansiCyan+"threshold_type:") {
		t.Errorf("key not colored cyan:\n%q", out)
	}
	if !strings.Contains(out, ansiGreen+"percent"+ansiReset) {
		t.Errorf("string value not colored:\n%q", out)
	}
}

func TestColorYAMLNumbersInYellow(t *testing.T) {
	out := colorYAML("threshold_value: 15\n")
	if !strings.Contains(out, ansiYellow+"15"+ansiReset) {
		t.Errorf("number not colored yellow:\n%q", out)
	}
}

func TestColorYAMLCommentInDim(t *testing.T) {
	out := colorYAML("# comment\n")
	if !strings.Contains(out, ansiDim+"# comment"+ansiReset) {
		t.Errorf("comment not dimmed:\n%q", out)
	}
}

func TestColorYAMLEmpty(t *testing.T) {
	if out := colorYAML(""); out != "" {
		t.Errorf("colorYAML empty = %q, want empty", out)
	}
}
