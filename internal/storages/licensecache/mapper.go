package licensecache

import (
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/pkg/generated/maintmode/public/model"
)

func toDB(license *entity.License) *model.LicenseCache {
	return &model.LicenseCache{
		ID:             true,
		Status:         string(license.Status),
		SeatsPurchased: license.SeatsPurchased,
		FetchedAt:      license.FetchedAt,
	}
}

func fromDB(license *model.LicenseCache) *entity.License {
	return &entity.License{
		Status:         entity.LicenseStatus(license.Status),
		SeatsPurchased: license.SeatsPurchased,
		FetchedAt:      license.FetchedAt,
	}
}
