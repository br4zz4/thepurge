package service

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestPlistContent(t *testing.T) {
	pl := PlistFor("/usr/local/bin/thepurge", 300)
	xml, err := RenderPlist(pl)
	if err != nil {
		t.Fatalf("RenderPlist: %v", err)
	}
	for _, want := range []string{
		"Label",
		"br4zz4.thepurge",
		"ProgramArguments",
		"thepurge",
		"StartInterval",
		"300",
		"RunAtLoad",
		"StandardOutPath",
		"StandardErrorPath",
	} {
		if !strings.Contains(xml, want) {
			t.Errorf("plist missing %q; got:\n%s", want, xml)
		}
	}
}

func TestRenderPlistValidatesWithPlutil(t *testing.T) {
	if _, err := exec.LookPath("plutil"); err != nil {
		t.Skip("plutil not available")
	}
	pl := PlistFor("/usr/local/bin/thepurge", 1800)
	xml, err := RenderPlist(pl)
	if err != nil {
		t.Fatal(err)
	}
	tmp := filepath.Join(t.TempDir(), "br4zz4.thepurge.plist")
	if err := os.WriteFile(tmp, []byte(xml), 0o644); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("plutil", "-lint", tmp).CombinedOutput(); err != nil {
		t.Errorf("plutil -lint failed: %v\n%s", err, out)
	}
}
