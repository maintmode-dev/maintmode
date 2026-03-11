// Package buildmeta provides application build metadata including version, OS, and architecture.
// Metadata is set at build time via ldflags and exposed through GetAppBuildMeta.
package buildmeta

import "runtime/debug"

var (
	app = &AppBuildMeta{}
)

// GetAppBuildMeta returns the initialized application build metadata.
func GetAppBuildMeta() *AppBuildMeta {
	return app
}

func init() {
	app = initAppBuildMeta()
}

func initAppBuildMeta() *AppBuildMeta {
	meta := &AppBuildMeta{
		AppName:   "maintmode",
		Version:   version,
		OS:        os,
		Arch:      arch,
		BuildTime: buildTime,
		ShaCommit: shaCommit,
	}

	if meta.Arch == "" || meta.OS == "" {
		applyFromBuildInfo(meta)
	}

	return meta
}

func applyFromBuildInfo(meta *AppBuildMeta) {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return
	}

	for _, setting := range info.Settings {
		switch setting.Key {
		case "GOARCH":
			if meta.Arch == "" {
				meta.Arch = setting.Value
			}
		case "GOOS":
			if meta.OS == "" {
				meta.OS = setting.Value
			}
		}
	}
}
