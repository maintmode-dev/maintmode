package datakey

import (
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/pkg/generated/maintmode/public/model"
)

func toDB(dk *entity.DataKey) *model.DataKeys {
	return &model.DataKeys{
		ID:           dk.ID,
		KekID:        dk.KEKID,
		EncryptedDek: dk.EncryptedDEK,
		Label:        dk.Label,
		CreatedAt:    dk.CreatedAt,
	}
}

func fromDB(m *model.DataKeys) *entity.DataKey {
	return &entity.DataKey{
		ID:           m.ID,
		KEKID:        m.KekID,
		EncryptedDEK: m.EncryptedDek,
		Label:        m.Label,
		CreatedAt:    m.CreatedAt,
	}
}

func fromDBList(models []model.DataKeys) []*entity.DataKey {
	out := make([]*entity.DataKey, 0, len(models))
	for i := range models {
		out = append(out, fromDB(&models[i]))
	}
	return out
}
