package version

import (
	"fmt"
	"runtime"
	"runtime/debug"
)

var (
	Version   = "0.1.0-dev"
	Commit    = "unknown"
	BuildTime = "unknown"
)

type Info struct {
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	BuildTime string `json:"build_time"`
	GoVersion string `json:"go_version"`
	Platform  string `json:"platform"`
	Protocol  int    `json:"protocol_version"`
}

func Get() Info {
	v, c, b := Version, Commit, BuildTime
	if bi, ok := debug.ReadBuildInfo(); ok {
		if v == "" || v == "0.1.0-dev" {
			if bi.Main.Version != "" && bi.Main.Version != "(devel)" {
				v = bi.Main.Version
			}
		}
		for _, s := range bi.Settings {
			switch s.Key {
			case "vcs.revision":
				if c == "unknown" && len(s.Value) >= 7 {
					c = s.Value[:7]
				}
			case "vcs.time":
				if b == "unknown" {
					b = s.Value
				}
			}
		}
	}
	return Info{
		Version:   v,
		Commit:    c,
		BuildTime: b,
		GoVersion: runtime.Version(),
		Platform:  runtime.GOOS + "/" + runtime.GOARCH,
	}
}

func (i Info) Line() string {
	return fmt.Sprintf("hdkt %s (%s, built %s, %s)", i.Version, i.Commit, i.BuildTime, i.Platform)
}
