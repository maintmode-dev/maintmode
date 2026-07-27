package apimodels_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	apimodels "github.com/ruko1202/maintmode/internal/app/api/public/users/models"
)

// TestUpdateUserTagsRequestWireNamesMatchConstants pins the one coupling the
// presence set depends on and nothing else enforces: the Field* constants are
// looked up against keys parsed straight out of the request body, so they have
// to be the JSON tags verbatim.
//
// Getting this wrong fails silently in the worst way. Rename a json tag, leave
// the constant, and Has() answers false forever: the endpoint keeps returning
// 200 while quietly ignoring that field. No handler test catches it either,
// because those build their expectations from the same constants — they would
// agree with the bug. Only a check that reads the struct tags themselves is
// independent enough to notice.
func TestUpdateUserTagsRequestWireNamesMatchConstants(t *testing.T) {
	t.Parallel()

	wireNames := jsonTagNames(t, apimodels.UpdateUserTagsRequest{})

	require.Contains(t, wireNames, apimodels.FieldTelegramTag)
	require.Contains(t, wireNames, apimodels.FieldSlackTag)

	// The reverse direction matters just as much: a field added to the struct
	// without a matching constant is a field the patch can never mark present,
	// so it would be accepted by the binder and then dropped.
	require.ElementsMatch(t,
		[]string{apimodels.FieldTelegramTag, apimodels.FieldSlackTag},
		wireNames,
		"every exported field needs a Field* constant, and vice versa")
}

// TestUpdateUserTagsRequestHasTracksEveryField exercises the same coupling from
// the outside — decoding a body that names each field and asserting Has() sees
// it. The reflective test above catches a renamed tag; this one catches an
// UnmarshalJSON that stops populating the set.
func TestUpdateUserTagsRequestHasTracksEveryField(t *testing.T) {
	t.Parallel()

	for _, field := range []string{apimodels.FieldTelegramTag, apimodels.FieldSlackTag} {
		var req apimodels.UpdateUserTagsRequest
		require.NoError(t, req.UnmarshalJSON([]byte(`{"`+field+`":"x"}`)))

		require.True(t, req.Has(field), "%q was in the body but Has() denies it", field)
	}
}

// jsonTagNames returns the wire name of every exported field of v, skipping
// fields excluded from serialization.
func jsonTagNames(t *testing.T, v any) []string {
	t.Helper()

	typ := reflect.TypeOf(v)
	require.Equal(t, reflect.Struct, typ.Kind())

	names := make([]string, 0, typ.NumField())
	for i := range typ.NumField() {
		field := typ.Field(i)
		if !field.IsExported() {
			continue
		}

		tag, ok := field.Tag.Lookup("json")
		require.Truef(t, ok, "exported field %s has no json tag", field.Name)

		name, _, _ := strings.Cut(tag, ",")
		if name == "-" {
			continue
		}

		names = append(names, name)
	}

	return names
}
