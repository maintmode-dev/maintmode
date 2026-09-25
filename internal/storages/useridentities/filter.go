package useridentities

import (
	"github.com/go-jet/jet/v2/postgres"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/pkg/generated/maintmode/public/table"
)

// methodFilter is the WHERE fragment selecting one sign-in method's rows.
//
// The unset branch is compared with IS NULL rather than left out. Omitting it
// would let a lookup for a registry provider also match a built-in row that
// happens to carry the same subject -- the cross-branch collision the partial
// unique indexes exist to prevent, reintroduced in the query instead.
//
// A reference naming nothing matches nothing. Matching everything is the other
// available reading and the wrong one: this fragment also reaches reads whose
// result decides which user is signed in.
func methodFilter(ref entity.SignInMethodRef) postgres.BoolExpression {
	switch {
	case ref.IntegrationID != nil:
		return table.UserIdentities.IntegrationID.EQ(postgres.UUID(*ref.IntegrationID)).
			AND(table.UserIdentities.BuiltinMethod.IS_NULL())
	case ref.BuiltinMethod != nil:
		return table.UserIdentities.BuiltinMethod.EQ(postgres.String(string(*ref.BuiltinMethod))).
			AND(table.UserIdentities.IntegrationID.IS_NULL())
	default:
		return postgres.Bool(false)
	}
}
