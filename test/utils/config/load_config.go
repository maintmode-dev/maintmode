package testconfigutils

import (
	"github.com/ruko1202/maintmode/internal/config"
)

func LoadMaintConfig() *config.AppConfig {
	cfg := config.LoadAppConfig()
	cfg.NotifyTransport.UseStub = true

	return cfg
}

func LoadAuthConfig() *config.AppConfig {
	cfg := config.LoadAppConfig()
	cfg.OauthProviders.UseStub = true
	cfg.NotifyTransport.UseStub = true

	return cfg
}
