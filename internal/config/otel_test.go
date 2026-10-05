package config

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/config/buildmeta"
)

// The semconv import must follow the otel SDK: resource.Default() carries the
// SDK's schema URL, and merging it with a resource built on another semconv
// version fails, which aborts startup before anything is served.
func TestInitTracerResource(t *testing.T) {
	_, err := InitTracerResource(&buildmeta.AppBuildMeta{AppName: "maintmode", Version: "test", ShaCommit: "abc"})
	require.NoError(t, err)
}
