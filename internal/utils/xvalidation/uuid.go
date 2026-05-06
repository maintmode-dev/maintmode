package xvalidation

import (
	"fmt"

	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/google/uuid"
	"github.com/samber/lo"
)

func UUIDNotNil(value any) error {
	var val string
	switch v := value.(type) {
	case *string:
		val = lo.FromPtr(v)
	case string:
		val = v
	case uuid.UUID:
		val = v.String()
	case *uuid.UUID:
		val = lo.FromPtr(v).String()
	default:
		return fmt.Errorf("invalid type: %T", value)
	}

	if val == uuid.Nil.String() {
		return validation.ErrNilOrNotEmpty
	}
	return nil
}
