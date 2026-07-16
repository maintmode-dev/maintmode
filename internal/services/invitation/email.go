package invitation

import (
	"context"
	"fmt"
	"html/template"
	"math"
	"strings"
	"time"

	"github.com/ruko1202/xlog"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/services/invitation/templates"
	"github.com/ruko1202/maintmode/internal/utils/xhash"
)

const (
	invitationEmailSubject  = "You've been invited to MaintMode"
	invitationEmailTemplate = "invitation_email.gohtml"
)

// invitationEmailTmpl is the HTML invitation body, parsed once from the embedded
// templates. The email transport derives a plain-text alternative from the
// rendered HTML, so this stays the single source. The template is deliberately
// minimal (plain semantic HTML, no branding). Branded email theming — a shared
// layout/wrapper across the invitation and notify emails — is intentionally
// deferred and not yet ticketed.
var invitationEmailTmpl = template.Must(
	template.ParseFS(templates.FS, invitationEmailTemplate),
)

// sendInvitationEmail enqueues the invite link for delivery over the email
// transport via the messaging outbox. It runs INSIDE the create/resend tx so the
// enqueue (a goque_task insert) commits atomically with the invitation — the
// actual SMTP delivery happens later in the auth task processor, off the request
// path. The idempotency key dedupes retries of the same invitation send.
func (s *Service) sendInvitationEmail(ctx context.Context, inv *entity.Invitation, link string) error {
	ctx, span := xlog.WithOperationSpan(ctx, "service.Invitation.sendInvitationEmail")
	defer span.End()

	body, err := renderInvitationEmail(link, s.ttl)
	if err != nil {
		return fmt.Errorf("render invitation email: %w", err)
	}

	if err := s.sender.SendAsync(ctx,
		entity.ProcessorTaskInvitationEmailSend,
		entity.NotifyTransportEmail,
		inv.Email,
		entity.NotifyMessage{
			Subject:     invitationEmailSubject,
			Body:        body,
			MessageMIME: entity.HTMLMessageMIME,
		},
		invitationEmailIdempotencyKey(inv),
	); err != nil {
		return fmt.Errorf("enqueue invitation email: %w", err)
	}

	return nil
}

// invitationEmailIdempotencyKey keys the outbox task on the invitation id and its
// current TokenHash. Create and each Resend mint a fresh random token (so a new
// TokenHash), making every send a distinct key while a retried request for the
// same send collapses onto one task. TokenHash is used instead of SentAt because
// it is unique per send regardless of clock precision — two resends in the same
// database-timestamp microsecond would otherwise collide and, since the enqueue
// is inside the invitation tx, a duplicate-key violation would roll the whole
// invitation back. Hashed via xhash to match the maint notify path.
func invitationEmailIdempotencyKey(inv *entity.Invitation) string {
	return xhash.HashSha256(fmt.Appendf(nil, "invitation-email:%s:%s", inv.ID, inv.TokenHash))
}

type invitationEmailData struct {
	Link string
	// ExpiresIn is a human phrase for the link lifetime, e.g. "7 days" or
	// "1 day", derived from the invitation TTL so the copy never contradicts the
	// actual expiry the service stamps on the invitation.
	ExpiresIn string
}

func renderInvitationEmail(link string, ttl time.Duration) (string, error) {
	var buf strings.Builder

	err := invitationEmailTmpl.Execute(&buf, invitationEmailData{
		Link:      link,
		ExpiresIn: expiresInPhrase(ttl),
	})
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(buf.String()), nil
}

// expiresInPhrase renders the invitation TTL as a whole-day phrase for the email
// copy. It rounds up so a sub-day remainder never understates the lifetime (a
// recipient told "1 day" for a 25h link is fine; "0 days" would be wrong), and
// floors at one day so a short TTL still reads sensibly. Days is the right unit:
// the default TTL is 7 days and the config is expressed in days.
func expiresInPhrase(ttl time.Duration) string {
	days := max(int(math.Ceil(ttl.Hours()/24)), 1)
	if days == 1 {
		return "1 day" //nolint:goconst
	}
	return fmt.Sprintf("%d days", days)
}
