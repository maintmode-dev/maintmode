package xvalidation

import (
	"context"
	"time"

	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"
)

func IsDuration(ctx context.Context, value any) error {
	var durationStr string
	switch v := value.(type) {
	case *string:
		durationStr = *v
	case string:
		durationStr = v
	default:
		return validation.NewError("validation_dutaions_invalid", "must be a string")
	}

	_, err := time.ParseDuration(durationStr)
	if err != nil {
		xlog.Error(ctx, "parse duration", xfield.Error(err))
		return validation.NewError("validation_dutaions_invalid", "must be a valid duration [10m, 1h]")
	}

	return nil
}
