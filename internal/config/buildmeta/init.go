// Package buildmeta provides application build metadata including version, OS, and architecture.
// Metadata is set at build time via ldflags and exposed through GetAppBuildMeta.
package buildmeta

import "runtime/debug"

const (
	MaintModeAppName = "maintmode"
	AuthAppName      = "auth"
)

var (
	maintmodeApp = &AppBuildMeta{}
	authApp      = &AppBuildMeta{}
)

// GetAppBuildMeta returns the initialized application build metadata.
func GetAppBuildMeta() *AppBuildMeta {
	return maintmodeApp
}

// GetAuthAppBuildMeta returns the initialized application build metadata.
func GetAuthAppBuildMeta() *AppBuildMeta {
	return authApp
}

func init() {
	maintmodeApp = initAppBuildMeta(MaintModeAppName)
	authApp = initAppBuildMeta(AuthAppName)
}

func initAppBuildMeta(appName string) *AppBuildMeta {
	meta := &AppBuildMeta{
		AppName:   appName,
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
