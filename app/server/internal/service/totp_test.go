package service

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base32"
	"encoding/binary"
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateTOTPSecret(t *testing.T) {
	secret, err := GenerateTOTPSecret()
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(secret), 16)
}

func TestVerifyTOTP_RoundTrip(t *testing.T) {
	secret := "JBSWY3DPEHPK3PXP"
	now := time.Unix(1111111109, 0)
	code := computeTestHOTP(secret, now.Unix()/30)
	assert.True(t, VerifyTOTP(secret, code, now))
	assert.False(t, VerifyTOTP(secret, "000000", now))
	assert.False(t, VerifyTOTP(secret, "123", now))
}

func computeTestHOTP(secret string, counter int64) string {
	key, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(secret)
	if err != nil {
		key, _ = base32.StdEncoding.DecodeString(secret)
	}
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, uint64(counter))
	mac := hmac.New(sha1.New, key)
	_, _ = mac.Write(buf)
	sum := mac.Sum(nil)
	offset := sum[len(sum)-1] & 0x0f
	truncated := binary.BigEndian.Uint32(sum[offset:offset+4]) & 0x7fffffff
	code := truncated % uint32(math.Pow10(6))
	return fmt.Sprintf("%06d", code)
}
