package xvalidation

import (
	"context"
	"time"

	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"
	"github.com/samber/lo"
)

func IsDuration(ctx context.Context, value any) error {
	durationStr, err := Parse[string](value)
	if err != nil {
		return err
	}

	if _, err := time.ParseDuration(lo.FromPtr(durationStr)); err != nil {
		xlog.Error(ctx, "parse duration", xfield.Error(err))
		return validation.NewError("validation_dutaions_invalid", "must be a valid duration [10m, 1h]")
	}

	return nil
}
