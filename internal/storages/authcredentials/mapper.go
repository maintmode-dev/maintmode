package authcredentials

import (
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/pkg/generated/maintmode/public/model"
)

func fromDB(r *model.AuthCredentials) *entity.AuthCredential {
	return &entity.AuthCredential{
		ID:           r.ID,
		UserID:       r.UserID,
		Kind:         entity.AuthCredentialKind(r.Kind),
		SecretHash:   r.SecretHash,
		ExpiresAt:    r.ExpiresAt,
		ConsumedAt:   r.ConsumedAt,
		Attempts:     r.Attempts,
		SessionNonce: r.SessionNonce,
		CreatedAt:    r.CreatedAt,
		UpdatedAt:    r.UpdatedAt,
	}
}

func toDB(c *entity.AuthCredential) *model.AuthCredentials {
	return &model.AuthCredentials{
		ID:           c.ID,
		UserID:       c.UserID,
		Kind:         string(c.Kind),
		SecretHash:   c.SecretHash,
		ExpiresAt:    c.ExpiresAt,
		ConsumedAt:   c.ConsumedAt,
		Attempts:     c.Attempts,
		SessionNonce: c.SessionNonce,
	}
}
