package bootstrapauth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"

	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/config"
)

// generatedPasswordBytes is the entropy of a generated break-glass password.
// 16 bytes is 128 bits, which base64url renders as 22 characters — well above
// the 12-character floor validateBootstrapConfig applies to a human-chosen one.
const generatedPasswordBytes = 16

// ResolvePassword returns the break-glass password to run with: the configured
// one, or a freshly generated one when none is configured.
//
// It is called ONCE at startup, and the generated value is logged ONCE, here.
// Nothing downstream may log it again — not on a login, not on a failure, not
// at debug level — which is why resolution is separated from the comparison
// that uses it.
//
// Two properties of the generated path are operator-visible and are stated in
// the log line rather than left to be discovered:
//
//   - it is per-replica. `docker compose up --scale maintmode=N` gives each
//     replica its own password, so a credential read from one replica's log
//     works only when the load balancer routes there. A scaled deployment must
//     configure the password explicitly.
//   - it reaches the log store. Container stdout is scraped into Loki, so this
//     line is durable and queryable rather than ephemeral console output. It
//     should be replaced by a configured password after first use.
func ResolvePassword(ctx context.Context, cfg config.BootstrapConfig) (string, error) {
	if cfg.Password != "" {
		return cfg.Password, nil
	}

	raw := make([]byte, generatedPasswordBytes)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("generate bootstrap password: %w", err)
	}
	password := base64.RawURLEncoding.EncodeToString(raw)

	xlog.Warn(ctx,
		"no bootstrap password configured, generated one for this replica; "+
			"it is written to the log store, valid only for this replica and only until restart — "+
			"set bootstrap/password in the secrets file to pin a stable credential",
		xfield.String("bootstrap_password", password),
	)

	return password, nil
}
