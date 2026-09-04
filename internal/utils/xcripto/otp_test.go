package xcripto

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"io"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/utils/xhash"
)

func TestGenerateOTPCode_ShapeAndDigest(t *testing.T) {
	t.Parallel()

	for range 100 {
		code, hash, err := GenerateOTPCode()
		require.NoError(t, err)

		// Six characters always, including the ~10% of values below 100000 that
		// an unpadded format would render shorter.
		require.Len(t, code, otpCodeDigits)
		_, err = strconv.Atoi(code)
		require.NoError(t, err, "code must be decimal digits only: %q", code)

		// The digest is of the padded string the user will type back, not of the
		// number behind it -- the contract verification depends on.
		require.Equal(t, xhash.HashSha256([]byte(code)), hash)

		raw, err := hex.DecodeString(hash)
		require.NoError(t, err, "hash must be hex")
		require.Len(t, raw, 32, "hash must be sha256")
	}
}

// A small value must still render as six characters. Drawn deliberately rather
// than hoped for: at 1-in-10 odds a random run would usually cover it, and
// "usually" is how a padding bug reaches production.
func TestGenerateOTPCode_PadsSmallValues(t *testing.T) {
	t.Parallel()

	code, hash, err := generateOTPCode(bytes.NewReader(uint32Bytes(42)))
	require.NoError(t, err)
	require.Equal(t, "000042", code)
	require.Equal(t, xhash.HashSha256([]byte("000042")), hash)
}

// A sample at or above the rejection ceiling must be discarded, not folded. The
// first word here would bias low digits under a plain modulo; the generator must
// skip it and use the second.
func TestGenerateOTPCode_RejectsBiasedSamples(t *testing.T) {
	t.Parallel()

	entropy := bytes.NewReader(append(
		uint32Bytes(otpRejectionCeiling), // must be rejected
		uint32Bytes(7)...,                // must be used
	))

	code, _, err := generateOTPCode(entropy)
	require.NoError(t, err)
	require.Equal(t, "000007", code)
}

func TestGenerateOTPCode_EntropyFailure(t *testing.T) {
	t.Parallel()

	_, _, err := generateOTPCode(bytes.NewReader(nil))
	require.ErrorIs(t, err, io.EOF)
}

func uint32Bytes(v uint32) []byte {
	b := make([]byte, 4)
	binary.BigEndian.PutUint32(b, v)
	return b
}
