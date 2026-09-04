package otp

import (
	"fmt"
	"html/template"
	"math"
	"strings"
	"time"

	"github.com/ruko1202/maintmode/internal/services/otp/templates"
)

// oneMinutePhrase is the singular expiry phrase. A constant because the package
// (tests included) repeats it enough for goconst to flag the literal.
const oneMinutePhrase = "1 minute"

const (
	// OTPEmailSubject is the subject of the code email. Exported because the
	// processor sends the message and this package owns the copy.
	OTPEmailSubject  = "Your MaintMode sign-in code"
	otpEmailTemplate = "otp_email.gohtml"
)

// otpEmailTmpl is the HTML code email, parsed once from the embedded templates.
// html/template, not text/template, and not string concatenation: the transport
// injects a rendered body into its branded frame as raw template.HTML, so a body
// that did not come from html/template would turn that frame into an
// HTML-injection sink. See notifytransport/email/layout.go.
var otpEmailTmpl = template.Must(
	template.ParseFS(templates.FS, otpEmailTemplate),
)

type otpEmailData struct {
	Code string
	// ExpiresIn is a human phrase for the code's lifetime, e.g. "5 minutes",
	// derived from the same TTL the credential row is stamped with so the copy
	// cannot contradict the actual expiry.
	ExpiresIn string
}

// RenderOTPEmail renders the code email body.
func RenderOTPEmail(code string, ttl time.Duration) (string, error) {
	var buf strings.Builder

	err := otpEmailTmpl.Execute(&buf, otpEmailData{
		Code:      code,
		ExpiresIn: expiresInPhrase(ttl),
	})
	if err != nil {
		return "", fmt.Errorf("render otp email: %w", err)
	}

	return strings.TrimSpace(buf.String()), nil
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
