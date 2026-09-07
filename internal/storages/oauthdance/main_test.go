package oauthdance_test

import (
	"os"
	"testing"

	valkeyDB "github.com/redis/go-redis/v9"

	"github.com/ruko1202/maintmode/internal/config"
	"github.com/ruko1202/maintmode/internal/utils/closer"
	testconfigutils "github.com/ruko1202/maintmode/test/utils/config"
	testdbconnutils "github.com/ruko1202/maintmode/test/utils/db/conn"
)

// These tests run against a LIVE Valkey, deliberately. The store's single-use
// guarantee rests on GETDEL being one atomic round trip; an in-memory fake would
// happily pass a GET-then-DEL implementation, which is precisely the bug these
// tests exist to catch.
var (
	valkey *valkeyDB.Client
	cfg    *config.AppConfig
)

func TestMain(m *testing.M) {
	cfg = testconfigutils.LoadAuthConfig()

	valkey = testdbconnutils.NewValkeyClient(cfg)
	closer.Add(valkey.Close)

	code := m.Run()

	os.Exit(code)
}
