package invitation

import (
	"context"
	"fmt"

	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/utils/xtime"

	"github.com/ruko1202/maintmode/internal/entity"
)

const (
	invitationEmailSubject = "You've been invited"
	// todo replace with a template
	invitationEmailMessageTemplate = `
You've been invited to join MaintMode.
MaintMode is a new way to manage your maintenance status.

	Accept your invitation: %s

If you don't know who you are, you can ignore this email.
`
)

// sendInvitationEmail delivers the invite link to the recipient over the email
// transport. It runs after the invitation is committed, so a delivery failure
// is logged but not propagated — an admin can always resend.
//
// The call is synchronous, which is fine today because the email transport is a
// stub (a log write). When real delivery (SMTP/SES) lands it will perform
// network I/O on the request path; at that point move it off the request —
// dispatch via the messaging scheduler / a background worker — so the API
// response isn't blocked by the mail server.
func (s *Service) sendInvitationEmail(ctx context.Context, inv *entity.Invitation, link string) {
	tr, err := s.notifyTransport.Get(ctx, entity.NotifyTransportEmail)
	if err != nil {
		xlog.Error(ctx, "get email transport", xfield.Error(err))
		return
	}

	err = tr.Send(ctx, inv.Email, entity.NotifyMessage{
		Subject: invitationEmailSubject,
		Body:    fmt.Sprintf(invitationEmailMessageTemplate, link),
	})
	if err != nil {
		xlog.Error(ctx, "send invitation email failed", xfield.Error(err))
		return
	}

	if err := s.store.SendAt(ctx, inv.ID, xtime.UTCNow()); err != nil {
		xlog.Error(ctx, "sendAt failed", xfield.Error(err))
	}
}
