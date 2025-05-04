package version

import (
	"encoding/json"
	"runtime"
)

// Use `-ldflags` to set these variables at build time
// Example: go build -ldflags "-X 'github.com/yourusername/yourapp/internal/util/version.BuildVersion=1.0.0' -X 'github.com/yourusername/yourapp/internal/util/version.GitCommit=abc123' -X 'github.com/yourusername/yourapp/internal/util/version.BuildTime=$(date -u +%Y-%m-%dT%H:%M:%SZ)'"
// to set the version information dynamically.
var (
	BuildVersion = "N/A"
	BuildTime    = "N/A"
	GitCommit    = "N/A"
)

type AppInfo struct {
	Name       string `json:"AppName"`
	AppVersion string `json:"AppVersion"`
	BuildTime  string `json:"BuildTime"`
	GitCommit  string `json:"GitCommit"`
	GoVersion  string `json:"GoVersion"`
	Compiler   string `json:"Compiler"`
	Platform   string `json:"Platform"`
}

func NewAppInfo(name string) *AppInfo {
	return &AppInfo{
		Name:       name,
		AppVersion: BuildVersion,
		GitCommit:  GitCommit,
		BuildTime:  BuildTime,
		GoVersion:  runtime.Version(),
		Compiler:   runtime.Compiler,
		Platform:   runtime.GOOS + "/" + runtime.GOARCH,
	}
}

func (a *AppInfo) String() string {
	data, err := json.Marshal(a)
	if err != nil {
		return "Error marshaling AppInfo"
	}
	return string(data)
}
