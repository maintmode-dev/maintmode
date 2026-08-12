package config

import (
	"context"
)

type Environment string

// Typed rather than untyped string constants so a future comparison against a
// raw string cannot silently succeed against the wrong type.
const (
	ProdEnvironment            Environment = "prod"
	DevEnvironment             Environment = "dev"
	LocalEnvironment           Environment = "local"
	PerformanceTestEnvironment Environment = "performance_test"
)

func (e Environment) IsProd() bool {
	return e == ProdEnvironment
}

func (e Environment) IsDev() bool {
	return e == DevEnvironment || e.IsLocal() || e.IsPerformanceTest()
}

func (e Environment) IsLocal() bool {
	return e == LocalEnvironment
}

func (e Environment) IsPerformanceTest() bool {
	return e == PerformanceTestEnvironment
}

type envCtxKeyType int

const envCtxKey envCtxKeyType = iota

func ContextWithEnv(ctx context.Context, env Environment) context.Context {
	return context.WithValue(ctx, envCtxKey, env)
}

func EnvFromContext(ctx context.Context) Environment {
	val := ctx.Value(envCtxKey)
	if val == nil {
		return DevEnvironment
	}

	env, ok := val.(Environment)
	if !ok {
		return DevEnvironment
	}

	return env
}
