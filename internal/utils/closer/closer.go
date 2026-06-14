// Package closer provides a global registry for cleanup functions to be called on application shutdown.
// It ensures all registered resources are properly closed in a coordinated manner.
package closer

import (
	"context"
	"reflect"
	"runtime"
	"sync"

	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"
)

type closeFunc struct {
	name   string
	f      func(context.Context) error
	closed bool
}

var (
	closeFuncs []*closeFunc
	mu         *sync.Mutex
)

func init() {
	onceFunc := sync.OnceFunc(func() {
		mu = &sync.Mutex{}
		closeFuncs = make([]*closeFunc, 0)
	})
	onceFunc()
}

// NoErrCloseFunc wraps a no-error cleanup function into an error-returning function.
func NoErrCloseFunc(f func()) func() error {
	return func() error {
		f()
		return nil
	}
}

// NoCtxCloseFunc wraps a no-ctx cleanup function
func NoCtxCloseFunc(f func() error) func(ctx context.Context) error {
	return func(_ context.Context) error {
		return f()
	}
}

// Add registers a cleanup function to be called on shutdown using the function's name.
func Add(f func() error) {
	AddWithName(getFuncName(f), NoCtxCloseFunc(f))
}

// AddWithCtx registers a cleanup function to be called on shutdown using the function's name.
func AddWithCtx(f func(ctx context.Context) error) {
	AddWithName(getFuncName(f), f)
}

// AddWithName registers a cleanup function with a custom name to be called on shutdown.
func AddWithName(name string, f func(ctx context.Context) error) {
	mu.Lock()
	defer mu.Unlock()

	closeFuncs = append(closeFuncs, &closeFunc{
		name:   name,
		f:      f,
		closed: false,
	})
}

// CloseAll executes all registered cleanup functions in order with panic recovery.
func CloseAll(ctx context.Context) {
	ctx = xlog.WithOperation(ctx, "closer.CloseAll")
	xlog.Info(ctx, "closing all handlers...")

	mu.Lock()
	defer mu.Unlock()

	for _, f := range closeFuncs {
		doClose(ctx, f)
	}
}

func doClose(ctx context.Context, closeFunc *closeFunc) {
	ctx = xlog.WithFields(ctx, xfield.String("name", closeFunc.name))
	xlog.Info(ctx, "closing handler")

	defer func() {
		if err := recover(); err != nil {
			xlog.Error(ctx, "close handler panic", xfield.Any("panic", err))
		}
	}()

	err := closeFunc.f(ctx)
	if err != nil {
		xlog.Error(ctx, "close handler failed", xfield.Error(err))
		return
	}
	closeFunc.closed = true

	xlog.Info(ctx, "close handler completed")
}

func getFuncName(f any) string {
	return runtime.FuncForPC(reflect.ValueOf(f).Pointer()).Name()
}
