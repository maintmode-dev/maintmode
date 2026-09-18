package authsettingsapi

import (
	apimodels "github.com/ruko1202/maintmode/internal/app/api/public/authsettings/models"
	"github.com/ruko1202/maintmode/internal/entity"
)

func toAPI(s *entity.AuthMethodSetting) apimodels.AuthMethodSetting {
	return apimodels.AuthMethodSetting{
		Method:    string(s.Method),
		Enabled:   s.Enabled,
		UpdatedAt: s.UpdatedAt,
	}
}

func toAPIList(settings []*entity.AuthMethodSetting) []apimodels.AuthMethodSetting {
	out := make([]apimodels.AuthMethodSetting, 0, len(settings))
	for _, s := range settings {
		out = append(out, toAPI(s))
	}

	return out
}
