package version

import (
	"fmt"
	"strings"

	"github.com/dogechain-lab/dogechain/command/helper"
)

type VersionResult struct {
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	BuildTime string `json:"buildTime"`
}

func (r *VersionResult) GetOutput() string {
	var s strings.Builder

	s.WriteString("Dogechain\n")
	s.WriteString(helper.FormatKV([]string{
		fmt.Sprintf("Version|%s", r.Version),
		fmt.Sprintf("Commit|%s", r.Commit),
		fmt.Sprintf("Build Time|%s", r.BuildTime),
	}))

	return s.String()
}
