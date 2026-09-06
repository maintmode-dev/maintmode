package xcripto

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	validation "github.com/go-ozzo/ozzo-validation/v4"
	"golang.org/x/crypto/argon2"
)

// Argon2id parameters for new hashes. They are also written into every PHC
// string, so retuning them here does not strand the hashes already stored --
// verification reads the parameters out of the record it is checking.
//
// p is 1 deliberately. In x/crypto/argon2 the lanes partition the same arena
// rather than buying parallel speedup, so a value tied to host CPU count would
// only make the cost of a hash depend on which replica computed it -- a
// difference the PHC string would then pin forever.
const (
	argon2Memory      uint32 = 64 * 1024 // 64 MiB
	argon2Iterations  uint32 = 3
	argon2Parallelism uint8  = 1
	argon2SaltBytes   uint32 = 16
	argon2KeyBytes    uint32 = 32
)

// Password length bounds. Length only, no character classes (NIST 800-63B).
// The ceiling exists because the body is attacker-controlled and argon2id is
// deliberately expensive; 256 is far above any password a person types.
const (
	minPasswordLen = 12
	maxPasswordLen = 256
)

// Caps on parameters parsed out of a stored hash. A record is data, and a
// record claiming m=4 GiB would otherwise be a one-row out-of-memory: the
// verifier would faithfully allocate whatever the column asked for.
const (
	maxStoredMemory     uint32 = 1 << 21 // 2 GiB, far above anything this writes
	maxStoredIterations uint32 = 16
	maxStoredLanes      uint8  = 16
)

var (
	// ErrMalformedHash means the stored credential is not a hash this code can
	// check -- the wrong algorithm, a truncated record, or a sha256 digest that
	// reached the password column. It is deliberately distinct from "the
	// password did not match": one is a bad password, the other is a bug, and
	// collapsing them hides the bug forever.
	ErrMalformedHash = errors.New("malformed password hash")

	// ErrPasswordPolicy means the password is outside the accepted length.
	ErrPasswordPolicy = errors.New("password does not meet the length policy")
)

// phcPrefix is what a password hash must start with. The column also holds
// sha256 hex digests for one-time codes, so the algorithm has to be readable
// out of the value itself rather than inferred from which query found it.
const phcPrefix = "$argon2id$"

// ValidatePasswordPolicy applies the length rule. Every entry point that
// accepts a new password calls this one function; a second copy is how the two
// drift apart.
//
// The bound is in BYTES: ozzo's Length rule measures a string by len(), which
// is what the previous hand-written check did too, so this is a refactor and
// not a change in behavior. It does mean a non-ASCII password reaches the
// twelve-byte floor in fewer typed characters -- six Cyrillic letters pass.
// Accepted rather than corrected here: raising it to runes would reject
// passwords that already exist, which is a migration and not a cleanup.
func ValidatePasswordPolicy(password string) error {
	err := validation.Validate(password,
		validation.Required,
		validation.Length(minPasswordLen, maxPasswordLen),
	)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrPasswordPolicy, err)
	}

	return nil
}

// HashPassword derives an argon2id hash and returns it PHC-encoded. The salt is
// fresh per call, so hashing one password twice yields two different records.
//
// The parameters are written into the record rather than being implied by it,
// which is what lets VerifyPassword honor a hash minted under older settings.
func HashPassword(password string) (string, error) {
	salt := make([]byte, argon2SaltBytes)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generate password salt: %w", err)
	}

	key := argon2.IDKey([]byte(password), salt, argon2Iterations, argon2Memory, argon2Parallelism, argon2KeyBytes)

	return fmt.Sprintf(
		"%sv=%d$m=%d,t=%d,p=%d$%s$%s",
		phcPrefix,
		argon2.Version,
		argon2Memory,
		argon2Iterations,
		argon2Parallelism,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(key),
	), nil
}

// VerifyPassword reports whether password matches the stored PHC record.
//
// It returns ErrMalformedHash rather than a false match when the record is not
// a well-formed argon2id string. Parsing is strict on purpose: falling back to
// the current constants on a malformed field would silently check a password
// against the wrong work factor.
func VerifyPassword(stored, password string) (bool, error) {
	rec, err := parsePHC(stored)
	if err != nil {
		return false, err
	}

	got := argon2.IDKey([]byte(password), rec.salt, rec.iterations, rec.memory, rec.parallelism, argon2KeyBytes)

	return subtle.ConstantTimeCompare(got, rec.key) == 1, nil
}

// phcRecord is one parsed password hash: the cost parameters it was written
// with, plus the salt and derived key to check against.
type phcRecord struct {
	memory      uint32
	iterations  uint32
	parallelism uint8
	salt        []byte
	key         []byte
}

// parsePHC pulls the parameters, salt and derived key out of a PHC record.
func parsePHC(stored string) (phcRecord, error) {
	var rec phcRecord

	if !strings.HasPrefix(stored, phcPrefix) {
		return rec, fmt.Errorf("%w: not an argon2id record", ErrMalformedHash)
	}

	// "", "argon2id", "v=19", "m=..,t=..,p=..", salt, key
	parts := strings.Split(stored, "$")
	if len(parts) != 6 {
		return rec, fmt.Errorf("%w: expected 6 fields, got %d", ErrMalformedHash, len(parts))
	}

	if err := checkVersion(parts[2]); err != nil {
		return rec, err
	}

	memory, iterations, parallelism, err := parseParams(parts[3])
	if err != nil {
		return rec, err
	}

	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil || len(salt) == 0 {
		return rec, fmt.Errorf("%w: unreadable salt", ErrMalformedHash)
	}

	key, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil || len(key) == 0 {
		return rec, fmt.Errorf("%w: unreadable key", ErrMalformedHash)
	}

	return phcRecord{
		memory:      memory,
		iterations:  iterations,
		parallelism: parallelism,
		salt:        salt,
		key:         key,
	}, nil
}

// checkVersion rejects any record not written by the argon2 version this build
// links, rather than attempting a best-effort verification against it.
func checkVersion(field string) error {
	var version int
	if _, err := fmt.Sscanf(field, "v=%d", &version); err != nil {
		return fmt.Errorf("%w: unreadable version", ErrMalformedHash)
	}

	if version != argon2.Version {
		return fmt.Errorf("%w: unsupported version %d", ErrMalformedHash, version)
	}

	return nil
}

// parseParams reads the cost parameters and bounds them. The caps matter
// because the record is data: without them a row claiming a huge arena would be
// honored, and verifying one password would exhaust the process.
func parseParams(field string) (memory, iterations uint32, parallelism uint8, err error) {
	if _, err = fmt.Sscanf(field, "m=%d,t=%d,p=%d", &memory, &iterations, &parallelism); err != nil {
		return 0, 0, 0, fmt.Errorf("%w: unreadable parameters", ErrMalformedHash)
	}

	if memory == 0 || memory > maxStoredMemory ||
		iterations == 0 || iterations > maxStoredIterations ||
		parallelism == 0 || parallelism > maxStoredLanes {
		return 0, 0, 0, fmt.Errorf("%w: parameters out of range", ErrMalformedHash)
	}

	return memory, iterations, parallelism, nil
}
