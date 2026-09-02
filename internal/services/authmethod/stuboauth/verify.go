package stuboauth

import (
	"context"
	"fmt"
	"net/mail"
	"strings"

	"github.com/ruko1202/xlog"

	"github.com/ruko1202/maintmode/internal/apperr"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/utils/xhash"
	"github.com/ruko1202/maintmode/internal/utils/xuuid"
)

func (p *Service) Authenticate(ctx context.Context, token string) (*entity.OAuthIDTokenClaims, error) {
	// The span name deliberately still says VerifyToken after the method was
	// renamed to Authenticate: it is a telemetry identifier that existing
	// dashboards and saved queries match on, not a code identifier.
	_, span := xlog.WithOperationSpan(ctx, "service.OAuth.Stub.VerifyToken")
	defer span.End()

	if token == "this-is-not-a-valid-jwt" {
		return nil, apperr.ErrInvalidAccessToken
	}

	// A token that parses as an email address is taken as the caller stating who
	// they are. Invitation accept compares the verified email against the invited
	// one in every environment -- there is no dev bypass -- so without this
	// the random identity below could never match an invitation and the flow
	// would be untestable on a dev stand. Subject is derived from the address so
	// a repeat login resolves to the same user instead of creating a new one.
	//
	// All three claims come from addr.Address, never from the raw token:
	// ParseAddress also accepts display-name forms ("Name <a@b.com>") and pads,
	// and Email is what reaches users.email and the invitation comparison. Using
	// the token verbatim would let Subject and Email describe different people.
	if addr, err := mail.ParseAddress(token); err == nil {
		// LastIndex, not Cut: a quoted local part may itself contain '@'.
		local := addr.Address[:strings.LastIndex(addr.Address, "@")]
		return &entity.OAuthIDTokenClaims{
			Subject: xhash.HashSha256([]byte(strings.ToLower(addr.Address))),
			Email:   addr.Address,
			Name:    fmt.Sprintf("User Name[%s]", local),
		}, nil
	}

	id := xuuid.NewString()
	return &entity.OAuthIDTokenClaims{
		Subject: xuuid.NewString(),
		Email:   fmt.Sprintf("%s@mail.com", id),
		Name:    fmt.Sprintf("User Name[%s]", id),
	}, nil
}
