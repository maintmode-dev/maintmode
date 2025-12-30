package buildmeta

import "runtime/debug"

var (
	app = &AppBuildMeta{}
)

func GetAppBuildMeta() *AppBuildMeta {
	return app
}

func init() {
	app = initAppBuildMeta()
}

func initAppBuildMeta() *AppBuildMeta {
	meta := &AppBuildMeta{
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
				meta.Arch = setting.Value
			}
		}
	}
}
