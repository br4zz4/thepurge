package service

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type funcRunner func(name string, args ...string) ([]byte, error)

func (f funcRunner) Run(name string, args ...string) ([]byte, error) {
	return f(name, args...)
}

func writeBinary(t *testing.T, path string) string {
	t.Helper()
	if err := os.WriteFile(path, []byte("binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestPlistProgram(t *testing.T) {
	xml, err := RenderPlist(PlistFor("/usr/local/bin/thepurge", 300, false))
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name string
		xml  string
		want string
	}{
		{
			name: "rendered plist",
			xml:  xml,
			want: "/usr/local/bin/thepurge",
		},
		{
			name: "program arguments key only",
			xml:  `<plist><dict><key>ProgramArguments</key><array></array></dict></plist>`,
			want: "",
		},
		{
			name: "no program arguments key",
			xml:  `<plist><dict><key>Label</key><string>br4zz4.thepurge</string></dict></plist>`,
			want: "",
		},
		{
			name: "no strings at all",
			xml:  `<plist><dict></dict></plist>`,
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// act
			got := plistProgram(tt.xml)

			// assert
			if got != tt.want {
				t.Errorf("plistProgram() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestPlistEscaping(t *testing.T) {
	bin := "/tmp/a & b/<the>purge"

	// act
	xml, err := RenderPlist(PlistFor(bin, 300, false))

	// assert
	if err != nil {
		t.Fatalf("RenderPlist() error = %v", err)
	}
	if !strings.Contains(xml, "/tmp/a &amp; b/&lt;the&gt;purge") {
		t.Errorf("plist does not escape ProgramPath; got:\n%s", xml)
	}
	if strings.Contains(xml, "<the>") {
		t.Errorf("plist contains raw angle brackets; got:\n%s", xml)
	}
	if got := plistProgram(xml); got != bin {
		t.Errorf("plistProgram() = %q, want %q", got, bin)
	}
}

func TestStatusMismatch(t *testing.T) {
	tests := []struct {
		name   string
		status Status
		want   bool
	}{
		{
			name:   "loaded with different programs",
			status: Status{Loaded: true, ProgramLoaded: "/old/thepurge", ProgramWanted: "/new/thepurge"},
			want:   true,
		},
		{
			name:   "loaded with same program",
			status: Status{Loaded: true, ProgramLoaded: "/bin/thepurge", ProgramWanted: "/bin/thepurge"},
			want:   false,
		},
		{
			name:   "not loaded",
			status: Status{Loaded: false, ProgramLoaded: "/old/thepurge", ProgramWanted: "/new/thepurge"},
			want:   false,
		},
		{
			name:   "loaded program unknown",
			status: Status{Loaded: true, ProgramLoaded: "", ProgramWanted: "/new/thepurge"},
			want:   false,
		},
		{
			name:   "wanted program unknown",
			status: Status{Loaded: true, ProgramLoaded: "/old/thepurge", ProgramWanted: ""},
			want:   false,
		},
		{
			name:   "zero status",
			status: Status{},
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// act
			got := tt.status.Mismatch()

			// assert
			if got != tt.want {
				t.Errorf("Mismatch() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestReadUnit(t *testing.T) {
	bin := writeBinary(t, filepath.Join(t.TempDir(), "thepurge"))
	absent := filepath.Join(t.TempDir(), "gone", "thepurge")
	xml, err := RenderPlist(PlistFor(bin, 300, false))
	if err != nil {
		t.Fatal(err)
	}
	xmlAbsent, err := RenderPlist(PlistFor(absent, 300, false))
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name        string
		content     string
		write       bool
		wantInst    bool
		wantProgram string
		wantExists  bool
	}{
		{
			name:        "installed with existing binary",
			content:     xml,
			write:       true,
			wantInst:    true,
			wantProgram: bin,
			wantExists:  true,
		},
		{
			name:        "installed with missing binary",
			content:     xmlAbsent,
			write:       true,
			wantInst:    true,
			wantProgram: absent,
		},
		{
			name:  "unit missing",
			write: false,
		},
		{
			name:     "unparseable unit",
			content:  "not a plist at all\n",
			write:    true,
			wantInst: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			unit := filepath.Join(t.TempDir(), "br4zz4.thepurge.plist")
			if tt.write {
				if err := os.WriteFile(unit, []byte(tt.content), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			st := Status{UnitPath: unit}

			// act
			err := readUnit(&st, plistProgram)

			// assert
			if err != nil {
				t.Fatalf("readUnit() error = %v", err)
			}
			if st.Installed != tt.wantInst {
				t.Errorf("Installed = %v, want %v", st.Installed, tt.wantInst)
			}
			if st.ProgramWanted != tt.wantProgram {
				t.Errorf("ProgramWanted = %q, want %q", st.ProgramWanted, tt.wantProgram)
			}
			if st.ProgramExists != tt.wantExists {
				t.Errorf("ProgramExists = %v, want %v", st.ProgramExists, tt.wantExists)
			}
		})
	}
}

func TestReadUnitUnreadable(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root bypasses file permissions")
	}
	unit := filepath.Join(t.TempDir(), "br4zz4.thepurge.plist")
	if err := os.WriteFile(unit, []byte("anything"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(unit, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(unit, 0o644) })
	st := Status{UnitPath: unit}

	// act
	err := readUnit(&st, plistProgram)

	// assert
	if err == nil {
		t.Fatal("readUnit() error = nil, want read error")
	}
	if !strings.Contains(err.Error(), "service status: read") {
		t.Errorf("error = %v, want service status read context", err)
	}
}

func TestReadUnitStatError(t *testing.T) {
	dir := t.TempDir()
	blocker := filepath.Join(dir, "blocker")
	if err := os.WriteFile(blocker, []byte("not a dir"), 0o644); err != nil {
		t.Fatal(err)
	}
	unit := filepath.Join(dir, "br4zz4.thepurge.plist")
	xml, err := RenderPlist(PlistFor(filepath.Join(blocker, "thepurge"), 300, false))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(unit, []byte(xml), 0o644); err != nil {
		t.Fatal(err)
	}
	st := Status{UnitPath: unit}

	// act
	err = readUnit(&st, plistProgram)

	// assert
	if err == nil {
		t.Fatal("readUnit() error = nil, want stat error")
	}
	if !strings.Contains(err.Error(), "service status") {
		t.Errorf("error = %v, want service status context", err)
	}
}

func TestExecStartPath(t *testing.T) {
	tests := []struct {
		name string
		unit string
		want string
	}{
		{
			name: "plain exec start",
			unit: "[Service]\nType=oneshot\nExecStart=/usr/local/bin/thepurge run\n",
			want: "/usr/local/bin/thepurge",
		},
		{
			name: "exec start with apply flag",
			unit: "ExecStart=/opt/bin/thepurge run --apply\n",
			want: "/opt/bin/thepurge",
		},
		{
			name: "padded line without inner padding",
			unit: "[Service]\n   ExecStart=/usr/bin/thepurge   run  \n",
			want: "/usr/bin/thepurge",
		},
		{
			name: "space after equals is tolerated",
			unit: "ExecStart= /usr/bin/thepurge run\n",
			want: "/usr/bin/thepurge",
		},
		{
			name: "space before key is tolerated",
			unit: "   ExecStart=   /usr/bin/thepurge run\n",
			want: "/usr/bin/thepurge",
		},
		{
			name: "ignore-failure prefix",
			unit: "ExecStart=-/bin/thepurge run\n",
			want: "/bin/thepurge",
		},
		{
			name: "argv0 prefix",
			unit: "ExecStart=@/bin/thepurge run\n",
			want: "/bin/thepurge",
		},
		{
			name: "privileged prefix",
			unit: "ExecStart=+/bin/thepurge run\n",
			want: "/bin/thepurge",
		},
		{
			name: "no-env-expand prefix",
			unit: "ExecStart=!/bin/thepurge run\n",
			want: "/bin/thepurge",
		},
		{
			name: "combined prefixes",
			unit: "ExecStart=-@/bin/thepurge run\n",
			want: "/bin/thepurge",
		},
		{
			name: "exec start pre is ignored",
			unit: "[Service]\nExecStartPre=/bin/true\nExecStart=/bin/thepurge run\n",
			want: "/bin/thepurge",
		},
		{
			name: "no exec start",
			unit: "[Service]\nType=oneshot\n",
			want: "",
		},
		{
			name: "empty exec start",
			unit: "ExecStart=\n",
			want: "",
		},
		{
			name: "whitespace only exec start",
			unit: "ExecStart=   \n",
			want: "",
		},
		{
			name: "prefix only",
			unit: "ExecStart=-\n",
			want: "",
		},
		{
			name: "commented out exec start",
			unit: "#ExecStart=/bin/thepurge run\n",
			want: "",
		},
		{
			name: "empty unit",
			unit: "",
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// act
			got := execStartPath(tt.unit)

			// assert
			if got != tt.want {
				t.Errorf("execStartPath() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestExecStartProperty(t *testing.T) {
	tests := []struct {
		name string
		out  string
		want string
	}{
		{
			name: "realistic show output",
			out:  "ExecStart={ path=/usr/local/bin/thepurge ; argv[]=/usr/local/bin/thepurge run ; ignore_errors=no ; start_time=[n/a] ; pid=0 ; code=(null) ; status=0/0 }\n",
			want: "/usr/local/bin/thepurge",
		},
		{
			name: "path and argv on separate lines",
			out:  "ExecStart={ path=/usr/bin/thepurge ;\n\targv[]=/usr/bin/thepurge run\n}\n",
			want: "/usr/bin/thepurge",
		},
		{
			name: "path followed by close brace",
			out:  "ExecStart={ path=/bin/thepurge }\n",
			want: "/bin/thepurge",
		},
		{
			name: "empty property",
			out:  "ExecStart=\n",
			want: "",
		},
		{
			name: "no path key",
			out:  "ExecStart={ argv[]=/bin/thepurge run }\n",
			want: "",
		},
		{
			name: "empty output",
			out:  "",
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// act
			got := execStartProperty(tt.out)

			// assert
			if got != tt.want {
				t.Errorf("execStartProperty() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestProperty(t *testing.T) {
	out := "MainPID=4242\nExecMainStatus=3\nActiveState=active\n"

	tests := []struct {
		name string
		out  string
		key  string
		want string
	}{
		{name: "main pid", out: out, key: "MainPID", want: "4242"},
		{name: "exit status", out: out, key: "ExecMainStatus", want: "3"},
		{name: "active state", out: out, key: "ActiveState", want: "active"},
		{name: "missing key", out: out, key: "UnitFileState", want: ""},
		{name: "padded value", out: "  ActiveState=  active  \n", key: "ActiveState", want: "active"},
		{name: "key prefix is not a match", out: "MainPIDX=1\n", key: "MainPID", want: ""},
		{name: "empty output", out: "", key: "MainPID", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// act
			got := property(tt.out, tt.key)

			// assert
			if got != tt.want {
				t.Errorf("property() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestAtoi(t *testing.T) {
	tests := []struct {
		in   string
		want int
	}{
		{"42", 42},
		{"0", 0},
		{"-9", -9},
		{"-", 0},
		{"", 0},
		{"abc", 0},
	}

	for _, tt := range tests {
		if got := atoi(tt.in); got != tt.want {
			t.Errorf("atoi(%q) = %d, want %d", tt.in, got, tt.want)
		}
	}
}
