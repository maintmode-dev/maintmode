package integration

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/config"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/utils/closer"
	"github.com/ruko1202/maintmode/internal/utils/xuuid"
	testdbconnutils "github.com/ruko1202/maintmode/test/utils/db/conn"
)

var (
	db    *sqlx.DB
	store *Store
)

func TestMain(m *testing.M) {
	db = testdbconnutils.NewDB(config.LoadAppConfig())
	closer.Add(db.Close)

	store = NewStore(db)

	code := m.Run()
	os.Exit(code)
}

// seedDEK inserts a data_keys row directly (the datakey store lives in another
// package) and returns its id, so an integration can satisfy the dek_id FK.
func seedDEK(ctx context.Context, t *testing.T) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	err := db.QueryRowxContext(ctx,
		`INSERT INTO data_keys (kek_id, encrypted_dek) VALUES ($1, $2) RETURNING id`,
		"test-kek", []byte("wrapped")).Scan(&id)
	require.NoError(t, err)
	return id
}

// newSetting builds an integration with a unique kind so parallel runs on the
// shared DB do not collide on UNIQUE(kind).
func newSetting(t *testing.T, dekID uuid.UUID) *entity.IntegrationSetting {
	t.Helper()
	return &entity.IntegrationSetting{
		Kind:            "slack-" + xuuid.NewString(),
		Enabled:         true,
		Config:          json.RawMessage(`{"api_url":"https://slack.test"}`),
		Secrets:         map[string]string{"bot_token": "ciphertext"},
		DEKID:           dekID,
		CreatedByUserID: lo.ToPtr(uuid.New()),
	}
}
