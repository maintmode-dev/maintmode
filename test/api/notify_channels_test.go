//go:build api

package api

import (
	"context"
	"net/http"
	"strconv"
	"testing"

	"github.com/google/uuid"
	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/entity"
	maintmodeclient "github.com/ruko1202/maintmode/internal/pkg/generated/clients/maintmode"
	"github.com/ruko1202/maintmode/internal/utils/xuuid"
)

// Paging defaults the endpoint inherits from xecho.PagingParams. Mirrored here
// so the assertions read as the contract rather than as magic numbers; the
// helper's own tests own the coercion rules themselves.
const (
	defaultChannelsPageSize = 50
	maxChannelsPageSize     = 200
)

// withRawQuery appends a query fragment the typed client cannot express — it
// takes *int for limit, so an unparseable one has to go on the wire by hand.
func withRawQuery(fragment string) maintmodeclient.RequestEditorFn {
	return func(_ context.Context, req *http.Request) error {
		if req.URL.RawQuery == "" {
			req.URL.RawQuery = fragment
			return nil
		}
		req.URL.RawQuery += "&" + fragment
		return nil
	}
}

// TestNotifyChannelsAPI_Create_HappyPath creates a channel as editor and verifies
// it then appears in GET /channels with the assigned UUID id — the DB-backed
// catalog is the cross-pod source of truth the FE picker reads.
func TestNotifyChannelsAPI_Create_HappyPath(t *testing.T) {
	ctx := ctxWithLogger(context.Background(), t)
	apiClient := setupMaintmodeTestClient() // admin (inherits editor)

	transportChannelID := "happy-" + xuuid.NewString()
	// Unique so the listing below can find this channel by name: the catalog is
	// paged now, and a shared-database run holds far more channels than a page.
	channelName := "Happy channel " + xuuid.NewString()
	createResp, err := apiClient.PostApiV1NotificationsChannelsWithResponse(ctx,
		maintmodeclient.PostApiV1NotificationsChannelsJSONRequestBody{
			Transport:          lo.ToPtr(string(entity.NotifyTransportSlack)),
			TransportChannelId: lo.ToPtr(transportChannelID),
			Name:               lo.ToPtr(channelName),
			Description:        lo.ToPtr("created by API test"),
		},
	)
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, createResp.StatusCode(), "unexpected status: %s", createResp.Body)
	require.NotNil(t, createResp.JSON201)

	gotID := lo.FromPtr(createResp.JSON201.Id)
	require.NotEqual(t, uuid.Nil, gotID, "channel id must be a non-nil UUID")
	require.Equal(t, transportChannelID, lo.FromPtr(createResp.JSON201.TransportChannelId))
	require.Nil(t, createResp.JSON201.ArchivedAt, "new channel must be active")

	listResp, err := apiClient.GetApiV1NotificationsChannelsWithResponse(ctx,
		&maintmodeclient.GetApiV1NotificationsChannelsParams{Name: lo.ToPtr(channelName)})
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, listResp.StatusCode())
	require.NotNil(t, listResp.JSON200)

	ids := lo.Map(lo.FromPtr(listResp.JSON200.Channels), func(ch maintmodeclient.ApimodelsChannel, _ int) string {
		return lo.FromPtr(ch.Id).String()
	})
	require.Contains(t, ids, gotID.String(), "created channel must be visible via GET /channels")
}

// TestNotifyChannelsAPI_Create_Duplicate verifies the unique (transport,
// transport_channel_id) constraint surfaces as 409.
func TestNotifyChannelsAPI_Create_Duplicate(t *testing.T) {
	ctx := ctxWithLogger(context.Background(), t)
	apiClient := setupMaintmodeTestClient() // admin

	body := maintmodeclient.PostApiV1NotificationsChannelsJSONRequestBody{
		Transport:          lo.ToPtr(string(entity.NotifyTransportTelegram)),
		TransportChannelId: lo.ToPtr("dup-" + xuuid.NewString()),
		Name:               lo.ToPtr("Dup channel"),
		Description:        lo.ToPtr("created by API test"),
	}

	first, err := apiClient.PostApiV1NotificationsChannelsWithResponse(ctx, body)
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, first.StatusCode(), "unexpected status: %s", first.Body)

	second, err := apiClient.PostApiV1NotificationsChannelsWithResponse(ctx, body)
	require.NoError(t, err)
	require.Equal(t, http.StatusConflict, second.StatusCode(), "duplicate must be 409: %s", second.Body)
}

// TestNotifyChannelsAPI_Create_InvalidTransport verifies transport validation.
func TestNotifyChannelsAPI_Create_InvalidTransport(t *testing.T) {
	ctx := ctxWithLogger(context.Background(), t)
	apiClient := setupMaintmodeTestClient() // admin

	resp, err := apiClient.PostApiV1NotificationsChannelsWithResponse(ctx,
		maintmodeclient.PostApiV1NotificationsChannelsJSONRequestBody{
			Transport:          lo.ToPtr("carrier-pigeon"),
			TransportChannelId: lo.ToPtr("bad-" + xuuid.NewString()),
			Name:               lo.ToPtr("Bad transport"),
			Description:        lo.ToPtr("created by API test"),
		},
	)
	require.NoError(t, err)
	require.Equal(t, http.StatusBadRequest, resp.StatusCode(), "unexpected status: %s", resp.Body)
}

// TestNotifyChannelsAPI_Create_Forbidden verifies catalog writes require at
// least editor: a guest (read-only) is rejected.
func TestNotifyChannelsAPI_Create_Forbidden(t *testing.T) {
	ctx := ctxWithLogger(context.Background(), t)
	apiClient := setupMaintmodeTestClientWithRoles(entity.RoleGuest)

	resp, err := apiClient.PostApiV1NotificationsChannelsWithResponse(ctx,
		maintmodeclient.PostApiV1NotificationsChannelsJSONRequestBody{
			Transport:          lo.ToPtr(string(entity.NotifyTransportSlack)),
			TransportChannelId: lo.ToPtr("forbidden-" + xuuid.NewString()),
			Name:               lo.ToPtr("Forbidden channel"),
			Description:        lo.ToPtr("created by API test"),
		},
	)
	require.NoError(t, err)
	require.Equal(t, http.StatusForbidden, resp.StatusCode(), "guest must be 403: %s", resp.Body)
}

// TestNotifyChannelsAPI_Archive_Unarchive walks the soft-delete lifecycle:
// archive hides the channel from the default listing but keeps it visible with
// include_archived=true; unarchive restores it.
func TestNotifyChannelsAPI_Archive_Unarchive(t *testing.T) {
	ctx := ctxWithLogger(context.Background(), t)
	apiClient := setupMaintmodeTestClient() // admin (inherits editor)

	channelID, channelName := createChannel(ctx, t, apiClient)

	// Archive → hidden from the default list, present with include_archived.
	archiveResp, err := apiClient.PostApiV1NotificationsChannelsIdArchiveWithResponse(ctx, channelID)
	require.NoError(t, err)
	require.Equal(t, http.StatusNoContent, archiveResp.StatusCode(), "unexpected status: %s", archiveResp.Body)

	require.NotContains(t, listChannelIDs(ctx, t, apiClient, channelName, false), channelID.String(),
		"archived channel must be hidden from the default listing")
	require.Contains(t, listChannelIDs(ctx, t, apiClient, channelName, true), channelID.String(),
		"archived channel must appear with include_archived=true")

	// Unarchive → back in the default list.
	unarchiveResp, err := apiClient.PostApiV1NotificationsChannelsIdUnarchiveWithResponse(ctx, channelID)
	require.NoError(t, err)
	require.Equal(t, http.StatusNoContent, unarchiveResp.StatusCode(), "unexpected status: %s", unarchiveResp.Body)

	require.Contains(t, listChannelIDs(ctx, t, apiClient, channelName, false), channelID.String(),
		"unarchived channel must reappear in the default listing")
}

// TestNotifyChannelsAPI_Archive_Idempotent verifies archiving twice and
// archiving an unknown id both succeed (no 404).
func TestNotifyChannelsAPI_Archive_Idempotent(t *testing.T) {
	ctx := ctxWithLogger(context.Background(), t)
	apiClient := setupMaintmodeTestClient()

	channelID, _ := createChannel(ctx, t, apiClient)

	for range 2 {
		resp, err := apiClient.PostApiV1NotificationsChannelsIdArchiveWithResponse(ctx, channelID)
		require.NoError(t, err)
		require.Equal(t, http.StatusNoContent, resp.StatusCode(), "repeated archive must stay 204: %s", resp.Body)
	}

	unknown, err := apiClient.PostApiV1NotificationsChannelsIdArchiveWithResponse(ctx, uuid.New())
	require.NoError(t, err)
	require.Equal(t, http.StatusNoContent, unknown.StatusCode(), "archiving unknown id must be a no-op success: %s", unknown.Body)
}

// TestNotifyChannelsAPI_Archive_Forbidden verifies archive requires editor.
func TestNotifyChannelsAPI_Archive_Forbidden(t *testing.T) {
	ctx := ctxWithLogger(context.Background(), t)

	channelID, _ := createChannel(ctx, t, setupMaintmodeTestClient())

	guest := setupMaintmodeTestClientWithRoles(entity.RoleGuest)
	resp, err := guest.PostApiV1NotificationsChannelsIdArchiveWithResponse(ctx, channelID)
	require.NoError(t, err)
	require.Equal(t, http.StatusForbidden, resp.StatusCode(), "guest must be 403: %s", resp.Body)
}

// TestNotifyChannelsAPI_Create_RecordsAuthor verifies create stamps created_by
// with the authenticated caller's id and leaves updated_by null.
func TestNotifyChannelsAPI_Create_RecordsAuthor(t *testing.T) {
	ctx := ctxWithLogger(context.Background(), t)

	authorID := xuuid.NewString()
	apiClient := setupMaintmodeTestClientWithToken(mustTestAccessTokenForUser(authorID, entity.RoleAdmin))

	resp, err := apiClient.PostApiV1NotificationsChannelsWithResponse(ctx,
		maintmodeclient.PostApiV1NotificationsChannelsJSONRequestBody{
			Transport:          lo.ToPtr(string(entity.NotifyTransportSlack)),
			TransportChannelId: lo.ToPtr("author-" + xuuid.NewString()),
			Name:               lo.ToPtr("Authored channel"),
			Description:        lo.ToPtr("created by API test"),
		},
	)
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode(), "unexpected status: %s", resp.Body)
	require.NotNil(t, resp.JSON201)

	require.NotNil(t, resp.JSON201.CreatedBy, "create response must carry created_by")
	require.Equal(t, authorID, lo.FromPtr(resp.JSON201.CreatedBy.Id).String(), "created_by.id must be the caller")
	require.NotNil(t, resp.JSON201.CreatedAt, "create response must carry created_at")
	require.Nil(t, resp.JSON201.UpdatedBy, "a freshly created channel has no editor")
}

// TestNotifyChannelsAPI_Get_HappyPath verifies single-read returns the channel
// with authorship metadata. Any authenticated role may read.
func TestNotifyChannelsAPI_Get_HappyPath(t *testing.T) {
	ctx := ctxWithLogger(context.Background(), t)
	apiClient := setupMaintmodeTestClient() // admin

	channelID, _ := createChannel(ctx, t, apiClient)

	resp, err := apiClient.GetApiV1NotificationsChannelsIdWithResponse(ctx, channelID)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode(), "unexpected status: %s", resp.Body)
	require.NotNil(t, resp.JSON200)
	require.Equal(t, channelID.String(), lo.FromPtr(resp.JSON200.Id).String())
	require.NotNil(t, resp.JSON200.CreatedAt, "single-read must carry created_at")
	require.NotNil(t, resp.JSON200.CreatedBy, "single-read must carry created_by")
}

// TestNotifyChannelsAPI_Get_Reader verifies a guest (read-only) can read a
// channel by id.
func TestNotifyChannelsAPI_Get_Reader(t *testing.T) {
	ctx := ctxWithLogger(context.Background(), t)

	channelID, _ := createChannel(ctx, t, setupMaintmodeTestClient())

	guest := setupMaintmodeTestClientWithRoles(entity.RoleGuest)
	resp, err := guest.GetApiV1NotificationsChannelsIdWithResponse(ctx, channelID)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode(), "guest must be able to read: %s", resp.Body)
}

// TestNotifyChannelsAPI_Get_NotFound verifies an unknown id is 404.
func TestNotifyChannelsAPI_Get_NotFound(t *testing.T) {
	ctx := ctxWithLogger(context.Background(), t)
	apiClient := setupMaintmodeTestClient()

	resp, err := apiClient.GetApiV1NotificationsChannelsIdWithResponse(ctx, uuid.New())
	require.NoError(t, err)
	require.Equal(t, http.StatusNotFound, resp.StatusCode(), "unknown id must be 404: %s", resp.Body)
}

// TestNotifyChannelsAPI_Update_HappyPath edits name/description and verifies the
// response reflects the change and stamps updated_by with the editor's id.
func TestNotifyChannelsAPI_Update_HappyPath(t *testing.T) {
	ctx := ctxWithLogger(context.Background(), t)

	editorID := xuuid.NewString()
	apiClient := setupMaintmodeTestClientWithToken(mustTestAccessTokenForUser(editorID, entity.RoleAdmin))

	channelID, _ := createChannel(ctx, t, apiClient)

	resp, err := apiClient.PatchApiV1NotificationsChannelsIdWithResponse(ctx, channelID,
		maintmodeclient.PatchApiV1NotificationsChannelsIdJSONRequestBody{
			Name:        lo.ToPtr("Renamed channel"),
			Description: lo.ToPtr("edited by API test"),
		},
	)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode(), "unexpected status: %s", resp.Body)
	require.NotNil(t, resp.JSON200)
	require.Equal(t, "Renamed channel", lo.FromPtr(resp.JSON200.Name))
	require.Equal(t, "edited by API test", lo.FromPtr(resp.JSON200.Description))
	require.NotNil(t, resp.JSON200.UpdatedAt, "update must stamp updated_at")
	require.NotNil(t, resp.JSON200.UpdatedBy, "update must stamp updated_by")
	require.Equal(t, editorID, lo.FromPtr(resp.JSON200.UpdatedBy.Id).String(), "updated_by.id must be the editor")

	// Persisted: a subsequent read reflects the edit.
	got, err := apiClient.GetApiV1NotificationsChannelsIdWithResponse(ctx, channelID)
	require.NoError(t, err)
	require.Equal(t, "Renamed channel", lo.FromPtr(got.JSON200.Name))
}

// TestNotifyChannelsAPI_Update_PartialLeavesOthers verifies an omitted field is
// left unchanged and a transport key in the body is ignored (transport immutable).
func TestNotifyChannelsAPI_Update_PartialLeavesOthers(t *testing.T) {
	ctx := ctxWithLogger(context.Background(), t)
	apiClient := setupMaintmodeTestClient()

	channelID, _ := createChannel(ctx, t, apiClient)

	before, err := apiClient.GetApiV1NotificationsChannelsIdWithResponse(ctx, channelID)
	require.NoError(t, err)
	originalTransport := lo.FromPtr(before.JSON200.Transport)
	originalChannelID := lo.FromPtr(before.JSON200.TransportChannelId)

	// Only description set: name and transport_channel_id must be untouched.
	resp, err := apiClient.PatchApiV1NotificationsChannelsIdWithResponse(ctx, channelID,
		maintmodeclient.PatchApiV1NotificationsChannelsIdJSONRequestBody{
			Description: lo.ToPtr("only-description-changed"),
		},
	)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode(), "unexpected status: %s", resp.Body)
	require.Equal(t, "only-description-changed", lo.FromPtr(resp.JSON200.Description))
	require.Equal(t, originalChannelID, lo.FromPtr(resp.JSON200.TransportChannelId), "transport_channel_id must be unchanged")
	require.Equal(t, originalTransport, lo.FromPtr(resp.JSON200.Transport), "transport must be immutable")
}

// TestNotifyChannelsAPI_Update_Forbidden verifies edit requires editor: a guest
// is rejected.
func TestNotifyChannelsAPI_Update_Forbidden(t *testing.T) {
	ctx := ctxWithLogger(context.Background(), t)

	channelID, _ := createChannel(ctx, t, setupMaintmodeTestClient())

	guest := setupMaintmodeTestClientWithRoles(entity.RoleGuest)
	resp, err := guest.PatchApiV1NotificationsChannelsIdWithResponse(ctx, channelID,
		maintmodeclient.PatchApiV1NotificationsChannelsIdJSONRequestBody{
			Name: lo.ToPtr("nope"),
		},
	)
	require.NoError(t, err)
	require.Equal(t, http.StatusForbidden, resp.StatusCode(), "guest must be 403: %s", resp.Body)
}

// TestNotifyChannelsAPI_Update_NotFound verifies editing an unknown id is 404.
func TestNotifyChannelsAPI_Update_NotFound(t *testing.T) {
	ctx := ctxWithLogger(context.Background(), t)
	apiClient := setupMaintmodeTestClient()

	resp, err := apiClient.PatchApiV1NotificationsChannelsIdWithResponse(ctx, uuid.New(),
		maintmodeclient.PatchApiV1NotificationsChannelsIdJSONRequestBody{
			Name: lo.ToPtr("ghost"),
		},
	)
	require.NoError(t, err)
	require.Equal(t, http.StatusNotFound, resp.StatusCode(), "unknown id must be 404: %s", resp.Body)
}

// TestNotifyChannelsAPI_Update_DuplicateIsConflict verifies editing a channel's
// transport_channel_id to collide with another channel of the same transport
// surfaces 409.
func TestNotifyChannelsAPI_Update_DuplicateIsConflict(t *testing.T) {
	ctx := ctxWithLogger(context.Background(), t)
	apiClient := setupMaintmodeTestClient()

	existingChannelID := "dup-target-" + xuuid.NewString()
	first, err := apiClient.PostApiV1NotificationsChannelsWithResponse(ctx,
		maintmodeclient.PostApiV1NotificationsChannelsJSONRequestBody{
			Transport:          lo.ToPtr(string(entity.NotifyTransportSlack)),
			TransportChannelId: lo.ToPtr(existingChannelID),
			Name:               lo.ToPtr("Existing"),
			Description:        lo.ToPtr("created by API test"),
		},
	)
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, first.StatusCode(), "unexpected status: %s", first.Body)

	victimID, _ := createChannel(ctx, t, apiClient) // slack transport

	resp, err := apiClient.PatchApiV1NotificationsChannelsIdWithResponse(ctx, victimID,
		maintmodeclient.PatchApiV1NotificationsChannelsIdJSONRequestBody{
			TransportChannelId: lo.ToPtr(existingChannelID),
		},
	)
	require.NoError(t, err)
	require.Equal(t, http.StatusConflict, resp.StatusCode(), "colliding transport_channel_id must be 409: %s", resp.Body)
}

// createChannel creates a channel as the given (admin) client and returns its
// UUID id.
// createChannel returns the new channel's id and its name. The name is unique
// per call so a caller can filter the paged listing down to exactly this
// channel — without that, an assertion about one channel's presence in a
// 3000-row catalog is really an assertion about sort position.
func createChannel(
	ctx context.Context,
	t *testing.T,
	apiClient *maintmodeclient.ClientWithResponses,
) (id uuid.UUID, name string) {
	t.Helper()

	name = "Lifecycle channel " + xuuid.NewString()
	resp, err := apiClient.PostApiV1NotificationsChannelsWithResponse(ctx,
		maintmodeclient.PostApiV1NotificationsChannelsJSONRequestBody{
			Transport:          lo.ToPtr(string(entity.NotifyTransportSlack)),
			TransportChannelId: lo.ToPtr("lifecycle-" + xuuid.NewString()),
			Name:               lo.ToPtr(name),
			Description:        lo.ToPtr("created by API test"),
		},
	)
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode(), "unexpected status: %s", resp.Body)
	require.NotNil(t, resp.JSON201)

	return lo.FromPtr(resp.JSON201.Id), name
}

// listChannelIDs returns the ids of the channels matching name, optionally
// including archived ones.
//
// The name filter is what makes the archived-channel assertions mean anything.
// Listing the whole catalog and checking that a channel is absent would pass
// whether it was archived or merely paged past — a green test asserting nothing.
func listChannelIDs(
	ctx context.Context,
	t *testing.T,
	apiClient *maintmodeclient.ClientWithResponses,
	name string,
	includeArchived bool,
) []string {
	t.Helper()

	params := &maintmodeclient.GetApiV1NotificationsChannelsParams{Name: lo.ToPtr(name)}
	if includeArchived {
		params.IncludeArchived = lo.ToPtr(true)
	}

	resp, err := apiClient.GetApiV1NotificationsChannelsWithResponse(ctx, params)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode(), "unexpected status: %s", resp.Body)
	require.NotNil(t, resp.JSON200)

	return channelIDsOf(resp.JSON200)
}

// TestNotifyChannelsAPI_List_DefaultPage covers the envelope an unparameterised
// read returns: one page plus the metadata the UI needs to know it is a page.
func TestNotifyChannelsAPI_List_DefaultPage(t *testing.T) {
	ctx := ctxWithLogger(context.Background(), t)
	apiClient := setupMaintmodeTestClient()

	resp, err := apiClient.GetApiV1NotificationsChannelsWithResponse(ctx, nil)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode(), "unexpected status: %s", resp.Body)
	require.NotNil(t, resp.JSON200)

	total := lo.FromPtr(resp.JSON200.Total)
	require.EqualValues(t, defaultChannelsPageSize, lo.FromPtr(resp.JSON200.Limit))
	require.EqualValues(t, 0, lo.FromPtr(resp.JSON200.Offset))

	// Asserted within the one response: the shared catalog changes under other
	// tests, so any claim about an absolute count would be a race.
	require.Len(t, lo.FromPtr(resp.JSON200.Channels), min(total, defaultChannelsPageSize))
}

// TestNotifyChannelsAPI_List_OffsetReachesSecondPage is the criterion the whole
// ticket rests on: a channel past the first page must still be reachable.
func TestNotifyChannelsAPI_List_OffsetReachesSecondPage(t *testing.T) {
	ctx := ctxWithLogger(context.Background(), t)
	apiClient := setupMaintmodeTestClient()

	const (
		seeded   = 5
		pageSize = 3
	)

	prefix := seedChannels(ctx, t, apiClient, seeded)

	first := listChannelPage(ctx, t, apiClient, prefix, pageSize, 0)
	require.EqualValues(t, seeded, lo.FromPtr(first.Total))
	require.Len(t, lo.FromPtr(first.Channels), pageSize)

	second := listChannelPage(ctx, t, apiClient, prefix, pageSize, pageSize)
	require.EqualValues(t, seeded, lo.FromPtr(second.Total), "total describes the filter, not the page")
	require.Len(t, lo.FromPtr(second.Channels), seeded-pageSize)

	ids := append(channelIDsOf(first), channelIDsOf(second)...)
	require.Len(t, lo.Uniq(ids), seeded, "the two pages must not overlap and must cover the seeded set")
}

// TestNotifyChannelsAPI_List_CoercesBadPaging pins the two behaviors this
// handler owns rather than inherits. The rest of the coercion matrix is already
// table-tested in internal/utils/xecho.
func TestNotifyChannelsAPI_List_CoercesBadPaging(t *testing.T) {
	ctx := ctxWithLogger(context.Background(), t)
	apiClient := setupMaintmodeTestClient()

	// Unparseable: a read-only reference list serves a best-effort page. The
	// shared paging helper reports this as an error and audit answers 400 with
	// it; this endpoint must drop it.
	unparseable, err := apiClient.GetApiV1NotificationsChannelsWithResponse(ctx, nil,
		withRawQuery("limit=abc"))
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, unparseable.StatusCode(),
		"an unparseable limit must not become a 400: %s", unparseable.Body)
	require.EqualValues(t, defaultChannelsPageSize, lo.FromPtr(unparseable.JSON200.Limit))

	// Over the maximum: reset to the default, not clamped to the maximum.
	overMax, err := apiClient.GetApiV1NotificationsChannelsWithResponse(ctx,
		&maintmodeclient.GetApiV1NotificationsChannelsParams{Limit: lo.ToPtr(maxChannelsPageSize + 1)})
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, overMax.StatusCode(), "unexpected status: %s", overMax.Body)
	require.EqualValues(t, defaultChannelsPageSize, lo.FromPtr(overMax.JSON200.Limit),
		"an out-of-range limit resets to the default rather than clamping to the maximum")
}

// TestNotifyChannelsAPI_List_ArchivedComposesWithSearch verifies the archived
// scope still works once a name filter and paging are in play.
func TestNotifyChannelsAPI_List_ArchivedComposesWithSearch(t *testing.T) {
	ctx := ctxWithLogger(context.Background(), t)
	apiClient := setupMaintmodeTestClient()

	// Kept under one page so presence in the response is not a statement about
	// sort position.
	const seeded = 3

	prefix := seedChannels(ctx, t, apiClient, seeded)

	active := listChannelPage(ctx, t, apiClient, prefix, defaultChannelsPageSize, 0)
	require.EqualValues(t, seeded, lo.FromPtr(active.Total))

	archivedID := uuid.MustParse(channelIDsOf(active)[0])
	archiveResp, err := apiClient.PostApiV1NotificationsChannelsIdArchiveWithResponse(ctx, archivedID)
	require.NoError(t, err)
	require.Equal(t, http.StatusNoContent, archiveResp.StatusCode(), "unexpected status: %s", archiveResp.Body)

	afterArchive := listChannelPage(ctx, t, apiClient, prefix, defaultChannelsPageSize, 0)
	require.EqualValues(t, seeded-1, lo.FromPtr(afterArchive.Total),
		"archiving must drop the channel from the filtered total")
	require.NotContains(t, channelIDsOf(afterArchive), archivedID.String())

	withArchived, err := apiClient.GetApiV1NotificationsChannelsWithResponse(ctx,
		&maintmodeclient.GetApiV1NotificationsChannelsParams{
			Name:            lo.ToPtr(prefix),
			Limit:           lo.ToPtr(defaultChannelsPageSize),
			IncludeArchived: lo.ToPtr(true),
		})
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, withArchived.StatusCode(), "unexpected status: %s", withArchived.Body)
	require.EqualValues(t, seeded, lo.FromPtr(withArchived.JSON200.Total),
		"include_archived must restore exactly the archived channel")
	require.Contains(t, channelIDsOf(withArchived.JSON200), archivedID.String())
}

// TestNotifyChannelsAPI_List_PageIsBoundedByLimit is the payload fix itself: a
// matching set larger than a page must still come back one page at a time.
// Bounding rows is what bounds the ~1.44 MB this ticket was filed about.
func TestNotifyChannelsAPI_List_PageIsBoundedByLimit(t *testing.T) {
	ctx := ctxWithLogger(context.Background(), t)
	apiClient := setupMaintmodeTestClient()

	// More than one default page, seeded under a prefix so the seeded rows are
	// what fills the page. Querying unfiltered would let any pre-existing rows
	// fill it and the assertion would hold without this test's own data.
	const seeded = defaultChannelsPageSize + 10

	prefix := seedChannels(ctx, t, apiClient, seeded)

	resp, err := apiClient.GetApiV1NotificationsChannelsWithResponse(ctx,
		&maintmodeclient.GetApiV1NotificationsChannelsParams{Name: lo.ToPtr(prefix)})
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode(), "unexpected status: %s", resp.Body)

	require.EqualValues(t, seeded, lo.FromPtr(resp.JSON200.Total))
	require.Len(t, lo.FromPtr(resp.JSON200.Channels), defaultChannelsPageSize,
		"the response must carry one page, however many channels match")
}

// seedChannels creates count channels sharing a prefix unique to this call, and
// returns the prefix. Filtering by it isolates them from every other row in the
// shared catalog.
func seedChannels(ctx context.Context, t *testing.T, apiClient *maintmodeclient.ClientWithResponses, count int) string {
	t.Helper()

	prefix := "paging-" + xuuid.NewString()
	for i := range count {
		resp, err := apiClient.PostApiV1NotificationsChannelsWithResponse(ctx,
			maintmodeclient.PostApiV1NotificationsChannelsJSONRequestBody{
				Transport:          lo.ToPtr(string(entity.NotifyTransportSlack)),
				TransportChannelId: lo.ToPtr(prefix + "-" + strconv.Itoa(i)),
				Name:               lo.ToPtr(prefix + "-" + strconv.Itoa(i)),
				Description:        lo.ToPtr("created by API test"),
			},
		)
		require.NoError(t, err)
		require.Equal(t, http.StatusCreated, resp.StatusCode(), "unexpected status: %s", resp.Body)
	}

	return prefix
}

// listChannelPage reads one page of the channels matching name.
func listChannelPage(
	ctx context.Context,
	t *testing.T,
	apiClient *maintmodeclient.ClientWithResponses,
	name string,
	limit, offset int,
) *maintmodeclient.ApimodelsChannelsResponse {
	t.Helper()

	resp, err := apiClient.GetApiV1NotificationsChannelsWithResponse(ctx,
		&maintmodeclient.GetApiV1NotificationsChannelsParams{
			Name:   lo.ToPtr(name),
			Limit:  lo.ToPtr(limit),
			Offset: lo.ToPtr(offset),
		})
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode(), "unexpected status: %s", resp.Body)
	require.NotNil(t, resp.JSON200)

	return resp.JSON200
}

func channelIDsOf(page *maintmodeclient.ApimodelsChannelsResponse) []string {
	return lo.Map(lo.FromPtr(page.Channels), func(ch maintmodeclient.ApimodelsChannel, _ int) string {
		return lo.FromPtr(ch.Id).String()
	})
}
