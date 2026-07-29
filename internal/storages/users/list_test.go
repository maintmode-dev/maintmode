package users

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/entity"
)

// TestListExcludeBlocked verifies the assignment-picker filter: with
// ExcludeBlocked the blocked user is hidden, without it the admin list still
// sees the blocked user, and block→hide / unblock→show round-trips correctly.
func TestListExcludeBlocked(t *testing.T) {
	// Subtests block then unblock one shared row in order, so they run
	// sequentially; the parent test is deliberately not parallelized.

	ctx := context.Background()
	store := NewStore(db)

	// Unique token isolates this test's rows from other parallel tests sharing
	// the DB, so Total counts are deterministic.
	token := "picker-" + uuid.NewString()

	active, err := store.Create(ctx, &entity.User{
		Email: uuid.NewString() + "@email.com",
		Name:  "Active " + token,
		Roles: entity.DefaultRoles,
	})
	require.NoError(t, err)

	blocked, err := store.Create(ctx, &entity.User{
		Email: uuid.NewString() + "@email.com",
		Name:  "Blocked " + token,
		Roles: entity.DefaultRoles,
	})
	require.NoError(t, err)

	blockedAt := time.Now()
	blocked.BlockedAt = &blockedAt
	require.NoError(t, store.Update(ctx, blocked))

	listIDs := func(excludeBlocked bool) []uuid.UUID {
		t.Helper()
		users, total, err := store.List(ctx, &entity.ListUsersCmd{
			Search:         token,
			Limit:          50,
			Offset:         0,
			ExcludeBlocked: excludeBlocked,
		})
		require.NoError(t, err)
		require.Equal(t, int64(len(users)), total)
		return lo.Map(users, func(u *entity.User, _ int) uuid.UUID { return u.ID })
	}

	t.Run("ExcludeBlocked hides blocked user", func(t *testing.T) {
		ids := listIDs(true)
		require.Contains(t, ids, active.ID)
		require.NotContains(t, ids, blocked.ID)
	})

	t.Run("admin list (no exclude) keeps blocked user", func(t *testing.T) {
		ids := listIDs(false)
		require.Contains(t, ids, active.ID)
		require.Contains(t, ids, blocked.ID)
	})

	t.Run("unblock makes the user visible to the picker again", func(t *testing.T) {
		blocked.BlockedAt = nil
		require.NoError(t, store.Update(ctx, blocked))

		ids := listIDs(true)
		require.Contains(t, ids, active.ID)
		require.Contains(t, ids, blocked.ID)
	})
}

// TestListRolesFilter verifies the roles[] filter is OR: a user matches when it
// has ANY of the listed roles. The token isolates this test's rows.
func TestListRolesFilter(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := NewStore(db)
	token := "roles-" + uuid.NewString()

	reviewer, err := store.Create(ctx, &entity.User{
		Email: uuid.NewString() + "@email.com",
		Name:  "Reviewer " + token,
		Roles: []entity.Role{entity.RoleReviewer},
	})
	require.NoError(t, err)

	editor, err := store.Create(ctx, &entity.User{
		Email: uuid.NewString() + "@email.com",
		Name:  "Editor " + token,
		Roles: []entity.Role{entity.RoleEditor},
	})
	require.NoError(t, err)

	guest, err := store.Create(ctx, &entity.User{
		Email: uuid.NewString() + "@email.com",
		Name:  "Guest " + token,
		Roles: []entity.Role{entity.RoleGuest},
	})
	require.NoError(t, err)

	listIDs := func(roles ...entity.Role) []uuid.UUID {
		t.Helper()
		users, total, err := store.List(ctx, &entity.ListUsersCmd{
			Search: token,
			Roles:  roles,
			Limit:  50,
		})
		require.NoError(t, err)
		require.Equal(t, int64(len(users)), total)
		return lo.Map(users, func(u *entity.User, _ int) uuid.UUID { return u.ID })
	}

	t.Run("single role keeps only that role", func(t *testing.T) {
		t.Parallel()
		ids := listIDs(entity.RoleReviewer)
		require.Contains(t, ids, reviewer.ID)
		require.NotContains(t, ids, editor.ID)
		require.NotContains(t, ids, guest.ID)
	})

	t.Run("multiple roles are OR'd", func(t *testing.T) {
		t.Parallel()
		ids := listIDs(entity.RoleReviewer, entity.RoleEditor)
		require.Contains(t, ids, reviewer.ID)
		require.Contains(t, ids, editor.ID)
		require.NotContains(t, ids, guest.ID)
	})
}

// TestListSearchByMessengerTag verifies the search box also matches messenger
// handles on the ADMIN listing path, which opts in via SearchMessengerTags — every
// helper here sets the flag. The picker side (flag off) is covered by
// TestListSearchMessengerTagsGate.
//
// Handles are stored verbatim ("@ruslan" stays "@ruslan"), so the "@" is
// stripped off the query at read time: an admin who pasted a handle out of a
// complaint must find the same row as one who typed the bare name. The column
// keeps its "@" — the pattern is unanchored, so it matches either way.
//
// Assertions are Contains/NotContains on ids, never on len(users) or total: unlike
// the sibling tests above, the queries here are handles rather than the row token,
// so rows from other runs (make tloc runs -count 2 against a shared DB) are not
// filtered out and any exact count would be flaky.
func TestListSearchByMessengerTag(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := NewStore(db)

	token := "tags-" + uuid.NewString()
	// Handles must not be substrings of their owner's name or email, or a tag-only
	// match would silently pass on the name branch and prove nothing.
	//
	// Mixed case is deliberate: uuid hex alone (0-9a-f) has no uppercase, so a
	// case-folding assertion built on it would hold even with LOWER dropped from
	// the column side.
	tgTag := "tG" + uuid.NewString()[:8]
	slackTag := "sL" + uuid.NewString()[:8]
	nameOnly := "nm" + uuid.NewString()[:8]

	// The users differ by intent, not just by data, so they are spelled out rather
	// than tabulated; create only fills in the parts every row shares.
	create := func(name string, tune func(*entity.User)) *entity.User {
		t.Helper()
		u := &entity.User{
			Email: uuid.NewString() + "@email.com",
			Name:  name,
			Roles: entity.DefaultRoles,
		}
		tune(u)
		created, err := store.Create(ctx, u)
		require.NoError(t, err)
		return created
	}

	// Stored with the leading "@" — the shape the search has to see through.
	tgUser := create("TG "+token, func(u *entity.User) {
		u.TelegramTag = lo.ToPtr("@" + tgTag)
	})

	// Stored without the "@" — the other half of the verbatim contract.
	slackUser := create("SL "+token, func(u *entity.User) {
		u.SlackTag = lo.ToPtr(slackTag)
	})

	// Both handle columns NULL: pins that LIKE on a NULL handle does not match.
	plainUser := create("Plain "+token, func(*entity.User) {})

	// Matches on name only — the control for the name/email branches.
	named := create(nameOnly, func(*entity.User) {})

	search := func(cmd *entity.ListUsersCmd) []uuid.UUID {
		t.Helper()
		cmd.Limit = 200
		users, _, err := store.List(ctx, cmd)
		require.NoError(t, err)
		return lo.Map(users, func(u *entity.User, _ int) uuid.UUID { return u.ID })
	}

	listIDs := func(query string) []uuid.UUID {
		t.Helper()
		return search(&entity.ListUsersCmd{Search: query, SearchMessengerTags: true})
	}

	listActiveIDs := func(query string) []uuid.UUID {
		t.Helper()
		return search(&entity.ListUsersCmd{Search: query, SearchMessengerTags: true, ExcludeBlocked: true})
	}

	t.Run("a pasted handle finds the same row as the bare one", func(t *testing.T) {
		t.Parallel()
		require.Contains(t, listIDs("@"+tgTag), tgUser.ID)
	})

	t.Run("search is case-insensitive", func(t *testing.T) {
		t.Parallel()
		// Both directions: the handle is stored in mixed case, so this exercises
		// the LOWER around the column, not just the one around the query.
		require.Contains(t, listIDs(strings.ToUpper(tgTag)), tgUser.ID)
		require.Contains(t, listIDs(strings.ToLower(tgTag)), tgUser.ID)
	})

	t.Run("a fragment of the handle is enough", func(t *testing.T) {
		t.Parallel()
		// The admin remembers part of a handle, not all of it. This also pins the
		// left "%" of the pattern: an anchored "tag%" would miss a suffix.
		require.Contains(t, listIDs(tgTag[3:]), tgUser.ID)
	})

	t.Run("surrounding blanks are trimmed", func(t *testing.T) {
		t.Parallel()
		require.Contains(t, listIDs("  @"+tgTag+"  "), tgUser.ID)
	})

	t.Run("slack handles match the same way", func(t *testing.T) {
		t.Parallel()
		require.Contains(t, listIDs(slackTag), slackUser.ID)
		require.Contains(t, listIDs("@"+slackTag), slackUser.ID)
	})

	t.Run("a tag-only match is enough", func(t *testing.T) {
		t.Parallel()
		// Neither name nor email contains the handle, so this row can only come
		// from the tag branches — the whole point of the change.
		ids := listIDs(tgTag)
		require.Contains(t, ids, tgUser.ID)
		require.NotContains(t, ids, slackUser.ID)
		require.NotContains(t, ids, plainUser.ID)
	})

	t.Run("a handle match still obeys ExcludeBlocked", func(t *testing.T) {
		t.Parallel()
		// The tag branches must stay AND-ed with the rest of the filter rather
		// than widening past it: an admin listing with active=true must not have
		// a handle match smuggle a blocked user back in.
		blockedTag := "bl" + uuid.NewString()[:8]
		blockedAt := time.Now()
		blocked := create("Blocked "+token, func(u *entity.User) {
			u.TelegramTag = lo.ToPtr("@" + blockedTag)
			u.BlockedAt = &blockedAt
		})

		require.Contains(t, listIDs(blockedTag), blocked.ID)
		require.NotContains(t, listActiveIDs(blockedTag), blocked.ID)
	})

	t.Run("name and email search still work", func(t *testing.T) {
		t.Parallel()
		require.Contains(t, listIDs(nameOnly), named.ID)
		require.Contains(t, listIDs(named.Email), named.ID)
	})

	t.Run("an unrelated query matches nothing of ours", func(t *testing.T) {
		t.Parallel()
		ids := listIDs("zz" + uuid.NewString())
		require.NotContains(t, ids, tgUser.ID)
		require.NotContains(t, ids, slackUser.ID)
		require.NotContains(t, ids, named.ID)
	})

	// A query that trims away to nothing must not widen the result: an empty tag
	// pattern is "%%", which matches every non-null handle and would turn the tag
	// branch always-true, pulling in rows the name/email search alone would never
	// have returned.
	//
	// The invariant is "the tag branches contribute nothing extra", not "nothing is
	// found". A bare "@" matches every row through the email branch and did so
	// before this change, so it proves nothing on its own; the blank queries below
	// match no name and no email at all, which makes any hit attributable to the
	// tag branches alone.
	//
	// The IDs filter is what makes the assertion sound rather than lucky. Without
	// it a dropped guard matches every handle in the shared database — hundreds of
	// rows — and Limit truncates the page before our fixtures appear, so the
	// subtest passes while the invariant is broken. Restricted to our own ids,
	// nothing can crowd them out.
	t.Run("a query that trims to nothing does not widen the result", func(t *testing.T) {
		t.Parallel()
		ours := []uuid.UUID{tgUser.ID, slackUser.ID, plainUser.ID, named.ID}
		for _, blank := range []string{"  ", " @ ", "\t"} {
			ids := search(&entity.ListUsersCmd{Search: blank, SearchMessengerTags: true, IDs: ours})
			require.Empty(t, ids, "search=%q must not reach any handle", blank)
		}
	})

	// LIKE metacharacters come from the search box like any other text, so they
	// have to match themselves. Untreated, "%" alone matches every row — a filter
	// that silently stops filtering. Scoped to our own ids for the same reason as
	// the subtest above: a leaked wildcard would otherwise be masked by Limit.
	t.Run("LIKE metacharacters match literally", func(t *testing.T) {
		t.Parallel()
		ours := []uuid.UUID{tgUser.ID, slackUser.ID, plainUser.ID, named.ID}
		for _, meta := range []string{"%", "_", "%%", "_%"} {
			ids := search(&entity.ListUsersCmd{Search: meta, SearchMessengerTags: true, IDs: ours})
			require.Empty(t, ids, "search=%q must not act as a wildcard", meta)
		}
	})

	t.Run("an underscore matches itself, not any character", func(t *testing.T) {
		t.Parallel()
		// "a_b" and "axb" differ only where the metacharacter would be permissive.
		stem := "us" + uuid.NewString()[:8]
		literal := create("Underscore "+token, func(u *entity.User) {
			u.TelegramTag = lo.ToPtr("@" + stem + "_x")
		})
		other := create("Wildcard "+token, func(u *entity.User) {
			u.TelegramTag = lo.ToPtr("@" + stem + "yx")
		})

		ids := listIDs(stem + "_x")
		require.Contains(t, ids, literal.ID)
		require.NotContains(t, ids, other.ID)
	})
}

// TestListSearchMessengerTagsGate verifies that the telegram/slack tags are
// matched ONLY when the caller opts in via SearchMessengerTags. Without it the
// tag columns must contribute nothing: the picker leaves the flag unset, and its
// response carries no tags, so a match there would answer "whose tag is this"
// for every role from guest up.
//
// The IDs filter scopes every assertion to this test's own fixtures. Without it
// a dropped gate matches tags across the whole shared database and Limit
// truncates the page before our rows appear — the test would pass while the
// invariant is broken.
func TestListSearchMessengerTagsGate(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := NewStore(db)

	// Handles must not be substrings of their owner's name or email, or a hit
	// would be attributable to the name/email branches and prove nothing.
	tgTag := "gt" + uuid.NewString()[:8]
	slackTag := "gs" + uuid.NewString()[:8]

	create := func(name string, tune func(*entity.User)) *entity.User {
		t.Helper()
		u := &entity.User{
			Email: uuid.NewString() + "@email.com",
			Name:  name,
			Roles: entity.DefaultRoles,
		}
		tune(u)
		created, err := store.Create(ctx, u)
		require.NoError(t, err)
		return created
	}

	token := "gate-" + uuid.NewString()
	tgUser := create("TG "+token, func(u *entity.User) {
		u.TelegramTag = lo.ToPtr("@" + tgTag)
	})
	slackUser := create("SL "+token, func(u *entity.User) {
		u.SlackTag = lo.ToPtr(slackTag)
	})

	ours := []uuid.UUID{tgUser.ID, slackUser.ID}

	search := func(query string, handles bool) []uuid.UUID {
		t.Helper()
		users, _, err := store.List(ctx, &entity.ListUsersCmd{
			Search:              query,
			SearchMessengerTags: handles,
			IDs:                 ours,
			Limit:               200,
		})
		require.NoError(t, err)
		return lo.Map(users, func(u *entity.User, _ int) uuid.UUID { return u.ID })
	}

	t.Run("without the flag a handle matches nothing", func(t *testing.T) {
		t.Parallel()
		// Whole, fragment and pasted-with-"@" — the three shapes the admin path
		// deliberately accepts. All three must miss when the flag is off, or the
		// gate only narrows the oracle instead of closing it.
		for _, query := range []string{tgTag, tgTag[3:], "@" + tgTag, "  @" + tgTag + "  "} {
			require.Empty(t, search(query, false),
				"search=%q must not reach a handle without SearchMessengerTags", query)
		}
		require.Empty(t, search(slackTag, false), "slack handle must not match either")
	})

	t.Run("with the flag the same queries match", func(t *testing.T) {
		t.Parallel()
		// The control: proves the queries above are well-formed and that the gate
		// is what suppressed them, not a typo in the fixture.
		require.Contains(t, search(tgTag, true), tgUser.ID)
		require.Contains(t, search("@"+tgTag, true), tgUser.ID)
		require.Contains(t, search(slackTag, true), slackUser.ID)
	})

	t.Run("the flag alone filters nothing", func(t *testing.T) {
		t.Parallel()
		// An empty Search short-circuits before the gate is ever consulted, so
		// turning the flag on must not start filtering by itself.
		users, _, err := store.List(ctx, &entity.ListUsersCmd{
			SearchMessengerTags: true,
			IDs:                 ours,
			Limit:               200,
		})
		require.NoError(t, err)
		require.Len(t, users, len(ours))
	})

	t.Run("the gate leaves name and email search alone", func(t *testing.T) {
		t.Parallel()
		// The gate must narrow the tag branches only. Name/email are the picker's
		// whole reason to exist, so breaking them would be a worse regression
		// than the oracle itself.
		require.Contains(t, search(token, false), tgUser.ID)
		require.Contains(t, search(token, false), slackUser.ID)
		require.Contains(t, search(tgUser.Email, false), tgUser.ID)
	})
}

// TestListIDsFilter verifies the ids[] batch filter restricts the result to the
// requested ids (author-resolution path).
func TestListIDsFilter(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := NewStore(db)
	token := "ids-" + uuid.NewString()

	u1, err := store.Create(ctx, &entity.User{
		Email: uuid.NewString() + "@email.com",
		Name:  "One " + token,
		Roles: entity.DefaultRoles,
	})
	require.NoError(t, err)

	u2, err := store.Create(ctx, &entity.User{
		Email: uuid.NewString() + "@email.com",
		Name:  "Two " + token,
		Roles: entity.DefaultRoles,
	})
	require.NoError(t, err)

	u3, err := store.Create(ctx, &entity.User{
		Email: uuid.NewString() + "@email.com",
		Name:  "Three " + token,
		Roles: entity.DefaultRoles,
	})
	require.NoError(t, err)

	users, total, err := store.List(ctx, &entity.ListUsersCmd{
		IDs:   []uuid.UUID{u1.ID, u3.ID},
		Limit: 50,
	})
	require.NoError(t, err)
	require.Equal(t, int64(len(users)), total)

	ids := lo.Map(users, func(u *entity.User, _ int) uuid.UUID { return u.ID })
	require.Contains(t, ids, u1.ID)
	require.Contains(t, ids, u3.ID)
	require.NotContains(t, ids, u2.ID)
}
