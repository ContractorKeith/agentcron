package service

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"strconv"
	"strings"
)

func systemdQuote(s string) string {
	s = strings.ReplaceAll(s, "%", "%%")
	s = strings.ReplaceAll(s, "$", "$$")
	return strconv.Quote(s)
}
func xmlText(s string) string {
	var b bytes.Buffer
	_ = xml.EscapeText(&b, []byte(s))
	return b.String()
}
func Render(o Options) (Definition, error) {
	d := Definition{Path: o.file()}
	switch o.OS {
	case "linux":
		args := o.args()
		for i := range args {
			args[i] = systemdQuote(args[i])
		}
		// Environment does not perform command-line dollar expansion.
		env := strconv.Quote("PATH=" + strings.ReplaceAll(o.Path, "%", "%%"))
		d.Unit = "[Unit]\nDescription=agentcron scheduler\n\n[Service]\nExecStart=" + strings.Join(args, " ") + "\nRestart=on-failure\nEnvironment=" + env + "\n\n[Install]\nWantedBy=default.target\n"
	case "darwin":
		var b strings.Builder
		b.WriteString("<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<!DOCTYPE plist PUBLIC \"-//Apple//DTD PLIST 1.0//EN\" \"http://www.apple.com/DTDs/PropertyList-1.0.dtd\">\n<plist version=\"1.0\"><dict>\n<key>Label</key><string>com.contractorkeith.agentcron</string>\n<key>ProgramArguments</key><array>")
		for _, a := range o.args() {
			b.WriteString("<string>" + xmlText(a) + "</string>")
		}
		b.WriteString("</array>\n<key>RunAtLoad</key><true/>\n<key>KeepAlive</key><dict><key>SuccessfulExit</key><false/></dict>\n<key>EnvironmentVariables</key><dict><key>PATH</key><string>" + xmlText(o.Path) + "</string></dict>\n")
		for _, k := range []string{"StandardOutPath", "StandardErrorPath"} {
			b.WriteString("<key>" + k + "</key><string>" + xmlText(o.DataDir+"/daemon.log") + "</string>\n")
		}
		b.WriteString("</dict></plist>\n")
		d.Unit = b.String()
	default:
		return d, fmt.Errorf("unsupported service OS %q", o.OS)
	}
	return d, nil
}
