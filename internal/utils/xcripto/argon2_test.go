package xcripto

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// A sha256 hex digest in the password column must fail loudly rather than
// quietly comparing as a mismatch: a row written by the wrong hash function is a
// bug, not a bad password.
func TestVerifyPasswordRejectsASha256Digest(t *testing.T) {
	t.Parallel()

	ok, err := VerifyPassword(strings.Repeat("a", 64), "whatever")
	require.ErrorIs(t, err, ErrMalformedHash)
	require.False(t, ok)
}

// Parameters are read out of the stored string, so a hash written under other
// settings keeps verifying after the constants are retuned.
// A record written under OTHER parameters than the current constants must still
// verify, or retuning them would strand every hash already stored.
//
// The fixture is literal rather than generated: it is a real argon2id record of
// "legacy password" at m=19456,t=2,p=1 -- one of OWASP's other published
// configurations. Being a constant is the point. It stays valid when the
// package constants change, which is exactly the scenario under test, and it
// cannot silently start agreeing with them.
func TestVerifyPasswordHonoursStoredParameters(t *testing.T) {
	t.Parallel()

	const legacy = "$argon2id$v=19$m=19456,t=2,p=1$" +
		"v+Tg1zKdll8ln5IGsyc4Tw$3I1qqjgOW8G0+tL7swEkdlrWzrTD3HG8dIfjxB6sUzs"

	ok, err := VerifyPassword(legacy, "legacy password")
	require.NoError(t, err)
	require.True(t, ok, "a hash written under older parameters must keep verifying")

	ok, err = VerifyPassword(legacy, "not the legacy password")
	require.NoError(t, err)
	require.False(t, ok)
}

func TestVerifyPasswordRejectsMalformedHashes(t *testing.T) {
	t.Parallel()

	valid, err := HashPassword("anchor")
	require.NoError(t, err)

	cases := map[string]string{
		"empty":              "",
		"wrong algorithm":    "$argon2i$v=19$m=65536,t=3,p=1$c2FsdHNhbHRzYWx0c2E$aGFzaGhhc2hoYXNoaGFzaGhhc2hoYXNoaGFzaGhhcw",
		"unknown version":    "$argon2id$v=16$m=65536,t=3,p=1$c2FsdHNhbHRzYWx0c2E$aGFzaGhhc2hoYXNoaGFzaGhhc2hoYXNoaGFzaGhhcw",
		"missing field":      "$argon2id$v=19$m=65536,t=3$c2FsdHNhbHRzYWx0c2E$aGFzaGhhc2hoYXNo",
		"non-numeric memory": "$argon2id$v=19$m=abc,t=3,p=1$c2FsdHNhbHRzYWx0c2E$aGFzaGhhc2hoYXNo",
		"bad base64 salt":    "$argon2id$v=19$m=65536,t=3,p=1$!!!not-base64!!!$aGFzaGhhc2hoYXNo",
		"truncated":          valid[:len(valid)-10],
		"memory above cap":   "$argon2id$v=19$m=99999999,t=3,p=1$c2FsdHNhbHRzYWx0c2E$aGFzaGhhc2hoYXNo",
		"zero iterations":    "$argon2id$v=19$m=65536,t=0,p=1$c2FsdHNhbHRzYWx0c2E$aGFzaGhhc2hoYXNo",
	}

	for name, stored := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			ok, err := VerifyPassword(stored, "anchor")
			require.ErrorIs(t, err, ErrMalformedHash, "must not fall back to defaults")
			require.False(t, ok)
		})
	}
}

func TestValidatePasswordPolicy(t *testing.T) {
	t.Parallel()

	require.ErrorIs(t, ValidatePasswordPolicy(""), ErrPasswordPolicy)
	require.ErrorIs(t, ValidatePasswordPolicy(strings.Repeat("a", 11)), ErrPasswordPolicy)
	require.NoError(t, ValidatePasswordPolicy(strings.Repeat("a", 12)))
	require.NoError(t, ValidatePasswordPolicy(strings.Repeat("a", 256)))
	require.ErrorIs(t, ValidatePasswordPolicy(strings.Repeat("a", 257)), ErrPasswordPolicy)

	// The bound is in BYTES, which is what ozzo's Length measures and what the
	// hand-written check measured before it. Six Cyrillic letters are twelve
	// bytes and pass; five are ten and do not. Pinned so the unit is a decision
	// on record rather than a detail someone rediscovers.
	require.NoError(t, ValidatePasswordPolicy(strings.Repeat("п", 6)))
	require.ErrorIs(t, ValidatePasswordPolicy(strings.Repeat("п", 5)), ErrPasswordPolicy)
}
