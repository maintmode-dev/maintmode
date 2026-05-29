package xvalidation

import (
	"fmt"

	validation "github.com/go-ozzo/ozzo-validation/v4"
)

func Parse[T any](value any) (*T, error) {
	var val T
	switch v := value.(type) {
	case *T:
		val = *v
	case T:
		val = v
	default:
		return nil, validation.NewError("validation_invalid", fmt.Sprintf("must be a %T", val))
	}

	return &val, nil
}
