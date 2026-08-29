package service

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestPlistContent(t *testing.T) {
	pl := PlistFor("/usr/local/bin/thepurge", 300, false)
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
	pl := PlistFor("/usr/local/bin/thepurge", 1800, false)
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

func TestPlistApplyArg(t *testing.T) {
	// apply=false -> only the "run" subcommand.
	xml, err := RenderPlist(PlistFor("/usr/local/bin/thepurge", 300, false))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(xml, "--apply") {
		t.Errorf("apply=false plist should not contain --apply:\n%s", xml)
	}

	// apply=true -> ProgramArguments carries --apply.
	xmlApply, err := RenderPlist(PlistFor("/usr/local/bin/thepurge", 300, true))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(xmlApply, "--apply") {
		t.Errorf("apply=true plist should contain --apply:\n%s", xmlApply)
	}
}
