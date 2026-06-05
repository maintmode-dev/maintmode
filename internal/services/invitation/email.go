package invitation

import (
	"context"
	"fmt"
	"html/template"
	"strings"

	"github.com/ruko1202/xlog"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/services/invitation/templates"
	"github.com/ruko1202/maintmode/internal/utils/xhash"
)

const (
	invitationEmailSubject  = "You've been invited"
	invitationEmailTemplate = "invitation_email.gohtml"
)

// invitationEmailTmpl is the HTML invitation body, parsed once from the embedded
// templates. The email transport derives a plain-text alternative from the
// rendered HTML, so this stays the single source. Email-specific theming beyond
// this minimal wrapper is a separate ticket (RUK-155 scope note).
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

	body, err := renderInvitationEmail(link)
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
}

func renderInvitationEmail(link string) (string, error) {
	var buf strings.Builder

	err := invitationEmailTmpl.Execute(&buf, invitationEmailData{Link: link})
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(buf.String()), nil
}
