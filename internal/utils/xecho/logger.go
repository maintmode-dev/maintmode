package xecho

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/labstack/gommon/log"
	"github.com/ruko1202/xlog"
)

// ZapAdapter adapts *zap.Logger to echo.Logger interface.
type ZapAdapter struct {
	prefix string
	logger xlog.Logger
	level  log.Lvl
}

func NewLogAdapter(logger xlog.Logger) *ZapAdapter {
	return &ZapAdapter{
		logger: logger,
		level:  log.INFO,
	}
}

func (l *ZapAdapter) Output() io.Writer {
	return io.Discard
}

func (l *ZapAdapter) SetOutput(_ io.Writer) {}

func (l *ZapAdapter) Prefix() string {
	return l.prefix
}

func (l *ZapAdapter) SetPrefix(p string) {
	l.prefix = p
	l.logger = l.logger.Named(p)
}

func (l *ZapAdapter) Level() log.Lvl {
	return l.level
}

func (l *ZapAdapter) SetLevel(v log.Lvl) {
	l.level = v
}

func (l *ZapAdapter) SetHeader(_ string) {}

func (l *ZapAdapter) Print(i ...any) {
	l.logger.Info(fmt.Sprint(i...))
}

func (l *ZapAdapter) Printf(format string, args ...any) {
	l.logger.Info(fmt.Sprintf(format, args...))
}

func (l *ZapAdapter) Printj(j log.JSON) {
	l.logger.Info(marshalJSON(j))
}

func (l *ZapAdapter) Debug(i ...any) {
	l.logger.Debug(fmt.Sprint(i...))
}

func (l *ZapAdapter) Debugf(format string, args ...any) {
	l.logger.Debug(fmt.Sprintf(format, args...))
}

func (l *ZapAdapter) Debugj(j log.JSON) {
	l.logger.Debug(marshalJSON(j))
}

func (l *ZapAdapter) Info(i ...any) {
	l.logger.Info(fmt.Sprint(i...))
}

func (l *ZapAdapter) Infof(format string, args ...any) {
	l.logger.Info(fmt.Sprintf(format, args...))
}

func (l *ZapAdapter) Infoj(j log.JSON) {
	l.logger.Info(marshalJSON(j))
}

func (l *ZapAdapter) Warn(i ...any) {
	l.logger.Warn(fmt.Sprint(i...))
}

func (l *ZapAdapter) Warnf(format string, args ...any) {
	l.logger.Warn(fmt.Sprintf(format, args...))
}

func (l *ZapAdapter) Warnj(j log.JSON) {
	l.logger.Warn(marshalJSON(j))
}

func (l *ZapAdapter) Error(i ...any) {
	l.logger.Error(fmt.Sprint(i...))
}

func (l *ZapAdapter) Errorf(format string, args ...any) {
	l.logger.Error(fmt.Sprintf(format, args...))
}

func (l *ZapAdapter) Errorj(j log.JSON) {
	l.logger.Error(marshalJSON(j))
}

func (l *ZapAdapter) Fatal(i ...any) {
	l.logger.Fatal(fmt.Sprint(i...))
}

func (l *ZapAdapter) Fatalf(format string, args ...any) {
	l.logger.Fatal(fmt.Sprintf(format, args...))
}

func (l *ZapAdapter) Fatalj(j log.JSON) {
	l.logger.Fatal(marshalJSON(j))
}

func (l *ZapAdapter) Panic(i ...any) {
	l.logger.Panic(fmt.Sprint(i...))
}

func (l *ZapAdapter) Panicf(format string, args ...any) {
	l.logger.Panic(fmt.Sprintf(format, args...))
}

func (l *ZapAdapter) Panicj(j log.JSON) {
	l.logger.Panic(marshalJSON(j))
}

func marshalJSON(j log.JSON) string {
	b, err := json.Marshal(j)
	if err != nil {
		b = []byte("failed marshal json")
	}
	return string(b)
}
