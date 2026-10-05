package useridentities

import (
	"github.com/go-jet/jet/v2/postgres"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/pkg/generated/maintmode/public/table"
)

// methodFilter is the WHERE fragment selecting one sign-in method's rows.
//
// A reference naming nothing matches nothing. Matching everything is the other
// available reading and the wrong one: this fragment also reaches reads whose
// result decides which user is signed in.
func methodFilter(ref entity.SignInMethodRef) postgres.BoolExpression {
	if ref.IntegrationID == nil {
		return postgres.Bool(false)
	}

	return table.UserIdentities.IntegrationID.EQ(postgres.UUID(*ref.IntegrationID))
}
