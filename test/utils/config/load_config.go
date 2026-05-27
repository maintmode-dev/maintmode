package testconfigutils

import (
	"github.com/ruko1202/maintmode/internal/config"
)

func LoadMaintConfig() *config.AppConfig {
	cfg := config.LoadAppConfig()
	cfg.Messengers.UseStub = true

	return cfg
}

func LoadAuthConfig() *config.AppConfig {
	cfg := config.LoadAuthAppConfig()
	cfg.OauthProviders.UseStub = true
	cfg.Messengers.UseStub = true

	return cfg
}
