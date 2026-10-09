package slack

import (
	"bytes"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
)

// formTokenField is the form field slack-go puts the bot token in.
const formTokenField = "token"

// doer is the client slack-go accepts through OptionHTTPClient.
type doer interface {
	Do(req *http.Request) (*http.Response, error)
}

// bearerClient moves the bot token out of the request body and into the
// Authorization header on every Slack Web API call.
//
// slack-go (v0.30) sends form-encoded methods — chat.postMessage among them —
// with the token as a `token=` form field. Slack accepts that but recommends
// `Authorization: Bearer`, and a token in the body is a token in anything that
// records bodies: a debugging proxy, a body-logging toggle, a request dump. In
// the header it is covered by the one redaction rule every outgoing client
// here already applies (xsanitize redacts Authorization).
//
// Done at the HTTP seam rather than per call, so every method the SDK sends —
// today's postMessage, any probe added later — gets the same treatment without
// remembering to.
type bearerClient struct {
	next  doer
	token string
}

func (c bearerClient) Do(req *http.Request) (*http.Response, error) {
	if err := stripFormToken(req); err != nil {
		return nil, err
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	return c.next.Do(req)
}

// stripFormToken removes the token field from a form-encoded body, keeping
// every other field, and rewires GetBody so a re-sent request carries the
// stripped body too. Non-form bodies (JSON, multipart) are left alone: slack-go
// already authenticates those with the header.
func stripFormToken(req *http.Request) error {
	if req.Body == nil || req.Body == http.NoBody {
		return nil
	}
	mediaType, _, err := mime.ParseMediaType(req.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/x-www-form-urlencoded" {
		return nil //nolint:nilerr // an unparseable content type is not a form; nothing to strip
	}

	raw, err := io.ReadAll(req.Body)
	_ = req.Body.Close()
	if err != nil {
		return fmt.Errorf("slack: read request body: %w", err)
	}

	values, err := url.ParseQuery(string(raw))
	if err != nil {
		return fmt.Errorf("slack: parse form body: %w", err)
	}
	values.Del(formTokenField)

	body := []byte(values.Encode())
	req.Body = io.NopCloser(bytes.NewReader(body))
	req.ContentLength = int64(len(body))
	req.GetBody = func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(body)), nil
	}

	return nil
}
