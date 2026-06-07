//go:build api

package api

import (
	"context"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/entity"
	maintmodeclient "github.com/ruko1202/maintmode/internal/pkg/generated/clients/maintmode"
	"github.com/ruko1202/maintmode/internal/utils/xuuid"
)

// TestNotifyChannelsAPI_Create_HappyPath creates a channel as editor and verifies
// it then appears in GET /channels with the assigned UUID id — the DB-backed
// catalog is the cross-pod source of truth the FE picker reads.
func TestNotifyChannelsAPI_Create_HappyPath(t *testing.T) {
	ctx := ctxWithLogger(context.Background(), t)
	apiClient := setupMaintmodeTestClient() // admin (inherits editor)

	transportChannelID := "happy-" + xuuid.NewString()
	createResp, err := apiClient.PostApiV1NotificationsChannelsWithResponse(ctx,
		maintmodeclient.PostApiV1NotificationsChannelsJSONRequestBody{
			Transport:          lo.ToPtr(string(entity.NotifyTransportSlack)),
			TransportChannelId: lo.ToPtr(transportChannelID),
			Name:               lo.ToPtr("Happy channel"),
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

	listResp, err := apiClient.GetApiV1NotificationsChannelsWithResponse(ctx, nil)
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

	channelID := createChannel(ctx, t, apiClient)

	// Archive → hidden from the default list, present with include_archived.
	archiveResp, err := apiClient.PostApiV1NotificationsChannelsIdArchiveWithResponse(ctx, channelID)
	require.NoError(t, err)
	require.Equal(t, http.StatusNoContent, archiveResp.StatusCode(), "unexpected status: %s", archiveResp.Body)

	require.NotContains(t, listChannelIDs(ctx, t, apiClient, false), channelID.String(),
		"archived channel must be hidden from the default listing")
	require.Contains(t, listChannelIDs(ctx, t, apiClient, true), channelID.String(),
		"archived channel must appear with include_archived=true")

	// Unarchive → back in the default list.
	unarchiveResp, err := apiClient.PostApiV1NotificationsChannelsIdUnarchiveWithResponse(ctx, channelID)
	require.NoError(t, err)
	require.Equal(t, http.StatusNoContent, unarchiveResp.StatusCode(), "unexpected status: %s", unarchiveResp.Body)

	require.Contains(t, listChannelIDs(ctx, t, apiClient, false), channelID.String(),
		"unarchived channel must reappear in the default listing")
}

// TestNotifyChannelsAPI_Archive_Idempotent verifies archiving twice and
// archiving an unknown id both succeed (no 404).
func TestNotifyChannelsAPI_Archive_Idempotent(t *testing.T) {
	ctx := ctxWithLogger(context.Background(), t)
	apiClient := setupMaintmodeTestClient()

	channelID := createChannel(ctx, t, apiClient)

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

	channelID := createChannel(ctx, t, setupMaintmodeTestClient())

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

	channelID := createChannel(ctx, t, apiClient)

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

	channelID := createChannel(ctx, t, setupMaintmodeTestClient())

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

	channelID := createChannel(ctx, t, apiClient)

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

	channelID := createChannel(ctx, t, apiClient)

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

	channelID := createChannel(ctx, t, setupMaintmodeTestClient())

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

	victimID := createChannel(ctx, t, apiClient) // slack transport

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
func createChannel(ctx context.Context, t *testing.T, apiClient *maintmodeclient.ClientWithResponses) uuid.UUID {
	t.Helper()

	resp, err := apiClient.PostApiV1NotificationsChannelsWithResponse(ctx,
		maintmodeclient.PostApiV1NotificationsChannelsJSONRequestBody{
			Transport:          lo.ToPtr(string(entity.NotifyTransportSlack)),
			TransportChannelId: lo.ToPtr("lifecycle-" + xuuid.NewString()),
			Name:               lo.ToPtr("Lifecycle channel"),
			Description:        lo.ToPtr("created by API test"),
		},
	)
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode(), "unexpected status: %s", resp.Body)
	require.NotNil(t, resp.JSON201)

	return lo.FromPtr(resp.JSON201.Id)
}

// listChannelIDs returns the catalog channel ids, optionally including archived.
func listChannelIDs(ctx context.Context, t *testing.T, apiClient *maintmodeclient.ClientWithResponses, includeArchived bool) []string {
	t.Helper()

	var params *maintmodeclient.GetApiV1NotificationsChannelsParams
	if includeArchived {
		params = &maintmodeclient.GetApiV1NotificationsChannelsParams{IncludeArchived: lo.ToPtr(true)}
	}

	resp, err := apiClient.GetApiV1NotificationsChannelsWithResponse(ctx, params)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode(), "unexpected status: %s", resp.Body)
	require.NotNil(t, resp.JSON200)

	return lo.Map(lo.FromPtr(resp.JSON200.Channels), func(ch maintmodeclient.ApimodelsChannel, _ int) string {
		return lo.FromPtr(ch.Id).String()
	})
}
