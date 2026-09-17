// Package github resolves the identity behind a GitHub access token.
//
// It is one vendor of the generic OAuth 2.0 gateway, and it owns EVERYTHING
// about asking GitHub who someone is: which endpoints answer, what they return,
// which address may be trusted, and the requests themselves. The
// authorization-code grant that obtained the token is the protocol package's,
// and is identical at every vendor.
//
// The requests are written here rather than through a shared helper, which is
// how gateways/license and gateways/oidcdiscovery write theirs. Each one owns
// its own client, and the redaction policy travels with the construction --
// see the package comment on utils/xsanitize, which is explicit that the safe
// default lives at the call site.
//
// The API base is configuration rather than a constant: GitHub publishes no
// discovery document, so the endpoints are supplied by the deployment catalog
// and stored on the row, which lets a stand point at a fake GitHub without a
// rebuild.
package github

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/ruko1202/xhttp/client"
	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/utils/xsanitize"
)

// emailsPerPage is how many addresses are requested, and the whole list is taken
// to be what comes back.
//
// Pagination is deliberately NOT followed. GitHub returns at most one primary
// address by construction, so a primary record beyond position 100 would need an
// account with more than a hundred addresses AND an ordering that buries the
// primary one. Walking pages would multiply the request budget of every sign-in
// for a case that does not occur. A stated bound, not an oversight.
const emailsPerPage = 100

// apiTimeout bounds ONE read of api.github.com.
//
// The discovery resolver's number, for its situation: a plain read from a
// CDN-fronted API. It bounds a single hung call; the caller's context bounds
// the pair of them together, and WithTimeout only ever tightens a deadline, so
// whichever is nearer wins.
const apiTimeout = 5 * time.Second

// maxResponseBytes bounds every response body.
//
// The license gateway's value. /user is a few hundred bytes and a hundred
// addresses are a few kilobytes, so this is orders of magnitude of headroom --
// it exists to stop an unbounded decode of a broken or hostile upstream, not to
// police a realistic payload.
const maxResponseBytes = 1 << 20

// acceptHeader is GitHub's API VERSIONING mechanism, not a content negotiation
// nicety.
//
// Without it GitHub serves a legacy representation, so dropping it changes the
// payloads this vendor decodes without changing any status code -- a silent
// break rather than a failure. It is the vendor's to send for exactly that
// reason: no other vendor's API is versioned this way.
const acceptHeader = "application/vnd.github+json"

// Vendor resolves GitHub identities.
//
// It holds its own HTTP client rather than borrowing one, which is how every
// other gateway in this repository is built: five of them construct a client at
// their own call site, and none is passed between packages. The construction is
// where the redaction policy attaches, so owning the client and owning the
// policy are the same act.
type Vendor struct {
	// apiBase is the API ROOT without a trailing slash. Trimmed once here
	// rather than at each call, because url.JoinPath on a base ending in "/" is
	// one of the two ways to get a doubled separator.
	apiBase string
	httpc   *http.Client
}

// New builds the GitHub vendor for one configured instance.
//
// apiBase comes from the stored row, so a stand can point at a fake GitHub
// without a rebuild; production rows carry api.github.com, fixed by the preset
// and refused as operator input.
func New(apiBase string) Vendor {
	return Vendor{
		apiBase: strings.TrimRight(apiBase, "/"),
		httpc: client.NewClient(
			client.WithTimeout(apiTimeout),
			client.WithSanitizer(xsanitize.New()),
		),
	}
}

// DefaultScopes is what a GitHub sign-in needs, and it is ONE scope.
//
// user:email is what makes /user/emails readable. read:user is deliberately
// absent: GET /user needs no scope at all on an OAuth App token, so requesting
// it would widen the consent screen the user reads while granting access to
// nothing this vendor touches. No org or team scopes either -- nothing here
// reads org membership.
func (Vendor) DefaultScopes() []string { return []string{"user:email"} }

// userResponse is the subset of GET /user this vendor reads.
//
// The email field is absent ON PURPOSE. /user does carry one, and it is the
// obvious thing to use, but it reports no verification status -- so trusting it
// would skip the verified check for every account whose address is public, which
// is most of them. Not decoding it at all is what makes that impossible to do by
// accident later.
type userResponse struct {
	ID    int64  `json:"id"`
	Login string `json:"login"`
	Name  string `json:"name"`
}

// emailRecord is one row of GET /user/emails.
type emailRecord struct {
	Email    string `json:"email"`
	Primary  bool   `json:"primary"`
	Verified bool   `json:"verified"`
}

// ResolveIdentity resolves the account behind an access token.
//
// Two calls, in order, because the second depends on nothing from the first but
// the failure of the first is cheaper to report: /user for the identity key and
// the display name, /user/emails for the address.
//
// The caller owns the TOTAL budget: both reads run under ctx, which the client
// already bounded, so three sequential calls cannot add up to a 20s callback.
func (v Vendor) ResolveIdentity(
	ctx context.Context, accessToken string,
) (*entity.OAuth2Identity, error) {
	var user userResponse
	if err := v.get(ctx, accessToken, "/user", nil, &user); err != nil {
		return nil, fmt.Errorf("fetch github user: %w", err)
	}

	// Refused before the second call: an identity with no subject cannot be
	// stored whatever its email turns out to be, and spending an outbound
	// request to learn that would be spending it to reach the same refusal.
	if user.ID == 0 {
		xlog.Warn(ctx, "github /user returned no usable numeric id",
			xfield.String("login", user.Login))

		return nil, apperr.ErrGithubIdentityUnusable
	}

	email, err := v.resolveEmail(ctx, accessToken)
	if err != nil {
		return nil, err
	}

	return &entity.OAuth2Identity{
		// The numeric user id, stringified. Never the login: logins are
		// renameable and, once renamed, claimable by someone else, so an account
		// keyed on one could be inherited.
		Subject: strconv.FormatInt(user.ID, 10),
		Email:   email,
		Name:    user.Name,
	}, nil
}

// resolveEmail reads GET /user/emails and picks the one address this backend may
// trust.
//
// Both halves of the rule matter and neither is negotiable. VERIFIED, because
// email is the key that matches an invitation to a person, and an unvouched
// address would let whoever holds it reach someone else's invitation. PRIMARY,
// because a person controls several verified addresses and only one of them is
// the account's identity -- picking any verified one would resolve the same
// GitHub account to different users depending on list order.
//
// It stops at the first match. Grafana's equivalent has no break and keeps the
// LAST primary record; with GitHub's API that is unobservable today, but "last
// of an unordered list" is not a rule anyone chose. Their loop also parses
// `verified` and never checks it, which is the gap this function exists to not
// have -- their own Google connector enforces the equivalent.
//
// There is no fallback, by design: not the public /user email, not the first
// merely verified address, not <login>@users.noreply.github.com.
func (v Vendor) resolveEmail(ctx context.Context, accessToken string) (string, error) {
	query := url.Values{"per_page": {strconv.Itoa(emailsPerPage)}}

	var records []emailRecord
	if err := v.get(ctx, accessToken, "/user/emails", query, &records); err != nil {
		return "", fmt.Errorf("fetch github emails: %w", err)
	}

	for _, record := range records {
		if record.Primary && record.Verified && record.Email != "" {
			return record.Email, nil
		}
	}

	// The person fixes this on GitHub by verifying their primary address, so it
	// must not reach them as a provider outage. The count is logged and the
	// addresses are not: which addresses an account holds is exactly the thing
	// not to write into a log line.
	xlog.Warn(ctx, "github account has no verified primary email",
		xfield.Int("addresses", len(records)))

	return "", apperr.ErrGithubEmailUnusable
}

// get performs one authenticated read against the API base and decodes the body.
//
// One helper for two calls rather than two near-identical bodies, because every
// property here has to hold for both: the bearer header (which the sanitizer
// redacts, and which is why the token is never a query parameter), the join
// against the API ROOT, the status check, and the BOUNDED decode.
//
// It is this package's, not the protocol layer's. What a vendor asks for and
// how it asks are the same decision -- GitHub versions its API through Accept,
// pages through per_page, and answers 404 where another vendor answers 200 with
// an empty list -- so a shared helper would have to grow a parameter for each
// of those anyway.
func (v Vendor) get(
	ctx context.Context,
	accessToken, path string,
	query url.Values,
	out any,
) error {
	endpoint, err := url.Parse(v.apiBase)
	if err != nil {
		return fmt.Errorf("parse github api base: %w", err)
	}

	// JoinPath escapes each segment; RawQuery is set on the parsed URL rather
	// than concatenated onto its string form, so a value carrying "&" or "#"
	// cannot break out of its parameter.
	endpoint = endpoint.JoinPath(path)
	endpoint.RawQuery = query.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), http.NoBody)
	if err != nil {
		return fmt.Errorf("build github request: %w", err)
	}

	// The token travels here and only here. In a query string it would be
	// readable in every access log and Referer; in this header xsanitize redacts
	// it.
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", acceptHeader)

	resp, err := v.httpc.Do(req)
	if err != nil {
		return fmt.Errorf("%w: %w", apperr.ErrAuthUnavailable, err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Every non-2xx is one refusal. A 403 for a missing scope and a 403 for a
	// secondary rate limit are deliberately not told apart: the browser-visible
	// answer is the same either way, and the status is in the log line for
	// whoever is diagnosing. What matters is that NONE of them can be mistaken
	// for an email problem, which would tell a user their address is unverified
	// when nothing about their address is known.
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		xlog.Warn(ctx, "github api call failed",
			xfield.String("path", path),
			xfield.Int("status", resp.StatusCode))

		return fmt.Errorf("%w: github api answered %d", apperr.ErrAuthUnavailable, resp.StatusCode)
	}

	if err := json.NewDecoder(io.LimitReader(resp.Body, maxResponseBytes)).Decode(out); err != nil {
		return fmt.Errorf("%w: decode github response: %w", apperr.ErrAuthUnavailable, err)
	}

	return nil
}
