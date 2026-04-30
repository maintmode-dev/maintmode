package xvalidation

import (
	"fmt"

	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/google/uuid"
)

func UUIDNotNil(value any) error {
	u, ok := value.(uuid.UUID)
	if !ok {
		return fmt.Errorf("invalid type: %T", value)
	}
	if u == uuid.Nil {
		return validation.ErrNilOrNotEmpty
	}
	return nil
}
