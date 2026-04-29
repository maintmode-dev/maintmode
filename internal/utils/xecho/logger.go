package xecho

import (
	"context"
	"log/slog"

	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"
	"github.com/samber/lo"
)

var _ slog.Handler = (*slogHandler)(nil)

func NewSlogAdapter(logger xlog.Logger) *slog.Logger {
	return slog.New(newSlogHandler(logger))
}

type slogHandler struct {
	logger xlog.Logger
}

func newSlogHandler(logger xlog.Logger) *slogHandler {
	return &slogHandler{
		logger: logger,
	}
}

func (h *slogHandler) Enabled(context.Context, slog.Level) bool {
	return true
}

func (h *slogHandler) Handle(_ context.Context, r slog.Record) error {
	fields := make([]xfield.Field, 0, r.NumAttrs())

	r.Attrs(func(attr slog.Attr) bool {
		fields = append(fields, attrToField(attr))
		return true
	})

	switch {
	case r.Level >= slog.LevelError:
		h.logger.Error(r.Message, fields...)
	case r.Level == slog.LevelWarn:
		h.logger.Warn(r.Message, fields...)
	case r.Level == slog.LevelInfo:
		h.logger.Info(r.Message, fields...)
	case r.Level == slog.LevelDebug:
		h.logger.Debug(r.Message, fields...)
	default:
		h.logger.Info(r.Message, fields...)
	}
	return nil
}

func (h *slogHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	fields := lo.Map(attrs, func(item slog.Attr, _ int) xfield.Field {
		return attrToField(item)
	})

	return newSlogHandler(h.logger.With(fields...))
}

func (h *slogHandler) WithGroup(name string) slog.Handler {
	return newSlogHandler(h.logger.Named(name))
}

func attrToField(attr slog.Attr) xfield.Field {
	attr.Value = attr.Value.Resolve()
	key := attr.Key

	return xfield.Any(key, attr.Value.Any())
}
