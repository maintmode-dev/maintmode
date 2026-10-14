package distributedlock_test

import (
	"os"
	"testing"

	valkeyDB "github.com/redis/go-redis/v9"

	"github.com/ruko1202/maintmode/internal/config"
	"github.com/ruko1202/maintmode/internal/utils/closer"
	testconfigutils "github.com/ruko1202/maintmode/test/utils/config"
	testdbconnutils "github.com/ruko1202/maintmode/test/utils/db/conn"
)

// These tests run against a LIVE Valkey: owner-only release rests on the
// compare and the delete being one atomic script, which a fake would not prove.
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
