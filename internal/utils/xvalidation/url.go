package xvalidation

import (
	"fmt"
	"net/url"

	validation "github.com/go-ozzo/ozzo-validation/v4"
)

// HTTPSURL refuses a URL that is not an absolute https one, that carries
// credentials in its userinfo, or that points at an address only the server
// itself can reach.
//
// The parts belong together because they answer the same question about the
// same field: is it safe to send a credential here, to believe what comes
// back, and to store this string. Plain http can be rewritten in flight; a
// private or loopback address turns an outbound fetch into a probe of the
// network the server sits on; userinfo puts a password in a cleartext column.
// Splitting them into separate rules would let a caller take one and forget
// the others, and the one they forget is the one that matters.
//
// No field name parameter: ozzo prefixes the field's json tag onto whatever a
// rule returns, so naming the field here renders it twice --
// "issuer_url: issuer_url must use https".
//
// An unparseable URL passes. is.URL is the rule that judges shape, and it runs
// beside this one; reporting the same mistake from two rules gives the operator
// two errors for one typo.
func HTTPSURL(value any) error {
	raw, ok := value.(string)
	if !ok {
		return fmt.Errorf("expected a url string, got %T", value)
	}

	if raw == "" {
		return nil // validation.Required reports an empty value; this judges form.
	}

	parsed, err := url.Parse(raw)
	if err != nil {
		return nil //nolint:nilerr // shape is judged by is.URL beside this rule
	}

	if parsed.Host == "" {
		return validation.NewError("validation_url_not_absolute", "must be an absolute url")
	}

	if parsed.Scheme != "https" {
		return validation.NewError("validation_url_not_https", "must use https")
	}

	// Userinfo is refused for where the value ENDS UP, not for how it looks. A
	// row's config is stored in cleartext -- only the secrets column is
	// encrypted -- so "https://user:pass@idp.example" writes a credential into a
	// plaintext column and hands it to every reader of the admin API. For an
	// issuer it is worse still: issuer_url is an input to the AAD of that
	// provider's client_secret, so the credential becomes part of the binding
	// and editing it later strands the secret.
	//
	// Nothing legitimate needs it here. A discovery document is public and the
	// client authenticates with client_id and client_secret, so userinfo in one
	// of these fields is a mistake, and the cheapest place to catch it is before
	// it is stored.
	//
	// The message deliberately does not echo the value: it is reported back
	// through the API and into logs, and repeating a credential in order to
	// complain about it is how it escapes.
	if parsed.User != nil {
		return validation.NewError("validation_url_has_userinfo",
			"must not carry credentials in the url")
	}

	return refuseInternalHost(raw)
}
