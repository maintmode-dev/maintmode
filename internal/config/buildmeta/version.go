package buildmeta

import "fmt"

var (
	version   = "undefined"
	os        = ""
	arch      = ""
	buildTime = ""
	shaCommit = ""
)

type AppBuildMeta struct {
	Version   string `json:"version"`
	OS        string `json:"os"`
	Arch      string `json:"arch"`
	BuildTime string `json:"build_time"`
	ShaCommit string `json:"sha_commit"`
}

func (v *AppBuildMeta) String() string {
	return fmt.Sprintf(
		"Version: %s, OS: %s, Arch: %s, BuildTime: %s, ShaCommit: %s",
		v.Version, v.OS, v.Arch, v.BuildTime, v.ShaCommit,
	)
}
