package service

import (
	"bytes"
	"text/template"
)

// Label is the launchd job identifier for The Purge's agent.
const Label = "br4zz4.thepurge"

// Plist captures the launchd agent configuration for the periodic service.
type Plist struct {
	Label             string
	ProgramPath       string // absolute path to the thepurge binary
	RunInterval       int    // seconds between StartInterval runs
	Apply             bool   // when true, the agent runs `thepurge run --apply` (deletes for real)
	RunAtLoad         bool
	StandardOutPath   string
	StandardErrorPath string
	LogPath           string // where the -service log writes
	KeepAlive         bool   // restart if agent dies
	ThrottleInterval  int    // minimum seconds between restarts (0 = default)
}

const plistTemplate = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>{{.Label}}</string>
	<key>ProgramArguments</key>
	<array>
		<string>{{.ProgramPath}}</string>
		<string>run</string>
		{{if .Apply}}<string>--apply</string>{{end}}
	</array>
	<key>StartInterval</key>
	<integer>{{.RunInterval}}</integer>
	<key>RunAtLoad</key>
	<{{if .RunAtLoad}}true{{else}}false{{end}}/>
	<key>ProcessType</key>
	<string>Background</string>
	<key>StandardOutPath</key>
	<string>{{.StandardOutPath}}</string>
	<key>StandardErrorPath</key>
	<string>{{.StandardErrorPath}}</string>
	<key>KeepAlive</key>
	<{{if .KeepAlive}}true{{else}}false{{end}}/>
	<key>ThrottleInterval</key>
	<integer>{{.ThrottleInterval}}</integer>
</dict>
</plist>
`

var tpl = template.Must(template.New("plist").Parse(plistTemplate))

// PlistFor returns a Plist for the periodic service at the given binary path,
// waking every intervalSeconds. When apply is true the scheduled run deletes
// for real; otherwise it stays in dry-run. Logs go to the standard log dir.
func PlistFor(binPath string, intervalSeconds int, apply bool) Plist {
	logPath := defaultLogPath()
	return Plist{
		Label:             Label,
		ProgramPath:       binPath,
		RunInterval:       intervalSeconds,
		Apply:             apply,
		RunAtLoad:         true,
		StandardOutPath:   logPath,
		StandardErrorPath: logPath,
		LogPath:           logPath,
		KeepAlive:         true,
		ThrottleInterval:  30,
	}
}

// RenderPlist serialises a Plist to the launchd XML property list format.
func RenderPlist(p Plist) (string, error) {
	var buf bytes.Buffer
	if p.Label == "" {
		p.Label = Label
	}
	if err := tpl.Execute(&buf, p); err != nil {
		return "", err
	}
	return buf.String(), nil
}
