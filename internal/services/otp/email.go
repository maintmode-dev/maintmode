package otp

import (
	"fmt"
	"html/template"
	"math"
	"strings"
	"time"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/services/otp/templates"
)

// oneMinutePhrase is the singular expiry phrase. A constant because the package
// (tests included) repeats it enough for goconst to flag the literal.
const oneMinutePhrase = "1 minute"

// codeEmail is one purpose's copy: a subject and the template for the body.
type codeEmail struct {
	subject  string
	template string
}

// codeEmails holds the copy per purpose. A reset code gets its own because the
// sign-in copy tells someone who did not ask to "safely ignore" it -- the wrong
// advice when what somebody is attempting is to replace their password.
var codeEmails = map[entity.OTPPurpose]codeEmail{
	entity.OTPPurposeSignIn: {
		subject:  "Your MaintMode sign-in code",
		template: "otp_email.gohtml",
	},
	entity.OTPPurposePasswordReset: {
		subject:  "Your MaintMode password reset code",
		template: "password_reset_email.gohtml",
	},
}

// codeEmailTmpl holds every code email, parsed once from the embedded
// templates. html/template, not text/template, and not string concatenation:
// the transport injects a rendered body into its branded frame as raw
// template.HTML, so a body that did not come from html/template would turn that
// frame into an HTML-injection sink. See notifytransport/email/layout.go.
var codeEmailTmpl = template.Must(
	template.ParseFS(templates.FS, "*.gohtml"),
)

type otpEmailData struct {
	Code string
	// ExpiresIn is a human phrase for the code's lifetime, e.g. "5 minutes",
	// derived from the same TTL the credential row is stamped with so the copy
	// cannot contradict the actual expiry.
	ExpiresIn string
}

// RenderOTPEmail renders the code email for a purpose, returning its subject
// and body. An empty purpose is a sign-in code: tasks queued before the purpose
// existed carry none, and every one of them was a sign-in.
func RenderOTPEmail(purpose entity.OTPPurpose, code string, ttl time.Duration) (subject, body string, err error) {
	if purpose == "" {
		purpose = entity.OTPPurposeSignIn
	}

	copyFor, ok := codeEmails[purpose]
	if !ok {
		return "", "", fmt.Errorf("render otp email: unknown purpose %q", purpose)
	}

	var buf strings.Builder

	err = codeEmailTmpl.ExecuteTemplate(&buf, copyFor.template, otpEmailData{
		Code:      code,
		ExpiresIn: expiresInPhrase(ttl),
	})
	if err != nil {
		return "", "", fmt.Errorf("render otp email: %w", err)
	}

	return copyFor.subject, strings.TrimSpace(buf.String()), nil
}

// expiresInPhrase renders a code TTL as a whole-minute phrase.
//
// The invitation email has a helper of the same shape that rounds to whole days;
// it is not reusable here, because every code lifetime this service issues would
// floor to its "1 day" minimum and the copy would be wrong by orders of
// magnitude. Rounding is up, so a sub-minute remainder never understates the
// lifetime, and the floor is one minute so a very short TTL still reads sensibly.
func expiresInPhrase(ttl time.Duration) string {
	minutes := max(int(math.Ceil(ttl.Minutes())), 1)
	if minutes == 1 {
		return oneMinutePhrase
	}
	return fmt.Sprintf("%d minutes", minutes)
}
