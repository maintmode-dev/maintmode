package authsettings

import (
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/pkg/generated/maintmode/public/model"
)

// toDB converts the entity into the generated row shape.
//
// CreatedAt is not carried: the column defaults on insert and nothing here ever
// inserts. UpdatedAt is stamped by the caller in update.go, where the store owns
// the modification timestamp.
func toDB(s *entity.AuthMethodSetting) *model.AuthSettings {
	return &model.AuthSettings{
		ID:              s.ID,
		Method:          string(s.Method),
		Enabled:         s.Enabled,
		CreatedAt:       s.CreatedAt,
		UpdatedAt:       s.UpdatedAt,
		UpdatedByUserID: s.UpdatedByUserID,
	}
}

// fromDB is toDB's inverse.
//
// The stored method string is widened into the typed name without validating
// it. Validation belongs to the service, which owns the closed set; a store that
// refused an unknown value would turn a row somebody inserted by hand into an
// error on every LIST, hiding the rest of the table behind it.
func fromDB(m *model.AuthSettings) *entity.AuthMethodSetting {
	return &entity.AuthMethodSetting{
		ID:              m.ID,
		Method:          entity.AuthMethodName(m.Method),
		Enabled:         m.Enabled,
		CreatedAt:       m.CreatedAt,
		UpdatedAt:       m.UpdatedAt,
		UpdatedByUserID: m.UpdatedByUserID,
	}
}
