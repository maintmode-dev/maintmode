package config

import (
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
)

// minS2SSecretLen is the floor for a service-to-service shared secret. The
// secret is machine-generated (openssl rand -hex 32 gives 64 characters), so
// the floor only has to reject a hand-typed or truncated value.
const minS2SSecretLen = 32

// publishedS2SSecret is the throwaway BFF secret committed in the dev, local and
// test samples. Those stacks run on it — the test stack as a "dev" environment —
// so only prod can refuse it.
const publishedS2SSecret = "local-bff-s2s-token-not-a-secret-0123456789" //nolint:gosec // published on purpose

// S2S authenticates one calling service — today the BFF — on the routes that
// carry RequireS2SToken. AvailableAPI lists the routes (as registered, e.g.
// /api/v1/refresh) this caller may use; the router refuses to start on an entry
// that is not a protected route.
type S2S struct {
	Secret       string   `mapstructure:"secret"`
	AvailableAPI []string `mapstructure:"available_api"`
}

// S2SConfig maps a calling service's name (the X-S2S-APPNAME value) to its
// credentials.
type S2SConfig = map[string]S2S

// validateS2SConfig refuses a configuration that would make every protected
// route answer 401 — no client at all, a client allowed nowhere — or that
// would authenticate with a weak secret, a placeholder, or (in prod) the value
// published in this repository. Each of these is a total sign-in outage or a
// hole, and both are better found at boot.
func (c *AppConfig) validateS2SConfig() error {
	if len(c.S2S) == 0 {
		return errors.New("s2s: no client configured; the token endpoints would refuse every caller")
	}

	var errs []error
	for _, name := range slices.Sorted(maps.Keys(c.S2S)) {
		client := c.S2S[name]
		switch {
		case len(client.Secret) < minS2SSecretLen:
			errs = append(errs, fmt.Errorf("s2s.%s.secret is too short: %d characters, expected at least %d",
				name, len(client.Secret), minS2SSecretLen))
		case strings.Contains(client.Secret, secretPlaceholderMarker):
			errs = append(errs, fmt.Errorf("s2s.%s.secret is a placeholder: set a real value in the secrets file", name))
		case c.Environment.IsProd() && client.Secret == publishedS2SSecret:
			errs = append(errs, fmt.Errorf("s2s.%s.secret is the value published in this repository: "+
				"generate one with openssl rand -hex 32", name))
		case strings.TrimSpace(client.Secret) != client.Secret:
			// A trailing newline from a secret store makes the two sides differ
			// while both look right in an editor.
			errs = append(errs, fmt.Errorf("s2s.%s.secret has leading or trailing whitespace", name))
		}

		if len(client.AvailableAPI) == 0 {
			errs = append(errs, fmt.Errorf("s2s.%s.available_api is empty", name))
		}
	}

	return errors.Join(errs...)
}
