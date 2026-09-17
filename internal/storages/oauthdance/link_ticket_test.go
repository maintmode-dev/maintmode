package oauthdance_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/utils/xhash"
)

func linkIntent() entity.LinkIntent {
	return entity.LinkIntent{UserID: uuid.New(), Provider: entity.AuthMethodGithub}
}

// TestLinkTicketRoundTrip pins both fields surviving the encoding.
//
// Both are load-bearing at the callback: the user id decides WHOSE account an
// identity is attached to, and the provider is what makes a ticket minted for
// one provider unusable on another's /start.
func TestLinkTicketRoundTrip(t *testing.T) {
	t.Parallel()

	store := newStore(t)
	ticket := randomSecret(t)
	intent := linkIntent()

	require.NoError(t, store.PutLinkTicket(t.Context(), ticket, intent))

	got, err := store.ConsumeLinkTicket(t.Context(), ticket)
	require.NoError(t, err)
	require.NotNil(t, got)

	assert.Equal(t, intent.UserID, got.UserID)
	assert.Equal(t, intent.Provider, got.Provider)
}

// TestPeekLinkTicketDoesNotConsume is the difference between the two reads, and
// the reason there are two.
//
// /start has to validate a ticket -- does it exist, was it minted for this
// provider -- before sending the browser to GitHub. Spending it there would
// leave the callback with nothing to redeem, so the spend belongs at the
// callback, where the link actually happens.
func TestPeekLinkTicketDoesNotConsume(t *testing.T) {
	t.Parallel()

	store := newStore(t)
	ticket := randomSecret(t)
	intent := linkIntent()

	require.NoError(t, store.PutLinkTicket(t.Context(), ticket, intent))

	first, err := store.PeekLinkTicket(t.Context(), ticket)
	require.NoError(t, err)
	require.NotNil(t, first)

	second, err := store.PeekLinkTicket(t.Context(), ticket)
	require.NoError(t, err)
	require.NotNil(t, second, "a peek must leave the ticket redeemable")

	consumed, err := store.ConsumeLinkTicket(t.Context(), ticket)
	require.NoError(t, err)
	require.NotNil(t, consumed)
	assert.Equal(t, intent.UserID, consumed.UserID)
}

// TestConsumeLinkTicketIsSingleUse is what makes a replay useless.
//
// The second consume must find nothing. With the spend at the callback, that is
// what stops a captured cookie from linking twice -- or from linking at all
// after the first completion.
func TestConsumeLinkTicketIsSingleUse(t *testing.T) {
	t.Parallel()

	store := newStore(t)
	ticket := randomSecret(t)

	require.NoError(t, store.PutLinkTicket(t.Context(), ticket, linkIntent()))

	first, err := store.ConsumeLinkTicket(t.Context(), ticket)
	require.NoError(t, err)
	require.NotNil(t, first)

	second, err := store.ConsumeLinkTicket(t.Context(), ticket)
	require.NoError(t, err)
	assert.Nil(t, second, "a spent ticket must redeem to nothing")
}

// TestLinkTicketMissIsNotAnError is the distinction the callback's whole guard
// rests on.
//
// A clean miss and a store failure must be TELLABLE APART by the caller: a miss
// is a refusal the user can be told about, while a failure must never be read as
// "no ticket" -- that would fall through to the sign-in branch and mint a
// session for whichever account was just authenticated. Flattening this into a
// boolean is what would make that mistake possible.
func TestLinkTicketMissIsNotAnError(t *testing.T) {
	t.Parallel()

	store := newStore(t)

	peeked, err := store.PeekLinkTicket(t.Context(), randomSecret(t))
	require.NoError(t, err)
	assert.Nil(t, peeked)

	consumed, err := store.ConsumeLinkTicket(t.Context(), randomSecret(t))
	require.NoError(t, err)
	assert.Nil(t, consumed)
}

// TestLinkTicketKeyIsHashed pins that the secret never becomes a key.
//
// An operator running KEYS, a slow-log entry or a memory dump must not hand
// anyone a usable ticket -- the same rule the code and invitation entries
// already follow.
func TestLinkTicketKeyIsHashed(t *testing.T) {
	t.Parallel()

	store := newStore(t)
	ticket := randomSecret(t)

	require.NoError(t, store.PutLinkTicket(t.Context(), ticket, linkIntent()))

	require.Zero(t, valkey.Exists(t.Context(), "oauth:link:"+ticket).Val(),
		"the raw ticket must not be a key")
	require.Equal(t, int64(1),
		valkey.Exists(t.Context(), "oauth:link:"+xhash.HashSha256([]byte(ticket))).Val())
}

// TestLinkTicketExpires pins that the entry carries a TTL at all.
//
// The value is policy the caller supplies; what must never happen is a ticket
// living forever, since it is a bearer credential that attaches a sign-in method
// to an account.
func TestLinkTicketExpires(t *testing.T) {
	t.Parallel()

	store := newStore(t)
	ticket := randomSecret(t)

	require.NoError(t, store.PutLinkTicket(t.Context(), ticket, linkIntent()))

	ttl := valkey.TTL(t.Context(), "oauth:link:"+xhash.HashSha256([]byte(ticket))).Val()
	assert.Positive(t, ttl, "a link ticket must expire on its own")
}
