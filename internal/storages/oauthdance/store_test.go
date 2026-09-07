package oauthdance_test

import (
	"context"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/storages/oauthdance"
	"github.com/ruko1202/maintmode/internal/utils/xhash"
	"github.com/ruko1202/maintmode/internal/utils/xuuid"
)

// newStore builds a store against the live Valkey. Codes are a per-run random
// secret, so parallel runs (make tloc uses -count 2) never collide on a key.
func newStore(t *testing.T) *oauthdance.Store {
	t.Helper()

	return oauthdance.NewStore(valkey)
}

func randomSecret(t *testing.T) string {
	t.Helper()

	return xuuid.NewString()
}

// TestConsumeCodeRoundTripPreservesSessionID guards the serialization choice.
// entity.TokenPair.SessionID is documented as never appearing in the API
// response, so encoding through the response DTO would drop it silently and the
// login audit would lose its session correlation.
func TestConsumeCodeRoundTripPreservesSessionID(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := newStore(t)
	code := randomSecret(t)

	want := &entity.TokenPair{
		AccessToken:  "access",
		RefreshToken: "refresh",
		ExpiresIn:    900,
		SessionID:    uuid.New(),
	}
	require.NoError(t, store.PutCode(ctx, code, want))

	got, err := store.ConsumeCode(ctx, code)
	require.NoError(t, err)
	require.NotNil(t, got)

	assert.Equal(t, want.AccessToken, got.AccessToken)
	assert.Equal(t, want.RefreshToken, got.RefreshToken)
	assert.Equal(t, want.ExpiresIn, got.ExpiresIn)
	assert.Equal(t, want.SessionID, got.SessionID, "SessionID must survive the round trip")
}

func TestConsumeCodeSingleUse(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := newStore(t)
	code := randomSecret(t)

	require.NoError(t, store.PutCode(ctx, code, &entity.TokenPair{AccessToken: "a"}))

	got, err := store.ConsumeCode(ctx, code)
	require.NoError(t, err)
	require.NotNil(t, got)

	got, err = store.ConsumeCode(ctx, code)
	require.NoError(t, err)
	assert.Nil(t, got, "a redeemed code must never be redeemable again")
}

// TestConsumeCodeConcurrent is the test a GET-then-DEL implementation fails and
// an in-memory fake would not: N racers, exactly one token pair handed out.
// TestConsumeCodeConcurrent is the test a GET-then-DEL implementation must
// fail, and the reason it is written this way rather than the obvious way.
//
// The obvious version — spawn N goroutines, hope they collide — PASSES against
// a GET-then-DEL store, verified by mutation ten runs out of ten. Goroutines do
// not start together: the first one completes its GET and its DEL before the
// others reach their GET, so the race window never opens and the test proves
// nothing.
//
// Two things fix that. Every racer blocks on a shared channel and is released
// at once, so they enter ConsumeCode inside the same instant. And the whole
// thing repeats: a race is a probability, not a certainty, and one round can
// legitimately serialize on its own.
func TestConsumeCodeConcurrent(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := newStore(t)

	const (
		racers = 8
		rounds = 20
	)

	for round := range rounds {
		code := randomSecret(t)
		require.NoError(t, store.PutCode(ctx, code, &entity.TokenPair{AccessToken: "only-one"}))

		var (
			wg      sync.WaitGroup
			mu      sync.Mutex
			winners int
		)

		start := make(chan struct{})

		wg.Add(racers)
		for range racers {
			go func() {
				defer wg.Done()

				<-start // every racer waits here, so they all enter together

				pair, err := store.ConsumeCode(ctx, code)
				assert.NoError(t, err)

				if pair != nil {
					mu.Lock()
					winners++
					mu.Unlock()
				}
			}()
		}

		close(start)
		wg.Wait()

		require.Equal(t, 1, winners,
			"exactly one racer may redeem the code (round %d)", round)
	}
}

// TestKeysAreHashed proves the code itself never becomes a Valkey key. An
// operator running KEYS, a slow-log entry or a memory dump must not yield
// anything replayable.
func TestKeysAreHashed(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := newStore(t)
	code := randomSecret(t)

	require.NoError(t, store.PutCode(ctx, code, &entity.TokenPair{AccessToken: "a"}))

	found, err := valkey.Keys(ctx, "*"+code+"*").Result()
	require.NoError(t, err)
	assert.Empty(t, found, "the raw code must not appear in any key")
}

// TestCodeExpires proves the stored pair carries an expiry at all.
//
// That is the whole assertion, and it is not a weak one: a key written with no
// TTL lives forever, and an immortal one-time code is the exact opposite of
// what this store is for. Verified by mutation — dropping the TTL from the
// constructor fails only here.
//
// It deliberately does NOT compare against the configured value. Reading the
// same constant the code wrote and asserting they match is a tautology: it
// stays green when the lifetime is stretched from a minute to half an hour,
// which is the change that would actually matter.
func TestCodeExpires(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	code := randomSecret(t)

	require.NoError(t, newStore(t).PutCode(ctx, code, &entity.TokenPair{AccessToken: "a"}))

	ttl, err := valkey.TTL(ctx, codeKeyForTest(code)).Result()
	require.NoError(t, err)

	assert.Positive(t, ttl, "a one-time code with no expiry is redeemable forever")
}

// codeKeyForTest rebuilds the store's key so this test can look the entry up.
//
// It duplicates the scheme on purpose rather than exporting it: a helper the
// production code also used would agree with itself by construction, including
// when both are wrong. TestKeysAreHashed is what pins the scheme, by searching
// for the raw code instead of computing anything.
func codeKeyForTest(code string) string {
	return "oauth:code:" + xhash.HashSha256([]byte(code))
}
