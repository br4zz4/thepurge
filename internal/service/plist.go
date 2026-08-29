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
	RunAtLoad         bool
	StandardOutPath   string
	StandardErrorPath string
	LogPath           string // where the -service log writes
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
</dict>
</plist>
`

var tpl = template.Must(template.New("plist").Parse(plistTemplate))

// PlistFor returns a Plist for the periodic service at the given binary path,
// waking every intervalSeconds. Logs are written next to the binary's log dir.
func PlistFor(binPath string, intervalSeconds int) Plist {
	logPath := defaultLogPath()
	return Plist{
		Label:             Label,
		ProgramPath:       binPath,
		RunInterval:       intervalSeconds,
		RunAtLoad:         true,
		StandardOutPath:   logPath,
		StandardErrorPath: logPath,
		LogPath:           logPath,
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
